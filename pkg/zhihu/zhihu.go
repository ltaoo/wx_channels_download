package zhihu

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/viper"
)

const (
	Protocol  = "zhihu"
	SourceURL = "https://www.zhihu.com/"
)

var answerURLRe = regexp.MustCompile(`^/question/([0-9]+|undefined)/answer/([0-9]+)$`)
var questionURLRe = regexp.MustCompile(`^/question/([0-9]+)$`)
var articleURLRe = regexp.MustCompile(`^/p/([0-9]+)$`)

type Client struct {
	HTTPClient *http.Client
	OnProgress func(downloaded int64)
}

type AnswerURL struct {
	QuestionID string
	AnswerID   string
	Canonical  string
}

type QuestionURL struct {
	QuestionID string
	Canonical  string
}

type ArticleURL struct {
	ArticleID string
	Canonical string
}

type User struct {
	ID                string            `json:"id"`
	URL               string            `json:"url"`
	URLToken          string            `json:"urlToken"`
	URLTokenSnake     string            `json:"url_token"`
	Name              string            `json:"name"`
	Headline          string            `json:"headline"`
	HeadlineRender    string            `json:"headlineRender"`
	AvatarURL         string            `json:"avatarUrl"`
	AvatarURLSnake    string            `json:"avatar_url"`
	AvatarURLTemplate string            `json:"avatarUrlTemplate"`
	UseDefaultAvatar  bool              `json:"useDefaultAvatar"`
	IsOrg             bool              `json:"isOrg"`
	Type              string            `json:"type"`
	UserType          string            `json:"userType"`
	Badge             []json.RawMessage `json:"badge"`
	BadgeV2           BadgeV2           `json:"badgeV2"`
	Gender            int               `json:"gender"`
	IsAdvertiser      bool              `json:"isAdvertiser"`
	IsPrivacy         bool              `json:"isPrivacy"`
	IsFollowed        bool              `json:"isFollowed"`
	IPInfo            string            `json:"ipInfo"`
	VIPInfo           VIPInfo           `json:"vipInfo"`
}

type Question struct {
	ID                     string                     `json:"id"`
	Type                   string                     `json:"type"`
	Title                  string                     `json:"title"`
	QuestionType           string                     `json:"questionType"`
	Created                int64                      `json:"created"`
	UpdatedTime            int64                      `json:"updatedTime"`
	URL                    string                     `json:"url"`
	IsMuted                bool                       `json:"isMuted"`
	IsVisible              bool                       `json:"isVisible"`
	IsNormal               bool                       `json:"isNormal"`
	IsEditable             bool                       `json:"isEditable"`
	AdminClosedComment     bool                       `json:"adminClosedComment"`
	HasPublishingDraft     bool                       `json:"hasPublishingDraft"`
	AnswerCount            int                        `json:"answerCount"`
	VisitCount             int                        `json:"visitCount"`
	CommentCount           int                        `json:"commentCount"`
	FollowerCount          int                        `json:"followerCount"`
	CollapsedAnswerCount   int                        `json:"collapsedAnswerCount"`
	Excerpt                string                     `json:"excerpt"`
	CommentPermission      string                     `json:"commentPermission"`
	Detail                 string                     `json:"detail"`
	EditableDetail         string                     `json:"editableDetail"`
	Status                 QuestionStatus             `json:"status"`
	Topics                 []Topic                    `json:"topics"`
	Author                 User                       `json:"author"`
	CanComment             CanComment                 `json:"canComment"`
	ThumbnailInfo          ThumbnailInfo              `json:"thumbnailInfo"`
	ReviewInfo             ReviewInfo                 `json:"reviewInfo"`
	RelatedCards           []json.RawMessage          `json:"relatedCards"`
	MuteInfo               MuteInfo                   `json:"muteInfo"`
	ShowAuthor             bool                       `json:"showAuthor"`
	IsLabeled              bool                       `json:"isLabeled"`
	IsBannered             bool                       `json:"isBannered"`
	ShowEncourageAuthor    bool                       `json:"showEncourageAuthor"`
	VoteupCount            int                        `json:"voteupCount"`
	CanVote                bool                       `json:"canVote"`
	ReactionInstruction    map[string]json.RawMessage `json:"reactionInstruction"`
	InvisibleAuthor        bool                       `json:"invisibleAuthor"`
	AnswerCountDescription string                     `json:"answerCountDescription"`
	Relationship           QuestionRelationship       `json:"relationship"`
}

type Answer struct {
	ID                          string                     `json:"id"`
	Type                        string                     `json:"type"`
	AdminClosedComment          bool                       `json:"adminClosedComment"`
	AllowSegmentInteraction     int                        `json:"allowSegmentInteraction"`
	AnnotationAction            json.RawMessage            `json:"annotationAction"`
	AnswerType                  string                     `json:"answerType"`
	Author                      User                       `json:"author"`
	BizExt                      AnswerBizExt               `json:"bizExt"`
	CanComment                  CanComment                 `json:"canComment"`
	CollapseReason              string                     `json:"collapseReason"`
	CollapsedBy                 string                     `json:"collapsedBy"`
	CommentCount                int                        `json:"commentCount"`
	CommentPermission           string                     `json:"commentPermission"`
	Content                     string                     `json:"content"`
	ContentNeedTruncated        bool                       `json:"contentNeedTruncated"`
	CreatedTime                 int64                      `json:"createdTime"`
	EditableContent             string                     `json:"editableContent"`
	Excerpt                     string                     `json:"excerpt"`
	Extras                      string                     `json:"extras"`
	FavlistsCount               int                        `json:"favlistsCount"`
	ForceLoginWhenClickReadMore bool                       `json:"forceLoginWhenClickReadMore"`
	HasColumn                   bool                       `json:"hasColumn"`
	IPInfo                      string                     `json:"ipInfo"`
	IsCollapsed                 bool                       `json:"isCollapsed"`
	IsCopyable                  bool                       `json:"isCopyable"`
	IsJumpNative                bool                       `json:"isJumpNative"`
	IsLabeled                   bool                       `json:"isLabeled"`
	IsNavigator                 bool                       `json:"isNavigator"`
	IsNormal                    bool                       `json:"isNormal"`
	IsSticky                    bool                       `json:"isSticky"`
	IsVisible                   bool                       `json:"isVisible"`
	NavigatorVote               bool                       `json:"navigatorVote"`
	PodcastAudioEnter           PodcastAudioEnter          `json:"podcastAudioEnter"`
	Question                    QuestionRef                `json:"question"`
	Reaction                    AnswerReaction             `json:"reaction"`
	ReactionInstruction         map[string]json.RawMessage `json:"reactionInstruction"`
	Relationship                AnswerRelationship         `json:"relationship"`
	RelevantInfo                RelevantInfo               `json:"relevantInfo"`
	ReshipmentSettings          string                     `json:"reshipmentSettings"`
	RewardInfo                  RewardInfo                 `json:"rewardInfo"`
	SuggestEdit                 SuggestEdit                `json:"suggestEdit"`
	ThanksCount                 int                        `json:"thanksCount"`
	UpdatedTime                 int64                      `json:"updatedTime"`
	URL                         string                     `json:"url"`
	VoteNextStep                string                     `json:"voteNextStep"`
	VoteupCount                 int                        `json:"voteupCount"`
}

type Article struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Excerpt     string `json:"excerpt"`
	ImageURL    string `json:"imageUrl"`
	ImageURLAlt string `json:"image_url"`
	Author      User   `json:"author"`
	CreatedTime int64  `json:"created"`
	UpdatedTime int64  `json:"updated"`
}

type Comment struct {
	ID          string
	ContentHTML string
	ContentText string
	CreatedTime int64
	Author      User
	ReplyTo     *User
	Replies     []Comment
}

type AnswerPage struct {
	URL             AnswerURL
	Source          string
	PageHTML        string
	Question        Question
	Answer          Answer
	Comments        []Comment
	InitialData     *InitialData
	InitialDataJSON json.RawMessage
}

type QuestionPage struct {
	URL             QuestionURL
	Source          string
	PageHTML        string
	Question        Question
	InitialData     *InitialData
	InitialDataJSON json.RawMessage
}

type ArticlePage struct {
	URL             ArticleURL
	Source          string
	PageHTML        string
	Article         Article
	InitialData     *InitialData
	InitialDataJSON json.RawMessage
}

func ParseAnswerURL(rawURL string) (AnswerURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return AnswerURL{}, false
	}
	if !strings.EqualFold(parsed.Hostname(), "www.zhihu.com") {
		return AnswerURL{}, false
	}
	matches := answerURLRe.FindStringSubmatch(parsed.EscapedPath())
	if len(matches) != 3 {
		return AnswerURL{}, false
	}
	questionID := matches[1]
	answerID := matches[2]
	canonical := canonicalAnswerURL(questionID, answerID)
	if questionID == "undefined" {
		questionID = ""
	}
	return AnswerURL{QuestionID: questionID, AnswerID: answerID, Canonical: canonical}, true
}

func ParseQuestionURL(rawURL string) (QuestionURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return QuestionURL{}, false
	}
	if !strings.EqualFold(parsed.Hostname(), "www.zhihu.com") {
		return QuestionURL{}, false
	}
	matches := questionURLRe.FindStringSubmatch(parsed.EscapedPath())
	if len(matches) != 2 {
		return QuestionURL{}, false
	}
	canonical := "https://www.zhihu.com/question/" + matches[1]
	return QuestionURL{QuestionID: matches[1], Canonical: canonical}, true
}

func ParseArticleURL(rawURL string) (ArticleURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ArticleURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "zhuanlan.zhihu.com" && host != "www.zhihu.com" {
		return ArticleURL{}, false
	}
	matches := articleURLRe.FindStringSubmatch(parsed.EscapedPath())
	if len(matches) != 2 {
		return ArticleURL{}, false
	}
	canonical := "https://zhuanlan.zhihu.com/p/" + matches[1]
	return ArticleURL{ArticleID: matches[1], Canonical: canonical}, true
}

func ResolveRealURL(rawURL string) string {
	if strings.HasPrefix(strings.ToLower(rawURL), Protocol+"://") {
		rawURL = rawURL[len(Protocol+"://"):]
		if !strings.HasPrefix(strings.ToLower(rawURL), "http") {
			rawURL = "https://" + rawURL
		}
	}
	return rawURL
}

func canonicalAnswerURL(questionID, answerID string) string {
	questionID = strings.TrimSpace(questionID)
	if questionID == "" {
		questionID = "undefined"
	}
	return "https://www.zhihu.com/question/" + questionID + "/answer/" + strings.TrimSpace(answerID)
}

func (c *Client) FetchAnswerPage(rawURL string) (*AnswerPage, error) {
	answerURL, ok := ParseAnswerURL(ResolveRealURL(rawURL))
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu answer url")
	}
	body, err := c.doBytes(http.MethodGet, answerURL.Canonical, answerURL.Canonical)
	if err != nil {
		return nil, err
	}
	page, err := parseAnswerPage(body, answerURL)
	if err != nil {
		return nil, err
	}
	page.Source = answerURL.Canonical
	if page.Answer.CommentCount > 0 {
		if comments, err := c.fetchAnswerComments(answerURL); err == nil {
			page.Comments = comments
		}
	}
	return page, nil
}

func (c *Client) FetchQuestionPage(rawURL string) (*QuestionPage, error) {
	questionURL, ok := ParseQuestionURL(ResolveRealURL(rawURL))
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu question url")
	}
	body, err := c.doBytes(http.MethodGet, questionURL.Canonical, questionURL.Canonical)
	if err != nil {
		return nil, err
	}
	page, err := parseQuestionPage(body, questionURL)
	if err != nil {
		return nil, err
	}
	page.Source = questionURL.Canonical
	return page, nil
}

func (c *Client) FetchArticlePage(rawURL string) (*ArticlePage, error) {
	articleURL, ok := ParseArticleURL(ResolveRealURL(rawURL))
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu article url")
	}
	body, err := c.doBytes(http.MethodGet, articleURL.Canonical, articleURL.Canonical)
	if err != nil {
		return nil, err
	}
	page, err := parseArticlePage(body, articleURL)
	if err != nil {
		return nil, err
	}
	page.Source = articleURL.Canonical
	return page, nil
}

func (c *Client) BuildHTMLFromURL(rawURL string) (string, error) {
	if articleURL, ok := ParseArticleURL(ResolveRealURL(rawURL)); ok {
		page, err := c.FetchArticlePage(articleURL.Canonical)
		if err != nil {
			return "", err
		}
		content := BuildArticleHTML(page)
		content, err = c.inlineRemoteImages(content, page.Source)
		if err != nil {
			return "", err
		}
		return content, nil
	}
	if questionURL, ok := ParseQuestionURL(ResolveRealURL(rawURL)); ok {
		page, err := c.FetchQuestionPage(questionURL.Canonical)
		if err != nil {
			return "", err
		}
		content := BuildQuestionHTML(page)
		content, err = c.inlineRemoteImages(content, page.Source)
		if err != nil {
			return "", err
		}
		return content, nil
	}
	page, err := c.FetchAnswerPage(rawURL)
	if err != nil {
		return "", err
	}
	content := BuildHTML(page)
	content, err = c.inlineRemoteImages(content, page.Source)
	if err != nil {
		return "", err
	}
	return content, nil
}

func (c *Client) doBytes(method, rawURL, referer string) ([]byte, error) {
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	setHeaders(req, referer)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("zhihu status %d debug=%s", resp.StatusCode, zhihuRequestDebug(req, resp, body))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err == nil && c.OnProgress != nil {
		c.OnProgress(int64(len(data)))
	}
	return data, err
}

func (c *Client) inlineRemoteImages(content string, referer string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", err
	}
	var firstErr error
	doc.Find("img[src]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src, _ := s.Attr("src")
		src = normalizeAssetURL(src, referer)
		if src == "" || strings.HasPrefix(src, "data:") {
			return true
		}
		dataURI, err := c.fetchImageDataURI(src, referer)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return true
		}
		s.SetAttr("src", dataURI)
		return true
	})
	if firstErr != nil {
		return "", firstErr
	}
	out, err := doc.Html()
	if err != nil {
		return "", err
	}
	return "<!doctype html>" + out, nil
}

func (c *Client) InlineRemoteImages(content string, referer string) (string, error) {
	return c.inlineRemoteImages(content, referer)
}

func (c *Client) LocalizeRemoteVideos(ctx context.Context, content string, referer string, htmlPath string) (string, error) {
	if strings.TrimSpace(content) == "" || strings.TrimSpace(htmlPath) == "" {
		return content, nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", err
	}
	assetsDirName := htmlAssetsDirName(htmlPath)
	assetsDirPath := filepath.Join(filepath.Dir(htmlPath), assetsDirName)
	downloaded := make(map[string]string)
	var firstErr error
	videoIndex := 0
	doc.Find("video[src], video source[src]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src, _ := s.Attr("src")
		src = normalizeAssetURL(src, referer)
		if src == "" || strings.HasPrefix(src, "data:") || !strings.HasPrefix(src, "http") {
			return true
		}
		localPath, ok := downloaded[src]
		if !ok {
			videoIndex++
			filename, err := c.downloadVideo(ctx, src, referer, assetsDirPath, videoIndex)
			if err != nil {
				firstErr = err
				return false
			}
			localPath = filepath.ToSlash(filepath.Join(assetsDirName, filename))
			downloaded[src] = localPath
		}
		s.SetAttr("src", localPath)
		if s.Is("video") {
			ensurePlayableVideo(s)
		} else {
			s.ParentFiltered("video").Each(func(_ int, video *goquery.Selection) {
				ensurePlayableVideo(video)
			})
		}
		return true
	})
	if firstErr != nil {
		return "", firstErr
	}
	out, err := doc.Html()
	if err != nil {
		return "", err
	}
	return "<!doctype html>" + out, nil
}

func ensurePlayableVideo(s *goquery.Selection) {
	s.SetAttr("controls", "controls")
	if _, ok := s.Attr("preload"); !ok {
		s.SetAttr("preload", "metadata")
	}
	if _, ok := s.Attr("style"); !ok {
		s.SetAttr("style", "max-width:100%;height:auto")
	}
}

func htmlAssetsDirName(htmlPath string) string {
	base := strings.TrimSuffix(filepath.Base(htmlPath), filepath.Ext(htmlPath))
	base = strings.TrimSpace(base)
	if base == "" || base == "." {
		base = "zhihu"
	}
	return base + "_files"
}

func (c *Client) downloadVideo(ctx context.Context, rawURL string, referer string, dir string, index int) (string, error) {
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	setHeaders(req, referer)
	req.Header.Set("accept", "video/mp4,video/webm,video/*,*/*;q=0.8")
	req.Header.Set("sec-fetch-dest", "video")
	req.Header.Set("sec-fetch-mode", "no-cors")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("zhihu video status %d debug=%s", resp.StatusCode, zhihuRequestDebug(req, resp, body))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("video_%02d%s", index, videoExt(rawURL, resp.Header.Get("content-type")))
	destPath := filepath.Join(dir, filename)
	file, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := copyWithClientProgress(file, resp.Body, c.OnProgress); err != nil {
		return "", err
	}
	return filename, nil
}

func copyWithClientProgress(dst io.Writer, src io.Reader, onProgress func(int64)) (int64, error) {
	buf := make([]byte, 64*1024)
	var written int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			m, writeErr := dst.Write(buf[:n])
			written += int64(m)
			if onProgress != nil && m > 0 {
				onProgress(int64(m))
			}
			if writeErr != nil {
				return written, writeErr
			}
			if m != n {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}

func videoExt(rawURL string, contentType string) string {
	if ext := strings.ToLower(pathExt(rawURL)); validMediaExt(ext) {
		return ext
	}
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	if exts, err := mime.ExtensionsByType(strings.TrimSpace(contentType)); err == nil {
		for _, ext := range exts {
			if validMediaExt(ext) {
				return ext
			}
		}
	}
	return ".mp4"
}

func validMediaExt(ext string) bool {
	switch ext {
	case ".mp4", ".m4v", ".mov", ".webm", ".mkv":
		return true
	default:
		return false
	}
}

func (c *Client) fetchImageDataURI(rawURL string, referer string) (string, error) {
	body, contentType, err := c.doImageBytes(rawURL, referer)
	if err != nil {
		return "", err
	}
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	if contentType == "" || contentType == "application/octet-stream" {
		if ext := strings.ToLower(pathExt(rawURL)); ext != "" {
			contentType = mime.TypeByExtension(ext)
		}
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

func (c *Client) doImageBytes(rawURL string, referer string) ([]byte, string, error) {
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	setHeaders(req, referer)
	req.Header.Set("accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, "", fmt.Errorf("zhihu image status %d debug=%s", resp.StatusCode, zhihuRequestDebug(req, resp, body))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err == nil && c.OnProgress != nil {
		c.OnProgress(int64(len(body)))
	}
	return body, resp.Header.Get("content-type"), err
}

func setHeaders(req *http.Request, referer string) {
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("priority", "u=0, i")
	req.Header.Set("sec-ch-ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-fetch-dest", "document")
	req.Header.Set("sec-fetch-mode", "navigate")
	req.Header.Set("sec-fetch-site", "none")
	req.Header.Set("sec-fetch-user", "?1")
	req.Header.Set("upgrade-insecure-requests", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	if referer != "" {
		req.Header.Set("referer", referer)
	}
	if cookie := strings.TrimSpace(viper.GetString("zhihu.cookie")); cookie != "" {
		fmt.Println("before set cookie", cookie)
		req.Header.Set("cookie", cookie)
	}
}

func zhihuRequestDebug(req *http.Request, resp *http.Response, body []byte) string {
	cookie := ""
	if req != nil {
		cookie = req.Header.Get("cookie")
	}
	keys := cookieKeys(cookie)
	bodyText := string(body)
	zseChallenge := strings.Contains(bodyText, `id="zh-zse-ck"`) || strings.Contains(bodyText, "zse-ck")

	debug := map[string]any{
		"cookie": map[string]any{
			"present": strings.TrimSpace(cookie) != "",
			"keys":    keys,
			"len":     len(cookie),
			"sha256":  sha256Hex(cookie),
			"has": map[string]bool{
				"d_c0":     getCookieValue(cookie, "d_c0") != "",
				"z_c0":     getCookieValue(cookie, "z_c0") != "",
				"__zse_ck": getCookieValue(cookie, "__zse_ck") != "",
				"_xsrf":    getCookieValue(cookie, "_xsrf") != "",
			},
			"source":      "viper:zhihu.cookie",
			"config_file": viper.ConfigFileUsed(),
		},
		"body": map[string]any{
			"len":       len(body),
			"snippet":   strings.TrimSpace(bodyText),
			"zse_ck":    zseChallenge,
			"truncated": len(body) >= 2048,
		},
	}
	if zseChallenge {
		debug["diagnosis"] = "zhihu_zse_ck_challenge"
		debug["hint"] = "Zhihu returned the browser JS challenge page. Refresh zhihu.cookie by opening Zhihu through the app proxy in a real browser, then retry; a plain Go HTTP request cannot execute the zse-ck challenge."
	}
	if req != nil {
		debug["request"] = map[string]any{
			"method":  req.Method,
			"url":     req.URL.String(),
			"headers": redactHeaders(req.Header),
			"curl":    buildReproCurl(req),
		}
	}
	if resp != nil {
		finalURL := ""
		if resp.Request != nil && resp.Request.URL != nil {
			finalURL = resp.Request.URL.String()
		}
		debug["response"] = map[string]any{
			"status":      resp.Status,
			"status_code": resp.StatusCode,
			"headers":     redactHeaders(resp.Header),
			"final_url":   finalURL,
		}
	}
	data, err := json.Marshal(debug)
	if err != nil {
		return fmt.Sprintf(`{"error":"marshal debug failed: %s"}`, err)
	}
	return string(data)
}

func redactHeaders(headers http.Header) map[string][]string {
	out := make(map[string][]string, len(headers))
	for key, values := range headers {
		lower := strings.ToLower(key)
		if lower == "cookie" || lower == "set-cookie" || lower == "setcookie" {
			joined := strings.Join(values, "; ")
			out[key] = []string{redactedCookieSummary(joined)}
			continue
		}
		out[key] = append([]string(nil), values...)
	}
	return out
}

func redactedCookieSummary(cookie string) string {
	keys := cookieKeys(cookie)
	return fmt.Sprintf("<redacted len=%d sha256=%s keys=%s>", len(cookie), sha256Hex(cookie), strings.Join(keys, ","))
}

func sha256Hex(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}

func buildReproCurl(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}
	parts := []string{"curl", "-i", "--max-time", "20", shellQuote(req.URL.String())}
	keys := make([]string, 0, len(req.Header))
	for key := range req.Header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.Join(req.Header.Values(key), ", ")
		if strings.EqualFold(key, "cookie") {
			value = "<paste zhihu.cookie from config>"
		}
		parts = append(parts, "-H", shellQuote(key+": "+value))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func cookieKeys(cookie string) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, _, ok := strings.Cut(part, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func parseAnswerPage(body []byte, answerURL AnswerURL) (*AnswerPage, error) {
	initialData, err := ParseInitialData(body)
	if err != nil {
		return nil, err
	}
	answer := initialData.InitialState.Entities.Answers[answerURL.AnswerID]
	if answer.ID == "" {
		return nil, fmt.Errorf("missing zhihu answer entity")
	}
	question := initialData.InitialState.Entities.Questions[answerURL.QuestionID]
	if question.ID == "" && answer.Question.ID != "" {
		question = initialData.InitialState.Entities.Questions[answer.Question.ID]
	}
	if question.ID == "" && len(initialData.InitialState.Entities.Questions) == 1 {
		for _, candidate := range initialData.InitialState.Entities.Questions {
			question = candidate
		}
	}
	if question.ID == "" {
		return nil, fmt.Errorf("missing zhihu question entity")
	}
	pageURL := answerURL
	if pageURL.QuestionID != question.ID {
		pageURL.QuestionID = question.ID
		pageURL.Canonical = canonicalAnswerURL(question.ID, answer.ID)
	}
	if pageURL.Canonical == "" {
		pageURL.Canonical = canonicalAnswerURL(pageURL.QuestionID, pageURL.AnswerID)
	}
	return &AnswerPage{
		URL:             pageURL,
		Source:          pageURL.Canonical,
		PageHTML:        string(body),
		Question:        question,
		Answer:          answer,
		InitialData:     initialData,
		InitialDataJSON: initialData.Raw,
	}, nil
}

func parseQuestionPage(body []byte, questionURL QuestionURL) (*QuestionPage, error) {
	initialData, err := ParseInitialData(body)
	if err != nil {
		return nil, err
	}
	question := initialData.InitialState.Entities.Questions[questionURL.QuestionID]
	if question.ID == "" {
		return nil, fmt.Errorf("missing zhihu question entity")
	}
	return &QuestionPage{
		URL:             questionURL,
		Source:          questionURL.Canonical,
		PageHTML:        string(body),
		Question:        question,
		InitialData:     initialData,
		InitialDataJSON: initialData.Raw,
	}, nil
}

func parseArticlePage(body []byte, articleURL ArticleURL) (*ArticlePage, error) {
	initialData, err := ParseInitialData(body)
	if err != nil {
		return nil, err
	}
	article := initialData.InitialState.Entities.Articles[articleURL.ArticleID]
	if article.ID == "" {
		article = initialData.InitialState.Entities.Posts[articleURL.ArticleID]
	}
	if article.ID == "" {
		return nil, fmt.Errorf("missing zhihu article entity")
	}
	return &ArticlePage{
		URL:             articleURL,
		Source:          articleURL.Canonical,
		PageHTML:        string(body),
		Article:         article,
		InitialData:     initialData,
		InitialDataJSON: initialData.Raw,
	}, nil
}

func (c *Client) fetchAnswerComments(answerURL AnswerURL) ([]Comment, error) {
	comments, err := c.fetchAnswerRootComments(answerURL)
	if err == nil {
		return comments, nil
	}
	return c.fetchAnswerCommentsV5(answerURL)
}

func (c *Client) fetchAnswerRootComments(answerURL AnswerURL) ([]Comment, error) {
	endpoint := fmt.Sprintf("/api/v4/answers/%s/root_comments?limit=20&offset=0&order=normal&status=open", url.PathEscape(answerURL.AnswerID))
	var comments []Comment
	for endpoint != "" {
		body, err := c.doAPIBytes(endpoint, answerURL.Canonical)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Data   []commentPayload `json:"data"`
			Paging struct {
				IsEnd bool   `json:"is_end"`
				Next  string `json:"next"`
			} `json:"paging"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		for _, item := range resp.Data {
			comments = append(comments, item.toComment())
		}
		if resp.Paging.IsEnd || resp.Paging.Next == "" || len(comments) >= 200 {
			break
		}
		endpoint = endpointFromURL(resp.Paging.Next)
	}
	return comments, nil
}

func (c *Client) fetchAnswerCommentsV5(answerURL AnswerURL) ([]Comment, error) {
	endpoint := fmt.Sprintf("/api/v4/comment_v5/answers/%s/root_comment?order_by=score&limit=20&offset=0&status=open", url.PathEscape(answerURL.AnswerID))
	body, err := c.doAPIBytes(endpoint, answerURL.Canonical)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []commentPayload `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	comments := make([]Comment, 0, len(resp.Data))
	for _, item := range resp.Data {
		comments = append(comments, item.toComment())
	}
	return comments, nil
}

func (c *Client) doAPIBytes(endpoint, referer string) ([]byte, error) {
	if !strings.HasPrefix(endpoint, "/") {
		return nil, fmt.Errorf("invalid zhihu api endpoint")
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequest(http.MethodGet, SourceURL+strings.TrimPrefix(endpoint, "/"), nil)
	if err != nil {
		return nil, err
	}
	setAPIHeaders(req, endpoint, referer)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("zhihu api status %d debug=%s", resp.StatusCode, zhihuRequestDebug(req, resp, body))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err == nil && c.OnProgress != nil {
		c.OnProgress(int64(len(data)))
	}
	return data, err
}

func setAPIHeaders(req *http.Request, endpoint, referer string) {
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("priority", "u=1, i")
	req.Header.Set("sec-ch-ua", `"Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	req.Header.Set("x-requested-with", "fetch")
	if referer != "" {
		req.Header.Set("referer", referer)
	}
	cookie := strings.TrimSpace(viper.GetString("zhihu.cookie"))
	if cookie != "" {
		req.Header.Set("cookie", cookie)
		if dc0 := strings.Trim(getCookieValue(cookie, "d_c0"), `"`); dc0 != "" {
			for k, v := range buildSignedHeader(endpoint, dc0) {
				req.Header.Set(k, v)
			}
		}
	}
}

func endpointFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
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

type commentPayload struct {
	ID         json.RawMessage  `json:"id"`
	Content    string           `json:"content"`
	ContentTag string           `json:"content_tag"`
	Created    int64            `json:"created_time"`
	CreatedAt  int64            `json:"createdAt"`
	Author     User             `json:"author"`
	ReplyTo    *User            `json:"reply_to_author"`
	Child      []commentPayload `json:"child_comments"`
}

func (p commentPayload) toComment() Comment {
	created := p.Created
	if created == 0 {
		created = p.CreatedAt
	}
	content := firstNonEmpty(p.Content, p.ContentTag)
	comment := Comment{
		ID:          rawIDString(p.ID),
		ContentHTML: content,
		ContentText: htmlToText(content),
		CreatedTime: created,
		Author:      p.Author,
		ReplyTo:     p.ReplyTo,
	}
	for _, child := range p.Child {
		comment.Replies = append(comment.Replies, child.toComment())
	}
	return comment
}

func rawIDString(raw json.RawMessage) string {
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

func BuildHTML(page *AnswerPage) string {
	if page == nil {
		return ""
	}
	var b strings.Builder
	title := strings.TrimSpace(page.Question.Title)
	if title == "" {
		title = "知乎回答"
	}
	b.WriteString("<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title><style>")
	b.WriteString(`body{margin:0;background:#f6f6f6;color:#1f2329;font:16px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{max-width:760px;margin:0 auto;padding:32px 18px 56px;background:#fff;min-height:100vh}h1{font-size:28px;line-height:1.35;margin:0 0 12px}h2{font-size:20px;margin:34px 0 12px;border-top:1px solid #e7e9ee;padding-top:24px}.meta{color:#69707a;font-size:14px;margin:0 0 18px}.author{display:flex;gap:12px;align-items:center;margin:0 0 18px;color:#69707a;font-size:14px}.avatar{width:42px;height:42px;border-radius:50%;object-fit:cover;background:#edf0f3;flex:0 0 auto}.author-name{font-weight:600;color:#1f2329}.content p{margin:0 0 14px}.content img{max-width:100%;height:auto}.comment{border-top:1px solid #edf0f3;padding:14px 0}.reply{margin-left:18px;border-left:3px solid #edf0f3;padding-left:12px}.source{word-break:break-all}a{color:#175199;text-decoration:none}a:hover{text-decoration:underline}`)
	b.WriteString("</style></head><body><main>")
	b.WriteString("<h1>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</h1><p class=\"meta\">问题作者：")
	b.WriteString(html.EscapeString(displayName(page.Question.Author)))
	b.WriteString(" · 问题原始链接：<a href=\"")
	b.WriteString(html.EscapeString(questionURL(page)))
	b.WriteString("\">")
	b.WriteString(html.EscapeString(questionURL(page)))
	b.WriteString("</a>")
	b.WriteString("</p>")
	if strings.TrimSpace(page.Question.Detail) != "" {
		b.WriteString("<section class=\"content\">")
		b.WriteString(sanitizeFragment(page.Question.Detail))
		b.WriteString("</section>")
	} else if strings.TrimSpace(page.Question.Excerpt) != "" {
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(page.Question.Excerpt))
		b.WriteString("</p>")
	}
	b.WriteString("<h2>回答</h2>")
	writeAuthorBlock(&b, "回答作者", page.Answer.Author, page.Source)
	if page.Answer.CreatedTime > 0 {
		b.WriteString("<p class=\"meta\">发布于 ")
		b.WriteString(html.EscapeString(formatTime(page.Answer.CreatedTime)))
		b.WriteString("</p>")
	}
	b.WriteString("<section class=\"content\">")
	b.WriteString(sanitizeFragment(page.Answer.Content))
	b.WriteString("</section>")
	if len(page.Comments) > 0 {
		b.WriteString("<h2>回答评论</h2>")
		for _, comment := range page.Comments {
			writeComment(&b, comment, false)
		}
	}
	b.WriteString("<h2>来源</h2><p class=\"source\"><a href=\"")
	b.WriteString(html.EscapeString(page.Source))
	b.WriteString("\">")
	b.WriteString(html.EscapeString(page.Source))
	b.WriteString("</a></p></main></body></html>")
	return b.String()
}

func BuildQuestionHTML(page *QuestionPage) string {
	if page == nil {
		return ""
	}
	var b strings.Builder
	title := strings.TrimSpace(page.Question.Title)
	if title == "" {
		title = "知乎问题"
	}
	b.WriteString("<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title><style>")
	b.WriteString(`body{margin:0;background:#f6f6f6;color:#1f2329;font:16px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{max-width:760px;margin:0 auto;padding:32px 18px 56px;background:#fff;min-height:100vh}h1{font-size:28px;line-height:1.35;margin:0 0 12px}.meta{color:#69707a;font-size:14px;margin:0 0 18px}.author{display:flex;gap:12px;align-items:center;margin:0 0 18px;color:#69707a;font-size:14px}.avatar{width:42px;height:42px;border-radius:50%;object-fit:cover;background:#edf0f3;flex:0 0 auto}.author-name{font-weight:600;color:#1f2329}.content p{margin:0 0 14px}.content img{max-width:100%;height:auto}.source{word-break:break-all}a{color:#175199;text-decoration:none}a:hover{text-decoration:underline}`)
	b.WriteString("</style></head><body><main><h1>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</h1>")
	writeAuthorBlock(&b, "问题作者", page.Question.Author, page.Source)
	if strings.TrimSpace(page.Question.Detail) != "" {
		b.WriteString("<section class=\"content\">")
		b.WriteString(sanitizeFragment(page.Question.Detail))
		b.WriteString("</section>")
	} else if strings.TrimSpace(page.Question.Excerpt) != "" {
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(page.Question.Excerpt))
		b.WriteString("</p>")
	}
	b.WriteString("<h2>来源</h2><p class=\"source\"><a href=\"")
	b.WriteString(html.EscapeString(page.Source))
	b.WriteString("\">")
	b.WriteString(html.EscapeString(page.Source))
	b.WriteString("</a></p></main></body></html>")
	return b.String()
}

func BuildArticleHTML(page *ArticlePage) string {
	if page == nil {
		return ""
	}
	var b strings.Builder
	title := strings.TrimSpace(page.Article.Title)
	if title == "" {
		title = "知乎文章"
	}
	b.WriteString("<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title><style>")
	b.WriteString(`body{margin:0;background:#f6f6f6;color:#1f2329;font:16px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{max-width:760px;margin:0 auto;padding:32px 18px 56px;background:#fff;min-height:100vh}h1{font-size:28px;line-height:1.35;margin:0 0 12px}.meta{color:#69707a;font-size:14px;margin:0 0 18px}.author{display:flex;gap:12px;align-items:center;margin:0 0 18px;color:#69707a;font-size:14px}.avatar{width:42px;height:42px;border-radius:50%;object-fit:cover;background:#edf0f3;flex:0 0 auto}.author-name{font-weight:600;color:#1f2329}.content p{margin:0 0 14px}.content img{max-width:100%;height:auto}.source{word-break:break-all}a{color:#175199;text-decoration:none}a:hover{text-decoration:underline}`)
	b.WriteString("</style></head><body><main><h1>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</h1>")
	writeAuthorBlock(&b, "文章作者", page.Article.Author, page.Source)
	if page.Article.CreatedTime > 0 {
		b.WriteString("<p class=\"meta\">发布于 ")
		b.WriteString(html.EscapeString(formatTime(page.Article.CreatedTime)))
		b.WriteString("</p>")
	}
	if strings.TrimSpace(page.Article.Content) != "" {
		b.WriteString("<section class=\"content\">")
		b.WriteString(sanitizeFragment(page.Article.Content))
		b.WriteString("</section>")
	} else if strings.TrimSpace(page.Article.Excerpt) != "" {
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(page.Article.Excerpt))
		b.WriteString("</p>")
	}
	b.WriteString("<h2>来源</h2><p class=\"source\"><a href=\"")
	b.WriteString(html.EscapeString(page.Source))
	b.WriteString("\">")
	b.WriteString(html.EscapeString(page.Source))
	b.WriteString("</a></p></main></body></html>")
	return b.String()
}

func writeComment(b *strings.Builder, comment Comment, reply bool) {
	className := "comment"
	if reply {
		className += " reply"
	}
	b.WriteString("<div class=\"")
	b.WriteString(className)
	b.WriteString("\"><p class=\"meta\">评论作者：")
	b.WriteString(html.EscapeString(displayName(comment.Author)))
	if comment.ReplyTo != nil && displayName(*comment.ReplyTo) != "" {
		b.WriteString(" 回复 ")
		b.WriteString(html.EscapeString(displayName(*comment.ReplyTo)))
	}
	b.WriteString("</p><div class=\"content\">")
	if comment.ContentHTML != "" {
		b.WriteString(sanitizeFragment(comment.ContentHTML))
	} else {
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(comment.ContentText))
		b.WriteString("</p>")
	}
	b.WriteString("</div>")
	for _, child := range comment.Replies {
		writeComment(b, child, true)
	}
	b.WriteString("</div>")
}

func writeAuthorBlock(b *strings.Builder, label string, user User, fallbackURL string) {
	profileURL := authorURL(user)
	if profileURL == "" {
		profileURL = fallbackURL
	}
	b.WriteString("<div class=\"author\">")
	if avatar := avatarURL(user); avatar != "" {
		b.WriteString("<img class=\"avatar\" src=\"")
		b.WriteString(html.EscapeString(avatar))
		b.WriteString("\" alt=\"")
		b.WriteString(html.EscapeString(displayName(user)))
		b.WriteString("\">")
	}
	b.WriteString("<div>")
	b.WriteString(html.EscapeString(label))
	b.WriteString("：")
	if profileURL != "" {
		b.WriteString("<a class=\"author-name\" href=\"")
		b.WriteString(html.EscapeString(profileURL))
		b.WriteString("\">")
		b.WriteString(html.EscapeString(displayName(user)))
		b.WriteString("</a>")
	} else {
		b.WriteString("<span class=\"author-name\">")
		b.WriteString(html.EscapeString(displayName(user)))
		b.WriteString("</span>")
	}
	if strings.TrimSpace(user.Headline) != "" {
		b.WriteString("<br>")
		b.WriteString(html.EscapeString(user.Headline))
	}
	b.WriteString("</div></div>")
}

func sanitizeFragment(fragment string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"wx-zhihu-root\">" + fragment + "</div>"))
	if err != nil {
		return html.EscapeString(htmlToText(fragment))
	}
	root := doc.Find("#wx-zhihu-root")
	root.Find("script,style,iframe,button,svg").Remove()
	root.Find("img").Each(func(_ int, s *goquery.Selection) {
		if src := bestZhihuImageSrc(s); src != "" {
			s.SetAttr("src", src)
		}
	})
	root.Find("*").Each(func(_ int, s *goquery.Selection) {
		for _, node := range s.Nodes {
			sort.Slice(node.Attr, func(i, j int) bool {
				return node.Attr[i].Key < node.Attr[j].Key
			})
			attrs := node.Attr[:0]
			for _, attr := range node.Attr {
				key := strings.ToLower(attr.Key)
				if key == "href" || key == "src" || key == "alt" || key == "title" || key == "width" || key == "height" {
					attrs = append(attrs, attr)
				}
			}
			node.Attr = attrs
		}
	})
	out, err := root.Html()
	if err != nil {
		return html.EscapeString(htmlToText(fragment))
	}
	return out
}

func bestZhihuImageSrc(s *goquery.Selection) string {
	for _, attr := range []string{"data-original", "data-actualsrc", "data-default-watermark-src", "src"} {
		value, ok := s.Attr(attr)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" || isPlaceholderImage(value) {
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
	var imageURL string
	doc.Find("#wx-zhihu-root img").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src := normalizeAssetURL(bestZhihuImageSrc(s), base)
		if src == "" || strings.HasPrefix(src, "data:") {
			return true
		}
		imageURL = src
		return false
	})
	return imageURL
}

func isPlaceholderImage(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "data:image/svg") ||
		strings.Contains(lower, "placeholder") ||
		strings.Contains(lower, "loading") ||
		strings.Contains(lower, "blank")
}

func htmlToText(fragment string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fragment))
	if err != nil {
		return strings.TrimSpace(fragment)
	}
	return strings.TrimSpace(doc.Text())
}

func displayName(user User) string {
	return firstNonEmpty(user.Name, user.URLToken, user.URLTokenSnake, user.ID, "匿名用户")
}

func avatarURL(user User) string {
	return firstNonEmpty(user.AvatarURL, user.AvatarURLSnake, user.AvatarURLTemplate)
}

func UserDisplayName(user User) string {
	return displayName(user)
}

func UserAvatarURL(user User) string {
	return avatarURL(user)
}

func authorURL(user User) string {
	token := firstNonEmpty(user.URLToken, user.URLTokenSnake)
	if token != "" {
		return "https://www.zhihu.com/people/" + url.PathEscape(token)
	}
	if strings.HasPrefix(user.URL, "https://www.zhihu.com/people/") {
		return user.URL
	}
	return ""
}

func normalizeAssetURL(rawURL string, base string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || strings.HasPrefix(rawURL, "data:") {
		return rawURL
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	parsed, err := url.Parse(rawURL)
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
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(parsed).String()
}

func pathExt(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path := parsed.EscapedPath()
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx:]
	}
	return ""
}

func questionURL(page *AnswerPage) string {
	if page == nil || page.URL.QuestionID == "" {
		return ""
	}
	return "https://www.zhihu.com/question/" + url.PathEscape(page.URL.QuestionID)
}

func formatTime(unix int64) string {
	return time.Unix(unix, 0).Format("2006-01-02 15:04")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
