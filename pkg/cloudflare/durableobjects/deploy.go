package durableobjects

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"wx_channel/pkg/cloudflare/worker"
)

// DurableObject describes one namespace binding and its exported class.
// The caller owns all application-specific names and storage choices.
type DurableObject struct {
	BindingName string
	ClassName   string
	ScriptName  string
	Environment string
	Storage     string
}

// DeployOptions contains the generic inputs needed to upload a Worker that
// exports one or more Durable Object classes.
type DeployOptions struct {
	AccountID          string
	AuthToken          string
	WorkerName         string
	ScriptContent      []byte
	CompatibilityDate  string
	CompatibilityFlags []string
	MainModule         string
	AdditionalFiles    map[string][]byte
	DurableObjects     []DurableObject
	Bindings           []worker.Binding
	Exports            map[string]worker.Export
	Secrets            map[string]string
	EnableSubdomain    bool
	APIClient          *worker.Client
}

// DeployResult identifies the uploaded Worker and the number of uploaded
// entry-module bytes.
type DeployResult struct {
	WorkerID    string
	WorkerName  string
	ScriptBytes int
}

// Deploy uploads caller-provided code and declarations, then installs the
// requested secrets and optionally enables the workers.dev route.
func Deploy(request_context context.Context, options DeployOptions) (*DeployResult, error) {
	if err := validate_deploy_options(options); err != nil {
		return nil, err
	}

	bindings := append([]worker.Binding(nil), options.Bindings...)
	exports := make(map[string]worker.Export, len(options.Exports)+len(options.DurableObjects))
	for export_name, worker_export := range options.Exports {
		exports[export_name] = worker_export
	}
	for _, durable_object := range options.DurableObjects {
		bindings = append(bindings, worker.Binding{
			Type:        "durable_object_namespace",
			Name:        strings.TrimSpace(durable_object.BindingName),
			ClassName:   strings.TrimSpace(durable_object.ClassName),
			ScriptName:  strings.TrimSpace(durable_object.ScriptName),
			Environment: strings.TrimSpace(durable_object.Environment),
		})
		exports[strings.TrimSpace(durable_object.ClassName)] = worker.Export{
			Type:    "durable-object",
			Storage: strings.TrimSpace(durable_object.Storage),
		}
	}

	api_client := options.APIClient
	if api_client == nil {
		api_client = worker.NewClient(worker.ClientOptions{})
	}
	worker_name := strings.TrimSpace(options.WorkerName)
	worker_id, err := api_client.Deploy(request_context, worker.DeployBody{
		AccountID:          strings.TrimSpace(options.AccountID),
		AuthToken:          strings.TrimSpace(options.AuthToken),
		WorkerName:         worker_name,
		ScriptContent:      options.ScriptContent,
		CompatibilityDate:  strings.TrimSpace(options.CompatibilityDate),
		CompatibilityFlags: options.CompatibilityFlags,
		Bindings:           bindings,
		Exports:            exports,
		MainModule:         strings.TrimSpace(options.MainModule),
		AdditionalFiles:    options.AdditionalFiles,
	})
	if err != nil {
		return nil, err
	}

	secret_names := make([]string, 0, len(options.Secrets))
	for secret_name := range options.Secrets {
		secret_names = append(secret_names, secret_name)
	}
	sort.Strings(secret_names)
	for _, secret_name := range secret_names {
		if err := api_client.PutSecret(
			request_context,
			strings.TrimSpace(options.AccountID),
			strings.TrimSpace(options.AuthToken),
			worker_name,
			strings.TrimSpace(secret_name),
			options.Secrets[secret_name],
		); err != nil {
			return nil, fmt.Errorf("Worker 已上传，但配置 secret %s 失败: %w", secret_name, err)
		}
	}
	if options.EnableSubdomain {
		if err := api_client.EnableSubdomain(
			request_context,
			strings.TrimSpace(options.AccountID),
			strings.TrimSpace(options.AuthToken),
			worker_name,
		); err != nil {
			return nil, fmt.Errorf("Worker 和 Secrets 已部署，但启用 workers.dev 地址失败: %w", err)
		}
	}

	return &DeployResult{
		WorkerID:    worker_id,
		WorkerName:  worker_name,
		ScriptBytes: len(options.ScriptContent),
	}, nil
}

func validate_deploy_options(options DeployOptions) error {
	if strings.TrimSpace(options.AccountID) == "" {
		return errors.New("Cloudflare account id is required")
	}
	if strings.TrimSpace(options.AuthToken) == "" {
		return errors.New("Cloudflare auth token is required")
	}
	if strings.TrimSpace(options.WorkerName) == "" {
		return errors.New("Worker name is required")
	}
	if len(options.ScriptContent) == 0 {
		return errors.New("Worker script content is required")
	}
	if len(options.DurableObjects) == 0 {
		return errors.New("at least one Durable Object is required")
	}

	binding_names := make(map[string]struct{}, len(options.Bindings)+len(options.DurableObjects))
	for _, binding := range options.Bindings {
		binding_name := strings.TrimSpace(binding.Name)
		if binding_name != "" {
			binding_names[binding_name] = struct{}{}
		}
	}
	class_names := make(map[string]struct{}, len(options.DurableObjects))
	for _, durable_object := range options.DurableObjects {
		binding_name := strings.TrimSpace(durable_object.BindingName)
		class_name := strings.TrimSpace(durable_object.ClassName)
		if binding_name == "" {
			return errors.New("Durable Object binding name is required")
		}
		if class_name == "" {
			return errors.New("Durable Object class name is required")
		}
		if _, exists := binding_names[binding_name]; exists {
			return fmt.Errorf("duplicate binding name %q", binding_name)
		}
		if _, exists := class_names[class_name]; exists {
			return fmt.Errorf("duplicate Durable Object class name %q", class_name)
		}
		if _, exists := options.Exports[class_name]; exists {
			return fmt.Errorf("duplicate export name %q", class_name)
		}
		binding_names[binding_name] = struct{}{}
		class_names[class_name] = struct{}{}
	}
	for secret_name := range options.Secrets {
		if strings.TrimSpace(secret_name) == "" {
			return errors.New("secret name is required")
		}
	}
	return nil
}
