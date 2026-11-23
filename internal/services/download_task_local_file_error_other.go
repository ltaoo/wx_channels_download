//go:build !windows

package services

func is_unaddressable_download_task_local_file_error(error) bool {
	return false
}
