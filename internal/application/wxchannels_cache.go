package application

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
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

type liteapp_cache_result struct {
	matched     int
	deferred    bool
	backup_dirs []string
}

// Run after the proxy starts so a subsequent SDK request receives the loader.
func start_wxchannels_liteapp_cache_cleanup(ctx context.Context, cfg *config.Config) {
	if runtime.GOOS != "darwin" || adapter.Get("wxchannels") == nil {
		return
	}
	home_dir, err := os.UserHomeDir()
	if err != nil {
		color.Yellow("无法检查微信 LiteApp SDK 缓存：%v", err)
		return
	}
	profiles_root := filepath.Join(home_dir, "Library", "Containers", "com.tencent.xinWeChat", "Data", "Documents", "app_data", "radium", "web", "profiles")
	is_running := func() (bool, error) { return wechat_cache_in_use(ctx) }
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
	result, err := prepare_liteapp_sdk_cache(profiles_root, is_running)
	report(result, err)
	if !result.deferred {
		return
	}
	color.Yellow("发现 %d 条微信 LiteApp SDK 缓存。请完全退出微信，应用会自动备份移走缓存；看到处理完成提示后再打开微信。", result.matched)
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
				result, err := prepare_liteapp_sdk_cache(profiles_root, is_running)
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

func wechat_cache_in_use(ctx context.Context) (bool, error) {
	check_ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Include Chromium helpers, which can outlive the main WeChat process.
	err := exec.CommandContext(check_ctx, "pgrep", "-x", "WeChat|WeChatAppEx.*").Run()
	if check_ctx.Err() != nil {
		return false, check_ctx.Err()
	}
	if err == nil {
		return true, nil
	}
	var exit_err *exec.ExitError
	if errors.As(err, &exit_err) && exit_err.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("检查微信进程失败: %w", err)
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
	var candidates []string
	var scan_errors []error
	for _, profile := range profiles {
		if !profile.IsDir() {
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
				candidates = append(candidates, cache_path)
			}
		}
	}
	result.matched = len(candidates)
	for _, cache_path := range candidates {
		running, err := is_running()
		if err != nil {
			return result, errors.Join(append(scan_errors, err)...)
		}
		if running {
			result.deferred = true
			break
		}
		backup_dir, err := move_liteapp_sdk_cache(cache_path)
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
	return info.IsDir(), nil
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
	// Cache keys can start with a partition prefix such as "1/0/".
	url_start := strings.Index(string(key), "https://")
	if url_start < 0 {
		return false, nil
	}
	sdk_url, err := url.Parse(string(key[url_start:]))
	return err == nil && sdk_url.Host == "res.wx.qq.com" && sdk_url.User == nil && liteapp_cache_sdk_reg.MatchString(sdk_url.Path), nil
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
	backup_dir, err := os.MkdirTemp(backup_root, time.Now().Format("20060102-150405")+"-*")
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
