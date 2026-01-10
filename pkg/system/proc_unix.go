//go:build unix || darwin || linux

package system

import (
	"os"
	"syscall"
)

func signals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}

func set_sys_proc_attr_for_daemon() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

func terminate_process(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

func is_process_running(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, syscall.Signal(0)) == nil
}
