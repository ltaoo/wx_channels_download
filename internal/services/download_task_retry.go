package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
)

func (s *DownloadTaskService) reset_retry_task(task *model.DownloadTask) error {
	info, err := s.fetch_retry_task(*task)
	if err != nil {
		return err
	}
	updated_task := *task
	now := time.Now().UnixMilli()
	if info != nil {
		updated_task.Name = info.Task.Name
		updated_task.SourceURL = info.Task.SourceURL
		updated_task.CoverURL, updated_task.CoverWidth, updated_task.CoverHeight = info.Task.CoverURL, info.Task.CoverWidth, info.Task.CoverHeight
		updated_task.MetadataJSON = info.Task.MetadataJSON
		if info.Content != nil {
			updated_task.ContentId = &info.Content.Id
		}
	}
	updated_task.Status, updated_task.ErrorMessage, updated_task.UpdatedAt = model.TaskStatusWaiting, "", now
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		claimed := tx.Model(&model.DownloadTask{}).
			Where("id = ? AND status IN ? AND updated_at = ? AND deleted_at IS NULL", task.Id, []int{model.TaskStatusFailed, model.TaskStatusCancelled}, task.UpdatedAt).
			Updates(map[string]any{"status": model.TaskStatusWaiting, "error_message": "", "updated_at": now})
		if claimed.Error != nil {
			return claimed.Error
		}
		if claimed.RowsAffected != 1 {
			return fmt.Errorf("下载任务已变更，请重新重试")
		}
		if info == nil {
			return nil
		}
		var config struct {
			DownloadDir string `json:"download_dir"`
		}
		if strings.TrimSpace(task.ConfigJSON) != "" {
			if err := json.Unmarshal([]byte(task.ConfigJSON), &config); err != nil {
				return err
			}
		}
		var resources []model.DownloadResource
		if err := tx.Where("task_id = ?", task.Id).Order("merge_order ASC, id ASC").Find(&resources).Error; err != nil {
			return err
		}
		resources_by_key := make(map[string]model.DownloadResource, len(resources))
		resource_key := func(resource model.DownloadResource) string {
			if resource.UniqueID != "" {
				return resource.UniqueID
			}
			return resource.Name + "\x00" + resource.Kind
		}
		for _, resource := range resources {
			resources_by_key[resource_key(resource)] = resource
		}
		selected_resources := make([]*adapter.ResourceInfo, 0, len(info.Resources))
		retained_ids := make([]int, 0, len(info.Resources))
		for _, resource_info := range info.Resources {
			resource := &resource_info.Resource
			previous, exists := resources_by_key[resource_key(*resource)]
			if strings.Contains(task.UniqueID, "_resources_") && !exists {
				continue
			}
			resource.Id, resource.TaskId = 0, &task.Id
			if exists {
				delete(resources_by_key, resource_key(*resource))
				// Keep the durable output path and offsets when only signed URLs change.
				resource.Id, resource.Name, resource.DownloadDir = previous.Id, previous.Name, previous.DownloadDir
				resource.CreatedAt, resource.Downloaded, resource.Status = previous.CreatedAt, previous.Downloaded, previous.Status
				resource.StartTime, resource.FinishTime = previous.StartTime, previous.FinishTime
				if resource.Size == 0 {
					resource.Size = previous.Size
				}
				var previous_extra, fresh_extra map[string]string
				if err := json.Unmarshal([]byte(previous.Extra), &previous_extra); err != nil && strings.TrimSpace(previous.Extra) != "" {
					return fmt.Errorf("解析原资源信息失败: %w", err)
				}
				if previous_extra == nil {
					previous_extra = make(map[string]string)
				}
				if err := json.Unmarshal([]byte(resource.Extra), &fresh_extra); err != nil && strings.TrimSpace(resource.Extra) != "" {
					return fmt.Errorf("解析新资源信息失败: %w", err)
				}
				for key, value := range fresh_extra {
					previous_extra[key] = value
				}
				extra_json, err := json.Marshal(previous_extra)
				if err != nil {
					return err
				}
				resource.Extra = string(extra_json)
			} else {
				resource.DownloadDir = config.DownloadDir
			}
			retained_ids = append(retained_ids, resource.Id)
			for index := range resource_info.Endpoints {
				resource_info.Endpoints[index].Id = 0
			}
			selected_resources = append(selected_resources, resource_info)
		}
		if len(selected_resources) == 0 || (strings.Contains(task.UniqueID, "_resources_") && len(selected_resources) != len(resources)) {
			return fmt.Errorf("来源已不包含原先选择的下载资源")
		}
		info.Resources = selected_resources
		resource_ids := tx.Model(&model.DownloadResource{}).Select("id").Where("task_id = ?", task.Id)
		endpoint_ids := tx.Model(&model.DownloadEndpoint{}).Select("id").Where("resource_id IN (?)", resource_ids)
		if err := tx.Where("endpoint_id IN (?)", endpoint_ids).Delete(&model.DownloadConnection{}).Error; err != nil {
			return err
		}
		if err := tx.Where("resource_id IN (?)", resource_ids).Delete(&model.DownloadEndpoint{}).Error; err != nil {
			return err
		}
		obsolete_ids := tx.Model(&model.DownloadResource{}).Select("id").Where("task_id = ? AND id NOT IN ?", task.Id, retained_ids)
		for _, record := range []any{&model.DownloadSegment{}, &model.DownloadResourceAsset{}} {
			if err := tx.Where("resource_id IN (?)", obsolete_ids).Delete(record).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("task_id = ? AND id NOT IN ?", task.Id, retained_ids).Delete(&model.DownloadResource{}).Error; err != nil {
			return err
		}
		_, err := persist_download_task_graph(tx, info, &updated_task, "", task.ParentTaskID, task.RelationType, now, true)
		return err
	}); err != nil {
		return err
	}
	*task = updated_task
	return nil
}

func (s *DownloadTaskService) fetch_retry_task(task model.DownloadTask) (*adapter.DownloadTaskResult, error) {
	if strings.TrimSpace(task.PlatformId) == "" || strings.TrimSpace(task.SourceURL) == "" {
		return nil, nil
	}
	handler := adapter.Get(task.PlatformId)
	if handler == nil {
		return nil, fmt.Errorf("不支持的平台: %s", task.PlatformId)
	}
	config_json := json.RawMessage(task.ConfigJSON)
	if strings.TrimSpace(task.ConfigJSON) == "" {
		config_json = json.RawMessage(`{}`)
	}
	fetch_context, cancel := context.WithTimeout(context.Background(), time.Duration(default_platform_fetch_timeout_seconds)*time.Second)
	defer cancel()
	var data any
	var err error
	if contextual, ok := handler.(adapter.ContextProgressFetchAdapter); ok {
		data, err = contextual.FetchWithProgressContext(fetch_context, task.SourceURL, adapter.FetchOptions{ForceRefresh: true})
	} else {
		if cached, ok := handler.(adapter.FetchCacheAdapter); ok {
			if _, err := cached.ClearFetchCache(task.SourceURL); err != nil {
				return nil, fmt.Errorf("清除过期详情缓存失败: %w", err)
			}
		}
		data, err = handler.Fetch(task.SourceURL)
	}
	if err != nil {
		return nil, fmt.Errorf("刷新平台详情失败: %w", err)
	}
	data, err = prepare_download_task_resources(handler, data, nil)
	if err != nil {
		return nil, fmt.Errorf("刷新下载资源失败: %w", err)
	}
	var info *adapter.DownloadTaskResult
	if builder, ok := handler.(adapter.FetchDownloadTaskBuilder); ok {
		info, err = builder.BuildDownloadTaskFromFetch(data, config_json)
	} else {
		content_json, marshal_err := json.Marshal(data)
		if marshal_err != nil {
			return nil, marshal_err
		}
		info, err = handler.BuildDownloadTask(content_json, config_json)
	}
	if err != nil {
		return nil, fmt.Errorf("重新构建下载任务失败: %w", err)
	}
	if info == nil || info.Task == nil || len(info.Resources) == 0 {
		return nil, fmt.Errorf("平台未返回可下载资源")
	}
	for _, resource := range info.Resources {
		if resource == nil {
			return nil, fmt.Errorf("平台返回空下载资源")
		}
	}
	if err := validate_download_task_endpoints(info); err != nil {
		return nil, err
	}
	if info.Content != nil && task.ContentId != nil && info.Content.Id != *task.ContentId {
		return nil, fmt.Errorf("刷新后的内容与原下载任务不一致")
	}
	if strings.TrimSpace(info.Task.SourceURL) == "" {
		info.Task.SourceURL = task.SourceURL
	}
	normalize_content_taxonomy(info.Content)
	return info, nil
}
