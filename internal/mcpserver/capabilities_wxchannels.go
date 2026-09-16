package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// WXChannelsRuntime performs the raw video-channel platform calls behind one
// transport. Implementations keep their own transport and wire behavior: the
// MCP server reaches the downloader over HTTP, the Bridge calls the in-process
// adapter, and neither is aware of the other.
type WXChannelsRuntime interface {
	SearchContact(ctx context.Context, keyword string, next_marker string) (json.RawMessage, error)
	FeedListOfContact(ctx context.Context, username string, next_marker string) (json.RawMessage, error)
	LiveReplayList(ctx context.Context, username string, next_marker string) (json.RawMessage, error)
	FeedProfile(ctx context.Context, oid string, nid string, request_url string, eid string) (json.RawMessage, error)
	FeedCommentList(ctx context.Context, oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error)
	FeedShareUrl(ctx context.Context, oid string) (json.RawMessage, error)
}

// WXChannelsCapability owns the argument normalization and validation shared by
// every caller. It returns the runtime's RawMessage untouched so 64-bit
// identifiers survive as bytes instead of being decoded into float64.
type WXChannelsCapability struct{ runtime WXChannelsRuntime }

// NewWXChannelsCapability wraps a transport-specific runtime.
func NewWXChannelsCapability(runtime WXChannelsRuntime) *WXChannelsCapability {
	return &WXChannelsCapability{runtime: runtime}
}

func (c *WXChannelsCapability) SearchContact(ctx context.Context, keyword string, next_marker string) (json.RawMessage, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("keyword 不能为空")
	}
	return c.runtime.SearchContact(ctx, keyword, next_marker)
}

func (c *WXChannelsCapability) FeedListOfContact(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username 不能为空")
	}
	return c.runtime.FeedListOfContact(ctx, username, next_marker)
}

func (c *WXChannelsCapability) LiveReplayList(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username 不能为空")
	}
	return c.runtime.LiveReplayList(ctx, username, next_marker)
}

func (c *WXChannelsCapability) FeedProfile(ctx context.Context, oid string, nid string, request_url string, eid string) (json.RawMessage, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	oid = strings.TrimSpace(oid)
	nid = strings.TrimSpace(nid)
	request_url = strings.TrimSpace(request_url)
	eid = strings.TrimSpace(eid)
	if oid == "" && request_url == "" && eid == "" {
		return nil, errors.New("oid、url 和 eid 至少需要提供一个")
	}
	oid, nid, request_url, eid = NormalizeFeedProfileArgs(oid, nid, request_url, eid)
	return c.runtime.FeedProfile(ctx, oid, nid, request_url, eid)
}

func (c *WXChannelsCapability) FeedCommentList(ctx context.Context, oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	oid = strings.TrimSpace(oid)
	nid = strings.TrimSpace(nid)
	comment_id = strings.TrimSpace(comment_id)
	if oid == "" {
		return nil, errors.New("oid 不能为空")
	}
	if nid == "" && comment_id == "" {
		return nil, errors.New("nid 和 comment_id 至少需要提供一个")
	}
	return c.runtime.FeedCommentList(ctx, oid, nid, comment_id, next_marker)
}

func (c *WXChannelsCapability) FeedShareUrl(ctx context.Context, oid string) (json.RawMessage, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	oid = strings.TrimSpace(oid)
	if oid == "" {
		return nil, errors.New("oid 不能为空")
	}
	return c.runtime.FeedShareUrl(ctx, oid)
}

func (c *WXChannelsCapability) ready() error {
	if c == nil || c.runtime == nil {
		return errors.New("视频号查询能力未初始化")
	}
	return nil
}

// NormalizeFeedProfileArgs reconciles the three accepted video selectors. A url
// carrying an eid becomes that eid, and a complete oid+nid pair clears url so
// the caller never parses a relative link.
func NormalizeFeedProfileArgs(oid string, nid string, request_url string, eid string) (string, string, string, string) {
	if eid == "" && request_url != "" {
		if parsed_url, err := url.Parse(request_url); err == nil {
			if parsed_eid := parsed_url.Query().Get("eid"); parsed_eid != "" {
				eid = parsed_eid
				request_url = ""
			}
		}
	}
	if oid != "" && nid != "" {
		request_url = ""
	}
	return oid, nid, request_url, eid
}

// api_wxchannels_runtime is the MCP transport runtime: it forwards each call to
// the downloader HTTP API, whose routes reach the same in-process client.
type api_wxchannels_runtime struct{ api *api_client }

func (r api_wxchannels_runtime) SearchContact(ctx context.Context, keyword string, next_marker string) (json.RawMessage, error) {
	return r.api.get_wxchannels_api(ctx, "/api/channels/contact/search", url.Values{
		"keyword":     []string{keyword},
		"next_marker": []string{next_marker},
	})
}

func (r api_wxchannels_runtime) FeedListOfContact(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	return r.api.get_wxchannels_api(ctx, "/api/channels/contact/feed/list", url.Values{
		"username":    []string{username},
		"next_marker": []string{next_marker},
	})
}

func (r api_wxchannels_runtime) LiveReplayList(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	return r.api.get_wxchannels_api(ctx, "/api/channels/live/replay/list", url.Values{
		"username":    []string{username},
		"next_marker": []string{next_marker},
	})
}

func (r api_wxchannels_runtime) FeedProfile(ctx context.Context, oid string, nid string, request_url string, eid string) (json.RawMessage, error) {
	return r.api.get_wxchannels_api(ctx, "/api/channels/feed/profile", url.Values{
		"url": []string{request_url},
		"oid": []string{oid},
		"nid": []string{nid},
		"eid": []string{eid},
	})
}

func (r api_wxchannels_runtime) FeedCommentList(ctx context.Context, oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error) {
	return r.api.get_wxchannels_api(ctx, "/api/channels/feed/comment/list", url.Values{
		"oid":         []string{oid},
		"nid":         []string{nid},
		"comment_id":  []string{comment_id},
		"next_marker": []string{next_marker},
	})
}

func (r api_wxchannels_runtime) FeedShareUrl(ctx context.Context, oid string) (json.RawMessage, error) {
	return r.api.get_wxchannels_api(ctx, "/api/channels/feed/share_url", url.Values{
		"oid": []string{oid},
	})
}

// wxchannels_capability builds the MCP transport capability on demand. The
// runtime is a single pointer, so no ToolSet field is needed.
func (s *ToolSet) wxchannels_capability() *WXChannelsCapability {
	return NewWXChannelsCapability(api_wxchannels_runtime{api: s.api_client})
}

// wxchannels_api_result interprets one raw API response: a non-zero errCode
// becomes a tool execution error and everything else is published as data.
func (s *ToolSet) wxchannels_api_result(raw_response json.RawMessage) (map[string]any, error) {
	var response wxchannels_api_response
	if err := json.Unmarshal(raw_response, &response); err != nil {
		return nil, fmt.Errorf("解析微信视频号响应失败: %w", err)
	}
	if response.ErrCode != 0 {
		message := value_or_default(response.ErrMsg, fmt.Sprintf("微信视频号返回错误码 %d", response.ErrCode))
		return nil, new_tool_execution_error(message, raw_json_value(raw_response))
	}
	return successful_tool_result(raw_json_value(raw_response))
}
