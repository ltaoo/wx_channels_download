package bilibiliadapter

import (
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/bilibili"
	"wx_channel/pkg/util"
)

func bilibili_bangumi_info_from_fetch(data any) (*bilibili.BangumiInfo, bool, error) {
	switch value := data.(type) {
	case *bilibili.PlayURLSSRData:
		if value == nil {
			return nil, true, fmt.Errorf("bilibili playurl SSR data is nil")
		}
		return bangumi_info_from_playurl(value), true, nil
	case bilibili.PlayURLSSRData:
		return bangumi_info_from_playurl(&value), true, nil
	case *bilibili.BangumiInfo:
		if value == nil {
			return nil, true, fmt.Errorf("bilibili bangumi info is nil")
		}
		return value, true, nil
	case bilibili.BangumiInfo:
		return &value, true, nil
	case json.RawMessage:
		var info bilibili.BangumiInfo
		if err := json.Unmarshal(value, &info); err == nil &&
			(info.EpisodeID > 0 ||
				info.SeasonID > 0 ||
				len(info.PlayURLSSRData.Data.Result.VideoInfo.Dash.Video) > 0) {
			return &info, true, nil
		}
		return nil, false, nil
	case []byte:
		return bilibili_bangumi_info_from_fetch(json.RawMessage(value))
	default:
		return nil, false, nil
	}
}

func bilibili_video_info_from_fetch(data any) (*bilibili.VideoInfo, error) {
	switch value := data.(type) {
	case []*bilibili.VideoInfo:
		if len(value) == 0 || value[0] == nil {
			return nil, fmt.Errorf("bilibili video list is empty")
		}
		return value[0], nil
	case []bilibili.VideoInfo:
		if len(value) == 0 {
			return nil, fmt.Errorf("bilibili video list is empty")
		}
		return &value[0], nil
	case *bilibili.VideoInfo:
		if value == nil {
			return nil, fmt.Errorf("bilibili video info is nil")
		}
		return value, nil
	case bilibili.VideoInfo:
		return &value, nil
	case json.RawMessage:
		var infos []bilibili.VideoInfo
		if err := json.Unmarshal(value, &infos); err == nil && len(infos) > 0 {
			return &infos[0], nil
		}
		var info bilibili.VideoInfo
		if err := json.Unmarshal(value, &info); err != nil {
			return nil, fmt.Errorf("decode bilibili video info: %w", err)
		}
		if strings.TrimSpace(info.VideoID) == "" {
			return nil, fmt.Errorf("bilibili video id is empty")
		}
		return &info, nil
	case []byte:
		return bilibili_video_info_from_fetch(json.RawMessage(value))
	default:
		return nil, fmt.Errorf("unsupported bilibili fetch data type %T", data)
	}
}

func (h *handler) ToContent(data any) (*model.Content, error) {
	bangumi_info, is_bangumi, err := bilibili_bangumi_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if is_bangumi {
		selected_stream := best_bangumi_video_stream(bangumi_info.PlayURLSSRData.Data.Result.VideoInfo.Dash.Video)
		content := bangumi_video_content(bangumi_info, selected_stream, util.NowMillis())
		if content == nil {
			return nil, fmt.Errorf("bilibili bangumi video id is empty")
		}
		return content, nil
	}

	video_info, err := bilibili_video_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if video_info.VideoID == "" {
		return nil, fmt.Errorf("bilibili video id is empty")
	}

	now := util.NowMillis()
	return &model.Content{
		Id:         BuildContentID(video_info.VideoID),
		PlatformId: PlatformID,
		ExternalId: video_info.VideoID,
		Type:       "video",
		Title:      video_info.Title,
		URL:        video_info.URL,
		CoverURL:   video_info.CoverURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func (h *handler) ToAccount(data any) (*model.Account, error) {
	bangumi_info, is_bangumi, err := bilibili_bangumi_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if is_bangumi {
		external_id := bangumi_external_id(bangumi_info)
		if external_id == "" {
			return nil, fmt.Errorf("bilibili bangumi id is empty")
		}
		return build_bangumi_account(util.NowMillis()), nil
	}

	video_info, err := bilibili_video_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if video_info.VideoID == "" {
		return nil, fmt.Errorf("bilibili video id is empty")
	}

	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(video_info.VideoID),
		PlatformId: PlatformID,
		ExternalId: video_info.VideoID,
		Nickname:   "B站用户",
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func (h *handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	bangumi_info, is_bangumi, err := bilibili_bangumi_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if is_bangumi {
		selected_stream := best_bangumi_video_stream(bangumi_info.PlayURLSSRData.Data.Result.VideoInfo.Dash.Video)
		return bangumi_content_details(bangumi_info, selected_stream, util.NowMillis())
	}

	video_info, err := bilibili_video_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content_id := BuildContentID(video_info.VideoID)
	return []adapter.ContentDetail{{
		Type: "video",
		Key:  content_id,
		Data: &model.ContentVideo{
			Id:     content_id,
			URL:    video_info.URL,
			Format: "mp4",
		},
	}}, nil
}

// BuildDownloadTaskFromFetch converts the structured result returned by Fetch
// without requesting the Bilibili page a second time.
func (h *handler) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	config := make(map[string]any)
	if config_text := strings.TrimSpace(string(config_json)); config_text != "" {
		if err := json.Unmarshal(config_json, &config); err != nil {
			return nil, fmt.Errorf("解析下载配置失败: %w", err)
		}
	}

	bangumi_info, is_bangumi, err := bilibili_bangumi_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if is_bangumi {
		return build_task_from_bangumi_info(bangumi_info, config)
	}
	video_info, err := bilibili_video_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return build_task_from_video_info(video_info, "", config)
}
