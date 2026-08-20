package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/hub"
	"wx_channel/pkg/scraper/wxchannels"
)

const (
	hub_method_wxchannels_contact_search    = "wxchannels.contact.search"
	hub_method_wxchannels_contact_feed_list = "wxchannels.contact.feed.list"
	hub_method_wxchannels_live_replay_list  = "wxchannels.live.replay.list"
	hub_method_wxchannels_feed_profile      = "wxchannels.feed.profile"
	hub_method_wxchannels_feed_comment_list = "wxchannels.feed.comment.list"
	hub_method_wxchannels_feed_share_url    = "wxchannels.feed.share_url"
)

// HubServiceOptions contains the dependencies and configuration for HubService.
type HubServiceOptions struct {
	ApplicationConfig   *config.Config
	Config              *hub.Config
	DownloadTaskService *DownloadTaskService
	MethodHandlers      map[string]HubMethodHandler
	Logger              *zerolog.Logger
}

// HubMethodHandler handles the args object for one registered Hub method.
type HubMethodHandler func(context.Context, json.RawMessage) (json.RawMessage, error)

// HubWXChannelsDownloadOptions controls local download creation after a remote fetch.
type HubWXChannelsDownloadOptions struct {
	DownloadDir string         `json:"download_dir"`
	Filename    string         `json:"filename"`
	Config      map[string]any `json:"config"`
	AutoStart   *bool          `json:"auto_start"`
}

// SubmitHubWXChannelsTaskRequest describes one remote wxchannels fetch request.
type SubmitHubWXChannelsTaskRequest struct {
	URL                  string                        `json:"url"`
	TargetDeviceID       string                        `json:"target_device_id"`
	LegacyTargetClientID string                        `json:"target_client_id,omitempty"`
	IdempotencyKey       string                        `json:"idempotency_key"`
	Download             *HubWXChannelsDownloadOptions `json:"download,omitempty"`
}

// SubmitHubDownloadTaskRequest describes one remote download creation request.
type SubmitHubDownloadTaskRequest struct {
	TargetDeviceID       string                       `json:"target_device_id"`
	LegacyTargetClientID string                       `json:"target_client_id,omitempty"`
	IdempotencyKey       string                       `json:"idempotency_key"`
	Request              *CreateDownloadTaskBody      `json:"request,omitempty"`
	URLRequest           *CreateDownloadTaskByURLBody `json:"url_request,omitempty"`
}

// SubmitHubCallRequest describes one generic CGI-style method call.
type SubmitHubCallRequest struct {
	Method               string         `json:"method"`
	Args                 map[string]any `json:"args"`
	TargetDeviceID       string         `json:"target_device_id"`
	LegacyTargetClientID string         `json:"target_client_id,omitempty"`
	IdempotencyKey       string         `json:"idempotency_key"`
}

type hub_wxchannels_args struct {
	URL      string                        `json:"url"`
	Download *HubWXChannelsDownloadOptions `json:"download,omitempty"`
}

type hub_download_args struct {
	Request    *CreateDownloadTaskBody      `json:"request,omitempty"`
	URLRequest *CreateDownloadTaskByURLBody `json:"url_request,omitempty"`
}

type hub_wxchannels_contact_search_args struct {
	Keyword    string `json:"keyword"`
	NextMarker string `json:"next_marker"`
}

type hub_wxchannels_account_page_args struct {
	Username   string `json:"username"`
	NextMarker string `json:"next_marker"`
}

type hub_wxchannels_feed_profile_args struct {
	OID string `json:"oid"`
	NID string `json:"nid"`
	URL string `json:"url"`
	EID string `json:"eid"`
}

type hub_wxchannels_feed_comment_list_args struct {
	OID        string `json:"oid"`
	NID        string `json:"nid"`
	CommentID  string `json:"comment_id"`
	NextMarker string `json:"next_marker"`
}

type hub_wxchannels_feed_share_url_args struct {
	OID string `json:"oid"`
}

type hub_wxchannels_adapter interface {
	SearchChannelsContact(keyword string, next_marker string) (*wxchannels.ChannelsContactSearchResp, error)
	FetchChannelsFeedListOfContact(username string, next_marker string) (*wxchannels.ChannelsFeedListOfAccountResp, error)
	FetchChannelsLiveReplayList(username string, next_marker string) (*wxchannels.ChannelsFeedListOfAccountResp, error)
	FetchChannelsFeedProfile(oid string, nid string, request_url string, eid string) (*wxchannels.ChannelsFeedProfileResp, error)
	FetchChannelsFeedCommentList(oid string, nid string, comment_id string, next_marker string) (*wxchannels.ChannelsFeedCommentListResp, error)
	FetchChannelsFeedShareUrl(oid string) (*wxchannels.ChannelsFeedShareUrlResp, error)
}

type hub_unavailable_error struct {
	message string
}

func (e *hub_unavailable_error) Error() string {
	return e.message
}

// IsHubUnavailableError reports whether the single Hub connection is unavailable.
func IsHubUnavailableError(err error) bool {
	var unavailable_error *hub_unavailable_error
	return errors.As(err, &unavailable_error)
}

// HubService owns Hub clients, their lifecycle, and application task dispatch.
type HubService struct {
	client                *hub.Client
	download_task_service *DownloadTaskService
	logger                zerolog.Logger
	config_error          error
	wxchannels_mu         sync.Mutex
	wxchannels_adapter    hub_wxchannels_adapter
	method_handlers       map[string]HubMethodHandler
}

// NewHubService creates the current operating-system device's single Hub client.
func NewHubService(options HubServiceOptions) *HubService {
	logger := zerolog.Nop()
	if options.Logger != nil {
		logger = options.Logger.With().Str("component", "hub_service").Logger()
	}
	hub_config := options.Config
	var config_error error
	if options.ApplicationConfig != nil {
		hub_config, config_error = load_hub_config(options.ApplicationConfig)
		if config_error != nil {
			logger.Error().Err(config_error).Msg("Hub 配置无效，Hub 服务将保持不可用")
		}
	}
	service := &HubService{
		download_task_service: options.DownloadTaskService,
		logger:                logger,
		config_error:          config_error,
		method_handlers:       make(map[string]HubMethodHandler),
	}
	wxchannels_handler := adapter.Get("wxchannels")
	if wxchannels_handler != nil {
		service.method_handlers[hub.MethodWXChannelsFetch] = service.execute_wxchannels_fetch
		if wxchannels_adapter, ok := wxchannels_handler.(hub_wxchannels_adapter); ok {
			service.wxchannels_adapter = wxchannels_adapter
			service.register_wxchannels_methods()
		}
	}
	if service.download_task_service != nil {
		service.method_handlers[hub.MethodDownloadCreate] = service.execute_download_create
	}
	for method, handler := range options.MethodHandlers {
		if handler != nil {
			service.method_handlers[strings.TrimSpace(method)] = handler
		}
	}
	if hub_config != nil {
		if hub_config.Methods != nil {
			selected_handlers := make(map[string]HubMethodHandler)
			for _, method := range hub_config.Methods {
				handler := service.method_handlers[method]
				if handler == nil {
					logger.Warn().Str("method", method).Msg("Hub 配置的方法在当前设备上没有处理函数")
					continue
				}
				selected_handlers[method] = handler
			}
			service.method_handlers = selected_handlers
		}
		resolved_config := *hub_config
		resolved_config.Methods = service.method_names()
		service.client = hub.NewClient(
			resolved_config,
			service.execute_task,
			service.handle_terminal_task,
			&service.logger,
		)
	}
	return service
}

func (s *HubService) register_wxchannels_methods() {
	if s == nil || s.wxchannels_adapter == nil {
		return
	}
	if s.method_handlers == nil {
		s.method_handlers = make(map[string]HubMethodHandler)
	}
	s.method_handlers[hub_method_wxchannels_contact_search] = s.execute_wxchannels_contact_search
	s.method_handlers[hub_method_wxchannels_contact_feed_list] = s.execute_wxchannels_contact_feed_list
	s.method_handlers[hub_method_wxchannels_live_replay_list] = s.execute_wxchannels_live_replay_list
	s.method_handlers[hub_method_wxchannels_feed_profile] = s.execute_wxchannels_feed_profile
	s.method_handlers[hub_method_wxchannels_feed_comment_list] = s.execute_wxchannels_feed_comment_list
	s.method_handlers[hub_method_wxchannels_feed_share_url] = s.execute_wxchannels_feed_share_url
}

// Start validates and starts the configured Hub client.
func (s *HubService) Start(parent_context context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	if err := s.client.Start(parent_context); err != nil {
		return fmt.Errorf("启动 Hub 失败: %w", err)
	}
	return nil
}

// Close stops all configured Hub clients.
func (s *HubService) Close() {
	if s == nil {
		return
	}
	if s.client != nil {
		s.client.Close()
	}
}

// Status returns this operating-system device's Hub connection status.
func (s *HubService) Status() (hub.Status, error) {
	empty_status := hub.Status{Methods: []string{}}
	if s == nil {
		return empty_status, &hub_unavailable_error{message: "Hub 未配置"}
	}
	if s.config_error != nil {
		return empty_status, &hub_unavailable_error{message: s.config_error.Error()}
	}
	if s.client == nil {
		return empty_status, &hub_unavailable_error{message: "Hub 未配置"}
	}
	return s.client.Status(), nil
}

// Call publishes one generic method invocation through the Hub.
func (s *HubService) Call(request_context context.Context, request SubmitHubCallRequest) (*hub.Task, error) {
	method := strings.TrimSpace(request.Method)
	if method == "" {
		return nil, errors.New("method 不能为空")
	}
	hub_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	task, err := hub_client.SubmitTask(request_context, hub.SubmitTaskRequest{
		Method:         method,
		TargetDeviceID: request.target_device_id(),
		IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		Args:           request.Args,
	})
	return task, err
}

// SubmitWXChannelsTask publishes a wxchannels fetch task through the Hub.
func (s *HubService) SubmitWXChannelsTask(request_context context.Context, request SubmitHubWXChannelsTaskRequest) (*hub.Task, error) {
	request.URL = strings.TrimSpace(request.URL)
	if request.URL == "" {
		return nil, errors.New("url 不能为空")
	}
	hub_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	task, err := hub_client.SubmitTask(request_context, hub.SubmitTaskRequest{
		Method:         hub.MethodWXChannelsFetch,
		TargetDeviceID: request.target_device_id(),
		IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		Args: hub_wxchannels_args{
			URL:      request.URL,
			Download: request.Download,
		},
	})
	return task, err
}

// SubmitDownloadTask publishes a download creation task through the Hub.
func (s *HubService) SubmitDownloadTask(request_context context.Context, request SubmitHubDownloadTaskRequest) (*hub.Task, error) {
	target_device_id := request.target_device_id()
	if target_device_id == "" {
		return nil, errors.New("target_device_id 不能为空")
	}
	if (request.Request == nil) == (request.URLRequest == nil) {
		return nil, errors.New("request 和 url_request 必须且只能提供一个")
	}
	hub_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	task, err := hub_client.SubmitTask(request_context, hub.SubmitTaskRequest{
		Method:         hub.MethodDownloadCreate,
		TargetDeviceID: target_device_id,
		IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		Args: hub_download_args{
			Request:    request.Request,
			URLRequest: request.URLRequest,
		},
	})
	return task, err
}

// GetTask retrieves one task through the Hub.
func (s *HubService) GetTask(request_context context.Context, task_id string) (*hub.Task, error) {
	hub_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	task, err := hub_client.GetTask(request_context, task_id)
	return task, err
}

// ListTasks retrieves tasks through the Hub.
func (s *HubService) ListTasks(request_context context.Context, status string, limit int) ([]hub.Task, error) {
	hub_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	tasks, err := hub_client.ListTasks(request_context, status, limit)
	return tasks, err
}

func (s *HubService) resolve_client() (*hub.Client, error) {
	if s == nil {
		return nil, &hub_unavailable_error{message: "Hub 未配置"}
	}
	if s.config_error != nil {
		return nil, &hub_unavailable_error{message: s.config_error.Error()}
	}
	if s.client == nil {
		return nil, &hub_unavailable_error{message: "Hub 未配置"}
	}
	if !s.client.Status().Enabled {
		return nil, &hub_unavailable_error{message: "Hub 未启用"}
	}
	return s.client, nil
}

func (request SubmitHubWXChannelsTaskRequest) target_device_id() string {
	if target_device_id := strings.TrimSpace(request.TargetDeviceID); target_device_id != "" {
		return target_device_id
	}
	return strings.TrimSpace(request.LegacyTargetClientID)
}

func (request SubmitHubDownloadTaskRequest) target_device_id() string {
	if target_device_id := strings.TrimSpace(request.TargetDeviceID); target_device_id != "" {
		return target_device_id
	}
	return strings.TrimSpace(request.LegacyTargetClientID)
}

func (request SubmitHubCallRequest) target_device_id() string {
	if target_device_id := strings.TrimSpace(request.TargetDeviceID); target_device_id != "" {
		return target_device_id
	}
	return strings.TrimSpace(request.LegacyTargetClientID)
}

func (s *HubService) method_names() []string {
	methods := make([]string, 0, len(s.method_handlers))
	for method := range s.method_handlers {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func (s *HubService) execute_task(task_context context.Context, task hub.Task) (json.RawMessage, error) {
	if err := task_context.Err(); err != nil {
		return nil, err
	}
	handler := s.method_handlers[task.Method]
	if handler == nil {
		return nil, fmt.Errorf("当前设备未注册 Hub 方法: %s", task.Method)
	}
	return handler(task_context, task.Args)
}

func (s *HubService) execute_wxchannels_fetch(task_context context.Context, args json.RawMessage) (json.RawMessage, error) {
	var request hub_wxchannels_args
	if err := json.Unmarshal(args, &request); err != nil {
		return nil, fmt.Errorf("解析视频号 Hub 任务失败: %w", err)
	}
	request.URL = strings.TrimSpace(request.URL)
	if request.URL == "" {
		return nil, errors.New("视频号 Hub 任务缺少 url")
	}
	handler := adapter.Get("wxchannels")
	if handler == nil {
		return nil, errors.New("当前设备未安装 wxchannels adapter")
	}
	s.wxchannels_mu.Lock()
	defer s.wxchannels_mu.Unlock()
	if err := task_context.Err(); err != nil {
		return nil, err
	}
	content, err := handler.Fetch(request.URL)
	if err != nil {
		return nil, fmt.Errorf("获取视频号内容失败: %w", err)
	}
	data, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("编码视频号内容失败: %w", err)
	}
	return data, nil
}

func (s *HubService) execute_wxchannels_contact_search(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request hub_wxchannels_contact_search_args
	if err := decode_hub_method_args(args, &request); err != nil {
		return nil, err
	}
	request.Keyword = strings.TrimSpace(request.Keyword)
	if request.Keyword == "" {
		return nil, errors.New("keyword 不能为空")
	}
	wxchannels_adapter, err := s.resolve_wxchannels_adapter(task_context)
	if err != nil {
		return nil, err
	}
	response, err := wxchannels_adapter.SearchChannelsContact(request.Keyword, request.NextMarker)
	return encode_hub_method_result(task_context, response, err)
}

func (s *HubService) execute_wxchannels_contact_feed_list(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request hub_wxchannels_account_page_args
	if err := decode_hub_method_args(args, &request); err != nil {
		return nil, err
	}
	request.Username = strings.TrimSpace(request.Username)
	if request.Username == "" {
		return nil, errors.New("username 不能为空")
	}
	wxchannels_adapter, err := s.resolve_wxchannels_adapter(task_context)
	if err != nil {
		return nil, err
	}
	response, err := wxchannels_adapter.FetchChannelsFeedListOfContact(request.Username, request.NextMarker)
	return encode_hub_method_result(task_context, response, err)
}

func (s *HubService) execute_wxchannels_live_replay_list(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request hub_wxchannels_account_page_args
	if err := decode_hub_method_args(args, &request); err != nil {
		return nil, err
	}
	request.Username = strings.TrimSpace(request.Username)
	if request.Username == "" {
		return nil, errors.New("username 不能为空")
	}
	wxchannels_adapter, err := s.resolve_wxchannels_adapter(task_context)
	if err != nil {
		return nil, err
	}
	response, err := wxchannels_adapter.FetchChannelsLiveReplayList(request.Username, request.NextMarker)
	return encode_hub_method_result(task_context, response, err)
}

func (s *HubService) execute_wxchannels_feed_profile(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request hub_wxchannels_feed_profile_args
	if err := decode_hub_method_args(args, &request); err != nil {
		return nil, err
	}
	request.OID = strings.TrimSpace(request.OID)
	request.NID = strings.TrimSpace(request.NID)
	request.URL = strings.TrimSpace(request.URL)
	request.EID = strings.TrimSpace(request.EID)
	if request.OID == "" && request.URL == "" && request.EID == "" {
		return nil, errors.New("oid、url 和 eid 至少需要提供一个")
	}
	request.OID, request.NID, request.URL, request.EID = normalize_hub_wxchannels_feed_profile_args(
		request.OID,
		request.NID,
		request.URL,
		request.EID,
	)
	wxchannels_adapter, err := s.resolve_wxchannels_adapter(task_context)
	if err != nil {
		return nil, err
	}
	response, err := wxchannels_adapter.FetchChannelsFeedProfile(
		request.OID,
		request.NID,
		request.URL,
		request.EID,
	)
	return encode_hub_method_result(task_context, response, err)
}

func (s *HubService) execute_wxchannels_feed_comment_list(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request hub_wxchannels_feed_comment_list_args
	if err := decode_hub_method_args(args, &request); err != nil {
		return nil, err
	}
	request.OID = strings.TrimSpace(request.OID)
	request.NID = strings.TrimSpace(request.NID)
	request.CommentID = strings.TrimSpace(request.CommentID)
	if request.OID == "" {
		return nil, errors.New("oid 不能为空")
	}
	if request.NID == "" && request.CommentID == "" {
		return nil, errors.New("nid 和 comment_id 至少需要提供一个")
	}
	wxchannels_adapter, err := s.resolve_wxchannels_adapter(task_context)
	if err != nil {
		return nil, err
	}
	response, err := wxchannels_adapter.FetchChannelsFeedCommentList(
		request.OID,
		request.NID,
		request.CommentID,
		request.NextMarker,
	)
	return encode_hub_method_result(task_context, response, err)
}

func (s *HubService) execute_wxchannels_feed_share_url(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request hub_wxchannels_feed_share_url_args
	if err := decode_hub_method_args(args, &request); err != nil {
		return nil, err
	}
	request.OID = strings.TrimSpace(request.OID)
	if request.OID == "" {
		return nil, errors.New("oid 不能为空")
	}
	wxchannels_adapter, err := s.resolve_wxchannels_adapter(task_context)
	if err != nil {
		return nil, err
	}
	response, err := wxchannels_adapter.FetchChannelsFeedShareUrl(request.OID)
	return encode_hub_method_result(task_context, response, err)
}

func (s *HubService) resolve_wxchannels_adapter(
	task_context context.Context,
) (hub_wxchannels_adapter, error) {
	if err := task_context.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.wxchannels_adapter == nil {
		return nil, errors.New("当前设备未安装 wxchannels adapter 查询能力")
	}
	return s.wxchannels_adapter, nil
}

func decode_hub_method_args(args json.RawMessage, target any) error {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(args, target); err != nil {
		return fmt.Errorf("解析 Hub 方法参数失败: %w", err)
	}
	return nil
}

func encode_hub_method_result(
	task_context context.Context,
	response any,
	response_error error,
) (json.RawMessage, error) {
	if response_error != nil {
		return nil, response_error
	}
	if err := task_context.Err(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("编码 Hub 方法结果失败: %w", err)
	}
	return data, nil
}

func normalize_hub_wxchannels_feed_profile_args(
	oid string,
	nid string,
	request_url string,
	eid string,
) (string, string, string, string) {
	if eid == "" && request_url != "" {
		if parsed_url, err := url.Parse(request_url); err == nil {
			if parsed_eid := parsed_url.Query().Get("eid"); parsed_eid != "" {
				eid = parsed_eid
				request_url = ""
			}
		}
	}
	if oid != "" && nid != "" {
		request_url = ""
	}
	return oid, nid, request_url, eid
}

func (s *HubService) execute_download_create(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	if s.download_task_service == nil {
		return nil, errors.New("下载任务服务未初始化")
	}
	var request hub_download_args
	if err := json.Unmarshal(args, &request); err != nil {
		return nil, fmt.Errorf("解析远程下载任务失败: %w", err)
	}
	if request.Request != nil && request.URLRequest != nil {
		return nil, errors.New("request 和 url_request 只能提供一个")
	}
	if request.Request != nil {
		created, err := s.download_task_service.CreateTask(*request.Request)
		if err != nil {
			return nil, err
		}
		return json.Marshal(BuildDownloadTaskItem(created))
	}
	if request.URLRequest != nil {
		created, err := s.download_task_service.CreateTaskByURL(*request.URLRequest)
		if err != nil {
			return nil, err
		}
		return json.Marshal(created)
	}
	return nil, errors.New("远程下载任务缺少 request 或 url_request")
}

func (s *HubService) handle_terminal_task(task hub.Task) {
	if s == nil || task.Status != "completed" {
		return
	}
	hub_client := s.client
	if hub_client == nil {
		return
	}
	status := hub_client.Status()
	if task.PublisherDeviceID != status.DeviceID || task.Method != hub.MethodWXChannelsFetch {
		return
	}
	var request hub_wxchannels_args
	if err := json.Unmarshal(task.Args, &request); err != nil || request.Download == nil {
		return
	}
	if s.download_task_service == nil {
		return
	}
	_, err := s.download_task_service.CreateTask(CreateDownloadTaskBody{
		Platform:       "wxchannels",
		Content:        task.Result,
		BuildFromFetch: true,
		DownloadDir:    request.Download.DownloadDir,
		Filename:       request.Download.Filename,
		Config:         request.Download.Config,
		AutoStart:      request.Download.AutoStart,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("device_id", status.DeviceID).Str("hub_task_id", task.ID).Msg("failed to create local download from completed hub task")
		return
	}
	s.logger.Info().Str("device_id", status.DeviceID).Str("hub_task_id", task.ID).Msg("created local download from completed hub task")
}
