package wxchannels

import (
	"encoding/json"
	"errors"
	"strconv"

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

// ToAccount converts a ChannelsObject into a model.Account.
func ToAccount(obj *scraper.ChannelsObject) (*model.Account, error) {
	if obj == nil {
		return nil, errors.New("channels object is nil")
	}

	contact, accountUsername := pickAccountContact(obj)

	now := util.NowMillis()
	acc := &model.Account{
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
	c.ContentURL = media.URL + media.URLToken
	c.URL = media.URL + media.URLToken
	c.CoverURL = media.CoverUrl
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
