package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// IsMissingFileError reports errors which prove that no filesystem entry can
// exist at the path. Cleanup uses this instead of os.IsNotExist directly
// because some platforms return a distinct error for paths which cannot name
// a filesystem entry.
func IsMissingFileError(err error) bool {
	return os.IsNotExist(err) || isUnaddressableFileError(err)
}

// ShowInExplorer opens the file explorer and highlights the specified file
func ShowInExplorer(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,", path)
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "linux":
		open_path := path
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			open_path = filepath.Dir(path)
		}
		cmd = exec.Command("xdg-open", open_path)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	return cmd.Start()
}
