package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	RootDir  string // 二进制文件所在目录
	Filename string // 配置文件名
	FullPath string // 配置文件完整路径
	Existing bool   // 配置文件是否存在
	Error    error
	Debug    bool
	Version  string
}

func New(ver string, mode string) *Config {
	exe, _ := os.Executable()
	exe_dir := filepath.Dir(exe)
	base_dir := exe_dir
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
	var config_filepath string
	var has_config bool
	for _, dir := range candidates {
		p := filepath.Join(dir, "config.yaml")
		if _, err := os.Stat(p); err == nil {
			base_dir = dir
			config_filepath = p
			has_config = true
			break
		}
	}
	filename := "config.yaml"
	if config_filepath == "" {
		config_filepath = filepath.Join(base_dir, filename)
	}
	viper.SetConfigFile(config_filepath)
	c := &Config{
		RootDir:  base_dir,
		Filename: filename,
		FullPath: config_filepath,
		Existing: has_config,
		Version:  ver,
	}
	return c
}

func (c *Config) LoadConfig() error {
	Register(ConfigItem{
		Key:         "proxy.system",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "是否设置系统代理为代理服务",
		Title:       "设置系统代理",
		Group:       "Proxy",
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
		Default:     2080,
		Description: "代理服务的端口",
		Title:       "代理端口",
		Group:       "Proxy",
	})
	Register(ConfigItem{
		Key:         "pagespy.enabled",
		Type:        ConfigTypeSelect,
		Default:     false,
		Description: "是否开启 PageSpy",
		Title:       "启用",
		Group:       "Pagespy",
	})
	Register(ConfigItem{
		Key:         "pagespy.protocol",
		Type:        ConfigTypeSelect,
		Default:     "https",
		Options:     []string{"http", "https"},
		Description: "PageSpy 调试协议",
		Title:       "协议头",
		Group:       "Pagespy",
	})
	Register(ConfigItem{
		Key:         "pagespy.api",
		Type:        ConfigTypeString,
		Default:     "debug.weixin.qq.com",
		Description: "PageSpy 调试 API 地址",
		Title:       "API 地址",
		Group:       "Pagespy",
	})
	Register(ConfigItem{
		Key:         "debug.error",
		Type:        ConfigTypeBool,
		Default:     true,
		Description: "是否全局捕获前端错误，出现错误时弹窗展示错误信息",
		Title:       "错误展示",
		Group:       "Debug",
	})
	Register(ConfigItem{
		Key:         "channels.disableLocationToHome",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否禁止从视频号详情页重定向到首页（视频号默认行为）",
		Title:       "禁止重定向",
		Group:       "Channels",
	})
	Register(ConfigItem{
		Key:         "channel.disableLocationToHome",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否禁止从视频号详情页重定向到首页（视频号默认行为）",
		Title:       "禁止重定向",
		Group:       "Channels",
	})
	Register(ConfigItem{
		Key:         "inject.extraScript.afterJSMain",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "额外注入的 JS 脚本路径",
		Title:       "注入脚本",
		Group:       "Inject",
	})
	Register(ConfigItem{
		Key:         "inject.globalScript",
		Type:        ConfigTypeString,
		Default:     "global.js",
		Description: "全局用户脚本",
		Title:       "全局脚本",
		Group:       "Inject",
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
		Key:         "download.inFrontend",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否通过前端解密、下载，不调用后台下载能力",
		Title:       "前端下载",
		Group:       "Download",
	})
	Register(ConfigItem{
		Key:         "download.defaultHighest",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "点击下载图标时是否下载原始视频",
		Title:       "原始视频",
		Group:       "Download",
	})
	Register(ConfigItem{
		Key:         "download.filenameTemplate",
		Type:        ConfigTypeString,
		Default:     "{{filename}}_{{spec}}",
		Description: "用于配置下载文件的名称，支持 {{filename}} 和 {{spec}} 等变量",
		Title:       "文件名模板",
		Group:       "Download",
	})
	Register(ConfigItem{
		Key:         "download.pauseWhenDownload",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "点击下载时是否暂停播放",
		Title:       "暂停播放",
		Group:       "Download",
	})
	Register(ConfigItem{
		Key:         "api.protocol",
		Type:        ConfigTypeString,
		Default:     "http",
		Description: "指定 API 服务的协议头",
		Title:       "API 服务协议",
		Group:       "API",
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
		Key:         "mp.disabled",
		Type:        ConfigTypeBool,
		Default:     false,
		Description: "是否禁用公众号本地服务，本地服务会提供接口、RSS 等功能",
		Title:       "启用本地服务",
		Group:       "OfficialAccount",
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
		Default:     "127.0.0.1",
		Description: "公众号远端服务主机名",
		Title:       "服务主机名",
		Group:       "OfficialAccount",
	})
	Register(ConfigItem{
		Key:         "mp.remoteServer.port",
		Type:        ConfigTypeInt,
		Default:     2022,
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
	})
	Register(ConfigItem{
		Key:         "mp.tokenFilepath",
		Type:        ConfigTypeString,
		Default:     "",
		Description: "公众号远端服务授权凭证",
		Title:       "授权凭证",
		Group:       "OfficialAccount",
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
