package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

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

func wxchannels_download_tool_definitions() []any {
	return []any{
		map[string]any{
			"name":        "download_wxchannels_live",
			"title":       "快速下载微信视频号直播",
			"description": "用户明确确认下载后，使用此命令按精确昵称或 username 自动定位当前直播并直接创建、启动直播下载任务。只需传 account；命令会完成账号搜索、直播流定位和原生任务创建。不要先获取流地址，也不要把直播 FLV 地址传给 download_content。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"account": map[string]any{
						"type":        "string",
						"minLength":   1,
						"description": "视频号精确昵称，或以 @finder 结尾的 username。推荐直接传用户说出的精确昵称。",
					},
					"download_dir": map[string]any{
						"type":        "string",
						"description": "可选下载目录；留空时使用应用配置。",
					},
					"filename": map[string]any{
						"type":        "string",
						"description": "可选自定义文件名。",
					},
					"existing_action": wxchannels_download_existing_action_schema(),
				},
				"required": []string{"account"},
			},
			"annotations": wxchannels_download_annotations(),
		},
		map[string]any{
			"name":        "download_wxchannels_video",
			"title":       "快速下载微信视频号视频",
			"description": "用户明确确认下载后，使用此命令获取单个视频详情并直接创建、启动下载任务。最快方式是只传视频号分享链接 url；已有列表结果时也可传 oid+nid，或只传 eid。无需先调用 get_wxchannels_video_profile、fetch_content 或 download_content。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"anyOf": []any{
					map[string]any{"required": []string{"url"}},
					map[string]any{"required": []string{"oid", "nid"}},
					map[string]any{"required": []string{"eid"}},
				},
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"format":      "uri",
						"description": "视频号视频分享链接；推荐使用此参数。",
					},
					"oid": map[string]any{
						"type":        "string",
						"description": "视频对象 ID；与 nid 配套使用。",
					},
					"nid": map[string]any{
						"type":        "string",
						"description": "视频对象 nonce ID；与 oid 配套使用。",
					},
					"eid": map[string]any{
						"type":        "string",
						"description": "加密的视频对象 ID。",
					},
					"download_dir": map[string]any{
						"type":        "string",
						"description": "可选下载目录；留空时使用应用配置。",
					},
					"filename": map[string]any{
						"type":        "string",
						"description": "可选自定义文件名。",
					},
					"existing_action": wxchannels_download_existing_action_schema(),
					"video_variant_key": map[string]any{
						"type":        "string",
						"description": "可选视频规格 variant_key。",
					},
					"video_variant_spec": map[string]any{
						"type":        "string",
						"description": "可选视频规格名称。",
					},
				},
			},
			"annotations": wxchannels_download_annotations(),
		},
	}
}

func wxchannels_download_annotations() map[string]any {
	return map[string]any{
		"readOnlyHint":    false,
		"destructiveHint": true,
		"idempotentHint":  false,
		"openWorldHint":   true,
	}
}

func wxchannels_download_existing_action_schema() map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        []string{"error", "skip", "overwrite", "duplicate"},
		"default":     "error",
		"description": "遇到相同任务时的处理方式；默认报错，避免重复下载。",
	}
}

func (s *Server) download_wxchannels_live(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

func (s *Server) download_wxchannels_video(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

func (s *Server) resolve_wxchannels_live(ctx context.Context, account string) (wxchannels_download_contact, wxchannels_download_live_object, error) {
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

func (s *Server) resolve_wxchannels_live_by_username(ctx context.Context, username string, fallback_contact wxchannels_download_contact) (wxchannels_download_contact, wxchannels_download_live_object, error) {
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

func (s *Server) fetch_wxchannels_data(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
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

func (s *Server) create_wxchannels_download_task(ctx context.Context, content json.RawMessage, options wxchannels_download_options, source map[string]any) (map[string]any, error) {
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
	create_response, err := s.api_client.create_download_task(ctx, map[string]any{
		"objects": []any{map[string]any{
			"platform":     "wxchannels",
			"content":      content,
			"download_dir": strings.TrimSpace(options.DownloadDir),
			"filename":     strings.TrimSpace(options.Filename),
			"config":       config,
			"auto_start":   true,
		}},
	})
	if err != nil {
		return nil, err
	}
	item := create_response.Tasks[0]
	if item.Code != 0 {
		return nil, new_tool_execution_error(value_or_default(item.Msg, "创建视频号下载任务失败"), raw_json_value(item.Data))
	}
	if existing_action == "skip" && download_item_was_skipped(item.Data) {
		return successful_tool_result(map[string]any{
			"created":       false,
			"started":       false,
			"skipped":       true,
			"existing_task": raw_json_value(item.Data),
			"source":        source,
		})
	}
	return successful_tool_result(map[string]any{
		"created": true,
		"started": true,
		"skipped": false,
		"task":    raw_json_value(item.Data),
		"ids":     create_response.IDs,
		"source":  source,
	})
}
