package x

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		{name: "x profile", raw: "https://x.com/Barret_China", username: "Barret_China", ok: true},
		{name: "twitter profile", raw: "https://twitter.com/Barret_China", username: "Barret_China", ok: true},
		{name: "bare username", raw: "@Barret_China", username: "Barret_China", ok: true},
		{name: "status URL", raw: "https://x.com/Barret_China/status/2067997733605331174", ok: false},
		{name: "reserved", raw: "https://x.com/home", ok: false},
		{name: "unsupported host", raw: "https://example.com/Barret_China", ok: false},
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

func TestParseProfilePageHTML(t *testing.T) {
	page, err := ParseProfilePageHTML("https://x.com/Barret_China", `<!doctype html><html><head>
<title>Barret (@Barret_China) / X</title>
<meta property="og:title" content="Barret (@Barret_China) on X">
<meta property="og:url" content="https://x.com/Barret_China">
<meta name="description" content="profile bio">
<meta property="og:image" content="https://pbs.twimg.com/profile_images/avatar_200x200.jpg">
<link rel="preload" as="image" href="https://pbs.twimg.com/profile_banners/272736093/1692884927">
<script>{"followers_count":81688,"friends_count":403,"statuses_count":2471,"media_count":827}</script>
</head><body></body></html>`)
	if err != nil {
		t.Fatalf("ParseProfilePageHTML() error = %v", err)
	}
	if page.Profile.ID != "272736093" || page.Profile.Username != "Barret_China" || page.Profile.Name != "Barret" {
		t.Fatalf("profile = %#v", page.Profile)
	}
	if page.Profile.FollowersCount != 81688 || page.Profile.FollowingCount != 403 || page.Profile.StatusesCount != 2471 {
		t.Fatalf("counts = %#v", page.Profile)
	}
	if page.Profile.BannerURL == "" || page.Profile.AvatarURL == "" {
		t.Fatalf("profile images = %#v", page.Profile)
	}
}

func TestParseUserTweetsResponse(t *testing.T) {
	decoded, err := ParseUserTweetsResponse([]byte(testTimelineJSON()))
	if err != nil {
		t.Fatalf("ParseUserTweetsResponse() error = %v", err)
	}
	profile := decoded.UserProfile()
	if profile.ID != "272736093" || profile.Username != "Barret_China" || profile.FollowersCount != 81688 {
		t.Fatalf("profile = %#v", profile)
	}
	posts := decoded.Posts()
	if len(posts) != 1 {
		t.Fatalf("posts len = %d", len(posts))
	}
	post := posts[0]
	if post.ID != "2067997733605331174" || !strings.Contains(post.Text, "long note text") {
		t.Fatalf("post = %#v", post)
	}
	if post.URL != "https://x.com/Barret_China/status/2067997733605331174" {
		t.Fatalf("post url = %q", post.URL)
	}
	if len(post.ImageURLs) != 1 || post.ImageURLs[0] != "https://pbs.twimg.com/media/post.jpg" {
		t.Fatalf("images = %#v", post.ImageURLs)
	}
	if post.FavoriteCount != 20 || post.ViewCount != 1234 {
		t.Fatalf("counts = %#v", post)
	}
	_, bottom := decoded.Cursors()
	if bottom != "bottom-cursor" {
		t.Fatalf("bottom cursor = %q", bottom)
	}
}

func TestFetchUserTimeline(t *testing.T) {
	var seenGuestActivate bool
	var seenTweets bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Barret_China":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.SetCookie(w, &http.Cookie{Name: "guest_id", Value: "v1:test"})
			_, _ = w.Write([]byte(`<!doctype html><html><head>
<meta property="og:title" content="Barret (@Barret_China) on X">
<meta property="og:url" content="https://x.com/Barret_China">
<meta name="description" content="profile bio">
<meta property="og:image" content="https://pbs.twimg.com/profile_images/avatar_200x200.jpg">
<link rel="preload" as="image" href="https://pbs.twimg.com/profile_banners/272736093/1692884927">
</head><body></body></html>`))
		case "/1.1/guest/activate.json":
			seenGuestActivate = true
			if r.Header.Get("Authorization") == "" {
				t.Fatal("missing authorization on guest activate")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"guest_token":"guest-test"}`))
		case "/i/api/graphql/RyDU3I9VJtPF-Pnl6vrRlw/UserTweets":
			seenTweets = true
			if r.Header.Get("X-Guest-Token") != "guest-test" {
				t.Fatalf("guest token header = %q", r.Header.Get("X-Guest-Token"))
			}
			var variables map[string]any
			if err := json.Unmarshal([]byte(r.URL.Query().Get("variables")), &variables); err != nil {
				t.Fatalf("variables json: %v", err)
			}
			if variables["userId"] != "272736093" || variables["count"] != float64(7) {
				t.Fatalf("variables = %#v", variables)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(testTimelineJSON()))
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	defer server.Close()

	client := NewClientWithOptions(server.Client(), "", "", "", "", "test-agent")
	client.BaseURL = server.URL
	client.GuestActivateURL = server.URL + "/1.1/guest/activate.json"
	page, err := client.FetchUserTimeline(context.Background(), "https://x.com/Barret_China", TimelineOptions{Count: 7})
	if err != nil {
		t.Fatalf("FetchUserTimeline() error = %v", err)
	}
	if !seenGuestActivate || !seenTweets {
		t.Fatalf("seen guest=%v tweets=%v", seenGuestActivate, seenTweets)
	}
	if page.Profile.ID != "272736093" || len(page.Posts) != 1 || page.Posts[0].ID != "2067997733605331174" {
		t.Fatalf("page = %#v", page)
	}
}

func testTimelineJSON() string {
	return `{
  "data": {
    "user": {
      "result": {
        "__typename": "User",
        "timeline": {
          "timeline": {
            "instructions": [
              {
                "type": "TimelineAddEntries",
                "entries": [
                  {
                    "entryId": "tweet-2067997733605331174",
                    "content": {
                      "entryType": "TimelineTimelineItem",
                      "itemContent": {
                        "itemType": "TimelineTweet",
                        "tweet_results": {
                          "result": {
                            "__typename": "Tweet",
                            "rest_id": "2067997733605331174",
                            "core": {
                              "user_results": {
                                "result": {
                                  "__typename": "User",
                                  "rest_id": "272736093",
                                  "is_blue_verified": true,
                                  "avatar": {"image_url": "https://pbs.twimg.com/profile_images/avatar_normal.jpg"},
                                  "core": {
                                    "created_at": "Sun Mar 27 02:48:44 +0000 2011",
                                    "name": "Barret",
                                    "screen_name": "Barret_China"
                                  },
                                  "legacy": {
                                    "description": "profile bio",
                                    "followers_count": 81688,
                                    "friends_count": 403,
                                    "statuses_count": 2471,
                                    "media_count": 827,
                                    "profile_banner_url": "https://pbs.twimg.com/profile_banners/272736093/1692884927"
                                  },
                                  "location": {"location": "Hangzhou"},
                                  "verification": {"verified": false}
                                }
                              }
                            },
                            "legacy": {
                              "created_at": "Fri Jun 19 15:47:35 +0000 2026",
                              "full_text": "short text https://t.co/a",
                              "id_str": "2067997733605331174",
                              "lang": "en",
                              "favorite_count": 20,
                              "reply_count": 1,
                              "retweet_count": 2,
                              "quote_count": 3,
                              "bookmark_count": 4,
                              "user_id_str": "272736093",
                              "entities": {
                                "urls": [
                                  {"url": "https://t.co/a", "expanded_url": "https://example.com/full"}
                                ],
                                "media": [
                                  {"type": "photo", "media_url_https": "https://pbs.twimg.com/media/post.jpg"}
                                ]
                              },
                              "extended_entities": {
                                "media": [
                                  {"type": "photo", "media_url_https": "https://pbs.twimg.com/media/post.jpg"}
                                ]
                              }
                            },
                            "note_tweet": {
                              "note_tweet_results": {
                                "result": {"text": "long note text https://t.co/a"}
                              }
                            },
                            "views": {"count": "1234", "state": "EnabledWithCount"}
                          }
                        }
                      }
                    }
                  },
                  {
                    "entryId": "cursor-bottom",
                    "content": {
                      "entryType": "TimelineTimelineCursor",
                      "value": "bottom-cursor",
                      "cursorType": "Bottom"
                    }
                  }
                ]
              }
            ]
          }
        }
      }
    }
  }
}`
}
