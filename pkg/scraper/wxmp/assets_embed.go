//go:build embed_frontend_inject

package wxmp

import (
	"embed"
	"io/fs"
)

//go:embed inject
var injectFS embed.FS

//go:embed ui/manager.html
var manager_html []byte

func embeddedRootFS() fs.FS { return injectFS }
func embeddedInjectFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "inject")
	return sub
}
