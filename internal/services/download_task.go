package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

// ---------------------------------------------------------------------------
// DownloadTaskService
// ---------------------------------------------------------------------------

type DownloadTaskService struct {
	db           *gorm.DB
	logger       *zerolog.Logger
	downloader   *hermes.HermesEngine
	hook_manager *hermes.HookManager
	work_dir     string
	download_dir string
}

func NewDownloadTaskService(
	db *gorm.DB,
	logger *zerolog.Logger,
	downloader *hermes.HermesEngine,
	hook_manager *hermes.HookManager,
	work_dir string,
	download_dir string,
) *DownloadTaskService {
	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}
	return &DownloadTaskService{
		db:           db,
		logger:       logger,
		downloader:   downloader,
		hook_manager: hook_manager,
		work_dir:     work_dir,
		download_dir: download_dir,
	}
}

// ---------------------------------------------------------------------------
// Request / body types
// ---------------------------------------------------------------------------

type CreateDownloadTaskRequest struct {
	Objects []CreateDownloadTaskBody `json:"objects"`
}

type CreateDownloadTaskBody struct {
	Platform     string          `json:"platform"`
	Content      json.RawMessage `json:"content"`
	DownloadDir  string          `json:"download_dir"`
	Filename     string          `json:"filename"`
	Config       map[string]any  `json:"config"`
	ParentTaskID *int            `json:"parent_task_id"`
	RelationType string          `json:"relation_type"`
}

type TaskV1IDBody struct {
	TaskID int `json:"task_id"`
}

type CreateDownloadTaskByURLRequest struct {
	Objects []CreateDownloadTaskByURLBody `json:"objects"`
}

type CreateDownloadTaskByURLBody struct {
	URL          string         `json:"url"`
	DownloadDir  string         `json:"download_dir"`
	Filename     string         `json:"filename"`
	Config       map[string]any `json:"config"`
	ParentTaskID *int           `json:"parent_task_id"`
	RelationType string         `json:"relation_type"`
}

// ---------------------------------------------------------------------------
// Error types
// ---------------------------------------------------------------------------

// DuplicateTaskError indicates an existing download task with the same content was found during task creation.
type DuplicateTaskError struct {
	ExistingTaskID   int
	ExistingTaskName string
	IncomingUniqueID string
}

func (e *DuplicateTaskError) Error() string {
	return "已存在该下载内容"
}

func (e *DuplicateTaskError) StatusCode() int {
	return 409
}

type duplicateConflict struct {
	Type        string
	TaskID      int
	FilePath    string
	ResourceKey string
}

func build_platform_config_json(config map[string]any, download_dir, filename string) ([]byte, error) {
	platform_config := make(map[string]any, len(config)+2)
	for key, value := range config {
		platform_config[key] = value
	}
	platform_config["download_dir"] = download_dir
	if filename != "" {
		platform_config["filename"] = filename
	}
	return json.Marshal(platform_config)
}

func task_config_with_download_dir(config_json string, download_dir string) (string, error) {
	task_config := make(map[string]any)
	if strings.TrimSpace(config_json) != "" {
		if err := json.Unmarshal([]byte(config_json), &task_config); err != nil {
			return "", err
		}
		if task_config == nil {
			task_config = make(map[string]any)
		}
	}
	task_config["download_dir"] = strings.TrimSpace(download_dir)
	data, err := json.Marshal(task_config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func download_config_bool(config map[string]any, key string) bool {
	value, _ := config[key].(bool)
	return value
}

// ---------------------------------------------------------------------------
// Result types (replace gin.H returns)
// ---------------------------------------------------------------------------

type EndpointDetail struct {
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
	Priority int    `json:"priority"`
}

type ResourceDetail struct {
	Index     int              `json:"index"`
	Name      string           `json:"name"`
	Kind      string           `json:"kind"`
	Endpoints []EndpointDetail `json:"endpoints"`
}

type ResourceTreeNode struct {
	Name      string              `json:"name"`
	Type      string              `json:"type"` // "file" | "directory"
	Kind      string              `json:"kind,omitempty"`
	Endpoints []EndpointDetail    `json:"endpoints,omitempty"`
	Children  []*ResourceTreeNode `json:"children,omitempty"`
}

type PrepareTaskResult struct {
	Platform      string            `json:"platform"`
	TaskName      string            `json:"task_name"`
	DownloadDir   string            `json:"download_dir"`
	Resources     []ResourceDetail  `json:"resources"`
	Tree          *ResourceTreeNode `json:"tree"`
	ResourceCount int               `json:"resource_count"`
	EndpointCount int               `json:"endpoint_count"`
	Content       any               `json:"content"`
	Account       any               `json:"account"`
}

type PrepareTaskByURLResult struct {
	URL           string           `json:"url"`
	Protocol      string           `json:"protocol"`
	TaskName      string           `json:"task_name"`
	DownloadDir   string           `json:"download_dir"`
	Resources     []ResourceDetail `json:"resources"`
	ResourceCount int              `json:"resource_count"`
	EndpointCount int              `json:"endpoint_count"`
}

type CreateTaskResult struct {
	Task      model.DownloadTask       `json:"task"`
	Resource  model.DownloadResource   `json:"resource"`
	Endpoint  model.DownloadEndpoint   `json:"endpoint"`
	Resources []model.DownloadResource `json:"resources"`
	Endpoints []model.DownloadEndpoint `json:"endpoints"`
	Content   any                      `json:"content"`
	Account   any                      `json:"account"`
}

type CreateTaskByURLResult struct {
	Task     model.DownloadTask     `json:"task"`
	Resource model.DownloadResource `json:"resource"`
	Endpoint model.DownloadEndpoint `json:"endpoint"`
}

type TaskListResult struct {
	List     []DownloadTaskRecord `json:"list"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// ---------------------------------------------------------------------------
// DownloadTaskRecord & DownloadTaskFileRecord
// ---------------------------------------------------------------------------

type DownloadTaskRecord struct {
	ID           int                      `json:"id"`
	ContentID    *string                  `json:"content_id,omitempty"`
	ContentType  string                   `json:"content_type,omitempty"`
	ParentTaskID *int                     `json:"parent_task_id,omitempty"`
	RootTaskID   int                      `json:"root_task_id"`
	RelationType string                   `json:"relation_type,omitempty"`
	ChildCount   int                      `json:"child_count"`
	Name         string                   `json:"name"`
	PlatformID   string                   `json:"platform_id"`
	Status       int                      `json:"status"`
	SourceURL    string                   `json:"source_url"`
	CoverURL     string                   `json:"cover_url"`
	CoverWidth   string                   `json:"cover_width"`
	CoverHeight  string                   `json:"cover_height"`
	ConfigJSON   string                   `json:"config_json"`
	MetadataJSON string                   `json:"metadata_json"`
	URL          string                   `json:"url"`
	Size         int64                    `json:"size"`
	Downloaded   int64                    `json:"downloaded"`
	Speed        int64                    `json:"speed"`
	Progress     float64                  `json:"progress"`
	Error        string                   `json:"error"`
	Files        []DownloadTaskFileRecord `json:"files"`
	FileCount    int                      `json:"file_count"`
	CreatedAt    int64                    `json:"created_at"`
	UpdatedAt    int64                    `json:"updated_at"`
}

type DownloadTaskFileRecord struct {
	ID          int     `json:"id"`
	DownloadDir string  `json:"download_dir"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	Size        int64   `json:"size"`
	Downloaded  int64   `json:"downloaded"`
	Speed       int64   `json:"speed"`
	Progress    float64 `json:"progress"`
	URL         string  `json:"url"`
	Error       string  `json:"error"`
}

// DownloadTaskStats holds counts of download tasks by status.
type DownloadTaskStats struct {
	Total       int `json:"total"`
	Downloading int `json:"downloading"`
	Paused      int `json:"paused"`
	Waiting     int `json:"waiting"`
	Finished    int `json:"finished"`
	Error       int `json:"error"`
}

// ---------------------------------------------------------------------------
// Business logic methods
// ---------------------------------------------------------------------------

// PrepareTask previews a single platform download task (no DB writes, no download start).
func (s *DownloadTaskService) PrepareTask(body CreateDownloadTaskBody) (*PrepareTaskResult, error) {
	if body.Platform == "" {
		return nil, fmt.Errorf("platform 不能为空")
	}

	h := adapter.Get(body.Platform)
	if h == nil {
		return nil, fmt.Errorf("不支持的平台: %s", body.Platform)
	}

	requested_download_dir := body.DownloadDir
	if strings.TrimSpace(requested_download_dir) == "" {
		requested_download_dir, _ = body.Config["download_dir"].(string)
	}
	save_dir, err := s.resolve_save_dir(requested_download_dir)
	if err != nil {
		return nil, fmt.Errorf("准备下载目录失败: %w", err)
	}

	config_json, err := build_platform_config_json(body.Config, save_dir, body.Filename)
	if err != nil {
		return nil, fmt.Errorf("构建下载配置失败: %w", err)
	}

	info, err := h.BuildDownloadTask(body.Content, json.RawMessage(config_json))
	if err != nil {
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	resource_infos := info.Resources
	for _, ri := range resource_infos {
		if len(ri.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 %s 没有下载端点", ri.Resource.Name)
		}
	}

	resources := make([]ResourceDetail, 0, len(resource_infos))
	total_endpoints := 0
	for i, ri := range resource_infos {
		eps := make([]EndpointDetail, 0, len(ri.Endpoints))
		for _, ep := range ri.Endpoints {
			eps = append(eps, EndpointDetail{
				Protocol: ep.Protocol,
				URL:      ep.URL,
				Priority: ep.Priority,
			})
		}
		resources = append(resources, ResourceDetail{
			Index:     i,
			Name:      ri.Resource.Name,
			Kind:      ri.Resource.Kind,
			Endpoints: eps,
		})
		total_endpoints += len(ri.Endpoints)
	}
	tree := build_resource_tree(resources)

	return &PrepareTaskResult{
		Platform:      body.Platform,
		TaskName:      info.Task.Name,
		DownloadDir:   save_dir,
		Resources:     resources,
		Tree:          tree,
		ResourceCount: len(resource_infos),
		EndpointCount: total_endpoints,
		Content:       info.Content,
		Account:       info.Account,
	}, nil
}

// PrepareTaskByURL previews a download task created via resource URL (no DB writes, no download start).
func (s *DownloadTaskService) PrepareTaskByURL(body CreateDownloadTaskByURLBody) (*PrepareTaskByURLResult, error) {
	if body.URL == "" {
		return nil, fmt.Errorf("url 不能为空")
	}

	parsed_url, err := url.Parse(body.URL)
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return nil, fmt.Errorf("无效的下载地址")
	}

	protocol := strings.ToUpper(parsed_url.Scheme)

	requested_download_dir := body.DownloadDir
	if requested_download_dir == "" {
		requested_download_dir, _ = body.Config["download_dir"].(string)
	}
	save_dir, err := s.resolve_save_dir(requested_download_dir)
	if err != nil {
		return nil, fmt.Errorf("准备下载目录失败: %w", err)
	}
	filename := body.Filename
	if filename == "" {
		filename, _ = body.Config["filename"].(string)
	}
	if filename == "" {
		base := filepath.Base(parsed_url.Path)
		if base != "" && base != "." && base != "/" {
			if decoded, err := url.QueryUnescape(base); err == nil {
				filename = decoded
			} else {
				filename = base
			}
		}
	}
	if filename == "" {
		filename = body.URL
	}
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return nil, fmt.Errorf("无法确定下载文件名")
	}

	download_dir := save_dir

	return &PrepareTaskByURLResult{
		URL:         body.URL,
		Protocol:    protocol,
		TaskName:    filename,
		DownloadDir: download_dir,
		Resources: []ResourceDetail{{
			Index: 0,
			Name:  filename,
			Kind:  "file",
			Endpoints: []EndpointDetail{{
				Protocol: protocol,
				URL:      body.URL,
				Priority: 0,
			}},
		}},
		ResourceCount: 1,
		EndpointCount: 1,
	}, nil
}

// CreateTask creates a single platform download task.
func (s *DownloadTaskService) CreateTask(body CreateDownloadTaskBody) (result *CreateTaskResult, ret_err error) {
	s.logger.Info().Str("platform", body.Platform).Msg("start processing single download task creation request")

	var task model.DownloadTask
	defer func() {
		if ret_err != nil && task.Id > 0 {
			s.db.Model(&task).Updates(map[string]any{
				"status":        model.TaskStatusFailed,
				"error_message": ret_err.Error(),
			})
			s.logger.Warn().Int("task_id", task.Id).Err(ret_err).Msg("subsequent steps after task creation failed, task marked as failed")
		}
	}()

	if s.db == nil {
		s.logger.Error().Msg("database not initialized, cannot create download task")
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	if body.Platform == "" {
		return nil, fmt.Errorf("platform 不能为空")
	}

	h := adapter.Get(body.Platform)
	if h == nil {
		s.logger.Warn().Str("platform", body.Platform).Msg("unsupported platform")
		return nil, fmt.Errorf("不支持的平台: %s", body.Platform)
	}

	requested_download_dir := body.DownloadDir
	if strings.TrimSpace(requested_download_dir) == "" {
		requested_download_dir, _ = body.Config["download_dir"].(string)
	}
	save_dir, err := s.resolve_save_dir(requested_download_dir)
	if err != nil {
		return nil, fmt.Errorf("准备下载目录失败: %w", err)
	}

	config_json, err := build_platform_config_json(body.Config, save_dir, body.Filename)
	if err != nil {
		return nil, fmt.Errorf("构建下载配置失败: %w", err)
	}

	info, err := h.BuildDownloadTask(body.Content, json.RawMessage(config_json))
	if err != nil {
		s.logger.Error().Str("platform", body.Platform).Err(err).Msg("platform failed to build download task")
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		s.logger.Warn().Str("platform", body.Platform).Msg("platform returned no download task info")
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	content := info.Content
	normalize_content_taxonomy(content)
	if b, err := json.Marshal(content); err == nil {
		s.logger.Info().RawJSON("content", b).Msg("content detail")
	}
	account := info.Account

	resource_infos := info.Resources
	s.logger.Info().Str("platform", body.Platform).Str("task_name", info.Task.Name).Int("resource_count", len(resource_infos)).Msg("platform download task built successfully")
	for _, ri := range resource_infos {
		if len(ri.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 %s 没有下载端点", ri.Resource.Name)
		}
	}

	task_name := info.Task.Name

	// Check for duplicates
	resource_keys := make([]string, 0, len(resource_infos))
	resource_names := make([]string, 0, len(resource_infos))
	for _, ri := range resource_infos {
		resource_keys = append(resource_keys, ri.Resource.UniqueID)
		resource_names = append(resource_names, ri.Resource.Name)
	}
	if err := s.check_duplicate(save_dir, info.Task.UniqueID, resource_keys, resource_names, download_config_bool(body.Config, "duplicate"), download_config_bool(body.Config, "overwrite")); err != nil {
		return nil, err
	}

	// onTaskCreate hook
	if s.hook_manager != nil && s.hook_manager.HasCreateHook() {
		task_input := s.build_task_input(info, task_name, save_dir, body.Filename, body.Config)
		modified, err := s.hook_manager.InvokeCreateHook(task_input)
		if err != nil {
			return nil, fmt.Errorf("onTaskCreate hook 执行失败: %w", err)
		}
		task_name, save_dir = s.apply_task_input_modifications(info, task_name, save_dir, modified)
	}
	save_dir, err = s.resolve_save_dir(save_dir)
	if err != nil {
		return nil, fmt.Errorf("准备下载目录失败: %w", err)
	}
	task_config_json, err := task_config_with_download_dir(info.Task.ConfigJSON, save_dir)
	if err != nil {
		return nil, fmt.Errorf("保存任务下载目录失败: %w", err)
	}

	// Write to database
	now := time.Now().UnixMilli()
	task = *info.Task
	if content != nil && content.Id != "" {
		content_id := content.Id
		task.ContentId = &content_id
	}
	task.Name = task_name
	task.ConfigJSON = task_config_json
	task.Status = model.TaskStatusWaiting
	task.CreatedAt = now
	task.UpdatedAt = now
	if err := database.ApplyTaskLineage(s.db, &task, body.ParentTaskID, body.RelationType); err != nil {
		return nil, err
	}
	if err := s.db.Create(&task).Error; err != nil {
		s.logger.Error().Str("platform", body.Platform).Err(err).Msg("failed to write download task to database")
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}
	if err := database.FinalizeTaskRoot(s.db, &task); err != nil {
		return nil, err
	}
	s.logger.Info().Int("task_id", task.Id).Str("task_name", task.Name).Str("platform", body.Platform).Msg("download task written to database")

	// Save Content
	if content != nil {
		content.UpdatedAt = now
		if err := s.db.Save(content).Error; err != nil {
			return nil, fmt.Errorf("保存 Content 失败: %w", err)
		}
	}

	// Save account and establish Content ↔ Account many-to-many association
	if account != nil && account.ExternalId != "" {
		content_id := ""
		if content != nil {
			content_id = content.Id
		}
		persisted_account, err := NewContentService(s.db).UpsertAccountAndLinkContent(content_id, account, "owner", now)
		if err != nil {
			return nil, err
		}
		account = persisted_account
	}

	if err := save_content_details(s.db, content, info.ContentDetail, info.ContentDetails, now); err != nil {
		return nil, fmt.Errorf("保存内容详情与关联关系失败: %w", err)
	}

	resources := make([]model.DownloadResource, 0, len(resource_infos))
	endpoints := make([]model.DownloadEndpoint, 0, len(resource_infos))
	for i := range resource_infos {
		resource := resource_infos[i].Resource
		task_id := task.Id
		resource.TaskId = &task_id
		resource.DownloadDir = save_dir
		if resource.CreatedAt == 0 {
			resource.CreatedAt = now
		}
		resource.UpdatedAt = now
		if err := s.db.Create(&resource).Error; err != nil {
			return nil, fmt.Errorf("创建资源失败: %w", err)
		}
		if err := save_download_resource_assets(
			s.db,
			&resource,
			resource_infos[i],
			content,
			info.ContentDetail,
			now,
		); err != nil {
			return nil, fmt.Errorf("关联下载资源与内容资产失败: %w", err)
		}
		resources = append(resources, resource)
		for _, endpoint_info := range resource_infos[i].Endpoints {
			endpoint := endpoint_info
			endpoint.ResourceId = resource.Id
			if endpoint.CreatedAt == 0 {
				endpoint.CreatedAt = now
			}
			endpoint.UpdatedAt = now
			if err := s.db.Create(&endpoint).Error; err != nil {
				return nil, fmt.Errorf("创建端点失败: %w", err)
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	if len(resources) == 0 || len(endpoints) == 0 {
		return nil, fmt.Errorf("平台未返回可下载资源或端点")
	}
	first_resource := resources[0]
	first_endpoint := endpoints[0]

	if err := s.start_created_download_task(task.Id); err != nil {
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}
	task.Status = model.TaskStatusPreparing

	return &CreateTaskResult{
		Task:      task,
		Resource:  first_resource,
		Endpoint:  first_endpoint,
		Resources: resources,
		Endpoints: endpoints,
		Content:   content,
		Account:   account,
	}, nil
}

// normalize_content_taxonomy keeps Content.Type small and stable. Historical
// adapter-specific type names remain available in Content.Subtype.
func normalize_content_taxonomy(content *model.Content) {
	if content == nil {
		return
	}
	content_type := strings.ToLower(strings.TrimSpace(content.Type))
	content_subtype := strings.ToLower(strings.TrimSpace(content.Subtype))
	canonical_type := content_type

	switch content_type {
	case "short_video", "long_video", "movie", "film", "tv_episode", "clip", "live_replay":
		canonical_type = model.ContentTypeVideo
	case "music", "audiobook", "voice", "radio", "space_recording":
		canonical_type = model.ContentTypeAudio
	case "image_set", "gallery", "photo_album", "illustration_set":
		canonical_type = model.ContentTypeAlbum
	case "blog", "news", "newsletter", "question", "answer", "wiki":
		canonical_type = model.ContentTypeArticle
	case "microblog", "tweet", "status", "thread", "comment":
		canonical_type = model.ContentTypePost
	case "ebook", "pdf", "slides", "spreadsheet":
		canonical_type = model.ContentTypeDocument
	case "livestream", "audio_room":
		canonical_type = model.ContentTypeLive
	case "playlist", "series", "feed", "bookmark_collection":
		canonical_type = model.ContentTypeCollection
	case "chat", "ai_chat", "human_chat", "email_thread":
		canonical_type = model.ContentTypeConversation
	}

	if content_subtype == "" && canonical_type != content_type {
		content_subtype = content_type
	}
	content.Type = canonical_type
	content.Subtype = content_subtype
}

// CreateTaskByURL creates a single download task via resource URL.
func (s *DownloadTaskService) CreateTaskByURL(body CreateDownloadTaskByURLBody) (*CreateTaskByURLResult, error) {
	if body.URL == "" {
		return nil, fmt.Errorf("url 不能为空")
	}

	parsed_url, err := url.Parse(body.URL)
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return nil, fmt.Errorf("无效的下载地址")
	}

	protocol := strings.ToUpper(parsed_url.Scheme)

	requested_download_dir := body.DownloadDir
	if requested_download_dir == "" {
		requested_download_dir, _ = body.Config["download_dir"].(string)
	}
	save_dir, err := s.resolve_save_dir(requested_download_dir)
	if err != nil {
		return nil, fmt.Errorf("准备下载目录失败: %w", err)
	}
	filename := body.Filename
	if filename == "" {
		filename, _ = body.Config["filename"].(string)
	}
	if filename == "" {
		base := filepath.Base(parsed_url.Path)
		if base != "" && base != "." && base != "/" {
			if decoded, err := url.QueryUnescape(base); err == nil {
				filename = decoded
			} else {
				filename = base
			}
		}
	}
	if filename == "" {
		filename = body.URL
	}
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return nil, fmt.Errorf("无法确定下载文件名")
	}

	download_dir := save_dir

	task_name := filename

	url_config := make(map[string]any, len(body.Config)+3)
	for key, value := range body.Config {
		url_config[key] = value
	}
	url_config["url"] = body.URL
	if filename != "" {
		url_config["filename"] = filename
	}
	url_config_json, err := json.Marshal(url_config)
	if err != nil {
		return nil, fmt.Errorf("构建下载配置失败: %w", err)
	}
	task_config_json, err := task_config_with_download_dir(string(url_config_json), download_dir)
	if err != nil {
		return nil, fmt.Errorf("保存任务下载目录失败: %w", err)
	}

	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	now := time.Now().UnixMilli()

	task := model.DownloadTask{
		Name:       task_name,
		Status:     model.TaskStatusWaiting,
		ConfigJSON: task_config_json,
	}
	task.CreatedAt = now
	task.UpdatedAt = now

	if err := database.ApplyTaskLineage(s.db, &task, body.ParentTaskID, body.RelationType); err != nil {
		return nil, err
	}
	if err := s.db.Create(&task).Error; err != nil {
		s.logger.Error().Str("url", body.URL).Err(err).Msg("failed to write URL download task to database")
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}
	if err := database.FinalizeTaskRoot(s.db, &task); err != nil {
		return nil, err
	}

	s.logger.Info().Int("task_id", task.Id).Str("url", body.URL).Str("download_dir", download_dir).Msg("URL download task written to database")

	task_id := task.Id
	resource := model.DownloadResource{
		TaskId:      &task_id,
		DownloadDir: download_dir,
		Name:        filename,
		Kind:        "file",
		Status:      0,
		MergeOrder:  0,
	}
	resource.CreatedAt = now
	resource.UpdatedAt = now

	if err := s.db.Create(&resource).Error; err != nil {
		return nil, fmt.Errorf("创建资源失败: %w", err)
	}

	endpoint := model.DownloadEndpoint{
		ResourceId: resource.Id,
		Protocol:   protocol,
		URL:        body.URL,
		Priority:   0,
		Enabled:    1,
		Status:     0,
	}
	endpoint.CreatedAt = now
	endpoint.UpdatedAt = now

	if err := s.db.Create(&endpoint).Error; err != nil {
		return nil, fmt.Errorf("创建端点失败: %w", err)
	}

	if err := s.start_created_download_task(task.Id); err != nil {
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}
	task.Status = model.TaskStatusPreparing

	return &CreateTaskByURLResult{
		Task:     task,
		Resource: resource,
		Endpoint: endpoint,
	}, nil
}

// StartTask starts a download task.
func (s *DownloadTaskService) StartTask(task_id int) (*model.DownloadTask, error) {
	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	var task model.DownloadTask
	if err := s.db.Where("id = ?", task_id).First(&task).Error; err != nil {
		return nil, fmt.Errorf("下载任务不存在")
	}

	if task.Status != model.TaskStatusWaiting &&
		task.Status != model.TaskStatusPaused &&
		task.Status != model.TaskStatusFailed {
		return nil, fmt.Errorf("当前状态不允许启动")
	}

	s.logger.Info().Int("task_id", task_id).Str("task_name", task.Name).Int("previous_status", task.Status).Msg("received start download task request")

	if err := s.downloader.StartTask(task.Id); err != nil {
		s.logger.Error().Int("task_id", task_id).Err(err).Msg("failed to start download task")
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}
	s.logger.Info().Int("task_id", task_id).Str("status", "preparing").Msg("download task started")

	task.Status = model.TaskStatusPreparing
	return &task, nil
}

// PauseTask pauses a download task. Returns the task, whether it is a stream resource, and error.
func (s *DownloadTaskService) PauseTask(task_id int) (*model.DownloadTask, bool, error) {
	if s.db == nil {
		return nil, false, fmt.Errorf("应用未初始化，数据库不可用")
	}

	var task model.DownloadTask
	if err := s.db.Where("id = ?", task_id).First(&task).Error; err != nil {
		return nil, false, fmt.Errorf("下载任务不存在")
	}

	if task.Status != model.TaskStatusPreparing && task.Status != model.TaskStatusDownloading {
		return nil, false, fmt.Errorf("当前状态不允许暂停")
	}

	is_stream := s.has_stream_resources(task.Id)

	if is_stream {
		if err := s.downloader.StopTask(task.Id); err != nil {
			return nil, true, fmt.Errorf("停止直播录制失败: %w", err)
		}
		if err := s.db.Where("id = ?", task_id).First(&task).Error; err != nil {
			return nil, true, fmt.Errorf("读取直播录制最终状态失败: %w", err)
		}
		if task.Status != model.TaskStatusFinished {
			message := strings.TrimSpace(task.ErrorMessage)
			if message == "" {
				message = fmt.Sprintf("收尾后的任务状态异常: %d", task.Status)
			}
			return &task, true, fmt.Errorf("直播录制收尾失败: %s", message)
		}
		return &task, true, nil
	}

	s.downloader.PauseTask(task.Id)
	task.Status = model.TaskStatusPaused
	return &task, false, nil
}

// ResumeTask resumes a download task.
func (s *DownloadTaskService) ResumeTask(task_id int) (*model.DownloadTask, error) {
	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	var task model.DownloadTask
	if err := s.db.Where("id = ?", task_id).First(&task).Error; err != nil {
		return nil, fmt.Errorf("下载任务不存在")
	}

	if task.Status != model.TaskStatusPaused {
		return nil, fmt.Errorf("当前状态不允许恢复")
	}

	if !s.downloader.HasAvailableSlot() {
		return nil, fmt.Errorf("exceeds maximum concurrent download tasks (%d)", s.downloader.MaxConcurrent())
	}

	if err := s.downloader.StartTask(task.Id); err != nil {
		return nil, fmt.Errorf("恢复下载任务失败: %w", err)
	}
	task.Status = model.TaskStatusPreparing

	return &task, nil
}

// DeleteTask deletes a download task and returns the deleted task's record.
func (s *DownloadTaskService) DeleteTask(task_id int) (*DownloadTaskRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	var task model.DownloadTask
	if err := s.db.Where("id = ?", task_id).First(&task).Error; err != nil {
		return nil, fmt.Errorf("下载任务不存在")
	}

	now := time.Now().UnixMilli()

	s.downloader.DeleteTask(task.Id)
	deleted_record, _ := s.BuildTaskRecord(task.Id)

	s.db.Model(&task).Update("deleted_at", now)

	s.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Update("deleted_at", now)

	var resource_ids []int
	s.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Pluck("id", &resource_ids)
	if len(resource_ids) > 0 {
		s.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resource_ids).Update("deleted_at", now)
		s.db.Model(&model.DownloadSegment{}).Where("resource_id IN ?", resource_ids).Update("deleted_at", now)

		var endpoint_ids []int
		s.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resource_ids).Pluck("id", &endpoint_ids)
		if len(endpoint_ids) > 0 {
			s.db.Model(&model.DownloadConnection{}).Where("endpoint_id IN ?", endpoint_ids).Update("deleted_at", now)
		}
	}

	return deleted_record, nil
}

// ListTasks queries the download task list.
func (s *DownloadTaskService) ListTasks(task_id int, page int, page_size int, status_filter string) (*TaskListResult, error) {
	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	// Query single task
	if task_id > 0 {
		record, err := s.BuildTaskRecord(task_id)
		if err != nil {
			return nil, fmt.Errorf("查询下载任务失败: %w", err)
		}
		if record == nil {
			return nil, fmt.Errorf("下载任务不存在")
		}
		return &TaskListResult{
			List:     []DownloadTaskRecord{*record},
			Total:    1,
			Page:     1,
			PageSize: 1,
		}, nil
	}

	if page < 1 {
		page = 1
	}
	if page_size < 1 || page_size > 100 {
		page_size = 20
	}

	var tasks []model.DownloadTask
	var total int64

	query := s.db.Model(&model.DownloadTask{}).Where("deleted_at IS NULL")
	if status_filter != "" {
		parts := strings.Split(status_filter, ",")
		ints := make([]int, 0, len(parts))
		for _, p := range parts {
			if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				ints = append(ints, v)
			}
		}
		if len(ints) == 1 {
			query = query.Where("status = ?", ints[0])
		} else if len(ints) > 1 {
			query = query.Where("status IN ?", ints)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("查询下载任务总数失败: %w", err)
	}
	if err := query.Order("id DESC").Offset((page - 1) * page_size).Limit(page_size).Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("查询下载任务失败: %w", err)
	}

	list, err := s.BuildTaskRecords(tasks)
	if err != nil {
		return nil, fmt.Errorf("构建下载任务记录失败: %w", err)
	}

	return &TaskListResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: page_size,
	}, nil
}

// StartAllTasks starts download tasks in batch.
func (s *DownloadTaskService) StartAllTasks(status string) (int, int, error) {
	if s.db == nil {
		return 0, 0, fmt.Errorf("应用未初始化，数据库不可用")
	}

	query := s.db.Where("deleted_at IS NULL")
	switch status {
	case "waiting":
		query = query.Where("status = ?", model.TaskStatusWaiting)
	case "paused":
		query = query.Where("status = ?", model.TaskStatusPaused)
	case "failed":
		query = query.Where("status = ?", model.TaskStatusFailed)
	default:
		query = query.Where("status IN (?, ?, ?)",
			model.TaskStatusWaiting, model.TaskStatusPaused, model.TaskStatusFailed)
	}

	var tasks []model.DownloadTask
	if err := query.Find(&tasks).Error; err != nil {
		return 0, 0, fmt.Errorf("查询下载任务失败: %w", err)
	}

	var started int
	for _, task := range tasks {
		if err := s.downloader.StartTask(task.Id); err != nil {
			continue
		}
		started++
	}

	return started, len(tasks), nil
}

// PauseAllTasks pauses download tasks in batch. Returns pause count, stream task ID list, and error.
func (s *DownloadTaskService) PauseAllTasks(status string) (int, []int, error) {
	if s.db == nil {
		return 0, nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	query := s.db.Where("deleted_at IS NULL")
	switch status {
	case "preparing":
		query = query.Where("status = ?", model.TaskStatusPreparing)
	case "downloading":
		query = query.Where("status = ?", model.TaskStatusDownloading)
	case "running":
		query = query.Where("status IN (?, ?)",
			model.TaskStatusPreparing, model.TaskStatusDownloading)
	default:
		query = query.Where("status IN (?, ?)",
			model.TaskStatusPreparing, model.TaskStatusDownloading)
	}

	var tasks []model.DownloadTask
	if err := query.Find(&tasks).Error; err != nil {
		return 0, nil, fmt.Errorf("查询下载任务失败: %w", err)
	}

	var paused int
	var stream_task_ids []int
	for _, task := range tasks {
		if s.has_stream_resources(task.Id) {
			if err := s.downloader.StopTask(task.Id); err != nil {
				return paused, stream_task_ids, fmt.Errorf("停止直播录制任务 %d 失败: %w", task.Id, err)
			}
			var stopped_task model.DownloadTask
			if err := s.db.Where("id = ?", task.Id).First(&stopped_task).Error; err != nil {
				return paused, stream_task_ids, fmt.Errorf("读取直播录制任务 %d 最终状态失败: %w", task.Id, err)
			}
			if stopped_task.Status != model.TaskStatusFinished {
				message := strings.TrimSpace(stopped_task.ErrorMessage)
				if message == "" {
					message = fmt.Sprintf("收尾后的任务状态异常: %d", stopped_task.Status)
				}
				return paused, stream_task_ids, fmt.Errorf("直播录制任务 %d 收尾失败: %s", task.Id, message)
			}
			stream_task_ids = append(stream_task_ids, task.Id)
		} else {
			s.downloader.PauseTask(task.Id)
		}
		paused++
	}

	return paused, stream_task_ids, nil
}

// ClearTasks clears completed/failed/cancelled download tasks.
func (s *DownloadTaskService) ClearTasks(delete_files bool) (int, error) {
	if s.db == nil {
		return 0, fmt.Errorf("应用未初始化，数据库不可用")
	}

	var tasks []model.DownloadTask
	if err := s.db.Where("deleted_at IS NULL").
		Where("status IN (?, ?, ?)",
			model.TaskStatusFinished, model.TaskStatusFailed, model.TaskStatusCancelled).
		Find(&tasks).Error; err != nil {
		return 0, fmt.Errorf("查询下载任务失败: %w", err)
	}

	now := time.Now().UnixMilli()
	var cleared int

	for _, task := range tasks {
		s.downloader.DeleteTask(task.Id)

		s.db.Model(&task).Update("deleted_at", now)

		s.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Update("deleted_at", now)

		var resource_ids []int
		s.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Pluck("id", &resource_ids)
		if len(resource_ids) > 0 {
			s.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resource_ids).Update("deleted_at", now)
			s.db.Model(&model.DownloadSegment{}).Where("resource_id IN ?", resource_ids).Update("deleted_at", now)

			var endpoint_ids []int
			s.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resource_ids).Pluck("id", &endpoint_ids)
			if len(endpoint_ids) > 0 {
				s.db.Model(&model.DownloadConnection{}).Where("endpoint_id IN ?", endpoint_ids).Update("deleted_at", now)
			}
		}

		cleared++
	}

	return cleared, nil
}

// BuildTaskRecord builds the DownloadTaskRecord for a single task.
func (s *DownloadTaskService) BuildTaskRecord(task_id int) (*DownloadTaskRecord, error) {
	if s.db == nil {
		return nil, errors.New("数据库不可用")
	}
	var task model.DownloadTask
	if err := s.db.Where("id = ?", task_id).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	records, err := s.BuildTaskRecords([]model.DownloadTask{task})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return &records[0], nil
}

// BuildTaskRecords builds DownloadTaskRecord lists for multiple tasks.
func (s *DownloadTaskService) BuildTaskRecords(tasks []model.DownloadTask) ([]DownloadTaskRecord, error) {
	records := make([]DownloadTaskRecord, 0, len(tasks))
	if len(tasks) == 0 {
		return records, nil
	}

	task_ids := make([]int, len(tasks))
	for i, task := range tasks {
		task_ids[i] = task.Id
	}
	progress_by_task := make(map[int]*hermes.TaskProgress, len(tasks))
	progress_resource_by_task := make(map[int]map[int]hermes.ResourceProgress, len(tasks))
	if s.downloader != nil {
		for _, task := range tasks {
			progress := s.downloader.CurrentProgress(task.Id)
			if progress == nil {
				continue
			}
			progress_by_task[task.Id] = progress
			by_resource := make(map[int]hermes.ResourceProgress, len(progress.Resources))
			for _, resource_progress := range progress.Resources {
				by_resource[resource_progress.ID] = resource_progress
			}
			progress_resource_by_task[task.Id] = by_resource
		}
	}

	type endpointInfo struct {
		TaskID     int    `gorm:"column:task_id"`
		ResourceID int    `gorm:"column:resource_id"`
		URL        string `gorm:"column:url"`
	}
	var endpoints []endpointInfo
	if err := s.db.Raw(`SELECT r.task_id, r.id AS resource_id, e.url FROM download_endpoint e
		JOIN download_resource r ON e.resource_id = r.id
		WHERE r.task_id IN ? AND r.deleted_at IS NULL AND e.deleted_at IS NULL AND e.enabled = 1
		ORDER BY r.task_id ASC, e.priority ASC, e.id ASC`, task_ids).Scan(&endpoints).Error; err != nil {
		return nil, err
	}
	url_by_task := make(map[int]string, len(tasks))
	url_by_resource := make(map[int]string)
	for _, ep := range endpoints {
		if _, exists := url_by_task[ep.TaskID]; !exists {
			url_by_task[ep.TaskID] = ep.URL
		}
		if _, exists := url_by_resource[ep.ResourceID]; !exists {
			url_by_resource[ep.ResourceID] = ep.URL
		}
	}

	type resourceInfo struct {
		ID           int    `gorm:"column:id"`
		TaskID       int    `gorm:"column:task_id"`
		DownloadDir  string `gorm:"column:download_dir"`
		Name         string `gorm:"column:name"`
		Kind         string `gorm:"column:kind"`
		ResourceType string `gorm:"column:type"`
		Size         int64  `gorm:"column:size"`
		Downloaded   int64  `gorm:"column:downloaded"`
		Speed        int64  `gorm:"column:speed"`
		Status       int    `gorm:"column:status"`
		MergeOrder   int    `gorm:"column:merge_order"`
	}
	var resources []resourceInfo
	if err := s.db.Table("download_resource").
		Select("id, task_id, download_dir, name, kind, type, size, downloaded, speed, status, merge_order").
		Where("task_id IN ? AND deleted_at IS NULL", task_ids).
		Order("task_id ASC, merge_order ASC, id ASC").
		Scan(&resources).Error; err != nil {
		return nil, err
	}
	size_by_task := make(map[int]int64, len(tasks))
	downloaded_by_task := make(map[int]int64, len(tasks))
	downloaded_by_resource := make(map[int]int64)
	speed_by_task := make(map[int]int64, len(tasks))
	speed_by_resource := make(map[int]int64)
	resources_by_task := make(map[int][]resourceInfo, len(tasks))
	for _, r := range resources {
		resources_by_task[r.TaskID] = append(resources_by_task[r.TaskID], r)
		if r.Size > 0 {
			size_by_task[r.TaskID] += r.Size
		}
		downloaded := r.Downloaded
		if downloaded <= 0 && r.Status == 2 && r.Size > 0 {
			downloaded = r.Size
		}
		if downloaded > 0 {
			downloaded_by_task[r.TaskID] += downloaded
			downloaded_by_resource[r.ID] = downloaded
		}
		if r.Speed > 0 {
			speed_by_task[r.TaskID] += r.Speed
			speed_by_resource[r.ID] = r.Speed
		}
	}
	for task_id, progress := range progress_by_task {
		if progress.TotalSize > 0 {
			size_by_task[task_id] = progress.TotalSize
		}
		downloaded_by_task[task_id] = progress.Downloaded
		speed_by_task[task_id] = progress.Speed
		for _, resource_progress := range progress.Resources {
			downloaded_by_resource[resource_progress.ID] = resource_progress.Downloaded
			speed_by_resource[resource_progress.ID] = resource_progress.Speed
		}
	}

	type childAggregate struct {
		ParentTaskID int `gorm:"column:parent_task_id"`
		Count        int `gorm:"column:count"`
	}
	var child_aggregates []childAggregate
	if err := s.db.Model(&model.DownloadTask{}).
		Select("parent_task_id, COUNT(*) AS count").
		Where("parent_task_id IN ? AND deleted_at IS NULL", task_ids).
		Group("parent_task_id").
		Scan(&child_aggregates).Error; err != nil {
		return nil, err
	}
	child_count_by_task := make(map[int]int, len(child_aggregates))
	for _, aggregate := range child_aggregates {
		child_count_by_task[aggregate.ParentTaskID] = aggregate.Count
	}

	content_ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.ContentId != nil && *task.ContentId != "" {
			content_ids = append(content_ids, *task.ContentId)
		}
	}
	content_type_by_id := make(map[string]string, len(content_ids))
	if len(content_ids) > 0 {
		type contentTypeRow struct {
			ID   string `gorm:"column:id"`
			Type string `gorm:"column:type"`
		}
		var content_type_rows []contentTypeRow
		if err := s.db.Table("content").
			Select("id, type").
			Where("id IN ?", content_ids).
			Scan(&content_type_rows).Error; err != nil {
			return nil, err
		}
		for _, row := range content_type_rows {
			content_type_by_id[row.ID] = row.Type
		}
	}

	for _, task := range tasks {
		total_size := size_by_task[task.Id]
		error_message := ""
		if task.Status == model.TaskStatusFailed {
			error_message = task.ErrorMessage
		}
		resource_rows := resources_by_task[task.Id]
		progress_resources := progress_resource_by_task[task.Id]
		files := make([]DownloadTaskFileRecord, 0, len(resource_rows))
		for _, r := range resource_rows {
			resource_size := r.Size
			resource_downloaded := downloaded_by_resource[r.ID]
			resource_speed := speed_by_resource[r.ID]
			if resource_progress, ok := progress_resources[r.ID]; ok {
				if resource_progress.Size > resource_size {
					resource_size = resource_progress.Size
				}
				resource_downloaded = resource_progress.Downloaded
				resource_speed = resource_progress.Speed
			}
			file_status := "waiting"
			switch r.Status {
			case 1:
				file_status = "downloading"
			case 2:
				file_status = "finished"
			}
			if resource_size > 0 && resource_downloaded >= resource_size {
				file_status = "finished"
			} else if resource_downloaded > 0 || resource_speed > 0 || task.Status == model.TaskStatusDownloading {
				file_status = "downloading"
			}
			file_error := ""
			if file_status != "finished" {
				switch task.Status {
				case model.TaskStatusFinished:
					file_status = "finished"
				case model.TaskStatusPaused:
					file_status = "paused"
				case model.TaskStatusFailed:
					file_status = "error"
					file_error = error_message
				case model.TaskStatusCancelled:
					file_status = "cancelled"
				}
			}
			files = append(files, DownloadTaskFileRecord{
				ID:          r.ID,
				DownloadDir: r.DownloadDir,
				Name:        r.Name,
				Kind:        r.Kind,
				Type:        r.ResourceType,
				Status:      file_status,
				Size:        resource_size,
				Downloaded:  resource_downloaded,
				Speed:       resource_speed,
				Progress:    TaskProgressPercent(resource_downloaded, resource_size, MapResourceTaskStatus(r.Status)),
				URL:         url_by_resource[r.ID],
				Error:       file_error,
			})
		}
		effective_status := ComputeEffectiveTaskStatus(task.Status, files)
		content_type := ""
		if task.ContentId != nil {
			content_type = content_type_by_id[*task.ContentId]
		}
		records = append(records, DownloadTaskRecord{
			ID:           task.Id,
			ContentID:    task.ContentId,
			ContentType:  content_type,
			ParentTaskID: task.ParentTaskID,
			RootTaskID:   task.RootTaskID,
			RelationType: task.RelationType,
			ChildCount:   child_count_by_task[task.Id],
			Name:         task.Name,
			PlatformID:   task.PlatformId,
			Status:       effective_status,
			SourceURL:    task.SourceURL,
			CoverURL:     task.CoverURL,
			CoverWidth:   task.CoverWidth,
			CoverHeight:  task.CoverHeight,
			ConfigJSON:   task.ConfigJSON,
			MetadataJSON: task.MetadataJSON,
			URL:          url_by_task[task.Id],
			Size:         total_size,
			Downloaded:   downloaded_by_task[task.Id],
			Speed:        speed_by_task[task.Id],
			Progress:     TaskProgressPercent(downloaded_by_task[task.Id], total_size, effective_status),
			Error:        error_message,
			Files:        files,
			FileCount:    len(files),
			CreatedAt:    task.CreatedAt,
			UpdatedAt:    task.UpdatedAt,
		})
	}
	return records, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func save_content_extension(db *gorm.DB, detail any) error {
	if db == nil {
		return ErrDBNotInitialized
	}
	if detail == nil {
		return nil
	}
	switch content_detail := detail.(type) {
	case *model.ContentVideo:
		return save_content_video(db, content_detail)
	case model.ContentVideo:
		content_video := content_detail
		return save_content_video(db, &content_video)
	case *model.ContentNovel:
		return save_content_novel(db, content_detail)
	case model.ContentNovel:
		content_novel := content_detail
		return save_content_novel(db, &content_novel)
	case *model.ContentAlbum:
		return save_content_album(db, content_detail)
	case model.ContentAlbum:
		content_album := content_detail
		return save_content_album(db, &content_album)
	case *model.ContentConversation:
		return save_content_conversation(db, content_detail)
	case model.ContentConversation:
		content_conversation := content_detail
		return save_content_conversation(db, &content_conversation)
	}
	return db.Session(&gorm.Session{FullSaveAssociations: true}).Save(detail).Error
}

func save_content_details(
	db *gorm.DB,
	root_content *model.Content,
	primary_detail any,
	content_details []adapter.ContentDetail,
	now int64,
) error {
	if db == nil {
		return ErrDBNotInitialized
	}
	if len(content_details) == 0 {
		return save_content_extension(db, primary_detail)
	}

	root_content_id := ""
	if root_content != nil {
		root_content_id = strings.TrimSpace(root_content.Id)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for detail_index := range content_details {
			detail := content_details[detail_index]
			related_content := detail.Content
			if related_content == nil {
				continue
			}
			related_content.Id = strings.TrimSpace(related_content.Id)
			if related_content.Id == "" {
				return fmt.Errorf("content detail %q has an empty content id", detail.Key)
			}
			if related_content.Id == root_content_id {
				continue
			}
			normalize_content_taxonomy(related_content)
			if related_content.CreatedAt == 0 {
				related_content.CreatedAt = now
			}
			related_content.UpdatedAt = now
			if err := tx.Save(related_content).Error; err != nil {
				return fmt.Errorf("save related content %q: %w", related_content.Id, err)
			}
		}

		for detail_index := range content_details {
			detail := content_details[detail_index]
			if err := save_content_extension(tx, detail.Data); err != nil {
				return fmt.Errorf("save content detail %q: %w", detail.Key, err)
			}
		}

		for detail_index := range content_details {
			detail := content_details[detail_index]
			content_id := strings.TrimSpace(detail.Key)
			if detail.Content != nil && strings.TrimSpace(detail.Content.Id) != "" {
				content_id = strings.TrimSpace(detail.Content.Id)
			}
			if err := save_content_influencer_references(tx, content_id, detail.Influencers, now); err != nil {
				return fmt.Errorf("save content detail %q influencers: %w", detail.Key, err)
			}
		}

		for detail_index := range content_details {
			detail := content_details[detail_index]
			if detail.Relation == nil {
				continue
			}
			relation := *detail.Relation
			relation.SourceContentId = strings.TrimSpace(relation.SourceContentId)
			relation.TargetContentId = strings.TrimSpace(relation.TargetContentId)
			relation.Type = strings.TrimSpace(relation.Type)
			if relation.SourceContentId == "" || relation.TargetContentId == "" || relation.Type == "" {
				return fmt.Errorf("content detail %q has an incomplete relation", detail.Key)
			}
			if relation.CreatedAt == 0 {
				relation.CreatedAt = now
			}
			if err := tx.Save(&relation).Error; err != nil {
				return fmt.Errorf(
					"save content relation %q --%s--> %q: %w",
					relation.SourceContentId,
					relation.Type,
					relation.TargetContentId,
					err,
				)
			}
		}
		return nil
	})
}

func save_content_influencer_references(
	db *gorm.DB,
	content_id string,
	references []adapter.ContentInfluencerReference,
	now int64,
) error {
	if len(references) == 0 {
		return nil
	}
	content_id = strings.TrimSpace(content_id)
	if content_id == "" {
		return fmt.Errorf("content influencer has an empty content id")
	}
	for reference_index := range references {
		reference := &references[reference_index]
		influencer, err := upsert_content_influencer(db, reference.Influencer, now)
		if err != nil {
			return err
		}
		for role_index := range reference.Roles {
			role := reference.Roles[role_index]
			role.Role = strings.TrimSpace(role.Role)
			if role.Role == "" {
				return fmt.Errorf("influencer %q has an empty role", influencer.Name)
			}
			relation := model.ContentInfluencer{
				ContentId:    content_id,
				InfluencerId: influencer.Id,
				Role:         role.Role,
				SortOrder:    role.SortOrder,
				MetadataJSON: role.MetadataJSON,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			var existing_relation model.ContentInfluencer
			find_err := db.Where(
				"content_id = ? AND influencer_id = ? AND role = ?",
				content_id,
				influencer.Id,
				role.Role,
			).First(&existing_relation).Error
			switch {
			case find_err == nil:
				relation.CreatedAt = existing_relation.CreatedAt
			case !errors.Is(find_err, gorm.ErrRecordNotFound):
				return find_err
			}
			if err := db.Save(&relation).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func upsert_content_influencer(db *gorm.DB, input *model.Influencer, now int64) (*model.Influencer, error) {
	if input == nil {
		return nil, fmt.Errorf("content influencer is nil")
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, fmt.Errorf("content influencer name is empty")
	}

	var persisted model.Influencer
	find_err := gorm.ErrRecordNotFound
	if input.Id > 0 {
		find_err = db.Where("deleted_at IS NULL AND id = ?", input.Id).First(&persisted).Error
	}
	for _, identity := range []struct {
		column string
		value  *string
	}{
		{column: "tmdb_id", value: input.TMDBId},
		{column: "douban_id", value: input.DoubanId},
		{column: "imdb_id", value: input.IMDBId},
	} {
		if !errors.Is(find_err, gorm.ErrRecordNotFound) {
			break
		}
		if identity.value == nil || strings.TrimSpace(*identity.value) == "" {
			continue
		}
		find_err = db.Where("deleted_at IS NULL").
			Where(identity.column+" = ?", strings.TrimSpace(*identity.value)).
			First(&persisted).Error
	}
	if errors.Is(find_err, gorm.ErrRecordNotFound) {
		find_err = db.Where("deleted_at IS NULL AND name = ?", input.Name).Order("id ASC").First(&persisted).Error
	}
	if find_err == nil {
		updates := influencer_profile_updates(&persisted, input, now)
		if len(updates) > 0 {
			if err := db.Model(&persisted).Updates(updates).Error; err != nil {
				return nil, err
			}
			if err := db.Where("id = ?", persisted.Id).First(&persisted).Error; err != nil {
				return nil, err
			}
		}
		return &persisted, nil
	}
	if !errors.Is(find_err, gorm.ErrRecordNotFound) {
		return nil, find_err
	}

	persisted = *input
	persisted.Id = 0
	persisted.CreatedAt = now
	persisted.UpdatedAt = now
	if err := db.Create(&persisted).Error; err != nil {
		return nil, err
	}
	return &persisted, nil
}

func influencer_profile_updates(existing *model.Influencer, input *model.Influencer, now int64) map[string]any {
	updates := map[string]any{"updated_at": now}
	for column, value := range map[string]string{
		"name":           input.Name,
		"alias":          input.Alias,
		"avatar_url":     input.AvatarURL,
		"description":    input.Description,
		"biography":      input.Biography,
		"profile_path":   input.ProfilePath,
		"birthday":       input.Birthday,
		"place_of_birth": input.PlaceOfBirth,
		"profile":        input.Profile,
	} {
		if strings.TrimSpace(value) != "" {
			updates[column] = value
		}
	}
	if existing.KnownForDepartment == "" && strings.TrimSpace(input.KnownForDepartment) != "" {
		updates["known_for_department"] = input.KnownForDepartment
	}
	if existing.MetadataJSON == "" && strings.TrimSpace(input.MetadataJSON) != "" {
		updates["metadata_json"] = input.MetadataJSON
	}
	for column, value := range map[string]*string{
		"tmdb_id":   input.TMDBId,
		"douban_id": input.DoubanId,
		"imdb_id":   input.IMDBId,
	} {
		if value != nil && strings.TrimSpace(*value) != "" {
			updates[column] = strings.TrimSpace(*value)
		}
	}
	return updates
}

func save_content_conversation(db *gorm.DB, content_conversation *model.ContentConversation) error {
	if content_conversation == nil {
		return nil
	}
	content_conversation.Id = strings.TrimSpace(content_conversation.Id)
	if content_conversation.Id == "" {
		return fmt.Errorf("content_conversation.id 不能为空")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		branches := content_conversation.Branches
		messages := content_conversation.Messages
		if content_conversation.MessageCount == 0 {
			content_conversation.MessageCount = len(messages)
		}
		if content_conversation.BranchCount == 0 {
			content_conversation.BranchCount = len(branches)
		}
		if err := tx.Omit("Branches", "Messages").Save(content_conversation).Error; err != nil {
			return err
		}

		for branch_index := range branches {
			branch := &branches[branch_index]
			branch.ConversationId = content_conversation.Id
			if strings.TrimSpace(branch.BranchKey) == "" {
				branch.BranchKey = model.BuildContentConversationBranchKey("", branch.LeafMessageKey, branch_index)
			}
			if branch.SortOrder < 0 {
				branch.SortOrder = branch_index
			}
			var existing_branch model.ContentConversationBranch
			find_err := tx.
				Where("conversation_id = ? AND branch_key = ?", branch.ConversationId, branch.BranchKey).
				First(&existing_branch).Error
			if find_err == nil {
				branch.Id = existing_branch.Id
			} else if !errors.Is(find_err, gorm.ErrRecordNotFound) {
				return find_err
			}
			if err := tx.Save(branch).Error; err != nil {
				return err
			}
		}

		for message_index := range messages {
			message := &messages[message_index]
			parts := message.Parts
			message.ConversationId = content_conversation.Id
			if strings.TrimSpace(message.MessageKey) == "" {
				message.MessageKey = model.BuildContentConversationMessageKey("", message_index)
			}
			if message.Sequence < 0 {
				message.Sequence = message_index
			}
			if strings.TrimSpace(message.Role) == "" {
				message.Role = model.ContentConversationMessageRoleUnknown
			}
			var existing_message model.ContentConversationMessage
			find_err := tx.
				Where("conversation_id = ? AND message_key = ?", message.ConversationId, message.MessageKey).
				First(&existing_message).Error
			if find_err == nil {
				message.Id = existing_message.Id
			} else if !errors.Is(find_err, gorm.ErrRecordNotFound) {
				return find_err
			}
			if err := tx.Omit("Parts", "Assets").Save(message).Error; err != nil {
				return err
			}

			for part_index := range parts {
				part := &parts[part_index]
				part.ConversationId = content_conversation.Id
				part.MessageId = message.Id
				part.MessageKey = message.MessageKey
				if strings.TrimSpace(part.PartKey) == "" {
					part.PartKey = model.BuildContentConversationMessagePartKey("", part_index)
				}
				part.SubjectKey = model.BuildContentConversationMessagePartSubjectKey(message.MessageKey, part.PartKey)
				if part.SortOrder < 0 {
					part.SortOrder = part_index
				}
				if strings.TrimSpace(part.Type) == "" {
					part.Type = model.ContentConversationPartTypeText
				}
				var existing_part model.ContentConversationMessagePart
				find_err := tx.
					Where(
						"conversation_id = ? AND message_key = ? AND part_key = ?",
						part.ConversationId,
						part.MessageKey,
						part.PartKey,
					).
					First(&existing_part).Error
				if find_err == nil {
					part.Id = existing_part.Id
				} else if !errors.Is(find_err, gorm.ErrRecordNotFound) {
					return find_err
				}
				if err := tx.Omit("Assets").Save(part).Error; err != nil {
					return err
				}
			}
			message.Parts = parts
		}

		content_conversation.Branches = branches
		content_conversation.Messages = messages
		return nil
	})
}

func save_content_album(db *gorm.DB, content_album *model.ContentAlbum) error {
	if content_album == nil {
		return nil
	}
	content_album.Id = strings.TrimSpace(content_album.Id)
	if content_album.Id == "" {
		return fmt.Errorf("content_album.id 不能为空")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		images := content_album.Images
		if err := tx.Omit("Images").Save(content_album).Error; err != nil {
			return err
		}
		for image_index := range images {
			content_image := &images[image_index]
			content_image.AlbumId = content_album.Id
			if content_image.SortOrder < 0 {
				content_image.SortOrder = image_index
			}
			if strings.TrimSpace(content_image.ImageKey) == "" {
				content_image.ImageKey = model.BuildContentAlbumImageKey("", content_image.URL, content_image.SortOrder)
			}
			var existing_image model.ContentImage
			find_err := tx.
				Where("album_id = ? AND image_key = ?", content_image.AlbumId, content_image.ImageKey).
				First(&existing_image).Error
			if find_err == nil {
				content_image.Id = existing_image.Id
			} else if !errors.Is(find_err, gorm.ErrRecordNotFound) {
				return find_err
			}
			if err := tx.Omit("Assets").Save(content_image).Error; err != nil {
				return err
			}
		}
		content_album.Images = images
		return nil
	})
}

func save_content_novel(db *gorm.DB, content_novel *model.ContentNovel) error {
	if content_novel == nil {
		return nil
	}
	content_novel.Id = strings.TrimSpace(content_novel.Id)
	if content_novel.Id == "" {
		return fmt.Errorf("content_novel.id 不能为空")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		volumes := content_novel.Volumes
		chapters := content_novel.Chapters
		if err := tx.Omit("Volumes", "Chapters").Save(content_novel).Error; err != nil {
			return err
		}

		volume_id_by_key := make(map[string]uint, len(volumes))
		for volume_index := range volumes {
			volume := &volumes[volume_index]
			volume.NovelId = content_novel.Id
			if volume.Idx <= 0 {
				volume.Idx = volume_index + 1
			}
			if strings.TrimSpace(volume.VolumeKey) == "" {
				volume.VolumeKey = model.BuildContentNovelVolumeKey("", volume.Idx)
			}
			var existing_volume model.ContentNovelVolume
			find_err := tx.
				Where("novel_id = ? AND volume_key = ?", volume.NovelId, volume.VolumeKey).
				First(&existing_volume).Error
			if find_err == nil {
				volume.Id = existing_volume.Id
			} else if !errors.Is(find_err, gorm.ErrRecordNotFound) {
				return find_err
			}
			if err := tx.Save(volume).Error; err != nil {
				return err
			}
			volume_id_by_key[volume.VolumeKey] = volume.Id
		}

		for chapter_index := range chapters {
			chapter := &chapters[chapter_index]
			chapter.NovelId = content_novel.Id
			if chapter.Idx <= 0 {
				chapter.Idx = chapter_index + 1
			}
			if strings.TrimSpace(chapter.ChapterKey) == "" {
				chapter.ChapterKey = model.BuildContentNovelChapterKey("", chapter.URL, chapter.Idx)
			}
			if volume_id, exists := volume_id_by_key[strings.TrimSpace(chapter.VolumeKey)]; exists {
				chapter.VolumeId = &volume_id
			}

			var existing_chapter model.ContentNovelChapter
			find_err := tx.
				Where("novel_id = ? AND chapter_key = ?", chapter.NovelId, chapter.ChapterKey).
				First(&existing_chapter).Error
			if find_err == nil {
				chapter.Id = existing_chapter.Id
			} else if !errors.Is(find_err, gorm.ErrRecordNotFound) {
				return find_err
			}
			if err := tx.Omit("Assets").Save(chapter).Error; err != nil {
				return err
			}
		}

		content_novel.Volumes = volumes
		content_novel.Chapters = chapters
		return nil
	})
}

func save_content_video(db *gorm.DB, content_video *model.ContentVideo) error {
	if content_video == nil {
		return nil
	}
	if strings.TrimSpace(content_video.Id) == "" {
		return fmt.Errorf("content_video.id 不能为空")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		variants := content_video.Variants
		subtitle_tracks := content_video.SubtitleTracks
		if len(subtitle_tracks) == 0 && strings.TrimSpace(content_video.SubtitleURL) != "" {
			subtitle_tracks = []model.ContentVideoSubtitleTrack{{
				TrackKey:     "legacy:und",
				LanguageCode: "und",
				Kind:         model.ContentVideoSubtitleKindSubtitle,
				Sources: []model.ContentVideoSubtitleSource{{
					SourceKey: "default",
					Format:    media_format_from_url(content_video.SubtitleURL),
					URL:       content_video.SubtitleURL,
				}},
			}}
		}
		if len(subtitle_tracks) > 0 {
			content_video.HasSubtitle = 1
		}
		if err := tx.Omit("Variants", "SubtitleTracks").Save(content_video).Error; err != nil {
			return err
		}

		now := time.Now().UnixMilli()
		if len(variants) == 0 {
			variants = []model.ContentVideoVariant{default_content_video_variant(content_video)}
		}
		for variant_index := range variants {
			variant := &variants[variant_index]
			if variant.VideoId == "" {
				variant.VideoId = content_video.Id
			}
			if err := save_content_video_variant(tx, variant, now); err != nil {
				return err
			}
		}

		for track_index := range subtitle_tracks {
			track := &subtitle_tracks[track_index]
			if track.VideoId == "" {
				track.VideoId = content_video.Id
			}
			if err := save_content_video_subtitle_track(tx, track, now); err != nil {
				return err
			}
		}

		content_video.Variants = variants
		content_video.SubtitleTracks = subtitle_tracks
		return nil
	})
}

func default_content_video_variant(content_video *model.ContentVideo) model.ContentVideoVariant {
	variant := model.ContentVideoVariant{
		VideoId:    content_video.Id,
		VariantKey: "default",
		Size:       content_video.Size,
		Codec:      content_video.Codec,
		Format:     content_video.Format,
		StreamType: model.ContentVideoVariantStreamTypeProgressive,
		HasVideo:   1,
		HasAudio:   1,
		IsDefault:  1,
		URL:        content_video.URL,
	}
	variant.Width = positive_int_pointer(content_video.Width)
	variant.Height = positive_int_pointer(content_video.Height)
	variant.FPS = positive_int_pointer(content_video.FPS)
	variant.Bitrate = positive_int_pointer(content_video.Bitrate)
	return variant
}

func positive_int_pointer(value int) *int {
	if value <= 0 {
		return nil
	}
	result := value
	return &result
}

func save_content_video_variant(db *gorm.DB, variant *model.ContentVideoVariant, now int64) error {
	variant.VideoId = strings.TrimSpace(variant.VideoId)
	variant.VariantKey = strings.TrimSpace(variant.VariantKey)
	if variant.VideoId == "" {
		return fmt.Errorf("content_video_variant.video_id 不能为空")
	}
	if variant.VariantKey == "" {
		variant.VariantKey = "default"
	}
	if variant.StreamType == "" {
		variant.StreamType = model.ContentVideoVariantStreamTypeProgressive
	}
	if variant.HasVideo == 0 {
		variant.HasVideo = 1
	}
	if variant.VariantKey == "default" {
		variant.IsDefault = 1
	}

	asset := model.ContentAsset{
		ContentId: variant.VideoId,
		Kind:      model.ContentAssetKindVideo,
		Role:      model.ContentAssetRoleVideoVariant,
		AssetKey:  variant.VariantKey,
		MIMEType:  content_asset_mime_type(model.ContentAssetKindVideo, variant.Format),
		Size:      variant.Size,
		Metadata:  variant.Metadata,
	}
	if err := ensure_content_asset(db, &asset, now); err != nil {
		return err
	}
	variant.AssetId = asset.Id
	if variant.CreatedAt == 0 {
		variant.CreatedAt = now
	}
	variant.UpdatedAt = now
	return db.Omit("Asset").Save(variant).Error
}

func save_content_video_subtitle_track(db *gorm.DB, track *model.ContentVideoSubtitleTrack, now int64) error {
	track.VideoId = strings.TrimSpace(track.VideoId)
	track.TrackKey = strings.TrimSpace(track.TrackKey)
	track.LanguageCode = strings.TrimSpace(track.LanguageCode)
	if track.VideoId == "" {
		return fmt.Errorf("content_video_subtitle_track.video_id 不能为空")
	}
	if track.LanguageCode == "" {
		track.LanguageCode = "und"
	}
	if track.Kind == "" {
		track.Kind = model.ContentVideoSubtitleKindSubtitle
	}
	if track.TrackKey == "" {
		track.TrackKey = track.LanguageCode + ":" + track.Kind
	}

	sources := track.Sources
	var existing_track model.ContentVideoSubtitleTrack
	find_err := db.
		Where("video_id = ? AND track_key = ?", track.VideoId, track.TrackKey).
		First(&existing_track).Error
	if find_err == nil {
		track.Id = existing_track.Id
	} else if !errors.Is(find_err, gorm.ErrRecordNotFound) {
		return find_err
	}
	if track.CreatedAt == 0 {
		track.CreatedAt = now
	}
	track.UpdatedAt = now
	if err := db.Omit("Sources").Save(track).Error; err != nil {
		return err
	}

	for source_index := range sources {
		source := &sources[source_index]
		source.TrackId = track.Id
		if err := save_content_video_subtitle_source(db, track, source, now); err != nil {
			return err
		}
	}
	track.Sources = sources
	return nil
}

func save_content_video_subtitle_source(
	db *gorm.DB,
	track *model.ContentVideoSubtitleTrack,
	source *model.ContentVideoSubtitleSource,
	now int64,
) error {
	source.SourceKey = strings.TrimSpace(source.SourceKey)
	if source.SourceKey == "" {
		source.SourceKey = strings.ToLower(strings.TrimSpace(source.Format))
	}
	if source.SourceKey == "" {
		source.SourceKey = "default"
	}
	asset_key := model.BuildContentVideoSubtitleAssetKey(track.TrackKey, source.SourceKey)
	asset := model.ContentAsset{
		ContentId:    track.VideoId,
		Kind:         model.ContentAssetKindText,
		Role:         model.ContentAssetRoleSubtitle,
		AssetKey:     asset_key,
		LanguageCode: track.LanguageCode,
		MIMEType:     content_asset_mime_type(model.ContentAssetKindText, source.Format),
		Metadata:     source.Metadata,
	}
	if err := ensure_content_asset(db, &asset, now); err != nil {
		return err
	}
	source.AssetId = asset.Id
	if source.CreatedAt == 0 {
		source.CreatedAt = now
	}
	source.UpdatedAt = now
	return db.Omit("Asset").Save(source).Error
}

func ensure_content_asset(db *gorm.DB, asset *model.ContentAsset, now int64) error {
	asset.ContentId = strings.TrimSpace(asset.ContentId)
	asset.Kind = strings.TrimSpace(asset.Kind)
	asset.Role = strings.TrimSpace(asset.Role)
	asset.AssetKey = strings.TrimSpace(asset.AssetKey)
	if asset.ContentId == "" || asset.Kind == "" || asset.Role == "" || asset.AssetKey == "" {
		return fmt.Errorf(
			"内容资产 identity 不完整: content_id=%q kind=%q role=%q asset_key=%q",
			asset.ContentId,
			asset.Kind,
			asset.Role,
			asset.AssetKey,
		)
	}

	var existing_asset model.ContentAsset
	find_err := db.
		Where("content_id = ? AND role = ? AND asset_key = ?", asset.ContentId, asset.Role, asset.AssetKey).
		First(&existing_asset).Error
	if find_err == nil {
		asset.Id = existing_asset.Id
		updates := make(map[string]any)
		if asset.MIMEType != "" && asset.MIMEType != existing_asset.MIMEType {
			updates["mime_type"] = asset.MIMEType
		}
		if asset.Size > 0 && asset.Size != existing_asset.Size {
			updates["size"] = asset.Size
		}
		if asset.Metadata != "" && asset.Metadata != existing_asset.Metadata {
			updates["metadata"] = asset.Metadata
		}
		if asset.Kind != "" && asset.Kind != existing_asset.Kind {
			updates["kind"] = asset.Kind
		}
		if asset.Label != "" && asset.Label != existing_asset.Label {
			updates["label"] = asset.Label
		}
		if asset.LanguageCode != "" && asset.LanguageCode != existing_asset.LanguageCode {
			updates["language_code"] = asset.LanguageCode
		}
		if existing_asset.DeletedAt != nil && asset.DeletedAt == nil {
			updates["deleted_at"] = nil
		}
		if len(updates) > 0 {
			updates["updated_at"] = now
			if err := db.Model(&existing_asset).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	}
	if !errors.Is(find_err, gorm.ErrRecordNotFound) {
		return find_err
	}
	if asset.CreatedAt == 0 {
		asset.CreatedAt = now
	}
	asset.UpdatedAt = now
	return db.Omit("DownloadResources").Create(asset).Error
}

func save_download_resource_assets(
	db *gorm.DB,
	resource *model.DownloadResource,
	resource_info *adapter.ResourceInfo,
	content *model.Content,
	content_detail any,
	now int64,
) error {
	content_id := ""
	if resource.ContentId != nil {
		content_id = strings.TrimSpace(*resource.ContentId)
	}
	if content_id == "" && content != nil {
		content_id = strings.TrimSpace(content.Id)
	}
	if content_id == "" {
		return nil
	}

	content_asset_references := resource_info.ContentAssets
	if len(content_asset_references) == 0 {
		content_asset_references = []adapter.ContentAssetReference{default_content_asset_reference(*resource, content)}
	}
	for reference_index := range content_asset_references {
		reference := content_asset_references[reference_index]
		reference.Kind = strings.TrimSpace(reference.Kind)
		reference.Role = strings.TrimSpace(reference.Role)
		reference.AssetKey = strings.TrimSpace(reference.AssetKey)
		reference.Relation = strings.TrimSpace(reference.Relation)
		if reference.Kind == "" || reference.Role == "" {
			inferred_kind, inferred_role := content_asset_kind_and_role_from_resource(*resource)
			if reference.Kind == "" {
				reference.Kind = inferred_kind
			}
			if reference.Role == "" {
				reference.Role = inferred_role
			}
		}
		if reference.AssetKey == "" {
			reference.AssetKey = content_asset_key_from_resource(*resource, reference.Role)
		}
		if reference.Relation == "" {
			reference.Relation = model.DownloadResourceAssetRelationSource
		}

		asset := model.ContentAsset{
			ContentId: content_id,
			Kind:      reference.Kind,
			Role:      reference.Role,
			AssetKey:  reference.AssetKey,
			MIMEType:  resource_mime_type(*resource),
			Size:      resource.Size,
		}
		if err := ensure_content_asset(db, &asset, now); err != nil {
			return err
		}
		if asset.Role == model.ContentAssetRoleVideoVariant {
			if err := ensure_resource_video_variant(db, &asset, resource_info, content_detail, now); err != nil {
				return err
			}
		}
		if err := save_content_asset_subject_link(db, &asset, reference, now); err != nil {
			return err
		}

		resource_asset := model.DownloadResourceAsset{
			ResourceId: resource.Id,
			AssetId:    asset.Id,
			Relation:   reference.Relation,
			CreatedAt:  now,
		}
		if err := db.FirstOrCreate(
			&resource_asset,
			model.DownloadResourceAsset{
				ResourceId: resource_asset.ResourceId,
				AssetId:    resource_asset.AssetId,
				Relation:   resource_asset.Relation,
			},
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensure_resource_video_variant(
	db *gorm.DB,
	asset *model.ContentAsset,
	resource_info *adapter.ResourceInfo,
	content_detail any,
	now int64,
) error {
	var existing_variant model.ContentVideoVariant
	find_err := db.Where("asset_id = ?", asset.Id).First(&existing_variant).Error
	if find_err == nil {
		return nil
	}
	if !errors.Is(find_err, gorm.ErrRecordNotFound) {
		return find_err
	}

	var content_video model.ContentVideo
	if err := db.Where("id = ?", asset.ContentId).First(&content_video).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	variant := model.ContentVideoVariant{
		AssetId:    asset.Id,
		VideoId:    asset.ContentId,
		VariantKey: asset.AssetKey,
		Size:       resource_info.Resource.Size,
		Format:     media_format_from_resource(resource_info.Resource),
		StreamType: model.ContentVideoVariantStreamTypeProgressive,
		HasVideo:   1,
		HasAudio:   1,
		URL:        first_resource_endpoint_url(resource_info),
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	if asset.AssetKey == "default" {
		variant.IsDefault = 1
	} else {
		variant.Spec = asset.AssetKey
	}
	if video_detail, ok := content_detail.(*model.ContentVideo); ok && asset.AssetKey == "default" {
		default_variant := default_content_video_variant(video_detail)
		default_variant.AssetId = asset.Id
		default_variant.CreatedAt = now
		default_variant.UpdatedAt = now
		if variant.URL != "" {
			default_variant.URL = variant.URL
		}
		if variant.Size > 0 {
			default_variant.Size = variant.Size
		}
		variant = default_variant
	}
	return db.Omit("Asset").Create(&variant).Error
}

func save_content_asset_subject_link(
	db *gorm.DB,
	asset *model.ContentAsset,
	reference adapter.ContentAssetReference,
	now int64,
) error {
	subject_type := strings.TrimSpace(reference.SubjectType)
	subject_key := strings.TrimSpace(reference.SubjectKey)
	if subject_type == "" && subject_key == "" {
		return nil
	}
	if subject_type == "" || subject_key == "" {
		return fmt.Errorf("内容资产 subject 不完整: type=%q key=%q", subject_type, subject_key)
	}

	switch subject_type {
	case model.ContentAssetSubjectNovelChapter:
		var chapter model.ContentNovelChapter
		if err := db.
			Where("novel_id = ? AND chapter_key = ?", asset.ContentId, subject_key).
			First(&chapter).Error; err != nil {
			return fmt.Errorf("查找小说章节 %q 失败: %w", subject_key, err)
		}
	case model.ContentAssetSubjectAlbumImage:
		var content_image model.ContentImage
		if err := db.
			Where("album_id = ? AND image_key = ?", asset.ContentId, subject_key).
			First(&content_image).Error; err != nil {
			return fmt.Errorf("查找图集图片 %q 失败: %w", subject_key, err)
		}
	case model.ContentAssetSubjectConversationMessage:
		var message model.ContentConversationMessage
		if err := db.
			Where("conversation_id = ? AND message_key = ?", asset.ContentId, subject_key).
			First(&message).Error; err != nil {
			return fmt.Errorf("查找对话消息 %q 失败: %w", subject_key, err)
		}
	case model.ContentAssetSubjectConversationMessagePart:
		var message_part model.ContentConversationMessagePart
		if err := db.
			Where("conversation_id = ? AND subject_key = ?", asset.ContentId, subject_key).
			First(&message_part).Error; err != nil {
			return fmt.Errorf("查找对话消息内容块 %q 失败: %w", subject_key, err)
		}
	default:
		// Other subject types are forward-compatible. Their owning adapter is
		// responsible for supplying a stable subject key.
	}

	subject_relation := strings.TrimSpace(reference.SubjectRelation)
	if subject_relation == "" {
		subject_relation = model.ContentAssetSubjectRelationRepresentation
	}
	asset_link := model.ContentAssetLink{
		ContentId:   asset.ContentId,
		SubjectType: subject_type,
		SubjectKey:  subject_key,
		AssetId:     asset.Id,
		Relation:    subject_relation,
		CreatedAt:   now,
	}
	return db.FirstOrCreate(
		&asset_link,
		model.ContentAssetLink{
			ContentId:   asset_link.ContentId,
			SubjectType: asset_link.SubjectType,
			SubjectKey:  asset_link.SubjectKey,
			AssetId:     asset_link.AssetId,
			Relation:    asset_link.Relation,
		},
	).Error
}

func default_content_asset_reference(resource model.DownloadResource, content *model.Content) adapter.ContentAssetReference {
	asset_kind, asset_role := content_asset_kind_and_role_from_resource(resource)
	reference := adapter.ContentAssetReference{
		Kind:     asset_kind,
		Role:     asset_role,
		AssetKey: content_asset_key_from_resource(resource, asset_role),
		Relation: model.DownloadResourceAssetRelationSource,
	}
	var extra struct {
		ChapterIndex int    `json:"chapter_index"`
		SourceURL    string `json:"source_url"`
	}
	if json.Unmarshal([]byte(resource.Extra), &extra) == nil && extra.ChapterIndex > 0 {
		chapter_key := model.BuildContentNovelChapterKey("", extra.SourceURL, extra.ChapterIndex)
		representation_key := media_format_from_resource(resource)
		if representation_key == "" {
			representation_key = "default"
		}
		reference.Role = model.ContentAssetRoleNovelChapter
		reference.AssetKey = model.BuildContentNovelChapterAssetKey(chapter_key, representation_key)
		reference.SubjectType = model.ContentAssetSubjectNovelChapter
		reference.SubjectKey = chapter_key
		reference.SubjectRelation = model.ContentAssetSubjectRelationRepresentation
	} else if content != nil {
		switch strings.ToLower(strings.TrimSpace(content.Type)) {
		case "article", "blog":
			if asset_kind == model.ContentAssetKindText || asset_kind == model.ContentAssetKindDocument {
				reference.Role = model.ContentAssetRoleArticleBody
			}
		case "novel":
			if asset_kind == model.ContentAssetKindText || asset_kind == model.ContentAssetKindDocument || asset_kind == model.ContentAssetKindArchive {
				reference.Role = model.ContentAssetRoleNovelBook
			}
		case "conversation":
			reference.Role = model.ContentAssetRoleConversationExport
		}
	}
	return reference
}

func content_asset_kind_and_role_from_resource(resource model.DownloadResource) (string, string) {
	kind := strings.ToLower(strings.TrimSpace(resource.Kind))
	name := strings.ToLower(strings.TrimSpace(resource.Name))
	unique_id := strings.ToLower(strings.TrimSpace(resource.UniqueID))
	switch {
	case kind == "video" || strings.HasPrefix(kind, "video/"):
		return model.ContentAssetKindVideo, model.ContentAssetRoleVideoVariant
	case kind == "audio" || strings.HasPrefix(kind, "audio/"):
		return model.ContentAssetKindAudio, model.ContentAssetRoleAudioVariant
	case strings.Contains(kind, "subtitle"),
		strings.HasSuffix(name, ".srt"),
		strings.HasSuffix(name, ".vtt"),
		strings.HasSuffix(name, ".ass"):
		return model.ContentAssetKindText, model.ContentAssetRoleSubtitle
	case strings.HasSuffix(unique_id, "_cover"):
		return model.ContentAssetKindImage, model.ContentAssetRoleCover
	case kind == "image" || strings.HasPrefix(kind, "image/"):
		return model.ContentAssetKindImage, model.ContentAssetRolePrimary
	case kind == "text" || strings.HasPrefix(kind, "text/") || kind == "html":
		return model.ContentAssetKindText, model.ContentAssetRolePrimary
	case kind == "application/json" || strings.HasSuffix(name, ".json"):
		return model.ContentAssetKindData, model.ContentAssetRoleAttachment
	case kind == "application/pdf" || strings.HasSuffix(name, ".pdf"):
		return model.ContentAssetKindDocument, model.ContentAssetRoleAttachment
	case kind == "application/zip" || kind == "application/x-zip-compressed" || strings.HasSuffix(name, ".zip"):
		return model.ContentAssetKindArchive, model.ContentAssetRoleAttachment
	default:
		return model.ContentAssetKindBinary, model.ContentAssetRoleAttachment
	}
}

func content_asset_key_from_resource(resource model.DownloadResource, asset_role string) string {
	if asset_role == model.ContentAssetRoleVideoVariant {
		var extra struct {
			Spec string `json:"spec"`
		}
		if json.Unmarshal([]byte(resource.Extra), &extra) == nil {
			spec := strings.TrimSpace(extra.Spec)
			if spec != "" && spec != "original" {
				return spec
			}
		}
		return "default"
	}
	if unique_id := strings.TrimSpace(resource.UniqueID); unique_id != "" {
		return unique_id
	}
	if name := strings.TrimSpace(resource.Name); name != "" {
		return name
	}
	return "resource:" + strconv.Itoa(resource.Id)
}

func resource_mime_type(resource model.DownloadResource) string {
	kind := strings.ToLower(strings.TrimSpace(resource.Kind))
	if strings.Contains(kind, "/") {
		return kind
	}
	format := media_format_from_resource(resource)
	asset_kind, _ := content_asset_kind_and_role_from_resource(resource)
	return content_asset_mime_type(asset_kind, format)
}

func media_format_from_resource(resource model.DownloadResource) string {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(resource.Name)), ".")
	if extension != "" {
		return extension
	}
	kind := strings.ToLower(strings.TrimSpace(resource.Kind))
	if separator_index := strings.Index(kind, "/"); separator_index >= 0 && separator_index+1 < len(kind) {
		return kind[separator_index+1:]
	}
	return ""
}

func media_format_from_url(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(parsed_url.Path)), ".")
}

func content_asset_mime_type(asset_kind string, format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return ""
	}
	switch asset_kind {
	case model.ContentAssetKindVideo:
		return "video/" + format
	case model.ContentAssetKindAudio:
		return "audio/" + format
	case model.ContentAssetKindText:
		switch format {
		case "vtt":
			return "text/vtt"
		case "srt":
			return "application/x-subrip"
		case "ass", "ssa":
			return "text/x-ssa"
		}
	}
	return ""
}

func first_resource_endpoint_url(resource_info *adapter.ResourceInfo) string {
	if resource_info == nil || len(resource_info.Endpoints) == 0 {
		return ""
	}
	return resource_info.Endpoints[0].URL
}

func (s *DownloadTaskService) resolve_save_dir(requested string) (string, error) {
	download_dir := strings.TrimSpace(requested)
	if download_dir == "" {
		download_dir = strings.TrimSpace(s.download_dir)
	}
	if download_dir == "" {
		return "", fmt.Errorf("下载目录不能为空")
	}

	work_dir := s.work_dir
	if work_dir == "" {
		var err error
		work_dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("获取工作目录失败: %w", err)
		}
	}

	download_dir = strings.ReplaceAll(download_dir, "%UserDownloads%", xdg.UserDirs.Download)
	download_dir = strings.ReplaceAll(download_dir, "%CWD%", work_dir)
	download_dir = filepath.Clean(download_dir)
	if !filepath.IsAbs(download_dir) {
		download_dir = filepath.Join(work_dir, download_dir)
	}

	if err := os.MkdirAll(download_dir, 0755); err != nil {
		return "", fmt.Errorf("创建下载目录 %q 失败: %w", download_dir, err)
	}

	return download_dir, nil
}

func (s *DownloadTaskService) check_duplicate(save_dir string, task_unique_id string, resource_keys []string, resource_names []string, duplicate bool, overwrite bool) error {
	if duplicate {
		return nil
	}

	var conflicts []duplicateConflict
	var existing_task_id int
	var existing_task_name string

	// Task-level duplicate check: any existing task with the same unique_id (regardless of status)
	if task_unique_id != "" {
		var existing_task model.DownloadTask
		err := s.db.Where("unique_id = ? AND deleted_at IS NULL", task_unique_id).First(&existing_task).Error
		if err == nil {
			existing_task_id = existing_task.Id
			existing_task_name = existing_task.Name
			s.logger.Warn().
				Int("existing_task_id", existing_task.Id).
				Str("existing_task_unique_id", existing_task.UniqueID).
				Str("incoming_task_unique_id", task_unique_id).
				Msg("checkDuplicate: task-level duplicate found")
			conflicts = append(conflicts, duplicateConflict{
				Type:        "task",
				TaskID:      existing_task.Id,
				ResourceKey: task_unique_id,
			})
		}
	}

	for i, key := range resource_keys {
		if key == "" {
			continue
		}
		var dup model.DownloadResource
		err := s.db.
			Joins("JOIN download_task ON download_task.id = download_resource.task_id").
			Where("download_resource.unique_id = ?", key).
			Where("download_task.deleted_at IS NULL").
			First(&dup).Error
		if err == nil {
			if dup.TaskId == nil {
				continue
			}
			existing_task_id = *dup.TaskId
			if existing_task_name == "" && *dup.TaskId > 0 {
				var task model.DownloadTask
				if task_err := s.db.
					Select("id", "name").
					Where("id = ? AND deleted_at IS NULL", *dup.TaskId).
					First(&task).Error; task_err == nil {
					existing_task_name = task.Name
				}
			}
			s.logger.Warn().
				Int("existing_task_id", *dup.TaskId).
				Str("existing_resource_unique_id", dup.UniqueID).
				Str("incoming_resource_unique_id", key).
				Str("resource_name", resource_names[i]).
				Msg("checkDuplicate: resource-level duplicate found")
			conflicts = append(conflicts, duplicateConflict{
				Type:        "resource",
				TaskID:      *dup.TaskId,
				ResourceKey: resource_names[i],
			})
		}
	}

	for _, name := range resource_names {
		file_path := filepath.Join(save_dir, filepath.Base(name))
		if file_info, err := os.Stat(file_path); err == nil && !file_info.IsDir() {
			if existing_task_name == "" {
				existing_task_name = filepath.Base(file_path)
			}
			conflicts = append(conflicts, duplicateConflict{
				Type:     "file",
				FilePath: file_path,
			})
		}
	}

	if len(conflicts) == 0 {
		return nil
	}

	if overwrite {
		for _, conflict := range conflicts {
			switch conflict.Type {
			case "task", "resource":
				if err := s.delete_task_with_files(conflict.TaskID); err != nil {
					return fmt.Errorf("覆盖已存在任务失败: %w", err)
				}
			case "file":
				if err := os.Remove(conflict.FilePath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("覆盖已存在文件失败: %w", err)
				}
			}
		}
		return nil
	}

	err_resp := &DuplicateTaskError{
		IncomingUniqueID: task_unique_id,
	}
	if existing_task_id > 0 {
		err_resp.ExistingTaskID = existing_task_id
	}
	if existing_task_name != "" {
		err_resp.ExistingTaskName = existing_task_name
	}
	s.logger.Warn().
		Int("existing_task_id", existing_task_id).
		Str("existing_task_name", existing_task_name).
		Str("incoming_task_unique_id", task_unique_id).
		Strs("incoming_resource_unique_ids", resource_keys).
		Msg("download task conflict detected")
	return err_resp
}

func (s *DownloadTaskService) delete_task_with_files(task_id int) error {
	var task model.DownloadTask
	if err := s.db.First(&task, task_id).Error; err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	return s.db.Model(&task).Updates(map[string]any{
		"deleted_at": time.Now().UnixMilli(),
	}).Error
}

func (s *DownloadTaskService) start_created_download_task(task_id int) error {
	if s.downloader == nil {
		s.logger.Error().Int("task_id", task_id).Msg("Hermes downloader not initialized, unable to start download task")
		return fmt.Errorf("Hermes 下载器未初始化")
	}
	s.logger.Info().Int("task_id", task_id).Msg("submitting download task to Hermes scheduler")
	if err := s.downloader.StartCreatedTask(task_id); err != nil {
		s.logger.Error().Int("task_id", task_id).Err(err).Msg("Hermes scheduler failed to start download task")
		return err
	}
	s.logger.Info().Int("task_id", task_id).Msg("download task submitted to Hermes schedule queue")
	return nil
}

func (s *DownloadTaskService) has_stream_resources(task_id int) bool {
	if s.db == nil {
		return false
	}
	var count int64
	s.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND UPPER(type) = ?", task_id, model.ResourceTypeStream).
		Count(&count)
	return count > 0
}

func (s *DownloadTaskService) build_task_input(info *adapter.DownloadTaskResult, task_name, task_download_dir, filename string, body_cfg map[string]any) *hermes.TaskInput {
	task_info := hermes.TaskInfo{
		Name:        task_name,
		DownloadDir: task_download_dir,
	}

	resources := make([]hermes.ResourceInfo, 0, len(info.Resources))
	for _, ri := range info.Resources {
		endpoints := make([]hermes.EndpointInfo, 0, len(ri.Endpoints))
		for _, ep := range ri.Endpoints {
			endpoints = append(endpoints, hermes.EndpointInfo{
				Protocol: ep.Protocol,
				URL:      ep.URL,
			})
		}
		resources = append(resources, hermes.ResourceInfo{
			ID:        ri.Resource.Id,
			Name:      ri.Resource.Name,
			Kind:      ri.Resource.Kind,
			Size:      ri.Resource.Size,
			UniqueID:  ri.Resource.UniqueID,
			Endpoints: endpoints,
		})
	}

	config := make(map[string]any, len(body_cfg)+2)
	for key, value := range body_cfg {
		config[key] = value
	}
	config["download_dir"] = task_download_dir
	if filename != "" {
		config["filename"] = filename
	}

	if info.Task.ConfigJSON != "" {
		var task_cfg map[string]any
		if json.Unmarshal([]byte(info.Task.ConfigJSON), &task_cfg) == nil {
			for k, v := range task_cfg {
				if _, exists := config[k]; !exists {
					config[k] = v
				}
			}
		}
	}
	if info.Task.MetadataJSON != "" {
		var meta map[string]any
		if json.Unmarshal([]byte(info.Task.MetadataJSON), &meta) == nil {
			for k, v := range meta {
				if _, exists := config[k]; !exists {
					config[k] = v
				}
			}
		}
	}

	metadata := make(map[string]any)
	if info.Task.MetadataJSON != "" {
		json.Unmarshal([]byte(info.Task.MetadataJSON), &metadata)
	}

	return &hermes.TaskInput{
		Task:      task_info,
		Config:    config,
		Metadata:  metadata,
		Resources: resources,
	}
}

func (s *DownloadTaskService) apply_task_input_modifications(info *adapter.DownloadTaskResult, task_name, task_download_dir string, modified *hermes.TaskInput) (string, string) {
	if modified == nil {
		return task_name, task_download_dir
	}

	if modified.Task.Name != "" {
		task_name = modified.Task.Name
	}
	if modified.Task.DownloadDir != "" {
		task_download_dir = modified.Task.DownloadDir
	}

	for i, mod_res := range modified.Resources {
		if i >= len(info.Resources) {
			break
		}
		if mod_res.Name != "" {
			info.Resources[i].Resource.Name = mod_res.Name
		}
	}

	return task_name, task_download_dir
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// build_resource_tree splits a flat resource list into a directory tree based on paths.
func build_resource_tree(resources []ResourceDetail) *ResourceTreeNode {
	root := &ResourceTreeNode{Name: "", Type: "directory", Children: []*ResourceTreeNode{}}
	for _, r := range resources {
		parts := strings.Split(r.Name, "/")

		node := root
		for i, part := range parts {
			if i == len(parts)-1 {
				node.Children = append(node.Children, &ResourceTreeNode{
					Name:      part,
					Type:      "file",
					Kind:      r.Kind,
					Endpoints: r.Endpoints,
				})
			} else {
				var dir *ResourceTreeNode
				for _, child := range node.Children {
					if child.Type == "directory" && child.Name == part {
						dir = child
						break
					}
				}
				if dir == nil {
					dir = &ResourceTreeNode{
						Name:     part,
						Type:     "directory",
						Children: []*ResourceTreeNode{},
					}
					node.Children = append(node.Children, dir)
				}
				node = dir
			}
		}
	}
	return root
}

// ---------------------------------------------------------------------------
// internal helpers (package-level)
// ---------------------------------------------------------------------------

// TaskProgressPercent calculates the completion percentage of a download task.
func TaskProgressPercent(downloaded, total int64, status int) float64 {
	if status == model.TaskStatusFinished {
		return 100
	}
	if downloaded <= 0 || total <= 0 {
		return 0
	}
	percent := float64(downloaded) * 100 / float64(total)
	if percent >= 100 {
		return 100
	}
	return math.Round(percent*100) / 100
}

// MapResourceTaskStatus maps a download resource status to a task status.
func MapResourceTaskStatus(status int) int {
	if status == 2 {
		return model.TaskStatusFinished
	}
	return model.TaskStatusDownloading
}

// ComputeEffectiveTaskStatus derives the effective task status from the database status and file states.
func ComputeEffectiveTaskStatus(db_status int, files []DownloadTaskFileRecord) int {
	switch db_status {
	case model.TaskStatusPaused, model.TaskStatusFinished, model.TaskStatusFailed,
		model.TaskStatusCancelled, model.TaskStatusMerging:
		return db_status
	}
	if len(files) == 0 {
		return db_status
	}
	all_finished := true
	has_downloading := false
	for _, f := range files {
		switch f.Status {
		case "finished":
		case "downloading":
			has_downloading = true
			all_finished = false
		default:
			all_finished = false
		}
	}
	if all_finished {
		return model.TaskStatusFinished
	}
	if has_downloading {
		return model.TaskStatusDownloading
	}
	return model.TaskStatusWaiting
}
