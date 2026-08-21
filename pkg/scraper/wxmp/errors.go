package wxmp

// ErrorKind classifies scraper failures without assigning an HTTP status or
// application response code. Transport adapters can map these kinds to their
// own error protocol.
type ErrorKind string

const (
	ErrorKindInvalidArgument   ErrorKind = "invalid_argument"
	ErrorKindMissingBiz        ErrorKind = "missing_biz"
	ErrorKindMissingURL        ErrorKind = "missing_url"
	ErrorKindMissingRefreshURI ErrorKind = "missing_refresh_uri"
	ErrorKindTokenInvalid      ErrorKind = "token_invalid"
	ErrorKindAccountNotFound   ErrorKind = "account_not_found"
	ErrorKindAccountExpired    ErrorKind = "account_expired"
	ErrorKindAccountBanned     ErrorKind = "account_banned"
	ErrorKindProxyRequest      ErrorKind = "proxy_request"
	ErrorKindProxyDispatch     ErrorKind = "proxy_dispatch"
	ErrorKindFetchMessage      ErrorKind = "fetch_message"
	ErrorKindFetchPageContent  ErrorKind = "fetch_page_content"
	ErrorKindJSAPI             ErrorKind = "jsapi"
	ErrorKindDataParse         ErrorKind = "data_parse"
	ErrorKindClientNotReady    ErrorKind = "client_not_ready"
	ErrorKindTimeout           ErrorKind = "timeout"
	ErrorKindClientBusy        ErrorKind = "client_busy"
)

var error_messages = map[ErrorKind]string{
	ErrorKindInvalidArgument:   "参数错误",
	ErrorKindMissingBiz:        "缺少参数：biz",
	ErrorKindMissingURL:        "缺少参数：url",
	ErrorKindMissingRefreshURI: "缺少参数：refresh_uri",
	ErrorKindTokenInvalid:      "令牌无效",
	ErrorKindAccountNotFound:   "未找到匹配的公众号",
	ErrorKindAccountExpired:    "公众号凭证已失效",
	ErrorKindAccountBanned:     "账号被封禁",
	ErrorKindProxyRequest:      "代理请求创建失败",
	ErrorKindProxyDispatch:     "代理请求转发失败",
	ErrorKindFetchMessage:      "获取消息列表失败",
	ErrorKindFetchPageContent:  "获取页面内容失败",
	ErrorKindJSAPI:             "JSAPI 调用失败",
	ErrorKindDataParse:         "数据解析失败",
	ErrorKindClientNotReady:    "请先初始化客户端 socket 连接",
	ErrorKindTimeout:           "请求超时",
	ErrorKindClientBusy:        "发送缓冲区已满，请稍后重试",
}

func ErrorMessage(kind ErrorKind) string {
	if message, ok := error_messages[kind]; ok {
		return message
	}
	return "未知错误"
}

// ErrorDetails exposes the domain classification and diagnostic location of
// scraper errors while keeping the concrete error implementation private.
func ErrorDetails(err error) (kind ErrorKind, message, location string, ok bool) {
	return scraper_error_of(err)
}
