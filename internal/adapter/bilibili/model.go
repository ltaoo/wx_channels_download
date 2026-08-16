package bilibiliadapter

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/bilibili"
	"wx_channel/pkg/util"
)

const bilibili_download_user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36"
const bilibili_bangumi_account_external_id = "bangumi"
const bilibili_bangumi_account_nickname = "bilibili番剧"

// BuildContentID returns the stable content ID for a Bilibili object.
func BuildContentID(external_id string) string {
	return platform_id_bilibili + ":" + external_id
}

// BuildAccountID returns the stable placeholder account ID for a Bilibili
// object whose uploader details are unavailable.
func BuildAccountID(external_id string) string {
	return platform_id_bilibili + ":" + external_id
}

func build_task_from_video_info(info *bilibili.VideoInfo, source_url string, config map[string]any) (*adapter.DownloadTaskResult, error) {
	if info == nil || strings.TrimSpace(info.VideoID) == "" {
		return nil, fmt.Errorf("B站视频信息为空")
	}
	now := util.NowMillis()
	external_id := strings.TrimSpace(info.VideoID)
	content_id := BuildContentID(external_id)
	content := &model.Content{
		Id:         content_id,
		PlatformId: platform_id_bilibili,
		ExternalId: external_id,
		Type:       model.ContentTypeVideo,
		Title:      info.Title,
		URL:        info.URL,
		CoverURL:   info.CoverURL,
		SourceURL:  source_url,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	account := build_bilibili_account(external_id, now)
	task_name := bilibili_task_name(config, info.Title, external_id)
	config_data, _ := json.Marshal(copy_config(config))
	metadata_data, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"external_id": external_id,
		"title":       info.Title,
		"download_at": time.Now().Unix(),
	})
	extra_json := build_extra_json(external_id, info.Title, source_url, nil)
	resources := make([]*adapter.ResourceInfo, 0, 3)

	if config_bool(config, "download_cover") && strings.TrimSpace(info.CoverURL) != "" {
		resources = append(resources, build_cover_resource(content_id, external_id, task_name, info.CoverURL, extra_json))
	}
	resources = append(resources, &adapter.ResourceInfo{
		Resource: model.DownloadResource{
			ContentId: &content_id,
			Name:      task_name,
			Kind:      "video",
			UniqueID:  external_id,
			Extra:     extra_json,
		},
		Endpoints: []model.DownloadEndpoint{{
			Protocol: bilibili_endpoint_protocol(info.URL),
			URL:      info.URL,
			Enabled:  1,
		}},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindVideo,
			Role:     model.ContentAssetRoleVideoVariant,
			AssetKey: "default",
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	})
	if strings.TrimSpace(info.AudioURL) != "" {
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId:  &content_id,
				Name:       task_name + "_audio",
				Kind:       "audio",
				UniqueID:   external_id + "_audio",
				MergeOrder: 1,
				Extra:      extra_json,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: bilibili_endpoint_protocol(info.AudioURL),
				URL:      info.AudioURL,
				Enabled:  1,
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindAudio,
				Role:     model.ContentAssetRoleAudioVariant,
				AssetKey: external_id + "_audio",
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content_id,
			Name:         task_name,
			UniqueID:     external_id,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    source_url,
			CoverURL:     info.CoverURL,
			ConfigJSON:   string(config_data),
			MetadataJSON: string(metadata_data),
		},
		Resources: resources,
		ContentDetail: &model.ContentVideo{
			Id:     content_id,
			URL:    info.URL,
			Format: "mp4",
		},
		Account: account,
		Content: content,
	}, nil
}

func build_task_from_bangumi_info(info *bilibili.BangumiInfo, config map[string]any) (*adapter.DownloadTaskResult, error) {
	if info == nil {
		return nil, fmt.Errorf("B站番剧信息为空")
	}
	external_id := bangumi_external_id(info)
	if external_id == "" {
		return nil, fmt.Errorf("B站番剧缺少 ep_id、bvid 和 aid")
	}

	video_stream, err := select_bangumi_video_stream(
		info.PlayURLSSRData.Data.Result.VideoInfo.Dash.Video,
		config,
	)
	if err != nil {
		return nil, err
	}
	if video_stream == nil {
		return nil, fmt.Errorf("B站番剧没有可下载的视频流")
	}
	audio_stream := best_bangumi_audio_stream(info.PlayURLSSRData.Data.Result.VideoInfo.Dash.Audio)
	now := util.NowMillis()
	content_id := bangumi_video_content_id(info)
	if content_id == "" {
		return nil, fmt.Errorf("B站番剧缺少底层视频标识")
	}
	content_title := bangumi_title(info)
	cover_url := bangumi_cover_url(info)
	source_url := bangumi_source_url(info)
	task_name := bilibili_task_name(config, content_title, "ep"+external_id)
	content_video := bangumi_content_video(info, video_stream)
	content := bangumi_video_content(info, video_stream, now)
	content_details, err := bangumi_content_details(info, video_stream, now)
	if err != nil {
		return nil, err
	}
	account := build_bangumi_account(now)
	config_data, _ := json.Marshal(copy_config(config))
	metadata_data, _ := json.Marshal(map[string]any{
		"platform":           PlatformID,
		"external_id":        external_id,
		"episode_id":         info.EpisodeID,
		"season_id":          info.SeasonID,
		"aid":                info.AID,
		"cid":                info.CID,
		"bvid":               info.BVID,
		"title":              content_title,
		"source_url":         source_url,
		"download_at":        time.Now().Unix(),
		"video_stream_count": len(info.PlayURLSSRData.Data.Result.VideoInfo.Dash.Video),
		"audio_stream_count": len(info.PlayURLSSRData.Data.Result.VideoInfo.Dash.Audio),
		"video_stream_id":    video_stream.ID,
		"video_codec":        video_stream.Codecs,
		"audio_stream_id":    bangumi_stream_id(audio_stream),
	})

	resources := make([]*adapter.ResourceInfo, 0, 3)
	if config_bool(config, "download_cover") && cover_url != "" {
		cover_extra := build_extra_json(external_id, content_title, source_url, nil)
		resources = append(resources, build_cover_resource(content_id, external_id, task_name, cover_url, cover_extra))
	}
	resources = append(resources, build_bangumi_stream_resource(
		content_id,
		external_id,
		task_name,
		source_url,
		"video",
		0,
		video_stream,
	))
	if audio_stream != nil {
		resources = append(resources, build_bangumi_stream_resource(
			content_id,
			external_id,
			task_name+"_audio",
			source_url,
			"audio",
			1,
			audio_stream,
		))
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content_id,
			Name:         task_name,
			UniqueID:     external_id,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    source_url,
			CoverURL:     cover_url,
			ConfigJSON:   string(config_data),
			MetadataJSON: string(metadata_data),
		},
		Resources:      resources,
		ContentDetail:  content_video,
		ContentDetails: content_details,
		Account:        account,
		Content:        content,
	}, nil
}

func build_bangumi_stream_resource(
	content_id string,
	external_id string,
	resource_name string,
	source_url string,
	stream_kind string,
	merge_order int,
	stream *bilibili.BangumiDashStream,
) *adapter.ResourceInfo {
	asset_kind := model.ContentAssetKindVideo
	asset_role := model.ContentAssetRoleVideoVariant
	if stream_kind == "audio" {
		asset_kind = model.ContentAssetKindAudio
		asset_role = model.ContentAssetRoleAudioVariant
	}
	stream_key := bangumi_stream_key(stream_kind, stream)
	extra_json := build_extra_json(external_id, resource_name, source_url, stream)
	return &adapter.ResourceInfo{
		Resource: model.DownloadResource{
			ContentId:  &content_id,
			Name:       resource_name,
			Kind:       bangumi_stream_mime_type(stream_kind, stream),
			UniqueID:   external_id + "_" + stream_key,
			Size:       stream.Size,
			MergeOrder: merge_order,
			Extra:      extra_json,
		},
		Endpoints: bangumi_stream_endpoints(stream, source_url),
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     asset_kind,
			Role:     asset_role,
			AssetKey: stream_key,
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	}
}

func build_cover_resource(content_id string, external_id string, task_name string, cover_url string, extra_json string) *adapter.ResourceInfo {
	return &adapter.ResourceInfo{
		Resource: model.DownloadResource{
			ContentId:  &content_id,
			Name:       task_name,
			Kind:       "image",
			UniqueID:   external_id + "_cover",
			MergeOrder: 999,
			Extra:      extra_json,
		},
		Endpoints: []model.DownloadEndpoint{{
			Protocol: bilibili_endpoint_protocol(cover_url),
			URL:      cover_url,
			Enabled:  1,
		}},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindImage,
			Role:     model.ContentAssetRoleCover,
			AssetKey: "cover",
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	}
}

func bangumi_content_video(info *bilibili.BangumiInfo, selected_stream *bilibili.BangumiDashStream) *model.ContentVideo {
	if info == nil {
		return nil
	}
	video_info := info.PlayURLSSRData.Data.Result.VideoInfo
	content_video := &model.ContentVideo{
		Id:              bangumi_video_content_id(info),
		Duration:        video_info.TimeLength / 1000,
		AudioTrackCount: len(video_info.Dash.Audio),
	}
	if target_episode := info.PageData.TargetEpisode(); target_episode != nil {
		if target_episode.Duration > 0 {
			content_video.Duration = target_episode.Duration / 1000
		}
		content_video.PlayTimes = target_episode.Stat.Play
	}
	if selected_stream != nil {
		content_video.Width = selected_stream.Width
		content_video.Height = selected_stream.Height
		content_video.FPS = parse_bangumi_frame_rate(selected_stream.FrameRate)
		content_video.Bitrate = int64_to_int(selected_stream.Bandwidth)
		content_video.Size = selected_stream.Size
		content_video.Codec = selected_stream.Codecs
		content_video.Format = bangumi_stream_format(selected_stream)
		content_video.URL = selected_stream.BaseURL
	}
	content_video.Variants = bangumi_video_variants(info, selected_stream)
	return content_video
}

func bangumi_content_details(info *bilibili.BangumiInfo, selected_stream *bilibili.BangumiDashStream, now int64) ([]adapter.ContentDetail, error) {
	video_content := bangumi_video_content(info, selected_stream, now)
	if video_content == nil || video_content.Id == "" {
		return nil, fmt.Errorf("B站番剧缺少底层视频标识")
	}
	episode_content := bangumi_episode_content(info, now)
	if episode_content == nil || episode_content.Id == "" {
		return nil, fmt.Errorf("B站番剧缺少 episode 标识")
	}
	series_content := bangumi_series_content(info, now)
	if series_content == nil || series_content.Id == "" {
		return nil, fmt.Errorf("B站番剧缺少 series 标识")
	}

	return []adapter.ContentDetail{
		{
			Type:    "video",
			Key:     video_content.Id,
			Data:    bangumi_content_video(info, selected_stream),
			Content: video_content,
		},
		{
			Type:    "episode",
			Key:     episode_content.Id,
			Data:    bangumi_content_episode(info),
			Content: episode_content,
			Relation: &model.ContentRelation{
				SourceContentId: video_content.Id,
				TargetContentId: episode_content.Id,
				Type:            model.ContentRelationPartOf,
				CreatedAt:       now,
			},
		},
		{
			Type:        "series",
			Key:         series_content.Id,
			Data:        bangumi_content_series(info),
			Content:     series_content,
			Influencers: bangumi_content_influencers(info),
			Relation: &model.ContentRelation{
				SourceContentId: episode_content.Id,
				TargetContentId: series_content.Id,
				Type:            model.ContentRelationEpisodeOf,
				SortOrder:       bangumi_episode_sort_order(info),
				CreatedAt:       now,
			},
		},
	}, nil
}

func bangumi_video_content(info *bilibili.BangumiInfo, selected_stream *bilibili.BangumiDashStream, now int64) *model.Content {
	video_external_id := bangumi_video_external_id(info)
	if video_external_id == "" {
		return nil
	}
	content_url := bangumi_source_url(info)
	if selected_stream != nil && strings.TrimSpace(selected_stream.BaseURL) != "" {
		content_url = selected_stream.BaseURL
	}
	content := &model.Content{
		Id:          BuildContentID(video_external_id),
		PlatformId:  PlatformID,
		ExternalId:  video_external_id,
		ExternalId2: strconv.FormatInt(info.AID, 10),
		ExternalId3: strconv.FormatInt(info.CID, 10),
		Type:        model.ContentTypeVideo,
		Subtype:     model.ContentSubtypeLongVideo,
		Title:       bangumi_title(info),
		Description: info.Description,
		URL:         content_url,
		SourceURL:   bangumi_source_url(info),
		CoverURL:    bangumi_cover_url(info),
		Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	enrich_bangumi_content(content, info)
	return content
}

func bangumi_episode_content(info *bilibili.BangumiInfo, now int64) *model.Content {
	episode_external_id := bangumi_episode_external_id(info)
	if episode_external_id == "" {
		return nil
	}
	content := &model.Content{
		Id:          BuildContentID(episode_external_id),
		PlatformId:  PlatformID,
		ExternalId:  episode_external_id,
		ExternalId2: strings.TrimSpace(info.BVID),
		ExternalId3: strconv.FormatInt(info.CID, 10),
		Type:        model.ContentTypeVideo,
		Subtype:     model.ContentSubtypeEpisode,
		Title:       bangumi_title(info),
		Description: info.Description,
		URL:         bangumi_source_url(info),
		SourceURL:   bangumi_source_url(info),
		CoverURL:    bangumi_cover_url(info),
		Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	enrich_bangumi_content(content, info)
	return content
}

func bangumi_series_content(info *bilibili.BangumiInfo, now int64) *model.Content {
	season_external_id := bangumi_season_external_id(info)
	if season_external_id == "" {
		return nil
	}
	season := &info.SeasonData.Result
	collect_count := season.Stat.Favorites
	if collect_count == 0 {
		collect_count = season.Stat.Favorite
	}
	content := &model.Content{
		Id:           BuildContentID(season_external_id),
		PlatformId:   PlatformID,
		ExternalId:   season_external_id,
		ExternalId2:  strconv.FormatInt(season.MediaID, 10),
		Type:         model.ContentTypeCollection,
		Subtype:      model.ContentSubtypeSeries,
		Title:        bangumi_season_title(info),
		Description:  first_non_empty_bilibili_value(season.Evaluate, info.Description),
		URL:          bangumi_season_source_url(info),
		SourceURL:    bangumi_season_source_url(info),
		CoverURL:     bangumi_season_cover_url(info),
		ViewCount:    season.Stat.Views,
		LikeCount:    season.Stat.Likes,
		CommentCount: season.Stat.Reply,
		ShareCount:   season.Stat.Share,
		CollectCount: collect_count,
		Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	content.PublishTime = bangumi_series_publish_time(info)
	return content
}

func bangumi_content_episode(info *bilibili.BangumiInfo) *model.ContentEpisode {
	if info == nil {
		return nil
	}
	target_episode := info.PageData.TargetEpisode()
	season_episode := info.SeasonData.TargetEpisode(info.EpisodeID)
	episode := &model.ContentEpisode{Id: BuildContentID(bangumi_episode_external_id(info))}
	if target_episode == nil && season_episode == nil {
		return episode
	}
	metadata_data, _ := json.Marshal(map[string]any{
		"page_episode":   target_episode,
		"season_episode": season_episode,
	})
	episode.MediaType = info.SeasonData.Result.Type
	if episode.MediaType == 0 {
		episode.MediaType = info.SeasonType
	}
	episode.SeasonNumber = 1
	episode.SortOrder = bangumi_episode_sort_order(info)
	episode.SectionId = bangumi_target_section_id(info)
	if target_episode != nil {
		episode.Name = strings.TrimSpace(target_episode.LongTitle)
		episode.Overview = strings.TrimSpace(target_episode.ShareCopy)
		episode.AirDate = bangumi_episode_air_date(target_episode.PublishTime, target_episode.ReleaseDate)
		episode.StillPath = normalize_bilibili_asset_url(target_episode.Cover)
		episode.EpisodeNumber = strings.TrimSpace(target_episode.Title)
		episode.LongTitle = strings.TrimSpace(target_episode.LongTitle)
		episode.Duration = target_episode.Duration / 1000
		episode.Runtime = int(target_episode.Duration / 60000)
		episode.SectionType = target_episode.SectionType
		episode.Badge = strings.TrimSpace(target_episode.Badge)
		episode.VoteCount = target_episode.Stat.Likes
		episode.ProductionCode = strings.TrimSpace(target_episode.BVID)
	}
	if season_episode != nil {
		if episode.Name == "" {
			episode.Name = strings.TrimSpace(season_episode.LongTitle)
		}
		if episode.AirDate == "" {
			episode.AirDate = bangumi_episode_air_date(season_episode.PublishTime, season_episode.ReleaseDate)
		}
		if episode.StillPath == "" {
			episode.StillPath = normalize_bilibili_asset_url(season_episode.Cover)
		}
		if episode.EpisodeNumber == "" {
			episode.EpisodeNumber = strings.TrimSpace(season_episode.Title)
		}
		if episode.LongTitle == "" {
			episode.LongTitle = strings.TrimSpace(season_episode.LongTitle)
		}
		if episode.Duration == 0 {
			episode.Duration = season_episode.Duration / 1000
			episode.Runtime = int(season_episode.Duration / 60000)
		}
		if episode.ProductionCode == "" {
			episode.ProductionCode = strings.TrimSpace(season_episode.BVID)
		}
	}
	if episode.Name == "" {
		episode.Name = bangumi_title(info)
	}
	episode.MetadataJSON = string(metadata_data)
	return episode
}

func bangumi_content_series(info *bilibili.BangumiInfo) *model.ContentSeries {
	if info == nil {
		return nil
	}
	season := &info.SeasonData.Result
	metadata_data, _ := json.Marshal(season)
	origin_country_data, _ := json.Marshal(bangumi_area_names(season.Areas))
	genres_data, _ := json.Marshal(season.Styles)
	vote_average := float64(0)
	vote_count := int64(0)
	if season.Rating != nil {
		vote_average = season.Rating.Score
		vote_count = int64(season.Rating.Count)
	}
	media_type := season.Type
	if media_type == 0 {
		media_type = info.SeasonType
	}
	return &model.ContentSeries{
		Id:                BuildContentID(bangumi_season_external_id(info)),
		MediaType:         media_type,
		Name:              bangumi_season_title(info),
		OriginalName:      strings.TrimSpace(season.JPTitle),
		Alias:             strings.TrimSpace(season.Alias),
		Overview:          first_non_empty_bilibili_value(season.Evaluate, info.Description),
		PosterPath:        normalize_bilibili_asset_url(season.Cover),
		BackdropPath:      normalize_bilibili_asset_url(season.Background),
		AirDate:           bangumi_series_air_date(info),
		OriginalLanguage:  bangumi_original_language(season.Areas),
		OriginCountryJSON: string(origin_country_data),
		GenresJSON:        string(genres_data),
		SourceCount:       bangumi_episode_count(info),
		EpisodeCount:      bangumi_episode_count(info),
		SectionCount:      len(info.PageData.Result.Sections),
		SeasonCount:       1,
		VoteAverage:       vote_average,
		VoteCount:         vote_count,
		Popularity:        float64(season.Stat.Views),
		InProduction:      bool_int(season.Publish.IsFinish == 0),
		Status:            bangumi_series_status(season.Publish),
		Tips:              strings.TrimSpace(season.Subtitle),
		Homepage:          bangumi_season_source_url(info),
		Tagline:           strings.TrimSpace(season.SeasonTitle),
		MetadataJSON:      string(metadata_data),
	}
}

func bangumi_content_influencers(info *bilibili.BangumiInfo) []adapter.ContentInfluencerReference {
	if info == nil || len(info.Credits) == 0 {
		return nil
	}
	references := make([]adapter.ContentInfluencerReference, 0)
	reference_index_by_name := make(map[string]int)
	for credit_index := range info.Credits {
		credit := info.Credits[credit_index]
		name := strings.TrimSpace(credit.Name)
		role := strings.TrimSpace(credit.Role)
		if name == "" || role == "" {
			continue
		}
		reference_index, exists := reference_index_by_name[name]
		if !exists {
			influencer_metadata, _ := json.Marshal(map[string]any{
				"source": PlatformID,
			})
			reference_index = len(references)
			reference_index_by_name[name] = reference_index
			references = append(references, adapter.ContentInfluencerReference{
				Influencer: &model.Influencer{
					Name:               name,
					KnownForDepartment: role,
					MetadataJSON:       string(influencer_metadata),
				},
			})
		}
		role_metadata, _ := json.Marshal(map[string]any{
			"source":       PlatformID,
			"source_field": credit.SourceField,
			"raw":          credit.Raw,
			"character":    credit.Character,
		})
		references[reference_index].Roles = append(references[reference_index].Roles, adapter.ContentInfluencerRole{
			Role:         role,
			SortOrder:    credit.SortOrder,
			MetadataJSON: string(role_metadata),
		})
	}
	return references
}

func first_non_empty_bilibili_value(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func enrich_bangumi_content(content *model.Content, info *bilibili.BangumiInfo) {
	if content == nil || info == nil {
		return
	}
	target_episode := info.PageData.TargetEpisode()
	if target_episode == nil {
		return
	}
	content.Title = bangumi_title(info)
	content.CoverURL = bangumi_cover_url(info)
	content.CoverWidth = strconv.Itoa(target_episode.Dimension.Width)
	content.CoverHeight = strconv.Itoa(target_episode.Dimension.Height)
	content.ViewCount = target_episode.Stat.Play
	content.LikeCount = target_episode.Stat.Likes
	content.CommentCount = target_episode.Stat.Reply
	if target_episode.PublishTime > 0 {
		publish_time := target_episode.PublishTime * 1000
		content.PublishTime = &publish_time
	}
}

func bangumi_video_variants(info *bilibili.BangumiInfo, selected_stream *bilibili.BangumiDashStream) []model.ContentVideoVariant {
	if info == nil {
		return nil
	}
	streams := info.PlayURLSSRData.Data.Result.VideoInfo.Dash.Video
	variants := make([]model.ContentVideoVariant, 0, len(streams))
	default_key := bangumi_stream_key("video", selected_stream)
	content_id := bangumi_video_content_id(info)
	now := util.NowMillis()
	for stream_index := range streams {
		stream := &streams[stream_index]
		if strings.TrimSpace(stream.BaseURL) == "" {
			continue
		}
		variant_key := bangumi_stream_key("video", stream)
		metadata_data, _ := json.Marshal(map[string]any{
			"id":           stream.ID,
			"mime_type":    stream.MIMEType,
			"codecs":       stream.Codecs,
			"bandwidth":    stream.Bandwidth,
			"frame_rate":   stream.FrameRate,
			"backup_url":   stream.BackupURL,
			"segment_base": stream.SegmentBase,
			"size":         stream.Size,
		})
		variant := model.ContentVideoVariant{
			VideoId:    content_id,
			VariantKey: variant_key,
			Spec:       bangumi_stream_spec(stream),
			Quality:    strconv.Itoa(stream.ID),
			Size:       stream.Size,
			Codec:      stream.Codecs,
			Format:     bangumi_stream_format(stream),
			StreamType: model.ContentVideoVariantStreamTypeVideoOnly,
			HasVideo:   1,
			HasAudio:   0,
			IsDefault:  bool_int(variant_key == default_key),
			URL:        stream.BaseURL,
			Metadata:   string(metadata_data),
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		variant.Width = positive_int_pointer(stream.Width)
		variant.Height = positive_int_pointer(stream.Height)
		variant.FPS = positive_int_pointer(parse_bangumi_frame_rate(stream.FrameRate))
		variant.Bitrate = positive_int_pointer(int64_to_int(stream.Bandwidth))
		variants = append(variants, variant)
	}
	return variants
}

func best_bangumi_video_stream(streams []bilibili.BangumiDashStream) *bilibili.BangumiDashStream {
	best_index := -1
	for stream_index := range streams {
		stream := &streams[stream_index]
		if strings.TrimSpace(stream.BaseURL) == "" {
			continue
		}
		if best_index < 0 || bangumi_video_stream_score(stream) > bangumi_video_stream_score(&streams[best_index]) {
			best_index = stream_index
		}
	}
	if best_index < 0 {
		return nil
	}
	return &streams[best_index]
}

func select_bangumi_video_stream(streams []bilibili.BangumiDashStream, config map[string]any) (*bilibili.BangumiDashStream, error) {
	configured_variant_key := config_string(config, "video_variant_key")
	configured_variant_spec := config_string(config, "video_variant_spec")
	if configured_variant_key == "" && configured_variant_spec == "" {
		return best_bangumi_video_stream(streams), nil
	}
	for stream_index := range streams {
		stream := &streams[stream_index]
		if strings.TrimSpace(stream.BaseURL) == "" {
			continue
		}
		if (configured_variant_key != "" && bangumi_stream_key("video", stream) == configured_variant_key) ||
			(configured_variant_spec != "" && bangumi_stream_spec(stream) == configured_variant_spec) {
			return stream, nil
		}
	}
	return nil, fmt.Errorf("B站番剧不包含所选视频规格 %s", first_non_empty_bilibili_value(configured_variant_key, configured_variant_spec))
}

func best_bangumi_audio_stream(streams []bilibili.BangumiDashStream) *bilibili.BangumiDashStream {
	best_index := -1
	for stream_index := range streams {
		stream := &streams[stream_index]
		if strings.TrimSpace(stream.BaseURL) == "" {
			continue
		}
		if best_index < 0 || stream.Bandwidth > streams[best_index].Bandwidth ||
			(stream.Bandwidth == streams[best_index].Bandwidth && stream.Size > streams[best_index].Size) {
			best_index = stream_index
		}
	}
	if best_index < 0 {
		return nil
	}
	return &streams[best_index]
}

func bangumi_video_stream_score(stream *bilibili.BangumiDashStream) int64 {
	if stream == nil {
		return -1
	}
	resolution := int64(stream.Width) * int64(stream.Height)
	frame_rate := int64(parse_bangumi_frame_rate(stream.FrameRate))
	return resolution*1_000_000_000 + frame_rate*1_000_000 + stream.Bandwidth
}

func bangumi_stream_endpoints(stream *bilibili.BangumiDashStream, source_url string) []model.DownloadEndpoint {
	if stream == nil {
		return nil
	}
	endpoint_urls := make([]string, 0, len(stream.BackupURL)+1)
	endpoint_urls = append(endpoint_urls, stream.BaseURL)
	endpoint_urls = append(endpoint_urls, stream.BackupURL...)
	headers_data, _ := json.Marshal(map[string]string{
		"Referer":    source_url,
		"User-Agent": bilibili_download_user_agent,
	})
	seen_urls := make(map[string]struct{}, len(endpoint_urls))
	endpoints := make([]model.DownloadEndpoint, 0, len(endpoint_urls))
	for endpoint_index, endpoint_url := range endpoint_urls {
		endpoint_url = strings.TrimSpace(endpoint_url)
		if endpoint_url == "" {
			continue
		}
		if _, exists := seen_urls[endpoint_url]; exists {
			continue
		}
		seen_urls[endpoint_url] = struct{}{}
		endpoints = append(endpoints, model.DownloadEndpoint{
			Protocol: bilibili_endpoint_protocol(endpoint_url),
			URL:      endpoint_url,
			Priority: endpoint_index,
			Enabled:  1,
			Headers:  string(headers_data),
		})
	}
	return endpoints
}

func build_bilibili_account(external_id string, now int64) *model.Account {
	return &model.Account{
		Id:         BuildAccountID(external_id),
		PlatformId: platform_id_bilibili,
		ExternalId: external_id,
		Nickname:   "B站用户",
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func build_bangumi_account(now int64) *model.Account {
	return &model.Account{
		Id:         BuildAccountID(bilibili_bangumi_account_external_id),
		PlatformId: platform_id_bilibili,
		ExternalId: bilibili_bangumi_account_external_id,
		Nickname:   bilibili_bangumi_account_nickname,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func build_extra_json(external_id string, title string, source_url string, stream *bilibili.BangumiDashStream) string {
	extra := map[string]any{
		"id":         external_id,
		"title":      title,
		"source_url": source_url,
	}
	if stream != nil {
		extra["stream_id"] = stream.ID
		extra["mime_type"] = stream.MIMEType
		extra["codecs"] = stream.Codecs
		extra["bandwidth"] = stream.Bandwidth
		extra["width"] = stream.Width
		extra["height"] = stream.Height
		extra["frame_rate"] = stream.FrameRate
		extra["segment_base"] = stream.SegmentBase
	}
	data, _ := json.Marshal(extra)
	return string(data)
}

func copy_config(config map[string]any) map[string]any {
	result := make(map[string]any, len(config))
	for key, value := range config {
		result[key] = value
	}
	return result
}

func bilibili_task_name(config map[string]any, title string, fallback string) string {
	if configured_name := config_string(config, "filename"); configured_name != "" {
		return configured_name
	}
	if title = strings.TrimSpace(title); title != "" {
		return title
	}
	return fallback
}

func bangumi_info_from_playurl(playurl_ssr_data *bilibili.PlayURLSSRData) *bilibili.BangumiInfo {
	if playurl_ssr_data == nil {
		return nil
	}
	result := playurl_ssr_data.Data.Result
	episode_info := result.PlayViewBusinessInfo.EpisodeInfo
	episode_id := episode_info.EpisodeID
	aid := result.Arc.AID
	cid := result.Arc.CID
	if aid == 0 {
		aid = episode_info.AID
	}
	if cid == 0 {
		cid = episode_info.CID
	}
	external_id := strings.TrimSpace(result.Arc.BVID)
	source_url := ""
	title := external_id
	if episode_id > 0 {
		external_id = strconv.FormatInt(episode_id, 10)
		source_url = "https://www.bilibili.com/bangumi/play/ep" + external_id
		title = "ep" + external_id
	} else if external_id == "" && aid > 0 {
		external_id = strconv.FormatInt(aid, 10)
		title = "av" + external_id
	}
	return &bilibili.BangumiInfo{
		SourceURL:      source_url,
		Title:          title,
		EpisodeID:      episode_id,
		SeasonID:       result.PlayViewBusinessInfo.SeasonInfo.SeasonID,
		SeasonType:     result.PlayViewBusinessInfo.SeasonInfo.SeasonType,
		AID:            aid,
		CID:            cid,
		BVID:           strings.TrimSpace(result.Arc.BVID),
		PlayURLSSRData: *playurl_ssr_data,
	}
}

func bangumi_video_external_id(info *bilibili.BangumiInfo) string {
	if info == nil {
		return ""
	}
	if bvid := strings.TrimSpace(info.BVID); bvid != "" {
		return bvid
	}
	if info.AID > 0 {
		return "av" + strconv.FormatInt(info.AID, 10)
	}
	if info.CID > 0 {
		return "cid" + strconv.FormatInt(info.CID, 10)
	}
	if info.EpisodeID > 0 {
		return "ep" + strconv.FormatInt(info.EpisodeID, 10) + ":video"
	}
	return ""
}

func bangumi_video_content_id(info *bilibili.BangumiInfo) string {
	video_external_id := bangumi_video_external_id(info)
	if video_external_id == "" {
		return ""
	}
	return BuildContentID(video_external_id)
}

func bangumi_episode_external_id(info *bilibili.BangumiInfo) string {
	if info == nil {
		return ""
	}
	episode_id := info.EpisodeID
	if target_episode := info.PageData.TargetEpisode(); target_episode != nil && target_episode.EpisodeID > 0 {
		episode_id = target_episode.EpisodeID
	}
	if episode_id <= 0 {
		return ""
	}
	return "ep" + strconv.FormatInt(episode_id, 10)
}

func bangumi_season_external_id(info *bilibili.BangumiInfo) string {
	if info == nil || info.SeasonID <= 0 {
		return ""
	}
	return "ss" + strconv.FormatInt(info.SeasonID, 10)
}

func bangumi_external_id(info *bilibili.BangumiInfo) string {
	if info == nil {
		return ""
	}
	if target_episode := info.PageData.TargetEpisode(); target_episode != nil && target_episode.EpisodeID > 0 {
		return strconv.FormatInt(target_episode.EpisodeID, 10)
	}
	if info.EpisodeID > 0 {
		return strconv.FormatInt(info.EpisodeID, 10)
	}
	if strings.TrimSpace(info.BVID) != "" {
		return strings.TrimSpace(info.BVID)
	}
	if info.AID > 0 {
		return strconv.FormatInt(info.AID, 10)
	}
	return ""
}

func bangumi_title(info *bilibili.BangumiInfo) string {
	if info == nil {
		return ""
	}
	if target_episode := info.PageData.TargetEpisode(); target_episode != nil {
		for _, title := range []string{target_episode.ShareCopy, target_episode.ShowTitle, target_episode.LongTitle, target_episode.Title} {
			if title = strings.TrimSpace(title); title != "" {
				return title
			}
		}
	}
	return strings.TrimSpace(info.Title)
}

func bangumi_season_title(info *bilibili.BangumiInfo) string {
	if info == nil {
		return ""
	}
	if season_title := strings.TrimSpace(info.SeasonTitle); season_title != "" {
		return season_title
	}
	episode_title := bangumi_title(info)
	if title_start := strings.Index(episode_title, "《"); title_start >= 0 {
		if title_end := strings.Index(episode_title[title_start+len("《"):], "》"); title_end >= 0 {
			return strings.TrimSpace(episode_title[title_start+len("《") : title_start+len("《")+title_end])
		}
	}
	return episode_title
}

func bangumi_cover_url(info *bilibili.BangumiInfo) string {
	if info == nil {
		return ""
	}
	if target_episode := info.PageData.TargetEpisode(); target_episode != nil {
		if cover_url := normalize_bilibili_asset_url(target_episode.Cover); cover_url != "" {
			return cover_url
		}
	}
	return normalize_bilibili_asset_url(info.CoverURL)
}

func bangumi_season_cover_url(info *bilibili.BangumiInfo) string {
	if info == nil {
		return ""
	}
	if cover_url := normalize_bilibili_asset_url(info.SeasonCoverURL); cover_url != "" {
		return cover_url
	}
	return bangumi_cover_url(info)
}

func bangumi_source_url(info *bilibili.BangumiInfo) string {
	if info == nil {
		return ""
	}
	if source_url := strings.TrimSpace(info.SourceURL); source_url != "" {
		return source_url
	}
	if target_episode := info.PageData.TargetEpisode(); target_episode != nil {
		if source_url := strings.TrimSpace(target_episode.ShareURL); source_url != "" {
			return source_url
		}
		if source_url := strings.TrimSpace(target_episode.Link); source_url != "" {
			return source_url
		}
	}
	if external_id := bangumi_external_id(info); external_id != "" {
		return "https://www.bilibili.com/bangumi/play/ep" + external_id
	}
	return ""
}

func bangumi_season_source_url(info *bilibili.BangumiInfo) string {
	season_external_id := bangumi_season_external_id(info)
	if season_external_id == "" {
		return ""
	}
	return "https://www.bilibili.com/bangumi/play/" + season_external_id
}

func bangumi_target_section_id(info *bilibili.BangumiInfo) int64 {
	if info == nil {
		return 0
	}
	target_episode_id := info.EpisodeID
	if target_episode := info.PageData.TargetEpisode(); target_episode != nil {
		target_episode_id = target_episode.EpisodeID
	}
	for section_index := range info.PageData.Result.Sections {
		section := &info.PageData.Result.Sections[section_index]
		for page_index := range section.Pages {
			for episode_index := range section.Pages[page_index].Episodes {
				if section.Pages[page_index].Episodes[episode_index].EpisodeID == target_episode_id {
					return section.SectionID
				}
			}
		}
	}
	return info.PageData.Result.Locator.SectionID
}

func bangumi_episode_count(info *bilibili.BangumiInfo) int {
	if info == nil {
		return 0
	}
	if info.SeasonData.Result.Total > 0 {
		return info.SeasonData.Result.Total
	}
	episode_ids := make(map[int64]struct{})
	for section_index := range info.PageData.Result.Sections {
		section := &info.PageData.Result.Sections[section_index]
		for page_index := range section.Pages {
			for episode_index := range section.Pages[page_index].Episodes {
				episode_id := section.Pages[page_index].Episodes[episode_index].EpisodeID
				if episode_id > 0 {
					episode_ids[episode_id] = struct{}{}
				}
			}
		}
	}
	for section_index := range info.PageData.Result.SectionsMeta {
		section_meta := &info.PageData.Result.SectionsMeta[section_index]
		if section_meta.EpisodeID > 0 {
			episode_ids[section_meta.EpisodeID] = struct{}{}
		}
		for _, episode_id := range section_meta.EpisodeIDs {
			if episode_id > 0 {
				episode_ids[episode_id] = struct{}{}
			}
		}
	}
	return len(episode_ids)
}

func bangumi_area_names(areas []bilibili.PGCArea) []string {
	area_names := make([]string, 0, len(areas))
	for area_index := range areas {
		if area_name := strings.TrimSpace(areas[area_index].Name); area_name != "" {
			area_names = append(area_names, area_name)
		}
	}
	return area_names
}

func bangumi_original_language(areas []bilibili.PGCArea) string {
	for area_index := range areas {
		area_name := strings.TrimSpace(areas[area_index].Name)
		if strings.Contains(area_name, "中国") {
			return "zh"
		}
		if strings.Contains(area_name, "日本") {
			return "ja"
		}
		if strings.Contains(area_name, "美国") || strings.Contains(area_name, "英国") {
			return "en"
		}
		if strings.Contains(area_name, "韩国") {
			return "ko"
		}
	}
	return ""
}

func bangumi_series_status(publish bilibili.PGCPublish) string {
	if publish.IsFinish > 0 {
		return "ended"
	}
	if publish.IsStarted > 0 {
		return "returning_series"
	}
	return "planned"
}

func bangumi_series_air_date(info *bilibili.BangumiInfo) string {
	if info == nil {
		return ""
	}
	publish_time := strings.TrimSpace(info.SeasonData.Result.Publish.PublishTime)
	if len(publish_time) >= len("2006-01-02") {
		return publish_time[:len("2006-01-02")]
	}
	return publish_time
}

func bangumi_series_publish_time(info *bilibili.BangumiInfo) *int64 {
	if info == nil {
		return nil
	}
	publish_time := strings.TrimSpace(info.SeasonData.Result.Publish.PublishTime)
	if publish_time == "" {
		return nil
	}
	parsed_time, err := time.ParseInLocation("2006-01-02 15:04:05", publish_time, time.Local)
	if err != nil {
		return nil
	}
	publish_millis := parsed_time.UnixMilli()
	return &publish_millis
}

func bangumi_episode_air_date(publish_time int64, release_date string) string {
	if release_date = strings.TrimSpace(release_date); release_date != "" {
		return release_date
	}
	if publish_time <= 0 {
		return ""
	}
	return time.Unix(publish_time, 0).Format("2006-01-02")
}

func bangumi_episode_sort_order(info *bilibili.BangumiInfo) int {
	if info == nil {
		return 0
	}
	return info.PageData.Result.Locator.IndexInPage
}

func bangumi_stream_id(stream *bilibili.BangumiDashStream) int {
	if stream == nil {
		return 0
	}
	return stream.ID
}

func bangumi_stream_key(stream_kind string, stream *bilibili.BangumiDashStream) string {
	if stream == nil {
		return ""
	}
	codec_key := strings.Map(func(char rune) rune {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			return unicode.ToLower(char)
		}
		return '_'
	}, stream.Codecs)
	codec_key = strings.Trim(codec_key, "_")
	if codec_key == "" {
		codec_key = "unknown"
	}
	return fmt.Sprintf("%s_%d_%s", stream_kind, stream.ID, codec_key)
}

func bangumi_stream_spec(stream *bilibili.BangumiDashStream) string {
	if stream == nil {
		return ""
	}
	resolution := ""
	if stream.Width > 0 && stream.Height > 0 {
		resolution = fmt.Sprintf("%dx%d", stream.Width, stream.Height)
	}
	if stream.FrameRate != "" {
		if resolution != "" {
			return resolution + "@" + stream.FrameRate
		}
		return stream.FrameRate + "fps"
	}
	return resolution
}

func bangumi_stream_mime_type(stream_kind string, stream *bilibili.BangumiDashStream) string {
	if stream != nil && strings.TrimSpace(stream.MIMEType) != "" {
		return strings.TrimSpace(stream.MIMEType)
	}
	return stream_kind + "/mp4"
}

func bangumi_stream_format(stream *bilibili.BangumiDashStream) string {
	if stream == nil {
		return ""
	}
	mime_type := strings.TrimSpace(stream.MIMEType)
	if _, format, found := strings.Cut(mime_type, "/"); found {
		if parameter_index := strings.IndexByte(format, ';'); parameter_index >= 0 {
			format = format[:parameter_index]
		}
		return strings.TrimSpace(format)
	}
	return "mp4"
}

func parse_bangumi_frame_rate(frame_rate string) int {
	frame_rate = strings.TrimSpace(frame_rate)
	if frame_rate == "" {
		return 0
	}
	if numerator_text, denominator_text, found := strings.Cut(frame_rate, "/"); found {
		numerator, numerator_err := strconv.ParseFloat(strings.TrimSpace(numerator_text), 64)
		denominator, denominator_err := strconv.ParseFloat(strings.TrimSpace(denominator_text), 64)
		if numerator_err == nil && denominator_err == nil && denominator > 0 {
			return int(math.Round(numerator / denominator))
		}
	}
	value, err := strconv.ParseFloat(frame_rate, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return int(math.Round(value))
}

func int64_to_int(value int64) int {
	if value <= 0 {
		return 0
	}
	max_int := int64(^uint(0) >> 1)
	if value > max_int {
		return int(max_int)
	}
	return int(value)
}

func positive_int_pointer(value int) *int {
	if value <= 0 {
		return nil
	}
	result := value
	return &result
}

func bool_int(value bool) int {
	if value {
		return 1
	}
	return 0
}

func config_string(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return strings.TrimSpace(value)
}

func config_bool(config map[string]any, key string) bool {
	value, _ := config[key].(bool)
	return value
}

func normalize_bilibili_asset_url(raw_url string) string {
	raw_url = strings.TrimSpace(raw_url)
	if strings.HasPrefix(raw_url, "//") {
		return "https:" + raw_url
	}
	if strings.HasPrefix(raw_url, "http://") {
		return "https://" + strings.TrimPrefix(raw_url, "http://")
	}
	return raw_url
}

func bilibili_endpoint_protocol(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
}
