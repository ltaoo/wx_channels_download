package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	u, _ := n.Config["url"].(string)
	if u == "" {
		return false, nil, fmt.Errorf("missing url")
	}
	method := "GET"
	if v, ok := n.Config["method"].(string); ok && v != "" {
		method = v
	}
	// keys maps the outgoing request parameter name to the scope path its
	// value is read from, e.g. {"username": "output.cleanup.username"}. An
	// object (rather than a list of context keys) lets the request parameter
	// name differ from the producer's key name.
	keys, err := api_call_keys(n.Config["keys"])
	if err != nil {
		return false, nil, err
	}
	values := make(map[string]interface{}, len(keys))
	for parameter_name, path := range keys {
		resolved, err := resolve_scope_path(ctx, path)
		if err != nil {
			return false, nil, fmt.Errorf("keys.%s: %w", parameter_name, err)
		}
		values[parameter_name] = resolved
	}
	var req *http.Request
	if method == "GET" {
		r, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return false, nil, err
		}
		q := r.URL.Query()
		for parameter_name, value := range values {
			q.Set(parameter_name, fmt.Sprint(value))
		}
		r.URL.RawQuery = q.Encode()
		req = r
	} else {
		b, err := json.Marshal(values)
		if err != nil {
			return false, nil, err
		}
		r, err := http.NewRequest(method, u, bytes.NewReader(b))
		if err != nil {
			return false, nil, err
		}
		r.Header.Set("Content-Type", "application/json")
		req = r
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, nil, err
	}
	var out interface{}
	var jm map[string]interface{}
	if err := json.Unmarshal(body, &jm); err == nil {
		out = jm
	} else {
		out = string(body)
	}
	outKey := default_api_output_key
	if v, ok := n.Config["output_key"].(string); ok && v != "" {
		outKey = v
	}
	ctx.Data[outKey] = out
	ctx.Data[outKey+"_status"] = resp.StatusCode
	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
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
