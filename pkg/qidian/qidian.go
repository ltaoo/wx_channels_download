package qidian

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var brRE = regexp.MustCompile(`(?i)<br\s*/?>`)

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
		BaseURL:    BaseURL,
		UserAgent:  DefaultUserAgent(),
	}
}

func DefaultUserAgent() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
}

func ParseURL(rawURL string) (URLParts, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return URLParts{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "qidian.com" && host != "www.qidian.com" && host != "m.qidian.com" {
		return URLParts{}, false
	}
	segments := splitPath(parsed)
	if len(segments) >= 2 && segments[0] == "book" && strings.TrimSpace(segments[1]) != "" {
		bookID := strings.TrimSpace(segments[1])
		return URLParts{
			BookID:    bookID,
			Canonical: CanonicalBookURL(bookID),
		}, true
	}
	return URLParts{}, false
}

func CanonicalBookURL(bookID string) string {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return ""
	}
	return BaseURL + "/book/" + url.PathEscape(bookID) + "/"
}

func FetchBookProfile(id string) (*BookProfile, error) {
	return NewClient(nil).FetchBookProfile(id)
}

func (c *Client) FetchBookProfile(id string) (*BookProfile, error) {
	reqURL := strings.TrimRight(firstNonEmpty(c.BaseURL, BaseURL), "/") + "/book/" + strings.TrimSpace(id) + "/"
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", BaseURL+"/")
	req.Header.Set("User-Agent", firstNonEmpty(c.UserAgent, DefaultUserAgent()))
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
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseBookProfile(reqURL, body)
}

func ParseBookProfile(reqURL string, body []byte) (*BookProfile, error) {
	if root, err := ParsePageContext(body); err == nil {
		profile := bookProfileFromPageContext(reqURL, root)
		profile.PageContext = root
		profile.PageContextJSON = root.Raw
		profile.PageHTML = string(body)
		if htmlProfile, err := parseHTMLBookProfile(reqURL, bytes.NewReader(body)); err == nil {
			mergeHTMLFallback(profile, htmlProfile)
		}
		return profile, nil
	}
	profile, err := parseHTMLBookProfile(reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	profile.PageHTML = string(body)
	return profile, nil
}

func ExtractPageContextJSON(body []byte) (json.RawMessage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(doc.Find(`script#vite-plugin-ssr_pageContext[type="application/json"]`).First().Text())
	if raw == "" {
		return nil, fmt.Errorf("missing qidian page context json")
	}
	if !json.Valid([]byte(raw)) {
		var payload any
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return nil, fmt.Errorf("invalid qidian page context json: %w", err)
		}
		return nil, fmt.Errorf("invalid qidian page context json")
	}
	return json.RawMessage(append([]byte(nil), raw...)), nil
}

func ParsePageContext(body []byte) (*PageContextRoot, error) {
	raw, err := ExtractPageContextJSON(body)
	if err != nil {
		return nil, err
	}
	var root PageContextRoot
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	root.Raw = raw
	return &root, nil
}

func bookProfileFromPageContext(reqURL string, root *PageContextRoot) *BookProfile {
	pageData := root.PageContext.PageProps.PageData
	info := pageData.BookInfo
	bookID := int64String(info.BookID)
	if bookID == "" {
		if parts, ok := ParseURL(reqURL); ok {
			bookID = parts.BookID
		}
	}
	authorID := int64String(firstNonZero(info.AuthorID, pageData.AuthorInfo.AuthorID))
	authorName := firstNonEmpty(info.AuthorName, pageData.AuthorInfo.AuthorName, pageData.AuthorInfo.Name, pageData.AuthorInfo.AuthorNickName)
	profile := &BookProfile{
		URL:              firstNonEmpty(CanonicalBookURL(bookID), reqURL),
		Title:            info.BookName,
		Description:      htmlFragmentText(info.Desc),
		Slogan:           firstNonEmpty(info.ActionStatus, info.SignStatus),
		LatestUpdateAt:   qidianUpdateTime(info.UpdTimes, firstRecentUpdateTime(pageData.RecentChapters)),
		Tags:             qidianTags(pageData),
		ChapterCount:     firstNonZeroInt(pageData.ChapterCount, len(pageData.RecentChapters)),
		WordCount:        info.WordsCnt,
		DisplayWordCount: info.ShowWordsCnt,
		Category:         info.ChanName,
		SubCategory:      info.SubCateName,
		Status:           firstNonEmpty(info.BookStatus, info.ActionStatus),
		Author: Author{
			ID:     authorID,
			Name:   authorName,
			URL:    authorURL(authorID),
			Avatar: normalizeQidianURL(pageData.AuthorInfo.Avatar),
			Desc:   pageData.AuthorInfo.Desc,
		},
	}
	profile.LatestChapter = Chapter{
		Idx:   1,
		Title: info.UpdChapterName,
		URL:   normalizeQidianURL(info.UpdChapterURL),
	}
	if profile.LatestChapter.Title == "" && len(pageData.RecentChapters) > 0 {
		profile.LatestChapter = recentChapterToChapter(bookID, 1, pageData.RecentChapters[0])
	}
	if len(pageData.RecentChapters) > 0 {
		volume := BookVolume{Idx: 1, Title: "最近更新"}
		for i, chapter := range pageData.RecentChapters {
			volume.Chapters = append(volume.Chapters, recentChapterToChapter(bookID, i+1, chapter))
		}
		profile.Volumes = []BookVolume{volume}
	}
	if profile.CoverURL == "" && bookID != "" {
		profile.CoverURL = "https://bookcover.yuewen.com/qdbimg/349573/" + bookID + "/180"
	}
	return profile
}

func parseHTMLBookProfile(reqURL string, r io.Reader) (*BookProfile, error) {
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
	if profile.Description == "" {
		profile.Description = strings.TrimSpace(doc.Find(`meta[property="og:description"]`).AttrOr("content", ""))
	}
	authorSection := doc.Find(".author-intro")
	profile.Author.Name = strings.TrimSpace(authorSection.Find(".writer-name").Text())
	if profile.Author.Name == "" {
		profile.Author.Name = strings.TrimSpace(doc.Find(".writer-name, .author a, .book-author").First().Text())
	}
	if profile.Author.Name == "" {
		profile.Author.Name = strings.TrimSpace(doc.Find(`meta[property="og:novel:author"]`).AttrOr("content", ""))
	}
	profile.Author.URL = normalizeQidianURL(firstNonEmpty(
		authorSection.Find(".writer-name").AttrOr("href", ""),
		doc.Find(`meta[property="og:novel:author_link"]`).AttrOr("content", ""),
	))
	profile.CoverURL = normalizeQidianURL(firstNonEmpty(
		doc.Find("#bookImg img").AttrOr("src", ""),
		doc.Find(".book-detail-img img").AttrOr("src", ""),
		doc.Find(".book-img img").AttrOr("src", ""),
		doc.Find(".detail__header-cover__img").AttrOr("data-src", ""),
		doc.Find(`meta[property="og:image"]`).AttrOr("content", ""),
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
	if profile.LatestChapter.Title == "" {
		profile.LatestChapter.Title = strings.TrimSpace(doc.Find(`meta[property="og:novel:latest_chapter_name"]`).AttrOr("content", ""))
		profile.LatestChapter.URL = normalizeQidianURL(doc.Find(`meta[property="og:novel:latest_chapter_url"]`).AttrOr("content", ""))
	}
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

func mergeHTMLFallback(profile, fallback *BookProfile) {
	if profile == nil || fallback == nil {
		return
	}
	profile.Title = firstNonEmpty(profile.Title, fallback.Title)
	profile.Description = firstNonEmpty(profile.Description, fallback.Description)
	profile.Slogan = firstNonEmpty(profile.Slogan, fallback.Slogan)
	profile.CoverURL = firstNonEmpty(profile.CoverURL, fallback.CoverURL)
	if profile.LatestUpdateAt.IsZero() {
		profile.LatestUpdateAt = fallback.LatestUpdateAt
	}
	if len(profile.Tags) == 0 {
		profile.Tags = fallback.Tags
	}
	if profile.LatestChapter.Title == "" {
		profile.LatestChapter = fallback.LatestChapter
	}
	if profile.ChapterCount == 0 {
		profile.ChapterCount = fallback.ChapterCount
	}
	if profile.Author.Name == "" {
		profile.Author = fallback.Author
	} else {
		profile.Author.URL = firstNonEmpty(profile.Author.URL, fallback.Author.URL)
		profile.Author.Avatar = firstNonEmpty(profile.Author.Avatar, fallback.Author.Avatar)
	}
	if len(profile.Volumes) == 0 {
		profile.Volumes = fallback.Volumes
	}
}

func recentChapterToChapter(bookID string, idx int, item RecentChapter) Chapter {
	chapterID := int64String(item.ID)
	chapterURL := normalizeQidianURL(item.URL)
	if chapterURL == "" && bookID != "" && chapterID != "" {
		chapterURL = BaseURL + "/chapter/" + url.PathEscape(bookID) + "/" + url.PathEscape(chapterID) + "/"
	}
	return Chapter{
		Idx:   idx,
		Title: item.Name,
		URL:   chapterURL,
	}
}

func qidianTags(pageData PageData) []string {
	var tags []string
	add := func(values ...string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			exists := false
			for _, existing := range tags {
				if existing == value {
					exists = true
					break
				}
			}
			if !exists {
				tags = append(tags, value)
			}
		}
	}
	add(pageData.BookInfo.ChanName, pageData.BookInfo.SubCateName, pageData.BookInfo.BookStatus, pageData.BookInfo.SignStatus, pageData.BookExtra.TagInfo.RankName)
	for _, tag := range pageData.BookExtra.UGCTagInfos {
		add(firstNonEmpty(tag.TagName, tag.TagName2))
	}
	for _, role := range pageData.Roles {
		add(role.RoleName)
	}
	return tags
}

func firstRecentUpdateTime(chapters []RecentChapter) string {
	if len(chapters) == 0 {
		return ""
	}
	return firstNonEmpty(chapters[0].UpdateTime2, chapters[0].UpdateTime)
}

func qidianUpdateTime(unixSeconds int64, text string) time.Time {
	if unixSeconds > 0 {
		return time.Unix(unixSeconds, 0)
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(text), time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func htmlFragmentText(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	fragment = brRE.ReplaceAllString(fragment, "\n")
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + fragment + "</div>"))
	if err != nil {
		return html.UnescapeString(fragment)
	}
	return strings.TrimSpace(doc.Find("div").First().Text())
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
		return BaseURL + value
	}
	return value
}

func authorURL(authorID string) string {
	authorID = strings.TrimSpace(authorID)
	if authorID == "" {
		return ""
	}
	return BaseURL + "/author/" + url.PathEscape(authorID) + "/"
}

func splitPath(parsed *url.URL) []string {
	if parsed == nil {
		return nil
	}
	pathValue := strings.Trim(parsed.EscapedPath(), "/")
	if pathValue == "" {
		return nil
	}
	return strings.Split(pathValue, "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func int64String(value int64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
