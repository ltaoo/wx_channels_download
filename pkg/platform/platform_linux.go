//go:build linux

package platform

func is_admin() bool {
	return false
}

func need_admin_permission() bool {
	return false
}

func request_admin_permission() bool {
	return false
}

