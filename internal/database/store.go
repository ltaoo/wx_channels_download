package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

// DBTaskStore is the GORM adapter for hermes.Store.
type DBTaskStore struct {
	db     *gorm.DB
	logger *zerolog.Logger
}

const persistence_batch_size = 50

func NewDBTaskStore(db *gorm.DB, logger *zerolog.Logger) *DBTaskStore {
	return &DBTaskStore{db: db, logger: logger}
}

// DB returns the underlying GORM database instance.
func (s *DBTaskStore) DB() *gorm.DB {
	return s.db
}

// Shutdown gracefully marks all in-progress download tasks as paused and
// deactivates active connections. Call this before shutting down the downloader.
func (s *DBTaskStore) Shutdown() {
	now := time.Now().UnixMilli()
	s.db.Model(&model.DownloadTask{}).
		Where("status = ? AND deleted_at IS NULL", model.TaskStatusDownloading).
		Updates(map[string]any{"status": model.TaskStatusPaused, "updated_at": now})
	s.db.Model(&model.DownloadConnection{}).
		Where("status = 1 AND deleted_at IS NULL").
		Updates(map[string]any{"status": 0, "speed": 0, "last_active": now, "updated_at": now})
}

func (s *DBTaskStore) debug(format string, args ...interface{}) {
	if s.logger != nil {
		s.logger.Info().Msgf("[dbTaskStore] "+format, args...)
	}
}

var _ hermes.Store = (*DBTaskStore)(nil)
var _ hermes.OutputNameStore = (*DBTaskStore)(nil)
var _ hermes.ResourceOutputStore = (*DBTaskStore)(nil)
var _ hermes.ResourceCleanupStore = (*DBTaskStore)(nil)
var _ hermes.StreamSegmentStore = (*DBTaskStore)(nil)
var _ hermes.StreamResultStore = (*DBTaskStore)(nil)
var _ hermes.ProgressBatchStore = (*DBTaskStore)(nil)

func (s *DBTaskStore) LoadTask(task_id int) (*hermes.TaskJob, error) {
	var task model.DownloadTask
	if err := s.db.Where("id = ?", task_id).First(&task).Error; err != nil {
		return nil, err
	}
	var resources []model.DownloadResource
	if err := s.db.Where("task_id = ?", task.Id).Order("merge_order ASC, id ASC").Find(&resources).Error; err != nil {
		return nil, err
	}
	if len(resources) == 0 {
		return nil, errors.New("任务没有下载资源")
	}
	resource_ids := make([]int, len(resources))
	for i, resource := range resources {
		resource_ids[i] = resource.Id
	}
	var endpoints []model.DownloadEndpoint
	if err := s.db.Where("resource_id IN ? AND enabled = ?", resource_ids, 1).
		Order("resource_id ASC, priority ASC, id ASC").Find(&endpoints).Error; err != nil {
		return nil, err
	}
	if len(endpoints) == 0 {
		return nil, errors.New("任务没有已启用的下载端点")
	}
	config := make(map[string]any)
	if strings.TrimSpace(task.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(task.ConfigJSON), &config); err != nil {
			return nil, fmt.Errorf("解析任务 %d config 失败: %w", task.Id, err)
		}
		if config == nil {
			config = make(map[string]any)
		}
	}
	download_dir, _ := config["download_dir"].(string)
	metadata := make(map[string]any)
	if strings.TrimSpace(task.MetadataJSON) != "" {
		if err := json.Unmarshal([]byte(task.MetadataJSON), &metadata); err != nil {
			return nil, fmt.Errorf("解析任务 %d metadata 失败: %w", task.Id, err)
		}
		if metadata == nil {
			metadata = make(map[string]any)
		}
	}
	endpoints_by_resource := make(map[int][]hermes.Endpoint, len(resources))
	for _, endpoint := range endpoints {
		headers := make(map[string]string)
		if strings.TrimSpace(endpoint.Headers) != "" {
			if err := json.Unmarshal([]byte(endpoint.Headers), &headers); err != nil {
				return nil, fmt.Errorf("解析端点 %d headers 失败: %w", endpoint.Id, err)
			}
		}
		endpoints_by_resource[endpoint.ResourceId] = append(endpoints_by_resource[endpoint.ResourceId], hermes.Endpoint{
			ID:       endpoint.Id,
			Protocol: endpoint.Protocol,
			URL:      endpoint.URL,
			Priority: endpoint.Priority,
			Headers:  headers,
			Cookies:  endpoint.Cookies,
		})
	}
	resource_infos := make([]hermes.ResourceJob, 0, len(resources))
	for _, resource := range resources {
		resource_endpoints := endpoints_by_resource[resource.Id]
		if len(resource_endpoints) == 0 {
			return nil, fmt.Errorf("资源 %d 没有已启用的下载端点", resource.Id)
		}
		extra := parseExtra(resource.Extra)
		resource_download_dir := strings.TrimSpace(resource.DownloadDir)
		if resource_download_dir == "" {
			resource_download_dir = strings.TrimSpace(download_dir)
		}
		resource_infos = append(resource_infos, hermes.ResourceJob{
			ID:            resource.Id,
			DownloadDir:   resource_download_dir,
			Name:          resource.Name,
			Kind:          resource.Kind,
			Type:          resource.Type,
			UniqueID:      resource.UniqueID,
			Endpoints:     resource_endpoints,
			Extra:         extra,
			StreamURL:     resource.StreamURL,
			RecordStart:   resource.RecordStart,
			RecordEnd:     resource.RecordEnd,
			Duration:      resource.Duration,
			RotateMinutes: resource.RotateMinutes,
			RotateSize:    resource.RotateSize,
			Size:          resource.Size,
			Downloaded:    resource.Downloaded,
			Speed:         resource.Speed,
		})
	}
	return &hermes.TaskJob{
		ID:          task.Id,
		Name:        task.Name,
		UniqueID:    task.UniqueID,
		DownloadDir: strings.TrimSpace(download_dir),
		Platform:    task.PlatformId,
		Resources:   resource_infos,
		Config:      config,
		Metadata:    metadata,
	}, nil
}

func (s *DBTaskStore) UpdateStatus(task_id int, status int) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&model.DownloadTask{}).Where("id = ?", task_id).
		Updates(map[string]any{"status": status, "updated_at": now}).Error
}

func (s *DBTaskStore) ActivateTask(task_id int) error {
	now := time.Now().UnixMilli()
	return s.db.Transaction(func(tx *gorm.DB) error {
		var endpoints []model.DownloadEndpoint
		if err := tx.
			Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", task_id).
			Find(&endpoints).Error; err != nil {
			return err
		}

		endpoint_ids := make([]int, 0, len(endpoints))
		for endpoint_index := range endpoints {
			endpoint_ids = append(endpoint_ids, endpoints[endpoint_index].Id)
		}
		existing_endpoint_ids := make(map[int]struct{}, len(endpoint_ids))
		if len(endpoint_ids) > 0 {
			var persisted_endpoint_ids []int
			if err := tx.Model(&model.DownloadConnection{}).
				Where("endpoint_id IN ?", endpoint_ids).
				Pluck("endpoint_id", &persisted_endpoint_ids).Error; err != nil {
				return err
			}
			for _, endpoint_id := range persisted_endpoint_ids {
				existing_endpoint_ids[endpoint_id] = struct{}{}
			}
		}

		connections := make([]model.DownloadConnection, 0, len(endpoints))
		for endpoint_index := range endpoints {
			endpoint := endpoints[endpoint_index]
			if _, exists := existing_endpoint_ids[endpoint.Id]; exists {
				continue
			}
			host := ""
			if parsed_url, err := url.Parse(endpoint.URL); err == nil {
				host = parsed_url.Host
			}
			connection := model.DownloadConnection{
				EndpointId: endpoint.Id,
				WorkerId:   "worker-" + strconv.Itoa(endpoint.Id),
				Host:       host,
				Status:     1,
				Bytes:      0,
				Speed:      0,
				LastActive: now,
			}
			connection.CreatedAt = now
			connection.UpdatedAt = now
			connections = append(connections, connection)
		}
		if len(connections) > 0 {
			if err := tx.CreateInBatches(&connections, persistence_batch_size).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&model.DownloadResource{}).
			Where("task_id = ? AND start_time IS NULL", task_id).
			Updates(map[string]any{"start_time": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.DownloadEndpoint{}).
			Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", task_id).
			Updates(map[string]any{"status": 1, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.DownloadConnection{}).
			Where("endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?))", task_id).
			Updates(map[string]any{"status": 1, "last_active": now, "updated_at": now}).Error
	})
}

func (s *DBTaskStore) UpdateProgress(task_id int, downloaded int64, speed int64) error {
	now := time.Now().UnixMilli()
	if err := s.db.Exec(`UPDATE download_connection SET speed = ?, bytes = ?, last_active = ?, updated_at = ?
		WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN (
				SELECT id FROM download_resource WHERE task_id = ?
			)
		)`, speed, downloaded, now, now, task_id).Error; err != nil {
		return err
	}
	return s.db.Exec(`UPDATE download_resource SET status = 1, updated_at = ? WHERE task_id = ? AND status IN (0,1)`,
		now, task_id).Error
}

func (s *DBTaskStore) UpdateResourceSize(task_id int, size int64) error {
	now := time.Now().UnixMilli()
	return s.db.Exec(`UPDATE download_resource SET size = ?, updated_at = ? WHERE task_id = ?`,
		size, now, task_id).Error
}

func (s *DBTaskStore) UpdateOutputName(update hermes.OutputNameUpdate) error {
	s.debug("UpdateOutputName called: task_id=%d resource_id=%d task_name=%q resource_name=%q",
		update.TaskID, update.ResourceID, update.TaskName, update.ResourceName)

	if update.TaskID <= 0 || update.ResourceID <= 0 || strings.TrimSpace(update.ResourceName) == "" {
		s.debug("UpdateOutputName invalid parameters, skipping update")
		return errors.New("下载文件名更新参数无效")
	}

	now := time.Now().UnixMilli()
	s.debug("UpdateOutputName starting transaction to update download_resource: id=%d task_id=%d name=%q",
		update.ResourceID, update.TaskID, update.ResourceName)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.DownloadResource{}).
			Where("id = ? AND task_id = ?", update.ResourceID, update.TaskID).
			Updates(map[string]any{"name": update.ResourceName, "updated_at": now})
		if result.Error != nil {
			s.debug("UpdateOutputName download_resource update failed: %v", result.Error)
			return result.Error
		}
		s.debug("UpdateOutputName download_resource RowsAffected=%d", result.RowsAffected)
		if result.RowsAffected == 0 {
			return fmt.Errorf("更新资源名未影响任何行: resource_id=%d task_id=%d new_name=%q",
				update.ResourceID, update.TaskID, update.ResourceName)
		}
		if update.TaskName == "" {
			return nil
		}
		return tx.Model(&model.DownloadTask{}).Where("id = ?", update.TaskID).
			Updates(map[string]any{"name": update.TaskName, "updated_at": now}).Error
	})
	if err != nil {
		s.debug("UpdateOutputName transaction failed: %v", err)
	} else {
		s.debug("UpdateOutputName transaction succeeded")
	}
	return err
}

func (s *DBTaskStore) UpdateResourceOutput(update hermes.ResourceOutputUpdate) error {
	if update.ResourceID <= 0 || strings.TrimSpace(update.ResourceName) == "" {
		return errors.New("最终资源更新参数无效")
	}
	now := time.Now().UnixMilli()
	query := s.db.Model(&model.DownloadResource{}).Where("id = ?", update.ResourceID)
	if update.TaskID > 0 {
		query = query.Where("task_id = ?", update.TaskID)
	}
	result := query.Updates(map[string]any{
		"download_dir": update.DownloadDir,
		"name":         update.ResourceName,
		"kind":         update.ResourceKind,
		"size":         update.ResourceSize,
		"updated_at":   now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("更新最终资源未影响任何行: resource_id=%d task_id=%d", update.ResourceID, update.TaskID)
	}
	return nil
}

func (s *DBTaskStore) DeleteStaleResources(task_id int, stale_resource_ids []int) error {
	if task_id <= 0 || len(stale_resource_ids) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete connections for endpoints of stale resources.
		if err := tx.Exec(`DELETE FROM download_connection WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN ?
		)`, stale_resource_ids).Error; err != nil {
			return err
		}
		// Delete endpoints of stale resources.
		if err := tx.Where("resource_id IN ?", stale_resource_ids).
			Delete(&model.DownloadEndpoint{}).Error; err != nil {
			return err
		}
		// Delete segments of stale resources.
		if err := tx.Where("resource_id IN ?", stale_resource_ids).
			Delete(&model.DownloadSegment{}).Error; err != nil {
			return err
		}
		// Delete the stale resources.
		if err := tx.Where("id IN ? AND task_id = ?", stale_resource_ids, task_id).
			Delete(&model.DownloadResource{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *DBTaskStore) UpdateResourceProgress(resource_id int, downloaded int64, speed int64) error {
	now := time.Now().UnixMilli()
	return s.updateResourceProgress(s.db, resource_id, downloaded, speed, now)
}

func (s *DBTaskStore) updateResourceProgress(db *gorm.DB, resource_id int, downloaded int64, speed int64, now int64) error {
	if err := db.Exec(`UPDATE download_connection SET speed = ?, bytes = ?, last_active = ?, updated_at = ?
		WHERE endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id = ?)`,
		speed, downloaded, now, now, resource_id).Error; err != nil {
		return err
	}
	return db.Exec(`UPDATE download_resource SET status = 1, downloaded = ?, speed = ?, updated_at = ? WHERE id = ?`,
		downloaded, speed, now, resource_id).Error
}

func (s *DBTaskStore) UpdateResourceSegmentProgress(resource_id int, segment_id int, downloaded int64, speed int64) error {
	now := time.Now().UnixMilli()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE download_segment SET downloaded = ?, updated_at = ? WHERE id = ?`,
			downloaded, now, segment_id).Error; err != nil {
			return err
		}
		return s.updateResourceProgress(tx, resource_id, downloaded, speed, now)
	})
}

func (s *DBTaskStore) UpdateAggregateResourceProgress(resource_id int, updates []hermes.SegmentProgressUpdate, downloaded int64, speed int64) error {
	now := time.Now().UnixMilli()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := update_segment_progress_batch(tx, updates, now); err != nil {
			return err
		}
		return s.updateResourceProgress(tx, resource_id, downloaded, speed, now)
	})
}

func update_segment_progress_batch(db *gorm.DB, updates []hermes.SegmentProgressUpdate, now int64) error {
	normalized_updates := make([]hermes.SegmentProgressUpdate, 0, len(updates))
	update_indexes := make(map[int]int, len(updates))
	for _, update := range updates {
		if update.SegmentID <= 0 {
			continue
		}
		if update_index, exists := update_indexes[update.SegmentID]; exists {
			normalized_updates[update_index].Downloaded = update.Downloaded
			continue
		}
		update_indexes[update.SegmentID] = len(normalized_updates)
		normalized_updates = append(normalized_updates, update)
	}

	for batch_start := 0; batch_start < len(normalized_updates); batch_start += persistence_batch_size {
		batch_end := batch_start + persistence_batch_size
		if batch_end > len(normalized_updates) {
			batch_end = len(normalized_updates)
		}
		batch := normalized_updates[batch_start:batch_end]
		var query strings.Builder
		query.Grow(96 + len(batch)*24)
		query.WriteString("UPDATE download_segment SET downloaded = CASE id")
		query_args := make([]any, 0, len(batch)*3+1)
		for _, update := range batch {
			query.WriteString(" WHEN ? THEN ?")
			query_args = append(query_args, update.SegmentID, update.Downloaded)
		}
		query.WriteString(" ELSE downloaded END, updated_at = ? WHERE id IN (")
		query_args = append(query_args, now)
		for update_index, update := range batch {
			if update_index > 0 {
				query.WriteByte(',')
			}
			query.WriteByte('?')
			query_args = append(query_args, update.SegmentID)
		}
		query.WriteByte(')')
		if err := db.Exec(query.String(), query_args...).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *DBTaskStore) UpdateResourceSizeByID(resource_id int, size int64) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&model.DownloadResource{}).Where("id = ?", resource_id).
		Updates(map[string]any{"size": size, "status": 1, "updated_at": now}).Error
}

func (s *DBTaskStore) FinishResource(resource_id int) error {
	now := time.Now().UnixMilli()
	if err := s.db.Model(&model.DownloadResource{}).Where("id = ?", resource_id).
		Updates(map[string]any{"status": 2, "finish_time": now, "updated_at": now}).Error; err != nil {
		return err
	}
	return s.db.Exec(`UPDATE download_connection SET speed = 0, status = 2, updated_at = ?
		WHERE endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id = ?)`, now, resource_id).Error
}

func (s *DBTaskStore) DeactivateConnections(task_id int) error {
	now := time.Now().UnixMilli()

	// Update resource status for this task's downloading resources.
	if err := s.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND status = 1", task_id).
		Updates(map[string]any{"status": 1, "updated_at": now}).Error; err != nil {
		return err
	}

	// Update segment status for this task's active segments.
	if err := s.db.Model(&model.DownloadSegment{}).
		Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?) AND status = 1", task_id).
		Updates(map[string]any{"status": 1, "updated_at": now}).Error; err != nil {
		return err
	}

	// Deactivate connections.
	return s.db.Exec(`UPDATE download_connection SET speed = 0, status = 2, updated_at = ?
		WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN (
				SELECT id FROM download_resource WHERE task_id = ?
			)
		)`, now, task_id).Error
}

func (s *DBTaskStore) FinishTask(task_id int) error {
	now := time.Now().UnixMilli()
	s.db.Model(&model.DownloadTask{}).Where("id = ?", task_id).
		Updates(map[string]any{"status": model.TaskStatusFinished, "updated_at": now})
	s.db.Exec(`UPDATE download_segment SET downloaded = CASE WHEN size > 0 THEN size ELSE downloaded END, status = 2, updated_at = ?
		WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)`, now, task_id)
	s.db.Exec(`UPDATE download_connection SET speed = 0, status = 2, updated_at = ?
		WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN (
				SELECT id FROM download_resource WHERE task_id = ?
			)
		)`, now, task_id)
	s.db.Exec(`UPDATE download_resource SET status = 2, updated_at = ? WHERE task_id = ?`, now, task_id)
	return nil
}

func (s *DBTaskStore) RecordError(task_id int, err_msg string) error {
	return s.db.Model(&model.DownloadTask{}).Where("id = ?", task_id).
		Updates(map[string]any{"error_message": err_msg, "updated_at": time.Now().UnixMilli()}).Error
}

func (s *DBTaskStore) CreateSegments(resource_id int, url string, ranges []hermes.SegmentRange) ([]int, error) {
	now := time.Now().UnixMilli()
	segments := make([]model.DownloadSegment, len(ranges))
	for range_index := range ranges {
		range_info := ranges[range_index]
		segments[range_index] = model.DownloadSegment{
			ResourceId:  resource_id,
			Index:       range_info.Index,
			URL:         url,
			OffsetStart: range_info.OffsetStart,
			OffsetEnd:   range_info.OffsetEnd,
			Size:        range_info.Size,
			Downloaded:  0,
			Status:      1,
			Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("resource_id = ?", resource_id).Delete(&model.DownloadSegment{}).Error; err != nil {
			return err
		}
		if len(segments) > 0 {
			if err := tx.CreateInBatches(&segments, persistence_batch_size).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	ids := make([]int, len(segments))
	for segment_index := range segments {
		ids[segment_index] = segments[segment_index].Id
	}
	return ids, nil
}

func (s *DBTaskStore) UpdateSegmentProgress(seg_id int, downloaded int64) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&model.DownloadSegment{}).Where("id = ?", seg_id).
		Updates(map[string]any{"downloaded": downloaded, "updated_at": now}).Error
}

// SyncStreamSegments upserts the time-based chunks produced by a live-stream
// recorder. Existing rows are retained across pause/resume so the database
// reflects the same deterministic chunk sequence that remains on disk.
func (s *DBTaskStore) SyncStreamSegments(resource_id int, stream_url string, segments []hermes.StreamSegmentState) error {
	if resource_id <= 0 || len(segments) == 0 {
		return nil
	}
	normalized_states := make([]hermes.StreamSegmentState, 0, len(segments))
	state_indexes := make(map[int]int, len(segments))
	for _, state := range segments {
		if state_index, exists := state_indexes[state.Index]; exists {
			normalized_states[state_index] = state
			continue
		}
		state_indexes[state.Index] = len(normalized_states)
		normalized_states = append(normalized_states, state)
	}

	now := time.Now().UnixMilli()
	return s.db.Transaction(func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{DisableNestedTransaction: true})
		var persisted_segments []model.DownloadSegment
		if err := tx.
			Select("id", "resource_id", "`index`", "url", "size", "downloaded", "status").
			Where("resource_id = ?", resource_id).
			Find(&persisted_segments).Error; err != nil {
			return err
		}
		persisted_by_index := make(map[int]model.DownloadSegment, len(persisted_segments))
		for _, persisted_segment := range persisted_segments {
			persisted_by_index[persisted_segment.Index] = persisted_segment
		}

		new_segments := make([]model.DownloadSegment, 0, len(normalized_states))
		changed_segments := make([]model.DownloadSegment, 0, len(normalized_states))
		for _, state := range normalized_states {
			status := 1
			if state.Complete {
				status = 2
			}
			persisted_segment, exists := persisted_by_index[state.Index]
			if !exists {
				new_segments = append(new_segments, model.DownloadSegment{
					ResourceId: resource_id,
					Index:      state.Index,
					URL:        stream_url,
					Size:       state.Size,
					Downloaded: state.Downloaded,
					Status:     status,
					Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
				})
				continue
			}
			if persisted_segment.URL == stream_url &&
				persisted_segment.Size == state.Size &&
				persisted_segment.Downloaded == state.Downloaded &&
				persisted_segment.Status == status {
				continue
			}
			persisted_segment.URL = stream_url
			persisted_segment.Size = state.Size
			persisted_segment.Downloaded = state.Downloaded
			persisted_segment.Status = status
			changed_segments = append(changed_segments, persisted_segment)
		}

		if len(new_segments) > 0 {
			if err := tx.CreateInBatches(&new_segments, persistence_batch_size).Error; err != nil {
				return err
			}
		}
		return update_stream_segments_batch(tx, changed_segments, stream_url, now)
	})
}

func update_stream_segments_batch(db *gorm.DB, segments []model.DownloadSegment, stream_url string, now int64) error {
	for batch_start := 0; batch_start < len(segments); batch_start += persistence_batch_size {
		batch_end := batch_start + persistence_batch_size
		if batch_end > len(segments) {
			batch_end = len(segments)
		}
		batch := segments[batch_start:batch_end]
		var query strings.Builder
		query.Grow(160 + len(batch)*64)
		query.WriteString("UPDATE download_segment SET url = ?, size = CASE id")
		query_args := make([]any, 0, len(batch)*7+2)
		query_args = append(query_args, stream_url)
		for _, segment := range batch {
			query.WriteString(" WHEN ? THEN ?")
			query_args = append(query_args, segment.Id, segment.Size)
		}
		query.WriteString(" ELSE size END, downloaded = CASE id")
		for _, segment := range batch {
			query.WriteString(" WHEN ? THEN ?")
			query_args = append(query_args, segment.Id, segment.Downloaded)
		}
		query.WriteString(" ELSE downloaded END, status = CASE id")
		for _, segment := range batch {
			query.WriteString(" WHEN ? THEN ?")
			query_args = append(query_args, segment.Id, segment.Status)
		}
		query.WriteString(" ELSE status END, updated_at = ? WHERE id IN (")
		query_args = append(query_args, now)
		for segment_index, segment := range batch {
			if segment_index > 0 {
				query.WriteByte(',')
			}
			query.WriteByte('?')
			query_args = append(query_args, segment.Id)
		}
		query.WriteByte(')')
		if err := db.Exec(query.String(), query_args...).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *DBTaskStore) UpdateStreamDuration(resource_id int, duration_seconds int64) error {
	return s.db.Model(&model.DownloadResource{}).Where("id = ?", resource_id).
		Updates(map[string]any{"duration": duration_seconds, "updated_at": time.Now().UnixMilli()}).Error
}

func (s *DBTaskStore) LoadSegmentInfo(resource_id int) ([]hermes.Segment, error) {
	var segs []model.DownloadSegment
	if err := s.db.Where("resource_id = ?", resource_id).Order("`index` ASC").Find(&segs).Error; err != nil {
		return nil, err
	}
	infos := make([]hermes.Segment, len(segs))
	for i, s := range segs {
		infos[i] = hermes.Segment{
			ID:          s.Id,
			Index:       s.Index,
			URL:         s.URL,
			OffsetStart: s.OffsetStart,
			OffsetEnd:   s.OffsetEnd,
			Size:        s.Size,
			Downloaded:  s.Downloaded,
		}
	}
	return infos, nil
}

// parseExtra parses a JSON string into map[string]string for passing through user-defined fields.
// It tolerates non-string values (e.g. numbers) by converting them to strings.
func parseExtra(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var attrs map[string]any
	if err := json.Unmarshal([]byte(raw), &attrs); err != nil {
		return nil
	}

	extras := make(map[string]string, len(attrs))
	for key, value := range attrs {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			extras[key] = typed
		case fmt.Stringer:
			extras[key] = typed.String()
		default:
			extras[key] = strings.TrimSpace(fmt.Sprintf("%v", value))
		}
	}

	if len(extras) == 0 {
		return nil
	}

	return extras

}
