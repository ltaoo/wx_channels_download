package platform

func IsAdmin() bool {
	return is_admin()
}

func NeedAdminPermission() bool {
	return need_admin_permission()
}

func RequestAdminPermission() bool {
	return request_admin_permission()
}

// RequestAdminPermissionAndWait starts an elevated copy of the current
// process and waits for it to exit. started is false when elevation was not
// accepted; exited is true only when the child process was observed exiting.
func RequestAdminPermissionAndWait() (started bool, exited bool) {
	return request_admin_permission_and_wait()
}
