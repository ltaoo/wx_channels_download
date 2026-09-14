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

type wxchannels_live_profile_arguments struct {
	Username      string `json:"username"`
	ObjectID      string `json:"oid"`
	ObjectNonceID string `json:"nid"`
	LiveID        string `json:"id"`
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

func (s *Server) get_wxchannels_live_profile(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_live_profile_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.Username = strings.TrimSpace(arguments.Username)
	arguments.ObjectID = strings.TrimSpace(arguments.ObjectID)
	arguments.ObjectNonceID = strings.TrimSpace(arguments.ObjectNonceID)
	arguments.LiveID = strings.TrimSpace(arguments.LiveID)
	if arguments.Username == "" {
		return nil, fmt.Errorf("username 不能为空")
	}
	if arguments.ObjectID == "" {
		return nil, fmt.Errorf("oid 不能为空")
	}
	if arguments.ObjectNonceID == "" {
		return nil, fmt.Errorf("nid 不能为空")
	}
	if arguments.LiveID == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	return s.call_wxchannels_api(ctx, "/api/channels/live/profile", url.Values{
		"username": []string{arguments.Username},
		"oid":      []string{arguments.ObjectID},
		"nid":      []string{arguments.ObjectNonceID},
		"id":       []string{arguments.LiveID},
	})
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
		return nil, fmt.Errorf("解析微信视频号响应失败: %w", err)
	}
	if response.ErrCode != 0 {
		message := value_or_default(response.ErrMsg, fmt.Sprintf("微信视频号返回错误码 %d", response.ErrCode))
		return nil, new_tool_execution_error(message, raw_json_value(raw_response))
	}
	return successful_tool_result(raw_json_value(raw_response))
}
