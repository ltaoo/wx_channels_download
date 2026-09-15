package mcpserver

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	servicetools "wx_channel/internal/services/tools"
)

// Pointer-backed interface stubs. Embedding the interface satisfies it without
// implementing every method, and the values are never called, only compared.
type (
	stub_data_reader           struct{ DataReader }
	stub_scraper_jobs          struct{ ScraperJobBackend }
	stub_download_task_creator struct{ DownloadTaskCreator }
	stub_download_task_deleter struct{ DownloadTaskDeleter }
	stub_sph_deployer          struct{ SphDeployer }
	stub_zhihu_collections     struct{ ZhihuCollectionReader }
	stub_zhihu_credentials     struct{ ZhihuCredentialReader }
	stub_automation            struct{ AutomationBackend }
)

func TestToolDeclarationsCoverCatalog(t *testing.T) {
	catalog_names := make(map[string]bool)
	for _, definition := range servicetools.BuiltinCatalog() {
		catalog_names[definition.Name] = true
	}
	if len(catalog_names) != 44 {
		t.Fatalf("catalog 工具数量 = %d, 期望 44", len(catalog_names))
	}
	if len(tool_registry) != 44 {
		t.Fatalf("registry 工具数量 = %d, 期望 44", len(tool_registry))
	}
	for name := range catalog_names {
		if _, exists := tool_registry[name]; !exists {
			t.Errorf("catalog 工具 %s 缺少声明", name)
		}
	}
	for name := range tool_registry {
		if !catalog_names[name] {
			t.Errorf("registry 工具 %s 不在 catalog 中", name)
		}
	}
}

func TestToolDeclarationNamesAreUnique(t *testing.T) {
	if len(tool_declarations) != len(tool_registry) {
		t.Fatalf("声明行数 %d != registry 名称数 %d", len(tool_declarations), len(tool_registry))
	}
	for _, declaration := range tool_declarations {
		if declaration.name == "" {
			t.Errorf("存在缺少名称的声明")
		}
		if declaration.supports == nil {
			t.Errorf("工具 %s 缺少可用性判断", declaration.name)
		}
		if declaration.handle == nil {
			t.Errorf("工具 %s 缺少执行器", declaration.name)
		}
	}
	if err := validate_tool_registry(); err != nil {
		t.Fatalf("validate_tool_registry() = %v, 期望 nil", err)
	}
}

func TestToolSupportsMatrix(t *testing.T) {
	api_group := []string{
		"get_config", "update_config", "get_restart_status", "get_platform_status",
		"decrypt_wxchannels_video", "get_wxchannels_status", "search_wxchannels_accounts",
		"get_wxchannels_account_videos", "get_wxchannels_live_replays", "get_wxchannels_live_profile",
		"get_wxchannels_interacted_videos", "get_wxchannels_followed_accounts",
		"get_wxchannels_play_history", "get_wxchannels_video_profile",
		"get_wxchannels_video_comments", "get_wxchannels_video_share_url",
	}
	scraper_group := []string{"fetch_content", "create_scraper_job", "get_scraper_job"}
	download_content_group := []string{"download_content"}
	wxchannels_download_group := []string{"download_wxchannels_live", "download_wxchannels_video"}
	data_group := []string{
		"get_download_tasks", "get_download_task_detail", "get_accounts",
		"get_browse_history", "get_logs", "get_certificate_status",
	}
	task_delete_group := []string{"delete_download_tasks"}
	task_create_group := []string{"create_download_task"}
	sph_group := []string{"deploy_sph_worker"}
	zhihu_group := []string{
		"get_zhihu_credential_status", "get_my_zhihu_collections", "get_zhihu_collection_contents",
		"get_my_zhihu_answers", "get_my_zhihu_posts", "get_my_zhihu_zvideos", "get_my_zhihu_columns",
	}
	automation_group := []string{
		"list_automation_schedules", "get_automation_schedule", "create_automation_schedule",
		"toggle_automation_schedule", "trigger_automation_schedule", "list_automation_runs",
	}

	api_only := concat(
		api_group, scraper_group, download_content_group, wxchannels_download_group,
		data_group, task_create_group,
	)
	all_backends := concat(
		api_group, scraper_group, download_content_group, wxchannels_download_group,
		data_group, task_delete_group, task_create_group, sph_group, zhihu_group, automation_group,
	)

	cases := []struct {
		name     string
		toolset  *ToolSet
		expected []string
	}{
		{"empty", &ToolSet{}, nil},
		{"api_only", &ToolSet{api_client: &api_client{}}, api_only},
		{"scraper_only", &ToolSet{scraper_jobs: &stub_scraper_jobs{}}, scraper_group},
		{"creator_only", &ToolSet{download_task_creator: &stub_download_task_creator{}}, task_create_group},
		{"deleter_only", &ToolSet{download_task_deleter: &stub_download_task_deleter{}}, task_delete_group},
		{"sph_only", &ToolSet{sph_deployer: &stub_sph_deployer{}}, sph_group},
		{"data_reader_only", &ToolSet{data_reader: &stub_data_reader{}}, data_group},
		{"zhihu_collections_only", &ToolSet{zhihu_collections: &stub_zhihu_collections{}}, nil},
		{"zhihu_both", &ToolSet{zhihu_collections: &stub_zhihu_collections{}, zhihu_credentials: &stub_zhihu_credentials{}}, zhihu_group},
		{"automation_only", &ToolSet{automation: &stub_automation{}}, automation_group},
		{"all_backends", &ToolSet{
			api_client:            &api_client{},
			data_reader:           &stub_data_reader{},
			scraper_jobs:          &stub_scraper_jobs{},
			download_task_creator: &stub_download_task_creator{},
			download_task_deleter: &stub_download_task_deleter{},
			sph_deployer:          &stub_sph_deployer{},
			zhihu_collections:     &stub_zhihu_collections{},
			zhihu_credentials:     &stub_zhihu_credentials{},
			automation:            &stub_automation{},
		}, all_backends},
	}

	for _, test_case := range cases {
		t.Run(test_case.name, func(t *testing.T) {
			actual := supported_names(test_case.toolset)
			expected := normalize_names(test_case.expected)
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("支持的工具集合不匹配\n实际: %v\n期望: %v", actual, expected)
			}
		})
	}
}

func TestExecuteToolUnknownName(t *testing.T) {
	toolset := &ToolSet{}
	_, err := toolset.execute_tool(context.Background(), "bogus", nil)
	if err == nil {
		t.Fatal("期望返回未知工具错误，实际为 nil")
	}
	if !errors.Is(err, servicetools.ErrUnknownTool) {
		t.Errorf("错误 %v 不满足 errors.Is(ErrUnknownTool)", err)
	}
	if err.Error() != "未知工具: bogus" {
		t.Errorf("错误文本 = %q, 期望 %q", err.Error(), "未知工具: bogus")
	}
}

func supported_names(toolset *ToolSet) []string {
	names := make([]string, 0, len(tool_declarations))
	for _, declaration := range tool_declarations {
		if toolset.supports_tool(declaration.name) {
			names = append(names, declaration.name)
		}
	}
	sort.Strings(names)
	return names
}

func normalize_names(names []string) []string {
	normalized := make([]string, 0, len(names))
	normalized = append(normalized, names...)
	sort.Strings(normalized)
	return normalized
}

func concat(groups ...[]string) []string {
	result := make([]string, 0)
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}
