package main

import (
	_ "embed"
	"fmt"

	"wx_channel/cmd"
	"wx_channel/config"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/platform"
)

//go:embed certs/SunnyRoot.cer
var cert_file []byte

//go:embed certs/private.key
var private_key_file []byte

//go:embed inject/lib/FileSaver.min.js
var js_file_saver []byte

//go:embed inject/lib/jszip.min.js
var js_zip []byte

//go:embed inject/lib/recorder.min.js
var js_recorder []byte

//go:embed inject/pagespy.min.js
var js_pagespy []byte

//go:embed inject/pagespy.js
var js_debug []byte

//go:embed inject/error.js
var js_error []byte

//go:embed inject/utils.js
var js_utils []byte

//go:embed inject/main.js
var js_main []byte

//go:embed inject/live.js
var js_live_main []byte

var FilesCert = &interceptor.ServerCertFiles{
	CertFile:       cert_file,
	PrivateKeyFile: private_key_file,
}
var FilesChannelScript = &interceptor.ChannelInjectedFiles{
	JSFileSaver: js_file_saver,
	JSZip:       js_zip,
	JSRecorder:  js_recorder,
	JSPageSpy:   js_pagespy,
	JSDebug:     js_debug,
	JSError:     js_error,
	JSUtils:     js_utils,
	JSMain:      js_main,
	JSLiveMain:  js_live_main,
}

var RootCertificateName = "SunnyNet"
var AppVer = "251202"

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("加载配置文件失败 %v", err.Error())
		return
	}
	if cfg.ProxySystem && platform.NeedAdminPermission() && !platform.IsAdmin() {
		if !platform.RequestAdminPermission() {
			fmt.Println("启动失败，请右键选择「以管理员身份运行」")
			return
		}
		return
	}
	if err := cmd.Execute(AppVer, RootCertificateName, FilesChannelScript, FilesCert, cfg); err != nil {
		fmt.Printf("初始化失败 %v\n", err.Error())
	}
}
