package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/util"
)

type remoteProxyRequest struct {
	Method  string                 `json:"method"`
	Path    string                 `json:"path"`
	Query   map[string]interface{} `json:"query"`
	Body    interface{}            `json:"body"`
	Headers map[string]string      `json:"headers"`
}

func (c *APIClient) handleFetchRemoteTaskList(ctx *gin.Context) {
	query := make(map[string]interface{})
	if page := firstQuery(ctx, "page"); page != "" {
		query["page"] = page
	}
	if pageSize := firstQuery(ctx, "page_size", "pageSize"); pageSize != "" {
		query["page_size"] = pageSize
	}
	if status := normalizeRemoteTaskStatus(firstQuery(ctx, "status")); status != "" {
		query["status"] = status
	}
	c.proxyRemoteRequest(ctx, remoteProxyRequest{
		Method: http.MethodGet,
		Path:   "/api/task/list",
		Query:  query,
	})
}

func (c *APIClient) handleRemoteProxyRequest(ctx *gin.Context) {
	var body remoteProxyRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "代理请求参数不合法："+err.Error())
		return
	}
	c.proxyRemoteRequest(ctx, body)
}

func (c *APIClient) proxyRemoteRequest(ctx *gin.Context, proxyReq remoteProxyRequest) {
	if strings.TrimSpace(c.cfg.RemoteServerHostname) == "" {
		result.Err(ctx, 400, "未配置 download.remoteServer.hostname")
		return
	}

	protocol := strings.TrimSpace(c.cfg.RemoteServerProtocol)
	if protocol == "" {
		protocol = "http"
	}
	port := c.cfg.RemoteServerPort
	if port <= 0 {
		result.Err(ctx, 400, "未配置 download.remoteServer.port")
		return
	}

	method := strings.ToUpper(strings.TrimSpace(proxyReq.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !isAllowedRemoteProxyMethod(method) {
		result.Err(ctx, 400, "不支持的远端代理请求方法")
		return
	}

	path := strings.TrimSpace(proxyReq.Path)
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		result.Err(ctx, 400, "远端代理路径不合法")
		return
	}

	target, err := url.Parse(fmt.Sprintf("%s://%s:%d", protocol, c.cfg.RemoteServerHostname, port))
	if err != nil {
		result.Err(ctx, 500, "远端服务地址不合法")
		return
	}
	target.Path = path

	q := target.Query()
	for key, value := range proxyReq.Query {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		q.Set(key, fmt.Sprint(value))
	}
	target.RawQuery = q.Encode()

	var reqBody io.Reader
	if proxyReq.Body != nil && method != http.MethodGet && method != http.MethodHead {
		data, err := json.Marshal(proxyReq.Body)
		if err != nil {
			result.Err(ctx, 400, "远端代理请求体不合法")
			return
		}
		reqBody = bytes.NewReader(data)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx.Request.Context(), method, target.String(), reqBody)
	if err != nil {
		result.Err(ctx, 500, "创建远端请求失败")
		return
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range proxyReq.Headers {
		key = strings.TrimSpace(key)
		if key == "" || strings.EqualFold(key, "Host") {
			continue
		}
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		result.Err(ctx, 502, "请求远端服务失败："+err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Err(ctx, 502, "读取远端响应失败："+err.Error())
		return
	}
	ctx.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func isAllowedRemoteProxyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead:
		return true
	default:
		return false
	}
}

func firstQuery(ctx *gin.Context, names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(ctx.Query(name)); v != "" {
			return v
		}
	}
	return ""
}

func normalizeRemoteTaskStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" || status == "all" {
		return status
	}
	if _, err := strconv.Atoi(status); err != nil {
		return status
	}
	switch status {
	case "1":
		return "ready"
	case "2":
		return "running"
	case "3":
		return "pause"
	case "4":
		return "wait"
	case "5":
		return "done"
	case "6":
		return "error"
	default:
		return ""
	}
}
