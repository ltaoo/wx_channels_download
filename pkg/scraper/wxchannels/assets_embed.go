//go:build embed_frontend_inject

package wxchannels

import (
	"embed"
	"io/fs"
)

//go:embed inject
var injectFS embed.FS

func embeddedRootFS() fs.FS { return injectFS }
func embeddedInjectFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "inject")
	return sub
}
