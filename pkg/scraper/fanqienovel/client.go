package fanqienovel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/pkg/clawreq"
)

const (
	fanqie_base_url = "https://fanqienovel.com"

	encoded_characters = ``
	decoded_characters = `0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ的一是了我不人在他有这个上们来到时大地为子中你说生国年着就那和要她出也得里后自以会家可下而过天去能对小多然于心学么之都好看起发当没成只如事把还用第样道想作种开美总从无情己面最女但现前些所同日手又行意动方期它头经长儿回位分爱老因很给名法间斯知世什两次使身者被高已亲其进此话常与活正感见明问力理尔点文几定本公特做外孩相西果走将月十实向声车全信重三机工物气每并别真打太新比才便夫再书部水像眼等体却加电主界门利海受听表德少克代员许稜先口由死安写性马光白或住难望救命花结乐色更拉东神记处让母父应直字场平报友关放至张认接告入笑内英军候民岁往何度山觉路带万男边风解叫任金快原吃妈变通师立象数四失满战远格士音轻目条呢`
)

var (
	// ErrUnsupportedURL is returned when Fetch receives a URL other than a
	// Fanqie book profile URL.
	ErrUnsupportedURL = errors.New("unsupported fanqienovel url")
	// ErrFetchInterrupted is returned after the fetch context is cancelled.
	ErrFetchInterrupted = errors.New("fanqienovel fetch interrupted")
)

// FetchParams contains the input accepted by Fetch.
type FetchParams struct {
	URL          string          `json:"url"`
	RequestID    string          `json:"request_id,omitempty"`
	ForceRefresh bool            `json:"force_refresh,omitempty"`
	Context      context.Context `json:"-"`
}

const (
	FetchStageStart       = "start"
	FetchStageProfile     = "profile"
	FetchStageDirectory   = "directory"
	FetchStageChapter     = "chapter"
	FetchStageComplete    = "complete"
	FetchStageFailed      = "failed"
	FetchStageInterrupted = "interrupted"

	FetchStatusRunning     = "running"
	FetchStatusCompleted   = "completed"
	FetchStatusFailed      = "failed"
	FetchStatusInterrupted = "interrupted"
)

// FetchProgress describes a progress snapshot emitted while Fetch loads a
// book profile, its directory, and each chapter.
type FetchProgress struct {
	RequestID    string                `json:"request_id"`
	Platform     string                `json:"platform"`
	URL          string                `json:"url"`
	BookID       string                `json:"book_id,omitempty"`
	BookTitle    string                `json:"book_title,omitempty"`
	Stage        string                `json:"stage"`
	Status       string                `json:"status"`
	Current      int                   `json:"current"`
	Total        int                   `json:"total"`
	Percent      float64               `json:"percent"`
	VolumeTitle  string                `json:"volume_title,omitempty"`
	ChapterID    string                `json:"chapter_id,omitempty"`
	ChapterTitle string                `json:"chapter_title,omitempty"`
	Message      string                `json:"message"`
	Error        string                `json:"error,omitempty"`
	Cached       bool                  `json:"cached,omitempty"`
	CacheHits    int                   `json:"cache_hits,omitempty"`
	Profile      *FanqieBookProfile    `json:"-"`
	Chapter      *FanqieFetchedChapter `json:"-"`
}

// FetchProgressHandler receives progress snapshots from Fetch.
type FetchProgressHandler func(FetchProgress)

// FanqieClient fetches Fanqie book profiles and chapter contents.
type FanqieClient struct {
	claw_client      *clawreq.Client
	claw_client_err  error
	base_url         string
	cookie           string
	progress_handler FetchProgressHandler
	work_dir         string
	cache_source_url string
	force_refresh    bool
	fetch_context    context.Context
}

// FanqieResp mirrors the response wrapper used by the original scraper.
type FanqieResp[T any] struct {
	Data   T    `json:"data"`
	Cached bool `json:"cached,omitempty"`
}

type FanqieAuthor struct {
	Name      string `json:"name"`
	Desc      string `json:"description"`
	AvatarURL string `json:"avatar_url"`
	URL       string `json:"url"`
}

type FanqieBookChapter struct {
	Idx   int    `json:"idx"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type FanqieBookVolume struct {
	Idx      int                 `json:"idx"`
	Title    string              `json:"title"`
	Chapters []FanqieBookChapter `json:"chapters"`
}

type FanqieBookProfile struct {
	URL            string             `json:"url"`
	Title          string             `json:"title"`
	Description    string             `json:"description"`
	Slogan         string             `json:"slogan"`
	CoverURL       string             `json:"cover_url"`
	LatestUpdateAt *time.Time         `json:"latest_update_at,omitempty"`
	Tags           []string           `json:"tags"`
	LatestChapter  FanqieBookChapter  `json:"latest_chapter"`
	ChapterCount   int                `json:"chapter_count"`
	Author         FanqieAuthor       `json:"author"`
	Volumes        []FanqieBookVolume `json:"volumes"`
}

type FanqieBookChapterProfile struct {
	Title     string     `json:"title"`
	PublishAt *time.Time `json:"publish_at,omitempty"`
	Content   string     `json:"content"`
	WordCount string     `json:"word_count"`
}

// FanqieFetchedChapter is a fetched chapter plus its position in the book.
type FanqieFetchedChapter struct {
	Idx         int        `json:"idx"`
	VolumeIdx   int        `json:"volume_idx"`
	VolumeTitle string     `json:"volume_title"`
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	PublishAt   *time.Time `json:"publish_at,omitempty"`
	Content     string     `json:"content"`
	WordCount   string     `json:"word_count"`
}

// FanqieFetchResult contains the profile and all fetched chapter contents.
type FanqieFetchResult struct {
	Profile  *FanqieBookProfile     `json:"profile"`
	Chapters []FanqieFetchedChapter `json:"chapters"`
}

func NewFanqieClient() *FanqieClient {
	claw_client, claw_client_err := clawreq.New(clawreq.Config{
		Profile:         clawreq.ProfileChrome,
		Timeout:         30 * time.Second,
		FollowRedirects: true,
	})
	return &FanqieClient{
		claw_client:     claw_client,
		claw_client_err: claw_client_err,
		base_url:        fanqie_base_url,
	}
}

// SetCookie configures an optional Cookie header for subsequent requests.
func (c *FanqieClient) SetCookie(cookie string) {
	if c == nil {
		return
	}
	c.cookie = strings.TrimSpace(cookie)
}

// SetWorkDir enables persistent HTML caching beneath the runtime workdir.
func (c *FanqieClient) SetWorkDir(work_dir string) {
	if c == nil {
		return
	}
	c.work_dir = strings.TrimSpace(work_dir)
}

// SetProgressHandler configures an optional progress callback for Fetch.
func (c *FanqieClient) SetProgressHandler(progress_handler FetchProgressHandler) {
	if c == nil {
		return
	}
	c.progress_handler = progress_handler
}

func (c *FanqieClient) report_progress(progress FetchProgress) {
	if c == nil || c.progress_handler == nil {
		return
	}
	func() {
		defer func() {
			_ = recover()
		}()
		c.progress_handler(progress)
	}()
}

func fetch_percent(current int, total int) float64 {
	if total <= 0 || current <= 0 {
		return 0
	}
	if current >= total {
		return 100
	}
	return float64(current) * 100 / float64(total)
}

func (c *FanqieClient) check_fetch_interrupted() error {
	if c == nil || c.fetch_context == nil {
		return nil
	}
	select {
	case <-c.fetch_context.Done():
		return fmt.Errorf("%w: %v", ErrFetchInterrupted, c.fetch_context.Err())
	default:
		return nil
	}
}

func normalize_fetch_error(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrFetchInterrupted) {
		return fmt.Errorf("%w: %v", ErrFetchInterrupted, err)
	}
	return err
}

// Fetch fetches a Fanqie book profile and every chapter in directory order.
func (c *FanqieClient) Fetch(params FetchParams) (fetch_result any, fetch_err error) {
	if c == nil {
		return nil, errors.New("fanqienovel client is not initialized")
	}
	raw_url := strings.TrimSpace(params.URL)
	c.cache_source_url = raw_url
	c.force_refresh = params.ForceRefresh
	c.fetch_context = params.Context
	if c.fetch_context == nil {
		c.fetch_context = context.Background()
	}
	progress := FetchProgress{
		RequestID: strings.TrimSpace(params.RequestID),
		Platform:  "fanqienovel",
		URL:       raw_url,
		Stage:     FetchStageStart,
		Status:    FetchStatusRunning,
		Message:   "正在准备获取番茄小说",
	}
	c.report_progress(progress)
	defer func() {
		if fetch_err == nil {
			return
		}
		if errors.Is(fetch_err, ErrFetchInterrupted) || errors.Is(fetch_err, context.Canceled) {
			progress.Stage = FetchStageInterrupted
			progress.Status = FetchStatusInterrupted
			progress.Message = "已中断获取番茄小说"
		} else {
			progress.Stage = FetchStageFailed
			progress.Status = FetchStatusFailed
			progress.Message = "获取番茄小说失败"
		}
		progress.Error = fetch_err.Error()
		c.report_progress(progress)
	}()

	book_id, err := parse_book_id(raw_url)
	if err != nil {
		return nil, err
	}
	progress.BookID = book_id
	if err := c.check_fetch_interrupted(); err != nil {
		return nil, err
	}
	progress.Stage = FetchStageProfile
	progress.Message = "正在获取小说信息和章节目录"
	c.report_progress(progress)

	profile_response, err := c.FetchBookProfile(book_id)
	if err != nil {
		return nil, fmt.Errorf("fetch profile: %w", normalize_fetch_error(err))
	}
	if profile_response == nil || profile_response.Data == nil {
		return nil, errors.New("fetch profile: empty response")
	}
	profile := profile_response.Data
	profile.URL = raw_url
	progress.BookTitle = profile.Title
	progress.Cached = profile_response.Cached
	if profile_response.Cached {
		progress.CacheHits++
	}

	total := 0
	for _, volume := range profile.Volumes {
		total += len(volume.Chapters)
	}
	profile.ChapterCount = total
	chapters := make([]FanqieFetchedChapter, 0, total)
	progress.Stage = FetchStageDirectory
	progress.Status = FetchStatusCompleted
	progress.Total = total
	progress.Message = fmt.Sprintf("章节目录获取完成，共 %d 章", total)
	progress.Profile = profile
	if profile_response.Cached {
		progress.Message = fmt.Sprintf("已复用章节目录缓存，共 %d 章", total)
	}
	c.report_progress(progress)

	chapter_index := 0
	for _, volume := range profile.Volumes {
		for _, chapter := range volume.Chapters {
			if err := c.check_fetch_interrupted(); err != nil {
				return nil, err
			}
			chapter_index++
			chapter_id, chapter_id_err := parse_chapter_id(chapter.URL)
			if chapter_id_err != nil {
				return nil, fmt.Errorf("fetch chapter %q: %w", chapter.Title, chapter_id_err)
			}
			progress.Stage = FetchStageChapter
			progress.Status = FetchStatusRunning
			progress.Current = chapter_index - 1
			progress.Percent = fetch_percent(progress.Current, total)
			progress.VolumeTitle = volume.Title
			progress.ChapterID = chapter_id
			progress.ChapterTitle = chapter.Title
			progress.Cached = false
			progress.Chapter = nil
			progress.Message = fmt.Sprintf("正在获取章节 %d/%d：%s", chapter_index, total, chapter.Title)
			c.report_progress(progress)

			chapter_response, fetch_err := c.FetchBookChapterProfile(chapter_id)
			if fetch_err != nil {
				return nil, fmt.Errorf("fetch chapter %q: %w", chapter.Title, normalize_fetch_error(fetch_err))
			}
			if chapter_response == nil || chapter_response.Data == nil {
				return nil, fmt.Errorf("fetch chapter %q: empty response", chapter.Title)
			}

			chapter_profile := chapter_response.Data
			chapter_title := strings.TrimSpace(chapter_profile.Title)
			if chapter_title == "" {
				chapter_title = chapter.Title
			}
			fetched_chapter := FanqieFetchedChapter{
				Idx:         chapter_index,
				VolumeIdx:   volume.Idx,
				VolumeTitle: volume.Title,
				URL:         c.absolute_url(chapter.URL),
				Title:       chapter_title,
				PublishAt:   chapter_profile.PublishAt,
				Content:     chapter_profile.Content,
				WordCount:   chapter_profile.WordCount,
			}
			chapters = append(chapters, fetched_chapter)
			progress.Status = FetchStatusCompleted
			progress.Current = chapter_index
			progress.Percent = fetch_percent(chapter_index, total)
			progress.ChapterTitle = chapter_title
			progress.Cached = chapter_response.Cached
			progress.Chapter = &fetched_chapter
			if chapter_response.Cached {
				progress.CacheHits++
				progress.Message = fmt.Sprintf("已复用章节缓存 %d/%d：%s", chapter_index, total, chapter_title)
			} else {
				progress.Message = fmt.Sprintf("章节获取完成 %d/%d：%s", chapter_index, total, chapter_title)
			}
			c.report_progress(progress)
		}
	}

	progress.Stage = FetchStageComplete
	progress.Status = FetchStatusCompleted
	progress.Current = total
	progress.Percent = 100
	progress.VolumeTitle = ""
	progress.ChapterID = ""
	progress.ChapterTitle = ""
	progress.Profile = nil
	progress.Chapter = nil
	progress.Message = fmt.Sprintf("小说获取完成，共 %d 章", total)
	progress.Cached = false
	c.report_progress(progress)
	return &FanqieFetchResult{Profile: profile, Chapters: chapters}, nil
}

// FetchBookProfile fetches and parses a Fanqie book profile page.
func (c *FanqieClient) FetchBookProfile(book_id string) (*FanqieResp[*FanqieBookProfile], error) {
	book_id = strings.TrimSpace(book_id)
	if !is_numeric_id(book_id) {
		return nil, fmt.Errorf("invalid book id %q", book_id)
	}

	request_url := strings.TrimRight(c.base_url, "/") + "/page/" + book_id
	document, cached, err := c.fetch_document(request_url, strings.TrimRight(c.base_url, "/")+"/")
	if err != nil {
		return nil, err
	}

	profile := &FanqieBookProfile{
		URL:         request_url,
		Title:       strings.TrimSpace(document.Find(".info-name").First().Text()),
		Description: strings.TrimSpace(document.Find(".page-abstract-content").First().Text()),
		Slogan:      strings.TrimSpace(document.Find(".page-abstract-header").First().Text()),
		CoverURL:    c.absolute_url(json_ld_cover_url(document)),
		Tags: document.Find(".info-label .info-label-grey").Map(func(_ int, selection *goquery.Selection) string {
			return strings.TrimSpace(selection.Text())
		}),
		Author: FanqieAuthor{
			Name:      strings.TrimSpace(document.Find(".author-name-text").First().Text()),
			Desc:      strings.TrimSpace(document.Find(".author-desc").First().Text()),
			AvatarURL: c.absolute_url(document.Find(".author-img").First().AttrOr("src", "")),
			URL:       c.absolute_url(document.Find(".author-name").First().AttrOr("href", "")),
		},
		LatestChapter: FanqieBookChapter{
			Idx:   1,
			Title: strings.TrimSpace(strings.TrimPrefix(document.Find(".info-last-title").First().Text(), "最新章节：")),
			URL:   c.absolute_url(document.Find(".chapter-name").First().AttrOr("href", "")),
		},
	}
	if profile.Title == "" {
		_ = c.remove_cached_html(request_url)
		return nil, errors.New("fanqienovel profile title is empty")
	}

	latest_update_at := strings.TrimSpace(document.Find(".info-last .info-last-time").First().Text())
	if latest_update_at != "" {
		if parsed_time, parse_err := time.ParseInLocation("2006-01-02 15:04:05", latest_update_at, time.Local); parse_err == nil {
			profile.LatestUpdateAt = &parsed_time
		}
	}

	document.Find(".page-directory-content > div").Each(func(volume_index int, selection *goquery.Selection) {
		volume := FanqieBookVolume{
			Idx:      volume_index + 1,
			Title:    strings.TrimSpace(selection.Find(".volume").First().Text()),
			Chapters: make([]FanqieBookChapter, 0),
		}
		selection.Find(".chapter-item").Each(func(chapter_index int, chapter_selection *goquery.Selection) {
			chapter_link := chapter_selection.Find(".chapter-item-title").First()
			volume.Chapters = append(volume.Chapters, FanqieBookChapter{
				Idx:   chapter_index + 1,
				Title: strings.TrimSpace(chapter_link.Text()),
				URL:   c.absolute_url(chapter_link.AttrOr("href", "")),
			})
		})
		profile.ChapterCount += len(volume.Chapters)
		profile.Volumes = append(profile.Volumes, volume)
	})

	return &FanqieResp[*FanqieBookProfile]{Data: profile, Cached: cached}, nil
}

func json_ld_cover_url(document *goquery.Document) string {
	if document == nil {
		return ""
	}

	images_urls := make([]string, 0)
	image_urls := make([]string, 0)
	document.Find(`script[type="application/ld+json"]`).Each(func(_ int, selection *goquery.Selection) {
		raw_json := strings.TrimSpace(selection.Text())
		if raw_json == "" {
			return
		}

		var json_value any
		if err := json.Unmarshal([]byte(raw_json), &json_value); err != nil {
			return
		}
		collect_json_ld_property_urls(json_value, "images", &images_urls)
		collect_json_ld_property_urls(json_value, "image", &image_urls)
	})

	for _, candidate_url := range append(images_urls, image_urls...) {
		if candidate_url = strings.TrimSpace(candidate_url); candidate_url != "" {
			return candidate_url
		}
	}
	return ""
}

func collect_json_ld_property_urls(json_value any, property_name string, urls *[]string) {
	switch typed_value := json_value.(type) {
	case []any:
		for _, item := range typed_value {
			collect_json_ld_property_urls(item, property_name, urls)
		}
	case map[string]any:
		for key, property_value := range typed_value {
			if strings.EqualFold(key, property_name) {
				append_json_ld_urls(property_value, urls)
			}
			if strings.EqualFold(key, "@graph") {
				collect_json_ld_property_urls(property_value, property_name, urls)
			}
		}
	}
}

func append_json_ld_urls(json_value any, urls *[]string) {
	switch typed_value := json_value.(type) {
	case string:
		*urls = append(*urls, typed_value)
	case []any:
		for _, item := range typed_value {
			append_json_ld_urls(item, urls)
		}
	case map[string]any:
		for _, property_name := range []string{"url", "contentUrl", "@id"} {
			if property_value, exists := typed_value[property_name]; exists {
				append_json_ld_urls(property_value, urls)
			}
		}
	}
}

// FetchBookChapterProfile fetches and decodes a Fanqie chapter page.
func (c *FanqieClient) FetchBookChapterProfile(chapter_id string) (*FanqieResp[*FanqieBookChapterProfile], error) {
	chapter_id = strings.TrimSpace(chapter_id)
	if !is_numeric_id(chapter_id) {
		return nil, fmt.Errorf("invalid chapter id %q", chapter_id)
	}

	request_url := strings.TrimRight(c.base_url, "/") + "/reader/" + chapter_id + "?enter_from=page"
	document, cached, err := c.fetch_document(request_url, strings.TrimRight(c.base_url, "/")+"/")
	if err != nil {
		return nil, err
	}

	profile := &FanqieBookChapterProfile{
		Title:     strings.TrimSpace(document.Find(".muye-reader-title").First().Text()),
		WordCount: strings.TrimSpace(strings.TrimPrefix(document.Find(".desc-item").First().Text(), "总字数：")),
	}
	publish_at := strings.TrimSpace(strings.TrimPrefix(document.Find(".desc-item").Eq(1).Text(), "发布时间："))
	if publish_at != "" {
		if parsed_time, parse_err := time.ParseInLocation("2006-01-02 15:04:05", publish_at, time.Local); parse_err == nil {
			profile.PublishAt = &parsed_time
		}
	}

	paragraphs := make([]string, 0)
	document.Find(".muye-reader-content > div > p").Each(func(_ int, selection *goquery.Selection) {
		paragraph := strings.TrimSpace(translate_text(selection.Text()))
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	})
	profile.Content = strings.Join(paragraphs, "\n")

	return &FanqieResp[*FanqieBookChapterProfile]{Data: profile, Cached: cached}, nil
}

func (c *FanqieClient) fetch_document(request_url string, referer string) (*goquery.Document, bool, error) {
	if c.claw_client == nil {
		if c.claw_client_err != nil {
			return nil, false, fmt.Errorf("initialize fanqienovel clawreq client: %w", c.claw_client_err)
		}
		return nil, false, errors.New("fanqienovel clawreq client is not initialized")
	}
	if err := c.check_fetch_interrupted(); err != nil {
		return nil, false, err
	}
	if !c.force_refresh {
		cached_data, cached, err := c.read_cached_html(request_url)
		if err != nil {
			return nil, false, err
		}
		if cached {
			document, parse_err := goquery.NewDocumentFromReader(bytes.NewReader(cached_data))
			if parse_err != nil {
				_ = c.remove_cached_html(request_url)
				return nil, false, fmt.Errorf("parse cached fanqienovel html: %w", parse_err)
			}
			return document, true, nil
		}
	}
	request_options := []clawreq.RequestOption{
		clawreq.WithReferer(referer),
		clawreq.WithHeaders(map[string]string{
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Cache-Control":   "no-cache",
			"Pragma":          "no-cache",
		}),
	}
	if c.cookie != "" {
		request_options = append(request_options, clawreq.WithCookie(c.cookie))
	}

	response, err := c.claw_client.Get(c.fetch_context, request_url, request_options...)
	if err != nil {
		return nil, false, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, false, fmt.Errorf("fanqienovel returned HTTP %d", response.StatusCode)
	}
	html_text, err := response.Text()
	if err != nil {
		return nil, false, err
	}
	html_data := []byte(html_text)
	if err := c.write_cached_html(request_url, html_data); err != nil {
		return nil, false, err
	}
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(html_data))
	if err != nil {
		_ = c.remove_cached_html(request_url)
		return nil, false, err
	}
	return document, false, nil
}

func (c *FanqieClient) absolute_url(reference string) string {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return ""
	}
	parsed_reference, err := url.Parse(reference)
	if err != nil {
		return reference
	}
	parsed_base, err := url.Parse(c.base_url)
	if err != nil {
		return reference
	}
	return parsed_base.ResolveReference(parsed_reference).String()
}

func parse_book_id(raw_url string) (string, error) {
	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return "", ErrUnsupportedURL
	}
	if parsed_url.Scheme != "http" && parsed_url.Scheme != "https" {
		return "", ErrUnsupportedURL
	}
	hostname := strings.ToLower(parsed_url.Hostname())
	if hostname != "fanqienovel.com" && hostname != "www.fanqienovel.com" {
		return "", ErrUnsupportedURL
	}
	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	if len(path_parts) != 2 || path_parts[0] != "page" || !is_numeric_id(path_parts[1]) {
		return "", ErrUnsupportedURL
	}
	return path_parts[1], nil
}

func parse_chapter_id(raw_url string) (string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return "", errors.New("invalid chapter url")
	}
	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	if len(path_parts) == 0 {
		return "", errors.New("chapter id is empty")
	}
	chapter_id := path_parts[len(path_parts)-1]
	if !is_numeric_id(chapter_id) {
		return "", fmt.Errorf("invalid chapter id %q", chapter_id)
	}
	return chapter_id, nil
}

func is_numeric_id(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

func translate_text(value string) string {
	return DecodeText(value).Text
}
