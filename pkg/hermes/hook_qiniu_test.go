package hermes

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wx_channel/pkg/dm"
)

// Opt in with a private JSON config outside the repository; normal test runs never upload.
func TestHookQiniuStreamUpload(t *testing.T) {
	config_path := os.Getenv("QINIU_HOOK_TEST_CONFIG")
	if config_path == "" {
		t.Skip("QINIU_HOOK_TEST_CONFIG is not set")
	}
	config_data, err := os.ReadFile(config_path)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		Bucket    string `json:"bucket"`
		Domain    string `json:"domain"`
	}
	if err := json.Unmarshal(config_data, &config); err != nil {
		t.Fatal("invalid Qiniu test config JSON")
	}
	if config.AccessKey == "" || config.SecretKey == "" || config.Bucket == "" || config.Domain == "" {
		t.Fatal("Qiniu test config is missing required fields")
	}
	image_data := image.NewRGBA(image.Rect(0, 0, 256, 256))
	if _, err := rand.Read(image_data.Pix); err != nil {
		t.Fatal(err)
	}
	var encoded_image bytes.Buffer
	if err := png.Encode(&encoded_image, image_data); err != nil {
		t.Fatal(err)
	}
	file_path := filepath.Join(t.TempDir(), "stream-upload-test.png")
	if err := os.WriteFile(file_path, encoded_image.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("codex-stream-upload-%d.png", time.Now().UnixNano())
	policy, _ := json.Marshal(map[string]any{
		"scope": config.Bucket + ":" + key, "deadline": time.Now().Add(15 * time.Minute).Unix(), "insertOnly": 1,
		"fsizeLimit": encoded_image.Len(), "mimeLimit": "image/png",
	})
	encoded_policy := base64.URLEncoding.EncodeToString(policy)
	signature := hmac.New(sha1.New, []byte(config.SecretKey))
	_, _ = signature.Write([]byte(encoded_policy))
	token := config.AccessKey + ":" + base64.URLEncoding.EncodeToString(signature.Sum(nil)) + ":" + encoded_policy
	metadata, _ := json.Marshal(map[string]string{"token": token, "key": key})
	hooks := NewHookManager()
	if err := hooks.Load(`import fs from "fs";
export async function onTaskSuccess(task) {
  const settings = JSON.parse(task.metadata_json);
  const form = new FormData();
  form.append("token", settings.token);
  form.append("key", settings.key);
  form.append("file", await fs.openAsBlob(task.files[0].file_path, {type: "image/png"}), task.files[0].name);
  const response = await fetch("https://upload-z2.qiniup.com", {method: "POST", body: form});
  const text = await response.text();
  if (!response.ok) throw new Error("Qiniu upload HTTP " + response.status + ": " + text);
  const result = JSON.parse(text);
  if (result.key !== settings.key || !result.hash) throw new Error("invalid Qiniu response");
  globalThis.qiniu_result = result;
}`); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{
		MetadataJSON: string(metadata),
		Files:        []dm.DownloadTaskFile{{Name: "stream-upload-test.png", FilePath: file_path, Kind: "image/png"}},
	}, ""); err != nil {
		t.Fatal(err)
	}
	result := hooks.vm.Get("qiniu_result").ToObject(hooks.vm)
	file_digest := sha1.Sum(encoded_image.Bytes())
	expected_etag := base64.URLEncoding.EncodeToString(append([]byte{0x16}, file_digest[:]...))
	if result.Get("hash").String() != expected_etag {
		t.Fatal("Qiniu ETag differs from the uploaded file")
	}
	domain := config.Domain
	if !strings.Contains(domain, "://") {
		domain = "https://" + domain
	}
	public_url := strings.TrimRight(domain, "/") + "/" + key
	t.Logf("Qiniu upload succeeded: bucket=%s bytes=%d key=%s hash=%s url=%s", config.Bucket, encoded_image.Len(), key, result.Get("hash").String(), public_url)
	report := map[string]any{"bucket": config.Bucket, "bytes": encoded_image.Len(), "key": key, "hash": result.Get("hash").String(), "url": public_url, "verified": false}
	save_report := func() {
		data, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(filepath.Join(filepath.Dir(config_path), "result.json"), data, 0600); err != nil {
			t.Error(err)
		}
	}
	save_report()
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(public_url)
	if err != nil {
		t.Fatalf("upload succeeded but public URL verification failed: %v", err)
	}
	defer response.Body.Close()
	downloaded, err := io.ReadAll(io.LimitReader(response.Body, int64(encoded_image.Len()+1)))
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("upload succeeded but public URL returned HTTP %d: %v", response.StatusCode, err)
	}
	if !bytes.Equal(downloaded, encoded_image.Bytes()) {
		t.Fatal("public file content differs from uploaded bytes")
	}
	report["verified"], report["sha256"] = true, fmt.Sprintf("%x", sha256.Sum256(downloaded))
	save_report()
	t.Logf("Public URL verified: HTTP 200, %d bytes, SHA-256 %s", len(downloaded), report["sha256"])
}
