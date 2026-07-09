package instagram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParseProfileURL(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		username string
		ok       bool
	}{
		{name: "profile", raw: "https://www.instagram.com/r_ap82_/", username: "r_ap82_", ok: true},
		{name: "with query", raw: "https://www.instagram.com/r_ap82_/?hl=zh-cn", username: "r_ap82_", ok: true},
		{name: "api", raw: "https://www.instagram.com/api/v1/users/web_profile_info/?username=r_ap82_", username: "r_ap82_", ok: true},
		{name: "bare username", raw: "@r_ap82_", username: "r_ap82_", ok: true},
		{name: "post path", raw: "https://www.instagram.com/p/ABC123/", ok: false},
		{name: "unsupported host", raw: "https://example.com/r_ap82_/", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseProfileURL(tt.raw)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if got.Username != tt.username || got.Canonical != CanonicalProfileURL(tt.username) {
				t.Fatalf("url = %#v", got)
			}
		})
	}
}

func TestParseProfilePageHTMLFixture(t *testing.T) {
	body, err := os.ReadFile("../../../scraper_examples/instagram/260618/author.html")
	if err != nil {
		t.Fatal(err)
	}
	page, err := ParseProfilePageHTML("https://www.instagram.com/r_ap82_/", string(body))
	if err != nil {
		t.Fatalf("ParseProfilePageHTML() error = %v", err)
	}
	if page.URL.Username != "r_ap82_" || page.Profile.Username != "r_ap82_" {
		t.Fatalf("profile username url=%#v profile=%#v", page.URL, page.Profile)
	}
	if page.Profile.ID != "11599648301" {
		t.Fatalf("profile id = %q", page.Profile.ID)
	}
	if page.Profile.FullName != "あまつまりな▽ Marina Amatsu" {
		t.Fatalf("full name = %q", page.Profile.FullName)
	}
	if page.AppID != DefaultAppID {
		t.Fatalf("app id = %q", page.AppID)
	}
	if page.Profile.FollowersCount != 185000 || page.Profile.FollowingCount != 1 || page.Profile.MediaCount != 702 {
		t.Fatalf("counts = followers:%d following:%d media:%d", page.Profile.FollowersCount, page.Profile.FollowingCount, page.Profile.MediaCount)
	}
	if !strings.Contains(page.Profile.Biography, "2.5") || page.Profile.ProfilePicURL == "" {
		t.Fatalf("profile = %#v", page.Profile)
	}
}

func TestFetchUserProfile(t *testing.T) {
	var seenPage bool
	var seenAPI bool
	var seenAppID string
	var seenReferer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/r_ap82_/":
			seenPage = true
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "csrf-test"})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head>
<title>Marina Amatsu (@r_ap82_) · Instagram photos and videos</title>
<meta property="og:title" content="Marina Amatsu (@r_ap82_) · Instagram photos and videos">
<meta property="og:url" content="https://www.instagram.com/r_ap82_/">
<meta property="og:image" content="https://cdn.example.com/avatar.jpg">
<meta name="description" content="185K Followers, 1 Following, 702 Posts - Instagram user Marina Amatsu (@r_ap82_): &quot;bio text&quot;">
<script>{"customHeaders":{"X-IG-App-ID":"936619743392459"},"profile_id":"11599648301"}</script>
</head><body></body></html>`))
		case "/api/v1/users/web_profile_info/":
			seenAPI = true
			seenAppID = r.Header.Get("X-IG-App-ID")
			seenReferer = r.Header.Get("Referer")
			if r.URL.Query().Get("username") != "r_ap82_" {
				t.Fatalf("username query = %q", r.URL.RawQuery)
			}
			if r.Header.Get("X-CSRFToken") != "csrf-test" {
				t.Fatalf("csrf header = %q", r.Header.Get("X-CSRFToken"))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ok",
				"data": map[string]any{
					"user": map[string]any{
						"id":                 "11599648301",
						"username":           "r_ap82_",
						"full_name":          "Marina Amatsu",
						"biography":          "api bio",
						"profile_pic_url_hd": "https://cdn.example.com/avatar-hd.jpg",
						"is_verified":        true,
						"edge_followed_by":   map[string]any{"count": 185123},
						"edge_follow":        map[string]any{"count": 1},
						"edge_owner_to_timeline_media": map[string]any{
							"count": 702,
							"edges": []any{
								map[string]any{
									"node": map[string]any{
										"id":                 "1",
										"shortcode":          "ABC123",
										"display_url":        "https://cdn.example.com/post.jpg",
										"thumbnail_src":      "https://cdn.example.com/thumb.jpg",
										"is_video":           false,
										"taken_at_timestamp": 1781796018,
										"dimensions":         map[string]any{"width": 1080, "height": 1350},
										"edge_media_to_caption": map[string]any{
											"edges": []any{
												map[string]any{"node": map[string]any{"text": "caption"}},
											},
										},
										"edge_liked_by":         map[string]any{"count": 10},
										"edge_media_to_comment": map[string]any{"count": 2},
									},
								},
							},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(server.Client(), "", "", "test-agent")
	client.BaseURL = server.URL
	page, err := client.FetchUserProfile(context.Background(), "https://www.instagram.com/r_ap82_/", ProfileOptions{})
	if err != nil {
		t.Fatalf("FetchUserProfile() error = %v", err)
	}
	if !seenPage || !seenAPI {
		t.Fatalf("seen page=%v api=%v", seenPage, seenAPI)
	}
	if seenAppID != DefaultAppID || seenReferer != "https://www.instagram.com/r_ap82_/" {
		t.Fatalf("headers app=%q referer=%q", seenAppID, seenReferer)
	}
	if page.Profile.FullName != "Marina Amatsu" || page.Profile.FollowersCount != 185123 || !page.Profile.IsVerified {
		t.Fatalf("profile = %#v", page.Profile)
	}
	if len(page.Posts) != 1 || page.Posts[0].Shortcode != "ABC123" || page.Posts[0].Caption != "caption" {
		t.Fatalf("posts = %#v", page.Posts)
	}
}
