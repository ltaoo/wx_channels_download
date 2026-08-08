//go:build !embed_inject && !embed_frontend_inject

package frontend

import "io/fs"

func embeddedRootFS() fs.FS   { return nil }
func embeddedSrcFS() fs.FS    { return nil }
func embeddedInjectFS() fs.FS { return nil }
func embeddedPublicFS() fs.FS { return nil }
