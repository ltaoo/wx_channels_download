// Package builtin links the adapters shipped with the application.
//
// Application code imports only this package for side effects. Concrete
// adapters register themselves and are subsequently accessed through registry.Get.
package builtinadapter

import (
	_ "wx_channel/internal/adapter/69shuba"
	_ "wx_channel/internal/adapter/bilibili"
	_ "wx_channel/internal/adapter/douyin"
	_ "wx_channel/internal/adapter/fanqienovel"
	_ "wx_channel/internal/adapter/weibo"
	_ "wx_channel/internal/adapter/wxchannels"
	_ "wx_channel/internal/adapter/wxmp"
	_ "wx_channel/internal/adapter/xiaohongshu"
	_ "wx_channel/internal/adapter/youtube"
	_ "wx_channel/internal/adapter/zhihu"
)
