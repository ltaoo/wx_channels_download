package interceptor

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed inject/lib
var libFS embed.FS

//go:embed inject/src
var srcFS embed.FS

// LibFS returns the embedded lib directory as an fs.FS rooted at "inject/lib".
func LibFS() (fs.FS, error) {
	return fs.Sub(libFS, "inject/lib")
}

// SrcFS returns the embedded src directory as an fs.FS rooted at "inject/src".
func SrcFS() (fs.FS, error) {
	return fs.Sub(srcFS, "inject/src")
}

// AssetDir is a helper that binds base URL, version, and optional extra attributes
// for generating script/link tags.
type AssetDir struct {
	base    string
	version string
	attr    string // extra attributes appended to <script> tags, e.g. ` nonce="xxx"`
}

func NewAssetDir(base, version string) *AssetDir {
	return &AssetDir{base: base, version: version}
}

func NewAssetDirWithAttr(base, version, attr string) *AssetDir {
	return &AssetDir{base: base, version: version, attr: attr}
}

// Tag generates a <script src> or <link rel="stylesheet" href> tag.
// path is like "lib/mitt.umd.js" or "src/utils.js".
func (d *AssetDir) Tag(path string) string {
	url := d.base + "/" + path + d.version
	if strings.HasSuffix(path, ".css") {
		return `<link rel="stylesheet" href="` + url + `">`
	}
	return `<script` + d.attr + ` src="` + url + `"></script>`
}

// Inline generates a <script>content</script> or <style>content</style> tag.
func (d *AssetDir) Inline(content string, isCSS bool) string {
	if isCSS {
		return `<style>` + content + `</style>`
	}
	return `<script` + d.attr + `>` + content + `</script>`
}

//go:embed inject/lib/mitt.umd.js
var JSMitt []byte

//go:embed inject/lib/weui.min.css
var CSSWeui []byte

//go:embed inject/lib/weui.min.js
var JSWeui []byte

//go:embed inject/lib/wui.umd.js
var JSWui []byte

//go:embed inject/lib/floating-ui.core.1.7.4.min.js
var JSFloatingUICore []byte

//go:embed inject/lib/floating-ui.dom.1.7.4.min.js
var JSFloatingUIDOM []byte

//go:embed inject/lib/pagespy.min.js
var JSPageSpy []byte

//go:embed inject/src/pagespy.js
var JSDebug []byte

//go:embed inject/src/error.js
var JSError []byte

//go:embed inject/src/eventbus.js
var JSEventBus []byte

//go:embed inject/src/components.js
var JSComponents []byte

//go:embed inject/src/utils.js
var JSUtils []byte

//go:embed inject/src/downloaderv2.js
var JSDownloader []byte

//go:embed inject/src/officialaccount.js
var JSWechatOfficialAccount []byte

//go:embed inject/src/home.js
var JSHomePage []byte

//go:embed inject/src/feed.js
var JSFeedProfilePage []byte

//go:embed inject/src/live.js
var JSLiveProfilePage []byte

//go:embed inject/src/profile.js
var JSContactPage []byte
