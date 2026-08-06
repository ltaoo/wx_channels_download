//go:build darwin

package cookies

import (
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	chromeKeySalt       = "saltysalt"
	chromeKeyIterations = 1003
)

func newChromeCookieDecryptor() (*chromeCookieDecryptor, error) {
	password, err := readChromeSafeStoragePassword()
	if err != nil {
		return nil, fmt.Errorf("cookies: read Chrome Safe Storage password: %w", err)
	}
	if password == "" {
		return nil, fmt.Errorf("cookies: Chrome Safe Storage password not found")
	}
	key := pbkdf2.Key([]byte(password), []byte(chromeKeySalt), chromeKeyIterations, 16, sha1.New)
	return &chromeCookieDecryptor{encryption: chromeCookieEncryptionCBC, key: key}, nil
}

func readChromeSafeStoragePassword() (string, error) {
	if out, err := exec.Command(
		"security", "find-generic-password", "-w", "-s", "Chrome Safe Storage",
	).Output(); err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	command := `security find-generic-password -w -s \"Chrome Safe Storage\"`
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, command)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("administrator prompt failed: %w (output: %s)", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func findChromeCookiesDB() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cookies: get home directory: %w", err)
	}

	candidates := []string{
		filepath.Join(home, "Library/Application Support/Google/Chrome/Default/Network/Cookies"),
		filepath.Join(home, "Library/Application Support/Google/Chrome/Default/Cookies"),
		filepath.Join(home, "Library/Application Support/Google/Chrome/Network/Cookies"),
		filepath.Join(home, "Library/Application Support/Google/Chrome/Cookies"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("cookies: Chrome Cookies database not found")
}
