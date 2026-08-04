package tasklineage

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

// Apply validates and assigns lineage before a DownloadTask is created.
func Apply(db *gorm.DB, task *model.DownloadTask, parentTaskID *int, relationType string) error {
	if db == nil {
		return fmt.Errorf("数据库不可用")
	}
	if task == nil {
		return fmt.Errorf("下载任务不能为空")
	}

	relationType = strings.TrimSpace(relationType)
	if parentTaskID == nil {
		if relationType != "" {
			return fmt.Errorf("relation_type 需要同时提供 parent_task_id")
		}
		task.ParentTaskID = nil
		task.RootTaskID = 0
		task.RelationType = ""
		return nil
	}
	if *parentTaskID <= 0 {
		return fmt.Errorf("parent_task_id 无效")
	}

	var parent model.DownloadTask
	err := db.Where("id = ? AND deleted_at IS NULL", *parentTaskID).First(&parent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("父下载任务不存在")
	}
	if err != nil {
		return fmt.Errorf("查询父下载任务失败: %w", err)
	}

	parentID := parent.Id
	task.ParentTaskID = &parentID
	task.RootTaskID = parent.RootTaskID
	if task.RootTaskID <= 0 {
		task.RootTaskID = parent.Id
	}
	if relationType == "" {
		relationType = model.TaskRelationDiscovered
	}
	task.RelationType = relationType
	return nil
}

// FinalizeRoot assigns a newly-created root task's own ID as root_task_id.
func FinalizeRoot(db *gorm.DB, task *model.DownloadTask) error {
	if task == nil || task.Id <= 0 {
		return fmt.Errorf("下载任务尚未创建")
	}
	if task.ParentTaskID != nil {
		return nil
	}

	task.RootTaskID = task.Id
	if err := db.Model(task).Update("root_task_id", task.RootTaskID).Error; err != nil {
		return fmt.Errorf("保存根任务关系失败: %w", err)
	}
	return nil
}
