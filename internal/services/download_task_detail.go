package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/dm"
	"wx_channel/pkg/hermes"
)

// BuildTaskDetail is the public task projection shared by REST, SDK and hooks.
// A non-nil snapshot replaces persisted resources with Hermes' final outputs.
func (s *DownloadTaskService) BuildTaskDetail(ctx context.Context, task_id int, snapshot []hermes.TaskFinishedResource) (*dm.DownloadTask, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("数据库不可用")
	}
	scoped_service := *s
	scoped_service.db = s.db.WithContext(ctx)
	record, err := scoped_service.BuildTaskRecord(task_id)
	if err != nil || record == nil {
		return record, err
	}
	if snapshot != nil {
		previous := make(map[int]dm.DownloadTaskFile, len(record.Files))
		for _, file := range record.Files {
			previous[file.ID] = file
		}
		record.Files = make([]dm.DownloadTaskFile, 0, len(snapshot))
		record.Size, record.Downloaded, record.Speed = 0, 0, 0
		for _, resource := range snapshot {
			file := previous[resource.ID]
			file.ID, file.DownloadDir, file.Name = resource.ID, resource.DownloadDir, resource.Name
			file.FilePath, file.Kind, file.Type = resource.FilePath, resource.Kind, strings.ToUpper(resource.Type)
			file.Size, file.Downloaded, file.Speed, file.Progress = resource.Size, resource.Size, 0, 100
			file.Status, file.Error = "finished", ""
			record.Files = append(record.Files, file)
			record.Size += file.Size
			record.Downloaded += file.Downloaded
		}
		record.Status, record.Progress, record.Error = model.TaskStatusFinished, 100, ""
	}
	if record.Files == nil {
		record.Files = []dm.DownloadTaskFile{}
	}
	record.FileCount = len(record.Files)
	if record.ContentID != nil && *record.ContentID != "" {
		var content model.Content
		err := scoped_service.db.Where("id = ?", *record.ContentID).First(&content).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil {
			record.Content = &dm.DownloadTaskContent{
				ID: content.Id, PlatformID: content.PlatformId, Type: content.Type,
				Title: content.Title, Description: content.Description, CoverURL: content.CoverURL,
				URL: content.URL, SourceURL: content.SourceURL, Accounts: []dm.DownloadTaskAccount{},
			}
			if content.PublishTime != nil {
				record.Content.PublishTime = *content.PublishTime
			}
			if err := scoped_service.db.Table("content_account").
				Select("content_account.account_id, account.nickname, account.avatar_url, account.external_id").
				Joins("JOIN account ON account.id = content_account.account_id").
				Where("content_account.content_id = ? AND account.deleted_at IS NULL", *record.ContentID).
				Order("content_account.account_id ASC").
				Scan(&record.Content.Accounts).Error; err != nil {
				return nil, err
			}
		}
	}
	availability := scoped_service.stream_playback_availability(model.DownloadTask{Id: record.ID, ConfigJSON: record.ConfigJSON})
	for index := range record.Files {
		file := &record.Files[index]
		if file.FilePath == "" {
			file.FilePath = filepath.Join(file.DownloadDir, file.Name)
		}
		file.LocalPath = file.FilePath
		_, stat_err := os.Stat(file.FilePath)
		file.Exists = stat_err == nil
		file.FileType = data_file_type_by_ext(file.Name)
		file.FileURL = data_api_file_url(file.FilePath)
		if strings.EqualFold(file.Type, model.ResourceTypeStream) {
			file.PlaybackURL = fmt.Sprintf("/api/v1/download_task/live/%d/%d/index.m3u8", record.ID, file.ID)
			file.PlaybackAvailable = availability[file.ID]
			file.FileType = "video"
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(file.Kind)), "audio/") {
				file.FileType = "audio"
			}
		}
	}
	return record, nil
}

// DownloadTaskLocalFileRoots retains the roots used for file and playback checks.
func (s *DownloadTaskService) DownloadTaskLocalFileRoots(task model.DownloadTask, resource *model.DownloadResource) map[string]string {
	roots := make(map[string]string)
	add_root := func(root, source string) {
		root = strings.TrimSpace(root)
		if root == "" {
			return
		}
		if !filepath.IsAbs(root) && strings.TrimSpace(s.work_dir) != "" {
			root = filepath.Join(s.work_dir, root)
		}
		absolute_root, err := filepath.Abs(root)
		if err != nil {
			s.logger.Warn().Int("task_id", task.Id).Str("path_source", source).Str("root", root).Err(err).Msg("Unable to resolve local file candidate root")
			return
		}
		roots[filepath.Clean(absolute_root)] = source
	}
	add_root(s.download_dir, "download_root")
	var task_config map[string]any
	if strings.TrimSpace(task.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(task.ConfigJSON), &task_config); err != nil {
			s.logger.Warn().Int("task_id", task.Id).Err(err).Msg("Unable to parse task config while resolving local file roots")
		} else {
			download_dir, _ := task_config["download_dir"].(string)
			add_root(download_dir, "task_config_download_dir")
		}
	}
	if resource != nil {
		resource_download_dir := strings.TrimSpace(resource.DownloadDir)
		if resource_download_dir != "" && !filepath.IsAbs(resource_download_dir) {
			resource_download_dir = filepath.Join(s.download_dir, resource_download_dir)
		}
		add_root(resource_download_dir, "resource_download_dir")
	}
	return roots
}

func (s *DownloadTaskService) StreamPlaybackLocation(task model.DownloadTask, resource model.DownloadResource) (string, []string, error) {
	output_path := hermes.ResolveOutputPath(s.download_dir, resource.DownloadDir, resource.UniqueID)
	output_path, err := filepath.Abs(output_path)
	if err != nil {
		return "", nil, err
	}
	output_path = filepath.Clean(output_path)
	roots_by_path := s.DownloadTaskLocalFileRoots(task, &resource)
	safe_roots := make([]string, 0, len(roots_by_path))
	for root := range roots_by_path {
		safe_roots = append(safe_roots, root)
		if resolved_root, resolve_err := filepath.EvalSymlinks(root); resolve_err == nil {
			if resolved_root, absolute_err := filepath.Abs(resolved_root); absolute_err == nil {
				resolved_root = filepath.Clean(resolved_root)
				if resolved_root != root {
					safe_roots = append(safe_roots, resolved_root)
				}
			}
		}
	}
	for _, root := range safe_roots {
		if PathWithinDownloadRoot(root, output_path) {
			return output_path, safe_roots, nil
		}
	}
	return "", nil, errors.New("stream output path is outside its download root")
}

func PathWithinDownloadRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (s *DownloadTaskService) stream_playback_availability(task model.DownloadTask) map[int]bool {
	availability := make(map[int]bool)
	var resources []model.DownloadResource
	if err := s.db.Where("task_id = ? AND deleted_at IS NULL AND UPPER(type) = ?", task.Id, model.ResourceTypeStream).Find(&resources).Error; err != nil {
		return availability
	}
	for _, resource := range resources {
		output_path, _, err := s.StreamPlaybackLocation(task, resource)
		if err != nil {
			continue
		}
		for _, playback_dir := range []string{filepath.Join(hermes.StreamRecordingDir(output_path), "playback"), hermes.StreamPlaybackDir(output_path)} {
			if info, stat_err := os.Lstat(filepath.Join(playback_dir, "index.m3u8")); stat_err == nil && info.Mode().IsRegular() && info.Size() > 0 {
				availability[resource.Id] = true
				break
			}
		}
	}
	return availability
}
