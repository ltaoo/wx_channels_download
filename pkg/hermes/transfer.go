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

func (d *HermesEngine) download_file(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	file_path string,
	task *TaskJob,
	resource *ResourceJob,
	prepared PreparedResource,
) error {
	resource_id := resource.ID
	task_id := task.ID
	segments, err := d.store.LoadSegmentInfo(resource_id)
	if err != nil {
		return fmt.Errorf("failed to load segment information: %w", err)
	}
	ranges := []SegmentRange{{Index: 0, OffsetStart: 0, OffsetEnd: max_int64(0, prepared.Size-1), Size: prepared.Size}}
	if !segments_match_ranges(segments, ranges) {
		ids, err := d.store.CreateSegments(resource_id, endpoint.URL, ranges)
		if err != nil {
			return fmt.Errorf("failed to create segment records: %w", err)
		}
		if len(ids) != 1 {
			return errors.New("failed to create segment records: incorrect number of IDs returned")
		}
		segments = []Segment{{ID: ids[0], Index: 0, URL: endpoint.URL, Size: prepared.Size, OffsetEnd: ranges[0].OffsetEnd}}
	}
	segment := &segments[0]
	part_path := file_path + partial_file_suffix

	// A crash may happen after persisting completion but before the atomic rename.
	// Prefer the durable partial file over an older same-sized destination.
	if prepared.Size > 0 && segment.Downloaded == prepared.Size && file_has_size(part_path, prepared.Size) {
		if err := finalize_partial_file(part_path, file_path); err != nil {
			return fmt.Errorf("failed to finalize temporary file: %w", err)
		}
		d.update_tracker(task_id, resource_id, prepared.Size, 0)
		return nil
	}
	// A same-sized destination alone is not proof that this resource completed.
	// Only persisted completion state may make an existing destination reusable.
	if prepared.Size > 0 && segment.Downloaded == prepared.Size && file_has_size(file_path, prepared.Size) {
		d.update_tracker(task_id, resource_id, prepared.Size, 0)
		return nil
	}

	var downloaded int64
	if fi, stat_err := os.Stat(part_path); stat_err == nil {
		downloaded = fi.Size()
	}
	if prepared.Size > 0 && (downloaded < 0 || downloaded > prepared.Size) {
		downloaded = 0
		if err := os.Truncate(part_path, 0); err != nil {
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

	for attempt := 0; attempt < max_read_attempts; attempt++ {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		// Start with a Range request whenever the source supports it so interrupted
		// downloads can resume from their durable offset.
		use_range := prepared.SupportsRange && prepared.Size > 0
		request := ReadRequest{OffsetStart: downloaded, OffsetEnd: prepared.Size - 1, UseRange: use_range}
		reader, err := d.open_with_limit(ctx, driver, endpoint, request)
		if err != nil {
			if !wait_for_retry(ctx, attempt) {
				return context.Cause(ctx)
			}
			if attempt == max_read_attempts-1 {
				return fmt.Errorf("failed to open download source: %w", err)
			}
			continue
		}

		flags := os.O_CREATE | os.O_WRONLY
		if use_range {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
			downloaded = 0
		}
		file, open_err := os.OpenFile(part_path, flags, 0644)
		if open_err != nil {
			reader.Close()
			return fmt.Errorf("failed to open temporary file: %w", open_err)
		}

		last_persist := time.Now()
		err = d.copy_reader(ctx, reader, file, prepared.Size, &downloaded, task_id, resource_id, func(total, speed int64, force bool) error {
			resource.Downloaded = total
			resource.Speed = speed
			d.update_tracker(task_id, resource_id, total, speed)
			if !force && time.Since(last_persist) < progress_interval {
				return nil
			}
			last_persist = time.Now()
			return d.persist_progress(task_id, resource_id, segment.ID, total, speed)
		})
		reader_close_err := reader.Close()
		if err == nil && reader_close_err != nil {
			err = reader_close_err
		}
		if err == nil && (prepared.Size <= 0 || downloaded == prepared.Size) {
			if sync_err := file.Sync(); sync_err != nil {
				err = sync_err
			} else if persist_err := d.persist_progress(task_id, resource_id, segment.ID, downloaded, 0); persist_err != nil {
				err = persist_err
			} else {
				resource.Downloaded = downloaded
				resource.Speed = 0
			}
		}
		close_err := file.Close()
		if err == nil {
			err = close_err
		}
		if err == nil && (prepared.Size <= 0 || downloaded == prepared.Size) {
			if rename_err := os.Rename(part_path, file_path); rename_err != nil {
				return fmt.Errorf("failed to commit downloaded file: %w", rename_err)
			}
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		if prepared.Size > 0 && downloaded >= prepared.Size {
			return fmt.Errorf("downloaded data size mismatch: expected %d bytes, got %d bytes", prepared.Size, downloaded)
		}
		if attempt == max_read_attempts-1 {
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			return fmt.Errorf("download read failed: %w", err)
		}
		if !prepared.SupportsRange {
			downloaded = 0
		}
		if !wait_for_retry(ctx, attempt) {
			return context.Cause(ctx)
		}
	}
	return io.ErrUnexpectedEOF
}

func (d *HermesEngine) copy_reader(
	ctx context.Context,
	reader io.Reader,
	writer io.Writer,
	expected_size int64,
	downloaded *int64,
	task_id int,
	resource_id int,
	on_progress func(total, speed int64, force bool) error,
) error {
	buf := make([]byte, read_buffer_size)
	speed_sampler := new_progress_speed_sampler(time.Now(), *downloaded)
	last_log := time.Now()
	last_log_downloaded := *downloaded
	for {
		chunk_start := time.Now()
		if err := context.Cause(ctx); err != nil {
			if progress_err := on_progress(*downloaded, 0, true); progress_err != nil {
				return progress_err
			}
			return err
		}
		read_buf := buf
		if expected_size > 0 {
			remaining := expected_size - *downloaded
			if remaining == 0 {
				return nil
			}
			if remaining < int64(len(read_buf)) {
				read_buf = read_buf[:remaining]
			}
		}
		n, read_err := reader.Read(read_buf)
		if n > 0 {
			if _, err := writer.Write(read_buf[:n]); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			*downloaded += int64(n)
			if d.cfg.SpeedLimit > 0 {
				expected := time.Duration(float64(n) / float64(d.cfg.SpeedLimit) * float64(time.Second))
				elapsed := time.Since(chunk_start)
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
		speed := speed_sampler.sample(now, *downloaded)
		if err := on_progress(*downloaded, speed, false); err != nil {
			return err
		}
		// Progress log every 3 seconds for diagnostics
		if now.Sub(last_log) >= progress_log_interval {
			if expected_size > 0 {
				pct := float64(*downloaded) * 100 / float64(expected_size)
				log_speed := calc_speed(last_log, last_log_downloaded, now, *downloaded)
				d.logger.Info().
					Int("task_id", task_id).
					Int("resource_id", resource_id).
					Int64("downloaded", *downloaded).
					Int64("total_size", expected_size).
					Float64("percent", pct).
					Str("speed", format_speed(log_speed)).
					Msg("download progress")
			} else {
				d.logger.Info().
					Int("task_id", task_id).
					Int("resource_id", resource_id).
					Int64("downloaded", *downloaded).
					Msg("download progress (size unknown)")
			}
			last_log = now
			last_log_downloaded = *downloaded
		}
		if read_err != nil {
			if read_err == io.EOF {
				if expected_size > 0 && *downloaded != expected_size {
					if progress_err := on_progress(*downloaded, 0, true); progress_err != nil {
						return progress_err
					}
					return io.ErrUnexpectedEOF
				}
				return nil
			}
			if progress_err := on_progress(*downloaded, 0, true); progress_err != nil {
				return progress_err
			}
			return read_err
		}
	}
}

func (d *HermesEngine) download_segments(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	file_path string,
	task *TaskJob,
	resource *ResourceJob,
	prepared PreparedResource,
	segment_count int,
) error {
	resource_id := resource.ID
	task_id := task.ID
	file_size := prepared.Size
	ranges := split_file(file_size, segment_count)
	segments, err := d.store.LoadSegmentInfo(resource_id)
	if err != nil {
		return fmt.Errorf("failed to load segment information: %w", err)
	}
	if !segments_match_ranges(segments, ranges) {
		ids, err := d.store.CreateSegments(resource_id, endpoint.URL, ranges)
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

	part_path := file_path + partial_file_suffix
	part_info, stat_err := os.Stat(part_path)
	part_valid := stat_err == nil && part_info.Mode().IsRegular() && part_info.Size() == file_size
	if stat_err != nil && !errors.Is(stat_err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect temporary file: %w", stat_err)
	}
	// Prefer a completed .part file: an older same-sized destination may still
	// exist if the process stopped between progress persistence and rename.
	if segments_complete(segments) && part_valid {
		if err := finalize_partial_file(part_path, file_path); err != nil {
			return fmt.Errorf("failed to commit completed temporary file: %w", err)
		}
		d.update_tracker(task_id, resource_id, file_size, 0)
		return nil
	}
	// A destination is reusable only when both its size and every persisted
	// segment prove completion. Size alone is unsafe for stale or sparse files.
	if segments_complete(segments) && file_has_size(file_path, file_size) {
		d.update_tracker(task_id, resource_id, file_size, 0)
		return nil
	}
	if !part_valid {
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

	part_file, err := os.OpenFile(part_path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open temporary file: %w", err)
	}
	part_closed := false
	defer func() {
		if !part_closed {
			_ = part_file.Close()
		}
	}()
	if !part_valid {
		if err := part_file.Truncate(file_size); err != nil {
			return fmt.Errorf("failed to preallocate temporary file: %w", err)
		}
	}

	worker_ctx, cancel_workers := context.WithCancel(ctx)
	defer cancel_workers()
	progress_ch := make(chan segment_progress, len(segments)*2)
	var wg sync.WaitGroup
	seg_sem := make(chan struct{}, d.cfg.SegmentConcurrency)
	for slot, segment := range segments {
		if segment.Downloaded >= segment.Size {
			progress_ch <- segment_progress{slot: slot, downloaded: segment.Size, done: true}
			continue
		}
		wg.Add(1)
		go func(slot int, segment Segment) {
			defer wg.Done()
			select {
			case seg_sem <- struct{}{}:
			case <-worker_ctx.Done():
				return
			}
			defer func() { <-seg_sem }()
			d.download_segment(worker_ctx, driver, endpoint, part_file, segment, slot, progress_ch)
		}(slot, segment)
	}
	go func() {
		wg.Wait()
		close(progress_ch)
	}()

	states := make([]segment_progress, len(segments))
	for i, segment := range segments {
		states[i].slot = i
		states[i].downloaded = segment.Downloaded
	}
	last_log := time.Now()
	last_log_downloaded := int64(0)
	for i := range states {
		last_log_downloaded += states[i].downloaded
	}
	last_persist := time.Now()
	var first_err error
	var progress_event_count int64
	last_progress_event_count := time.Now()
	last_progress_event_count_n := int64(0)
	for progress := range progress_ch {
		progress_event_count++
		if progress.slot < 0 || progress.slot >= len(states) {
			if first_err == nil {
				first_err = errors.New("received an invalid segment progress index")
				cancel_workers()
			}
			continue
		}
		states[progress.slot] = progress
		if progress.err != nil && first_err == nil {
			first_err = progress.err
			cancel_workers()
		}
		// Always update in-memory tracker for real-time WS progress.
		var total_state_dl, total_state_spd int64
		for _, s := range states {
			total_state_dl += s.downloaded
			total_state_spd += s.speed
		}
		d.update_tracker(task_id, resource_id, total_state_dl, total_state_spd)
		resource.Downloaded = total_state_dl
		resource.Speed = total_state_spd
		// Throttle DB persistence to progressInterval to avoid excessive writes.
		if time.Since(last_persist) >= progress_interval || progress.done || progress.err != nil {
			// The partial file is pre-sized, so its length cannot validate which
			// ranges survived a crash. Make the data durable before allowing the
			// persisted per-segment offsets to advance.
			persist_err := part_file.Sync()
			if persist_err == nil {
				persist_err = d.persist_aggregate(task_id, resource_id, segments, states)
			}
			if persist_err != nil && first_err == nil {
				first_err = persist_err
				cancel_workers()
			}
			last_persist = time.Now()
		}
		// Progress log every 3 seconds for diagnostics
		if time.Since(last_log) >= progress_log_interval {
			var total_dl int64
			for _, s := range states {
				total_dl += s.downloaded
			}
			pct := float64(total_dl) * 100 / float64(file_size)
			log_speed := calc_speed(last_log, last_log_downloaded, time.Now(), total_dl)
			// Count how many progress events were received per second in this window
			event_window := time.Since(last_progress_event_count)
			events_per_sec := float64(progress_event_count-last_progress_event_count_n) / event_window.Seconds()
			d.logger.Info().
				Int("task_id", task_id).
				Int("resource_id", resource_id).
				Int64("downloaded", total_dl).
				Int64("total_size", file_size).
				Float64("percent", pct).
				Str("speed", format_speed(log_speed)).
				Int("segment_count", len(segments)).
				Int64("total_events", progress_event_count).
				Float64("events_per_sec", events_per_sec).
				Msg("segment download progress")
			// Per-slot detail for diagnostics.
			for _, s := range states {
				seg_size := segments[s.slot].Size
				var seg_pct float64
				if seg_size > 0 {
					seg_pct = float64(s.downloaded) * 100 / float64(seg_size)
				}
				d.logger.Info().
					Int("slot", s.slot).
					Int64("downloaded", s.downloaded).
					Int64("size", seg_size).
					Float64("pct", seg_pct).
					Str("speed", format_speed(s.speed)).
					Bool("done", s.done).
					Msg("seg: detail")
			}
			last_log = time.Now()
			last_log_downloaded = total_dl
			last_progress_event_count = time.Now()
			last_progress_event_count_n = progress_event_count
		}
	}
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	if first_err != nil {
		return fmt.Errorf("segment download failed: %w", first_err)
	}
	for _, state := range states {
		if !state.done {
			return errors.New("segment download did not complete")
		}
	}
	// Make data durable before marking all ranges complete. If persistence then
	// succeeds but rename is interrupted, the next run finalizes the .part file.
	if err := part_file.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}
	if err := d.persist_aggregate(task_id, resource_id, segments, states); err != nil {
		return err
	}
	if err := part_file.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	part_closed = true
	if err := os.Rename(part_path, file_path); err != nil {
		return fmt.Errorf("failed to commit downloaded file: %w", err)
	}
	return nil
}

func segments_match_ranges(segments []Segment, ranges []SegmentRange) bool {
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

func (d *HermesEngine) download_segment(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	file *os.File,
	segment Segment,
	slot int,
	progress_ch chan<- segment_progress,
) {
	downloaded := segment.Downloaded
	speed_sampler := new_progress_speed_sampler(time.Now(), downloaded)
	last_seg_log := time.Now()
	last_seg_log_downloaded := downloaded
	connection_attempt := 0
	stalled_attempts := 0
	var last_err error

	// A CDN may intentionally return less than the requested range, or close a
	// long response early. Such a connection is still useful when it delivered
	// bytes: resume at the new offset and only exhaust the retry budget after
	// consecutive attempts that make no progress.
	for stalled_attempts < max_read_attempts {
		if err := context.Cause(ctx); err != nil {
			progress_ch <- segment_progress{slot: slot, downloaded: downloaded, err: err}
			return
		}
		connection_attempt++
		attempt_start := downloaded
		request := ReadRequest{OffsetStart: segment.OffsetStart + downloaded, OffsetEnd: segment.OffsetEnd, UseRange: true}
		open_start := time.Now()
		d.logger.Info().
			Int("slot", slot).
			Int("attempt", connection_attempt).
			Int("consecutive_stalls", stalled_attempts).
			Int64("offset", segment.OffsetStart+downloaded).
			Int64("remaining", segment.Size-downloaded).
			Msg("seg: Open() starting")
		reader, err := d.open_with_limit(ctx, driver, endpoint, request)
		open_elapsed := time.Since(open_start)
		if err != nil {
			last_err = err
			stalled_attempts++
			d.logger.Info().
				Int("slot", slot).
				Int("attempt", connection_attempt).
				Int("consecutive_stalls", stalled_attempts).
				Dur("elapsed", open_elapsed).
				Err(err).
				Msg("seg: Open() failed")
			if stalled_attempts >= max_read_attempts {
				progress_ch <- segment_progress{slot: slot, downloaded: downloaded, done: true, err: err}
				return
			}
			if !wait_for_retry(ctx, stalled_attempts-1) {
				progress_ch <- segment_progress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
				return
			}
			continue
		}
		d.logger.Info().
			Int("slot", slot).
			Int("attempt", connection_attempt).
			Dur("open_elapsed", open_elapsed).
			Msg("seg: Open() done, reading")

		buf := make([]byte, read_buffer_size)
		for downloaded < segment.Size {
			chunk_start := time.Now()
			remaining := segment.Size - downloaded
			read_buf := buf
			if remaining < int64(len(read_buf)) {
				read_buf = read_buf[:remaining]
			}
			n, read_err := d.read_with_timeout(reader, read_buf)
			if n > 0 {
				written, err := file.WriteAt(read_buf[:n], segment.OffsetStart+downloaded)
				if err == nil && written != n {
					err = io.ErrShortWrite
				}
				if err != nil {
					reader.Close()
					progress_ch <- segment_progress{slot: slot, downloaded: downloaded, done: true, err: err}
					return
				}
				downloaded += int64(n)
				if d.cfg.SpeedLimit > 0 {
					expected := time.Duration(float64(n) / float64(d.cfg.SpeedLimit) * float64(time.Second))
					elapsed := time.Since(chunk_start)
					if expected > elapsed {
						select {
						case <-ctx.Done():
							reader.Close()
							progress_ch <- segment_progress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
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
			speed := speed_sampler.sample(now, downloaded)
			progress_ch <- segment_progress{slot: slot, downloaded: downloaded, speed: speed}
			// Periodic per-segment progress log for diagnostics (every ~1.5s or at 5% increments).
			seg_pct := float64(downloaded) * 100 / float64(segment.Size)
			if time.Since(last_seg_log) >= 1500*time.Millisecond ||
				int64(seg_pct) >= int64(float64(last_seg_log_downloaded)*100/float64(segment.Size))+5 {
				seg_log_speed := calc_speed(last_seg_log, last_seg_log_downloaded, now, downloaded)
				d.logger.Info().
					Int("slot", slot).
					Int("index", segment.Index).
					Int64("dl", downloaded).
					Int64("size", segment.Size).
					Float64("pct", seg_pct).
					Str("speed", format_speed(seg_log_speed)).
					Msg("seg: progress")
				last_seg_log = now
				last_seg_log_downloaded = downloaded
			}
			if read_err != nil {
				reader.Close()
				if errors.Is(read_err, io.EOF) {
					last_err = io.ErrUnexpectedEOF
				} else {
					last_err = read_err
				}
				if errors.Is(read_err, err_read_timeout) {
					d.logger.Info().
						Int("slot", slot).
						Int64("progress", downloaded).
						Int64("total", segment.Size).
						Msg("seg: Read() timeout, will retry")
				} else {
					d.logger.Info().
						Int("slot", slot).
						Err(read_err).
						Int64("progress", downloaded).
						Int64("total", segment.Size).
						Msg("seg: Read() error, will retry")
				}
				if errors.Is(read_err, context.Canceled) || ctx.Err() != nil {
					progress_ch <- segment_progress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
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
			progress_ch <- segment_progress{slot: slot, downloaded: downloaded, done: true}
			return
		}
		if downloaded > attempt_start {
			stalled_attempts = 0
		} else {
			stalled_attempts++
		}
		if stalled_attempts >= max_read_attempts {
			break
		}
		retry_attempt := stalled_attempts - 1
		if retry_attempt < 0 {
			retry_attempt = 0
		}
		if !wait_for_retry(ctx, retry_attempt) {
			progress_ch <- segment_progress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
			return
		}
	}
	if last_err == nil {
		last_err = io.ErrUnexpectedEOF
	}
	progress_ch <- segment_progress{slot: slot, downloaded: downloaded, done: true, err: last_err}
}

func file_has_size(path string, size int64) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() == size
}

func segments_complete(segments []Segment) bool {
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

func finalize_partial_file(part_path, file_path string) error {
	file, err := os.OpenFile(part_path, os.O_RDWR, 0644)
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
	return os.Rename(part_path, file_path)
}

var err_read_timeout = errors.New("read timeout: CDN connection stalled")

// readWithTimeout performs a single Read with a deadline. If the Read does not
// return within the timeout, the reader is closed to unblock the goroutine and
// errReadTimeout is returned so the caller can retry.
// The per-read goroutine overhead is amortized by the 256 KiB read buffer,
// which reduces the number of Read calls by 8× compared to the old 32 KiB buffer.
func (d *HermesEngine) read_with_timeout(reader io.Reader, buf []byte) (int, error) {
	type read_result struct {
		n   int
		err error
	}
	done := make(chan read_result, 1)
	go func() {
		n, err := reader.Read(buf)
		done <- read_result{n, err}
	}()

	timer := time.NewTimer(d.cfg.ReadTimeout)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.n, r.err
	case <-timer.C:
		return 0, err_read_timeout
	}
}

func wait_for_retry(ctx context.Context, attempt int) bool {
	if attempt >= max_read_attempts-1 {
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

func (d *HermesEngine) persist_progress(task_id, resource_id, segment_id int, downloaded, speed int64) error {
	if store, ok := d.store.(ProgressBatchStore); ok {
		if err := store.UpdateResourceSegmentProgress(resource_id, segment_id, downloaded, speed); err != nil {
			return fmt.Errorf("failed to update progress: %w", err)
		}
		d.update_tracker(task_id, resource_id, downloaded, speed)
		return nil
	}
	if err := d.store.UpdateSegmentProgress(segment_id, downloaded); err != nil {
		return fmt.Errorf("failed to update segment progress: %w", err)
	}
	if err := d.update_resource_progress(task_id, resource_id, downloaded, speed); err != nil {
		return fmt.Errorf("failed to update task progress: %w", err)
	}
	d.update_tracker(task_id, resource_id, downloaded, speed)
	return nil
}

func (d *HermesEngine) persist_aggregate(task_id, resource_id int, segments []Segment, states []segment_progress) error {
	var total_downloaded int64
	var total_speed int64
	updates := make([]SegmentProgressUpdate, 0, len(states))
	for i, state := range states {
		total_downloaded += state.downloaded
		total_speed += state.speed
		updates = append(updates, SegmentProgressUpdate{
			SegmentID:  segments[i].ID,
			Downloaded: state.downloaded,
		})
	}
	if store, ok := d.store.(ProgressBatchStore); ok {
		if err := store.UpdateAggregateResourceProgress(resource_id, updates, total_downloaded, total_speed); err != nil {
			return fmt.Errorf("failed to update aggregate progress: %w", err)
		}
		d.update_tracker(task_id, resource_id, total_downloaded, total_speed)
		return nil
	}
	for i, state := range states {
		if err := d.store.UpdateSegmentProgress(segments[i].ID, state.downloaded); err != nil {
			return fmt.Errorf("failed to update segment progress: %w", err)
		}
	}
	if err := d.update_resource_progress(task_id, resource_id, total_downloaded, total_speed); err != nil {
		return fmt.Errorf("failed to update task progress: %w", err)
	}
	d.update_tracker(task_id, resource_id, total_downloaded, total_speed)
	return nil
}
