package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type create_scraper_job_arguments struct {
	URL          string `json:"url"`
	ForceRefresh bool   `json:"force_refresh"`
}

type get_scraper_job_arguments struct {
	ID string `json:"id"`
}

func scraper_job_tool_definitions() []any {
	return []any{
		map[string]any{
			"name":        "create_scraper_job",
			"title":       "创建页面抓取任务",
			"description": "创建异步页面抓取任务并立即返回任务快照。使用 get_scraper_job 查询状态和结果；需要直接等待结果时可使用 fetch_content。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"format":      "uri",
						"description": "要抓取的平台或普通页面链接。",
					},
					"force_refresh": map[string]any{
						"type":        "boolean",
						"default":     false,
						"description": "忽略抓取缓存并重新获取。",
					},
				},
				"required": []string{"url"},
			},
			"annotations": map[string]any{
				"readOnlyHint":    true,
				"destructiveHint": false,
				"idempotentHint":  false,
				"openWorldHint":   true,
			},
		},
		map[string]any{
			"name":        "get_scraper_job",
			"title":       "获取页面抓取任务",
			"description": "按任务 ID 查询抓取状态、进度和完成后的输出。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "string",
						"description": "create_scraper_job 返回的任务 ID。",
					},
				},
				"required": []string{"id"},
			},
			"annotations": map[string]any{
				"readOnlyHint":    true,
				"destructiveHint": false,
				"idempotentHint":  true,
				"openWorldHint":   false,
			},
		},
	}
}

func (s *Server) create_scraper_job_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments create_scraper_job_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.URL = strings.TrimSpace(arguments.URL)
	if err := validate_source_url(arguments.URL); err != nil {
		return nil, err
	}
	job, err := s.create_scraper_job(ctx, arguments.URL, arguments.ForceRefresh)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(job)
}

func (s *Server) get_scraper_job_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments get_scraper_job_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.ID = strings.TrimSpace(arguments.ID)
	if arguments.ID == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	job, err := s.get_scraper_job(ctx, arguments.ID)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(job)
}
