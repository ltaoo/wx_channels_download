package services

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"wx_channel/pkg/cookies"
)

// Generated independently with Node's AES-128-CBC, a zero IV and PKCS#7 padding.
const cookie_update_encrypted_fixture = "yNCP7ZYYy9cD9I/SLB6wRcCcBALpUdGVT2VBRiQbLIj1I0Axi62BvDorUaDTKk7HIzqtWO1j7W16mbuHY4mGm2mVfkw+wKJI7DGCZtC5jdQ="

// Generated independently with Node's AES-256-CBC and OpenSSL EVP_BytesToKey.
const cookie_update_salted_fixture = "U2FsdGVkX18xMjM0NTY3OOLzETzn4kr7wHTt2ZCV8+pAPblACCh83mVLS6xsqySPULtrwlDYu6vMDkXx0ulFOmQUx02YwzQ+mrC8XQNuuBTvsT0rqiRS6Ni1Q15B8A6d"

func TestCookieUpdatePlaintext(t *testing.T) {
	for _, body := range []string{
		`{"cookies":[{"name":"sid","value":"fixture","domain":".example.com"}]}`,
		`{"uuid":"device","crypto_type":"none","cookies":[{"name":"sid","value":"fixture","domain":".example.com"}]}`,
		`{"uuid":"device","cookies":[{"name":"sid","value":"fixture","domain":".example.com"}]}`,
		`{"data":"{\"cookies\":[{\"name\":\"sid\",\"value\":\"fixture\",\"domain\":\".example.com\"}]}"}`,
		`{"crypto_type":" NONE ","data":"{\"cookies\":[{\"name\":\"sid\",\"value\":\"fixture\",\"domain\":\".example.com\"}]}"}`,
		`{"crypto_type":"none","data":"[{\"name\":\"sid\",\"value\":\"fixture\",\"domain\":\".example.com\"}]","update_time":"2026-10-19T07:27:06.611Z"}`,
		`{"data":" \n[{\"name\":\"sid\",\"value\":\"fixture\",\"domain\":\".example.com\"}] "}`,
		`{"cookies":[{"name":"sid","value":"fixture","domain":".example.com"}],"local_storage_data":"ignored"}`,
	} {
		result, err := ProcessCookieCloudUpdate([]byte(body), CookieCloudConfig{})
		if err != nil {
			t.Fatal(err)
		}
		want := []cookies.Cookie{{Name: "sid", Value: "fixture", Domain: ".example.com", Path: "/", Expires: -1}}
		if !reflect.DeepEqual(result.Cookies, want) || result.Skipped != 0 {
			t.Fatalf("unexpected result: %+v", result)
		}
	}
	for _, body := range []string{`{"cookies":[]}`, `{"crypto_type":"none","data":"[]","update_time":"2026-10-19T07:27:06.611Z"}`} {
		result, err := ProcessCookieCloudUpdate([]byte(body), CookieCloudConfig{})
		if err != nil || result.Cookies == nil || len(result.Cookies) != 0 {
			t.Fatalf("empty array: result=%+v err=%v", result, err)
		}
	}
}

func TestCookieUpdateEncryption(t *testing.T) {
	cases := []struct {
		name        string
		uuid        string
		crypto_type string
		encrypted   string
		cfg         CookieCloudConfig
		want_error  error
	}{
		{name: "AES array", uuid: "device", crypto_type: "aes-128-cbc-fixed", encrypted: cookie_update_encrypted_fixture, cfg: CookieCloudConfig{UUID: "device", Key: "0123456789abcdef"}},
		{name: "AES 128 JSON array", uuid: "device", crypto_type: "aes-128-cbc-fixed", encrypted: "e8CCWbBoAURuuGp3brhZCfHROMdAfC9Ukei94H/uTWW1FoJrWCajcTjREpGranfo04zR/xObJebefgH6y9mwWg==", cfg: CookieCloudConfig{UUID: "device", Key: "0123456789abcdef"}},
		{name: "AES 256 JSON array", uuid: "device", crypto_type: "aes-256-cbc", encrypted: "U2FsdGVkX18xMjM0NTY3OPUTyVlw6qAYf4+K09W9vgZjB4tBmpHFGITk4Ll4WipDqwacejzu7vm5ilryxSh8n026u6tI2HiDmDML4aj9PY8=", cfg: CookieCloudConfig{UUID: "device", Key: "0123456789abcdef"}},
		{name: "legacy JSON array", uuid: "device", crypto_type: "legacy", encrypted: "U2FsdGVkX18xMjM0NTY3OPUTyVlw6qAYf4+K09W9vgZjB4tBmpHFGITk4Ll4WipDqwacejzu7vm5ilryxSh8n026u6tI2HiDmDML4aj9PY8=", cfg: CookieCloudConfig{UUID: "device", Key: "0123456789abcdef"}},
		{name: "normalized key", uuid: " device ", crypto_type: " AES-128-CBC-FIXED ", encrypted: cookie_update_encrypted_fixture, cfg: CookieCloudConfig{UUID: "device", Key: "0123456789ABCDEF"}},
		{name: "password derived key", uuid: "device", crypto_type: "aes-128-cbc-fixed", encrypted: "n5VzW9UtNvKEhRUtO3vGY6wMyYYIZx9RH3vp5lE+m4LYGF/smRyPDylEour/r33q7UIzJoKql8TCHLNAfYDiUR6ayyTpvryRe266mHMX/10=", cfg: CookieCloudConfig{UUID: "device", Password: "password"}},
		{name: "AES 256", uuid: "device", crypto_type: "aes-256-cbc", encrypted: cookie_update_salted_fixture, cfg: CookieCloudConfig{UUID: "device", Key: "0123456789abcdef"}},
		{name: "legacy", uuid: "device", crypto_type: "legacy", encrypted: cookie_update_salted_fixture, cfg: CookieCloudConfig{UUID: "device", Key: "0123456789abcdef"}},
		{name: "second salt", uuid: "device", crypto_type: " AES-256-CBC ", encrypted: "U2FsdGVkX19mZWRjYmE5OKnTfgcBXm8VDN/CMjmblPMAaWql4vld9uS+F0hf3DaLXwiz7Y9/RtOOwIKeOkHTHMerF3yBcOt8zqhoRYYDy3pDHEQXQF7WuwZRAdUV80GC", cfg: CookieCloudConfig{UUID: "device", Key: "0123456789ABCDEF"}},
		{name: "UUID mismatch", uuid: "wrong", crypto_type: "aes-128-cbc-fixed", encrypted: cookie_update_encrypted_fixture, cfg: CookieCloudConfig{UUID: "device", Key: "0123456789abcdef"}, want_error: ErrCookieCloudUUIDMismatch},
		{name: "missing UUID", crypto_type: "aes-128-cbc-fixed", encrypted: cookie_update_encrypted_fixture, want_error: ErrCookieCloudIncompleteRequest},
		{name: "missing key", uuid: "device", crypto_type: "aes-128-cbc-fixed", encrypted: cookie_update_encrypted_fixture, want_error: ErrCookieCloudKeyNotConfigured},
		{name: "invalid key", uuid: "device", crypto_type: "aes-128-cbc-fixed", encrypted: cookie_update_encrypted_fixture, cfg: CookieCloudConfig{Key: "short"}, want_error: ErrCookieCloudInvalidConfig},
		{name: "wrong key", uuid: "device", crypto_type: "aes-128-cbc-fixed", encrypted: cookie_update_encrypted_fixture, cfg: CookieCloudConfig{Key: "fedcba9876543210"}, want_error: ErrCookieCloudDecryption},
		{name: "invalid base64", uuid: "device", crypto_type: "aes-128-cbc-fixed", encrypted: "!invalid!", cfg: CookieCloudConfig{Key: "0123456789abcdef"}, want_error: ErrCookieCloudDecryption},
		{name: "invalid ciphertext length", uuid: "device", crypto_type: "aes-128-cbc-fixed", encrypted: "YWJj", cfg: CookieCloudConfig{Key: "0123456789abcdef"}, want_error: ErrCookieCloudDecryption},
		{name: "legacy rejected", uuid: "device", crypto_type: "legacy", encrypted: cookie_update_encrypted_fixture, cfg: CookieCloudConfig{Key: "0123456789abcdef"}, want_error: ErrCookieCloudDecryption},
		{name: "wrong salted key", uuid: "device", crypto_type: "aes-256-cbc", encrypted: cookie_update_salted_fixture, cfg: CookieCloudConfig{Key: "fedcba9876543210"}, want_error: ErrCookieCloudDecryption},
		{name: "truncated salted data", uuid: "device", crypto_type: "aes-256-cbc", encrypted: "U2FsdGVkX18xMjM0NTY3OA==", cfg: CookieCloudConfig{Key: "0123456789abcdef"}, want_error: ErrCookieCloudDecryption},
		{name: "unknown crypto", uuid: "device", crypto_type: "unknown", encrypted: cookie_update_encrypted_fixture, cfg: CookieCloudConfig{Key: "0123456789abcdef"}, want_error: ErrCookieCloudDecryption},
		{name: "missing crypto type", uuid: "device", encrypted: cookie_update_encrypted_fixture, cfg: CookieCloudConfig{Key: "0123456789abcdef"}, want_error: ErrCookieCloudDecryption},
		// This fixture decrypts to the former grouped cookie_data object.
		{name: "encrypted old format rejected", uuid: "device", crypto_type: "aes-128-cbc-fixed", encrypted: "79KRXdz8Bs623ukjpd6gkuLVh8ixAoCe6sSHsx67pvtn0nEy8QHntLfxup5aYAmBP2B14Ayaku8ZoLVZwEacfhV9B/1HUshfT4YnS27gw4Q=", cfg: CookieCloudConfig{Key: "0123456789abcdef"}, want_error: ErrCookieCloudInvalidData},
	}
	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			for _, field := range []string{"data", "encrypted"} {
				// A missing crypto_type defaults to plaintext only for the data envelope.
				if test_case.name == "missing crypto type" && field == "data" {
					continue
				}
				body, err := json.Marshal(map[string]string{"uuid": test_case.uuid, "crypto_type": test_case.crypto_type, field: test_case.encrypted})
				if err != nil {
					t.Fatal(err)
				}
				result, err := ProcessCookieCloudUpdate(body, test_case.cfg)
				if !errors.Is(err, test_case.want_error) {
					t.Fatalf("error=%v, want %v", err, test_case.want_error)
				}
				if err == nil && (len(result.Cookies) != 1 || result.Cookies[0].Value != "fixture" || result.CryptoType != normalize_cookie_cloud_crypto_type(test_case.crypto_type)) {
					t.Fatalf("unexpected decrypted result: %+v", result)
				}
			}
		})
	}
}

func TestCookieUpdateRejectsInvalidArrays(t *testing.T) {
	for _, body := range []string{
		`{"crypto_type":"none","cookie_data":{"example.com":[]}}`,
		`{"crypto_type":"none"}`,
		`{"cookies":null}`,
		`{"cookies":{}}`,
		`{"cookies":"invalid"}`,
		`{"cookies":[1]}`,
		`{"data":null}`,
		`{"data":[]}`,
		`{"data":" "}`,
		`{"data":"{"}`,
		`{"data":"null"}`,
		`{"data":"[1]"}`,
		`{"data":"[{\"name\":\"\",\"domain\":\".qidian.com\"}]"}`,
		`{"data":"{\"cookie_data\":{\"example.com\":[]}}"}`,
		`{"data":"{\"cookies\":null}"}`,
		`{"cookies":[{"name":"","domain":".example.com"}]}`,
		`{"cookies":[{"name":"sid"}]}`,
	} {
		if _, err := ProcessCookieCloudUpdate([]byte(body), CookieCloudConfig{}); !errors.Is(err, ErrCookieCloudInvalidData) {
			t.Fatalf("body=%s, error=%v, want invalid data", body, err)
		}
	}
}

func TestCookieUpdateNormalizesArray(t *testing.T) {
	result, err := ProcessCookieCloudUpdate([]byte(`{"cookies":[
		{"name":"sid","value":"first","domain":".example.com","secure":true,"httpOnly":true,"sameSite":"no_restriction","expirationDate":2000000000.9},
		{"name":"sid","value":"duplicate","domain":".EXAMPLE.COM","path":"/"},
		{"name":"missing_domain","value":"skip"},
		{"domain":".example.com","value":"missing name"},
		{"name":"session","domain":".example.com","session":true,"expirationDate":2000000000}
	]}`), CookieCloudConfig{})
	if err != nil {
		t.Fatal(err)
	}
	want := []cookies.Cookie{
		{Name: "session", Domain: ".example.com", Path: "/", Expires: -1},
		{Name: "sid", Value: "first", Domain: ".example.com", Path: "/", Secure: true, HTTPOnly: true, SameSite: "None", Expires: 2000000000},
	}
	if !reflect.DeepEqual(result.Cookies, want) || result.Skipped != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
