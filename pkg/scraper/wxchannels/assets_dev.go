//go:build !embed_frontend_inject

package wxchannels

import "io/fs"

func embeddedRootFS() fs.FS   { return nil }
func embeddedInjectFS() fs.FS { return nil }
