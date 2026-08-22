package api

import (
	"errors"
	"os"
	"testing"
)

func TestIsMissingDownloadTaskLocalFileError(t *testing.T) {
	if !isMissingDownloadTaskLocalFileError(os.ErrNotExist) {
		t.Fatal("os.ErrNotExist should be treated as a missing local file")
	}
	if isMissingDownloadTaskLocalFileError(errors.New("permission denied")) {
		t.Fatal("unrelated filesystem errors must not be treated as a missing local file")
	}
}
