package qidian

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
)

const (
	PlatformID = "qidian"
	baseURL    = "https://www.qidian.com"
)

type BookVolume struct {
	Idx      int       `json:"idx"`
	Title    string    `json:"title"`
	Chapters []Chapter `json:"chapters"`
}

type Chapter struct {
	Idx   int    `json:"idx"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Author struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type BookProfile struct {
	URL            string       `json:"url"`
	Title          string       `json:"title"`
	Description    string       `json:"description"`
	Slogan         string       `json:"slogan"`
	CoverURL       string       `json:"cover_url"`
	LatestUpdateAt time.Time    `json:"latest_update_at,omitempty"`
	Tags           []string     `json:"tags,omitempty"`
	LatestChapter  Chapter      `json:"latest_chapter"`
	ChapterCount   int          `json:"chapter_count"`
	Author         Author       `json:"author"`
	Volumes        []BookVolume `json:"volumes,omitempty"`
}

type Fetcher interface {
	FetchBookProfile(id string) (*BookProfile, error)
}

type Handler struct {
	Fetcher Fetcher
}

type parsedURL struct {
	BookID    string
	Canonical string
}

func New(fetcher Fetcher) *Handler {
	if fetcher == nil {
		fetcher = NewClient(nil)
	}
	return &Handler{Fetcher: fetcher}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	_, ok := ParseURL(rawURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	parts, ok := ParseURL(input.URL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	profile, err := h.Fetcher.FetchBookProfile(parts.BookID)
	if err != nil {
		return nil, fmt.Errorf("fetch qidian book profile: %w", err)
	}
	profile.URL = novelutil.FirstNonEmpty(profile.URL, parts.Canonical)
	title := novelutil.FirstNonEmpty(profile.Title, "qidian_"+parts.BookID)
	bodyHTML := novelutil.RenderBookHTML("起点中文网", novelutil.Book{
		Title:       title,
		URL:         parts.Canonical,
		Author:      profile.Author.Name,
		BookID:      parts.BookID,
		Description: profile.Description,
		CoverURL:    profile.CoverURL,
		Tags:        profile.Tags,
		Chapters:    qidianChapters(profile.Volumes),
	})
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: parts.Canonical,
		ContentID:    parts.BookID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           "novel",
			ID:             parts.BookID,
			Title:          title,
			Description:    novelutil.FirstNonEmpty(profile.Slogan, profile.Description),
			Author:         profile.Author.Name,
			URL:            parts.Canonical,
			SourceURL:      parts.Canonical,
			AuthorNickname: profile.Author.Name,
			CoverURL:       profile.CoverURL,
		}, profile, map[string]any{
			"book_id":          parts.BookID,
			"author_url":       profile.Author.URL,
			"chapter_count":    profile.ChapterCount,
			"latest_chapter":   profile.LatestChapter.Title,
			"latest_update_at": profile.LatestUpdateAt,
			"source_url":       parts.Canonical,
		}, map[string]any{
			"format":        "html",
			"content_type":  "novel",
			"title":         title,
			"source_url":    parts.Canonical,
			"canonical_url": parts.Canonical,
			"body_html":     bodyHTML,
		}),
		Variants: []contentdownload.Variant{
			novelutil.HTMLVariant("目录 HTML", "novel"),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{"profile": profile},
	}, nil
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	return novelutil.ResolveInlineHTML(ctx, PlatformID, input, h.Probe)
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return novelutil.HTMLPlan(PlatformID), nil
}

func ParseURL(rawURL string) (parsedURL, bool) {
	parsed, ok := novelutil.IsHTTPHost(rawURL, "qidian.com", "www.qidian.com", "m.qidian.com")
	if !ok {
		return parsedURL{}, false
	}
	segments := novelutil.SplitPath(parsed)
	if len(segments) >= 2 && segments[0] == "book" && strings.TrimSpace(segments[1]) != "" {
		bookID := strings.TrimSpace(segments[1])
		return parsedURL{
			BookID:    bookID,
			Canonical: baseURL + "/book/" + bookID + "/",
		}, true
	}
	return parsedURL{}, false
}

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	UserAgent  string
}

func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{}
	}
	return &Client{
		HTTPClient: client,
		BaseURL:    baseURL,
		UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
	}
}

func (c *Client) FetchBookProfile(id string) (*BookProfile, error) {
	reqURL := strings.TrimRight(novelutil.FirstNonEmpty(c.BaseURL, baseURL), "/") + "/book/" + strings.TrimSpace(id) + "/"
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, reqURL)
	}
	return c.ParseBookProfile(reqURL, resp.Body)
}

func (c *Client) ParseBookProfile(reqURL string, r io.Reader) (*BookProfile, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	profile := &BookProfile{URL: reqURL}
	profile.Title = strings.TrimSpace(doc.Find("#bookName").Text())
	if profile.Title == "" {
		profile.Title = strings.TrimSpace(doc.Find("h1").First().Text())
	}
	profile.Slogan = strings.TrimSpace(doc.Find(".intro").Text())
	profile.Description = strings.TrimSpace(doc.Find("#book-intro-detail").Text())
	if profile.Description == "" {
		profile.Description = strings.TrimSpace(doc.Find(".book-intro, .book-info-detail, .intro-detail").First().Text())
	}
	authorSection := doc.Find(".author-intro")
	profile.Author.Name = strings.TrimSpace(authorSection.Find(".writer-name").Text())
	if profile.Author.Name == "" {
		profile.Author.Name = strings.TrimSpace(doc.Find(".writer-name, .author a, .book-author").First().Text())
	}
	profile.Author.URL = normalizeQidianURL(authorSection.Find(".writer-name").AttrOr("href", ""))
	profile.CoverURL = normalizeQidianURL(novelutil.FirstNonEmpty(
		doc.Find("#bookImg img").AttrOr("src", ""),
		doc.Find(".book-detail-img img").AttrOr("src", ""),
		doc.Find(".book-img img").AttrOr("src", ""),
	))
	doc.Find(".all-label a, .tag-wrap a, .book-label a").Each(func(_ int, s *goquery.Selection) {
		tag := strings.TrimSpace(s.Text())
		if tag != "" {
			profile.Tags = append(profile.Tags, tag)
		}
	})
	latestChapter := doc.Find(".book-latest-chapter")
	profile.LatestChapter.Title = strings.TrimSpace(strings.TrimPrefix(latestChapter.Text(), "最新章节:"))
	profile.LatestChapter.URL = normalizeQidianURL(latestChapter.AttrOr("href", ""))
	timeStr := strings.TrimSpace(strings.TrimPrefix(doc.Find(".update-time").Text(), "更新时间:"))
	if t, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
		profile.LatestUpdateAt = t
	}
	chapterCount := 0
	doc.Find("#allCatalog .catalog-volume, .catalog-volume").Each(func(i int, s *goquery.Selection) {
		volume := BookVolume{
			Idx:   i + 1,
			Title: strings.TrimSpace(s.Find(".volume-name").Contents().Not("span").Text()),
		}
		if volume.Title == "" {
			volume.Title = strings.TrimSpace(s.Find(".volume-name").First().Text())
		}
		s.Find(".volume-chapters .chapter-item, .chapter-list .chapter-item, li").Each(func(_ int, cs *goquery.Selection) {
			link := cs.Find(".chapter-name, a").First()
			title := strings.TrimSpace(link.Text())
			if title == "" {
				return
			}
			chapterCount++
			volume.Chapters = append(volume.Chapters, Chapter{
				Idx:   chapterCount,
				Title: title,
				URL:   normalizeQidianURL(link.AttrOr("href", "")),
			})
		})
		if len(volume.Chapters) > 0 || volume.Title != "" {
			profile.Volumes = append(profile.Volumes, volume)
		}
	})
	profile.ChapterCount = chapterCount
	return profile, nil
}

func qidianChapters(volumes []BookVolume) []novelutil.Chapter {
	var out []novelutil.Chapter
	for _, volume := range volumes {
		for _, chapter := range volume.Chapters {
			out = append(out, novelutil.Chapter{Index: chapter.Idx, Title: chapter.Title, URL: chapter.URL})
		}
	}
	return out
}

func normalizeQidianURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	if strings.HasPrefix(value, "/") {
		return baseURL + value
	}
	return value
}
