package api

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
	"wx_channel/pkg/hermes"
)

// ponytail: metadata stays in memory; switch to a streamed manifest only if 256 MB becomes a real limit.
const sync_metadata_limit = 256 << 20

// ponytail: one global lock keeps file replacement atomic; use per-download-root locks if concurrent imports are needed.
var sync_import_mu sync.Mutex

type sync_server_config struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type sync_server_record struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type sync_bundle struct {
	Version            int                             `json:"version"`
	SourceID           string                          `json:"source_id,omitempty"`
	Contents           []model.Content                 `json:"contents"`
	Tasks              []model.DownloadTask            `json:"tasks"`
	Resources          []model.DownloadResource        `json:"resources"`
	Records            []*services.DownloadTaskArchive `json:"records,omitempty"`
	TargetParentTaskID *int                            `json:"target_parent_task_id,omitempty"`
}

type sync_source_file struct {
	TaskID      int
	Path        string
	ArchivePath string
	Size        int64
}

type sync_file_stage struct {
	TempPath   string
	FinalPath  string
	BackupPath string
	Installed  bool
}

type sync_import_result struct {
	Contents      int            `json:"contents"`
	Tasks         int            `json:"tasks"`
	Resources     int            `json:"resources"`
	Files         int            `json:"files"`
	Bytes         int64          `json:"bytes"`
	Accounts      int            `json:"accounts"`
	TargetTaskIDs []int          `json:"target_task_ids,omitempty"`
	Tables        map[string]int `json:"tables,omitempty"`
	task_ids      []int
}

func (c *APIClient) sync_servers() ([]sync_server_config, error) {
	if c.cfg == nil || c.cfg.Original == nil {
		return nil, fmt.Errorf("配置未初始化")
	}
	raw_servers := c.cfg.Original.GetRaw("sync.servers")
	if raw_servers == nil {
		return []sync_server_config{}, nil
	}
	encoded_servers, err := json.Marshal(raw_servers)
	if err != nil {
		return nil, fmt.Errorf("读取同步服务配置失败: %w", err)
	}
	var servers []sync_server_config
	if err := json.Unmarshal(encoded_servers, &servers); err != nil {
		return nil, fmt.Errorf("解析同步服务配置失败: %w", err)
	}
	for server_index := range servers {
		server := &servers[server_index]
		server.Name = strings.TrimSpace(server.Name)
		server.URL = strings.TrimRight(strings.TrimSpace(server.URL), "/")
		if server.Name == "" {
			return nil, fmt.Errorf("第 %d 个同步服务缺少 name", server_index+1)
		}
		parsed_url, parse_err := url.Parse(server.URL)
		if parse_err != nil || parsed_url.Host == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") || parsed_url.RawQuery != "" || parsed_url.Fragment != "" {
			return nil, fmt.Errorf("同步服务 %q 的 url 无效", server.Name)
		}
	}
	return servers, nil
}

func (c *APIClient) handle_sync_servers(ctx *gin.Context) {
	servers, err := c.sync_servers()
	if err != nil {
		result.Err(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	records := make([]sync_server_record, 0, len(servers))
	for server_index, server := range servers {
		records = append(records, sync_server_record{ID: server_index, Name: server.Name, URL: sync_server_public_url(server.URL)})
	}
	result.Ok(ctx, gin.H{"servers": records})
}

func (c *APIClient) handle_sync_to_server(ctx *gin.Context) {
	server_id, err := strconv.Atoi(ctx.Param("server_id"))
	if err != nil || server_id < 0 {
		result.Err(ctx, http.StatusBadRequest, "同步服务 ID 无效")
		return
	}
	servers, err := c.sync_servers()
	if err != nil {
		result.Err(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	if server_id >= len(servers) {
		result.Err(ctx, http.StatusNotFound, "同步服务不存在")
		return
	}
	task_ids, err := requested_sync_task_ids(ctx)
	if err != nil {
		result.Err(ctx, http.StatusBadRequest, err.Error())
		return
	}
	sync_result, err := c.send_sync_bundle(ctx.Request, servers[server_id], nil, task_ids...)
	if err != nil {
		result.ErrWithData(ctx, http.StatusBadGateway, err.Error(), sync_result)
		return
	}
	result.Ok(ctx, sync_result)
}

func (c *APIClient) send_sync_bundle(source_request *http.Request, server sync_server_config, progress func(int, int, *sync_import_result), task_ids ...int) (*sync_import_result, error) {
	if err := check_sync_receiver(source_request.Context(), server); err != nil {
		return nil, err
	}
	bundle, source_files, err := c.build_sync_bundle(task_ids...)
	if err != nil {
		return nil, err
	}
	files_by_task := make(map[int][]sync_source_file)
	for _, file := range source_files {
		files_by_task[file.TaskID] = append(files_by_task[file.TaskID], file)
	}
	imported_ids := make(map[int]int)
	confirmed := sync_bundle{}
	confirmed_contents := make(map[string]bool)
	imported := &sync_import_result{}
	pending := bundle.Records
	for len(pending) > 0 {
		next := []*services.DownloadTaskArchive{}
		for _, record := range pending {
			task := record.Task
			var parent_id *int
			if task.ParentTaskID != nil {
				mapped_id, ok := imported_ids[*task.ParentTaskID]
				if !ok {
					next = append(next, record)
					continue
				}
				parent_id = &mapped_id
			}
			if progress != nil {
				progress(len(bundle.Tasks), task.Id, imported)
			}
			single := sync_bundle{Version: 3, SourceID: bundle.SourceID, Tasks: []model.DownloadTask{task}, Records: []*services.DownloadTaskArchive{record}, TargetParentTaskID: parent_id}
			for _, resource := range record.Resources {
				single.Resources = append(single.Resources, resource.Resource)
			}
			for _, content := range record.Contents {
				single.Contents = append(single.Contents, content.Content)
			}
			completed, err := send_sync_archive(source_request, server, single, files_by_task[task.Id])
			if err != nil {
				return imported, fmt.Errorf("同步任务 %d 失败（已完成 %d/%d）: %w", task.Id, imported.Tasks, len(bundle.Tasks), err)
			}
			if completed.Tasks != 1 || len(completed.TargetTaskIDs) != 1 || completed.TargetTaskIDs[0] <= 0 {
				return imported, fmt.Errorf("同步任务 %d 返回无效结果，请核对目标记录后重试", task.Id)
			}
			imported_ids[task.Id] = completed.TargetTaskIDs[0]
			confirmed.Tasks = append(confirmed.Tasks, task)
			confirmed.Records = append(confirmed.Records, record)
			confirmed.Resources = append(confirmed.Resources, single.Resources...)
			for _, content := range single.Contents {
				if !confirmed_contents[content.Id] {
					confirmed.Contents = append(confirmed.Contents, content)
					confirmed_contents[content.Id] = true
				}
			}
			counts := sync_record_counts(confirmed)
			imported = &sync_import_result{
				Tasks: len(confirmed.Tasks), Resources: len(confirmed.Resources), Contents: len(confirmed.Contents),
				Accounts: counts["account"], Tables: counts,
				Files: imported.Files + completed.Files, Bytes: imported.Bytes + completed.Bytes,
				TargetTaskIDs: append(append([]int(nil), imported.TargetTaskIDs...), completed.TargetTaskIDs...),
			}
			if progress != nil {
				progress(len(bundle.Tasks), 0, imported)
			}
		}
		if len(next) == len(pending) {
			return imported, fmt.Errorf("下载任务父子关系包含循环或缺失的父任务")
		}
		pending = next
	}
	return imported, nil
}

func send_sync_archive(source_request *http.Request, server sync_server_config, bundle sync_bundle, source_files []sync_source_file) (*sync_import_result, error) {
	pipe_reader, pipe_writer := io.Pipe()
	defer pipe_reader.Close()
	go func() {
		archive_writer := tar.NewWriter(pipe_writer)
		write_err := write_sync_archive(archive_writer, bundle, source_files)
		if close_err := archive_writer.Close(); write_err == nil {
			write_err = close_err
		}
		_ = pipe_writer.CloseWithError(write_err)
	}()

	target_url := server.URL + "/api/v1/sync/import"
	request, err := http.NewRequestWithContext(source_request.Context(), http.MethodPost, target_url, pipe_reader)
	if err != nil {
		_ = pipe_reader.Close()
		return nil, fmt.Errorf("创建同步请求失败: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-tar")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("连接同步服务 %q 失败: %w", server.Name, err)
	}
	defer response.Body.Close()
	var envelope struct {
		Code int                `json:"code"`
		Msg  string             `json:"msg"`
		Data sync_import_result `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("同步服务 %q 返回无效响应: %w", server.Name, err)
	}
	if response.StatusCode != http.StatusOK || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Msg)
		if message == "" {
			message = response.Status
		}
		return nil, fmt.Errorf("同步服务 %q 拒绝同步: %s", server.Name, message)
	}
	return &envelope.Data, nil
}

func (c *APIClient) build_sync_bundle(task_ids ...int) (sync_bundle, []sync_source_file, error) {
	bundle := sync_bundle{Version: 2}
	hostname, _ := os.Hostname()
	source_hash := sha256.Sum256([]byte(hostname + "\x00" + c.cfg.DBPath + "\x00" + c.cfg.WorkDir))
	bundle.SourceID = fmt.Sprintf("%x", source_hash[:16])
	if err := c.db.Transaction(func(tx *gorm.DB) error {
		return build_sync_task_records(tx, &bundle, task_ids...)
	}); err != nil {
		return bundle, nil, fmt.Errorf("读取下载任务归档失败: %w", err)
	}
	tasks_by_id := make(map[int]model.DownloadTask, len(bundle.Tasks))
	for _, task := range bundle.Tasks {
		tasks_by_id[task.Id] = task
	}
	var source_files []sync_source_file
	for _, resource := range bundle.Resources {
		task := model.DownloadTask{}
		if resource.TaskId != nil {
			task = tasks_by_id[*resource.TaskId]
		}
		files, err := c.sync_resource_files(task, resource)
		if err != nil {
			return bundle, nil, err
		}
		for index := range files {
			files[index].TaskID = task.Id
		}
		source_files = append(source_files, files...)
	}
	return bundle, source_files, nil
}

func (c *APIClient) sync_resource_files(task model.DownloadTask, resource model.DownloadResource) ([]sync_source_file, error) {
	seen_types := make(map[string]bool)
	var files []sync_source_file
	for _, candidate := range c.download_task_local_file_candidates(task, resource) {
		if candidate.CandidateType == "partial" || seen_types[candidate.CandidateType] {
			continue
		}
		file_info, err := os.Stat(candidate.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("读取资源文件 %q 失败: %w", candidate.Path, err)
		}
		archive_prefix := path.Join("files", strconv.Itoa(resource.Id), candidate.CandidateType)
		if file_info.Mode().IsRegular() {
			files = append(files, sync_source_file{Path: candidate.Path, ArchivePath: path.Join(archive_prefix, "file"), Size: file_info.Size()})
			seen_types[candidate.CandidateType] = true
			continue
		}
		if !file_info.IsDir() {
			continue
		}
		walk_err := filepath.Walk(candidate.Path, func(file_path string, info os.FileInfo, walk_err error) error {
			if walk_err != nil {
				return walk_err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			relative_path, rel_err := filepath.Rel(candidate.Path, file_path)
			if rel_err != nil {
				return rel_err
			}
			files = append(files, sync_source_file{Path: file_path, ArchivePath: path.Join(archive_prefix, filepath.ToSlash(relative_path)), Size: info.Size()})
			return nil
		})
		if walk_err != nil {
			return nil, fmt.Errorf("读取资源目录 %q 失败: %w", candidate.Path, walk_err)
		}
		seen_types[candidate.CandidateType] = true
	}
	return files, nil
}

func write_sync_archive(archive_writer *tar.Writer, bundle sync_bundle, source_files []sync_source_file) error {
	metadata, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("编码同步记录失败: %w", err)
	}
	if err := archive_writer.WriteHeader(&tar.Header{Name: "metadata.json", Mode: 0600, Size: int64(len(metadata))}); err != nil {
		return err
	}
	if _, err := archive_writer.Write(metadata); err != nil {
		return err
	}
	for _, source_file := range source_files {
		file, err := os.Open(source_file.Path)
		if err != nil {
			return fmt.Errorf("打开同步文件 %q 失败: %w", source_file.Path, err)
		}
		if err := archive_writer.WriteHeader(&tar.Header{Name: source_file.ArchivePath, Mode: 0600, Size: source_file.Size}); err != nil {
			_ = file.Close()
			return err
		}
		_, copy_err := io.Copy(archive_writer, file)
		close_err := file.Close()
		if copy_err != nil {
			return copy_err
		}
		if close_err != nil {
			return close_err
		}
	}
	return nil
}

func (c *APIClient) handle_sync_import(ctx *gin.Context) {
	if !strings.HasPrefix(strings.ToLower(ctx.GetHeader("Content-Type")), "application/x-tar") {
		result.Err(ctx, http.StatusUnsupportedMediaType, "同步数据格式无效")
		return
	}
	import_result, err := c.import_sync_archive(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, http.StatusBadRequest, err.Error())
		return
	}
	if c.download_task_broadcaster != nil && len(import_result.task_ids) > 0 {
		c.download_task_broadcaster.broadcast_download_task_upsert(import_result.task_ids)
		c.download_task_broadcaster.broadcast_download_task_stats()
	}
	result.Ok(ctx, import_result)
}

func (c *APIClient) import_sync_archive(reader io.Reader) (*sync_import_result, error) {
	archive_reader := tar.NewReader(reader)
	metadata_header, err := archive_reader.Next()
	if err != nil || metadata_header.Name != "metadata.json" || !metadata_header.FileInfo().Mode().IsRegular() {
		return nil, fmt.Errorf("同步包缺少 metadata.json")
	}
	if metadata_header.Size > sync_metadata_limit {
		return nil, fmt.Errorf("同步记录超过 %d MB 限制", sync_metadata_limit>>20)
	}
	metadata, err := io.ReadAll(io.LimitReader(archive_reader, sync_metadata_limit+1))
	if err != nil || int64(len(metadata)) > sync_metadata_limit {
		return nil, fmt.Errorf("读取同步记录失败")
	}
	var bundle sync_bundle
	if err := json.Unmarshal(metadata, &bundle); err != nil || (bundle.Version != 1 && bundle.Version != 2 && bundle.Version != 3) {
		return nil, fmt.Errorf("同步记录版本无效")
	}
	if bundle.Version == 3 {
		if len(bundle.Tasks) != 1 || len(bundle.Records) != 1 || bundle.Records[0] == nil || bundle.Records[0].Task.Id != bundle.Tasks[0].Id {
			return nil, fmt.Errorf("逐条同步包必须包含一个下载任务及其归档")
		}
		if (bundle.Records[0].Task.ParentTaskID == nil) != (bundle.TargetParentTaskID == nil) || (bundle.TargetParentTaskID != nil && *bundle.TargetParentTaskID <= 0) {
			return nil, fmt.Errorf("逐条同步父任务映射无效")
		}
	}
	resources_by_id := make(map[int]model.DownloadResource, len(bundle.Resources))
	tasks_by_id := make(map[int]model.DownloadTask, len(bundle.Tasks))
	for _, task := range bundle.Tasks {
		tasks_by_id[task.Id] = task
	}
	for resource_index := range bundle.Resources {
		resource := bundle.Resources[resource_index]
		if resource.Id <= 0 {
			return nil, fmt.Errorf("同步资源 ID 无效")
		}
		if _, exists := resources_by_id[resource.Id]; exists {
			return nil, fmt.Errorf("同步资源 ID %d 重复", resource.Id)
		}
		if strings.EqualFold(resource.Type, model.ResourceTypeStream) {
			var extra map[string]string
			if err := json.Unmarshal([]byte(resource.Extra), &extra); err != nil && strings.TrimSpace(resource.Extra) != "" {
				return nil, fmt.Errorf("直播资源附加数据无效: %w", err)
			}
			if extra == nil {
				extra = make(map[string]string)
			}
			if extra["_hermes_download_name"] == "" {
				var source_task model.DownloadTask
				if resource.TaskId != nil {
					source_task = tasks_by_id[*resource.TaskId]
				}
				var task_config map[string]any
				_ = json.Unmarshal([]byte(source_task.ConfigJSON), &task_config)
				duplicate, _ := task_config["duplicate"].(bool)
				extra["_hermes_download_name"] = hermes.ResourceDownloadName(source_task.Id, resource.Id, resource.UniqueID, duplicate)
				encoded_extra, err := json.Marshal(extra)
				if err != nil {
					return nil, err
				}
				resource.Extra = string(encoded_extra)
			}
		}
		if bundle.Version >= 2 {
			if _, err := sync_resource_path(c.cfg.DownloadDir, resource.Name); err != nil {
				return nil, err
			}
			// A stable per-source/resource directory prevents unrelated NAS files
			// with the same display name from being replaced.
			identity := fmt.Sprintf("%s\x00%d\x00%d\x00%s", bundle.SourceID, resource.Id, resource.CreatedAt, resource.UniqueID)
			digest := sha256.Sum256([]byte(identity))
			resource.Name = filepath.Join("synced", fmt.Sprintf("%x", digest[:16]), filepath.Base(resource.Name))
		}
		var extra map[string]string
		if json.Unmarshal([]byte(resource.Extra), &extra) == nil && extra["_hermes_postprocessed"] != "" {
			extra["_hermes_postprocessed"] = filepath.Base(resource.Name)
			encoded_extra, err := json.Marshal(extra)
			if err != nil {
				return nil, err
			}
			resource.Extra = string(encoded_extra)
		}
		bundle.Resources[resource_index] = resource
		resources_by_id[resource.Id] = resource
	}
	stages := make([]sync_file_stage, 0)
	seen_paths := make(map[string]bool)
	resource_bytes := make(map[int]int64)
	resource_files := make(map[int]bool)
	var total_bytes int64
	defer func() {
		for _, stage := range stages {
			if stage.TempPath != "" {
				_ = os.Remove(stage.TempPath)
			}
		}
	}()
	for {
		header, next_err := archive_reader.Next()
		if errors.Is(next_err, io.EOF) {
			break
		}
		if next_err != nil {
			return nil, fmt.Errorf("读取同步文件失败: %w", next_err)
		}
		if !header.FileInfo().Mode().IsRegular() {
			return nil, fmt.Errorf("同步包包含不支持的文件类型")
		}
		final_path, path_err := c.sync_archive_destination(header.Name, resources_by_id)
		if path_err != nil {
			return nil, path_err
		}
		if seen_paths[final_path] {
			return nil, fmt.Errorf("同步包包含重复文件 %q", final_path)
		}
		seen_paths[final_path] = true
		if err := os.MkdirAll(filepath.Dir(final_path), 0755); err != nil {
			return nil, fmt.Errorf("创建同步文件目录失败: %w", err)
		}
		temp_file, err := os.CreateTemp(filepath.Dir(final_path), ".sync-*")
		if err != nil {
			return nil, fmt.Errorf("创建同步临时文件失败: %w", err)
		}
		temp_path := temp_file.Name()
		copied, copy_err := io.Copy(temp_file, archive_reader)
		close_err := temp_file.Close()
		if copy_err != nil || close_err != nil || copied != header.Size {
			_ = os.Remove(temp_path)
			return nil, fmt.Errorf("写入同步文件失败")
		}
		stages = append(stages, sync_file_stage{TempPath: temp_path, FinalPath: final_path})
		total_bytes += copied
		parts := strings.Split(header.Name, "/")
		resource_id, _ := strconv.Atoi(parts[1])
		resource_files[resource_id] = true
		resource_bytes[resource_id] += copied
	}
	if bundle.Version >= 2 {
		for index := range bundle.Resources {
			resource := &bundle.Resources[index]
			if !resource_files[resource.Id] || resource.Status != model.ResourceStatusFinished {
				resource.Status = model.ResourceStatusWaiting
			}
			resource.Speed = 0
			resource.Downloaded = resource_bytes[resource.Id]
		}
	}

	import_result := &sync_import_result{Contents: len(bundle.Contents), Tasks: len(bundle.Tasks), Resources: len(bundle.Resources), Files: len(stages), Bytes: total_bytes}
	sync_import_mu.Lock()
	defer sync_import_mu.Unlock()
	transaction_err := c.db.Transaction(func(tx *gorm.DB) error {
		if err := c.import_sync_records(tx, bundle, import_result); err != nil {
			return err
		}
		return install_sync_files(c.cfg.DownloadDir, stages)
	})
	if transaction_err != nil {
		rollback_sync_files(stages)
		return nil, transaction_err
	}
	finish_sync_files(stages)
	return import_result, nil
}

func (c *APIClient) sync_archive_destination(archive_path string, resources_by_id map[int]model.DownloadResource) (string, error) {
	parts := strings.Split(archive_path, "/")
	if len(parts) < 4 || parts[0] != "files" {
		return "", fmt.Errorf("同步文件路径无效")
	}
	resource_id, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("同步文件资源 ID 无效")
	}
	resource, ok := resources_by_id[resource_id]
	if !ok {
		return "", fmt.Errorf("同步文件引用了未知资源 %d", resource_id)
	}
	base_path, err := sync_resource_path(c.cfg.DownloadDir, resource.Name)
	if err != nil {
		return "", fmt.Errorf("资源 %d: %w", resource_id, err)
	}
	candidate_type := parts[2]
	relative_path := path.Clean(strings.Join(parts[3:], "/"))
	if relative_path == "." || relative_path == ".." || strings.HasPrefix(relative_path, "../") {
		return "", fmt.Errorf("同步文件相对路径无效")
	}
	var candidate_root string
	if candidate_type == "recording" || candidate_type == "playback" {
		download_name := resource.UniqueID
		var extra map[string]string
		if json.Unmarshal([]byte(resource.Extra), &extra) == nil && extra["_hermes_download_name"] != "" {
			download_name = extra["_hermes_download_name"]
		}
		if download_name == "" || download_name == "." || download_name == ".." || filepath.Base(download_name) != download_name {
			return "", fmt.Errorf("直播资源编号无效")
		}
		base_path = filepath.Join(filepath.Dir(base_path), download_name)
	}
	switch candidate_type {
	case "final":
		if relative_path != "file" {
			return "", fmt.Errorf("资源文件路径无效")
		}
		return base_path, nil
	case "recording":
		candidate_root = hermes.StreamRecordingDir(base_path)
	case "playback":
		candidate_root = hermes.StreamPlaybackDir(base_path)
	default:
		return "", fmt.Errorf("同步文件类型无效")
	}
	final_path, err := path_within_root(candidate_root, filepath.Join(candidate_root, filepath.FromSlash(relative_path)))
	if err != nil {
		return "", err
	}
	return path_within_root(c.cfg.DownloadDir, final_path)
}

func sync_resource_path(download_dir string, resource_name string) (string, error) {
	resource_name = strings.TrimSpace(resource_name)
	if resource_name == "" || filepath.IsAbs(resource_name) {
		return "", fmt.Errorf("资源文件名无效")
	}
	clean_name := filepath.Clean(resource_name)
	if clean_name == "." || clean_name == ".." || strings.HasPrefix(clean_name, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("资源文件名无效")
	}
	return path_within_root(download_dir, filepath.Join(download_dir, clean_name))
}

func path_within_root(root string, target string) (string, error) {
	absolute_root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absolute_target, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	relative_path, err := filepath.Rel(absolute_root, absolute_target)
	if err != nil || relative_path == "." || relative_path == ".." || strings.HasPrefix(relative_path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("文件路径超出下载目录")
	}
	// The configured root may be a symlink; archive-controlled children must not be.
	for current := absolute_target; current != absolute_root; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("同步文件路径包含符号链接: %s", current)
		}
	}
	return absolute_target, nil
}

func (c *APIClient) import_sync_records(tx *gorm.DB, bundle sync_bundle, import_result *sync_import_result) error {
	if bundle.Version >= 2 {
		return c.recreate_sync_tasks(tx, bundle, import_result)
	}
	for content_index := range bundle.Contents {
		content := bundle.Contents[content_index]
		if strings.TrimSpace(content.Id) == "" {
			return fmt.Errorf("同步内容 ID 为空")
		}
		if err := tx.Omit("Assets", "TextTracks").Save(&content).Error; err != nil {
			return fmt.Errorf("保存内容 %q 失败: %w", content.Id, err)
		}
	}

	task_ids := make(map[int]int, len(bundle.Tasks))
	target_tasks := make([]model.DownloadTask, len(bundle.Tasks))
	for task_index := range bundle.Tasks {
		source_task := bundle.Tasks[task_index]
		source_id := source_task.Id
		if source_id <= 0 || source_task.CreatedAt <= 0 {
			return fmt.Errorf("同步下载任务 ID 或创建时间无效")
		}
		if _, exists := task_ids[source_id]; exists {
			return fmt.Errorf("同步下载任务 ID %d 重复", source_id)
		}
		target_task, err := upsert_sync_task(tx, source_task, c.cfg.DownloadDir)
		if err != nil {
			return err
		}
		task_ids[source_id] = target_task.Id
		target_tasks[task_index] = target_task
		import_result.task_ids = append(import_result.task_ids, target_task.Id)
	}
	for task_index, source_task := range bundle.Tasks {
		target_task := target_tasks[task_index]
		updates := map[string]interface{}{"root_task_id": target_task.Id, "parent_task_id": nil}
		if mapped_root_id, ok := task_ids[source_task.RootTaskID]; ok {
			updates["root_task_id"] = mapped_root_id
		}
		if source_task.ParentTaskID != nil {
			if mapped_parent_id, ok := task_ids[*source_task.ParentTaskID]; ok {
				updates["parent_task_id"] = mapped_parent_id
			}
		}
		if err := tx.Model(&model.DownloadTask{}).Where("id = ?", target_task.Id).Updates(updates).Error; err != nil {
			return fmt.Errorf("更新同步任务关系失败: %w", err)
		}
	}

	for resource_index := range bundle.Resources {
		source_resource := bundle.Resources[resource_index]
		if source_resource.Id <= 0 || source_resource.CreatedAt <= 0 {
			return fmt.Errorf("同步下载资源 ID 或创建时间无效")
		}
		if source_resource.TaskId != nil {
			mapped_task_id, ok := task_ids[*source_resource.TaskId]
			if !ok {
				return fmt.Errorf("同步资源 %d 引用了未知任务", source_resource.Id)
			}
			source_resource.TaskId = &mapped_task_id
		}
		resource_path, path_err := sync_resource_path(c.cfg.DownloadDir, source_resource.Name)
		if path_err != nil {
			return fmt.Errorf("同步资源 %d: %w", source_resource.Id, path_err)
		}
		source_resource.DownloadDir = filepath.Dir(resource_path)
		source_resource.Name = filepath.Base(resource_path)
		if _, err := upsert_sync_resource(tx, source_resource); err != nil {
			return err
		}
	}
	return nil
}

func upsert_sync_task(tx *gorm.DB, source_task model.DownloadTask, download_dir string) (model.DownloadTask, error) {
	var target_task model.DownloadTask
	query := tx.Where("platform_id = ? AND unique_id = ? AND created_at = ?", source_task.PlatformId, source_task.UniqueID, source_task.CreatedAt)
	if source_task.UniqueID == "" {
		query = tx.Where("platform_id = ? AND source_url = ? AND name = ? AND created_at = ?", source_task.PlatformId, source_task.SourceURL, source_task.Name, source_task.CreatedAt)
	}
	find_err := query.First(&target_task).Error
	if find_err != nil && !errors.Is(find_err, gorm.ErrRecordNotFound) {
		return target_task, find_err
	}
	target_id := 0
	if find_err == nil {
		target_id = target_task.Id
	}
	source_task.Id = target_id
	source_task.ParentTaskID = nil
	source_task.RootTaskID = 0
	source_task.ConfigJSON = sync_task_config(source_task.ConfigJSON, download_dir)
	if err := tx.Save(&source_task).Error; err != nil {
		return target_task, fmt.Errorf("保存下载任务 %q 失败: %w", source_task.Name, err)
	}
	return source_task, nil
}

func sync_task_config(config_json string, download_dir string) string {
	config_value := make(map[string]interface{})
	if strings.TrimSpace(config_json) != "" {
		_ = json.Unmarshal([]byte(config_json), &config_value)
	}
	config_value["download_dir"] = download_dir
	encoded_config, err := json.Marshal(config_value)
	if err != nil {
		return config_json
	}
	return string(encoded_config)
}

func upsert_sync_resource(tx *gorm.DB, source_resource model.DownloadResource) (model.DownloadResource, error) {
	var target_resource model.DownloadResource
	query := tx
	if source_resource.TaskId == nil {
		query = query.Where("task_id IS NULL")
	} else {
		query = query.Where("task_id = ?", *source_resource.TaskId)
	}
	if source_resource.UniqueID != "" {
		query = query.Where("unique_id = ? AND created_at = ?", source_resource.UniqueID, source_resource.CreatedAt)
	} else {
		query = query.Where("name = ? AND created_at = ?", source_resource.Name, source_resource.CreatedAt)
	}
	find_err := query.First(&target_resource).Error
	if find_err != nil && !errors.Is(find_err, gorm.ErrRecordNotFound) {
		return target_resource, find_err
	}
	if find_err == nil {
		source_resource.Id = target_resource.Id
	} else {
		source_resource.Id = 0
	}
	if err := tx.Save(&source_resource).Error; err != nil {
		return target_resource, fmt.Errorf("保存下载资源 %q 失败: %w", source_resource.Name, err)
	}
	return source_resource, nil
}

func install_sync_files(download_dir string, stages []sync_file_stage) error {
	for stage_index := range stages {
		stage := &stages[stage_index]
		for _, path := range []string{stage.TempPath, stage.FinalPath} {
			if _, err := path_within_root(download_dir, path); err != nil {
				return err
			}
		}
		if info, err := os.Lstat(stage.FinalPath); err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("同步目标路径不是普通文件")
			}
			backup_file, create_err := os.CreateTemp(filepath.Dir(stage.FinalPath), ".sync-backup-*")
			if create_err != nil {
				return create_err
			}
			stage.BackupPath = backup_file.Name()
			if close_err := backup_file.Close(); close_err != nil {
				return close_err
			}
			if remove_err := os.Remove(stage.BackupPath); remove_err != nil {
				return remove_err
			}
			if rename_err := os.Rename(stage.FinalPath, stage.BackupPath); rename_err != nil {
				return rename_err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(stage.TempPath, stage.FinalPath); err != nil {
			if stage.BackupPath != "" {
				_ = os.Rename(stage.BackupPath, stage.FinalPath)
				stage.BackupPath = ""
			}
			return err
		}
		stage.TempPath = ""
		stage.Installed = true
	}
	return nil
}

func rollback_sync_files(stages []sync_file_stage) {
	for stage_index := len(stages) - 1; stage_index >= 0; stage_index-- {
		stage := &stages[stage_index]
		if stage.Installed {
			_ = os.Remove(stage.FinalPath)
		}
		if stage.BackupPath != "" {
			_ = os.Rename(stage.BackupPath, stage.FinalPath)
		}
	}
}

func finish_sync_files(stages []sync_file_stage) {
	for stage_index := range stages {
		if stages[stage_index].BackupPath != "" {
			_ = os.Remove(stages[stage_index].BackupPath)
		}
	}
}
