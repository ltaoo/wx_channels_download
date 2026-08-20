package bridge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"wx_channel/pkg/cloudflare/durableobjects"
	"wx_channel/pkg/cloudflare/pages"
	"wx_channel/pkg/cloudflare/worker"
)

const (
	default_api_base_url            = "https://api.cloudflare.com/client/v4"
	default_worker_name             = "dm-bridge"
	worker_compatibility_date       = "2026-05-03"
	worker_class_name               = "BridgeDurableObject"
	worker_binding_name             = "BRIDGES"
	bridge_token_binding_name       = "BRIDGE_TOKEN"
	bridge_admin_token_binding_name = "BRIDGE_ADMIN_TOKEN"
	worker_main_module              = "bridge.js"
	pages_compatibility_date        = "2026-08-19"
	pages_production_branch         = "main"
	pages_service_binding           = "BRIDGE"
	worker_deploy_timeout           = 2 * time.Minute
	worker_subdomain_timeout        = 30 * time.Second
	pages_build_timeout             = 30 * time.Second
	pages_deploy_timeout            = 5 * time.Minute
	cloudflare_http_request_timeout = 2 * time.Minute
)

// DeployStage identifies the current Bridge deployment phase.
type DeployStage string

const (
	DeployStageWorker          DeployStage = "worker"
	DeployStageWorkerSubdomain DeployStage = "worker_subdomain"
	DeployStagePagesBuild      DeployStage = "pages_build"
	DeployStagePagesDeploy     DeployStage = "pages_deploy"
)

// DeployProgress is emitted before each Bridge deployment phase.
type DeployProgress struct {
	Stage   DeployStage
	Message string
}

// DeployOptions contains the Cloudflare credentials and Bridge deployment names.
type DeployOptions struct {
	AccountID        string
	AuthToken        string
	WorkerName       string
	PagesProjectName string
	BridgeToken      string
	AdminToken       string
	RepositoryDir    string
	APIBaseURL       string
	HTTPClient       *http.Client
	Progress         func(DeployProgress)
}

// DeployResult describes both the Worker and Pages deployment.
type DeployResult struct {
	WorkerID           string
	WorkerName         string
	WorkerURL          string
	WorkerURLWarning   string
	ScriptBytes        int
	PagesProjectName   string
	PagesURL           string
	PagesDeploymentID  string
	PagesDeploymentURL string
	PagesFiles         int
}

// Deploy uploads the Bridge Worker, builds the admin assets, and deploys the
// Cloudflare Pages management project.
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
	worker_api_client := worker.NewClient(worker.ClientOptions{
		BaseURL:    options.APIBaseURL,
		HTTPClient: http_client,
	})
	pages_api_client := pages.NewClient(pages.ClientOptions{
		BaseURL:    options.APIBaseURL,
		HTTPClient: http_client,
	})

	notify_progress(options, DeployStageWorker, "正在部署 Bridge Worker...")
	worker_context, cancel_worker := context.WithTimeout(request_context, worker_deploy_timeout)
	worker_result, err := durableobjects.Deploy(worker_context, durableobjects.DeployOptions{
		AccountID:         options.AccountID,
		AuthToken:         options.AuthToken,
		WorkerName:        options.WorkerName,
		ScriptContent:     []byte(BridgeWorkerJavaScript()),
		CompatibilityDate: worker_compatibility_date,
		MainModule:        worker_main_module,
		DurableObjects: []durableobjects.DurableObject{
			{
				BindingName: worker_binding_name,
				ClassName:   worker_class_name,
				Storage:     "sqlite",
			},
		},
		Secrets: map[string]string{
			bridge_token_binding_name:       options.BridgeToken,
			bridge_admin_token_binding_name: options.AdminToken,
		},
		EnableSubdomain: true,
		APIClient:       worker_api_client,
	})
	cancel_worker()
	if err != nil {
		return nil, fmt.Errorf("部署 Bridge Worker 失败: %w", err)
	}

	result := &DeployResult{
		WorkerID:         worker_result.WorkerID,
		WorkerName:       worker_result.WorkerName,
		WorkerURL:        fmt.Sprintf("https://%s.<your-subdomain>.workers.dev", worker_result.WorkerName),
		ScriptBytes:      worker_result.ScriptBytes,
		PagesProjectName: options.PagesProjectName,
	}

	notify_progress(options, DeployStageWorkerSubdomain, "正在获取 Worker 访问地址...")
	subdomain_context, cancel_subdomain := context.WithTimeout(request_context, worker_subdomain_timeout)
	subdomain, subdomain_err := worker_api_client.GetSubdomain(
		subdomain_context,
		options.AccountID,
		options.AuthToken,
	)
	cancel_subdomain()
	if subdomain_err != nil {
		result.WorkerURLWarning = fmt.Sprintf("获取 Worker 子域名失败: %v", subdomain_err)
	} else {
		result.WorkerURL = fmt.Sprintf("https://%s.%s.workers.dev", worker_result.WorkerName, subdomain)
	}

	admin_dir := filepath.Join(options.RepositoryDir, "internal", "workers", "bridge", "admin")
	notify_progress(options, DeployStagePagesBuild, "正在准备 Bridge Pages 静态资源...")
	build_context, cancel_build := context.WithTimeout(request_context, pages_build_timeout)
	err = build_pages_assets(build_context, admin_dir)
	cancel_build()
	if err != nil {
		return result, fmt.Errorf("Bridge Worker 已部署，但 Pages 管理页面构建失败: %w", err)
	}

	notify_progress(options, DeployStagePagesDeploy, "正在通过 Cloudflare API 部署 Bridge Pages...")
	pages_context, cancel_pages := context.WithTimeout(request_context, pages_deploy_timeout)
	pages_result, err := pages_api_client.DeployProject(pages_context, pages.DeployOptions{
		AccountID:         options.AccountID,
		AuthToken:         options.AuthToken,
		ProjectName:       options.PagesProjectName,
		ProductionBranch:  pages_production_branch,
		CompatibilityDate: pages_compatibility_date,
		Directory:         filepath.Join(admin_dir, "dist"),
		Secrets: map[string]string{
			bridge_admin_token_binding_name: options.AdminToken,
		},
		Services: map[string]pages.ServiceBinding{
			pages_service_binding: {Service: worker_result.WorkerName},
		},
	})
	cancel_pages()
	if err != nil {
		return result, fmt.Errorf("Bridge Worker 已部署，但 Pages 管理页面部署失败: %w", err)
	}

	result.PagesProjectName = pages_result.ProjectName
	result.PagesURL = pages_result.ProjectURL
	result.PagesDeploymentID = pages_result.DeploymentID
	result.PagesDeploymentURL = pages_result.DeploymentURL
	result.PagesFiles = pages_result.Files
	return result, nil
}

func normalize_deploy_options(options DeployOptions) (DeployOptions, error) {
	options.AccountID = strings.TrimSpace(options.AccountID)
	options.AuthToken = strings.TrimSpace(options.AuthToken)
	options.WorkerName = strings.TrimSpace(options.WorkerName)
	options.PagesProjectName = strings.TrimSpace(options.PagesProjectName)
	options.BridgeToken = strings.TrimSpace(options.BridgeToken)
	options.AdminToken = strings.TrimSpace(options.AdminToken)
	options.RepositoryDir = strings.TrimSpace(options.RepositoryDir)
	options.APIBaseURL = strings.TrimRight(strings.TrimSpace(options.APIBaseURL), "/")
	if options.APIBaseURL == "" {
		options.APIBaseURL = default_api_base_url
	}
	if options.WorkerName == "" {
		options.WorkerName = default_worker_name
	}
	if options.PagesProjectName == "" {
		options.PagesProjectName = options.WorkerName + "-admin"
	}
	if options.AccountID == "" {
		return options, errors.New("未配置 cloudflare.accountId")
	}
	if options.AuthToken == "" {
		return options, errors.New("未配置 cloudflare.apiToken")
	}
	if options.BridgeToken == "" {
		return options, errors.New("未配置 bridge.deploy.token；该值会作为 Worker 的 BRIDGE_TOKEN secret")
	}
	if options.AdminToken == "" {
		return options, errors.New("未配置 bridge.deploy.adminToken；该值用于保护 Bridge 管理页面")
	}
	if options.AdminToken == options.BridgeToken {
		return options, errors.New("bridge.deploy.adminToken 不能与 bridge.deploy.token 相同")
	}
	if options.RepositoryDir == "" {
		return options, errors.New("项目根目录不能为空")
	}
	return options, nil
}

func notify_progress(options DeployOptions, stage DeployStage, message string) {
	if options.Progress != nil {
		options.Progress(DeployProgress{Stage: stage, Message: message})
	}
}

func build_pages_assets(request_context context.Context, admin_dir string) error {
	build_script := filepath.Join(admin_dir, "build.sh")
	build_command := exec.CommandContext(request_context, build_script)
	build_command.Dir = admin_dir
	build_output, err := build_command.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(build_output))
	if message != "" {
		return fmt.Errorf("%w: %s", err, message)
	}
	return err
}
