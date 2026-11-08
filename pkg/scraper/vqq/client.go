// Package vqq extracts Tencent Video metadata and playback sources.
package vqq

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/internal/extract"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

// PlatformID preserves the identifier used by existing content and download archives.
const PlatformID = "tencent"

var tencent_cover_pattern = regexp.MustCompile(`^/x/cover/([A-Za-z0-9_]{5,64})(?:/([A-Za-z0-9_]{5,64}))?\.html$`)
var tencent_page_pattern = regexp.MustCompile(`^/x/page/([A-Za-z0-9_]{5,64})\.html$`)
var tencent_detail_pattern = regexp.MustCompile(`^/detail/[A-Za-z0-9]/([A-Za-z0-9_]{5,64})\.html$`)
var tencent_id_pattern = regexp.MustCompile(`^[A-Za-z0-9_]{5,64}$`)

type backend struct {
	*sourceutil.RequestClient
	small_ranges bool
}
type online_episode struct {
	ID        string
	Order     int
	Duration  float64
	Title     string
	CoverURL  string
	LongTitle string
	Badge     string
}

var source_policy = sourceutil.Policy{Metadata: []string{"v.qq.com", "video.qq.com"}, Media: []string{"qq.com", "gtimg.com", "smtcdns.com"}}

func new_backend(cookie string) extract.Backend {
	return &backend{RequestClient: &sourceutil.RequestClient{Policy: source_policy, Cookie: cookie}}
}

// FetchResult contains extracted source metadata, before downloader adaptation.
type FetchResult extract.Result

// Client retrieves native platform metadata and playback sources.
type Client struct{ extractor *extract.Client }

func NewClient(cookie string) *Client {
	return &Client{extractor: extract.NewClient(PlatformID, cookie, source_policy, new_backend)}
}
func (c *Client) SetHTTPClient(http_client *http.Client) { c.extractor.SetHTTPClient(http_client) }
func (c *Client) Fetch(raw_url string) (*FetchResult, error) {
	return c.FetchContext(context.Background(), raw_url)
}
func (c *Client) FetchContext(ctx context.Context, raw_url string) (*FetchResult, error) {
	result, err := c.extractor.FetchContext(ctx, raw_url)
	return (*FetchResult)(result), err
}

// FetchDetailsContext reads cover catalogue metadata independently of playback
// availability. Standalone /x/page URLs retain playback-based metadata fetching.
func (c *Client) FetchDetailsContext(ctx context.Context, raw_url string) (*FetchResult, error) {
	_, cover_id, _, err := ParseURL(raw_url)
	if err != nil {
		return nil, err
	}
	if cover_id == "" {
		return c.FetchContext(ctx, raw_url)
	}
	result, err := c.extractor.FetchMetadataContext(ctx, raw_url)
	return (*FetchResult)(result), err
}

func (c *Client) FetchDetails(raw_url string) (*FetchResult, error) {
	return c.FetchDetailsContext(context.Background(), raw_url)
}
func (c *Client) ResolveContext(ctx context.Context, source_url string, episode scraper.Episode) (scraper.Video, error) {
	return c.extractor.ResolveContext(ctx, source_url, episode)
}
func ParseURL(raw_url string) (string, string, string, error) {
	link, err := extract.ParsePageURL(raw_url, "v.qq.com")
	if err != nil {
		return "", "", "", err
	}
	if match := tencent_cover_pattern.FindStringSubmatch(link.Path); match != nil {
		link.Scheme, link.Host, link.RawQuery, link.Fragment = "https", "v.qq.com", "", ""
		return link.String(), match[1], match[2], nil
	}
	if match := tencent_page_pattern.FindStringSubmatch(link.Path); match != nil {
		return "https://v.qq.com/x/page/" + match[1] + ".html", "", match[1], nil
	}
	if match := tencent_detail_pattern.FindStringSubmatch(link.Path); match != nil {
		return "https://v.qq.com/x/cover/" + match[1] + ".html", match[1], "", nil
	}
	return "", "", "", errors.New("腾讯视频需要 /x/cover/、/x/page/ 或 /detail/ 官网链接")
}
func (c *backend) ParseURL(raw_url string) (string, string, string, error) { return ParseURL(raw_url) }
func (c *backend) SetHTTPClient(client *http.Client)                       { c.HTTPClient = client }
func (c *backend) List(ctx context.Context, page, cover_id, video_id string) (scraper.Profile, []scraper.Episode, error) {
	c.Referer = page
	profile := scraper.Profile{ID: cover_id, Kind: "video"}
	rows := []online_episode{{ID: video_id}}
	if cover_id != "" {
		var err error
		rows, profile.Title, err = c.tencent_episodes(ctx, cover_id, video_id)
		if err != nil {
			return profile, nil, err
		}
		if c.Profile != nil {
			profile = *c.Profile
		}
	}
	if len(rows) > 1 && profile.Kind != "movie" {
		profile.Kind = "series"
		if profile.EpisodeCount == 0 {
			profile.EpisodeCount = len(rows)
		}
	}
	episodes := make([]scraper.Episode, 0, len(rows))
	for _, row := range rows {
		source_url := "https://v.qq.com/x/page/" + row.ID + ".html"
		if cover_id != "" {
			source_url = "https://v.qq.com/x/cover/" + cover_id + "/" + row.ID + ".html"
		}
		episodes = append(episodes, scraper.Episode{ID: row.ID, Order: row.Order, Duration: row.Duration, Title: row.Title, SourceURL: source_url, CoverURL: row.CoverURL, LongTitle: row.LongTitle, Badge: row.Badge})
	}
	return profile, episodes, nil
}
func (c *backend) Resolve(ctx context.Context, cover_id string, episode scraper.Episode) (scraper.Video, error) {
	c.Referer = episode.SourceURL
	return c.tencent_video(ctx, cover_id, online_episode{ID: episode.ID, Order: episode.Order, Duration: episode.Duration, Title: episode.Title})
}
func min_int64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
