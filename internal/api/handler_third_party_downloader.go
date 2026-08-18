package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
)

const third_party_downloader_response_limit = int64(1024 * 1024)

var third_party_downloader_http_client = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		Proxy: nil,
	},
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type third_party_downloader_request struct {
	Kind      string `json:"kind"`
	Endpoint  string `json:"endpoint"`
	Token     string `json:"token"`
	URL       string `json:"url"`
	Filename  string `json:"filename"`
	Directory string `json:"directory"`
	Referer   string `json:"referer"`
	Cookie    string `json:"cookie"`
	UserAgent string `json:"user_agent"`
	TaskID    string `json:"task_id"`
}

type third_party_downloader_result struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Version  string `json:"version,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
}

type third_party_downloader_status_result struct {
	Kind         string  `json:"kind"`
	Name         string  `json:"name"`
	TaskID       string  `json:"task_id"`
	Status       string  `json:"status"`
	Completed    int64   `json:"completed"`
	Total        int64   `json:"total"`
	Speed        int64   `json:"speed"`
	Percent      float64 `json:"percent"`
	FilePath     string  `json:"file_path,omitempty"`
	ErrorMessage string  `json:"error_message,omitempty"`
}

type aria2_rpc_response struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type gopeed_api_response struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

func (c *APIClient) handle_probe_third_party_downloader(ctx *gin.Context) {
	var body third_party_downloader_request
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "三方下载器配置格式错误")
		return
	}

	probe_result, err := probe_third_party_downloader(ctx.Request.Context(), body)
	if err != nil {
		result.Err(ctx, http.StatusBadGateway, err.Error())
		return
	}
	result.Ok(ctx, probe_result)
}

func (c *APIClient) handle_create_third_party_download(ctx *gin.Context) {
	var body third_party_downloader_request
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "下载任务参数格式错误")
		return
	}

	create_result, err := create_third_party_download(ctx.Request.Context(), body)
	if err != nil {
		result.Err(ctx, http.StatusBadGateway, err.Error())
		return
	}
	c.logger.Info().
		Str("kind", create_result.Kind).
		Str("endpoint", create_result.Endpoint).
		Str("task_id", create_result.TaskID).
		Msg("third-party download task created")
	result.Ok(ctx, create_result)
}

func (c *APIClient) handle_third_party_download_status(ctx *gin.Context) {
	var body third_party_downloader_request
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "下载进度参数格式错误")
		return
	}

	status_result, err := get_third_party_download_status(ctx.Request.Context(), body)
	if err != nil {
		result.Err(ctx, http.StatusBadGateway, err.Error())
		return
	}
	result.Ok(ctx, status_result)
}

func probe_third_party_downloader(ctx context.Context, request third_party_downloader_request) (*third_party_downloader_result, error) {
	kind, err := normalize_third_party_downloader_kind(request.Kind)
	if err != nil {
		return nil, err
	}
	endpoint, token, err := normalize_third_party_downloader_endpoint(kind, request.Endpoint, request.Token)
	if err != nil {
		return nil, err
	}

	switch kind {
	case "aria2", "motrix":
		version, err := probe_aria2_downloader(ctx, endpoint, token)
		if err != nil {
			return nil, err
		}
		return &third_party_downloader_result{
			Kind:     kind,
			Name:     third_party_downloader_name(kind),
			Endpoint: endpoint.String(),
			Version:  version,
		}, nil
	case "gopeed":
		version, err := probe_gopeed_downloader(ctx, endpoint, token)
		if err != nil {
			return nil, err
		}
		return &third_party_downloader_result{
			Kind:     kind,
			Name:     third_party_downloader_name(kind),
			Endpoint: endpoint.String(),
			Version:  version,
		}, nil
	default:
		return nil, fmt.Errorf("不支持的三方下载器：%s", request.Kind)
	}
}

func create_third_party_download(ctx context.Context, request third_party_downloader_request) (*third_party_downloader_result, error) {
	kind, err := normalize_third_party_downloader_kind(request.Kind)
	if err != nil {
		return nil, err
	}
	download_url, err := normalize_third_party_download_url(request.URL)
	if err != nil {
		return nil, err
	}
	if kind != "gopeed" && strings.HasPrefix(strings.ToLower(download_url), "ed2k://") {
		return nil, errors.New("aria2 与 Motrix 不支持 ED2K 地址，请改用 Gopeed")
	}
	endpoint, token, err := normalize_third_party_downloader_endpoint(kind, request.Endpoint, request.Token)
	if err != nil {
		return nil, err
	}

	var task_id string
	switch kind {
	case "aria2", "motrix":
		task_id, err = create_aria2_download(ctx, endpoint, token, request, download_url)
	case "gopeed":
		task_id, err = create_gopeed_download(ctx, endpoint, token, request, download_url)
	default:
		err = fmt.Errorf("不支持的三方下载器：%s", request.Kind)
	}
	if err != nil {
		return nil, err
	}

	return &third_party_downloader_result{
		Kind:     kind,
		Name:     third_party_downloader_name(kind),
		Endpoint: endpoint.String(),
		TaskID:   task_id,
	}, nil
}

func get_third_party_download_status(ctx context.Context, request third_party_downloader_request) (*third_party_downloader_status_result, error) {
	kind, err := normalize_third_party_downloader_kind(request.Kind)
	if err != nil {
		return nil, err
	}
	task_id := strings.TrimSpace(request.TaskID)
	if task_id == "" {
		return nil, errors.New("缺少三方下载任务 ID")
	}
	endpoint, token, err := normalize_third_party_downloader_endpoint(kind, request.Endpoint, request.Token)
	if err != nil {
		return nil, err
	}

	var status_result *third_party_downloader_status_result
	switch kind {
	case "aria2", "motrix":
		status_result, err = get_aria2_download_status(ctx, endpoint, token, task_id, request)
	case "gopeed":
		status_result, err = get_gopeed_download_status(ctx, endpoint, token, task_id, request)
	default:
		err = fmt.Errorf("不支持的三方下载器：%s", request.Kind)
	}
	if err != nil {
		return nil, err
	}
	status_result.Kind = kind
	status_result.Name = third_party_downloader_name(kind)
	status_result.TaskID = task_id
	if status_result.Total > 0 {
		status_result.Percent = float64(status_result.Completed) * 100 / float64(status_result.Total)
		if status_result.Percent > 100 {
			status_result.Percent = 100
		}
	}
	return status_result, nil
}

func normalize_third_party_downloader_kind(value string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(value))
	switch kind {
	case "aria", "aria2", "ariang":
		return "aria2", nil
	case "motrix":
		return "motrix", nil
	case "gopeed":
		return "gopeed", nil
	default:
		return "", fmt.Errorf("不支持的三方下载器：%s", value)
	}
}

func third_party_downloader_name(kind string) string {
	switch kind {
	case "motrix":
		return "Motrix"
	case "gopeed":
		return "Gopeed"
	default:
		return "aria2"
	}
}

func default_third_party_downloader_endpoint(kind string) string {
	switch kind {
	case "motrix":
		return "http://127.0.0.1:16800/jsonrpc"
	case "gopeed":
		return "http://127.0.0.1:9999"
	default:
		return "http://127.0.0.1:6800/jsonrpc"
	}
}

func normalize_third_party_downloader_endpoint(kind string, raw_endpoint string, raw_token string) (*url.URL, string, error) {
	value := strings.TrimSpace(raw_endpoint)
	if value == "" {
		value = default_third_party_downloader_endpoint(kind)
	}
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, "", errors.New("下载器地址无效")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, "", errors.New("下载器地址仅支持 http 或 https")
	}

	token := strings.TrimSpace(raw_token)
	if endpoint.User != nil {
		password, has_password := endpoint.User.Password()
		if endpoint.User.Username() != "token" || !has_password {
			return nil, "", errors.New("下载器地址中的认证信息无效")
		}
		if token == "" {
			token = password
		}
		endpoint.User = nil
	}
	if !is_loopback_downloader_host(endpoint.Hostname()) {
		return nil, "", errors.New("为安全起见，三方下载器地址必须是本机环回地址")
	}
	if endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, "", errors.New("下载器地址不能包含查询参数或片段")
	}

	if kind == "aria2" || kind == "motrix" {
		if endpoint.Path == "" || endpoint.Path == "/" {
			endpoint.Path = "/jsonrpc"
		}
	} else {
		endpoint.Path = normalize_gopeed_base_path(endpoint.Path)
	}
	return endpoint, token, nil
}

func normalize_gopeed_base_path(value string) string {
	cleaned := strings.TrimRight(value, "/")
	for _, suffix := range []string{"/api/v1/info", "/api/v1/tasks", "/api/v1"} {
		if strings.HasSuffix(cleaned, suffix) {
			return strings.TrimSuffix(cleaned, suffix)
		}
	}
	return cleaned
}

func is_loopback_downloader_host(host string) bool {
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return true
	}
	ip_address := net.ParseIP(strings.TrimSpace(host))
	return ip_address != nil && ip_address.IsLoopback()
}

func normalize_third_party_download_url(value string) (string, error) {
	raw_url := strings.TrimSpace(value)
	if raw_url == "" {
		return "", errors.New("请输入下载地址")
	}
	lower_url := strings.ToLower(raw_url)
	if strings.HasPrefix(lower_url, "magnet:?") || strings.HasPrefix(lower_url, "ed2k://") {
		return raw_url, nil
	}
	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Scheme == "" {
		return "", errors.New("下载地址无效")
	}
	switch strings.ToLower(parsed_url.Scheme) {
	case "http", "https", "ftp", "sftp", "magnet", "ed2k":
		return raw_url, nil
	default:
		return "", fmt.Errorf("不支持的下载协议：%s", parsed_url.Scheme)
	}
}

func probe_aria2_downloader(ctx context.Context, endpoint *url.URL, token string) (string, error) {
	params := aria2_rpc_params(token)
	response, err := call_aria2_rpc(ctx, endpoint, "aria2.getVersion", params)
	if err != nil {
		return "", fmt.Errorf("无法连接 aria2：%w", err)
	}
	var version struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(response.Result, &version); err != nil {
		return "", errors.New("aria2 返回了无法识别的版本信息")
	}
	return strings.TrimSpace(version.Version), nil
}

func create_aria2_download(ctx context.Context, endpoint *url.URL, token string, request third_party_downloader_request, download_url string) (string, error) {
	options := make(map[string]any)
	if filename := strings.TrimSpace(request.Filename); filename != "" {
		options["out"] = filename
	}
	if directory := strings.TrimSpace(request.Directory); directory != "" {
		options["dir"] = directory
	}
	if referer := strings.TrimSpace(request.Referer); referer != "" {
		options["referer"] = referer
	}
	if user_agent := strings.TrimSpace(request.UserAgent); user_agent != "" {
		options["user-agent"] = user_agent
	}
	if cookie := strings.TrimSpace(request.Cookie); cookie != "" {
		options["header"] = []string{"Cookie: " + cookie}
	}
	params := aria2_rpc_params(token, []string{download_url}, options)
	response, err := call_aria2_rpc(ctx, endpoint, "aria2.addUri", params)
	if err != nil {
		return "", fmt.Errorf("aria2 创建任务失败：%w", err)
	}
	var gid string
	if err := json.Unmarshal(response.Result, &gid); err != nil || strings.TrimSpace(gid) == "" {
		return "", errors.New("aria2 未返回有效的任务 ID")
	}
	return gid, nil
}

func get_aria2_download_status(ctx context.Context, endpoint *url.URL, token string, task_id string, request third_party_downloader_request) (*third_party_downloader_status_result, error) {
	fields := []string{"status", "totalLength", "completedLength", "downloadSpeed", "files", "errorMessage"}
	params := aria2_rpc_params(token, task_id, fields)
	response, err := call_aria2_rpc(ctx, endpoint, "aria2.tellStatus", params)
	if err != nil {
		return nil, fmt.Errorf("aria2 获取下载进度失败：%w", err)
	}
	var status struct {
		Status          string `json:"status"`
		TotalLength     string `json:"totalLength"`
		CompletedLength string `json:"completedLength"`
		DownloadSpeed   string `json:"downloadSpeed"`
		ErrorMessage    string `json:"errorMessage"`
		Files           []struct {
			Path     string `json:"path"`
			Selected string `json:"selected"`
		} `json:"files"`
	}
	if err := json.Unmarshal(response.Result, &status); err != nil {
		return nil, errors.New("aria2 返回了无法识别的下载进度")
	}
	status_result := &third_party_downloader_status_result{
		Status:       normalize_third_party_download_status(status.Status),
		Completed:    parse_downloader_int64(status.CompletedLength),
		Total:        parse_downloader_int64(status.TotalLength),
		Speed:        parse_downloader_int64(status.DownloadSpeed),
		ErrorMessage: strings.TrimSpace(status.ErrorMessage),
	}
	for _, file := range status.Files {
		if strings.TrimSpace(file.Path) != "" && file.Selected != "false" {
			status_result.FilePath = strings.TrimSpace(file.Path)
			break
		}
	}
	if status_result.FilePath == "" && status_result.Status == "completed" {
		status_result.FilePath = fallback_download_file_path(request.Directory, request.Filename)
	}
	return status_result, nil
}

func aria2_rpc_params(token string, values ...any) []any {
	params := make([]any, 0, len(values)+1)
	if token = strings.TrimSpace(token); token != "" {
		params = append(params, "token:"+token)
	}
	return append(params, values...)
}

func call_aria2_rpc(ctx context.Context, endpoint *url.URL, method string, params []any) (*aria2_rpc_response, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "wx-channels-download",
		"method":  method,
		"params":  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	response_body, err := call_local_downloader(ctx, http.MethodPost, endpoint, "", body)
	if err != nil {
		return nil, err
	}
	var response aria2_rpc_response
	if err := json.Unmarshal(response_body, &response); err != nil {
		return nil, errors.New("aria2 返回了无效的 JSON")
	}
	if response.Error != nil {
		return nil, fmt.Errorf("%s（%d）", response.Error.Message, response.Error.Code)
	}
	if len(response.Result) == 0 || string(response.Result) == "null" {
		return nil, errors.New("aria2 返回结果为空")
	}
	return &response, nil
}

func probe_gopeed_downloader(ctx context.Context, endpoint *url.URL, token string) (string, error) {
	response, err := call_gopeed_api(ctx, endpoint, token, http.MethodGet, "/api/v1/info", nil)
	if err != nil {
		return "", fmt.Errorf("无法连接 Gopeed：%w", err)
	}
	var info struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(response.Data, &info); err != nil {
		return "", errors.New("Gopeed 返回了无法识别的版本信息")
	}
	return strings.TrimSpace(info.Version), nil
}

func create_gopeed_download(ctx context.Context, endpoint *url.URL, token string, request third_party_downloader_request, download_url string) (string, error) {
	request_data := map[string]any{"url": download_url}
	headers := make(map[string]string)
	if referer := strings.TrimSpace(request.Referer); referer != "" {
		headers["Referer"] = referer
	}
	if cookie := strings.TrimSpace(request.Cookie); cookie != "" {
		headers["Cookie"] = cookie
	}
	if user_agent := strings.TrimSpace(request.UserAgent); user_agent != "" {
		headers["User-Agent"] = user_agent
	}
	if len(headers) > 0 {
		request_data["extra"] = map[string]any{"header": headers}
	}
	options := make(map[string]any)
	if filename := strings.TrimSpace(request.Filename); filename != "" {
		options["name"] = filename
	}
	if directory := strings.TrimSpace(request.Directory); directory != "" {
		options["path"] = directory
	}
	payload := map[string]any{
		"req":  request_data,
		"opts": options,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	response, err := call_gopeed_api(ctx, endpoint, token, http.MethodPost, "/api/v1/tasks", body)
	if err != nil {
		return "", fmt.Errorf("Gopeed 创建任务失败：%w", err)
	}
	var task_id string
	if err := json.Unmarshal(response.Data, &task_id); err != nil || strings.TrimSpace(task_id) == "" {
		return "", errors.New("Gopeed 未返回有效的任务 ID")
	}
	return task_id, nil
}

func get_gopeed_download_status(ctx context.Context, endpoint *url.URL, token string, task_id string, request third_party_downloader_request) (*third_party_downloader_status_result, error) {
	request_url := gopeed_endpoint_url(endpoint, "/api/v1/tasks")
	query := request_url.Query()
	query.Set("id", task_id)
	request_url.RawQuery = query.Encode()
	response, err := call_gopeed_api_url(ctx, request_url, token, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("Gopeed 获取下载进度失败：%w", err)
	}
	var tasks []struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Name     string `json:"name"`
		Size     int64  `json:"size"`
		Progress struct {
			Downloaded int64 `json:"downloaded"`
			Speed      int64 `json:"speed"`
		} `json:"progress"`
		Meta struct {
			Opts struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"opts"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Data, &tasks); err != nil {
		return nil, errors.New("Gopeed 返回了无法识别的下载进度")
	}
	if len(tasks) == 0 {
		return nil, errors.New("Gopeed 中未找到该下载任务")
	}
	task_index := 0
	task_found := false
	for index, candidate := range tasks {
		if strings.TrimSpace(candidate.ID) == task_id {
			task_index = index
			task_found = true
			break
		}
	}
	if !task_found && len(tasks) > 1 {
		return nil, errors.New("Gopeed 中未找到该下载任务")
	}
	task := tasks[task_index]
	status_result := &third_party_downloader_status_result{
		Status:    normalize_third_party_download_status(task.Status),
		Completed: task.Progress.Downloaded,
		Total:     task.Size,
		Speed:     task.Progress.Speed,
	}
	if status_result.Status == "completed" {
		download_dir := strings.TrimSpace(task.Meta.Opts.Path)
		if download_dir == "" {
			download_dir = strings.TrimSpace(request.Directory)
		}
		filename := strings.TrimSpace(task.Name)
		if filename == "" {
			filename = strings.TrimSpace(task.Meta.Opts.Name)
		}
		if filename == "" {
			filename = strings.TrimSpace(request.Filename)
		}
		status_result.FilePath = fallback_download_file_path(download_dir, filename)
	}
	return status_result, nil
}

func normalize_third_party_download_status(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active", "running", "downloading":
		return "downloading"
	case "waiting", "wait", "ready", "pending":
		return "waiting"
	case "paused", "pause":
		return "paused"
	case "complete", "completed", "done", "finish", "finished":
		return "completed"
	case "error", "failed", "removed", "deleted":
		return "failed"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func parse_downloader_int64(value string) int64 {
	parsed_value, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed_value < 0 {
		return 0
	}
	return parsed_value
}

func fallback_download_file_path(download_dir string, filename string) string {
	download_dir = strings.TrimSpace(download_dir)
	filename = strings.TrimSpace(filename)
	if download_dir == "" || filename == "" || !filepath.IsAbs(download_dir) {
		return ""
	}
	return filepath.Join(download_dir, filepath.Base(filename))
}

func call_gopeed_api(ctx context.Context, endpoint *url.URL, token string, method string, api_path string, body []byte) (*gopeed_api_response, error) {
	return call_gopeed_api_url(ctx, gopeed_endpoint_url(endpoint, api_path), token, method, body)
}

func call_gopeed_api_url(ctx context.Context, request_url *url.URL, token string, method string, body []byte) (*gopeed_api_response, error) {
	response_body, err := call_local_downloader(ctx, method, request_url, token, body)
	if err != nil {
		return nil, err
	}
	var response gopeed_api_response
	if err := json.Unmarshal(response_body, &response); err != nil {
		return nil, errors.New("Gopeed 返回了无效的 JSON")
	}
	if response.Code != 0 {
		message := strings.TrimSpace(response.Message)
		if message == "" {
			message = strings.TrimSpace(response.Msg)
		}
		if message == "" {
			message = "Gopeed 请求失败"
		}
		return nil, errors.New(message)
	}
	return &response, nil
}

func gopeed_endpoint_url(endpoint *url.URL, api_path string) *url.URL {
	request_url := *endpoint
	request_url.Path = path.Join(strings.TrimRight(endpoint.Path, "/"), api_path)
	return &request_url
}

func call_local_downloader(ctx context.Context, method string, endpoint *url.URL, token string, body []byte) ([]byte, error) {
	var body_reader io.Reader
	if body != nil {
		body_reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body_reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token = strings.TrimSpace(token); token != "" {
		request.Header.Set("X-Api-Token", token)
	}
	response, err := third_party_downloader_http_client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	response_body, err := io.ReadAll(io.LimitReader(response.Body, third_party_downloader_response_limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(response_body)) > third_party_downloader_response_limit {
		return nil, errors.New("下载器响应过大")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("下载器返回 HTTP %d", response.StatusCode)
	}
	return response_body, nil
}
