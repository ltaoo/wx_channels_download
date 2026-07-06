//go:build !embed_inject

package interceptor

import "io/fs"

func embeddedRootFS() fs.FS { return nil }
func embeddedLibFS() fs.FS  { return nil }
func embeddedSrcFS() fs.FS  { return nil }
