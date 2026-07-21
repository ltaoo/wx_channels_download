package wxchannels

import (
	"encoding/json"
	"strings"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
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
	coverURL := strings.TrimSpace(content.CoverURL)
	if len(obj.ObjectDesc.Media) > 0 {
		if candidate := strings.TrimSpace(obj.ObjectDesc.Media[0].CoverUrl); candidate != "" {
			coverURL = candidate
		} else if candidate := strings.TrimSpace(obj.ObjectDesc.Media[0].ThumbUrl); candidate != "" {
			coverURL = candidate
		}
	}
	savePath := config.SavePath
	if savePath == "" {
		savePath = "/downloads/wx_channels"
	}

	configJSON, _ := json.Marshal(map[string]any{
		"platform":       PlatformID,
		"external_id":    content.ExternalId,
		"nonce_id":       content.ExternalId2,
		"spec":           spec,
		"download_cover": config.DownloadCover,
		"overwrite":      config.Overwrite,
		"skip_duplicate": config.SkipDuplicate,
		"source_url":     content.SourceURL,
		"content_url":    content.ContentURL,
		"cover_url":      coverURL,
	})

	videoResource := model.DownloadResource{
		Name: title + ".mp4",
		Kind: "video",
		Size: content.Size,
	}
	videoEndpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      downloadURL,
		Enabled:  1,
	}
	resources := []registry.DownloadResourceInfo{{
		Resource:  videoResource,
		Endpoints: []model.DownloadEndpoint{videoEndpoint},
	}}
	resourceType := model.ResourceTypeFile
	if config.DownloadCover && coverURL != "" {
		resourceType = model.ResourceTypeCollection
		resources = append(resources, registry.DownloadResourceInfo{
			Resource: model.DownloadResource{
				Name:       title + ".jpg",
				Kind:       "cover",
				MergeOrder: 1,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      coverURL,
				Enabled:  1,
			}},
		})
	}

	return &registry.DownloadInfo{
		Task: model.DownloadTaskV1{
			Name:         title,
			ResourceType: resourceType,
			Status:       model.TaskStatusWaiting,
			SavePath:     savePath,
			ConfigJSON:   string(configJSON),
		},
		Resource:  videoResource,
		Endpoint:  videoEndpoint,
		Resources: resources,
	}, content, account, nil
}
