package wxmp

import (
	"embed"
	"io/fs"
	"strings"
)

var Assets = NewAssets()

type InjectedAssets struct{ InjectFS fs.FS }

func NewAssets() *InjectedAssets { return &InjectedAssets{InjectFS: embeddedInjectFS()} }

func (a *InjectedAssets) ReadInject(name string) ([]byte, error) {
	return fs.ReadFile(a.InjectFS, name)
}

func ChannelInjectAssetURL(baseURL, name string) string {
	return strings.TrimRight(baseURL, "/") + "/inject/" + strings.TrimLeft(name, "/")
}

//go:embed ui/manager.html
var manager_html []byte
