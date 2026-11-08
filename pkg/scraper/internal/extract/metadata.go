package extract

import (
	"context"
	"io"
	"net/http"
	"time"
	"wx_channel/pkg/scraper"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

func (c *Client) enrich_profile(ctx context.Context, page string, profile *scraper.Profile) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", sourceutil.UserAgent)
	http_client := *c.http_client
	http_client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		_, _, _, err := c.new_backend().ParseURL(next.URL.String())
		return err
	}
	response, err := http_client.Do(req)
	if err != nil {
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(body) > 8<<20 {
		return
	}
	page_profile := sourceutil.ParsePageMetadata(body)
	if page_profile.Kind == "video" && profile.Title != "" {
		page_profile.Title = ""
	}
	sourceutil.MergeProfile(profile, page_profile)
}
