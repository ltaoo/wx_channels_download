package hermes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

func (d *HermesEngine) downloadFile(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	filePath string,
	task *TaskJob,
	resource *ResourceJob,
	prepared PreparedResource,
) error {
	resourceID := resource.ID
	taskID := task.ID
	segments, err := d.store.LoadSegmentInfo(resourceID)
	if err != nil {
		return fmt.Errorf("failed to load segment information: %w", err)
	}
	ranges := []SegmentRange{{Index: 0, OffsetStart: 0, OffsetEnd: maxInt64(0, prepared.Size-1), Size: prepared.Size}}
	if !segmentsMatchRanges(segments, ranges) {
		ids, err := d.store.CreateSegments(resourceID, endpoint.URL, ranges)
		if err != nil {
			return fmt.Errorf("failed to create segment records: %w", err)
		}
		if len(ids) != 1 {
			return errors.New("failed to create segment records: incorrect number of IDs returned")
		}
		segments = []Segment{{ID: ids[0], Index: 0, URL: endpoint.URL, Size: prepared.Size, OffsetEnd: ranges[0].OffsetEnd}}
	}
	segment := &segments[0]
	partPath := filePath + partialFileSuffix

	// A crash may happen after persisting completion but before the atomic rename.
	// Prefer the durable partial file over an older same-sized destination.
	if prepared.Size > 0 && segment.Downloaded == prepared.Size && fileHasSize(partPath, prepared.Size) {
		if err := finalizePartialFile(partPath, filePath); err != nil {
			return fmt.Errorf("failed to finalize temporary file: %w", err)
		}
		d.updateTracker(taskID, resourceID, prepared.Size, 0)
		return nil
	}
	// A same-sized destination alone is not proof that this resource completed.
	// Only persisted completion state may make an existing destination reusable.
	if prepared.Size > 0 && segment.Downloaded == prepared.Size && fileHasSize(filePath, prepared.Size) {
		d.updateTracker(taskID, resourceID, prepared.Size, 0)
		return nil
	}

	var downloaded int64
	if fi, statErr := os.Stat(partPath); statErr == nil {
		downloaded = fi.Size()
	}
	if prepared.Size > 0 && (downloaded < 0 || downloaded > prepared.Size) {
		downloaded = 0
		if err := os.Truncate(partPath, 0); err != nil {
			return fmt.Errorf("failed to reset temporary file: %w", err)
		}
	}
	if segment.Downloaded != downloaded {
		segment.Downloaded = downloaded
		if err := d.store.UpdateSegmentProgress(segment.ID, downloaded); err != nil {
			return fmt.Errorf("failed to reconcile segment progress: %w", err)
		}
	}
	if !prepared.SupportsRange {
		downloaded = 0
	}

	for attempt := 0; attempt < maxReadAttempts; attempt++ {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		useRange := prepared.SupportsRange && downloaded > 0
		request := ReadRequest{OffsetStart: downloaded, OffsetEnd: prepared.Size - 1, UseRange: useRange}
		reader, err := driver.Open(ctx, endpoint, request)
		if err != nil {
			if !waitForRetry(ctx, attempt) {
				return context.Cause(ctx)
			}
			if attempt == maxReadAttempts-1 {
				return fmt.Errorf("failed to open download source: %w", err)
			}
			continue
		}

		flags := os.O_CREATE | os.O_WRONLY
		if useRange {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
			downloaded = 0
		}
		file, openErr := os.OpenFile(partPath, flags, 0644)
		if openErr != nil {
			reader.Close()
			return fmt.Errorf("failed to open temporary file: %w", openErr)
		}

		lastPersist := time.Now()
		err = d.copyReader(ctx, reader, file, prepared.Size, &downloaded, taskID, resourceID, func(total, speed int64, force bool) error {
			resource.Downloaded = total
			resource.Speed = speed
			d.updateTracker(taskID, resourceID, total, speed)
			if !force && time.Since(lastPersist) < progressInterval {
				return nil
			}
			lastPersist = time.Now()
			return d.persistProgress(taskID, resourceID, segment.ID, total, speed)
		})
		readerCloseErr := reader.Close()
		if err == nil && readerCloseErr != nil {
			err = readerCloseErr
		}
		if err == nil && (prepared.Size <= 0 || downloaded == prepared.Size) {
			if syncErr := file.Sync(); syncErr != nil {
				err = syncErr
			} else if persistErr := d.persistProgress(taskID, resourceID, segment.ID, downloaded, 0); persistErr != nil {
				err = persistErr
			} else {
				resource.Downloaded = downloaded
				resource.Speed = 0
			}
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil && (prepared.Size <= 0 || downloaded == prepared.Size) {
			if renameErr := os.Rename(partPath, filePath); renameErr != nil {
				return fmt.Errorf("failed to commit downloaded file: %w", renameErr)
			}
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		if prepared.Size > 0 && downloaded >= prepared.Size {
			return fmt.Errorf("downloaded data size mismatch: expected %d bytes, got %d bytes", prepared.Size, downloaded)
		}
		if attempt == maxReadAttempts-1 {
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			return fmt.Errorf("download read failed: %w", err)
		}
		if !prepared.SupportsRange {
			downloaded = 0
		}
		if !waitForRetry(ctx, attempt) {
			return context.Cause(ctx)
		}
	}
	return io.ErrUnexpectedEOF
}

func (d *HermesEngine) copyReader(
	ctx context.Context,
	reader io.Reader,
	writer io.Writer,
	expectedSize int64,
	downloaded *int64,
	taskID int,
	resourceID int,
	onProgress func(total, speed int64, force bool) error,
) error {
	buf := make([]byte, readBufferSize)
	speedSampler := newProgressSpeedSampler(time.Now(), *downloaded)
	lastLog := time.Now()
	lastLogDownloaded := *downloaded
	for {
		chunkStart := time.Now()
		if err := context.Cause(ctx); err != nil {
			if progressErr := onProgress(*downloaded, 0, true); progressErr != nil {
				return progressErr
			}
			return err
		}
		readBuf := buf
		if expectedSize > 0 {
			remaining := expectedSize - *downloaded
			if remaining == 0 {
				return nil
			}
			if remaining < int64(len(readBuf)) {
				readBuf = readBuf[:remaining]
			}
		}
		n, readErr := reader.Read(readBuf)
		if n > 0 {
			if _, err := writer.Write(readBuf[:n]); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			*downloaded += int64(n)
			if d.cfg.SpeedLimit > 0 {
				expected := time.Duration(float64(n) / float64(d.cfg.SpeedLimit) * float64(time.Second))
				elapsed := time.Since(chunkStart)
				if expected > elapsed {
					select {
					case <-ctx.Done():
						return context.Cause(ctx)
					case <-time.After(expected - elapsed):
					}
				}
			}
		}
		now := time.Now()
		// A single Read can finish in a few microseconds. Extrapolating that one
		// 32 KiB block to bytes/second produces meaningless GB/s spikes, so only
		// refresh the displayed speed after a representative sampling window.
		speed := speedSampler.Sample(now, *downloaded)
		if err := onProgress(*downloaded, speed, false); err != nil {
			return err
		}
		// Progress log every 3 seconds for diagnostics
		if now.Sub(lastLog) >= progressLogInterval {
			if expectedSize > 0 {
				pct := float64(*downloaded) * 100 / float64(expectedSize)
				logSpeed := calcSpeed(lastLog, lastLogDownloaded, now, *downloaded)
				d.logger.Info().
					Int("taskID", taskID).
					Int("resourceID", resourceID).
					Int64("downloaded", *downloaded).
					Int64("totalSize", expectedSize).
					Float64("percent", pct).
					Str("speed", formatSpeed(logSpeed)).
					Msg("download progress")
			} else {
				d.logger.Info().
					Int("taskID", taskID).
					Int("resourceID", resourceID).
					Int64("downloaded", *downloaded).
					Msg("download progress (size unknown)")
			}
			lastLog = now
			lastLogDownloaded = *downloaded
		}
		if readErr != nil {
			if readErr == io.EOF {
				if expectedSize > 0 && *downloaded != expectedSize {
					if progressErr := onProgress(*downloaded, 0, true); progressErr != nil {
						return progressErr
					}
					return io.ErrUnexpectedEOF
				}
				return nil
			}
			if progressErr := onProgress(*downloaded, 0, true); progressErr != nil {
				return progressErr
			}
			return readErr
		}
	}
}

func (d *HermesEngine) downloadSegments(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	filePath string,
	task *TaskJob,
	resource *ResourceJob,
	prepared PreparedResource,
	segmentCount int,
) error {
	resourceID := resource.ID
	taskID := task.ID
	fileSize := prepared.Size
	ranges := splitFile(fileSize, segmentCount)
	segments, err := d.store.LoadSegmentInfo(resourceID)
	if err != nil {
		return fmt.Errorf("failed to load segment information: %w", err)
	}
	if !segmentsMatchRanges(segments, ranges) {
		ids, err := d.store.CreateSegments(resourceID, endpoint.URL, ranges)
		if err != nil {
			return fmt.Errorf("failed to create segment records: %w", err)
		}
		if len(ids) != len(ranges) {
			return errors.New("failed to create segment records: incorrect number of IDs returned")
		}
		segments = make([]Segment, len(ranges))
		for i, r := range ranges {
			segments[i] = Segment{ID: ids[i], Index: r.Index, URL: endpoint.URL, OffsetStart: r.OffsetStart, OffsetEnd: r.OffsetEnd, Size: r.Size}
		}
	}

	partPath := filePath + partialFileSuffix
	partInfo, statErr := os.Stat(partPath)
	partValid := statErr == nil && partInfo.Mode().IsRegular() && partInfo.Size() == fileSize
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect temporary file: %w", statErr)
	}
	// Prefer a completed .part file: an older same-sized destination may still
	// exist if the process stopped between progress persistence and rename.
	if segmentsComplete(segments) && partValid {
		if err := finalizePartialFile(partPath, filePath); err != nil {
			return fmt.Errorf("failed to commit completed temporary file: %w", err)
		}
		d.updateTracker(taskID, resourceID, fileSize, 0)
		return nil
	}
	// A destination is reusable only when both its size and every persisted
	// segment prove completion. Size alone is unsafe for stale or sparse files.
	if segmentsComplete(segments) && fileHasSize(filePath, fileSize) {
		d.updateTracker(taskID, resourceID, fileSize, 0)
		return nil
	}
	if !partValid {
		for i := range segments {
			if segments[i].Downloaded == 0 {
				continue
			}
			segments[i].Downloaded = 0
			if err := d.store.UpdateSegmentProgress(segments[i].ID, 0); err != nil {
				return fmt.Errorf("failed to reset segment progress: %w", err)
			}
		}
	}

	partFile, err := os.OpenFile(partPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open temporary file: %w", err)
	}
	partClosed := false
	defer func() {
		if !partClosed {
			_ = partFile.Close()
		}
	}()
	if !partValid {
		if err := partFile.Truncate(fileSize); err != nil {
			return fmt.Errorf("failed to preallocate temporary file: %w", err)
		}
	}

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	progressCh := make(chan segmentProgress, len(segments)*2)
	var wg sync.WaitGroup
	segSem := make(chan struct{}, d.cfg.SegmentConcurrency)
	for slot, segment := range segments {
		if segment.Downloaded >= segment.Size {
			progressCh <- segmentProgress{slot: slot, downloaded: segment.Size, done: true}
			continue
		}
		wg.Add(1)
		go func(slot int, segment Segment) {
			defer wg.Done()
			select {
			case segSem <- struct{}{}:
			case <-workerCtx.Done():
				return
			}
			defer func() { <-segSem }()
			d.downloadSegment(workerCtx, driver, endpoint, partFile, segment, slot, progressCh)
		}(slot, segment)
	}
	go func() {
		wg.Wait()
		close(progressCh)
	}()

	states := make([]segmentProgress, len(segments))
	for i, segment := range segments {
		states[i].slot = i
		states[i].downloaded = segment.Downloaded
	}
	lastLog := time.Now()
	lastLogDownloaded := int64(0)
	for i := range states {
		lastLogDownloaded += states[i].downloaded
	}
	lastPersist := time.Now()
	var firstErr error
	var progressEventCount int64
	lastProgressEventCount := time.Now()
	lastProgressEventCountN := int64(0)
	for progress := range progressCh {
		progressEventCount++
		if progress.slot < 0 || progress.slot >= len(states) {
			if firstErr == nil {
				firstErr = errors.New("received an invalid segment progress index")
				cancelWorkers()
			}
			continue
		}
		states[progress.slot] = progress
		if progress.err != nil && firstErr == nil {
			firstErr = progress.err
			cancelWorkers()
		}
		// Always update in-memory tracker for real-time WS progress.
		var totalStateDL, totalStateSpd int64
		for _, s := range states {
			totalStateDL += s.downloaded
			totalStateSpd += s.speed
		}
		d.updateTracker(taskID, resourceID, totalStateDL, totalStateSpd)
		resource.Downloaded = totalStateDL
		resource.Speed = totalStateSpd
		// Throttle DB persistence to progressInterval to avoid excessive writes.
		if time.Since(lastPersist) >= progressInterval || progress.done || progress.err != nil {
			// The partial file is pre-sized, so its length cannot validate which
			// ranges survived a crash. Make the data durable before allowing the
			// persisted per-segment offsets to advance.
			persistErr := partFile.Sync()
			if persistErr == nil {
				persistErr = d.persistAggregate(taskID, resourceID, segments, states)
			}
			if persistErr != nil && firstErr == nil {
				firstErr = persistErr
				cancelWorkers()
			}
			lastPersist = time.Now()
		}
		// Progress log every 3 seconds for diagnostics
		if time.Since(lastLog) >= progressLogInterval {
			var totalDl int64
			for _, s := range states {
				totalDl += s.downloaded
			}
			pct := float64(totalDl) * 100 / float64(fileSize)
			logSpeed := calcSpeed(lastLog, lastLogDownloaded, time.Now(), totalDl)
			// Count how many progress events were received per second in this window
			eventWindow := time.Since(lastProgressEventCount)
			eventsPerSec := float64(progressEventCount-lastProgressEventCountN) / eventWindow.Seconds()
			d.logger.Info().
				Int("taskID", taskID).
				Int("resourceID", resourceID).
				Int64("downloaded", totalDl).
				Int64("totalSize", fileSize).
				Float64("percent", pct).
				Str("speed", formatSpeed(logSpeed)).
				Int("segmentCount", len(segments)).
				Int64("totalEvents", progressEventCount).
				Float64("eventsPerSec", eventsPerSec).
				Msg("segment download progress")
			// Per-slot detail for diagnostics.
			for _, s := range states {
				segSize := segments[s.slot].Size
				var segPct float64
				if segSize > 0 {
					segPct = float64(s.downloaded) * 100 / float64(segSize)
				}
				d.logger.Info().
					Int("slot", s.slot).
					Int64("downloaded", s.downloaded).
					Int64("size", segSize).
					Float64("pct", segPct).
					Str("speed", formatSpeed(s.speed)).
					Bool("done", s.done).
					Msg("seg: detail")
			}
			lastLog = time.Now()
			lastLogDownloaded = totalDl
			lastProgressEventCount = time.Now()
			lastProgressEventCountN = progressEventCount
		}
	}
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	if firstErr != nil {
		return fmt.Errorf("segment download failed: %w", firstErr)
	}
	for _, state := range states {
		if !state.done {
			return errors.New("segment download did not complete")
		}
	}
	// Make data durable before marking all ranges complete. If persistence then
	// succeeds but rename is interrupted, the next run finalizes the .part file.
	if err := partFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}
	if err := d.persistAggregate(taskID, resourceID, segments, states); err != nil {
		return err
	}
	if err := partFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	partClosed = true
	if err := os.Rename(partPath, filePath); err != nil {
		return fmt.Errorf("failed to commit downloaded file: %w", err)
	}
	return nil
}

func segmentsMatchRanges(segments []Segment, ranges []SegmentRange) bool {
	if len(segments) != len(ranges) {
		return false
	}
	for i, segment := range segments {
		r := ranges[i]
		if segment.Index != r.Index || segment.OffsetStart != r.OffsetStart || segment.OffsetEnd != r.OffsetEnd || segment.Size != r.Size || segment.Downloaded < 0 || segment.Downloaded > segment.Size {
			return false
		}
	}
	return true
}

func (d *HermesEngine) downloadSegment(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	file *os.File,
	segment Segment,
	slot int,
	progressCh chan<- segmentProgress,
) {
	downloaded := segment.Downloaded
	speedSampler := newProgressSpeedSampler(time.Now(), downloaded)
	lastSegLog := time.Now()
	lastSegLogDownloaded := downloaded

	for attempt := 0; attempt < maxReadAttempts; attempt++ {
		if err := context.Cause(ctx); err != nil {
			progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: err}
			return
		}
		request := ReadRequest{OffsetStart: segment.OffsetStart + downloaded, OffsetEnd: segment.OffsetEnd, UseRange: true}
		openStart := time.Now()
		d.logger.Info().
			Int("slot", slot).
			Int("attempt", attempt+1).
			Int64("offset", segment.OffsetStart+downloaded).
			Int64("remaining", segment.Size-downloaded).
			Msg("seg: Open() starting")
		reader, err := driver.Open(ctx, endpoint, request)
		openElapsed := time.Since(openStart)
		if err != nil {
			d.logger.Info().
				Int("slot", slot).
				Int("attempt", attempt+1).
				Dur("elapsed", openElapsed).
				Err(err).
				Msg("seg: Open() failed")
			if attempt == maxReadAttempts-1 {
				progressCh <- segmentProgress{slot: slot, downloaded: downloaded, done: true, err: err}
				return
			}
			if !waitForRetry(ctx, attempt) {
				progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
				return
			}
			continue
		}
		d.logger.Info().
			Int("slot", slot).
			Int("attempt", attempt+1).
			Dur("openElapsed", openElapsed).
			Msg("seg: Open() done, reading")

		buf := make([]byte, readBufferSize)
		for downloaded < segment.Size {
			chunkStart := time.Now()
			remaining := segment.Size - downloaded
			readBuf := buf
			if remaining < int64(len(readBuf)) {
				readBuf = readBuf[:remaining]
			}
			n, readErr := d.readWithTimeout(reader, readBuf)
			if n > 0 {
				written, err := file.WriteAt(readBuf[:n], segment.OffsetStart+downloaded)
				if err == nil && written != n {
					err = io.ErrShortWrite
				}
				if err != nil {
					reader.Close()
					progressCh <- segmentProgress{slot: slot, downloaded: downloaded, done: true, err: err}
					return
				}
				downloaded += int64(n)
				if d.cfg.SpeedLimit > 0 {
					expected := time.Duration(float64(n) / float64(d.cfg.SpeedLimit) * float64(time.Second))
					elapsed := time.Since(chunkStart)
					if expected > elapsed {
						select {
						case <-ctx.Done():
							progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
							return
						case <-time.After(expected - elapsed):
						}
					}
				}
			}
			now := time.Now()
			// Sample across at least progressInterval. Per-read timings are far too
			// short and can turn a normal 32 KiB read into an apparent multi-GB/s
			// transfer rate.
			speed := speedSampler.Sample(now, downloaded)
			progressCh <- segmentProgress{slot: slot, downloaded: downloaded, speed: speed}
			// Periodic per-segment progress log for diagnostics (every ~1.5s or at 5% increments).
			segPct := float64(downloaded) * 100 / float64(segment.Size)
			if time.Since(lastSegLog) >= 1500*time.Millisecond ||
				int64(segPct) >= int64(float64(lastSegLogDownloaded)*100/float64(segment.Size))+5 {
				segLogSpeed := calcSpeed(lastSegLog, lastSegLogDownloaded, now, downloaded)
				d.logger.Info().
					Int("slot", slot).
					Int("index", segment.Index).
					Int64("dl", downloaded).
					Int64("size", segment.Size).
					Float64("pct", segPct).
					Str("speed", formatSpeed(segLogSpeed)).
					Msg("seg: progress")
				lastSegLog = now
				lastSegLogDownloaded = downloaded
			}
			if readErr != nil {
				reader.Close()
				if errors.Is(readErr, errReadTimeout) {
					d.logger.Info().
						Int("slot", slot).
						Int64("progress", downloaded).
						Int64("total", segment.Size).
						Msg("seg: Read() timeout, will retry")
				} else {
					d.logger.Info().
						Int("slot", slot).
						Err(readErr).
						Int64("progress", downloaded).
						Int64("total", segment.Size).
						Msg("seg: Read() error, will retry")
				}
				if errors.Is(readErr, context.Canceled) || ctx.Err() != nil {
					progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
					return
				}
				break
			}
		}
		reader.Close()
		if downloaded == segment.Size {
			d.logger.Info().
				Int("slot", slot).
				Int64("size", segment.Size).
				Msg("seg: finished")
			progressCh <- segmentProgress{slot: slot, downloaded: downloaded, done: true}
			return
		}
		if attempt < maxReadAttempts-1 && !waitForRetry(ctx, attempt) {
			progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
			return
		}
	}
	progressCh <- segmentProgress{slot: slot, downloaded: downloaded, done: true, err: io.ErrUnexpectedEOF}
}

func fileHasSize(path string, size int64) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() == size
}

func segmentsComplete(segments []Segment) bool {
	if len(segments) == 0 {
		return false
	}
	for _, segment := range segments {
		if segment.Size < 0 || segment.Downloaded != segment.Size {
			return false
		}
	}
	return true
}

func finalizePartialFile(partPath, filePath string) error {
	file, err := os.OpenFile(partPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(partPath, filePath)
}

var errReadTimeout = errors.New("read timeout: CDN connection stalled")

// readWithTimeout performs a single Read with a deadline. If the Read does not
// return within the timeout, the reader is closed to unblock the goroutine and
// errReadTimeout is returned so the caller can retry.
// The per-read goroutine overhead is amortized by the 256 KiB read buffer,
// which reduces the number of Read calls by 8× compared to the old 32 KiB buffer.
func (d *HermesEngine) readWithTimeout(reader io.Reader, buf []byte) (int, error) {
	type readResult struct {
		n   int
		err error
	}
	done := make(chan readResult, 1)
	go func() {
		n, err := reader.Read(buf)
		done <- readResult{n, err}
	}()

	timer := time.NewTimer(d.cfg.ReadTimeout)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.n, r.err
	case <-timer.C:
		return 0, errReadTimeout
	}
}

func waitForRetry(ctx context.Context, attempt int) bool {
	if attempt >= maxReadAttempts-1 {
		return true
	}
	delay := time.Duration(1<<attempt) * 100 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (d *HermesEngine) persistProgress(taskID, resourceID, segmentID int, downloaded, speed int64) error {
	if store, ok := d.store.(ProgressBatchStore); ok {
		if err := store.UpdateResourceSegmentProgress(resourceID, segmentID, downloaded, speed); err != nil {
			return fmt.Errorf("failed to update progress: %w", err)
		}
		d.updateTracker(taskID, resourceID, downloaded, speed)
		return nil
	}
	if err := d.store.UpdateSegmentProgress(segmentID, downloaded); err != nil {
		return fmt.Errorf("failed to update segment progress: %w", err)
	}
	if err := d.updateResourceProgress(taskID, resourceID, downloaded, speed); err != nil {
		return fmt.Errorf("failed to update task progress: %w", err)
	}
	d.updateTracker(taskID, resourceID, downloaded, speed)
	return nil
}

func (d *HermesEngine) persistAggregate(taskID, resourceID int, segments []Segment, states []segmentProgress) error {
	var totalDownloaded int64
	var totalSpeed int64
	updates := make([]SegmentProgressUpdate, 0, len(states))
	for i, state := range states {
		totalDownloaded += state.downloaded
		totalSpeed += state.speed
		updates = append(updates, SegmentProgressUpdate{
			SegmentID:  segments[i].ID,
			Downloaded: state.downloaded,
		})
	}
	if store, ok := d.store.(ProgressBatchStore); ok {
		if err := store.UpdateAggregateResourceProgress(resourceID, updates, totalDownloaded, totalSpeed); err != nil {
			return fmt.Errorf("failed to update aggregate progress: %w", err)
		}
		d.updateTracker(taskID, resourceID, totalDownloaded, totalSpeed)
		return nil
	}
	for i, state := range states {
		if err := d.store.UpdateSegmentProgress(segments[i].ID, state.downloaded); err != nil {
			return fmt.Errorf("failed to update segment progress: %w", err)
		}
	}
	if err := d.updateResourceProgress(taskID, resourceID, totalDownloaded, totalSpeed); err != nil {
		return fmt.Errorf("failed to update task progress: %w", err)
	}
	d.updateTracker(taskID, resourceID, totalDownloaded, totalSpeed)
	return nil
}
