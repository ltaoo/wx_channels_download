package shuba69adapter

import (
	"encoding/json"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

const platformID = "69shuba"

func init() {
	adapter.Register(&handler{})
}

type handler struct{}

func (h *handler) PlatformID() string { return platformID }

func (h *handler) Fetch(rawURL string) (any, error) {
	return MockNovel(), nil
}

func (h *handler) BuildBrowseHistory(contentJSON json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, configRaw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	// Use mock data for frontend testing
	novel := MockNovel()

	info, err := BuildDownloadTask(novel, configRaw)
	if err != nil {
		return nil, err
	}

	now := util.NowMillis()
	content := &model.Content{
		Id:         platformID + ":" + novel.ProfileURL,
		PlatformId: platformID,
		ExternalId: novel.ProfileURL,
		Type:       "novel",
		Title:      novel.Name,
		CoverURL:   novel.CoverURL,
		URL:        novel.ProfileURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	var account *model.Account
	if novel.Author != "" {
		account = &model.Account{
			PlatformId: platformID,
			ExternalId: novel.Author,
			Nickname:   novel.Author,
			Timestamps: model.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
	}

	info.ContentDetail = &model.ContentNovel{
		Id:           content.Id,
		AuthorName:   novel.Author,
		ChapterCount: len(novel.Chapters),
		VolumeCount:  len(novel.Volumes),
	}
	info.Account = account
	info.Content = content
	contentID := content.Id
	info.Task.ContentId = &contentID

	return info, nil
}
