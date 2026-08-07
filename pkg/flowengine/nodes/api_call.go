package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"wx_channel/pkg/flowengine/engine"
)

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
	keys := []string{}
	if v, ok := n.Config["keys"].([]string); ok {
		keys = v
	} else if v, ok := n.Config["keys"].([]interface{}); ok {
		for _, kv := range v {
			if s, ok := kv.(string); ok {
				keys = append(keys, s)
			}
		}
	}
	var req *http.Request
	if method == "GET" {
		r, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return false, nil, err
		}
		q := r.URL.Query()
		for _, k := range keys {
			if val, ok := ctx.Data[k]; ok {
				q.Set(k, fmt.Sprint(val))
			}
		}
		r.URL.RawQuery = q.Encode()
		req = r
	} else {
		payload := map[string]interface{}{}
		for _, k := range keys {
			if val, ok := ctx.Data[k]; ok {
				payload[k] = val
			}
		}
		b, err := json.Marshal(payload)
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
	outKey := "api_response"
	if v, ok := n.Config["output_key"].(string); ok && v != "" {
		outKey = v
	}
	ctx.Data[outKey] = out
	ctx.Data[outKey+"_status"] = resp.StatusCode
	next := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, next, nil
}
