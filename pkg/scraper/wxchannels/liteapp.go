package wxchannels

import (
	"context"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/ltaoo/echo"
	"github.com/rs/zerolog"
)

var liteapp_sdk_reg = regexp.MustCompile(`^/t/wx_fed/cdn_libs/res/libpag/[^/]+/libpag(?:\.min)?\.js$`)

const liteapp_asset_path = "/__assets/wxchannels/liteapp/"

//go:embed inject/runtime.js
var liteapp_runtime_js string

func inject_liteapp_runtime(ctx *echo.Context, logger *zerolog.Logger, inline bool) bool {
	if ctx.Req.URL.Hostname() != "res.wx.qq.com" || !liteapp_sdk_reg.MatchString(ctx.Req.URL.Path) {
		return false
	}
	if ctx.Res == nil || ctx.Res.StatusCode != http.StatusOK || !strings.Contains(strings.ToLower(ctx.GetResponseHeader("Content-Type")), "javascript") {
		return true
	}
	ctx.SetResponseHeader("Cache-Control", "no-store")
	ctx.DelResponseHeader("ETag")
	ctx.DelResponseHeader("Last-Modified")
	if ctx.Req.Method != http.MethodGet {
		return true
	}
	sdk_source, err := ctx.GetResponseBody()
	if err != nil {
		logger.Warn().Err(err).Msg("failed to read LiteApp SDK response")
		return true
	}
	script := liteapp_runtime_js
	if !inline {
		script = `(function () {
  if (location.origin !== "https://liteapp.weixin.qq.com") {
    return;
  }
  const script = document.createElement("script");
  script.setAttribute("data-n", "wxchannels-liteapp-runtime");
  script.crossOrigin = "anonymous";
  script.src = "https://res.wx.qq.com` + liteapp_asset_path + `runtime.js?t=" + Date.now();
  script.onerror = () => console.warn("[wxchannels LiteApp] failed to load runtime");
  (document.head || document.documentElement).appendChild(script);
})();`
	}
	ctx.SetResponseBody(sdk_source + "\n;" + script)
	return true
}

func dev_liteapp_runtime_path() string {
	candidates := []string{filepath.Join("pkg", "scraper", "wxchannels", "inject", "runtime.js")}
	if _, source_file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(source_file), "inject", "runtime.js"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}

func new_liteapp_runtime(page_plugin *echo.Plugin, logger *zerolog.Logger, inline bool) func(*echo.Context) bool {
	var mutex sync.Mutex
	last_status := `{}`
	runtime_path := ""
	if !inline {
		runtime_path = dev_liteapp_runtime_path()
	}
	return func(ctx *echo.Context) bool {
		pathname := ctx.Req.URL.Path
		if !strings.HasPrefix(pathname, liteapp_asset_path) {
			return false
		}
		headers := map[string]string{
			"Access-Control-Allow-Origin":  "https://liteapp.weixin.qq.com",
			"Access-Control-Allow-Methods": "GET, HEAD, POST, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
			"Cache-Control":                "no-store",
			"Content-Type":                 "application/json; charset=utf-8",
			"X-Content-Type-Options":       "nosniff",
		}
		if origin := ctx.Req.Header.Get("Origin"); origin != "" && origin != "https://liteapp.weixin.qq.com" {
			ctx.Mock(http.StatusForbidden, headers, `{"error":"unexpected origin"}`)
			return true
		}
		if ctx.Req.Method == http.MethodOptions {
			ctx.Mock(http.StatusNoContent, headers, "")
			return true
		}
		if !inline && pathname == liteapp_asset_path+"runtime.js" && (ctx.Req.Method == http.MethodGet || ctx.Req.Method == http.MethodHead) {
			// Read on every request so development edits do not require restarting Go.
			source, err := os.ReadFile(runtime_path)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to read LiteApp runtime source")
				ctx.Mock(http.StatusInternalServerError, headers, `{"error":"failed to read LiteApp runtime"}`)
				return true
			}
			headers["Content-Type"] = "application/javascript; charset=utf-8"
			body := string(source)
			if ctx.Req.Method == http.MethodHead {
				body = ""
			}
			ctx.Mock(http.StatusOK, headers, body)
			return true
		}
		if pathname == liteapp_asset_path+"bootstrap" && ctx.Req.Method == http.MethodGet {
			req, _ := http.NewRequest(http.MethodGet, "https://channels.weixin.qq.com/web/pages/feed", nil)
			page_ctx := &echo.Context{Req: req, Res: &http.Response{StatusCode: http.StatusOK,
				Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader("<html><head></head><body></body></html>"))}}
			page_plugin.OnResponse(page_ctx)
			bootstrap, err := page_ctx.GetResponseBody()
			if err != nil {
				ctx.Mock(http.StatusInternalServerError, headers, `{"error":"failed to build injection assets"}`)
				return true
			}
			bootstrap = strings.ReplaceAll(bootstrap, `"assets_base_url":"/__assets"`, `"assets_base_url":"https://res.wx.qq.com/__assets"`)
			response, _ := json.Marshal(map[string]string{"bootstrap": bootstrap})
			ctx.Mock(http.StatusOK, headers, string(response))
			return true
		}
		if pathname == liteapp_asset_path+"status" {
			if ctx.Req.Method == http.MethodPost {
				var status struct {
					Stage        string `json:"stage"`
					Revision     string `json:"revision"`
					Methods      int    `json:"methods"`
					InitEvents   int    `json:"init_events"`
					DetailEvents int    `json:"detail_events"`
					AppPreserved bool   `json:"app_preserved"`
					Error        string `json:"error,omitempty"`
				}
				body, err := io.ReadAll(io.LimitReader(ctx.Req.Body, 4097))
				if err != nil || len(body) > 4096 || json.Unmarshal(body, &status) != nil {
					ctx.Mock(http.StatusBadRequest, headers, `{"error":"invalid status"}`)
					return true
				}
				body, _ = json.Marshal(status)
				mutex.Lock()
				last_status = string(body)
				mutex.Unlock()
				logger.Info().RawJSON("status", body).Msg("LiteApp rewrite status")
			}
			if ctx.Req.Method == http.MethodGet || ctx.Req.Method == http.MethodPost {
				mutex.Lock()
				body := last_status
				mutex.Unlock()
				ctx.Mock(http.StatusOK, headers, body)
				return true
			}
		}
		ctx.Mock(http.StatusNotFound, headers, `{"error":"unknown LiteApp asset"}`)
		return true
	}
}

var liteapp_cache_file_reg = regexp.MustCompile(`^[0-9a-f]{16}_0$`)

const liteapp_cache_version_filename = ".wxchannels-liteapp-sdk-version"

type liteapp_cache_result struct {
	matched     int
	deferred    bool
	backup_dirs []string
}

// Runs after the proxy starts so a subsequent SDK request receives the loader.
func start_liteapp_cache_cleanup(
	ctx context.Context, version string, logger *zerolog.Logger,
	profiles_root_path func() (string, error),
	cache_in_use func(context.Context) (bool, error),
	prepare_cache_for_platform func(string, func() (bool, error)) (liteapp_cache_result, error),
) {
	profiles_root, err := profiles_root_path()
	if err != nil {
		color.Yellow("无法检查微信 LiteApp SDK 缓存：%v", err)
		return
	}
	is_running := func() (bool, error) { return cache_in_use(ctx) }
	prepare_unversioned := func() (liteapp_cache_result, error) {
		return prepare_cache_for_platform(profiles_root, is_running)
	}
	prepare_cache := func() (liteapp_cache_result, error) {
		return prepare_liteapp_sdk_cache_for_version(profiles_root, version, prepare_unversioned)
	}
	report := func(result liteapp_cache_result, err error) {
		for _, backup_dir := range result.backup_dirs {
			color.Green("已移走微信 LiteApp SDK 缓存，备份：%s", backup_dir)
			logger.Info().Str("backup_dir", backup_dir).Msg("moved WeChat LiteApp SDK cache")
		}
		if err != nil && ctx.Err() == nil {
			color.Yellow("微信 LiteApp SDK 缓存处理未完成：%v", err)
			logger.Warn().Err(err).Msg("failed to prepare WeChat LiteApp SDK cache")
		}
	}
	result, err := prepare_cache()
	report(result, err)
	if !result.deferred {
		return
	}
	color.Yellow("微信仍在运行。请完全退出微信，应用会自动检查并备份移走 LiteApp SDK 缓存；看到处理完成提示后再打开微信。")
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				running, err := is_running()
				if err != nil {
					report(liteapp_cache_result{}, err)
					return
				}
				if running {
					continue
				}
				result, err := prepare_cache()
				report(result, err)
				if !result.deferred {
					if err == nil {
						color.Green("微信 LiteApp SDK 缓存检查完成，可以重新打开微信。")
					}
					return
				}
			}
		}
	}()
}

// The marker belongs to the WeChat profiles, so changing the downloader's work
// directory does not cause another cleanup of the same cache for this version.
func prepare_liteapp_sdk_cache_for_version(profiles_root, version string, prepare func() (liteapp_cache_result, error)) (liteapp_cache_result, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return liteapp_cache_result{}, fmt.Errorf("empty application version for LiteApp SDK cache cleanup")
	}
	valid, err := liteapp_cache_directory(profiles_root)
	if err != nil || !valid {
		return liteapp_cache_result{}, err
	}
	version_path := filepath.Join(profiles_root, liteapp_cache_version_filename)
	file, err := open_liteapp_cache_file(version_path)
	if err != nil && !os.IsNotExist(err) {
		return liteapp_cache_result{}, err
	}
	if err == nil {
		data, read_err := io.ReadAll(io.LimitReader(file, 4096))
		close_err := file.Close()
		if err := errors.Join(read_err, close_err); err != nil {
			return liteapp_cache_result{}, err
		}
		if strings.TrimSpace(string(data)) == version {
			return liteapp_cache_result{}, nil
		}
	}
	result, err := prepare()
	if err != nil || result.deferred {
		return result, err
	}
	// Commit only a completed scan/cleanup. Failures and process-exit waits must
	// remain retryable on the next attempt, including when no marker existed yet.
	return result, write_liteapp_cache_version(version_path, version)
}

func write_liteapp_cache_version(path, version string) error {
	file, err := os.CreateTemp(filepath.Dir(path), liteapp_cache_version_filename+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := io.WriteString(file, version+"\n"); err != nil {
		file.Close()
		return err
	}
	sync_err := file.Sync()
	close_err := file.Close()
	if err := errors.Join(sync_err, close_err); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// Windows Chromium can exclusively lock the index while running. Defer the
// scan itself, not just the move, so a locked index is retried after exit.
func prepare_liteapp_sdk_cache_when_idle(profiles_root string, is_running func() (bool, error)) (liteapp_cache_result, error) {
	valid, err := liteapp_cache_directory(profiles_root)
	if err != nil || !valid {
		return liteapp_cache_result{}, err
	}
	running, err := is_running()
	if err != nil || running {
		return liteapp_cache_result{deferred: running}, err
	}
	return prepare_liteapp_sdk_cache(profiles_root, is_running)
}

func prepare_liteapp_sdk_cache(profiles_root string, is_running func() (bool, error)) (liteapp_cache_result, error) {
	result := liteapp_cache_result{}
	profiles, err := os.ReadDir(profiles_root)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	type candidate struct {
		path      string
		blockfile bool
	}
	var candidates []candidate
	var scan_errors []error
	for _, profile := range profiles {
		if !profile.IsDir() {
			continue
		}
		valid_profile, err := liteapp_cache_directory(filepath.Join(profiles_root, profile.Name()))
		if err != nil {
			scan_errors = append(scan_errors, err)
		}
		if !valid_profile {
			continue
		}
		cache_root := filepath.Join(profiles_root, profile.Name(), "Cache")
		cache_dir := filepath.Join(cache_root, "Cache_Data")
		// Do not follow symlinks into another application's cache.
		valid_cache_dir := true
		for _, directory := range []string{cache_root, cache_dir} {
			valid, err := liteapp_cache_directory(directory)
			if err != nil {
				scan_errors = append(scan_errors, err)
			}
			if !valid {
				valid_cache_dir = false
				break
			}
		}
		if !valid_cache_dir {
			continue
		}
		blockfile_matches, err := liteapp_blockfile_cache_matches(cache_dir)
		if err != nil {
			scan_errors = append(scan_errors, fmt.Errorf("read %s: %w", cache_dir, err))
			continue
		}
		if blockfile_matches {
			candidates = append(candidates, candidate{path: cache_dir, blockfile: true})
			continue
		}
		files, err := os.ReadDir(cache_dir)
		if err != nil {
			scan_errors = append(scan_errors, err)
			continue
		}
		for _, file := range files {
			if !liteapp_cache_file_reg.MatchString(file.Name()) || !file.Type().IsRegular() {
				continue
			}
			cache_path := filepath.Join(cache_dir, file.Name())
			matches, err := liteapp_sdk_cache_matches(cache_path)
			if err != nil {
				scan_errors = append(scan_errors, fmt.Errorf("read %s: %w", cache_path, err))
			} else if matches {
				candidates = append(candidates, candidate{path: cache_path})
			}
		}
	}
	result.matched = len(candidates)
	for _, candidate := range candidates {
		running, err := is_running()
		if err != nil {
			return result, errors.Join(append(scan_errors, err)...)
		}
		if running {
			result.deferred = true
			break
		}
		var backup_dir string
		if candidate.blockfile {
			backup_dir, err = move_liteapp_blockfile_cache(candidate.path)
		} else {
			backup_dir, err = move_liteapp_sdk_cache(candidate.path)
		}
		if err != nil {
			scan_errors = append(scan_errors, err)
		} else if backup_dir != "" {
			result.backup_dirs = append(result.backup_dirs, backup_dir)
		}
	}
	return result, errors.Join(scan_errors...)
}

func liteapp_cache_directory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir() && info.Mode()&os.ModeSymlink == 0, nil
}

func liteapp_sdk_cache_matches(path string) (bool, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	var header [24]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return false, nil
		}
		return false, err
	}
	// Chromium Simple Cache v5: magic, version, key length, key hash, padding.
	if binary.LittleEndian.Uint64(header[:8]) != 0xfcfb6d1ba7725c30 || binary.LittleEndian.Uint32(header[8:12]) != 5 {
		return false, nil
	}
	key_length := binary.LittleEndian.Uint32(header[12:16])
	if key_length == 0 || key_length > 16*1024 {
		return false, nil
	}
	key := make([]byte, key_length)
	if _, err := io.ReadFull(file, key); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return false, nil
		}
		return false, err
	}
	return liteapp_sdk_cache_key_matches(key), nil
}

func liteapp_sdk_cache_key_matches(key []byte) bool {
	// Cache keys can start with a partition prefix such as "1/0/".
	url_start := strings.Index(string(key), "https://")
	if url_start < 0 {
		return false
	}
	sdk_url, err := url.Parse(string(key[url_start:]))
	return err == nil && sdk_url.Host == "res.wx.qq.com" && sdk_url.User == nil && liteapp_sdk_reg.MatchString(sdk_url.Path)
}

func move_liteapp_sdk_cache(cache_path string) (string, error) {
	// Recheck the key immediately before moving; another process may have evicted it.
	matches, err := liteapp_sdk_cache_matches(cache_path)
	if err != nil || !matches {
		return "", err
	}
	var source_paths []string
	entry_prefix := strings.TrimSuffix(cache_path, "_0")
	for _, suffix := range []string{"_0", "_1", "_s"} {
		path := entry_prefix + suffix
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("refusing to move non-regular cache file: %s", path)
		}
		source_paths = append(source_paths, path)
	}
	if len(source_paths) == 0 || source_paths[0] != cache_path {
		return "", nil
	}
	profile_dir := filepath.Dir(filepath.Dir(filepath.Dir(cache_path)))
	backup_dir, err := new_liteapp_cache_backup(profile_dir)
	if err != nil {
		return "", err
	}
	for index, path := range source_paths {
		if err := os.Rename(path, filepath.Join(backup_dir, filepath.Base(path))); err != nil {
			move_errors := []error{fmt.Errorf("move %s to %s: %w", path, backup_dir, err)}
			for _, previous_path := range source_paths[:index] {
				if _, stat_err := os.Lstat(previous_path); !os.IsNotExist(stat_err) {
					move_errors = append(move_errors, fmt.Errorf("cache path reappeared; backup retained at %s", backup_dir))
					continue
				}
				move_errors = append(move_errors, os.Rename(filepath.Join(backup_dir, filepath.Base(previous_path)), previous_path))
			}
			return "", errors.Join(move_errors...)
		}
	}
	return backup_dir, nil
}

func new_liteapp_cache_backup(profile_dir string) (string, error) {
	backup_root := filepath.Join(profile_dir, "wxchannels-liteapp-sdk-backup")
	if err := os.MkdirAll(backup_root, 0700); err != nil {
		return "", err
	}
	valid_backup_root, err := liteapp_cache_directory(backup_root)
	if err != nil {
		return "", err
	}
	if !valid_backup_root {
		return "", fmt.Errorf("invalid cache backup directory: %s", backup_root)
	}
	// Keep backups outside Cache on the same filesystem for atomic renames.
	return os.MkdirTemp(backup_root, time.Now().Format("20060102-150405")+"-*")
}

// Chromium's Blockfile layout is defined in net/disk_cache/blockfile/
// disk_format.h, disk_format_base.h and addr.h. Entries share data_N files;
// moving individual files would corrupt unrelated entries in the same cache.
const liteapp_blockfile_index_header_size = 368

func liteapp_blockfile_cache_matches(cache_dir string) (bool, error) {
	index, err := open_liteapp_cache_file(filepath.Join(cache_dir, "index"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer index.Close()
	var header [liteapp_blockfile_index_header_size]byte
	if _, err := io.ReadFull(index, header[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return false, nil
		}
		return false, err
	}
	u32 := binary.LittleEndian.Uint32
	version := u32(header[4:8])
	if u32(header[:4]) != 0xc103cac3 || (version != 0x20000 && version != 0x20001 && version != 0x30000) {
		return false, nil
	}
	table_len := u32(header[28:32])
	if table_len == 0 {
		table_len = 0x10000
	}
	if table_len > 1<<20 {
		return false, fmt.Errorf("invalid Blockfile index table length: %d", table_len)
	}
	table := make([]byte, int(table_len)*4)
	if _, err := io.ReadFull(index, table); err != nil {
		return false, err
	}
	reader := liteapp_blockfile_reader{dir: cache_dir, files: make(map[uint32]*os.File)}
	defer reader.close()
	visited := make(map[uint32]bool)
	for offset := 0; offset < len(table); offset += 4 {
		for addr := u32(table[offset : offset+4]); addr != 0; {
			if visited[addr] {
				return false, fmt.Errorf("duplicate or cyclic Blockfile entry: %08x", addr)
			}
			if len(visited) >= 1<<20 {
				return false, fmt.Errorf("too many Blockfile entries")
			}
			visited[addr] = true
			// EntryStore always lives in one to four 256-byte blocks.
			if (addr>>28)&7 != 2 {
				return false, fmt.Errorf("invalid Blockfile entry address: %08x", addr)
			}
			entry, err := reader.read(addr, 256*int(1+(addr>>24)&3))
			if err != nil {
				return false, err
			}
			if u32(entry[20:24]) == 0 { // ENTRY_NORMAL
				key_len := u32(entry[32:36])
				long_key := u32(entry[36:40])
				if key_len > 0 && key_len <= 16*1024 {
					var key []byte
					if long_key != 0 {
						key, err = reader.read(long_key, int(key_len)+1)
						if err != nil {
							return false, err
						}
					} else if int(key_len) < len(entry)-96 {
						key = entry[96 : 96+int(key_len)+1]
					}
					if len(key) == int(key_len)+1 && key[key_len] == 0 && liteapp_sdk_cache_key_matches(key[:key_len]) {
						return true, nil
					}
				}
			}
			addr = u32(entry[4:8])
		}
	}
	return false, nil
}

func open_liteapp_cache_file(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular cache file: %s", path)
	}
	return os.Open(path)
}

type liteapp_blockfile_reader struct {
	dir   string
	files map[uint32]*os.File
}

func (r *liteapp_blockfile_reader) close() {
	for _, file := range r.files {
		file.Close()
	}
}

func (r *liteapp_blockfile_reader) read(addr uint32, length int) ([]byte, error) {
	if addr&0x80000000 == 0 || ((addr>>28)&7 != 0 && addr&0x0c000000 != 0) {
		return nil, fmt.Errorf("invalid Blockfile address: %08x", addr)
	}
	file_type := (addr >> 28) & 7
	file_number := (addr >> 16) & 255
	name := fmt.Sprintf("data_%d", file_number)
	var offset int64
	var block_size uint32
	if file_type == 0 {
		file_number = addr & 0x0fffffff
		name = fmt.Sprintf("f_%06x", file_number)
	} else {
		switch file_type {
		case 2:
			block_size = 256
		case 3:
			block_size = 1024
		case 4:
			block_size = 4096
		default:
			return nil, fmt.Errorf("unsupported Blockfile address: %08x", addr)
		}
		if length > int(block_size*(1+(addr>>24)&3)) {
			return nil, fmt.Errorf("Blockfile key exceeds allocated blocks")
		}
		offset = 8192 + int64(addr&0xffff)*int64(block_size)
	}
	file_id := (file_type << 28) | file_number
	file := r.files[file_id]
	if file == nil {
		var err error
		file, err = open_liteapp_cache_file(filepath.Join(r.dir, name))
		if err != nil {
			return nil, err
		}
		r.files[file_id] = file
		if file_type != 0 {
			var header [16]byte
			if _, err := file.ReadAt(header[:], 0); err != nil {
				return nil, err
			}
			u32 := binary.LittleEndian.Uint32
			if u32(header[:4]) != 0xc104cac3 || u32(header[4:8]) != 0x20000 || uint32(binary.LittleEndian.Uint16(header[8:10])) != file_number || u32(header[12:16]) != block_size {
				return nil, fmt.Errorf("invalid Blockfile header: %s", name)
			}
		}
	}
	data := make([]byte, length)
	_, err := file.ReadAt(data, offset)
	return data, err
}

func move_liteapp_blockfile_cache(cache_dir string) (string, error) {
	for _, directory := range []string{filepath.Dir(cache_dir), cache_dir} {
		valid, err := liteapp_cache_directory(directory)
		if err != nil {
			return "", err
		}
		if !valid {
			return "", fmt.Errorf("invalid cache directory: %s", directory)
		}
	}
	matches, err := liteapp_blockfile_cache_matches(cache_dir)
	if err != nil || !matches {
		return "", err
	}
	backup_dir, err := new_liteapp_cache_backup(filepath.Dir(filepath.Dir(cache_dir)))
	if err != nil {
		return "", err
	}
	// A single rename keeps the index, shared blocks and external streams intact.
	// WeChat creates a fresh Cache_Data directory on its next launch.
	if err := os.Rename(cache_dir, filepath.Join(backup_dir, "Cache_Data")); err != nil {
		return "", fmt.Errorf("move %s to %s: %w", cache_dir, backup_dir, err)
	}
	return backup_dir, nil
}
