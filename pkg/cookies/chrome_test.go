package cookies

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFilterByDomain(t *testing.T) {
	cookieList := []Cookie{
		{Name: "exact", Domain: "example.com"},
		{Name: "leading-dot", Domain: ".EXAMPLE.com"},
		{Name: "subdomain", Domain: "www.example.com"},
		{Name: "other", Domain: "example.org"},
	}

	got := filterByDomain(cookieList, " .Example.COM ")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "exact" || got[1].Name != "leading-dot" {
		t.Fatalf("cookies = %#v, want exact domain matches", got)
	}
}

func TestSaveJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "cookies.json")
	want := []Cookie{{Name: "session", Value: "secret", Domain: "example.com", Path: "/"}}
	if err := saveJSON(want, path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got []Cookie
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("cookies = %#v, want %#v", got, want)
	}
}

func TestChromeCookieDecryptorGCM(t *testing.T) {
	key := []byte("0123456789abcdef")
	nonce := []byte("0123456789ab")
	hostKey := ".example.com"
	value := []byte("cookie-value")
	hostHash := sha256.Sum256([]byte(hostKey))
	plaintext := append(hostHash[:], value...)

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	encrypted := append([]byte("v10"), nonce...)
	encrypted = gcm.Seal(encrypted, nonce, plaintext, nil)

	decryptor := chromeCookieDecryptor{encryption: chromeCookieEncryptionGCM, key: key}
	got, ok := decryptor.decrypt("", encrypted, hostKey, 24)
	if !ok {
		t.Fatal("decrypt returned false")
	}
	if got != string(value) {
		t.Fatalf("value = %q, want %q", got, value)
	}
}

func TestChromeCookieDecryptorPlainAndLegacyValues(t *testing.T) {
	decryptor := chromeCookieDecryptor{
		legacyDecrypt: func(value []byte) ([]byte, error) {
			return append([]byte("decoded-"), value...), nil
		},
	}

	if got, ok := decryptor.decrypt("plain", nil, "example.com", 0); !ok || got != "plain" {
		t.Fatalf("plain value = %q, %v", got, ok)
	}
	if got, ok := decryptor.decrypt("", []byte("legacy"), "example.com", 0); !ok || got != "decoded-legacy" {
		t.Fatalf("legacy value = %q, %v", got, ok)
	}
}

func TestChromeExpiresToUnix(t *testing.T) {
	expiresUTC := int64(chromeEpochOffset+123) * 1000000
	if got := chromeExpiresToUnix(expiresUTC); got != 123 {
		t.Fatalf("expires = %d, want 123", got)
	}
	if got := chromeExpiresToUnix(0); got != -1 {
		t.Fatalf("session expires = %d, want -1", got)
	}
}
