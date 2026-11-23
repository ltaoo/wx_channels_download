package wxchannels

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
func StartLiteAppCacheCleanup(ctx context.Context, version string, logger *zerolog.Logger) {
	start_liteapp_cache_cleanup(ctx, version, logger, liteapp_cache_profiles_root, wechat_cache_in_use, prepare_liteapp_sdk_cache)
}
