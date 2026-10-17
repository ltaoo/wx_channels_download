package wxchannelsadapter

import (
	"encoding/json"
	"strings"
	"testing"

	"wx_channel/internal/database/model"
)

func TestBuildDownloadTaskWithStringMediaIntegers(t *testing.T) {
	body := json.RawMessage(`{
		"id":"1234567890",
		"objectNonceId":"nonce",
		"contact":{"username":"test_author","nickname":"Author"},
		"objectDesc":{
			"description":"test video",
			"mediaType":4,
			"media":[{
				"url":"https://example.com/video.mp4",
				"mediaType":"4",
				"videoPlayLen":"120",
				"fileSize":"123456789",
				"width":1920,
				"height":1080,
				"decodeKey":"12345678901234567890"
			}]
		}
	}`)
	for _, payload := range []json.RawMessage{body, json.RawMessage(`{"data":{"object":` + string(body) + `}}`)} {
		info, err := NewChannelsAdapter().BuildDownloadTask(payload, json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		video, ok := info.ContentDetail.(*model.ContentVideo)
		if !ok || video.Duration != 120 || video.Size != 123456789 || video.Width != 1920 {
			t.Fatalf("unexpected video detail: %+v", info.ContentDetail)
		}
		if len(info.Resources) == 0 || len(info.Resources[0].Endpoints) == 0 || !strings.HasPrefix(info.Resources[0].Endpoints[0].URL, "https://example.com/video.mp4") {
			t.Fatalf("missing video download endpoint: %+v", info.Resources)
		}
	}
}
