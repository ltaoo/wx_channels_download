package api

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed ui/index.html
var html_home []byte

//go:embed ui/preview.html
var preview_home []byte

//go:embed ui/filehelper.html
var filehelper_home []byte

//go:embed all:ui
var uiFS embed.FS

// UIFS returns the ui directory as an fs.FS rooted at "ui".
// It prefers the on-disk directory for development (hot-reload),
// falling back to the embedded FS for release builds.
func UIFS() (fs.FS, error) {
	dir := "internal/api/ui"
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return os.DirFS(dir), nil
	}
	return fs.Sub(uiFS, "ui")
}

type Assets struct {
	HTMLHome       []byte
	HTMLPreview    []byte
	HTMLFilehelper []byte
}

var files = &Assets{
	HTMLHome:       html_home,
	HTMLPreview:    preview_home,
	HTMLFilehelper: filehelper_home,
}
