//go:build !windows

package application

func run_application_update_helper_if_requested() (bool, error) {
	return false, nil
}

func cleanup_application_update_helper_if_requested() error {
	return nil
}

func restart_staged_application_update_if_requested() (bool, error) {
	return false, nil
}
