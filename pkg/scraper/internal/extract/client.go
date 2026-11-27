// Package extract supplies source-only fetching shared by platform scrapers.
package extract

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"wx_channel/pkg/scraper"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

type Result struct {
	Platform  string            `json:"platform"`
	SourceURL string            `json:"source_url"`
	Profile   scraper.Profile   `json:"profile"`
	Episodes  []scraper.Episode `json:"episodes"`
}
type Backend interface {
	ParseURL(string) (string, string, string, error)
	List(context.Context, string, string, string) (scraper.Profile, []scraper.Episode, error)
	Resolve(context.Context, string, scraper.Episode) (scraper.Video, error)
	SetHTTPClient(*http.Client)
}
type Client struct {
	platform    string
	cookie      string
	http_client *http.Client
	factory     func(string) Backend
	policy      sourceutil.Policy
}

func NewClient(platform, cookie string, policy sourceutil.Policy, factory func(string) Backend) *Client {
	return &Client{platform: platform, cookie: cookie, policy: policy, factory: factory, http_client: &http.Client{Timeout: 90 * time.Second}}
}
func (c *Client) SetHTTPClient(http_client *http.Client) {
	if http_client != nil {
		c.http_client = http_client
	}
}
func (c *Client) new_backend() Backend {
	backend := c.factory(c.cookie)
	backend.SetHTTPClient(c.http_client)
	return backend
}

// ParsePageURL validates a platform's official entry URL without network access.
func ParsePageURL(raw_url string, hosts ...string) (*url.URL, error) {
	raw_url = strings.TrimSpace(raw_url)
	if !strings.Contains(raw_url, "://") {
		raw_url = "https://" + raw_url
	}
	link, err := url.Parse(raw_url)
	if err != nil || len(raw_url) > 8192 || link.User != nil || link.Port() != "" || (link.Scheme != "https" && link.Scheme != "http") {
		return nil, errors.New("请输入有效的官网播放链接")
	}
	for _, host := range hosts {
		if strings.EqualFold(link.Hostname(), host) {
			return link, nil
		}
	}
	return nil, errors.New("播放链接域名不属于当前平台")
}

func (c *Client) Fetch(raw_url string) (*Result, error) {
	return c.FetchContext(context.Background(), raw_url)
}
func (c *Client) FetchContext(ctx context.Context, raw_url string) (*Result, error) {
	result, backend, cover_id, err := c.fetch_metadata(ctx, raw_url)
	if err != nil {
		return nil, err
	}
	first := &result.Episodes[0]
	item, err := backend.Resolve(ctx, cover_id, *first)
	if err != nil {
		return nil, err
	}
	first.Title = sourceutil.FirstText(item.Title, first.Title, result.Profile.Title, first.ID)
	first.Duration, first.Width, first.Height, first.Codec = item.Info.Duration, item.Info.Width, item.Info.Height, item.Info.Codec
	first.URL = item.Info.Address
	result.Profile.Title = sourceutil.FirstText(result.Profile.Title, item.Title, first.ID)
	if item.Profile != nil {
		sourceutil.MergeProfile(&result.Profile, *item.Profile)
	}
	c.enrich_profile(ctx, result.SourceURL, &result.Profile)
	if result.Profile.ID == "" {
		result.Profile.ID = first.ID
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return result, nil
}

// FetchMetadataContext retrieves source catalogue metadata without resolving a
// playback stream. Download preparation must still call ResolveContext.
func (c *Client) FetchMetadataContext(ctx context.Context, raw_url string) (*Result, error) {
	result, _, _, err := c.fetch_metadata(ctx, raw_url)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if result.Profile.ID == "" {
		result.Profile.ID = result.Episodes[0].ID
	}
	return result, nil
}

func (c *Client) fetch_metadata(ctx context.Context, raw_url string) (*Result, Backend, string, error) {
	backend := c.new_backend()
	if backend == nil {
		return nil, nil, "", errors.New("不支持的视频平台")
	}
	page, cover_id, video_id, err := backend.ParseURL(raw_url)
	if err != nil {
		return nil, nil, "", err
	}
	profile, episodes, err := backend.List(ctx, page, cover_id, video_id)
	if err != nil {
		return nil, nil, "", err
	}
	if len(episodes) == 0 {
		return nil, nil, "", errors.New("分集列表不包含请求的视频")
	}
	result := &Result{Platform: c.platform, SourceURL: page, Profile: profile, Episodes: episodes}
	return result, backend, cover_id, nil
}

// ResolveContext refreshes one requested playback source without writing media.
func (c *Client) ResolveContext(ctx context.Context, source_url string, episode scraper.Episode) (scraper.Video, error) {
	backend := c.new_backend()
	page, cover_id, video_id, err := backend.ParseURL(source_url)
	if err != nil {
		return scraper.Video{}, err
	}
	if video_id == "" || episode.ID != video_id {
		return scraper.Video{}, errors.New("分集与来源 URL 不匹配")
	}
	episode.SourceURL = page
	video, err := backend.Resolve(ctx, cover_id, episode)
	if err != nil {
		return scraper.Video{}, err
	}
	video.CheckMediaAddress = c.policy.CheckMediaAddress
	if video.UserAgent == "" {
		video.UserAgent = sourceutil.UserAgent
	}
	return video, nil
}
