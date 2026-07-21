package wxchannels

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

func TestBuildDownloadTaskWithCoverCreatesMultipleResources(t *testing.T) {
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
			MediaType:   4,
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

	info, _, _, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{
		SavePath:      "/downloads",
		Filename:      "自定义名称",
		DownloadCover: true,
	})
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, model.ResourceTypeCollection, info.Task.ResourceType)
	require.Len(t, info.Resources, 2)
	assert.Equal(t, "video", info.Resources[0].Resource.Kind)
	assert.Equal(t, "自定义名称.mp4", info.Resources[0].Resource.Name)
	assert.Equal(t, "https://video.example.com/video.mp4?token=video", info.Resources[0].Endpoints[0].URL)
	assert.Equal(t, "cover", info.Resources[1].Resource.Kind)
	assert.Equal(t, "自定义名称.jpg", info.Resources[1].Resource.Name)
	assert.Equal(t, "https://image.example.com/cover.jpg?token=cover", info.Resources[1].Endpoints[0].URL)
}
