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
		resource_infos = append(resource_infos, hermes.ResourceJob{
			ID:         resource.Id,
			Name:       resource.Name,
			Kind:       resource.Kind,
			Type:       resource.Type,
			UniqueID:   resource.UniqueID,
			Endpoints:  resource_endpoints,
			Extra:      extra,
			Size:       resource.Size,
			Downloaded: resource.Downloaded,
			Speed:      resource.Speed,
		})
	}
	return &hermes.TaskJob{
		ID:        task.Id,
		Name:      task.Name,
		UniqueID:  task.UniqueID,
		Platform:  task.PlatformId,
		Resources: resource_infos,
		Config:    config,
		Metadata:  metadata,
	}, nil
}

func (s *DBTaskStore) UpdateStatus(task_id int, status int) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&model.DownloadTask{}).Where("id = ?", task_id).
		Updates(map[string]any{"status": status, "updated_at": now}).Error
}

func (s *DBTaskStore) ActivateTask(task_id int) error {
	now := time.Now().UnixMilli()

	// Get all endpoints for this task.
	var endpoints []model.DownloadEndpoint
	if err := s.db.Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", task_id).Find(&endpoints).Error; err != nil {
		return err
	}

	// Create connections for endpoints that don't have one yet.
	for _, ep := range endpoints {
		var count int64
		if err := s.db.Model(&model.DownloadConnection{}).Where("endpoint_id = ?", ep.Id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			host := ""
			if parsed_url, err := url.Parse(ep.URL); err == nil {
				host = parsed_url.Host
			}
			conn := model.DownloadConnection{
				EndpointId: ep.Id,
				WorkerId:   "worker-" + strconv.Itoa(ep.Id),
				Host:       host,
				Status:     1,
				Bytes:      0,
				Speed:      0,
				LastActive: now,
			}
			conn.CreatedAt = now
			conn.UpdatedAt = now
			if err := s.db.Create(&conn).Error; err != nil {
				return err
			}
		}
	}

	// Set start_time for all resources.
	if err := s.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND start_time IS NULL", task_id).
		Updates(map[string]any{"start_time": now, "updated_at": now}).Error; err != nil {
		return err
	}

	// Activate all endpoints.
	if err := s.db.Model(&model.DownloadEndpoint{}).
		Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", task_id).
		Updates(map[string]any{"status": 1, "updated_at": now}).Error; err != nil {
		return err
	}

	// Activate all connections.
	return s.db.Model(&model.DownloadConnection{}).
		Where("endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?))", task_id).
		Updates(map[string]any{"status": 1, "last_active": now, "updated_at": now}).Error
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
	s.debug("UpdateOutputName called: task_id=%d resource_id=%d task_name=%q resource_name=%q save_path=%q",
		update.TaskID, update.ResourceID, update.TaskName, update.ResourceName, update.SavePath)

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
			Updates(map[string]any{"name": update.TaskName, "save_path": update.SavePath, "updated_at": now}).Error
	})
	if err != nil {
		s.debug("UpdateOutputName transaction failed: %v", err)
	} else {
		s.debug("UpdateOutputName transaction succeeded")
	}
	return err
}

func (s *DBTaskStore) UpdateResourceOutput(update hermes.ResourceOutputUpdate) error {
	if update.TaskID <= 0 || update.ResourceID <= 0 || strings.TrimSpace(update.ResourceName) == "" {
		return errors.New("最终资源更新参数无效")
	}
	now := time.Now().UnixMilli()
	result := s.db.Model(&model.DownloadResource{}).
		Where("id = ? AND task_id = ?", update.ResourceID, update.TaskID).
		Updates(map[string]any{
			"name":       update.ResourceName,
			"kind":       update.ResourceKind,
			"size":       update.ResourceSize,
			"updated_at": now,
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
	if err := s.db.Exec(`UPDATE download_connection SET speed = ?, bytes = ?, last_active = ?, updated_at = ?
		WHERE endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id = ?)`,
		speed, downloaded, now, now, resource_id).Error; err != nil {
		return err
	}
	return s.db.Model(&model.DownloadResource{}).Where("id = ?", resource_id).
		Updates(map[string]any{"status": 1, "downloaded": downloaded, "speed": speed, "updated_at": now}).Error
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
	var ids []int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("resource_id = ?", resource_id).Delete(&model.DownloadSegment{}).Error; err != nil {
			return err
		}
		for _, r := range ranges {
			seg := model.DownloadSegment{
				ResourceId:  resource_id,
				Index:       r.Index,
				URL:         url,
				OffsetStart: r.OffsetStart,
				OffsetEnd:   r.OffsetEnd,
				Size:        r.Size,
				Downloaded:  0,
				Status:      1,
			}
			seg.CreatedAt = now
			seg.UpdatedAt = now
			if err := tx.Create(&seg).Error; err != nil {
				return err
			}
			ids = append(ids, seg.Id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *DBTaskStore) UpdateSegmentProgress(seg_id int, downloaded int64) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&model.DownloadSegment{}).Where("id = ?", seg_id).
		Updates(map[string]any{"downloaded": downloaded, "updated_at": now}).Error
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
