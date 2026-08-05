package wxmp

import (
	"encoding/json"

	"wx_channel/internal/adapter"
)

var wechatHeaders string

func init() {
	adapter.Register(&handler{})
	h := map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN",
		"Referer":    "https://mp.weixin.qq.com/",
	}
	b, _ := json.Marshal(h)
	wechatHeaders = string(b)
}

type handler struct{}

func (h *handler) PlatformID() string { return PlatformID }
