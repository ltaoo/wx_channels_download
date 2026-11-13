// Package youku extracts Youku metadata and playback sources.
package youku

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"time"
	"wx_channel/pkg/scraper"
	"wx_channel/pkg/scraper/internal/extract"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const PlatformID = "youku"
const youku_user_agent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

var youku_id_pattern = regexp.MustCompile(`^X[A-Za-z0-9=_-]{4,100}$`)
var youku_path_pattern = regexp.MustCompile(`^/v_show/id_(X[A-Za-z0-9=_-]{4,100})\.html$`)
var youku_show_id_pattern = regexp.MustCompile(`^[A-Za-z0-9_-]{5,100}$`)
var youku_show_path_pattern = regexp.MustCompile(`^/v_nextstage/id_([A-Za-z0-9_-]{5,100})\.html$`)

type backend struct{ *sourceutil.RequestClient }

var source_policy = sourceutil.Policy{AnonymousHosts: []string{"log.mmstat.com"}, UpgradeHTTPS: true, Metadata: []string{"youku.com", "log.mmstat.com"}, Media: []string{"youku.com", "ykimg.com", "ykugc.com", "youkudns.com", "cp31.ott.cibntv.net"}}

func new_backend(cookie string) extract.Backend {
	return &backend{RequestClient: &sourceutil.RequestClient{Policy: source_policy, Cookie: cookie, UserAgent: youku_user_agent}}
}

// FetchResult contains extracted source metadata, before downloader adaptation.
type FetchResult extract.Result

// Client retrieves native platform metadata and playback sources.
type Client struct {
	extractor   *extract.Client
	cookie      string
	http_client *http.Client
}

func NewClient(cookie string) *Client {
	client := &Client{extractor: extract.NewClient(PlatformID, cookie, source_policy, new_backend), cookie: cookie}
	client.SetHTTPClient(&http.Client{Timeout: 90 * time.Second})
	return client
}
func (c *Client) SetHTTPClient(http_client *http.Client) {
	if http_client != nil {
		c.http_client = http_client
		c.extractor.SetHTTPClient(http_client)
	}
}
func (c *Client) Fetch(raw_url string) (*FetchResult, error) {
	return c.FetchContext(context.Background(), raw_url)
}
func (c *Client) FetchContext(ctx context.Context, raw_url string) (*FetchResult, error) {
	page, show_id, _, err := ParseURL(raw_url)
	if err != nil {
		return nil, err
	}
	if show_id != "" {
		details, err := c.FetchSeriesDetailsContext(ctx, raw_url)
		if err != nil {
			return nil, err
		}
		return &FetchResult{Platform: PlatformID, SourceURL: page, Profile: details.Profile, Episodes: details.Episodes}, nil
	}
	result, err := c.extractor.FetchContext(ctx, raw_url)
	if err != nil {
		return nil, err
	}
	// Catalogue enrichment does not turn one requested video into an entire
	// show download or discard a playable video if the catalogue is unavailable.
	details, detail_err := c.FetchSeriesDetailsContext(ctx, raw_url)
	if detail_err == nil {
		result.Profile = details.Profile
		for _, episode := range details.Episodes {
			if episode.ID == result.Episodes[0].ID {
				played := result.Episodes[0]
				episode.Duration, episode.Width, episode.Height, episode.Codec, episode.URL = played.Duration, played.Width, played.Height, played.Codec, played.URL
				result.Episodes[0] = episode
				break
			}
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return (*FetchResult)(result), nil
}

func (c *Client) FetchSeriesDetailsContext(ctx context.Context, raw_url string) (*SeriesDetails, error) {
	page, show_id, video_id, err := ParseURL(raw_url)
	if err != nil {
		return nil, err
	}
	backend := new_backend(c.cookie).(*backend)
	backend.SetHTTPClient(c.http_client)
	return backend.series_details(ctx, page, show_id, video_id)
}

func (c *Client) FetchSeriesDetails(raw_url string) (*SeriesDetails, error) {
	return c.FetchSeriesDetailsContext(context.Background(), raw_url)
}
func (c *Client) ResolveContext(ctx context.Context, source_url string, episode scraper.Episode) (scraper.Video, error) {
	return c.extractor.ResolveContext(ctx, source_url, episode)
}
func ParseURL(raw_url string) (string, string, string, error) {
	link, err := extract.ParsePageURL(raw_url, "v.youku.com", "m.youku.com", "www.youku.com", "youku.com")
	if err != nil {
		return "", "", "", err
	}
	if match := youku_show_path_pattern.FindStringSubmatch(link.Path); match != nil {
		return "https://v.youku.com/v_nextstage/id_" + match[1] + ".html", match[1], "", nil
	}
	id := link.Query().Get("vid")
	if match := youku_path_pattern.FindStringSubmatch(link.Path); match != nil {
		id = match[1]
	}
	if !youku_id_pattern.MatchString(id) {
		return "", "", "", errors.New("优酷需要 /v_show/id_X编号.html 播放页")
	}
	return "https://v.youku.com/v_show/id_" + id + ".html", "", id, nil
}
func (c *backend) ParseURL(raw_url string) (string, string, string, error) { return ParseURL(raw_url) }
func (c *backend) SetHTTPClient(client *http.Client)                       { c.HTTPClient = client }
func (c *backend) List(_ context.Context, page, cover_id, video_id string) (scraper.Profile, []scraper.Episode, error) {
	return scraper.Profile{Kind: "video"}, []scraper.Episode{{ID: video_id, SourceURL: page}}, nil
}
func (c *backend) Resolve(ctx context.Context, _ string, episode scraper.Episode) (scraper.Video, error) {
	c.Referer = episode.SourceURL
	return c.youku_video(ctx, episode.ID)
}
