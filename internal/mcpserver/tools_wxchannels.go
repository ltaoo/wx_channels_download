package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
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

func (s *ToolSet) get_wxchannels_status(ctx context.Context) (map[string]any, error) {
	return s.call_wxchannels_api(ctx, "/api/channels/status", nil)
}

func (s *ToolSet) search_wxchannels_accounts(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_search_accounts_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	raw_response, err := s.wxchannels_capability().SearchContact(ctx, arguments.Keyword, arguments.NextMarker)
	if err != nil {
		return nil, err
	}
	return s.wxchannels_api_result(raw_response)
}

func (s *ToolSet) get_wxchannels_account_videos(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_account_page_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	raw_response, err := s.wxchannels_capability().FeedListOfContact(ctx, arguments.Username, arguments.NextMarker)
	if err != nil {
		return nil, err
	}
	return s.wxchannels_api_result(raw_response)
}

func (s *ToolSet) get_wxchannels_live_replays(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_account_page_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	raw_response, err := s.wxchannels_capability().LiveReplayList(ctx, arguments.Username, arguments.NextMarker)
	if err != nil {
		return nil, err
	}
	return s.wxchannels_api_result(raw_response)
}

func (s *ToolSet) get_wxchannels_live_profile(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

func (s *ToolSet) get_wxchannels_interacted_videos(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

func (s *ToolSet) get_wxchannels_followed_accounts(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_wxchannels_page(ctx, raw_arguments, "/api/channels/follow/list")
}

func (s *ToolSet) get_wxchannels_play_history(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	return s.get_wxchannels_page(ctx, raw_arguments, "/api/channels/play/history")
}

func (s *ToolSet) get_wxchannels_page(ctx context.Context, raw_arguments json.RawMessage, path string) (map[string]any, error) {
	var arguments wxchannels_page_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	return s.call_wxchannels_api(ctx, path, url.Values{
		"next_marker": []string{arguments.NextMarker},
	})
}

func (s *ToolSet) get_wxchannels_video_profile(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

	raw_response, err := s.wxchannels_capability().FeedProfile(
		ctx,
		arguments.ObjectID,
		arguments.ObjectNonceID,
		arguments.URL,
		arguments.EncryptedObjectID,
	)
	if err != nil {
		return nil, err
	}
	return s.wxchannels_api_result(raw_response)
}

func (s *ToolSet) get_wxchannels_video_comments(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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
	raw_response, err := s.wxchannels_capability().FeedCommentList(
		ctx,
		arguments.ObjectID,
		arguments.ObjectNonceID,
		arguments.CommentID,
		arguments.NextMarker,
	)
	if err != nil {
		return nil, err
	}
	return s.wxchannels_api_result(raw_response)
}

func (s *ToolSet) get_wxchannels_video_share_url(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments wxchannels_video_share_url_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	raw_response, err := s.wxchannels_capability().FeedShareUrl(ctx, arguments.ObjectID)
	if err != nil {
		return nil, err
	}
	return s.wxchannels_api_result(raw_response)
}

func (s *ToolSet) call_wxchannels_api(ctx context.Context, path string, query url.Values) (map[string]any, error) {
	raw_response, err := s.api_client.get_wxchannels_api(ctx, path, query)
	if err != nil {
		return nil, err
	}
	return s.wxchannels_api_result(raw_response)
}

type download_wxchannels_live_arguments struct {
	Account        string `json:"account"`
	DownloadDir    string `json:"download_dir"`
	Filename       string `json:"filename"`
	ExistingAction string `json:"existing_action"`
}

type download_wxchannels_video_arguments struct {
	URL               string `json:"url"`
	ObjectID          string `json:"oid"`
	ObjectNonceID     string `json:"nid"`
	EncryptedObjectID string `json:"eid"`
	DownloadDir       string `json:"download_dir"`
	Filename          string `json:"filename"`
	ExistingAction    string `json:"existing_action"`
	VideoVariantKey   string `json:"video_variant_key"`
	VideoVariantSpec  string `json:"video_variant_spec"`
}

type decrypt_wxchannels_video_arguments struct {
	FilePath string `json:"file_path"`
	Key      string `json:"key"`
}

type wxchannels_download_contact struct {
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	HeadURL     string `json:"headUrl"`
	Signature   string `json:"signature"`
	CoverImgURL string `json:"coverImgUrl"`
	LiveStatus  int    `json:"liveStatus"`
}

type wxchannels_download_live_object struct {
	ID            string                      `json:"id"`
	ObjectNonceID string                      `json:"objectNonceId"`
	Username      string                      `json:"username"`
	Contact       wxchannels_download_contact `json:"contact"`
	ObjectDesc    struct {
		Description string `json:"description"`
	} `json:"objectDesc"`
	LiveInfo struct {
		LiveID      string `json:"liveId"`
		LiveStatus  int    `json:"liveStatus"`
		StartTime   int64  `json:"startTime"`
		StreamURL   string `json:"streamUrl"`
		LiveSDKInfo struct {
			LiveCDNURL string `json:"liveCdnUrl"`
		} `json:"liveSdkInfo"`
	} `json:"liveInfo"`
}

type wxchannels_download_options struct {
	DownloadDir      string
	Filename         string
	ExistingAction   string
	VideoVariantKey  string
	VideoVariantSpec string
}

func (s *ToolSet) download_wxchannels_live(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments download_wxchannels_live_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	account := strings.TrimSpace(arguments.Account)
	if account == "" {
		return nil, fmt.Errorf("account 不能为空")
	}
	contact, live_object, err := s.resolve_wxchannels_live(ctx, account)
	if err != nil {
		return nil, err
	}
	stream_url := strings.TrimSpace(live_object.LiveInfo.LiveSDKInfo.LiveCDNURL)
	if stream_url == "" {
		stream_url = strings.TrimSpace(live_object.LiveInfo.StreamURL)
	}
	if strings.HasPrefix(stream_url, "?") || validate_source_url(stream_url) != nil {
		return nil, new_tool_execution_error("当前直播缺少完整直播流地址", map[string]any{
			"account": contact.Nickname,
			"live_id": live_object.LiveInfo.LiveID,
		})
	}

	title := strings.TrimSpace(live_object.ObjectDesc.Description)
	if title == "" {
		title = "直播"
	}
	content, err := json.Marshal(map[string]any{
		"liveSdkInfo": map[string]any{
			"liveCdnUrl": stream_url,
		},
		"liveInfo": map[string]any{
			"liveId":    live_object.LiveInfo.LiveID,
			"startTime": live_object.LiveInfo.StartTime,
		},
		"liveDescription": title,
		"nickname":        contact.Nickname,
		"username":        contact.Username,
		"anchorContact": map[string]any{
			"username":    contact.Username,
			"nickname":    contact.Nickname,
			"headUrl":     contact.HeadURL,
			"signature":   contact.Signature,
			"coverImgUrl": contact.CoverImgURL,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("编码直播下载数据失败: %w", err)
	}
	return s.create_wxchannels_download_task(ctx, content, wxchannels_download_options{
		DownloadDir:    arguments.DownloadDir,
		Filename:       arguments.Filename,
		ExistingAction: arguments.ExistingAction,
	}, map[string]any{
		"type":     "live",
		"account":  contact.Nickname,
		"username": contact.Username,
		"live_id":  live_object.LiveInfo.LiveID,
	})
}

func (s *ToolSet) download_wxchannels_video(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments download_wxchannels_video_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.URL = strings.TrimSpace(arguments.URL)
	arguments.ObjectID = strings.TrimSpace(arguments.ObjectID)
	arguments.ObjectNonceID = strings.TrimSpace(arguments.ObjectNonceID)
	arguments.EncryptedObjectID = strings.TrimSpace(arguments.EncryptedObjectID)
	if err := validate_wxchannels_video_selector(arguments.URL, arguments.ObjectID, arguments.ObjectNonceID, arguments.EncryptedObjectID); err != nil {
		return nil, err
	}
	data, err := s.fetch_wxchannels_data(ctx, "/api/channels/feed/profile", url.Values{
		"url": []string{arguments.URL},
		"oid": []string{arguments.ObjectID},
		"nid": []string{arguments.ObjectNonceID},
		"eid": []string{arguments.EncryptedObjectID},
	})
	if err != nil {
		return nil, err
	}
	var profile struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("解析视频号视频详情失败: %w", err)
	}
	if !has_json_value(profile.Object) {
		return nil, fmt.Errorf("视频号视频详情缺少 object")
	}
	return s.create_wxchannels_download_task(ctx, profile.Object, wxchannels_download_options{
		DownloadDir:      arguments.DownloadDir,
		Filename:         arguments.Filename,
		ExistingAction:   arguments.ExistingAction,
		VideoVariantKey:  arguments.VideoVariantKey,
		VideoVariantSpec: arguments.VideoVariantSpec,
	}, map[string]any{
		"type": "video",
		"url":  arguments.URL,
		"oid":  arguments.ObjectID,
		"nid":  arguments.ObjectNonceID,
		"eid":  arguments.EncryptedObjectID,
	})
}

func (s *ToolSet) decrypt_wxchannels_video(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments decrypt_wxchannels_video_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	file_path := strings.TrimSpace(arguments.FilePath)
	if file_path == "" {
		return nil, fmt.Errorf("file_path 不能为空")
	}
	if !filepath.IsAbs(file_path) {
		return nil, fmt.Errorf("file_path 必须是运行下载器服务所在机器上的绝对路径")
	}
	decode_key := strings.TrimSpace(arguments.Key)
	key, err := strconv.ParseUint(decode_key, 10, 64)
	if err != nil || key == 0 {
		return nil, fmt.Errorf("key 必须是非零十进制整数")
	}
	if _, err := s.api_client.decrypt_wxchannels_video(ctx, file_path, decode_key); err != nil {
		return nil, err
	}
	return successful_tool_result(map[string]any{
		"decrypted": true,
		"file_path": file_path,
	})
}

func validate_wxchannels_video_selector(raw_url string, object_id string, object_nonce_id string, encrypted_object_id string) error {
	if raw_url != "" {
		if err := validate_source_url(raw_url); err != nil {
			return err
		}
		if object_id != "" || object_nonce_id != "" || encrypted_object_id != "" {
			return fmt.Errorf("url 不能与 oid、nid 或 eid 同时使用")
		}
		return nil
	}
	if encrypted_object_id != "" {
		if object_id != "" || object_nonce_id != "" {
			return fmt.Errorf("eid 不能与 oid 或 nid 同时使用")
		}
		return nil
	}
	if object_id == "" || object_nonce_id == "" {
		return fmt.Errorf("需要提供 url、eid，或同时提供 oid 与 nid")
	}
	return nil
}

func (s *ToolSet) resolve_wxchannels_live(ctx context.Context, account string) (wxchannels_download_contact, wxchannels_download_live_object, error) {
	if strings.HasSuffix(account, "@finder") {
		return s.resolve_wxchannels_live_by_username(ctx, account, wxchannels_download_contact{Username: account})
	}
	data, err := s.fetch_wxchannels_data(ctx, "/api/channels/contact/search", url.Values{"keyword": []string{account}})
	if err != nil {
		return wxchannels_download_contact{}, wxchannels_download_live_object{}, err
	}
	var search struct {
		InfoList []struct {
			Contact wxchannels_download_contact `json:"contact"`
		} `json:"infoList"`
		ObjectList []json.RawMessage `json:"objectList"`
	}
	if err := json.Unmarshal(data, &search); err != nil {
		return wxchannels_download_contact{}, wxchannels_download_live_object{}, fmt.Errorf("解析视频号账号搜索结果失败: %w", err)
	}
	exact_matches := make([]wxchannels_download_contact, 0, 1)
	candidates := make([]map[string]any, 0, len(search.InfoList))
	seen_usernames := map[string]struct{}{}
	for _, item := range search.InfoList {
		contact := item.Contact
		if contact.Username == "" {
			continue
		}
		candidates = append(candidates, map[string]any{
			"nickname":    contact.Nickname,
			"username":    contact.Username,
			"live_status": contact.LiveStatus,
		})
		if contact.Nickname == account {
			if _, exists := seen_usernames[contact.Username]; !exists {
				exact_matches = append(exact_matches, contact)
				seen_usernames[contact.Username] = struct{}{}
			}
		}
	}
	if len(exact_matches) == 0 {
		return wxchannels_download_contact{}, wxchannels_download_live_object{}, new_tool_execution_error("未找到昵称完全匹配的视频号账号", map[string]any{
			"account":    account,
			"candidates": candidates,
		})
	}
	if len(exact_matches) > 1 {
		return wxchannels_download_contact{}, wxchannels_download_live_object{}, new_tool_execution_error("存在多个同名视频号账号，请改用 username", map[string]any{
			"account": account,
			"matches": exact_matches,
		})
	}
	contact := exact_matches[0]
	if contact.LiveStatus != 1 {
		return wxchannels_download_contact{}, wxchannels_download_live_object{}, new_tool_execution_error("该视频号当前没有直播", map[string]any{
			"account":     contact.Nickname,
			"username":    contact.Username,
			"live_status": contact.LiveStatus,
		})
	}
	if live_object, ok := find_wxchannels_live_object(search.ObjectList, contact.Username); ok {
		return contact, live_object, nil
	}
	return s.resolve_wxchannels_live_by_username(ctx, contact.Username, contact)
}

func (s *ToolSet) resolve_wxchannels_live_by_username(ctx context.Context, username string, fallback_contact wxchannels_download_contact) (wxchannels_download_contact, wxchannels_download_live_object, error) {
	data, err := s.fetch_wxchannels_data(ctx, "/api/channels/contact/feed/list", url.Values{"username": []string{username}})
	if err != nil {
		return wxchannels_download_contact{}, wxchannels_download_live_object{}, err
	}
	var page struct {
		Contact     wxchannels_download_contact `json:"contact"`
		LiveObjects []json.RawMessage           `json:"liveObjects"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return wxchannels_download_contact{}, wxchannels_download_live_object{}, fmt.Errorf("解析视频号账号视频列表失败: %w", err)
	}
	contact := page.Contact
	if contact.Username == "" {
		contact = fallback_contact
	}
	if contact.Username == "" {
		contact.Username = username
	}
	live_object, ok := find_wxchannels_live_object(page.LiveObjects, username)
	if !ok {
		return wxchannels_download_contact{}, wxchannels_download_live_object{}, new_tool_execution_error("该视频号当前没有可下载的直播", map[string]any{
			"account":     contact.Nickname,
			"username":    username,
			"live_status": contact.LiveStatus,
		})
	}
	return contact, live_object, nil
}

func find_wxchannels_live_object(raw_objects []json.RawMessage, username string) (wxchannels_download_live_object, bool) {
	for _, raw_object := range raw_objects {
		var live_object wxchannels_download_live_object
		if json.Unmarshal(raw_object, &live_object) != nil {
			continue
		}
		object_username := live_object.Username
		if object_username == "" {
			object_username = live_object.Contact.Username
		}
		if username != "" && object_username != username {
			continue
		}
		if strings.TrimSpace(live_object.LiveInfo.LiveID) == "" {
			continue
		}
		return live_object, true
	}
	return wxchannels_download_live_object{}, false
}

func (s *ToolSet) fetch_wxchannels_data(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	raw_response, err := s.api_client.get_wxchannels_api(ctx, path, query)
	if err != nil {
		return nil, err
	}
	var response struct {
		ErrCode int             `json:"errCode"`
		ErrMsg  string          `json:"errMsg"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw_response, &response); err != nil {
		return nil, fmt.Errorf("解析微信视频号响应失败: %w", err)
	}
	if response.ErrCode != 0 {
		message := value_or_default(response.ErrMsg, fmt.Sprintf("微信视频号返回错误码 %d", response.ErrCode))
		return nil, new_tool_execution_error(message, raw_json_value(raw_response))
	}
	if !has_json_value(response.Data) {
		return nil, fmt.Errorf("微信视频号响应缺少 data")
	}
	return response.Data, nil
}

func (s *ToolSet) create_wxchannels_download_task(ctx context.Context, content json.RawMessage, options wxchannels_download_options, source map[string]any) (map[string]any, error) {
	existing_action := strings.TrimSpace(options.ExistingAction)
	if existing_action == "" {
		existing_action = "error"
	}
	if !is_existing_action(existing_action) {
		return nil, fmt.Errorf("existing_action 必须是 error、skip、overwrite 或 duplicate")
	}
	config := map[string]any{
		"platform":        "wxchannels",
		"existing_action": existing_action,
	}
	if value := strings.TrimSpace(options.VideoVariantKey); value != "" {
		config["video_variant_key"] = value
	}
	if value := strings.TrimSpace(options.VideoVariantSpec); value != "" {
		config["video_variant_spec"] = value
		config["spec"] = value
	}
	if existing_action == "overwrite" {
		config["overwrite"] = true
	}
	if existing_action == "duplicate" {
		config["duplicate"] = true
	}
	auto_start := true
	create_result, err := s.create_download_task(ctx, DownloadTaskCreateRequest{
		Platform:    "wxchannels",
		Content:     content,
		DownloadDir: strings.TrimSpace(options.DownloadDir),
		Filename:    strings.TrimSpace(options.Filename),
		Config:      config,
		AutoStart:   &auto_start,
	}, "创建视频号下载任务失败")
	if err != nil {
		return nil, err
	}
	if create_result.Skipped {
		return successful_tool_result(map[string]any{
			"created":       false,
			"started":       false,
			"skipped":       true,
			"existing_task": create_result.Task,
			"source":        source,
		})
	}
	return successful_tool_result(map[string]any{
		"created": true,
		"started": true,
		"skipped": false,
		"task":    create_result.Task,
		"ids":     create_result.IDs,
		"source":  source,
	})
}

// tools_wxchannels declares the tools whose handlers live in this file. The row
// order here is filesystem-local only; the published order is fixed
// by the concatenation in tool_declarations (registry.go).
var tools_wxchannels = []tool{
	{
		name:         "get_wxchannels_status",
		title:        `获取微信视频号连接状态`,
		description:  `检查是否已有视频号页面通过 WebSocket 连接到下载器。其他微信视频号工具依赖此连接。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       without_arguments((*ToolSet).get_wxchannels_status),
	},
	{
		name:         "search_wxchannels_accounts",
		title:        `搜索微信视频号账号`,
		description:  `按关键词搜索微信视频号账号。继续翻页时，把响应中的 lastBuff 原样传给 next_marker。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"keyword":{"description":"账号昵称等搜索关键词。","minLength":1,"type":"string"},"next_marker":{"description":"上一页响应 data.lastBuff 中的分页游标。","type":"string"}},"required":["keyword"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).search_wxchannels_accounts,
	},
	{
		name:         "get_wxchannels_account_videos",
		title:        `获取微信视频号账号的视频列表`,
		description:  `获取指定视频号账号发布的视频。username 可使用搜索结果中的 username；缺少 @finder 后缀时会自动补齐。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"next_marker":{"description":"上一页响应 data.lastBuffer 中的分页游标。","type":"string"},"username":{"description":"视频号账号 username。","minLength":1,"type":"string"}},"required":["username"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_wxchannels_account_videos,
	},
	{
		name:         "get_wxchannels_live_replays",
		title:        `获取微信视频号直播回放`,
		description:  `获取指定视频号账号的直播回放列表。username 可使用搜索或关注列表返回的 username。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"next_marker":{"description":"上一页响应 data.lastBuffer 中的分页游标。","type":"string"},"username":{"description":"视频号账号 username。","minLength":1,"type":"string"}},"required":["username"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_wxchannels_live_replays,
	},
	{
		name:         "get_wxchannels_live_profile",
		title:        `获取微信视频号直播详情`,
		description:  `通过视频号页面的 joinLive 能力获取直播详情和直播流信息。username、oid、nid 和 id 分别对应 finderUsername、objectId、objectNonceId 和 liveId。所有 ID 均以字符串传入，避免大整数精度损失。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"id":{"description":"直播 ID，对应 liveId。","minLength":1,"type":"string"},"nid":{"description":"直播对象 nonce ID，对应 objectNonceId。","minLength":1,"type":"string"},"oid":{"description":"直播对象 ID，对应 objectId。","minLength":1,"type":"string"},"username":{"description":"传给 joinLive 的 finderUsername。","minLength":1,"type":"string"}},"required":["username","oid","nid","id"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_wxchannels_live_profile,
	},
	{
		name:         "get_wxchannels_interacted_videos",
		title:        `获取微信视频号赞或收藏的视频`,
		description:  `获取当前微信用户赞过或收藏的视频。flag 是视频号页面使用的 tabFlag，默认值 7；继续翻页时传入响应中的 lastBuffer。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"flag":{"default":7,"description":"视频号交互列表的 tabFlag；留空时使用 7。","minimum":1,"type":"integer"},"next_marker":{"description":"上一页响应 data.lastBuffer 中的分页游标。","type":"string"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_wxchannels_interacted_videos,
	},
	{
		name:         "get_wxchannels_followed_accounts",
		title:        `获取关注的微信视频号账号`,
		description:  `获取当前微信用户关注的视频号账号列表。继续翻页时，把响应中的 lastBuffer 原样传给 next_marker。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"next_marker":{"description":"上一页响应 data.lastBuffer 中的分页游标。","type":"string"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_wxchannels_followed_accounts,
	},
	{
		name:         "get_wxchannels_play_history",
		title:        `获取微信视频号播放记录`,
		description:  `获取当前微信用户最近的视频号播放记录。响应同时包含 recentNDays 和分页游标 lastBuffer。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"next_marker":{"description":"上一页响应 data.lastBuffer 中的分页游标。","type":"string"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_wxchannels_play_history,
	},
	{
		name:         "get_wxchannels_video_profile",
		title:        `获取微信视频号视频详情`,
		description:  `获取单个视频号内容详情。优先直接传视频链接；也可传 oid 与 nid，或只传 eid。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"anyOf":[{"required":["url"]},{"required":["oid","nid"]},{"required":["eid"]}],"properties":{"eid":{"description":"加密的视频对象 ID。","type":"string"},"nid":{"description":"视频对象 nonce ID；与 oid 配套使用。","type":"string"},"oid":{"description":"视频对象 ID；使用它时还需提供 nid。","type":"string"},"url":{"description":"视频号内容链接。","format":"uri","type":"string"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_wxchannels_video_profile,
	},
	{
		name:         "get_wxchannels_video_comments",
		title:        `获取微信视频号视频评论`,
		description:  `获取视频评论或指定根评论的回复。oid 必填；查询一级评论时传 nid，查询回复时传 comment_id。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"anyOf":[{"required":["nid"]},{"required":["comment_id"]}],"properties":{"comment_id":{"description":"查询某条根评论的回复时使用的评论 ID。","type":"string"},"next_marker":{"description":"上一页响应 data.lastBuffer 中的分页游标。","type":"string"},"nid":{"description":"查询一级评论时需要的视频对象 nonce ID。","type":"string"},"oid":{"description":"视频对象 ID。","minLength":1,"type":"string"}},"required":["oid"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_wxchannels_video_comments,
	},
	{
		name:         "get_wxchannels_video_share_url",
		title:        `获取微信视频号视频分享链接`,
		description:  `根据视频对象 ID 获取可分享的 H5 链接。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"oid":{"description":"视频对象 ID。","minLength":1,"type":"string"}},"required":["oid"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_wxchannels_video_share_url,
	},
	{
		name:         "download_wxchannels_live",
		title:        `快速下载微信视频号直播`,
		description:  `用户明确确认下载后，使用此命令按精确昵称或 username 自动定位当前直播并直接创建、启动直播下载任务。只需传 account；命令会完成账号搜索、直播流定位和原生任务创建。不要先获取流地址，也不要把直播 FLV 地址传给 download_content。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"account":{"description":"视频号精确昵称，或以 @finder 结尾的 username。推荐直接传用户说出的精确昵称。","minLength":1,"type":"string"},"download_dir":{"description":"可选下载目录；留空时使用应用配置。","type":"string"},"existing_action":{"default":"error","description":"遇到相同任务时的处理方式；默认报错，避免重复下载。","enum":["error","skip","overwrite","duplicate"],"type":"string"},"filename":{"description":"可选自定义文件名。","type":"string"}},"required":["account"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":true,"idempotentHint":false,"openWorldHint":true,"readOnlyHint":false}`),
		supports:     supports_wxchannels_download,
		handle:       (*ToolSet).download_wxchannels_live,
	},
	{
		name:         "download_wxchannels_video",
		title:        `快速下载微信视频号视频`,
		description:  `用户明确确认下载后，使用此命令获取单个视频详情并直接创建、启动下载任务。最快方式是只传视频号分享链接 url；已有列表结果时也可传 oid+nid，或只传 eid。无需先调用 get_wxchannels_video_profile、fetch_content 或 download_content。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"anyOf":[{"required":["url"]},{"required":["oid","nid"]},{"required":["eid"]}],"properties":{"download_dir":{"description":"可选下载目录；留空时使用应用配置。","type":"string"},"eid":{"description":"加密的视频对象 ID。","type":"string"},"existing_action":{"default":"error","description":"遇到相同任务时的处理方式；默认报错，避免重复下载。","enum":["error","skip","overwrite","duplicate"],"type":"string"},"filename":{"description":"可选自定义文件名。","type":"string"},"nid":{"description":"视频对象 nonce ID；与 oid 配套使用。","type":"string"},"oid":{"description":"视频对象 ID；与 nid 配套使用。","type":"string"},"url":{"description":"视频号视频分享链接；推荐使用此参数。","format":"uri","type":"string"},"video_variant_key":{"description":"可选视频规格 variant_key。","type":"string"},"video_variant_spec":{"description":"可选视频规格名称。","type":"string"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":true,"idempotentHint":false,"openWorldHint":true,"readOnlyHint":false}`),
		supports:     supports_wxchannels_download,
		handle:       (*ToolSet).download_wxchannels_video,
	},
	{
		name:         "decrypt_wxchannels_video",
		title:        `解密微信视频号视频`,
		description:  `原地解密已经下载到本机的微信视频号视频。file_path 必须是运行下载器服务的同一台机器上的绝对路径；仅当 fetch_content 返回的 download_resources 项中 requires_decryption 为 true 时调用，并原样传入 decode_key。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"file_path":{"description":"第三方下载器已下载完成的视频绝对路径。文件会被原地覆盖为解密后内容。","type":"string"},"key":{"description":"fetch_content 返回的 decode_key，使用字符串传递以避免整数精度丢失。","pattern":"^[1-9][0-9]*$","type":"string"}},"required":["file_path","key"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":true,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).decrypt_wxchannels_video,
	},
}
