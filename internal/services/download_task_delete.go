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

type download_task_local_file_candidate struct {
	path           string
	candidate_type string
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
		return nil
	}
	if err := s.soft_delete_task_graph([]int{task.Id}, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("删除下载任务失败: %w", err)
	}
	return nil
}

func (s *DownloadTaskService) delete_download_task_local_files(task model.DownloadTask, resources []model.DownloadResource) error {
	deletion_errors := make([]string, 0)
	for _, resource := range resources {
		candidates := s.download_task_local_file_candidates(task, resource)
		if len(candidates) == 0 {
			deletion_errors = append(deletion_errors, fmt.Sprintf("资源 %d (%q) 没有可安全删除的本地文件路径", resource.Id, resource.Name))
			continue
		}
		for _, candidate := range candidates {
			info, err := os.Lstat(candidate.path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				deletion_errors = append(deletion_errors, fmt.Sprintf("检查 %q 失败: %v", candidate.path, err))
				continue
			}
			is_sidecar_dir := (candidate.candidate_type == "recording" && strings.HasSuffix(candidate.path, ".recording")) ||
				(candidate.candidate_type == "playback" && strings.HasSuffix(candidate.path, ".playback"))
			is_sidecar_dir = is_sidecar_dir && info.IsDir()
			if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !is_sidecar_dir) {
				deletion_errors = append(deletion_errors, fmt.Sprintf("拒绝删除非普通文件 %q (mode=%s)", candidate.path, info.Mode()))
				continue
			}
			remove := os.Remove
			if is_sidecar_dir {
				remove = os.RemoveAll
			}
			if err := remove(candidate.path); err != nil && !os.IsNotExist(err) {
				deletion_errors = append(deletion_errors, fmt.Sprintf("删除 %q 失败: %v", candidate.path, err))
			}
		}
	}
	if len(deletion_errors) > 0 {
		return errors.New(strings.Join(deletion_errors, "; "))
	}
	return nil
}

func (s *DownloadTaskService) download_task_local_file_candidates(task model.DownloadTask, resource model.DownloadResource) []download_task_local_file_candidate {
	names := []string{strings.TrimSpace(resource.Name), strings.TrimSpace(resource.UniqueID)}
	roots := s.download_task_local_file_roots(task, resource)
	seen := make(map[string]struct{})
	candidates := make([]download_task_local_file_candidate, 0, len(roots)*6)
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
			resource_candidates := []download_task_local_file_candidate{
				{path: path, candidate_type: "final"},
				{path: path + ".part", candidate_type: "partial"},
			}
			if strings.EqualFold(resource.Type, model.ResourceTypeStream) {
				resource_candidates = append(resource_candidates,
					download_task_local_file_candidate{path: hermes.StreamRecordingDir(path), candidate_type: "recording"},
					download_task_local_file_candidate{path: hermes.StreamPlaybackDir(path), candidate_type: "playback"},
				)
			}
			for _, candidate := range resource_candidates {
				if _, exists := seen[candidate.path]; exists {
					continue
				}
				seen[candidate.path] = struct{}{}
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
	add_root(s.download_dir)

	if strings.TrimSpace(task.ConfigJSON) != "" {
		var task_config map[string]any
		if json.Unmarshal([]byte(task.ConfigJSON), &task_config) == nil {
			download_dir, _ := task_config["download_dir"].(string)
			add_root(download_dir)
		}
	}
	resource_download_dir := strings.TrimSpace(resource.DownloadDir)
	if resource_download_dir != "" && !filepath.IsAbs(resource_download_dir) {
		resource_download_dir = filepath.Join(s.download_dir, resource_download_dir)
	}
	add_root(resource_download_dir)
	return roots
}

func download_task_path_within_root(root string, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
