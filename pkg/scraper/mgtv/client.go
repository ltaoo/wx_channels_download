// Package mgtv extracts MGTV metadata and playback sources.
package mgtv

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/internal/extract"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const PlatformID = "mgtv"

var mgtv_path_pattern = regexp.MustCompile(`^/b/([0-9]{1,20})/([0-9]{1,20})\.html$`)

type backend struct{ *sourceutil.RequestClient }

var source_policy = sourceutil.Policy{Metadata: []string{"pcweb.api.mgtv.com"}, Media: []string{"mgtv.com", "hitv.com", "hunantv.com"}}

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
func (c *Client) ResolveContext(ctx context.Context, source_url string, episode scraper.Episode) (scraper.Video, error) {
	return c.extractor.ResolveContext(ctx, source_url, episode)
}
func ParseURL(raw_url string) (string, string, string, error) {
	link, err := extract.ParsePageURL(raw_url, "www.mgtv.com", "mgtv.com")
	if err != nil {
		return "", "", "", err
	}
	if match := mgtv_path_pattern.FindStringSubmatch(link.Path); match != nil {
		return "https://www.mgtv.com" + link.Path, match[1], match[2], nil
	}
	return "", "", "", errors.New("芒果 TV 需要 /b/专辑编号/视频编号.html 播放页")
}
func (c *backend) ParseURL(raw_url string) (string, string, string, error) { return ParseURL(raw_url) }
func (c *backend) SetHTTPClient(client *http.Client)                       { c.HTTPClient = client }
func (c *backend) List(_ context.Context, page, cover_id, video_id string) (scraper.Profile, []scraper.Episode, error) {
	return scraper.Profile{ID: cover_id, Kind: "video"}, []scraper.Episode{{ID: video_id, SourceURL: page}}, nil
}
func (c *backend) Resolve(ctx context.Context, cover_id string, episode scraper.Episode) (scraper.Video, error) {
	c.Referer = episode.SourceURL
	return c.mgtv_video(ctx, cover_id, episode.ID)
}
