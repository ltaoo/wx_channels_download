package wxmpadapter

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/config"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/scraper/wxmp"
)

var jsapi_category_paths = []string{
	"/api/mp/jsapi/control",
	"/api/mp/jsapi/network_auth",
	"/api/mp/jsapi/page_navigation",
	"/api/mp/jsapi/mini_program_finder",
	"/api/mp/jsapi/contact_share_card",
	"/api/mp/jsapi/media_audio",
	"/api/mp/jsapi/ad_download_app",
	"/api/mp/jsapi/reporting",
	"/api/mp/jsapi/events",
}

// These numeric codes belong to the wxmp HTTP API contract. The scraper only
// exposes ErrorKind values and does not depend on this transport convention.
const (
	api_code_invalid_params      = 400
	api_code_missing_biz         = 4001
	api_code_missing_url         = 4002
	api_code_missing_refresh_uri = 4004
	api_code_token_invalid       = 1002
	api_code_account_not_found   = 1003
	api_code_account_expired     = 1004
	api_code_account_banned      = 1005
	api_code_proxy_request       = 2000
	api_code_proxy_dispatch      = 2001
	api_code_fetch_message       = 2002
	api_code_data_parse          = 2003
	api_code_fetch_page_content  = 2004
	api_code_jsapi               = 2005
	api_code_client_not_ready    = 5001
	api_code_timeout             = 5002
	api_code_client_busy         = 5003
)

var api_error_messages = map[int]string{
	api_code_invalid_params:      "参数错误",
	api_code_missing_biz:         "缺少参数：biz",
	api_code_missing_url:         "缺少参数：url",
	api_code_missing_refresh_uri: "缺少参数：refresh_uri",
	api_code_token_invalid:       "令牌无效",
	api_code_account_not_found:   "未找到匹配的公众号",
	api_code_account_expired:     "公众号凭证已失效",
	api_code_account_banned:      "账号被封禁",
	api_code_proxy_request:       "代理请求创建失败",
	api_code_proxy_dispatch:      "代理请求转发失败",
	api_code_fetch_message:       "获取消息列表失败",
	api_code_data_parse:          "数据解析失败",
	api_code_fetch_page_content:  "获取页面内容失败",
	api_code_jsapi:               "JSAPI 调用失败",
	api_code_client_not_ready:    "请先初始化客户端 socket 连接",
	api_code_timeout:             "请求超时",
	api_code_client_busy:         "发送缓冲区已满，请稍后重试",
}

func write_api_error(ctx *gin.Context, code int) {
	message, ok := api_error_messages[code]
	if !ok {
		message = "未知错误"
	}
	result.Err(ctx, code, message)
}

func api_code_for_error_kind(kind wxmp.ErrorKind, fallback_code int) int {
	switch kind {
	case wxmp.ErrorKindInvalidArgument:
		return api_code_invalid_params
	case wxmp.ErrorKindMissingBiz:
		return api_code_missing_biz
	case wxmp.ErrorKindMissingURL:
		return api_code_missing_url
	case wxmp.ErrorKindMissingRefreshURI:
		return api_code_missing_refresh_uri
	case wxmp.ErrorKindTokenInvalid:
		return api_code_token_invalid
	case wxmp.ErrorKindAccountNotFound:
		return api_code_account_not_found
	case wxmp.ErrorKindAccountExpired:
		return api_code_account_expired
	case wxmp.ErrorKindAccountBanned:
		return api_code_account_banned
	case wxmp.ErrorKindProxyRequest:
		return api_code_proxy_request
	case wxmp.ErrorKindProxyDispatch:
		return api_code_proxy_dispatch
	case wxmp.ErrorKindFetchMessage:
		return api_code_fetch_message
	case wxmp.ErrorKindFetchPageContent:
		return api_code_fetch_page_content
	case wxmp.ErrorKindJSAPI:
		return api_code_jsapi
	case wxmp.ErrorKindDataParse:
		return api_code_data_parse
	case wxmp.ErrorKindClientNotReady:
		return api_code_client_not_ready
	case wxmp.ErrorKindTimeout:
		return api_code_timeout
	case wxmp.ErrorKindClientBusy:
		return api_code_client_busy
	default:
		return fallback_code
	}
}

// Routes owns the official-account server lifecycle and endpoints.
type Routes struct {
	server *wxmp.OfficialAccountServer
}

func NewRoutes(cfg *config.Config, logger *zerolog.Logger) *Routes {
	if cfg == nil || logger == nil {
		return &Routes{}
	}
	server := wxmp.NewOfficialAccountServer(new_official_account_config(cfg), logger)
	return &Routes{server: server}
}

func (r *Routes) set_persistent_cache(file_cache *cache.CacheProvider) {
	if r == nil || r.server == nil {
		return
	}
	r.server.SetPersistentCache(file_cache)
}

// RegisterRoutes installs the previously local-only official-account routes.
func (r *Routes) RegisterRoutes(registrar adapter.RouteRegistrar) {
	if r == nil || r.server == nil || registrar == nil {
		return
	}
	registrar.RegisterGET(wxmp.WebsocketPath, r.handle_websocket)
	registrar.RegisterGET("/api/mp/ws_pool", r.handle_fetch_official_account_clients)
	registrar.RegisterGET("/api/mp/list", r.handle_fetch_list)
	registrar.RegisterGET("/api/mp/msg/list", r.handle_fetch_message_list)
	registrar.RegisterGET("/api/mp/article/list", r.handle_fetch_article_list)
	registrar.RegisterGET("/api/mp/biz/msg/list", r.handle_fetch_biz_msg_list)
	registrar.RegisterGET("/api/mp/page/content", r.handle_fetch_page_content)
	// registrar.RegisterPOST("/api/mp/jsapi", r.handle_jsapi)
	// for _, jsapi_category_path := range jsapi_category_paths {
	// 	registrar.RegisterPOST(jsapi_category_path, r.handle_jsapi)
	// }
	// registrar.RegisterGET("/api/mp/postprocess/flows", r.HandleFetchPostprocessFlows)
	registrar.RegisterPOST("/api/mp/refresh_with_frontend", r.handle_refresh_with_frontend)
	registrar.RegisterPOST("/api/mp/delete", r.handle_delete)
	registrar.RegisterPOST("/api/mp/refresh", r.handle_refresh_event)
	registrar.RegisterGET("/rss/mp", r.handle_official_account_rss)
	registrar.RegisterGET("/rss/mpbiz", r.handle_mpbiz_feed)
	registrar.RegisterGET("/rss/mpbiz/:format", r.handle_mpbiz_feed)
	registrar.RegisterGET("/mp/proxy", r.handle_official_account_proxy)
}

// HandleFetchPostprocessFlows returns wxmp postprocess flow configs for read-only visualization.
func (r *Routes) HandleFetchPostprocessFlows(ctx *gin.Context) {
	flow_id := ctx.Query("flow_id")
	payload, err := GetWXMPPostprocessFlowVisualization(flow_id)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, payload)
}

func (r *Routes) Stop() {
	if r != nil && r.server != nil {
		r.server.Stop()
	}
}
