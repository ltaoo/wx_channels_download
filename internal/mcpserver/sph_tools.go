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
