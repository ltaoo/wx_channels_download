package mcpserver

import (
	"context"
	"encoding/json"
)

// SphDeployResult describes a deployed video-channel query Worker.
type SphDeployResult struct {
	WorkerID         string `json:"worker_id"`
	WorkerName       string `json:"worker_name"`
	WorkerURL        string `json:"worker_url"`
	WorkerURLWarning string `json:"worker_url_warning,omitempty"`
	ScriptBytes      int    `json:"script_bytes"`
}

// SphDeployer deploys the video-channel query Worker using application-owned
// credentials. Sensitive deployment settings are intentionally not MCP tool
// arguments.
type SphDeployer interface {
	DeploySphWorker(ctx context.Context) (*SphDeployResult, error)
}

func sph_tool_definitions() []any {
	return []any{
		map[string]any{
			"name":        "deploy_sph_worker",
			"title":       "部署视频号查询 Worker",
			"description": "读取应用配置中的 cloudflare.accountId、cloudflare.apiToken、cloudflare.sphWorkerName、cloudflare.sphCookie 和 cloudflare.sphCredential，部署或覆盖 Cloudflare 视频号查询 Worker，并返回 workers.dev 地址。调用前应获得用户确认；get_config 可用时，可先用它确认相关配置均已设置。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			},
			"annotations": map[string]any{
				"readOnlyHint":    false,
				"destructiveHint": true,
				"idempotentHint":  false,
				"openWorldHint":   true,
			},
		},
	}
}

func (s *Server) deploy_sph_worker(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments struct{}
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	result, err := s.sph_deployer.DeploySphWorker(ctx)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(result)
}
