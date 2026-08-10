package hermes

import (
	"fmt"
	"time"
)

func (d *HermesEngine) pause_task(task_id int) {
	_ = d.store.UpdateStatus(task_id, TaskStatusPaused)
	_ = d.store.DeactivateConnections(task_id)
	d.logger.Info().Int("task_id", task_id).Msg("task paused")
	d.emit(EventPaused, TaskPausedEventData{TaskID: task_id})
	d.delete_tracker(task_id)
}

func (d *HermesEngine) fail_task(task_id int, err_msg string) {
	_ = d.store.UpdateStatus(task_id, TaskStatusFailed)
	_ = d.store.DeactivateConnections(task_id)
	_ = d.store.RecordError(task_id, err_msg)
	d.logger.Error().Int("task_id", task_id).Str("error", err_msg).Msg("task failed")
	d.emit(EventFailed, TaskFailedEventData{TaskID: task_id, Error: err_msg})
	d.delete_tracker(task_id)
}

func (d *HermesEngine) emit(event EventType, data EventData) {
	d.event_mu.Lock()
	if d.replay_events {
		d.event_history = append(d.event_history, event_record{event: event, data: data})
	}
	handler := d.on_event
	d.event_mu.Unlock()
	if handler != nil {
		handler(event, data)
	}
}

// emitProgress emits an EventProgress event with the current in-memory
// progress snapshot, avoiding any database round-trip.
func (d *HermesEngine) emit_progress(task_id int) {
	p := d.snapshot_progress(task_id)
	if p == nil {
		d.logger.Info().Int("task_id", task_id).Msg("progress: skip emit (no change)")
		return
	}
	msg := "progress: emit to handler"
	if p.Keepalive {
		msg = "progress: emit keepalive"
	}
	d.logger.Info().
		Int("task_id", task_id).
		Int64("downloaded", p.Downloaded).
		Int64("total_size", p.TotalSize).
		Int64("speed", p.Speed).
		Float64("pct", float64(p.Downloaded)*100/float64(max_int64(1, p.TotalSize))).
		Msg(msg)
	d.event_mu.RLock()
	handler := d.on_event
	d.event_mu.RUnlock()
	if handler != nil {
		handler(EventProgress, TaskProgressEventData{TaskID: task_id, Progress: p})
	}
}

// CurrentProgress returns the latest in-memory progress for a task without
// changing broadcast throttling state. It is intended for read APIs that want
// fresh progress without querying persisted segment/connection tables.
func (d *HermesEngine) CurrentProgress(task_id int) *TaskProgress {
	return d.current_progress(task_id, false)
}

// snapshotProgress builds a TaskProgress from the in-memory tracker.
// Speed values come from the download loop (copyReader/downloadSegment) which
// computes speed every progressInterval. The periodic ticker decouples
// emission frequency from the download loop, making it independently configurable
// via ProgressEmitInterval without losing speed accuracy.
// Returns nil if no progress tracker exists for the given task.
func (d *HermesEngine) snapshot_progress(task_id int) *TaskProgress {
	return d.current_progress(task_id, true)
}

func (d *HermesEngine) current_progress(task_id int, mark_emit bool) *TaskProgress {
	d.progress_mu.Lock()
	tracker, ok := d.progress_cache[task_id]
	d.progress_mu.Unlock()
	if !ok {
		return nil
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	var total_downloaded int64
	var total_speed int64
	p := &TaskProgress{
		Resources: make([]ResourceProgress, 0, len(tracker.resources)),
	}
	for _, r_id := range tracker.order {
		r := tracker.resources[r_id]
		p.TotalSize += r.size
		p.Downloaded += r.downloaded
		p.Speed += r.speed
		total_downloaded += r.downloaded
		total_speed += r.speed
		p.ResourceCount++
		p.Resources = append(p.Resources, ResourceProgress{
			ID:         r_id,
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
	// shorter than the segment progress reporting interval.
	// However, if >500ms has passed since the last real emission, emit anyway so
	// the frontend knows the download is still alive (e.g. during segment
	// connection establishment which can take 1-5 seconds).
	if mark_emit {
		is_keepalive := false
		if total_downloaded == tracker.last_emit_downloaded && total_speed == tracker.last_emit_speed && total_downloaded != p.TotalSize {
			if time.Since(tracker.last_emit_time) < 500*time.Millisecond {
				return nil
			}
			is_keepalive = true
		}
		p.Keepalive = is_keepalive
		tracker.last_emit_downloaded = total_downloaded
		tracker.last_emit_speed = total_speed
		tracker.last_emit_time = time.Now()
	}
	return p
}

// initTracker creates a new progress tracker for the given task.
func (d *HermesEngine) init_tracker(task_id int, resource_sizes map[int]int64, resources []ResourceJob) {
	tracker := &progress_tracker{
		resources: make(map[int]*resource_tracker),
		order:     make([]int, 0, len(resources)),
	}
	for _, r := range resources {
		sz := resource_sizes[r.ID]
		tracker.resources[r.ID] = &resource_tracker{
			size: sz,
			name: r.Name,
			kind: r.Kind,
			typ:  r.Type,
		}
		tracker.order = append(tracker.order, r.ID)
	}
	d.progress_mu.Lock()
	d.progress_cache[task_id] = tracker
	d.progress_mu.Unlock()
}

// updateTracker updates a resource's downloaded bytes and speed in the in-memory tracker.
// speed comes from the download loop (copyReader/downloadSegment) computed at progressInterval.
func (d *HermesEngine) update_tracker(task_id, resource_id int, downloaded, speed int64) {
	d.progress_mu.Lock()
	tracker, ok := d.progress_cache[task_id]
	d.progress_mu.Unlock()
	if !ok {
		return
	}
	tracker.mu.Lock()
	if r, ok := tracker.resources[resource_id]; ok {
		r.downloaded = downloaded
		r.speed = speed
	}
	tracker.mu.Unlock()
}

// updateTrackerSize updates a resource's total size in the in-memory tracker.
// Called when Prepare discovers the actual file size during download, ensuring
// the WebSocket progress push reflects accurate size even for single-resource tasks.
func (d *HermesEngine) update_tracker_size(task_id, resource_id int, size int64) {
	d.progress_mu.Lock()
	tracker, ok := d.progress_cache[task_id]
	d.progress_mu.Unlock()
	if !ok {
		return
	}
	tracker.mu.Lock()
	if r, ok := tracker.resources[resource_id]; ok {
		r.size = size
	}
	tracker.mu.Unlock()
}

// deleteTracker removes the progress tracker for a finished/failed task.
func (d *HermesEngine) delete_tracker(task_id int) {
	d.progress_mu.Lock()
	delete(d.progress_cache, task_id)
	d.progress_mu.Unlock()
}

func max_int64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func calc_speed(t0 time.Time, downloaded0 int64, t1 time.Time, downloaded1 int64) int64 {
	elapsed := t1.Sub(t0).Seconds()
	if elapsed <= 0 || downloaded1 <= downloaded0 {
		return 0
	}
	return int64(float64(downloaded1-downloaded0) / elapsed)
}

// progressSpeedSampler keeps the most recent speed calculated across a stable
// sampling window. Reads commonly complete in microseconds, so calculating a
// rate from every individual read greatly exaggerates normal burstiness.
type progress_speed_sampler struct {
	sampled_at time.Time
	downloaded int64
	speed      int64
}

func new_progress_speed_sampler(now time.Time, downloaded int64) *progress_speed_sampler {
	return &progress_speed_sampler{sampled_at: now, downloaded: downloaded}
}

func (s *progress_speed_sampler) sample(now time.Time, downloaded int64) int64 {
	if now.Sub(s.sampled_at) < progress_interval {
		return s.speed
	}
	s.speed = calc_speed(s.sampled_at, s.downloaded, now, downloaded)
	s.sampled_at = now
	s.downloaded = downloaded
	return s.speed
}

// formatSpeed formats a speed value (bytes/sec) into a human-readable string.
func format_speed(bytes_per_sec int64) string {
	if bytes_per_sec <= 0 {
		return "0 B/s"
	}
	// Keep it simple: KB/s or MB/s
	if bytes_per_sec >= 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", float64(bytes_per_sec)/(1024*1024))
	}
	return fmt.Sprintf("%.0f KB/s", float64(bytes_per_sec)/1024)
}

// formatSize formats a byte count into a human-readable string.
func format_size(bytes int64) string {
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
