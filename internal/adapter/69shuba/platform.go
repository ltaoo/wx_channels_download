package shuba69

import (
	"encoding/json"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	"wx_channel/pkg/util"
)

const platformID = "69shuba"

func init() {
	registry.Register(&handler{})
}

type handler struct{}

func (h *handler) PlatformID() string { return platformID }

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, config registry.DownloadConfig) (*registry.DownloadInfo, *model.Content, *model.Account, error) {
	// 使用 mock 数据，用于前端测试
	novel := MockNovel()

	info, err := BuildDownloadTask(novel, config)
	if err != nil {
		return nil, nil, nil, err
	}

	now := util.NowMillis()
	content := &model.Content{
		PlatformId:  platformID,
		ExternalId:  novel.ProfileURL,
		ContentType: "novel",
		Title:       novel.Name,
		CoverURL:    novel.CoverURL,
		URL:         novel.ProfileURL,
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

	return info, content, account, nil
}
