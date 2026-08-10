package bilibiliadapter

import (
	"fmt"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/bilibili"
	"wx_channel/pkg/util"
)

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
	default:
		return nil, fmt.Errorf("unsupported bilibili fetch data type %T", data)
	}
}

func (h *handler) ToContent(data any) (*model.Content, error) {
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
