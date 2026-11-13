package mcpserver

import (
	"context"
	_ "embed"

	mcp "wx_channel/pkg/mcp"
)

//go:embed skills/sync-downloads/SKILL.md
var sync_skill_text string

const sync_skill_uri = "dm://skills/sync-downloads"

func (s *ToolSet) get_sync_skill(ctx context.Context) (map[string]any, error) {
	return successful_tool_result(map[string]any{"name": "sync-downloads", "uri": sync_skill_uri, "mime_type": "text/markdown", "instructions": sync_skill_text})
}

func (s *ToolSet) register_skills(server *mcp.Server) {
	if s.api_client == nil {
		return
	}
	server.AddResource(mcp.Resource{URI: sync_skill_uri, Name: "sync-downloads", Title: "同步已有下载", Description: "通过 MCP 将选定下载记录及已有文件同步到另一台下载器或 NAS。", MimeType: "text/markdown", Text: sync_skill_text})
}
