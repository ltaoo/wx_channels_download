//go:build windows

package platform

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modshell32       = windows.NewLazySystemDLL("shell32.dll")
	modkernel32      = windows.NewLazySystemDLL("kernel32.dll")
	shell_execute_ex = modshell32.NewProc("ShellExecuteExW")
	free_console     = modkernel32.NewProc("FreeConsole")
)

const see_mask_nocloseprocess = 0x00000040

type shell_execute_info struct {
	cb_size       uint32
	f_mask        uint32
	hwnd          uintptr
	lp_verb       *uint16
	lp_file       *uint16
	lp_parameters *uint16
	lp_directory  *uint16
	n_show        int32
	h_inst_app    uintptr
	lp_id_list    uintptr
	lp_class      *uint16
	hkey_class    uintptr
	dw_hot_key    uint32
	h_icon        uintptr
	h_process     windows.Handle
}

func is_admin() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}
func need_admin_permission() bool {
	args := os.Args[1:]
	if len(args) == 0 {
		return true
	}
	return false
}
func request_admin_permission() bool {
	process, started := start_admin_process()
	if process != 0 {
		_ = windows.CloseHandle(process)
	}
	return started
}

func request_admin_permission_and_wait() (started bool, exited bool) {
	process, started := start_admin_process()
	if !started {
		return false, false
	}
	defer windows.CloseHandle(process)

	// The original process is the cleanup guardian. Detaching it prevents a
	// close event for the elevated console from terminating both processes.
	_, _, _ = free_console.Call()

	event, err := windows.WaitForSingleObject(process, windows.INFINITE)
	return true, err == nil && event == windows.WAIT_OBJECT_0
}

func start_admin_process() (windows.Handle, bool) {
	exe, err := os.Executable()
	if err != nil {
		return 0, false
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return 0, false
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, false
	}
	param_ptr, err := windows.UTF16PtrFromString(windows_command_line(os.Args[1:]))
	if err != nil {
		return 0, false
	}

	var directory_ptr *uint16
	if directory, getwd_err := os.Getwd(); getwd_err == nil {
		directory_ptr, _ = windows.UTF16PtrFromString(directory)
	}
	info := shell_execute_info{
		f_mask:        see_mask_nocloseprocess,
		lp_verb:       verb,
		lp_file:       file,
		lp_parameters: param_ptr,
		lp_directory:  directory_ptr,
		n_show:        windows.SW_SHOWNORMAL,
	}
	info.cb_size = uint32(unsafe.Sizeof(info))

	result, _, _ := shell_execute_ex.Call(uintptr(unsafe.Pointer(&info)))
	if result == 0 || info.h_process == 0 {
		return 0, false
	}
	return info.h_process, true
}

func windows_command_line(args []string) string {
	escaped := make([]string, 0, len(args))
	for _, arg := range args {
		escaped = append(escaped, syscall.EscapeArg(arg))
	}
	return strings.Join(escaped, " ")
}
