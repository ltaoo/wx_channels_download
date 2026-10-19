package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wx_channel/pkg/flowengine/engine"
)

const default_api_output_key = "api_response"

type APICallNode struct {
	Id     string
	Config map[string]interface{}
}

func NewAPICallNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &APICallNode{Id: id, Config: config}
}

func (n *APICallNode) ID() string   { return n.Id }
func (n *APICallNode) Type() string { return "APICallNode" }

func (n *APICallNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	resolved_url, err := resolve_service_template(n.Config["url"], ctx)
	if err != nil {
		return false, nil, err
	}
	request_url, _ := resolved_url.(string)
	parsed_url, err := url.ParseRequestURI(request_url)
	if err != nil || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
		return false, nil, fmt.Errorf("url must be an absolute HTTP(S) URL")
	}
	method, _ := n.Config["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
	default:
		return false, nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}
	keys, err := api_call_keys(n.Config["keys"])
	if err != nil {
		return false, nil, err
	}
	values := map[string]interface{}{}
	for name, path := range keys {
		value, err := resolve_scope_path(ctx, path)
		if err != nil {
			return false, nil, err
		}
		values[name] = value
	}
	query := parsed_url.Query()
	if method == "GET" || method == "HEAD" {
		for name, value := range values {
			query.Set(name, fmt.Sprint(value))
		}
	}
	if raw_query, ok := n.Config["query"].(map[string]interface{}); ok {
		for name, value := range raw_query {
			resolved, err := resolve_service_template(value, ctx)
			if err != nil {
				return false, nil, err
			}
			query.Set(name, fmt.Sprint(resolved))
		}
	}
	parsed_url.RawQuery = query.Encode()
	var body io.Reader
	content_type := "application/json"
	if method != "GET" && method != "HEAD" {
		raw_body, has_body := n.Config["body"]
		if !has_body {
			raw_body = values
		}
		resolved_body, err := resolve_service_template(raw_body, ctx)
		if err != nil {
			return false, nil, err
		}
		body_type, _ := n.Config["body_type"].(string)
		switch body_type {
		case "", "json":
			raw, err := json.Marshal(resolved_body)
			if err != nil {
				return false, nil, err
			}
			body = bytes.NewReader(raw)
		case "form":
			fields, ok := resolved_body.(map[string]interface{})
			if !ok {
				return false, nil, fmt.Errorf("form body must be an object")
			}
			form := url.Values{}
			for name, value := range fields {
				form.Set(name, fmt.Sprint(value))
			}
			body = strings.NewReader(form.Encode())
			content_type = "application/x-www-form-urlencoded"
		case "raw":
			text, ok := resolved_body.(string)
			if !ok {
				return false, nil, fmt.Errorf("raw body must be a string")
			}
			body = strings.NewReader(text)
			content_type = "text/plain"
		default:
			return false, nil, fmt.Errorf("unsupported body_type: %s", body_type)
		}
	}
	request_context := ctx.ExecutionContext()
	timeout := service_timeout_seconds(n.Config)
	if timeout == 0 {
		timeout = 15
	}
	request_context, cancel := context.WithTimeout(request_context, time.Duration(timeout)*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(request_context, method, parsed_url.String(), body)
	if err != nil {
		return false, nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", content_type)
	}
	if headers, ok := n.Config["headers"].(map[string]interface{}); ok {
		for name, value := range headers {
			resolved, err := resolve_service_template(value, ctx)
			if err != nil {
				return false, nil, err
			}
			request.Header.Set(name, fmt.Sprint(resolved))
		}
	}
	if auth, ok := n.Config["authentication"].(map[string]interface{}); ok {
		resolved, err := resolve_service_template(auth, ctx)
		if err != nil {
			return false, nil, err
		}
		fields := resolved.(map[string]interface{})
		switch fields["type"] {
		case "bearer":
			token, _ := fields["token"].(string)
			request.Header.Set("Authorization", "Bearer "+token)
		case "basic":
			username, _ := fields["username"].(string)
			password, _ := fields["password"].(string)
			request.SetBasicAuth(username, password)
		case "none", nil, "":
		default:
			return false, nil, fmt.Errorf("unsupported authentication type")
		}
	}
	response, err := (&http.Client{}).Do(request)
	if err != nil {
		return false, nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
	if err != nil {
		return false, nil, err
	}
	if len(raw) > 8*1024*1024 {
		return false, nil, fmt.Errorf("HTTP response exceeds 8 MiB")
	}
	var output interface{}
	if json.Unmarshal(raw, &output) != nil {
		output = string(raw)
	}
	output_key, _ := n.Config["output_key"].(string)
	if output_key == "" {
		output_key = default_api_output_key
	}
	ctx.SetOutput(n.Id, output_key, output)
	ctx.SetOutput(n.Id, output_key+"_status", response.StatusCode)
	ctx.SetOutput(n.Id, output_key+"_headers", response.Header)
	fail_on_status, configured := n.Config["fail_on_status"].(bool)
	if (!configured || fail_on_status) && (response.StatusCode < 200 || response.StatusCode >= 300) {
		return false, nil, fmt.Errorf("HTTP request returned status %d", response.StatusCode)
	}
	return true, ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id), nil
}

// api_call_keys normalizes config["keys"] into {request parameter: scope path}.
// Both map[string]string and the JSON-decoded map[string]interface{} shape are
// accepted; the legacy list form is rejected.
func api_call_keys(value interface{}) (map[string]string, error) {
	switch configured := value.(type) {
	case nil:
		return map[string]string{}, nil
	case map[string]string:
		return configured, nil
	case map[string]interface{}:
		keys := make(map[string]string, len(configured))
		for parameter_name, raw_path := range configured {
			path, ok := raw_path.(string)
			if !ok || strings.TrimSpace(path) == "" {
				return nil, fmt.Errorf("keys.%s must be a scope path string", parameter_name)
			}
			keys[parameter_name] = strings.TrimSpace(path)
		}
		return keys, nil
	default:
		return nil, fmt.Errorf("keys must be an object mapping request parameter names to scope paths")
	}
}

// resolve_scope_path resolves a keys value: a {{...}} template is rendered by
// the template resolver, a bare dotted path is looked up directly in the three
// scopes.
func resolve_scope_path(ctx *engine.ProcessContext, path string) (any, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("empty scope path")
	}
	if strings.Contains(path, "{{") {
		return resolve_service_template(path, ctx)
	}
	return service_template_lookup(ctx, path)
}
