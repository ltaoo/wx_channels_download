package wxchannels

import (
	"encoding/json"

	"wx_channel/internal/database/model"
	"wx_channel/internal/webcontent/registry"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

func init() {
	registry.Register(&handler{})
}

type handler struct{}

func (h *handler) PlatformID() string { return PlatformID }

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, config registry.DownloadConfig) (*registry.DownloadInfo, *model.Content, *model.Account, error) {
	var obj scraper.ChannelsObject
	if err := json.Unmarshal(contentJSON, &obj); err != nil {
		return nil, nil, nil, err
	}

	content, err := ToContent(&obj)
	if err != nil {
		return nil, nil, nil, err
	}
	account, err := ToAccount(&obj)
	if err != nil {
		return nil, nil, nil, err
	}

	title := config.Filename
	if title == "" {
		title = ObjectTitle(&obj)
	}
	spec := config.Spec
	if spec == "" {
		spec = PickSpec(&obj)
	}
	downloadURL := BuildDownloadURLWithSpec(&obj, spec)
	savePath := config.SavePath
	if savePath == "" {
		savePath = "/downloads/wx_channels"
	}

	configJSON, _ := json.Marshal(map[string]any{
		"platform":       PlatformID,
		"external_id":    content.ExternalId,
		"nonce_id":       content.ExternalId2,
		"spec":           spec,
		"overwrite":      config.Overwrite,
		"skip_duplicate": config.SkipDuplicate,
		"source_url":     content.SourceURL,
		"content_url":    content.ContentURL,
	})

	return &registry.DownloadInfo{
		Task: model.DownloadTaskV1{
			Name:         title,
			ResourceType: model.ResourceTypeFile,
			Status:       model.TaskStatusWaiting,
			SavePath:     savePath,
			ConfigJSON:   string(configJSON),
		},
		Resource: model.DownloadResource{
			Name: title + ".mp4",
			Kind: "video",
			Size: content.Size,
		},
		Endpoint: model.DownloadEndpoint{
			Protocol: "https",
			URL:      downloadURL,
			Enabled:  1,
		},
	}, content, account, nil
}
