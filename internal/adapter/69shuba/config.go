package shuba69adapter

import (
	"wx_channel/internal/config"
	"wx_channel/pkg/configapi"
)

// Shuba69PluginConfig implements config.Configurable for 69shuba plugin config.
type Shuba69PluginConfig struct {
	Cookie            string
	Fetcher           string
	CDPEndpoint       string
	CDPTimeout        int
	CDPWait           int
	SandboxAPIBaseURL string
	SandboxID         string
}

func (c *Shuba69PluginConfig) ConfigNamespace() string { return "69shuba" }

func (c *Shuba69PluginConfig) ConfigSchema() []configapi.Item {
	return []configapi.Item{
		{
			Key:         "cookie",
			Type:        configapi.TypeText,
			Default:     "",
			Description: "69书吧请求 Cookie，用于访问 Cloudflare 验证后的页面",
			Title:       "69书吧 Cookie",
			Group:       "69shuba",
			Reload:      configapi.ReloadHot,
		},
		{
			Key:         "fetcher",
			Type:        configapi.TypeSelect,
			Default:     "clawreq",
			Options:     []string{"clawreq", "http", "cdp", "sandbox"},
			Description: "69书吧 HTML 抓取方式，clawreq 使用浏览器指纹 HTTP client，http 使用 Go client，cdp 使用 CDP 服务地址，sandbox 使用 webarchive 沙箱浏览器 API",
			Title:       "69书吧抓取方式",
			Group:       "69shuba",
		},
		{
			Key:         "cdpEndpoint",
			Type:        configapi.TypeString,
			Default:     "http://127.0.0.1:9222",
			Description: "CDP 服务地址，仅 fetcher=cdp 时使用；可以是本机浏览器或容器暴露的 CDP HTTP/WS 地址",
			Title:       "69书吧 CDP 地址",
			Group:       "69shuba",
		},
		{
			Key:         "cdpTimeout",
			Type:        configapi.TypeInt,
			Default:     30,
			Description: "69书吧 CDP 单次页面抓取超时时间（秒）",
			Title:       "69书吧 CDP 超时",
			Group:       "69shuba",
		},
		{
			Key:         "cdpWait",
			Type:        configapi.TypeInt,
			Default:     8,
			Description: "69书吧 CDP 页面加载完成后的额外等待时间（秒），用于等待 Cloudflare 跳转",
			Title:       "69书吧 CDP 等待",
			Group:       "69shuba",
		},
		{
			Key:         "sandboxAPIBaseURL",
			Type:        configapi.TypeString,
			Default:     "http://127.0.0.1:2021/api/v1",
			Description: "webarchive 风格沙箱 API 地址，仅 fetcher=sandbox 时使用",
			Title:       "69书吧沙箱 API",
			Group:       "69shuba",
		},
		{
			Key:         "sandboxID",
			Type:        configapi.TypeString,
			Default:     "",
			Description: "用于抓取 69书吧页面的沙箱 ID，仅 fetcher=sandbox 时使用",
			Title:       "69书吧沙箱 ID",
			Group:       "69shuba",
		},
	}
}

func (c *Shuba69PluginConfig) ApplyConfig(sub *config.ScopedConfig) error {
	c.Cookie = sub.GetString("cookie")
	c.Fetcher = sub.GetString("fetcher")
	c.CDPEndpoint = sub.GetString("cdpEndpoint")
	c.CDPTimeout = sub.GetInt("cdpTimeout")
	c.CDPWait = sub.GetInt("cdpWait")
	c.SandboxAPIBaseURL = sub.GetString("sandboxAPIBaseURL")
	c.SandboxID = sub.GetString("sandboxID")
	return nil
}

// GetShuba69Config returns the registered 69shuba plugin config if available.
func GetShuba69Config() *Shuba69PluginConfig {
	return shuba69PluginConfig
}

var shuba69PluginConfig *Shuba69PluginConfig

func init() {
	shuba69PluginConfig = &Shuba69PluginConfig{}
	config.RegisterPlugin(shuba69PluginConfig)
}
