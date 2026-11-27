package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

type DownloadTaskLocalFileCandidate struct {
	Path          string
	PathSource    string
	CandidateType string
}

// DeleteTaskWithFiles stops and soft-deletes one task, optionally removing its files first.
func (s *DownloadTaskService) DeleteTaskWithFiles(task_id int, delete_files bool) error {
	if task_id <= 0 {
		return fmt.Errorf("task_id 无效")
	}
	if s == nil || s.db == nil {
		return fmt.Errorf("应用未初始化，数据库不可用")
	}

	var task model.DownloadTask
	if err := s.db.Unscoped().Where("id = ?", task_id).First(&task).Error; err != nil {
		return fmt.Errorf("下载任务不存在")
	}

	var resources []model.DownloadResource
	if err := s.db.Unscoped().Where("task_id = ?", task.Id).Order("merge_order ASC, id ASC").Find(&resources).Error; err != nil {
		return fmt.Errorf("查询下载任务资源失败: %w", err)
	}

	if err := s.CancelTask(task.Id); err != nil {
		return err
	}
	if delete_files {
		if err := s.delete_download_task_local_files(task, resources); err != nil {
			return fmt.Errorf("删除任务关联的本地文件失败: %w", err)
		}
	}
	if task.DeletedAt != nil {
		s.publish_download_tasks_deleted([]int{task.Id})
		return nil
	}
	if err := s.soft_delete_task_graph([]int{task.Id}, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("删除下载任务失败: %w", err)
	}
	return nil
}

func (s *DownloadTaskService) delete_download_task_local_files(task model.DownloadTask, resources []model.DownloadResource) error {
	protected_paths := make(map[string]bool)
	if s.db != nil {
		var other_resources []model.DownloadResource
		if err := s.db.Table("download_resource AS r").Select("r.*").
			Joins("LEFT JOIN download_task AS t ON t.id = r.task_id").
			Where("r.deleted_at IS NULL AND (r.task_id IS NULL OR (r.task_id <> ? AND t.deleted_at IS NULL))", task.Id).
			Find(&other_resources).Error; err != nil {
			return fmt.Errorf("检查共享文件失败: %w", err)
		}
		var other_tasks []model.DownloadTask
		if err := s.db.Where("deleted_at IS NULL AND id <> ?", task.Id).Find(&other_tasks).Error; err != nil {
			return fmt.Errorf("检查共享任务失败: %w", err)
		}
		tasks_by_id := make(map[int]model.DownloadTask, len(other_tasks))
		for _, other_task := range other_tasks {
			tasks_by_id[other_task.Id] = other_task
		}
		for _, other_resource := range other_resources {
			var other_task model.DownloadTask
			if other_resource.TaskId != nil {
				other_task = tasks_by_id[*other_resource.TaskId]
			}
			for _, candidate := range s.DownloadTaskLocalFileCandidates(other_task, other_resource) {
				protected_paths[candidate.Path] = true
				if resolved_path, err := filepath.EvalSymlinks(candidate.Path); err == nil {
					protected_paths[resolved_path] = true
				}
			}
		}
	}
	deletion_errors := make([]string, 0)
	for _, resource := range resources {
		candidates := s.DownloadTaskLocalFileCandidates(task, resource)
		if len(candidates) == 0 {
			deletion_errors = append(deletion_errors, fmt.Sprintf("资源 %d (%q) 没有可安全删除的本地文件路径", resource.Id, resource.Name))
			continue
		}
		for _, candidate := range candidates {
			if protected_paths[candidate.Path] {
				continue
			}
			if resolved_path, err := filepath.EvalSymlinks(candidate.Path); err == nil && protected_paths[resolved_path] {
				continue
			}
			info, err := os.Lstat(candidate.Path)
			if is_missing_download_task_local_file_error(err) {
				continue
			}
			if err != nil {
				deletion_errors = append(deletion_errors, fmt.Sprintf("检查 %q 失败: %v", candidate.Path, err))
				continue
			}
			is_sidecar_dir := (candidate.CandidateType == "recording" && strings.HasSuffix(candidate.Path, ".recording")) ||
				(candidate.CandidateType == "playback" && strings.HasSuffix(candidate.Path, ".playback"))
			is_sidecar_dir = is_sidecar_dir && info.IsDir()
			if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !is_sidecar_dir) {
				deletion_errors = append(deletion_errors, fmt.Sprintf("拒绝删除非普通文件 %q (mode=%s)", candidate.Path, info.Mode()))
				continue
			}
			remove := os.Remove
			if is_sidecar_dir {
				remove = os.RemoveAll
			}
			if err := remove(candidate.Path); err != nil && !is_missing_download_task_local_file_error(err) {
				deletion_errors = append(deletion_errors, fmt.Sprintf("删除 %q 失败: %v", candidate.Path, err))
			}
		}
	}
	if len(deletion_errors) > 0 {
		return errors.New(strings.Join(deletion_errors, "; "))
	}
	return nil
}

func (s *DownloadTaskService) DownloadTaskLocalFileCandidates(task model.DownloadTask, resource model.DownloadResource) []DownloadTaskLocalFileCandidate {
	var task_config map[string]any
	_ = json.Unmarshal([]byte(task.ConfigJSON), &task_config)
	duplicate, _ := task_config["duplicate"].(bool)
	download_name := hermes.ResourceDownloadName(task.Id, resource.Id, resource.UniqueID, duplicate)
	var extra map[string]string
	_ = json.Unmarshal([]byte(resource.Extra), &extra)
	if extra["_hermes_download_name"] != "" {
		download_name = extra["_hermes_download_name"]
	}
	names := []string{strings.TrimSpace(download_name)}
	if task.Status == model.TaskStatusFinished || extra["_hermes_finalized"] == "1" {
		names = append([]string{strings.TrimSpace(resource.Name)}, names...)
	}
	if pending_name := extra["_hermes_postprocessed"]; pending_name != "" {
		names = append(names, pending_name)
	}
	roots := s.download_task_local_file_roots(task, resource)
	seen := make(map[string]struct{})
	candidates := make([]DownloadTaskLocalFileCandidate, 0, len(roots)*6)
	for root := range roots {
		for _, name := range names {
			if name == "" {
				continue
			}
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			path = filepath.Clean(path)
			if !download_task_path_within_root(root, path) {
				continue
			}
			resource_candidates := []DownloadTaskLocalFileCandidate{
				{Path: path, PathSource: "resource_path", CandidateType: "final"},
				{Path: path + ".part", PathSource: "resource_path", CandidateType: "partial"},
			}
			if strings.EqualFold(resource.Type, model.ResourceTypeStream) {
				resource_candidates = append(resource_candidates,
					DownloadTaskLocalFileCandidate{Path: hermes.StreamRecordingDir(path), CandidateType: "recording"},
					DownloadTaskLocalFileCandidate{Path: hermes.StreamPlaybackDir(path), CandidateType: "playback"},
				)
			}
			for _, candidate := range resource_candidates {
				if _, exists := seen[candidate.Path]; exists {
					continue
				}
				seen[candidate.Path] = struct{}{}
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates
}

func (s *DownloadTaskService) download_task_local_file_roots(task model.DownloadTask, resource model.DownloadResource) map[string]struct{} {
	roots := make(map[string]struct{})
	add_root := func(root string) {
		root = strings.TrimSpace(root)
		if root == "" {
			return
		}
		if !filepath.IsAbs(root) && strings.TrimSpace(s.work_dir) != "" {
			root = filepath.Join(s.work_dir, root)
		}
		absolute_root, err := filepath.Abs(root)
		if err == nil {
			roots[filepath.Clean(absolute_root)] = struct{}{}
		}
	}
	if strings.TrimSpace(resource.DownloadDir) != "" {
		add_root(hermes.ResolveOutputPath(s.download_dir, resource.DownloadDir, ""))
		return roots
	}
	if strings.TrimSpace(task.ConfigJSON) != "" {
		var task_config map[string]any
		if json.Unmarshal([]byte(task.ConfigJSON), &task_config) == nil {
			download_dir, _ := task_config["download_dir"].(string)
			add_root(download_dir)
		}
	}
	if len(roots) == 0 {
		add_root(s.download_dir)
	}
	return roots
}

func download_task_path_within_root(root string, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." {
		return false
	}
	if strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	resolved_root, root_err := filepath.EvalSymlinks(root)
	resolved_parent, parent_err := filepath.EvalSymlinks(filepath.Dir(target))
	if root_err == nil && parent_err == nil {
		relative, err = filepath.Rel(resolved_root, resolved_parent)
		return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
	}
	return (root_err == nil || os.IsNotExist(root_err)) && (parent_err == nil || os.IsNotExist(parent_err))
}
