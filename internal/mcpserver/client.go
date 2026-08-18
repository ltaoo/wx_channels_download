package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const default_poll_interval = 500 * time.Millisecond

type api_client struct {
	base_url      string
	http_client   *http.Client
	poll_interval time.Duration
}

type api_envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type scraper_job struct {
	ID       string          `json:"id"`
	Platform string          `json:"platform"`
	URL      string          `json:"url"`
	Status   string          `json:"status"`
	Progress json.RawMessage `json:"progress"`
	Output   json.RawMessage `json:"output"`
	Error    string          `json:"error"`
}

type scraper_output struct {
	JobID        string          `json:"job_id"`
	Platform     string          `json:"platform"`
	URL          string          `json:"url"`
	Result       json.RawMessage `json:"result"`
	DownloadInfo json.RawMessage `json:"download_info"`
}

type download_create_response struct {
	Tasks []download_create_item `json:"tasks"`
	IDs   []int                  `json:"ids"`
}

type download_create_item struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type task_list_response struct {
	List []download_task `json:"list"`
}

type download_task struct {
	ID           int             `json:"id"`
	Name         string          `json:"name"`
	Status       int             `json:"status"`
	Error        string          `json:"error"`
	ErrorMessage string          `json:"error_message"`
	Files        json.RawMessage `json:"files"`
}

func new_api_client(raw_base_url string, http_client *http.Client, poll_interval time.Duration) (*api_client, error) {
	raw_base_url = strings.TrimSpace(raw_base_url)
	if raw_base_url == "" {
		return nil, fmt.Errorf("API 地址不能为空")
	}
	parsed_url, err := url.Parse(raw_base_url)
	if err != nil || parsed_url.Host == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
		return nil, fmt.Errorf("无效的 API 地址: %s", raw_base_url)
	}
	if parsed_url.RawQuery != "" || parsed_url.Fragment != "" {
		return nil, fmt.Errorf("API 地址不能包含 query 或 fragment")
	}
	if http_client == nil {
		http_client = &http.Client{}
	}
	if poll_interval <= 0 {
		poll_interval = default_poll_interval
	}
	return &api_client{
		base_url:      strings.TrimRight(parsed_url.String(), "/"),
		http_client:   http_client,
		poll_interval: poll_interval,
	}, nil
}

func (c *api_client) get_platform_status(ctx context.Context) (json.RawMessage, error) {
	return c.do_json(ctx, http.MethodGet, "/api/scraper/platform/status", nil)
}

func (c *api_client) decrypt_wxchannels_video(ctx context.Context, file_path string, key string) (json.RawMessage, error) {
	query := url.Values{
		"filepath": []string{file_path},
		"key":      []string{key},
	}
	return c.do_json(ctx, http.MethodPost, "/api/channels/decrypt?"+query.Encode(), nil)
}

func (c *api_client) get_wxchannels_api(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return c.do_json(ctx, http.MethodGet, path, nil)
}

func (c *api_client) create_scraper_job(ctx context.Context, raw_url string, force_refresh bool) (*scraper_job, error) {
	body := map[string]any{"url": raw_url, "force_refresh": force_refresh}
	raw_data, err := c.do_json(ctx, http.MethodPost, "/api/scraper/fetch", body)
	if err != nil {
		return nil, err
	}
	return decode_scraper_job(raw_data)
}

func (c *api_client) get_scraper_job(ctx context.Context, job_id string) (*scraper_job, error) {
	query := url.Values{"id": []string{job_id}}
	raw_data, err := c.do_json(ctx, http.MethodGet, "/api/scraper/job?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	return decode_scraper_job(raw_data)
}

func (c *api_client) wait_scraper_job(ctx context.Context, job *scraper_job) (*scraper_job, error) {
	if job == nil || strings.TrimSpace(job.ID) == "" {
		return nil, fmt.Errorf("抓取任务响应缺少 id")
	}
	current_job := job
	for {
		switch current_job.Status {
		case "completed":
			if !has_json_value(current_job.Output) {
				return nil, fmt.Errorf("抓取任务已完成，但响应缺少 output")
			}
			return current_job, nil
		case "failed":
			return nil, new_tool_execution_error(value_or_default(current_job.Error, "抓取内容失败"), raw_json_value(current_job.Progress))
		case "interrupted":
			return nil, new_tool_execution_error(value_or_default(current_job.Error, "抓取任务已中断"), raw_json_value(current_job.Progress))
		}

		select {
		case <-ctx.Done():
			return nil, new_tool_execution_error("等待抓取任务超时或已取消: "+ctx.Err().Error(), raw_json_value(current_job.Progress))
		case <-time.After(c.poll_interval):
		}
		next_job, err := c.get_scraper_job(ctx, current_job.ID)
		if err != nil {
			return nil, err
		}
		current_job = next_job
	}
}

func (c *api_client) create_download_task(ctx context.Context, body any) (*download_create_response, error) {
	raw_data, err := c.do_json(ctx, http.MethodPost, "/api/v1/download_task/create", body)
	if err != nil {
		return nil, err
	}
	var response download_create_response
	if err := json.Unmarshal(raw_data, &response); err != nil {
		return nil, fmt.Errorf("解析下载任务响应失败: %w", err)
	}
	if len(response.Tasks) == 0 {
		return nil, fmt.Errorf("下载任务响应缺少 tasks")
	}
	return &response, nil
}

func (c *api_client) wait_download_task(ctx context.Context, task_id int) (*download_task, error) {
	for {
		query := url.Values{"task_id": []string{strconv.Itoa(task_id)}}
		raw_data, err := c.do_json(ctx, http.MethodGet, "/api/v1/download_task/list?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		task, err := decode_download_task(raw_data, task_id)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, fmt.Errorf("下载任务不存在: %d", task_id)
		}
		switch task.Status {
		case 5:
			return task, nil
		case 6, 7:
			message := value_or_default(task.ErrorMessage, task.Error)
			message = value_or_default(message, fmt.Sprintf("下载任务以状态 %d 结束", task.Status))
			return nil, new_tool_execution_error(message, task)
		}

		select {
		case <-ctx.Done():
			return nil, new_tool_execution_error("等待下载完成超时或已取消: "+ctx.Err().Error(), task)
		case <-time.After(c.poll_interval):
		}
	}
}

func decode_download_task(raw_data json.RawMessage, task_id int) (*download_task, error) {
	var task download_task
	if err := json.Unmarshal(raw_data, &task); err != nil {
		return nil, fmt.Errorf("解析下载进度响应失败: %w", err)
	}
	if task.ID > 0 {
		if task_id <= 0 || task.ID == task_id {
			return &task, nil
		}
		return nil, nil
	}

	var response task_list_response
	if err := json.Unmarshal(raw_data, &response); err != nil {
		return nil, fmt.Errorf("解析下载进度响应失败: %w", err)
	}
	for index := range response.List {
		if task_id <= 0 || response.List[index].ID == task_id {
			return &response.List[index], nil
		}
	}
	return nil, nil
}

func (c *api_client) do_json(ctx context.Context, method string, path string, body any) (json.RawMessage, error) {
	var request_body io.Reader
	if body != nil {
		encoded_body, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("编码 API 请求失败: %w", err)
		}
		request_body = bytes.NewReader(encoded_body)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.base_url+path, request_body)
	if err != nil {
		return nil, fmt.Errorf("创建 API 请求失败: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http_client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("调用下载器 API 失败（请确认主服务已启动）: %w", err)
	}
	defer response.Body.Close()
	response_data, err := io.ReadAll(io.LimitReader(response.Body, 64*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("读取下载器 API 响应失败: %w", err)
	}
	if len(response_data) > 64*1024*1024 {
		return nil, fmt.Errorf("下载器 API 响应超过 64 MB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("下载器 API 返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(response_data)))
	}
	var envelope api_envelope
	if err := json.Unmarshal(response_data, &envelope); err != nil {
		return nil, fmt.Errorf("解析下载器 API 响应失败: %w", err)
	}
	if envelope.Code != 0 {
		return nil, new_tool_execution_error(
			value_or_default(envelope.Msg, fmt.Sprintf("下载器 API 返回错误码 %d", envelope.Code)),
			raw_json_value(envelope.Data),
		)
	}
	if !has_json_value(envelope.Data) {
		return json.RawMessage("{}"), nil
	}
	return envelope.Data, nil
}

func decode_scraper_job(raw_data json.RawMessage) (*scraper_job, error) {
	var job scraper_job
	if err := json.Unmarshal(raw_data, &job); err != nil {
		return nil, fmt.Errorf("解析抓取任务响应失败: %w", err)
	}
	if strings.TrimSpace(job.ID) == "" {
		return nil, fmt.Errorf("抓取任务响应缺少 id")
	}
	return &job, nil
}

func has_json_value(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func raw_json_value(raw json.RawMessage) any {
	if !has_json_value(raw) {
		return nil
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return string(raw)
	}
	return value
}

func value_or_default(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
