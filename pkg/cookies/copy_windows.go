//go:build windows

package cookies

import (
	"fmt"
	"syscall"
	"unsafe"
)

var procCopyFileW = kernel32.NewProc("CopyFileW")

func copyFile(src, dst string) error {
	srcPtr, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	dstPtr, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}

	ret, _, callErr := procCopyFileW.Call(
		uintptr(unsafe.Pointer(srcPtr)),
		uintptr(unsafe.Pointer(dstPtr)),
		0,
	)
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("CopyFileW failed")
	}
	return nil
}
