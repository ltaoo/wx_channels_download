package api

import (
	"encoding/xml"
	"net/http"
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

	// Use service
	resp, err := c.channelsService.FetchFeedList(username, nextMarker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	entries := []AtomEntry{}
	atom := AtomFeed{
		Title:   "WeChat Channels",
		ID:      username,
		Updated: time.Now().Format(time.RFC3339),
		Link: []AtomLink{
			{Rel: "self", Href: "http://" + ctx.Request.Host + ctx.Request.RequestURI},
			{Rel: "alternate", Href: "https://channels.weixin.qq.com"},
		},
		Entry: entries,
	}

	_ = resp // 已使用 channelsService
	ctx.Header("Content-Type", "application/atom+xml; charset=utf-8")
	ctx.XML(http.StatusOK, atom)
}
