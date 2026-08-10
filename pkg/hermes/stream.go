package hermes

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// downloadStream records a live resource through the driver's optional
// StreamRecorder capability. It deliberately bypasses downloadFile: a live
// source cannot be resumed with byte ranges and must preserve closed media
// chunks across pause/reconnect instead of truncating a single .part file.
func (d *HermesEngine) download_stream(
	ctx context.Context,
	recorder StreamRecorder,
	endpoint Endpoint,
	file_path string,
	task *TaskJob,
	resource *ResourceJob,
) error {
	if recorder == nil {
		return errors.New("stream recorder is nil")
	}
	if err := wait_for_record_start(ctx, resource.RecordStart); err != nil {
		return err
	}

	request := StreamRecordRequest{
		OutputPath:    file_path,
		StopAt:        stream_timestamp(resource.RecordEnd),
		RotateMinutes: resource.RotateMinutes,
		RotateSize:    resource.RotateSize,
	}
	if resource.Duration > 0 {
		request.Duration = time.Duration(resource.Duration) * time.Second
	}

	started_at := time.Now()
	result, err := recorder.RecordStream(ctx, endpoint, request, func(progress StreamRecordProgress) error {
		if progress.Finalizing {
			if err := d.store.UpdateStatus(task.ID, TaskStatusMerging); err != nil {
				return fmt.Errorf("failed to update live task status to merging: %w", err)
			}
		}
		resource.Downloaded = progress.Downloaded
		resource.Speed = progress.Speed
		if store, ok := d.store.(StreamSegmentStore); ok && len(progress.Segments) > 0 {
			if err := store.SyncStreamSegments(resource.ID, endpoint.URL, progress.Segments); err != nil {
				return fmt.Errorf("failed to persist stream segments: %w", err)
			}
		}
		if err := d.update_resource_progress(task.ID, resource.ID, progress.Downloaded, progress.Speed); err != nil {
			return fmt.Errorf("failed to persist stream progress: %w", err)
		}
		d.update_tracker(task.ID, resource.ID, progress.Downloaded, progress.Speed)
		return nil
	})
	if err != nil {
		return err
	}

	if result.FilePath != "" && result.FilePath != file_path {
		return fmt.Errorf("stream recorder returned an unexpected output path: %s", result.FilePath)
	}
	resource.FilePath = file_path
	resource.Size = result.Size
	resource.Downloaded = result.Size
	resource.Speed = 0
	if result.Duration <= 0 {
		result.Duration = time.Since(started_at)
	}
	resource.Duration = int64(result.Duration.Round(time.Second) / time.Second)
	if resource.Extra == nil {
		resource.Extra = make(map[string]string)
	}
	resource.Extra["record_duration"] = fmt.Sprintf("%.3f", result.Duration.Seconds())
	if store, ok := d.store.(StreamResultStore); ok {
		if err := store.UpdateStreamDuration(resource.ID, resource.Duration); err != nil {
			return fmt.Errorf("failed to persist stream duration: %w", err)
		}
	}

	if err := d.update_resource_size(task.ID, resource.ID, result.Size); err != nil {
		return fmt.Errorf("failed to update final stream size: %w", err)
	}
	if err := d.update_resource_progress(task.ID, resource.ID, result.Size, 0); err != nil {
		return fmt.Errorf("failed to update final stream progress: %w", err)
	}
	d.update_tracker_size(task.ID, resource.ID, result.Size)
	d.update_tracker(task.ID, resource.ID, result.Size, 0)
	return nil
}

func wait_for_record_start(ctx context.Context, value *int64) error {
	start := stream_timestamp(value)
	if start.IsZero() || !start.After(time.Now()) {
		return nil
	}
	timer := time.NewTimer(time.Until(start))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return nil
	}
}

// streamTimestamp accepts both Unix seconds and Unix milliseconds because the
// persisted schema historically exposed raw int64 timestamps to integrations.
func stream_timestamp(value *int64) time.Time {
	if value == nil || *value <= 0 {
		return time.Time{}
	}
	if *value < 1_000_000_000_000 {
		return time.Unix(*value, 0)
	}
	return time.UnixMilli(*value)
}
