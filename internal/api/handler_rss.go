package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/util"
)

type AtomAuthor struct {
	Name string `xml:"name"`
}

type AtomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type AtomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type AtomEntry struct {
	Title     string      `xml:"title"`
	ID        string      `xml:"id"`
	Updated   string      `xml:"updated"`
	Published string      `xml:"published"`
	Link      []AtomLink  `xml:"link"`
	Content   AtomContent `xml:"content"`
	Author    AtomAuthor  `xml:"author"`
}

type AtomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Link    []AtomLink  `xml:"link"`
	Author  AtomAuthor  `xml:"author"`
	Entry   []AtomEntry `xml:"entry"`
}

func (c *APIClient) handleFetchFeedListOfContactRSS(ctx *gin.Context) {
	username := ctx.Query("username")
	nextMarker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsFeedListOfContact(username, nextMarker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	entries := make([]AtomEntry, 0, len(resp.Data.Object))
	for _, obj := range resp.Data.Object {
		var mediaURL, coverURL string
		if len(obj.ObjectDesc.Media) > 0 {
			m := obj.ObjectDesc.Media[0]
			videoURL := m.URL + m.URLToken
			addr := c.cfg.Protocol + "://" + c.cfg.Hostname
			if c.cfg.Port != 80 {
				addr += ":" + strconv.Itoa(c.cfg.Port)
			}
			mediaURL = addr + "/play?url=" + url.QueryEscape(videoURL) + "&key=" + m.DecodeKey
			coverURL = m.CoverUrl
		}

		desc := obj.ObjectDesc.Description
		if coverURL != "" && mediaURL != "" {
			desc = fmt.Sprintf(`<img src="%s" style="display: none;" /><video controls poster="%s"><source src="%s" type="video/mp4"></video><br/>%s`, coverURL, coverURL, mediaURL, desc)
		} else if coverURL != "" {
			desc = fmt.Sprintf(`<img src="%s" /><br/>%s`, coverURL, desc)
		}

		pubDate := time.Unix(int64(obj.CreateTime), 0).Format(time.RFC3339)

		entries = append(entries, AtomEntry{
			Title:     obj.ObjectDesc.Description,
			ID:        obj.ID,
			Updated:   pubDate,
			Published: pubDate,
			Link: []AtomLink{
				{Rel: "alternate", Href: mediaURL},
			},
			Content: AtomContent{
				Type: "html",
				Body: desc,
			},
			Author: AtomAuthor{
				Name: obj.Contact.Nickname,
			},
		})
	}

	links := []AtomLink{
		{Rel: "self", Href: "http://" + ctx.Request.Host + ctx.Request.RequestURI},
		{Rel: "alternate", Href: "https://channels.weixin.qq.com"},
	}

	if resp.Data.ContinueFlag != 0 && resp.Data.LastBuffer != "" {
		u := ctx.Request.URL
		q := u.Query()
		q.Set("next_marker", resp.Data.LastBuffer)
		u.RawQuery = q.Encode()
		nextLink := "http://" + ctx.Request.Host + u.String()
		links = append(links, AtomLink{Rel: "next", Href: nextLink})
	}

	atom := AtomFeed{
		Title:   resp.Data.Contact.Nickname,
		ID:      resp.Data.Contact.Username,
		Updated: time.Now().Format(time.RFC3339),
		Link:    links,
		Author: AtomAuthor{
			Name: resp.Data.Contact.Nickname,
		},
		Entry: entries,
	}

	ctx.Header("Content-Type", "application/atom+xml; charset=utf-8")
	ctx.XML(http.StatusOK, atom)
}

