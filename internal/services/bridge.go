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
	"wx_channel/internal/bridge"
	"wx_channel/internal/config"
)

const (
	bridge_method_wxchannels_contact_search    = "wxchannels.contact.search"
	bridge_method_wxchannels_contact_feed_list = "wxchannels.contact.feed.list"
	bridge_method_wxchannels_live_replay_list  = "wxchannels.live.replay.list"
	bridge_method_wxchannels_feed_profile      = "wxchannels.feed.profile"
	bridge_method_wxchannels_feed_comment_list = "wxchannels.feed.comment.list"
	bridge_method_wxchannels_feed_share_url    = "wxchannels.feed.share_url"
)

// BridgeServiceOptions contains the dependencies and configuration for BridgeService.
type BridgeServiceOptions struct {
	ApplicationConfig   *config.Config
	Config              *bridge.Config
	DownloadTaskService *DownloadTaskService
	MethodHandlers      map[string]BridgeMethodHandler
	Logger              *zerolog.Logger
}

// BridgeMethodHandler handles the args object for one registered Bridge method.
type BridgeMethodHandler func(context.Context, json.RawMessage) (json.RawMessage, error)

// BridgeWXChannelsDownloadOptions controls local download creation after a remote fetch.
type BridgeWXChannelsDownloadOptions struct {
	DownloadDir string         `json:"download_dir"`
	Filename    string         `json:"filename"`
	Config      map[string]any `json:"config"`
	AutoStart   *bool          `json:"auto_start"`
}

// SubmitBridgeWXChannelsTaskRequest describes one remote wxchannels fetch request.
type SubmitBridgeWXChannelsTaskRequest struct {
	URL                  string                           `json:"url"`
	TargetDeviceID       string                           `json:"target_device_id"`
	LegacyTargetClientID string                           `json:"target_client_id,omitempty"`
	IdempotencyKey       string                           `json:"idempotency_key"`
	Download             *BridgeWXChannelsDownloadOptions `json:"download,omitempty"`
}

// SubmitBridgeDownloadTaskRequest describes one remote download creation request.
type SubmitBridgeDownloadTaskRequest struct {
	TargetDeviceID       string                       `json:"target_device_id"`
	LegacyTargetClientID string                       `json:"target_client_id,omitempty"`
	IdempotencyKey       string                       `json:"idempotency_key"`
	Request              *CreateDownloadTaskBody      `json:"request,omitempty"`
	URLRequest           *CreateDownloadTaskByURLBody `json:"url_request,omitempty"`
}

// SubmitBridgeCallRequest describes one generic CGI-style method call.
type SubmitBridgeCallRequest struct {
	Method               string         `json:"method"`
	Args                 map[string]any `json:"args"`
	TargetDeviceID       string         `json:"target_device_id"`
	LegacyTargetClientID string         `json:"target_client_id,omitempty"`
	IdempotencyKey       string         `json:"idempotency_key"`
}

type bridge_wxchannels_args struct {
	URL      string                           `json:"url"`
	Download *BridgeWXChannelsDownloadOptions `json:"download,omitempty"`
}

type bridge_download_args struct {
	Request    *CreateDownloadTaskBody      `json:"request,omitempty"`
	URLRequest *CreateDownloadTaskByURLBody `json:"url_request,omitempty"`
}

type bridge_wxchannels_contact_search_args struct {
	Keyword    string `json:"keyword"`
	NextMarker string `json:"next_marker"`
}

type bridge_wxchannels_account_page_args struct {
	Username   string `json:"username"`
	NextMarker string `json:"next_marker"`
}

type bridge_wxchannels_feed_profile_args struct {
	OID string `json:"oid"`
	NID string `json:"nid"`
	URL string `json:"url"`
	EID string `json:"eid"`
}

type bridge_wxchannels_feed_comment_list_args struct {
	OID        string `json:"oid"`
	NID        string `json:"nid"`
	CommentID  string `json:"comment_id"`
	NextMarker string `json:"next_marker"`
}

type bridge_wxchannels_feed_share_url_args struct {
	OID string `json:"oid"`
}

type bridge_wxchannels_adapter interface {
	SearchChannelsContact(keyword string, next_marker string) (json.RawMessage, error)
	FetchChannelsFeedListOfContact(username string, next_marker string) (json.RawMessage, error)
	FetchChannelsLiveReplayList(username string, next_marker string) (json.RawMessage, error)
	FetchChannelsFeedProfile(oid string, nid string, request_url string, eid string) (json.RawMessage, error)
	FetchChannelsFeedCommentList(oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error)
	FetchChannelsFeedShareUrl(oid string) (json.RawMessage, error)
}

type bridge_unavailable_error struct {
	message string
}

func (e *bridge_unavailable_error) Error() string {
	return e.message
}

// IsBridgeUnavailableError reports whether the single Bridge connection is unavailable.
func IsBridgeUnavailableError(err error) bool {
	var unavailable_error *bridge_unavailable_error
	return errors.As(err, &unavailable_error)
}

// BridgeService owns Bridge clients, their lifecycle, and application task dispatch.
type BridgeService struct {
	client                *bridge.Client
	download_task_service *DownloadTaskService
	logger                zerolog.Logger
	config_error          error
	wxchannels_mu         sync.Mutex
	wxchannels_adapter    bridge_wxchannels_adapter
	method_handlers       map[string]BridgeMethodHandler
}

// NewBridgeService creates the current operating-system device's single Bridge client.
func NewBridgeService(options BridgeServiceOptions) *BridgeService {
	logger := zerolog.Nop()
	if options.Logger != nil {
		logger = options.Logger.With().Str("component", "bridge_service").Logger()
	}
	bridge_config := options.Config
	var config_error error
	if options.ApplicationConfig != nil {
		bridge_config, config_error = load_bridge_config(options.ApplicationConfig)
		if config_error != nil {
			logger.Error().Err(config_error).Msg("Bridge 配置无效，Bridge 服务将保持不可用")
		}
	}
	service := &BridgeService{
		download_task_service: options.DownloadTaskService,
		logger:                logger,
		config_error:          config_error,
		method_handlers:       make(map[string]BridgeMethodHandler),
	}
	wxchannels_handler := adapter.Get("wxchannels")
	if wxchannels_handler != nil {
		service.method_handlers[bridge.MethodWXChannelsFetch] = service.execute_wxchannels_fetch
		if wxchannels_adapter, ok := wxchannels_handler.(bridge_wxchannels_adapter); ok {
			service.wxchannels_adapter = wxchannels_adapter
			service.register_wxchannels_methods()
		}
	}
	if service.download_task_service != nil {
		service.method_handlers[bridge.MethodDownloadCreate] = service.execute_download_create
	}
	for method, handler := range options.MethodHandlers {
		if handler != nil {
			service.method_handlers[strings.TrimSpace(method)] = handler
		}
	}
	if bridge_config != nil {
		if bridge_config.Methods != nil {
			selected_handlers := make(map[string]BridgeMethodHandler)
			for _, method := range bridge_config.Methods {
				handler := service.method_handlers[method]
				if handler == nil {
					logger.Warn().Str("method", method).Msg("Bridge 配置的方法在当前设备上没有处理函数")
					continue
				}
				selected_handlers[method] = handler
			}
			service.method_handlers = selected_handlers
		}
		resolved_config := *bridge_config
		resolved_config.Methods = service.method_names()
		service.client = bridge.NewClient(
			resolved_config,
			service.execute_task,
			service.handle_terminal_task,
			&service.logger,
		)
	}
	return service
}

func (s *BridgeService) register_wxchannels_methods() {
	if s == nil || s.wxchannels_adapter == nil {
		return
	}
	if s.method_handlers == nil {
		s.method_handlers = make(map[string]BridgeMethodHandler)
	}
	s.method_handlers[bridge_method_wxchannels_contact_search] = s.execute_wxchannels_contact_search
	s.method_handlers[bridge_method_wxchannels_contact_feed_list] = s.execute_wxchannels_contact_feed_list
	s.method_handlers[bridge_method_wxchannels_live_replay_list] = s.execute_wxchannels_live_replay_list
	s.method_handlers[bridge_method_wxchannels_feed_profile] = s.execute_wxchannels_feed_profile
	s.method_handlers[bridge_method_wxchannels_feed_comment_list] = s.execute_wxchannels_feed_comment_list
	s.method_handlers[bridge_method_wxchannels_feed_share_url] = s.execute_wxchannels_feed_share_url
}

// Start validates and starts the configured Bridge client.
func (s *BridgeService) Start(parent_context context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	if err := s.client.Start(parent_context); err != nil {
		return fmt.Errorf("启动 Bridge 失败: %w", err)
	}
	return nil
}

// Close stops all configured Bridge clients.
func (s *BridgeService) Close() {
	if s == nil {
		return
	}
	if s.client != nil {
		s.client.Close()
	}
}

// Status returns this operating-system device's Bridge connection status.
func (s *BridgeService) Status() (bridge.Status, error) {
	empty_status := bridge.Status{Methods: []string{}}
	if s == nil {
		return empty_status, &bridge_unavailable_error{message: "Bridge 未配置"}
	}
	if s.config_error != nil {
		return empty_status, &bridge_unavailable_error{message: s.config_error.Error()}
	}
	if s.client == nil {
		return empty_status, &bridge_unavailable_error{message: "Bridge 未配置"}
	}
	return s.client.Status(), nil
}

// Call publishes one generic method invocation through the Bridge.
func (s *BridgeService) Call(request_context context.Context, request SubmitBridgeCallRequest) (*bridge.Task, error) {
	method := strings.TrimSpace(request.Method)
	if method == "" {
		return nil, errors.New("method 不能为空")
	}
	bridge_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	task, err := bridge_client.SubmitTask(request_context, bridge.SubmitTaskRequest{
		Method:         method,
		TargetDeviceID: request.target_device_id(),
		IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		Args:           request.Args,
	})
	return task, err
}

// SubmitWXChannelsTask publishes a wxchannels fetch task through the Bridge.
func (s *BridgeService) SubmitWXChannelsTask(request_context context.Context, request SubmitBridgeWXChannelsTaskRequest) (*bridge.Task, error) {
	request.URL = strings.TrimSpace(request.URL)
	if request.URL == "" {
		return nil, errors.New("url 不能为空")
	}
	bridge_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	task, err := bridge_client.SubmitTask(request_context, bridge.SubmitTaskRequest{
		Method:         bridge.MethodWXChannelsFetch,
		TargetDeviceID: request.target_device_id(),
		IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		Args: bridge_wxchannels_args{
			URL:      request.URL,
			Download: request.Download,
		},
	})
	return task, err
}

// SubmitDownloadTask publishes a download creation task through the Bridge.
func (s *BridgeService) SubmitDownloadTask(request_context context.Context, request SubmitBridgeDownloadTaskRequest) (*bridge.Task, error) {
	target_device_id := request.target_device_id()
	if target_device_id == "" {
		return nil, errors.New("target_device_id 不能为空")
	}
	if (request.Request == nil) == (request.URLRequest == nil) {
		return nil, errors.New("request 和 url_request 必须且只能提供一个")
	}
	bridge_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	task, err := bridge_client.SubmitTask(request_context, bridge.SubmitTaskRequest{
		Method:         bridge.MethodDownloadCreate,
		TargetDeviceID: target_device_id,
		IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		Args: bridge_download_args{
			Request:    request.Request,
			URLRequest: request.URLRequest,
		},
	})
	return task, err
}

// GetTask retrieves one task through the Bridge.
func (s *BridgeService) GetTask(request_context context.Context, task_id string) (*bridge.Task, error) {
	bridge_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	task, err := bridge_client.GetTask(request_context, task_id)
	return task, err
}

// ListTasks retrieves tasks through the Bridge.
func (s *BridgeService) ListTasks(request_context context.Context, status string, limit int) ([]bridge.Task, error) {
	bridge_client, err := s.resolve_client()
	if err != nil {
		return nil, err
	}
	tasks, err := bridge_client.ListTasks(request_context, status, limit)
	return tasks, err
}

func (s *BridgeService) resolve_client() (*bridge.Client, error) {
	if s == nil {
		return nil, &bridge_unavailable_error{message: "Bridge 未配置"}
	}
	if s.config_error != nil {
		return nil, &bridge_unavailable_error{message: s.config_error.Error()}
	}
	if s.client == nil {
		return nil, &bridge_unavailable_error{message: "Bridge 未配置"}
	}
	if !s.client.Status().Enabled {
		return nil, &bridge_unavailable_error{message: "Bridge 未启用"}
	}
	return s.client, nil
}

func (request SubmitBridgeWXChannelsTaskRequest) target_device_id() string {
	if target_device_id := strings.TrimSpace(request.TargetDeviceID); target_device_id != "" {
		return target_device_id
	}
	return strings.TrimSpace(request.LegacyTargetClientID)
}

func (request SubmitBridgeDownloadTaskRequest) target_device_id() string {
	if target_device_id := strings.TrimSpace(request.TargetDeviceID); target_device_id != "" {
		return target_device_id
	}
	return strings.TrimSpace(request.LegacyTargetClientID)
}

func (request SubmitBridgeCallRequest) target_device_id() string {
	if target_device_id := strings.TrimSpace(request.TargetDeviceID); target_device_id != "" {
		return target_device_id
	}
	return strings.TrimSpace(request.LegacyTargetClientID)
}

func (s *BridgeService) method_names() []string {
	methods := make([]string, 0, len(s.method_handlers))
	for method := range s.method_handlers {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func (s *BridgeService) execute_task(task_context context.Context, task bridge.Task) (json.RawMessage, error) {
	if err := task_context.Err(); err != nil {
		return nil, err
	}
	handler := s.method_handlers[task.Method]
	if handler == nil {
		return nil, fmt.Errorf("当前设备未注册 Bridge 方法: %s", task.Method)
	}
	return handler(task_context, task.Args)
}

func (s *BridgeService) execute_wxchannels_fetch(task_context context.Context, args json.RawMessage) (json.RawMessage, error) {
	var request bridge_wxchannels_args
	if err := json.Unmarshal(args, &request); err != nil {
		return nil, fmt.Errorf("解析视频号 Bridge 任务失败: %w", err)
	}
	request.URL = strings.TrimSpace(request.URL)
	if request.URL == "" {
		return nil, errors.New("视频号 Bridge 任务缺少 url")
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

func (s *BridgeService) execute_wxchannels_contact_search(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request bridge_wxchannels_contact_search_args
	if err := decode_bridge_method_args(args, &request); err != nil {
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
	return encode_bridge_method_result(task_context, response, err)
}

func (s *BridgeService) execute_wxchannels_contact_feed_list(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request bridge_wxchannels_account_page_args
	if err := decode_bridge_method_args(args, &request); err != nil {
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
	return encode_bridge_method_result(task_context, response, err)
}

func (s *BridgeService) execute_wxchannels_live_replay_list(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request bridge_wxchannels_account_page_args
	if err := decode_bridge_method_args(args, &request); err != nil {
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
	return encode_bridge_method_result(task_context, response, err)
}

func (s *BridgeService) execute_wxchannels_feed_profile(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request bridge_wxchannels_feed_profile_args
	if err := decode_bridge_method_args(args, &request); err != nil {
		return nil, err
	}
	request.OID = strings.TrimSpace(request.OID)
	request.NID = strings.TrimSpace(request.NID)
	request.URL = strings.TrimSpace(request.URL)
	request.EID = strings.TrimSpace(request.EID)
	if request.OID == "" && request.URL == "" && request.EID == "" {
		return nil, errors.New("oid、url 和 eid 至少需要提供一个")
	}
	request.OID, request.NID, request.URL, request.EID = normalize_bridge_wxchannels_feed_profile_args(
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
	return encode_bridge_method_result(task_context, response, err)
}

func (s *BridgeService) execute_wxchannels_feed_comment_list(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request bridge_wxchannels_feed_comment_list_args
	if err := decode_bridge_method_args(args, &request); err != nil {
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
	return encode_bridge_method_result(task_context, response, err)
}

func (s *BridgeService) execute_wxchannels_feed_share_url(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request bridge_wxchannels_feed_share_url_args
	if err := decode_bridge_method_args(args, &request); err != nil {
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
	return encode_bridge_method_result(task_context, response, err)
}

func (s *BridgeService) resolve_wxchannels_adapter(
	task_context context.Context,
) (bridge_wxchannels_adapter, error) {
	if err := task_context.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.wxchannels_adapter == nil {
		return nil, errors.New("当前设备未安装 wxchannels adapter 查询能力")
	}
	return s.wxchannels_adapter, nil
}

func decode_bridge_method_args(args json.RawMessage, target any) error {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(args, target); err != nil {
		return fmt.Errorf("解析 Bridge 方法参数失败: %w", err)
	}
	return nil
}

func encode_bridge_method_result(
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
		return nil, fmt.Errorf("编码 Bridge 方法结果失败: %w", err)
	}
	return data, nil
}

func normalize_bridge_wxchannels_feed_profile_args(
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

func (s *BridgeService) execute_download_create(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	if s.download_task_service == nil {
		return nil, errors.New("下载任务服务未初始化")
	}
	var request bridge_download_args
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

func (s *BridgeService) handle_terminal_task(task bridge.Task) {
	if s == nil || task.Status != "completed" {
		return
	}
	bridge_client := s.client
	if bridge_client == nil {
		return
	}
	status := bridge_client.Status()
	if task.PublisherDeviceID != status.DeviceID || task.Method != bridge.MethodWXChannelsFetch {
		return
	}
	var request bridge_wxchannels_args
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
		s.logger.Error().Err(err).Str("device_id", status.DeviceID).Str("bridge_task_id", task.ID).Msg("failed to create local download from completed bridge task")
		return
	}
	s.logger.Info().Str("device_id", status.DeviceID).Str("bridge_task_id", task.ID).Msg("created local download from completed bridge task")
}
