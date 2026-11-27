package douyin

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAlbumDetailLinks(t *testing.T) {
	const item_id = "7703540661005991538"
	const share_url = "https://www.iesdouyin.com/share/video/" + item_id + "/"
	for _, raw_url := range []string{
		"https://v.douyin.com/Qle8HgQUY8g/",
		"https://www.douyin.com/note/" + item_id,
		"https://www.douyin.com/note/" + item_id + "/?from_aid=6383&share_token=fixture",
		"https://www.douyin.com/video/" + item_id,
		"https://www.iesdouyin.com/share/slides/" + item_id + "/",
	} {
		t.Run(raw_url, func(t *testing.T) {
			page_requests, detail_requests := 0, 0
			client := NewClient("")
			client.pc.http_client = &http.Client{Transport: home_test_transport(func(req *http.Request) (*http.Response, error) {
				status, body := http.StatusOK, ""
				headers := make(http.Header)
				switch {
				case req.URL.Host == "v.douyin.com":
					status = http.StatusFound
					headers.Set("Location", share_url)
				case req.URL.Path == "/web/api/v2/aweme/iteminfo/":
					detail_requests++
					query := req.URL.Query()
					if query.Get("item_ids") != item_id || query.Get("reflow_id") == "" {
						t.Fatalf("invalid iteminfo request: %s", req.URL.Path)
					}
					if strings.Contains(raw_url, "share_token=") && (query.Get("share_token") != "fixture" || query.Get("aid") != "6383") {
						t.Fatal("note share parameters must reach iteminfo")
					}
					body = `{"status_code":0,"item_list":[{"aweme_id":"` + item_id + `","aweme_type":2,"desc":"album","video":{"cover":{}},"images":[{"url_list":["https://media.example/image.jpg"]}]}]}`
				case strings.HasPrefix(req.URL.Path, "/share/") || strings.HasPrefix(req.URL.Path, "/video/"):
					page_requests++
					if (strings.Contains(raw_url, "/note/") || strings.Contains(raw_url, "www.douyin.com/video/")) && (req.URL.Host != "www.iesdouyin.com" || req.URL.Path != "/share/video/"+item_id+"/") {
						t.Fatalf("desktop detail links must use the mobile share page: %s", req.URL)
					}
					body = `<html><div id="douyin_reflow_webId" webId="7712664324800562751"></div><div id="douyin_reflow_token" xsstoken="fixture-token"></div></html>`
				case req.URL.Path == "/web/api/v2/aweme/slidesinfo/":
					body = `{"aweme_details":[{"images":[{"url_list":["https://media.example/image.jpg"]}]}]}`
				default:
					t.Fatalf("unexpected request: %s", req.URL)
				}
				return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})}
			result, err := client.Fetch(raw_url)
			if err != nil {
				t.Fatal(err)
			}
			var detail struct {
				AwemeDetail struct {
					AwemeID string            `json:"aweme_id"`
					Images  []json.RawMessage `json:"images"`
				} `json:"aweme_detail"`
			}
			raw_detail, ok := result.(json.RawMessage)
			if !ok {
				t.Fatalf("expected normalized detail JSON, got %T", result)
			}
			if err := json.Unmarshal(raw_detail, &detail); err != nil {
				t.Fatal(err)
			}
			if page_requests != 1 || detail_requests != 1 || detail.AwemeDetail.AwemeID != item_id || len(detail.AwemeDetail.Images) != 1 {
				t.Fatalf("album detail lost: pages=%d details=%d result=%s", page_requests, detail_requests, raw_detail)
			}
		})
	}
}

func TestWebExtractNoteID(t *testing.T) {
	for _, raw_url := range []string{
		"https://www.douyin.com/note/7703540661005991538",
		"分享图集 https://www.douyin.com/note/7703540661005991538/?from=share",
	} {
		item_id, err := NewDouyinWebClient("").ExtraVideoId(raw_url)
		if err != nil || item_id != "7703540661005991538" {
			t.Fatalf("note ID extraction failed: id=%q err=%v", item_id, err)
		}
	}
}
