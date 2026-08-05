package douyin

import (
	"encoding/json"
	"fmt"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/internal/adapter"
	scraper "wx_channel/pkg/scraper/douyin"
	"wx_channel/pkg/util"
)

const platformIDDouyin = "douyin"

// PlatformID is the platform identifier for douyin.
const PlatformID = platformIDDouyin

func init() {
	adapter.Register(&handler{})
}

type handler struct{}

func (h *handler) PlatformID() string { return PlatformID }

type douyinContentJSON struct {
	URL string `json:"url"`
}

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, configRaw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	var input douyinContentJSON
	if err := json.Unmarshal(contentJSON, &input); err != nil {
		return nil, fmt.Errorf("解析抖音URL失败: %w", err)
	}
	if input.URL == "" {
		return nil, fmt.Errorf("抖音URL不能为空")
	}

	// Get cookie from config
	cookie := ""
	if cfg := GetDouyinConfig(); cfg != nil {
		cookie = cfg.Cookie
	}

	// Call scraper to get video info (prefer mobile, fall back to web)
	client := scraper.NewClient(cookie)
	videoInfo, err := client.GetVideoInfo(input.URL)
	if err != nil {
		return nil, fmt.Errorf("获取抖音视频信息失败: %w", err)
	}

	now := util.NowMillis()

	// Build Content
	content := &model.Content{
		Id:         BuildContentID(videoInfo.VideoID),
		PlatformId: platformIDDouyin,
		ExternalId: videoInfo.VideoID,
		Type:       "video",
		Title:      videoInfo.Title,
		URL:        videoInfo.URL,
		CoverURL:   videoInfo.CoverURL,
		SourceURL:  input.URL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// Build Account (Douyin mobile scraper lacks author info, falls back to videoID)
	accountExternalID := videoInfo.VideoID
	account := &model.Account{
		Id:         BuildAccountID(accountExternalID),
		PlatformId: platformIDDouyin,
		ExternalId: accountExternalID,
		Nickname:   "抖音用户",
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// Build download task
	title, _ := config["filename"].(string)
	if title == "" {
		title = videoInfo.Title
	}

	savePath, _ := config["save_path"].(string)
	if savePath == "" {
		savePath = "/downloads/douyin"
	}

	configJSON, _ := json.Marshal(buildConfigJSON(config))
	metadataJSON, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"external_id": videoInfo.VideoID,
		"title":       videoInfo.Title,
		"source":      videoInfo.Source,
		"download_at": time.Now().Unix(),
	})

	extraJSON := buildExtraJSON(videoInfo.VideoID, videoInfo.Title)
	contentID := content.Id

	// Cover resource
	var resources []*adapter.ResourceInfo

	downloadCover, _ := config["download_cover"].(bool)
	if downloadCover && videoInfo.CoverURL != "" {
		coverResource := model.DownloadResource{
			ContentId: &contentID,
			Name:      title,
			Kind:      "image",
			UniqueID:  videoInfo.VideoID + "_cover",
			Extra:     extraJSON,
		}
		coverEndpoint := model.DownloadEndpoint{
			Protocol: "https",
			URL:      videoInfo.CoverURL,
			Enabled:  1,
		}
		resources = append(resources, &adapter.ResourceInfo{
			DownloadResource: coverResource,
			Endpoints:        []model.DownloadEndpoint{coverEndpoint},
		})
	}

	// Video resource
	videoResource := model.DownloadResource{
		ContentId: &contentID,
		Name:      title + ".mp4",
		Kind:      "video",
		UniqueID:  videoInfo.VideoID,
		Extra:     extraJSON,
	}
	videoEndpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      videoInfo.URL,
		Enabled:  1,
	}
	resources = append(resources, &adapter.ResourceInfo{
		DownloadResource: videoResource,
		Endpoints:        []model.DownloadEndpoint{videoEndpoint},
	})

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     videoInfo.VideoID,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			ConfigJSON:   string(configJSON),
			MetadataJSON: string(metadataJSON),
		},
		Resources: resources,
		ContentDetail: &model.ContentVideo{
			Id:     content.Id,
			URL:    videoInfo.URL,
			Format: "mp4",
		},
		Account: account,
		Content: content,
	}, nil
}

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(externalID string) string {
	return platformIDDouyin + ":" + externalID
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(externalID string) string {
	return platformIDDouyin + ":" + externalID
}

// buildExtraJSON builds the resource.Extra JSON string.
func buildExtraJSON(id, title string) string {
	data, _ := json.Marshal(map[string]string{
		"id":    id,
		"title": title,
	})
	return string(data)
}

// buildConfigJSON returns a map containing only the non-empty config fields.
func buildConfigJSON(config map[string]any) map[string]any {
	m := make(map[string]any, len(config))
	for key, value := range config {
		m[key] = value
	}
	return m
}
