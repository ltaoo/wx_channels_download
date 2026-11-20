//go:build !windows

package application

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
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
