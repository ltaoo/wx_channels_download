package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"wx_channel/internal/config"
	"wx_channel/internal/database/model"
	"wx_channel/internal/mcpserver"
)

// This opt-in acceptance test writes exactly one selected task to a real
// target. All three environment variables must be supplied deliberately.
func TestSyncLiveSelectedTaskViaMCP(t *testing.T) {
	target_url := strings.TrimRight(os.Getenv("WX_SYNC_LIVE_TARGET"), "/")
	db_path := os.Getenv("WX_SYNC_FIXTURE_DB")
	task_id, err := strconv.Atoi(os.Getenv("WX_SYNC_LIVE_TASK_ID"))
	if target_url == "" || db_path == "" || err != nil || task_id <= 0 {
		t.Skip("requires WX_SYNC_LIVE_TARGET, WX_SYNC_FIXTURE_DB and WX_SYNC_LIVE_TASK_ID")
	}
	parsed, err := url.Parse(target_url)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		t.Fatal("invalid live target")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	previous_client := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport, Timeout: 2 * time.Minute}
	t.Cleanup(func() { http.DefaultClient = previous_client; transport.CloseIdleConnections() })
	read_json := func(path string, destination any) {
		t.Helper()
		response, err := http.DefaultClient.Get(target_url + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var envelope struct {
			Code int             `json:"code"`
			Msg  string          `json:"msg"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 200 || envelope.Code != 0 {
			t.Fatalf("target %s: code=%d %s", path, envelope.Code, envelope.Msg)
		}
		if err := json.Unmarshal(envelope.Data, destination); err != nil {
			t.Fatal(err)
		}
	}
	var receiver struct {
		Version int  `json:"version"`
		Enabled bool `json:"receive_enabled"`
	}
	read_json("/api/v1/sync/info", &receiver)
	if receiver.Version < 2 || !receiver.Enabled {
		t.Fatal("target not ready; no synchronization attempted")
	}
	db, err := gorm.Open(sqlite.Open("file:"+db_path+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sql_db.Close()
	var source_task model.DownloadTask
	if err := db.First(&source_task, task_id).Error; err != nil {
		t.Fatal(err)
	}
	var source_resource model.DownloadResource
	if err := db.Where("task_id = ?", task_id).First(&source_resource).Error; err != nil {
		t.Fatal(err)
	}
	log := zerolog.Nop()
	source := &APIClient{db: db, logger: &log, cfg: &APIConfig{DBPath: db_path, WorkDir: filepath.Dir(db_path), DownloadDir: source_resource.DownloadDir, Original: &config.Config{FullPath: filepath.Join(t.TempDir(), "config.yaml")}}}
	bundle, files, err := source.build_sync_bundle(task_id)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Tasks) != 1 || len(files) != 1 || files[0].Size <= 0 {
		t.Fatal("live single-file acceptance requires exactly one task and one existing file")
	}
	local_file, err := os.Open(files[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	local_hash := sha256.New()
	_, hash_err := io.Copy(local_hash, local_file)
	_ = local_file.Close()
	if hash_err != nil {
		t.Fatal(hash_err)
	}
	previous_servers := viper.Get("sync.servers")
	viper.Set("sync.servers", []sync_server_config{})
	t.Cleanup(func() { viper.Set("sync.servers", previous_servers) })
	router := gin.New()
	router.GET("/api/v1/sync/servers", source.handle_sync_servers)
	router.POST("/api/v1/sync/servers", source.handle_sync_configure)
	router.GET("/api/v1/sync/preview", source.handle_sync_preview)
	router.POST("/api/v1/sync/servers/:server_id/jobs", source.handle_sync_start)
	router.GET("/api/v1/sync/jobs/:job_id", source.handle_sync_job)
	api_server := httptest.NewServer(router)
	defer api_server.Close()
	server, _, err := mcpserver.NewRuntime(mcpserver.Config{APIBaseURL: api_server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.CallTool(context.Background(), "get_sync_skill", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	call := func(name string, arguments any, destination any) {
		t.Helper()
		raw, err := json.Marshal(arguments)
		if err != nil {
			t.Fatal(err)
		}
		value, err := server.CallTool(context.Background(), name, raw)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if destination != nil {
			if err := json.Unmarshal(encoded, destination); err != nil {
				t.Fatal(err)
			}
		}
	}
	var preview sync_import_result
	call("preview_sync", map[string]any{"task_ids": []int{task_id}}, &preview)
	if preview.Tasks != 1 || preview.Files != 1 {
		t.Fatalf("preview expanded selection: %+v", preview)
	}
	var target sync_server_record
	call("configure_sync_server", map[string]any{"name": "live-selected-task-test", "url": target_url}, &target)
	var job sync_job
	call("sync_to_server", map[string]any{"server_id": target.ID, "task_ids": []int{task_id}}, &job)
	job_id := job.ID
	deadline := time.Now().Add(2 * time.Minute)
	for job.Status == "running" && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		call("get_sync_status", map[string]any{"job_id": job_id}, &job)
	}
	if job.Status != "completed" || job.Result == nil {
		t.Fatalf("job %s status=%s error=%s; inspect target before retry", job_id, job.Status, job.Error)
	}
	t.Logf("MCP sync committed: source_task=%d tasks=%d files=%d bytes=%d job=%s", task_id, job.Result.Tasks, job.Result.Files, job.Result.Bytes, job_id)
	if job.Result.Tasks != 1 || job.Result.Files != 1 || job.Result.Bytes != files[0].Size {
		t.Fatal("unexpected committed scope; inspect target before retry")
	}
	var listing struct {
		List []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			CreatedAt int64  `json:"created_at"`
		} `json:"list"`
	}
	read_json("/api/v1/download_task/list?page=1&page_size=100", &listing)
	target_task_id := 0
	for _, record := range listing.List {
		if record.Name == source_task.Name && record.CreatedAt == source_task.CreatedAt {
			target_task_id = record.ID
			break
		}
	}
	if target_task_id == 0 {
		t.Fatal("committed task not found; do not rerun blindly")
	}
	var detail struct {
		ID      int `json:"id"`
		Status  int `json:"status"`
		Content struct {
			ID       string `json:"id"`
			Accounts []struct {
				ExternalID string `json:"external_id"`
			} `json:"accounts"`
		} `json:"content"`
		Files []struct {
			Exists bool   `json:"exists"`
			URL    string `json:"file_url"`
			Status string `json:"status"`
			Size   int64  `json:"size"`
		} `json:"files"`
	}
	read_json("/api/v1/download_task/detail?id="+strconv.Itoa(target_task_id), &detail)
	if detail.Status != model.TaskStatusFinished || len(detail.Files) != 1 || !detail.Files[0].Exists || detail.Files[0].Status != "finished" {
		t.Fatalf("target record/file incomplete: task=%d status=%d", detail.ID, detail.Status)
	}
	if preview.Contents > 0 && detail.Content.ID == "" {
		t.Fatal("content association missing")
	}
	if preview.Accounts > 0 && len(detail.Content.Accounts) == 0 {
		t.Fatal("account association missing")
	}
	response, err := http.DefaultClient.Get(target_url + detail.Files[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	remote_hash := sha256.New()
	copied, err := io.Copy(remote_hash, io.LimitReader(response.Body, files[0].Size+1))
	if err != nil || response.StatusCode != 200 || copied != files[0].Size || !bytes.Equal(local_hash.Sum(nil), remote_hash.Sum(nil)) {
		t.Fatal("uploaded file checksum mismatch; inspect target before retry")
	}
	t.Logf("VERIFIED source_task=%d target_task=%d files=1 bytes=%d content=%s accounts=%d sha256=%s", task_id, target_task_id, copied, detail.Content.ID, len(detail.Content.Accounts), fmt.Sprintf("%x", remote_hash.Sum(nil)))
}
