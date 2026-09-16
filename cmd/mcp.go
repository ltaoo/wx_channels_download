package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"wx_channel/internal/application"
	"wx_channel/internal/mcpserver"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/dm"
	mcp "wx_channel/pkg/mcp"
	"wx_channel/pkg/scraper/zhihu"
)

var mcp_cmd = &cobra.Command{
	Use:   "mcp",
	Short: "以 stdio 运行 MCP server",
	Long: "通过 stdio 运行 MCP server，把全部工具调用转发给已运行的下载器 API。\n" +
		"需要先执行 wx_video_download server 启动 API server，本命令不建立数据库、下载引擎或平台 adapter。\n" +
		"API 地址取自 api.protocol、api.hostname 和 api.port 配置。",
	RunE: func(cmd *cobra.Command, args []string) error {
		api_base_url := configured_api_base_url()
		server, _, err := new_remote_tool_server(
			api_base_url,
			cmd.InOrStdin(),
			cmd.OutOrStdout(),
			cmd.ErrOrStderr(),
		)
		if err != nil {
			return err
		}
		warn_if_downloader_api_down(cmd.Context(), api_base_url, cmd.ErrOrStderr())
		return server.Serve(cmd.Context())
	},
}

func init() {
	root_cmd.AddCommand(mcp_cmd)
}

// new_remote_tool_server builds a tool registry whose backends all reach an
// already-running downloader over its HTTP API. It never opens the database or
// the download engine itself, so it depends on `wx_video_download server` being
// up.
func new_remote_tool_server(api_base_url string, input io.Reader, output io.Writer, error_output io.Writer) (*mcp.Server, *mcpserver.ToolSet, error) {
	cookie_reader := cookies.NewPersistentReader(Cfg.WorkDir)
	return mcpserver.NewRuntime(mcpserver.Config{
		APIBaseURL:       api_base_url,
		Version:          Version,
		Input:            input,
		Output:           output,
		ErrorOutput:      error_output,
		SphDeployer:      application.NewMCPSphDeployer(Cfg),
		ZhihuCollections: zhihu.NewClient(cookie_reader, Cfg.Logger()),
		ZhihuCredentials: cookie_reader,
	})
}

// warn_if_downloader_api_down probes the downloader once before a long-lived
// stdio session starts serving. Every tool call reaches the downloader over
// HTTP, so a missing API server fails all of them; saying it up front beats
// repeating the same failure on each call. The probe only warns and never
// aborts, because the downloader may legitimately be started later.
func warn_if_downloader_api_down(ctx context.Context, api_base_url string, warning_writer io.Writer) {
	probe_ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	client, err := dm.NewClient(dm.ClientOptions{BaseURL: api_base_url})
	if err == nil {
		_, err = client.Status(probe_ctx)
	}
	if err == nil {
		return
	}
	fmt.Fprintf(
		warning_writer,
		"警告: 无法连接下载器 API Server (%s): %v\n"+
			"请保证 wx_video_download server 处于运行状态，否则本次会话的全部工具调用都会失败。\n",
		api_base_url,
		err,
	)
}

func report_mcp_startup_error(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "MCP server 启动失败: %v\n", err)
}
