package wxmp

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/clawreq"
)

// Client fetches pages and API responses from WeChat.
// Server-side RSS, API, and WebSocket state belongs to OfficialAccountServer.
type Client struct {
	logger     *zerolog.Logger
	file_cache *cache.CacheProvider
}

func NewClient(_ *OfficialAccountConfig, parent_logger *zerolog.Logger) *Client {
	logger := parent_logger.With().Str("service", "Client").Logger()
	return &Client{logger: &logger}
}

func (c *Client) Fetch(target_url string, referer string) (*http.Response, error) {
	response, err := c.fetch_with_clawreq(target_url, 15*time.Second, windows_wechat_clawreq_headers(referer))
	if err != nil {
		return nil, err
	}
	return http_response_from_clawreq(response), nil
}

func (c *Client) FetchArticle(raw_url string) (*WechatOfficialArticle, error) {
	content, err := c.Scrape(raw_url)
	if err != nil {
		return nil, err
	}
	content_text := string(content)
	var publish_time_text string
	create_time_pattern := regexp.MustCompile(`var\s+createTime\s*=\s*'([^']+)'`)
	matches := create_time_pattern.FindStringSubmatch(content_text)
	if len(matches) > 1 {
		publish_time_text = format_publish_time(matches[1], 0)
	}
	data, err := parse_cgi_datanew(content_text)
	if err != nil {
		article, fallback_err := parse_article_from_dom(content_text)
		if fallback_err != nil {
			return nil, err
		}
		return article, nil
	}
	if publish_time_text == "" {
		publish_time_text = format_publish_time(data.CreateTime, int(data.OriCreateTime))
	}
	article := new_wechat_official_article(data, publish_time_text)
	article.PageJSON = data
	article.PageHTML = content_text
	return article, nil
}

func (c *Client) Scrape(raw_url string) ([]byte, error) {
	if raw_url == "" {
		return nil, fmt.Errorf("url is empty")
	}
	response, err := c.fetch_with_clawreq(raw_url, 25*time.Second, windows_wechat_clawreq_headers(""))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s body=%s", response.Status, strings.TrimSpace(string(debug_body_snippet(response.Body))))
	}
	if is_verification_page(string(response.Body)) {
		return nil, fmt.Errorf("wechat verification page returned for %s", raw_url)
	}
	if err := c.write_cached_html(raw_url, response.Body); err != nil {
		return nil, fmt.Errorf("cache wxmp html response for %q: %w", raw_url, err)
	}
	return response.Body, nil
}

func (c *Client) fetch_with_clawreq(raw_url string, timeout time.Duration, headers map[string]string) (*clawreq.Response, error) {
	client, err := clawreq.New(clawreq.Config{
		Profile:         clawreq.ProfileChrome,
		Timeout:         timeout,
		FollowRedirects: true,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize wxmp clawreq client: %w", err)
	}
	opts := []clawreq.RequestOption{
		clawreq.WithOnlyHeaders(headers),
	}
	return client.Do(context.Background(), http.MethodGet, raw_url, nil, opts...)
}

func http_response_from_clawreq(response *clawreq.Response) *http.Response {
	if response == nil {
		return &http.Response{
			StatusCode: 0,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     http.Header{},
		}
	}
	return &http.Response{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Header:     response.Header.Clone(),
		Body:       io.NopCloser(bytes.NewReader(response.Body)),
	}
}

func windows_wechat_clawreq_headers(referer string) map[string]string {
	return map[string]string{
		"Content-Type":    "application/json",
		"Accept-Language": "en-US,en;q=0.9",
		"Priority":        "u=1, i",
		"Referer":         referer,
		"Sec-Fetch-Dest":  "empty",
		"Sec-Fetch-Mode":  "cors",
		"Sec-Fetch-Site":  "same-origin",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63090a13) UnifiedPCWindowsWechat(0xf2541022) XWEB/16467 Flue",
	}
}

func debug_body_snippet(body []byte) []byte {
	if len(body) <= 2048 {
		return body
	}
	return body[:2048]
}

func new_wechat_official_article(data *CgiDataNew, publish_time_text string) *WechatOfficialArticle {
	if data == nil {
		return &WechatOfficialArticle{}
	}
	article := &WechatOfficialArticle{
		Type:                data.PageType,
		Title:               data.Title,
		Content:             data.ContentNoEncode,
		PublishTimeStr:      publish_time_text,
		ContentLength:       len(data.ContentNoEncode),
		Creator:             data.Author,
		AuthorNickname:      data.NickName,
		AuthorAvatar:        first_non_empty(data.RoundHeadImg, data.OriHeadImgUrl, data.HdHeadImg),
		AuthorID:            data.UserName,
		Images:              make([]string, 0),
		Videos:              data.VideoPageInfos,
		PicturePageInfoList: data.PicturePageInfoList,
	}
	for _, item := range data.PicturePageInfoList {
		if item.CdnUrl != "" {
			article.Images = append(article.Images, item.CdnUrl)
		}
	}
	return article
}

func ExtractArticleID(raw_url string) string {
	u := raw_url
	lower_url := strings.ToLower(u)
	if strings.HasPrefix(lower_url, "officialaccount://") {
		u = u[len("officialaccount://"):]
		if !strings.HasPrefix(u, "http") {
			u = "https://" + u
		}
	}

	parsed_url, err := url.Parse(u)
	if err != nil {
		return ""
	}
	if !strings.EqualFold(parsed_url.Hostname(), "mp.weixin.qq.com") {
		return ""
	}
	path := strings.TrimRight(parsed_url.Path, "/")
	if strings.HasPrefix(path, "/s/") {
		short_id := strings.TrimPrefix(path, "/s/")
		if short_id != "" && !strings.Contains(short_id, "/") {
			return short_id
		}
	}
	query := parsed_url.Query()
	mid := query.Get("mid")
	idx := query.Get("idx")
	if mid != "" {
		if idx == "" {
			idx = "1"
		}
		return mid + "_" + idx
	}
	if query.Get("__biz") != "" {
		sn := query.Get("sn")
		if sn != "" {
			return query.Get("__biz") + "_" + sn
		}
		sum := md5.Sum([]byte(parsed_url.EscapedPath() + "?" + parsed_url.RawQuery))
		return query.Get("__biz") + "_" + hex.EncodeToString(sum[:8])
	}
	return ""
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (c *Client) BuildURL(raw_url string, params map[string]string) string {
	u, _ := url.Parse(raw_url)
	query := u.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) FetchProxy(target_url string) (*http.Response, error) {
	target_url = strings.ReplaceAll(target_url, "&amp;", "&")
	http_client := &http.Client{}
	request, err := http.NewRequest(http.MethodGet, target_url, nil)
	if err != nil {
		return nil, new_scraper_error(ErrorKindProxyRequest, ErrorMessage(ErrorKindProxyRequest), err)
	}
	request.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	request.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	request.Header.Set("priority", "u=0, i")
	request.Header.Set("sec-ch-ua", `"Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"`)
	request.Header.Set("sec-ch-ua-mobile", "?0")
	request.Header.Set("sec-ch-ua-platform", `"macOS"`)
	request.Header.Set("sec-fetch-dest", "document")
	request.Header.Set("sec-fetch-mode", "navigate")
	request.Header.Set("sec-fetch-site", "none")
	request.Header.Set("sec-fetch-user", "?1")
	request.Header.Set("upgrade-insecure-requests", "1")
	request.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	response, err := http_client.Do(request)
	if err != nil {
		return nil, new_scraper_error(ErrorKindProxyDispatch, ErrorMessage(ErrorKindProxyDispatch), err)
	}
	return response, nil
}

func (c *Client) BuildMsgListURL(acct *OfficialAccount, offset int) string {
	return c.BuildURL("https://mp.weixin.qq.com/mp/profile_ext", map[string]string{
		"action":      "getmsg",
		"__biz":       acct.Biz,
		"uin":         acct.Uin,
		"key":         acct.Key,
		"pass_ticket": acct.PassTicket,
		"wxtoken":     "",
		"x5":          "0",
		"count":       "10",
		"offset":      strconv.Itoa(offset),
		"f":           "json",
	})
}

func (c *Client) BuildMsgListReferer(acct *OfficialAccount) string {
	params := url.Values{}
	params.Set("action", "home")
	params.Set("__biz", acct.Biz)
	params.Set("scene", "124")
	params.Set("uin", acct.Uin)
	params.Set("key", acct.Key)
	params.Set("devicetype", "UnifiedPCWindows")
	params.Set("version", "f2541022")
	params.Set("lang", "zh_CN")
	params.Set("a8scene", "1")
	params.Set("acctmode", "0")
	params.Set("pass_ticket", acct.PassTicket)
	return "https://mp.weixin.qq.com/mp/profile_ext?" + params.Encode()
}

func (c *Client) FetchMsgList(acct *OfficialAccount, offset int) (*OfficialMsgListResp, error) {
	if acct == nil {
		return nil, new_scraper_error(ErrorKindMissingBiz, ErrorMessage(ErrorKindMissingBiz), nil)
	}
	logger := c.logger.With().
		Str("biz", acct.Biz).
		Int("offset", offset).
		Logger()
	return c.fetch_msg_list(logger, acct, offset, true)
}

func (c *Client) fetch_msg_list_inline(logger zerolog.Logger, acct *OfficialAccount, offset int) (*OfficialMsgListResp, error) {
	return c.fetch_msg_list(logger, acct, offset, false)
}

func (c *Client) fetch_msg_list(logger zerolog.Logger, acct *OfficialAccount, offset int, classify_account_error bool) (*OfficialMsgListResp, error) {
	logger.Info().Msg("fetch msg list: start")
	if acct == nil || acct.Biz == "" {
		return nil, new_scraper_error(ErrorKindMissingBiz, ErrorMessage(ErrorKindMissingBiz), nil)
	}
	target_url := c.BuildMsgListURL(acct, offset)
	referer := c.BuildMsgListReferer(acct)
	response, err := c.Fetch(target_url, referer)
	if err != nil {
		kind := ErrorKindFetchMessage
		message := ErrorMessage(kind)
		reason := safe_net_reason(err)
		if reason == ErrorMessage(ErrorKindTimeout) {
			kind = ErrorKindTimeout
			message = reason
		} else if reason != "" {
			message = fmt.Sprintf("%s: %s", message, reason)
		}
		return nil, new_scraper_error(kind, message, err)
	}
	defer response.Body.Close()
	response_body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, new_scraper_error(ErrorKindFetchMessage, "读取响应失败", err)
	}
	var data OfficialMsgListResp
	if err := json.Unmarshal(response_body, &data); err != nil {
		return nil, new_scraper_error(ErrorKindDataParse, ErrorMessage(ErrorKindDataParse), err)
	}
	if data.Ret != 0 {
		if classify_account_error && data.Ret == -3 {
			return nil, new_scraper_error(ErrorKindAccountExpired, ErrorMessage(ErrorKindAccountExpired), nil)
		}
		if classify_account_error && data.Ret == -6 {
			return nil, new_scraper_error(ErrorKindAccountBanned, ErrorMessage(ErrorKindAccountBanned), nil)
		}
		message := data.ErrMsg
		if strings.TrimSpace(message) == "" {
			message = ErrorMessage(ErrorKindFetchMessage)
		}
		return nil, new_scraper_error(ErrorKindFetchMessage, message, nil)
	}
	logger.Info().Int("ret", data.Ret).Msg("fetch msg list: completed")
	return &data, nil
}

func (c *Client) FetchArticleList(acct *OfficialAccount) (*ArticleListResponse, error) {
	if acct == nil {
		return nil, new_scraper_error(ErrorKindAccountNotFound, ErrorMessage(ErrorKindAccountNotFound), nil)
	}
	if strings.TrimSpace(acct.AuthorId) == "" {
		return nil, new_scraper_error(ErrorKindInvalidArgument, "缺少 author_id", nil)
	}
	if strings.TrimSpace(acct.AppmsgToken) == "" {
		return nil, new_scraper_error(ErrorKindInvalidArgument, "缺少 appmsg_token", nil)
	}
	if acct.Cookie == "" || time.Now().Unix() >= acct.CookieExpiration {
		if err := c.fetch_cookie(acct); err != nil {
			return nil, err
		}
	}

	target_url := fmt.Sprintf("https://mp.weixin.qq.com/mp/author?action=get_articles&author_id=%s&scene=142&limit=30&version=undefined&appmsg_token=%s&x5=0&f=json&user_article_role=0", acct.AuthorId, acct.AppmsgToken)
	request, err := http.NewRequest(http.MethodGet, target_url, nil)
	if err != nil {
		return nil, err
	}
	referer_params := url.Values{}
	referer_params.Set("action", "show")
	referer_params.Set("__biz", acct.Biz)
	referer_params.Set("idx", "1")
	referer_params.Set("author_id", acct.AuthorId)
	referer_params.Set("scene", "142")
	referer_params.Set("rscene", "128")
	referer_params.Set("uin", acct.Uin)
	referer_params.Set("key", acct.Key)
	referer_params.Set("devicetype", "UnifiedPCMac")
	referer_params.Set("version", "f2640619")
	referer_params.Set("lang", "zh_CN")
	referer_params.Set("ascene", "1")
	referer_params.Set("acctmode", "0")
	referer_params.Set("pass_ticket", acct.PassTicket)
	referer_params.Set("countrycode", "CN")
	request.Header.Set("Cookie", acct.Cookie)
	request.Header.Set("accept", "*/*")
	request.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	request.Header.Set("priority", "u=1, i")
	request.Header.Set("referer", "https://mp.weixin.qq.com/mp/author?"+referer_params.Encode())
	request.Header.Set("sec-ch-ua", `"Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"`)
	request.Header.Set("sec-ch-ua-mobile", "?0")
	request.Header.Set("sec-ch-ua-platform", `"macOS"`)
	request.Header.Set("sec-fetch-dest", "empty")
	request.Header.Set("sec-fetch-mode", "cors")
	request.Header.Set("sec-fetch-site", "same-origin")
	request.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	request.Header.Set("x-requested-with", "XMLHttpRequest")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	response_body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var data ArticleListResponse
	if err := json.Unmarshal(response_body, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *Client) fetch_cookie(acct *OfficialAccount) error {
	u := url.URL{Scheme: "https", Host: "mp.weixin.qq.com", Path: "/mp/author"}
	query := u.Query()
	query.Set("action", "show")
	query.Set("__biz", acct.Biz)
	query.Set("idx", "1")
	if acct.AuthorId != "" {
		query.Set("author_id", acct.AuthorId)
	}
	query.Set("scene", "142")
	query.Set("rscene", "128")
	query.Set("uin", acct.Uin)
	query.Set("key", acct.Key)
	query.Set("devicetype", "UnifiedPCMac")
	query.Set("version", "f2640619")
	query.Set("lang", "zh_CN")
	query.Set("ascene", "1")
	query.Set("acctmode", "0")
	query.Set("pass_ticket", acct.PassTicket)
	query.Set("countrycode", "CN")
	u.RawQuery = query.Encode()
	request, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	response_cookies := response.Cookies()
	if len(response_cookies) == 0 {
		return errors.New("no cookie found")
	}
	cookie_parts := make([]string, 0, len(response_cookies))
	for _, cookie := range response_cookies {
		cookie_parts = append(cookie_parts, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	acct.Cookie = strings.Join(cookie_parts, "; ")
	acct.CookieExpiration = time.Now().Add(24 * time.Hour).Unix()
	return nil
}

func (c *Client) FetchAllMsgURLs(acct *OfficialAccount) ([]string, error) {
	var urls []string
	offset := 0
	logger := c.logger.With().Str("biz", acct.Biz).Logger()
	build_url := func(raw_url string) string {
		if raw_url == "" {
			return ""
		}
		raw_url = html.UnescapeString(raw_url)
		if strings.HasPrefix(raw_url, "http://") || strings.HasPrefix(raw_url, "https://") {
			return raw_url
		}
		return "https://mp.weixin.qq.com" + raw_url
	}
	for {
		data, err := c.fetch_msg_list_inline(logger, acct, offset)
		if err != nil {
			return urls, err
		}
		var message_list struct {
			List []OfficialAccountMsgListRespItem `json:"list"`
		}
		if err := json.Unmarshal([]byte(data.MsgList), &message_list); err != nil {
			return urls, fmt.Errorf("解析推送列表失败: %w", err)
		}
		for _, item := range message_list.List {
			if article_url := build_url(item.MsgExtInfo.ContentUrl); article_url != "" {
				urls = append(urls, article_url)
			}
			for _, sub_item := range item.MsgExtInfo.MultiAppMsgItemList {
				if article_url := build_url(sub_item.ContentUrl); article_url != "" {
					urls = append(urls, article_url)
				}
			}
		}
		if data.HasMore == 0 {
			break
		}
		offset = data.NextOffset
	}
	return urls, nil
}

func (c *Client) FetchFullContent(raw_url string) string {
	request, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return ""
	}
	request.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	request.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	request.Header.Set("upgrade-insecure-requests", "1")
	request.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	response_body, err := io.ReadAll(response.Body)
	if err != nil {
		return ""
	}
	if err := c.write_cached_html(raw_url, response_body); err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Str("url", raw_url).Msg("cache wxmp full-content HTML response")
		}
		return ""
	}
	body := string(response_body)
	content_pattern := regexp.MustCompile(`(?s)<div[^>]*id="js_content"[^>]*>(.*?)</div>`)
	matches := content_pattern.FindStringSubmatch(body)
	if len(matches) < 2 {
		return body
	}
	content := matches[1]
	content = regexp.MustCompile(`\sdata-src="([^"]+)"`).ReplaceAllString(content, ` src="$1"`)
	return strings.ReplaceAll(content, `src="//`, `src="https://`)
}

type Article struct {
	Biz            string `json:"__biz"`
	CoverURL       string `json:"cover_url"`
	IsPaid         int    `json:"is_paid"`
	IsPaySubscribe int    `json:"is_pay_subscribe"`
	ItemShowType   int    `json:"item_show_type"`
	Mid            string `json:"mid"`
	PublishTime    int64  `json:"publish_time"`
	Title          string `json:"title"`
	URL            string `json:"url"`
}

type ArticleListResponse struct {
	Ret      int       `json:"ret"`
	ErrMsg   string    `json:"errmsg"`
	Articles []Article `json:"articles"`
	BaseResp struct {
		ExportKeyToken string `json:"exportkey_token"`
		Ret            int    `json:"ret"`
	} `json:"base_resp"`
	MaxArticleID string `json:"max_article_id"`
}
