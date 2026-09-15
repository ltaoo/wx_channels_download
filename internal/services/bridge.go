package services

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"os"
	"runtime"
	"time"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	wxchannelsadapter "wx_channel/internal/adapter/wxchannels"
	"wx_channel/internal/bridge"
	"wx_channel/internal/config"
	"wx_channel/pkg/scraper/wxchannels"
)

const (
	bridge_method_wxchannels_contact_search    = "wxchannels.contact.search"
	bridge_method_wxchannels_contact_feed_list = "wxchannels.contact.feed.list"
	bridge_method_wxchannels_live_replay_list  = "wxchannels.live.replay.list"
	bridge_method_wxchannels_feed_profile      = "wxchannels.feed.profile"
	bridge_method_wxchannels_feed_comment_list = "wxchannels.feed.comment.list"
	bridge_method_wxchannels_feed_share_url    = "wxchannels.feed.share_url"
	bridge_method_wxmp_biz_msg_list            = "wxmp.biz.msg.list"
)

var bridge_device_id_pattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

type bridge_device_settings struct {
	Enabled            bool
	URL                string
	DeviceID           string
	DeviceName         string
	Token              string
	HTTPTimeoutSeconds int
	Methods            []string
}

type legacy_bridge_capability_settings struct {
	WXChannels bool `json:"wxchannels"`
	Download   bool `json:"download"`
}

type legacy_bridge_instance_settings struct {
	Name               string                             `json:"name"`
	Enabled            bool                               `json:"enabled"`
	URL                string                             `json:"url"`
	ID                 string                             `json:"id"`
	ClientID           string                             `json:"clientId"`
	Token              string                             `json:"token"`
	HTTPTimeoutSeconds int                                `json:"httpTimeoutSeconds"`
	Capabilities       *legacy_bridge_capability_settings `json:"capabilities"`
}

func load_bridge_config(application_config *config.Config) (*bridge.Config, error) {
	if application_config == nil {
		return nil, nil
	}
	configured_methods, err := parse_bridge_methods(application_config.GetString("bridge.methods"))
	if err != nil {
		return nil, err
	}
	settings := bridge_device_settings{
		Enabled:            application_config.GetBool("bridge.enabled"),
		URL:                application_config.GetString("bridge.url"),
		DeviceID:           application_config.GetString("bridge.deviceId"),
		DeviceName:         application_config.GetString("bridge.deviceName"),
		Token:              application_config.GetString("bridge.token"),
		HTTPTimeoutSeconds: application_config.GetInt("bridge.httpTimeoutSeconds"),
		Methods:            configured_methods,
	}
	if bridge_device_configured(settings) {
		return resolve_bridge_config(settings, "")
	}

	legacy_instances, err := decode_legacy_bridge_instances(application_config.GetRaw("bridge.instances"))
	if err != nil {
		return nil, err
	}
	if len(legacy_instances) == 0 {
		return nil, nil
	}
	if len(legacy_instances) > 1 {
		return nil, errors.New("当前版本一个设备只能连接一个 Bridge；请将 bridge.instances 迁移为单 Bridge 配置")
	}
	legacy := legacy_instances[0]
	var legacy_methods []string
	if legacy.Capabilities != nil {
		legacy_methods = []string{}
		if legacy.Capabilities.WXChannels {
			legacy_methods = append(legacy_methods, bridge.MethodWXChannelsFetch)
		}
		if legacy.Capabilities.Download {
			legacy_methods = append(legacy_methods, bridge.MethodDownloadCreate)
		}
	}
	return resolve_bridge_config(
		bridge_device_settings{
			Enabled:            legacy.Enabled,
			URL:                legacy.URL,
			DeviceID:           legacy.ClientID,
			DeviceName:         legacy.ClientID,
			Token:              legacy.Token,
			HTTPTimeoutSeconds: legacy.HTTPTimeoutSeconds,
			Methods:            legacy_methods,
		},
		strings.TrimSpace(legacy.ID),
	)
}

func bridge_device_configured(settings bridge_device_settings) bool {
	return settings.Enabled ||
		strings.TrimSpace(settings.URL) != "" ||
		strings.TrimSpace(settings.DeviceID) != "" ||
		strings.TrimSpace(settings.DeviceName) != "" ||
		strings.TrimSpace(settings.Token) != "" ||
		settings.Methods != nil
}

func resolve_bridge_config(settings bridge_device_settings, legacy_bridge_id string) (*bridge.Config, error) {
	hostname, _ := os.Hostname()
	device_name := strings.TrimSpace(settings.DeviceName)
	if device_name == "" {
		device_name = strings.TrimSpace(hostname)
	}
	if device_name == "" {
		device_name = "Unnamed device"
	}
	if len(device_name) > 128 {
		return nil, errors.New("bridge.deviceName 不能超过 128 个字符")
	}
	if strings.ContainsAny(device_name, "\r\n") {
		return nil, errors.New("bridge.deviceName 不能包含换行符")
	}
	device_id := strings.TrimSpace(settings.DeviceID)
	if device_id == "" {
		device_id = normalize_device_id(hostname)
	}

	http_timeout_seconds := settings.HTTPTimeoutSeconds
	if http_timeout_seconds <= 0 {
		http_timeout_seconds = 30
	}
	var configured_methods []string
	if settings.Methods != nil {
		configured_methods = append([]string{}, settings.Methods...)
	}
	bridge_config := &bridge.Config{
		Enabled:        settings.Enabled,
		URL:            strings.TrimSpace(settings.URL),
		DeviceID:       device_id,
		DeviceName:     device_name,
		DeviceOS:       runtime.GOOS,
		Token:          strings.TrimSpace(settings.Token),
		Methods:        configured_methods,
		HTTPTimeout:    time.Duration(http_timeout_seconds) * time.Second,
		LegacyBridgeID: legacy_bridge_id,
	}
	if !settings.Enabled {
		return bridge_config, nil
	}
	if bridge_config.URL == "" {
		return nil, errors.New("bridge.url 不能为空")
	}
	parsed_url, err := url.Parse(bridge_config.URL)
	if err != nil || parsed_url.Host == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
		return nil, errors.New("bridge.url 必须是 HTTP 或 HTTPS URL")
	}
	if !bridge_device_id_pattern.MatchString(bridge_config.DeviceID) {
		return nil, errors.New("bridge.deviceId 不合法；仅支持字母、数字、点、下划线、冒号和连字符")
	}
	if bridge_config.Token == "" {
		return nil, errors.New("bridge.token 不能为空")
	}
	return bridge_config, nil
}

func parse_bridge_methods(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "auto") {
		return nil, nil
	}
	if strings.EqualFold(value, "none") {
		return []string{}, nil
	}
	methods := make([]string, 0)
	seen_methods := make(map[string]struct{})
	for _, item := range strings.Split(value, ",") {
		method := strings.TrimSpace(item)
		if !bridge_device_id_pattern.MatchString(method) {
			return nil, fmt.Errorf("bridge.methods 包含不合法的方法名 %q", method)
		}
		if _, exists := seen_methods[method]; exists {
			continue
		}
		seen_methods[method] = struct{}{}
		methods = append(methods, method)
	}
	return methods, nil
}

func normalize_device_id(value string) string {
	value = strings.TrimSpace(value)
	var result strings.Builder
	result.Grow(len(value))
	for _, character := range value {
		valid := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == ':' || character == '-'
		if valid {
			result.WriteRune(character)
		} else if result.Len() > 0 {
			result.WriteByte('-')
		}
		if result.Len() >= 128 {
			break
		}
	}
	device_id := strings.Trim(result.String(), "-.:_")
	if device_id == "" {
		return "device"
	}
	return device_id
}

func decode_legacy_bridge_instances(raw_instances any) ([]legacy_bridge_instance_settings, error) {
	if raw_instances == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw_instances)
	if err != nil {
		return nil, fmt.Errorf("编码 bridge.instances 失败: %w", err)
	}
	if string(data) == "null" || string(data) == "[]" {
		return nil, nil
	}
	var settings []legacy_bridge_instance_settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("解析 bridge.instances 失败: %w", err)
	}
	if settings == nil {
		return nil, errors.New("bridge.instances 必须是数组")
	}
	return settings, nil
}


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

type bridge_wxchannels_account struct {
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Signature string `json:"signature"`
}

type bridge_wxchannels_article struct {
	ID          string `json:"id"`
	Idx         int    `json:"idx"`
	Title       string `json:"title"`
	Digest      string `json:"digest"`
	URL         string `json:"url"`
	SourceURL   string `json:"source_url"`
	CoverURL    string `json:"cover_url"`
	DecodeKey   string `json:"decode_key"`
	PublishTime int64  `json:"publish_time"`
}

type bridge_wxchannels_article_list struct {
	Account  bridge_wxchannels_account   `json:"account"`
	Articles []bridge_wxchannels_article `json:"articles"`
	Offset   string                      `json:"offset"`
	IsEnd    bool                        `json:"is_end"`
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

type bridge_wxmp_biz_msg_list_args struct {
	Username string `json:"username"`
	Offset   string `json:"offset"`
}

type bridge_wxchannels_adapter interface {
	SearchChannelsContact(keyword string, next_marker string) (json.RawMessage, error)
	FetchChannelsFeedListOfContact(username string, next_marker string) (json.RawMessage, error)
	FetchChannelsLiveReplayList(username string, next_marker string) (json.RawMessage, error)
	FetchChannelsFeedProfile(oid string, nid string, request_url string, eid string) (json.RawMessage, error)
	FetchChannelsFeedCommentList(oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error)
	FetchChannelsFeedShareUrl(oid string) (json.RawMessage, error)
}

type bridge_wxmp_adapter interface {
	FetchBizMsgList(username string, offset string) (json.RawMessage, error)
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
	// download_task_service *DownloadTaskService
	logger                zerolog.Logger
	config_error          error
	wxchannels_mu         sync.Mutex
	wxchannels_adapter    bridge_wxchannels_adapter
	wxmp_adapter          bridge_wxmp_adapter
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
		// download_task_service: options.DownloadTaskService,
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
	wxmp_handler := adapter.Get("wxmp")
	if wxmp_adapter, ok := wxmp_handler.(bridge_wxmp_adapter); ok {
		service.wxmp_adapter = wxmp_adapter
		service.method_handlers[bridge_method_wxmp_biz_msg_list] = service.execute_wxmp_biz_msg_list
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
	if err != nil {
		return encode_bridge_method_result(task_context, nil, err)
	}
	result, err := normalize_bridge_wxchannels_article_list(response)
	return encode_bridge_method_result(task_context, result, err)
}

func normalize_bridge_wxchannels_article_list(response_json json.RawMessage) (*bridge_wxchannels_article_list, error) {
	var response wxchannels.ChannelsFeedListOfAccountResp
	if err := json.Unmarshal(response_json, &response); err != nil {
		return nil, fmt.Errorf("解析视频号视频列表失败: %w", err)
	}
	if response.ErrCode != 0 {
		message := strings.TrimSpace(response.ErrMsg)
		if message == "" {
			message = fmt.Sprintf("视频号返回错误码 %d", response.ErrCode)
		}
		return nil, errors.New(message)
	}
	if response.Data.BaseResponse.Ret != 0 {
		message := strings.TrimSpace(response.Data.BaseResponse.ErrMsg.String)
		if message == "" {
			message = fmt.Sprintf("视频号返回错误码 %d", response.Data.BaseResponse.Ret)
		}
		return nil, errors.New(message)
	}

	contact := response.Data.Contact
	result := &bridge_wxchannels_article_list{
		Account: bridge_wxchannels_account{
			Username:  strings.TrimSpace(contact.Username),
			Nickname:  strings.TrimSpace(contact.Nickname),
			AvatarURL: strings.TrimSpace(contact.HeadUrl),
			Signature: strings.TrimSpace(contact.Signature),
		},
		Articles: make([]bridge_wxchannels_article, 0, len(response.Data.Object)),
		Offset:   response.Data.LastBuffer,
		IsEnd:    response.Data.ContinueFlag == 0,
	}
	for index := range response.Data.Object {
		object := &response.Data.Object[index]
		if object.ObjectDesc.MediaType != wxchannels.MediaTypeVideo {
			continue
		}
		result.Articles = append(result.Articles, bridge_wxchannels_article{
			ID:          strings.TrimSpace(object.ID),
			Idx:         len(result.Articles),
			Title:       bridge_wxchannels_article_title(object),
			Digest:      object.ObjectDesc.Description,
			URL:         bridge_wxchannels_article_url(object),
			SourceURL:   bridge_wxchannels_article_source_url(object, contact.Username),
			CoverURL:    bridge_wxchannels_article_cover_url(object),
			DecodeKey:   bridge_wxchannels_article_decode_key(object),
			PublishTime: int64(object.CreateTime),
		})
	}
	return result, nil
}

func bridge_wxchannels_article_title(object *wxchannels.ChannelsObject) string {
	for _, short_title := range object.ObjectDesc.ShortTitle {
		if title := strings.TrimSpace(short_title.ShortTitle); title != "" {
			return title
		}
	}
	return object.ObjectDesc.Description
}

// bridge_wxchannels_article_source_url builds the feed page URL of the object.
// The URL returned by the upstream response wins over the generated one.
func bridge_wxchannels_article_source_url(object *wxchannels.ChannelsObject, account_username string) string {
	object_username := strings.TrimSpace(object.Contact.Username)
	if object_username == "" {
		object_username = strings.TrimSpace(account_username)
	}
	return wxchannelsadapter.BuildJumpURLFromParts(
		object.ID,
		object.ObjectNonceId,
		strings.TrimSpace(object.SourceURL),
		object_username,
	)
}

func bridge_wxchannels_article_cover_url(object *wxchannels.ChannelsObject) string {
	if len(object.ObjectDesc.Media) == 0 {
		return ""
	}
	media := object.ObjectDesc.Media[0]
	if cover_url := strings.TrimSpace(media.ThumbUrl); cover_url != "" {
		return cover_url
	}
	return strings.TrimSpace(media.CoverUrl)
}

func bridge_wxchannels_article_decode_key(object *wxchannels.ChannelsObject) string {
	if len(object.ObjectDesc.Media) == 0 {
		return ""
	}
	return strings.TrimSpace(object.ObjectDesc.Media[0].DecodeKey)
}

// bridge_wxchannels_article_url builds the playable media URL from media[0].
// The file format spec is appended as a query parameter only when present.
func bridge_wxchannels_article_url(object *wxchannels.ChannelsObject) string {
	if len(object.ObjectDesc.Media) == 0 {
		return ""
	}
	media := object.ObjectDesc.Media[0]
	if strings.TrimSpace(media.URL) == "" {
		return ""
	}
	article_url := media.URL + media.URLToken
	if len(media.Spec) == 0 {
		return article_url
	}
	file_format := strings.TrimSpace(media.Spec[0].FileFormat)
	if file_format == "" {
		return article_url
	}
	return article_url + "&X-snsvideoflag=" + file_format
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

func (s *BridgeService) execute_wxmp_biz_msg_list(
	task_context context.Context,
	args json.RawMessage,
) (json.RawMessage, error) {
	var request bridge_wxmp_biz_msg_list_args
	if err := decode_bridge_method_args(args, &request); err != nil {
		return nil, err
	}
	request.Username = strings.TrimSpace(request.Username)
	request.Offset = strings.TrimSpace(request.Offset)
	if request.Username == "" {
		return nil, errors.New("username 不能为空")
	}
	if err := task_context.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.wxmp_adapter == nil {
		return nil, errors.New("当前设备未安装 wxmp adapter 查询能力")
	}
	response, err := s.wxmp_adapter.FetchBizMsgList(request.Username, request.Offset)
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

