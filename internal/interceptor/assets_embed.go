//go:build embed_inject

package interceptor

import (
	"embed"
	"io/fs"
)

//go:embed inject/lib inject/src inject/*.html
var injectFS embed.FS

func embeddedRootFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "inject")
	return sub
}

func embeddedLibFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "inject/lib")
	return sub
}

func embeddedSrcFS() fs.FS {
	sub, _ := fs.Sub(injectFS, "inject/src")
	return sub
}
