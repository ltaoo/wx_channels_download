//go:build embed_frontend_inject

package wxmp

import (
	"embed"
	"io/fs"
)

//go:embed inject
var injectFS embed.FS

func embeddedInjectFS() fs.FS { sub, _ := fs.Sub(injectFS, "inject"); return sub }
