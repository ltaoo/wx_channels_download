package wxchannels

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/util"
)

const wxchannels = "wxchannels"

// PlatformID is the platform identifier for wechat channels.
const PlatformID = wxchannels

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(externalID string) string {
	return wxchannels + ":" + externalID
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(externalID string) string {
	return wxchannels + ":" + externalID
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

// ToAccount converts a ChannelsObject into a model.Account.
func ToAccount(obj *scraper.ChannelsObject) (*model.Account, error) {
	if obj == nil {
		return nil, errors.New("channels object is nil")
	}

	contact, accountUsername := pickAccountContact(obj)

	now := util.NowMillis()
	acc := &model.Account{
		Id:         BuildAccountID(accountUsername),
		PlatformId: wxchannels,
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
func pickAccountContact(obj *scraper.ChannelsObject) (scraper.ChannelsContact, string) {
	if obj.LiveInfo != nil && obj.AnchorContact != nil {
		return *obj.AnchorContact, obj.AnchorContact.Username
	}
	return obj.Contact, obj.Contact.Username
}

// ToContent converts a ChannelsObject into a slim model.Content and an extension struct.
// Returns (content, extension, error). extension is nil for live content, *ContentVideo for video,
// []*ContentImage for single picture, *ContentAlbum for album.
func ToContent(obj *scraper.ChannelsObject) (*model.Content, any, error) {
	if obj == nil {
		return nil, nil, errors.New("channels object is nil")
	}
	if obj.ID == "" {
		return nil, nil, errors.New("缺少 id 字段")
	}

	now := util.NowMillis()
	c := &model.Content{
		Id:          BuildContentID(obj.ID),
		PlatformId:  wxchannels,
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
	if obj.ObjectDesc.MediaType == scraper.MediaTypePicture {
		files := obj.Files
		if len(files) == 0 {
			files = obj.ObjectDesc.Media
		}
		if len(files) == 0 {
			return nil, nil, errors.New("picture 类型缺少 files 数据")
		}
		c.Type = "album"
		c.Title = obj.ObjectDesc.Description
		c.Description = obj.ObjectDesc.Description
		c.CoverURL = files[0].CoverUrl
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
	if obj.ObjectDesc.MediaType == scraper.MediaTypeLive {
		return nil, nil, errors.New("不支持直播回放（mediaType=9）")
	}

	if len(obj.ObjectDesc.Media) == 0 {
		return nil, nil, errors.New("objectDesc.media 为空")
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

// PickSpec returns the first h264 spec's FileFormat from the object, or "original" if none.
func PickSpec(obj *scraper.ChannelsObject) string {
	specs := obj.Spec
	if len(obj.ObjectDesc.Media) > 0 && len(obj.ObjectDesc.Media[0].Spec) > 0 {
		specs = obj.ObjectDesc.Media[0].Spec
	}
	if len(specs) > 0 {
		return specs[0].FileFormat
	}
	return ""
}

// BuildDownloadURLWithSpec returns the download URL for the given spec.
//
//   - If spec is a codec name (e.g. "xWT111"), appends &X-snsvideoflag= to the base URL.
//   - If spec is "" or "original", strips all query params except encfilekey and token,
//     mirroring the JS __wx_channels_download4 original-video logic.
//   - zip:// URLs are returned as-is.
func BuildDownloadURLWithSpec(obj *scraper.ChannelsObject, spec string) string {
	baseURL := ObjectURL(obj)

	// When spec is non-empty: append X-snsvideoflag parameter
	if spec != "" {
		return baseURL + "&X-snsvideoflag=" + spec
	}

	// When spec is empty, download the original video, keeping only encfilekey and token
	if u, err := url.Parse(baseURL); err == nil {
		filekey := u.Query().Get("encfilekey")
		token := u.Query().Get("token")
		if filekey != "" && token != "" {
			clean := &url.URL{
				Scheme: u.Scheme,
				Host:   u.Host,
				Path:   u.Path,
			}
			q := clean.Query()
			q.Set("encfilekey", filekey)
			q.Set("token", token)
			clean.RawQuery = q.Encode()
			return clean.String()
		}
	}

	return baseURL
}

// DecryptKeyInt returns the video decrypt key as int, or 0 on failure.
func DecryptKeyInt(obj *scraper.ChannelsObject) int {
	if len(obj.ObjectDesc.Media) == 0 {
		return 0
	}
	key, err := strconv.Atoi(obj.ObjectDesc.Media[0].DecodeKey)
	if err != nil {
		return 0
	}
	return key
}

// ObjectTitle returns the object title with fallback logic (description → ID → timestamp).
func ObjectTitle(obj *scraper.ChannelsObject) string {
	if obj.LiveInfo != nil {
		return "直播"
	}
	title := strings.TrimSpace(obj.ObjectDesc.Description)
	if title != "" {
		return title
	}
	if strings.TrimSpace(obj.ID) != "" {
		return obj.ID
	}
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// ObjectURL returns the download URL (video = media.URL + URLToken, picture/live returns "").
func ObjectURL(obj *scraper.ChannelsObject) string {
	if obj.LiveInfo != nil {
		return ""
	}
	if obj.Type == "picture" || obj.ObjectDesc.MediaType == scraper.MediaTypePicture {
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
