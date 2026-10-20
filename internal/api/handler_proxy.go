package api

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/services"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/system"
)

type proxy_config_update_body struct {
	Values  map[string]interface{} `json:"values"`
	Restart bool                   `json:"restart"`
}

type proxy_certificate_generate_body struct {
	Name       string `json:"name"`
	Install    bool   `json:"install"`
	Restart    bool   `json:"restart"`
	ValidYears int    `json:"valid_years"`
}

func (c *APIClient) handle_proxy_status(ctx *gin.Context) {
	result.Ok(ctx, c.proxy_status_data())
}

func (c *APIClient) handle_proxy_config_update(ctx *gin.Context) {
	if c.cfg == nil || c.cfg.Original == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}
	var body proxy_config_update_body
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
	if err := c.save_config_values(updated); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if body.Restart {
		if err := c.restart_proxy_service(); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	} else if !c.proxy_service_running() {
		if err := c.apply_proxy_settings_from_config(); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	}
	result.Ok(ctx, c.proxy_status_data())
}

func (c *APIClient) handle_proxy_restart(ctx *gin.Context) {
	if err := c.restart_proxy_service(); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxy_status_data())
}

func (c *APIClient) handle_proxy_system_enable(ctx *gin.Context) {
	settings := c.system_proxy_settings()
	if err := system.EnableProxy(settings); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if err := c.save_config_values(map[string]interface{}{"proxy.system": true}); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxy_status_data())
}

func (c *APIClient) handle_proxy_system_disable(ctx *gin.Context) {
	settings := c.system_proxy_settings()
	if err := system.DisableProxy(settings); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if err := c.save_config_values(map[string]interface{}{"proxy.system": false}); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxy_status_data())
}

func (c *APIClient) handle_proxy_certificate_status(ctx *gin.Context) {
	result.Ok(ctx, c.certificate_status_data())
}

func (c *APIClient) handle_proxy_certificate_generate(ctx *gin.Context) {
	if c.cfg == nil || c.cfg.Original == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}
	var body proxy_certificate_generate_body
	_ = ctx.ShouldBindJSON(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = certificate.LocalCAName
	}
	years := body.ValidYears
	if years <= 0 {
		years = 10
	}
	if years > 30 {
		years = 30
	}

	cert_pem, key_pem, err := certificate.GenerateRootCA(name, time.Duration(years)*365*24*time.Hour)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	cert_path, key_path, err := services.WriteGeneratedCertificate(c.cfg.Original, name, cert_pem, key_pem)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if err := c.save_config_values(map[string]interface{}{
		"cert.file": cert_path,
		"cert.key":  key_path,
		"cert.name": name,
	}); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if body.Install {
		if err := certificate.InstallCertificate(cert_pem); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	}
	if body.Restart {
		if err := c.restart_proxy_service(); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	} else if !c.proxy_service_running() {
		if err := c.apply_proxy_settings_from_config(); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	}
	result.Ok(ctx, c.proxy_status_data())
}

func (c *APIClient) handle_proxy_certificate_install(ctx *gin.Context) {
	cert := services.LoadCertFiles()
	if err := certificate.InstallCertificate(cert.Cert); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxy_status_data())
}

func (c *APIClient) handle_proxy_certificate_uninstall(ctx *gin.Context) {
	cert := services.LoadCertFiles()
	if err := certificate.UninstallCertificate(cert.Name); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxy_status_data())
}

type uninstall_cert_by_name_body struct {
	Name string `json:"name"`
}

func (c *APIClient) handle_proxy_certificate_uninstall_by_name(ctx *gin.Context) {
	var body uninstall_cert_by_name_body
	if err := ctx.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		result.Err(ctx, 400, "缺少证书名称")
		return
	}
	name := strings.TrimSpace(body.Name)
	if err := certificate.UninstallCertificate(name); err != nil {
		result.Err(ctx, 500, "卸载证书失败: "+err.Error())
		return
	}
	result.Ok(ctx, c.proxy_status_data())
}

func (c *APIClient) handle_proxy_certificate_replace(ctx *gin.Context) {
	if c.cfg == nil || c.cfg.Original == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}

	// 1. Get old cert info
	cert_info := services.LoadCertFilesWithInfo()
	old_name := cert_info.Cert.Name

	// 2. If installed, uninstall old cert
	installed, _ := certificate.CheckHasCertificate(old_name)
	if installed {
		if err := certificate.UninstallCertificate(old_name); err != nil {
			result.Err(ctx, 500, "卸载旧证书失败: "+err.Error())
			return
		}
	}

	// 3. Generate new RootCA
	name := certificate.LocalCAName
	cert_pem, key_pem, err := certificate.GenerateRootCA(name, 10*365*24*time.Hour)
	if err != nil {
		result.Err(ctx, 500, "生成新证书失败: "+err.Error())
		return
	}

	// 4. Write to certs/ directory
	cert_path, key_path, err := services.WriteGeneratedCertificate(c.cfg.Original, name, cert_pem, key_pem)
	if err != nil {
		result.Err(ctx, 500, "写入证书文件失败: "+err.Error())
		return
	}

	// 5. Update config
	if err := c.save_config_values(map[string]interface{}{
		"cert.file": cert_path,
		"cert.key":  key_path,
		"cert.name": name,
	}); err != nil {
		result.Err(ctx, 500, "保存配置失败: "+err.Error())
		return
	}

	// 6. Install new cert
	if err := certificate.InstallCertificate(cert_pem); err != nil {
		result.Err(ctx, 500, "安装新证书失败: "+err.Error())
		return
	}

	// 7. Restart proxy
	if err := c.restart_proxy_service(); err != nil {
		result.Err(ctx, 500, "重启代理服务失败: "+err.Error())
		return
	}

	result.Ok(ctx, c.proxy_status_data())
}

func (c *APIClient) handle_proxy_certificate_pem(ctx *gin.Context) {
	cert := services.LoadCertFiles()
	ctx.Header("Content-Type", "application/x-pem-file; charset=utf-8")
	ctx.Header("Content-Disposition", `attachment; filename="root-ca.pem"`)
	ctx.String(200, string(cert.Cert))
}

func (c *APIClient) proxy_status_data() gin.H {
	config_data := c.proxy_config_data()
	return gin.H{
		"os":           runtime.GOOS,
		"config":       config_data,
		"service":      c.proxy_service_status_data(),
		"system_proxy": c.system_proxy_status_data(),
		"certificate":  c.certificate_status_data(),
	}
}

func (c *APIClient) proxy_config_data() gin.H {
	var original *config.Config
	if c.cfg != nil {
		original = c.cfg.Original
	}
	host := "127.0.0.1"
	port := 2023
	if original != nil {
		if value := strings.TrimSpace(original.GetString("proxy.hostname")); value != "" {
			host = value
		}
		if value := original.GetInt("proxy.port"); value > 0 {
			port = value
		}
	}
	return gin.H{
		"hostname":               host,
		"port":                   port,
		"addr":                   net.JoinHostPort(host, strconv.Itoa(port)),
		"enabled":                original == nil || original.GetBool("proxy.enabled"),
		"system":                 original != nil && original.GetBool("proxy.system"),
		"tun":                    original != nil && original.GetBool("proxy.tun"),
		"default_interface":      get_config_string(original, "proxy.defaultInterface"),
		"network_service":        get_config_string(original, "proxy.networkService"),
		"skip_install_root_cert": original != nil && original.GetBool("proxy.skipInstallRootCert"),
		"upstream_proxy":         get_config_string(original, "proxy.upstreamProxy"),
		"tcp_relay": gin.H{
			"enabled":  original != nil && original.GetBool("proxy.tcpRelay.enabled"),
			"hostname": proxy_first_non_empty(get_config_string(original, "proxy.tcpRelay.hostname"), "127.0.0.1"),
			"port":     proxy_first_positive(get_config_int(original, "proxy.tcpRelay.port"), 9900),
		},
		"cert": gin.H{
			"name": get_config_string(original, "cert.name"),
			"file": get_config_string(original, "cert.file"),
			"key":  get_config_string(original, "cert.key"),
		},
	}
}

func (c *APIClient) proxy_service_status_data() gin.H {
	proxy_status := c.runtime_status_service.ProxyStatus()
	addr := proxy_status.Addr
	status := proxy_status.Status
	if status == "" {
		status = "stopped"
	}
	if addr == "" {
		cfg := c.proxy_config_data()
		addr, _ = cfg["addr"].(string)
	}
	return gin.H{
		"name":      "interceptor",
		"addr":      addr,
		"status":    status,
		"listening": addr != "" && check_port(addr),
	}
}

func (c *APIClient) system_proxy_status_data() gin.H {
	expected := c.system_proxy_settings()
	cur, err := system.FetchCurProxy(expected)
	configured := false
	if c.cfg != nil && c.cfg.Original != nil {
		configured = c.cfg.Original.GetBool("proxy.system")
	}
	data := gin.H{
		"configured": configured,
		"expected": gin.H{
			"hostname": expected.Hostname,
			"port":     expected.Port,
			"device":   expected.Device,
		},
		"enabled": false,
		"matched": false,
	}
	if err != nil {
		data["error"] = err.Error()
		return data
	}
	if cur == nil {
		return data
	}
	data["enabled"] = true
	data["current"] = gin.H{
		"hostname": cur.Hostname,
		"port":     cur.Port,
		"device":   cur.Device,
	}
	data["matched"] = cur.Hostname == expected.Hostname && cur.Port == expected.Port
	return data
}

func (c *APIClient) certificate_status_data() gin.H {
	if c == nil || c.certificate_service == nil {
		return gin.H{}
	}
	return gin.H(c.certificate_service.Status())
}

func (c *APIClient) system_proxy_settings() system.ProxySettings {
	cfg := c.proxy_config_data()
	device, _ := cfg["network_service"].(string)
	return system.ProxySettings{
		// Empty means "detect the primary network service", matching what enable_proxy does.
		Device:   device,
		Hostname: fmt.Sprint(cfg["hostname"]),
		Port:     strconv.Itoa(proxy_first_positive(proxy_to_int_default(cfg["port"], 2023), 2023)),
	}
}

func service_config_bool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return false, fmt.Errorf("必须是布尔值")
		}
		return parsed, nil
	default:
		return false, fmt.Errorf("必须是布尔值")
	}
}

func (c *APIClient) save_config_values(values map[string]interface{}) error {
	if c.cfg == nil || c.cfg.Original == nil {
		return fmt.Errorf("配置未初始化")
	}
	for key, value := range values {
		c.cfg.Original.Update(key, value)
	}
	if dir := filepath.Dir(c.cfg.Original.FullPath); dir != "" && dir != "." {
		if err := config.EnsureDirIfMissing(dir); err != nil {
			return err
		}
	}
	if err := c.cfg.Original.Save(); err != nil {
		return err
	}
	c.cfg.Original.Existing = true
	return nil
}

func (c *APIClient) proxy_service_running() bool {
	status := c.runtime_status_service.ProxyStatus().Status
	return status == "running" || status == "stopping"
}

func (c *APIClient) restart_proxy_service() error {
	if c.event_publisher == nil {
		return fmt.Errorf("event bus not initialized")
	}
	c.event_publisher.Publish(events.ProxyCommand{Action: events.ProxyRestart})
	return nil
}

func (c *APIClient) apply_proxy_settings_from_config() error {
	if c.event_publisher == nil {
		return nil
	}
	c.event_publisher.Publish(events.ProxyCommand{Action: events.ProxyApplySettings})
	return nil
}

func get_config_string(c *config.Config, key string) string {
	if c == nil {
		return ""
	}
	return c.GetString(key)
}

func get_config_int(c *config.Config, key string) int {
	if c == nil {
		return 0
	}
	return c.GetInt(key)
}

func proxy_first_non_empty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func proxy_first_positive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func proxy_to_int_default(value interface{}, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}
