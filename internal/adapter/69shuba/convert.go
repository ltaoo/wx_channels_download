package shuba69adapter

import (
	"fmt"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

func novel_detail_from_fetch(data any) (*NovelDetail, error) {
	switch value := data.(type) {
	case *NovelDetail:
		if value == nil {
			return nil, fmt.Errorf("69shuba novel is nil")
		}
		return value, nil
	case NovelDetail:
		return &value, nil
	default:
		return nil, fmt.Errorf("unsupported 69shuba fetch data type %T", data)
	}
}

func (h *handler) ToContent(data any) (*model.Content, error) {
	novel, err := novel_detail_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if novel.ProfileURL == "" {
		return nil, fmt.Errorf("69shuba profile url is empty")
	}

	now := util.NowMillis()
	return &model.Content{
		Id:         platformID + ":" + novel.ProfileURL,
		PlatformId: platformID,
		ExternalId: novel.ProfileURL,
		Type:       "novel",
		Title:      novel.Name,
		URL:        novel.ProfileURL,
		SourceURL:  novel.ProfileURL,
		CoverURL:   novel.CoverURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func (h *handler) ToAccount(data any) (*model.Account, error) {
	novel, err := novel_detail_from_fetch(data)
	if err != nil {
		return nil, err
	}
	if novel.Author == "" {
		return nil, nil
	}

	now := util.NowMillis()
	return &model.Account{
		Id:         platformID + ":" + novel.Author,
		PlatformId: platformID,
		ExternalId: novel.Author,
		Nickname:   novel.Author,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func (h *handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	novel, err := novel_detail_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content_id := h.PlatformID() + ":" + novel.ProfileURL
	details := []adapter.ContentDetail{{
		Type: "novel",
		Key:  content_id,
		Data: &model.ContentNovel{
			Id:           content_id,
			AuthorName:   novel.Author,
			ChapterCount: len(novel.Chapters),
			VolumeCount:  len(novel.Volumes),
		},
	}}
	for volume_index, volume := range novel.Volumes {
		idx := volume.Idx
		if idx <= 0 {
			idx = volume_index + 1
		}
		details = append(details, adapter.ContentDetail{
			Type: "novel_volume",
			Key:  fmt.Sprintf("%s:volume:%d", content_id, idx),
			Data: &model.ContentNovelVolume{NovelId: content_id, Idx: idx, Title: volume.Title},
		})
	}
	for chapter_index, chapter := range novel.Chapters {
		idx := chapter.Index
		if idx <= 0 {
			idx = chapter_index + 1
		}
		details = append(details, adapter.ContentDetail{
			Type: "novel_chapter",
			Key:  fmt.Sprintf("%s:chapter:%d", content_id, idx),
			Data: &model.ContentNovelChapter{NovelId: content_id, Idx: idx, Title: chapter.Title, URL: chapter.URL},
		})
	}
	return details, nil
}
