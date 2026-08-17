package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/events"
)

type service_action_body struct {
	Name string `json:"name"`
}

type service_config_body struct {
	Values map[string]interface{} `json:"values"`
}

func (c *APIClient) handle_service_start(ctx *gin.Context) {
	name := c.request_service_name(ctx)
	if name == "" {
		result.Err(ctx, 400, "service is required")
		return
	}
	if c.bus == nil {
		result.Err(ctx, 500, "event bus not initialized")
		return
	}
	c.bus.Publish(events.ServiceCommand{Name: name, Action: "start"})
	result.Ok(ctx, c.service_statuses_map())
}

func (c *APIClient) handle_service_stop(ctx *gin.Context) {
	name := c.request_service_name(ctx)
	if name == "" {
		result.Err(ctx, 400, "service is required")
		return
	}
	if name == "api" {
		result.Err(ctx, 400, "api service cannot stop itself from HTTP")
		return
	}
	if c.bus == nil {
		result.Err(ctx, 500, "event bus not initialized")
		return
	}
	c.bus.Publish(events.ServiceCommand{Name: name, Action: "stop"})
	result.Ok(ctx, c.service_statuses_map())
}

func (c *APIClient) request_service_name(ctx *gin.Context) string {
	var body service_action_body
	_ = ctx.ShouldBindJSON(&body)
	name := body.Name
	if name == "" {
		name = ctx.Query("name")
	}
	return normalize_service_name(name)
}

func normalize_service_name(name string) string {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "proxy":
		return "interceptor"
	default:
		return strings.TrimSpace(strings.ToLower(name))
	}
}

func (c *APIClient) handle_service_config_update(ctx *gin.Context) {
	if c.cfg == nil || c.cfg.Original == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}
	var body service_config_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if len(body.Values) == 0 {
		result.Err(ctx, 400, "缺少配置项")
		return
	}

	updated := map[string]interface{}{}
	for key, value := range body.Values {
		converted, err := convert_service_config_value(key, value)
		if err != nil {
			result.Err(ctx, 400, err.Error())
			return
		}
		updated[key] = converted
	}
	for key, value := range updated {
		c.cfg.Original.Update(key, value)
	}
	if err := c.cfg.Original.Save(); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"values": updated})
}

func convert_service_config_value(key string, value interface{}) (interface{}, error) {
	switch key {
	case "api.hostname", "proxy.hostname", "proxy.tcpRelay.hostname", "proxy.defaultInterface", "proxy.upstreamProxy", "cert.file", "cert.key", "cert.name":
		return strings.TrimSpace(fmt.Sprint(value)), nil
	case "api.port", "proxy.port", "proxy.tcpRelay.port":
		return service_config_port(value)
	case "proxy.enabled", "proxy.system", "proxy.tun", "proxy.tcpRelay.enabled", "proxy.skipInstallRootCert":
		return service_config_bool(value)
	default:
		return nil, fmt.Errorf("未知配置项: %s", key)
	}
}

func service_config_port(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		if v <= 0 {
			return 0, fmt.Errorf("端口必须大于 0")
		}
		return v, nil
	case float64:
		if v != float64(int(v)) || v <= 0 {
			return 0, fmt.Errorf("端口必须是大于 0 的整数")
		}
		return int(v), nil
	case string:
		port, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || port <= 0 {
			return 0, fmt.Errorf("端口必须是大于 0 的整数")
		}
		return port, nil
	default:
		return 0, fmt.Errorf("端口必须是大于 0 的整数")
	}
}
