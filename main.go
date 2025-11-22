package main

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"wx_channel/cmd"
	"wx_channel/config"
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

var RootCertificateName = "SunnyNet"
var AppVer = "251122"

var (
	modshell32    = syscall.NewLazyDLL("shell32.dll")
	shell_execute = modshell32.NewProc("ShellExecuteW")
)

func isAdmin() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}
func needAdminPermission() bool {
	args := os.Args[1:]
	os_env := runtime.GOOS
	if os_env == "windows" {
		if len(args) == 0 {
			return true
		}
		if strings.Contains(args[0], "--") {
			return true
		}
	}
	return false
}
func requestAdminPermission() bool {
	exe, _ := os.Executable()
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)

	params := ""
	for _, arg := range os.Args[1:] {
		params += arg + " "
	}
	paramPtr, _ := syscall.UTF16PtrFromString(params)

	ret, _, _ := shell_execute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(paramPtr)),
		0,
		1,
	)

	return ret > 32
}

func main() {
	files := &application.BizFiles{
		CertFile:       cert_file,
		PrivateKeyFile: private_key_file,
		JSFileSaver:    js_file_saver,
		JSZip:          js_zip,
		JSPageSpy:      js_pagespy,
		JSDebug:        js_debug,
		JSError:        js_error,
		JSUtils:        js_utils,
		JSMain:         js_main,
		JSLiveMain:     js_live_main,
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("加载配置文件失败 %v", err.Error())
		return
	}
	if cfg.ProxySystem && needAdminPermission() && !isAdmin() {
		if !requestAdminPermission() {
			fmt.Println("启动失败，请右键选择「以管理员身份运行」")
			return
		}
		return
	}
	cmd.Initialize(AppVer, RootCertificateName, files, cfg)
	if err := cmd.Execute(); err != nil {
		fmt.Printf("初始化失败 %v", err.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
}
