package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	servicetools "wx_channel/internal/services/tools"
	mcp "wx_channel/pkg/mcp"
)

const server_name = "dm"

// Config configures the MCP tool backends.
type Config struct {
	APIBaseURL          string
	Version             string
	Input               io.Reader
	Output              io.Writer
	ErrorOutput         io.Writer
	HTTPClient          *http.Client
	PollInterval        time.Duration
	DataReader          DataReader
	ScraperJobs         ScraperJobBackend
	DownloadTaskCreator DownloadTaskCreator
	DownloadTaskDeleter DownloadTaskDeleter
	SphDeployer         SphDeployer
	ZhihuCollections    ZhihuCollectionReader
	ZhihuCredentials    ZhihuCredentialReader
	Automation          AutomationBackend
}

// ToolSet owns the business tool backends and registers their tools into an MCP
// protocol engine.
type ToolSet struct {
	api_client            *api_client
	data_reader           DataReader
	scraper_jobs          ScraperJobBackend
	download_task_creator DownloadTaskCreator
	download_task_deleter DownloadTaskDeleter
	sph_deployer          SphDeployer
	zhihu_collections     ZhihuCollectionReader
	zhihu_credentials     ZhihuCredentialReader
	automation            AutomationBackend
	tool_service          *servicetools.Service
}

// NewToolSet validates the configured tool backends and builds the business
// tool registry.
func NewToolSet(config Config) (*ToolSet, error) {
	var client *api_client
	if strings.TrimSpace(config.APIBaseURL) != "" {
		var err error
		client, err = new_api_client(config.APIBaseURL, config.HTTPClient, config.PollInterval)
		if err != nil {
			return nil, err
		}
	}
	if (config.ZhihuCollections == nil) != (config.ZhihuCredentials == nil) {
		return nil, fmt.Errorf("知乎 MCP 工具需要同时配置收藏夹读取器和凭证读取器")
	}
	if client == nil && config.DataReader == nil && config.ScraperJobs == nil && config.DownloadTaskCreator == nil && config.DownloadTaskDeleter == nil && config.SphDeployer == nil && config.ZhihuCollections == nil && config.Automation == nil {
		return nil, fmt.Errorf("至少需要配置一种工具后端")
	}
	toolset := &ToolSet{
		api_client:            client,
		data_reader:           config.DataReader,
		scraper_jobs:          config.ScraperJobs,
		download_task_creator: config.DownloadTaskCreator,
		download_task_deleter: config.DownloadTaskDeleter,
		sph_deployer:          config.SphDeployer,
		zhihu_collections:     config.ZhihuCollections,
		zhihu_credentials:     config.ZhihuCredentials,
		automation:            config.Automation,
	}
	tool_service, err := servicetools.NewBuiltin(toolset.supports_tool, toolset.execute_tool)
	if err != nil {
		return nil, fmt.Errorf("初始化工具服务失败: %w", err)
	}
	toolset.tool_service = tool_service
	if err := validate_tool_registry(); err != nil {
		return nil, err
	}
	return toolset, nil
}

// NewRuntime builds the MCP protocol engine and registers the business toolset
// into it. It is the single composition point for every transport.
func NewRuntime(config Config) (*mcp.Server, *ToolSet, error) {
	toolset, err := NewToolSet(config)
	if err != nil {
		return nil, nil, err
	}
	server := mcp.NewServer(mcp.ServerOptions{
		Name:         server_name,
		Version:      config.Version,
		Instructions: server_instructions(),
		Input:        config.Input,
		Output:       config.Output,
		ErrorOutput:  config.ErrorOutput,
	})
	if err := toolset.Register(server); err != nil {
		return nil, nil, err
	}
	return server, toolset, nil
}

// Register snapshots the enabled tools into the protocol engine. The filtered
// set is materialized once here instead of being recomputed per tools/list.
func (t *ToolSet) Register(server *mcp.Server) error {
	if server == nil {
		return errors.New("MCP 服务未初始化")
	}
	if t == nil || t.tool_service == nil {
		return errors.New("工具服务未初始化")
	}
	for _, definition := range t.tool_service.Definitions() {
		name := definition.Name
		server.AddTool(mcp.Tool{
			Name:        definition.Name,
			Title:       definition.Title,
			Description: definition.Description,
			InputSchema: definition.InputSchema,
			Annotations: definition.Annotations,
		}, func(ctx context.Context, request *mcp.CallToolRequest) (any, error) {
			envelope, err := t.tool_service.Call(ctx, name, request.Arguments)
			if err != nil {
				if errors.Is(err, servicetools.ErrUnknownTool) {
					return nil, fmt.Errorf("%w: %s", mcp.ErrToolNotFound, name)
				}
				return nil, err
			}
			return envelope["structuredContent"], nil
		})
	}
	return nil
}

func server_instructions() string {
	return "查询下载器平台状态、解析受支持平台的内容链接，并创建和启动内容下载任务。调用 download_content、download_wxchannels_live、download_wxchannels_video、delete_download_tasks 或 deploy_sph_worker 前应先获得用户确认。deploy_sph_worker 会读取应用中的 Cloudflare 敏感配置，并覆盖同名的远端视频号查询 Worker；get_config 可用时，应先用它确认 cloudflare.accountId、cloudflare.apiToken、cloudflare.sphWorkerName、cloudflare.sphCookie 和 cloudflare.sphCredential 均已配置。凡涉及下载文件的文件系统操作，包括重命名、移动、删除以及修改文件名或路径，都必须在同一业务流程中同步更新数据库中的 DownloadResource 表记录，禁止仅操作本地文件。若当前工具无法保证文件系统与 DownloadResource 记录一致，必须停止操作并明确告知用户暂不支持，不得使用其他本地文件工具绕过该约束。只读数据工具可查询下载任务及详情、账号、浏览记录、应用日志和代理证书状态；列表结果支持分页，应优先使用筛选参数限制返回量。知乎工具使用 cookies.json 中的 z_c0 登录 Cookie；可先调用 get_zhihu_credential_status 检查登录态，再用 get_my_zhihu_collections 获取当前账号的公开及私密收藏夹，或用 get_my_zhihu_answers、get_my_zhihu_posts、get_my_zhihu_zvideos 和 get_my_zhihu_columns 获取当前账号发布或参与的内容。知乎列表响应 has_next=true 时，应将 next_page 传给对应工具的下一次调用。微信视频号工具可搜索账号、查询账号视频、直播详情与直播回放、赞或收藏的视频、关注账号、播放记录、视频详情、评论及分享链接；这些工具依赖已连接的视频号页面，调用前可先使用 get_wxchannels_status。用户确认后，下载当前直播应直接调用 download_wxchannels_live，只传精确昵称或 username，由命令自动定位直播并创建任务；不要先获取 FLV 流地址，也不要把直播流交给 download_content。下载单个视频应优先直接调用 download_wxchannels_video，传分享链接 url，或使用 oid+nid/eid；无需先调用视频详情、fetch_content 或 download_content。分页时把上一次响应的 lastBuffer 原样传给 next_marker。微信视频号 fetch_content 结果包含可供 aria2 等第三方下载器使用的 download_resources；第三方下载完成后，仅在 requires_decryption 为 true 时使用 decode_key 调用 decrypt_wxchannels_video。解密会原地覆盖文件。"
}
