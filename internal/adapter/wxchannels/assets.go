package wxchannelsadapter

import (
	"wx_channel/internal/webassets"
	"wx_channel/pkg/scraper/wxchannels"
)

// RegisterStaticAssets delegates to the scraper to register platform-owned assets.
func RegisterStaticAssets(registry *webassets.Registry) error {
	return wxchannels.RegisterStaticAssets(registry)
}
