package wxchannelsadapter

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/util"
)

const platform_id_wx_channels = "wxchannels"

// PlatformID is the platform identifier for wechat channels.
const PlatformID = platform_id_wx_channels

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(external_id string) string {
	return PlatformID + ":" + external_id
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(external_id string) string {
	return PlatformID + ":" + external_id
}

type metadata_kv struct {
	Key string `json:"key"`
}

// clean_media_url removes CDN routing parameters (hy, idx, m, uzid) from the media URL.
func clean_media_url(raw_url string) string {
	u, err := url.Parse(raw_url)
	if err != nil || u == nil {
		return raw_url
	}
	q := u.Query()
	q.Del("hy")
	q.Del("idx")
	q.Del("m")
	q.Del("uzid")
	u.RawQuery = q.Encode()
	return u.String()
}

func first_media_cover_url(file wxchannels.ChannelsMediaItem) string {
	cover_url := strings.TrimSpace(file.CoverUrl)
	if cover_url != "" {
		return cover_url
	}
	media_url := strings.TrimSpace(file.URL + file.URLToken)
	if media_url != "" {
		return media_url
	}
	return ""
}

func positive_dimension_pointer(value float32) *int {
	dimension := int(value)
	if dimension <= 0 {
		return nil
	}
	return &dimension
}

func to_content_video_variants(obj *wxchannels.ChannelsObject, video_id string) []model.ContentVideoVariant {
	if obj == nil || len(obj.ObjectDesc.Media) == 0 {
		return nil
	}

	media := obj.ObjectDesc.Media[0]
	specs := obj.Spec
	if len(media.Spec) > 0 {
		specs = media.Spec
	}

	variants := make([]model.ContentVideoVariant, 0, len(specs)+1)
	// Temporarily disabled: do not expose the synthetic original-video option.
	// variants = append(variants, model.ContentVideoVariant{
	// 	VideoId:    video_id,
	// 	VariantKey: "default",
	// 	Spec:       "original",
	// 	Width:      positive_dimension_pointer(media.Width),
	// 	Height:     positive_dimension_pointer(media.Height),
	// 	Size:       int64(media.FileSize),
	// 	StreamType: model.ContentVideoVariantStreamTypeProgressive,
	// 	HasVideo:   1,
	// 	HasAudio:   1,
	// 	IsDefault:  1,
	// 	URL:        BuildDownloadURLWithSpec(obj, ""),
	// })

	seen_variant_keys := make(map[string]struct{}, len(specs)+1)
	// seen_variant_keys["default"] = struct{}{}
	for _, spec := range specs {
		variant_key := strings.TrimSpace(spec.FileFormat)
		if variant_key == "" {
			continue
		}
		if _, ok := seen_variant_keys[variant_key]; ok {
			continue
		}
		seen_variant_keys[variant_key] = struct{}{}

		variant := model.ContentVideoVariant{
			VideoId:    video_id,
			VariantKey: variant_key,
			Spec:       variant_key,
			Width:      positive_dimension_pointer(spec.Width),
			Height:     positive_dimension_pointer(spec.Height),
			StreamType: model.ContentVideoVariantStreamTypeProgressive,
			HasVideo:   1,
			HasAudio:   1,
			URL:        BuildDownloadURLWithSpec(obj, variant_key),
		}
		variants = append(variants, variant)
	}

	return variants
}

func select_content_video_variant(detail any, spec string) {
	video, ok := detail.(*model.ContentVideo)
	if !ok || video == nil {
		return
	}

	selected_key := strings.TrimSpace(spec)
	if selected_key == "" || selected_key == "original" || selected_key == "default" {
		selected_key = "default"
	}
	for index := range video.Variants {
		variant := &video.Variants[index]
		variant.IsDefault = 0
		if variant.VariantKey == selected_key || (selected_key != "default" && variant.Spec == selected_key) {
			variant.IsDefault = 1
		}
	}
}

func selected_content_video_size(detail any) int64 {
	video, ok := detail.(*model.ContentVideo)
	if !ok || video == nil {
		return 0
	}
	for variant_index := range video.Variants {
		variant := &video.Variants[variant_index]
		if variant.IsDefault == 0 {
			continue
		}
		resource_size := variant.Size
		if resource_size <= 0 && variant.VariantKey == "default" {
			resource_size = video.Size
		}
		variant.Size = resource_size
		video.Size = resource_size
		return resource_size
	}
	return 0
}

func resolve_video_download_spec(obj *wxchannels.ChannelsObject, config map[string]any, default_highest bool) string {
	configured_variant_key := config_string(config, "video_variant_key")
	configured_spec := config_string(config, "video_variant_spec")
	if configured_spec == "" {
		configured_spec = config_string(config, "spec")
	}
	_, explicit_spec := config["spec"]
	if configured_variant_key == "default" {
		configured_spec = "original"
	} else if configured_spec == "" && configured_variant_key != "" {
		configured_spec = configured_variant_key
	}

	if configured_spec == "" {
		// The injected downloader historically uses spec: "" to request the
		// original resource. Preserve the distinction between that explicit
		// value and an omitted spec, which selects the normal default variant.
		if explicit_spec {
			return ""
		}
		if default_highest {
			return ""
		}
		return PickSpec(obj)
	}
	if configured_spec == "original" {
		return ""
	}
	return configured_spec
}

func (a *ChannelsAdapter) video_download_spec(obj *wxchannels.ChannelsObject, config map[string]any) string {
	default_highest := a.config_bool("channels.download.defaultHighest") || a.config_bool("download.defaultHighest")
	return resolve_video_download_spec(obj, config, default_highest)
}

// ToAccount converts a ChannelsObject into a model.Account.
func ToAccount(obj *wxchannels.ChannelsObject) (*model.Account, error) {
	if obj == nil {
		return nil, errors.New("channels object is nil")
	}

	contact, account_username := pick_account_contact(obj)

	now := util.NowMillis()
	acc := &model.Account{
		Id:         BuildAccountID(account_username),
		PlatformId: PlatformID,
		ExternalId: account_username,
		Nickname:   contact.Nickname,
		Signature:  strings.TrimSpace(contact.Signature),
		AvatarURL:  contact.HeadUrl,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	return acc, nil
}

// pick_account_contact selects the appropriate contact and external ID for an account.
// For live objects, prefers AnchorContact over Contact.
func pick_account_contact(obj *wxchannels.ChannelsObject) (wxchannels.ChannelsContact, string) {
	if obj.LiveInfo != nil && obj.AnchorContact != nil {
		return *obj.AnchorContact, obj.AnchorContact.Username
	}
	return obj.Contact, obj.Contact.Username
}

// ToContent converts a ChannelsObject into a slim model.Content and an extension struct.
// Returns (content, extension, error). extension is nil for live content, *ContentVideo for video,
// []*ContentImage for single picture, *ContentAlbum for album.
func ToContent(obj *wxchannels.ChannelsObject) (*model.Content, any, error) {
	if obj == nil {
		return nil, nil, errors.New("channels object is nil")
	}
	if obj.ID == "" {
		return nil, nil, errors.New("missing object id field")
	}

	now := util.NowMillis()
	c := &model.Content{
		Id:          BuildContentID(obj.ID),
		PlatformId:  PlatformID,
		ExternalId:  obj.ID,
		ExternalId2: obj.ObjectNonceId,
		SourceURL:   obj.SourceURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// Live
	if obj.LiveInfo != nil {
		c.Type = "live"
		c.Title = "直播"
		if obj.AnchorContact != nil {
			c.CoverURL = obj.AnchorContact.CoverImgUrl
		}
		if c.CoverURL == "" && len(obj.ObjectDesc.Media) > 0 && obj.ObjectDesc.Media[0].CoverUrl != "" {
			c.CoverURL = obj.ObjectDesc.Media[0].CoverUrl
		}
		if obj.CreateTime > 0 {
			publish_time := int64(obj.CreateTime)
			c.PublishTime = &publish_time
		}
		return c, nil, nil
	}

	// Picture
	if obj.ObjectDesc.MediaType == wxchannels.MediaTypePicture {
		files := obj.Files
		if len(files) == 0 {
			files = obj.ObjectDesc.Media
		}
		if len(files) == 0 {
			return nil, nil, errors.New("picture object missing files data")
		}
		c.Type = "album"
		c.Title = obj.ObjectDesc.Description
		c.Description = obj.ObjectDesc.Description
		c.CoverURL = first_media_cover_url(files[0])
		c.CoverWidth = strconv.Itoa(int(files[0].Width))
		c.CoverHeight = strconv.Itoa(int(files[0].Height))
		if obj.CreateTime > 0 {
			publish_time := int64(obj.CreateTime)
			c.PublishTime = &publish_time
		}

		md, _ := json.Marshal(metadata_kv{Key: files[0].DecodeKey})
		c.Metadata = string(md)
		images := make([]model.ContentImage, 0, len(files))
		for i, file := range files {
			images = append(images, model.ContentImage{
				AlbumId:   c.Id,
				ImageKey:  model.BuildContentAlbumImageKey(file.DecodeKey, file.URL+file.URLToken, i),
				SortOrder: i,
				URL:       file.URL + file.URLToken,
				Width:     int(file.Width),
				Height:    int(file.Height),
				Size:      int64(file.FileSize),
				ImageType: model.ContentImageTypeStill,
			})
		}
		album := &model.ContentAlbum{
			Id:          c.Id,
			ImageCount:  len(images),
			Description: obj.ObjectDesc.Description,
			Images:      images,
		}
		if len(images) > 0 {
			album.CoverWidth = int(files[0].Width)
			album.CoverHeight = int(files[0].Height)
		}
		return c, album, nil
	}

	// Media (video)
	if obj.ObjectDesc.MediaType == wxchannels.MediaTypeLive {
		return nil, nil, errors.New("live replay is not supported (mediaType=9)")
	}

	if len(obj.ObjectDesc.Media) == 0 {
		return nil, nil, errors.New("objectDesc.media is empty")
	}
	media := obj.ObjectDesc.Media[0]

	c.Type = "video"
	c.Title = obj.ObjectDesc.Description
	c.Description = obj.ObjectDesc.Description
	c.URL = clean_media_url(media.URL) + media.URLToken
	c.CoverURL = media.ThumbUrl
	c.CoverWidth = strconv.Itoa(int(media.Width))
	c.CoverHeight = strconv.Itoa(int(media.Height))
	if c.SourceURL == "" {
		_, contact_username := pick_account_contact(obj)
		c.SourceURL = BuildJumpURLFromParts(obj.ID, obj.ObjectNonceId, "", contact_username)
	}

	if obj.CreateTime > 0 {
		publish_time := int64(obj.CreateTime)
		c.PublishTime = &publish_time
	}

	md, _ := json.Marshal(metadata_kv{Key: media.DecodeKey})
	c.Metadata = string(md)
	ext := &model.ContentVideo{
		Id:       c.Id,
		Duration: int64(media.VideoPlayLen),
		Width:    int(media.Width),
		Height:   int(media.Height),
		Size:     int64(media.FileSize),
		URL:      c.URL,
		Variants: to_content_video_variants(obj, c.Id),
	}

	return c, ext, nil
}

// BuildBrowseHistory converts an intercepted ChannelsObject into the standard
// browse history result.
func (a *ChannelsAdapter) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(content_json, &obj); err != nil {
		return nil, fmt.Errorf("解析视频号内容失败: %w", err)
	}

	account_username := strings.TrimSpace(obj.Contact.Username)
	now := util.NowMillis()

	var key string
	if len(obj.ObjectDesc.Media) > 0 {
		key = obj.ObjectDesc.Media[0].DecodeKey
	}

	extra_data, _ := json.Marshal(map[string]any{
		"id":         obj.ID,
		"nonce_id":   obj.ObjectNonceId,
		"decode_key": key,
	})

	browse_id := PlatformID + ":" + obj.ID
	content_source_url := obj.SourceURL
	if content_source_url == "" {
		content_source_url = BuildJumpURLFromParts(obj.ID, obj.ObjectNonceId, "", account_username)
	}

	cover_url := ""
	cover_width := ""
	cover_height := ""
	media_list := obj.Files
	if len(media_list) == 0 {
		media_list = obj.ObjectDesc.Media
	}
	if len(media_list) > 0 {
		media := media_list[0]
		if obj.ObjectDesc.MediaType == wxchannels.MediaTypePicture {
			cover_url = first_media_cover_url(media)
		} else {
			cover_url = strings.TrimSpace(media.ThumbUrl)
		}
		cover_width = strconv.Itoa(int(media.Width))
		cover_height = strconv.Itoa(int(media.Height))
	}
	publish_time := int64(obj.CreateTime)

	content_type := "video"
	if obj.ObjectDesc.MediaType == wxchannels.MediaTypePicture {
		content_type = "album"
	}

	account, err := ToAccount(&obj)
	if err != nil {
		return nil, err
	}

	return &adapter.BrowseHistoryResult{
		BrowseHistory: &model.BrowseHistory{
			Id:           browse_id,
			PlatformId:   PlatformID,
			VisitedTimes: 1,
			Type:         content_type,
			ExternalId:   obj.ID,
			Title:        obj.ObjectDesc.Description,
			URL:          ObjectURL(&obj),
			SourceURL:    content_source_url,
			CoverURL:     cover_url,
			CoverWidth:   cover_width,
			CoverHeight:  cover_height,
			PublishTime:  &publish_time,
			ExtraData:    string(extra_data),
			Timestamps: model.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Account: account,
	}, nil
}

// ObjectURL returns the download URL (video = media.URL + URLToken, picture/live returns "").
func ObjectURL(obj *wxchannels.ChannelsObject) string {
	if obj.LiveInfo != nil {
		return ""
	}
	if obj.Type == "picture" || obj.ObjectDesc.MediaType == wxchannels.MediaTypePicture {
		return ""
	}
	if len(obj.ObjectDesc.Media) == 0 {
		return ""
	}
	return clean_media_url(obj.ObjectDesc.Media[0].URL) + obj.ObjectDesc.Media[0].URLToken
}

// BuildJumpURLFromParts builds a channels.weixin.qq.com feed page URL from individual fields.
func BuildJumpURLFromParts(object_id, nonce_id, source_url, username string) string {
	origin := "https://channels.weixin.qq.com"
	if source_url != "" {
		return source_url
	}

	oid := object_id
	nid := nonce_id
	u := origin + "/web/pages/feed"
	if username != "" {
		u += "?username=" + url.QueryEscape(username)
	} else {
		u += "?"
	}

	if oid != "" {
		encoded_oid := util.EncodeUint64ToBase64(oid)
		if encoded_oid != "" {
			u += "&oid=" + url.QueryEscape(encoded_oid)
		}
	}

	if nid != "" {
		// NonceId may contain underscore-separated segments (e.g. "123_0_146_0_0").
		// The first segment is the numeric ID used for encoding.
		if idx := strings.IndexByte(nid, '_'); idx >= 0 {
			nid = nid[:idx]
		}
		encoded_nid := util.EncodeUint64ToBase64(nid)
		if encoded_nid != "" {
			u += "&nid=" + url.QueryEscape(encoded_nid)
		}
	}

	return strings.TrimSuffix(strings.Replace(u, "?&", "?", 1), "?")
}

const (
	mime_image_jpeg      = "image/jpeg"
	mime_audio_mpeg      = "audio/mpeg"
	mime_video_mp4       = "video/mp4"
	mime_video_matroska  = "video/x-matroska"
	mime_application_zip = "application/zip"
)

func (a *ChannelsAdapter) BuildDownloadTask(content_json json.RawMessage, config_raw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(config_raw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	// Live stream detection: joinLive response contains liveSdkInfo
	var jl wxchannels.JoinLivePayload
	if json.Unmarshal(content_json, &jl) == nil && jl.LiveSdkInfo != nil && jl.LiveSdkInfo.LiveCdnUrl != "" {
		return a.build_live_download_task(&jl, config)
	}

	obj, err := parse_channels_object_for_download(content_json)
	if err != nil {
		return nil, err
	}

	content, ext, err := ToContent(&obj)
	if err != nil {
		return nil, err
	}
	account, err := ToAccount(&obj)
	if err != nil {
		return nil, err
	}

	title := config_string(config, "filename")
	if title == "" {
		title = ObjectTitle(&obj)
	}
	spec := a.video_download_spec(&obj, config)
	select_content_video_variant(ext, spec)
	cover_url := strings.TrimSpace(content.CoverURL)
	if len(obj.ObjectDesc.Media) > 0 {
		if candidate := strings.TrimSpace(obj.ObjectDesc.Media[0].CoverUrl); candidate != "" {
			cover_url = candidate
		} else if candidate := strings.TrimSpace(obj.ObjectDesc.Media[0].ThumbUrl); candidate != "" {
			cover_url = candidate
		}
	}
	contact, _ := pick_account_contact(&obj)
	decrypt_key := parse_key_from_content(content)
	base_extra_json := build_resource_extra_json(obj.ID, title, spec, int64(obj.CreateTime), contact.Nickname, "", 0, obj.ObjectDesc.MediaType)
	decrypt_extra_json := build_resource_extra_json(obj.ID, title, spec, int64(obj.CreateTime), contact.Nickname, decrypt_key, 0, obj.ObjectDesc.MediaType)
	content_id := content.Id

	is_download_feed_cover := config_suffix_is_cover(config)
	// Cover download: create cover resource only
	if is_download_feed_cover && cover_url != "" {
		cover_config := build_config_json(config, spec, obj.ObjectDesc.MediaType)
		task_unique_id := BuildDownloadTaskUniqueID(content.ExternalId, cover_config)
		config_json, _ := json.Marshal(cover_config)
		info := &adapter.DownloadTaskResult{
			Task: &model.DownloadTask{
				ContentId:  &content_id,
				Name:       title,
				UniqueID:   task_unique_id,
				PlatformId: PlatformID,
				Status:     model.TaskStatusWaiting,
				SourceURL:  content.SourceURL,
				CoverURL:   content.CoverURL,
				ConfigJSON: string(config_json),
			},
			Resources:      []*adapter.ResourceInfo{build_cover_resource_info(content_id, task_unique_id, title, cover_url, base_extra_json)},
			Account:        account,
			Content:        content,
			ContentDetail:  ext,
			ContentDetails: channels_content_details(content, ext),
		}
		a.preview_download_resource_names(info, cover_config)
		return info, nil
	}

	// Picture type: create a download resource for each media item, plus background music
	if obj.ObjectDesc.MediaType == wxchannels.MediaTypePicture {
		files := obj.Files
		if len(files) == 0 {
			files = obj.ObjectDesc.Media
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("图片类型缺少文件数据")
		}

		picture_config := build_config_json(config, spec, wxchannels.MediaTypePicture)
		task_unique_id := BuildDownloadTaskUniqueID(content.ExternalId, picture_config)
		resources := make([]*adapter.ResourceInfo, 0, len(files)+2)
		for i, file := range files {
			media_url := get_media_url(file)
			if media_url == "" {
				return nil, fmt.Errorf("图片 %d 下载地址为空", i+1)
			}
			image_name := title
			if len(files) > 1 {
				image_name = fmt.Sprintf("%s_%d", title, i+1)
			}
			image_extra_json := build_resource_extra_json(obj.ID, title, spec, int64(obj.CreateTime), contact.Nickname, decrypt_key, i+1, obj.ObjectDesc.MediaType)
			image_key := model.BuildContentAlbumImageKey(file.DecodeKey, media_url, i)
			resources = append(resources, &adapter.ResourceInfo{
				Resource: model.DownloadResource{
					ContentId: &content_id,
					Name:      sanitize_filename(image_name),
					Kind:      mime_image_jpeg,
					Size:      int64(file.FileSize),
					UniqueID:  content.ExternalId + "_" + strconv.Itoa(i),
					Extra:     image_extra_json,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "https",
					URL:      media_url,
					Enabled:  1,
				}},
				ContentAssets: []adapter.ContentAssetReference{{
					Kind:            model.ContentAssetKindImage,
					Role:            model.ContentAssetRolePrimary,
					AssetKey:        model.BuildContentAlbumImageAssetKey(image_key, "jpeg"),
					Relation:        model.DownloadResourceAssetRelationSource,
					SubjectType:     model.ContentAssetSubjectAlbumImage,
					SubjectKey:      image_key,
					SubjectRelation: model.ContentAssetSubjectRelationRepresentation,
				}},
			})
		}

		// Background music
		bgm := format_bgm(&obj)
		if bgm != nil {
			resources = append(resources, &adapter.ResourceInfo{
				Resource: model.DownloadResource{
					ContentId: &content_id,
					Name:      bgm.name,
					Kind:      mime_audio_mpeg,
					UniqueID:  content.ExternalId + "_bgm",
					Extra:     base_extra_json,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: endpoint_protocol(bgm.url),
					URL:      bgm.url,
					Enabled:  1,
				}},
			})
		}
		if a.config_bool("channels.download.cover") && !is_download_feed_cover && cover_url != "" {
			resources = append(resources, build_cover_resource_info(content_id, build_sidecar_cover_unique_id(task_unique_id), title, cover_url, base_extra_json))
		}

		config_json, _ := json.Marshal(picture_config)

		info := &adapter.DownloadTaskResult{
			Task: &model.DownloadTask{
				ContentId:  &content_id,
				Name:       title,
				UniqueID:   task_unique_id,
				PlatformId: PlatformID,
				Status:     model.TaskStatusWaiting,
				SourceURL:  content.SourceURL,
				CoverURL:   content.CoverURL,
				ConfigJSON: string(config_json),
			},
			Resources:      resources,
			ContentDetail:  ext,
			ContentDetails: channels_content_details(content, ext),
			Account:        account,
			Content:        content,
		}
		a.preview_download_resource_names(info, picture_config)
		return info, nil
	}

	// Video type
	download_url := BuildDownloadURLWithSpec(&obj, spec)
	if download_url == "" {
		return nil, fmt.Errorf("无法获取视频下载地址")
	}

	video_config := build_config_json(config, spec, obj.ObjectDesc.MediaType)
	task_unique_id := BuildDownloadTaskUniqueID(content.ExternalId, video_config)
	config_json, _ := json.Marshal(video_config)

	resource_unique_id := task_unique_id
	resource_kind := mime_video_mp4
	if config_suffix_is_mp3(config) {
		resource_kind = mime_audio_mpeg
	}
	video_resource := model.DownloadResource{
		ContentId: &content_id,
		Name:      title,
		Kind:      resource_kind,
		UniqueID:  resource_unique_id,
		Extra:     decrypt_extra_json,
	}
	if resource_kind == mime_video_mp4 {
		video_resource.Size = selected_content_video_size(ext)
	}
	video_endpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      download_url,
		Enabled:  1,
	}
	content_asset_kind := model.ContentAssetKindVideo
	content_asset_role := model.ContentAssetRoleVideoVariant
	content_asset_key := spec
	if content_asset_key == "" {
		content_asset_key = "default"
	}
	if resource_kind == mime_audio_mpeg {
		content_asset_kind = model.ContentAssetKindAudio
		content_asset_role = model.ContentAssetRoleAudioVariant
		content_asset_key = resource_unique_id
	}
	resources := []*adapter.ResourceInfo{{
		Resource:  video_resource,
		Endpoints: []model.DownloadEndpoint{video_endpoint},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     content_asset_kind,
			Role:     content_asset_role,
			AssetKey: content_asset_key,
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	}}
	if a.config_bool("channels.download.cover") && !is_download_feed_cover && cover_url != "" {
		resources = append(resources, build_cover_resource_info(content_id, build_sidecar_cover_unique_id(task_unique_id), title, cover_url, base_extra_json))
	}

	info := &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:  &content_id,
			Name:       title,
			UniqueID:   task_unique_id,
			PlatformId: PlatformID,
			Status:     model.TaskStatusWaiting,
			SourceURL:  content.SourceURL,
			CoverURL:   content.CoverURL,
			ConfigJSON: string(config_json),
		},
		Resources:      resources,
		ContentDetail:  ext,
		ContentDetails: channels_content_details(content, ext),
		Account:        account,
		Content:        content,
	}
	a.preview_download_resource_names(info, video_config)
	return info, nil
}

func channels_content_details(content *model.Content, detail any) []adapter.ContentDetail {
	if content == nil || detail == nil {
		return nil
	}
	return []adapter.ContentDetail{{
		Type: content.Type,
		Key:  content.Id,
		Data: detail,
	}}
}

func build_cover_resource_info(content_id, resource_unique_id, title, cover_url, extra_json string) *adapter.ResourceInfo {
	return &adapter.ResourceInfo{
		Resource: model.DownloadResource{
			ContentId:  &content_id,
			Name:       title,
			Kind:       mime_image_jpeg,
			UniqueID:   resource_unique_id,
			MergeOrder: 0,
			Extra:      extra_json,
		},
		Endpoints: []model.DownloadEndpoint{{
			Protocol: "https",
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

func build_sidecar_cover_unique_id(task_unique_id string) string {
	return task_unique_id + "_sidecar_cover"
}

// build_live_download_task builds a live-stream download task from a joinLive response.
func (a *ChannelsAdapter) build_live_download_task(jl *wxchannels.JoinLivePayload, config map[string]any) (*adapter.DownloadTaskResult, error) {
	live_id := ""
	session_start_time := int64(0)
	if jl.LiveInfo != nil {
		live_id = jl.LiveInfo.LiveId
		session_start_time = int64(jl.LiveInfo.StartTime)
	}

	author_nickname := jl.Nickname
	author_username := jl.Username
	author_avatar_url := ""
	if jl.LiveInfo != nil && jl.AnchorContact != nil {
		if jl.AnchorContact.Nickname != "" {
			author_nickname = jl.AnchorContact.Nickname
		}
		if jl.AnchorContact.Username != "" {
			author_username = jl.AnchorContact.Username
		}
		author_avatar_url = jl.AnchorContact.HeadUrl
	} else if jl.Contact != nil {
		if jl.Contact.Nickname != "" {
			author_nickname = jl.Contact.Nickname
		}
		if jl.Contact.Username != "" {
			author_username = jl.Contact.Username
		}
		author_avatar_url = jl.Contact.HeadUrl
	}

	title := config_string(config, "filename")
	if title == "" {
		if jl.LiveDescription != "" {
			title = jl.LiveDescription
		} else {
			title = "直播"
		}
	}

	now := time.Now().Unix()
	live_config := build_config_json(config, config_string(config, "spec"), wxchannels.MediaTypeLive)
	config_json, _ := json.Marshal(live_config)
	metadata_json, _ := json.Marshal(map[string]any{
		"platform":     PlatformID,
		"id":           live_id,
		"content_type": "live",
		"author":       author_nickname,
		"download_at":  now,
	})

	content := &model.Content{
		Id:         BuildContentID(live_id),
		PlatformId: PlatformID,
		ExternalId: live_id,
		Type:       "live",
		Title:      title,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if session_start_time > 0 {
		pt := session_start_time
		content.PublishTime = &pt
	}

	account := &model.Account{
		Id:         BuildAccountID(author_username),
		PlatformId: PlatformID,
		ExternalId: author_username,
		Nickname:   author_nickname,
		AvatarURL:  author_avatar_url,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	unique_id := live_id + "_" + strconv.FormatInt(session_start_time, 10)
	if session_start_time == 0 {
		unique_id = live_id + "_" + strconv.FormatInt(now, 10)
	}

	stream_resource := model.DownloadResource{
		ContentId:     &content.Id,
		Name:          title,
		Kind:          mime_video_matroska,
		Type:          model.ResourceTypeStream,
		RotateMinutes: 10,
		StreamURL:     jl.LiveSdkInfo.LiveCdnUrl,
		UniqueID:      unique_id,
		Extra:         build_resource_extra_json(live_id, title, config_string(config, "spec"), session_start_time, author_nickname, "", 0, wxchannels.MediaTypeLive),
	}
	stream_endpoint := model.DownloadEndpoint{
		Protocol: "livestream",
		URL:      jl.LiveSdkInfo.LiveCdnUrl,
		Enabled:  1,
	}

	info := &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     unique_id,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			ConfigJSON:   string(config_json),
			MetadataJSON: string(metadata_json),
		},
		Resources: []*adapter.ResourceInfo{{
			Resource:  stream_resource,
			Endpoints: []model.DownloadEndpoint{stream_endpoint},
		}},
		ContentDetail: nil,
		Account:       account,
		Content:       content,
	}
	a.preview_download_resource_names(info, live_config)
	return info, nil
}

// get_media_url returns the combined download URL for a media item (url + urlToken).
func get_media_url(media wxchannels.ChannelsMediaItem) string {
	return media.URL + media.URLToken
}

func parse_channels_object_for_download(content_json json.RawMessage) (wxchannels.ChannelsObject, error) {
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(content_json, &obj); err != nil {
		return obj, err
	}
	if channels_object_has_download_shape(&obj) {
		return obj, nil
	}
	var profile_resp wxchannels.ChannelsFeedProfileResp
	if err := json.Unmarshal(content_json, &profile_resp); err == nil {
		if profile_resp.ErrCode != 0 {
			return obj, fmt.Errorf("fetch channels feed profile: %s", profile_resp.ErrMsg)
		}
		if channels_object_has_download_shape(&profile_resp.Data.Object) {
			return profile_resp.Data.Object, nil
		}
	}
	shared_obj, ok, err := shared_feed_profile_to_channels_object(content_json)
	if err != nil {
		return obj, err
	}
	if ok {
		return *shared_obj, nil
	}
	return obj, nil
}

func channels_object_has_download_shape(obj *wxchannels.ChannelsObject) bool {
	if obj == nil {
		return false
	}
	return obj.LiveInfo != nil ||
		obj.ObjectDesc.MediaType != 0 ||
		len(obj.ObjectDesc.Media) > 0 ||
		len(obj.Files) > 0
}

func shared_feed_profile_to_channels_object(content_json json.RawMessage) (*wxchannels.ChannelsObject, bool, error) {
	resp, ok, err := parse_shared_feed_profile(content_json, true)
	if err != nil || !ok {
		return nil, ok, err
	}
	feed_info := resp.Data.Feedinfo
	switch feed_info.MediaType {
	case wxchannels.MediaTypeVideo:
		return shared_video_profile_to_channels_object(resp, content_json)
	case wxchannels.MediaTypePicture:
		return shared_picture_profile_to_channels_object(resp, content_json)
	}
	if len(feed_info.Picinfo) > 0 {
		return shared_picture_profile_to_channels_object(resp, content_json)
	}
	return nil, false, nil
}

func shared_video_profile_to_channels_object(resp wxchannels.ChannelsSharedFeedProfileResp, content_json json.RawMessage) (*wxchannels.ChannelsObject, bool, error) {
	feed_info := resp.Data.Feedinfo
	video_url := shared_feed_video_url(feed_info)
	if video_url == "" {
		return nil, true, errors.New("分享详情视频类型缺少 videoUrl")
	}

	bgm_url := shared_feed_bgm_url(feed_info.Bgminfo)
	contact_username := shared_feed_author_id(resp.Data.Authorinfo)
	object_id := shared_feed_object_id(resp, content_json)
	media := []wxchannels.ChannelsMediaItem{{
		URL:       video_url,
		ThumbUrl:  strings.TrimSpace(feed_info.Coverurl),
		CoverUrl:  strings.TrimSpace(feed_info.Coverurl),
		MediaType: wxchannels.MediaTypeVideo,
	}}
	obj := &wxchannels.ChannelsObject{
		ID:            object_id,
		ObjectNonceId: object_id,
		CreateTime:    feed_info.Createtime,
		Type:          "video",
		Contact: wxchannels.ChannelsContact{
			Username: contact_username,
			Nickname: strings.TrimSpace(resp.Data.Authorinfo.Nickname),
			HeadUrl:  strings.TrimSpace(resp.Data.Authorinfo.Headimgurl),
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: strings.TrimSpace(feed_info.Description),
			MediaType:   wxchannels.MediaTypeVideo,
			Media:       media,
			FollowPostInfo: wxchannels.ChannelsFollowPostInfo{
				MusicInfo: wxchannels.ChannelsMusicInfo{
					DocId:             feed_info.Bgminfo.DocID,
					DocType:           feed_info.Bgminfo.DocType,
					Name:              feed_info.Bgminfo.Name,
					Artist:            feed_info.Bgminfo.Artist,
					MediaStreamingUrl: bgm_url,
				},
			},
		},
	}
	return obj, true, nil
}

func shared_picture_profile_to_channels_object(resp wxchannels.ChannelsSharedFeedProfileResp, content_json json.RawMessage) (*wxchannels.ChannelsObject, bool, error) {
	feed_info := resp.Data.Feedinfo
	if len(feed_info.Picinfo) == 0 {
		return nil, true, errors.New("分享详情图片类型缺少 picInfo")
	}

	media := make([]wxchannels.ChannelsMediaItem, 0, len(feed_info.Picinfo))
	for _, pic := range feed_info.Picinfo {
		pic_url := strings.TrimSpace(pic.URL)
		if pic_url == "" {
			continue
		}
		media = append(media, wxchannels.ChannelsMediaItem{
			URL:      pic_url,
			Width:    pic.Width,
			Height:   pic.Height,
			FileSize: pic.FileSize,
			CoverUrl: strings.TrimSpace(feed_info.Coverurl),
		})
	}
	if len(media) == 0 {
		return nil, true, errors.New("分享详情图片类型 picInfo 未包含下载地址")
	}

	bgm_url := shared_feed_bgm_url(feed_info.Bgminfo)
	contact_username := shared_feed_author_id(resp.Data.Authorinfo)
	object_id := shared_feed_object_id(resp, content_json)
	obj := &wxchannels.ChannelsObject{
		ID:            object_id,
		ObjectNonceId: object_id,
		CreateTime:    feed_info.Createtime,
		Type:          "picture",
		Contact: wxchannels.ChannelsContact{
			Username: contact_username,
			Nickname: strings.TrimSpace(resp.Data.Authorinfo.Nickname),
			HeadUrl:  strings.TrimSpace(resp.Data.Authorinfo.Headimgurl),
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: strings.TrimSpace(feed_info.Description),
			MediaType:   wxchannels.MediaTypePicture,
			Media:       media,
			FollowPostInfo: wxchannels.ChannelsFollowPostInfo{
				MusicInfo: wxchannels.ChannelsMusicInfo{
					DocId:             feed_info.Bgminfo.DocID,
					DocType:           feed_info.Bgminfo.DocType,
					Name:              feed_info.Bgminfo.Name,
					Artist:            feed_info.Bgminfo.Artist,
					MediaStreamingUrl: bgm_url,
				},
			},
		},
		Files: media,
	}
	return obj, true, nil
}

func shared_feed_video_url(feed_info wxchannels.SharedFeedinfo) string {
	if video_url := strings.TrimSpace(feed_info.H264VideoInfo.VideoURL); video_url != "" {
		return video_url
	}
	if video_url := strings.TrimSpace(feed_info.VideoURL); video_url != "" {
		return video_url
	}
	return strings.TrimSpace(feed_info.H265VideoInfo.VideoURL)
}

func parse_shared_feed_profile(content_json json.RawMessage, allow_envelope bool) (wxchannels.ChannelsSharedFeedProfileResp, bool, error) {
	var resp wxchannels.ChannelsSharedFeedProfileResp
	direct_err := json.Unmarshal(content_json, &resp)
	if direct_err == nil {
		if len(resp.Data.Feedinfo.Picinfo) > 0 || resp.Data.Feedinfo.MediaType != 0 {
			return resp, true, nil
		}
	}

	if !allow_envelope {
		return resp, false, direct_err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(content_json, &envelope); err != nil {
		return resp, false, err
	}
	if _, has_code := envelope["code"]; !has_code {
		return resp, false, nil
	}
	data, has_data := envelope["data"]
	if !has_data || len(data) == 0 {
		return resp, false, nil
	}
	unwrapped, ok, err := parse_shared_feed_profile(data, false)
	if ok || err == nil {
		return unwrapped, ok, err
	}
	if direct_err != nil {
		return resp, false, direct_err
	}
	return resp, false, err
}

func shared_feed_bgm_url(info wxchannels.SharedFeedBGMInfo) string {
	if u := strings.TrimSpace(info.BGMURL); u != "" {
		return u
	}
	return strings.TrimSpace(info.MediaStreamingURL)
}

func shared_feed_object_id(resp wxchannels.ChannelsSharedFeedProfileResp, content_json json.RawMessage) string {
	candidate := strings.TrimSpace(resp.Data.Sceneinfo.Dynamicexportid)
	if candidate == "" {
		candidate = strings.TrimSpace(resp.Data.Feedinfo.Coverurl)
	}
	if candidate == "" && len(resp.Data.Feedinfo.Picinfo) > 0 {
		candidate = strings.TrimSpace(resp.Data.Feedinfo.Picinfo[0].URL)
	}
	if candidate == "" {
		return "shared_" + hash_string(string(content_json))[:16]
	}
	safe := safe_identifier(candidate)
	if safe == "" {
		return "shared_" + hash_string(candidate)[:16]
	}
	if len(safe) > 96 {
		return safe[:80] + "_" + hash_string(candidate)[:12]
	}
	return safe
}

func shared_feed_author_id(author wxchannels.SharedFeedAuthorinfo) string {
	candidate := strings.TrimSpace(author.Headimgurl)
	if candidate == "" {
		candidate = strings.TrimSpace(author.Nickname)
	}
	if candidate == "" {
		return ""
	}
	return "shared_author_" + hash_string(candidate)[:16]
}

func safe_identifier(value string) string {
	var b strings.Builder
	last_underscore := false
	for _, r := range strings.TrimSpace(value) {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if ok {
			b.WriteRune(r)
			last_underscore = false
			continue
		}
		if !last_underscore {
			b.WriteByte('_')
			last_underscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func hash_string(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func endpoint_protocol(raw_url string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw_url))
	if err == nil && parsed.Scheme != "" {
		return strings.ToLower(parsed.Scheme)
	}
	return "https"
}

// bgm_info holds background music download info extracted from a picture feed.
type bgm_info struct {
	url  string
	name string
}

// format_bgm extracts background music info from a picture feed's followPostInfo.
func format_bgm(obj *wxchannels.ChannelsObject) *bgm_info {
	music_info := obj.ObjectDesc.FollowPostInfo.MusicInfo
	if music_info.MediaStreamingUrl == "" {
		return nil
	}
	name := "bgm"
	if music_info.Name != "" {
		name = sanitize_bgm_name(music_info.Name)
	}
	return &bgm_info{url: music_info.MediaStreamingUrl, name: name}
}

// sanitize_bgm_name removes characters unsafe for filenames.
func sanitize_bgm_name(name string) string {
	r := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return r.Replace(name)
}

// BuildDownloadTaskUniqueID computes a unique task ID from the content ID and download configuration.
func BuildDownloadTaskUniqueID(external_id string, config map[string]any) string {
	suffix_config := normalized_config_suffix(config)
	if suffix_config == "jpg" || suffix_config == "jpeg" {
		return external_id + "_cover"
	}
	unique_id := external_id
	if spec := config_string(config, "spec"); spec != "" {
		unique_id += "_" + spec
	}
	if suffix_config != "" {
		if suffix := safe_identifier(suffix_config); suffix != "" {
			unique_id += "_" + suffix
		}
	}
	return unique_id
}

// build_resource_extra_json builds the resource.Extra JSON string.
func build_resource_extra_json(id, title, spec string, created_at int64, author string, decode_key string, idx int, media_type int) string {

	now := time.Now().Unix()
	filename := title
	if filename == "" {
		filename = id
	}
	if filename == "" {
		filename = strconv.FormatInt(now, 10)
	}

	type extra struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Filename   string `json:"filename"`
		Spec       string `json:"spec"`
		CreatedAt  string `json:"created_at"`
		DownloadAt string `json:"download_at"`
		Author     string `json:"author"`
		Idx        int    `json:"idx,omitempty"`
		MediaType  int    `json:"type,omitempty"`
		DecodeKey  string `json:"decode_key,omitempty"`
	}
	data, _ := json.Marshal(extra{
		ID:         id,
		Title:      title,
		Filename:   filename,
		Spec:       spec,
		CreatedAt:  strconv.FormatInt(created_at, 10),
		DownloadAt: strconv.FormatInt(now, 10),
		Author:     author,
		Idx:        idx,
		MediaType:  media_type,
		DecodeKey:  decode_key,
	})
	return string(data)
}

func (a *ChannelsAdapter) preview_download_resource_names(info *adapter.DownloadTaskResult, config map[string]any) {
	if info == nil || info.Task == nil {
		return
	}
	hooks, logger := a.filename_hook_context()
	template := a.filename_template()
	apply_download_resource_names(info, config, template, hooks, logger)
}

func apply_download_resource_names(info *adapter.DownloadTaskResult, config map[string]any, template string, hooks *hermes.HookManager, logger *zerolog.Logger) {
	if info == nil || (template == "" && (hooks == nil || !hooks.HasFilenameHook())) {
		return
	}
	for _, resource_info := range info.Resources {
		if resource_info == nil {
			continue
		}
		resource := &resource_info.Resource
		resolved := hermes.BuildFinalResourceName(hermes.FinalResourceNameInput{
			TaskID:           info.Task.Id,
			TaskName:         info.Task.Name,
			TaskConfig:       config,
			FilenameTemplate: template,
			ResourceID:       resource.Id,
			ResourceName:     resource.Name,
			ResourceKind:     resource.Kind,
			ResourceType:     resource.Type,
			ResourceExtra:    resource_extra_map(resource.Extra),
			Hooks:            hooks,
		})
		log_filename_resolution(logger, resolved)
		// DownloadResource.Name is the extensionless logical name. Hermes adds
		// the canonical suffix derived from Resource.Kind only when resolving the
		// final output path.
		if resolved.BaseName != "" {
			resource.Name = resolved.BaseName
		}
	}
}

func log_filename_resolution(logger *zerolog.Logger, resolved hermes.FinalResourceNameResult) {
	if logger != nil && resolved.TemplateError != nil {
		logger.Warn().Err(resolved.TemplateError).Msg("wxchannels preview filename template error")
	}
	if logger != nil && resolved.HookError != nil {
		logger.Warn().
			Err(resolved.HookError).
			Interface("meta", resolved.HookMeta).
			Msg("wxchannels preview filename hook failed")
	}
}

func (a *ChannelsAdapter) filename_template() string {
	return strings.TrimSpace(a.config_string("download.filenameTemplate"))
}

func (a *ChannelsAdapter) filename_hook_context() (*hermes.HookManager, *zerolog.Logger) {
	if a == nil {
		return nil, nil
	}
	a.runtime_mu.Lock()
	hooks := a.hooks
	logger := a.logger
	a.runtime_mu.Unlock()
	return hooks, logger
}

func resource_extra_map(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var attrs map[string]any
	if err := json.Unmarshal([]byte(raw), &attrs); err != nil {
		return nil
	}

	extra := make(map[string]string, len(attrs))
	for key, value := range attrs {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			extra[key] = typed
		case fmt.Stringer:
			extra[key] = typed.String()
		default:
			extra[key] = strings.TrimSpace(fmt.Sprintf("%v", value))
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

func normalized_config_suffix(config map[string]any) string {
	suffix := strings.ToLower(strings.TrimSpace(config_string(config, "suffix")))
	return strings.TrimLeft(suffix, ".")
}

func config_suffix_is_cover(config map[string]any) bool {
	suffix := normalized_config_suffix(config)
	return suffix == "jpg" || suffix == "jpeg"
}

func config_suffix_is_mp3(config map[string]any) bool {
	return normalized_config_suffix(config) == "mp3"
}

// build_config_json returns a map containing the config fields whose value is set / true,
// plus the media type so post-processing can branch on it.
func build_config_json(config map[string]any, spec string, media_type int) map[string]any {
	m := make(map[string]any, len(config)+2)
	for key, value := range config {
		m[key] = value
	}
	if spec != "" {
		m["spec"] = spec
	}
	m["type"] = media_type
	return m
}

func config_string(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}

func config_int(config map[string]any, key string) int {
	switch value := config[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		n, _ := strconv.Atoi(value)
		return n
	default:
		return 0
	}
}

// parse_key_from_content extracts the decrypt key from Content.Metadata.
func parse_key_from_content(c *model.Content) string {
	if c == nil || c.Metadata == "" {
		return ""
	}
	var kv struct {
		Key string `json:"key"`
	}
	if json.Unmarshal([]byte(c.Metadata), &kv) == nil {
		return kv.Key
	}
	return ""
}

// sanitize_filename ensures the filename has an extension, extracting from the URL if needed.
func sanitize_filename(name string) string {
	name_without_ext := strings.TrimSuffix(name, filepath.Ext(name))
	return name_without_ext
}
