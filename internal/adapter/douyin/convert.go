package douyinadapter

import (
	"fmt"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/douyin"
	"wx_channel/pkg/util"
)

func douyin_video_info_from_fetch(data any) (*douyin.VideoInfo, error) {
	switch value := data.(type) {
	case *douyin.VideoInfo:
		if value == nil {
			return nil, fmt.Errorf("douyin video info is nil")
		}
		return value, nil
	case douyin.VideoInfo:
		return &value, nil
	default:
		return nil, fmt.Errorf("unsupported douyin fetch data type %T", data)
	}
}

func (h *handler) ToContent(data any) (*model.Content, error) {
	video_info, err := douyin_video_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if video_info.VideoID == "" {
		return nil, fmt.Errorf("douyin video id is empty")
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
	video_info, err := douyin_video_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if video_info.VideoID == "" {
		return nil, fmt.Errorf("douyin video id is empty")
	}

	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(video_info.VideoID),
		PlatformId: PlatformID,
		ExternalId: video_info.VideoID,
		Nickname:   "抖音用户",
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func (h *handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	video_info, err := douyin_video_info_from_fetch(data)
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
