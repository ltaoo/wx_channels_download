package douyinadapter

import (
	"encoding/json"
	"testing"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/douyin"
)

func TestVideoVariantsReachDetailsAndDownloads(t *testing.T) {
	item := json.RawMessage(`{
		"aweme_id":"123", "desc":"test video", "video":{
			"width":1080, "height":1920,
			"play_addr":{"url_list":["https://media.example/playwm/1080.mp4"]},
			"bit_rate":[
				{"gear_name":"normal_720","bit_rate":1000000,"play_addr":{"width":720,"height":1280,"data_size":100,"url_list":[" ","https://media.example/playwm/720.mp4"]}},
				{"gear_name":"normal_1080","bit_rate":2000000,"is_h265":1,"play_addr":{"width":1080,"height":1920,"data_size":200,"url_list":["https://media.example/play/1080.mp4"]}},
				{"gear_name":"duplicate","play_addr":{"url_list":["https://media.example/play/720.mp4"]}},
				{"gear_name":"unavailable","play_addr":{"url_list":[" "]}}
			]
		}
	}`)
	pc_json, err := json.Marshal(map[string]any{"aweme_detail": item})
	if err != nil {
		t.Fatal(err)
	}
	mobile_json, err := json.Marshal(map[string]any{"loaderData": map[string]any{
		douyin_mobile_video_page_key: map[string]any{"videoInfoRes": map[string]any{"item_list": []json.RawMessage{item}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var web_response douyin.DouyinWebVideoProfileResp
	if err := json.Unmarshal(pc_json, &web_response); err != nil {
		t.Fatal(err)
	}
	web_info := &douyin.VideoInfo{
		VideoID: "123", Title: "test video", URL: "https://media.example/play/1080.mp4",
		Video: &web_response.AwemeDetail.Video,
	}
	web_json, err := json.Marshal(web_info)
	if err != nil {
		t.Fatal(err)
	}
	for _, test_case := range []struct {
		name string
		data any
		raw  json.RawMessage
	}{
		{"PC detail", pc_json, pc_json},
		{"mobile router", mobile_json, mobile_json},
		{"web result", web_info, web_json},
		{"serialized web result", web_json, web_json},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			h := &handler{}
			details, err := h.ToContentDetails(test_case.data)
			if err != nil {
				t.Fatal(err)
			}
			video := details[0].Data.(*model.ContentVideo)
			if len(video.Variants) != 2 {
				t.Fatalf("expected two usable variants, got %+v", video.Variants)
			}
			low, high := video.Variants[0], video.Variants[1]
			if low.Quality != "720p" || high.Quality != "1080p" || *low.Width != 720 || *high.Width != 1080 || *low.Bitrate != 1000 || high.Codec != "h265" {
				t.Fatalf("incorrect variant metadata: %+v, %+v", low, high)
			}
			if low.IsDefault != 0 || high.IsDefault != 1 || low.HasAudio != 1 || low.HasVideo != 1 || low.StreamType != model.ContentVideoVariantStreamTypeProgressive {
				t.Fatal("variants must preserve the default and describe muxed video/audio")
			}
			preview, err := h.BuildDownloadTaskFromFetch(test_case.data, json.RawMessage(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			if preview.Resources[0].Endpoints[0].URL != high.URL || preview.Resources[0].ContentAssets[0].AssetKey != high.VariantKey {
				t.Fatal("default download must reference the default variant")
			}
			for _, selection := range []map[string]string{
				{"video_variant_key": low.VariantKey},
				{"video_variant_spec": low.Spec},
				{"spec": low.Spec},
			} {
				config_json, err := json.Marshal(selection)
				if err != nil {
					t.Fatal(err)
				}
				selected, err := h.BuildDownloadTaskFromFetch(test_case.data, config_json)
				if err != nil {
					t.Fatal(err)
				}
				resource := selected.Resources[0]
				selected_video := selected.ContentDetail.(*model.ContentVideo)
				if resource.Endpoints[0].URL != low.URL || resource.Resource.Size != 100 || resource.ContentAssets[0].AssetKey != low.VariantKey || selected.Content.URL != low.URL || selected_video.Width != 720 || selected_video.Size != 100 {
					t.Fatalf("selected download must use the low-resolution representation: %+v", selected)
				}
				if _, err := h.BuildDownloadTask(test_case.raw, config_json); err != nil {
					t.Fatalf("direct download must support the same selection: %v", err)
				}
			}
			for _, invalid_config := range []string{
				`{"video_variant_key":"missing","video_variant_spec":"normal_720"}`,
				`{"video_variant_spec":"missing"}`,
			} {
				if _, err := h.BuildDownloadTaskFromFetch(test_case.data, json.RawMessage(invalid_config)); err == nil {
					t.Fatal("invalid selection must fail instead of downloading the default")
				}
			}
		})
	}
}

func TestVideoVariantFallbacks(t *testing.T) {
	for _, test_case := range []struct {
		name  string
		video string
		count int
		url   string
	}{
		{"legacy single URL", `{"play_addr":{"url_list":["https://media.example/default.mp4"]}}`, 1, "https://media.example/default.mp4"},
		{"bitrate without default URL", `{"bit_rate":[{"play_addr":{"url_list":["https://media.example/720.mp4"]}}]}`, 1, "https://media.example/720.mp4"},
		{"separate default URL", `{"play_addr":{"url_list":["https://media.example/default.mp4"]},"bit_rate":[{"play_addr":{"url_list":["https://media.example/720.mp4"]}}]}`, 2, "https://media.example/default.mp4"},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			payload := json.RawMessage(`{"aweme_detail":{"aweme_id":"123","video":` + test_case.video + `}}`)
			preview, err := (&handler{}).BuildDownloadTaskFromFetch(payload, json.RawMessage(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			video := preview.ContentDetail.(*model.ContentVideo)
			if len(video.Variants) != test_case.count || preview.Resources[0].Endpoints[0].URL != test_case.url {
				t.Fatalf("unexpected fallback: %+v", video)
			}
		})
	}
}
