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

	"github.com/spf13/viper"

	"wx_channel/pkg/certificate"
)

type Config struct {
	RootDir  string // 二进制文件所在目录
	WorkDir  string // 运行时数据目录
	Filename string // 配置文件名
	FullPath string // 配置文件完整路径
	Existing bool   // 配置文件是否存在
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
		Key:         "channels.disableLocationToHome",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否禁止从视频号详情页重定向到首页（视频号默认行为）",
		Title:       "禁止重定向",
		Group:       "Channels",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "channel.disableLocationToHome",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否禁止从视频号详情页重定向到首页（视频号默认行为）",
		Title:       "禁止重定向",
		Group:       "Channels",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "channels.refreshInterval",
		Type:        ConfigTypeInt,
		Default:     0,
		Description: "视频号页面定时刷新时间间隔（秒），0 为不刷新",
		Title:       "定时刷新间隔",
		Group:       "Channels",
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
		Key:         "download.frontend",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否通过前端解密、下载，不调用后台下载能力",
		Title:       "前端下载",
		Group:       "Download",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "download.defaultHighest",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "点击下载图标时是否下载原始视频（该配置不再生效）",
		Title:       "原始视频",
		Group:       "Download",
		HotReload:   true,
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
		Key:         "download.forceCheckAllFeeds",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "批量下载时是否强制检查所有视频",
		Title:       "检查所有视频",
		Group:       "Download",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "download.pauseWhenDownload",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "点击下载时是否暂停播放",
		Title:       "暂停播放",
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
		Key:         "mp.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否启用公众号本地服务，本地服务会提供接口、RSS 等功能",
		Title:       "启用本地服务",
		Group:       "OfficialAccount",
		Deprecated:  true,
	})
	Register(ConfigItem{
		Key:         "mp.remoteServer.protocol",
		Type:        ConfigTypeString,
		Default:     "http",
		Description: "公众号远端服务协议头",
		Title:       "服务协议头",
		Group:       "OfficialAccount",
	})
	Register(ConfigItem{
		Key:         "mp.remoteServer.hostname",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "公众号远端服务主机名",
		Title:       "服务主机名",
		Group:       "OfficialAccount",
	})
	Register(ConfigItem{
		Key:         "mp.remoteServer.port",
		Type:        ConfigTypeInt,
		Default:     80,
		Description: "公众号远端服务端口",
		Title:       "服务端口",
		Group:       "OfficialAccount",
	})
	Register(ConfigItem{
		Key:         "mp.refreshToken",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "公众号远端服务刷新凭证",
		Title:       "刷新凭证",
		Group:       "OfficialAccount",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "mp.tokenFilepath",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "公众号远端服务授权凭证",
		Title:       "授权凭证",
		Group:       "OfficialAccount",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "mp.accountIdsRefreshInterval",
		Type:        ConfigTypeText,
		Default:     []string{},
		Description: "需要定时刷新的帐号列表",
		Title:       "定时刷新列表",
		Group:       "OfficialAccount",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "mp.refreshSkipMinutes",
		Type:        ConfigTypeInt,
		Default:     20,
		Description: "刷新时若账号在最近 N 分钟已更新则跳过",
		Title:       "刷新跳过时间（分钟）",
		Group:       "OfficialAccount",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "zhihu.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否记录知乎页面浏览记录",
		Title:       "记录知乎浏览",
		Group:       "Zhihu",
	})
	Register(ConfigItem{
		Key:         "zhihu.cookie",
		Type:        ConfigTypeText,
		Default:     "",
		Description: "知乎请求 Cookie，用于访问需要登录态的知乎接口",
		Title:       "知乎 Cookie",
		Group:       "Zhihu",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "69shuba.cookie",
		Type:        ConfigTypeText,
		Default:     "",
		Description: "69书吧请求 Cookie，用于访问 Cloudflare 验证后的页面",
		Title:       "69书吧 Cookie",
		Group:       "69shuba",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "69shuba.fetcher",
		Type:        ConfigTypeSelect,
		Default:     "clawreq",
		Options:     []string{"clawreq", "http", "cdp", "sandbox"},
		Description: "69书吧 HTML 抓取方式，clawreq 使用浏览器指纹 HTTP client，http 使用 Go client，cdp 使用 CDP 服务地址，sandbox 使用 webarchive 沙箱浏览器 API",
		Title:       "69书吧抓取方式",
		Group:       "69shuba",
	})
	Register(ConfigItem{
		Key:         "69shuba.cdpEndpoint",
		Type:        ConfigTypeString,
		Default:     "http://127.0.0.1:9222",
		Description: "CDP 服务地址，仅 fetcher=cdp 时使用；可以是本机浏览器或容器暴露的 CDP HTTP/WS 地址",
		Title:       "69书吧 CDP 地址",
		Group:       "69shuba",
	})
	Register(ConfigItem{
		Key:         "69shuba.cdpTimeout",
		Type:        ConfigTypeInt,
		Default:     30,
		Description: "69书吧 CDP 单次页面抓取超时时间（秒）",
		Title:       "69书吧 CDP 超时",
		Group:       "69shuba",
	})
	Register(ConfigItem{
		Key:         "69shuba.cdpWait",
		Type:        ConfigTypeInt,
		Default:     8,
		Description: "69书吧 CDP 页面加载完成后的额外等待时间（秒），用于等待 Cloudflare 跳转",
		Title:       "69书吧 CDP 等待",
		Group:       "69shuba",
	})
	Register(ConfigItem{
		Key:         "69shuba.sandboxAPIBaseURL",
		Type:        ConfigTypeString,
		Default:     "http://127.0.0.1:2021/api/v1",
		Description: "webarchive 风格沙箱 API 地址，仅 fetcher=sandbox 时使用",
		Title:       "69书吧沙箱 API",
		Group:       "69shuba",
	})
	Register(ConfigItem{
		Key:         "69shuba.sandboxID",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "用于抓取 69书吧页面的沙箱 ID，仅 fetcher=sandbox 时使用",
		Title:       "69书吧沙箱 ID",
		Group:       "69shuba",
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
		Key:         "xiaohongshu.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否记录小红书页面浏览记录",
		Title:       "记录小红书浏览",
		Group:       "Xiaohongshu",
	})
	Register(ConfigItem{
		Key:         "bilibili.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否记录 B 站页面浏览记录",
		Title:       "记录 B 站浏览",
		Group:       "Bilibili",
	})
	Register(ConfigItem{
		Key:         "bilibili.cookie",
		Type:        ConfigTypeText,
		Default:     "",
		Description: "B 站请求 Cookie，用于访问账号可看的高清清晰度；不会输出到日志",
		Title:       "B 站 Cookie",
		Group:       "Bilibili",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "youtube.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否记录 YouTube 页面浏览记录",
		Title:       "记录 YouTube 浏览",
		Group:       "YouTube",
	})
	Register(ConfigItem{
		Key:         "youtube.cookie",
		Type:        ConfigTypeText,
		Default:     "",
		Description: "YouTube 请求 Cookie，用于访问需要登录态的视频；不会输出到日志",
		Title:       "YouTube Cookie",
		Group:       "YouTube",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "youtube.poToken",
		Type:        ConfigTypeText,
		Default:     "",
		Description: "YouTube GVS PO Token，兼容 yt-dlp 的 client.gvs+TOKEN 格式；用于避免部分 videoplayback 403",
		Title:       "YouTube PO Token",
		Group:       "YouTube",
		HotReload:   true,
	})
	Register(ConfigItem{
		Key:         "weibo.enabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否记录微博页面浏览记录",
		Title:       "记录微博浏览",
		Group:       "Weibo",
	})
	Register(ConfigItem{
		Key:         "weibo.cookie",
		Type:        ConfigTypeText,
		Default:     "",
		Description: "微博请求 Cookie，用于访问需要登录态的微博列表接口；不会输出到日志",
		Title:       "微博 Cookie",
		Group:       "Weibo",
		HotReload:   true,
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
	// Update 自更新配置
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

	// FileHelper 微信文件传输助手配置
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

func LoadCertFiles() *certificate.CertFileAndKeyFile {
	if cert, ok := loadConfiguredCertFiles(); ok {
		return cert
	}
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
