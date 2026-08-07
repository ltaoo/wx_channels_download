package wxmpadapter

import (
	"wx_channel/internal/webassets"
	"wx_channel/pkg/scraper/wxmp"
)

// RegisterStaticAssets delegates to the scraper to register platform-owned assets.
func RegisterStaticAssets(registry *webassets.Registry) error {
	return wxmp.RegisterStaticAssets(registry)
}
