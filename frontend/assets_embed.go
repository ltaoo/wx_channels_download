//go:build embed_frontend_inject

package frontend

import (
	"embed"
	"io/fs"
)

//go:embed lib src inject public *.html
var injectFS embed.FS

func embeddedRootFS() fs.FS {
	return injectFS
}

func embeddedLibFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "lib")
	return sub
}

func embeddedSrcFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "src")
	return sub
}

func embeddedInjectFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "inject")
	return sub
}

func embeddedPublicFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "public")
	return sub
}
