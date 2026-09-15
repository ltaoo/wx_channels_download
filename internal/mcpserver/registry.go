package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	servicetools "wx_channel/internal/services/tools"
)

// tool_handler executes one tool with its JSON-encoded arguments.
type tool_handler func(*ToolSet, context.Context, json.RawMessage) (map[string]any, error)

// tool_declaration is the single source of truth for one business tool: its
// catalog name, the predicate that decides whether it is available in the
// current runtime, and the handler that executes it.
type tool_declaration struct {
	name     string
	supports func(*ToolSet) bool
	handle   tool_handler
}

// without_arguments adapts a context-only handler to tool_handler.
func without_arguments(handler func(*ToolSet, context.Context) (map[string]any, error)) tool_handler {
	return func(toolset *ToolSet, ctx context.Context, _ json.RawMessage) (map[string]any, error) {
		return handler(toolset, ctx)
	}
}

// Availability predicates. Each body is transcribed verbatim from the
// supports_tool switch this registry replaces, so backend wiring changes are
// never silently reinterpreted.

func supports_api_client(s *ToolSet) bool {
	return s.api_client != nil
}

func supports_scraper(s *ToolSet) bool {
	return s.scraper_jobs != nil || s.api_client != nil
}

func supports_download_content(s *ToolSet) bool {
	return (s.scraper_jobs != nil || s.api_client != nil) && (s.download_task_creator != nil || s.api_client != nil)
}

func supports_wxchannels_download(s *ToolSet) bool {
	return s.api_client != nil && (s.download_task_creator != nil || s.api_client != nil)
}

func supports_data(s *ToolSet) bool {
	return s.data_reader != nil || s.api_client != nil
}

func supports_task_delete(s *ToolSet) bool {
	return s.download_task_deleter != nil
}

func supports_task_create(s *ToolSet) bool {
	return s.download_task_creator != nil || s.api_client != nil
}

func supports_sph(s *ToolSet) bool {
	return s.sph_deployer != nil
}

func supports_zhihu(s *ToolSet) bool {
	return s.zhihu_collections != nil && s.zhihu_credentials != nil
}

func supports_automation(s *ToolSet) bool {
	return s.automation != nil
}

// tool_declarations holds one row per tool in catalog.json declaration order.
// That order is preserved by servicetools.Definitions(), so tools/list and
// `tool list` ordering are unchanged.
var tool_declarations = []tool_declaration{
	{"get_config", supports_api_client, without_arguments((*ToolSet).get_config)},
	{"update_config", supports_api_client, (*ToolSet).update_config},
	{"get_restart_status", supports_api_client, (*ToolSet).get_restart_status},
	{"get_platform_status", supports_api_client, without_arguments((*ToolSet).get_platform_status)},
	{"fetch_content", supports_scraper, (*ToolSet).fetch_content},
	{"download_content", supports_download_content, (*ToolSet).download_content},
	{"decrypt_wxchannels_video", supports_api_client, (*ToolSet).decrypt_wxchannels_video},
	{"create_scraper_job", supports_scraper, (*ToolSet).create_scraper_job_tool},
	{"get_scraper_job", supports_scraper, (*ToolSet).get_scraper_job_tool},
	{"get_wxchannels_status", supports_api_client, without_arguments((*ToolSet).get_wxchannels_status)},
	{"search_wxchannels_accounts", supports_api_client, (*ToolSet).search_wxchannels_accounts},
	{"get_wxchannels_account_videos", supports_api_client, (*ToolSet).get_wxchannels_account_videos},
	{"get_wxchannels_live_replays", supports_api_client, (*ToolSet).get_wxchannels_live_replays},
	{"get_wxchannels_live_profile", supports_api_client, (*ToolSet).get_wxchannels_live_profile},
	{"get_wxchannels_interacted_videos", supports_api_client, (*ToolSet).get_wxchannels_interacted_videos},
	{"get_wxchannels_followed_accounts", supports_api_client, (*ToolSet).get_wxchannels_followed_accounts},
	{"get_wxchannels_play_history", supports_api_client, (*ToolSet).get_wxchannels_play_history},
	{"get_wxchannels_video_profile", supports_api_client, (*ToolSet).get_wxchannels_video_profile},
	{"get_wxchannels_video_comments", supports_api_client, (*ToolSet).get_wxchannels_video_comments},
	{"get_wxchannels_video_share_url", supports_api_client, (*ToolSet).get_wxchannels_video_share_url},
	{"download_wxchannels_live", supports_wxchannels_download, (*ToolSet).download_wxchannels_live},
	{"download_wxchannels_video", supports_wxchannels_download, (*ToolSet).download_wxchannels_video},
	{"deploy_sph_worker", supports_sph, (*ToolSet).deploy_sph_worker},
	{get_zhihu_credential_status_tool_name, supports_zhihu, (*ToolSet).get_zhihu_credential_status},
	{get_my_zhihu_collections_tool_name, supports_zhihu, (*ToolSet).get_my_zhihu_collections},
	{get_zhihu_collection_contents_tool_name, supports_zhihu, (*ToolSet).get_zhihu_collection_contents},
	{get_my_zhihu_answers_tool_name, supports_zhihu, (*ToolSet).get_my_zhihu_answers},
	{get_my_zhihu_posts_tool_name, supports_zhihu, (*ToolSet).get_my_zhihu_posts},
	{get_my_zhihu_zvideos_tool_name, supports_zhihu, (*ToolSet).get_my_zhihu_zvideos},
	{get_my_zhihu_columns_tool_name, supports_zhihu, (*ToolSet).get_my_zhihu_columns},
	{list_automation_schedules_tool_name, supports_automation, without_arguments((*ToolSet).list_automation_schedules_tool)},
	{get_automation_schedule_tool_name, supports_automation, (*ToolSet).get_automation_schedule_tool},
	{create_automation_schedule_tool_name, supports_automation, (*ToolSet).create_automation_schedule_tool},
	{toggle_automation_schedule_tool_name, supports_automation, (*ToolSet).toggle_automation_schedule_tool},
	{trigger_automation_schedule_tool_name, supports_automation, (*ToolSet).trigger_automation_schedule_tool},
	{list_automation_runs_tool_name, supports_automation, (*ToolSet).list_automation_runs_tool},
	{"get_download_tasks", supports_data, (*ToolSet).get_download_tasks},
	{"get_download_task_detail", supports_data, (*ToolSet).get_download_task_detail},
	{"delete_download_tasks", supports_task_delete, (*ToolSet).delete_download_tasks},
	{"create_download_task", supports_task_create, (*ToolSet).create_download_task_tool},
	{"get_accounts", supports_data, (*ToolSet).get_accounts},
	{"get_browse_history", supports_data, (*ToolSet).get_browse_history},
	{"get_logs", supports_data, (*ToolSet).get_logs},
	{"get_certificate_status", supports_data, without_arguments((*ToolSet).get_certificate_status)},
}

func build_tool_registry(declarations []tool_declaration) map[string]tool_declaration {
	registry := make(map[string]tool_declaration, len(declarations))
	for _, declaration := range declarations {
		registry[declaration.name] = declaration
	}
	return registry
}

// tool_registry resolves at package init; callers verify with
// validate_tool_registry.
var tool_registry = build_tool_registry(tool_declarations)

// validate_tool_registry reports duplicate, empty, or incomplete declarations,
// and any catalog tool that has no declaration row.
func validate_tool_registry() error {
	if len(tool_registry) != len(tool_declarations) {
		return fmt.Errorf("工具声明名称重复: %d 行映射到 %d 个名称", len(tool_declarations), len(tool_registry))
	}
	for _, declaration := range tool_declarations {
		if declaration.name == "" {
			return fmt.Errorf("工具声明缺少名称")
		}
		if declaration.supports == nil {
			return fmt.Errorf("工具 %s 缺少可用性判断", declaration.name)
		}
		if declaration.handle == nil {
			return fmt.Errorf("工具 %s 缺少执行器", declaration.name)
		}
	}
	for _, definition := range servicetools.BuiltinCatalog() {
		if _, exists := tool_registry[definition.Name]; !exists {
			return fmt.Errorf("工具 %s 缺少声明", definition.Name)
		}
	}
	return nil
}

func (s *ToolSet) supports_tool(name string) bool {
	if s == nil {
		return false
	}
	declaration, exists := tool_registry[name]
	return exists && declaration.supports(s)
}

func (s *ToolSet) execute_tool(ctx context.Context, name string, raw_arguments json.RawMessage) (map[string]any, error) {
	declaration, exists := tool_registry[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", servicetools.ErrUnknownTool, name)
	}
	return declaration.handle(s, ctx, raw_arguments)
}
