// Tests adapted from dramadl; see ../NOTICE and ../LICENSE.
package hongguo

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

func TestSpadeMatchesReference(t *testing.T) {
	fixture := load_signing_fixture(t)
	key, err := derive_content_key(fixture.Spade)
	if err != nil || hex.EncodeToString(key) != fixture.ContentKey {
		t.Fatalf("key derivation failed: %v", err)
	}
	seed, err := sourceutil.DecodeBase64(fixture.URLSeed)
	if err != nil {
		t.Fatal(err)
	}
	address, err := decrypt_spade_url(fixture.EncryptedURL, seed)
	if err != nil || address != fixture.MediaURL {
		t.Fatalf("URL decryption failed: %v", err)
	}
	for _, value := range []string{"", "bad", "aGVsbG8=", strings.Repeat("!", 100)} {
		if _, err := derive_content_key(value); err == nil {
			t.Errorf("accepted invalid spade %q", value)
		}
		if _, err := decrypt_spade_url(value, seed); err == nil {
			t.Errorf("accepted invalid URL encoding %q", value)
		}
	}
}

func TestAnonymousRegistration(t *testing.T) {
	var cdid string
	client := mock_client(t, func(req *http.Request) (string, int) {
		if req.URL.String() != register_url || req.Method != "POST" || req.Header.Get("Content-Type") != "application/octet-stream;tt-data=a" {
			t.Fatal("wrong registration endpoint or headers")
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Fatal("registration contains account credentials")
		}
		data, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		digest := md5.Sum(data)
		if req.Header.Get("X-Ss-Stub") != strings.ToUpper(hex.EncodeToString(digest[:])) {
			t.Fatal("invalid registration checksum")
		}
		if len(data) < 54 || !bytes.Equal(data[:6], hex_bytes("746305100000")) || (len(data)-38)%16 != 0 {
			t.Fatal("invalid encrypted envelope")
		}
		material := derive_aes_material(data[6:38])
		block, err := aes.NewCipher(material[:16])
		if err != nil {
			t.Fatal(err)
		}
		plaintext := make([]byte, len(data)-38)
		cipher.NewCBCDecrypter(block, material[16:32]).CryptBlocks(plaintext, data[38:])
		padding := int(plaintext[len(plaintext)-1])
		plaintext = plaintext[:len(plaintext)-padding]
		hash := sha512.Sum512(plaintext[64:])
		if !bytes.Equal(hash[:], plaintext[:64]) {
			t.Fatal("invalid compressed payload digest")
		}
		reader, err := gzip.NewReader(bytes.NewReader(plaintext[64:]))
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		var payload struct {
			MagicTag string         `json:"magic_tag"`
			Header   map[string]any `json:"header"`
		}
		if err := json.NewDecoder(reader).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.MagicTag != "ss_app_log" || payload.Header["os_version"] != "16" || payload.Header["device_model"] != "25053RT47C" {
			t.Fatalf("wrong anonymous profile: %+v", payload)
		}
		cdid, _ = payload.Header["cdid"].(string)
		for _, key := range []string{"device_id", "install_id", "cookie", "device_token", "account", "user_id"} {
			if _, exists := payload.Header[key]; exists {
				t.Errorf("unexpected identity field %s", key)
			}
		}
		return `{"device_id":9007199254740993,"install_id_str":"9007199254741993","device_token":"unused"}`, 200
	})
	identity, err := client.register_device(context.Background())
	if err != nil || identity.device_id != "9007199254740993" || identity.iid != "9007199254741993" || identity.cdid != cdid {
		t.Fatalf("identity lost precision: %+v err=%v", identity, err)
	}
	for _, body := range []string{`{}`, `{"code":1,"device_id":123,"install_id":456}`, `{"device_id":0,"install_id":"456"}`, `{"device_id":"123","install_id":"invalid"}`} {
		client := mock_client(t, func(req *http.Request) (string, int) { return body, 200 })
		if _, err := client.register_device(context.Background()); err == nil {
			t.Errorf("accepted invalid registration %s", body)
		}
	}
}

func TestChooseStream(t *testing.T) {
	fixture := load_signing_fixture(t)
	row := func(codec string, width int, address string, encrypt any) map[string]any {
		return map[string]any{"codec_type": codec, "width": width, "height": width * 2, "main_url": address, "encrypt": encrypt}
	}
	cases := []struct {
		name  string
		rows  any
		ok    bool
		width int
	}{
		{"plaintext", []any{row("h264", 720, test_media, false)}, true, 720},
		{"H264 preferred", []any{row("hevc", 1080, test_media, false), row("h264", 720, test_media, false)}, true, 720},
		{"highest H264", []any{row("h264", 360, test_media, false), row("h264", 1080, test_media, false)}, true, 1080},
		{"metadata dimensions", []any{map[string]any{"video_meta": map[string]any{"codec_type": "hevc", "vwidth": "1080", "vheight": 1920}, "main_url": test_media, "spade_a": fixture.Spade}}, true, 1080},
		{"encrypted URL", []any{map[string]any{"codec_type": "hevc", "width": 1080, "height": 1920, "main_url": fixture.EncryptedURL, "spade_a": fixture.Spade}}, false, 0},
		{"unknown encryption", []any{row("h264", 720, test_media, nil)}, false, 0},
		{"missing key", []any{row("h264", 720, test_media, true)}, false, 0},
		{"ByteVC2", []any{row("bytevc2", 1080, test_media, false)}, false, 0},
		{"foreign address", []any{row("h264", 720, "https://evil.example/video.mp4", false)}, false, 0},
		{"suffix confusion", []any{row("h264", 720, "https://qznovelvod.com.evil.example/video.mp4", false)}, false, 0},
		{"insecure", []any{row("h264", 720, strings.Replace(test_media, "https:", "http:", 1), false)}, false, 0},
		{"missing dimensions", []any{row("h264", 0, test_media, false)}, false, 0},
		{"map list", map[string]any{"one": row("h264", 720, test_media, 0)}, true, 720},
		{"backup", []any{map[string]any{"codec_type": "h264", "width": 720, "height": 1280, "main_url": "https://evil.example/bad", "backup_url": test_media, "encrypt": false}}, true, 720},
	}
	for _, example := range cases {
		t.Run(example.name, func(t *testing.T) {
			raw, err := json.Marshal(example.rows)
			if err != nil {
				t.Fatal(err)
			}
			model := app_model{VideoID: "synthetic", VideoDuration: "77.2", VideoList: raw}
			info, err := choose_stream(model, 0)
			if (err == nil) != example.ok || (example.ok && info.Width != example.width) {
				t.Fatalf("selection=%+v err=%v", info, err)
			}
		})
	}
	raw, _ := json.Marshal([]any{map[string]any{"codec_type": "h264", "width": 720, "height": 1280, "main_url": fixture.EncryptedURL, "spade_a": fixture.Spade}})
	info, err := choose_stream(app_model{VideoID: "synthetic", VideoDuration: "77.2", VideoList: raw, KeySeed: fixture.URLSeed}, 0)
	if err != nil || info.Address != fixture.MediaURL || hex.EncodeToString(info.Key) != fixture.ContentKey {
		t.Fatalf("encrypted model failed: %v", err)
	}
}

func TestAppMediaMatchingAndBoundedRetry(t *testing.T) {
	for _, mode := range []string{"valid", "wrong episode", "rejected", "empty then success", "always empty"} {
		t.Run(mode, func(t *testing.T) {
			registrations, models := 0, 0
			client := mock_client(t, func(req *http.Request) (string, int) {
				if req.URL.Path == "/service/2/device_register/" {
					registrations++
					return `{"device_id":12345,"install_id":67890}`, 200
				}
				models++
				if requested_vid(t, req) != test_vid || req.Header.Get("X-Medusa") == "" || req.Header.Get("X-Gorgon") == "" {
					t.Fatal("unsigned or wrong episode request")
				}
				switch mode {
				case "wrong episode":
					return model_response("999"), 200
				case "rejected":
					return `{"code":1}`, 200
				case "empty then success":
					if models == 1 {
						return "", 200
					}
				case "always empty":
					return "", 200
				}
				return model_response(test_vid), 200
			})
			info, err := client.app_media(context.Background(), test_vid)
			ok := mode == "valid" || mode == "empty then success"
			if (err == nil) != ok || (ok && info.Duration != 2) {
				t.Fatalf("selection=%+v err=%v", info, err)
			}
			if registrations > 2 || models > 2 || ((mode == "empty then success" || mode == "always empty") && models != 2) {
				t.Fatalf("unbounded retry: registrations=%d models=%d", registrations, models)
			}
		})
	}
}

func TestInlineFallbackIdentityAndDuration(t *testing.T) {
	for _, mode := range []string{"valid", "wrong identity", "preview", "lower quality", "foreign endpoint"} {
		t.Run(mode, func(t *testing.T) {
			fallback_calls := 0
			client := mock_client(t, func(req *http.Request) (string, int) {
				if req.URL.Path == "/service/2/device_register/" {
					return `{"device_id":12345,"install_id":67890}`, 200
				}
				if req.URL.Path == "/novel/player/multi_video_model/v1/" {
					fallback := "https://vod.snssdk.com/info"
					if mode == "foreign endpoint" {
						fallback = "https://evil.example/info"
					}
					model := map[string]any{"video_id": "synthetic", "video_duration": 77, "fallback_api": fallback, "video_list": []any{map[string]any{"codec_type": "h264", "width": 720, "height": 1280, "main_url": test_media}}}
					raw, _ := json.Marshal(map[string]any{"data": map[string]any{test_vid: map[string]any{"video_model": model}}})
					return string(raw), 200
				}
				fallback_calls++
				if req.URL.Host != "vod.snssdk.com" {
					t.Fatal("foreign fallback requested")
				}
				id, duration, width := "synthetic", 77, 720
				if mode == "wrong identity" {
					id = "another"
				}
				if mode == "preview" {
					duration = 10
				}
				if mode == "lower quality" {
					width = 360
				}
				model := map[string]any{"video_id": id, "video_duration": duration, "video_list": map[string]any{"one": map[string]any{"codec_type": "h264", "width": width, "height": 1280, "main_url": test_media, "encrypt": false}}}
				raw, _ := json.Marshal(map[string]any{"video_info": map[string]any{"data": model}})
				return string(raw), 200
			})
			_, err := client.app_media(context.Background(), test_vid)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("fallback acceptance mismatch: %v", err)
			}
			if mode == "foreign endpoint" && fallback_calls != 0 {
				t.Fatal("unsafe endpoint was requested")
			}
		})
	}
}

func TestAppDoesNotFollowRedirects(t *testing.T) {
	requests := 0
	client := &api_client{HTTPClient: &http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://evil.example/steal"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})}}
	req, _ := http.NewRequest(http.MethodPost, "https://api5-normal-sinfonlineb.fqnovel.com/test", nil)
	if _, err := client.app_json(context.Background(), req); err == nil || requests != 1 {
		t.Fatalf("redirect followed: calls=%d err=%v", requests, err)
	}
}
