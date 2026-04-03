package interceptor

import (
	_ "embed"
)

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

//go:embed inject/lib/weui.min.css
var css_weui []byte

//go:embed inject/lib/weui.min.js
var js_weui []byte

//go:embed inject/lib/wui.umd.js
var js_wui []byte

//go:embed inject/lib/timeless.reactive.umd.min.js
var js_timeless_reactive []byte

//go:embed inject/lib/timeless.headless.umd.min.js
var js_timeless_headless []byte

//go:embed inject/lib/timeless.utils.umd.min.js
var js_timeless_utils []byte

//go:embed inject/lib/timeless.kit.umd.min.js
var js_timeless_kit []byte

//go:embed inject/lib/timeless.ui.umd.min.js
var js_timeless_ui []byte

//go:embed inject/lib/timeless.icons.umd.min.js
var js_timeless_icons []byte

//go:embed inject/lib/timeless.web.umd.min.js
var js_timeless_web []byte

//go:embed inject/lib/recorder.min.js
var js_recorder []byte

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

//go:embed inject/src/base64.js
var js_base64 []byte

//go:embed inject/src/router.js
var js_router []byte

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
	JSFileSaver             []byte
	JSZip                   []byte
	JSRecorder              []byte
	JSPageSpy               []byte
	JSFloatingUICore        []byte
	JSFloatingUIDOM         []byte
	JSWeui                  []byte
	CSSWeui                 []byte
	JSWui                   []byte
	JSBox                   []byte
	JSMitt                  []byte
	JSDebug                 []byte
	JSEventBus              []byte
	JSTimelessReactive      []byte
	JSTimelessHeadless      []byte
	JSTimelessUtils         []byte
	JSTimelessKit           []byte
	JSTimelessIcons         []byte
	JSTimelessUI            []byte
	JSTimelessProviderWeb   []byte
	JSComponents            []byte
	JSDownloader            []byte
	JSUtils                 []byte
	JSBase64                []byte
	JSRouter                []byte
	JSError                 []byte
	JSHomePage              []byte
	JSFeedProfilePage       []byte
	JSLiveProfilePage       []byte
	JSContactPage           []byte
	JSWechatOfficialAccount []byte
}

var Assets = &ChannelInjectedFiles{
	JSFileSaver:             js_file_saver,
	JSZip:                   js_zip,
	JSRecorder:              js_recorder,
	JSPageSpy:               js_pagespy,
	JSFloatingUICore:        js_floating_ui_core,
	JSFloatingUIDOM:         js_floating_ui_dom,
	JSWeui:                  js_weui,
	CSSWeui:                 css_weui,
	JSWui:                   js_wui,
	JSMitt:                  js_mitt,
	JSDebug:                 js_debug,
	JSError:                 js_error,
	JSEventBus:              js_eventbus,
	JSTimelessReactive:      js_timeless_reactive,
	JSTimelessHeadless:      js_timeless_headless,
	JSTimelessUtils:         js_timeless_utils,
	JSTimelessIcons:         js_timeless_icons,
	JSTimelessKit:           js_timeless_kit,
	JSTimelessUI:            js_timeless_ui,
	JSTimelessProviderWeb:   js_timeless_web,
	JSComponents:            js_components,
	JSUtils:                 js_utils,
	JSBase64:                js_base64,
	JSRouter:                js_router,
	JSDownloader:            js_downloader,
	JSHomePage:              js_home_page,
	JSFeedProfilePage:       js_feed_profile_page,
	JSLiveProfilePage:       js_live_profile_page,
	JSContactPage:           js_contact_profile_page,
	JSWechatOfficialAccount: js_wechat_officialaccount,
}
