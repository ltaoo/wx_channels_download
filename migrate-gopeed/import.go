package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	wxchannelsadapter "wx_channel/internal/adapter/wxchannels"
	wxmpadapter "wx_channel/internal/adapter/wxmp"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

func import_target_download_record(db *gorm.DB, task *downloadpkg.Task, labels map[string]string, oid, uid, save_path string, object_raw json.RawMessage) (int, error) {
	config_json := build_migration_task_config_json(filepath.Dir(save_path), filepath.Base(save_path), labels)
	info, err := wxchannelsadapter.NewChannelsAdapter().BuildDownloadTask(normalize_raw_json_object(object_raw), json.RawMessage(config_json))
	if err != nil {
		return 0, err
	}
	if info.Content != nil && info.Content.ExternalId2 == "" {
		info.Content.ExternalId2 = uid
	}
	return recreate_gopeed_task(db, task, labels, platform_wx_channels, oid, info.Task.SourceURL, save_path, info)
}

func import_target_wxmp_download_record(db *gorm.DB, task *downloadpkg.Task, labels map[string]string, article_id, article_url, save_path string, content_raw json.RawMessage) (int, error) {
	config_json := build_wxmp_migration_task_config_json(filepath.Dir(save_path), filepath.Base(save_path), labels)
	info, err := wxmpadapter.NewOfficialAccountAdapter().BuildDownloadTask(normalize_raw_json_object(content_raw), json.RawMessage(config_json))
	if err != nil {
		return 0, err
	}
	return recreate_gopeed_task(db, task, labels, platform_wxmp, article_id, article_url, save_path, info)
}

func import_target_download_task_without_content(db *gorm.DB, task *downloadpkg.Task, labels map[string]string, platform_id, external_id, source_url, save_path string) (int, error) {
	return recreate_gopeed_task(db, task, labels, platform_id, external_id, source_url, save_path, &adapter.DownloadTaskResult{Task: &model.DownloadTask{}})
}

// Gopeed owns the downloaded file; adapters supply the current content graph.
// Persistence is deliberately the same offline recreation used by sync imports.
func recreate_gopeed_task(db *gorm.DB, task *downloadpkg.Task, labels map[string]string, platform_id, external_id, source_url, save_path string, info *adapter.DownloadTaskResult) (int, error) {
	if db == nil || task == nil || safe_task_id(task) == "" {
		return 0, fmt.Errorf("target database and Gopeed task ID are required")
	}
	if save_path == "" {
		return 0, fmt.Errorf("missing downloaded file path")
	}
	now := time.Now().UnixMilli()
	created_at := time_to_millis_or_default(task_created_at(task), now)
	updated_at := time_to_millis_or_default(task_updated_at(task), created_at)
	if updated_at < created_at {
		updated_at = created_at
	}
	timestamps := model.Timestamps{CreatedAt: created_at, UpdatedAt: updated_at}
	download_dir, filename := filepath.Dir(save_path), filepath.Base(save_path)
	task_record := *info.Task
	task_record.PlatformId = platform_id
	task_record.Name = first_non_empty(task_name(task), task_record.Name, filename, external_id)
	task_record.SourceURL = first_non_empty(task_record.SourceURL, task_url(task), source_url)
	if platform_id == platform_wxmp {
		task_record.SourceURL = first_non_empty(info.Task.SourceURL, source_url, task_url(task))
	}
	task_record.Status = model.TaskStatusPaused
	task_record.Timestamps = timestamps
	if task_record.UniqueID == "" {
		task_record.UniqueID = migrated_task_unique_id(first_non_empty(external_id, safe_task_id(task)), labels, save_path)
		if platform_id == platform_wxmp {
			task_record.UniqueID = migrated_wxmp_task_unique_id(first_non_empty(external_id, safe_task_id(task)), save_path)
		}
	}
	if task_record.ConfigJSON == "" {
		task_record.ConfigJSON = build_migration_task_config_json(download_dir, filename, labels)
	}
	metadata := map[string]any{"gopeedid": safe_task_id(task), "migrated": true}
	if info.Content == nil {
		metadata["detail_missing"] = true
	}
	if platform_id == platform_wxmp {
		metadata["article_id"], metadata["article_url"] = external_id, source_url
	}
	task_record.MetadataJSON = merge_migration_metadata_json(task_record.MetadataJSON, metadata)

	// shortcut: one Gopeed task restores one existing file, expand when importing multi-file Gopeed tasks.
	resource_info := &adapter.ResourceInfo{}
	for _, candidate := range info.Resources {
		if candidate.Resource.Kind == resource_kind_for_path(save_path) && candidate.Resource.ContentId != nil && task_record.ContentId != nil && *candidate.Resource.ContentId == *task_record.ContentId {
			copy_info := *candidate
			resource_info = &copy_info
			break
		}
	}
	resource := &resource_info.Resource
	resource.ContentId = task_record.ContentId
	resource.DownloadDir, resource.Name = download_dir, filename
	resource.Type, resource.Kind = model.ResourceTypeFile, resource_kind_for_path(save_path)
	resource.UniqueID = first_non_empty(resource.UniqueID, task_record.UniqueID)
	resource.Timestamps = timestamps
	resource.Size = migrated_size(task, save_path)
	resource.Downloaded = task_downloaded(task)
	resource.Status = model.ResourceStatusWaiting
	file_info, stat_err := os.Stat(save_path)
	if stat_err != nil && !errors.Is(stat_err, os.ErrNotExist) {
		return 0, fmt.Errorf("read downloaded file: %w", stat_err)
	}
	if stat_err == nil && !file_info.Mode().IsRegular() {
		return 0, fmt.Errorf("downloaded file path is not a regular file")
	}
	if errors.Is(stat_err, os.ErrNotExist) {
		resource.Downloaded = 0
	}
	if stat_err == nil && task_status(task) == base.DownloadStatusDone {
		task_record.Status, resource.Status = model.TaskStatusFinished, model.ResourceStatusFinished
		resource.Size, resource.Downloaded = file_info.Size(), file_info.Size()
		resource.StartTime, resource.FinishTime = &created_at, &updated_at
	}
	resource.Extra = merge_migration_metadata_json(resource.Extra, map[string]any{"gopeedid": safe_task_id(task), "file_path": save_path, "migrated": true})
	endpoint_url := task_url(task)
	if platform_id == platform_wxmp || (endpoint_url == "" && len(resource_info.Endpoints) == 0) {
		endpoint_url = first_non_empty(source_url, endpoint_url)
	}
	if endpoint_url != "" || len(resource_info.Endpoints) == 0 {
		resource_info.Endpoints = []model.DownloadEndpoint{{Protocol: endpoint_protocol(endpoint_url, task_protocol(task)), URL: endpoint_url, Enabled: 1, Status: resource.Status, Timestamps: timestamps}}
	}
	archive := services.DownloadTaskArchive{Task: task_record, Resources: []*adapter.ResourceInfo{resource_info}}
	contents := make(map[string]*services.TaskContentArchive)
	if info.Content != nil {
		content := *info.Content
		content.Timestamps = timestamps
		saved := &services.TaskContentArchive{Content: content, Details: make(map[string]json.RawMessage)}
		if info.Account != nil {
			account := *info.Account
			account.Timestamps = timestamps
			role := "owner"
			if platform_id == platform_wxmp {
				role = "publisher"
			}
			saved.Accounts = []adapter.ContentAccountReference{{Account: &account, Role: role}}
		}
		contents[content.Id] = saved
	}
	for _, detail := range info.ContentDetails {
		if detail.Content != nil && contents[detail.Content.Id] == nil {
			content := *detail.Content
			content.Timestamps = timestamps
			contents[content.Id] = &services.TaskContentArchive{Content: content, Details: make(map[string]json.RawMessage)}
		}
		saved := contents[detail.Key]
		if saved == nil {
			return 0, fmt.Errorf("content detail references unknown content %q", detail.Key)
		}
		if detail.Data != nil {
			raw, err := json.Marshal(detail.Data)
			if err != nil {
				return 0, err
			}
			saved.Details[detail.Type] = raw
		}
		saved.Accounts = append(saved.Accounts, detail.Accounts...)
		saved.Influencers = append(saved.Influencers, detail.Influencers...)
		saved.Relations = append(saved.Relations, detail.Relations...)
		if detail.Relation != nil {
			saved.Relations = append(saved.Relations, *detail.Relation)
		}
	}
	for _, content := range contents {
		archive.Contents = append(archive.Contents, *content)
	}
	var target_id int
	err := db.Transaction(func(tx *gorm.DB) error {
		if existing, err := find_migrated_download_task_by_platform(tx, platform_id, safe_task_id(task)); err == nil {
			target_id = existing.Id
			return duplicate_import_error{TaskID: target_id}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		created, err := services.RecreateArchivedTask(tx, archive, download_dir, nil)
		if err != nil {
			return err
		}
		target_id = created.Task.Id
		return nil
	})
	return target_id, err
}
