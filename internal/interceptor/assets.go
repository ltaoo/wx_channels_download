package interceptor

import (
	"embed"
)

//go:embed inject/lib
var inject_lib_fs embed.FS

//go:embed inject/src
var inject_src_fs embed.FS

//go:embed inject/lib/FileSaver.min.js
var js_file_saver []byte

//go:embed inject/lib/jszip.min.js
var js_zip []byte

//go:embed inject/lib/floating-ui.core.1.7.4.min.js
var js_floating_ui_core []byte

//go:embed inject/lib/floating-ui.dom.1.7.4.min.js
var js_floating_ui_dom []byte

//go:embed inject/lib/mitt.umd.js
var js_mitt []byte

//go:embed inject/lib/timeless/0.26.0/timeless.umd.min.js
var js_timeless []byte

//go:embed inject/lib/timeless/0.26.0/timeless.utils.umd.min.js
var js_timeless_utils []byte

//go:embed inject/lib/timeless/0.26.0/timeless.shadcn.css
var css_timeless_shadcn []byte

//go:embed inject/lib/timeless/0.26.0/timeless.shadcn.umd.min.js
var js_timeless_shadcn []byte

//go:embed inject/lib/timeless/0.26.0/timeless.dom.umd.min.js
var js_timeless_dom []byte

//go:embed inject/lib/timeless/0.26.0/timeless.web.umd.min.js
var js_timeless_web []byte

//go:embed inject/lib/recorder.min.js
var js_recorder []byte

//go:embed inject/lib/axios.min.js
var js_axios []byte

//go:embed inject/lib/getFeedInfo.js
var js_get_feed_info []byte

//go:embed inject/lib/pagespy.min.js
var js_pagespy []byte

//go:embed inject/src/pagespy.js
var js_debug []byte

//go:embed inject/src/error.js
var js_error []byte

//go:embed inject/src/eventbus.js
var js_eventbus []byte

//go:embed inject/src/components.js
var js_components []byte

//go:embed inject/src/utils.js
var js_utils []byte

//go:embed inject/src/downloaderv2.js
var js_downloader []byte

//go:embed inject/src/officialaccount.js
var js_wechat_officialaccount []byte

//go:embed inject/src/home.js
var js_home_page []byte

//go:embed inject/src/feed.js
var js_feed_profile_page []byte

//go:embed inject/src/live.js
var js_live_profile_page []byte

//go:embed inject/src/profile.js
var js_contact_profile_page []byte

type ChannelInjectedFiles struct {
	LibFS                   embed.FS
	SrcFS                   embed.FS
	JSFileSaver             []byte
	JSZip                   []byte
	JSRecorder              []byte
	JSPageSpy               []byte
	JSFloatingUICore        []byte
	JSFloatingUIDOM         []byte
	JSBox                   []byte
	JSMitt                  []byte
	JSAxios                 []byte
	JSGetFeedInfo           []byte
	JSDebug                 []byte
	JSEventBus              []byte
	JSTimeless              []byte
	JSTimelessUtils         []byte
	CSSTimelessShadcn       []byte
	JSTimelessShadcn        []byte
	JSTimelessDOM           []byte
	JSTimelessWeb           []byte
	JSComponents            []byte
	JSDownloader            []byte
	JSUtils                 []byte
	JSError                 []byte
	JSHomePage              []byte
	JSFeedProfilePage       []byte
	JSLiveProfilePage       []byte
	JSContactPage           []byte
	JSWechatOfficialAccount []byte
}

var Assets = &ChannelInjectedFiles{
	LibFS:                   inject_lib_fs,
	SrcFS:                   inject_src_fs,
	JSFileSaver:             js_file_saver,
	JSZip:                   js_zip,
	JSRecorder:              js_recorder,
	JSPageSpy:               js_pagespy,
	JSFloatingUICore:        js_floating_ui_core,
	JSFloatingUIDOM:         js_floating_ui_dom,
	JSMitt:                  js_mitt,
	JSDebug:                 js_debug,
	JSError:                 js_error,
	JSEventBus:              js_eventbus,
	JSAxios:                 js_axios,
	JSGetFeedInfo:           js_get_feed_info,
	JSTimeless:              js_timeless,
	JSTimelessUtils:         js_timeless_utils,
	CSSTimelessShadcn:       css_timeless_shadcn,
	JSTimelessShadcn:        js_timeless_shadcn,
	JSTimelessDOM:           js_timeless_dom,
	JSTimelessWeb:           js_timeless_web,
	JSComponents:            js_components,
	JSUtils:                 js_utils,
	JSDownloader:            js_downloader,
	JSHomePage:              js_home_page,
	JSFeedProfilePage:       js_feed_profile_page,
	JSLiveProfilePage:       js_live_profile_page,
	JSContactPage:           js_contact_profile_page,
	JSWechatOfficialAccount: js_wechat_officialaccount,
}
