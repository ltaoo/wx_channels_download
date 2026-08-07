package wxchannelsadapter

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/util"
)

const platformIDWxChannels = "wxchannels"

// PlatformID is the platform identifier for wechat channels.
const PlatformID = platformIDWxChannels

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(externalID string) string {
	return PlatformID + ":" + externalID
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(externalID string) string {
	return PlatformID + ":" + externalID
}

type metadataKV struct {
	Key string `json:"key"`
}

// cleanMediaURL removes CDN routing parameters (hy, idx, m, uzid) from the media URL.
func cleanMediaURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u == nil {
		return rawURL
	}
	q := u.Query()
	q.Del("hy")
	q.Del("idx")
	q.Del("m")
	q.Del("uzid")
	u.RawQuery = q.Encode()
	return u.String()
}

func firstMediaCoverURL(file wxchannels.ChannelsMediaItem) string {
	coverURL := strings.TrimSpace(file.CoverUrl)
	if coverURL != "" {
		return coverURL
	}
	mediaURL := strings.TrimSpace(file.URL + file.URLToken)
	if mediaURL != "" {
		return mediaURL
	}
	return ""
}

// ToAccount converts a ChannelsObject into a model.Account.
func ToAccount(obj *wxchannels.ChannelsObject) (*model.Account, error) {
	if obj == nil {
		return nil, errors.New("channels object is nil")
	}

	contact, accountUsername := pickAccountContact(obj)

	now := util.NowMillis()
	acc := &model.Account{
		Id:         BuildAccountID(accountUsername),
		PlatformId: PlatformID,
		ExternalId: accountUsername,
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

// pickAccountContact selects the appropriate contact and external ID for an account.
// For live objects, prefers AnchorContact over Contact.
func pickAccountContact(obj *wxchannels.ChannelsObject) (wxchannels.ChannelsContact, string) {
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
			publishTime := int64(obj.CreateTime)
			c.PublishTime = &publishTime
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
		c.CoverURL = firstMediaCoverURL(files[0])
		c.CoverWidth = strconv.Itoa(int(files[0].Width))
		c.CoverHeight = strconv.Itoa(int(files[0].Height))
		if obj.CreateTime > 0 {
			publishTime := int64(obj.CreateTime)
			c.PublishTime = &publishTime
		}

		md, _ := json.Marshal(metadataKV{Key: files[0].DecodeKey})
		c.Metadata = string(md)
		images := make([]model.ContentImage, 0, len(files))
		for i, file := range files {
			images = append(images, model.ContentImage{
				AlbumId:   c.Id,
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
	c.URL = cleanMediaURL(media.URL) + media.URLToken
	c.CoverURL = media.ThumbUrl
	c.CoverWidth = strconv.Itoa(int(media.Width))
	c.CoverHeight = strconv.Itoa(int(media.Height))
	if c.SourceURL == "" {
		_, contactUsername := pickAccountContact(obj)
		c.SourceURL = BuildJumpURLFromParts(obj.ID, obj.ObjectNonceId, "", contactUsername)
	}

	if obj.CreateTime > 0 {
		publishTime := int64(obj.CreateTime)
		c.PublishTime = &publishTime
	}

	md, _ := json.Marshal(metadataKV{Key: media.DecodeKey})
	c.Metadata = string(md)
	ext := &model.ContentVideo{
		Id:       c.Id,
		Duration: int64(media.VideoPlayLen),
		Width:    int(media.Width),
		Height:   int(media.Height),
		Size:     int64(media.FileSize),
		URL:      c.URL,
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

	accountUsername := strings.TrimSpace(obj.Contact.Username)
	now := util.NowMillis()

	var key string
	if len(obj.ObjectDesc.Media) > 0 {
		key = obj.ObjectDesc.Media[0].DecodeKey
	}

	extraData, _ := json.Marshal(map[string]any{
		"id":         obj.ID,
		"nonce_id":   obj.ObjectNonceId,
		"decode_key": key,
	})

	browseID := PlatformID + ":" + obj.ID
	contentSourceURL := obj.SourceURL
	if contentSourceURL == "" {
		contentSourceURL = BuildJumpURLFromParts(obj.ID, obj.ObjectNonceId, "", accountUsername)
	}

	coverURL := ""
	coverWidth := ""
	coverHeight := ""
	mediaList := obj.Files
	if len(mediaList) == 0 {
		mediaList = obj.ObjectDesc.Media
	}
	if len(mediaList) > 0 {
		media := mediaList[0]
		if obj.ObjectDesc.MediaType == wxchannels.MediaTypePicture {
			coverURL = firstMediaCoverURL(media)
		} else {
			coverURL = strings.TrimSpace(media.ThumbUrl)
		}
		coverWidth = strconv.Itoa(int(media.Width))
		coverHeight = strconv.Itoa(int(media.Height))
	}
	publishTime := int64(obj.CreateTime)

	contentType := "video"
	if obj.ObjectDesc.MediaType == wxchannels.MediaTypePicture {
		contentType = "album"
	}

	account, err := ToAccount(&obj)
	if err != nil {
		return nil, err
	}

	return &adapter.BrowseHistoryResult{
		BrowseHistory: &model.BrowseHistory{
			Id:           browseID,
			PlatformId:   PlatformID,
			VisitedTimes: 1,
			Type:         contentType,
			ExternalId:   obj.ID,
			Title:        obj.ObjectDesc.Description,
			URL:          ObjectURL(&obj),
			SourceURL:    contentSourceURL,
			CoverURL:     coverURL,
			CoverWidth:   coverWidth,
			CoverHeight:  coverHeight,
			PublishTime:  &publishTime,
			ExtraData:    string(extraData),
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
	return cleanMediaURL(obj.ObjectDesc.Media[0].URL) + obj.ObjectDesc.Media[0].URLToken
}

// BuildJumpURLFromParts builds a channels.weixin.qq.com feed page URL from individual fields.
func BuildJumpURLFromParts(objectId, nonceId, sourceURL, username string) string {
	origin := "https://channels.weixin.qq.com"
	if sourceURL != "" {
		return sourceURL
	}

	oid := objectId
	nid := nonceId
	u := origin + "/web/pages/feed"
	if username != "" {
		u += "?username=" + url.QueryEscape(username)
	} else {
		u += "?"
	}

	if oid != "" {
		encodedOid := util.EncodeUint64ToBase64(oid)
		if encodedOid != "" {
			u += "&oid=" + url.QueryEscape(encodedOid)
		}
	}

	if nid != "" {
		// NonceId may contain underscore-separated segments (e.g. "123_0_146_0_0").
		// The first segment is the numeric ID used for encoding.
		if idx := strings.IndexByte(nid, '_'); idx >= 0 {
			nid = nid[:idx]
		}
		encodedNid := util.EncodeUint64ToBase64(nid)
		if encodedNid != "" {
			u += "&nid=" + url.QueryEscape(encodedNid)
		}
	}

	return strings.TrimSuffix(strings.Replace(u, "?&", "?", 1), "?")
}

const (
	mimeImageJPEG     = "image/jpeg"
	mimeAudioMPEG     = "audio/mpeg"
	mimeVideoMP4      = "video/mp4"
	mimeVideoMatroska = "video/x-matroska"
)

func (a *ChannelsAdapter) BuildDownloadTask(contentJSON json.RawMessage, configRaw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	// Live stream detection: joinLive response contains liveSdkInfo
	var jl wxchannels.JoinLivePayload
	if json.Unmarshal(contentJSON, &jl) == nil && jl.LiveSdkInfo != nil && jl.LiveSdkInfo.LiveCdnUrl != "" {
		return buildLiveDownloadTask(&jl, config)
	}

	obj, err := parseChannelsObjectForDownload(contentJSON)
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
		_ = ext
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
		info := &adapter.DownloadTaskResult{
			Task: task(configJSON),
			Resources: []*adapter.ResourceInfo{{
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
	if obj.ObjectDesc.MediaType == wxchannels.MediaTypePicture {
		files := obj.Files
		if len(files) == 0 {
			files = obj.ObjectDesc.Media
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("图片类型缺少文件数据")
		}

		resources := make([]*adapter.ResourceInfo, 0, len(files)+1)
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
			resources = append(resources, &adapter.ResourceInfo{
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
			resources = append(resources, &adapter.ResourceInfo{
				DownloadResource: model.DownloadResource{
					ContentId: &contentID,
					Name:      bgm.Name,
					Kind:      mimeAudioMPEG,
					UniqueID:  content.ExternalId + "_bgm",
					Extra:     baseExtraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: endpointProtocol(bgm.URL),
					URL:      bgm.URL,
					Enabled:  1,
				}},
			})
		}

		pictureConfig := buildConfigJSON(config, spec, wxchannels.MediaTypePicture)
		if configString(pictureConfig, "suffix") == "" {
			pictureConfig["suffix"] = ".zip"
		}
		configJSON, _ := json.Marshal(pictureConfig)

		info := &adapter.DownloadTaskResult{
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
	resources := []*adapter.ResourceInfo{{
		DownloadResource: videoResource,
		Endpoints:        []model.DownloadEndpoint{videoEndpoint},
	}}

	info := &adapter.DownloadTaskResult{
		Task:      task(configJSON),
		Resources: resources,
		Account:   account,
		Content:   content,
	}
	setContentExt(info, ext)
	return info, nil
}

// buildLiveDownloadTask builds a live-stream download task from a joinLive response.
func buildLiveDownloadTask(jl *wxchannels.JoinLivePayload, config map[string]any) (*adapter.DownloadTaskResult, error) {
	liveId := ""
	sessionStartTime := int64(0)
	if jl.LiveInfo != nil {
		liveId = jl.LiveInfo.LiveId
		sessionStartTime = int64(jl.LiveInfo.StartTime)
	}

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
	configJSON, _ := json.Marshal(buildConfigJSON(config, configString(config, "spec"), wxchannels.MediaTypeLive))
	metadataJSON, _ := json.Marshal(map[string]any{
		"platform":     PlatformID,
		"id":           liveId,
		"content_type": "live",
		"author":       authorNickname,
		"download_at":  now,
	})

	content := &model.Content{
		Id:         BuildContentID(liveId),
		PlatformId: PlatformID,
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
		PlatformId: PlatformID,
		ExternalId: authorUsername,
		Nickname:   authorNickname,
		AvatarURL:  authorAvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	uniqueID := liveId + "_" + strconv.FormatInt(sessionStartTime, 10)
	if sessionStartTime == 0 {
		uniqueID = liveId + "_" + strconv.FormatInt(now, 10)
	}

	streamResource := model.DownloadResource{
		ContentId:     &content.Id,
		Name:          title,
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

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     uniqueID,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			ConfigJSON:   string(configJSON),
			MetadataJSON: string(metadataJSON),
		},
		Resources: []*adapter.ResourceInfo{{
			DownloadResource: streamResource,
			Endpoints:        []model.DownloadEndpoint{streamEndpoint},
		}},
		ContentDetail: nil,
		Account:       account,
		Content:       content,
	}, nil
}

// getMediaURL returns the combined download URL for a media item (url + urlToken).
func getMediaURL(media wxchannels.ChannelsMediaItem) string {
	return media.URL + media.URLToken
}

func parseChannelsObjectForDownload(contentJSON json.RawMessage) (wxchannels.ChannelsObject, error) {
	var obj wxchannels.ChannelsObject
	if err := json.Unmarshal(contentJSON, &obj); err != nil {
		return obj, err
	}
	if channelsObjectHasDownloadShape(&obj) {
		return obj, nil
	}
	sharedObj, ok, err := sharedFeedProfileToChannelsObject(contentJSON)
	if err != nil {
		return obj, err
	}
	if ok {
		return *sharedObj, nil
	}
	return obj, nil
}

func channelsObjectHasDownloadShape(obj *wxchannels.ChannelsObject) bool {
	if obj == nil {
		return false
	}
	return obj.LiveInfo != nil ||
		obj.ObjectDesc.MediaType != 0 ||
		len(obj.ObjectDesc.Media) > 0 ||
		len(obj.Files) > 0
}

func sharedFeedProfileToChannelsObject(contentJSON json.RawMessage) (*wxchannels.ChannelsObject, bool, error) {
	resp, ok, err := parseSharedFeedProfile(contentJSON, true)
	if err != nil || !ok {
		return nil, ok, err
	}
	feedInfo := resp.Data.Feedinfo
	if feedInfo.MediaType != wxchannels.MediaTypePicture && len(feedInfo.Picinfo) == 0 {
		return nil, false, nil
	}
	if len(feedInfo.Picinfo) == 0 {
		return nil, true, errors.New("分享详情图片类型缺少 picInfo")
	}

	media := make([]wxchannels.ChannelsMediaItem, 0, len(feedInfo.Picinfo))
	for _, pic := range feedInfo.Picinfo {
		picURL := strings.TrimSpace(pic.URL)
		if picURL == "" {
			continue
		}
		media = append(media, wxchannels.ChannelsMediaItem{
			URL:      picURL,
			Width:    pic.Width,
			Height:   pic.Height,
			FileSize: pic.FileSize,
			CoverUrl: strings.TrimSpace(feedInfo.Coverurl),
		})
	}
	if len(media) == 0 {
		return nil, true, errors.New("分享详情图片类型 picInfo 未包含下载地址")
	}

	bgmURL := sharedFeedBGMURL(feedInfo.Bgminfo)
	contactUsername := sharedFeedAuthorID(resp.Data.Authorinfo)
	objectID := sharedFeedObjectID(resp, contentJSON)
	obj := &wxchannels.ChannelsObject{
		ID:            objectID,
		ObjectNonceId: objectID,
		CreateTime:    feedInfo.Createtime,
		Type:          "picture",
		Contact: wxchannels.ChannelsContact{
			Username: contactUsername,
			Nickname: strings.TrimSpace(resp.Data.Authorinfo.Nickname),
			HeadUrl:  strings.TrimSpace(resp.Data.Authorinfo.Headimgurl),
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: strings.TrimSpace(feedInfo.Description),
			MediaType:   wxchannels.MediaTypePicture,
			Media:       media,
			FollowPostInfo: wxchannels.ChannelsFollowPostInfo{
				MusicInfo: wxchannels.ChannelsMusicInfo{
					DocId:             feedInfo.Bgminfo.DocID,
					DocType:           feedInfo.Bgminfo.DocType,
					Name:              feedInfo.Bgminfo.Name,
					Artist:            feedInfo.Bgminfo.Artist,
					MediaStreamingUrl: bgmURL,
				},
			},
		},
		Files: media,
	}
	return obj, true, nil
}

func parseSharedFeedProfile(contentJSON json.RawMessage, allowEnvelope bool) (wxchannels.ChannelsSharedFeedProfileResp, bool, error) {
	var resp wxchannels.ChannelsSharedFeedProfileResp
	directErr := json.Unmarshal(contentJSON, &resp)
	if directErr == nil {
		if len(resp.Data.Feedinfo.Picinfo) > 0 || resp.Data.Feedinfo.MediaType != 0 {
			return resp, true, nil
		}
	}

	if !allowEnvelope {
		return resp, false, directErr
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(contentJSON, &envelope); err != nil {
		return resp, false, err
	}
	if _, hasCode := envelope["code"]; !hasCode {
		return resp, false, nil
	}
	data, hasData := envelope["data"]
	if !hasData || len(data) == 0 {
		return resp, false, nil
	}
	unwrapped, ok, err := parseSharedFeedProfile(data, false)
	if ok || err == nil {
		return unwrapped, ok, err
	}
	if directErr != nil {
		return resp, false, directErr
	}
	return resp, false, err
}

func sharedFeedBGMURL(info wxchannels.SharedFeedBGMInfo) string {
	if u := strings.TrimSpace(info.BGMURL); u != "" {
		return u
	}
	return strings.TrimSpace(info.MediaStreamingURL)
}

func sharedFeedObjectID(resp wxchannels.ChannelsSharedFeedProfileResp, contentJSON json.RawMessage) string {
	candidate := strings.TrimSpace(resp.Data.Sceneinfo.Dynamicexportid)
	if candidate == "" {
		candidate = strings.TrimSpace(resp.Data.Feedinfo.Coverurl)
	}
	if candidate == "" && len(resp.Data.Feedinfo.Picinfo) > 0 {
		candidate = strings.TrimSpace(resp.Data.Feedinfo.Picinfo[0].URL)
	}
	if candidate == "" {
		return "shared_" + hashString(string(contentJSON))[:16]
	}
	safe := safeIdentifier(candidate)
	if safe == "" {
		return "shared_" + hashString(candidate)[:16]
	}
	if len(safe) > 96 {
		return safe[:80] + "_" + hashString(candidate)[:12]
	}
	return safe
}

func sharedFeedAuthorID(author wxchannels.SharedFeedAuthorinfo) string {
	candidate := strings.TrimSpace(author.Headimgurl)
	if candidate == "" {
		candidate = strings.TrimSpace(author.Nickname)
	}
	if candidate == "" {
		return ""
	}
	return "shared_author_" + hashString(candidate)[:16]
}

func safeIdentifier(value string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.TrimSpace(value) {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func hashString(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func endpointProtocol(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err == nil && parsed.Scheme != "" {
		return strings.ToLower(parsed.Scheme)
	}
	return "https"
}

// bgmInfo holds background music download info extracted from a picture feed.
type bgmInfo struct {
	URL  string
	Name string
}

// formatBGM extracts background music info from a picture feed's followPostInfo.
func formatBGM(obj *wxchannels.ChannelsObject) *bgmInfo {
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
func setContentExt(dtr *adapter.DownloadTaskResult, ext any) {
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
