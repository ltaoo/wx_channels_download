package wxchannels_test

import (
	"encoding/json"
	"testing"

	wxchannels "wx_channel/internal/adapter/wxchannels"
	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
	example "wx_channel/scraper_examples"
)

func TestToAccount_FromLiveFeedJSON(t *testing.T) {
	raw, err := example.Load("wxchannels/260726/live.json")
	require.NoError(t, err)

	var obj wxchannels.ChannelsObject
	require.NoError(t, json.Unmarshal(raw, &obj))

	account, err := wxchannels.ToAccount(&obj)
	require.NoError(t, err)
	require.NotNil(t, account)

	// Live object with anchorContact: pickAccountContact prefers AnchorContact
	assert.Equal(t, "wxchannels:anchor_user_123", account.Id)
	assert.Equal(t, "anchor_user_123", account.ExternalId)
	assert.Equal(t, "主播昵称", account.Nickname)
	assert.Equal(t, "欢迎来到我的直播间", account.Signature)
	assert.Equal(t, "https://wx.qlogo.cn/finderhead/anchor_avatar.jpg", account.AvatarURL)
	assert.Equal(t, "wxchannels", account.PlatformId)
}

func TestToContent_FromLiveFeedJSON(t *testing.T) {
	raw, err := example.Load("wxchannels/260726/live.json")
	require.NoError(t, err)

	var obj wxchannels.ChannelsObject
	require.NoError(t, json.Unmarshal(raw, &obj))

	content, _, err := wxchannels.ToContent(&obj)
	require.NoError(t, err)
	require.NotNil(t, content)

	assert.Equal(t, "wxchannels:14962698468287781449", content.Id)
	assert.Equal(t, "wxchannels", content.PlatformId)
	assert.Equal(t, "14962698468287781449", content.ExternalId)
	assert.Equal(t, "live_nonce_123_0", content.ExternalId2)
	assert.Equal(t, "live", content.Type)
	assert.Equal(t, "直播", content.Title)
	assert.Equal(t, "https://example.com/anchor.jpg", content.CoverURL)

	require.NotNil(t, content.PublishTime)
	assert.Equal(t, int64(1785075244), *content.PublishTime)
}

// Test live feed without anchorContact: should fall back to Contact
func TestToAccount_FromLiveFeed_NoAnchorContact(t *testing.T) {
	payload := `{
		"id": "123",
		"nickname": "顶层昵称",
		"username": "top_user",
		"contact": {
			"username": "contact_user",
			"nickname": "联系人昵称",
			"headUrl": "http://example.com/contact.jpg"
		},
		"liveInfo": {
			"anchorStatusFlag": "123"
		}
	}`

	var obj wxchannels.ChannelsObject
	require.NoError(t, json.Unmarshal([]byte(payload), &obj))

	account, err := wxchannels.ToAccount(&obj)
	require.NoError(t, err)
	require.NotNil(t, account)

	assert.Equal(t, "联系人昵称", account.Nickname)
	assert.Equal(t, "http://example.com/contact.jpg", account.AvatarURL)
}

// Test non-live object with anchorContact: should fall back to Contact (anchorContact only used for live)
func TestToAccount_FromNonLive_WithAnchorContact(t *testing.T) {
	payload := `{
		"id": "123",
		"nickname": "顶层昵称",
		"username": "top_user",
		"contact": {
			"username": "contact_user",
			"nickname": "联系人昵称"
		},
		"anchorContact": {
			"username": "anchor_user",
			"nickname": "主播"
		},
		"objectDesc": {
			"description": "视频",
			"mediaType": 4,
			"media": [
				{
					"url": "https://example.com/video.mp4",
					"thumbUrl": "https://example.com/thumb.jpg",
					"fileSize": 100,
					"videoPlayLen": 10,
					"width": 1920,
					"height": 1080,
					"decodeKey": "key123"
				}
			]
		}
	}`

	var obj wxchannels.ChannelsObject
	require.NoError(t, json.Unmarshal([]byte(payload), &obj))

	account, err := wxchannels.ToAccount(&obj)
	require.NoError(t, err)
	require.NotNil(t, account)

	// No liveInfo, so anchorContact is NOT preferred — falls back to Contact
	assert.Equal(t, "联系人昵称", account.Nickname)
}
