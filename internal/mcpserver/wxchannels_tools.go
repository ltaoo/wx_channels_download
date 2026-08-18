package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type wxchannels_page_arguments struct {
	NextMarker string `json:"next_marker"`
}

type wxchannels_search_accounts_arguments struct {
	Keyword    string `json:"keyword"`
	NextMarker string `json:"next_marker"`
}

type wxchannels_account_page_arguments struct {
	Username   string `json:"username"`
	NextMarker string `json:"next_marker"`
}

type wxchannels_interacted_videos_arguments struct {
	Flag       int    `json:"flag"`
	NextMarker string `json:"next_marker"`
}

type wxchannels_video_profile_arguments struct {
	URL               string `json:"url"`
	ObjectID          string `json:"oid"`
	ObjectNonceID     string `json:"nid"`
	EncryptedObjectID string `json:"eid"`
}

type wxchannels_video_comments_arguments struct {
	ObjectID      string `json:"oid"`
	ObjectNonceID string `json:"nid"`
	CommentID     string `json:"comment_id"`
	NextMarker    string `json:"next_marker"`
}

type wxchannels_video_share_url_arguments struct {
	ObjectID string `json:"oid"`
}

type wxchannels_api_response struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
}

func wxchannels_tool_definitions() []any {
	return []any{
		wxchannels_tool_definition(
			"get_wxchannels_status",
			"获取微信视频号连接状态",
			"检查是否已有视频号页面通过 WebSocket 连接到下载器。其他微信视频号 API 工具依赖此连接。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			},
		),
		wxchannels_tool_definition(
			"search_wxchannels_accounts",
			"搜索微信视频号账号",
			"按关键词搜索微信视频号账号。继续翻页时，把响应中的 lastBuff 原样传给 next_marker。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"keyword": map[string]any{
						"type":        "string",
						"minLength":   1,
						"description": "账号昵称等搜索关键词。",
					},
					"next_marker": wxchannels_next_marker_schema("上一页响应 data.lastBuff 中的分页游标。"),
				},
				"required": []string{"keyword"},
			},
		),
		wxchannels_tool_definition(
			"get_wxchannels_account_videos",
			"获取微信视频号账号的视频列表",
			"获取指定视频号账号发布的视频。username 可使用搜索结果中的 username；缺少 @finder 后缀时会自动补齐。",
			wxchannels_account_page_schema("上一页响应 data.lastBuffer 中的分页游标。"),
		),
		wxchannels_tool_definition(
			"get_wxchannels_live_replays",
			"获取微信视频号直播回放",
			"获取指定视频号账号的直播回放列表。username 可使用搜索或关注列表返回的 username。",
			wxchannels_account_page_schema("上一页响应 data.lastBuffer 中的分页游标。"),
		),
		wxchannels_tool_definition(
			"get_wxchannels_interacted_videos",
			"获取微信视频号赞或收藏的视频",
			"获取当前微信用户赞过或收藏的视频。flag 是视频号页面使用的 tabFlag，默认值 7；继续翻页时传入响应中的 lastBuffer。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"flag": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"default":     7,
						"description": "视频号交互列表的 tabFlag；留空时使用 7。",
					},
					"next_marker": wxchannels_next_marker_schema("上一页响应 data.lastBuffer 中的分页游标。"),
				},
			},
		),
		wxchannels_tool_definition(
			"get_wxchannels_followed_accounts",
			"获取关注的微信视频号账号",
			"获取当前微信用户关注的视频号账号列表。继续翻页时，把响应中的 lastBuffer 原样传给 next_marker。",
			wxchannels_page_schema("上一页响应 data.lastBuffer 中的分页游标。"),
		),
		wxchannels_tool_definition(
			"get_wxchannels_play_history",
			"获取微信视频号播放记录",
			"获取当前微信用户最近的视频号播放记录。响应同时包含 recentNDays 和分页游标 lastBuffer。",
			wxchannels_page_schema("上一页响应 data.lastBuffer 中的分页游标。"),
		),
		wxchannels_tool_definition(
			"get_wxchannels_video_profile",
			"获取微信视频号视频详情",
			"获取单个视频号内容详情。优先直接传视频链接；也可传 oid 与 nid，或只传 eid。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"format":      "uri",
						"description": "视频号内容链接。",
					},
					"oid": map[string]any{
						"type":        "string",
						"description": "视频对象 ID；使用它时还需提供 nid。",
					},
					"nid": map[string]any{
						"type":        "string",
						"description": "视频对象 nonce ID；与 oid 配套使用。",
					},
					"eid": map[string]any{
						"type":        "string",
						"description": "加密的视频对象 ID。",
					},
				},
				"anyOf": []any{
					map[string]any{"required": []string{"url"}},
					map[string]any{"required": []string{"oid", "nid"}},
					map[string]any{"required": []string{"eid"}},
				},
			},
		),
		wxchannels_tool_definition(
			"get_wxchannels_video_comments",
			"获取微信视频号视频评论",
			"获取视频评论或指定根评论的回复。oid 必填；查询一级评论时传 nid，查询回复时传 comment_id。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"oid": map[string]any{
						"type":        "string",
						"minLength":   1,
						"description": "视频对象 ID。",
					},
					"nid": map[string]any{
						"type":        "string",
						"description": "查询一级评论时需要的视频对象 nonce ID。",
					},
					"comment_id": map[string]any{
						"type":        "string",
						"description": "查询某条根评论的回复时使用的评论 ID。",
					},
					"next_marker": wxchannels_next_marker_schema("上一页响应 data.lastBuffer 中的分页游标。"),
				},
				"required": []string{"oid"},
				"anyOf": []any{
					map[string]any{"required": []string{"nid"}},
					map[string]any{"required": []string{"comment_id"}},
				},
			},
		),
		wxchannels_tool_definition(
			"get_wxchannels_video_share_url",
			"获取微信视频号视频分享链接",
			"根据视频对象 ID 获取可分享的 H5 链接。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"oid": map[string]any{
						"type":        "string",
						"minLength":   1,
						"description": "视频对象 ID。",
					},
				},
				"required": []string{"oid"},
			},
		),
	}
}

func wxchannels_tool_definition(name string, title string, description string, input_schema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"title":       title,
		"description": description,
		"inputSchema": input_schema,
		"annotations": map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
			"openWorldHint":   true,
		},
	}
}

func wxchannels_next_marker_schema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func wxchannels_page_schema(next_marker_description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"next_marker": wxchannels_next_marker_schema(next_marker_description),
		},
	}
}

func wxchannels_account_page_schema(next_marker_description string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"username": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "视频号账号 username。",
			},
			"next_marker": wxchannels_next_marker_schema(next_marker_description),
		},
		"required": []string{"username"},
	}
}

func (s *Server) get_wxchannels_status(ctx context.Context) (map[string]any, error) {
	return s.call_wxchannels_api(ctx, "/api/channels/status", nil)
}

func (s *Server) search_wxchannels_accounts(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_search_accounts_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	keyword := strings.TrimSpace(arguments.Keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword 不能为空")
	}
	return s.call_wxchannels_api(ctx, "/api/channels/contact/search", url.Values{
		"keyword":     []string{keyword},
		"next_marker": []string{arguments.NextMarker},
	})
}

func (s *Server) get_wxchannels_account_videos(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_wxchannels_account_page(ctx, raw_arguments, "/api/channels/contact/feed/list")
}

func (s *Server) get_wxchannels_live_replays(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_wxchannels_account_page(ctx, raw_arguments, "/api/channels/live/replay/list")
}

func (s *Server) get_wxchannels_account_page(ctx context.Context, raw_arguments json.RawMessage, path string) (map[string]any, error) {
	var arguments wxchannels_account_page_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	username := strings.TrimSpace(arguments.Username)
	if username == "" {
		return nil, fmt.Errorf("username 不能为空")
	}
	return s.call_wxchannels_api(ctx, path, url.Values{
		"username":    []string{username},
		"next_marker": []string{arguments.NextMarker},
	})
}

func (s *Server) get_wxchannels_interacted_videos(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_interacted_videos_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	flag := arguments.Flag
	if flag == 0 {
		flag = 7
	}
	if flag < 1 {
		return nil, fmt.Errorf("flag 必须是正整数")
	}
	return s.call_wxchannels_api(ctx, "/api/channels/interactioned/list", url.Values{
		"flag":        []string{strconv.Itoa(flag)},
		"next_marker": []string{arguments.NextMarker},
	})
}

func (s *Server) get_wxchannels_followed_accounts(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_wxchannels_page(ctx, raw_arguments, "/api/channels/follow/list")
}

func (s *Server) get_wxchannels_play_history(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_wxchannels_page(ctx, raw_arguments, "/api/channels/play/history")
}

func (s *Server) get_wxchannels_page(ctx context.Context, raw_arguments json.RawMessage, path string) (map[string]any, error) {
	var arguments wxchannels_page_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	return s.call_wxchannels_api(ctx, path, url.Values{
		"next_marker": []string{arguments.NextMarker},
	})
}

func (s *Server) get_wxchannels_video_profile(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_video_profile_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.URL = strings.TrimSpace(arguments.URL)
	arguments.ObjectID = strings.TrimSpace(arguments.ObjectID)
	arguments.ObjectNonceID = strings.TrimSpace(arguments.ObjectNonceID)
	arguments.EncryptedObjectID = strings.TrimSpace(arguments.EncryptedObjectID)

	if arguments.URL != "" {
		if err := validate_source_url(arguments.URL); err != nil {
			return nil, err
		}
		if arguments.ObjectID != "" || arguments.ObjectNonceID != "" || arguments.EncryptedObjectID != "" {
			return nil, fmt.Errorf("url 不能与 oid、nid 或 eid 同时使用")
		}
	} else if arguments.EncryptedObjectID != "" {
		if arguments.ObjectID != "" || arguments.ObjectNonceID != "" {
			return nil, fmt.Errorf("eid 不能与 oid 或 nid 同时使用")
		}
	} else if arguments.ObjectID == "" || arguments.ObjectNonceID == "" {
		return nil, fmt.Errorf("需要提供 url、eid，或同时提供 oid 与 nid")
	}

	return s.call_wxchannels_api(ctx, "/api/channels/feed/profile", url.Values{
		"url": []string{arguments.URL},
		"oid": []string{arguments.ObjectID},
		"nid": []string{arguments.ObjectNonceID},
		"eid": []string{arguments.EncryptedObjectID},
	})
}

func (s *Server) get_wxchannels_video_comments(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_video_comments_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.ObjectID = strings.TrimSpace(arguments.ObjectID)
	arguments.ObjectNonceID = strings.TrimSpace(arguments.ObjectNonceID)
	arguments.CommentID = strings.TrimSpace(arguments.CommentID)
	if arguments.ObjectID == "" {
		return nil, fmt.Errorf("oid 不能为空")
	}
	if arguments.ObjectNonceID == "" && arguments.CommentID == "" {
		return nil, fmt.Errorf("nid 和 comment_id 至少需要提供一个")
	}
	return s.call_wxchannels_api(ctx, "/api/channels/feed/comment/list", url.Values{
		"oid":         []string{arguments.ObjectID},
		"nid":         []string{arguments.ObjectNonceID},
		"comment_id":  []string{arguments.CommentID},
		"next_marker": []string{arguments.NextMarker},
	})
}

func (s *Server) get_wxchannels_video_share_url(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_video_share_url_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	object_id := strings.TrimSpace(arguments.ObjectID)
	if object_id == "" {
		return nil, fmt.Errorf("oid 不能为空")
	}
	return s.call_wxchannels_api(ctx, "/api/channels/feed/share_url", url.Values{
		"oid": []string{object_id},
	})
}

func (s *Server) call_wxchannels_api(ctx context.Context, path string, query url.Values) (map[string]any, error) {
	raw_response, err := s.api_client.get_wxchannels_api(ctx, path, query)
	if err != nil {
		return nil, err
	}
	var response wxchannels_api_response
	if err := json.Unmarshal(raw_response, &response); err != nil {
		return nil, fmt.Errorf("解析微信视频号 API 响应失败: %w", err)
	}
	if response.ErrCode != 0 {
		message := value_or_default(response.ErrMsg, fmt.Sprintf("微信视频号 API 返回错误码 %d", response.ErrCode))
		return nil, new_tool_execution_error(message, raw_json_value(raw_response))
	}
	return successful_tool_result(raw_json_value(raw_response))
}
