//go:build windows

package application

import (
	"fmt"

	"golang.org/x/sys/windows"
)

var set_console_ctrl_handler = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler")

func registerConsoleCloseHandler(request_shutdown func()) (func(), error) {
	if err := set_console_ctrl_handler.Find(); err != nil {
		return nil, err
	}

	shutdown_complete := make(chan struct{})
	handler := windows.NewCallback(newConsoleControlHandler(request_shutdown, shutdown_complete))
	result, _, call_err := set_console_ctrl_handler.Call(handler, 1)
	if result == 0 {
		return nil, fmt.Errorf("SetConsoleCtrlHandler failed: %w", call_err)
	}

	return func() {
		close(shutdown_complete)
		_, _, _ = set_console_ctrl_handler.Call(handler, 0)
	}, nil
}

func newConsoleControlHandler(request_shutdown func(), shutdown_complete <-chan struct{}) func(uint32) uintptr {
	return func(control_type uint32) uintptr {
		switch control_type {
		case windows.CTRL_CLOSE_EVENT, windows.CTRL_LOGOFF_EVENT, windows.CTRL_SHUTDOWN_EVENT:
			// Do not run service shutdown or child processes on Windows' console
			// control thread. Wake the main goroutine, then keep this handler alive
			// until Start has completed cleanup and unregisters it.
			request_shutdown()
			<-shutdown_complete
			return 1
		default:
			// Let Go's os/signal handler process Ctrl+C and Ctrl+Break.
			return 0
		}
	}
}
