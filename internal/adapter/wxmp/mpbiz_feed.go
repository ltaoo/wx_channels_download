package wxmpadapter

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/pkg/scraper/wxmp"
)

type mpbiz_account struct {
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Signature string `json:"signature"`
}

type mpbiz_article struct {
	MsgID       int64  `json:"msg_id"`
	AppMsgID    int64  `json:"app_msg_id"`
	ItemIndex   int    `json:"item_index"`
	Title       string `json:"title"`
	Digest      string `json:"digest"`
	URL         string `json:"url"`
	CoverURL    string `json:"cover_url"`
	PublishTime int64  `json:"publish_time"`
}

type mpbiz_message_list struct {
	Account  mpbiz_account   `json:"account"`
	Articles []mpbiz_article `json:"articles"`
	Offset   string          `json:"offset"`
	IsEnd    bool            `json:"is_end"`
	ErrCode  *int            `json:"errCode"`
	ErrMsg   string          `json:"errMsg"`
}

type mpbiz_feed struct {
	ID          string
	Title       string
	Description string
	HomeURL     string
	FeedURL     string
	NextURL     string
	Icon        string
	Updated     time.Time
	Articles    []mpbiz_article
}

type mpbiz_rss struct {
	XMLName    xml.Name          `xml:"rss"`
	Version    string            `xml:"version,attr"`
	XMLNSAtom  string            `xml:"xmlns:atom,attr"`
	XMLNSMedia string            `xml:"xmlns:media,attr"`
	Channel    mpbiz_rss_channel `xml:"channel"`
}

type mpbiz_rss_channel struct {
	Title         string                `xml:"title"`
	Link          string                `xml:"link"`
	Description   string                `xml:"description"`
	Language      string                `xml:"language"`
	LastBuildDate string                `xml:"lastBuildDate"`
	Generator     string                `xml:"generator"`
	AtomLinks     []mpbiz_rss_atom_link `xml:"atom:link"`
	Image         *mpbiz_rss_image      `xml:"image,omitempty"`
	Items         []mpbiz_rss_item      `xml:"item"`
}

type mpbiz_rss_atom_link struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type mpbiz_rss_image struct {
	URL   string `xml:"url"`
	Title string `xml:"title"`
	Link  string `xml:"link"`
}

type mpbiz_rss_item struct {
	Title       string               `xml:"title"`
	Link        string               `xml:"link"`
	Description string               `xml:"description"`
	GUID        mpbiz_rss_guid       `xml:"guid"`
	PubDate     string               `xml:"pubDate"`
	Thumbnail   *wxmp.MediaThumbnail `xml:"media:thumbnail,omitempty"`
}

type mpbiz_rss_guid struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type mpbiz_json_feed struct {
	Version     string                   `json:"version"`
	Title       string                   `json:"title"`
	HomePageURL string                   `json:"home_page_url"`
	FeedURL     string                   `json:"feed_url"`
	NextURL     string                   `json:"next_url,omitempty"`
	Description string                   `json:"description,omitempty"`
	Icon        string                   `json:"icon,omitempty"`
	Authors     []mpbiz_json_feed_author `json:"authors,omitempty"`
	Items       []mpbiz_json_feed_item   `json:"items"`
}

type mpbiz_json_feed_author struct {
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

type mpbiz_json_feed_item struct {
	ID            string                   `json:"id"`
	URL           string                   `json:"url,omitempty"`
	Title         string                   `json:"title"`
	ContentHTML   string                   `json:"content_html"`
	Summary       string                   `json:"summary,omitempty"`
	Image         string                   `json:"image,omitempty"`
	DatePublished string                   `json:"date_published"`
	Authors       []mpbiz_json_feed_author `json:"authors,omitempty"`
}

func (r *Routes) handle_mpbiz_feed(ctx *gin.Context) {
	format := strings.ToLower(ctx.Param("format"))
	if format == "" {
		format = "rss"
	}
	if format != "rss" && format != "rss2" && format != "atom" && format != "json" && format != "jsonfeed" {
		result.Err(ctx, api_code_invalid_params, "不支持的订阅格式："+format)
		return
	}

	raw_data, err := r.server.FetchBizMsgList(ctx.Query("username"), ctx.Query("offset"))
	if err != nil {
		write_client_error(ctx, err, api_code_fetch_message)
		return
	}
	var data mpbiz_message_list
	if err := json.Unmarshal(raw_data, &data); err != nil {
		write_client_error(ctx, err, api_code_data_parse)
		return
	}
	if error_code, error_message, is_error := data.error_response(); is_error {
		result.Err(ctx, error_code, error_message)
		return
	}

	feed := new_mpbiz_feed(data, ctx.Query("username"), absolute_request_url(ctx))
	switch format {
	case "atom":
		ctx.Header("Content-Type", "application/atom+xml; charset=utf-8")
		ctx.XML(http.StatusOK, build_mpbiz_atom(feed))
	case "json", "jsonfeed":
		ctx.Header("Content-Type", "application/feed+json; charset=utf-8")
		ctx.JSON(http.StatusOK, build_mpbiz_json_feed(feed))
	default:
		ctx.Header("Content-Type", "application/rss+xml; charset=utf-8")
		ctx.XML(http.StatusOK, build_mpbiz_rss(feed))
	}
}

func (data mpbiz_message_list) error_response() (int, string, bool) {
	if data.ErrCode == nil {
		return 0, "", false
	}
	error_code := *data.ErrCode
	if error_code == 0 {
		error_code = api_code_fetch_message
	}
	error_message := strings.TrimSpace(data.ErrMsg)
	if error_message == "" {
		error_message = api_error_messages[api_code_fetch_message]
	}
	return error_code, error_message, true
}

func absolute_request_url(ctx *gin.Context) string {
	scheme := "http"
	if ctx.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded_scheme := ctx.GetHeader("X-Forwarded-Proto"); forwarded_scheme == "http" || forwarded_scheme == "https" {
		scheme = forwarded_scheme
	}
	return scheme + "://" + ctx.Request.Host + ctx.Request.URL.RequestURI()
}

func new_mpbiz_feed(data mpbiz_message_list, username, feed_url string) mpbiz_feed {
	feed_id := data.Account.Username
	if feed_id == "" {
		feed_id = username
	}
	title := data.Account.Nickname
	if title == "" {
		title = feed_id
	}
	if title == "" {
		title = "微信公众号"
	}
	home_url := "https://mp.weixin.qq.com/"
	updated := time.Time{}
	for _, article := range data.Articles {
		if article.URL != "" && home_url == "https://mp.weixin.qq.com/" {
			home_url = article.URL
		}
		if article.PublishTime > updated.Unix() {
			updated = time.Unix(article.PublishTime, 0).UTC()
		}
	}
	if updated.IsZero() {
		updated = time.Now().UTC()
	}
	description := data.Account.Signature
	if description == "" {
		description = title + "的微信公众号文章"
	}

	return mpbiz_feed{
		ID:          feed_id,
		Title:       title,
		Description: description,
		HomeURL:     home_url,
		FeedURL:     feed_url,
		NextURL:     mpbiz_next_url(feed_url, data.Offset, data.IsEnd),
		Icon:        data.Account.AvatarURL,
		Updated:     updated,
		Articles:    data.Articles,
	}
}

func mpbiz_next_url(feed_url, offset string, is_end bool) string {
	if is_end || offset == "" {
		return ""
	}
	next_url, err := url.Parse(feed_url)
	if err != nil {
		return ""
	}
	query := next_url.Query()
	query.Set("offset", offset)
	next_url.RawQuery = query.Encode()
	return next_url.String()
}

func mpbiz_article_id(feed_id string, article mpbiz_article) string {
	if article.URL != "" {
		return article.URL
	}
	app_msg_id := article.AppMsgID
	if app_msg_id == 0 {
		app_msg_id = article.MsgID
	}
	return fmt.Sprintf("urn:wechat:mpbiz:%s:%d:%d", feed_id, app_msg_id, article.ItemIndex)
}

func mpbiz_article_html(article mpbiz_article) string {
	digest := html.EscapeString(article.Digest)
	if article.CoverURL == "" {
		return digest
	}
	return fmt.Sprintf(`<img src="%s" alt=""><br>%s`, html.EscapeString(article.CoverURL), digest)
}

func build_mpbiz_rss(feed mpbiz_feed) mpbiz_rss {
	links := []mpbiz_rss_atom_link{{Href: feed.FeedURL, Rel: "self", Type: "application/rss+xml"}}
	items := make([]mpbiz_rss_item, 0, len(feed.Articles))
	for _, article := range feed.Articles {
		article_id := mpbiz_article_id(feed.ID, article)
		item := mpbiz_rss_item{
			Title:       article.Title,
			Link:        article.URL,
			Description: mpbiz_article_html(article),
			GUID:        mpbiz_rss_guid{IsPermaLink: article.URL != "", Value: article_id},
			PubDate:     time.Unix(article.PublishTime, 0).UTC().Format(time.RFC1123Z),
		}
		if article.CoverURL != "" {
			item.Thumbnail = &wxmp.MediaThumbnail{
				XMLNSMedia: "http://search.yahoo.com/mrss/",
				URL:        article.CoverURL,
			}
		}
		items = append(items, item)
	}
	var image *mpbiz_rss_image
	if feed.Icon != "" {
		image = &mpbiz_rss_image{URL: feed.Icon, Title: feed.Title, Link: feed.HomeURL}
	}
	return mpbiz_rss{
		Version:    "2.0",
		XMLNSAtom:  "http://www.w3.org/2005/Atom",
		XMLNSMedia: "http://search.yahoo.com/mrss/",
		Channel: mpbiz_rss_channel{
			Title:         feed.Title,
			Link:          feed.HomeURL,
			Description:   feed.Description,
			Language:      "zh-CN",
			LastBuildDate: feed.Updated.Format(time.RFC1123Z),
			Generator:     "FindRSS",
			AtomLinks:     links,
			Image:         image,
			Items:         items,
		},
	}
}

func build_mpbiz_atom(feed mpbiz_feed) wxmp.AtomFeed {
	links := []wxmp.AtomLink{
		{Rel: "self", Href: feed.FeedURL},
		{Rel: "alternate", Href: feed.HomeURL},
	}
	if feed.NextURL != "" {
		links = append(links, wxmp.AtomLink{Rel: "next", Href: feed.NextURL})
	}
	author := wxmp.AtomAuthor{Name: feed.Title, URI: feed.HomeURL}
	entries := make([]wxmp.AtomEntry, 0, len(feed.Articles))
	for _, article := range feed.Articles {
		published := time.Unix(article.PublishTime, 0).UTC().Format(time.RFC3339)
		entry := wxmp.AtomEntry{
			ID:        mpbiz_article_id(feed.ID, article),
			Title:     article.Title,
			Updated:   published,
			Published: published,
			Author:    author,
			Link:      []wxmp.AtomLink{{Rel: "alternate", Href: article.URL}},
			Content:   wxmp.AtomContent{Type: "html", Body: mpbiz_article_html(article)},
			Summary:   wxmp.AtomContent{Type: "text", Body: article.Digest},
		}
		if article.CoverURL != "" {
			entry.MediaThumbnail = &wxmp.MediaThumbnail{
				XMLNSMedia: "http://search.yahoo.com/mrss/",
				URL:        article.CoverURL,
			}
		}
		entries = append(entries, entry)
	}
	return wxmp.AtomFeed{
		Title:     feed.Title,
		ID:        "urn:wechat:mpbiz:" + feed.ID,
		Updated:   feed.Updated.Format(time.RFC3339),
		Generator: "FindRSS",
		Icon:      feed.Icon,
		Category:  []wxmp.AtomCategory{{Term: "微信公众号"}},
		Link:      links,
		Author:    author,
		Entry:     entries,
	}
}

func build_mpbiz_json_feed(feed mpbiz_feed) mpbiz_json_feed {
	author := mpbiz_json_feed_author{Name: feed.Title, URL: feed.HomeURL, Avatar: feed.Icon}
	items := make([]mpbiz_json_feed_item, 0, len(feed.Articles))
	for _, article := range feed.Articles {
		items = append(items, mpbiz_json_feed_item{
			ID:            mpbiz_article_id(feed.ID, article),
			URL:           article.URL,
			Title:         article.Title,
			ContentHTML:   mpbiz_article_html(article),
			Summary:       article.Digest,
			Image:         article.CoverURL,
			DatePublished: time.Unix(article.PublishTime, 0).UTC().Format(time.RFC3339),
			Authors:       []mpbiz_json_feed_author{author},
		})
	}
	return mpbiz_json_feed{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       feed.Title,
		HomePageURL: feed.HomeURL,
		FeedURL:     feed.FeedURL,
		NextURL:     feed.NextURL,
		Description: feed.Description,
		Icon:        feed.Icon,
		Authors:     []mpbiz_json_feed_author{author},
		Items:       items,
	}
}
