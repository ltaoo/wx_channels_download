package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	servicetools "wx_channel/internal/services/tools"
)

var tool_arguments string

var tool_cmd = &cobra.Command{
	Use:   "tool",
	Short: "列出或直接调用应用服务工具",
}

var tool_list_cmd = &cobra.Command{
	Use:   "list",
	Short: "列出当前可调用的服务工具及参数 schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		tool_service, err := new_cli_tool_service(cmd)
		if err != nil {
			return err
		}
		return write_tool_json(cmd.OutOrStdout(), tool_service.Definitions())
	},
}

var tool_call_cmd = &cobra.Command{
	Use:   "call <name>",
	Short: "按名称直接调用一个服务工具",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		arguments := map[string]any{}
		if raw_arguments := strings.TrimSpace(tool_arguments); raw_arguments != "" {
			if err := json.Unmarshal([]byte(raw_arguments), &arguments); err != nil {
				return fmt.Errorf("解析 --arguments 失败: %w", err)
			}
		}
		tool_service, err := new_cli_tool_service(cmd)
		if err != nil {
			return err
		}
		result, err := tool_service.Execute(cmd.Context(), args[0], arguments)
		if err != nil {
			return err
		}
		return write_tool_json(cmd.OutOrStdout(), result)
	},
}

func init() {
	tool_cmd.PersistentFlags().StringVar(
		&mcp_api_base_url,
		"api-base-url",
		"",
		"下载器 API 地址，默认读取 api.protocol、api.hostname 和 api.port 配置",
	)
	tool_call_cmd.Flags().StringVarP(
		&tool_arguments,
		"arguments",
		"a",
		"{}",
		"工具参数 JSON 对象",
	)
	tool_cmd.AddCommand(tool_list_cmd, tool_call_cmd)
	root_cmd.AddCommand(tool_cmd)
}

func new_cli_tool_service(cmd *cobra.Command) (*servicetools.Service, error) {
	api_base_url := strings.TrimSpace(mcp_api_base_url)
	if api_base_url == "" {
		api_base_url = configured_api_base_url()
	}
	_, toolset, err := new_remote_tool_server(api_base_url, strings.NewReader(""), io.Discard, cmd.ErrOrStderr())
	if err != nil {
		return nil, err
	}
	return toolset.ToolService(), nil
}

func write_tool_json(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
