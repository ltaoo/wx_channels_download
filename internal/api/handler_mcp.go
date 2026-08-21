package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
)

func (c *APIClient) handle_mcp_status(ctx *gin.Context) {
	result.Ok(ctx, c.mcp_status_data(ctx.Request))
}

func (c *APIClient) handle_mcp_enable(ctx *gin.Context) {
	if err := c.mcp_service.Enable(); err != nil {
		result.Err(ctx, http.StatusServiceUnavailable, err.Error())
		return
	}
	result.Ok(ctx, c.mcp_status_data(ctx.Request))
}

func (c *APIClient) handle_mcp_disable(ctx *gin.Context) {
	if err := c.mcp_service.Disable(); err != nil {
		result.Err(ctx, http.StatusServiceUnavailable, err.Error())
		return
	}
	result.Ok(ctx, c.mcp_status_data(ctx.Request))
}

func (c *APIClient) mcp_status_data(request *http.Request) gin.H {
	status := c.mcp_service.Status()
	return gin.H{
		"enabled":   status.Enabled,
		"status":    status.Status,
		"transport": status.Transport,
		"endpoint":  mcp_request_endpoint(request),
		"tools":     status.Tools,
	}
}

func (c *APIClient) handle_mcp_transport(ctx *gin.Context) {
	c.mcp_service.ServeHTTP(ctx.Writer, ctx.Request)
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
