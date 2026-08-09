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
	db          *gorm.DB
	logger      *zerolog.Logger
	downloader  *hermes.HermesEngine
	hookManager *hermes.HookManager
	workDir     string
	downloadDir string
}

func NewDownloadTaskService(
	db *gorm.DB,
	logger *zerolog.Logger,
	downloader *hermes.HermesEngine,
	hookManager *hermes.HookManager,
	workDir string,
	downloadDir string,
) *DownloadTaskService {
	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}
	return &DownloadTaskService{
		db:          db,
		logger:      logger,
		downloader:  downloader,
		hookManager: hookManager,
		workDir:     workDir,
		downloadDir: downloadDir,
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
	SavePath     string          `json:"save_path"`
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
	SavePath     string         `json:"save_path"`
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

func buildPlatformConfigJSON(config map[string]any, savePath, filename string) ([]byte, error) {
	platformConfig := make(map[string]any, len(config)+2)
	for key, value := range config {
		platformConfig[key] = value
	}
	platformConfig["save_path"] = savePath
	if filename != "" {
		platformConfig["filename"] = filename
	}
	return json.Marshal(platformConfig)
}

func downloadConfigBool(config map[string]any, key string) bool {
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
	SavePath      string            `json:"save_path"`
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
	SavePath      string           `json:"save_path"`
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
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	Size       int64   `json:"size"`
	Downloaded int64   `json:"downloaded"`
	Speed      int64   `json:"speed"`
	Progress   float64 `json:"progress"`
	URL        string  `json:"url"`
	OutputPath string  `json:"output_path"`
	Error      string  `json:"error"`
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

	saveDir, err := s.resolveSaveDir(body.SavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}

	configJSON, err := buildPlatformConfigJSON(body.Config, saveDir, body.Filename)
	if err != nil {
		return nil, fmt.Errorf("构建下载配置失败: %w", err)
	}

	info, err := h.BuildDownloadTask(body.Content, json.RawMessage(configJSON))
	if err != nil {
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	resourceInfos := info.Resources
	for _, ri := range resourceInfos {
		if len(ri.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 %s 没有下载端点", ri.Name)
		}
	}

	resources := make([]ResourceDetail, 0, len(resourceInfos))
	totalEndpoints := 0
	for i, ri := range resourceInfos {
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
			Name:      ri.Name,
			Kind:      ri.Kind,
			Endpoints: eps,
		})
		totalEndpoints += len(ri.Endpoints)
	}
	tree := buildResourceTree(resources)

	return &PrepareTaskResult{
		Platform:      body.Platform,
		TaskName:      info.Task.Name,
		SavePath:      saveDir,
		Resources:     resources,
		Tree:          tree,
		ResourceCount: len(resourceInfos),
		EndpointCount: totalEndpoints,
		Content:       info.Content,
		Account:       info.Account,
	}, nil
}

// PrepareTaskByURL previews a download task created via resource URL (no DB writes, no download start).
func (s *DownloadTaskService) PrepareTaskByURL(body CreateDownloadTaskByURLBody) (*PrepareTaskByURLResult, error) {
	if body.URL == "" {
		return nil, fmt.Errorf("url 不能为空")
	}

	parsedURL, err := url.Parse(body.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("无效的下载地址")
	}

	protocol := strings.ToUpper(parsedURL.Scheme)

	requestedSavePath := body.SavePath
	if requestedSavePath == "" {
		requestedSavePath, _ = body.Config["save_path"].(string)
	}
	saveDir, err := s.resolveSaveDir(requestedSavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}
	filename := body.Filename
	if filename == "" {
		filename, _ = body.Config["filename"].(string)
	}
	if filename == "" {
		base := filepath.Base(parsedURL.Path)
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

	savePath := saveDir

	return &PrepareTaskByURLResult{
		URL:      body.URL,
		Protocol: protocol,
		TaskName: filename,
		SavePath: savePath,
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
func (s *DownloadTaskService) CreateTask(body CreateDownloadTaskBody) (result *CreateTaskResult, retErr error) {
	s.logger.Info().Str("platform", body.Platform).Msg("start processing single download task creation request")

	var task model.DownloadTask
	defer func() {
		if retErr != nil && task.Id > 0 {
			s.db.Model(&task).Updates(map[string]any{
				"status":        model.TaskStatusFailed,
				"error_message": retErr.Error(),
			})
			s.logger.Warn().Int("task_id", task.Id).Err(retErr).Msg("subsequent steps after task creation failed, task marked as failed")
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

	saveDir, err := s.resolveSaveDir(body.SavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}

	configJSON, err := buildPlatformConfigJSON(body.Config, saveDir, body.Filename)
	if err != nil {
		return nil, fmt.Errorf("构建下载配置失败: %w", err)
	}

	info, err := h.BuildDownloadTask(body.Content, json.RawMessage(configJSON))
	if err != nil {
		s.logger.Error().Str("platform", body.Platform).Err(err).Msg("platform failed to build download task")
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		s.logger.Warn().Str("platform", body.Platform).Msg("platform returned no download task info")
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	content := info.Content
	if b, err := json.Marshal(content); err == nil {
		s.logger.Info().RawJSON("content", b).Msg("content detail")
	}
	account := info.Account

	resourceInfos := info.Resources
	s.logger.Info().Str("platform", body.Platform).Str("task_name", info.Task.Name).Int("resource_count", len(resourceInfos)).Msg("platform download task built successfully")
	for _, ri := range resourceInfos {
		if len(ri.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 %s 没有下载端点", ri.Name)
		}
	}

	taskName := info.Task.Name

	// Check for duplicates
	resourceKeys := make([]string, 0, len(resourceInfos))
	resourceNames := make([]string, 0, len(resourceInfos))
	for _, ri := range resourceInfos {
		resourceKeys = append(resourceKeys, ri.UniqueID)
		resourceNames = append(resourceNames, ri.Name)
	}
	if err := s.checkDuplicate(saveDir, info.Task.UniqueID, resourceKeys, resourceNames, downloadConfigBool(body.Config, "duplicate"), downloadConfigBool(body.Config, "overwrite")); err != nil {
		return nil, err
	}

	// onTaskCreate hook
	if s.hookManager != nil && s.hookManager.HasCreateHook() {
		taskInput := s.buildTaskInput(info, taskName, saveDir, body.Filename, body.Config)
		modified, err := s.hookManager.InvokeCreateHook(taskInput)
		if err != nil {
			return nil, fmt.Errorf("onTaskCreate hook 执行失败: %w", err)
		}
		taskName, saveDir = s.applyTaskInputModifications(info, taskName, saveDir, modified)
	}

	// Write to database
	now := time.Now().UnixMilli()
	task = *info.Task
	if content != nil && content.Id != "" {
		contentID := content.Id
		task.ContentId = &contentID
	}
	task.Name = taskName
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
		contentID := ""
		if content != nil {
			contentID = content.Id
		}
		persistedAccount, err := NewContentService(s.db).UpsertAccountAndLinkContent(contentID, account, "owner", now)
		if err != nil {
			return nil, err
		}
		account = persistedAccount
	}

	if err := saveContentExtension(s.db, info.ContentDetail); err != nil {
		return nil, fmt.Errorf("保存扩展数据失败: %w", err)
	}

	// Save novel volumes
	if len(info.NovelVolumes) > 0 {
		if err := s.db.Create(&info.NovelVolumes).Error; err != nil {
			return nil, fmt.Errorf("保存小说卷失败: %w", err)
		}
	}

	// Save novel chapters
	if len(info.NovelChapters) > 0 {
		if err := s.db.Create(&info.NovelChapters).Error; err != nil {
			return nil, fmt.Errorf("保存小说章节失败: %w", err)
		}
	}

	resources := make([]model.DownloadResource, 0, len(resourceInfos))
	endpoints := make([]model.DownloadEndpoint, 0, len(resourceInfos))
	for i := range resourceInfos {
		resource := resourceInfos[i].DownloadResource
		resource.TaskId = task.Id
		if resource.CreatedAt == 0 {
			resource.CreatedAt = now
		}
		resource.UpdatedAt = now
		if err := s.db.Create(&resource).Error; err != nil {
			return nil, fmt.Errorf("创建资源失败: %w", err)
		}
		resources = append(resources, resource)
		for _, endpointInfo := range resourceInfos[i].Endpoints {
			endpoint := endpointInfo
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
	firstResource := resources[0]
	firstEndpoint := endpoints[0]

	if err := s.startCreatedDownloadTask(task.Id); err != nil {
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}
	task.Status = model.TaskStatusPreparing

	return &CreateTaskResult{
		Task:      task,
		Resource:  firstResource,
		Endpoint:  firstEndpoint,
		Resources: resources,
		Endpoints: endpoints,
		Content:   content,
		Account:   account,
	}, nil
}

// CreateTaskByURL creates a single download task via resource URL.
func (s *DownloadTaskService) CreateTaskByURL(body CreateDownloadTaskByURLBody) (*CreateTaskByURLResult, error) {
	if body.URL == "" {
		return nil, fmt.Errorf("url 不能为空")
	}

	parsedURL, err := url.Parse(body.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("无效的下载地址")
	}

	protocol := strings.ToUpper(parsedURL.Scheme)

	requestedSavePath := body.SavePath
	if requestedSavePath == "" {
		requestedSavePath, _ = body.Config["save_path"].(string)
	}
	saveDir, err := s.resolveSaveDir(requestedSavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}
	filename := body.Filename
	if filename == "" {
		filename, _ = body.Config["filename"].(string)
	}
	if filename == "" {
		base := filepath.Base(parsedURL.Path)
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

	savePath := saveDir

	taskName := filename

	configJSON, _ := json.Marshal(map[string]string{
		"url": body.URL,
	})

	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	now := time.Now().UnixMilli()

	task := model.DownloadTask{
		Name:       taskName,
		Status:     model.TaskStatusWaiting,
		ConfigJSON: string(configJSON),
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

	s.logger.Info().Int("task_id", task.Id).Str("url", body.URL).Str("save_path", savePath).Msg("URL download task written to database")

	resource := model.DownloadResource{
		TaskId:     task.Id,
		Name:       filename,
		Kind:       "file",
		Status:     0,
		MergeOrder: 0,
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

	if err := s.startCreatedDownloadTask(task.Id); err != nil {
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
func (s *DownloadTaskService) StartTask(taskID int) (*model.DownloadTask, error) {
	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	var task model.DownloadTask
	if err := s.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil, fmt.Errorf("下载任务不存在")
	}

	if task.Status != model.TaskStatusWaiting &&
		task.Status != model.TaskStatusPaused &&
		task.Status != model.TaskStatusFailed {
		return nil, fmt.Errorf("当前状态不允许启动")
	}

	s.logger.Info().Int("task_id", taskID).Str("task_name", task.Name).Int("previous_status", task.Status).Msg("received start download task request")

	if err := s.downloader.StartTask(task.Id); err != nil {
		s.logger.Error().Int("task_id", taskID).Err(err).Msg("failed to start download task")
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}
	s.logger.Info().Int("task_id", taskID).Str("status", "preparing").Msg("download task started")

	task.Status = model.TaskStatusPreparing
	return &task, nil
}

// PauseTask pauses a download task. Returns the task, whether it is a stream resource, and error.
func (s *DownloadTaskService) PauseTask(taskID int) (*model.DownloadTask, bool, error) {
	if s.db == nil {
		return nil, false, fmt.Errorf("应用未初始化，数据库不可用")
	}

	var task model.DownloadTask
	if err := s.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil, false, fmt.Errorf("下载任务不存在")
	}

	if task.Status != model.TaskStatusPreparing && task.Status != model.TaskStatusDownloading {
		return nil, false, fmt.Errorf("当前状态不允许暂停")
	}

	isStream := s.hasStreamResources(task.Id)

	if isStream {
		if err := s.downloader.StopTask(task.Id); err != nil {
			return nil, true, fmt.Errorf("停止直播录制失败: %w", err)
		}
		if err := s.db.Where("id = ?", taskID).First(&task).Error; err != nil {
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
func (s *DownloadTaskService) ResumeTask(taskID int) (*model.DownloadTask, error) {
	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	var task model.DownloadTask
	if err := s.db.Where("id = ?", taskID).First(&task).Error; err != nil {
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
func (s *DownloadTaskService) DeleteTask(taskID int) (*DownloadTaskRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	var task model.DownloadTask
	if err := s.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil, fmt.Errorf("下载任务不存在")
	}

	now := time.Now().UnixMilli()

	s.downloader.DeleteTask(task.Id)
	deletedRecord, _ := s.BuildTaskRecord(task.Id)

	s.db.Model(&task).Update("deleted_at", now)

	s.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Update("deleted_at", now)

	var resourceIDs []int
	s.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Pluck("id", &resourceIDs)
	if len(resourceIDs) > 0 {
		s.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)
		s.db.Model(&model.DownloadSegment{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)

		var endpointIDs []int
		s.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Pluck("id", &endpointIDs)
		if len(endpointIDs) > 0 {
			s.db.Model(&model.DownloadConnection{}).Where("endpoint_id IN ?", endpointIDs).Update("deleted_at", now)
		}
	}

	return deletedRecord, nil
}

// ListTasks queries the download task list.
func (s *DownloadTaskService) ListTasks(taskID int, page int, pageSize int, statusFilter string) (*TaskListResult, error) {
	if s.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	// Query single task
	if taskID > 0 {
		record, err := s.BuildTaskRecord(taskID)
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
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var tasks []model.DownloadTask
	var total int64

	query := s.db.Model(&model.DownloadTask{}).Where("deleted_at IS NULL")
	if statusFilter != "" {
		parts := strings.Split(statusFilter, ",")
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
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
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
		PageSize: pageSize,
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
	var streamTaskIDs []int
	for _, task := range tasks {
		if s.hasStreamResources(task.Id) {
			if err := s.downloader.StopTask(task.Id); err != nil {
				return paused, streamTaskIDs, fmt.Errorf("停止直播录制任务 %d 失败: %w", task.Id, err)
			}
			var stoppedTask model.DownloadTask
			if err := s.db.Where("id = ?", task.Id).First(&stoppedTask).Error; err != nil {
				return paused, streamTaskIDs, fmt.Errorf("读取直播录制任务 %d 最终状态失败: %w", task.Id, err)
			}
			if stoppedTask.Status != model.TaskStatusFinished {
				message := strings.TrimSpace(stoppedTask.ErrorMessage)
				if message == "" {
					message = fmt.Sprintf("收尾后的任务状态异常: %d", stoppedTask.Status)
				}
				return paused, streamTaskIDs, fmt.Errorf("直播录制任务 %d 收尾失败: %s", task.Id, message)
			}
			streamTaskIDs = append(streamTaskIDs, task.Id)
		} else {
			s.downloader.PauseTask(task.Id)
		}
		paused++
	}

	return paused, streamTaskIDs, nil
}

// ClearTasks clears completed/failed/cancelled download tasks.
func (s *DownloadTaskService) ClearTasks(deleteFiles bool) (int, error) {
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

		var resourceIDs []int
		s.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Pluck("id", &resourceIDs)
		if len(resourceIDs) > 0 {
			s.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)
			s.db.Model(&model.DownloadSegment{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)

			var endpointIDs []int
			s.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Pluck("id", &endpointIDs)
			if len(endpointIDs) > 0 {
				s.db.Model(&model.DownloadConnection{}).Where("endpoint_id IN ?", endpointIDs).Update("deleted_at", now)
			}
		}

		cleared++
	}

	return cleared, nil
}

// BuildTaskRecord builds the DownloadTaskRecord for a single task.
func (s *DownloadTaskService) BuildTaskRecord(taskID int) (*DownloadTaskRecord, error) {
	if s.db == nil {
		return nil, errors.New("数据库不可用")
	}
	var task model.DownloadTask
	if err := s.db.Where("id = ?", taskID).First(&task).Error; err != nil {
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

	taskIDs := make([]int, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.Id
	}
	progressByTask := make(map[int]*hermes.TaskProgress, len(tasks))
	progressResourceByTask := make(map[int]map[int]hermes.ResourceProgress, len(tasks))
	if s.downloader != nil {
		for _, task := range tasks {
			progress := s.downloader.CurrentProgress(task.Id)
			if progress == nil {
				continue
			}
			progressByTask[task.Id] = progress
			byResource := make(map[int]hermes.ResourceProgress, len(progress.Resources))
			for _, resourceProgress := range progress.Resources {
				byResource[resourceProgress.ID] = resourceProgress
			}
			progressResourceByTask[task.Id] = byResource
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
		ORDER BY r.task_id ASC, e.priority ASC, e.id ASC`, taskIDs).Scan(&endpoints).Error; err != nil {
		return nil, err
	}
	urlByTask := make(map[int]string, len(tasks))
	urlByResource := make(map[int]string)
	for _, ep := range endpoints {
		if _, exists := urlByTask[ep.TaskID]; !exists {
			urlByTask[ep.TaskID] = ep.URL
		}
		if _, exists := urlByResource[ep.ResourceID]; !exists {
			urlByResource[ep.ResourceID] = ep.URL
		}
	}

	type resourceInfo struct {
		ID           int    `gorm:"column:id"`
		TaskID       int    `gorm:"column:task_id"`
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
		Select("id, task_id, name, kind, type, size, downloaded, speed, status, merge_order").
		Where("task_id IN ? AND deleted_at IS NULL", taskIDs).
		Order("task_id ASC, merge_order ASC, id ASC").
		Scan(&resources).Error; err != nil {
		return nil, err
	}
	sizeByTask := make(map[int]int64, len(tasks))
	downloadedByTask := make(map[int]int64, len(tasks))
	downloadedByResource := make(map[int]int64)
	speedByTask := make(map[int]int64, len(tasks))
	speedByResource := make(map[int]int64)
	resourcesByTask := make(map[int][]resourceInfo, len(tasks))
	for _, r := range resources {
		resourcesByTask[r.TaskID] = append(resourcesByTask[r.TaskID], r)
		if r.Size > 0 {
			sizeByTask[r.TaskID] += r.Size
		}
		downloaded := r.Downloaded
		if downloaded <= 0 && r.Status == 2 && r.Size > 0 {
			downloaded = r.Size
		}
		if downloaded > 0 {
			downloadedByTask[r.TaskID] += downloaded
			downloadedByResource[r.ID] = downloaded
		}
		if r.Speed > 0 {
			speedByTask[r.TaskID] += r.Speed
			speedByResource[r.ID] = r.Speed
		}
	}
	for taskID, progress := range progressByTask {
		if progress.TotalSize > 0 {
			sizeByTask[taskID] = progress.TotalSize
		}
		downloadedByTask[taskID] = progress.Downloaded
		speedByTask[taskID] = progress.Speed
		for _, resourceProgress := range progress.Resources {
			downloadedByResource[resourceProgress.ID] = resourceProgress.Downloaded
			speedByResource[resourceProgress.ID] = resourceProgress.Speed
		}
	}

	type childAggregate struct {
		ParentTaskID int `gorm:"column:parent_task_id"`
		Count        int `gorm:"column:count"`
	}
	var childAggregates []childAggregate
	if err := s.db.Model(&model.DownloadTask{}).
		Select("parent_task_id, COUNT(*) AS count").
		Where("parent_task_id IN ? AND deleted_at IS NULL", taskIDs).
		Group("parent_task_id").
		Scan(&childAggregates).Error; err != nil {
		return nil, err
	}
	childCountByTask := make(map[int]int, len(childAggregates))
	for _, aggregate := range childAggregates {
		childCountByTask[aggregate.ParentTaskID] = aggregate.Count
	}

	contentIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.ContentId != nil && *task.ContentId != "" {
			contentIDs = append(contentIDs, *task.ContentId)
		}
	}
	contentTypeByID := make(map[string]string, len(contentIDs))
	if len(contentIDs) > 0 {
		type contentTypeRow struct {
			ID   string `gorm:"column:id"`
			Type string `gorm:"column:type"`
		}
		var contentTypeRows []contentTypeRow
		if err := s.db.Table("content").
			Select("id, type").
			Where("id IN ?", contentIDs).
			Scan(&contentTypeRows).Error; err != nil {
			return nil, err
		}
		for _, row := range contentTypeRows {
			contentTypeByID[row.ID] = row.Type
		}
	}

	for _, task := range tasks {
		totalSize := sizeByTask[task.Id]
		errorMessage := ""
		if task.Status == model.TaskStatusFailed {
			errorMessage = task.ErrorMessage
		}
		resourceRows := resourcesByTask[task.Id]
		progressResources := progressResourceByTask[task.Id]
		files := make([]DownloadTaskFileRecord, 0, len(resourceRows))
		for _, r := range resourceRows {
			resourceSize := r.Size
			resourceDownloaded := downloadedByResource[r.ID]
			resourceSpeed := speedByResource[r.ID]
			if resourceProgress, ok := progressResources[r.ID]; ok {
				if resourceProgress.Size > resourceSize {
					resourceSize = resourceProgress.Size
				}
				resourceDownloaded = resourceProgress.Downloaded
				resourceSpeed = resourceProgress.Speed
			}
			outputPath := r.Name
			fileStatus := "waiting"
			switch r.Status {
			case 1:
				fileStatus = "downloading"
			case 2:
				fileStatus = "finished"
			}
			if resourceSize > 0 && resourceDownloaded >= resourceSize {
				fileStatus = "finished"
			} else if resourceDownloaded > 0 || resourceSpeed > 0 || task.Status == model.TaskStatusDownloading {
				fileStatus = "downloading"
			}
			fileError := ""
			if fileStatus != "finished" {
				switch task.Status {
				case model.TaskStatusFinished:
					fileStatus = "finished"
				case model.TaskStatusPaused:
					fileStatus = "paused"
				case model.TaskStatusFailed:
					fileStatus = "error"
					fileError = errorMessage
				case model.TaskStatusCancelled:
					fileStatus = "cancelled"
				}
			}
			files = append(files, DownloadTaskFileRecord{
				ID:         r.ID,
				Name:       r.Name,
				Kind:       r.Kind,
				Type:       r.ResourceType,
				Status:     fileStatus,
				Size:       resourceSize,
				Downloaded: resourceDownloaded,
				Speed:      resourceSpeed,
				Progress:   TaskProgressPercent(resourceDownloaded, resourceSize, MapResourceTaskStatus(r.Status)),
				URL:        urlByResource[r.ID],
				OutputPath: outputPath,
				Error:      fileError,
			})
		}
		effectiveStatus := ComputeEffectiveTaskStatus(task.Status, files)
		contentType := ""
		if task.ContentId != nil {
			contentType = contentTypeByID[*task.ContentId]
		}
		records = append(records, DownloadTaskRecord{
			ID:           task.Id,
			ContentID:    task.ContentId,
			ContentType:  contentType,
			ParentTaskID: task.ParentTaskID,
			RootTaskID:   task.RootTaskID,
			RelationType: task.RelationType,
			ChildCount:   childCountByTask[task.Id],
			Name:         task.Name,
			PlatformID:   task.PlatformId,
			Status:       effectiveStatus,
			SourceURL:    task.SourceURL,
			CoverURL:     task.CoverURL,
			CoverWidth:   task.CoverWidth,
			CoverHeight:  task.CoverHeight,
			ConfigJSON:   task.ConfigJSON,
			MetadataJSON: task.MetadataJSON,
			URL:          urlByTask[task.Id],
			Size:         totalSize,
			Downloaded:   downloadedByTask[task.Id],
			Speed:        speedByTask[task.Id],
			Progress:     TaskProgressPercent(downloadedByTask[task.Id], totalSize, effectiveStatus),
			Error:        errorMessage,
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

func saveContentExtension(db *gorm.DB, detail any) error {
	if db == nil {
		return ErrDBNotInitialized
	}
	if detail == nil {
		return nil
	}
	return db.Session(&gorm.Session{FullSaveAssociations: true}).Save(detail).Error
}

func (s *DownloadTaskService) resolveSaveDir(requested string) (string, error) {
	savePath := strings.TrimSpace(requested)
	if savePath == "" {
		savePath = strings.TrimSpace(s.downloadDir)
	}
	if savePath == "" {
		return "", fmt.Errorf("保存目录不能为空")
	}

	workDir := s.workDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("获取工作目录失败: %w", err)
		}
	}

	savePath = strings.ReplaceAll(savePath, "%UserDownloads%", xdg.UserDirs.Download)
	savePath = strings.ReplaceAll(savePath, "%CWD%", workDir)
	savePath = filepath.Clean(savePath)
	if !filepath.IsAbs(savePath) {
		savePath = filepath.Join(workDir, savePath)
	}

	if err := os.MkdirAll(savePath, 0755); err != nil {
		return "", fmt.Errorf("创建保存目录 %q 失败: %w", savePath, err)
	}

	return savePath, nil
}

func (s *DownloadTaskService) checkDuplicate(saveDir string, taskUniqueID string, resourceKeys []string, resourceNames []string, duplicate bool, overwrite bool) error {
	if duplicate {
		return nil
	}

	var conflicts []duplicateConflict
	var existingTaskID int
	var existingTaskName string

	// Task-level duplicate check: any existing task with the same unique_id (regardless of status)
	if taskUniqueID != "" {
		var existingTask model.DownloadTask
		err := s.db.Where("unique_id = ? AND deleted_at IS NULL", taskUniqueID).First(&existingTask).Error
		if err == nil {
			existingTaskID = existingTask.Id
			existingTaskName = existingTask.Name
			s.logger.Warn().
				Int("existing_task_id", existingTask.Id).
				Str("existing_task_unique_id", existingTask.UniqueID).
				Str("incoming_task_unique_id", taskUniqueID).
				Msg("checkDuplicate: task-level duplicate found")
			conflicts = append(conflicts, duplicateConflict{
				Type:        "task",
				TaskID:      existingTask.Id,
				ResourceKey: taskUniqueID,
			})
		}
	}

	for i, key := range resourceKeys {
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
			existingTaskID = dup.TaskId
			if existingTaskName == "" && dup.TaskId > 0 {
				var task model.DownloadTask
				if taskErr := s.db.
					Select("id", "name").
					Where("id = ? AND deleted_at IS NULL", dup.TaskId).
					First(&task).Error; taskErr == nil {
					existingTaskName = task.Name
				}
			}
			s.logger.Warn().
				Int("existing_task_id", dup.TaskId).
				Str("existing_resource_unique_id", dup.UniqueID).
				Str("incoming_resource_unique_id", key).
				Str("resource_name", resourceNames[i]).
				Msg("checkDuplicate: resource-level duplicate found")
			conflicts = append(conflicts, duplicateConflict{
				Type:        "resource",
				TaskID:      dup.TaskId,
				ResourceKey: resourceNames[i],
			})
		}
	}

	for _, name := range resourceNames {
		filePath := filepath.Join(saveDir, filepath.Base(name))
		if fileInfo, err := os.Stat(filePath); err == nil && !fileInfo.IsDir() {
			if existingTaskName == "" {
				existingTaskName = filepath.Base(filePath)
			}
			conflicts = append(conflicts, duplicateConflict{
				Type:     "file",
				FilePath: filePath,
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
				if err := s.deleteTaskWithFiles(conflict.TaskID); err != nil {
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

	errResp := &DuplicateTaskError{
		IncomingUniqueID: taskUniqueID,
	}
	if existingTaskID > 0 {
		errResp.ExistingTaskID = existingTaskID
	}
	if existingTaskName != "" {
		errResp.ExistingTaskName = existingTaskName
	}
	s.logger.Warn().
		Int("existing_task_id", existingTaskID).
		Str("existing_task_name", existingTaskName).
		Str("incoming_task_unique_id", taskUniqueID).
		Strs("incoming_resource_unique_ids", resourceKeys).
		Msg("download task conflict detected")
	return errResp
}

func (s *DownloadTaskService) deleteTaskWithFiles(taskID int) error {
	var task model.DownloadTask
	if err := s.db.First(&task, taskID).Error; err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	return s.db.Model(&task).Updates(map[string]any{
		"deleted_at": time.Now().UnixMilli(),
	}).Error
}

func (s *DownloadTaskService) startCreatedDownloadTask(taskID int) error {
	if s.downloader == nil {
		s.logger.Error().Int("task_id", taskID).Msg("Hermes downloader not initialized, unable to start download task")
		return fmt.Errorf("Hermes 下载器未初始化")
	}
	s.logger.Info().Int("task_id", taskID).Msg("submitting download task to Hermes scheduler")
	if err := s.downloader.StartTask(taskID); err != nil {
		s.logger.Error().Int("task_id", taskID).Err(err).Msg("Hermes scheduler failed to start download task")
		return err
	}
	s.logger.Info().Int("task_id", taskID).Msg("download task submitted to Hermes schedule queue")
	return nil
}

func (s *DownloadTaskService) hasStreamResources(taskID int) bool {
	if s.db == nil {
		return false
	}
	var count int64
	s.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND UPPER(type) = ?", taskID, model.ResourceTypeStream).
		Count(&count)
	return count > 0
}

func (s *DownloadTaskService) buildTaskInput(info *adapter.DownloadTaskResult, taskName, taskSavePath, filename string, bodyCfg map[string]any) *hermes.TaskInput {
	taskInfo := hermes.TaskInfo{
		Name:     taskName,
		SavePath: taskSavePath,
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
			ID:        ri.Id,
			Name:      ri.Name,
			Kind:      ri.Kind,
			Size:      ri.Size,
			UniqueID:  ri.UniqueID,
			Endpoints: endpoints,
		})
	}

	config := make(map[string]any, len(bodyCfg)+2)
	for key, value := range bodyCfg {
		config[key] = value
	}
	config["save_path"] = taskSavePath
	if filename != "" {
		config["filename"] = filename
	}

	if info.Task.ConfigJSON != "" {
		var taskCfg map[string]any
		if json.Unmarshal([]byte(info.Task.ConfigJSON), &taskCfg) == nil {
			for k, v := range taskCfg {
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
		Task:      taskInfo,
		Config:    config,
		Metadata:  metadata,
		Resources: resources,
	}
}

func (s *DownloadTaskService) applyTaskInputModifications(info *adapter.DownloadTaskResult, taskName, taskSavePath string, modified *hermes.TaskInput) (string, string) {
	if modified == nil {
		return taskName, taskSavePath
	}

	if modified.Task.Name != "" {
		taskName = modified.Task.Name
	}
	if modified.Task.SavePath != "" {
		taskSavePath = modified.Task.SavePath
	}

	for i, modRes := range modified.Resources {
		if i >= len(info.Resources) {
			break
		}
		if modRes.Name != "" {
			info.Resources[i].Name = modRes.Name
		}
	}

	return taskName, taskSavePath
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// buildResourceTree splits a flat resource list into a directory tree based on paths.
func buildResourceTree(resources []ResourceDetail) *ResourceTreeNode {
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
func ComputeEffectiveTaskStatus(dbStatus int, files []DownloadTaskFileRecord) int {
	switch dbStatus {
	case model.TaskStatusPaused, model.TaskStatusFinished, model.TaskStatusFailed,
		model.TaskStatusCancelled, model.TaskStatusMerging:
		return dbStatus
	}
	if len(files) == 0 {
		return dbStatus
	}
	allFinished := true
	hasDownloading := false
	for _, f := range files {
		switch f.Status {
		case "finished":
		case "downloading":
			hasDownloading = true
			allFinished = false
		default:
			allFinished = false
		}
	}
	if allFinished {
		return model.TaskStatusFinished
	}
	if hasDownloading {
		return model.TaskStatusDownloading
	}
	return model.TaskStatusWaiting
}
