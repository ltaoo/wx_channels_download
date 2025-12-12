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

//go:embed inject/lib/weui.umd.js
var js_weui []byte

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

//go:embed inject/src/main.js
var js_main []byte

//go:embed inject/src/live.js
var js_live_main []byte

var Assets = &ChannelInjectedFiles{
	JSFileSaver:      js_file_saver,
	JSZip:            js_zip,
	JSRecorder:       js_recorder,
	JSPageSpy:        js_pagespy,
	JSFloatingUICore: js_floating_ui_core,
	JSFloatingUIDOM:  js_floating_ui_dom,
	JSWeui:           js_weui,
	JSMitt:           js_mitt,
	JSDebug:          js_debug,
	JSError:          js_error,
	JSEventBus:       js_eventbus,
	JSComponents:     js_components,
	JSUtils:          js_utils,
	JSMain:           js_main,
	JSLiveMain:       js_live_main,
}
