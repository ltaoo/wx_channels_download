package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/glebarez/sqlite"
	"golang.org/x/crypto/pbkdf2"

	result "wx_channel/internal/util"
	"wx_channel/pkg/cookies"
)

const (
	chromeKeySalt       = "saltysalt"
	chromeKeyIterations = 1003
	chromeEpochOffset   = 11644473600
)

var chromeCBCIV = []byte("                ") // 16 spaces

// handleCookieExtract escalates privileges, extracts Chrome cookies,
// saves them to workdir/cookies.json, and returns them to the caller.
func (c *APIClient) handleCookieExtract(ctx *gin.Context) {
	// Step 1: Privilege escalation – read Chrome Safe Storage password from Keychain.
	// This triggers a macOS administrator privileges dialog via osascript.
	password, err := readChromePasswordWithEscalation()
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to read Chrome Safe Storage password")
		result.Err(ctx, 500, "提权获取Keychain密码失败: "+err.Error())
		return
	}
	if password == "" {
		result.Err(ctx, 500, "未找到Chrome Safe Storage密码，请确认Chrome已安装并存储过密码")
		return
	}

	// Step 2: Read and decrypt Chrome cookies from the SQLite database.
	chromeCookies, importInfo, err := extractChromeCookies(password)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to extract Chrome cookies")
		result.Err(ctx, 500, "提取Chrome Cookie失败: "+err.Error())
		return
	}

	// Step 3: Filter by domain if specified.
	if domain := ctx.Query("domain"); domain != "" {
		chromeCookies = filterCookiesByDomain(chromeCookies, domain)
	}

	// Step 4: Save to cookies.json under workdir.
	cookiePath := filepath.Join(c.cfg.WorkDir, "cookies.json")
	if err := saveCookiesToJSON(chromeCookies, cookiePath); err != nil {
		c.logger.Error().Err(err).Msg("failed to save cookies.json")
		result.Err(ctx, 500, "保存Cookie文件失败: "+err.Error())
		return
	}

	c.logger.Info().
		Int("loaded", importInfo.loaded).
		Int("skipped", importInfo.skipped).
		Str("path", cookiePath).
		Msg("Chrome cookies extracted and saved")

	// Step 5: Return cookies to the caller.
	result.Ok(ctx, gin.H{
		"count":   len(chromeCookies),
		"skipped": importInfo.skipped,
		"path":    cookiePath,
		"cookies": chromeCookies,
	})
}

// readChromePasswordWithEscalation uses osascript with administrator
// privileges to read the Chrome Safe Storage password from the macOS Keychain.
// This follows the same privilege escalation pattern used by
// pkg/certificate/certificate_darwin.go.
func readChromePasswordWithEscalation() (string, error) {
	// Try reading the password directly first (already authorized).
	if out, err := exec.Command(
		"security", "find-generic-password", "-w", "-s", "Chrome Safe Storage",
	).Output(); err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	// Not authorized – escalate via osascript (triggers GUI prompt).
	escapedCommand := `security find-generic-password -w -s \"Chrome Safe Storage\"`
	script := fmt.Sprintf(
		`do shell script "%s" with administrator privileges`,
		escapedCommand,
	)

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("osascript escalation failed: %w (output: %s)", err, string(out))
	}

	password := strings.TrimSpace(string(out))
	return password, nil
}

// extractChromeCookies reads the Chrome Cookies SQLite database, derives the
// AES decryption key from the given password, and returns decoded cookies.
func extractChromeCookies(password string) ([]cookies.Cookie, *chromeImportInfo, error) {
	dbPath, err := findChromeCookiesDB()
	if err != nil {
		return nil, nil, err
	}

	// Copy the database to a temp directory to avoid WAL lock conflicts
	// with a running Chrome process.
	tmpDir, err := os.MkdirTemp("", "obscura-chrome-cookies")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpDB := filepath.Join(tmpDir, "Cookies")
	if err := copyFile(dbPath, tmpDB); err != nil {
		return nil, nil, fmt.Errorf("copy cookie db: %w", err)
	}

	db, err := sql.Open("sqlite", tmpDB+"?mode=ro")
	if err != nil {
		return nil, nil, fmt.Errorf("open cookie db: %w", err)
	}
	defer db.Close()

	// Derive the AES-128 key using PBKDF2-HMAC-SHA1.
	key := pbkdf2.Key([]byte(password), []byte(chromeKeySalt), chromeKeyIterations, 16, sha1.New)

	// Read the database version to decide if host_key hash stripping is needed.
	var dbVersion int
	row := db.QueryRow("SELECT value FROM meta WHERE key = 'version'")
	if err := row.Scan(&dbVersion); err != nil {
		dbVersion = 0 // pre-v24 database
	}

	rows, err := db.Query(
		"SELECT host_key, name, value, encrypted_value, path, expires_utc, " +
			"is_secure, is_httponly, samesite FROM cookies",
	)
	if err != nil {
		return nil, nil, fmt.Errorf("query cookies: %w", err)
	}
	defer rows.Close()

	var chromeCookies []cookies.Cookie
	skipped := 0

	for rows.Next() {
		var hostKey, name, value, path string
		var encryptedValue []byte
		var expiresUTC int64
		var isSecure, isHTTPOnly bool
		var sameSite int64

		if err := rows.Scan(
			&hostKey, &name, &value, &encryptedValue, &path,
			&expiresUTC, &isSecure, &isHTTPOnly, &sameSite,
		); err != nil {
			skipped++
			continue
		}

		cookieValue, ok := decryptChromeCookieValue(value, encryptedValue, key, hostKey, dbVersion)
		if !ok {
			skipped++
			continue
		}

		domain := strings.TrimPrefix(hostKey, ".")
		if domain == "" || name == "" {
			skipped++
			continue
		}

		if path == "" {
			path = "/"
		}

		expires := chromeExpiresToUnix(expiresUTC)
		sameSiteStr := chromeSameSiteValue(sameSite)

		chromeCookies = append(chromeCookies, cookies.Cookie{
			Name:     name,
			Value:    cookieValue,
			Domain:   domain,
			Path:     path,
			Secure:   isSecure,
			HTTPOnly: isHTTPOnly,
			SameSite: sameSiteStr,
			Expires:  expires,
		})
	}

	info := &chromeImportInfo{
		loaded:  len(chromeCookies),
		skipped: skipped,
	}

	return chromeCookies, info, nil
}

// decryptChromeCookieValue decrypts a single cookie's encrypted value.
// Returns the plaintext value and whether decryption succeeded.
func decryptChromeCookieValue(
	plainValue string,
	encryptedValue []byte,
	key []byte,
	hostKey string,
	dbVersion int,
) (string, bool) {
	// If there's a plaintext value, use it directly.
	if plainValue != "" {
		return plainValue, true
	}

	if len(encryptedValue) == 0 {
		return "", false
	}

	// Only handle v10/v11 encrypted values.
	if !strings.HasPrefix(string(encryptedValue), "v10") &&
		!strings.HasPrefix(string(encryptedValue), "v11") {
		return string(encryptedValue), true
	}

	if len(key) == 0 || len(encryptedValue) < 3 {
		return "", false
	}

	ciphertext := encryptedValue[3:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", false
	}

	mode := cipher.NewCBCDecrypter(block, chromeCBCIV)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding.
	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return "", false
	}

	// For Chrome v24+, strip the SHA-256 host_key prefix.
	if dbVersion >= 24 {
		hash := sha256.Sum256([]byte(hostKey))
		if bytesHasPrefix(plaintext, hash[:]) {
			plaintext = plaintext[len(hash):]
		}
	}

	return string(plaintext), true
}

// findChromeCookiesDB locates the Chrome Cookies database file.
func findChromeCookiesDB() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	candidates := []string{
		filepath.Join(home, "Library/Application Support/Google/Chrome/Default/Cookies"),
		filepath.Join(home, "Library/Application Support/Google/Chrome/Default/Network/Cookies"),
		filepath.Join(home, "Library/Application Support/Google/Chrome/Cookies"),
		filepath.Join(home, "Library/Application Support/Google/Chrome/Network/Cookies"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到Chrome Cookies数据库，请确认Chrome已安装")
}

// saveCookiesToJSON writes cookies to a JSON file.
func saveCookiesToJSON(ck []cookies.Cookie, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	data, err := json.MarshalIndent(ck, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

// chromeExpiresToUnix converts Chrome's WebKit microsecond timestamp to Unix seconds.
func chromeExpiresToUnix(expiresUTC int64) int64 {
	if expiresUTC <= 0 {
		return -1
	}
	return expiresUTC/1000000 - chromeEpochOffset
}

// chromeSameSiteValue maps Chrome's integer SameSite value to string.
func chromeSameSiteValue(val int64) string {
	switch val {
	case 0:
		return "None"
	case 1:
		return "Lax"
	case 2:
		return "Strict"
	default:
		return "Lax"
	}
}

// pkcs7Unpad removes PKCS7 padding from the decrypted plaintext.
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > aes.BlockSize || padLen > len(data) {
		return nil, fmt.Errorf("invalid padding length: %d", padLen)
	}
	// Verify all padding bytes are equal to padLen.
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, fmt.Errorf("invalid padding byte at %d", i)
		}
	}
	return data[:len(data)-padLen], nil
}

// bytesHasPrefix is a byte-level HasPrefix.
func bytesHasPrefix(b, prefix []byte) bool {
	if len(b) < len(prefix) {
		return false
	}
	for i, v := range prefix {
		if b[i] != v {
			return false
		}
	}
	return true
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

// filterCookiesByDomain filters the cookie list to only include cookies
// whose domain matches the target domain. Matching is case-insensitive
// and supports both exact match and suffix match (leading dot).
func filterCookiesByDomain(cks []cookies.Cookie, domain string) []cookies.Cookie {
	domain = strings.ToLower(domain)
	var filtered []cookies.Cookie
	for _, ck := range cks {
		ckDomain := strings.ToLower(ck.Domain)
		if ckDomain == domain || ckDomain == "."+domain {
			filtered = append(filtered, ck)
		}
	}
	return filtered
}

type chromeImportInfo struct {
	loaded  int
	skipped int
}
