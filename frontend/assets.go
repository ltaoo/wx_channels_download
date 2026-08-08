package frontend

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
)

const default_assets_dir = "frontend"

type UserScripts struct {
	root_fs          fs.FS
	src_fs           fs.FS
	inject_script_fs fs.FS
	public_fs        fs.FS
}

var assets = NewUserScripts("")

func Assets() *UserScripts {
	return assets
}

const assets_path = "/__assets"
const defaultUserGlobalScriptAssetName = "global.js"
const PublicAssetCacheControl = "no-cache"
const SrcAssetCacheControl = "no-cache"

func UserGlobalScriptAssetPath(script_path string) string {
	name := strings.TrimSpace(filepath.Base(script_path))
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		name = defaultUserGlobalScriptAssetName
	}
	return path.Join(assets_path, "user", name)
}

func AssetsBaseURL(protocol string, hostname string, port int) string {
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
	host, embedded_port, err := net.SplitHostPort(hostname)
	if err == nil {
		host = normalize_asset_hostname(host)
		host = net.JoinHostPort(host, embedded_port)
	} else {
		host = normalize_asset_hostname(hostname)
		if port > 0 {
			host = net.JoinHostPort(host, strconv.Itoa(port))
		}
	}
	return (&url.URL{
		Scheme: protocol,
		Host:   host,
		Path:   assets_path,
	}).String()
}

func normalize_asset_hostname(hostname string) string {
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

func AssetsBaseURLFromConfig(protocol string, hostname string, port int) string {
	return AssetsBaseURL(protocol, hostname, port)
}

func NewURLBuild(base_url string, query url.Values) func(asset_path string, query ...url.Values) string {
	base_url = strings.TrimRight(base_url, "/")
	base_query := clone_url_values(query)
	return func(asset_path string, query ...url.Values) string {
		raw_url := base_url + "/" + strings.TrimLeft(asset_path, "/")
		asset_url, err := url.Parse(raw_url)
		if err != nil {
			return raw_url
		}
		search := clone_url_values(base_query)
		override_url_values(search, asset_url.Query())
		for _, values := range query {
			override_url_values(search, values)
		}
		asset_url.RawQuery = search.Encode()
		return asset_url.String()
	}
}

func clone_url_values(source url.Values) url.Values {
	cloned := make(url.Values, len(source))
	for key, values := range source {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func override_url_values(target url.Values, source url.Values) {
	for key, values := range source {
		target.Del(key)
		for _, value := range values {
			target.Add(key, value)
		}
	}
}

func AppendScripts(b *strings.Builder, attr string, srcs ...string) {
	for _, src := range srcs {
		if src == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(`<script%s src="%s"></script>`, attr, src))
	}
}

func AppendStylesheets(b *strings.Builder, attr string, hrefs ...string) {
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

func StaticAssetResponseData(rel string, data []byte) []byte {
	if strings.HasSuffix(rel, "timeless.shadcn.css") {
		return []byte(strip_top_level_cascade_layers(string(data)))
	}
	return data
}

func strip_top_level_cascade_layers(css string) string {
	var b strings.Builder
	for i := 0; i < len(css); {
		if has_top_level_layer_at(css, i) {
			end, content_start, content_end, ok := top_level_layer_rule(css, i)
			if ok {
				if content_start >= 0 {
					b.WriteString(css[content_start:content_end])
				}
				i = end
				continue
			}
		}
		next := copy_css_unit(&b, css, i)
		if next <= i {
			next = i + 1
		}
		i = next
	}
	return b.String()
}

func has_top_level_layer_at(css string, i int) bool {
	if !strings.HasPrefix(css[i:], "@layer") {
		return false
	}
	end := i + len("@layer")
	if end >= len(css) {
		return true
	}
	return !is_css_ident_char(css[end])
}

func top_level_layer_rule(css string, start int) (end int, content_start int, content_end int, ok bool) {
	i := start + len("@layer")
	for i < len(css) {
		switch css[i] {
		case '\'', '"':
			i = skip_css_string(css, i)
		case '/':
			if i+1 < len(css) && css[i+1] == '*' {
				i = skip_css_comment(css, i)
			} else {
				i++
			}
		case ';':
			return i + 1, -1, -1, true
		case '{':
			close := find_matching_css_brace(css, i)
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

func copy_css_unit(b *strings.Builder, css string, i int) int {
	switch css[i] {
	case '\'', '"':
		next := skip_css_string(css, i)
		b.WriteString(css[i:next])
		return next
	case '/':
		if i+1 < len(css) && css[i+1] == '*' {
			next := skip_css_comment(css, i)
			b.WriteString(css[i:next])
			return next
		}
	}
	b.WriteByte(css[i])
	return i + 1
}

func skip_css_string(css string, start int) int {
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

func skip_css_comment(css string, start int) int {
	if end := strings.Index(css[start+2:], "*/"); end >= 0 {
		return start + 2 + end + 2
	}
	return len(css)
}

func find_matching_css_brace(css string, open int) int {
	depth := 0
	for i := open; i < len(css); {
		switch css[i] {
		case '\'', '"':
			i = skip_css_string(css, i)
			continue
		case '/':
			if i+1 < len(css) && css[i+1] == '*' {
				i = skip_css_comment(css, i)
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

func is_css_ident_char(c byte) bool {
	return c == '-' || c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func StaticAssetContentType(rel string) string {
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

func StaticAssetETag(data []byte) string {
	hash := sha256.Sum256(data)
	return `"` + hex.EncodeToString(hash[:]) + `"`
}

func NewUserScripts(inject_dir string) *UserScripts {
	if root_fs := embeddedRootFS(); root_fs != nil {
		return &UserScripts{
			root_fs:          root_fs,
			src_fs:           embeddedSrcFS(),
			inject_script_fs: embeddedInjectFS(),
			public_fs:        embeddedPublicFS(),
		}
	}
	if inject_dir == "" {
		inject_dir = find_assets_dir()
	}
	if abs, err := filepath.Abs(inject_dir); err == nil {
		inject_dir = abs
	}
	return &UserScripts{
		root_fs:          os.DirFS(inject_dir),
		src_fs:           os.DirFS(filepath.Join(inject_dir, "src")),
		inject_script_fs: os.DirFS(filepath.Join(inject_dir, "inject")),
		public_fs:        os.DirFS(filepath.Join(inject_dir, "public")),
	}
}

func (files *UserScripts) ReadSrc(rel string) ([]byte, error) {
	return read_asset(files.src_fs, rel)
}

func (files *UserScripts) ReadInject(rel string) ([]byte, error) {
	return read_asset(files.inject_script_fs, rel)
}

func (files *UserScripts) ReadRoot(rel string) ([]byte, error) {
	return read_asset(files.root_fs, rel)
}

func (files *UserScripts) ReadPublic(rel string) ([]byte, error) {
	return read_asset(files.public_fs, rel)
}

func read_asset(asset_fs fs.FS, rel string) ([]byte, error) {
	if asset_fs == nil {
		return nil, fs.ErrNotExist
	}
	clean, ok := clean_asset_rel(rel)
	if !ok {
		return nil, fs.ErrInvalid
	}
	return fs.ReadFile(asset_fs, clean)
}

func clean_asset_rel(rel string) (string, bool) {
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

func find_assets_dir() string {
	candidates := []string{default_assets_dir}
	if exe, err := os.Executable(); err == nil {
		exe_dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exe_dir, "inject"),
			filepath.Join(exe_dir, default_assets_dir),
		)
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(filepath.Join(candidate, "public")); err == nil && stat.IsDir() {
			return candidate
		}
	}
	return default_assets_dir
}
