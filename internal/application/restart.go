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

// RunApplicationUpdateHelperIfRequested handles the private update-helper
// process before normal command/configuration initialization begins.
func RunApplicationUpdateHelperIfRequested() (bool, error) {
	return run_application_update_helper_if_requested()
}

// CleanupApplicationUpdateHelperIfRequested removes a completed helper's
// staging directory from the newly started process.
func CleanupApplicationUpdateHelperIfRequested() error {
	return cleanup_application_update_helper_if_requested()
}

// RestartIfRequested replaces the stopped application when an update or
// configuration change requested a restart. It must be called only after Start
// has returned and released its resources.
func RestartIfRequested() error {
	if started, err := restart_staged_application_update_if_requested(); started || err != nil {
		return err
	}
	_, err := process_restart_manager.ReplaceIfRequested()
	return err
}
