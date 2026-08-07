package api

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/config"
	result "wx_channel/internal/util"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/configapi"
	"wx_channel/pkg/events"
	"wx_channel/pkg/system"
)

type proxyConfigUpdateBody struct {
	Values  map[string]interface{} `json:"values"`
	Restart bool                   `json:"restart"`
}

type proxyCertificateGenerateBody struct {
	Name       string `json:"name"`
	Install    bool   `json:"install"`
	Restart    bool   `json:"restart"`
	ValidYears int    `json:"valid_years"`
}

func (c *APIClient) handleProxyStatus(ctx *gin.Context) {
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxyConfigUpdate(ctx *gin.Context) {
	if c.config_store == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}
	var body proxyConfigUpdateBody
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
		converted, err := convertProxyConfigValue(key, value)
		if err != nil {
			result.Err(ctx, 400, err.Error())
			return
		}
		updated[key] = converted
	}
	if err := c.saveConfigValues(updated); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if body.Restart {
		if err := c.restartProxyService(); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	} else if !c.proxyServiceRunning() {
		if err := c.applyProxySettingsFromConfig(); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	}
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxyRestart(ctx *gin.Context) {
	if err := c.restartProxyService(); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxySystemEnable(ctx *gin.Context) {
	settings := c.systemProxySettings()
	if err := system.EnableProxy(settings); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if err := c.saveConfigValues(map[string]interface{}{"proxy.system": true}); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxySystemDisable(ctx *gin.Context) {
	settings := c.systemProxySettings()
	if err := system.DisableProxy(settings); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if err := c.saveConfigValues(map[string]interface{}{"proxy.system": false}); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxyCertificateStatus(ctx *gin.Context) {
	result.Ok(ctx, c.certificateStatusData())
}

func (c *APIClient) handleProxyCertificateGenerate(ctx *gin.Context) {
	if c.config_store == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}
	var body proxyCertificateGenerateBody
	_ = ctx.ShouldBindJSON(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = strings.TrimSpace(c.current_certificate_config().Name)
	}
	if name == "" {
		name = "wx_channels_download"
	}
	years := body.ValidYears
	if years <= 0 {
		years = 10
	}
	if years > 30 {
		years = 30
	}

	certPEM, keyPEM, err := certificate.GenerateRootCA(name, time.Duration(years)*365*24*time.Hour)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	certPath, keyPath, err := c.writeGeneratedCertificate(name, certPEM, keyPEM)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if err := c.saveConfigValues(map[string]interface{}{
		"cert.file": certPath,
		"cert.key":  keyPath,
		"cert.name": name,
	}); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if body.Install {
		if err := certificate.InstallCertificate(certPEM); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	}
	if body.Restart {
		if err := c.restartProxyService(); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	} else if !c.proxyServiceRunning() {
		if err := c.applyProxySettingsFromConfig(); err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	}
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxyCertificateInstall(ctx *gin.Context) {
	cert := config.LoadCertFiles(c.config_store)
	if err := certificate.InstallCertificate(cert.Cert); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxyCertificateUninstall(ctx *gin.Context) {
	cert := config.LoadCertFiles(c.config_store)
	if err := certificate.UninstallCertificate(cert.Name); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxyStatusData())
}

type uninstallCertByNameBody struct {
	Name string `json:"name"`
}

func (c *APIClient) handleProxyCertificateUninstallByName(ctx *gin.Context) {
	var body uninstallCertByNameBody
	if err := ctx.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		result.Err(ctx, 400, "缺少证书名称")
		return
	}
	name := strings.TrimSpace(body.Name)
	if err := certificate.UninstallCertificate(name); err != nil {
		result.Err(ctx, 500, "卸载证书失败: "+err.Error())
		return
	}
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxyCertificateReplace(ctx *gin.Context) {
	if c.config_store == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}

	// 1. Get old cert info
	certInfo := config.LoadCertFilesWithInfo(c.config_store)
	oldName := certInfo.Cert.Name

	// 2. If installed, uninstall old cert
	installed, _ := certificate.CheckHasCertificate(oldName)
	if installed {
		if err := certificate.UninstallCertificate(oldName); err != nil {
			result.Err(ctx, 500, "卸载旧证书失败: "+err.Error())
			return
		}
	}

	// 3. Generate new RootCA
	name := "wx_channels_download"
	certPEM, keyPEM, err := certificate.GenerateRootCA(name, 10*365*24*time.Hour)
	if err != nil {
		result.Err(ctx, 500, "生成新证书失败: "+err.Error())
		return
	}

	// 4. Write to certs/ directory
	certPath, keyPath, err := c.writeGeneratedCertificate(name, certPEM, keyPEM)
	if err != nil {
		result.Err(ctx, 500, "写入证书文件失败: "+err.Error())
		return
	}

	// 5. Update config
	if err := c.saveConfigValues(map[string]interface{}{
		"cert.file": certPath,
		"cert.key":  keyPath,
		"cert.name": name,
	}); err != nil {
		result.Err(ctx, 500, "保存配置失败: "+err.Error())
		return
	}

	// 6. Install new cert
	if err := certificate.InstallCertificate(certPEM); err != nil {
		result.Err(ctx, 500, "安装新证书失败: "+err.Error())
		return
	}

	// 7. Restart proxy
	if err := c.restartProxyService(); err != nil {
		result.Err(ctx, 500, "重启代理服务失败: "+err.Error())
		return
	}

	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxyCertificatePEM(ctx *gin.Context) {
	cert := config.LoadCertFiles(c.config_store)
	ctx.Header("Content-Type", "application/x-pem-file; charset=utf-8")
	ctx.Header("Content-Disposition", `attachment; filename="root-ca.pem"`)
	ctx.String(200, string(cert.Cert))
}

func (c *APIClient) proxyStatusData() gin.H {
	configData := c.proxyConfigData()
	return gin.H{
		"os":           runtime.GOOS,
		"config":       configData,
		"service":      c.proxyServiceStatusData(),
		"system_proxy": c.systemProxyStatusData(),
		"certificate":  c.certificateStatusData(),
	}
}

func (c *APIClient) proxyConfigData() gin.H {
	proxy_config := c.current_proxy_config()
	certificate_config := c.current_certificate_config()
	return gin.H{
		"hostname":               proxy_config.Hostname,
		"port":                   proxy_config.Port,
		"addr":                   net.JoinHostPort(proxy_config.Hostname, strconv.Itoa(proxy_config.Port)),
		"system":                 proxy_config.System,
		"tun":                    proxy_config.Tun,
		"default_interface":      proxy_config.DefaultInterface,
		"skip_install_root_cert": proxy_config.SkipInstallRootCert,
		"upstream_proxy":         proxy_config.UpstreamProxy,
		"tcp_relay": gin.H{
			"enabled":  proxy_config.TCPRelay.Enabled,
			"hostname": proxy_config.TCPRelay.Hostname,
			"port":     proxy_config.TCPRelay.Port,
		},
		"cert": gin.H{
			"name": certificate_config.Name,
			"file": certificate_config.File,
			"key":  certificate_config.Key,
		},
	}
}

func (c *APIClient) proxyServiceStatusData() gin.H {
	c.proxy_status_mu.RLock()
	addr := c.cached_proxy_addr
	status := c.cached_proxy_status
	c.proxy_status_mu.RUnlock()
	if status == "" {
		status = "stopped"
	}
	if addr == "" {
		cfg := c.proxyConfigData()
		addr, _ = cfg["addr"].(string)
	}
	return gin.H{
		"name":      "interceptor",
		"addr":      addr,
		"status":    status,
		"listening": addr != "" && check_port(addr),
	}
}

func (c *APIClient) systemProxyStatusData() gin.H {
	expected := c.systemProxySettings()
	cur, err := system.FetchCurProxy(expected)
	configured := c.current_proxy_config().System
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

func (c *APIClient) certificateStatusData() gin.H {
	certInfo := config.LoadCertFilesWithInfo(c.config_store)
	cert := certInfo.Cert
	installed, installErr := certificate.CheckHasCertificate(cert.Name)
	data := gin.H{
		"name":          cert.Name,
		"source":        string(certInfo.Source),
		"is_legacy":     certInfo.IsLegacy,
		"risk_warnings": certInfo.RiskWarnings,
		"installed":     installed,
		"trusted":       false,
		"pem":           string(cert.Cert),
	}
	if installed {
		trusted, trustErr := certificate.CheckCertificateTrusted(cert.Name)
		if trustErr == nil {
			data["trusted"] = trusted
		}
		if trustErr != nil && installErr == nil {
			installErr = trustErr
		}
	}
	if installErr != nil {
		data["install_status_error"] = installErr.Error()
	}
	if details, err := inspectCertificate(cert.Cert); err == nil {
		data["detail"] = details
	} else {
		data["parse_error"] = err.Error()
	}
	configured_certificate := c.current_certificate_config()
	data["configured"] = gin.H{
		"name": configured_certificate.Name,
		"file": configured_certificate.File,
		"key":  configured_certificate.Key,
	}

	// Build list of all available certificates with their system-install status
	allCerts := config.ScanAvailableCerts(c.config_store)
	certList := make([]gin.H, 0, len(allCerts))
	for _, ac := range allCerts {
		entry := c.buildCertEntry(ac)
		certList = append(certList, entry)
	}
	data["all_certificates"] = certList

	return data
}

func (c *APIClient) buildCertEntry(ac config.AvailableCert) gin.H {
	installed, _ := certificate.CheckHasCertificate(ac.Cert.Name)
	entry := gin.H{
		"name":          ac.Cert.Name,
		"source":        string(ac.Source),
		"is_legacy":     ac.IsLegacy,
		"is_active":     ac.IsActive,
		"installed":     installed,
		"trusted":       false,
		"risk_warnings": ac.RiskWarnings,
	}
	if installed {
		trusted, trustErr := certificate.CheckCertificateTrusted(ac.Cert.Name)
		if trustErr == nil {
			entry["trusted"] = trusted
		}
	}
	if details, err := inspectCertificate(ac.Cert.Cert); err == nil {
		entry["detail"] = details
	}
	return entry
}

func (c *APIClient) systemProxySettings() system.ProxySettings {
	cfg := c.proxyConfigData()
	return system.ProxySettings{
		Hostname: fmt.Sprint(cfg["hostname"]),
		Port:     strconv.Itoa(proxyFirstPositive(proxyToIntDefault(cfg["port"], 2023), 2023)),
	}
}

func convertProxyConfigValue(key string, value interface{}) (interface{}, error) {
	switch key {
	case "proxy.hostname", "proxy.tcpRelay.hostname", "proxy.defaultInterface", "proxy.upstreamProxy", "cert.file", "cert.key", "cert.name":
		return strings.TrimSpace(fmt.Sprint(value)), nil
	case "proxy.port", "proxy.tcpRelay.port":
		return proxyConfigPort(value)
	case "proxy.system", "proxy.tun", "proxy.tcpRelay.enabled", "proxy.skipInstallRootCert":
		return proxyConfigBool(value)
	default:
		return nil, fmt.Errorf("未知配置项: %s", key)
	}
}

func proxyConfigPort(value interface{}) (int, error) {
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

func proxyConfigBool(value interface{}) (bool, error) {
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

func (c *APIClient) saveConfigValues(values map[string]interface{}) error {
	if c.config_store == nil {
		return fmt.Errorf("配置未初始化")
	}
	_, err := c.config_store.Apply(context.Background(), configapi.UpdateRequest{
		Values:           values,
		ExpectedRevision: c.config_store.Revision(),
	})
	return err
}

func (c *APIClient) proxyServiceRunning() bool {
	c.proxy_status_mu.RLock()
	status := c.cached_proxy_status
	c.proxy_status_mu.RUnlock()
	return status == "running" || status == "stopping"
}

func (c *APIClient) restartProxyService() error {
	if c.bus == nil {
		return fmt.Errorf("event bus not initialized")
	}
	c.bus.Publish(events.ProxyCommand{Action: events.ProxyRestart})
	return nil
}

func (c *APIClient) applyProxySettingsFromConfig() error {
	if c.bus == nil {
		return nil
	}
	c.bus.Publish(events.ProxyCommand{Action: events.ProxyApplySettings})
	return nil
}

func (c *APIClient) writeGeneratedCertificate(name string, certPEM []byte, keyPEM []byte) (string, string, error) {
	baseDir := c.cfg.WorkDir
	if strings.TrimSpace(baseDir) == "" {
		baseDir = c.cfg.RootDir
	}
	dir := filepath.Join(baseDir, "certs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", err
	}
	slug := certificateFilenameSlug(name)
	certPath := filepath.Join(dir, slug+".pem")
	keyPath := filepath.Join(dir, slug+".key")
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return "", "", err
	}
	return certPath, keyPath, nil
}

func inspectCertificate(data []byte) (gin.H, error) {
	cert, err := parseFirstCertificate(data)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(cert.Raw)
	return gin.H{
		"subject_common_name": cert.Subject.CommonName,
		"issuer_common_name":  cert.Issuer.CommonName,
		"serial_number":       cert.SerialNumber.String(),
		"not_before":          cert.NotBefore.Format(time.RFC3339),
		"not_after":           cert.NotAfter.Format(time.RFC3339),
		"expired":             time.Now().After(cert.NotAfter),
		"is_ca":               cert.IsCA,
		"dns_names":           cert.DNSNames,
		"organizations":       cert.Subject.Organization,
		"fingerprint_sha256":  formatFingerprint(sum[:]),
	}, nil
}

func parseFirstCertificate(data []byte) (*x509.Certificate, error) {
	rest := data
	for {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = next
		if block.Type != "CERTIFICATE" {
			continue
		}
		return x509.ParseCertificate(block.Bytes)
	}
	return x509.ParseCertificate(data)
}

func formatFingerprint(bytes []byte) string {
	encoded := strings.ToUpper(hex.EncodeToString(bytes))
	parts := make([]string, 0, len(encoded)/2)
	for i := 0; i+2 <= len(encoded); i += 2 {
		parts = append(parts, encoded[i:i+2])
	}
	return strings.Join(parts, ":")
}

func certificateFilenameSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevUnderscore := false
	for _, r := range name {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if ok {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		return "wx_channels_download"
	}
	return slug
}

func proxyFirstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func proxyToIntDefault(value interface{}, fallback int) int {
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
