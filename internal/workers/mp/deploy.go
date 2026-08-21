package mp

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wx_channel/pkg/cloudflare/worker"
)

const (
	default_api_base_url            = "https://api.cloudflare.com/client/v4"
	worker_compatibility_date       = "2024-01-01"
	worker_main_module              = "index.js"
	d1_operation_timeout            = 5 * time.Minute
	worker_deploy_timeout           = 2 * time.Minute
	worker_subdomain_timeout        = 30 * time.Second
	cloudflare_http_request_timeout = 2 * time.Minute
)

//go:embed index.js
var worker_javascript []byte

//go:embed migrations/*.sql
var migration_files embed.FS

// DeployStage identifies the current MP deployment phase.
type DeployStage string

const (
	DeployStageD1Lookup        DeployStage = "d1_lookup"
	DeployStageD1Create        DeployStage = "d1_create"
	DeployStageD1Verify        DeployStage = "d1_verify"
	DeployStageD1Migrate       DeployStage = "d1_migrate"
	DeployStageWorker          DeployStage = "worker"
	DeployStageWorkerSubdomain DeployStage = "worker_subdomain"
)

// DeployProgress is emitted before each MP deployment phase.
type DeployProgress struct {
	Stage   DeployStage
	Message string
}

// DeployOptions contains the Cloudflare and MP Worker deployment settings.
type DeployOptions struct {
	AccountID            string
	AuthToken            string
	WorkerName           string
	D1DatabaseID         string
	D1DatabaseName       string
	AdminToken           string
	RefreshToken         string
	RemoteServerHostname string
	APIBaseURL           string
	HTTPClient           *http.Client
	Progress             func(DeployProgress)
}

// DeployResult identifies the D1 database and deployed MP Worker.
type DeployResult struct {
	WorkerID         string
	WorkerName       string
	WorkerURL        string
	WorkerURLWarning string
	ScriptBytes      int
	D1DatabaseID     string
	D1DatabaseName   string
	MigrationWarning string
}

// Deploy prepares D1, applies embedded migrations, uploads the MP Worker and
// resolves its workers.dev address.
func Deploy(request_context context.Context, options DeployOptions) (*DeployResult, error) {
	normalized_options, err := normalize_deploy_options(options)
	if err != nil {
		return nil, err
	}
	options = normalized_options

	http_client := options.HTTPClient
	if http_client == nil {
		http_client = &http.Client{Timeout: cloudflare_http_request_timeout}
	}
	d1_client := &d1_api_client{
		base_url:    options.APIBaseURL,
		http_client: http_client,
		account_id:  options.AccountID,
		auth_token:  options.AuthToken,
	}
	worker_client := worker.NewClient(worker.ClientOptions{
		BaseURL:    options.APIBaseURL,
		HTTPClient: http_client,
	})

	database_id := options.D1DatabaseID
	d1_context, cancel_d1 := context.WithTimeout(request_context, d1_operation_timeout)
	if options.D1DatabaseName != "" {
		notify_progress(
			options,
			DeployStageD1Lookup,
			fmt.Sprintf("正在查找 D1 数据库 %s...", options.D1DatabaseName),
		)
		database_id, err = d1_client.find_database_by_name(d1_context, options.D1DatabaseName)
		if err != nil {
			cancel_d1()
			return nil, fmt.Errorf("查找 D1 数据库失败: %w", err)
		}
		if database_id == "" {
			notify_progress(
				options,
				DeployStageD1Create,
				fmt.Sprintf("正在创建 D1 数据库 %s...", options.D1DatabaseName),
			)
			database_id, err = d1_client.create_database(d1_context, options.D1DatabaseName)
			if err != nil {
				cancel_d1()
				return nil, fmt.Errorf("创建 D1 数据库失败: %w", err)
			}
		}
	}
	if database_id == "" {
		cancel_d1()
		return nil, errors.New("未配置 cloudflare.d1Id，且无法通过 cloudflare.d1Name 查找或创建数据库")
	}

	result := &DeployResult{
		WorkerName:     options.WorkerName,
		D1DatabaseID:   database_id,
		D1DatabaseName: options.D1DatabaseName,
	}
	notify_progress(options, DeployStageD1Verify, "正在验证 D1 数据库连接...")
	if err := d1_client.verify_database(d1_context, database_id); err != nil {
		cancel_d1()
		return result, fmt.Errorf("验证 D1 数据库失败: %w", err)
	}

	notify_progress(options, DeployStageD1Migrate, "正在检查并执行 D1 数据库迁移...")
	if err := run_migrations(d1_context, d1_client, database_id); err != nil {
		result.MigrationWarning = fmt.Sprintf("D1 数据库迁移失败: %v", err)
	}
	cancel_d1()

	notify_progress(options, DeployStageWorker, "正在部署公众号 Worker...")
	worker_context, cancel_worker := context.WithTimeout(request_context, worker_deploy_timeout)
	worker_id, err := worker_client.Deploy(worker_context, worker.DeployBody{
		AccountID:         options.AccountID,
		AuthToken:         options.AuthToken,
		WorkerName:        options.WorkerName,
		ScriptContent:     worker_javascript,
		CompatibilityDate: worker_compatibility_date,
		MainModule:        worker_main_module,
		Bindings: []worker.Binding{
			{Type: "d1", Name: "DB", ID: database_id},
			{Type: "plain_text", Name: "ADMIN_TOKEN", Text: options.AdminToken},
			{Type: "plain_text", Name: "REFRESH_TOKEN", Text: options.RefreshToken},
			{Type: "plain_text", Name: "REMOTE_SERVER", Text: options.RemoteServerHostname},
		},
	})
	cancel_worker()
	if err != nil {
		return result, fmt.Errorf("部署公众号 Worker 失败: %w", err)
	}
	result.WorkerID = worker_id
	result.ScriptBytes = len(worker_javascript)
	result.WorkerURL = fmt.Sprintf("https://%s.<your-subdomain>.workers.dev", options.WorkerName)

	warnings := make([]string, 0, 2)
	subdomain_context, cancel_subdomain := context.WithTimeout(request_context, worker_subdomain_timeout)
	if err := worker_client.EnableSubdomain(
		subdomain_context,
		options.AccountID,
		options.AuthToken,
		options.WorkerName,
	); err != nil {
		warnings = append(warnings, fmt.Sprintf("启用 workers.dev 地址失败: %v", err))
	}
	cancel_subdomain()

	notify_progress(options, DeployStageWorkerSubdomain, "正在获取 Worker 访问地址...")
	subdomain_context, cancel_subdomain = context.WithTimeout(request_context, worker_subdomain_timeout)
	subdomain, subdomain_err := worker_client.GetSubdomain(
		subdomain_context,
		options.AccountID,
		options.AuthToken,
	)
	cancel_subdomain()
	if subdomain_err != nil {
		warnings = append(warnings, fmt.Sprintf("获取 Worker 子域名失败: %v", subdomain_err))
	} else {
		result.WorkerURL = fmt.Sprintf("https://%s.%s.workers.dev", options.WorkerName, subdomain)
	}
	result.WorkerURLWarning = strings.Join(warnings, "; ")
	return result, nil
}

func normalize_deploy_options(options DeployOptions) (DeployOptions, error) {
	options.AccountID = strings.TrimSpace(options.AccountID)
	options.AuthToken = strings.TrimSpace(options.AuthToken)
	options.WorkerName = strings.TrimSpace(options.WorkerName)
	options.D1DatabaseID = strings.TrimSpace(options.D1DatabaseID)
	options.D1DatabaseName = strings.TrimSpace(options.D1DatabaseName)
	options.AdminToken = strings.TrimSpace(options.AdminToken)
	options.RefreshToken = strings.TrimSpace(options.RefreshToken)
	options.RemoteServerHostname = strings.TrimSpace(options.RemoteServerHostname)
	options.APIBaseURL = strings.TrimRight(strings.TrimSpace(options.APIBaseURL), "/")
	if options.APIBaseURL == "" {
		options.APIBaseURL = default_api_base_url
	}
	if options.AccountID == "" {
		return options, errors.New("未配置 cloudflare.accountId")
	}
	if options.AuthToken == "" {
		return options, errors.New("未配置 cloudflare.apiToken")
	}
	if options.WorkerName == "" {
		return options, errors.New("未配置 cloudflare.workerName")
	}
	return options, nil
}

func notify_progress(options DeployOptions, stage DeployStage, message string) {
	if options.Progress != nil {
		options.Progress(DeployProgress{Stage: stage, Message: message})
	}
}

type d1_api_client struct {
	base_url    string
	http_client *http.Client
	account_id  string
	auth_token  string
}

type d1_response struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result []struct {
		Meta struct {
			ChangedDB bool    `json:"changed_db"`
			Changes   int     `json:"changes"`
			Duration  float64 `json:"duration"`
		} `json:"meta"`
		Results []map[string]any `json:"results"`
		Success bool             `json:"success"`
	} `json:"result"`
}

func (c *d1_api_client) database_endpoint() string {
	return c.base_url + "/accounts/" + url.PathEscape(c.account_id) + "/d1/database"
}

func (c *d1_api_client) do(
	request_context context.Context,
	method string,
	endpoint string,
	payload any,
) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("编码 D1 API 请求失败: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(request_context, method, endpoint, body)
	if err != nil {
		return nil, 0, fmt.Errorf("创建 D1 API 请求失败: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.auth_token)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http_client.Do(request)
	if err != nil {
		return nil, 0, fmt.Errorf("调用 D1 API 失败: %w", err)
	}
	defer response.Body.Close()
	response_body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("读取 D1 API 响应失败: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response_body, response.StatusCode, d1_response_error(response.StatusCode, response_body)
	}
	return response_body, response.StatusCode, nil
}

func (c *d1_api_client) find_database_by_name(
	request_context context.Context,
	database_name string,
) (string, error) {
	response_body, _, err := c.do(request_context, http.MethodGet, c.database_endpoint(), nil)
	if err != nil {
		return "", err
	}
	var response struct {
		Success bool `json:"success"`
		Result  []struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response_body, &response); err != nil {
		return "", fmt.Errorf("解析 D1 数据库列表失败: %w", err)
	}
	if !response.Success {
		return "", errors.New("Cloudflare D1 数据库列表返回 success=false")
	}
	for _, database := range response.Result {
		if database.Name == database_name {
			return database.UUID, nil
		}
	}
	return "", nil
}

func (c *d1_api_client) create_database(
	request_context context.Context,
	database_name string,
) (string, error) {
	response_body, _, err := c.do(
		request_context,
		http.MethodPost,
		c.database_endpoint(),
		map[string]string{"name": database_name},
	)
	if err != nil {
		return "", err
	}
	var response struct {
		Success bool `json:"success"`
		Result  struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response_body, &response); err != nil {
		return "", fmt.Errorf("解析 D1 数据库创建响应失败: %w", err)
	}
	if !response.Success || strings.TrimSpace(response.Result.UUID) == "" {
		return "", errors.New("Cloudflare D1 创建数据库返回了空 UUID")
	}
	return response.Result.UUID, nil
}

func (c *d1_api_client) verify_database(
	request_context context.Context,
	database_id string,
) error {
	endpoint := c.database_endpoint() + "/" + url.PathEscape(database_id)
	response_body, _, err := c.do(request_context, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	var response struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(response_body, &response); err != nil {
		return fmt.Errorf("解析 D1 数据库验证响应失败: %w", err)
	}
	if !response.Success {
		return errors.New("Cloudflare D1 数据库验证返回 success=false")
	}
	return nil
}

func (c *d1_api_client) query(
	request_context context.Context,
	database_id string,
	sql string,
	params []any,
) (*d1_response, error) {
	if params == nil {
		params = []any{}
	}
	endpoint := c.database_endpoint() + "/" + url.PathEscape(database_id) + "/query"
	response_body, status_code, err := c.do(
		request_context,
		http.MethodPost,
		endpoint,
		map[string]any{"sql": sql, "params": params},
	)
	if err != nil {
		return nil, err
	}
	var response d1_response
	if err := json.Unmarshal(response_body, &response); err != nil {
		return nil, fmt.Errorf("解析 D1 查询响应失败: %w", err)
	}
	if !response.Success {
		return nil, d1_response_error(status_code, response_body)
	}
	return &response, nil
}

func run_migrations(
	request_context context.Context,
	d1_client *d1_api_client,
	database_id string,
) error {
	if _, err := d1_client.query(request_context, database_id, `CREATE TABLE IF NOT EXISTS d1_migrations (
		id INTEGER PRIMARY KEY,
		name TEXT,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`, nil); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}
	applied_response, err := d1_client.query(
		request_context,
		database_id,
		"SELECT id FROM d1_migrations",
		nil,
	)
	if err != nil {
		return fmt.Errorf("查询已执行迁移失败: %w", err)
	}
	applied := make(map[int]bool)
	if len(applied_response.Result) > 0 {
		for _, row := range applied_response.Result[0].Results {
			if migration_id, ok := row["id"].(float64); ok {
				applied[int(migration_id)] = true
			}
		}
	}

	entries, err := fs.ReadDir(migration_files, "migrations")
	if err != nil {
		return fmt.Errorf("读取嵌入的迁移文件失败: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		var migration_id int
		if _, err := fmt.Sscanf(entry.Name(), "%d_", &migration_id); err != nil {
			continue
		}
		if applied[migration_id] {
			continue
		}
		content, err := migration_files.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("读取迁移文件 %s 失败: %w", entry.Name(), err)
		}
		migration_name := strings.ReplaceAll(entry.Name(), "'", "''")
		full_sql := string(content) + fmt.Sprintf(
			"\nINSERT INTO d1_migrations (id, name) VALUES (%d, '%s');",
			migration_id,
			migration_name,
		)
		if _, err := d1_client.query(request_context, database_id, full_sql, nil); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", entry.Name(), err)
		}
	}
	return nil
}

func d1_response_error(status_code int, response_body []byte) error {
	var response d1_response
	if err := json.Unmarshal(response_body, &response); err == nil && len(response.Errors) > 0 {
		messages := make([]string, 0, len(response.Errors))
		for _, api_error := range response.Errors {
			message := fmt.Sprintf("[%d] %s", api_error.Code, api_error.Message)
			if api_error.Code == 7500 || strings.Contains(api_error.Message, "SQLITE_AUTH") {
				message += "（请检查 Account → D1:Edit 权限及数据库所属账户）"
			}
			messages = append(messages, message)
		}
		return fmt.Errorf("D1 API 返回 %d: %s", status_code, strings.Join(messages, "; "))
	}
	message := strings.TrimSpace(string(response_body))
	if message == "" {
		message = http.StatusText(status_code)
	}
	return fmt.Errorf("D1 API 返回 %d: %s", status_code, message)
}
