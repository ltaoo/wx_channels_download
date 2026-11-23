//go:build windows

package services

import (
	"errors"

	"golang.org/x/sys/windows"
)

func is_unaddressable_download_task_local_file_error(err error) bool {
	// Legacy task records can contain control characters or otherwise invalid
	// Windows names. Such a path cannot identify an existing filesystem entry,
	// but CreateFile reports ERROR_INVALID_NAME/ERROR_BAD_PATHNAME rather than
	// ERROR_FILE_NOT_FOUND. Treat it as already absent during idempotent cleanup.
	return errors.Is(err, windows.ERROR_INVALID_NAME) ||
		errors.Is(err, windows.ERROR_BAD_PATHNAME)
}
