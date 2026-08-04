package wxmp

import (
	"wx_channel/internal/webassets"
	scraper "wx_channel/pkg/scraper/wxmp"
)

// RegisterStaticAssets delegates to the scraper to register platform-owned assets.
func RegisterStaticAssets(registry *webassets.Registry) error {
	return scraper.RegisterStaticAssets(registry)
}
