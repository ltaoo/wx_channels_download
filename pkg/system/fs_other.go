//go:build !windows

package system

func isUnaddressableFileError(error) bool {
	return false
}
