package interceptor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"wx_channel/internal/interceptor/proxy"
)

const defaultChannelInjectDir = "internal/interceptor/inject"

type ChannelInjectedFiles struct {
	InjectDir               string
	RootFS                  fs.FS
	LibFS                   fs.FS
	SrcFS                   fs.FS
	JSFileSaver             []byte
	JSZip                   []byte
	JSRecorder              []byte
	JSPageSpy               []byte
	JSBox                   []byte
	JSMitt                  []byte
	JSAxios                 []byte
	JSGetFeedInfo           []byte
	JSDebug                 []byte
	JSEventBus              []byte
	JSEnv                   []byte
	JSEnvChannels           []byte
	JSEnvMock               []byte
	JSTimeless              []byte
	JSTimelessUtils         []byte
	CSSTimelessShadcn       []byte
	JSTimelessShadcn        []byte
	JSTimelessDOM           []byte
	JSTimelessWeb           []byte
	CSSComponents           []byte
	JSComponents            []byte
	JSChannels              []byte
	JSDownloadCore          []byte
	JSDownloadPanel         []byte
	JSDownloadIndex         []byte
	JSDownloader            []byte
	JSUtils                 []byte
	JSError                 []byte
	JSHomePage              []byte
	JSFeedProfilePage       []byte
	JSLiveProfilePage       []byte
	JSContactPage           []byte
	JSWechatOfficialAccount []byte
}

var Assets = NewChannelInjectedFiles("")

const channelAssetsPath = "/__wx_channels_assets"
const ChannelLibAssetCacheControl = "public, max-age=2592000, immutable"
const ChannelSrcAssetCacheControl = "no-cache"
const channelLibAssetCacheControl = ChannelLibAssetCacheControl
const channelSrcAssetCacheControl = ChannelSrcAssetCacheControl

const timelessBridgeScript = `
Object.assign(Timeless, Timeless.weui.kit);
Object.assign(Timeless, Timeless.weui);
Object.assign(window, Timeless);
`

func ChannelAssetsBaseURL(protocol string, hostname string, port int) string {
	protocol = strings.TrimSpace(protocol)
	protocol = strings.TrimSuffix(protocol, "://")
	protocol = strings.TrimSuffix(protocol, ":")
	if protocol == "" {
		protocol = "http"
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "127.0.0.1"
	}
	host, embeddedPort, err := net.SplitHostPort(hostname)
	if err == nil {
		host = normalizeChannelAssetHostname(host)
		host = net.JoinHostPort(host, embeddedPort)
	} else {
		host = normalizeChannelAssetHostname(hostname)
		if port > 0 {
			host = net.JoinHostPort(host, strconv.Itoa(port))
		}
	}
	return (&url.URL{
		Scheme: protocol,
		Host:   host,
		Path:   channelAssetsPath,
	}).String()
}

func normalizeChannelAssetHostname(hostname string) string {
	hostname = strings.TrimSpace(hostname)
	if strings.HasPrefix(hostname, "[") && strings.HasSuffix(hostname, "]") {
		hostname = strings.TrimPrefix(strings.TrimSuffix(hostname, "]"), "[")
	}
	switch hostname {
	case "", "0.0.0.0", "::":
		return "127.0.0.1"
	default:
		return hostname
	}
}

func ChannelAssetsBaseURLFromConfig(cfg *InterceptorConfig) string {
	if cfg == nil {
		return ChannelAssetsBaseURL("", "", 0)
	}
	return ChannelAssetsBaseURL(cfg.APIServerProtocol, cfg.APIServerHostname, cfg.APIServerPort)
}

func ChannelAssetsSameOriginBaseURL() string {
	return channelAssetsPath
}

func ChannelLibAssetURL(baseURL string, version string, rel string) string {
	if version == "" {
		version = "static"
	}
	return strings.TrimRight(baseURL, "/") + "/lib/" + rel + "?v=" + url.QueryEscape(version)
}

func ChannelSrcAssetURL(baseURL string, rel string) string {
	return strings.TrimRight(baseURL, "/") + "/src/" + rel
}

func AppendScriptSrcs(b *strings.Builder, attr string, srcs ...string) {
	for _, src := range srcs {
		if src == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(`<script%s src="%s"></script>`, attr, src))
	}
}

func AppendStylesheetHrefs(b *strings.Builder, attr string, hrefs ...string) {
	for _, href := range hrefs {
		if href == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(`<link%s rel="stylesheet" href="%s">`, attr, href))
	}
}

func AppendInlineScript(b *strings.Builder, attr string, script string) {
	if script == "" {
		return
	}
	b.WriteString(fmt.Sprintf(`<script%s>%s</script>`, attr, script))
}

func AppendInlineStyle(b *strings.Builder, attr string, css string) {
	if css == "" {
		return
	}
	css = strings.ReplaceAll(css, "</style", `<\/style`)
	b.WriteString(fmt.Sprintf(`<style%s>%s</style>`, attr, css))
}

func AppendSharedLibAssets(b *strings.Builder, baseURL string, version string, scriptAttr string, styleAttr string) {
	AppendScriptSrcs(
		b,
		scriptAttr,
		ChannelLibAssetURL(baseURL, version, "mitt.umd.js"),
		ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.umd.min.js"),
		ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.utils.umd.min.js"),
	)
	AppendStylesheetHrefs(b, styleAttr, ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.weui.css"))
	AppendScriptSrcs(b, scriptAttr, ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.weui.umd.min.js"))
	AppendInlineScript(b, scriptAttr, timelessBridgeScript)
	AppendScriptSrcs(
		b,
		scriptAttr,
		ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.dom.umd.min.js"),
		ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.web.umd.min.js"),
	)
}

func AppendSharedLibAssetsWithInlineShadcnCSS(b *strings.Builder, baseURL string, version string, scriptAttr string, styleAttr string, shadcnCSS []byte) {
	AppendScriptSrcs(
		b,
		scriptAttr,
		ChannelLibAssetURL(baseURL, version, "mitt.umd.js"),
		ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.umd.min.js"),
		ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.utils.umd.min.js"),
	)
	if len(shadcnCSS) > 0 {
		shadcnCSS = ChannelStaticAssetResponseData("timeless.weui.css", shadcnCSS)
		AppendInlineStyle(b, styleAttr, string(shadcnCSS))
	} else {
		AppendStylesheetHrefs(b, styleAttr, ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.weui.css"))
	}
	AppendScriptSrcs(b, scriptAttr, ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.weui.umd.min.js"))
	AppendInlineScript(b, scriptAttr, timelessBridgeScript)
	AppendScriptSrcs(
		b,
		scriptAttr,
		ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.dom.umd.min.js"),
		ChannelLibAssetURL(baseURL, version, "timeless/0.27.1/timeless.web.umd.min.js"),
	)
}

func mockChannelStaticAsset(ctx proxy.Context, pathname string, files *ChannelInjectedFiles) bool {
	if files == nil {
		return false
	}
	if rel, ok := channelStaticAssetRel(pathname, "lib"); ok {
		data, err := files.ReadLib(rel)
		if err != nil {
			return false
		}
		data = ChannelStaticAssetResponseData(rel, data)
		ctx.Mock(200, map[string]string{
			"Content-Type":  channelStaticAssetContentType(rel),
			"Cache-Control": channelLibAssetCacheControl,
		}, string(data))
		return true
	}
	if rel, ok := channelStaticAssetRel(pathname, "src"); ok {
		data, err := files.ReadSrc(rel)
		if err != nil {
			return false
		}
		etag := channelStaticAssetETag(data)
		headers := map[string]string{
			"Content-Type":  channelStaticAssetContentType(rel),
			"Cache-Control": channelSrcAssetCacheControl,
			"ETag":          etag,
		}
		if req := ctx.Req(); req != nil && req.Header != nil {
			if strings.Contains(req.Header.Get("If-None-Match"), etag) {
				ctx.Mock(304, headers, "")
				return true
			}
		}
		ctx.Mock(200, headers, string(data))
		return true
	}
	return false
}

func MockChannelStaticAsset(ctx proxy.Context, pathname string, files *ChannelInjectedFiles) bool {
	return mockChannelStaticAsset(ctx, pathname, files)
}

func ChannelStaticAssetResponseData(rel string, data []byte) []byte {
	if strings.HasSuffix(rel, "timeless.shadcn.css") {
		return []byte(stripTopLevelCascadeLayers(string(data)))
	}
	return data
}

func stripTopLevelCascadeLayers(css string) string {
	var b strings.Builder
	for i := 0; i < len(css); {
		if hasTopLevelLayerAt(css, i) {
			end, contentStart, contentEnd, ok := topLevelLayerRule(css, i)
			if ok {
				if contentStart >= 0 {
					b.WriteString(css[contentStart:contentEnd])
				}
				i = end
				continue
			}
		}
		next := copyCSSUnit(&b, css, i)
		if next <= i {
			next = i + 1
		}
		i = next
	}
	return b.String()
}

func hasTopLevelLayerAt(css string, i int) bool {
	if !strings.HasPrefix(css[i:], "@layer") {
		return false
	}
	end := i + len("@layer")
	if end >= len(css) {
		return true
	}
	return !isCSSIdentChar(css[end])
}

func topLevelLayerRule(css string, start int) (end int, contentStart int, contentEnd int, ok bool) {
	i := start + len("@layer")
	for i < len(css) {
		switch css[i] {
		case '\'', '"':
			i = skipCSSString(css, i)
		case '/':
			if i+1 < len(css) && css[i+1] == '*' {
				i = skipCSSComment(css, i)
			} else {
				i++
			}
		case ';':
			return i + 1, -1, -1, true
		case '{':
			close := findMatchingCSSBrace(css, i)
			if close < 0 {
				return 0, 0, 0, false
			}
			return close + 1, i + 1, close, true
		default:
			i++
		}
	}
	return 0, 0, 0, false
}

func copyCSSUnit(b *strings.Builder, css string, i int) int {
	switch css[i] {
	case '\'', '"':
		next := skipCSSString(css, i)
		b.WriteString(css[i:next])
		return next
	case '/':
		if i+1 < len(css) && css[i+1] == '*' {
			next := skipCSSComment(css, i)
			b.WriteString(css[i:next])
			return next
		}
	}
	b.WriteByte(css[i])
	return i + 1
}

func skipCSSString(css string, start int) int {
	quote := css[start]
	i := start + 1
	for i < len(css) {
		if css[i] == '\\' {
			i += 2
			continue
		}
		i++
		if css[i-1] == quote {
			return i
		}
	}
	return len(css)
}

func skipCSSComment(css string, start int) int {
	if end := strings.Index(css[start+2:], "*/"); end >= 0 {
		return start + 2 + end + 2
	}
	return len(css)
}

func findMatchingCSSBrace(css string, open int) int {
	depth := 0
	for i := open; i < len(css); {
		switch css[i] {
		case '\'', '"':
			i = skipCSSString(css, i)
			continue
		case '/':
			if i+1 < len(css) && css[i+1] == '*' {
				i = skipCSSComment(css, i)
				continue
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

func isCSSIdentChar(c byte) bool {
	return c == '-' || c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func channelStaticAssetRel(pathname string, dir string) (string, bool) {
	marker := "/" + dir + "/"
	idx := strings.LastIndex(pathname, marker)
	if idx < 0 {
		trimmed := strings.TrimPrefix(pathname, "/")
		prefix := dir + "/"
		if !strings.HasPrefix(trimmed, prefix) {
			return "", false
		}
		rel := strings.TrimPrefix(trimmed, prefix)
		if rel == "" || strings.Contains(rel, "..") {
			return "", false
		}
		return rel, true
	}
	rel := pathname[idx+len(marker):]
	if rel == "" || strings.Contains(rel, "..") {
		return "", false
	}
	return rel, true
}

func channelStaticAssetContentType(rel string) string {
	return ChannelStaticAssetContentType(rel)
}

func ChannelStaticAssetContentType(rel string) string {
	switch {
	case strings.HasSuffix(rel, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(rel, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(rel, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(rel, ".json"):
		return "application/json; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}

func channelStaticAssetETag(data []byte) string {
	return ChannelStaticAssetETag(data)
}

func ChannelStaticAssetETag(data []byte) string {
	hash := sha256.Sum256(data)
	return `"` + hex.EncodeToString(hash[:]) + `"`
}

func NewChannelInjectedFiles(injectDir string) *ChannelInjectedFiles {
	if injectDir == "" {
		injectDir = findChannelInjectDir()
	}
	if abs, err := filepath.Abs(injectDir); err == nil {
		injectDir = abs
	}
	files := &ChannelInjectedFiles{
		InjectDir: injectDir,
		RootFS:    os.DirFS(injectDir),
		LibFS:     os.DirFS(filepath.Join(injectDir, "lib")),
		SrcFS:     os.DirFS(filepath.Join(injectDir, "src")),
	}
	files.JSFileSaver = files.readLib("FileSaver.min.js")
	files.JSZip = files.readLib("jszip.min.js")
	files.JSRecorder = files.readLib("recorder.min.js")
	files.JSPageSpy = files.readLib("pagespy.min.js")
	files.JSMitt = files.readLib("mitt.umd.js")
	files.JSAxios = files.readLib("axios.min.js")
	files.JSGetFeedInfo = files.readLib("getFeedInfo.js")
	files.JSTimeless = files.readLib("timeless/0.27.1/timeless.umd.min.js")
	files.JSTimelessUtils = files.readLib("timeless/0.27.1/timeless.utils.umd.min.js")
	files.CSSTimelessShadcn = files.readLib("timeless/0.27.1/timeless.weui.css")
	files.JSTimelessShadcn = files.readLib("timeless/0.27.1/timeless.weui.umd.min.js")
	files.JSTimelessDOM = files.readLib("timeless/0.27.1/timeless.dom.umd.min.js")
	files.JSTimelessWeb = files.readLib("timeless/0.27.1/timeless.web.umd.min.js")
	files.CSSComponents = files.readSrc("components.css")
	files.JSDebug = files.readSrc("pagespy.js")
	files.JSError = files.readSrc("error.js")
	files.JSEventBus = files.readSrc("eventbus.js")
	files.JSEnv = files.readSrc("env.js")
	files.JSEnvChannels = files.readSrc("env.channels.js")
	files.JSEnvMock = files.readSrc("env.mock.js")
	files.JSComponents = files.readSrc("components.js")
	files.JSUtils = files.readSrc("utils.js")
	files.JSChannels = files.readSrc("channels.js")
	files.JSDownloadCore = files.readSrc("download/core.js")
	files.JSDownloadPanel = files.readSrc("download/panel.js")
	files.JSDownloadIndex = files.readSrc("download/index.js")
	files.JSDownloader = files.JSDownloadPanel
	files.JSWechatOfficialAccount = files.readSrc("officialaccount.js")
	files.JSHomePage = files.readSrc("home.js")
	files.JSFeedProfilePage = files.readSrc("feed.js")
	files.JSLiveProfilePage = files.readSrc("live.js")
	files.JSContactPage = files.readSrc("profile.js")
	return files
}

func (files *ChannelInjectedFiles) ReadLib(rel string) ([]byte, error) {
	return readChannelAsset(files.LibFS, rel)
}

func (files *ChannelInjectedFiles) ReadSrc(rel string) ([]byte, error) {
	return readChannelAsset(files.SrcFS, rel)
}

func (files *ChannelInjectedFiles) ReadRoot(rel string) ([]byte, error) {
	return readChannelAsset(files.RootFS, rel)
}

func (files *ChannelInjectedFiles) readLib(rel string) []byte {
	data, _ := files.ReadLib(rel)
	return data
}

func (files *ChannelInjectedFiles) readSrc(rel string) []byte {
	data, _ := files.ReadSrc(rel)
	return data
}

func readChannelAsset(assetFS fs.FS, rel string) ([]byte, error) {
	clean, ok := cleanChannelAssetRel(rel)
	if !ok {
		return nil, fs.ErrInvalid
	}
	return fs.ReadFile(assetFS, clean)
}

func cleanChannelAssetRel(rel string) (string, bool) {
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") || strings.ContainsRune(rel, 0) {
		return "", false
	}
	clean := path.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", false
	}
	return clean, true
}

func findChannelInjectDir() string {
	candidates := []string{defaultChannelInjectDir}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "inject"),
			filepath.Join(exeDir, defaultChannelInjectDir),
		)
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(filepath.Join(candidate, "lib")); err == nil && stat.IsDir() {
			return candidate
		}
	}
	return defaultChannelInjectDir
}
