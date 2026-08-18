package application

import (
	"github.com/ltaoo/velo/updater/restart"
)

type process_restart_manager_interface interface {
	RequestCurrent(request_shutdown func()) error
	ReplaceIfRequested() (bool, error)
}

var process_restart_manager process_restart_manager_interface = restart.NewManager()

func restart_current_process(request_shutdown func()) error {
	return process_restart_manager.RequestCurrent(request_shutdown)
}

// RestartIfRequested replaces the stopped application when an update or
// configuration change requested a restart. It must be called only after Start
// has returned and released its resources.
func RestartIfRequested() error {
	_, err := process_restart_manager.ReplaceIfRequested()
	return err
}
