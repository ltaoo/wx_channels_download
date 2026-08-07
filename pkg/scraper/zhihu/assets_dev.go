//go:build !embed_frontend_inject

package zhihu

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Non-embedded builds are used by `go run` during development. Serve the
// package-local files directly instead of leaving the platform asset registry
// with a nil filesystem.
func embeddedInjectFS() fs.FS { return os.DirFS(devInjectDir()) }

func devInjectDir() string {
	candidates := []string{filepath.Join("pkg", "scraper", "zhihu", "inject")}
	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(sourceFile), "inject"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}
