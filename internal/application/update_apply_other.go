//go:build !windows

package application

func apply_update_archive_with_velo(update_path string, exe_path string) error {
	return apply_update_archive_now_with_velo(update_path, exe_path)
}
