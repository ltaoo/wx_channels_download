package services

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"wx_channel/pkg/cookies"
)

const cookie_cloud_crypto_aes128_cbc_fixed = "aes-128-cbc-fixed"

var (
	ErrCookieCloudInvalidRequest    = errors.New("CookieCloud request is invalid")
	ErrCookieCloudIncompleteRequest = errors.New("CookieCloud request is missing uuid or encrypted data")
	ErrCookieCloudUUIDMismatch      = errors.New("CookieCloud UUID does not match configuration")
	ErrCookieCloudKeyNotConfigured  = errors.New("CookieCloud decryption key is not configured")
	ErrCookieCloudInvalidConfig     = errors.New("CookieCloud decryption configuration is invalid")
	ErrCookieCloudDecryption        = errors.New("CookieCloud decryption failed")
	ErrCookieCloudInvalidData       = errors.New("CookieCloud data is invalid")
)

type CookieCloudConfig struct {
	UUID     string
	Password string
	Key      string
}

type CookieCloudUpdateResult struct {
	Cookies    []cookies.Cookie
	Skipped    int
	CryptoType string
}

type cookie_cloud_update_request struct {
	UUID       string          `json:"uuid"`
	Encrypted  string          `json:"encrypted"`
	CryptoType string          `json:"crypto_type"`
	Cookies    json.RawMessage `json:"cookies"`
	Data       json.RawMessage `json:"data"`
}

type cookie_cloud_sync_data struct {
	Cookies []cookie_cloud_entry `json:"cookies"`
}

type cookie_cloud_entry struct {
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	Domain         string   `json:"domain"`
	Path           string   `json:"path"`
	Secure         bool     `json:"secure"`
	HTTPOnly       bool     `json:"httpOnly"`
	SameSite       string   `json:"sameSite"`
	Session        bool     `json:"session"`
	ExpirationDate *float64 `json:"expirationDate"`
}

type cookie_cloud_identity struct {
	name   string
	domain string
	path   string
}

func ProcessCookieCloudUpdate(body []byte, cfg CookieCloudConfig) (CookieCloudUpdateResult, error) {
	var request cookie_cloud_update_request
	if err := json.Unmarshal(body, &request); err != nil {
		return CookieCloudUpdateResult{}, fmt.Errorf("%w: %v", ErrCookieCloudInvalidRequest, err)
	}
	request.UUID = strings.TrimSpace(request.UUID)
	request.Encrypted = strings.TrimSpace(request.Encrypted)

	plaintext := body
	crypto_type := normalize_cookie_cloud_crypto_type(request.CryptoType)
	if request.Data != nil {
		var data string
		if err := json.Unmarshal(request.Data, &data); err != nil || strings.TrimSpace(data) == "" {
			return CookieCloudUpdateResult{}, fmt.Errorf("%w: data must be a non-empty string", ErrCookieCloudInvalidData)
		}
		plaintext = []byte(data)
		request.Encrypted = strings.TrimSpace(data)
		if crypto_type == "" {
			crypto_type = "none"
		}
	} else if crypto_type == "" && request.Cookies != nil && request.Encrypted == "" {
		crypto_type = "none"
	}
	if crypto_type != "none" {
		if request.UUID == "" || request.Encrypted == "" {
			return CookieCloudUpdateResult{}, ErrCookieCloudIncompleteRequest
		}
		key, err := cookie_cloud_decryption_key(cfg, request.UUID)
		if err != nil {
			return CookieCloudUpdateResult{}, err
		}
		plaintext, err = decrypt_cookie_cloud(request.Encrypted, crypto_type, key)
		if err != nil {
			return CookieCloudUpdateResult{}, fmt.Errorf("%w: %v", ErrCookieCloudDecryption, err)
		}
	} else {
		crypto_type = "plaintext"
	}

	sync_data, err := parse_cookie_cloud_sync_data(plaintext)
	if err != nil {
		return CookieCloudUpdateResult{}, fmt.Errorf("%w: %v", ErrCookieCloudInvalidData, err)
	}
	cookie_list, skipped := flatten_cookie_cloud_data(sync_data)
	if len(sync_data.Cookies) > 0 && len(cookie_list) == 0 {
		return CookieCloudUpdateResult{}, fmt.Errorf("%w: no valid named cookies", ErrCookieCloudInvalidData)
	}
	return CookieCloudUpdateResult{
		Cookies:    cookie_list,
		Skipped:    skipped,
		CryptoType: crypto_type,
	}, nil
}

func cookie_cloud_decryption_key(cfg CookieCloudConfig, uuid string) ([]byte, error) {
	configured_uuid := strings.TrimSpace(cfg.UUID)
	if configured_uuid != "" && subtle.ConstantTimeCompare([]byte(configured_uuid), []byte(uuid)) != 1 {
		return nil, ErrCookieCloudUUIDMismatch
	}

	configured_key := strings.ToLower(strings.TrimSpace(cfg.Key))
	if configured_key != "" {
		if len(configured_key) != aes.BlockSize {
			return nil, fmt.Errorf("%w: cookie.key must contain exactly 16 hexadecimal characters", ErrCookieCloudInvalidConfig)
		}
		if _, err := hex.DecodeString(configured_key); err != nil {
			return nil, fmt.Errorf("%w: cookie.key must contain exactly 16 hexadecimal characters", ErrCookieCloudInvalidConfig)
		}
		return []byte(configured_key), nil
	}

	if cfg.Password == "" {
		return nil, ErrCookieCloudKeyNotConfigured
	}
	digest := md5.Sum([]byte(uuid + "-" + cfg.Password))
	derived_key := hex.EncodeToString(digest[:])[:aes.BlockSize]
	return []byte(derived_key), nil
}

func decrypt_cookie_cloud(encrypted string, crypto_type string, key []byte) ([]byte, error) {
	cipher_data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypted))
	if err != nil {
		return nil, fmt.Errorf("decode encrypted data: %w", err)
	}

	switch crypto_type {
	case cookie_cloud_crypto_aes128_cbc_fixed:
		return decrypt_cookie_cloud_aes_cbc(cipher_data, key, make([]byte, aes.BlockSize))
	case "aes-256-cbc", "legacy":
		if len(cipher_data) < 32 || (len(cipher_data)-16)%aes.BlockSize != 0 || string(cipher_data[:8]) != "Salted__" {
			return nil, fmt.Errorf("invalid OpenSSL ciphertext")
		}
		salt := cipher_data[8:16]
		key_iv := make([]byte, 0, 48)
		var previous []byte
		// CryptoJS/OpenSSL EVP_BytesToKey derives a 32-byte key and a 16-byte IV.
		for len(key_iv) < 48 {
			digest := md5.New()
			digest.Write(previous)
			digest.Write(key)
			digest.Write(salt)
			previous = digest.Sum(nil)
			key_iv = append(key_iv, previous...)
		}
		return decrypt_cookie_cloud_aes_cbc(cipher_data[16:], key_iv[:32], key_iv[32:])
	default:
		return nil, fmt.Errorf("unsupported crypto_type %q", crypto_type)
	}
}

func normalize_cookie_cloud_crypto_type(crypto_type string) string {
	return strings.ToLower(strings.TrimSpace(crypto_type))
}

func decrypt_cookie_cloud_aes_cbc(cipher_data []byte, key []byte, iv []byte) ([]byte, error) {
	if len(cipher_data) == 0 || len(cipher_data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("AES-CBC ciphertext has an invalid length")
	}
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("AES-CBC IV has an invalid length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	plaintext := make([]byte, len(cipher_data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, cipher_data)
	return cookie_cloud_pkcs7_unpad(plaintext)
}

func cookie_cloud_pkcs7_unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("PKCS#7 plaintext is empty")
	}
	padding_length := int(data[len(data)-1])
	if padding_length == 0 || padding_length > aes.BlockSize || padding_length > len(data) {
		return nil, fmt.Errorf("PKCS#7 padding is invalid")
	}
	for _, value := range data[len(data)-padding_length:] {
		if int(value) != padding_length {
			return nil, fmt.Errorf("PKCS#7 padding is invalid")
		}
	}
	return data[:len(data)-padding_length], nil
}

func parse_cookie_cloud_sync_data(plaintext []byte) (cookie_cloud_sync_data, error) {
	var sync_data cookie_cloud_sync_data
	var target interface{} = &sync_data
	plaintext = bytes.TrimSpace(plaintext)
	if len(plaintext) > 0 && plaintext[0] == '[' {
		target = &sync_data.Cookies
	}
	if err := json.Unmarshal(plaintext, target); err != nil {
		return cookie_cloud_sync_data{}, fmt.Errorf("parse decrypted CookieCloud data: %w", err)
	}
	if sync_data.Cookies == nil {
		return cookie_cloud_sync_data{}, fmt.Errorf("cookies must be an array")
	}
	return sync_data, nil
}

func flatten_cookie_cloud_data(sync_data cookie_cloud_sync_data) ([]cookies.Cookie, int) {
	cookie_list := make([]cookies.Cookie, 0)
	identities := make(map[cookie_cloud_identity]struct{})
	skipped := 0
	for _, entry := range sync_data.Cookies {
		cookie, ok := cookie_cloud_entry_to_cookie(entry)
		if !ok {
			skipped++
			continue
		}
		identity := cookie_cloud_identity{
			name:   cookie.Name,
			domain: strings.ToLower(cookie.Domain),
			path:   cookie.Path,
		}
		if _, exists := identities[identity]; exists {
			skipped++
			continue
		}
		identities[identity] = struct{}{}
		cookie_list = append(cookie_list, cookie)
	}

	sort.Slice(cookie_list, func(left_index, right_index int) bool {
		left := cookie_list[left_index]
		right := cookie_list[right_index]
		if left.Domain != right.Domain {
			return left.Domain < right.Domain
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Name < right.Name
	})
	return cookie_list, skipped
}

func cookie_cloud_entry_to_cookie(entry cookie_cloud_entry) (cookies.Cookie, bool) {
	domain := strings.TrimSpace(entry.Domain)
	if entry.Name == "" || domain == "" {
		return cookies.Cookie{}, false
	}

	path := entry.Path
	if path == "" {
		path = "/"
	}
	expires := int64(-1)
	if !entry.Session && entry.ExpirationDate != nil {
		expiration_date := *entry.ExpirationDate
		if math.IsNaN(expiration_date) || math.IsInf(expiration_date, 0) || expiration_date > float64(math.MaxInt64) {
			return cookies.Cookie{}, false
		}
		if expiration_date > 0 {
			expires = int64(math.Floor(expiration_date))
		}
	}

	return cookies.Cookie{
		Name:     entry.Name,
		Value:    entry.Value,
		Domain:   domain,
		Path:     path,
		Secure:   entry.Secure,
		HTTPOnly: entry.HTTPOnly,
		SameSite: normalize_cookie_cloud_same_site(entry.SameSite),
		Expires:  expires,
	}, true
}

func normalize_cookie_cloud_same_site(same_site string) string {
	switch strings.ToLower(strings.TrimSpace(same_site)) {
	case "none", "no_restriction":
		return "None"
	case "lax":
		return "Lax"
	case "strict":
		return "Strict"
	default:
		return ""
	}
}
