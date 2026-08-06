package bilibili

import (
	"encoding/json"
	"fmt"
	"time"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/bilibili"
	"wx_channel/pkg/util"
)

const platformIDBilibili = "bilibili"

// PlatformID is the platform identifier for bilibili.
const PlatformID = platformIDBilibili

func init() {
	adapter.Register(&handler{})
}

type handler struct{}

func (h *handler) PlatformID() string { return PlatformID }

func (h *handler) BuildBrowseHistory(contentJSON json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

type bilibiliContentJSON struct {
	URL     string `json:"url"`
	PageNum int    `json:"page_num"`
}

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, configRaw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	var input bilibiliContentJSON
	if err := json.Unmarshal(contentJSON, &input); err != nil {
		return nil, fmt.Errorf("解析B站URL失败: %w", err)
	}
	if input.URL == "" {
		return nil, fmt.Errorf("B站URL不能为空")
	}

	cookie := ""
	if cfg := GetBilibiliConfig(); cfg != nil {
		cookie = cfg.Cookie
	}

	// Call scraper to get video info
	client := scraper.NewClient(cookie)
	videoInfos, err := client.GetVideoInfo(input.URL, input.PageNum)
	if err != nil {
		return nil, fmt.Errorf("获取B站视频信息失败: %w", err)
	}
	if len(videoInfos) == 0 {
		return nil, fmt.Errorf("未获取到B站视频")
	}

	return buildTaskFromVideoInfo(videoInfos[0], input.URL, config)
}

func buildTaskFromVideoInfo(info *scraper.VideoInfo, sourceURL string, config map[string]any) (*adapter.DownloadTaskResult, error) {
	now := util.NowMillis()

	content := &model.Content{
		Id:         BuildContentID(info.VideoID),
		PlatformId: platformIDBilibili,
		ExternalId: info.VideoID,
		Type:       "video",
		Title:      info.Title,
		URL:        info.URL,
		CoverURL:   info.CoverURL,
		SourceURL:  sourceURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	account := &model.Account{
		Id:         BuildAccountID(info.VideoID),
		PlatformId: platformIDBilibili,
		ExternalId: info.VideoID,
		Nickname:   "B站用户",
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	title, _ := config["filename"].(string)
	if title == "" {
		title = info.Title
	}

	savePath, _ := config["save_path"].(string)
	if savePath == "" {
		savePath = "/downloads/bilibili"
	}

	configJSON, _ := json.Marshal(buildConfigJSON(config))
	metadataJSON, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"external_id": info.VideoID,
		"title":       info.Title,
		"download_at": time.Now().Unix(),
	})

	extraJSON := buildExtraJSON(info.VideoID, info.Title)
	contentID := content.Id

	var resources []*adapter.ResourceInfo

	// Cover resource (optional)
	downloadCover, _ := config["download_cover"].(bool)
	if downloadCover && info.CoverURL != "" {
		coverResource := model.DownloadResource{
			ContentId: &contentID,
			Name:      title,
			Kind:      "image",
			UniqueID:  info.VideoID + "_cover",
			Extra:     extraJSON,
		}
		resources = append(resources, &adapter.ResourceInfo{
			DownloadResource: coverResource,
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      info.CoverURL,
				Enabled:  1,
			}},
		})
	}

	// Video resource
	videoResource := model.DownloadResource{
		ContentId: &contentID,
		Name:      title + ".mp4",
		Kind:      "video",
		UniqueID:  info.VideoID,
		Extra:     extraJSON,
	}
	videoEndpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      info.URL,
		Enabled:  1,
	}
	resources = append(resources, &adapter.ResourceInfo{
		DownloadResource: videoResource,
		Endpoints:        []model.DownloadEndpoint{videoEndpoint},
	})

	// DASH audio resource (anime/courses etc.)
	if info.AudioURL != "" {
		audioResource := model.DownloadResource{
			ContentId:  &contentID,
			Name:       title + ".audio.m4a",
			Kind:       "audio",
			UniqueID:   info.VideoID + "_audio",
			MergeOrder: 1,
			Extra:      extraJSON,
		}
		resources = append(resources, &adapter.ResourceInfo{
			DownloadResource: audioResource,
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      info.AudioURL,
				Enabled:  1,
			}},
		})
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     info.VideoID,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			ConfigJSON:   string(configJSON),
			MetadataJSON: string(metadataJSON),
		},
		Resources: resources,
		ContentDetail: &model.ContentVideo{
			Id:     content.Id,
			URL:    info.URL,
			Format: "mp4",
		},
		Account: account,
		Content: content,
	}, nil
}

func BuildContentID(externalID string) string {
	return platformIDBilibili + ":" + externalID
}

func BuildAccountID(externalID string) string {
	return platformIDBilibili + ":" + externalID
}

func buildExtraJSON(id, title string) string {
	data, _ := json.Marshal(map[string]string{
		"id":    id,
		"title": title,
	})
	return string(data)
}

func buildConfigJSON(config map[string]any) map[string]any {
	m := make(map[string]any, len(config))
	for key, value := range config {
		m[key] = value
	}
	return m
}
