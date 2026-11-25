package services

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
	"wx_channel/internal/database/model"
)

type UpdateDownloadTaskBody struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Files []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	} `json:"files"`
}

// UpdateTask changes a failed task without starting it or replacing its files.
func (s *DownloadTaskService) UpdateTask(body UpdateDownloadTaskBody) error {
	if s == nil || s.db == nil {
		return errors.New("数据库不可用")
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.ID <= 0 || body.Name == "" {
		return errors.New("任务 ID 和名称不能为空")
	}
	seen_ids := make(map[int]bool, len(body.Files))
	for index := range body.Files {
		file := &body.Files[index]
		file.URL = strings.TrimSpace(file.URL)
		parsed_url, err := url.Parse(file.URL)
		if file.ID <= 0 || seen_ids[file.ID] || err != nil || parsed_url.Hostname() == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
			return errors.New("文件 ID 不合法或下载链接不是有效的 HTTP/HTTPS 地址")
		}
		seen_ids[file.ID] = true
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UnixMilli()
		updated := tx.Model(&model.DownloadTask{}).
			Where("id = ? AND status IN ?", body.ID, []int{model.TaskStatusFailed, model.TaskStatusCancelled}).
			Updates(map[string]any{"name": body.Name, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errors.New("任务不存在或当前状态不允许编辑，仅支持失败任务")
		}
		for _, file := range body.Files {
			var resource model.DownloadResource
			if err := tx.Where("id = ? AND task_id = ?", file.ID, body.ID).First(&resource).Error; err != nil {
				return fmt.Errorf("文件不属于当前下载任务: %w", err)
			}
			var endpoint model.DownloadEndpoint
			err := tx.Where("resource_id = ? AND enabled = 1", file.ID).Order("priority ASC, id ASC").First(&endpoint).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			parsed_url, _ := url.Parse(file.URL)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				endpoint = model.DownloadEndpoint{ResourceId: file.ID, Enabled: 1, Timestamps: model.Timestamps{CreatedAt: now}}
			}
			endpoint.URL, endpoint.Protocol, endpoint.UpdatedAt = file.URL, strings.ToUpper(parsed_url.Scheme), now
			if err := tx.Save(&endpoint).Error; err != nil {
				return err
			}
			if strings.EqualFold(resource.Type, model.ResourceTypeStream) {
				if err := tx.Model(&resource).Updates(map[string]any{"stream_url": file.URL, "updated_at": now}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
