package zhihu

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/minib"
)

const (
	Protocol  = "zhihu"
	SourceURL = "https://www.zhihu.com/"
)

const current_user_api_url = "https://www.zhihu.com/api/v4/me"

var answer_url_re = regexp.MustCompile(`^/question/([0-9]+|undefined)/answer/([0-9]+)$`)
var question_url_re = regexp.MustCompile(`^/question/([0-9]+)$`)
var article_url_re = regexp.MustCompile(`^/p/([0-9]+)$`)
var article_appview_url_re = regexp.MustCompile(`^/appview/p/([0-9]+)$`)
var collection_id_re = regexp.MustCompile(`^[0-9]+$`)
var collection_url_re = regexp.MustCompile(`^/collection/([0-9]+)$`)
var collection_list_url_re = regexp.MustCompile(`^/people/([^/]+)/collections/?$`)
var collection_total_count_re = regexp.MustCompile(`([0-9][0-9,.]*\s*[万亿]?)\s*条内容`)
var collection_metadata_re = regexp.MustCompile(`([0-9]{4}-[0-9]{2}-[0-9]{2})\s*更新\s*·\s*([0-9][0-9,.]*\s*[万亿]?)\s*条内容\s*·\s*([0-9][0-9,.]*\s*[万亿]?)\s*人关注`)

const collection_page_size = 20

type Client struct {
	http_client         *http.Client
	cookie_reader       *cookies.Reader
	logger              *zerolog.Logger
	file_cache          *cache.CacheProvider
	profile_api_fetcher func(endpoint string, referer string) ([]byte, error)
	pcweb_zse_mutex     sync.RWMutex
	pcweb_zse_cookie    string
	OnProgress          func(downloaded int64)
}

func (c *Client) Fetch(raw_url string) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("知乎URL不能为空")
	}

	resolved_url := ResolveRealURL(raw_url)
	if article_url, ok := ParseArticleURL(resolved_url); ok {
		return c.FetchArticlePage(article_url.Canonical)
	}
	if question_url, ok := ParseQuestionURL(resolved_url); ok {
		return c.FetchQuestionPage(question_url.Canonical)
	}
	if answer_url, ok := ParseAnswerURL(resolved_url); ok {
		return c.FetchAnswerPage(answer_url.Canonical)
	}
	user_content_url, matched, err := parse_user_content_list_url(resolved_url)
	if matched {
		if err != nil {
			return nil, err
		}
		return c.fetch_user_content_list(user_content_url)
	}
	return nil, fmt.Errorf("不支持的知乎URL: %s", raw_url)
}

func ParseAnswerURL(raw_url string) (AnswerURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return AnswerURL{}, false
	}
	if !strings.EqualFold(parsed.Hostname(), "www.zhihu.com") {
		return AnswerURL{}, false
	}
	matches := answer_url_re.FindStringSubmatch(parsed.EscapedPath())
	if len(matches) != 3 {
		return AnswerURL{}, false
	}
	question_id := matches[1]
	answer_id := matches[2]
	canonical := canonical_answer_url(question_id, answer_id)
	if question_id == "undefined" {
		question_id = ""
	}
	return AnswerURL{QuestionID: question_id, AnswerID: answer_id, Canonical: canonical}, true
}

func ParseQuestionURL(raw_url string) (QuestionURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return QuestionURL{}, false
	}
	if !strings.EqualFold(parsed.Hostname(), "www.zhihu.com") {
		return QuestionURL{}, false
	}
	matches := question_url_re.FindStringSubmatch(parsed.EscapedPath())
	if len(matches) != 2 {
		return QuestionURL{}, false
	}
	canonical := "https://www.zhihu.com/question/" + matches[1]
	return QuestionURL{QuestionID: matches[1], Canonical: canonical}, true
}

func ParseArticleURL(raw_url string) (ArticleURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ArticleURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	var matches []string
	switch host {
	case "zhuanlan.zhihu.com":
		matches = article_url_re.FindStringSubmatch(parsed.EscapedPath())
	case "zhihu.com", "www.zhihu.com":
		matches = article_url_re.FindStringSubmatch(parsed.EscapedPath())
		if len(matches) != 2 {
			matches = article_appview_url_re.FindStringSubmatch(parsed.EscapedPath())
		}
	default:
		return ArticleURL{}, false
	}
	if len(matches) != 2 {
		return ArticleURL{}, false
	}
	canonical := "https://zhuanlan.zhihu.com/p/" + matches[1]
	return ArticleURL{ArticleID: matches[1], Canonical: canonical}, true
}

func ResolveRealURL(raw_url string) string {
	if strings.HasPrefix(strings.ToLower(raw_url), Protocol+"://") {
		raw_url = raw_url[len(Protocol+"://"):]
		if !strings.HasPrefix(strings.ToLower(raw_url), "http") {
			raw_url = "https://" + raw_url
		}
	}
	return raw_url
}

func canonical_answer_url(question_id, answer_id string) string {
	question_id = strings.TrimSpace(question_id)
	if question_id == "" {
		question_id = "undefined"
	}
	return "https://www.zhihu.com/question/" + question_id + "/answer/" + strings.TrimSpace(answer_id)
}

// NewClient creates a Zhihu scraper client using application-provided
// capabilities. The cookie reader remains owned by the caller and reads the
// latest persistent cookie data on demand.
func NewClient(cookie_reader *cookies.Reader, logger *zerolog.Logger) *Client {
	c := &Client{
		cookie_reader: cookie_reader,
		logger:        logger,
		http_client:   &http.Client{Timeout: 120 * time.Second, Transport: zhihu_http_transport()},
	}
	return c
}

// SetHTTPTimeout overrides the standard HTTP client timeout. It is primarily
// used by lightweight availability checks so startup status cannot hang for the
// longer content-fetch timeout.
func (c *Client) SetHTTPTimeout(timeout time.Duration) {
	if c == nil || timeout <= 0 {
		return
	}
	if c.http_client == nil {
		c.http_client = &http.Client{Transport: zhihu_http_transport()}
	}
	c.http_client.Timeout = timeout
}

// cookie returns the current URL-matched cookie string from cookies.json.
func (c *Client) cookie(raw_url string) string {
	if c != nil && c.cookie_reader != nil {
		cookie_value, err := c.cookie_reader.HeaderForURL(raw_url)
		if err == nil {
			return cookie_value
		}
		if !errors.Is(err, cookies.ErrCookieNotFound) {
			log.Printf("zhihu: failed to read persistent cookies: %v", err)
		}
	}
	return ""
}

func (c *Client) FetchAnswerPage(raw_url string) (*AnswerPage, error) {
	answer_url, ok := ParseAnswerURL(ResolveRealURL(raw_url))
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu answer url")
	}
	body, err := c.do_bytes(http.MethodGet, answer_url.Canonical, answer_url.Canonical)
	if err != nil {
		return nil, err
	}
	page, err := parse_answer_page(body, answer_url)
	if err != nil {
		return nil, err
	}
	page.Source = answer_url.Canonical
	if page.Answer.CommentCount > 0 {
		if comments, err := c.fetch_answer_comments(answer_url); err == nil {
			page.Comments = comments
		}
	}
	return page, nil
}

func (c *Client) FetchQuestionPage(raw_url string) (*QuestionPage, error) {
	question_url, ok := ParseQuestionURL(ResolveRealURL(raw_url))
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu question url")
	}
	body, err := c.do_bytes(http.MethodGet, question_url.Canonical, question_url.Canonical)
	if err != nil {
		return nil, err
	}
	page, err := parse_question_page(body, question_url)
	if err != nil {
		return nil, err
	}
	page.Source = question_url.Canonical
	return page, nil
}

func (c *Client) FetchArticlePage(raw_url string) (*ArticlePage, error) {
	article_url, ok := ParseArticleURL(ResolveRealURL(raw_url))
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu article url")
	}
	body, err := c.do_bytes(http.MethodGet, article_url.Canonical, article_url.Canonical)
	if err != nil {
		return nil, err
	}
	page, err := parse_article_page(body, article_url)
	if err != nil {
		return nil, err
	}
	page.Source = article_url.Canonical
	return page, nil
}

// FetchContentListOfCollection renders one collection page with authenticated
// cookies and returns only the saved answer/article/question title links.
func (c *Client) FetchContentListOfCollection(collection_id string, page int) (*CollectionContentList, error) {
	collection_id = strings.TrimSpace(collection_id)
	if !collection_id_re.MatchString(collection_id) {
		return nil, fmt.Errorf("invalid zhihu collection id %q", collection_id)
	}
	if page < 1 {
		return nil, fmt.Errorf("zhihu collection page must be greater than zero")
	}
	page_url := "https://www.zhihu.com/collection/" + collection_id
	if page > 1 {
		page_url += "?page=" + strconv.Itoa(page)
	}
	rendered_html, final_url, err := c.fetch_rendered_collection_page(page_url)
	if err != nil {
		return nil, fmt.Errorf("fetch zhihu collection %s page %d: %w", collection_id, page, err)
	}
	result, err := parse_collection_content_list(rendered_html, final_url, collection_id, page)
	if err != nil {
		return nil, fmt.Errorf("parse zhihu collection %s page %d: %w", collection_id, page, err)
	}
	return result, nil
}

// FetchCollectionList renders a user's collection page. Authenticated cookies
// allow the owner's private collections to be returned alongside public ones.
func (c *Client) FetchCollectionList(raw_url string) (*CollectionList, error) {
	page_url, owner_url_token, err := normalize_collection_list_url(raw_url)
	if err != nil {
		return nil, err
	}
	rendered_html, final_url, err := c.fetch_rendered_collection_page(page_url)
	if err != nil {
		return nil, fmt.Errorf("fetch zhihu collection list for %s: %w", owner_url_token, err)
	}
	result, err := parse_collection_list(rendered_html, final_url, owner_url_token)
	if err != nil {
		return nil, fmt.Errorf("parse zhihu collection list for %s: %w", owner_url_token, err)
	}
	return result, nil
}

// FetchCurrentUser returns the Zhihu account represented by the current
// persistent login cookies.
func (c *Client) FetchCurrentUser() (*User, error) {
	if c == nil {
		return nil, fmt.Errorf("zhihu client is nil")
	}
	body, err := c.do_bytes(http.MethodGet, current_user_api_url, SourceURL)
	if err != nil {
		return nil, fmt.Errorf("fetch current zhihu user: %w", err)
	}
	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parse current zhihu user: %w", err)
	}
	url_token := strings.TrimSpace(user.URLTokenSnake)
	if url_token == "" {
		url_token = strings.TrimSpace(user.URLToken)
	}
	if url_token == "" {
		return nil, fmt.Errorf("current zhihu user response has no url_token")
	}
	user.URLToken = url_token
	user.URLTokenSnake = url_token
	return &user, nil
}

// FetchContentListOfUser fetches one page from a user's answers, posts,
// zvideos, or columns profile tab. The tab is selected from raw_url.
func (c *Client) FetchContentListOfUser(raw_url string, page int) (*UserContentList, error) {
	user_content_url, matched, err := parse_user_content_list_url(raw_url)
	if !matched {
		return nil, fmt.Errorf("unsupported zhihu user content list URL")
	}
	if err != nil {
		return nil, err
	}
	if page < 1 || page > max_user_content_page {
		return nil, fmt.Errorf("zhihu user content page must be between 1 and %d", max_user_content_page)
	}
	user_content_url.Page = page
	user_content_url.Canonical = canonical_user_content_list_url(
		user_content_url.OwnerURLToken,
		user_content_url.Kind,
		page,
	)
	return c.fetch_user_content_list(user_content_url)
}

// FetchAnswerListOfUser fetches one page from a user's answers tab.
func (c *Client) FetchAnswerListOfUser(raw_url string, page int) (*UserContentList, error) {
	return c.fetch_user_content_list_of_kind(raw_url, UserContentKindAnswers, page)
}

// FetchPostListOfUser fetches one page from a user's posts tab.
func (c *Client) FetchPostListOfUser(raw_url string, page int) (*UserContentList, error) {
	return c.fetch_user_content_list_of_kind(raw_url, UserContentKindPosts, page)
}

// FetchZvideoListOfUser fetches one page from a user's video tab.
func (c *Client) FetchZvideoListOfUser(raw_url string, page int) (*UserContentList, error) {
	return c.fetch_user_content_list_of_kind(raw_url, UserContentKindZvideos, page)
}

// FetchColumnListOfUser fetches one page from a user's columns tab.
func (c *Client) FetchColumnListOfUser(raw_url string, page int) (*UserContentList, error) {
	return c.fetch_user_content_list_of_kind(raw_url, UserContentKindColumns, page)
}

func (c *Client) fetch_rendered_collection_page(raw_url string) (string, string, error) {
	if c == nil {
		return "", "", fmt.Errorf("zhihu client is nil")
	}
	timeout := 2 * time.Minute
	if c.http_client != nil && c.http_client.Timeout > 0 {
		timeout = c.http_client.Timeout
	}
	browser, err := minib.NewMiniBrowser(timeout, c.cookie_reader)
	if err != nil {
		return "", "", fmt.Errorf("create minib browser: %w", err)
	}
	defer browser.Close()

	navigation_request, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return "", "", err
	}
	set_zhihu_document_headers(navigation_request, SourceURL)
	navigation_context, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	page, err := browser.Navigate(navigation_context, raw_url, navigation_request.Header, minib.NavigateOptions{
		DisableCache:      true,
		DisableCSS:        true,
		DisableImages:     true,
		DisableMedia:      true,
		JavaScriptTimeout: 20 * time.Second,
		ResourceTimeout:   20 * time.Second,
		WaitUntil:         minib.WaitUntilLoad,
	})
	if err != nil {
		return "", "", err
	}
	if page.StatusCode < http.StatusOK || page.StatusCode >= http.StatusMultipleChoices {
		return "", page.URL, fmt.Errorf("upstream returned HTTP %d", page.StatusCode)
	}
	if strings.Contains(page.URL, "/signin") {
		return "", page.URL, fmt.Errorf("authentication redirected to sign-in")
	}
	rendered_html := strings.TrimSpace(page.RenderedHTML)
	if rendered_html == "" {
		return "", page.URL, fmt.Errorf("rendered page is empty")
	}
	document, parse_err := goquery.NewDocumentFromReader(strings.NewReader(rendered_html))
	if parse_err != nil {
		return "", page.URL, parse_err
	}
	body_text := strings.Join(strings.Fields(document.Find("body").Text()), " ")
	if strings.Contains(body_text, "请登录后查看") {
		return "", page.URL, fmt.Errorf("authenticated cookies are required")
	}
	if strings.Contains(body_text, "安全验证") {
		return "", page.URL, fmt.Errorf("zhihu returned a verification page")
	}
	return rendered_html, page.URL, nil
}

func normalize_collection_list_url(raw_url string) (string, string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Hostname() == "" {
		return "", "", fmt.Errorf("invalid zhihu collection list URL")
	}
	if parsed_url.Scheme != "https" && parsed_url.Scheme != "http" {
		return "", "", fmt.Errorf("unsupported zhihu collection list URL scheme")
	}
	if !strings.EqualFold(parsed_url.Hostname(), "www.zhihu.com") {
		return "", "", fmt.Errorf("unsupported zhihu collection list host %q", parsed_url.Hostname())
	}
	matches := collection_list_url_re.FindStringSubmatch(parsed_url.EscapedPath())
	if len(matches) != 2 {
		return "", "", fmt.Errorf("unsupported zhihu collection list URL")
	}
	owner_url_token, err := url.PathUnescape(matches[1])
	if err != nil || strings.TrimSpace(owner_url_token) == "" {
		return "", "", fmt.Errorf("invalid zhihu collection owner token")
	}
	page_url := "https://www.zhihu.com/people/" + url.PathEscape(owner_url_token) + "/collections"
	return page_url, owner_url_token, nil
}

func parse_collection_content_list(rendered_html string, page_url string, collection_id string, page int) (*CollectionContentList, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(rendered_html))
	if err != nil {
		return nil, err
	}
	base_url, err := url.Parse(page_url)
	if err != nil {
		return nil, err
	}
	items := make([]CollectionContentItem, 0)
	seen_urls := make(map[string]bool)
	document.Find(".CollectionDetailPageItem .ContentItem-title a[href]").Each(func(_ int, selection *goquery.Selection) {
		item, ok := collection_content_item_from_link(base_url, selection.AttrOr("href", ""), selection.Text())
		if !ok || seen_urls[item.URL] {
			return
		}
		seen_urls[item.URL] = true
		items = append(items, item)
	})

	total_count := collection_total_count(document, base_url, collection_id)
	has_next := total_count > page*collection_page_size
	if total_count == 0 && len(items) >= collection_page_size {
		has_next = true
	}
	result := &CollectionContentList{
		CollectionID: collection_id,
		Page:         page,
		Source:       page_url,
		Title:        collection_page_title(document),
		TotalCount:   total_count,
		Items:        items,
		HasNext:      has_next,
	}
	if has_next {
		result.NextPage = page + 1
	}
	return result, nil
}

func collection_content_item_from_link(base_url *url.URL, raw_url string, raw_title string) (CollectionContentItem, bool) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || base_url == nil {
		return CollectionContentItem{}, false
	}
	parsed_url = base_url.ResolveReference(parsed_url)
	parsed_url.Fragment = ""
	resolved_url := parsed_url.String()
	title := strings.TrimSpace(strings.ReplaceAll(raw_title, "\u200b", ""))
	if answer_url, ok := ParseAnswerURL(resolved_url); ok {
		return CollectionContentItem{
			ID:         answer_url.AnswerID,
			Type:       "answer",
			Title:      title,
			URL:        answer_url.Canonical,
			QuestionID: answer_url.QuestionID,
			AnswerID:   answer_url.AnswerID,
		}, true
	}
	if article_url, ok := ParseArticleURL(resolved_url); ok {
		return CollectionContentItem{
			ID:        article_url.ArticleID,
			Type:      "article",
			Title:     title,
			URL:       article_url.Canonical,
			ArticleID: article_url.ArticleID,
		}, true
	}
	if question_url, ok := ParseQuestionURL(resolved_url); ok {
		return CollectionContentItem{
			ID:         question_url.QuestionID,
			Type:       "question",
			Title:      title,
			URL:        question_url.Canonical,
			QuestionID: question_url.QuestionID,
		}, true
	}
	return CollectionContentItem{}, false
}

func parse_collection_list(rendered_html string, page_url string, owner_url_token string) (*CollectionList, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(rendered_html))
	if err != nil {
		return nil, err
	}
	base_url, err := url.Parse(page_url)
	if err != nil {
		return nil, err
	}
	collections := make([]Collection, 0)
	seen_ids := make(map[string]bool)
	document.Find(".SelfCollectionItem").Each(func(_ int, card *goquery.Selection) {
		var item Collection
		card.Find(`a[href*="/collection/"]`).EachWithBreak(func(_ int, selection *goquery.Selection) bool {
			collection_id, collection_url, ok := collection_link(base_url, selection.AttrOr("href", ""))
			if !ok {
				return true
			}
			item.ID = collection_id
			item.URL = collection_url
			item.Title = strings.TrimSpace(strings.ReplaceAll(selection.Text(), "\u200b", ""))
			return false
		})
		if item.ID == "" || seen_ids[item.ID] {
			return
		}
		seen_ids[item.ID] = true
		item.IsPrivate = card.Find("svg.Zi--Lock, .Zi--Lock").Length() > 0
		item.Visibility = CollectionVisibilityPublic
		if item.IsPrivate {
			item.Visibility = CollectionVisibilityPrivate
		}
		metadata_matches := collection_metadata_re.FindStringSubmatch(strings.Join(strings.Fields(card.Text()), " "))
		if len(metadata_matches) == 4 {
			item.UpdatedAt = metadata_matches[1]
			item.ContentCount = parse_zhihu_display_count(metadata_matches[2])
			item.FollowerCount = parse_zhihu_display_count(metadata_matches[3])
		}
		collections = append(collections, item)
	})
	return &CollectionList{
		Source:        page_url,
		OwnerURLToken: owner_url_token,
		Collections:   collections,
	}, nil
}

func collection_link(base_url *url.URL, raw_url string) (string, string, bool) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || base_url == nil {
		return "", "", false
	}
	parsed_url = base_url.ResolveReference(parsed_url)
	if !strings.EqualFold(parsed_url.Hostname(), "www.zhihu.com") {
		return "", "", false
	}
	matches := collection_url_re.FindStringSubmatch(parsed_url.EscapedPath())
	if len(matches) != 2 {
		return "", "", false
	}
	collection_id := matches[1]
	return collection_id, "https://www.zhihu.com/collection/" + collection_id, true
}

func collection_total_count(document *goquery.Document, base_url *url.URL, collection_id string) int {
	if document == nil {
		return 0
	}
	header_count := 0
	document.Find(".CollectionDetailPageHeader").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		header_count = first_collection_total_count(selection.Text())
		return header_count == 0
	})
	if header_count > 0 {
		return header_count
	}

	sidebar_count := 0
	document.Find(".SideBarCollectionItem").EachWithBreak(func(_ int, card *goquery.Selection) bool {
		matches_collection := false
		card.Find(`a[href*="/collection/"]`).EachWithBreak(func(_ int, selection *goquery.Selection) bool {
			linked_id, _, ok := collection_link(base_url, selection.AttrOr("href", ""))
			matches_collection = ok && linked_id == collection_id
			return !matches_collection
		})
		if !matches_collection {
			return true
		}
		sidebar_count = first_collection_total_count(card.Text())
		return false
	})
	return sidebar_count
}

func first_collection_total_count(text string) int {
	matches := collection_total_count_re.FindStringSubmatch(text)
	if len(matches) != 2 {
		return 0
	}
	return parse_zhihu_display_count(matches[1])
}

func parse_zhihu_display_count(raw_count string) int {
	raw_count = strings.ReplaceAll(strings.Join(strings.Fields(raw_count), ""), ",", "")
	if raw_count == "" {
		return 0
	}
	multiplier := float64(1)
	switch {
	case strings.HasSuffix(raw_count, "万"):
		multiplier = 10000
		raw_count = strings.TrimSuffix(raw_count, "万")
	case strings.HasSuffix(raw_count, "亿"):
		multiplier = 100000000
		raw_count = strings.TrimSuffix(raw_count, "亿")
	}
	count, err := strconv.ParseFloat(raw_count, 64)
	if err != nil || count < 0 {
		return 0
	}
	return int(count * multiplier)
}

func collection_page_title(document *goquery.Document) string {
	if document == nil {
		return ""
	}
	if title := strings.TrimSpace(strings.ReplaceAll(document.Find(".CollectionDetailPageHeader-title").First().Text(), "\u200b", "")); title != "" {
		return title
	}
	title := strings.TrimSpace(document.Find("title").First().Text())
	if suffix_index := strings.Index(title, " - 收藏夹 - 知乎"); suffix_index >= 0 {
		title = title[:suffix_index]
	}
	if strings.HasPrefix(title, "(") {
		if prefix_end := strings.Index(title, ") "); prefix_end >= 0 {
			title = title[prefix_end+2:]
		}
	}
	return strings.TrimSpace(title)
}

func (c *Client) do_bytes(method, raw_url, referer string) ([]byte, error) {
	if method != http.MethodGet {
		return nil, fmt.Errorf("unsupported zhihu HTTP method %s", method)
	}
	resolved_url := ResolveRealURL(raw_url)
	cached_html, cached, err := c.read_cached_html(raw_url)
	if err != nil {
		return nil, fmt.Errorf("read cached zhihu HTML response for %q: %w", raw_url, err)
	}
	if cached {
		if cached_zhihu_document_is_usable(cached_html, resolved_url) {
			return cached_html, nil
		}
		_ = c.remove_cached_html(raw_url)
	}
	var fetch_pcweb_document func(string) ([]byte, error)
	if _, is_answer := ParseAnswerURL(resolved_url); is_answer {
		fetch_pcweb_document = c.fetch_pcweb_answer_document
	} else if _, is_article := ParseArticleURL(resolved_url); is_article {
		fetch_pcweb_document = c.fetch_pcweb_article_document
	}
	if fetch_pcweb_document != nil {
		html_data, fetch_err := fetch_pcweb_document(raw_url)
		if fetch_err != nil {
			return nil, fetch_err
		}
		if c.OnProgress != nil {
			c.OnProgress(int64(len(html_data)))
		}
		if _, parse_err := ParseInitialData(html_data); parse_err == nil {
			if err := c.write_cached_html(raw_url, html_data); err != nil {
				return nil, fmt.Errorf("cache zhihu HTML response for %q: %w", raw_url, err)
			}
		}
		return html_data, nil
	}
	req, err := http.NewRequest(method, raw_url, nil)
	if err != nil {
		return nil, err
	}
	set_zhihu_document_headers(req, referer)
	cookie_header := c.cookie(raw_url)
	if cookie_header != "" {
		req.Header.Set("Cookie", cookie_header)
	}
	c.log_request(method, raw_url, cookie_header)
	resp, err := c.http_client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.log_response(method, raw_url, resp.StatusCode)
	html_data, read_err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if read_err != nil {
		return nil, read_err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zhihu HTTP status %d body=%s", resp.StatusCode, debug_snippet(html_data))
	}
	if c.OnProgress != nil {
		c.OnProgress(int64(len(html_data)))
	}
	if _, parse_err := ParseInitialData(html_data); parse_err == nil {
		if err := c.write_cached_html(raw_url, html_data); err != nil {
			return nil, fmt.Errorf("cache zhihu HTML response for %q: %w", raw_url, err)
		}
	}
	return html_data, nil
}

func cached_zhihu_document_is_usable(body []byte, resolved_url string) bool {
	if article_url, is_article := ParseArticleURL(resolved_url); is_article {
		return pcweb_has_article(body, article_url.ArticleID)
	}
	_, parse_err := ParseInitialData(body)
	return parse_err == nil
}

func set_zhihu_document_headers(req *http.Request, referer string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("Sec-CH-UA", `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", zhihu_user_agent)
	if strings.TrimSpace(referer) == "" {
		referer = SourceURL
	}
	req.Header.Set("Referer", referer)
}

func zhihu_http_transport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	return transport
}

func debug_snippet(body []byte) string {
	if len(body) <= 256 {
		return string(body)
	}
	return string(body[:256])
}

func (c *Client) inline_remote_images(content string, referer string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", err
	}
	var first_err error
	doc.Find("img[src]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src, _ := s.Attr("src")
		src = normalize_asset_url(src, referer)
		if src == "" || strings.HasPrefix(src, "data:") {
			return true
		}
		data_uri, err := c.fetch_image_data_uri(src, referer)
		if err != nil {
			if first_err == nil {
				first_err = err
			}
			return true
		}
		s.SetAttr("src", data_uri)
		return true
	})
	if first_err != nil {
		return "", first_err
	}
	out, err := doc.Html()
	if err != nil {
		return "", err
	}
	return "<!doctype html>" + out, nil
}

func (c *Client) InlineRemoteImages(content string, referer string) (string, error) {
	return c.inline_remote_images(content, referer)
}

func (c *Client) LocalizeRemoteVideos(ctx context.Context, content string, referer string, html_path string) (string, error) {
	if strings.TrimSpace(content) == "" || strings.TrimSpace(html_path) == "" {
		return content, nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", err
	}
	assets_dir_name := html_assets_dir_name(html_path)
	assets_dir_path := filepath.Join(filepath.Dir(html_path), assets_dir_name)
	downloaded := make(map[string]string)
	var first_err error
	video_index := 0
	doc.Find("video[src], video source[src]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src, _ := s.Attr("src")
		src = normalize_asset_url(src, referer)
		if src == "" || strings.HasPrefix(src, "data:") || !strings.HasPrefix(src, "http") {
			return true
		}
		local_path, ok := downloaded[src]
		if !ok {
			video_index++
			filename, err := c.download_video(ctx, src, referer, assets_dir_path, video_index)
			if err != nil {
				first_err = err
				return false
			}
			local_path = filepath.ToSlash(filepath.Join(assets_dir_name, filename))
			downloaded[src] = local_path
		}
		s.SetAttr("src", local_path)
		if s.Is("video") {
			ensure_playable_video(s)
		} else {
			s.ParentFiltered("video").Each(func(_ int, video *goquery.Selection) {
				ensure_playable_video(video)
			})
		}
		return true
	})
	if first_err != nil {
		return "", first_err
	}
	out, err := doc.Html()
	if err != nil {
		return "", err
	}
	return "<!doctype html>" + out, nil
}

func ensure_playable_video(s *goquery.Selection) {
	s.SetAttr("controls", "controls")
	if _, ok := s.Attr("preload"); !ok {
		s.SetAttr("preload", "metadata")
	}
	if _, ok := s.Attr("style"); !ok {
		s.SetAttr("style", "max-width:100%;height:auto")
	}
}

func html_assets_dir_name(html_path string) string {
	base := strings.TrimSuffix(filepath.Base(html_path), filepath.Ext(html_path))
	base = strings.TrimSpace(base)
	if base == "" || base == "." {
		base = "zhihu"
	}
	return base + "_files"
}

func (c *Client) download_video(ctx context.Context, raw_url string, referer string, dir string, index int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw_url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "video/mp4,video/webm,video/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36")
	req.Header.Set("Sec-Fetch-Dest", "video")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	cookie_header := c.cookie(raw_url)
	if cookie_header != "" {
		req.Header.Set("Cookie", cookie_header)
	}

	c.log_request(http.MethodGet, raw_url, cookie_header)
	resp, err := c.http_client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	c.log_response(http.MethodGet, raw_url, resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("zhihu video status %d body=%s", resp.StatusCode, debug_snippet(body))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("video_%02d%s", index, video_ext(raw_url, resp.Header.Get("content-type")))
	dest_path := filepath.Join(dir, filename)
	file, err := os.Create(dest_path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := copy_with_client_progress(file, resp.Body, c.OnProgress); err != nil {
		return "", err
	}
	return filename, nil
}

func copy_with_client_progress(dst io.Writer, src io.Reader, on_progress func(int64)) (int64, error) {
	buf := make([]byte, 64*1024)
	var written int64
	for {
		n, read_err := src.Read(buf)
		if n > 0 {
			m, write_err := dst.Write(buf[:n])
			written += int64(m)
			if on_progress != nil && m > 0 {
				on_progress(int64(m))
			}
			if write_err != nil {
				return written, write_err
			}
			if m != n {
				return written, io.ErrShortWrite
			}
		}
		if read_err != nil {
			if read_err == io.EOF {
				return written, nil
			}
			return written, read_err
		}
	}
}

func video_ext(raw_url string, content_type string) string {
	if ext := strings.ToLower(path_ext(raw_url)); valid_media_ext(ext) {
		return ext
	}
	if idx := strings.Index(content_type, ";"); idx >= 0 {
		content_type = strings.TrimSpace(content_type[:idx])
	}
	if exts, err := mime.ExtensionsByType(strings.TrimSpace(content_type)); err == nil {
		for _, ext := range exts {
			if valid_media_ext(ext) {
				return ext
			}
		}
	}
	return ".mp4"
}

func valid_media_ext(ext string) bool {
	switch ext {
	case ".mp4", ".m4v", ".mov", ".webm", ".mkv":
		return true
	default:
		return false
	}
}

func (c *Client) fetch_image_data_uri(raw_url string, referer string) (string, error) {
	body, content_type, err := c.do_image_bytes(raw_url, referer)
	if err != nil {
		return "", err
	}
	if content_type == "" {
		content_type = http.DetectContentType(body)
	}
	if idx := strings.Index(content_type, ";"); idx >= 0 {
		content_type = strings.TrimSpace(content_type[:idx])
	}
	if content_type == "" || content_type == "application/octet-stream" {
		if ext := strings.ToLower(path_ext(raw_url)); ext != "" {
			content_type = mime.TypeByExtension(ext)
		}
	}
	if content_type == "" {
		content_type = "application/octet-stream"
	}
	return "data:" + content_type + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

func (c *Client) do_image_bytes(raw_url string, referer string) ([]byte, string, error) {
	return c.do_image_bytes_with_http(raw_url, referer)
}

func (c *Client) do_image_bytes_with_http(raw_url string, referer string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	cookie_header := c.cookie(raw_url)
	if cookie_header != "" {
		req.Header.Set("Cookie", cookie_header)
	}

	c.log_request(http.MethodGet, raw_url, cookie_header)
	resp, err := c.http_client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	c.log_response(http.MethodGet, raw_url, resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, "", fmt.Errorf("zhihu image status %d body=%s", resp.StatusCode, debug_snippet(body))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err == nil && c.OnProgress != nil {
		c.OnProgress(int64(len(body)))
	}
	return body, resp.Header.Get("content-type"), err
}

func parse_answer_page(body []byte, answer_url AnswerURL) (*AnswerPage, error) {
	initial_data, err := ParseInitialData(body)
	if err != nil {
		return nil, err
	}
	answer := initial_data.InitialState.Entities.Answers[answer_url.AnswerID]
	if answer.ID == "" {
		return nil, fmt.Errorf("missing zhihu answer entity")
	}
	question := initial_data.InitialState.Entities.Questions[answer_url.QuestionID]
	if question.ID == "" && answer.Question.ID != "" {
		question = initial_data.InitialState.Entities.Questions[answer.Question.ID]
	}
	if question.ID == "" && len(initial_data.InitialState.Entities.Questions) == 1 {
		for _, candidate := range initial_data.InitialState.Entities.Questions {
			question = candidate
		}
	}
	if question.ID == "" {
		return nil, fmt.Errorf("missing zhihu question entity")
	}
	page_url := answer_url
	if page_url.QuestionID != question.ID {
		page_url.QuestionID = question.ID
		page_url.Canonical = canonical_answer_url(question.ID, answer.ID)
	}
	if page_url.Canonical == "" {
		page_url.Canonical = canonical_answer_url(page_url.QuestionID, page_url.AnswerID)
	}
	return &AnswerPage{
		URL:             page_url,
		Source:          page_url.Canonical,
		PageHTML:        string(body),
		Question:        question,
		Answer:          answer,
		InitialData:     initial_data,
		InitialDataJSON: initial_data.Raw,
	}, nil
}

func parse_question_page(body []byte, question_url QuestionURL) (*QuestionPage, error) {
	initial_data, err := ParseInitialData(body)
	if err != nil {
		return nil, err
	}
	question := initial_data.InitialState.Entities.Questions[question_url.QuestionID]
	if question.ID == "" {
		return nil, fmt.Errorf("missing zhihu question entity")
	}
	return &QuestionPage{
		URL:             question_url,
		Source:          question_url.Canonical,
		PageHTML:        string(body),
		Question:        question,
		InitialData:     initial_data,
		InitialDataJSON: initial_data.Raw,
	}, nil
}

func parse_article_page(body []byte, article_url ArticleURL) (*ArticlePage, error) {
	initial_data, err := ParseInitialData(body)
	if err != nil {
		return nil, err
	}
	article, ok := article_from_initial_data(initial_data, article_url.ArticleID)
	if !ok {
		return nil, fmt.Errorf("missing zhihu article entity")
	}
	if !article_has_complete_content(article) {
		return nil, fmt.Errorf("zhihu article content is truncated")
	}
	return &ArticlePage{
		URL:             article_url,
		Source:          article_url.Canonical,
		PageHTML:        string(body),
		Article:         article,
		InitialData:     initial_data,
		InitialDataJSON: initial_data.Raw,
	}, nil
}

func (c *Client) fetch_answer_comments(answer_url AnswerURL) ([]Comment, error) {
	comments, err := c.fetch_answer_root_comments(answer_url)
	if err == nil {
		return comments, nil
	}
	return c.fetch_answer_comments_v5(answer_url)
}

func (c *Client) fetch_answer_root_comments(answer_url AnswerURL) ([]Comment, error) {
	endpoint := fmt.Sprintf("/api/v4/answers/%s/root_comments?limit=20&offset=0&order=normal&status=open", url.PathEscape(answer_url.AnswerID))
	var comments []Comment
	for endpoint != "" {
		body, err := c.do_api_bytes(endpoint, answer_url.Canonical)
		if err != nil {
			return nil, err
		}
		var response comment_response
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Data {
			comments = append(comments, item.to_comment())
		}
		if response.Paging.IsEnd || response.Paging.Next == "" || len(comments) >= 200 {
			break
		}
		endpoint = endpoint_from_url(response.Paging.Next)
	}
	return comments, nil
}

func (c *Client) fetch_answer_comments_v5(answer_url AnswerURL) ([]Comment, error) {
	endpoint := fmt.Sprintf("/api/v4/comment_v5/answers/%s/root_comment?order_by=score&limit=20&offset=0&status=open", url.PathEscape(answer_url.AnswerID))
	body, err := c.do_api_bytes(endpoint, answer_url.Canonical)
	if err != nil {
		return nil, err
	}
	var response comment_response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	comments := make([]Comment, 0, len(response.Data))
	for _, item := range response.Data {
		comments = append(comments, item.to_comment())
	}
	return comments, nil
}

func (c *Client) do_api_bytes(endpoint, referer string) ([]byte, error) {
	if !strings.HasPrefix(endpoint, "/") {
		return nil, fmt.Errorf("invalid zhihu api endpoint")
	}
	api_url := SourceURL + strings.TrimPrefix(endpoint, "/")
	req, err := http.NewRequest(http.MethodGet, api_url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", zhihu_user_agent)
	req.Header.Set("X-Requested-With", "fetch")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	cookie_header := c.cookie(api_url)
	if cookie_header != "" {
		req.Header.Set("Cookie", cookie_header)
	}
	c.log_request(http.MethodGet, api_url, cookie_header)
	resp, err := c.http_client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.log_response(http.MethodGet, api_url, resp.StatusCode)
	body, read_err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if read_err != nil {
		return nil, read_err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zhihu api status %d body=%s", resp.StatusCode, debug_snippet(body))
	}
	if c.OnProgress != nil {
		c.OnProgress(int64(len(body)))
	}
	return body, nil
}

func endpoint_from_url(raw_url string) string {
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Hostname(), "www.zhihu.com") {
		return ""
	}
	if parsed.RawQuery == "" {
		return parsed.EscapedPath()
	}
	return parsed.EscapedPath() + "?" + parsed.RawQuery
}

func (p comment_payload) to_comment() Comment {
	created := p.Created
	if created == 0 {
		created = p.CreatedAt
	}
	content := first_non_empty(p.Content, p.ContentTag)
	comment := Comment{
		ID:          raw_id_string(p.ID),
		ContentHTML: content,
		ContentText: html_to_text(content),
		CreatedTime: created,
		Author:      p.Author,
		ReplyTo:     p.ReplyTo,
	}
	for _, child := range p.Child {
		comment.Replies = append(comment.Replies, child.to_comment())
	}
	return comment
}

func raw_id_string(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	return strings.Trim(string(raw), `"`)
}

func best_zhihu_image_src(s *goquery.Selection) string {
	for _, attr := range []string{"data-original", "data-actualsrc", "data-default-watermark-src", "src"} {
		value, ok := s.Attr(attr)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" || is_placeholder_image(value) {
			continue
		}
		return value
	}
	return ""
}

func FirstImageURL(fragment string, base string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"wx-zhihu-root\">" + fragment + "</div>"))
	if err != nil {
		return ""
	}
	var image_url string
	doc.Find("#wx-zhihu-root img").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src := normalize_asset_url(best_zhihu_image_src(s), base)
		if src == "" || strings.HasPrefix(src, "data:") {
			return true
		}
		image_url = src
		return false
	})
	return image_url
}

func is_placeholder_image(raw_url string) bool {
	lower := strings.ToLower(raw_url)
	return strings.Contains(lower, "data:image/svg") ||
		strings.Contains(lower, "placeholder") ||
		strings.Contains(lower, "loading") ||
		strings.Contains(lower, "blank")
}

func html_to_text(fragment string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fragment))
	if err != nil {
		return strings.TrimSpace(fragment)
	}
	return strings.TrimSpace(doc.Text())
}

func display_name(user User) string {
	return first_non_empty(user.Name, user.URLToken, user.URLTokenSnake, user.ID, "匿名用户")
}

func avatar_url(user User) string {
	return first_non_empty(user.AvatarURL, user.AvatarURLSnake, user.AvatarURLTemplate)
}

func UserDisplayName(user User) string {
	return display_name(user)
}

func UserAvatarURL(user User) string {
	return avatar_url(user)
}

func UserURL(user User) string {
	return author_url(user)
}

func author_url(user User) string {
	token := first_non_empty(user.URLToken, user.URLTokenSnake)
	if token != "" {
		return "https://www.zhihu.com/people/" + url.PathEscape(token)
	}
	if strings.HasPrefix(user.URL, "https://www.zhihu.com/people/") {
		return user.URL
	}
	return ""
}

func normalize_asset_url(raw_url string, base string) string {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" || strings.HasPrefix(raw_url, "data:") {
		return raw_url
	}
	if strings.HasPrefix(raw_url, "//") {
		return "https:" + raw_url
	}
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		if parsed.Scheme == "http" || parsed.Scheme == "https" {
			return parsed.String()
		}
		return ""
	}
	if base == "" {
		return ""
	}
	base_url, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return base_url.ResolveReference(parsed).String()
}

func path_ext(raw_url string) string {
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	path := parsed.EscapedPath()
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx:]
	}
	return ""
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

const zhihu_user_agent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

func (c *Client) log_request(method string, raw_url string, cookie_header string) {
	if c == nil || c.logger == nil {
		return
	}
	cookie_count, cookie_names := summarize_cookie_header(cookie_header)
	c.logger.Info().
		Str("component", "zhihu_scraper").
		Str("method", method).
		Str("url", raw_url).
		Bool("cookie_present", cookie_count > 0).
		Int("cookie_count", cookie_count).
		Strs("cookie_names", cookie_names).
		Msg("zhihu outbound request")
}

func (c *Client) log_response(method string, raw_url string, status_code int) {
	if c == nil || c.logger == nil {
		return
	}
	c.logger.Info().
		Str("component", "zhihu_scraper").
		Str("method", method).
		Str("url", raw_url).
		Int("status_code", status_code).
		Msg("zhihu outbound response")
}

func summarize_cookie_header(cookie_header string) (int, []string) {
	cookie_names := make(map[string]struct{})
	cookie_count := 0
	for _, cookie_part := range strings.Split(cookie_header, ";") {
		cookie_part = strings.TrimSpace(cookie_part)
		if cookie_part == "" {
			continue
		}
		cookie_name, _, has_value := strings.Cut(cookie_part, "=")
		cookie_name = strings.TrimSpace(cookie_name)
		if !has_value || cookie_name == "" {
			continue
		}
		cookie_count++
		cookie_names[cookie_name] = struct{}{}
	}
	unique_names := make([]string, 0, len(cookie_names))
	for cookie_name := range cookie_names {
		unique_names = append(unique_names, cookie_name)
	}
	sort.Strings(unique_names)
	return cookie_count, unique_names
}
