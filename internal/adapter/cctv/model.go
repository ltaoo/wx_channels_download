package cctvadapter

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/cctv"
	"wx_channel/pkg/util"
)

// PlatformID is the platform identifier used for CCTV content.
const PlatformID = cctv.PlatformID

// VideoModelSet contains all shared database models derived from a CCTV fetch.
type VideoModelSet struct {
	Content *model.Content
	Video   *model.ContentVideo
	Account *model.Account
}

// BuildContentID builds a stable content ID from a CCTV video GUID.
func BuildContentID(pid string) string {
	return PlatformID + ":" + strings.ToLower(strings.TrimSpace(pid))
}

// BuildAccountID builds a stable account ID from a CCTV producer ID.
func BuildAccountID(external_id string) string {
	return PlatformID + ":" + strings.ToLower(strings.TrimSpace(external_id))
}

// ToContent converts a CCTV fetch result into the common content model.
func ToContent(result *cctv.FetchResult) (*model.Content, error) {
	result, err := validate_result(result)
	if err != nil {
		return nil, err
	}
	metadata_data, _ := json.Marshal(map[string]any{
		"ack":             result.Data.Ack,
		"status":          result.Data.Status,
		"api_url":         result.APIURL,
		"page_info_url":   result.PageInfoURL,
		"page_info":       result.PageInfo.Data,
		"play_channel":    result.Data.PlayChannel,
		"produce":         result.Data.Produce,
		"produce_id":      result.Data.ProduceID,
		"editor_name":     result.Data.EditorName,
		"column":          result.Data.Column,
		"default_stream":  result.Data.DefaultStream,
		"manifest":        result.Data.Manifest,
		"page_content_id": result.PageContentID,
		"cmstid":          result.CMSTID,
	})
	tags := ""
	if tag := first_non_empty(result.Data.Tag, result.PageKeywords); tag != "" {
		tags_data, _ := json.Marshal([]string{tag})
		tags = string(tags_data)
	}
	now := util.NowMillis()
	return &model.Content{
		Id:          BuildContentID(result.PID),
		PlatformId:  PlatformID,
		Type:        model.ContentTypeVideo,
		ExternalId:  strings.ToLower(strings.TrimSpace(result.PID)),
		ExternalId2: strings.TrimSpace(result.PageContentID),
		ExternalId3: strings.TrimSpace(result.CMSTID),
		Title:       first_non_empty(result.Data.Title, result.PageTitle, result.PID),
		Description: strings.TrimSpace(result.PageDescription),
		URL:         strings.TrimSpace(result.Data.HLSURL),
		SourceURL:   strings.TrimSpace(result.PageURL),
		CoverURL:    strings.TrimSpace(result.Data.Image),
		PublishTime: cctv_publish_time(result.Data.ProgramTime),
		Tags:        tags,
		Category:    first_non_empty(result.Data.Column, result.Data.PlayChannel),
		Metadata:    string(metadata_data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToAccount converts CCTV producer metadata into the common account model.
func ToAccount(result *cctv.FetchResult) (*model.Account, error) {
	result, err := validate_result(result)
	if err != nil {
		return nil, err
	}
	media_page := result.PageInfo.Data
	external_id := first_non_empty(media_page.MediaID, result.Data.ProduceID, result.Data.PlayChannel, "cctv")
	nickname := first_non_empty(media_page.MediaName, result.Data.Produce, result.Data.PlayChannel, "央视网")
	now := util.NowMillis()
	return &model.Account{
		Id:            BuildAccountID(external_id),
		PlatformId:    PlatformID,
		ExternalId:    strings.TrimSpace(external_id),
		Nickname:      nickname,
		Signature:     strings.TrimSpace(media_page.Brief),
		AvatarURL:     strings.TrimSpace(media_page.LogoImage),
		ProfileURL:    first_non_empty(media_page.WebURL, "https://www.cctv.com/"),
		FollowerCount: media_page.BeSubscribed,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToContentVideo converts the CCTV HLS stream into the video extension model.
func ToContentVideo(result *cctv.FetchResult) (*model.ContentVideo, error) {
	result, err := validate_result(result)
	if err != nil {
		return nil, err
	}
	content_id := BuildContentID(result.PID)
	hls_url := strings.TrimSpace(result.Data.HLSURL)
	video := &model.ContentVideo{
		Id:       content_id,
		Duration: video_duration_seconds(result.Data.Video.TotalLength),
		Format:   manifest_format(hls_url),
		URL:      hls_url,
	}
	if hls_url == "" {
		return video, nil
	}
	now := util.NowMillis()
	variant_metadata, _ := json.Marshal(map[string]any{
		"cdn":            result.Data.HLSCDNInfo,
		"default_stream": result.Data.DefaultStream,
	})
	video.Variants = []model.ContentVideoVariant{{
		VideoId:    content_id,
		VariantKey: "hls",
		Quality:    strings.TrimSpace(result.Data.DefaultStream),
		Format:     manifest_format(hls_url),
		StreamType: model.ContentVideoVariantStreamTypeManifest,
		HasVideo:   1,
		HasAudio:   1,
		IsDefault:  1,
		URL:        hls_url,
		Metadata:   string(variant_metadata),
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}}
	return video, nil
}

// ToContentDetails returns the normalized video detail for a CCTV fetch.
func ToContentDetails(result *cctv.FetchResult) ([]adapter.ContentDetail, error) {
	content, err := ToContent(result)
	if err != nil {
		return nil, err
	}
	video, err := ToContentVideo(result)
	if err != nil {
		return nil, err
	}
	return []adapter.ContentDetail{{
		Type: content.Type,
		Key:  content.Id,
		Data: video,
	}}, nil
}

// ToVideoModelSet converts a CCTV fetch into every supported shared model.
func ToVideoModelSet(result *cctv.FetchResult) (*VideoModelSet, error) {
	content, err := ToContent(result)
	if err != nil {
		return nil, err
	}
	video, err := ToContentVideo(result)
	if err != nil {
		return nil, err
	}
	account, err := ToAccount(result)
	if err != nil {
		return nil, err
	}
	return &VideoModelSet{Content: content, Video: video, Account: account}, nil
}

// BuildDownloadTask builds a finite CCTV VOD recording task from its HLS
// manifest. The livestream protocol delegates HLS segment download and remuxing
// to FFmpeg instead of saving only the m3u8 manifest.
func BuildDownloadTask(result *cctv.FetchResult, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	model_set, err := ToVideoModelSet(result)
	if err != nil {
		return nil, err
	}
	hls_url := strings.TrimSpace(result.Data.HLSURL)
	if hls_url == "" {
		return nil, fmt.Errorf("CCTV 视频未返回 HLS 地址")
	}
	config := make(map[string]any)
	config_text := strings.TrimSpace(string(config_json))
	if config_text != "" && config_text != "null" {
		if err := json.Unmarshal(config_json, &config); err != nil {
			return nil, fmt.Errorf("解析 CCTV 下载配置失败: %w", err)
		}
	}
	if config == nil {
		config = make(map[string]any)
	}
	task_name := strings.TrimSpace(config_string(config, "filename"))
	if task_name == "" {
		task_name = model_set.Content.Title
	}
	config_data, _ := json.Marshal(config)
	metadata_data, _ := json.Marshal(map[string]any{
		"platform":      PlatformID,
		"external_id":   model_set.Content.ExternalId,
		"title":         model_set.Content.Title,
		"api_url":       result.APIURL,
		"page_info_url": result.PageInfoURL,
		"media_id":      result.PageInfo.Data.MediaID,
		"media_name":    result.PageInfo.Data.MediaName,
		"duration":      model_set.Video.Duration,
	})
	resource_extra, _ := json.Marshal(map[string]any{
		"source_url": result.PageURL,
		"hls_url":    hls_url,
	})
	headers_data, _ := json.Marshal(map[string]string{
		"User-Agent": cctv.DefaultUserAgent,
		"Referer":    "https://v.cctv.com/",
	})
	content_id := model_set.Content.Id
	now := util.NowMillis()
	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content_id,
			Name:         task_name,
			UniqueID:     model_set.Content.ExternalId,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    model_set.Content.SourceURL,
			CoverURL:     model_set.Content.CoverURL,
			ConfigJSON:   string(config_data),
			MetadataJSON: string(metadata_data),
			Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		Resources: []*adapter.ResourceInfo{{
			Resource: model.DownloadResource{
				ContentId: &content_id,
				Name:      task_name,
				Kind:      "video/x-matroska",
				UniqueID:  model_set.Content.ExternalId,
				Type:      model.ResourceTypeStream,
				StreamURL: hls_url,
				Duration:  model_set.Video.Duration,
				Extra:     string(resource_extra),
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "livestream",
				URL:      hls_url,
				Enabled:  1,
				Headers:  string(headers_data),
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindVideo,
				Role:     model.ContentAssetRoleVideoVariant,
				AssetKey: "hls",
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		}},
		ContentDetail: model_set.Video,
		ContentDetails: []adapter.ContentDetail{{
			Type: model_set.Content.Type,
			Key:  model_set.Content.Id,
			Data: model_set.Video,
		}},
		Account: model_set.Account,
		Content: model_set.Content,
	}, nil
}

func validate_result(result *cctv.FetchResult) (*cctv.FetchResult, error) {
	if result == nil {
		return nil, fmt.Errorf("cctv fetch result is nil")
	}
	if strings.TrimSpace(result.PID) == "" {
		return nil, fmt.Errorf("cctv video pid is empty")
	}
	if strings.TrimSpace(result.Data.Title) == "" && strings.TrimSpace(result.PageTitle) == "" {
		return nil, fmt.Errorf("cctv video title is empty")
	}
	return result, nil
}

func cctv_publish_time(value string) *int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	china_standard_time := time.FixedZone("CST", 8*60*60)
	parsed_time, err := time.ParseInLocation("2006-01-02 15:04:05", value, china_standard_time)
	if err != nil {
		return nil
	}
	publish_time := parsed_time.UnixMilli()
	return &publish_time
}

func video_duration_seconds(value string) int64 {
	duration, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || duration <= 0 {
		return 0
	}
	return int64(math.Round(duration))
}

func manifest_format(raw_url string) string {
	if strings.Contains(strings.ToLower(raw_url), ".m3u8") {
		return "m3u8"
	}
	return ""
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func config_string(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}
