//go:build !embed_frontend_inject

package wxmp

import "io/fs"

func embeddedInjectFS() fs.FS { return nil }
