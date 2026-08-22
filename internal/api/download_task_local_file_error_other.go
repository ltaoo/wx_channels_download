//go:build !windows

package api

func isUnaddressableDownloadTaskLocalFileError(error) bool {
	return false
}
