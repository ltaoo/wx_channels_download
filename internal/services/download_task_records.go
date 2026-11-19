package services

import (
	"fmt"

	"gorm.io/gorm"
	"wx_channel/internal/adapter"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
)

// persist_download_task_graph is shared by ordinary task creation and offline
// recreation. It only writes records; starting downloads belongs to the caller.
func persist_download_task_graph(tx *gorm.DB, info *adapter.DownloadTaskResult, task *model.DownloadTask, download_dir string, parent_id *int, relation string, now int64, archived bool) (*CreateTaskResult, error) {
	tx = tx.Session(&gorm.Session{DisableNestedTransaction: true})
	if err := database.ApplyTaskLineage(tx, task, parent_id, relation); err != nil {
		return nil, err
	}
	var err error
	if archived && task.Id > 0 {
		err = tx.Save(task).Error
	} else {
		err = tx.Create(task).Error
	}
	if err != nil {
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}
	if err := database.FinalizeTaskRoot(tx, task); err != nil {
		return nil, err
	}
	content, account := info.Content, info.Account
	if content != nil {
		content.UpdatedAt = now
		if err := save_content_with_text_tracks(tx, content, now); err != nil {
			return nil, fmt.Errorf("保存 Content 失败: %w", err)
		}
	}
	if account != nil && account.ExternalId != "" {
		content_id := ""
		if content != nil {
			content_id = content.Id
		}
		account, err = NewContentService(tx).UpsertAccountAndLinkContent(content_id, account, "owner", now)
		if err != nil {
			return nil, err
		}
	}
	if err := save_content_details(tx, content, info.ContentDetail, info.ContentDetails, info.Resources, now); err != nil {
		return nil, fmt.Errorf("保存内容详情与关联关系失败: %w", err)
	}
	if len(info.Resources) == 0 && !archived {
		return nil, fmt.Errorf("平台未返回可下载资源或端点")
	}
	resources := make([]model.DownloadResource, len(info.Resources))
	for resource_index, resource_info := range info.Resources {
		resource := resource_info.Resource
		resource.TaskId = &task.Id
		if !archived {
			resource.DownloadDir = download_dir
		}
		if resource.CreatedAt == 0 {
			resource.CreatedAt = now
		}
		resource.UpdatedAt = now
		resources[resource_index] = resource
	}
	if archived {
		for index := range resources {
			if err := tx.Save(&resources[index]).Error; err != nil {
				return nil, err
			}
		}
	} else if err := tx.CreateInBatches(&resources, download_task_persistence_batch_size).Error; err != nil {
		return nil, fmt.Errorf("批量创建资源失败: %w", err)
	}
	var endpoints []model.DownloadEndpoint
	for index := range resources {
		if err := save_download_resource_assets(tx, &resources[index], info.Resources[index], content, info.ContentDetail, now); err != nil {
			return nil, fmt.Errorf("关联下载资源与内容资产失败: %w", err)
		}
		for _, endpoint := range info.Resources[index].Endpoints {
			endpoint.ResourceId = resources[index].Id
			if endpoint.CreatedAt == 0 {
				endpoint.CreatedAt = now
			}
			endpoint.UpdatedAt = now
			if archived {
				var existing model.DownloadEndpoint
				find_err := tx.Where("resource_id = ? AND protocol = ? AND url = ?", endpoint.ResourceId, endpoint.Protocol, endpoint.URL).First(&existing).Error
				if find_err == nil {
					endpoint.Id = existing.Id
				} else if find_err != gorm.ErrRecordNotFound {
					return nil, find_err
				}
				if err := tx.Save(&endpoint).Error; err != nil {
					return nil, err
				}
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	if !archived {
		if len(endpoints) == 0 {
			return nil, fmt.Errorf("平台未返回可下载资源或端点")
		}
		if err := tx.CreateInBatches(&endpoints, download_task_persistence_batch_size).Error; err != nil {
			return nil, fmt.Errorf("批量创建端点失败: %w", err)
		}
	}
	result := &CreateTaskResult{Task: *task, Resources: resources, Endpoints: endpoints, Content: content, Account: account}
	if len(resources) > 0 {
		result.Resource = resources[0]
	}
	if len(endpoints) > 0 {
		result.Endpoint = endpoints[0]
	}
	return result, nil
}
