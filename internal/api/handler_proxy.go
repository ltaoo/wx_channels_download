package api

import (
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
	"wx_channel/internal/interceptor"
	"wx_channel/internal/manager"
	result "wx_channel/internal/util"
	"wx_channel/pkg/certificate"
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
	if c.cfg == nil || c.cfg.Original == nil {
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

	updated, err := convertProxyConfigValues(body.Values)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
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
	if c.cfg == nil || c.cfg.Original == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}
	var body proxyCertificateGenerateBody
	_ = ctx.ShouldBindJSON(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = strings.TrimSpace(c.cfg.Original.GetString("cert.name"))
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
	cert := config.LoadCertFiles()
	if err := certificate.InstallCertificate(cert.Cert); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxyCertificateUninstall(ctx *gin.Context) {
	cert := config.LoadCertFiles()
	if err := certificate.UninstallCertificate(cert.Name); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, c.proxyStatusData())
}

func (c *APIClient) handleProxyCertificatePEM(ctx *gin.Context) {
	cert := config.LoadCertFiles()
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
		"system":                 original != nil && original.GetBool("proxy.system"),
		"tun":                    original != nil && original.GetBool("proxy.tun"),
		"default_interface":      getConfigString(original, "proxy.defaultInterface"),
		"skip_install_root_cert": original != nil && original.GetBool("proxy.skipInstallRootCert"),
		"upstream_proxy":         getConfigString(original, "proxy.upstreamProxy"),
		"tcp_relay": gin.H{
			"enabled":  original != nil && original.GetBool("proxy.tcpRelay.enabled"),
			"hostname": proxyFirstNonEmpty(getConfigString(original, "proxy.tcpRelay.hostname"), "127.0.0.1"),
			"port":     proxyFirstPositive(getConfigInt(original, "proxy.tcpRelay.port"), 9900),
		},
		"cert": gin.H{
			"name": getConfigString(original, "cert.name"),
			"file": getConfigString(original, "cert.file"),
			"key":  getConfigString(original, "cert.key"),
		},
	}
}

func (c *APIClient) proxyServiceStatusData() gin.H {
	status := manager.StatusStopped
	addr := ""
	if c.serviceMgr != nil {
		if server := c.serviceMgr.GetServer("interceptor"); server != nil {
			addr = server.Addr()
			if serverStatus, err := c.serviceMgr.GetStatus("interceptor"); err == nil {
				status = serverStatus
			}
		}
	}
	if addr == "" {
		cfg := c.proxyConfigData()
		addr, _ = cfg["addr"].(string)
	}
	return gin.H{
		"name":      "interceptor",
		"addr":      addr,
		"status":    string(status),
		"listening": addr != "" && checkPort(addr),
	}
}

func (c *APIClient) systemProxyStatusData() gin.H {
	expected := c.systemProxySettings()
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

func (c *APIClient) certificateStatusData() gin.H {
	cert := config.LoadCertFiles()
	installed, installErr := certificate.CheckHasCertificate(cert.Name)
	data := gin.H{
		"name":      cert.Name,
		"installed": installed,
		"pem":       string(cert.Cert),
	}
	if installErr != nil {
		data["install_status_error"] = installErr.Error()
	}
	if details, err := inspectCertificate(cert.Cert); err == nil {
		data["detail"] = details
	} else {
		data["parse_error"] = err.Error()
	}
	if c.cfg != nil && c.cfg.Original != nil {
		data["configured"] = gin.H{
			"name": c.cfg.Original.GetString("cert.name"),
			"file": c.cfg.Original.GetString("cert.file"),
			"key":  c.cfg.Original.GetString("cert.key"),
		}
	}
	return data
}

func (c *APIClient) systemProxySettings() system.ProxySettings {
	cfg := c.proxyConfigData()
	return system.ProxySettings{
		Hostname: fmt.Sprint(cfg["hostname"]),
		Port:     strconv.Itoa(proxyFirstPositive(proxyToIntDefault(cfg["port"], 2023), 2023)),
	}
}

func convertProxyConfigValues(values map[string]interface{}) (map[string]interface{}, error) {
	updated := map[string]interface{}{}
	for key, value := range values {
		converted, err := convertProxyConfigValue(key, value)
		if err != nil {
			return nil, err
		}
		updated[key] = converted
	}
	return updated, nil
}

func convertProxyConfigValue(key string, value interface{}) (interface{}, error) {
	switch key {
	case "proxy.hostname", "proxy.tcpRelay.hostname", "proxy.defaultInterface", "proxy.upstreamProxy", "cert.file", "cert.key", "cert.name":
		return strings.TrimSpace(fmt.Sprint(value)), nil
	case "proxy.port", "proxy.tcpRelay.port":
		return serviceConfigPort(value)
	case "proxy.system", "proxy.tun", "proxy.tcpRelay.enabled", "proxy.skipInstallRootCert":
		return serviceConfigBool(value)
	default:
		return nil, fmt.Errorf("未知配置项: %s", key)
	}
}

func serviceConfigBool(value interface{}) (bool, error) {
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

func (c *APIClient) proxyServiceRunning() bool {
	if c.serviceMgr == nil {
		return false
	}
	status, err := c.serviceMgr.GetStatus("interceptor")
	if err != nil {
		return false
	}
	return status == manager.StatusRunning || status == manager.StatusStarting || status == manager.StatusStopping
}

func (c *APIClient) restartProxyService() error {
	if c.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}
	status, _ := c.serviceMgr.GetStatus("interceptor")
	if status == manager.StatusRunning || status == manager.StatusStarting || status == manager.StatusStopping {
		if err := c.serviceMgr.StopServer("interceptor"); err != nil {
			return err
		}
	}
	if err := c.applyProxySettingsFromConfig(); err != nil {
		return err
	}
	return c.serviceMgr.StartServer("interceptor")
}

func (c *APIClient) applyProxySettingsFromConfig() error {
	if c.serviceMgr == nil {
		return nil
	}
	server := c.serviceMgr.GetServer("interceptor")
	if server == nil {
		return nil
	}
	interceptorServer, ok := server.(*interceptor.InterceptorServer)
	if !ok {
		return fmt.Errorf("interceptor service type mismatch")
	}
	interceptorServer.ApplySettings(interceptor.NewInterceptorSettings(c.cfg.Original), config.LoadCertFiles())
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

func getConfigString(c *config.Config, key string) string {
	if c == nil {
		return ""
	}
	return c.GetString(key)
}

func getConfigInt(c *config.Config, key string) int {
	if c == nil {
		return 0
	}
	return c.GetInt(key)
}

func proxyFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
