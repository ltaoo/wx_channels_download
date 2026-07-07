package xiaohongshu

import (
	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
)

func CreatePlugin(onLoaded func(profile *Profile)) *proxy.Plugin {
	return interceptor.CreatePlatformBrowserPluginWithScript(Match, Script(), func(profile *interceptor.PlatformBrowserProfile) {
		if onLoaded != nil {
			onLoaded(FormatProfile(profile))
		}
	})
}
