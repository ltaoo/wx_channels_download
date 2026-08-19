package sph

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/cloudflare/worker"
)

const (
	default_api_base_url            = "https://api.cloudflare.com/client/v4"
	worker_compatibility_date       = "2024-01-01"
	worker_main_module              = "worker.js"
	worker_deploy_timeout           = 2 * time.Minute
	worker_subdomain_timeout        = 30 * time.Second
	cloudflare_http_request_timeout = 2 * time.Minute
)

//go:embed worker.js
var worker_javascript []byte

//go:embed index.html
var index_html []byte

// DeployStage identifies the current Sph deployment phase.
type DeployStage string

const (
	DeployStageWorker          DeployStage = "worker"
	DeployStageWorkerSubdomain DeployStage = "worker_subdomain"
)

// DeployProgress is emitted before each Sph deployment phase.
type DeployProgress struct {
	Stage   DeployStage
	Message string
}

// DeployOptions contains the configuration needed to deploy the Sph Worker.
type DeployOptions struct {
	AccountID     string
	AuthToken     string
	WorkerName    string
	Cookie        string
	Credential    string
	RepositoryDir string
	APIBaseURL    string
	HTTPClient    *http.Client
	Progress      func(DeployProgress)
}

// DeployResult identifies the deployed Sph Worker and its workers.dev URL.
type DeployResult struct {
	WorkerID         string
	WorkerName       string
	WorkerURL        string
	WorkerURLWarning string
	ScriptBytes      int
}

// Deploy uploads the embedded Sph Worker, page and icon module, enables its
// workers.dev route, and resolves the account subdomain.
func Deploy(request_context context.Context, options DeployOptions) (*DeployResult, error) {
	normalized_options, err := normalize_deploy_options(options)
	if err != nil {
		return nil, err
	}
	options = normalized_options

	icon_path := filepath.Join(options.RepositoryDir, "build", "icon.png")
	icon_bytes, err := os.ReadFile(icon_path)
	if err != nil {
		return nil, fmt.Errorf("读取 Sph Worker 图标 %s 失败: %w", icon_path, err)
	}
	icon_module := build_icon_module(icon_bytes)

	http_client := options.HTTPClient
	if http_client == nil {
		http_client = &http.Client{Timeout: cloudflare_http_request_timeout}
	}
	api_client := worker.NewClient(worker.ClientOptions{
		BaseURL:    options.APIBaseURL,
		HTTPClient: http_client,
	})

	notify_progress(options, DeployStageWorker, "正在部署视频号查询 Worker...")
	deploy_context, cancel_deploy := context.WithTimeout(request_context, worker_deploy_timeout)
	worker_id, err := api_client.Deploy(deploy_context, worker.DeployBody{
		AccountID:         options.AccountID,
		AuthToken:         options.AuthToken,
		WorkerName:        options.WorkerName,
		ScriptContent:     worker_javascript,
		CompatibilityDate: worker_compatibility_date,
		MainModule:        worker_main_module,
		Bindings: []worker.Binding{
			{Type: "plain_text", Name: "COOKIE", Text: options.Cookie},
			{Type: "plain_text", Name: "ACCESS_CREDENTIAL", Text: options.Credential},
		},
		AdditionalFiles: map[string][]byte{
			"index.html": index_html,
			"icon.js":    icon_module,
		},
	})
	cancel_deploy()
	if err != nil {
		return nil, fmt.Errorf("部署视频号查询 Worker 失败: %w", err)
	}

	result := &DeployResult{
		WorkerID:    worker_id,
		WorkerName:  options.WorkerName,
		WorkerURL:   fmt.Sprintf("https://%s.<your-subdomain>.workers.dev", options.WorkerName),
		ScriptBytes: len(worker_javascript),
	}
	warnings := make([]string, 0, 2)

	subdomain_context, cancel_subdomain := context.WithTimeout(request_context, worker_subdomain_timeout)
	if err := api_client.EnableSubdomain(
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
	subdomain, subdomain_err := api_client.GetSubdomain(
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
	options.Cookie = strings.TrimSpace(options.Cookie)
	options.Credential = strings.TrimSpace(options.Credential)
	options.RepositoryDir = strings.TrimSpace(options.RepositoryDir)
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
		return options, errors.New("未配置 cloudflare.sphWorkerName")
	}
	if options.Credential == "" {
		return options, errors.New("未配置 cloudflare.sphCredential")
	}
	if options.RepositoryDir == "" {
		return options, errors.New("项目根目录不能为空")
	}
	return options, nil
}

func build_icon_module(icon_bytes []byte) []byte {
	icon_base64 := base64.StdEncoding.EncodeToString(icon_bytes)
	return []byte("export default " + strconv.Quote(icon_base64) + ";")
}

func notify_progress(options DeployOptions, stage DeployStage, message string) {
	if options.Progress != nil {
		options.Progress(DeployProgress{Stage: stage, Message: message})
	}
}
