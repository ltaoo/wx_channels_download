package application

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const test_liteapp_sdk_url = "https://res.wx.qq.com/t/wx_fed/cdn_libs/res/libpag/4.3.48/libpag.min.js"

func write_test_liteapp_cache(t *testing.T, path, key string) []byte {
	t.Helper()
	data := make([]byte, 24)
	binary.LittleEndian.PutUint64(data[:8], 0xfcfb6d1ba7725c30)
	binary.LittleEndian.PutUint32(data[8:12], 5)
	binary.LittleEndian.PutUint32(data[12:16], uint32(len(key)))
	data = append(data, key...)
	data = append(data, []byte("original cached response\x00cache-control:max-age=31536000")...)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLiteAppSDKCacheMovesOnlyMatchingEntries(t *testing.T) {
	root := t.TempDir()
	wanted := map[string][]byte{}
	untouched := map[string][]byte{}
	for index, key := range []string{"1/0/" + test_liteapp_sdk_url, "1/0/" + strings.Replace(test_liteapp_sdk_url, "libpag.min.js", "libpag.js?t=261106", 1)} {
		profile := []string{"multitab_account", "webview_account"}[index]
		path := filepath.Join(root, profile, "Cache", "Cache_Data", "a9500941d7561aa1_0")
		wanted[path] = write_test_liteapp_cache(t, path, key)
		for _, suffix := range []string{"_1", "_s"} {
			companion := strings.TrimSuffix(path, "_0") + suffix
			data := []byte("preserve companion " + suffix)
			if err := os.WriteFile(companion, data, 0600); err != nil {
				t.Fatal(err)
			}
			wanted[companion] = data
		}
		other_path := filepath.Join(filepath.Dir(path), "1111111111111111_0")
		// A source file that references the SDK must not count as an SDK cache key.
		untouched[other_path] = write_test_liteapp_cache(t, other_path, "1/0/https://liteapp.weixin.qq.com/feed.js")
		untouched[other_path] = append(untouched[other_path], test_liteapp_sdk_url...)
		if err := os.WriteFile(other_path, untouched[other_path], 0600); err != nil {
			t.Fatal(err)
		}
		index_path := filepath.Join(filepath.Dir(path), "index")
		untouched[index_path] = []byte("cache index")
		if err := os.WriteFile(index_path, untouched[index_path], 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := prepare_liteapp_sdk_cache(root, func() (bool, error) { return false, nil })
	if err != nil || result.deferred || result.matched != 2 || len(result.backup_dirs) != 2 {
		t.Fatalf("cleanup: %+v, %v", result, err)
	}
	for original, expected := range wanted {
		if _, err := os.Stat(original); !os.IsNotExist(err) {
			t.Fatalf("original still exists: %s (%v)", original, err)
		}
		profile_dir := filepath.Dir(filepath.Dir(filepath.Dir(original)))
		backup_dir := ""
		for _, candidate := range result.backup_dirs {
			if strings.HasPrefix(candidate, profile_dir+string(os.PathSeparator)) {
				backup_dir = candidate
			}
		}
		actual, err := os.ReadFile(filepath.Join(backup_dir, filepath.Base(original)))
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("backup differs: %s (%v)", original, err)
		}
	}
	for path, expected := range untouched {
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("unrelated cache changed: %s (%v)", path, err)
		}
	}
	result, err = prepare_liteapp_sdk_cache(root, func() (bool, error) {
		t.Fatal("no matching cache should not check processes")
		return false, nil
	})
	if err != nil || result.matched != 0 || len(result.backup_dirs) != 0 {
		t.Fatalf("repeated cleanup: %+v, %v", result, err)
	}
}

func TestLiteAppSDKCacheDefersUntilWeChatExits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "multitab", "Cache", "Cache_Data", "a9500941d7561aa1_0")
	expected := write_test_liteapp_cache(t, path, "1/0/"+test_liteapp_sdk_url)
	result, err := prepare_liteapp_sdk_cache(root, func() (bool, error) { return true, nil })
	if err != nil || !result.deferred || result.matched != 1 || len(result.backup_dirs) != 0 {
		t.Fatalf("running WeChat: %+v, %v", result, err)
	}
	process_err := errors.New("process inspection denied")
	result, err = prepare_liteapp_sdk_cache(root, func() (bool, error) { return false, process_err })
	if !errors.Is(err, process_err) || len(result.backup_dirs) != 0 {
		t.Fatalf("failed process inspection: %+v, %v", result, err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("cache changed while WeChat was running or its state was unknown")
	}
	result, err = prepare_liteapp_sdk_cache(root, func() (bool, error) { return false, nil })
	if err != nil || result.deferred || len(result.backup_dirs) != 1 {
		t.Fatalf("after WeChat exits: %+v, %v", result, err)
	}
}

func TestLiteAppSDKCacheRejectsUnrelatedOrInvalidKeys(t *testing.T) {
	for _, key := range []string{
		"1/0/https://res.wx.qq.com.evil.example/t/wx_fed/cdn_libs/res/libpag/4.3.48/libpag.min.js",
		"1/0/https://res.wx.qq.com/other/libpag.min.js",
		"1/0/http://res.wx.qq.com/t/wx_fed/cdn_libs/res/libpag/4.3.48/libpag.min.js",
		"1/0/https://res.wx.qq.com/t/wx_fed/cdn_libs/res/libpag/4.3.48/libpag.min.js.map",
		"1/0/https://user@res.wx.qq.com/t/wx_fed/cdn_libs/res/libpag/4.3.48/libpag.min.js",
	} {
		t.Run(key, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cache")
			write_test_liteapp_cache(t, path, key)
			if matches, err := liteapp_sdk_cache_matches(path); err != nil || matches {
				t.Fatalf("unexpected match: %v, %v", matches, err)
			}
		})
	}
	for _, corrupt := range []string{"magic", "version", "key_length", "truncated_key", "truncated_header"} {
		t.Run(corrupt, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cache")
			data := write_test_liteapp_cache(t, path, "1/0/"+test_liteapp_sdk_url)
			switch corrupt {
			case "magic":
				data[0] = 0
			case "version":
				binary.LittleEndian.PutUint32(data[8:12], 999)
			case "key_length":
				binary.LittleEndian.PutUint32(data[12:16], 0xffffffff)
			case "truncated_key":
				data = data[:25]
			case "truncated_header":
				data = data[:12]
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if matches, err := liteapp_sdk_cache_matches(path); err != nil || matches {
				t.Fatalf("invalid entry accepted: %v, %v", matches, err)
			}
		})
	}
}

func TestLiteAppSDKCacheBackupFailurePreservesOriginal(t *testing.T) {
	root := t.TempDir()
	profile_dir := filepath.Join(root, "multitab")
	path := filepath.Join(profile_dir, "Cache", "Cache_Data", "a9500941d7561aa1_0")
	expected := write_test_liteapp_cache(t, path, "1/0/"+test_liteapp_sdk_url)
	if err := os.WriteFile(filepath.Join(profile_dir, "wxchannels-liteapp-sdk-backup"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := prepare_liteapp_sdk_cache(root, func() (bool, error) { return false, nil })
	if err == nil || len(result.backup_dirs) != 0 {
		t.Fatalf("expected backup failure: %+v, %v", result, err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("failed backup damaged the source")
	}
}

func TestLiteAppSDKCacheSkipsSymlinksAndMissingProfiles(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	path := filepath.Join(outside, "Cache", "Cache_Data", "a9500941d7561aa1_0")
	expected := write_test_liteapp_cache(t, path, "1/0/"+test_liteapp_sdk_url)
	if err := os.Symlink(outside, filepath.Join(root, "linked_profile")); err != nil {
		t.Skip(err)
	}
	if err := os.Mkdir(filepath.Join(root, "profile"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "Cache"), filepath.Join(root, "profile", "Cache")); err != nil {
		t.Fatal(err)
	}
	for _, profiles_root := range []string{root, filepath.Join(root, "missing")} {
		result, err := prepare_liteapp_sdk_cache(profiles_root, func() (bool, error) {
			t.Fatal("unrelated or missing profiles should not check processes")
			return false, nil
		})
		if err != nil || result.matched != 0 {
			t.Fatalf("unexpected candidates: %+v, %v", result, err)
		}
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("symlink target changed")
	}
}

func TestLiteAppSDKCacheRejectsSymlinkCompanionsAndBackupDirectory(t *testing.T) {
	for _, linked := range []string{"entry", "companion", "backup"} {
		t.Run(linked, func(t *testing.T) {
			root := t.TempDir()
			profile_dir := filepath.Join(root, "multitab")
			path := filepath.Join(profile_dir, "Cache", "Cache_Data", "a9500941d7561aa1_0")
			expected := write_test_liteapp_cache(t, path, "1/0/"+test_liteapp_sdk_url)
			target := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(target, expected, 0600); err != nil {
				t.Fatal(err)
			}
			link_path := strings.TrimSuffix(path, "_0") + "_1"
			switch linked {
			case "entry":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				link_path = path
			case "backup":
				link_path = filepath.Join(profile_dir, "wxchannels-liteapp-sdk-backup")
				target = filepath.Dir(target)
			}
			if err := os.Symlink(target, link_path); err != nil {
				t.Skip(err)
			}
			result, err := prepare_liteapp_sdk_cache(root, func() (bool, error) { return false, nil })
			if len(result.backup_dirs) != 0 || (linked != "entry" && err == nil) {
				t.Fatalf("symlink was accepted: %+v, %v", result, err)
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, expected) {
				t.Fatal("symlink handling changed the cache entry")
			}
		})
	}
}
