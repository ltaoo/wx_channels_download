package fanqienovel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
	fanqiepkg "wx_channel/pkg/fanqienovel"
)

const (
	PlatformID = "fanqienovel"
	baseURL    = "https://fanqienovel.com"
)

type Author struct {
	Name      string `json:"name"`
	Desc      string `json:"desc,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	URL       string `json:"url,omitempty"`
}

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

type BookProfile struct {
	URL              string          `json:"url"`
	Title            string          `json:"title"`
	Description      string          `json:"description"`
	Slogan           string          `json:"slogan"`
	CoverURL         string          `json:"cover_url"`
	LatestUpdateAt   *time.Time      `json:"latest_update_at,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
	LatestChapter    Chapter         `json:"latest_chapter"`
	ChapterCount     int             `json:"chapter_count"`
	Author           Author          `json:"author"`
	Volumes          []BookVolume    `json:"volumes,omitempty"`
	InitialStateJSON json.RawMessage `json:"-"`
}

type ChapterContent struct {
	Title            string          `json:"title"`
	PublishAt        *time.Time      `json:"publish_at,omitempty"`
	Content          string          `json:"content"`
	WorkCount        string          `json:"work_count,omitempty"`
	InitialStateJSON json.RawMessage `json:"-"`
}

type Fetcher interface {
	FetchBookProfile(id string) (*BookProfile, error)
	FetchBookChapterProfile(chapterID string) (*ChapterContent, error)
}

type Handler struct {
	Fetcher Fetcher
}

type parsedURL struct {
	Kind      string
	ID        string
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
	if parts.Kind == "chapter" {
		return h.probeChapter(input.URL, parts)
	}
	return h.probeNovel(input.URL, parts)
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	return novelutil.ResolveInlineHTML(ctx, PlatformID, input, h.Probe)
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return novelutil.HTMLPlan(PlatformID), nil
}

func (h *Handler) probeNovel(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	profile, err := h.Fetcher.FetchBookProfile(parts.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch fanqie novel: %w", err)
	}
	profile.URL = novelutil.FirstNonEmpty(profile.URL, parts.Canonical)
	title := novelutil.FirstNonEmpty(profile.Title, "fanqie_"+parts.ID)
	bodyHTML := novelutil.RenderBookHTML("番茄小说", novelutil.Book{
		Title:       title,
		URL:         parts.Canonical,
		Author:      profile.Author.Name,
		BookID:      parts.ID,
		Description: profile.Description,
		CoverURL:    profile.CoverURL,
		Tags:        profile.Tags,
		Chapters:    fanqieChapters(profile.Volumes),
	})
	internal := map[string]any{"profile": profile}
	if len(profile.InitialStateJSON) > 0 {
		internal["pagejson"] = json.RawMessage(append([]byte(nil), profile.InitialStateJSON...))
	}
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    parts.ID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "novel",
			ID:              parts.ID,
			Title:           title,
			Description:     profile.Description,
			Author:          profile.Author.Name,
			URL:             parts.Canonical,
			SourceURL:       parts.Canonical,
			AuthorNickname:  profile.Author.Name,
			AuthorAvatarURL: profile.Author.AvatarURL,
			CoverURL:        profile.CoverURL,
		}, profile, map[string]any{
			"book_id":          parts.ID,
			"author_desc":      profile.Author.Desc,
			"chapter_count":    profile.ChapterCount,
			"latest_chapter":   profile.LatestChapter.Title,
			"latest_update_at": profile.LatestUpdateAt,
			"source_url":       parts.Canonical,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  "novel",
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{novelutil.HTMLVariant("目录 HTML", "novel")},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: internal,
	}, nil
}

func (h *Handler) probeChapter(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	chapter, err := h.Fetcher.FetchBookChapterProfile(parts.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch fanqie chapter: %w", err)
	}
	title := novelutil.FirstNonEmpty(chapter.Title, "fanqie_"+parts.ID)
	bodyHTML := novelutil.RenderChapterHTML("番茄小说", title, parts.Canonical, chapter.Content)
	internal := map[string]any{"chapter": chapter}
	if len(chapter.InitialStateJSON) > 0 {
		internal["pagejson"] = json.RawMessage(append([]byte(nil), chapter.InitialStateJSON...))
	}
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    parts.ID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:    PlatformID,
			Type:        "chapter",
			ID:          parts.ID,
			Title:       title,
			Description: chapter.WorkCount,
			URL:         parts.Canonical,
			SourceURL:   parts.Canonical,
		}, chapter, map[string]any{
			"chapter_id": parts.ID,
			"publish_at": chapter.PublishAt,
			"work_count": chapter.WorkCount,
			"source_url": parts.Canonical,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  "chapter",
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{novelutil.HTMLVariant("章节 HTML", "chapter")},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: internal,
	}, nil
}

func ParseURL(rawURL string) (parsedURL, bool) {
	parsed, ok := novelutil.IsHTTPHost(rawURL, "fanqienovel.com", "www.fanqienovel.com")
	if !ok {
		return parsedURL{}, false
	}
	segments := novelutil.SplitPath(parsed)
	if len(segments) < 2 {
		return parsedURL{}, false
	}
	switch segments[0] {
	case "page":
		return parsedURL{Kind: "novel", ID: segments[1], Canonical: baseURL + "/page/" + segments[1]}, true
	case "reader":
		return parsedURL{Kind: "chapter", ID: segments[1], Canonical: baseURL + "/reader/" + segments[1]}, true
	default:
		return parsedURL{}, false
	}
}

type Client struct {
	HTTPClient *http.Client
	UserAgent  string
	Cookie     string
}

func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{}
	}
	return &Client{
		HTTPClient: client,
		UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
	}
}

func (c *Client) FetchBookProfile(id string) (*BookProfile, error) {
	reqURL := baseURL + "/page/" + strings.TrimSpace(id)
	req, err := c.newRequest(reqURL, baseURL+"/")
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, reqURL)
	}
	return c.parseBookProfile(reqURL, resp.Body)
}

func (c *Client) FetchBookChapterProfile(chapterID string) (*ChapterContent, error) {
	reqURL := baseURL + "/reader/" + strings.TrimSpace(chapterID) + "?enter_from=page"
	req, err := c.newRequest(reqURL, baseURL+"/")
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, reqURL)
	}
	return c.parseChapterProfile(resp.Body)
}

func (c *Client) newRequest(reqURL, referer string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", c.UserAgent)
	if strings.TrimSpace(c.Cookie) != "" {
		req.Header.Set("Cookie", c.Cookie)
	}
	return req, nil
}

func (c *Client) parseBookProfile(reqURL string, r io.Reader) (*BookProfile, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if profile, err := fanqiepkg.ParseBookProfileHTML(reqURL, string(body)); err == nil {
		return bookProfileFromPkg(profile), nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	profile := &BookProfile{
		URL:         reqURL,
		Title:       strings.TrimSpace(doc.Find(".info-name").Text()),
		CoverURL:    normalizeFanqieURL(doc.Find(".book-cover-img").AttrOr("src", "")),
		Description: strings.TrimSpace(doc.Find(".page-abstract-content").Text()),
		Tags: doc.Find(".info-label .info-label-grey").Map(func(i int, s *goquery.Selection) string {
			return strings.TrimSpace(s.Text())
		}),
		Author: Author{
			Name:      strings.TrimSpace(doc.Find(".author-name-text").Text()),
			Desc:      strings.TrimSpace(doc.Find(".author-desc").Text()),
			AvatarURL: normalizeFanqieURL(doc.Find(".author-img").AttrOr("src", "")),
		},
		LatestChapter: Chapter{
			Idx:   1,
			Title: strings.TrimSpace(strings.TrimPrefix(doc.Find(".info-last-title").Text(), "最新章节：")),
			URL:   normalizeFanqieURL(doc.Find(".chapter-name").AttrOr("href", "")),
		},
	}
	if profile.Title == "" {
		profile.Title = strings.TrimSpace(doc.Find("h1").First().Text())
	}
	latestUpdateAt := strings.TrimSpace(doc.Find(".info-last .info-last-time").Text())
	if t, err := time.Parse("2006-01-02 15:04:05", latestUpdateAt); err == nil {
		profile.LatestUpdateAt = &t
	}
	chapterCount := 0
	doc.Find(".page-directory-content>div").Each(func(i int, s *goquery.Selection) {
		volume := BookVolume{Idx: i + 1, Title: strings.TrimSpace(s.Find(".volume").Text())}
		s.Find(".chapter-item").Each(func(_ int, cs *goquery.Selection) {
			link := cs.Find(".chapter-item-title")
			title := strings.TrimSpace(link.Text())
			if title == "" {
				return
			}
			chapterCount++
			volume.Chapters = append(volume.Chapters, Chapter{
				Idx:   chapterCount,
				Title: title,
				URL:   normalizeFanqieURL(link.AttrOr("href", "")),
			})
		})
		if len(volume.Chapters) > 0 || volume.Title != "" {
			profile.Volumes = append(profile.Volumes, volume)
		}
	})
	if chapterCount == 0 {
		volume := BookVolume{Idx: 1, Title: "默认"}
		doc.Find(".page-directory-content .chapter_item, .page-directory-content .chapter-item").Each(func(_ int, cs *goquery.Selection) {
			link := cs.Find(".chapter-title, .chapter-item-title, a").First()
			title := strings.TrimSpace(link.Text())
			if title == "" {
				return
			}
			chapterCount++
			volume.Chapters = append(volume.Chapters, Chapter{
				Idx:   chapterCount,
				Title: title,
				URL:   normalizeFanqieURL(link.AttrOr("href", "")),
			})
		})
		if len(volume.Chapters) > 0 {
			profile.Volumes = append(profile.Volumes, volume)
		}
	}
	profile.ChapterCount = chapterCount
	return profile, nil
}

func (c *Client) parseChapterProfile(r io.Reader) (*ChapterContent, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if chapter, err := fanqiepkg.ParseChapterContentHTML(string(body)); err == nil {
		return chapterContentFromPkg(chapter), nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	profile := &ChapterContent{
		Title:     strings.TrimSpace(doc.Find(".muye-reader-title").Text()),
		WorkCount: strings.TrimSpace(strings.TrimPrefix(doc.Find(".desc-item:first-child").Text(), "总字数：")),
	}
	if profile.Title == "" {
		profile.Title = strings.TrimSpace(doc.Find("h1").First().Text())
	}
	publishAt := strings.TrimSpace(strings.TrimPrefix(doc.Find(".desc-item:nth-child(2)").Text(), "发布时间："))
	if t, err := time.Parse("2006-01-02 15:04:05", publishAt); err == nil {
		profile.PublishAt = &t
	}
	doc.Find(".muye-reader-content>div>p, .muye-reader-content p, article p").Each(func(_ int, s *goquery.Selection) {
		if text := strings.TrimSpace(translateString(s.Text())); text != "" {
			profile.Content += text + "\n"
		}
	})
	return profile, nil
}

func fanqieChapters(volumes []BookVolume) []novelutil.Chapter {
	var out []novelutil.Chapter
	for _, volume := range volumes {
		for _, chapter := range volume.Chapters {
			out = append(out, novelutil.Chapter{Index: chapter.Idx, Title: chapter.Title, URL: chapter.URL})
		}
	}
	return out
}

func normalizeFanqieURL(value string) string {
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

func translateString(value string) string {
	var b strings.Builder
	for _, r := range value {
		b.WriteString(TranslateText(string(r)))
	}
	return b.String()
}

func bookProfileFromPkg(in *fanqiepkg.BookProfile) *BookProfile {
	if in == nil {
		return nil
	}
	return &BookProfile{
		URL:              in.URL,
		Title:            in.Title,
		Description:      in.Description,
		Slogan:           in.Slogan,
		CoverURL:         in.CoverURL,
		LatestUpdateAt:   in.LatestUpdateAt,
		Tags:             append([]string(nil), in.Tags...),
		LatestChapter:    chapterFromPkg(in.LatestChapter),
		ChapterCount:     in.ChapterCount,
		InitialStateJSON: append(json.RawMessage(nil), in.InitialStateJSON...),
		Author: Author{
			Name:      in.Author.Name,
			Desc:      in.Author.Desc,
			AvatarURL: in.Author.AvatarURL,
			URL:       in.Author.URL,
		},
		Volumes: volumesFromPkg(in.Volumes),
	}
}

func volumesFromPkg(in []fanqiepkg.BookVolume) []BookVolume {
	out := make([]BookVolume, 0, len(in))
	for _, volume := range in {
		chapters := make([]Chapter, 0, len(volume.Chapters))
		for _, chapter := range volume.Chapters {
			chapters = append(chapters, chapterFromPkg(chapter))
		}
		out = append(out, BookVolume{Idx: volume.Idx, Title: volume.Title, Chapters: chapters})
	}
	return out
}

func chapterFromPkg(in fanqiepkg.Chapter) Chapter {
	return Chapter{Idx: in.Idx, Title: in.Title, URL: in.URL}
}

func chapterContentFromPkg(in *fanqiepkg.ChapterContent) *ChapterContent {
	if in == nil {
		return nil
	}
	return &ChapterContent{
		Title:            in.Title,
		PublishAt:        in.PublishAt,
		Content:          in.Content,
		WorkCount:        in.WorkCount,
		InitialStateJSON: append(json.RawMessage(nil), in.InitialStateJSON...),
	}
}
