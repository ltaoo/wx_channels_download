package services

import (
	"context"
	"fmt"
	"strings"

	"wx_channel/internal/database/model"
)

const MaxDownloadTaskContentChecks = 10000

type DownloadTaskContentCheck struct {
	ContentID string `json:"content_id"`
	Exists    bool   `json:"exists"`
	TaskCount int64  `json:"task_count"`
}

type DownloadTaskContentChecks struct {
	List  []DownloadTaskContentCheck `json:"list"`
	Total int                        `json:"total"`
}

// CheckContentIDs checks task existence without loading task details or files.
// Results follow the first occurrence of each trimmed ID, including missing IDs.
func (s *DownloadTaskService) CheckContentIDs(ctx context.Context, content_ids []string) (*DownloadTaskContentChecks, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("下载任务数据库服务未初始化")
	}
	if len(content_ids) > MaxDownloadTaskContentChecks {
		return nil, fmt.Errorf("content_ids 最多允许 %d 项", MaxDownloadTaskContentChecks)
	}
	ids := make([]string, 0, len(content_ids))
	seen := make(map[string]bool, len(content_ids))
	for _, raw := range content_ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, fmt.Errorf("content_ids 不能包含空 ID")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	counts := make(map[string]int64, len(ids))
	for start := 0; start < len(ids); start += duplicate_query_batch_size {
		end := start + duplicate_query_batch_size
		if end > len(ids) {
			end = len(ids)
		}
		var rows []struct {
			ContentID string `gorm:"column:content_id"`
			TaskCount int64  `gorm:"column:task_count"`
		}
		if err := s.db.WithContext(ctx).Model(&model.DownloadTask{}).
			Select("content_id, COUNT(*) AS task_count").
			Where("content_id IN ? AND deleted_at IS NULL", ids[start:end]).
			Group("content_id").Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("批量查询下载任务失败: %w", err)
		}
		for _, row := range rows {
			counts[row.ContentID] = row.TaskCount
		}
	}
	result := &DownloadTaskContentChecks{List: make([]DownloadTaskContentCheck, 0, len(ids)), Total: len(ids)}
	for _, id := range ids {
		result.List = append(result.List, DownloadTaskContentCheck{ContentID: id, Exists: counts[id] > 0, TaskCount: counts[id]})
	}
	return result, nil
}
