package config

import "wx_channel/pkg/configapi"

// common_config_declaration owns configuration shared by multiple modules.
// Consumers still declare the namespaces they read with configapi.Declare.
var common_config_declaration = configapi.DeclareModule(
	"common",
	configapi.Item{
		Key:         "pagespy.enabled",
		Type:        configapi.TypeBool,
		Default:     false,
		Description: "是否开启 PageSpy",
		Title:       "启用",
		Group:       "Pagespy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "pagespy.protocol",
		Type:        configapi.TypeSelect,
		Default:     "http",
		Options:     []string{"http", "https"},
		Description: "PageSpy 调试协议",
		Title:       "协议头",
		Group:       "Pagespy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "pagespy.api",
		Type:        configapi.TypeString,
		Default:     "127.0.0.1:6752",
		Description: "PageSpy 调试 API 地址",
		Title:       "API 地址",
		Group:       "Pagespy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "debug.error",
		Type:        configapi.TypeBool,
		Default:     true,
		Description: "是否全局捕获前端错误，出现错误时弹窗展示错误信息",
		Title:       "错误展示",
		Group:       "Debug",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "debug.echolog",
		Type:        configapi.TypeBool,
		Default:     false,
		Description: "是否启用 Echo 代理日志",
		Title:       "Echo 日志",
		Group:       "Debug",
		Reload:      configapi.ReloadHot,
	},
)
