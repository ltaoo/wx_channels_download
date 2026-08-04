package wxchannels

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	"wx_channel/internal/download/types"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

func init() {
	registry.Register(&handler{})
}

type handler struct{}

const (
	mimeImageJPEG     = "image/jpeg"
	mimeAudioMPEG     = "audio/mpeg"
	mimeVideoMP4      = "video/mp4"
	mimeVideoMatroska = "video/x-matroska"
)

func (h *handler) PlatformID() string { return PlatformID }

// DecryptKey exposes the legacy channels conversion capability through the
// registered handler, so callers do not need to import this package.
func (h *handler) DecryptKey(contentJSON json.RawMessage) (int, error) {
	var obj scraper.ChannelsObject
	if err := json.Unmarshal(contentJSON, &obj); err != nil {
		return 0, err
	}
	return DecryptKeyInt(&obj), nil
}

// ConvertContent converts a raw channels object into the shared content model.
func (h *handler) ConvertContent(contentJSON json.RawMessage) (*model.Content, error) {
	var obj scraper.ChannelsObject
	if err := json.Unmarshal(contentJSON, &obj); err != nil {
		return nil, err
	}
	content, _, err := ToContent(&obj)
	return content, err
}

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, configRaw json.RawMessage) (*types.DownloadTaskResult, error) {
	// fmt.Println("[wxchannels]BuildDownloadTask")
	var config map[string]any
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	// Live stream detection: joinLive response contains liveSdkInfo
	var jl scraper.JoinLivePayload
	if json.Unmarshal(contentJSON, &jl) == nil && jl.LiveSdkInfo != nil && jl.LiveSdkInfo.LiveCdnUrl != "" {
		return buildLiveDownloadTask(&jl, config)
	}

	var obj scraper.ChannelsObject
	if err := json.Unmarshal(contentJSON, &obj); err != nil {
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

	title := configString(config, "filename")
	if title == "" {
		title = ObjectTitle(&obj)
	}
	configuredSpec := configString(config, "spec")
	var spec string
	if configuredSpec == "" {
		if !GetChannelsConfig().DownloadDefaultHighest {
			spec = PickSpec(&obj)
		}
	} else if configuredSpec != "original" {
		spec = configuredSpec
	}
	log.Printf("DownloadDefaultHighest=%v, config.Spec=%q, final spec=%q", GetChannelsConfig().DownloadDefaultHighest, configuredSpec, spec)
	// "original" means highest quality → spec stays ""
	coverURL := strings.TrimSpace(content.CoverURL)
	if len(obj.ObjectDesc.Media) > 0 {
		if candidate := strings.TrimSpace(obj.ObjectDesc.Media[0].CoverUrl); candidate != "" {
			coverURL = candidate
		} else if candidate := strings.TrimSpace(obj.ObjectDesc.Media[0].ThumbUrl); candidate != "" {
			coverURL = candidate
		}
	}
	contact, _ := pickAccountContact(&obj)
	decryptKey := parseKeyFromContent(content)
	baseExtraJSON := buildResourceExtraJSON(obj.ID, title, spec, int64(obj.CreateTime), contact.Nickname, "", 0, obj.ObjectDesc.MediaType)
	decryptExtraJSON := buildResourceExtraJSON(obj.ID, title, spec, int64(obj.CreateTime), contact.Nickname, decryptKey, 0, obj.ObjectDesc.MediaType)
	contentID := content.Id
	task := func(configJSON []byte) *model.DownloadTask {
		contentID := content.Id
		task := &model.DownloadTask{
			ContentId:  &contentID,
			Name:       title,
			UniqueID:   BuildDownloadTaskUniqueID(content.ExternalId, map[string]any{"suffix": configString(config, "suffix"), "spec": spec}),
			PlatformId: PlatformID,
			Status:     model.TaskStatusWaiting,
			SourceURL:  content.SourceURL,
			CoverURL:   content.CoverURL,
			ConfigJSON: string(configJSON),
		}
		_ = ext // No-op: ContentVideo no longer has CoverWidth/CoverHeight
		return task
	}

	// Cover download: create cover resource only
	if configString(config, "suffix") == ".jpg" && coverURL != "" {
		configJSON, _ := json.Marshal(buildConfigJSON(config, spec, obj.ObjectDesc.MediaType))
		coverResource := model.DownloadResource{
			ContentId:  &contentID,
			Name:       title,
			Kind:       mimeImageJPEG,
			UniqueID:   content.ExternalId + "_cover",
			MergeOrder: 0,
			Extra:      baseExtraJSON,
		}
		coverEndpoint := model.DownloadEndpoint{
			Protocol: "https",
			URL:      coverURL,
			Enabled:  1,
		}
		info := &types.DownloadTaskResult{
			Task: task(configJSON),
			Resources: []*types.ResourceInfo{{
				DownloadResource: coverResource,
				Endpoints:        []model.DownloadEndpoint{coverEndpoint},
			}},
			Account: account,
			Content: content,
		}
		setContentExt(info, ext)
		return info, nil
	}

	// Picture type: create a download resource for each media item, plus background music
	if obj.ObjectDesc.MediaType == scraper.MediaTypePicture {
		files := obj.Files
		if len(files) == 0 {
			files = obj.ObjectDesc.Media
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("图片类型缺少文件数据")
		}

		resources := make([]*types.ResourceInfo, 0, len(files)+1)
		for i, file := range files {
			mediaURL := getMediaURL(file)
			if mediaURL == "" {
				return nil, fmt.Errorf("图片 %d 下载地址为空", i+1)
			}
			imageName := title
			if len(files) > 1 {
				imageName = fmt.Sprintf("%s_%d", title, i+1)
			}
			imageExtraJSON := buildResourceExtraJSON(obj.ID, title, spec, int64(obj.CreateTime), contact.Nickname, decryptKey, i, obj.ObjectDesc.MediaType)
			resources = append(resources, &types.ResourceInfo{
				DownloadResource: model.DownloadResource{
					ContentId: &contentID,
					Name:      sanitizeFilename(imageName),
					Kind:      mimeImageJPEG,
					Size:      int64(file.FileSize),
					UniqueID:  content.ExternalId + "_" + strconv.Itoa(i),
					Extra:     imageExtraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "https",
					URL:      mediaURL,
					Enabled:  1,
				}},
			})
		}

		// Background music
		bgm := formatBGM(&obj)
		if bgm != nil {
			resources = append(resources, &types.ResourceInfo{
				DownloadResource: model.DownloadResource{
					ContentId: &contentID,
					Name:      bgm.Name,
					Kind:      mimeAudioMPEG,
					UniqueID:  content.ExternalId + "_bgm",
					Extra:     baseExtraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "http",
					URL:      bgm.URL,
					Enabled:  1,
				}},
			})
		}

		configJSON, _ := json.Marshal(buildConfigJSON(config, spec, scraper.MediaTypePicture))

		info := &types.DownloadTaskResult{
			Task:      task(configJSON),
			Resources: resources,
			Account:   account,
			Content:   content,
		}
		setContentExt(info, ext)
		return info, nil
	}

	// Video type
	downloadURL := BuildDownloadURLWithSpec(&obj, spec)
	if downloadURL == "" {
		return nil, fmt.Errorf("无法获取视频下载地址")
	}

	configJSON, _ := json.Marshal(buildConfigJSON(config, spec, obj.ObjectDesc.MediaType))

	resourceUniqueID := content.ExternalId
	resourceKind := mimeVideoMP4
	if spec != "" {
		resourceUniqueID = content.ExternalId + "_" + spec
	}
	if configString(config, "suffix") == ".mp3" {
		resourceUniqueID += "_mp3"
		resourceKind = mimeAudioMPEG
	}
	videoResource := model.DownloadResource{
		ContentId: &contentID,
		Name:      title,
		Kind:      resourceKind,
		UniqueID:  resourceUniqueID,
		Extra:     decryptExtraJSON,
	}
	if ve, ok := ext.(*model.ContentVideo); ok {
		videoResource.Size = ve.Size
	}
	videoEndpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      downloadURL,
		Enabled:  1,
	}
	resources := []*types.ResourceInfo{{
		DownloadResource: videoResource,
		Endpoints:        []model.DownloadEndpoint{videoEndpoint},
	}}

	info := &types.DownloadTaskResult{
		Task:      task(configJSON),
		Resources: resources,
		Account:   account,
		Content:   content,
	}
	setContentExt(info, ext)
	return info, nil
}

// buildLiveDownloadTask builds a live-stream download task from a joinLive response.
// Author info prefers anchorContact (live-specific), then contact, then top-level nickname/username.
//
// UniqueID uses the combination of liveId + sessionStartTime to differentiate
// different sessions of the same live (e.g. the stream was interrupted and restarted,
// each session has a different startTime).
func buildLiveDownloadTask(jl *scraper.JoinLivePayload, config map[string]any) (*types.DownloadTaskResult, error) {
	liveId := ""
	sessionStartTime := int64(0)
	if jl.LiveInfo != nil {
		liveId = jl.LiveInfo.LiveId
		sessionStartTime = int64(jl.LiveInfo.StartTime)
	}

	// Pick author info: prefer AnchorContact for live, then Contact, then top-level fields.
	authorNickname := jl.Nickname
	authorUsername := jl.Username
	authorAvatarURL := ""
	if jl.LiveInfo != nil && jl.AnchorContact != nil {
		if jl.AnchorContact.Nickname != "" {
			authorNickname = jl.AnchorContact.Nickname
		}
		if jl.AnchorContact.Username != "" {
			authorUsername = jl.AnchorContact.Username
		}
		authorAvatarURL = jl.AnchorContact.HeadUrl
	} else if jl.Contact != nil {
		if jl.Contact.Nickname != "" {
			authorNickname = jl.Contact.Nickname
		}
		if jl.Contact.Username != "" {
			authorUsername = jl.Contact.Username
		}
		authorAvatarURL = jl.Contact.HeadUrl
	}

	title := configString(config, "filename")
	if title == "" {
		if jl.LiveDescription != "" {
			title = jl.LiveDescription
		} else {
			title = "直播"
		}
	}

	now := time.Now().Unix()
	configJSON, _ := json.Marshal(buildConfigJSON(config, configString(config, "spec"), scraper.MediaTypeLive))
	metadataJSON, _ := json.Marshal(map[string]any{
		"platform":     PlatformID,
		"id":           liveId,
		"content_type": "live",
		"author":       authorNickname,
		"download_at":  now,
	})

	content := &model.Content{
		Id:         BuildContentID(liveId),
		PlatformId: wxchannels,
		ExternalId: liveId,
		Type:       "live",
		Title:      title,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if sessionStartTime > 0 {
		pt := sessionStartTime
		content.PublishTime = &pt
	}

	account := &model.Account{
		Id:         BuildAccountID(authorUsername),
		PlatformId: wxchannels,
		ExternalId: authorUsername,
		Nickname:   authorNickname,
		AvatarURL:  authorAvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// UniqueID = liveId + sessionStartTime, so different sessions of the same live can create separate download tasks.
	uniqueID := liveId + "_" + strconv.FormatInt(sessionStartTime, 10)
	if sessionStartTime == 0 {
		uniqueID = liveId + "_" + strconv.FormatInt(now, 10)
	}

	streamResource := model.DownloadResource{
		ContentId:     &content.Id,
		Name:          title + ".mkv",
		Kind:          mimeVideoMatroska,
		Type:          model.ResourceTypeStream,
		RotateMinutes: 10,
		StreamURL:     jl.LiveSdkInfo.LiveCdnUrl,
		UniqueID:      uniqueID,
	}
	streamEndpoint := model.DownloadEndpoint{
		Protocol: "livestream",
		URL:      jl.LiveSdkInfo.LiveCdnUrl,
		Enabled:  1,
	}

	return &types.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     uniqueID,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			ConfigJSON:   string(configJSON),
			MetadataJSON: string(metadataJSON),
		},
		Resources: []*types.ResourceInfo{{
			DownloadResource: streamResource,
			Endpoints:        []model.DownloadEndpoint{streamEndpoint},
		}},
		ContentDetail: nil,
		Account:       account,
		Content:       content,
	}, nil
}

// getMediaURL returns the combined download URL for a media item (url + urlToken).
// Mirrors JS get_media_url.
func getMediaURL(media scraper.ChannelsMediaItem) string {
	return media.URL + media.URLToken
}

// bgmInfo holds background music download info extracted from a picture feed.
type bgmInfo struct {
	URL  string
	Name string
}

// formatBGM extracts background music info from a picture feed's followPostInfo.
// Returns nil if no valid music URL is found. Mirrors JS format_bgm.
func formatBGM(obj *scraper.ChannelsObject) *bgmInfo {
	musicInfo := obj.ObjectDesc.FollowPostInfo.MusicInfo
	if musicInfo.MediaStreamingUrl == "" {
		return nil
	}
	name := "bgm"
	if musicInfo.Name != "" {
		name = sanitizeBGMName(musicInfo.Name)
	}
	return &bgmInfo{URL: musicInfo.MediaStreamingUrl, Name: name}
}

// sanitizeBGMName removes characters unsafe for filenames.
func sanitizeBGMName(name string) string {
	r := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return r.Replace(name)
}

// BuildDownloadTaskUniqueID computes a unique task ID from the content ID and download configuration.
//
// The ID ensures that different download modes of the same content — cover-only (.jpg),
// mp3 conversion, picture album, or video at different quality specs — each produce
// distinct tasks without duplicate conflicts.
//
// Formats:
//   - Cover-only:            {externalID}_cover           (config.Suffix == ".jpg")
//   - Picture album:         {externalID}_picture         (isPicture)
//   - MP3 + spec:           {externalID}_{spec}_mp3      (config.Suffix == ".mp3", config.Spec is a codec name)
//   - MP3 (default):         {externalID}_mp3             (config.Suffix == ".mp3")
//   - Video + spec:         {externalID}_{spec}           (config.Spec is a codec name)
//   - Video (default):       {externalID}                 (config.Spec is "" or "original")
func BuildDownloadTaskUniqueID(externalID string, config map[string]any) string {
	suffixConfig := configString(config, "suffix")
	if suffixConfig == ".jpg" {
		return externalID + "_cover"
	}
	var suffix string
	if spec := configString(config, "spec"); spec != "" {
		suffix = "_" + spec
	}
	if suffixConfig == ".mp3" {
		suffix += "_mp3"
	}
	return externalID + suffix
}

// buildResourceExtraJSON builds the resource.Extra JSON string.
// When decodeKey is non-empty, the resource needs decryption; this is recorded in Extra for the postprocess pipeline.
func buildResourceExtraJSON(id, title, spec string, createdAt int64, author string, decodeKey string, idx int, mediaType int) string {

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
		CreatedAt:  strconv.FormatInt(createdAt, 10),
		DownloadAt: strconv.FormatInt(now, 10),
		Author:     author,
		Idx:        idx,
		MediaType:  mediaType,
		DecodeKey:  decodeKey,
	})
	return string(data)
}

// buildConfigJSON returns a map containing the config fields whose value is set / true,
// plus the media type so post-processing can branch on it.
// This keeps config_json compact by omitting empty/false fields.
func buildConfigJSON(config map[string]any, spec string, mediaType int) map[string]any {
	m := make(map[string]any, len(config)+2)
	for key, value := range config {
		m[key] = value
	}
	if spec != "" {
		m["spec"] = spec
	}
	m["type"] = mediaType
	return m
}

func configString(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}

// parseKeyFromContent extracts the decrypt key from Content.Metadata.
func parseKeyFromContent(c *model.Content) string {
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

// sanitizeFilename ensures the filename has an extension, extracting from the URL if needed.
func sanitizeFilename(name string) string {
	nameWithoutExt := strings.TrimSuffix(name, filepath.Ext(name))
	return nameWithoutExt
}

// setContentExt routes the extension data: *ContentAlbum.Image goes to AlbumImages,
// []*ContentImage goes to AlbumImages, other non-nil values go to ContentDetail.
func setContentExt(dtr *types.DownloadTaskResult, ext any) {
	switch e := ext.(type) {
	case *model.ContentAlbum:
		dtr.ContentDetail = e
		dtr.AlbumImages = contentAlbumImages(e.Images)
	case []*model.ContentImage:
		dtr.AlbumImages = e
	case nil:
	default:
		dtr.ContentDetail = e
	}
}

func contentAlbumImages(images []model.ContentImage) []*model.ContentImage {
	if len(images) == 0 {
		return nil
	}
	result := make([]*model.ContentImage, len(images))
	for i := range images {
		result[i] = &images[i]
	}
	return result
}
