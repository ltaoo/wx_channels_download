package x

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestExtractPostSerializedResponse(t *testing.T) {
	const status_id = "2111832941865144809"
	const source_url = "https://x.com/example/status/" + status_id
	const tweet_result = `{"rest_id":"2111832941865144809","result":{
		"__typename":"Tweet","rest_id":"2111832941865144809",
		"details":{"full_text":"Post text","created_at_ms":1792335183000},
		"counts":{"favorite_count":12,"reply_count":3,"retweet_count":4},"views":{"count":"12345"},
		"core":{"user_results":{"result":{"rest_id":"1549552361269510147","core":{"name":"Example","screen_name":"example"},"avatar":{"image_url":"https://pbs.twimg.com/profile_images/example.jpg"}}}},
		"media_entities2":[
			{"id_str":"video","type":"video","media_url_https":"https://pbs.twimg.com/video.jpg","original_info":{"width":1920,"height":1080},"video_info":{"duration_millis":4200,"variants":[{"content_type":"application/x-mpegURL","url":"https://video.twimg.com/example.m3u8"},{"bitrate":256000,"content_type":"video/mp4","url":"https://video.twimg.com/low.mp4"},{"bitrate":832000,"content_type":"video/mp4","url":"https://video.twimg.com/high.mp4"}]}},
			{"id_str":"photo","type":"photo","media_url_https":"https://pbs.twimg.com/photo.jpg","original_info":{"width":1200,"height":800},"ext_alt_text":"Photo description"}
		],
		"quoted_status_result":{"result":{"rest_id":"999","details":{"full_text":"Other post"}}}
	}}`
	const response = `{"data":{"tweet_result_by_rest_id":` + tweet_result + `}}`
	for _, test_case := range []struct {
		name          string
		json_response string
		want_error    bool
	}{
		{name: "focal query", json_response: response},
		{name: "conversation", json_response: `{"data":{"conversation":[{"tweet_results":` + tweet_result + `}]}}`},
		{name: "other post", json_response: strings.ReplaceAll(response, status_id, "123"), want_error: true},
		{name: "missing result", json_response: `{"data":{"tweet_result_by_rest_id":{"rest_id":"2111832941865144809"}}}`, want_error: true},
		{name: "malformed JSON", json_response: `{"data":`, want_error: true},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			if !test_case.want_error && !json.Valid([]byte(test_case.json_response)) {
				t.Fatal("invalid test response")
			}
			source := `<html><head><meta property="og:url" content="` + source_url + `"></head><body><script>` +
				`$R[26]={result:$R[27]={kind:"Result.Ok",value:` + strconv.Quote(test_case.json_response) + `}}` +
				`</script></body></html>`
			document, err := html.Parse(strings.NewReader(source))
			if err != nil {
				t.Fatal(err)
			}
			result, err := extract_post(source, document, status_id, source_url)
			if test_case.want_error {
				if err == nil {
					t.Fatal("expected missing post error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			result, err = normalize_result(result, status_id, source_url)
			if err != nil {
				t.Fatal(err)
			}
			if result.ExternalID != status_id || result.BodyText != "Post text" || result.SourceURL != source_url || result.PublishTime != 1792335183000 {
				t.Fatalf("unexpected post: %+v", result)
			}
			if result.AuthorID != "1549552361269510147" || result.AuthorName != "Example" || result.AuthorUsername != "example" || result.AuthorAvatar != "https://pbs.twimg.com/profile_images/example.jpg" {
				t.Fatalf("unexpected author: %+v", result)
			}
			if result.ViewCount != 12345 || result.LikeCount != 12 || result.CommentCount != 3 || result.ShareCount != 4 {
				t.Fatalf("unexpected counts: %+v", result)
			}
			if len(result.Videos) != 1 || result.Videos[0].URL != "https://video.twimg.com/high.mp4" || result.Videos[0].HLSURL != "https://video.twimg.com/example.m3u8" || result.Videos[0].DurationMillis != 4200 || result.Videos[0].Width != 1920 || result.Videos[0].Height != 1080 {
				t.Fatalf("unexpected videos: %+v", result.Videos)
			}
			if len(result.Images) != 1 || result.Images[0].URL != "https://pbs.twimg.com/photo.jpg" || result.Images[0].AltText != "Photo description" || result.Images[0].Width != 1200 || result.Images[0].Height != 800 {
				t.Fatalf("unexpected images: %+v", result.Images)
			}
		})
	}
}

func TestTSRFindTweetSlotRecords(t *testing.T) {
	source := `$R[0]={rest_id:"123",details:$R[1]={full_text:"Short"}};` +
		`$R[2]={rest_id:"123",details:$R[3]={full_text:"Full"},media_entities2:$R[4]=[{type:"photo"}]}`
	record := tsr_find_tweet(source, "123")
	if tsr_string(tsr_object(record["details"])["full_text"]) != "Full" {
		t.Fatalf("expected richer slot record, got %+v", record)
	}
}
