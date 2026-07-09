//go:build !embed_frontend_inject

package frontend

import "io/fs"

func embeddedRootFS() fs.FS   { return nil }
func embeddedLibFS() fs.FS    { return nil }
func embeddedSrcFS() fs.FS    { return nil }
func embeddedInjectFS() fs.FS { return nil }
