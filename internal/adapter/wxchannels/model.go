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

const platformIDWxChannels = "wx_channels"

// PlatformID is the platform identifier for wechat channels.
const PlatformID = platformIDWxChannels

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(externalID string) string {
	return platformIDWxChannels + ":" + externalID
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(externalID string) string {
	return platformIDWxChannels + ":" + externalID
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
		PlatformId: platformIDWxChannels,
		ExternalId: accountUsername,
		Username:   accountUsername,
		Nickname:   contact.Nickname,
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

// ToContent converts a ChannelsObject into a model.Content.
func ToContent(obj *scraper.ChannelsObject) (*model.Content, error) {
	if obj == nil {
		return nil, errors.New("channels object is nil")
	}
	if obj.ID == "" {
		return nil, errors.New("缺少 id 字段")
	}

	now := util.NowMillis()
	c := &model.Content{
		Id:          BuildContentID(obj.ID),
		PlatformId:  platformIDWxChannels,
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
		c.ContentType = "live"
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
		return c, nil
	}

	// Picture
	if obj.Type == "picture" || obj.ObjectDesc.MediaType == 2 {
		files := obj.Files
		if len(files) == 0 {
			files = obj.ObjectDesc.Media
		}
		if len(files) == 0 {
			return nil, errors.New("picture 类型缺少 files 数据")
		}
		c.ContentType = "picture"
		c.Title = obj.ObjectDesc.Description
		c.Description = obj.ObjectDesc.Description
		c.CoverURL = files[0].CoverUrl
		if obj.CreateTime > 0 {
			publishTime := int64(obj.CreateTime)
			c.PublishTime = &publishTime
		}

		md, _ := json.Marshal(metadataKV{Key: files[0].DecodeKey})
		c.Metadata = string(md)
		return c, nil
	}

	// Media (video)
	if obj.ObjectDesc.MediaType == 9 {
		return nil, errors.New("不支持直播回放（mediaType=9）")
	}

	if len(obj.ObjectDesc.Media) == 0 {
		return nil, errors.New("objectDesc.media 为空")
	}
	media := obj.ObjectDesc.Media[0]

	c.ContentType = "video"
	c.Title = obj.ObjectDesc.Description
	c.Description = obj.ObjectDesc.Description
	c.ContentURL = cleanMediaURL(media.URL) + media.URLToken
	c.URL = cleanMediaURL(media.URL) + media.URLToken
	c.CoverURL = media.ThumbUrl
	if c.SourceURL == "" {
		_, contactUsername := pickAccountContact(obj)
		c.SourceURL = BuildJumpURLFromParts(obj.ID, obj.ObjectNonceId, "", contactUsername)
	}
	c.CoverWidth = strconv.Itoa(int(media.Width))
	c.CoverHeight = strconv.Itoa(int(media.Height))
	c.Duration = int64(media.VideoPlayLen)
	c.Size = int64(media.FileSize)
	c.ExternalId3 = media.DecodeKey

	if obj.CreateTime > 0 {
		publishTime := int64(obj.CreateTime)
		c.PublishTime = &publishTime
	}

	md, _ := json.Marshal(metadataKV{Key: media.DecodeKey})
	c.Metadata = string(md)

	return c, nil
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
	return "original"
}

// BuildDownloadURLWithSpec appends the X-snsvideoflag spec parameter to the base ObjectURL.
// Returns the unmodified ObjectURL if spec is empty, "original", or the URL is a zip:// scheme.
func BuildDownloadURLWithSpec(obj *scraper.ChannelsObject, spec string) string {
	baseURL := ObjectURL(obj)
	if spec == "" || spec == "original" || strings.Contains(baseURL, "zip://") {
		return baseURL
	}
	return baseURL + "&X-snsvideoflag=" + spec
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
	if obj.Type == "picture" || obj.ObjectDesc.MediaType == 2 {
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
