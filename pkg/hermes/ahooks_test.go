package hermes

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"wx_channel/pkg/dm"
)

func TestWorkdirQiniuSuccessHook(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "workdir", "ahooks.js"))
	if os.IsNotExist(err) {
		t.Skip("local workdir/ahooks.js is unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, fail_second := range []bool{false, true} {
		t.Run(fmt.Sprint(fail_second), func(t *testing.T) {
			directory := t.TempDir()
			task := &dm.DownloadTask{ID: 42, Status: 5, Files: []dm.DownloadTaskFile{}}
			contents := [][]byte{[]byte("finished-video\x00\xff"), {}}
			for index, name := range []string{"后处理成片.mp4", "empty.txt"} {
				path := filepath.Join(directory, name)
				if err := os.WriteFile(path, contents[index], 0600); err != nil {
					t.Fatal(err)
				}
				task.Files = append(task.Files, dm.DownloadTaskFile{ID: index + 1, Name: "logical name", FilePath: path, Kind: "application/octet-stream"})
			}
			upload_count, webhook_count := 0, 0
			var webhook_task dm.DownloadTask
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/webhook" {
					webhook_count++
					if upload_count != len(contents) {
						t.Error("webhook was sent before all files finished uploading")
					}
					if err := json.NewDecoder(request.Body).Decode(&webhook_task); err != nil {
						t.Error(err)
					}
					writer.WriteHeader(http.StatusNoContent)
					return
				}
				if err := request.ParseMultipartForm(1024); err != nil {
					t.Error(err)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				defer request.MultipartForm.RemoveAll()
				file, header, err := request.FormFile("file")
				if err != nil {
					t.Error(err)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				defer file.Close()
				data, err := io.ReadAll(file)
				if err != nil || !bytes.Equal(data, contents[upload_count]) || header.Filename != filepath.Base(task.Files[upload_count].FilePath) {
					t.Error("incorrect final file upload")
				}
				token := strings.Split(request.FormValue("token"), ":")
				if len(token) != 3 || token[0] != "test-access-key" {
					t.Error("invalid upload token")
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				mac := hmac.New(sha1.New, []byte("test-secret-key"))
				_, _ = mac.Write([]byte(token[2]))
				if token[1] != base64.URLEncoding.EncodeToString(mac.Sum(nil)) {
					t.Error("invalid Qiniu token signature")
				}
				policy_data, err := base64.URLEncoding.DecodeString(token[2])
				if err != nil {
					t.Error(err)
				}
				var policy struct {
					Scope      string `json:"scope"`
					Deadline   int64  `json:"deadline"`
					InsertOnly int    `json:"insertOnly"`
					FsizeLimit int64  `json:"fsizeLimit"`
				}
				key := request.FormValue("key")
				if err := json.Unmarshal(policy_data, &policy); err != nil || policy.Scope != "test-bucket:"+key || policy.Deadline <= time.Now().Unix() || policy.InsertOnly != 1 || policy.FsizeLimit != int64(len(data)) {
					t.Error("invalid upload policy")
				}
				upload_count++
				if fail_second && upload_count == 2 {
					writer.WriteHeader(http.StatusForbidden)
					_, _ = writer.Write([]byte(`{"error":"test failure"}`))
					return
				}
				_ = json.NewEncoder(writer).Encode(map[string]string{"key": key, "hash": "test-etag"})
			}))
			defer server.Close()
			config_path := filepath.Join(t.TempDir(), "qiniu-upload.json")
			config, _ := json.Marshal(map[string]string{
				"access_key": "test-access-key", "secret_key": "test-secret-key", "bucket": "test-bucket",
				"domain": "https://cdn.example.com", "upload_url": server.URL + "/upload",
			})
			if err := os.WriteFile(config_path, config, 0600); err != nil {
				t.Fatal(err)
			}
			config_pattern := regexp.MustCompile(`const QINIU_CONFIG_PATH\s*=\s*"[^"]+";`)
			if !config_pattern.Match(script) {
				t.Fatal("could not replace the private config path for testing")
			}
			configured_script := config_pattern.ReplaceAllLiteralString(string(script), fmt.Sprintf("const QINIU_CONFIG_PATH = %q;", config_path))
			configured_script = strings.ReplaceAll(configured_script, "http://localhost:8080/webhook/download_task/success", server.URL+"/webhook")
			var log_output bytes.Buffer
			logger := zerolog.New(&log_output)
			hooks := NewHookManager(&logger)
			if err := hooks.Load(configured_script); err != nil {
				t.Fatal(err)
			}
			hooks.runtime.timeout = 2 * time.Second
			err = hooks.InvokeSuccessHook(task, directory)
			if fail_second {
				if err == nil || !strings.Contains(err.Error(), "HTTP 403") || webhook_count != 0 {
					t.Fatalf("upload failure was lost: %v, webhook=%d", err, webhook_count)
				}
			} else if err != nil || webhook_count != 1 || !reflect.DeepEqual(webhook_task, *task) {
				t.Fatalf("success webhook mismatch: err=%v, count=%d", err, webhook_count)
			}
			logs := log_output.String()
			for _, message := range []string{"任务完成回调开始", "读取七牛配置", "开始七牛上传", "收到七牛响应", "上传清单已保存"} {
				if !strings.Contains(logs, message) {
					t.Errorf("missing diagnostic stage: %s", message)
				}
			}
			if strings.Contains(logs, "test-access-key") || strings.Contains(logs, "test-secret-key") {
				t.Error("diagnostic logs exposed credentials")
			}
			if fail_second && !strings.Contains(logs, "任务完成回调执行失败") || !fail_second && !strings.Contains(logs, "收到 Webhook 响应") {
				t.Error("missing terminal diagnostic result")
			}
			manifest, err := os.ReadFile(filepath.Join(directory, "qiniu-upload-42.json"))
			if err != nil {
				t.Fatal(err)
			}
			var uploaded []struct {
				FilePath string `json:"file_path"`
				Key      string `json:"key"`
				URL      string `json:"url"`
			}
			if err := json.Unmarshal(manifest, &uploaded); err != nil {
				t.Fatal(err)
			}
			expected_count := 2
			if fail_second {
				expected_count = 1
			}
			if len(uploaded) != expected_count || uploaded[0].FilePath != task.Files[0].FilePath || !strings.HasPrefix(uploaded[0].URL, "https://cdn.example.com/") || !strings.Contains(uploaded[0].URL, "%") {
				t.Fatalf("incorrect upload manifest: %s", manifest)
			}
		})
	}
}
