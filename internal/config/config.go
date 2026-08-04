package config

import (
	"bytes"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"

	"wx_channel/pkg/certificate"
)

type Config struct {
	RootDir  string // Directory where the binary is located
	WorkDir  string // Runtime data directory
	Filename string // Config file name
	FullPath string // Full path to the config file
	Existing bool   // Whether the config file exists
	Error    error
	Debug    bool
	Version  string
	Mode     string

	DBType         string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBPath         string
	MigrationsPath string
}

const EnvConfigPath = "WX_CHANNELS_DOWNLOAD_CONFIG_FILEPATH"

func New(ver string, mode string) *Config {
	exe, _ := os.Executable()
	exe_dir := filepath.Dir(exe)
	base_dir := exe_dir
	var config_filepath string
	var has_config bool
	filename := "config.yaml"
	if env_config_filepath := strings.TrimSpace(os.Getenv(EnvConfigPath)); env_config_filepath != "" {
		config_filepath = env_config_filepath
		if abs, err := filepath.Abs(env_config_filepath); err == nil {
			config_filepath = abs
		}
		base_dir = filepath.Dir(config_filepath)
		filename = filepath.Base(config_filepath)
		if _, err := os.Stat(config_filepath); err == nil {
			has_config = true
		}
		viper.SetConfigFile(config_filepath)
		return &Config{
			RootDir:  base_dir,
			Filename: filename,
			FullPath: config_filepath,
			Existing: has_config,
			Version:  ver,
			Mode:     mode,
		}
	}

	var candidates []string
	candidates = append(candidates, exe_dir)
	if _, caller_file, _, ok := runtime.Caller(1); ok {
		caller_dir := filepath.Dir(caller_file)
		candidates = append(candidates, caller_dir)
	}
	if _, this_file, _, ok2 := runtime.Caller(0); ok2 {
		cfg_dir := filepath.Dir(this_file)
		proj_root := filepath.Dir(cfg_dir)
		candidates = append(candidates, proj_root)
	}
	for _, dir := range candidates {
		p := filepath.Join(dir, filename)
		if _, err := os.Stat(p); err == nil {
			base_dir = dir
			config_filepath = p
			has_config = true
			break
		}
	}
	if config_filepath == "" {
		config_filepath = filepath.Join(base_dir, filename)
	}
	viper.SetConfigFile(config_filepath)
	c := &Config{
		RootDir:  base_dir,
		WorkDir:  base_dir,
		Filename: filename,
		FullPath: config_filepath,
		Existing: has_config,
		Version:  ver,
		Mode:     mode,
	}
	return c
}

func (c *Config) LoadConfig() error {
	Register(ConfigItem{
		Key:         "workdir",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "运行时工作目录，日志、数据库等运行时文件将写入该目录",
		Title:       "工作目录",
		Group:       "General",
	})
	Register(ConfigItem{
		Key:         "proxy.system",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "是否设置系统代理为代理服务",
		Title:       "设置系统代理",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "proxy.hostname",
		Type:        ConfigTypeString,
		Default:     "127.0.0.1",
		Description: "代理服务的主机名",
		Title:       "代理主机",
		Group:       "Proxy",
	})
	Register(ConfigItem{
		Key:         "proxy.port",
		Type:        ConfigTypeInt,
		Default:     2023,
		Description: "代理服务的端口",
		Title:       "代理端口",
		Group:       "Proxy",
	})
	Register(ConfigItem{
		Key:         "proxy.tcpRelay.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否启用 TCP relay，用于接收 iptables/nftables 透明重定向的原始 TCP 流量",
		Title:       "启用 TCP Relay",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "proxy.tcpRelay.hostname",
		Type:        ConfigTypeString,
		Default:     "127.0.0.1",
		Description: "TCP relay 监听主机名",
		Title:       "TCP Relay 主机",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "proxy.tcpRelay.port",
		Type:        ConfigTypeInt,
		Default:     9900,
		Description: "TCP relay 监听端口，必须与代理端口不同",
		Title:       "TCP Relay 端口",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "cert.file",
		Type:        ConfigTypeFile,
		Default:     "",
		Description: "自定义证书文件绝对路径",
		Title:       "证书文件",
		Group:       "Proxy",
		Accept:      ".pem,.cer,.crt,.key",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "cert.key",
		Type:        ConfigTypeFile,
		Default:     "",
		Description: "自定义私钥文件绝对路径",
		Title:       "私钥文件",
		Group:       "Proxy",
		Accept:      ".pem,.key",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "cert.name",
		Type:        ConfigTypeString,
		Default:     "Echo",
		Description: "自定义证书名称",
		Title:       "证书名称",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "proxy.tun",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "启用 TUN 模式（网络层流量转发），开启后不会设置系统代理",
		Title:       "TUN 模式",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "proxy.defaultInterface",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "TUN 模式下指定默认出口网卡名称，留空时自动检测",
		Title:       "默认网卡",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "proxy.skipInstallRootCert",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否跳过安装根证书（需要自行手动信任/导入证书）",
		Title:       "不安装根证书",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "proxy.upstreamProxy",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "上游代理地址，用于转发所有请求到指定代理（如 http://127.0.0.1:7890）",
		Title:       "上游代理",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "pagespy.enabled",
		Type:        ConfigTypeSelect,
		Default:     false,
		Description: "是否开启 PageSpy",
		Title:       "启用",
		Group:       "Pagespy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "pagespy.protocol",
		Type:        ConfigTypeSelect,
		Default:     "http",
		Options:     []string{"http", "https"},
		Description: "PageSpy 调试协议",
		Title:       "协议头",
		Group:       "Pagespy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "pagespy.api",
		Type:        ConfigTypeString,
		Default:     "127.0.0.1:6752",
		Description: "PageSpy 调试 API 地址",
		Title:       "API 地址",
		Group:       "Pagespy",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "debug.error",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "是否全局捕获前端错误，出现错误时弹窗展示错误信息",
		Title:       "错误展示",
		Group:       "Debug",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "debug.echolog",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否启用 Echo 代理日志",
		Title:       "Echo 日志",
		Group:       "Debug",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "inject.extraScript.afterJSMain",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "额外注入的 JS 脚本路径",
		Title:       "注入脚本",
		Group:       "Inject",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "inject.globalScript",
		Type:        ConfigTypeString,
		Default:     "global.js",
		Description: "全局用户脚本",
		Title:       "全局脚本",
		Group:       "Inject",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "download.dir",
		Type:        ConfigTypeString,
		Default:     "%UserDownloads%",
		Description: "指定下载的目录，当 frontend 为 true 时不生效",
		Title:       "下载目录",
		Group:       "Download",
	})
	Register(ConfigItem{
		Key:         "download.filenameTemplate",
		Type:        ConfigTypeString,
		Default:     "{{filename}}_{{spec}}",
		Description: "用于配置下载文件的名称，支持 {{filename}} 和 {{spec}} 等变量",
		Title:       "文件名模板",
		Group:       "Download",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "download.playDoneAudio",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "下载完成时是否播放完成音效",
		Title:       "播放完成音效",
		Group:       "Download",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "db.type",
		Type:        ConfigTypeSelect,
		Default:     "sqlite",
		Options:     []string{"sqlite", "mysql", "postgres"},
		Description: "数据库类型",
		Title:       "数据库类型",
		Group:       "Database",
	})
	Register(ConfigItem{
		Key:         "db.host",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库主机名",
		Title:       "数据库主机",
		Group:       "Database",
	})
	Register(ConfigItem{
		Key:         "db.port",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库端口",
		Title:       "数据库端口",
		Group:       "Database",
	})
	Register(ConfigItem{
		Key:         "db.username",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库用户名",
		Title:       "数据库用户名",
		Group:       "Database",
	})
	Register(ConfigItem{
		Key:         "db.password",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库密码",
		Title:       "数据库密码",
		Group:       "Database",
	})
	Register(ConfigItem{
		Key:         "db.filename",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库名称（mysql/postgres）或文件名（sqlite）",
		Title:       "数据库名称",
		Group:       "Database",
	})
	Register(ConfigItem{
		Key:         "db.filepath",
		Type:        ConfigTypeString,
		Default:     "%CWD%/data.db",
		Description: "SQLite 数据库文件路径",
		Title:       "SQLite 路径",
		Group:       "Database",
	})
	Register(ConfigItem{
		Key:         "db.migration",
		Type:        ConfigTypeString,
		Default:     "%CWD%/migrations",
		Description: "数据库迁移文件目录",
		Title:       "迁移目录",
		Group:       "Database",
	})
	Register(ConfigItem{
		Key:         "api.protocol",
		Type:        ConfigTypeString,
		Default:     "http",
		Description: "指定 API 服务的协议头",
		Title:       "API 服务协议",
		Group:       "API",
		Readonly:    true,
	})
	Register(ConfigItem{
		Key:         "api.hostname",
		Type:        ConfigTypeString,
		Default:     "127.0.0.1",
		Description: "指定 API 服务的主机名",
		Title:       "API 服务主机",
		Group:       "API",
	})
	Register(ConfigItem{
		Key:         "api.port",
		Type:        ConfigTypeInt,
		Default:     2022,
		Description: "指定 API 服务的端口",
		Title:       "API 服务端口",
		Group:       "API",
	})
	Register(ConfigItem{
		Key:         "admin.hostname",
		Type:        ConfigTypeString,
		Default:     "127.0.0.1",
		Description: "指定 GUI/Admin 服务的主机名",
		Title:       "Admin 服务主机",
		Group:       "Admin",
	})
	Register(ConfigItem{
		Key:         "admin.port",
		Type:        ConfigTypeInt,
		Default:     2021,
		Description: "指定 GUI/Admin 服务的端口",
		Title:       "Admin 服务端口",
		Group:       "Admin",
	})

	Register(ConfigItem{
		Key:         "sandbox.dockerImage",
		Type:        ConfigTypeString,
		Default:     "lscr.io/linuxserver/chromium:latest",
		Description: "浏览器沙箱 Docker 镜像，默认使用带 Web 桌面的 Chromium 镜像",
		Title:       "沙箱镜像",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.dockerEntrypoint",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "浏览器沙箱 Docker --entrypoint；默认留空以使用 webtop 镜像自己的桌面启动流程",
		Title:       "沙箱 Entrypoint",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.dockerNetwork",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "浏览器沙箱 Docker 网络，留空使用默认网络",
		Title:       "沙箱网络",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.cdpPortMin",
		Type:        ConfigTypeInt,
		Default:     39222,
		Description: "浏览器沙箱 CDP 宿主机端口范围起点",
		Title:       "CDP 端口起点",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.cdpPortMax",
		Type:        ConfigTypeInt,
		Default:     39322,
		Description: "浏览器沙箱 CDP 宿主机端口范围终点",
		Title:       "CDP 端口终点",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.desktopPortMin",
		Type:        ConfigTypeInt,
		Default:     39000,
		Description: "浏览器沙箱 Web 桌面宿主机端口范围起点",
		Title:       "桌面端口起点",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.desktopPortMax",
		Type:        ConfigTypeInt,
		Default:     39122,
		Description: "浏览器沙箱 Web 桌面宿主机端口范围终点",
		Title:       "桌面端口终点",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.resolution",
		Type:        ConfigTypeString,
		Default:     "1920x1080x24",
		Description: "浏览器沙箱 Web 桌面分辨率",
		Title:       "桌面分辨率",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.shmSize",
		Type:        ConfigTypeString,
		Default:     "1g",
		Description: "浏览器沙箱 Docker --shm-size",
		Title:       "共享内存",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.memoryLimit",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "浏览器沙箱 Docker --memory，留空不限制",
		Title:       "内存限制",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "sandbox.chromeCommand",
		Type:        ConfigTypeText,
		Default:     "",
		Description: "浏览器沙箱容器启动命令，留空时自动查找 Chrome/Chromium 并启用 0.0.0.0:9222 remote debugging",
		Title:       "Chrome 启动命令",
		Group:       "Sandbox",
	})
	Register(ConfigItem{
		Key:         "cloudflare.accountId",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "Cloudflare 帐号 ID",
		Title:       "Account ID",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "cloudflare.apiToken",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "Cloudflare Worker 认证 Token",
		Title:       "API Token",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "cloudflare.refreshToken",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "调用 mp-rss 凭证刷新接口所需的 token",
		Title:       "Refresh Token",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "cloudflare.adminToken",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "调用 mp-rss 管理员接口所需的凭证",
		Title:       "Admin Token",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "cloudflare.workerName",
		Type:        ConfigTypeString,
		Default:     "official-account-api",
		Description: "Cloudflare mp-rss Worker 名称",
		Title:       "Worker Name",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "cloudflare.d1Id",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "Cloudflare mp-rss d1数据库 ID",
		Title:       "D1 Database ID",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "cloudflare.d1Name",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "Cloudflare mp-rss d1数据库 Name",
		Title:       "D1 Database Name",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	// Update auto-update configuration
	Register(ConfigItem{
		Key:         "update.proxy",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "update 命令从 GitHub 下载更新时使用的代理地址（如 http://127.0.0.1:7890），与 proxy.upstreamProxy 不同",
		Title:       "更新代理",
		Group:       "Update",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "update.mirror",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "update 命令从 GitHub 下载更新时使用的镜像地址（如 https://ghproxy.com/），会拼接在原始 URL 之前",
		Title:       "更新镜像",
		Group:       "Update",
		HotReload:   true,
	})

	// FileHelper WeChat file transfer helper configuration
	Register(ConfigItem{
		Key:         "filehelper.enabled",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "是否开启文件传输助手自动下载视频号功能",
		Title:       "自动下载",
		Group:       "FileHelper",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "filehelper.callbackUrl",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "文件传输助手消息回调地址",
		Title:       "回调地址",
		Group:       "FileHelper",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "filehelper.syncInterval",
		Type:        ConfigTypeInt,
		Default:     5,
		Description: "消息同步间隔（秒）",
		Title:       "同步间隔",
		Group:       "FileHelper",
		HotReload:   true,
	})

	if c.Existing {
		// config.FilePath = config_filepath
		if err := viper.ReadInConfig(); err != nil {
			var nf viper.ConfigFileNotFoundError
			if !(errors.As(err, &nf) || errors.Is(err, os.ErrNotExist)) {
				c.Error = err
				return err
			}
		}
	}

	// Load plugin configs: each plugin declares its schema and reads its own config
	LoadPluginConfigs()

	c.DBType = viper.GetString("db.type")
	c.DBHost = viper.GetString("db.host")
	c.DBPort = viper.GetString("db.port")
	c.DBUser = viper.GetString("db.username")
	c.DBPassword = viper.GetString("db.password")
	c.DBName = viper.GetString("db.filename")

	workDir := strings.TrimSpace(viper.GetString("workdir"))
	if workDir == "" {
		workDir = c.RootDir
	}
	workDir = strings.ReplaceAll(workDir, "%CWD%", c.RootDir)
	workDir = filepath.Clean(workDir)
	if !filepath.IsAbs(workDir) {
		workDir = filepath.Join(c.RootDir, workDir)
	}
	c.WorkDir = workDir
	if err := os.MkdirAll(c.WorkDir, 0755); err != nil {
		c.Error = err
		return err
	}

	dbPath := viper.GetString("db.filepath")
	dbPath = strings.ReplaceAll(dbPath, "%CWD%", c.WorkDir)
	dbPath = filepath.Clean(dbPath)
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(c.WorkDir, dbPath)
	}
	c.DBPath = dbPath

	migPath := viper.GetString("db.migration")
	migPath = strings.ReplaceAll(migPath, "%CWD%", c.WorkDir)
	migPath = filepath.Clean(migPath)
	if !filepath.IsAbs(migPath) {
		migPath = filepath.Join(c.WorkDir, migPath)
	}
	c.MigrationsPath = migPath

	return nil
}

// GetDebugInfo returns debug information about how the base directory was determined
func (c *Config) GetDebugInfo() map[string]string {
	exe, _ := os.Executable()
	exe_dir := filepath.Dir(exe)

	info := map[string]string{
		"executable":    exe,
		"exe_dir":       exe_dir,
		"base_dir":      c.RootDir,
		"work_dir":      c.WorkDir,
		"config_path":   c.FullPath,
		"config_exists": fmt.Sprintf("%v", c.Existing),
	}

	// Determine run mode
	if filepath.Base(exe_dir) == "exe" || strings.Contains(exe, "go-build") {
		info["run_mode"] = "go run (development)"
	} else {
		info["run_mode"] = "compiled binary"
	}

	return info
}

func (c *Config) Update(key string, value interface{}) {
	viper.Set(key, value)
}

func (c *Config) Save() error {
	return viper.WriteConfigAs(c.FullPath)
}

func (c *Config) GetAll() map[string]interface{} {
	return viper.AllSettings()
}

func (c *Config) Get(key string) interface{} {
	return viper.Get(key)
}

// Typed getters with dotted path support, e.g. "a.b.c"
func (c *Config) GetString(path string) string   { return viper.GetString(path) }
func (c *Config) GetInt(path string) int         { return viper.GetInt(path) }
func (c *Config) GetBool(path string) bool       { return viper.GetBool(path) }
func (c *Config) GetFloat64(path string) float64 { return viper.GetFloat64(path) }

// GetDownloadDir resolves and returns the absolute download directory path.
func (c *Config) GetDownloadDir() string {
	dir := viper.GetString("download.dir")
	dir = strings.ReplaceAll(dir, "%UserDownloads%", xdg.UserDirs.Download)
	dir = strings.ReplaceAll(dir, "%CWD%", c.WorkDir)
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(c.WorkDir, dir)
	}
	return dir
}

func IsMPEnabled() bool {
	if viper.IsSet("mp.enabled") {
		return viper.GetBool("mp.enabled")
	}
	return !viper.GetBool("mp.disabled")
}

func EnsureDirIfMissing(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return err
}

type CertSource string

const (
	CertSourceSunnyNet   CertSource = "sunny_net"
	CertSourceMitmproxy  CertSource = "mitmproxy"
	CertSourceConfigured CertSource = "configured"
	CertSourceGenerated  CertSource = "generated"
)

type CertFilesInfo struct {
	Cert         *certificate.CertFileAndKeyFile
	Source       CertSource
	IsLegacy     bool
	RiskWarnings []string
}

func LoadCertFilesWithInfo() CertFilesInfo {
	cert := LoadCertFiles()
	info := CertFilesInfo{Cert: cert}

	if cert.Name == certificate.DefaultCertFiles.Name {
		info.Source = CertSourceSunnyNet
		info.IsLegacy = true
		info.RiskWarnings = []string{"该证书为旧版SunnyNet证书，使用硬编码密钥对，存在安全风险，建议删除后安装本机专有证书"}
		return info
	}

	if cert.Name == "mitmproxy" {
		info.Source = CertSourceMitmproxy
		info.IsLegacy = true
		info.RiskWarnings = []string{"当前使用第三方mitmproxy证书，非本机生成，存在潜在安全风险"}
		return info
	}

	// cert.file and cert.key are configured; determine if user-configured or app-generated.
	// App-generated certs are written to the certs/ subdirectory of the work dir.
	certFile := viper.GetString("cert.file")
	if certFile != "" {
		if absPath, err := filepath.Abs(certFile); err == nil && isUnderCertsDir(absPath) {
			info.Source = CertSourceGenerated
			return info
		}
	}

	info.Source = CertSourceConfigured
	return info
}

// AvailableCert represents a certificate available to the proxy.
type AvailableCert struct {
	Cert         *certificate.CertFileAndKeyFile
	Source       CertSource
	IsLegacy     bool
	IsActive     bool
	RiskWarnings []string
}

// ScanAvailableCerts returns all certificates known to the system,
// including the built-in SunnyNet cert, mitmproxy cert (if present),
// and the user-configured or generated cert. Exactly one cert is marked active.
func ScanAvailableCerts() []AvailableCert {
	activeCert := LoadCertFiles()
	var certs []AvailableCert

	// 1. SunnyNet (always available as fallback)
	sunnyEntry := AvailableCert{
		Cert:     certificate.DefaultCertFiles,
		Source:   CertSourceSunnyNet,
		IsLegacy: true,
		IsActive: activeCert.Name == certificate.DefaultCertFiles.Name,
		RiskWarnings: []string{
			"该证书为旧版SunnyNet证书，使用硬编码密钥对，存在安全风险，建议替换为本机生成的证书",
		},
	}
	certs = append(certs, sunnyEntry)

	// 2. mitmproxy (available if cert files exist on disk)
	if mitmCert := tryLoadMitmproxyCert(); mitmCert != nil {
		mitmEntry := AvailableCert{
			Cert:     mitmCert,
			Source:   CertSourceMitmproxy,
			IsLegacy: true,
			IsActive: activeCert.Name == "mitmproxy",
			RiskWarnings: []string{
				"当前使用第三方mitmproxy证书，非本机生成，存在潜在安全风险，建议替换为本机生成的证书",
			},
		}
		certs = append(certs, mitmEntry)
	}

	// 3. Configured/generated cert (available if cert.file + cert.key are set)
	if confCert, ok := loadConfiguredCertFiles(); ok {
		source := CertSourceConfigured
		if absPath, err := filepath.Abs(viper.GetString("cert.file")); err == nil && isUnderCertsDir(absPath) {
			source = CertSourceGenerated
		}
		isActive := activeCert.Name == confCert.Name // compare name since object identities differ
		// Also compare by source: only the configured/generated cert can be active
		// when activeCert is neither SunnyNet nor mitmproxy.
		if !isActive && activeCert.Name != certificate.DefaultCertFiles.Name && activeCert.Name != "mitmproxy" {
			isActive = true
		}
		confEntry := AvailableCert{
			Cert:     confCert,
			Source:   source,
			IsLegacy: false,
			IsActive: isActive,
		}
		certs = append(certs, confEntry)
	}

	return certs
}

// tryLoadMitmproxyCert loads the mitmproxy certificate from standard locations.
// Returns nil if the cert files are not found.
func tryLoadMitmproxyCert() *certificate.CertFileAndKeyFile {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".mitmproxy"))
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			dirs = append(dirs, filepath.Join(appdata, "mitmproxy"))
		}
	}
	for _, dir := range dirs {
		cert_path := filepath.Join(dir, "mitmproxy-ca-cert.pem")
		key_path := filepath.Join(dir, "mitmproxy-ca.pem")
		if cert_bytes, err1 := os.ReadFile(cert_path); err1 == nil {
			if key_bytes, err2 := os.ReadFile(key_path); err2 == nil {
				return &certificate.CertFileAndKeyFile{
					Name:       "mitmproxy",
					Cert:       cert_bytes,
					PrivateKey: key_bytes,
				}
			}
		}
		if key_bytes, err := os.ReadFile(key_path); err == nil {
			rest := key_bytes
			var certBlocks [][]byte
			var keyBlock []byte
			for {
				block, rem := pem.Decode(rest)
				if block == nil {
					break
				}
				rest = rem
				if block.Type == "CERTIFICATE" {
					enc := pem.EncodeToMemory(block)
					if enc != nil {
						certBlocks = append(certBlocks, enc)
					}
				} else if strings.Contains(block.Type, "PRIVATE KEY") {
					enc := pem.EncodeToMemory(block)
					if enc != nil {
						keyBlock = enc
					}
				}
			}
			if len(certBlocks) > 0 && len(keyBlock) > 0 {
				return &certificate.CertFileAndKeyFile{
					Name:       "mitmproxy",
					Cert:       bytes.Join(certBlocks, []byte("")),
					PrivateKey: keyBlock,
				}
			}
		}
	}
	return nil
}

func LoadCertFiles() *certificate.CertFileAndKeyFile {
	if cert, ok := loadConfiguredCertFiles(); ok {
		return cert
	}
	if mitmCert := tryLoadMitmproxyCert(); mitmCert != nil {
		return mitmCert
	}
	return certificate.DefaultCertFiles
}

func loadConfiguredCertFiles() (*certificate.CertFileAndKeyFile, bool) {
	cert_filepath := viper.GetString("cert.file")
	certkey_filepath := viper.GetString("cert.key")
	if cert_filepath != "" && certkey_filepath != "" {
		if cert_bytes, err := os.ReadFile(cert_filepath); err == nil {
			if certkey_bytes, err2 := os.ReadFile(certkey_filepath); err2 == nil {
				certname := viper.GetString("cert.name")
				if strings.TrimSpace(certname) == "" {
					certname = certificate.DefaultCertFiles.Name
				}
				return &certificate.CertFileAndKeyFile{
					Name:       certname,
					Cert:       cert_bytes,
					PrivateKey: certkey_bytes,
				}, true
			}
		}
	}
	return nil, false
}

func isUnderCertsDir(absCertPath string) bool {
	return filepath.Base(filepath.Dir(absCertPath)) == "certs"
}
