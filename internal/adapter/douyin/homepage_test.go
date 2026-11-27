package douyinadapter

import (
	"encoding/json"
	"testing"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/douyin"
)

func TestHomeMediaBuildsDownloadPreview(t *testing.T) {
	for _, test_case := range []struct {
		name         string
		media        string
		content_type string
		urls         []string
	}{
		{"video", `"video":{"play_addr":{"url_list":["https://media.example/123.mp4"]}}`, "video", []string{"https://media.example/123.mp4"}},
		{"album without video", `"aweme_type":68,"images":[{"url_list":["https://media.example/1.jpg"]},{"url_list":["https://media.example/2.jpg"]}]`, "album", []string{"https://media.example/1.jpg", "https://media.example/2.jpg"}},
		{"album with live photo and music", `"aweme_type":68,"images":[{"url_list":["https://media.example/1.jpg"]},{"url_list":["https://media.example/2.jpg"],"video":{"play_addr":{"url_list":["https://media.example/live.mp4"]}}}],"video":{"play_addr":{"uri":"https://media.example/music.mp3"}}`, "album", []string{"https://media.example/1.jpg", "https://media.example/2.jpg", "https://media.example/live.mp4", "https://media.example/music.mp3"}},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			raw := json.RawMessage(`{"aweme_id":"123","desc":"fixture work","author":{"sec_uid":"owner","nickname":"fixture author"},` + test_case.media + `}`)
			item_type := "video"
			if test_case.content_type == "album" {
				item_type = "note"
			}
			contents := douyin_home_contents_from_items([]douyin.HomeContent{{ID: "123", Type: item_type, Raw: raw}}, &model.Account{ExternalId: "owner"})
			if len(contents) != 1 || contents[0].Metadata != string(raw) || contents[0].Type != test_case.content_type {
				t.Fatal("list content must retain its media data and type")
			}
			var item json.RawMessage
			if err := json.Unmarshal([]byte(contents[0].Metadata), &item); err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(map[string]any{"aweme_detail": item})
			if err != nil {
				t.Fatal(err)
			}
			result, err := (&handler{}).BuildDownloadTaskFromFetch(payload, json.RawMessage(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			if result.Content.Id != "douyin:123" || result.Content.Type != test_case.content_type || result.Account.ExternalId != "owner" || len(result.Resources) != len(test_case.urls) {
				t.Fatalf("unexpected download preview: %+v", result)
			}
			for index, resource := range result.Resources {
				if len(resource.Endpoints) != 1 || resource.Endpoints[0].URL != test_case.urls[index] {
					t.Fatalf("resource %d must retain its media URL", index)
				}
			}
		})
	}
}
