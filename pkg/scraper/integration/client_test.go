package drama_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/hongguo"
	"wx_channel/pkg/scraper/internal/extract"
	"wx_channel/pkg/scraper/mgtv"
	"wx_channel/pkg/scraper/vqq"
	"wx_channel/pkg/scraper/youku"
)

func TestFourPlatformsFetchAndResolveMedia(t *testing.T) {
	ts, err := os.ReadFile("testdata/tiny.ts")
	if err != nil {
		t.Fatal(err)
	}
	mp4, err := os.ReadFile("testdata/tiny.mp4")
	if err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"youku", "tencent", "mgtv", "hongguo"} {
		t.Run(platform, func(t *testing.T) {
			var registrations atomic.Int32
			transport := round_trip_func(func(req *http.Request) (*http.Response, error) {
				body, status := "", 200
				switch req.URL.Path {
				case "/eg.js":
				case "/h5/mtop.youku.columbus.gateway.new.execute/1.0/":
					body = `{"ret":["FAIL::optional catalogue unavailable"],"data":{}}`
				case "/ups/get.json":
					if req.Header.Get("Cookie") == "" {
						t.Error("missing official cookie")
					}
					body = fmt.Sprintf(`{"data":{"video":{"encodeid":%q,"title":"测试电影","seconds":2},"stream":[{"m3u8_url":"https://pl-ali.youku.com/index.m3u8","width":32,"height":32,"milliseconds_video":2000}]}}`, test_youku_id)
				case "/getvinfo":
					body = tencent_playback_fixture(req.URL.Query().Get("vid"), "0", 2)
				case "/player/video":
					body = platform_json(mgtv_playback_fixture())
				case "/player/getSource":
					body = platform_json(mgtv_source_fixture())
				case "/high":
					body = `{"status":"ok","info":"https://pcvideo.titan.mgtv.com/index.m3u8"}`
				case "/index.m3u8", "/vod/index.m3u8":
					if req.Header.Get("Cookie") != "" {
						t.Error("account cookie leaked to CDN")
					}
					body = mgtv_playlist_fixture()
				case "/segment.ts", "/vod/segment.ts":
					body = string(ts)
				case "/player/" + test_series:
					body = router_html(test_series, `["`+test_vid+`","7688396347107445823"]`)
				case "/service/2/device_register/":
					registrations.Add(1)
					body = `{"device_id":12345,"install_id":67890}`
				case "/novel/player/multi_video_model/v1/":
					body = model_response(requested_vid(t, req))
				case "/" + test_vid + ".mp4", "/7688396347107445823.mp4":
					if req.Header.Get("Cookie") != "" {
						t.Error("account cookie leaked to Hongguo CDN")
					}
					body = string(mp4)
				default:
					if req.URL.Hostname() == "v.youku.com" || req.URL.Hostname() == "v.qq.com" || req.URL.Hostname() == "www.mgtv.com" {
						kind := "Movie"
						if platform == "mgtv" {
							kind = "TVSeries"
						}
						body = `<script type="application/ld+json">{"@type":"` + kind + `","name":"测试作品","description":"作品简介","datePublished":"2026-01-02","image":"https://img.example/cover.jpg"}</script>`
					} else {
						t.Errorf("unexpected fixture request %s", req.URL)
						status = 404
					}
				}
				response := &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header), Request: req}
				if req.URL.Path == "/eg.js" {
					response.Header.Set("ETag", `"synthetic-utid"`)
				}
				return response, nil
			})
			client := platform_client(platform, "session=fixture")
			client.SetHTTPClient(&http.Client{Transport: transport})
			page := map[string]string{"youku": "https://v.youku.com/v_show/id_" + test_youku_id + ".html", "tencent": "https://v.qq.com/x/page/" + test_tencent_first + ".html", "mgtv": test_mgtv_page, "hongguo": site + "/player/" + test_series}[platform]
			result, err := client.FetchContext(context.Background(), page)
			if err != nil {
				t.Fatal(err)
			}
			if result.Episodes[0].Duration != 2 || result.Episodes[0].Width != 32 {
				t.Fatalf("missing media metadata: %+v", result)
			}
			if platform == "hongguo" {
				if registrations.Load() != 1 || len(result.Episodes) != 2 || result.Profile.EpisodeCount != 2 {
					t.Fatalf("eager series playback requests: %+v", result)
				}
			} else if result.Profile.Description != "作品简介" || result.Profile.ReleaseDate != "2026-01-02" {
				t.Fatalf("lost movie/show metadata: %+v", result.Profile)
			}
			episode := result.Episodes[len(result.Episodes)-1]
			source, err := client.ResolveContext(context.Background(), episode.SourceURL, episode)
			if err != nil {
				t.Fatal(err)
			}
			if source.CheckMediaAddress == nil || source.CheckMediaAddress("https://evil.example/resource") == nil {
				t.Fatal("resolved playback lost its platform address policy")
			}
			if source.Info.Duration != 2 || source.Info.Width != 32 || (source.Playlist == "" && source.Info.Address == "") {
				t.Fatalf("missing resolved media source: %+v", source)
			}
			if platform == "hongguo" && registrations.Load() != 2 {
				t.Fatal("wrong lazy episode resolution")
			}
		})
	}
}

func TestCancelledFetchAndMismatchedResolution(t *testing.T) {
	client := hongguo.NewClient("")
	client.SetHTTPClient(&http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) { return nil, req.Context().Err() })})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.FetchContext(ctx, site+"/player/"+test_series); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	if _, err := client.ResolveContext(context.Background(), site+"/player/"+test_series+"/"+test_vid, scraper.Episode{ID: "9999"}); err == nil {
		t.Fatal("downloaded a different episode")
	}
}

type test_source struct {
	fetch           func(context.Context, string) (*extract.Result, error)
	resolve         func(context.Context, string, scraper.Episode) (scraper.Video, error)
	set_http_client func(*http.Client)
}

func (c *test_source) FetchContext(ctx context.Context, raw_url string) (*extract.Result, error) {
	return c.fetch(ctx, raw_url)
}
func (c *test_source) ResolveContext(ctx context.Context, source_url string, episode scraper.Episode) (scraper.Video, error) {
	return c.resolve(ctx, source_url, episode)
}
func (c *test_source) SetHTTPClient(client *http.Client) {
	c.set_http_client(client)
}
func platform_client(platform, cookie string) *test_source {
	if platform == "youku" {
		client := youku.NewClient(cookie)
		return &test_source{set_http_client: client.SetHTTPClient, resolve: client.ResolveContext, fetch: func(ctx context.Context, raw_url string) (*extract.Result, error) {
			result, err := client.FetchContext(ctx, raw_url)
			return (*extract.Result)(result), err
		}}
	}
	if platform == "tencent" {
		client := vqq.NewClient(cookie)
		return &test_source{set_http_client: client.SetHTTPClient, resolve: client.ResolveContext, fetch: func(ctx context.Context, raw_url string) (*extract.Result, error) {
			result, err := client.FetchContext(ctx, raw_url)
			return (*extract.Result)(result), err
		}}
	}
	if platform == "mgtv" {
		client := mgtv.NewClient(cookie)
		return &test_source{set_http_client: client.SetHTTPClient, resolve: client.ResolveContext, fetch: func(ctx context.Context, raw_url string) (*extract.Result, error) {
			result, err := client.FetchContext(ctx, raw_url)
			return (*extract.Result)(result), err
		}}
	}
	if platform == "hongguo" {
		client := hongguo.NewClient(cookie)
		return &test_source{set_http_client: client.SetHTTPClient, resolve: client.ResolveContext, fetch: func(ctx context.Context, raw_url string) (*extract.Result, error) {
			result, err := client.FetchContext(ctx, raw_url)
			return (*extract.Result)(result), err
		}}
	}
	panic(platform)
}
