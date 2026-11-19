package api

import (
	"fmt"
	"path/filepath"

	"gorm.io/gorm"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

func build_sync_task_records(db *gorm.DB, bundle *sync_bundle, task_ids ...int) error {
	query := db
	selected := make(map[int]bool, len(task_ids))
	for _, id := range task_ids {
		if id <= 0 {
			return fmt.Errorf("task_ids 必须是正整数")
		}
		selected[id] = true
	}
	if len(selected) > 0 {
		query = query.Where("id IN ?", task_ids)
	}
	if err := query.Order("id").Find(&bundle.Tasks).Error; err != nil {
		return err
	}
	if len(selected) > 0 && len(bundle.Tasks) != len(selected) {
		return fmt.Errorf("指定的下载任务不存在，未执行同步")
	}
	seen_content := make(map[string]bool)
	for task_index := range bundle.Tasks {
		task := bundle.Tasks[task_index]
		if len(selected) > 0 && task.ParentTaskID != nil && !selected[*task.ParentTaskID] {
			// A selection never expands to additional tasks implicitly.
			task.ParentTaskID, task.RelationType, task.RootTaskID = nil, "", task.Id
			bundle.Tasks[task_index] = task
		}
		record, err := services.ExportDownloadTaskArchive(db, task)
		if err != nil {
			return fmt.Errorf("归档下载任务 %d: %w", task.Id, err)
		}
		bundle.Records = append(bundle.Records, record)
		for _, resource := range record.Resources {
			bundle.Resources = append(bundle.Resources, resource.Resource)
		}
		for _, content := range record.Contents {
			if !seen_content[content.Content.Id] {
				seen_content[content.Content.Id] = true
				bundle.Contents = append(bundle.Contents, content.Content)
			}
		}
	}
	return nil
}

func sync_record_counts(bundle sync_bundle) map[string]int {
	accounts := make(map[string]bool)
	for _, task := range bundle.Records {
		for _, content := range task.Contents {
			for _, reference := range content.Accounts {
				if reference.Account != nil {
					accounts[reference.Account.PlatformId+"\x00"+reference.Account.ExternalId] = true
				}
			}
		}
	}
	return map[string]int{"account": len(accounts), "content": len(bundle.Contents), "download_task": len(bundle.Tasks), "download_resource": len(bundle.Resources)}
}

func (c *APIClient) recreate_sync_tasks(tx *gorm.DB, bundle sync_bundle, result *sync_import_result) error {
	if len(bundle.Records) != len(bundle.Tasks) {
		return fmt.Errorf("同步包缺少下载任务归档")
	}
	resources := make(map[int]model.DownloadResource, len(bundle.Resources))
	for _, resource := range bundle.Resources {
		resources[resource.Id] = resource
	}
	task_ids := make(map[int]int)
	pending := append([]*services.DownloadTaskArchive{}, bundle.Records...)
	for len(pending) > 0 {
		next := []*services.DownloadTaskArchive{}
		for _, record := range pending {
			if record == nil || record.Task.Id <= 0 {
				return fmt.Errorf("归档任务 ID 无效")
			}
			if _, exists := task_ids[record.Task.Id]; exists {
				return fmt.Errorf("归档任务 ID 重复")
			}
			var parent_id *int
			if record.Task.ParentTaskID != nil {
				mapped_id, ok := task_ids[*record.Task.ParentTaskID]
				if !ok {
					next = append(next, record)
					continue
				}
				parent_id = &mapped_id
			}
			// Keep source IDs solely for matching transmitted files to inputs.
			// The shared creation pipeline allocates every target numeric ID.
			prepared := *record
			prepared.Resources = nil
			for _, info := range record.Resources {
				if info == nil {
					return fmt.Errorf("归档资源为空")
				}
				resource, ok := resources[info.Resource.Id]
				if !ok || resource.TaskId == nil || *resource.TaskId != record.Task.Id {
					return fmt.Errorf("归档资源与任务不匹配")
				}
				target_path, err := sync_resource_path(c.cfg.DownloadDir, resource.Name)
				if err != nil {
					return err
				}
				resource.DownloadDir, resource.Name = filepath.Dir(target_path), filepath.Base(target_path)
				copy_info := *info
				copy_info.Resource = resource
				prepared.Resources = append(prepared.Resources, &copy_info)
			}
			created, err := services.RecreateArchivedTask(tx, prepared, c.cfg.DownloadDir, parent_id)
			if err != nil {
				return fmt.Errorf("重建下载任务 %d: %w", record.Task.Id, err)
			}
			task_ids[record.Task.Id] = created.Task.Id
			result.TargetTaskIDs = append(result.TargetTaskIDs, created.Task.Id)
			result.task_ids = append(result.task_ids, created.Task.Id)
		}
		if len(next) == len(pending) {
			return fmt.Errorf("下载任务父子关系包含循环或缺失的父任务")
		}
		pending = next
	}
	result.Tables = sync_record_counts(bundle)
	result.Accounts = result.Tables["account"]
	return nil
}
