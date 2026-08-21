package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

const default_api_base_url = "https://api.cloudflare.com/client/v4"

// DeployBody defines the parameters required for a Worker module upload.
type DeployBody struct {
	AccountID          string
	AuthToken          string
	WorkerName         string
	ScriptContent      []byte
	CompatibilityDate  string
	CompatibilityFlags []string
	Bindings           []Binding
	Exports            map[string]Export
	MainModule         string            // ES module entry point, defaults to index.js.
	AdditionalFiles    map[string][]byte // Extra ES modules or assets included in the multipart upload.
}

// Metadata is the JSON metadata part accepted by the Workers Scripts Upload API.
type Metadata struct {
	MainModule         string            `json:"main_module"`
	CompatibilityDate  string            `json:"compatibility_date,omitempty"`
	CompatibilityFlags []string          `json:"compatibility_flags,omitempty"`
	Bindings           []Binding         `json:"bindings,omitempty"`
	Exports            map[string]Export `json:"exports,omitempty"`
}

// Binding describes one Cloudflare Worker binding. Durable Object bindings use
// Type=durable_object_namespace together with Name and ClassName.
type Binding struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	NamespaceID string `json:"namespace_id,omitempty"`
	Text        string `json:"text,omitempty"`
	ID          string `json:"id,omitempty"`
	ClassName   string `json:"class_name,omitempty"`
	ScriptName  string `json:"script_name,omitempty"`
	Environment string `json:"environment,omitempty"`
}

// Export declares the lifecycle and storage backend of one Worker export.
// New Durable Object classes should use Type=durable-object and Storage=sqlite.
type Export struct {
	Type      string `json:"type"`
	Storage   string `json:"storage,omitempty"`
	State     string `json:"state,omitempty"`
	Container string `json:"container,omitempty"`
}

// DeployResult is the standard Workers Scripts upload response.
type DeployResult struct {
	Success bool  `json:"success"`
	Errors  []any `json:"errors"`
	Result  struct {
		ID string `json:"id"`
	} `json:"result"`
}

// ClientOptions controls the Cloudflare API transport. BaseURL and HTTPClient
// are primarily useful for tests and private API gateways.
type ClientOptions struct {
	BaseURL    string
	HTTPClient *http.Client
}

// Client calls the Cloudflare Workers REST API.
type Client struct {
	base_url    string
	http_client *http.Client
}

// NewClient creates a Workers REST API client.
func NewClient(options ClientOptions) *Client {
	base_url := strings.TrimRight(strings.TrimSpace(options.BaseURL), "/")
	if base_url == "" {
		base_url = default_api_base_url
	}
	http_client := options.HTTPClient
	if http_client == nil {
		http_client = http.DefaultClient
	}
	return &Client{base_url: base_url, http_client: http_client}
}

// Deploy preserves the previous convenience API: upload the Worker and enable
// its workers.dev subdomain. Call Client.Deploy directly when those operations
// need to be coordinated with secret creation.
func Deploy(deploy_body DeployBody) (string, error) {
	api_client := NewClient(ClientOptions{})
	worker_id, err := api_client.Deploy(context.Background(), deploy_body)
	if err != nil {
		return "", err
	}
	if err := api_client.EnableSubdomain(
		context.Background(),
		deploy_body.AccountID,
		deploy_body.AuthToken,
		deploy_body.WorkerName,
	); err != nil {
		// Keep backward compatibility: a successful script upload remains a
		// success even when the optional workers.dev route cannot be enabled.
		return worker_id, nil
	}
	return worker_id, nil
}

// Deploy uploads one ES module Worker with multipart metadata.
func (c *Client) Deploy(request_context context.Context, deploy_body DeployBody) (string, error) {
	if err := validate_deploy_body(deploy_body); err != nil {
		return "", err
	}
	main_module := strings.TrimSpace(deploy_body.MainModule)
	if main_module == "" {
		main_module = "index.js"
	}
	metadata := Metadata{
		MainModule:         main_module,
		CompatibilityDate:  strings.TrimSpace(deploy_body.CompatibilityDate),
		CompatibilityFlags: deploy_body.CompatibilityFlags,
		Bindings:           deploy_body.Bindings,
		Exports:            deploy_body.Exports,
	}
	body, content_type, err := build_multipart(metadata, main_module, deploy_body.ScriptContent, deploy_body.AdditionalFiles)
	if err != nil {
		return "", err
	}
	endpoint := c.script_endpoint(deploy_body.AccountID, deploy_body.WorkerName)
	request, err := http.NewRequestWithContext(request_context, http.MethodPut, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("创建 Worker 上传请求失败: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+deploy_body.AuthToken)
	request.Header.Set("Content-Type", content_type)

	response_body, status_code, err := c.do(request)
	if err != nil {
		return "", fmt.Errorf("上传 Worker 失败: %w", err)
	}
	if status_code < http.StatusOK || status_code >= http.StatusMultipleChoices {
		return "", fmt.Errorf("部署失败 (Status: %d): %s", status_code, strings.TrimSpace(string(response_body)))
	}
	var result DeployResult
	if err := json.Unmarshal(response_body, &result); err != nil {
		return "", fmt.Errorf("解析 Worker 部署响应失败: %w, body: %s", err, string(response_body))
	}
	if !result.Success {
		return "", fmt.Errorf("部署失败 (API Error): %s", string(response_body))
	}
	return result.Result.ID, nil
}

// PutSecret creates or replaces one secret_text binding for a Worker script.
func (c *Client) PutSecret(
	request_context context.Context,
	account_id string,
	auth_token string,
	worker_name string,
	secret_name string,
	secret_text string,
) error {
	if strings.TrimSpace(secret_name) == "" {
		return errors.New("secret name is required")
	}
	payload, err := json.Marshal(map[string]string{
		"name": strings.TrimSpace(secret_name),
		"text": secret_text,
		"type": "secret_text",
	})
	if err != nil {
		return fmt.Errorf("编码 Worker secret 失败: %w", err)
	}
	endpoint := c.script_endpoint(account_id, worker_name) + "/secrets"
	request, err := http.NewRequestWithContext(request_context, http.MethodPut, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("创建 Worker secret 请求失败: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+auth_token)
	request.Header.Set("Content-Type", "application/json")
	response_body, status_code, err := c.do(request)
	if err != nil {
		return fmt.Errorf("写入 Worker secret 失败: %w", err)
	}
	if status_code < http.StatusOK || status_code >= http.StatusMultipleChoices {
		return fmt.Errorf("写入 Worker secret 失败 (Status: %d): %s", status_code, strings.TrimSpace(string(response_body)))
	}
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(response_body, &result); err != nil {
		return fmt.Errorf("解析 Worker secret 响应失败: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("写入 Worker secret 失败: %s", string(response_body))
	}
	return nil
}

// EnableSubdomain enables the workers.dev route for a Worker script.
func (c *Client) EnableSubdomain(
	request_context context.Context,
	account_id string,
	auth_token string,
	worker_name string,
) error {
	endpoint := c.script_endpoint(account_id, worker_name) + "/subdomain"
	request, err := http.NewRequestWithContext(
		request_context,
		http.MethodPost,
		endpoint,
		bytes.NewReader([]byte(`{"enabled":true}`)),
	)
	if err != nil {
		return fmt.Errorf("创建 workers.dev 子域请求失败: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+auth_token)
	request.Header.Set("Content-Type", "application/json")
	response_body, status_code, err := c.do(request)
	if err != nil {
		return fmt.Errorf("启用 workers.dev 子域失败: %w", err)
	}
	if status_code < http.StatusOK || status_code >= http.StatusMultipleChoices {
		return fmt.Errorf("启用 workers.dev 子域失败 (Status: %d): %s", status_code, strings.TrimSpace(string(response_body)))
	}
	return nil
}

// GetSubdomain returns the account-level workers.dev subdomain.
func (c *Client) GetSubdomain(
	request_context context.Context,
	account_id string,
	auth_token string,
) (string, error) {
	endpoint := fmt.Sprintf(
		"%s/accounts/%s/workers/subdomain",
		c.base_url,
		url.PathEscape(strings.TrimSpace(account_id)),
	)
	request, err := http.NewRequestWithContext(request_context, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("创建 Worker 子域名请求失败: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(auth_token))
	response_body, status_code, err := c.do(request)
	if err != nil {
		return "", fmt.Errorf("查询 Worker 子域名失败: %w", err)
	}
	var result struct {
		Success bool `json:"success"`
		Result  struct {
			Subdomain string `json:"subdomain"`
		} `json:"result"`
		Errors []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(response_body, &result); err != nil {
		return "", fmt.Errorf("解析 Worker 子域名响应失败: %w", err)
	}
	if status_code < http.StatusOK || status_code >= http.StatusMultipleChoices || !result.Success {
		message := strings.TrimSpace(string(response_body))
		if len(result.Errors) > 0 && result.Errors[0].Message != "" {
			message = result.Errors[0].Message
		}
		return "", fmt.Errorf("Cloudflare API 返回 %d: %s", status_code, message)
	}
	subdomain := strings.TrimSpace(result.Result.Subdomain)
	if subdomain == "" {
		return "", errors.New("Cloudflare API 返回了空的 Worker 子域名")
	}
	return subdomain, nil
}

func validate_deploy_body(deploy_body DeployBody) error {
	if strings.TrimSpace(deploy_body.AccountID) == "" {
		return errors.New("Cloudflare account id is required")
	}
	if strings.TrimSpace(deploy_body.AuthToken) == "" {
		return errors.New("Cloudflare auth token is required")
	}
	if strings.TrimSpace(deploy_body.WorkerName) == "" {
		return errors.New("Worker name is required")
	}
	if len(deploy_body.ScriptContent) == 0 {
		return errors.New("Worker script content is required")
	}
	return nil
}

func build_multipart(
	metadata Metadata,
	main_module string,
	script_content []byte,
	additional_files map[string][]byte,
) (*bytes.Buffer, string, error) {
	metadata_json, err := json.Marshal(metadata)
	if err != nil {
		return nil, "", fmt.Errorf("构造 Worker metadata 失败: %w", err)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := write_part(writer, "metadata", "", "application/json", metadata_json); err != nil {
		return nil, "", err
	}
	if err := write_part(writer, main_module, main_module, "application/javascript+module", script_content); err != nil {
		return nil, "", err
	}
	filenames := make([]string, 0, len(additional_files))
	for filename := range additional_files {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	for _, filename := range filenames {
		if err := write_part(
			writer,
			filename,
			filename,
			detect_content_type(filename),
			additional_files[filename],
		); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("完成 Worker multipart 请求失败: %w", err)
	}
	return body, writer.FormDataContentType(), nil
}

func write_part(writer *multipart.Writer, name string, filename string, content_type string, content []byte) error {
	headers := make(textproto.MIMEHeader)
	disposition := fmt.Sprintf(`form-data; name=%q`, name)
	if filename != "" {
		disposition += fmt.Sprintf(`; filename=%q`, filename)
	}
	headers.Set("Content-Disposition", disposition)
	headers.Set("Content-Type", content_type)
	part, err := writer.CreatePart(headers)
	if err != nil {
		return fmt.Errorf("创建 Worker multipart part %s 失败: %w", name, err)
	}
	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("写入 Worker multipart part %s 失败: %w", name, err)
	}
	return nil
}

func detect_content_type(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".html", ".htm":
		return "application/octet-stream"
	case ".js", ".mjs":
		return "application/javascript+module"
	case ".css":
		return "text/css"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".wasm":
		return "application/wasm"
	default:
		return "application/octet-stream"
	}
}

func (c *Client) script_endpoint(account_id string, worker_name string) string {
	return fmt.Sprintf(
		"%s/accounts/%s/workers/scripts/%s",
		c.base_url,
		url.PathEscape(strings.TrimSpace(account_id)),
		url.PathEscape(strings.TrimSpace(worker_name)),
	)
}

func (c *Client) do(request *http.Request) ([]byte, int, error) {
	response, err := c.http_client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	response_body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, err
	}
	return response_body, response.StatusCode, nil
}
