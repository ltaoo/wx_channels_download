package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"wx_channel/internal/mcpserver"
)

var mcp_api_base_url string

var mcp_cmd = &cobra.Command{
	Use:   "mcp",
	Short: "运行 MCP stdio server",
	Long:  "通过 stdio 运行 MCP server，连接到已启动的下载器 API",
	RunE: func(cmd *cobra.Command, args []string) error {
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
