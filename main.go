package main

import (
	_ "embed"
	"fmt"

	"wx_channel/cmd"
	"wx_channel/internal/application"
)

//go:embed certs/SunnyRoot.cer
var cert_file []byte

//go:embed certs/private.key
var private_key_file []byte

//go:embed lib/FileSaver.min.js
var js_file_saver []byte

//go:embed lib/jszip.min.js
var js_zip []byte

//go:embed inject/pagespy.min.js
var js_pagespy1 []byte

//go:embed inject/pagespy.js
var js_pagespy2 []byte

//go:embed inject/error.js
var js_error []byte

//go:embed inject/main.js
var js_main []byte

var RootCertificateName = "SunnyNet"
var AppVer = "250913"

func main() {
	files := &application.BizFiles{
		CertFile:       cert_file,
		PrivateKeyFile: private_key_file,
		JSFileSaver:    js_file_saver,
		JSZip:          js_zip,
		JSPageSpy1:     js_pagespy1,
		JSPageSpy2:     js_pagespy2,
		JSMain:         js_main,
		JSError:        js_error,
	}
	cmd.Initialize(AppVer, RootCertificateName, files)
	if err := cmd.Execute(); err != nil {
		fmt.Printf("初始化失败 %v", err.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
}
