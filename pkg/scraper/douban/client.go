package douban

import (
	"context"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"

type Client struct {
	HTTPClient *http.Client
	UserAgent  string
	Debug      bool
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.HTTPClient = client
	}
}

func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		c.UserAgent = userAgent
	}
}

func WithDebug(debug bool) Option {
	return func(c *Client) {
		c.Debug = debug
	}
}

func NewClient(opts ...Option) *Client {
	jar, _ := cookiejar.New(nil)
	c := &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second, Jar: jar},
		UserAgent:  defaultUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 30 * time.Second, Jar: jar}
	}
	if strings.TrimSpace(c.UserAgent) == "" {
		c.UserAgent = defaultUserAgent
	}
	return c
}

func (c *Client) Search(ctx context.Context, keyword string) (*SearchResult, error) {
	u, err := url.Parse("https://search.douban.com/movie/subject_search")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("search_text", keyword)
	u.RawQuery = q.Encode()
	html, err := c.getText(ctx, u.String())
	if err != nil {
		return nil, err
	}
	return ParseSearchHTML(html)
}

func (c *Client) FetchMediaProfile(ctx context.Context, id any) (*MediaProfile, error) {
	endpoint := fmt.Sprintf("https://movie.douban.com/subject/%s/", idString(id))
	return c.FetchSubjectProfile(ctx, endpoint)
}

func (c *Client) FetchSubjectProfile(ctx context.Context, rawURL string) (*MediaProfile, error) {
	endpoint := strings.TrimSpace(rawURL)
	if endpoint == "" {
		return nil, fmt.Errorf("empty douban subject url")
	}
	html, err := c.getText(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	profile, err := ParseProfilePageHTML(html)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func (c *Client) FetchGroupTopic(ctx context.Context, rawURL string) (*GroupTopicProfile, error) {
	endpoint := strings.TrimSpace(rawURL)
	if endpoint == "" {
		return nil, fmt.Errorf("empty douban group topic url")
	}
	html, err := c.getText(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	profile, err := ParseGroupTopicHTML(html, endpoint)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func (c *Client) FetchMediaRank(ctx context.Context, mediaType MediaType) (*RankResult, error) {
	u, err := url.Parse("https://movie.douban.com/j/search_subjects")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("type", string(mediaType))
	q.Set("tag", "热门")
	q.Set("page_limit", "50")
	q.Set("page_start", "0")
	u.RawQuery = q.Encode()
	var resp struct {
		Subjects []struct {
			EpisodesInfo string `json:"episodes_info"`
			Rate         string `json:"rate"`
			Title        string `json:"title"`
			ID           string `json:"id"`
		} `json:"subjects"`
	}
	if err := c.getJSON(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	out := RankResult{List: make([]RankItem, 0, len(resp.Subjects))}
	for i, item := range resp.Subjects {
		rate, _ := strconv.ParseFloat(item.Rate, 64)
		out.List = append(out.List, RankItem{
			Name:      item.Title,
			Order:     i + 1,
			Rate:      rate,
			ExtraText: item.EpisodesInfo,
			DoubanID:  item.ID,
		})
	}
	return &out, nil
}

func (c *Client) MatchExactMedia(media MatchMedia, list []SearchItem) (*SearchItem, error) {
	return MatchExactMedia(media, list)
}

func (c *Client) getText(ctx context.Context, rawURL string) (string, error) {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, out any) error {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if isSecurityChallenge(resp, body) {
		return c.resolveSecurityChallenge(ctx, body)
	}
	return body, nil
}

var (
	encryptedDataRE = regexp.MustCompile(`__DATA__\s*=\s*"([^"]+)"`)
	plainDataRE     = regexp.MustCompile(`__DATA__\s*=\s*(\{[\s\S]*?\})\s*;`)
	yearInNameRE    = regexp.MustCompile(`\(([0-9]{4})\)`)
	hiddenInputRE   = regexp.MustCompile(`id="([^"]+)"\s+name="[^"]+"\s+value="([^"]*)"`)
)

func isSecurityChallenge(resp *http.Response, body []byte) bool {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil && resp.Request.URL.Hostname() == "sec.douban.com" {
		return true
	}
	text := string(body)
	return strings.Contains(text, `id="sec"`) && strings.Contains(text, `id="tok"`) && strings.Contains(text, `id="cha"`)
}

func (c *Client) resolveSecurityChallenge(ctx context.Context, body []byte) ([]byte, error) {
	form := map[string]string{}
	for _, match := range hiddenInputRE.FindAllStringSubmatch(string(body), -1) {
		if len(match) > 2 {
			form[match[1]] = match[2]
		}
	}
	tok, cha, red := form["tok"], form["cha"], form["red"]
	if tok == "" || cha == "" || red == "" {
		return nil, fmt.Errorf("incomplete douban security challenge")
	}
	formValues := url.Values{}
	formValues.Set("tok", tok)
	formValues.Set("cha", cha)
	formValues.Set("sol", solveChallenge(cha, 4))
	formValues.Set("red", red)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://sec.douban.com/c", strings.NewReader(formValues.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Referer", "https://sec.douban.com/")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	nextBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(nextBody)))
	}
	if isSecurityChallenge(resp, nextBody) {
		return nil, fmt.Errorf("douban security challenge was not accepted")
	}
	return nextBody, nil
}

func solveChallenge(challenge string, difficulty int) string {
	prefix := strings.Repeat("0", difficulty)
	for nonce := 1; ; nonce++ {
		sum := sha512.Sum512([]byte(challenge + strconv.Itoa(nonce)))
		if strings.HasPrefix(fmt.Sprintf("%x", sum), prefix) {
			return strconv.Itoa(nonce)
		}
	}
}

func ParseSearchHTML(html string) (*SearchResult, error) {
	rawItems, err := extractSearchItems(html)
	if err != nil {
		return nil, err
	}
	out := SearchResult{List: make([]SearchItem, 0, len(rawItems))}
	for _, raw := range rawItems {
		if strings.TrimSpace(raw.Abstract) == "" {
			continue
		}
		fields := splitAndTrim(raw.Abstract, "/")
		genres := make([]Genre, 0, len(fields))
		for _, field := range fields {
			if value := genreTextToValue[field]; value != 0 {
				genres = append(genres, Genre{Value: value, Label: field})
			}
		}
		mediaType := ""
		if len(raw.Labels) > 0 {
			if raw.Labels[0].Text == "剧集" {
				mediaType = string(MediaTypeTV)
			} else {
				mediaType = string(MediaTypeMovie)
			}
		}
		name := CleanName(raw.Title)
		airDate := ""
		if match := yearInNameRE.FindStringSubmatch(name); len(match) > 1 {
			airDate = match[1]
			name = strings.TrimSpace(strings.Replace(name, match[0], "", 1))
		}
		names := SplitNameAndOriginalName(strings.Join(strings.Fields(name), " "))
		voteAverage := 0.0
		if raw.Rating != nil {
			voteAverage = raw.Rating.Value
		}
		out.List = append(out.List, SearchItem{
			ID:           idString(raw.ID),
			Name:         strings.TrimSpace(names.Name),
			OriginalName: names.OriginalName,
			Overview:     "",
			PosterPath:   raw.CoverURL,
			AirDate:      airDate,
			VoteAverage:  voteAverage,
			Type:         mediaType,
			Genres:       genres,
			Raw:          raw,
		})
	}
	return &out, nil
}

func extractSearchItems(html string) ([]RawSearchItem, error) {
	if match := encryptedDataRE.FindStringSubmatch(html); len(match) > 1 {
		data, err := DecryptSearchData(match[1])
		if err != nil {
			return nil, err
		}
		return data.Items, nil
	}
	if match := plainDataRE.FindStringSubmatch(html); len(match) > 1 {
		var data searchData
		if err := json.Unmarshal([]byte(match[1]), &data); err != nil {
			return nil, err
		}
		return data.Items, nil
	}
	return nil, fmt.Errorf("missing __DATA__")
}

type searchData struct {
	Items []RawSearchItem `json:"items"`
}
