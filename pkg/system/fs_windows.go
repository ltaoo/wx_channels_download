//go:build windows

package system

import (
	"errors"

	"golang.org/x/sys/windows"
)

func isUnaddressableFileError(err error) bool {
	// Paths containing control characters or otherwise invalid Windows names
	// cannot identify an existing filesystem entry, but CreateFile reports
	// ERROR_INVALID_NAME/ERROR_BAD_PATHNAME rather than ERROR_FILE_NOT_FOUND.
	// Treat it as already absent during idempotent cleanup.
	return errors.Is(err, windows.ERROR_INVALID_NAME) ||
		errors.Is(err, windows.ERROR_BAD_PATHNAME)
}
