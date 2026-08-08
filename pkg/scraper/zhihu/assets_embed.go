//go:build embed_inject || embed_frontend_inject

package zhihu

import (
	"embed"
	"io/fs"
)

//go:embed inject
var injectFS embed.FS

func embeddedInjectFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "inject")
	return sub
}
