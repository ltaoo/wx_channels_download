package cookies

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/glebarez/sqlite"
)

const chromeEpochOffset = 11644473600

var chromeCBCIV = []byte("                ") // 16 spaces

// ChromeImportOptions controls a Chrome cookie import.
type ChromeImportOptions struct {
	// Domain limits the saved and returned cookies to an exact domain match.
	// A leading dot is ignored. An empty value imports every domain.
	Domain string
	// OutputPath is the optional JSON file to write after importing.
	OutputPath string
}

// ChromeImportResult describes a completed Chrome cookie import.
type ChromeImportResult struct {
	Cookies []Cookie
	Loaded  int
	Skipped int
}

// ImportChrome extracts cookies from the current user's Chrome profile. All
// browser location, decryption, and operating-system behavior is encapsulated
// by this package.
func ImportChrome(options ChromeImportOptions) (ChromeImportResult, error) {
	chromeCookies, skipped, err := extractChromeCookies()
	if err != nil {
		return ChromeImportResult{}, err
	}

	result := ChromeImportResult{
		Cookies: chromeCookies,
		Loaded:  len(chromeCookies),
		Skipped: skipped,
	}
	if options.Domain != "" {
		result.Cookies = filterByDomain(result.Cookies, options.Domain)
	}
	if options.OutputPath != "" {
		if err := SaveJSON(result.Cookies, options.OutputPath); err != nil {
			return ChromeImportResult{}, err
		}
	}

	return result, nil
}

func extractChromeCookies() ([]Cookie, int, error) {
	dbPath, err := findChromeCookiesDB()
	if err != nil {
		return nil, 0, err
	}

	decryptor, err := newChromeCookieDecryptor()
	if err != nil {
		return nil, 0, err
	}

	// Chrome may keep the live database and its WAL locked. Work from a copied
	// snapshot so importing does not interfere with a running browser.
	tmpDir, err := os.MkdirTemp("", "obscura-chrome-cookies")
	if err != nil {
		return nil, 0, fmt.Errorf("cookies: create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpDB := filepath.Join(tmpDir, "Cookies")
	if err := copyChromeCookiesDB(dbPath, tmpDB); err != nil {
		return nil, 0, fmt.Errorf("cookies: copy Chrome database: %w", err)
	}

	db, err := sql.Open("sqlite", tmpDB+"?mode=ro")
	if err != nil {
		return nil, 0, fmt.Errorf("cookies: open Chrome database: %w", err)
	}
	defer db.Close()

	var dbVersion int
	if err := db.QueryRow("SELECT value FROM meta WHERE key = 'version'").Scan(&dbVersion); err != nil {
		dbVersion = 0
	}

	rows, err := db.Query(
		"SELECT host_key, name, value, encrypted_value, path, expires_utc, " +
			"is_secure, is_httponly, samesite FROM cookies",
	)
	if err != nil {
		return nil, 0, fmt.Errorf("cookies: query Chrome database: %w", err)
	}
	defer rows.Close()

	var chromeCookies []Cookie
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

		cookieValue, ok := decryptor.decrypt(value, encryptedValue, hostKey, dbVersion)
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

		chromeCookies = append(chromeCookies, Cookie{
			Name:     name,
			Value:    cookieValue,
			Domain:   domain,
			Path:     path,
			Secure:   isSecure,
			HTTPOnly: isHTTPOnly,
			SameSite: chromeSameSiteValue(sameSite),
			Expires:  chromeExpiresToUnix(expiresUTC),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, skipped, fmt.Errorf("cookies: read Chrome database: %w", err)
	}

	return chromeCookies, skipped, nil
}

type chromeCookieEncryption int

const (
	chromeCookieEncryptionCBC chromeCookieEncryption = iota
	chromeCookieEncryptionGCM
)

type chromeCookieDecryptor struct {
	encryption    chromeCookieEncryption
	key           []byte
	legacyDecrypt func([]byte) ([]byte, error)
}

func (d chromeCookieDecryptor) decrypt(
	plainValue string,
	encryptedValue []byte,
	hostKey string,
	dbVersion int,
) (string, bool) {
	if plainValue != "" {
		return plainValue, true
	}
	if len(encryptedValue) == 0 {
		return "", false
	}
	if !strings.HasPrefix(string(encryptedValue), "v10") &&
		!strings.HasPrefix(string(encryptedValue), "v11") {
		if d.legacyDecrypt != nil {
			plaintext, err := d.legacyDecrypt(encryptedValue)
			if err == nil {
				return string(plaintext), true
			}
			return "", false
		}
		return string(encryptedValue), true
	}
	if len(d.key) == 0 || len(encryptedValue) < 3 {
		return "", false
	}
	if d.encryption == chromeCookieEncryptionGCM {
		return d.decryptGCM(encryptedValue, hostKey, dbVersion)
	}
	return d.decryptCBC(encryptedValue[3:], hostKey, dbVersion)
}

func (d chromeCookieDecryptor) decryptCBC(ciphertext []byte, hostKey string, dbVersion int) (string, bool) {
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", false
	}

	block, err := aes.NewCipher(d.key)
	if err != nil {
		return "", false
	}

	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, chromeCBCIV).CryptBlocks(plaintext, ciphertext)
	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return "", false
	}
	plaintext = stripHostHash(plaintext, hostKey, dbVersion)
	return string(plaintext), true
}

func (d chromeCookieDecryptor) decryptGCM(encryptedValue []byte, hostKey string, dbVersion int) (string, bool) {
	const (
		prefixLen = 3
		nonceLen  = 12
		tagLen    = 16
	)
	if len(encryptedValue) <= prefixLen+nonceLen+tagLen {
		return "", false
	}

	block, err := aes.NewCipher(d.key)
	if err != nil {
		return "", false
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", false
	}

	nonce := encryptedValue[prefixLen : prefixLen+nonceLen]
	plaintext, err := gcm.Open(nil, nonce, encryptedValue[prefixLen+nonceLen:], nil)
	if err != nil {
		return "", false
	}
	plaintext = stripHostHash(plaintext, hostKey, dbVersion)
	return string(plaintext), true
}

func stripHostHash(plaintext []byte, hostKey string, dbVersion int) []byte {
	if dbVersion < 24 {
		return plaintext
	}
	hash := sha256.Sum256([]byte(hostKey))
	if bytesHasPrefix(plaintext, hash[:]) {
		return plaintext[len(hash):]
	}
	return plaintext
}

func chromeExpiresToUnix(expiresUTC int64) int64 {
	if expiresUTC <= 0 {
		return -1
	}
	return expiresUTC/1000000 - chromeEpochOffset
}

func chromeSameSiteValue(value int64) string {
	switch value {
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

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > aes.BlockSize || padLen > len(data) {
		return nil, fmt.Errorf("invalid padding length: %d", padLen)
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, fmt.Errorf("invalid padding byte at %d", i)
		}
	}
	return data[:len(data)-padLen], nil
}

func bytesHasPrefix(data, prefix []byte) bool {
	if len(data) < len(prefix) {
		return false
	}
	for i, value := range prefix {
		if data[i] != value {
			return false
		}
	}
	return true
}

func copyChromeCookiesDB(src, dst string) error {
	if err := copyFile(src, dst); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecarSrc := src + suffix
		if _, err := os.Stat(sidecarSrc); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat %s: %w", filepath.Base(sidecarSrc), err)
		}
		if err := copyFile(sidecarSrc, dst+suffix); err != nil {
			return fmt.Errorf("copy %s: %w", filepath.Base(sidecarSrc), err)
		}
	}
	return nil
}

func filterByDomain(cookieList []Cookie, domain string) []Cookie {
	domain = normalizeDomain(domain)
	filtered := make([]Cookie, 0, len(cookieList))
	for _, cookie := range cookieList {
		if normalizeDomain(cookie.Domain) == domain {
			filtered = append(filtered, cookie)
		}
	}
	return filtered
}

func normalizeDomain(domain string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
}
