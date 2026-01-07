package system

import (
	"os"
	"syscall"
)

func Signals() []os.Signal {
	return signals()
}

func SetSysProcAttrForDaemon() *syscall.SysProcAttr {
	return set_sys_proc_attr_for_daemon()
}

func TerminateProcess(pid int) error {
	return terminate_process(pid)
}

func IsProcessRunning(pid int) bool {
	return is_process_running(pid)
}
