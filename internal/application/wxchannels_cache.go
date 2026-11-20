package application

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
)

var liteapp_cache_file_reg = regexp.MustCompile(`^[0-9a-f]{16}_0$`)
var liteapp_cache_sdk_reg = regexp.MustCompile(`^/t/wx_fed/cdn_libs/res/libpag/[^/]+/libpag(?:\.min)?\.js$`)

const liteapp_cache_version_filename = ".wxchannels-liteapp-sdk-version"

type liteapp_cache_result struct {
	matched     int
	deferred    bool
	backup_dirs []string
}

// Run after the proxy starts so a subsequent SDK request receives the loader.
func start_wxchannels_liteapp_cache_cleanup(ctx context.Context, cfg *config.Config) {
	if (runtime.GOOS != "darwin" && runtime.GOOS != "windows") || adapter.Get("wxchannels") == nil {
		return
	}
	profiles_root, err := liteapp_cache_profiles_root(runtime.GOOS)
	if err != nil {
		color.Yellow("无法检查微信 LiteApp SDK 缓存：%v", err)
		return
	}
	is_running := func() (bool, error) { return wechat_cache_in_use(ctx) }
	prepare_unversioned := func() (liteapp_cache_result, error) {
		if runtime.GOOS == "windows" {
			return prepare_liteapp_sdk_cache_when_idle(profiles_root, is_running)
		}
		return prepare_liteapp_sdk_cache(profiles_root, is_running)
	}
	prepare_cache := func() (liteapp_cache_result, error) {
		return prepare_liteapp_sdk_cache_for_version(profiles_root, cfg.Version, prepare_unversioned)
	}
	report := func(result liteapp_cache_result, err error) {
		for _, backup_dir := range result.backup_dirs {
			color.Green("已移走微信 LiteApp SDK 缓存，备份：%s", backup_dir)
			cfg.Logger().Info().Str("backup_dir", backup_dir).Msg("moved WeChat LiteApp SDK cache")
		}
		if err != nil && ctx.Err() == nil {
			color.Yellow("微信 LiteApp SDK 缓存处理未完成：%v", err)
			cfg.Logger().Warn().Err(err).Msg("failed to prepare WeChat LiteApp SDK cache")
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

func liteapp_cache_profiles_root(goos string) (string, error) {
	if goos == "windows" {
		// UserConfigDir uses APPDATA on Windows, including redirected profiles.
		config_dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(config_dir, "Tencent", "xwechat", "radium", "web", "profiles"), nil
	}
	home_dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home_dir, "Library", "Containers", "com.tencent.xinWeChat", "Data", "Documents", "app_data", "radium", "web", "profiles"), nil
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
	return err == nil && sdk_url.Host == "res.wx.qq.com" && sdk_url.User == nil && liteapp_cache_sdk_reg.MatchString(sdk_url.Path)
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
