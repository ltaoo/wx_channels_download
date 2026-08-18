package application

// Import the adapters shipped with the application for their registration
// side effects. Concrete adapters register themselves with adapter.Register.
import (
	// _ "wx_channel/internal/adapter/bilibili"
	// _ "wx_channel/internal/adapter/cctv"
	_ "wx_channel/internal/adapter/douyin"
	_ "wx_channel/internal/adapter/webpage"
	_ "wx_channel/internal/adapter/wxchannels"
	_ "wx_channel/internal/adapter/wxmp"
	// _ "wx_channel/internal/adapter/youtube"
	_ "wx_channel/internal/adapter/zhihu"
)
