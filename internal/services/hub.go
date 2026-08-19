package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/hub"
)

// NamedHubConfig associates a stable local name with one Hub client configuration.
type NamedHubConfig struct {
	Name   string
	Config hub.Config
}

// HubServiceOptions contains the dependencies and configuration for HubService.
type HubServiceOptions struct {
	ApplicationConfig   *config.Config
	Configs             []NamedHubConfig
	DefaultName         string
	DownloadTaskService *DownloadTaskService
	Logger              *zerolog.Logger
}

// HubWXChannelsDownloadOptions controls local download creation after a remote fetch.
type HubWXChannelsDownloadOptions struct {
	DownloadDir string         `json:"download_dir"`
	Filename    string         `json:"filename"`
	Config      map[string]any `json:"config"`
	AutoStart   *bool          `json:"auto_start"`
}

// SubmitHubWXChannelsTaskRequest describes one remote wxchannels fetch request.
type SubmitHubWXChannelsTaskRequest struct {
	Hub            string                        `json:"hub"`
	URL            string                        `json:"url"`
	TargetClientID string                        `json:"target_client_id"`
	IdempotencyKey string                        `json:"idempotency_key"`
	Download       *HubWXChannelsDownloadOptions `json:"download,omitempty"`
}

// SubmitHubDownloadTaskRequest describes one remote download creation request.
type SubmitHubDownloadTaskRequest struct {
	Hub            string                       `json:"hub"`
	TargetClientID string                       `json:"target_client_id"`
	IdempotencyKey string                       `json:"idempotency_key"`
	Request        *CreateDownloadTaskBody      `json:"request,omitempty"`
	URLRequest     *CreateDownloadTaskByURLBody `json:"url_request,omitempty"`
}

// HubNamedStatus associates connection status with its configured Hub name.
type HubNamedStatus struct {
	Name   string
	Status hub.Status
}

// HubStatusSnapshot contains every Hub status and the selected/default status.
type HubStatusSnapshot struct {
	Status     hub.Status
	DefaultHub string
	Hubs       []HubNamedStatus
}

type hub_selection_error struct {
	message string
}

func (e *hub_selection_error) Error() string {
	return e.message
}

// IsHubSelectionError reports whether an operation failed while selecting a configured Hub.
func IsHubSelectionError(err error) bool {
	var selection_error *hub_selection_error
	return errors.As(err, &selection_error)
}

type hub_wxchannels_payload struct {
	URL      string                        `json:"url"`
	Download *HubWXChannelsDownloadOptions `json:"download,omitempty"`
}

type hub_download_payload struct {
	Request    *CreateDownloadTaskBody      `json:"request,omitempty"`
	URLRequest *CreateDownloadTaskByURLBody `json:"url_request,omitempty"`
}

// HubService owns Hub clients, their lifecycle, and application task dispatch.
type HubService struct {
	clients               map[string]*hub.Client
	order                 []string
	default_name          string
	download_task_service *DownloadTaskService
	logger                zerolog.Logger
	config_error          error
	wxchannels_mu         sync.Mutex
}

// NewHubService creates all configured Hub clients without starting their connections.
func NewHubService(options HubServiceOptions) *HubService {
	logger := zerolog.Nop()
	if options.Logger != nil {
		logger = options.Logger.With().Str("component", "hub_service").Logger()
	}
	named_configs := options.Configs
	default_name := options.DefaultName
	var config_error error
	if options.ApplicationConfig != nil {
		named_configs, default_name, config_error = load_hub_configs(options.ApplicationConfig)
		if config_error != nil {
			logger.Error().Err(config_error).Msg("Hub 配置无效，Hub 服务将保持不可用")
		}
	}
	service := &HubService{
		clients:               make(map[string]*hub.Client, len(named_configs)),
		order:                 make([]string, 0, len(named_configs)),
		default_name:          strings.TrimSpace(default_name),
		download_task_service: options.DownloadTaskService,
		logger:                logger,
		config_error:          config_error,
	}
	for _, named_config := range named_configs {
		hub_name := named_config.Name
		service.order = append(service.order, hub_name)
		service.clients[hub_name] = hub.NewClient(
			named_config.Config,
			service.execute_task,
			func(task hub.Task) {
				service.handle_terminal_task(hub_name, task)
			},
			&service.logger,
		)
	}
	return service
}

// Start validates and starts all configured Hub clients.
func (s *HubService) Start(parent_context context.Context) error {
	if s == nil {
		return nil
	}
	started_clients := make([]*hub.Client, 0, len(s.order))
	for _, hub_name := range s.order {
		hub_client := s.clients[hub_name]
		if hub_client == nil {
			continue
		}
		if err := hub_client.Start(parent_context); err != nil {
			for _, started_client := range started_clients {
				started_client.Close()
			}
			return fmt.Errorf("启动 Hub %q 失败: %w", hub_name, err)
		}
		started_clients = append(started_clients, hub_client)
	}
	return nil
}

// Close stops all configured Hub clients.
func (s *HubService) Close() {
	if s == nil {
		return
	}
	for _, hub_name := range s.order {
		if hub_client := s.clients[hub_name]; hub_client != nil {
			hub_client.Close()
		}
	}
}

// Status returns every Hub status plus the requested or default Hub status.
func (s *HubService) Status(requested_name string) (HubStatusSnapshot, error) {
	snapshot := HubStatusSnapshot{
		Status: hub.Status{Capabilities: []string{}},
		Hubs:   []HubNamedStatus{},
	}
	if s == nil {
		return snapshot, &hub_selection_error{message: "Hub 未配置"}
	}
	if s.config_error != nil {
		return snapshot, &hub_selection_error{message: s.config_error.Error()}
	}
	snapshot.DefaultHub = s.default_name
	snapshot.Hubs = make([]HubNamedStatus, 0, len(s.order))
	for _, hub_name := range s.order {
		hub_client := s.clients[hub_name]
		if hub_client == nil {
			continue
		}
		snapshot.Hubs = append(snapshot.Hubs, HubNamedStatus{
			Name:   hub_name,
			Status: hub_client.Status(),
		})
	}
	selected_name := strings.TrimSpace(requested_name)
	if selected_name == "" {
		selected_name = s.default_name
	}
	if selected_name == "" {
		return snapshot, nil
	}
	selected_client := s.clients[selected_name]
	if selected_client == nil {
		return snapshot, &hub_selection_error{message: fmt.Sprintf("Hub %q 不存在", selected_name)}
	}
	snapshot.Status = selected_client.Status()
	return snapshot, nil
}

// SubmitWXChannelsTask publishes a wxchannels fetch task through the selected Hub.
func (s *HubService) SubmitWXChannelsTask(request_context context.Context, request SubmitHubWXChannelsTaskRequest) (string, *hub.Task, error) {
	request.URL = strings.TrimSpace(request.URL)
	if request.URL == "" {
		return "", nil, errors.New("url 不能为空")
	}
	hub_name, hub_client, err := s.resolve_client(request.Hub)
	if err != nil {
		return "", nil, err
	}
	task, err := hub_client.SubmitTask(request_context, hub.SubmitTaskRequest{
		Kind:               hub.KindWXChannelsFetch,
		TargetClientID:     strings.TrimSpace(request.TargetClientID),
		RequiredCapability: hub.CapabilityWXChannelsFetch,
		IdempotencyKey:     strings.TrimSpace(request.IdempotencyKey),
		Payload: hub_wxchannels_payload{
			URL:      request.URL,
			Download: request.Download,
		},
	})
	return hub_name, task, err
}

// SubmitDownloadTask publishes a download creation task through the selected Hub.
func (s *HubService) SubmitDownloadTask(request_context context.Context, request SubmitHubDownloadTaskRequest) (string, *hub.Task, error) {
	request.TargetClientID = strings.TrimSpace(request.TargetClientID)
	if request.TargetClientID == "" {
		return "", nil, errors.New("target_client_id 不能为空")
	}
	if (request.Request == nil) == (request.URLRequest == nil) {
		return "", nil, errors.New("request 和 url_request 必须且只能提供一个")
	}
	hub_name, hub_client, err := s.resolve_client(request.Hub)
	if err != nil {
		return "", nil, err
	}
	task, err := hub_client.SubmitTask(request_context, hub.SubmitTaskRequest{
		Kind:               hub.KindDownloadCreate,
		TargetClientID:     request.TargetClientID,
		RequiredCapability: hub.CapabilityDownloadCreate,
		IdempotencyKey:     strings.TrimSpace(request.IdempotencyKey),
		Payload: hub_download_payload{
			Request:    request.Request,
			URLRequest: request.URLRequest,
		},
	})
	return hub_name, task, err
}

// GetTask retrieves one task through the selected Hub.
func (s *HubService) GetTask(request_context context.Context, requested_name string, task_id string) (string, *hub.Task, error) {
	hub_name, hub_client, err := s.resolve_client(requested_name)
	if err != nil {
		return "", nil, err
	}
	task, err := hub_client.GetTask(request_context, task_id)
	return hub_name, task, err
}

// ListTasks retrieves tasks through the selected Hub.
func (s *HubService) ListTasks(request_context context.Context, requested_name string, status string, limit int) (string, []hub.Task, error) {
	hub_name, hub_client, err := s.resolve_client(requested_name)
	if err != nil {
		return "", nil, err
	}
	tasks, err := hub_client.ListTasks(request_context, status, limit)
	return hub_name, tasks, err
}

func (s *HubService) resolve_client(requested_name string) (string, *hub.Client, error) {
	if s == nil {
		return "", nil, &hub_selection_error{message: "Hub 未配置"}
	}
	if s.config_error != nil {
		return "", nil, &hub_selection_error{message: s.config_error.Error()}
	}
	hub_name := strings.TrimSpace(requested_name)
	if hub_name == "" {
		hub_name = s.default_name
	}
	if hub_name == "" || len(s.clients) == 0 {
		return "", nil, &hub_selection_error{message: "hub.instances 未配置"}
	}
	hub_client := s.clients[hub_name]
	if hub_client == nil {
		return "", nil, &hub_selection_error{message: fmt.Sprintf("Hub %q 不存在", hub_name)}
	}
	if !hub_client.Status().Enabled {
		return "", nil, &hub_selection_error{message: fmt.Sprintf("Hub %q 未启用", hub_name)}
	}
	return hub_name, hub_client, nil
}

func (s *HubService) execute_task(task_context context.Context, task hub.Task) (json.RawMessage, error) {
	if err := task_context.Err(); err != nil {
		return nil, err
	}
	switch task.Kind {
	case hub.KindWXChannelsFetch:
		var payload hub_wxchannels_payload
		if err := json.Unmarshal(task.Payload, &payload); err != nil {
			return nil, fmt.Errorf("解析视频号 Hub 任务失败: %w", err)
		}
		payload.URL = strings.TrimSpace(payload.URL)
		if payload.URL == "" {
			return nil, errors.New("视频号 Hub 任务缺少 url")
		}
		handler := adapter.Get("wxchannels")
		if handler == nil {
			return nil, errors.New("当前实例未安装 wxchannels adapter")
		}
		s.wxchannels_mu.Lock()
		defer s.wxchannels_mu.Unlock()
		if err := task_context.Err(); err != nil {
			return nil, err
		}
		content, err := handler.Fetch(payload.URL)
		if err != nil {
			return nil, fmt.Errorf("获取视频号内容失败: %w", err)
		}
		data, err := json.Marshal(content)
		if err != nil {
			return nil, fmt.Errorf("编码视频号内容失败: %w", err)
		}
		return data, nil

	case hub.KindDownloadCreate:
		if s.download_task_service == nil {
			return nil, errors.New("下载任务服务未初始化")
		}
		var payload hub_download_payload
		if err := json.Unmarshal(task.Payload, &payload); err != nil {
			return nil, fmt.Errorf("解析远程下载任务失败: %w", err)
		}
		if payload.Request != nil && payload.URLRequest != nil {
			return nil, errors.New("request 和 url_request 只能提供一个")
		}
		if payload.Request != nil {
			created, err := s.download_task_service.CreateTask(*payload.Request)
			if err != nil {
				return nil, err
			}
			return json.Marshal(BuildDownloadTaskItem(created))
		}
		if payload.URLRequest != nil {
			created, err := s.download_task_service.CreateTaskByURL(*payload.URLRequest)
			if err != nil {
				return nil, err
			}
			return json.Marshal(created)
		}
		return nil, errors.New("远程下载任务缺少 request 或 url_request")
	default:
		return nil, fmt.Errorf("不支持的 Hub 任务类型: %s", task.Kind)
	}
}

func (s *HubService) handle_terminal_task(hub_name string, task hub.Task) {
	if s == nil || task.Status != "completed" {
		return
	}
	hub_client := s.clients[hub_name]
	if hub_client == nil {
		return
	}
	status := hub_client.Status()
	if task.PublisherID != status.ClientID || task.Kind != hub.KindWXChannelsFetch {
		return
	}
	var payload hub_wxchannels_payload
	if err := json.Unmarshal(task.Payload, &payload); err != nil || payload.Download == nil {
		return
	}
	if s.download_task_service == nil {
		return
	}
	_, err := s.download_task_service.CreateTask(CreateDownloadTaskBody{
		Platform:       "wxchannels",
		Content:        task.Result,
		BuildFromFetch: true,
		DownloadDir:    payload.Download.DownloadDir,
		Filename:       payload.Download.Filename,
		Config:         payload.Download.Config,
		AutoStart:      payload.Download.AutoStart,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("hub", hub_name).Str("hub_task_id", task.ID).Msg("failed to create local download from completed hub task")
		return
	}
	s.logger.Info().Str("hub", hub_name).Str("hub_task_id", task.ID).Msg("created local download from completed hub task")
}
