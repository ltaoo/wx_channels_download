package application

// Import the adapters enabled in this application for their registration side
// effects. Concrete adapters register themselves with adapter.Register, and
// every adapter listed here is initialized without a separate config gate.
import (
	// _ "wx_channel/internal/adapter/bilibili"
	// _ "wx_channel/internal/adapter/cctv"
	_ "wx_channel/internal/adapter/douyin"
	_ "wx_channel/internal/adapter/feishu"
	_ "wx_channel/internal/adapter/kuaishou"
	// _ "wx_channel/internal/adapter/ucdrive"
	_ "wx_channel/internal/adapter/webpage"
	// _ "wx_channel/internal/adapter/weibo"
	_ "wx_channel/internal/adapter/wxchannels"
	_ "wx_channel/internal/adapter/wxmp"
	_ "wx_channel/internal/adapter/x"
	_ "wx_channel/internal/adapter/xiaohongshu"
	// _ "wx_channel/internal/adapter/youtube"
	_ "wx_channel/internal/adapter/zhihu"
)
