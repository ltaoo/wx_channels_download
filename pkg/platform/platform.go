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
