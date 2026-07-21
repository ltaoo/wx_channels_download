package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/config"
	"wx_channel/internal/events"
	result "wx_channel/internal/util"
	"wx_channel/pkg/certificate"
)

type serviceActionBody struct {
	Name string `json:"name"`
}

type serviceConfigBody struct {
	Values map[string]interface{} `json:"values"`
}

func (c *APIClient) handleServiceStart(ctx *gin.Context) {
	name := c.requestServiceName(ctx)
	if name == "" {
		result.Err(ctx, 400, "service is required")
		return
	}
	if c.bus == nil {
		result.Err(ctx, 500, "event bus not initialized")
		return
	}
	c.bus.Publish(events.ServiceCommand{Name: name, Action: "start"})
	result.Ok(ctx, c.serviceStatusesMap())
}

func (c *APIClient) handleServiceStop(ctx *gin.Context) {
	name := c.requestServiceName(ctx)
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
	result.Ok(ctx, c.serviceStatusesMap())
}

func (c *APIClient) requestServiceName(ctx *gin.Context) string {
	var body serviceActionBody
	_ = ctx.ShouldBindJSON(&body)
	name := body.Name
	if name == "" {
		name = ctx.Query("name")
	}
	return normalizeServiceName(name)
}

func normalizeServiceName(name string) string {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "proxy":
		return "interceptor"
	default:
		return strings.TrimSpace(strings.ToLower(name))
	}
}

func (c *APIClient) handleServiceConfigUpdate(ctx *gin.Context) {
	if c.cfg == nil || c.cfg.Original == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}
	var body serviceConfigBody
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
		converted, err := convertServiceConfigValue(key, value)
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

func convertServiceConfigValue(key string, value interface{}) (interface{}, error) {
	switch key {
	case "api.hostname", "proxy.hostname", "proxy.tcpRelay.hostname", "proxy.defaultInterface", "proxy.upstreamProxy", "cert.file", "cert.key", "cert.name":
		return strings.TrimSpace(fmt.Sprint(value)), nil
	case "api.port", "proxy.port", "proxy.tcpRelay.port":
		return serviceConfigPort(value)
	case "proxy.system", "proxy.tun", "proxy.tcpRelay.enabled", "proxy.skipInstallRootCert":
		return serviceConfigBool(value)
	default:
		return nil, fmt.Errorf("未知配置项: %s", key)
	}
}

func serviceConfigPort(value interface{}) (int, error) {
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

func (c *APIClient) handleRootCertificateStatus(ctx *gin.Context) {
	cert := config.LoadCertFiles()
	installed, err := certificate.CheckHasCertificate(cert.Name)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"name":      cert.Name,
		"installed": installed,
	})
}

func (c *APIClient) handleRootCertificateInstall(ctx *gin.Context) {
	cert := config.LoadCertFiles()
	if err := certificate.InstallCertificate(cert.Cert); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"name":      cert.Name,
		"installed": true,
	})
}

func (c *APIClient) handleRootCertificateUninstall(ctx *gin.Context) {
	cert := config.LoadCertFiles()
	if err := certificate.UninstallCertificate(cert.Name); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"name":      cert.Name,
		"installed": false,
	})
}
