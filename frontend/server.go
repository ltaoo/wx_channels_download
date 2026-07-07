package frontend

import (
	"embed"
	"net/http"

	"github.com/ltaoo/velo/frontendserver"
)

//go:embed index.html src public lib
var embeddedFS embed.FS

func NewServer(mode string) http.Handler {
	serverMode := frontendserver.ModeDev
	root := "frontend"
	var embedded embed.FS

	if mode == "release" || mode == "prod" {
		serverMode = frontendserver.ModeProd
		root = "."
		embedded = embeddedFS
	}

	return frontendserver.New(frontendserver.Options{
		Mode:                serverMode,
		Root:                root,
		Embedded:            embedded,
		EntryPage:           "index.html",
		StaticAssetPrefixes: []string{"/public", "/src", "/lib"},
		NoFallbackPrefixes:  []string{"/api", "/ws", "/rss"},
	})
}
