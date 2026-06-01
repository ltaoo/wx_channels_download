package frontend

import (
	_ "embed"
)

//go:embed lib/FileSaver.min.js
var js_file_saver []byte

//go:embed lib/jszip.min.js
var js_zip []byte

//go:embed lib/floating-ui.core.1.7.4.min.js
var js_floating_ui_core []byte

//go:embed lib/floating-ui.dom.1.7.4.min.js
var js_floating_ui_dom []byte

//go:embed lib/mitt.umd.js
var js_mitt []byte

//go:embed lib/weui.min.css
var css_weui []byte

//go:embed lib/weui.min.js
var js_weui []byte

//go:embed lib/wui.umd.js
var js_wui []byte

//go:embed lib/timeless.reactive.umd.min.js
var js_timeless_reactive []byte

//go:embed lib/timeless.headless.umd.min.js
var js_timeless_headless []byte

//go:embed lib/timeless.utils.umd.min.js
var js_timeless_utils []byte

//go:embed lib/timeless.kit.umd.min.js
var js_timeless_kit []byte

//go:embed lib/timeless.ui.umd.min.js
var js_timeless_ui []byte

//go:embed lib/timeless.icons.umd.min.js
var js_timeless_icons []byte

//go:embed lib/timeless.web.umd.min.js
var js_timeless_web []byte

//go:embed lib/recorder.min.js
var js_recorder []byte

//go:embed lib/axios.min.js
var js_axios []byte

//go:embed lib/getFeedInfo.js
var js_get_feed_info []byte

//go:embed lib/pagespy.min.js
var js_pagespy []byte

//go:embed src/pagespy.js
var js_debug []byte

//go:embed src/error.js
var js_error []byte

//go:embed src/eventbus.js
var js_eventbus []byte

//go:embed src/components.js
var js_components []byte

//go:embed src/utils.js
var js_utils []byte

//go:embed src/base64.js
var js_base64 []byte

//go:embed src/router.js
var js_router []byte

//go:embed src/downloaderv2.js
var js_downloader []byte

//go:embed src/officialaccount.js
var js_wechat_officialaccount []byte

//go:embed src/home.js
var js_home_page []byte

//go:embed src/feed.js
var js_feed_profile_page []byte

//go:embed src/live.js
var js_live_profile_page []byte

//go:embed src/profile.js
var js_contact_profile_page []byte

//go:embed public/tailwindcss@4.js
var js_tailwindcss []byte

//go:embed public/avatar.jpeg
var avatar_jpeg []byte

//go:embed public/timeless/0.26.0/timeless.umd.min.js
var js_public_timeless_umd []byte

//go:embed public/timeless/0.26.0/timeless.dom.umd.min.js
var js_public_timeless_dom []byte

//go:embed public/timeless/0.26.0/timeless.shadcn.umd.min.js
var js_public_timeless_shadcn []byte

//go:embed public/timeless/0.26.0/timeless.shadcn.css
var css_public_timeless_shadcn []byte

//go:embed public/timeless/0.26.0/timeless.web.umd.min.js
var js_public_timeless_web []byte

//go:embed public/timeless/0.26.0/timeless.utils.umd.min.js
var js_public_timeless_utils []byte

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
	JSAxios                 []byte
	JSGetFeedInfo           []byte
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
	JSTailwindcss           []byte
	Avatar                  []byte
	JSPublicTimelessUMD     []byte
	JSPublicTimelessDOM     []byte
	JSPublicTimelessShadcn  []byte
	CSSPublicTimelessShadcn []byte
	JSPublicTimelessWeb     []byte
	JSPublicTimelessUtils   []byte
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
	JSAxios:                 js_axios,
	JSGetFeedInfo:           js_get_feed_info,
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
	JSTailwindcss:           js_tailwindcss,
	Avatar:                  avatar_jpeg,
	JSPublicTimelessUMD:     js_public_timeless_umd,
	JSPublicTimelessDOM:     js_public_timeless_dom,
	JSPublicTimelessShadcn:  js_public_timeless_shadcn,
	CSSPublicTimelessShadcn: css_public_timeless_shadcn,
	JSPublicTimelessWeb:     js_public_timeless_web,
	JSPublicTimelessUtils:   js_public_timeless_utils,
}
