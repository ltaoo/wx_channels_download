package weibo

import "wx_channel/internal/interceptor"

func Script() string {
	return interceptor.BuildPlatformBrowserScript(Config())
}
