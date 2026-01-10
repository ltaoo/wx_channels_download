package util

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 错误码定义
const (
	CodeSuccess           = 0
	CodeInvalidParams     = 400
	CodeMissingBiz        = 4001
	CodeMissingUrl        = 4002
	CodeMissingKey        = 4003
	CodeMissingRefreshUri = 4004
	CodeTooManyAccounts   = 4005
	CodeTokenInvalid      = 1002 // 统一 Token 无效错误码
	CodeAccountNotFound   = 1003
	CodeAccountExpired    = 1004
	CodeAccountBanned     = 1005
	CodeClientNotReady    = 5001
	CodeFetchMsgFailed    = 2002
	CodeDataParseFailed   = 2003
	CodeProxyRequestErr   = 2000
	CodeProxyDispatchErr  = 2001
	CodeRemotePushFailed  = 2004
	CodeTimeout           = 5002
	CodeClientBusy        = 5003
)

// 错误文案映射 [0]英文 [1]中文
var errMsgMap = map[int][2]string{
	CodeSuccess:           {"success", "成功"},
	CodeInvalidParams:     {"Invalid parameters", "参数错误"},
	CodeMissingBiz:        {"Missing biz parameter", "缺少参数：biz"},
	CodeMissingUrl:        {"Missing url parameter", "缺少参数：url"},
	CodeMissingKey:        {"Missing key parameter", "缺少参数：key"},
	CodeMissingRefreshUri: {"Missing refresh uri parameter", "缺少参数：refresh_uri"},
	CodeTooManyAccounts:   {"Too many accounts", "公众号数量已达上限"},
	CodeTokenInvalid:      {"Invalid token", "令牌无效"},
	CodeAccountNotFound:   {"Account not found", "未找到匹配的公众号"},
	CodeAccountExpired:    {"Account expired", "公众号凭证已失效"},
	CodeAccountBanned:     {"Account banned", "账号被封禁"},
	CodeClientNotReady:    {"Client not ready", "客户端未就绪"},
	CodeFetchMsgFailed:    {"Fetch message list failed", "获取消息列表失败"},
	CodeDataParseFailed:   {"Data parse error", "数据解析失败"},
	CodeProxyRequestErr:   {"Proxy request creation failed", "代理请求创建失败"},
	CodeProxyDispatchErr:  {"Proxy request dispatch failed", "代理请求转发失败"},
	CodeRemotePushFailed:  {"Push credential failed", "同步凭证到远程服务器失败"},
	CodeTimeout:           {"Request timeout", "请求超时"},
	CodeClientBusy:        {"Client busy", "发送缓冲区已满，请稍后重试"},
}

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func GetMsg(code int) string {
	if msgs, ok := errMsgMap[code]; ok {
		return msgs[1]
	}
	return "Unknown error"
}

func Ok(ctx *gin.Context, data interface{}) {
	ctx.JSON(http.StatusOK, Response{
		Code: CodeSuccess,
		Msg:  GetMsg(CodeSuccess),
		Data: data,
	})
}

func Err(ctx *gin.Context, code int, msg string) {
	ctx.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
	})
}

func ErrCode(ctx *gin.Context, code int) {
	ctx.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  GetMsg(code),
	})
}

func writeErrorResponse(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(Response{
		Code: code,
		Msg:  msg,
	})
}
