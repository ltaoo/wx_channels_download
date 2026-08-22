//go:build windows

package application

// Windows keeps the running executable locked. Keep the downloaded archive
// and a helper copy of the current executable until graceful shutdown; the
// helper performs the actual replacement after this process exits.
func apply_update_archive_with_velo(update_path string, exe_path string) error {
	return stage_application_update(update_path, exe_path)
}
