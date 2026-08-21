package pages

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// ServiceBinding describes a Pages Worker service binding.
type ServiceBinding struct {
	Service     string `json:"service"`
	Environment string `json:"environment,omitempty"`
	Entrypoint  string `json:"entrypoint,omitempty"`
}

// Project is the subset of a Cloudflare Pages project used during deployment.
type Project struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Subdomain        string `json:"subdomain"`
	ProductionBranch string `json:"production_branch"`
	DeploymentConfig struct {
		Production project_environment `json:"production"`
		Preview    project_environment `json:"preview"`
	} `json:"deployment_configs"`
}

type project_environment struct {
	WranglerConfigHash string `json:"wrangler_config_hash,omitempty"`
}

type project_variable struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type project_deployment_config struct {
	CompatibilityDate  string                      `json:"compatibility_date,omitempty"`
	CompatibilityFlags []string                    `json:"compatibility_flags,omitempty"`
	EnvVars            map[string]project_variable `json:"env_vars,omitempty"`
	Services           map[string]ServiceBinding   `json:"services,omitempty"`
	WranglerConfigHash string                      `json:"wrangler_config_hash,omitempty"`
}

// EnsureProjectOptions contains the desired configuration for a Pages project.
type EnsureProjectOptions struct {
	AccountID          string
	AuthToken          string
	ProjectName        string
	ProductionBranch   string
	CompatibilityDate  string
	CompatibilityFlags []string
	Secrets            map[string]string
	Services           map[string]ServiceBinding
}

// EnsureProject creates the Pages project when absent and updates its
// production and preview bindings when it already exists.
func (c *Client) EnsureProject(
	request_context context.Context,
	options EnsureProjectOptions,
) (*Project, error) {
	if err := validate_project_options(options); err != nil {
		return nil, err
	}
	project, exists, err := c.GetProject(
		request_context,
		options.AccountID,
		options.AuthToken,
		options.ProjectName,
	)
	if err != nil {
		return nil, err
	}
	production_hash := ""
	preview_hash := ""
	if exists {
		production_hash = project.DeploymentConfig.Production.WranglerConfigHash
		preview_hash = project.DeploymentConfig.Preview.WranglerConfigHash
	}
	payload := struct {
		Name              string `json:"name,omitempty"`
		ProductionBranch  string `json:"production_branch"`
		DeploymentConfigs struct {
			Production project_deployment_config `json:"production"`
			Preview    project_deployment_config `json:"preview"`
		} `json:"deployment_configs"`
	}{
		Name:             options.ProjectName,
		ProductionBranch: options.ProductionBranch,
	}
	payload.DeploymentConfigs.Production = build_project_deployment_config(options, production_hash)
	payload.DeploymentConfigs.Preview = build_project_deployment_config(options, preview_hash)

	var updated Project
	method := http.MethodPatch
	endpoint := project_endpoint(options.AccountID, options.ProjectName)
	if !exists {
		method = http.MethodPost
		endpoint = projects_endpoint(options.AccountID)
	}
	if _, err := c.request_json(
		request_context,
		method,
		endpoint,
		options.AuthToken,
		payload,
		&updated,
	); err != nil {
		action := "更新"
		if !exists {
			action = "创建"
		}
		return nil, fmt.Errorf("%s Pages 项目 %s 失败: %w", action, options.ProjectName, err)
	}
	return &updated, nil
}

// GetProject returns exists=false when the Pages project is absent.
func (c *Client) GetProject(
	request_context context.Context,
	account_id string,
	auth_token string,
	project_name string,
) (*Project, bool, error) {
	var project Project
	status_code, err := c.request_json(
		request_context,
		http.MethodGet,
		project_endpoint(account_id, project_name),
		auth_token,
		nil,
		&project,
	)
	if status_code == http.StatusNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("查询 Pages 项目 %s 失败: %w", project_name, err)
	}
	return &project, true, nil
}

func build_project_deployment_config(
	options EnsureProjectOptions,
	wrangler_config_hash string,
) project_deployment_config {
	variables := make(map[string]project_variable, len(options.Secrets))
	for name, value := range options.Secrets {
		variables[name] = project_variable{Type: "secret_text", Value: value}
	}
	return project_deployment_config{
		CompatibilityDate:  options.CompatibilityDate,
		CompatibilityFlags: append([]string(nil), options.CompatibilityFlags...),
		EnvVars:            variables,
		Services:           options.Services,
		WranglerConfigHash: wrangler_config_hash,
	}
}

func validate_project_options(options EnsureProjectOptions) error {
	if strings.TrimSpace(options.AccountID) == "" {
		return errors.New("Cloudflare account id is required")
	}
	if strings.TrimSpace(options.AuthToken) == "" {
		return errors.New("Cloudflare auth token is required")
	}
	if strings.TrimSpace(options.ProjectName) == "" {
		return errors.New("Pages project name is required")
	}
	if strings.TrimSpace(options.ProductionBranch) == "" {
		return errors.New("Pages production branch is required")
	}
	return nil
}

func projects_endpoint(account_id string) string {
	return "/accounts/" + url.PathEscape(account_id) + "/pages/projects"
}

func project_endpoint(account_id string, project_name string) string {
	return projects_endpoint(account_id) + "/" + url.PathEscape(project_name)
}

func (c *Client) fetch_upload_token(
	request_context context.Context,
	account_id string,
	auth_token string,
	project_name string,
) (string, error) {
	var result struct {
		JWT string `json:"jwt"`
	}
	if _, err := c.request_json(
		request_context,
		http.MethodGet,
		project_endpoint(account_id, project_name)+"/upload-token",
		auth_token,
		nil,
		&result,
	); err != nil {
		return "", fmt.Errorf("获取 Pages 上传凭证失败: %w", err)
	}
	if strings.TrimSpace(result.JWT) == "" {
		return "", errors.New("Cloudflare Pages 返回了空的上传凭证")
	}
	return result.JWT, nil
}

func (c *Client) fetch_missing_files(
	request_context context.Context,
	hashes []string,
	jwt string,
) ([]string, error) {
	var missing []string
	if _, err := c.request_json(
		request_context,
		http.MethodPost,
		"/pages/assets/check-missing",
		jwt,
		map[string]any{"hashes": hashes},
		&missing,
	); err != nil {
		return nil, fmt.Errorf("检查 Pages 缺失资源失败: %w", err)
	}
	return missing, nil
}

func (c *Client) upload_batch(
	request_context context.Context,
	files []FilePayloadToUpload,
	jwt string,
) error {
	if _, err := c.request_json(
		request_context,
		http.MethodPost,
		"/pages/assets/upload",
		jwt,
		files,
		nil,
	); err != nil {
		return fmt.Errorf("上传 Pages 资源失败: %w", err)
	}
	return nil
}

func (c *Client) upsert_hashes(
	request_context context.Context,
	hashes []string,
	jwt string,
) error {
	if _, err := c.request_json(
		request_context,
		http.MethodPost,
		"/pages/assets/upsert-hashes",
		jwt,
		map[string]any{"hashes": hashes},
		nil,
	); err != nil {
		return fmt.Errorf("更新 Pages 资源哈希失败: %w", err)
	}
	return nil
}

// The Api_* helpers are kept for compatibility with earlier callers. New code
// should use Client methods so the API token and request context are explicit.
func Api_fetch_missing_files(hashes []string, jwt string) ([]string, error) {
	return NewClient(ClientOptions{}).fetch_missing_files(context.Background(), hashes, jwt)
}

func Api_upload(files []FilePayloadToUpload, jwt string) (string, error) {
	err := NewClient(ClientOptions{}).upload_batch(context.Background(), files, jwt)
	return "", err
}

func Api_upsert_hashes(hashes []string, jwt string) (string, error) {
	err := NewClient(ClientOptions{}).upsert_hashes(context.Background(), hashes, jwt)
	return "", err
}

type UploadTokenResp struct {
	Result struct {
		JWT string `json:"jwt"`
	} `json:"result"`
	Success bool `json:"success"`
}

func Api_fetch_upload_token(account_id string, project_name string) (*UploadTokenResp, error) {
	auth_token := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	if auth_token == "" {
		return nil, errors.New("CLOUDFLARE_API_TOKEN is required")
	}
	jwt, err := NewClient(ClientOptions{}).fetch_upload_token(
		context.Background(),
		account_id,
		auth_token,
		project_name,
	)
	if err != nil {
		return nil, err
	}
	response := &UploadTokenResp{Success: true}
	response.Result.JWT = jwt
	return response, nil
}
