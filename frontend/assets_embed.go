//go:build embed_inject || embed_frontend_inject

package frontend

import (
	"embed"
	"io/fs"
)

//go:embed inject public *.html
var injectFS embed.FS

func embeddedRootFS() fs.FS {
	return injectFS
}

func embeddedSrcFS() fs.FS {
	return emptyFS{}
}

func embeddedInjectFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "inject")
	return sub
}

func embeddedPublicFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "public")
	return sub
}

type emptyFS struct{}

func (emptyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}
