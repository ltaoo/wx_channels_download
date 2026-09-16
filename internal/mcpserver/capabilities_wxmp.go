package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// WXMPRuntime performs the raw official-account platform calls behind one
// transport. The Bridge reaches it through the in-process adapter; a runtime
// without an installed adapter is simply not injected, which hides the tool.
type WXMPRuntime interface {
	BizMsgList(ctx context.Context, username string, offset string) (json.RawMessage, error)
}

// WXMPCapability owns the argument normalization and validation shared by every
// caller of the official-account history capability.
type WXMPCapability struct{ runtime WXMPRuntime }

// NewWXMPCapability wraps a transport-specific runtime.
func NewWXMPCapability(runtime WXMPRuntime) *WXMPCapability {
	return &WXMPCapability{runtime: runtime}
}

func (c *WXMPCapability) BizMsgList(ctx context.Context, username string, offset string) (json.RawMessage, error) {
	if c == nil || c.runtime == nil {
		return nil, errors.New("公众号历史消息能力未初始化")
	}
	username = strings.TrimSpace(username)
	offset = strings.TrimSpace(offset)
	if username == "" {
		return nil, errors.New("username 不能为空")
	}
	return c.runtime.BizMsgList(ctx, username, offset)
}

// wxmp_capability builds the MCP transport capability from the injected runtime.
func (s *ToolSet) wxmp_capability() *WXMPCapability {
	return NewWXMPCapability(s.wxmp)
}
