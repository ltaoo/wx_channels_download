package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/adrg/xdg"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
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
	logger   *zerolog.Logger
	log_file *os.File
	log_path string

	// Resolved global script
	GlobalScriptPath string // Absolute path to configured global script

	// Resolved hook script
	HookScriptPath string // Absolute path to configured download hook script

	// Resolved content script
	ContentScriptContent string // Content of content script

	DBType     string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPath     string
}

// UpdateSource represents one auto-update source. Its fields intentionally
// match velo/updater/types.UpdateSource so the same YAML can be reused.
type UpdateSource struct {
	Type              string `json:"type" yaml:"type" mapstructure:"type"`
	Priority          int    `json:"priority" yaml:"priority" mapstructure:"priority"`
	GitHubRepo        string `json:"github_repo,omitempty" yaml:"github_repo,omitempty" mapstructure:"github_repo"`
	GitHubToken       string `json:"github_token,omitempty" yaml:"github_token,omitempty" mapstructure:"github_token"`
	ManifestURL       string `json:"manifest_url,omitempty" yaml:"manifest_url,omitempty" mapstructure:"manifest_url"`
	SelfURL           string `json:"self_url,omitempty" yaml:"self_url,omitempty" mapstructure:"self_url"`
	Enabled           bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	NeedCheckChecksum bool   `json:"need_check_checksum" yaml:"need_check_checksum" mapstructure:"need_check_checksum"`
}

// LoadUpdateSources decodes update.sources from the active Viper config.
func LoadUpdateSources() ([]UpdateSource, error) {
	var sources []UpdateSource
	if err := viper.UnmarshalKey("update.sources", &sources); err != nil {
		return nil, fmt.Errorf("decode update sources: %w", err)
	}
	return sources, nil
}

const EnvConfigPath = "WX_CHANNELS_DOWNLOAD_CONFIG_FILEPATH"

var config_write_mu sync.Mutex

func New(ver string, mode string, logger *zerolog.Logger, log_file *os.File, log_path string) *Config {
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
		c := &Config{
			RootDir:  base_dir,
			Filename: filename,
			FullPath: config_filepath,
			Existing: has_config,
			Version:  ver,
			Mode:     mode,
		}
		c.set_logger(logger, log_file, log_path)
		return c
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
	c.set_logger(logger, log_file, log_path)
	return c
}

func (c *Config) set_logger(logger *zerolog.Logger, log_file *os.File, log_path string) {
	if c == nil {
		return
	}
	c.logger = logger
	c.log_file = log_file
	c.log_path = log_path
}

func (c *Config) Logger() *zerolog.Logger {
	return c.logger
}

func (c *Config) LogFile() *os.File {
	return c.log_file
}

func (c *Config) LogPath() string {
	return c.log_path
}

func (ctx ConfigValueContext) Get(key string) interface{} {
	if ctx.Config == nil {
		return viper.Get(key)
	}
	return ctx.Config.Get(key)
}

func (ctx ConfigValueContext) GetString(key string) string {
	if ctx.Config == nil {
		return viper.GetString(key)
	}
	return ctx.Config.GetString(key)
}

func (ctx ConfigValueContext) GetInt(key string) int {
	if ctx.Config == nil {
		return viper.GetInt(key)
	}
	return ctx.Config.GetInt(key)
}

func (ctx ConfigValueContext) GetBool(key string) bool {
	if ctx.Config == nil {
		return viper.GetBool(key)
	}
	return ctx.Config.GetBool(key)
}

func (ctx ConfigValueContext) RootDir() string {
	if ctx.Config == nil {
		return ""
	}
	return ctx.Config.RootDir
}

func (ctx ConfigValueContext) WorkDir() string {
	if ctx.Config == nil {
		return ""
	}
	if strings.TrimSpace(ctx.Config.WorkDir) != "" {
		return ctx.Config.WorkDir
	}
	return ctx.Config.GetString("workdir")
}

func ResolveWorkDirValue(value interface{}, ctx ConfigValueContext) interface{} {
	root_dir := strings.TrimSpace(ctx.RootDir())
	return resolve_path_value(value, root_dir, root_dir, root_dir)
}

func ResolveWorkDirPathValue(value interface{}, ctx ConfigValueContext) interface{} {
	work_dir := strings.TrimSpace(ctx.WorkDir())
	return ResolveWorkDirPath(value, work_dir)
}

// ResolveWorkDirPath expands config path placeholders and resolves relative paths
// from the runtime working directory.
func ResolveWorkDirPath(value interface{}, work_dir string) string {
	work_dir = strings.TrimSpace(work_dir)
	return resolve_path_value(value, work_dir, work_dir, "")
}

func resolve_path_value(value interface{}, base_dir string, cwd string, fallback string) string {
	path := strings.TrimSpace(config_value_string(value))
	if path == "" {
		path = fallback
	}
	path = strings.ReplaceAll(path, "%UserDownloads%", xdg.UserDirs.Download)
	path = strings.ReplaceAll(path, "%CWD%", cwd)
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) && base_dir != "" {
		path = filepath.Join(base_dir, path)
	}
	return path
}

func (c *Config) LoadConfig() error {
	Register(ConfigField{
		Key:          "workdir",
		Type:         ConfigTypeString,
		Default:      "",
		Description:  "运行时工作目录，数据库、用户脚本等运行时文件将写入该目录",
		Title:        "工作目录",
		Group:        "General",
		ProcessValue: ResolveWorkDirValue,
	})
	Register(ConfigField{
		Key:         "proxy.enabled",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "是否启动代理服务",
		Title:       "启动代理服务",
		Group:       "Proxy",
	})
	Register(ConfigField{
		Key:         "proxy.system",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "是否设置系统代理为代理服务",
		Title:       "设置系统代理",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "proxy.hostname",
		Type:        ConfigTypeString,
		Default:     "127.0.0.1",
		Description: "代理服务的主机名",
		Title:       "代理主机",
		Group:       "Proxy",
	})
	Register(ConfigField{
		Key:         "proxy.port",
		Type:        ConfigTypeInt,
		Default:     2023,
		Description: "代理服务的端口",
		Title:       "代理端口",
		Group:       "Proxy",
	})
	Register(ConfigField{
		Key:         "proxy.tcpRelay.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否启用 TCP relay，用于接收 iptables/nftables 透明重定向的原始 TCP 流量",
		Title:       "启用 TCP Relay",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "proxy.tcpRelay.hostname",
		Type:        ConfigTypeString,
		Default:     "127.0.0.1",
		Description: "TCP relay 监听主机名",
		Title:       "TCP Relay 主机",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "proxy.tcpRelay.port",
		Type:        ConfigTypeInt,
		Default:     9900,
		Description: "TCP relay 监听端口，必须与代理端口不同",
		Title:       "TCP Relay 端口",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "cert.file",
		Type:        ConfigTypeFile,
		Default:     "",
		Description: "自定义证书文件绝对路径",
		Title:       "证书文件",
		Group:       "Proxy",
		Accept:      ".pem,.cer,.crt,.key",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "cert.key",
		Type:        ConfigTypeFile,
		Default:     "",
		Description: "自定义私钥文件绝对路径",
		Title:       "私钥文件",
		Group:       "Proxy",
		Accept:      ".pem,.key",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "cert.name",
		Type:        ConfigTypeString,
		Default:     "Echo",
		Description: "自定义证书名称",
		Title:       "证书名称",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "proxy.tun",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "启用 TUN 模式（网络层流量转发），开启后不会设置系统代理",
		Title:       "TUN 模式",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "proxy.defaultInterface",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "TUN 模式下指定默认出口网卡名称，留空时自动检测",
		Title:       "默认网卡",
		Group:       "Proxy",
		HotReload:   true,
	})
	// 不支持热重载：切换网络服务需要先关闭旧服务上的系统代理，否则旧服务会残留一条
	// 指向已停止的代理的设置，导致该服务被选为主服务时无法联网。
	Register(ConfigField{
		Key:         "proxy.networkService",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "设置系统代理时使用的网络服务名称，留空时自动取当前主服务。仅在自动检测失败时才需要指定，可用值见 networksetup -listallnetworkservices",
		Title:       "网络服务",
		Group:       "Proxy",
	})
	Register(ConfigField{
		Key:         "proxy.skipInstallRootCert",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否跳过安装根证书（需要自行手动信任/导入证书）",
		Title:       "不安装根证书",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "proxy.upstreamProxy",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "上游代理地址，用于转发所有请求到指定代理（如 http://127.0.0.1:7890）",
		Title:       "上游代理",
		Group:       "Proxy",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "debug.error",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "是否全局捕获前端错误，出现错误时弹窗展示错误信息",
		Title:       "错误展示",
		Group:       "Debug",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "debug.echolog",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否启用 Echo 代理日志",
		Title:       "Echo 日志",
		Group:       "Debug",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:          "inject.globalScript",
		Type:         ConfigTypeString,
		Default:      "global.js",
		Description:  "全局用户脚本",
		Title:        "全局脚本",
		Group:        "Inject",
		HotReload:    true,
		ProcessValue: ResolveWorkDirPathValue,
	})
	Register(ConfigField{
		Key:         "inject.contentScript",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "注入到页面的内容脚本路径",
		Title:       "内容脚本",
		Group:       "Inject",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:          "download.dir",
		Type:         ConfigTypeString,
		Default:      "%UserDownloads%",
		Description:  "指定下载的目录，当 frontend 为 true 时不生效",
		Title:        "下载目录",
		Group:        "Download",
		ProcessValue: ResolveWorkDirPathValue,
	})
	Register(ConfigField{
		Key:         "download.filenameTemplate",
		Type:        ConfigTypeString,
		Default:     "{{filename}}_{{spec}}",
		Description: "用于配置下载文件的名称，支持 {{filename}} 和 {{spec}} 等变量",
		Title:       "文件名模板",
		Group:       "Download",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:          "download.hooksScript",
		Type:         ConfigTypeString,
		Default:      "hooks.js",
		Description:  "下载任务 Hook 脚本",
		Title:        "Hook 脚本",
		Group:        "Download",
		HotReload:    true,
		ProcessValue: ResolveWorkDirPathValue,
	})
	Register(ConfigField{
		Key:         "download.playDoneAudio",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "下载完成时是否播放完成音效",
		Title:       "播放完成音效",
		Group:       "Download",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "download.defaultActionWhenExisting",
		Type:        ConfigTypeSelect,
		Default:     "",
		Options:     []string{"", "skip", "duplicate", "overwrite"},
		Description: "Default action when a download task already exists; leave empty to return a conflict for the caller to choose",
		Title:       "Default duplicate action",
		Group:       "Download",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "db.type",
		Type:        ConfigTypeSelect,
		Default:     "sqlite",
		Options:     []string{"sqlite", "mysql", "postgres"},
		Description: "数据库类型",
		Title:       "数据库类型",
		Group:       "Database",
	})
	Register(ConfigField{
		Key:         "db.host",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库主机名",
		Title:       "数据库主机",
		Group:       "Database",
	})
	Register(ConfigField{
		Key:         "db.port",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库端口",
		Title:       "数据库端口",
		Group:       "Database",
	})
	Register(ConfigField{
		Key:         "db.username",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库用户名",
		Title:       "数据库用户名",
		Group:       "Database",
	})
	Register(ConfigField{
		Key:         "db.password",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库密码",
		Title:       "数据库密码",
		Group:       "Database",
		Sensitive:   true,
	})
	Register(ConfigField{
		Key:         "db.filename",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "数据库名称（mysql/postgres）或文件名（sqlite）",
		Title:       "数据库名称",
		Group:       "Database",
	})
	Register(ConfigField{
		Key:          "db.filepath",
		Type:         ConfigTypeString,
		Default:      "%CWD%/data.db",
		Description:  "SQLite 数据库文件路径",
		Title:        "SQLite 路径",
		Group:        "Database",
		ProcessValue: ResolveWorkDirPathValue,
	})
	Register(ConfigField{
		Key:         "api.protocol",
		Type:        ConfigTypeString,
		Default:     "http",
		Description: "指定 API 服务的协议头",
		Title:       "API 服务协议",
		Group:       "API",
		Readonly:    true,
	})
	Register(ConfigField{
		Key:         "api.hostname",
		Type:        ConfigTypeString,
		Default:     "127.0.0.1",
		Description: "指定 API 服务的主机名",
		Title:       "API 服务主机",
		Group:       "API",
	})
	Register(ConfigField{
		Key:         "api.port",
		Type:        ConfigTypeInt,
		Default:     2022,
		Description: "指定 API 服务的端口",
		Title:       "API 服务端口",
		Group:       "API",
	})
	Register(ConfigField{
		Key:         "mcp.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否在应用启动时初始化并启用 MCP 服务；关闭时仍可通过 API 按需启用",
		Title:       "启动 MCP 服务",
		Group:       "MCP",
	})
	Register(ConfigField{
		Key:         "scraper.retainedJobs",
		Type:        ConfigTypeInt,
		Default:     20,
		Description: "内存中保留的已完成、失败或中断抓取任务数量",
		Title:       "抓取任务保留数量",
		Group:       "Scraper",
	})
	Register(ConfigField{
		Key:         "cookie.uuid",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "允许向 Cookie 更新接口提交数据的 CookieCloud UUID；留空时接受请求中的 UUID",
		Title:       "CookieCloud UUID",
		Group:       "Cookie",
	})
	Register(ConfigField{
		Key:         "cookie.password",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "CookieCloud 密码，用于派生 Cookie 更新接口的解密密钥",
		Title:       "CookieCloud 密码",
		Group:       "Cookie",
		Sensitive:   true,
	})
	Register(ConfigField{
		Key:         "cookie.key",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "可选的 16 位 CookieCloud 解密密钥；配置后优先于 password",
		Title:       "CookieCloud 密钥",
		Group:       "Cookie",
		Sensitive:   true,
	})
	Register(ConfigField{
		Key:         "cloudflare.accountId",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "Cloudflare 帐号 ID",
		Title:       "Account ID",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "cloudflare.apiToken",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "Cloudflare Worker 认证 Token",
		Title:       "API Token",
		Group:       "Cloudflare",
		HotReload:   true,
		Sensitive:   true,
	})
	Register(ConfigField{
		Key:         "cloudflare.refreshToken",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "调用 mp-rss 凭证刷新接口所需的 token",
		Title:       "Refresh Token",
		Group:       "Cloudflare",
		HotReload:   true,
		Sensitive:   true,
	})
	Register(ConfigField{
		Key:         "cloudflare.adminToken",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "调用 mp-rss 管理员接口所需的凭证",
		Title:       "Admin Token",
		Group:       "Cloudflare",
		HotReload:   true,
		Sensitive:   true,
	})
	Register(ConfigField{
		Key:         "cloudflare.workerName",
		Type:        ConfigTypeString,
		Default:     "official-account-api",
		Description: "Cloudflare mp-rss Worker 名称",
		Title:       "Worker Name",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "cloudflare.d1Id",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "Cloudflare mp-rss d1数据库 ID",
		Title:       "D1 Database ID",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "cloudflare.d1Name",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "Cloudflare mp-rss d1数据库 Name",
		Title:       "D1 Database Name",
		Group:       "Cloudflare",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "bridge.deploy.workerName",
		Type:        ConfigTypeString,
		Default:     "dm-bridge",
		Description: "部署 Durable Objects Bridge 桥接/转发服务时使用的 Cloudflare Worker 名称",
		Title:       "Bridge Worker 名称",
		Group:       "Bridge",
	})
	Register(ConfigField{
		Key:         "bridge.deploy.pagesProjectName",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "Bridge 管理页面的 Cloudflare Pages 项目名；留空时使用 <workerName>-admin",
		Title:       "Bridge Pages 项目名",
		Group:       "Bridge",
	})
	Register(ConfigField{
		Key:         "bridge.deploy.token",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "部署为 BRIDGE_TOKEN secret 的设备连接凭证；不要分发给外部调用者",
		Title:       "Bridge 设备 Secret",
		Group:       "Bridge",
		Sensitive:   true,
	})
	Register(ConfigField{
		Key:         "bridge.deploy.adminToken",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "保护 Bridge 管理页面和调用 Token 管理 API 的独立管理员密码，不能与设备 Secret 相同",
		Title:       "Bridge 管理员 Token",
		Group:       "Bridge",
		Sensitive:   true,
	})
	Register(ConfigField{
		Key:         "bridge.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否将当前操作系统设备连接到个人 Bridge 桥接/转发服务",
		Title:       "启用 Bridge 设备连接",
		Group:       "Bridge",
	})
	Register(ConfigField{
		Key:         "bridge.url",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "个人 Bridge 桥接/转发 Worker 的 HTTPS 地址",
		Title:       "Bridge 地址",
		Group:       "Bridge",
	})
	Register(ConfigField{
		Key:         "bridge.deviceId",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "当前操作系统实例的稳定唯一标识；留空时使用系统主机名",
		Title:       "设备 ID",
		Group:       "Bridge",
	})
	Register(ConfigField{
		Key:         "bridge.deviceName",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "管理页中显示的设备名称；留空时使用系统主机名",
		Title:       "设备名称",
		Group:       "Bridge",
	})
	Register(ConfigField{
		Key:         "bridge.token",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "设备连接个人 Bridge 使用的 Secret；必须与 Bridge 部署的 BRIDGE_TOKEN 一致",
		Title:       "Bridge 设备 Secret",
		Group:       "Bridge",
		Sensitive:   true,
	})
	Register(ConfigField{
		Key:         "bridge.httpTimeoutSeconds",
		Type:        ConfigTypeInt,
		Default:     30,
		Description: "设备调用 Bridge HTTP API 的超时时间（秒）",
		Title:       "Bridge 请求超时",
		Group:       "Bridge",
	})
	Register(ConfigField{
		Key:         "bridge.methods",
		Type:        ConfigTypeString,
		Default:     "auto",
		Description: "当前设备开放的方法；auto 自动发布全部已注册方法，none 不执行远程调用，也可填写逗号分隔的方法名",
		Title:       "Bridge 方法白名单",
		Group:       "Bridge",
	})
	// Update auto-update configuration
	Register(ConfigField{
		Key:         "update.proxy",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "update 命令从 GitHub 下载更新时使用的代理地址（如 http://127.0.0.1:7890），与 proxy.upstreamProxy 不同",
		Title:       "更新代理",
		Group:       "Update",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "update.mirror",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "update 命令从 GitHub 下载更新时使用的镜像地址（如 https://ghproxy.com/），会拼接在原始 URL 之前",
		Title:       "更新镜像",
		Group:       "Update",
		HotReload:   true,
	})

	// FileHelper WeChat file transfer helper configuration
	Register(ConfigField{
		Key:         "filehelper.enabled",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "是否开启文件传输助手自动下载视频号功能",
		Title:       "自动下载",
		Group:       "FileHelper",
		HotReload:   true,
	})
	Register(ConfigField{
		Key:         "filehelper.callbackUrl",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "文件传输助手消息回调地址",
		Title:       "回调地址",
		Group:       "FileHelper",
		HotReload:   true,
	})
	Register(ConfigField{
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

	c.DBType = c.GetString("db.type")
	c.DBHost = c.GetString("db.host")
	c.DBPort = c.GetString("db.port")
	c.DBUser = c.GetString("db.username")
	c.DBPassword = c.GetString("db.password")
	c.DBName = c.GetString("db.filename")

	work_dir := c.GetString("workdir")
	c.WorkDir = work_dir
	if err := os.MkdirAll(c.WorkDir, 0755); err != nil {
		c.Error = err
		return err
	}

	c.DBPath = c.GetString("db.filepath")

	// Resolve inject scripts
	c.resolve_script_path("inject.globalScript", &c.GlobalScriptPath)
	c.resolve_script_path("download.hooksScript", &c.HookScriptPath)
	c.resolve_script("inject.contentScript", &c.ContentScriptContent)

	return nil
}

func (c *Config) resolve_script_path(config_key string, path_field *string) {
	raw_script_path := strings.TrimSpace(viper.GetString(config_key))
	script_path := c.GetString(config_key)
	if script_path == "" {
		c.logger.Info().
			Str("file", "internal/config/config.go").
			Str("config_key", config_key).
			Str("raw_path", raw_script_path).
			Str("workdir", c.WorkDir).
			Str("rootdir", c.RootDir).
			Msg("config resolve_script_path: configured script path is not resolved")
		return
	}
	if !filepath.IsAbs(script_path) {
		base_dir := c.WorkDir
		if strings.TrimSpace(base_dir) == "" {
			base_dir = c.RootDir
		}
		script_path = filepath.Join(base_dir, script_path)
	}
	script_path = filepath.Clean(script_path)
	info, err := os.Stat(script_path)
	if err != nil {
		c.logger.Info().
			Err(err).
			Str("file", "internal/config/config.go").
			Str("config_key", config_key).
			Str("raw_path", raw_script_path).
			Str("workdir", c.WorkDir).
			Str("rootdir", c.RootDir).
			Str("resolved_path", script_path).
			Msg("config resolve_script_path: configured script path does not exist")
		return
	}
	*path_field = script_path
	c.logger.Info().
		Str("file", "internal/config/config.go").
		Str("config_key", config_key).
		Str("raw_path", raw_script_path).
		Str("workdir", c.WorkDir).
		Str("rootdir", c.RootDir).
		Str("resolved_path", script_path).
		Bool("is_dir", info.IsDir()).
		Int64("size", info.Size()).
		Msg("config resolve_script_path: configured script path resolved")
}

func (c *Config) resolve_script(config_key string, content_field *string) {
	script_path := c.GetString(config_key)
	if script_path == "" {
		return
	}
	if !filepath.IsAbs(script_path) {
		script_path = filepath.Join(c.RootDir, script_path)
	}
	script_path = filepath.Clean(script_path)
	data, err := os.ReadFile(script_path)
	if err != nil {
		return
	}
	*content_field = string(data)
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
	config_write_mu.Lock()
	defer config_write_mu.Unlock()
	return c.save_atomic()
}

// UpdateAndSave applies a validated set of values and atomically persists the
// resulting configuration file.
func (c *Config) UpdateAndSave(values map[string]interface{}) error {
	config_write_mu.Lock()
	defer config_write_mu.Unlock()

	previous_values := make(map[string]interface{}, len(values))
	for key, value := range values {
		previous_values[key] = viper.Get(key)
		viper.Set(key, value)
	}
	if err := c.save_atomic(); err != nil {
		for key, value := range previous_values {
			viper.Set(key, value)
		}
		return err
	}
	c.Existing = true
	return nil
}

func (c *Config) save_atomic() error {
	config_path := strings.TrimSpace(c.FullPath)
	if config_path == "" {
		return fmt.Errorf("配置文件路径为空")
	}
	config_dir := filepath.Dir(config_path)
	if err := os.MkdirAll(config_dir, 0755); err != nil {
		return err
	}
	extension := filepath.Ext(config_path)
	if extension == "" {
		extension = ".yaml"
	}
	base_name := strings.TrimSuffix(filepath.Base(config_path), filepath.Ext(config_path))
	temp_file, err := os.CreateTemp(config_dir, "."+base_name+"-*.tmp"+extension)
	if err != nil {
		return err
	}
	temp_path := temp_file.Name()
	if err := temp_file.Close(); err != nil {
		_ = os.Remove(temp_path)
		return err
	}
	defer os.Remove(temp_path)

	if info, stat_err := os.Stat(config_path); stat_err == nil {
		if chmod_err := os.Chmod(temp_path, info.Mode().Perm()); chmod_err != nil {
			return chmod_err
		}
	}
	if err := viper.WriteConfigAs(temp_path); err != nil {
		return err
	}
	return os.Rename(temp_path, config_path)
}

func (c *Config) GetAll() map[string]interface{} {
	return viper.AllSettings()
}

// GetRaw returns the unprocessed value selected by Viper.
func (c *Config) GetRaw(key string) interface{} {
	return viper.Get(key)
}

// IsInConfig reports whether a value was explicitly present in config.yaml.
func (c *Config) IsInConfig(key string) bool {
	return viper.InConfig(key)
}

// Revision returns a stable digest of all registered raw configuration values.
// It can be compared across processes without exposing sensitive values.
func (c *Config) Revision() (string, error) {
	values := viper.AllSettings()
	data, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("编码配置摘要失败: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func (c *Config) Get(key string) interface{} {
	return c.process_value(key, viper.Get(key))
}

// Typed getters with dotted path support, e.g. "a.b.c"
func (c *Config) GetString(path string) string {
	if !has_value_processor(path) {
		return viper.GetString(path)
	}
	return config_value_string(c.process_value(path, viper.Get(path)))
}

func (c *Config) GetInt(path string) int {
	if !has_value_processor(path) {
		return viper.GetInt(path)
	}
	return config_value_int(c.process_value(path, viper.Get(path)))
}

func (c *Config) GetBool(path string) bool {
	if !has_value_processor(path) {
		return viper.GetBool(path)
	}
	return config_value_bool(c.process_value(path, viper.Get(path)))
}

func (c *Config) GetStringSlice(path string) []string {
	return viper.GetStringSlice(path)
}

func (c *Config) IsSet(path string) bool {
	return viper.IsSet(path)
}

func (c *Config) GetFloat64(path string) float64 {
	if !has_value_processor(path) {
		return viper.GetFloat64(path)
	}
	return config_value_float64(c.process_value(path, viper.Get(path)))
}

func (c *Config) process_value(path string, value interface{}) interface{} {
	item, ok := Lookup(path)
	if !ok || item.ProcessValue == nil {
		return value
	}
	return item.ProcessValue(value, ConfigValueContext{Config: c})
}

func has_value_processor(path string) bool {
	item, ok := Lookup(path)
	return ok && item.ProcessValue != nil
}

// GetDownloadDir resolves and returns the absolute download directory path.
func (c *Config) GetDownloadDir() string {
	return c.GetString("download.dir")
}

func config_value_string(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func config_value_int(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(v))
		return i
	case bool:
		if v {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func config_value_bool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(strings.TrimSpace(v))
		return b
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	case float32:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func config_value_float64(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	case bool:
		if v {
			return 1
		}
		return 0
	default:
		return 0
	}
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
