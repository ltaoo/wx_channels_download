package vod

import (
	"encoding/json"
	"fmt"
	"strings"

	"strconv"
	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

func (h *Handler) platform_account() *model.Account {
	return &model.Account{Id: h.platform + ":official", PlatformId: h.platform, ExternalId: "official", Nickname: h.spec.Name, ProfileURL: h.spec.Homepage}
}

func (h *Handler) build_models(result *Result) (*model.Content, any, []adapter.ContentDetail, []*model.Content) {
	now := util.NowMillis()
	metadata, _ := json.Marshal(result.Profile)
	profile := result.Profile
	var series *model.Content
	var series_detail *model.ContentSeries
	details := []adapter.ContentDetail{}
	if profile.Kind == "series" {
		series_id := result.Platform + ":series:" + profile.ID
		series = &model.Content{
			Id: series_id, PlatformId: result.Platform, ExternalId: "series:" + profile.ID,
			Type: model.ContentTypeCollection, Subtype: model.ContentSubtypeSeries,
			Title: profile.Title, Description: profile.Description, CoverURL: profile.CoverURL,
			SourceURL: first_text(profile.Homepage, result.SourceURL), Metadata: string(metadata),
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		series_detail = &model.ContentSeries{
			Id: series_id, Name: profile.Title, Overview: profile.Description,
			PosterPath: profile.CoverURL, BackdropPath: profile.BackdropURL, AirDate: profile.ReleaseDate,
			EpisodeCount: profile.EpisodeCount, SourceCount: profile.AvailableEpisodeCount, MetadataJSON: string(metadata), Homepage: first_text(profile.Homepage, result.SourceURL),
			GenresJSON: optional_json(profile.Genres), OriginCountryJSON: optional_json(profile.Countries),
			Status: profile.Status, Tips: profile.Tips, Tagline: profile.Tagline,
		}
		if profile.Status == "连载中" {
			series_detail.InProduction = 1
		}
		details = append(details, adapter.ContentDetail{Type: "series", Key: series_id, Content: series, Data: series_detail, Influencers: content_influencers(profile.Credits, result.Platform)})
	}
	contents := make([]*model.Content, 0, len(result.Episodes))
	var primary_video *model.ContentVideo
	for _, episode := range result.Episodes {
		content_id := result.Platform + ":" + episode.ID
		title := strings.TrimSpace(episode.Title)
		if title == "" {
			title = profile.Title
			if episode.Order > 0 {
				title = fmt.Sprintf("%s 第%d集", title, episode.Order)
			}
		}
		if title == "" {
			title = episode.ID
		}
		subtype := model.ContentSubtypeLongVideo
		if profile.Kind == "movie" {
			subtype = model.ContentSubtypeMovie
		} else if series != nil {
			subtype = model.ContentSubtypeEpisode
		}
		content := &model.Content{
			Id: content_id, PlatformId: result.Platform, ExternalId: episode.ID,
			Type: model.ContentTypeVideo, Subtype: subtype, Title: title,
			Description: first_text(episode.Description, profile.Description), CoverURL: first_text(episode.CoverURL, profile.CoverURL),
			SourceURL: episode.SourceURL, URL: episode.URL, Metadata: string(metadata),
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		video := &model.ContentVideo{
			Id: content_id, URL: episode.URL, Duration: int64(episode.Duration),
			Width: episode.Width, Height: episode.Height, Codec: episode.Codec, Format: "mp4",
		}
		stream_type := h.spec.VideoStreamType
		if stream_type == "" {
			stream_type = model.ContentVideoVariantStreamTypeManifest
		}
		variant := model.ContentVideoVariant{VideoId: content_id, VariantKey: "main", StreamType: stream_type, Format: "mp4", Codec: episode.Codec, URL: episode.URL, HasVideo: 1, HasAudio: 1, IsDefault: 1}
		if episode.Width > 0 {
			width := episode.Width
			variant.Width = &width
		}
		if episode.Height > 0 {
			height := episode.Height
			variant.Height = &height
		}
		video.Variants = []model.ContentVideoVariant{variant}
		video_detail := adapter.ContentDetail{Type: "video", Key: content_id, Content: content, Data: video}
		if series == nil {
			video_detail.Influencers = content_influencers(profile.Credits, result.Platform)
		}
		details = append(details, video_detail)
		if series != nil {
			episode_metadata, _ := json.Marshal(episode)
			episode_detail := &model.ContentEpisode{
				Id: content_id, Name: title, Overview: first_text(episode.Description, profile.Description), StillPath: first_text(episode.CoverURL, profile.CoverURL),
				SortOrder: episode.Order, EpisodeNumber: episode_number(episode.Order),
				Duration: int64(episode.Duration), Runtime: int(episode.Duration / 60), MetadataJSON: string(episode_metadata),
				AirDate: episode.ReleaseDate, LongTitle: episode.LongTitle, Badge: episode.Badge,
			}
			details = append(details, adapter.ContentDetail{
				Type: "episode", Key: content_id + ":episode", Content: content, Data: episode_detail,
				Relation: &model.ContentRelation{
					SourceContentId: content_id, TargetContentId: series.Id,
					Type: model.ContentRelationEpisodeOf, SortOrder: episode.Order, CreatedAt: now,
				},
			})
		}
		contents = append(contents, content)
		if primary_video == nil {
			primary_video = video
		}
	}
	_, _, video_id, _ := h.spec.ParseURL(result.SourceURL)
	if series != nil && video_id == "" {
		return series, series_detail, details, contents
	}
	return contents[0], primary_video, details, contents
}

func optional_json(values []string) string {
	if len(values) == 0 {
		return ""
	}
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func first_text(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func (h *Handler) build_task(result *Result, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	settings := map[string]any{}
	if len(config_json) > 0 && string(config_json) != "null" {
		if err := json.Unmarshal(config_json, &settings); err != nil {
			return nil, fmt.Errorf("解析下载配置失败: %w", err)
		}
	}
	content, primary, details, contents := h.build_models(result)
	task_name := content.Title
	if filename, ok := settings["filename"].(string); ok && strings.TrimSpace(filename) != "" {
		task_name = strings.TrimSpace(filename)
	}
	config_data, _ := json.Marshal(settings)
	metadata, _ := json.Marshal(result.Profile)
	now := util.NowMillis()
	task := &model.DownloadTask{Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now}, ContentId: &content.Id, Name: task_name, UniqueID: content.ExternalId, PlatformId: h.platform, Status: model.TaskStatusWaiting, SourceURL: result.SourceURL, CoverURL: content.CoverURL, ConfigJSON: string(config_data), MetadataJSON: string(metadata)}
	output := &adapter.DownloadTaskResult{Task: task, Content: content, Account: h.platform_account(), ContentDetail: primary, ContentDetails: details}
	// The cookie travels only to official metadata endpoints. The scraper's media
	// transport intentionally omits it from CDN requests and cross-host redirects.
	headers, _ := json.Marshal(map[string]string{"Cookie": h.cookie()})
	for index, episode := range result.Episodes {
		episode_content := contents[index]
		input_episode := episode
		input_episode.URL = ""
		payload, _ := json.Marshal(Input{Platform: h.platform, SourceURL: episode.SourceURL, Episode: input_episode})
		name := task_name
		if len(result.Episodes) > 1 {
			name = fmt.Sprintf("%03d_%s", index+1, episode_content.Title)
		}
		output.Resources = append(output.Resources, &adapter.ResourceInfo{
			Resource:      model.DownloadResource{Extra: string_marshal(map[string]string{"vod_input": string(payload)}), ContentId: &episode_content.Id, Name: name, Kind: "video/mp4", UniqueID: episode.ID, Duration: int64(episode.Duration), MergeOrder: index},
			Endpoints:     []model.DownloadEndpoint{{Protocol: SourceProtocol, URL: string(payload), Enabled: 1, Headers: string(headers)}},
			ContentAssets: []adapter.ContentAssetReference{{Kind: model.ContentAssetKindVideo, Role: model.ContentAssetRoleVideoVariant, AssetKey: "main", Relation: model.DownloadResourceAssetRelationSource}},
		})
	}
	if download_cover, ok := settings["download_cover"].(bool); ok && download_cover && content.CoverURL != "" {
		output.Resources = append(output.Resources, &adapter.ResourceInfo{Resource: model.DownloadResource{ContentId: &content.Id, Name: task_name + "_cover", Kind: "image/jpeg", UniqueID: content.ExternalId + "_cover"}, Endpoints: []model.DownloadEndpoint{{Protocol: "https", URL: content.CoverURL, Enabled: 1}}, ContentAssets: []adapter.ContentAssetReference{{Kind: model.ContentAssetKindImage, Role: model.ContentAssetRoleCover, AssetKey: "cover", Relation: model.DownloadResourceAssetRelationSource}}})
	}
	return output, nil
}

func episode_number(order int) string {
	if order <= 0 {
		return ""
	}
	return strconv.Itoa(order)
}
