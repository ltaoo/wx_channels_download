package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	download_api_base_url     string
	download_existing_action  string
	download_force_refresh    bool
	download_timeout          time.Duration
	download_poll_interval    = 500 * time.Millisecond
	download_max_response     = 64 * 1024 * 1024
	download_existing_actions = []string{"error", "skip", "overwrite", "duplicate"}
)

type download_api_envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type download_scraper_job struct {
	ID       string          `json:"id"`
	Platform string          `json:"platform"`
	URL      string          `json:"url"`
	Status   string          `json:"status"`
	Error    string          `json:"error"`
	Output   json.RawMessage `json:"output"`
}

type download_scraper_output struct {
	JobID        string          `json:"job_id"`
	Platform     string          `json:"platform"`
	URL          string          `json:"url"`
	Result       json.RawMessage `json:"result"`
	Content      json.RawMessage `json:"content"`
	DownloadInfo json.RawMessage `json:"download_info"`
}

type download_scraper_content struct {
	Title string `json:"title"`
	ID    string `json:"id"`
}

type download_task_create_body struct {
	Platform       string          `json:"platform"`
	Content        json.RawMessage `json:"content"`
	BuildFromFetch bool            `json:"build_from_fetch"`
	Config         map[string]any  `json:"config"`
	AutoStart      *bool           `json:"auto_start"`
}

type download_task_create_request struct {
	Objects []download_task_create_body `json:"objects"`
}

type download_task_create_response struct {
	Tasks []download_task_create_item `json:"tasks"`
	IDs   []int                       `json:"ids"`
}

type download_task_create_item struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type download_task_item struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

type download_duplicate_data struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var download_cmd = &cobra.Command{
	Use:   "download <url>",
	Short: "解析内容链接并创建下载任务",
	Long: "\n通过已启动的下载器服务解析内容 URL，解析成功后创建并启动下载任务；解析失败时给出错误原因。" +
		"\n需要主服务已启动（默认读取 api.protocol、api.hostname 和 api.port 配置）。",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		raw_url := strings.TrimSpace(args[0])
		if raw_url == "" {
			return fmt.Errorf("url 不能为空")
		}
		download_existing_action = strings.TrimSpace(download_existing_action)
		if download_existing_action == "" {
			download_existing_action = "error"
		}
		if !download_existing_action_allowed(download_existing_action) {
			return fmt.Errorf("existing-action 必须是 %s", strings.Join(download_existing_actions, "、"))
		}
		api_base_url := strings.TrimSpace(download_api_base_url)
		if api_base_url == "" {
			api_base_url = configured_api_base_url()
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), download_timeout)
		defer cancel()

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "正在解析: %s\n", raw_url)
		output, err := download_fetch_content(ctx, api_base_url, raw_url)
		if err != nil {
			fmt.Fprintf(root_cmd.ErrOrStderr(), "%s 解析失败: %v\n", error_prefix, err)
			os.Exit(1)
		}

		title := download_output_title(output)
		fmt.Fprintf(out, "解析成功: [%s] %s\n", output.Platform, title)

		task_id, task_name, err := download_create_task(ctx, api_base_url, output, raw_url)
		if err != nil {
			fmt.Fprintf(root_cmd.ErrOrStderr(), "%s 创建下载任务失败: %v\n", error_prefix, err)
			os.Exit(1)
		}
		if task_id > 0 {
			fmt.Fprintf(out, "下载任务创建成功: #%d %s\n", task_id, task_name)
		} else {
			fmt.Fprintln(out, "已存在相同下载任务，按 existing-action 处理")
		}
		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	download_cmd.Flags().StringVar(
		&download_api_base_url,
		"api-base-url",
		"",
		"下载器 API 地址，默认读取 api.protocol、api.hostname 和 api.port 配置",
	)
	download_cmd.Flags().StringVar(
		&download_existing_action,
		"existing-action",
		"error",
		"解析结果已有下载任务时的处理方式：error、skip、overwrite、duplicate",
	)
	download_cmd.Flags().BoolVar(
		&download_force_refresh,
		"force-refresh",
		false,
		"忽略抓取缓存，强制重新请求平台",
	)
	download_cmd.Flags().DurationVar(
		&download_timeout,
		"timeout",
		10*time.Minute,
		"解析与创建任务的总超时时间",
	)
	Register(download_cmd)
}

func download_existing_action_allowed(action string) bool {
	for _, allowed := range download_existing_actions {
		if allowed == action {
			return true
		}
	}
	return false
}

func download_fetch_content(ctx context.Context, api_base_url string, raw_url string) (*download_scraper_output, error) {
	body := map[string]any{
		"url":           raw_url,
		"force_refresh": download_force_refresh,
	}
	raw_data, err := download_api_json(ctx, api_base_url, http.MethodPost, "/api/scraper/fetch", body)
	if err != nil {
		return nil, err
	}
	var job download_scraper_job
	if err := json.Unmarshal(raw_data, &job); err != nil {
		return nil, fmt.Errorf("解析抓取任务响应失败: %w", err)
	}
	if strings.TrimSpace(job.ID) == "" {
		return nil, fmt.Errorf("创建抓取任务失败：响应缺少 id")
	}

	for {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("等待解析完成超时或已取消: %w", ctx.Err())
		}
		if job.Status == "completed" {
			break
		}
		if job.Status == "failed" || job.Status == "interrupted" {
			message := strings.TrimSpace(job.Error)
			if message == "" {
				message = "抓取任务以状态 " + job.Status + " 结束"
			}
			return nil, fmt.Errorf("%s", message)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("等待解析完成超时或已取消: %w", ctx.Err())
		case <-time.After(download_poll_interval):
		}
		query := url.Values{"id": []string{job.ID}}
		raw_data, err := download_api_json(ctx, api_base_url, http.MethodGet, "/api/scraper/job?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw_data, &job); err != nil {
			return nil, fmt.Errorf("解析抓取任务响应失败: %w", err)
		}
	}

	if len(bytes.TrimSpace(job.Output)) == 0 {
		return nil, fmt.Errorf("抓取任务已完成，但缺少解析结果")
	}
	var output download_scraper_output
	if err := json.Unmarshal(job.Output, &output); err != nil {
		return nil, fmt.Errorf("解析抓取结果失败: %w", err)
	}
	if strings.TrimSpace(output.Platform) == "" {
		output.Platform = job.Platform
	}
	if len(bytes.TrimSpace(output.Result)) == 0 {
		return nil, fmt.Errorf("抓取结果缺少 result")
	}
	return &output, nil
}

func download_create_task(ctx context.Context, api_base_url string, output *download_scraper_output, raw_url string) (int, string, error) {
	config := map[string]any{
		"platform":        output.Platform,
		"existing_action": download_existing_action,
	}
	if download_existing_action == "overwrite" {
		config["overwrite"] = true
	}
	if download_existing_action == "duplicate" {
		config["duplicate"] = true
	}
	auto_start := true
	request := download_task_create_request{
		Objects: []download_task_create_body{{
			Platform:       output.Platform,
			Content:        output.Result,
			BuildFromFetch: len(bytes.TrimSpace(output.DownloadInfo)) > 0,
			Config:         config,
			AutoStart:      &auto_start,
		}},
	}
	raw_data, err := download_api_json(ctx, api_base_url, http.MethodPost, "/api/v1/download_task/create", request)
	if err != nil {
		return 0, "", err
	}
	var response download_task_create_response
	if err := json.Unmarshal(raw_data, &response); err != nil {
		return 0, "", fmt.Errorf("解析下载任务响应失败: %w", err)
	}
	if len(response.Tasks) == 0 {
		return 0, "", fmt.Errorf("下载任务响应缺少 tasks")
	}
	item := response.Tasks[0]
	if item.Code != 0 {
		var duplicate download_duplicate_data
		_ = json.Unmarshal(item.Data, &duplicate)
		message := strings.TrimSpace(item.Msg)
		if message == "" {
			message = fmt.Sprintf("下载任务创建失败，错误码 %d", item.Code)
		}
		if duplicate.ID > 0 || duplicate.Name != "" {
			message += fmt.Sprintf("（已有任务 #%d %s）", duplicate.ID, duplicate.Name)
		}
		return 0, "", fmt.Errorf("%s", message)
	}
	var task download_task_item
	_ = json.Unmarshal(item.Data, &task)
	task_id := task.ID
	if task_id == 0 && len(response.IDs) > 0 {
		task_id = response.IDs[0]
	}
	if task_id == 0 {
		return 0, "", fmt.Errorf("下载任务响应缺少任务 id")
	}
	name := strings.TrimSpace(task.Name)
	if name == "" {
		name = download_output_title(output)
	}
	if name == "" {
		name = raw_url
	}
	return task_id, name, nil
}

func download_output_title(output *download_scraper_output) string {
	var content download_scraper_content
	if json.Unmarshal(output.Content, &content) == nil {
		if title := strings.TrimSpace(content.Title); title != "" {
			return title
		}
		if id := strings.TrimSpace(content.ID); id != "" {
			return id
		}
	}
	return strings.TrimSpace(output.URL)
}

func download_api_json(ctx context.Context, api_base_url string, method string, path string, body any) (json.RawMessage, error) {
	var request_body io.Reader
	if body != nil {
		encoded_body, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("编码请求失败: %w", err)
		}
		request_body = bytes.NewReader(encoded_body)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(api_base_url, "/")+path, request_body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("调用下载器服务失败（请确认主服务已启动）: %w", err)
	}
	defer response.Body.Close()
	response_data, err := io.ReadAll(io.LimitReader(response.Body, int64(download_max_response)+1))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if len(response_data) > download_max_response {
		return nil, fmt.Errorf("服务响应超过 64 MB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("下载器服务返回状态码 %d: %s", response.StatusCode, strings.TrimSpace(string(response_data)))
	}
	var envelope download_api_envelope
	if err := json.Unmarshal(response_data, &envelope); err != nil {
		return nil, fmt.Errorf("解析服务响应失败: %w", err)
	}
	if envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Msg)
		if message == "" {
			message = fmt.Sprintf("下载器返回错误码 %d", envelope.Code)
		}
		return nil, fmt.Errorf("%s", message)
	}
	if len(bytes.TrimSpace(envelope.Data)) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		return json.RawMessage("{}"), nil
	}
	return envelope.Data, nil
}
