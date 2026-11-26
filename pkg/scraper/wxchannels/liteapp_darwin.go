package wxchannels

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

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

func liteapp_cache_profiles_root() (string, error) {
	home_dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home_dir, "Library", "Containers", "com.tencent.xinWeChat", "Data", "Documents", "app_data", "radium", "web", "profiles"), nil
}

// StartLiteAppCacheCleanup runs after the proxy starts so a subsequent SDK request receives the loader.
// The cache is keyed on the installed WeChat version; fallback_version (the
// downloader's own version) is used only when WeChat cannot be located.
func StartLiteAppCacheCleanup(ctx context.Context, fallback_version string, logger *zerolog.Logger) {
	version, err := wechat_app_version(ctx)
	if err != nil || version == "" {
		logger.Debug().Err(err).Msg("failed to detect WeChat version, falling back to the application version for the LiteApp SDK cache marker")
		version = fallback_version
	}
	start_liteapp_cache_cleanup(ctx, version, logger, liteapp_cache_profiles_root, wechat_cache_in_use, prepare_liteapp_sdk_cache)
}

// wechat_app_version reads the installed WeChat version from its app bundle.
func wechat_app_version(ctx context.Context) (string, error) {
	bundles := []string{"/Applications/WeChat.app", "/Applications/微信.app"}
	if home_dir, err := os.UserHomeDir(); err == nil {
		bundles = append(bundles,
			filepath.Join(home_dir, "Applications", "WeChat.app"),
			filepath.Join(home_dir, "Applications", "微信.app"),
		)
	}
	var last_err error
	for _, bundle := range bundles {
		plist := filepath.Join(bundle, "Contents", "Info.plist")
		if _, err := os.Stat(plist); err != nil {
			last_err = err
			continue
		}
		for _, key := range []string{"WeChatBundleVersion", "CFBundleShortVersionString"} {
			version, err := plutil_string(ctx, key, plist)
			if err == nil && version != "" {
				return version, nil
			}
			if err != nil {
				last_err = err
			}
		}
		return "", fmt.Errorf("读取微信版本失败: %s", plist)
	}
	if last_err == nil {
		last_err = errors.New("未找到微信应用")
	}
	return "", last_err
}

func plutil_string(ctx context.Context, key, plist string) (string, error) {
	read_ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(read_ctx, "/usr/bin/plutil", "-extract", key, "raw", "-o", "-", plist).Output()
	if read_ctx.Err() != nil {
		return "", read_ctx.Err()
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
