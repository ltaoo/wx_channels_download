package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	result "wx_channel/internal/apiresult"
)

type sync_job struct {
	ID       string              `json:"job_id"`
	ServerID int                 `json:"server_id"`
	TaskIDs  []int               `json:"task_ids,omitempty"`
	Status   string              `json:"status"`
	Result   *sync_import_result `json:"result,omitempty"`
	Error    string              `json:"error,omitempty"`
}

// Jobs outlive a single MCP/HTTP request. Completed results are retained until
// the next 20 jobs or process restart; at most one export runs per process.
var sync_jobs = struct {
	sync.Mutex
	active bool
	order  []string
	jobs   map[string]sync_job
}{jobs: make(map[string]sync_job)}

func (c *APIClient) handle_sync_info(ctx *gin.Context) {
	result.Ok(ctx, gin.H{"version": 2, "receive_enabled": true})
}

func check_sync_receiver(ctx context.Context, server sync_server_config) error {
	probe_ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(probe_ctx, http.MethodGet, server.URL+"/api/v1/sync/info", nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("检查同步目标失败: %w", err)
	}
	defer response.Body.Close()
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Version        int  `json:"version"`
			ReceiveEnabled bool `json:"receive_enabled"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil || response.StatusCode != http.StatusOK || envelope.Code != 0 || envelope.Data.Version < 2 {
		return fmt.Errorf("目标服务不支持完整数据同步，请先更新目标服务")
	}
	if !envelope.Data.ReceiveEnabled {
		return fmt.Errorf("目标服务未启用同步接收")
	}
	return nil
}

func (c *APIClient) handle_sync_preview(ctx *gin.Context) {
	task_ids, err := requested_sync_task_ids(ctx)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	bundle, files, err := c.build_sync_bundle(task_ids...)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	counts := sync_record_counts(bundle)
	var bytes int64
	for _, file := range files {
		bytes += file.Size
	}
	result.Ok(ctx, sync_import_result{Contents: len(bundle.Contents), Tasks: len(bundle.Tasks), Resources: len(bundle.Resources), Accounts: counts["account"], Files: len(files), Bytes: bytes, Tables: counts})
}

func (c *APIClient) handle_sync_start(ctx *gin.Context) {
	server_id, err := strconv.Atoi(ctx.Param("server_id"))
	servers, config_err := c.sync_servers()
	if config_err != nil {
		result.Err(ctx, 500, config_err.Error())
		return
	}
	if err != nil || server_id < 0 || server_id >= len(servers) {
		result.Err(ctx, 400, "同步服务 ID 无效")
		return
	}
	task_ids, err := requested_sync_task_ids(ctx)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	sync_jobs.Lock()
	if sync_jobs.active {
		sync_jobs.Unlock()
		result.Err(ctx, 409, "已有同步任务正在执行，请先查询其状态")
		return
	}
	job := sync_job{ID: uuid.NewString(), ServerID: server_id, Status: "running", TaskIDs: task_ids}
	sync_jobs.active = true
	sync_jobs.jobs[job.ID] = job
	sync_jobs.order = append(sync_jobs.order, job.ID)
	if len(sync_jobs.order) > 20 {
		delete(sync_jobs.jobs, sync_jobs.order[0])
		sync_jobs.order = sync_jobs.order[1:]
	}
	sync_jobs.Unlock()
	go func(job sync_job) {
		job_ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		request := (&http.Request{}).WithContext(job_ctx)
		imported, sync_err := c.send_sync_bundle(request, servers[server_id], job.TaskIDs...)
		job.Result = imported
		job.Status = "completed"
		if sync_err != nil {
			job.Status = "failed"
			job.Error = sync_err.Error()
		}
		sync_jobs.Lock()
		sync_jobs.jobs[job.ID] = job
		sync_jobs.active = false
		sync_jobs.Unlock()
	}(job)
	result.Ok(ctx, job)
}

func requested_sync_task_ids(ctx *gin.Context) ([]int, error) {
	var body struct {
		TaskIDs []int `json:"task_ids"`
	}
	if ctx.Request.Method == http.MethodGet {
		if raw := strings.TrimSpace(ctx.Query("task_ids")); raw != "" {
			for _, part := range strings.Split(raw, ",") {
				id, err := strconv.Atoi(strings.TrimSpace(part))
				if err != nil {
					return nil, fmt.Errorf("task_ids 必须是逗号分隔的正整数")
				}
				body.TaskIDs = append(body.TaskIDs, id)
			}
		}
	} else if ctx.Request.Body != nil && ctx.Request.ContentLength != 0 {
		decoder := json.NewDecoder(ctx.Request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			return nil, fmt.Errorf("同步任务参数无效: %w", err)
		}
	}
	if len(body.TaskIDs) > 1000 {
		return nil, fmt.Errorf("一次最多选择 1000 个任务")
	}
	ids := []int{}
	seen := make(map[int]bool)
	for _, id := range body.TaskIDs {
		if id <= 0 {
			return nil, fmt.Errorf("task_ids 必须是正整数")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

func (c *APIClient) handle_sync_job(ctx *gin.Context) {
	sync_jobs.Lock()
	job, ok := sync_jobs.jobs[ctx.Param("job_id")]
	sync_jobs.Unlock()
	if !ok {
		result.Err(ctx, 404, "同步任务不存在或服务已重启，请核对目标数据后再决定是否重试")
		return
	}
	result.Ok(ctx, job)
}

// Configuration is read for every sync, so saving a target needs no restart.
func (c *APIClient) handle_sync_configure(ctx *gin.Context) {
	var server sync_server_config
	if err := ctx.ShouldBindJSON(&server); err != nil {
		result.Err(ctx, 400, "同步目标参数无效")
		return
	}
	server.Name = strings.TrimSpace(server.Name)
	server.URL = strings.TrimRight(strings.TrimSpace(server.URL), "/")
	parsed, err := url.Parse(server.URL)
	if server.Name == "" || err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		result.Err(ctx, 400, "需要名称和有效的 HTTP(S) 地址")
		return
	}
	servers, err := c.sync_servers()
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	server_id := len(servers)
	for index, existing := range servers {
		if existing.Name == server.Name {
			server_id = index
			break
		}
	}
	if server_id == len(servers) {
		servers = append(servers, server)
	} else {
		servers[server_id] = server
	}
	if err := c.cfg.Original.UpdateAndSave(map[string]interface{}{"sync.servers": servers}); err != nil {
		result.Err(ctx, 500, "保存同步目标失败")
		return
	}
	result.Ok(ctx, sync_server_record{ID: server_id, Name: server.Name, URL: server.URL})
}
