package weibo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseUserURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		uid  string
		ok   bool
	}{
		{name: "user path", raw: "https://weibo.com/u/1926245291", uid: "1926245291", ok: true},
		{name: "numeric path", raw: "https://weibo.com/1926245291", uid: "1926245291", ok: true},
		{name: "mymblog api", raw: "https://weibo.com/ajax/statuses/mymblog?uid=1926245291&page=1&feature=0", uid: "1926245291", ok: true},
		{name: "plain uid", raw: "1926245291", uid: "1926245291", ok: true},
		{name: "unsupported", raw: "https://example.com/u/1926245291", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseUserURL(tt.raw)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if got.UID != tt.uid || got.Canonical != "https://weibo.com/u/"+tt.uid {
				t.Fatalf("url = %#v", got)
			}
		})
	}
}

func TestFetchUserTimeline(t *testing.T) {
	var seenCookie string
	var seenXSRF string
	var seenReferer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ajax/statuses/mymblog" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("uid") != "1926245291" ||
			r.URL.Query().Get("page") != "2" ||
			r.URL.Query().Get("feature") != "0" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		seenCookie = r.Header.Get("Cookie")
		seenXSRF = r.Header.Get("X-XSRF-TOKEN")
		seenReferer = r.Header.Get("Referer")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": 1,
			"data": map[string]any{
				"since_id": "next",
				"total":    1365,
				"list": []any{
					map[string]any{
						"created_at":      "Thu Jun 18 22:03:00 +0800 2026",
						"id":              int64(5311280362561538),
						"idstr":           "5311280362561538",
						"mid":             "5311280362561538",
						"mblogid":         "R4Js8aKn8",
						"text_raw":        "習作 ​​​",
						"source":          "微博网页版",
						"pic_ids":         []string{"pic1"},
						"pic_num":         1,
						"reposts_count":   6,
						"comments_count":  2,
						"attitudes_count": 161,
						"user": map[string]any{
							"id":                int64(1926245291),
							"idstr":             "1926245291",
							"screen_name":       "Krenz",
							"profile_url":       "/u/1926245291",
							"profile_image_url": "https://example.com/avatar.jpg",
							"verified":          true,
						},
						"pic_infos": map[string]any{
							"pic1": map[string]any{
								"original": map[string]any{
									"url":    "https://wx2.sinaimg.cn/orj1080/pic1.jpg",
									"width":  850,
									"height": 945,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClientWithOptions(server.Client(), "XSRF-TOKEN=test-xsrf; SUB=test-sub", "test-agent")
	client.BaseURL = server.URL
	page, err := client.FetchUserTimeline(context.Background(), "https://weibo.com/u/1926245291", TimelineOptions{Page: 2})
	if err != nil {
		t.Fatalf("FetchUserTimeline() error = %v", err)
	}
	if seenCookie != "XSRF-TOKEN=test-xsrf; SUB=test-sub" || seenXSRF != "test-xsrf" {
		t.Fatalf("headers cookie=%q xsrf=%q", seenCookie, seenXSRF)
	}
	if seenReferer != "https://weibo.com/u/1926245291" {
		t.Fatalf("referer = %q", seenReferer)
	}
	if page.URL.UID != "1926245291" || page.SinceID != "next" || page.Total != 1365 {
		t.Fatalf("page = %#v", page)
	}
	if page.User.ScreenName != "Krenz" || page.User.IDStr != "1926245291" {
		t.Fatalf("user = %#v", page.User)
	}
	if len(page.Posts) != 1 {
		t.Fatalf("posts len = %d", len(page.Posts))
	}
	post := page.Posts[0]
	if post.ID != "5311280362561538" ||
		post.URL != "https://weibo.com/1926245291/R4Js8aKn8" ||
		post.CoverURL != "https://wx2.sinaimg.cn/orj1080/pic1.jpg" ||
		post.CreatedTime == 0 ||
		post.RepostsCount != 6 ||
		post.CommentsCount != 2 ||
		post.AttitudesCount != 161 {
		t.Fatalf("post = %#v", post)
	}
}
