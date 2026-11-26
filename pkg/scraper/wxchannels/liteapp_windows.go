package wxchannels

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

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
// The cache is keyed on the installed WeChat version; fallback_version (the
// downloader's own version) is used only when WeChat cannot be located.
func StartLiteAppCacheCleanup(ctx context.Context, fallback_version string, logger *zerolog.Logger) {
	version, err := wechat_app_version()
	if err != nil || version == "" {
		logger.Debug().Err(err).Msg("failed to detect WeChat version, falling back to the application version for the LiteApp SDK cache marker")
		version = fallback_version
	}
	start_liteapp_cache_cleanup(ctx, version, logger, liteapp_cache_profiles_root, wechat_cache_in_use, prepare_liteapp_sdk_cache_when_idle)
}

// wechat_app_version reads the installed WeChat/Weixin executable's file version.
func wechat_app_version() (string, error) {
	for _, dir := range wechat_install_paths() {
		for _, exe := range []string{"Weixin.exe", "WeChat.exe"} {
			path := filepath.Join(dir, exe)
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}
			version, err := windows_file_version(path)
			if err == nil && version != "" {
				return version, nil
			}
		}
	}
	return "", errors.New("未找到微信安装版本信息")
}

func wechat_install_paths() []string {
	keys := []struct {
		root registry.Key
		path string
	}{
		{registry.CURRENT_USER, `Software\Tencent\Weixin`},
		{registry.CURRENT_USER, `Software\Tencent\WeChat`},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Tencent\Weixin`},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Tencent\WeChat`},
		{registry.LOCAL_MACHINE, `Software\Tencent\Weixin`},
		{registry.LOCAL_MACHINE, `Software\Tencent\WeChat`},
	}
	var paths []string
	for _, key_entry := range keys {
		key, err := registry.OpenKey(key_entry.root, key_entry.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		value, _, err := key.GetStringValue("InstallPath")
		key.Close()
		if err == nil && strings.TrimSpace(value) != "" {
			paths = append(paths, value)
		}
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if base == "" {
			continue
		}
		paths = append(paths, filepath.Join(base, "Tencent", "Weixin"))
		paths = append(paths, filepath.Join(base, "Tencent", "WeChat"))
	}
	return paths
}

type vs_fixed_file_info struct {
	signature          uint32
	struct_version     uint32
	file_version_ms    uint32
	file_version_ls    uint32
	product_version_ms uint32
	product_version_ls uint32
	file_flags_mask    uint32
	file_flags         uint32
	file_os            uint32
	file_type          uint32
	file_subtype       uint32
	file_date_ms       uint32
	file_date_ls       uint32
}

func windows_file_version(path string) (string, error) {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil {
		return "", err
	}
	if size == 0 {
		return "", errors.New("empty version info")
	}
	buffer := make([]byte, size)
	defer runtime.KeepAlive(buffer)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buffer[0])); err != nil {
		return "", err
	}
	var fixed *vs_fixed_file_info
	var fixed_size uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buffer[0]), `\`, unsafe.Pointer(&fixed), &fixed_size); err != nil {
		return "", err
	}
	if fixed == nil || fixed_size < uint32(unsafe.Sizeof(vs_fixed_file_info{})) {
		return "", errors.New("invalid version info")
	}
	return fmt.Sprintf("%d.%d.%d.%d",
		fixed.file_version_ms>>16, fixed.file_version_ms&0xffff,
		fixed.file_version_ls>>16, fixed.file_version_ls&0xffff), nil
}
