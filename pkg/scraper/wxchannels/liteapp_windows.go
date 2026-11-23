package wxchannels

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/rs/zerolog"
)

func wechat_cache_in_use(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	// Inspect image names without requiring access to another process's memory.
	// Native enumeration also avoids localized tasklist output and console windows.
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, fmt.Errorf("检查微信进程失败: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if liteapp_wechat_process_name(windows.UTF16ToString(entry.ExeFile[:])) {
			return true, nil
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return false, fmt.Errorf("检查微信进程失败: %w", err)
	}
	return false, nil
}

func liteapp_wechat_process_name(name string) bool {
	switch strings.ToLower(name) {
	case "weixin.exe", "wechat.exe", "wechatappex.exe", "weixinappex.exe", "wechatplayer.exe", "weixinplayer.exe":
		return true
	}
	return false
}

func liteapp_cache_profiles_root() (string, error) {
	// UserConfigDir uses APPDATA on Windows, including redirected profiles.
	config_dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config_dir, "Tencent", "xwechat", "radium", "web", "profiles"), nil
}

// StartLiteAppCacheCleanup runs after the proxy starts so a subsequent SDK request receives the loader.
func StartLiteAppCacheCleanup(ctx context.Context, version string, logger *zerolog.Logger) {
	start_liteapp_cache_cleanup(ctx, version, logger, liteapp_cache_profiles_root, wechat_cache_in_use, prepare_liteapp_sdk_cache_when_idle)
}
