package services

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine/engine"
	"wx_channel/pkg/flowengine/nodes"
)

const (
	platform_fetch_detail_node_suffix = "FetchDetailNode"
	platform_fetch_list_node_suffix   = "FetchListNode"

	default_platform_detail_output_key = "detail"
	default_platform_list_output_key   = "list"

	default_platform_fetch_timeout_seconds = 300
)

// platform_display_names maps adapter platform ids to user-facing labels used
// in the flow node catalog.
var platform_display_names = map[string]string{
	"wxchannels":  "视频号",
	"wxmp":        "公众号",
	"douyin":      "抖音",
	"kuaishou":    "快手",
	"youtube":     "YouTube",
	"x":           "X (Twitter)",
	"instagram":   "Instagram",
	"bilibili":    "哔哩哔哩",
	"xiaohongshu": "小红书",
	"weibo":       "微博",
	"zhihu":       "知乎",
	"cctv":        "央视网",
	"feishu":      "飞书",
	"hongguo":     "红果短剧",
	"iqiyi":       "爱奇艺",
	"mgtv":        "芒果TV",
	"quark":       "夸克",
	"ucdrive":     "UC 网盘",
	"vqq":         "腾讯视频",
	"youku":       "优酷",
	"singlefile":  "单文件",
	"webpage":     "网页提取",
	"69shuba":     "69书吧",
	"fanqienovel": "番茄小说",
	"ttk":         "TT看书",
	"tencent":     "腾讯视频",
}

func platform_display_name(platform_id string) string {
	if name, ok := platform_display_names[platform_id]; ok && name != "" {
		return name
	}
	return platform_id
}

// platform_node_type_token converts a platform id into a stable PascalCase
// token, e.g. "wxchannels" -> "Wxchannels", "x" -> "X".
func platform_node_type_token(platform_id string) string {
	parts := strings.FieldsFunc(platform_id, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' '
	})
	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		builder.WriteString(part[1:])
	}
	if builder.Len() == 0 {
		return "Platform"
	}
	return builder.String()
}

func platform_fetch_detail_node_type(platform_id string) string {
	return platform_node_type_token(platform_id) + platform_fetch_detail_node_suffix
}

func platform_fetch_list_node_type(platform_id string) string {
	return platform_node_type_token(platform_id) + platform_fetch_list_node_suffix
}

// is_platform_fetch_node_type reports whether node_type is one of the dynamic
// platform detail/list node types that RegisterPlatformFetchNodes registers.
// It mirrors that registration scope: every adapter gets a detail node, but only
// homepage-capable adapters get a list node.
func is_platform_fetch_node_type(node_type string) bool {
	if node_type == "" {
		return false
	}
	for _, platform_id := range adapter.IDs() {
		if node_type == platform_fetch_detail_node_type(platform_id) {
			return true
		}
		if node_type != platform_fetch_list_node_type(platform_id) {
			continue
		}
		if _, ok := adapter.Get(platform_id).(adapter.HomeContentsBuilder); ok {
			return true
		}
	}
	return false
}

// platform_fetch_node is a per-platform flow node. In detail mode it fetches a
// single content URL through the scraper job pipeline and returns the
// normalized detail payload; in list mode it reads one account homepage tab and
// returns an iterable list of normalized content records.
type platform_fetch_node struct {
	node_id      string
	node_type    string
	platform_id  string
	list         bool
	Config       map[string]interface{}
	scraper_jobs *ScraperJobService
}

func new_platform_fetch_node_factory(platform_id string, list bool, jobs *ScraperJobService) func(map[string]interface{}) engine.Node {
	node_type := platform_fetch_detail_node_type(platform_id)
	if list {
		node_type = platform_fetch_list_node_type(platform_id)
	}
	return func(config map[string]interface{}) engine.Node {
		id, _ := config["id"].(string)
		return &platform_fetch_node{
			node_id:      id,
			node_type:    node_type,
			platform_id:  platform_id,
			list:         list,
			Config:       config,
			scraper_jobs: jobs,
		}
	}
}

func (n *platform_fetch_node) ID() string   { return n.node_id }
func (n *platform_fetch_node) Type() string { return n.node_type }

func (n *platform_fetch_node) Execute(process_context *engine.ProcessContext) (bool, []string, error) {
	var err error
	if n.list {
		err = n.execute_list(process_context)
	} else {
		err = n.execute_detail(process_context)
	}
	if err != nil {
		return false, nil, err
	}
	next_ids := process_context.EngineRef.GetNextNodeIDsFromDefinition(process_context, n.node_id)
	return true, next_ids, nil
}

// execute_detail fetches one content URL through the scraper job service and
// stores the full fetch output (job_id, platform, url, result, content,
// account, content_details, download_info) under output_key.
func (n *platform_fetch_node) execute_detail(process_context *engine.ProcessContext) error {
	if n.scraper_jobs == nil {
		return fmt.Errorf("抓取任务服务未初始化，无法执行 %s", n.node_type)
	}
	raw_url, err := n.resolved_string(process_context, "url")
	if err != nil {
		return err
	}
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return fmt.Errorf("url 不能为空")
	}
	resolution, err := ResolveScraperPlatform(raw_url)
	if err != nil {
		return err
	}
	if resolution.Platform != n.platform_id {
		return fmt.Errorf("%s 节点无法处理平台 %s 的链接（期望 %s）", n.node_type, resolution.Platform, n.platform_id)
	}
	output, err := n.wait_scraper_fetch(process_context, raw_url)
	if err != nil {
		return err
	}
	process_context.SetOutput(n.node_id, n.output_key(default_platform_detail_output_key), output)
	return nil
}

func (n *platform_fetch_node) wait_scraper_fetch(process_context *engine.ProcessContext, raw_url string) (*ScraperFetchOutput, error) {
	job, err := n.scraper_jobs.Create(ScraperFetchRequest{URL: raw_url})
	if err != nil {
		return nil, fmt.Errorf("创建抓取任务失败: %w", err)
	}
	job_id := job.ID
	timeout_seconds := n.timeout_seconds()
	deadline := time.Now().Add(time.Duration(timeout_seconds) * time.Second)
	execution_context := process_context.ExecutionContext()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		current := n.scraper_jobs.Get(job_id, true)
		if current == nil {
			return nil, fmt.Errorf("抓取任务 %s 已被清理", job_id)
		}
		switch current.Status {
		case ScraperJobStatusCompleted:
			if current.Output == nil {
				return nil, fmt.Errorf("抓取任务 %s 未返回结果", job_id)
			}
			return current.Output, nil
		case ScraperJobStatusFailed:
			message := strings.TrimSpace(current.Error)
			if message == "" {
				message = "未知错误"
			}
			return nil, fmt.Errorf("抓取失败: %s", message)
		case ScraperJobStatusInterrupted:
			return nil, fmt.Errorf("抓取任务已中断")
		}
		if time.Now().After(deadline) {
			n.scraper_jobs.Interrupt(job_id)
			return nil, fmt.Errorf("抓取超时（%d 秒）", timeout_seconds)
		}
		select {
		case <-execution_context.Done():
			n.scraper_jobs.Interrupt(job_id)
			return nil, fmt.Errorf("抓取已取消: %w", execution_context.Err())
		case <-ticker.C:
		}
	}
}

// execute_list reads one account homepage tab and stores an iterable payload
// (platform, scope, list, total, next_marker, tabs) under output_key.
func (n *platform_fetch_node) execute_list(process_context *engine.ProcessContext) error {
	handler := adapter.Get(n.platform_id)
	if handler == nil {
		return fmt.Errorf("未注册的平台 adapter: %s", n.platform_id)
	}
	builder, ok := handler.(adapter.HomeContentsBuilder)
	if !ok {
		return fmt.Errorf("平台 %s 不支持获取列表", n.platform_id)
	}
	profile, err := n.resolved_string(process_context, "profile")
	if err != nil {
		return err
	}
	user_id, err := n.resolved_string(process_context, "user_id")
	if err != nil {
		return err
	}
	account, err := build_platform_home_account(n.platform_id, profile, user_id)
	if err != nil {
		return err
	}
	scope, err := n.resolved_string(process_context, "scope")
	if err != nil {
		return err
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = default_platform_home_scope(builder, account)
	}
	page, err := n.resolved_string(process_context, "page")
	if err != nil {
		return err
	}
	contents, next_marker, used_scope, err := n.fetch_home_list(builder, handler, account, scope, strings.TrimSpace(page))
	if err != nil {
		return err
	}
	output := map[string]any{
		"platform":    n.platform_id,
		"scope":       used_scope,
		"total":       len(contents),
		"list":        contents,
		"next_marker": next_marker,
		"tabs":        builder.HomeContentTabs(account),
	}
	process_context.SetOutput(n.node_id, n.output_key(default_platform_list_output_key), output)
	return nil
}

// fetch_home_list reads one homepage tab. When the node config declares a
// "page" key and the adapter supports pagination, it fetches exactly that page
// (page may be empty for the first page) and returns the next marker; otherwise
// it falls back to a single unpaginated snapshot.
func (n *platform_fetch_node) fetch_home_list(builder adapter.HomeContentsBuilder, handler adapter.AdapterHandler, account *model.Account, scope, page string) ([]model.Content, string, string, error) {
	_, has_page := n.Config["page"]
	return fetch_platform_home_list(builder, handler, account, scope, page, has_page)
}

func fetch_platform_home_list(builder adapter.HomeContentsBuilder, handler adapter.AdapterHandler, account *model.Account, scope, page string, paginate bool) ([]model.Content, string, string, error) {
	if paginate {
		if fetcher, ok := handler.(adapter.HomeDetailsFetcher); ok {
			details, err := fetcher.FetchHomeDetails(account, scope, page)
			if err != nil {
				return nil, "", "", fmt.Errorf("获取主页列表失败: %w", err)
			}
			if details == nil {
				return nil, "", "", fmt.Errorf("平台 %s 返回了空的主页列表", handler.PlatformID())
			}
			used_scope := first_non_empty(details.Scope, scope)
			return home_details_contents(details.Contents), details.NextMarker, used_scope, nil
		}
	}
	home, err := builder.BuildHomeContents(account, scope)
	if err != nil {
		return nil, "", "", fmt.Errorf("获取主页列表失败: %w", err)
	}
	if home == nil {
		return nil, "", "", fmt.Errorf("平台 %s 返回了空的主页列表", handler.PlatformID())
	}
	contents := home.Contents
	if contents == nil {
		contents = []model.Content{}
	}
	return contents, home.NextMarker, first_non_empty(home.Scope, scope), nil
}

// home_details_contents coerces the loosely-typed HomeDetails.Contents into the
// normalized content list every adapter actually returns.
func home_details_contents(value any) []model.Content {
	switch typed := value.(type) {
	case []model.Content:
		if typed == nil {
			return []model.Content{}
		}
		return typed
	case *[]model.Content:
		if typed == nil {
			return []model.Content{}
		}
		return *typed
	}
	return []model.Content{}
}

func (n *platform_fetch_node) resolved_string(process_context *engine.ProcessContext, key string) (string, error) {
	raw, exists := n.Config[key]
	if !exists || raw == nil {
		return "", nil
	}
	process_context.Mu.Lock()
	resolved, err := nodes.ResolveTemplates(raw, process_context)
	process_context.Mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	if resolved == nil {
		return "", nil
	}
	return fmt.Sprint(resolved), nil
}

func (n *platform_fetch_node) output_key(fallback string) string {
	if key, ok := n.Config["output_key"].(string); ok {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func (n *platform_fetch_node) timeout_seconds() int {
	switch value := n.Config["timeout_seconds"].(type) {
	case int:
		if value > 0 {
			return value
		}
	case int64:
		if value > 0 {
			return int(value)
		}
	case float64:
		if value > 0 {
			return int(value)
		}
	}
	return default_platform_fetch_timeout_seconds
}

// build_platform_home_account turns a homepage URL or username into the
// minimal account model each platform's HomeContentsBuilder expects.
func build_platform_home_account(platform_id, profile, user_id string) (*model.Account, error) {
	profile = strings.TrimSpace(profile)
	user_id = strings.TrimSpace(user_id)
	account := &model.Account{PlatformId: platform_id}
	switch platform_id {
	case "douyin":
		if profile == "" {
			return nil, fmt.Errorf("抖音需要主页 URL 或 sec_uid")
		}
		account.ExternalId = first_non_empty(platform_path_id(profile, "douyin.com", "user"), profile)
	case "kuaishou":
		if profile == "" {
			return nil, fmt.Errorf("快手需要主页 URL 或用户 ID")
		}
		account.ExternalId = first_non_empty(platform_path_id(profile, "kuaishou.com", "profile"), profile)
	case "weibo":
		if profile == "" {
			return nil, fmt.Errorf("微博需要主页 URL 或 UID")
		}
		account.ExternalId = platform_path_id(profile, "weibo.com", "u")
		if account.ExternalId == "" {
			if is_http_url(profile) {
				account.ProfileURL = profile
			} else {
				account.ExternalId = profile
			}
		}
	case "bilibili":
		if profile == "" {
			return nil, fmt.Errorf("哔哩哔哩需要主页 URL 或 mid")
		}
		account.ExternalId = platform_path_id(profile, "space.bilibili.com", "")
		if account.ExternalId == "" {
			if is_http_url(profile) {
				account.ProfileURL = profile
			} else {
				account.ExternalId = profile
			}
		}
	case "zhihu":
		if profile == "" {
			return nil, fmt.Errorf("知乎需要主页 URL 或 URL token")
		}
		account.ExternalId = platform_path_id(profile, "zhihu.com", "people")
		if account.ExternalId == "" {
			if is_http_url(profile) {
				account.ProfileURL = profile
			} else {
				account.ExternalId = profile
			}
		}
	case "xiaohongshu":
		if profile == "" {
			return nil, fmt.Errorf("小红书需要主页 URL 或用户 ID")
		}
		if is_http_url(profile) {
			account.ProfileURL = profile
		} else {
			account.ExternalId = profile
		}
	case "youtube":
		if profile == "" {
			return nil, fmt.Errorf("YouTube 需要频道 URL 或频道 ID")
		}
		if is_http_url(profile) {
			account.ProfileURL = profile
		} else {
			account.ExternalId = profile
		}
	case "instagram":
		if profile == "" {
			return nil, fmt.Errorf("Instagram 需要用户名或主页 URL")
		}
		if is_http_url(profile) {
			account.ProfileURL = profile
		} else {
			account.Alias = profile
		}
	case "wxchannels":
		if profile == "" {
			return nil, fmt.Errorf("视频号需要用户名")
		}
		account.ExternalId = profile
	case "x":
		if is_http_url(profile) {
			account.ProfileURL = profile
		} else if profile != "" {
			account.Alias = strings.TrimPrefix(profile, "@")
		}
		account.ExternalId = user_id
		if account.ProfileURL == "" && account.Alias == "" {
			return nil, fmt.Errorf("X 主页列表需要用户名或主页 URL")
		}
		if account.ExternalId == "" {
			return nil, fmt.Errorf("X 主页列表还需要 user_id（数字 ID），可从主页请求或浏览器获得")
		}
	default:
		if profile == "" {
			return nil, fmt.Errorf("平台 %s 需要主页 URL 或账号 ID", platform_id)
		}
		account.ExternalId = profile
		if is_http_url(profile) {
			account.ProfileURL = profile
		}
	}
	if account.ExternalId == "" && account.ProfileURL == "" && account.Alias == "" {
		return nil, fmt.Errorf("平台 %s 需要主页 URL 或用户名", platform_id)
	}
	return account, nil
}

func default_platform_home_scope(builder adapter.HomeContentsBuilder, account *model.Account) string {
	for _, tab := range builder.HomeContentTabs(account) {
		if scope := strings.TrimSpace(tab.Scope); scope != "" {
			return scope
		}
	}
	return "posts"
}

func is_http_url(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// platform_path_id extracts an id from a homepage URL. When marker is empty it
// returns the first non-empty path segment; otherwise it returns the segment
// that follows marker.
func platform_path_id(raw_url, host_suffix, marker string) string {
	if !is_http_url(raw_url) {
		return ""
	}
	parsed, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	host_suffix = strings.ToLower(host_suffix)
	if host != host_suffix && !strings.HasSuffix(host, "."+host_suffix) {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if marker == "" {
		if len(parts) > 0 {
			return parts[0]
		}
		return ""
	}
	for index := 0; index+1 < len(parts); index++ {
		if parts[index] == marker {
			return parts[index+1]
		}
	}
	return ""
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// RegisterPlatformFetchNodes adds a detail node for every registered adapter
// and a list node for every adapter exposing a homepage content builder.
func RegisterPlatformFetchNodes(flow_engine *engine.FlowEngine, scraper_jobs *ScraperJobService) {
	if flow_engine == nil {
		return
	}
	for _, platform_id := range adapter.IDs() {
		flow_engine.RegisterNode(
			platform_fetch_detail_node_type(platform_id),
			new_platform_fetch_node_factory(platform_id, false, scraper_jobs),
		)
		handler := adapter.Get(platform_id)
		if _, ok := handler.(adapter.HomeContentsBuilder); !ok {
			continue
		}
		flow_engine.RegisterNode(
			platform_fetch_list_node_type(platform_id),
			new_platform_fetch_node_factory(platform_id, true, scraper_jobs),
		)
	}
}

// platform_node_catalog builds the add-node catalog entries for the platform
// fetch nodes. Detail nodes cover every registered adapter; list nodes cover
// only adapters that expose a homepage content builder.
func platform_node_catalog() []UserFlowNodeCatalogItem {
	platform_ids := adapter.IDs()
	items := make([]UserFlowNodeCatalogItem, 0, len(platform_ids)*2)
	for _, platform_id := range platform_ids {
		name := platform_display_name(platform_id)
		items = append(items, UserFlowNodeCatalogItem{
			Type:        platform_fetch_detail_node_type(platform_id),
			Name:        name + " · 获取详情",
			Description: fmt.Sprintf("抓取 %s 单条内容的详情，返回 job_id / result / download_info，可直接用于创建下载任务", name),
			ConfigKeys: []UserFlowConfigKey{
				{Key: "url", Type: "string", Required: true, Description: "内容详情链接"},
				{Key: "output_key", Type: "string", Description: "结果写入上下文的键，默认 " + default_platform_detail_output_key},
				{Key: "timeout_seconds", Type: "number", Description: "抓取超时秒数，默认 300"},
			},
		})
		if _, ok := adapter.Get(platform_id).(adapter.HomeContentsBuilder); !ok {
			continue
		}
		items = append(items, UserFlowNodeCatalogItem{
			Type:        platform_fetch_list_node_type(platform_id),
			Name:        name + " · 获取列表",
			Description: fmt.Sprintf("抓取 %s 账号主页的内容列表，返回可遍历的 list，供后续逐个创建下载任务", name),
			ConfigKeys: []UserFlowConfigKey{
				{Key: "profile", Type: "string", Required: true, Description: "主页 URL 或用户名/账号 ID"},
				{Key: "user_id", Type: "string", Description: "X 等平台需要的数字用户 ID"},
				{Key: "scope", Type: "string", Description: "主页 tab，留空使用平台首个 tab"},
				{Key: "page", Type: "string", Description: "分页游标；配置此项后按页抓取并输出 next_marker，留空取第一页"},
				{Key: "output_key", Type: "string", Description: "结果写入上下文的键，默认 " + default_platform_list_output_key},
			},
		})
	}
	return items
}
