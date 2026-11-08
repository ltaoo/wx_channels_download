// Package iqiyi extracts IQiyi metadata and playback sources.
package iqiyi

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/internal/extract"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const PlatformID = "iqiyi"

var id_pattern = regexp.MustCompile(`^\d{1,30}$`)

type backend struct {
	*sourceutil.RequestClient
	device_id string
}

var source_policy = sourceutil.Policy{Metadata: []string{"iqiyi.com"}, Media: []string{"iqiyi.com", "qiyi.com", "qy.net", "qyvideo.net", "71edge.com"}}

func new_backend(cookie string) extract.Backend {
	return &backend{RequestClient: &sourceutil.RequestClient{Policy: source_policy, Cookie: cookie}, device_id: strings.ReplaceAll(sourceutil.RandomUUID(), "-", "")}
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

// FetchDetailsContext reads content and episode identities independently of
// playback availability; the adapter resolves streams when a download starts.
func (c *Client) FetchDetailsContext(ctx context.Context, raw_url string) (*FetchResult, error) {
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
	raw_url = strings.TrimSpace(raw_url)
	if !strings.Contains(raw_url, "://") {
		raw_url = "https://" + raw_url
	}
	page, id, album, err := iqiyi_page(raw_url)
	if err != nil {
		return "", "", "", err
	}
	if album {
		return page, id, "", nil
	}
	return page, "", id, nil
}
func (c *backend) ParseURL(raw_url string) (string, string, string, error) { return ParseURL(raw_url) }
func (c *backend) SetHTTPClient(client *http.Client)                       { c.HTTPClient = client }
func (c *backend) List(ctx context.Context, page, cover_id, video_id string) (scraper.Profile, []scraper.Episode, error) {
	c.Referer = page
	series, err := c.series(ctx, page, sourceutil.FirstText(cover_id, video_id), cover_id != "")
	if err != nil {
		return scraper.Profile{}, nil, err
	}
	profile := series.profile
	episodes := []scraper.Episode{}
	for _, row := range series.episodes {
		if video_id != "" && string(row.TVID) != video_id {
			continue
		}
		source_url := row.PageURL
		if source_url == "" {
			number, err := strconv.ParseUint(string(row.TVID), 10, 64)
			if err != nil {
				return profile, nil, err
			}
			source_url = "https://www.iqiyi.com/v_" + strconv.FormatUint(number^0x75706971676c, 36) + ".html"
		}
		episodes = append(episodes, scraper.Episode{ID: string(row.TVID), SourceURL: source_url, Title: sourceutil.FirstText(row.SubTitle, row.Title), Order: row.Order})
	}
	return profile, episodes, nil
}
func (c *backend) Resolve(ctx context.Context, _ string, episode scraper.Episode) (scraper.Video, error) {
	c.Referer = episode.SourceURL
	playlist, info, err := c.playlist(ctx, iqiyi_episode{TVID: sourceutil.TextValue(episode.ID), Order: episode.Order, Title: episode.Title, PageURL: episode.SourceURL})
	return scraper.Video{Title: episode.Title, Playlist: playlist, Info: info}, err
}
