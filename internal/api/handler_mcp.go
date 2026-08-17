package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/mcpserver"
)

func (c *APIClient) new_mcp_handler() http.Handler {
	server, err := mcpserver.NewServer(mcpserver.Config{
		APIBaseURL: c.mcp_api_base_url(),
		Version:    c.cfg.Version,
	})
	if err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Msg("mcp http handler initialization failed")
		}
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			write_mcp_http_error(writer, http.StatusInternalServerError, "MCP 服务初始化失败")
		})
	}
	return mcpserver.NewHTTPHandler(server)
}

func (c *APIClient) mcp_api_base_url() string {
	hostname := strings.TrimSpace(c.cfg.Hostname)
	parsed_ip := net.ParseIP(hostname)
	if hostname == "" || (parsed_ip != nil && parsed_ip.IsUnspecified()) {
		hostname = "127.0.0.1"
	}
	port := c.cfg.Port
	if port <= 0 {
		port = 2022
	}
	return "http://" + net.JoinHostPort(hostname, strconv.Itoa(port))
}

func (c *APIClient) handle_mcp_status(ctx *gin.Context) {
	result.Ok(ctx, c.mcp_status_data(ctx.Request))
}

func (c *APIClient) handle_mcp_enable(ctx *gin.Context) {
	c.update_mcp_enabled(ctx, true)
}

func (c *APIClient) handle_mcp_disable(ctx *gin.Context) {
	c.update_mcp_enabled(ctx, false)
}

func (c *APIClient) update_mcp_enabled(ctx *gin.Context, enabled bool) {
	c.mcp_enabled.Store(enabled)
	result.Ok(ctx, c.mcp_status_data(ctx.Request))
}

func (c *APIClient) mcp_status_data(request *http.Request) gin.H {
	enabled := c.mcp_enabled.Load()
	status := "stopped"
	if enabled {
		status = "running"
	}
	return gin.H{
		"enabled":   enabled,
		"status":    status,
		"transport": "streamable_http",
		"endpoint":  mcp_request_endpoint(request),
		"tools":     []string{"get_platform_status", "fetch_content", "download_content"},
	}
}

func (c *APIClient) handle_mcp_transport(ctx *gin.Context) {
	if !c.mcp_enabled.Load() {
		write_mcp_http_error(ctx.Writer, http.StatusServiceUnavailable, "MCP 服务未启用")
		return
	}
	if c.mcp_handler == nil {
		write_mcp_http_error(ctx.Writer, http.StatusInternalServerError, "MCP 服务未初始化")
		return
	}
	c.mcp_handler.ServeHTTP(ctx.Writer, ctx.Request)
}

func mcp_request_endpoint(request *http.Request) string {
	if request == nil {
		return "/mcp"
	}
	scheme := "http"
	if forwarded_protocol := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")); forwarded_protocol != "" {
		scheme = strings.Split(forwarded_protocol, ",")[0]
	} else if request.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(request.Host)
	if host == "" {
		return "/mcp"
	}
	return (&url.URL{Scheme: strings.TrimSpace(scheme), Host: host, Path: "/mcp"}).String()
}

func write_mcp_http_error(writer http.ResponseWriter, status_code int, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status_code)
	_ = json.NewEncoder(writer).Encode(gin.H{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": gin.H{
			"code":    -32000,
			"message": message,
		},
	})
}
