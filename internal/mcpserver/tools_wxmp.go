package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// WXMPRuntime performs the raw official-account platform calls behind one
// backend. A process without an installed adapter simply does not inject one,
// which hides the tool.
type WXMPRuntime interface {
	BizMsgList(ctx context.Context, username string, offset string) (json.RawMessage, error)
}

type wxmp_biz_msg_list_arguments struct {
	Username string `json:"username"`
	Offset   string `json:"offset"`
}

func (s *ToolSet) get_wxmp_biz_msg_list(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxmp_biz_msg_list_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	username := strings.TrimSpace(arguments.Username)
	if username == "" {
		return nil, errors.New("username 不能为空")
	}
	raw_response, err := s.wxmp.BizMsgList(ctx, username, strings.TrimSpace(arguments.Offset))
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_response))
}

// tools_wxmp declares the tools whose handlers live in this file. The row
// order here is filesystem-local only; the published order is fixed
// by the concatenation in tool_declarations (registry.go).
var tools_wxmp = []tool{
	{
		name:         "get_wxmp_biz_msg_list",
		title:        `获取公众号历史消息`,
		description:  `获取指定微信公众号的历史消息列表。username 是公众号的 FakeID，也就是会话消息里的 __biz 值；继续翻页时，把上一页响应中的分页游标传给 offset。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"offset":{"description":"上一页响应中的分页游标。","type":"string"},"username":{"description":"公众号 FakeID，即会话消息里的 __biz 值。","minLength":1,"type":"string"}},"required":["username"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_wxmp,
		handle:       (*ToolSet).get_wxmp_biz_msg_list,
	},
}
