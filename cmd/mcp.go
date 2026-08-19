package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"wx_channel/internal/application"
	"wx_channel/internal/mcpserver"
)

var mcp_api_base_url string
var mcp_standalone bool

var mcp_cmd = &cobra.Command{
	Use:   "mcp",
	Short: "运行 MCP stdio server",
	Long:  "通过 stdio 运行 MCP server；可连接已启动的下载器 API，或使用 --standalone 在进程内启动只读查询与抓取服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		if mcp_standalone {
			if strings.TrimSpace(mcp_api_base_url) != "" {
				return fmt.Errorf("--standalone 与 --api-base-url 不能同时使用")
			}
			return application.ServeMCPStdio(cmd.Context(), Cfg, application.MCPStdioConfig{
				Input:       cmd.InOrStdin(),
				Output:      cmd.OutOrStdout(),
				ErrorOutput: cmd.ErrOrStderr(),
			})
		}
		api_base_url := strings.TrimSpace(mcp_api_base_url)
		if api_base_url == "" {
			api_base_url = configured_api_base_url()
		}
		server, err := mcpserver.NewServer(mcpserver.Config{
			APIBaseURL:  api_base_url,
			Version:     Version,
			Input:       cmd.InOrStdin(),
			Output:      cmd.OutOrStdout(),
			ErrorOutput: cmd.ErrOrStderr(),
		})
		if err != nil {
			return err
		}
		return server.Serve(context.Background())
	},
}

func init() {
	mcp_cmd.Flags().BoolVar(
		&mcp_standalone,
		"standalone",
		false,
		"不启动 API server，在当前进程中提供数据库查询、证书状态和页面抓取工具",
	)
	mcp_cmd.Flags().StringVar(
		&mcp_api_base_url,
		"api-base-url",
		"",
		"下载器 API 地址，默认读取 api.protocol、api.hostname 和 api.port 配置",
	)
	root_cmd.AddCommand(mcp_cmd)
}

func configured_api_base_url() string {
	protocol := strings.TrimSpace(Cfg.GetString("api.protocol"))
	if protocol == "" {
		protocol = "http"
	}
	hostname := strings.TrimSpace(Cfg.GetString("api.hostname"))
	if hostname == "" || hostname == "0.0.0.0" || hostname == "::" {
		hostname = "127.0.0.1"
	}
	port := Cfg.GetInt("api.port")
	return fmt.Sprintf("%s://%s", protocol, net.JoinHostPort(hostname, strconv.Itoa(port)))
}

func report_mcp_startup_error(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "MCP server 启动失败: %v\n", err)
}
