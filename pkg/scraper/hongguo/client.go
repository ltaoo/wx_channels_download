// Package hongguo extracts Hongguo metadata and playback sources.
package hongguo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/internal/extract"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const PlatformID = "hongguo"

type backend struct{ api *api_client }

var source_policy = sourceutil.Policy{RequireHTTPS: true, Metadata: []string{"hongguoduanju.com", "fqnovel.com", "snssdk.com"}, Media: []string{"qznovelvod.com", "fqnovelvod.com", "douyinvod.com"}}

func new_backend(string) extract.Backend { return &backend{api: &api_client{}} }

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
	raw_url = strings.TrimSpace(raw_url)
	if !strings.Contains(raw_url, "://") {
		raw_url = "https://" + raw_url
	}
	series_id, err := parse_series_id(raw_url)
	if err != nil {
		return "", "", "", err
	}
	link, _ := url.Parse(raw_url)
	video_id := ""
	parts := strings.Split(strings.Trim(link.Path, "/"), "/")
	if len(parts) == 3 && parts[0] == "player" {
		video_id = parts[2]
		if !id_pattern.MatchString(video_id) {
			return "", "", "", errors.New("红果分集编号无效")
		}
	}
	page := site + "/player/" + series_id
	if video_id != "" {
		page += "/" + video_id
	}
	return page, series_id, video_id, nil
}
func (c *backend) ParseURL(raw_url string) (string, string, string, error) { return ParseURL(raw_url) }
func (c *backend) SetHTTPClient(client *http.Client)                       { c.api.HTTPClient = client }
func (c *backend) List(ctx context.Context, _ string, cover_id, video_id string) (scraper.Profile, []scraper.Episode, error) {
	profile := scraper.Profile{ID: cover_id, Kind: "series"}
	detail, err := c.api.series(ctx, cover_id)
	if err != nil {
		return profile, nil, err
	}
	profile.Title, profile.EpisodeCount = detail.SeriesName, len(detail.VidList)
	episodes := []scraper.Episode{}
	for index, id := range detail.VidList {
		if video_id != "" && string(id) != video_id {
			continue
		}
		episodes = append(episodes, scraper.Episode{ID: string(id), Order: index + 1, Title: fmt.Sprintf("第%d集", index+1), SourceURL: site + "/player/" + cover_id + "/" + string(id)})
	}
	return profile, episodes, nil
}
func (c *backend) Resolve(ctx context.Context, _ string, episode scraper.Episode) (scraper.Video, error) {
	info, err := c.api.app_media(ctx, episode.ID)
	return scraper.Video{Title: episode.Title, Info: info, UserAgent: app_user_agent, Referer: "https://novelquickapp.com/"}, err
}
