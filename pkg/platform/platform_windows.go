//go:build windows

package platform

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modshell32    = syscall.NewLazyDLL("shell32.dll")
	shell_execute = modshell32.NewProc("ShellExecuteW")
)

func is_admin() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}
func need_admin_permission() bool {
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
func request_admin_permission() bool {
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
