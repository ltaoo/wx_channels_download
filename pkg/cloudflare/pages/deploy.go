package pages

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DeployOptions contains everything required to create or update a direct
// upload Pages project and publish one production deployment.
type DeployOptions struct {
	AccountID          string
	AuthToken          string
	ProjectName        string
	ProductionBranch   string
	CompatibilityDate  string
	CompatibilityFlags []string
	Directory          string
	Secrets            map[string]string
	Services           map[string]ServiceBinding
	UploadJWT          string
}

// DeployResult identifies the Pages project and the created deployment.
type DeployResult struct {
	ProjectName   string
	ProjectURL    string
	DeploymentID  string
	DeploymentURL string
	Files         int
}

// Deployment is the subset of the Pages deployment response used by callers.
type Deployment struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Aliases     []string `json:"aliases"`
	Environment string   `json:"environment"`
}

// DeployProject ensures project settings, uploads static assets, and creates a
// production deployment. If Directory contains _worker.js it is uploaded in
// Pages advanced mode and is applied to every route.
func (c *Client) DeployProject(
	request_context context.Context,
	options DeployOptions,
) (*DeployResult, error) {
	if err := validate_deploy_options(options); err != nil {
		return nil, err
	}
	project, err := c.EnsureProject(request_context, EnsureProjectOptions{
		AccountID:          options.AccountID,
		AuthToken:          options.AuthToken,
		ProjectName:        options.ProjectName,
		ProductionBranch:   options.ProductionBranch,
		CompatibilityDate:  options.CompatibilityDate,
		CompatibilityFlags: options.CompatibilityFlags,
		Secrets:            options.Secrets,
		Services:           options.Services,
	})
	if err != nil {
		return nil, err
	}
	files_map, err := Validate(options.Directory)
	if err != nil {
		return nil, fmt.Errorf("校验 Pages 静态资源失败: %w", err)
	}
	if len(files_map) == 0 {
		return nil, errors.New("Pages 静态资源目录为空")
	}
	upload_jwt := strings.TrimSpace(options.UploadJWT)
	if upload_jwt == "" {
		upload_jwt, err = c.fetch_upload_token(
			request_context,
			options.AccountID,
			options.AuthToken,
			options.ProjectName,
		)
		if err != nil {
			return nil, err
		}
	}
	upload_result, err := c.upload_assets(request_context, files_map, upload_jwt)
	if err != nil {
		return nil, err
	}
	worker_script, err := read_optional_file(filepath.Join(options.Directory, "_worker.js"))
	if err != nil {
		return nil, err
	}
	routes, err := read_optional_file(filepath.Join(options.Directory, "_routes.json"))
	if err != nil {
		return nil, err
	}
	if len(worker_script) > 0 && len(routes) == 0 {
		routes = []byte(`{"version":1,"include":["/*"],"exclude":[]}`)
	}
	deployment, err := c.create_deployment(
		request_context,
		options,
		upload_result.Files,
		worker_script,
		routes,
	)
	if err != nil {
		return nil, err
	}
	return &DeployResult{
		ProjectName:   options.ProjectName,
		ProjectURL:    pages_url(project.Subdomain),
		DeploymentID:  deployment.ID,
		DeploymentURL: deployment.URL,
		Files:         len(files_map),
	}, nil
}

func (c *Client) create_deployment(
	request_context context.Context,
	options DeployOptions,
	manifest map[string]string,
	worker_script []byte,
	routes []byte,
) (*Deployment, error) {
	body, content_type, err := build_deployment_body(
		manifest,
		options.ProductionBranch,
		options.CompatibilityDate,
		options.CompatibilityFlags,
		options.Services,
		worker_script,
		routes,
	)
	if err != nil {
		return nil, err
	}
	response_body, status_code, err := c.request(
		request_context,
		http.MethodPost,
		project_endpoint(options.AccountID, options.ProjectName)+"/deployments",
		options.AuthToken,
		content_type,
		body,
	)
	if err != nil {
		return nil, fmt.Errorf("创建 Pages 部署失败: %w", err)
	}
	if status_code < http.StatusOK || status_code >= http.StatusMultipleChoices {
		return nil, response_error(status_code, response_body)
	}
	var response api_response
	if err := json.Unmarshal(response_body, &response); err != nil {
		return nil, fmt.Errorf("解析 Pages 部署响应失败: %w", err)
	}
	if !response.Success {
		return nil, response_error(status_code, response_body)
	}
	var deployment Deployment
	if err := json.Unmarshal(response.Result, &deployment); err != nil {
		return nil, fmt.Errorf("解析 Pages deployment result 失败: %w", err)
	}
	return &deployment, nil
}

func build_deployment_body(
	manifest map[string]string,
	production_branch string,
	compatibility_date string,
	compatibility_flags []string,
	services map[string]ServiceBinding,
	worker_script []byte,
	routes []byte,
) (*bytes.Buffer, string, error) {
	manifest_json, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", fmt.Errorf("编码 Pages manifest 失败: %w", err)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("manifest", string(manifest_json)); err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("branch", production_branch); err != nil {
		return nil, "", err
	}
	if len(worker_script) > 0 {
		worker_bundle, _, err := build_worker_bundle(
			worker_script,
			compatibility_date,
			compatibility_flags,
			services,
		)
		if err != nil {
			return nil, "", err
		}
		if err := write_multipart_file(
			writer,
			"_worker.bundle",
			"_worker.bundle",
			"application/octet-stream",
			worker_bundle,
		); err != nil {
			return nil, "", err
		}
	}
	if len(routes) > 0 {
		if err := write_multipart_file(
			writer,
			"_routes.json",
			"_routes.json",
			"application/json",
			routes,
		); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("完成 Pages deployment multipart 失败: %w", err)
	}
	return body, writer.FormDataContentType(), nil
}

func build_worker_bundle(
	worker_script []byte,
	compatibility_date string,
	compatibility_flags []string,
	services map[string]ServiceBinding,
) ([]byte, string, error) {
	type worker_binding struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Service     string `json:"service"`
		Environment string `json:"environment,omitempty"`
		Entrypoint  string `json:"entrypoint,omitempty"`
	}
	metadata := struct {
		MainModule         string           `json:"main_module"`
		Bindings           []worker_binding `json:"bindings,omitempty"`
		CompatibilityDate  string           `json:"compatibility_date,omitempty"`
		CompatibilityFlags []string         `json:"compatibility_flags,omitempty"`
	}{
		MainModule:         "_worker.js",
		CompatibilityDate:  compatibility_date,
		CompatibilityFlags: compatibility_flags,
	}
	service_names := make([]string, 0, len(services))
	for service_name := range services {
		service_names = append(service_names, service_name)
	}
	sort.Strings(service_names)
	for _, service_name := range service_names {
		service := services[service_name]
		metadata.Bindings = append(metadata.Bindings, worker_binding{
			Name:        service_name,
			Type:        "service",
			Service:     service.Service,
			Environment: service.Environment,
			Entrypoint:  service.Entrypoint,
		})
	}
	metadata_json, err := json.Marshal(metadata)
	if err != nil {
		return nil, "", fmt.Errorf("编码 Pages Worker metadata 失败: %w", err)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("metadata", string(metadata_json)); err != nil {
		return nil, "", err
	}
	if err := write_multipart_file(
		writer,
		"_worker.js",
		"_worker.js",
		"application/javascript+module",
		worker_script,
	); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("完成 Pages Worker bundle 失败: %w", err)
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func write_multipart_file(
	writer *multipart.Writer,
	field_name string,
	filename string,
	content_type string,
	content []byte,
) error {
	header := make(textproto.MIMEHeader)
	header.Set(
		"Content-Disposition",
		fmt.Sprintf(`form-data; name=%q; filename=%q`, field_name, filename),
	)
	header.Set("Content-Type", content_type)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	return nil
}

func read_optional_file(file_path string) ([]byte, error) {
	content, err := os.ReadFile(file_path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 Pages 文件 %s 失败: %w", file_path, err)
	}
	return content, nil
}

func pages_url(subdomain string) string {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return ""
	}
	if strings.HasPrefix(subdomain, "http://") || strings.HasPrefix(subdomain, "https://") {
		return strings.TrimRight(subdomain, "/")
	}
	return "https://" + strings.TrimRight(subdomain, "/")
}

func validate_deploy_options(options DeployOptions) error {
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
	if strings.TrimSpace(options.Directory) == "" {
		return errors.New("Pages asset directory is required")
	}
	return nil
}

// DeployBody and Deploy preserve the package's original convenience entry
// point while removing its former implicit, hard-coded authentication.
type DeployBody struct {
	Directory         string `json:"directory"`
	JWT               string `json:"jwt"`
	AccountId         string `json:"account_id"`
	AuthToken         string `json:"auth_token"`
	ProjectName       string `json:"project_name"`
	ProductionBranch  string `json:"production_branch"`
	CompatibilityDate string `json:"compatibility_date"`
}

func Deploy(body DeployBody) error {
	_, err := NewClient(ClientOptions{}).DeployProject(context.Background(), DeployOptions{
		AccountID:         body.AccountId,
		AuthToken:         body.AuthToken,
		ProjectName:       body.ProjectName,
		ProductionBranch:  body.ProductionBranch,
		CompatibilityDate: body.CompatibilityDate,
		Directory:         body.Directory,
		UploadJWT:         body.JWT,
	})
	return err
}
