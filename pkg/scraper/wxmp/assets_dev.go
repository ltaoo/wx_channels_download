//go:build !embed_frontend_inject

package wxmp

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

var manager_html []byte

// Non-embedded builds are used by `go run` during development. Serve the
// package-local files directly instead of leaving the platform asset registry
// with a nil filesystem.
func embeddedRootFS() fs.FS   { return os.DirFS(filepath.Dir(devInjectDir())) }
func embeddedInjectFS() fs.FS { return os.DirFS(devInjectDir()) }

func devInjectDir() string {
	candidates := []string{filepath.Join("pkg", "scraper", "wxmp", "inject")}
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
