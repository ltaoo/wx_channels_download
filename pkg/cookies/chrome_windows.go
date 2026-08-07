//go:build windows

package cookies

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

type chromeLocalState struct {
	OSCrypt struct {
		EncryptedKey string `json:"encrypted_key"`
	} `json:"os_crypt"`
}

func newChromeCookieDecryptor() (*chromeCookieDecryptor, error) {
	localStatePath, err := findChromeLocalState()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("cookies: read Chrome Local State: %w", err)
	}

	var state chromeLocalState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("cookies: parse Chrome Local State: %w", err)
	}
	if state.OSCrypt.EncryptedKey == "" {
		return nil, fmt.Errorf("cookies: Chrome Local State has no encrypted key")
	}

	encryptedKey, err := base64.StdEncoding.DecodeString(state.OSCrypt.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("cookies: decode Chrome encrypted key: %w", err)
	}
	if !strings.HasPrefix(string(encryptedKey), "DPAPI") {
		return nil, fmt.Errorf("cookies: Chrome encrypted key has no DPAPI prefix")
	}

	key, err := dpapiUnprotect(encryptedKey[len("DPAPI"):])
	if err != nil {
		return nil, fmt.Errorf("cookies: decrypt Chrome key with DPAPI: %w", err)
	}
	if len(key) == 0 {
		return nil, fmt.Errorf("cookies: DPAPI returned an empty Chrome key")
	}

	return &chromeCookieDecryptor{
		encryption:    chromeCookieEncryptionGCM,
		key:           key,
		legacyDecrypt: dpapiUnprotect,
	}, nil
}

func findChromeCookiesDB() (string, error) {
	userDataDir, err := findChromeUserDataDir()
	if err != nil {
		return "", err
	}

	for _, path := range chromeCookieDBCandidates(userDataDir, "Default") {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	entries, err := os.ReadDir(userDataDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "Profile ") {
				continue
			}
			for _, path := range chromeCookieDBCandidates(userDataDir, entry.Name()) {
				if _, err := os.Stat(path); err == nil {
					return path, nil
				}
			}
		}
	}

	return "", fmt.Errorf("cookies: Chrome Cookies database not found")
}

func chromeCookieDBCandidates(userDataDir, profile string) []string {
	return []string{
		filepath.Join(userDataDir, profile, "Network", "Cookies"),
		filepath.Join(userDataDir, profile, "Cookies"),
	}
}

func findChromeLocalState() (string, error) {
	userDataDir, err := findChromeUserDataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(userDataDir, "Local State")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("cookies: Chrome Local State not found")
}

func findChromeUserDataDir() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return "", fmt.Errorf("cookies: LOCALAPPDATA is empty")
	}

	candidates := []string{
		filepath.Join(localAppData, "Google", "Chrome", "User Data"),
		filepath.Join(localAppData, "Google", "Chrome Beta", "User Data"),
		filepath.Join(localAppData, "Chromium", "User Data"),
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("cookies: Chrome user data directory not found")
}

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

func dpapiUnprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	in := dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
	var out dataBlob
	ret, _, callErr := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return nil, callErr
		}
		return nil, syscall.GetLastError()
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	decrypted := unsafe.Slice(out.pbData, out.cbData)
	result := make([]byte, len(decrypted))
	copy(result, decrypted)
	return result, nil
}
