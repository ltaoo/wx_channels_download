package hermes

import (
	"fmt"
	"time"
)

func (d *HermesEngine) pauseTask(taskID int) {
	_ = d.store.UpdateStatus(taskID, TaskStatusPaused)
	_ = d.store.DeactivateConnections(taskID)
	d.logger.Info().Int("taskID", taskID).Msg("task paused")
	d.emit(taskID, EventPaused)
	d.deleteTracker(taskID)
}

func (d *HermesEngine) failTask(taskID int, errMsg string) {
	_ = d.store.UpdateStatus(taskID, TaskStatusFailed)
	_ = d.store.DeactivateConnections(taskID)
	_ = d.store.RecordError(taskID, errMsg)
	d.logger.Error().Int("taskID", taskID).Str("error", errMsg).Msg("task failed")
	d.emit(taskID, EventFailed)
	d.deleteTracker(taskID)
}

func (d *HermesEngine) emit(taskID int, event EventType) {
	d.eventMu.RLock()
	handler := d.onEvent
	d.eventMu.RUnlock()
	if handler != nil {
		handler(taskID, event, nil)
	}
}

// emitProgress emits an EventProgress event with the current in-memory
// progress snapshot, avoiding any database round-trip.
func (d *HermesEngine) emitProgress(taskID int) {
	p := d.snapshotProgress(taskID)
	if p == nil {
		d.logger.Info().Int("taskID", taskID).Msg("progress: skip emit (no change)")
		return
	}
	msg := "progress: emit to handler"
	if p.Keepalive {
		msg = "progress: emit keepalive"
	}
	d.logger.Info().
		Int("taskID", taskID).
		Int64("downloaded", p.Downloaded).
		Int64("totalSize", p.TotalSize).
		Int64("speed", p.Speed).
		Float64("pct", float64(p.Downloaded)*100/float64(maxInt64(1, p.TotalSize))).
		Msg(msg)
	d.eventMu.RLock()
	handler := d.onEvent
	d.eventMu.RUnlock()
	if handler != nil {
		handler(taskID, EventProgress, p)
	}
}

// snapshotProgress builds a TaskProgress from the in-memory tracker.
// Speed values come from the download loop (copyReader/downloadSegment) which
// computes speed every progressInterval (500ms). The periodic ticker decouples
// emission frequency from the download loop, making it independently configurable
// via ProgressEmitInterval without losing speed accuracy.
// Returns nil if no progress tracker exists for the given task.
func (d *HermesEngine) snapshotProgress(taskID int) *TaskProgress {
	d.progressMu.Lock()
	tracker, ok := d.progressCache[taskID]
	d.progressMu.Unlock()
	if !ok {
		return nil
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	var totalDownloaded int64
	var totalSpeed int64
	p := &TaskProgress{
		Resources: make([]ResourceProgress, 0, len(tracker.resources)),
	}
	for _, rID := range tracker.order {
		r := tracker.resources[rID]
		p.TotalSize += r.size
		p.Downloaded += r.downloaded
		p.Speed += r.speed
		totalDownloaded += r.downloaded
		totalSpeed += r.speed
		p.ResourceCount++
		p.Resources = append(p.Resources, ResourceProgress{
			ID:         rID,
			Name:       r.name,
			Kind:       r.kind,
			Type:       r.typ,
			Size:       r.size,
			Downloaded: r.downloaded,
			Speed:      r.speed,
		})
	}
	// Skip emission if downloaded and speed haven't changed since last broadcast.
	// This prevents duplicate WS pushes when the progress emit interval (180ms) is
	// shorter than the segment progress reporting interval (500ms).
	// However, if >500ms has passed since the last real emission, emit anyway so
	// the frontend knows the download is still alive (e.g. during segment
	// connection establishment which can take 1-5 seconds).
	isKeepalive := false
	if totalDownloaded == tracker.lastEmitDownloaded && totalSpeed == tracker.lastEmitSpeed && totalDownloaded != p.TotalSize {
		if time.Since(tracker.lastEmitTime) < 500*time.Millisecond {
			return nil
		}
		isKeepalive = true
	}
	p.Keepalive = isKeepalive
	tracker.lastEmitDownloaded = totalDownloaded
	tracker.lastEmitSpeed = totalSpeed
	tracker.lastEmitTime = time.Now()
	return p
}

// initTracker creates a new progress tracker for the given task.
func (d *HermesEngine) initTracker(taskID int, resourceSizes map[int]int64, resources []Resource) {
	tracker := &progressTracker{
		resources: make(map[int]*resourceTracker),
		order:     make([]int, 0, len(resources)),
	}
	for _, r := range resources {
		sz := resourceSizes[r.ID]
		tracker.resources[r.ID] = &resourceTracker{
			size: sz,
			name: r.Name,
			kind: r.Kind,
			typ:  r.Type,
		}
		tracker.order = append(tracker.order, r.ID)
	}
	d.progressMu.Lock()
	d.progressCache[taskID] = tracker
	d.progressMu.Unlock()
}

// updateTracker updates a resource's downloaded bytes and speed in the in-memory tracker.
// speed comes from the download loop (copyReader/downloadSegment) computed at progressInterval (500ms).
func (d *HermesEngine) updateTracker(taskID, resourceID int, downloaded, speed int64) {
	d.progressMu.Lock()
	tracker, ok := d.progressCache[taskID]
	d.progressMu.Unlock()
	if !ok {
		return
	}
	tracker.mu.Lock()
	if r, ok := tracker.resources[resourceID]; ok {
		r.downloaded = downloaded
		r.speed = speed
	}
	tracker.mu.Unlock()
}

// updateTrackerSize updates a resource's total size in the in-memory tracker.
// Called when Prepare discovers the actual file size during download, ensuring
// the WebSocket progress push reflects accurate size even for single-resource tasks.
func (d *HermesEngine) updateTrackerSize(taskID, resourceID int, size int64) {
	d.progressMu.Lock()
	tracker, ok := d.progressCache[taskID]
	d.progressMu.Unlock()
	if !ok {
		return
	}
	tracker.mu.Lock()
	if r, ok := tracker.resources[resourceID]; ok {
		r.size = size
	}
	tracker.mu.Unlock()
}

// deleteTracker removes the progress tracker for a finished/failed task.
func (d *HermesEngine) deleteTracker(taskID int) {
	d.progressMu.Lock()
	delete(d.progressCache, taskID)
	d.progressMu.Unlock()
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// buildResourceMeta builds a simplified ResourceMeta from resource.Extra and task config.
// resource.Extra is populated by each platform's BuildDownloadTask; fields vary by platform.
func calcSpeed(t0 time.Time, downloaded0 int64, t1 time.Time, downloaded1 int64) int64 {
	elapsed := t1.Sub(t0).Seconds()
	if elapsed <= 0 || downloaded1 <= downloaded0 {
		return 0
	}
	return int64(float64(downloaded1-downloaded0) / elapsed)
}

// progressSpeedSampler keeps the most recent speed calculated across a stable
// sampling window. Reads commonly complete in microseconds, so calculating a
// rate from every individual read greatly exaggerates normal burstiness.
type progressSpeedSampler struct {
	sampledAt  time.Time
	downloaded int64
	speed      int64
}

func newProgressSpeedSampler(now time.Time, downloaded int64) *progressSpeedSampler {
	return &progressSpeedSampler{sampledAt: now, downloaded: downloaded}
}

func (s *progressSpeedSampler) Sample(now time.Time, downloaded int64) int64 {
	if now.Sub(s.sampledAt) < progressInterval {
		return s.speed
	}
	s.speed = calcSpeed(s.sampledAt, s.downloaded, now, downloaded)
	s.sampledAt = now
	s.downloaded = downloaded
	return s.speed
}

// formatSpeed formats a speed value (bytes/sec) into a human-readable string.
func formatSpeed(bytesPerSec int64) string {
	if bytesPerSec <= 0 {
		return "0 B/s"
	}
	// Keep it simple: KB/s or MB/s
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", float64(bytesPerSec)/(1024*1024))
	}
	return fmt.Sprintf("%.0f KB/s", float64(bytesPerSec)/1024)
}

// formatSize formats a byte count into a human-readable string.
func formatSize(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// getConfigBool reads the value of a specified boolean key from the task's config JSON.
