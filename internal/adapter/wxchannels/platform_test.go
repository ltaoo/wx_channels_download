package wxchannels

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
)

func TestBuildResourceExtraJSONIncludesFilenameAndDownloadAt(t *testing.T) {
	tests := []struct {
		name         string
		id           string
		title        string
		wantFilename string
	}{
		{name: "title", id: "feed123", title: "视频标题", wantFilename: "视频标题"},
		{name: "id fallback", id: "feed123", wantFilename: "feed123"},
		{name: "current time fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now().Unix()
			raw := buildResourceExtraJSON(tt.id, tt.title, "original", 123, "作者", "")
			after := time.Now().Unix()

			var got map[string]string
			require.NoError(t, json.Unmarshal([]byte(raw), &got))
			downloadAt, err := strconv.ParseInt(got["download_at"], 10, 64)
			require.NoError(t, err)
			if downloadAt < before || downloadAt > after {
				t.Fatalf("download_at = %d, want current Unix time in [%d, %d]", downloadAt, before, after)
			}

			wantFilename := tt.wantFilename
			if wantFilename == "" {
				wantFilename = got["download_at"]
			}
			assert.Equal(t, wantFilename, got["filename"])
		})
	}
}

func TestBuildDownloadTaskVideoCreatesSingleResource(t *testing.T) {
	obj := scraper.ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		Type:          "media",
		Contact: scraper.ChannelsContact{
			Username: "test_user",
			Nickname: "测试用户",
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   scraper.MediaTypeVideo,
			Media: []scraper.ChannelsMediaItem{{
				URL:      "https://video.example.com/video.mp4?token=video",
				CoverUrl: "https://image.example.com/cover.jpg?token=cover",
				ThumbUrl: "https://image.example.com/thumb.jpg",
				FileSize: 1024,
			}},
		},
	}
	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	info, err := (&handler{}).BuildDownloadTask(raw, toConfigJSON(map[string]any{
		"filename": "自定义名称",
	}))
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Len(t, info.Resources, 1)
	assert.Equal(t, "video/mp4", info.Resources[0].DownloadResource.Kind)
	assert.Equal(t, "自定义名称", info.Resources[0].DownloadResource.Name)
	assert.Equal(t, "https://video.example.com/video.mp4?token=video", info.Resources[0].Endpoints[0].URL)
}

// mergedLiveContentJSON simulates what the frontend sends after
// Object.assign({}, joinLiveData, profileData).  The profile (ChannelsObject)
// contributes anchorContact, contact, nickname, username; the joinLive
// response contributes liveSdkInfo, liveInfo, liveDescription.
const mergedLiveContentJSON = `{
	"liveSdkInfo": {
		"liveCdnUrl": "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_live_stream.flv?token=abc123"
	},
	"liveInfo": {
		"liveId": "2078967496773105135",
		"startTime": 1785075244
	},
	"liveDescription": "谁可以无缘无故给我刷个岛",
	"nickname": "小玉来了哦",
	"username": "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder",
	"contact": {
		"username": "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder",
		"nickname": "小玉来了哦",
		"headUrl": "https://example.com/contact_avatar.jpg"
	},
	"anchorContact": {
		"username": "anchor_user",
		"nickname": "主播",
		"headUrl": "https://example.com/anchor_avatar.jpg",
		"coverImgUrl": "https://example.com/live.jpg"
	}
}`

func TestBuildDownloadTask_LiveStream_DetectsJoinLiveContent(t *testing.T) {
	raw := json.RawMessage(mergedLiveContentJSON)
	info, err := (&handler{}).BuildDownloadTask(raw, toConfigJSON(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, info)
	content := info.Content
	account := info.Account
	require.NotNil(t, content)
	require.NotNil(t, account)

	// ---- Verify Content ----
	t.Run("Content", func(t *testing.T) {
		assert.Equal(t, "wxchannels:2078967496773105135", content.Id)
		assert.Equal(t, "wxchannels", content.PlatformId)
		assert.Equal(t, "2078967496773105135", content.ExternalId)
		assert.Equal(t, "live", content.Type)
		assert.Equal(t, "谁可以无缘无故给我刷个岛", content.Title)
		require.NotNil(t, content.PublishTime)
		assert.Equal(t, int64(1785075244), *content.PublishTime)
	})

	// ---- Verify Account (anchorContact preferred for live) ----
	t.Run("Account", func(t *testing.T) {
		assert.Equal(t, "wxchannels:anchor_user", account.Id)
		assert.Equal(t, "anchor_user", account.ExternalId)
		assert.Equal(t, "主播", account.Nickname)
		assert.Equal(t, "https://example.com/anchor_avatar.jpg", account.AvatarURL)
	})

	// ---- Verify DownloadTask ----
	t.Run("DownloadTask", func(t *testing.T) {
		require.NotNil(t, info.Task.ContentId)
		assert.Equal(t, "wxchannels:2078967496773105135", *info.Task.ContentId)
		assert.Equal(t, "谁可以无缘无故给我刷个岛", info.Task.Name)
		assert.Equal(t, model.TaskStatusWaiting, info.Task.Status)
		var meta map[string]any
		require.NoError(t, json.Unmarshal([]byte(info.Task.MetadataJSON), &meta))
		assert.Equal(t, "wxchannels", meta["platform"])
		assert.Equal(t, "2078967496773105135", meta["id"])
		assert.Equal(t, "live", meta["content_type"])
		assert.Equal(t, "主播", meta["author"])
	})

	// ---- Verify DownloadResource (STREAM type) ----
	t.Run("DownloadResource", func(t *testing.T) {
		r := info.Resources[0].DownloadResource
		assert.Equal(t, "谁可以无缘无故给我刷个岛.mkv", r.Name)
		assert.Equal(t, "video/x-matroska", r.Kind)
		assert.Equal(t, model.ResourceTypeStream, r.Type)
		assert.Equal(t, 10, r.RotateMinutes)
		assert.Equal(t, "2078967496773105135_1785075244", r.UniqueID)
		assert.Equal(t, "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_live_stream.flv?token=abc123", r.StreamURL)
	})

	// ---- Verify DownloadEndpoint ----
	t.Run("DownloadEndpoint", func(t *testing.T) {
		assert.Equal(t, "livestream", info.Resources[0].Endpoints[0].Protocol)
		assert.Equal(t, "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_live_stream.flv?token=abc123", info.Resources[0].Endpoints[0].URL)
		assert.Equal(t, 1, info.Resources[0].Endpoints[0].Enabled)
	})

	// ---- Verify Resources list ----
	require.Len(t, info.Resources, 1)
	require.Len(t, info.Resources[0].Endpoints, 1)
}

func TestBuildDownloadTask_LiveStream_ContactFallback(t *testing.T) {
	// Payload with contact but NO anchorContact — should fall back to contact
	raw := json.RawMessage(`{
		"liveSdkInfo": { "liveCdnUrl": "http://live.example.com/stream.flv" },
		"liveInfo": { "liveId": "abc123", "startTime": 1700000000 },
		"liveDescription": "测试直播",
		"contact": {
			"username": "contact_user",
			"nickname": "联系人昵称",
			"headUrl": "https://example.com/contact.jpg"
		}
	}`)

	info, err := (&handler{}).BuildDownloadTask(raw, toConfigJSON(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, info)
	account := info.Account
	require.NotNil(t, account)

	assert.Equal(t, "wxchannels:contact_user", account.Id)
	assert.Equal(t, "联系人昵称", account.Nickname)
	assert.Equal(t, "https://example.com/contact.jpg", account.AvatarURL)
	assert.Equal(t, "测试直播", info.Task.Name)

	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(info.Task.MetadataJSON), &meta))
	assert.Equal(t, "联系人昵称", meta["author"])
}

func TestBuildDownloadTask_LiveStream_NicknameFallback(t *testing.T) {
	// Payload with NO contact and NO anchorContact — should fall back to top-level fields
	raw := json.RawMessage(`{
		"liveSdkInfo": { "liveCdnUrl": "http://live.example.com/stream.flv" },
		"liveInfo": { "liveId": "simple_live", "startTime": 1700000000 },
		"liveDescription": "极简直播",
		"nickname": "顶层昵称",
		"username": "top_level_user"
	}`)

	info, err := (&handler{}).BuildDownloadTask(raw, toConfigJSON(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, info)
	account := info.Account
	require.NotNil(t, account)

	assert.Equal(t, "wxchannels:top_level_user", account.Id)
	assert.Equal(t, "顶层昵称", account.Nickname)
	assert.Equal(t, "", account.AvatarURL)
	assert.Equal(t, "极简直播", info.Task.Name)

	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(info.Task.MetadataJSON), &meta))
	assert.Equal(t, "顶层昵称", meta["author"])
}

func TestBuildDownloadTask_LiveStream_NoLiveDescription(t *testing.T) {
	// Payload without liveDescription — title should default to "live"
	raw := json.RawMessage(`{
		"liveSdkInfo": { "liveCdnUrl": "http://live.example.com/stream.flv" },
		"liveInfo": { "liveId": "no_desc_live" },
		"nickname": "阿强",
		"username": "aqiang"
	}`)

	info, err := (&handler{}).BuildDownloadTask(raw, toConfigJSON(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, info)
	content := info.Content
	require.NotNil(t, content)

	assert.Equal(t, "直播", content.Title)
	assert.Equal(t, "直播.mkv", info.Resources[0].DownloadResource.Name)
}

func TestBuildDownloadTask_LiveStream_CustomFilename(t *testing.T) {
	raw := json.RawMessage(mergedLiveContentJSON)
	info, err := (&handler{}).BuildDownloadTask(raw, toConfigJSON(map[string]any{
		"filename": "我的直播录制",
	}))
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "我的直播录制", info.Task.Name)
	assert.Equal(t, "我的直播录制.mkv", info.Resources[0].DownloadResource.Name)
}

func TestBuildDownloadTask_LiveStream_NoLiveId(t *testing.T) {
	// Payload without liveInfo — liveId should be empty string
	raw := json.RawMessage(`{
		"liveSdkInfo": { "liveCdnUrl": "http://live.example.com/stream.flv" },
		"liveDescription": "无ID直播",
		"nickname": "测试用户",
		"username": "test_user"
	}`)

	info, err := (&handler{}).BuildDownloadTask(raw, toConfigJSON(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, info)
	content := info.Content
	account := info.Account

	assert.Equal(t, "wxchannels:", content.Id)
	assert.Equal(t, "", content.ExternalId)
	assert.NotEmpty(t, info.Resources[0].DownloadResource.UniqueID)
	assert.Contains(t, info.Resources[0].DownloadResource.UniqueID, "_")
	assert.NotNil(t, account)
}

func TestJoinLivePayload_Parse(t *testing.T) {
	var jl scraper.JoinLivePayload
	require.NoError(t, json.Unmarshal([]byte(mergedLiveContentJSON), &jl))

	// liveSdkInfo
	require.NotNil(t, jl.LiveSdkInfo)
	assert.Equal(t, "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_live_stream.flv?token=abc123", jl.LiveSdkInfo.LiveCdnUrl)

	// liveInfo
	require.NotNil(t, jl.LiveInfo)
	assert.Equal(t, "2078967496773105135", jl.LiveInfo.LiveId)
	assert.Equal(t, 1785075244, jl.LiveInfo.StartTime)

	// liveDescription
	assert.Equal(t, "谁可以无缘无故给我刷个岛", jl.LiveDescription)

	// Top-level fields
	assert.Equal(t, "小玉来了哦", jl.Nickname)
	assert.Equal(t, "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder", jl.Username)

	// anchorContact (from profile)
	require.NotNil(t, jl.AnchorContact)
	assert.Equal(t, "anchor_user", jl.AnchorContact.Username)
	assert.Equal(t, "主播", jl.AnchorContact.Nickname)
	assert.Equal(t, "https://example.com/anchor_avatar.jpg", jl.AnchorContact.HeadUrl)
	assert.Equal(t, "https://example.com/live.jpg", jl.AnchorContact.CoverImgUrl)

	// contact (from profile)
	require.NotNil(t, jl.Contact)
	assert.Equal(t, "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder", jl.Contact.Username)
	assert.Equal(t, "小玉来了哦", jl.Contact.Nickname)
	assert.Equal(t, "https://example.com/contact_avatar.jpg", jl.Contact.HeadUrl)
}

func TestBuildDownloadTask_NotLive_NotJoinLive(t *testing.T) {
	// A regular video feed should NOT be detected as joinLive
	obj := scraper.ChannelsObject{
		ID:   "video_feed_123",
		Type: "media",
		Contact: scraper.ChannelsContact{
			Username: "test_user",
			Nickname: "测试用户",
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   scraper.MediaTypeVideo,
			Media: []scraper.ChannelsMediaItem{{
				URL:      "https://video.example.com/video.mp4",
				CoverUrl: "https://image.example.com/cover.jpg",
				FileSize: 1024,
			}},
		},
	}
	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	info, err := (&handler{}).BuildDownloadTask(raw, toConfigJSON(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, info)
	content := info.Content
	require.NotNil(t, content)

	// Should be video type, NOT live/stream
	assert.Equal(t, "video", content.Type)
	assert.Equal(t, "video/mp4", info.Resources[0].DownloadResource.Kind)
}

func TestJoinLivePayload_Detection_NoLiveSdkInfo(t *testing.T) {
	// JSON that has liveDescription but NO liveSdkInfo — should NOT trigger joinLive path
	raw := json.RawMessage(`{
		"liveDescription": "some text",
		"liveInfo": { "liveId": "123" },
		"nickname": "test",
		"username": "test_user"
	}`)

	// This should fail since there's no liveSdkInfo (falls through to ChannelsObject,
	// which will fail because it's not valid ChannelsObject format)
	_, err := (&handler{}).BuildDownloadTask(raw, toConfigJSON(map[string]any{}))
	assert.Error(t, err)
}

func toConfigJSON(cfg map[string]any) json.RawMessage {
	data, _ := json.Marshal(cfg)
	return json.RawMessage(data)
}

func TestBuildDownloadTaskUniqueID(t *testing.T) {
	const externalID = "feed123"
	xWT111 := "xWT111"
	xWT200 := "xWT200"

	tests := []struct {
		name   string
		config map[string]any
		want   string
	}{
		{
			name:   "cover only (suffix .jpg)",
			config: map[string]any{"suffix": ".jpg"},
			want:   "feed123_cover",
		},
		{
			name:   "mp3 conversion (suffix .mp3)",
			config: map[string]any{"suffix": ".mp3"},
			want:   "feed123_mp3",
		},
		{
			name:   "mp3 with spec",
			config: map[string]any{"suffix": ".mp3", "spec": xWT111},
			want:   "feed123_xWT111_mp3",
		},
		{
			name:   "video with explicit spec",
			config: map[string]any{"spec": xWT111},
			want:   "feed123_xWT111",
		},
		{
			name:   "video default (no spec, no suffix)",
			config: map[string]any{},
			want:   "feed123",
		},
		{
			name:   "different spec produces different ID",
			config: map[string]any{"spec": xWT200},
			want:   "feed123_xWT200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDownloadTaskUniqueID(externalID, tt.config)
			assert.Equal(t, tt.want, got)
		})
	}
}
