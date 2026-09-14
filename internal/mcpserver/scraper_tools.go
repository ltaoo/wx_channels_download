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
