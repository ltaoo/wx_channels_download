package yeudusk

import "wx_channel/pkg/scraper/novelsource"

const PlatformID = "yeudusk"

var Source = novelsource.MustSource(PlatformID)

type Client = novelsource.Client
type PageURL = novelsource.PageURL
type Novel = novelsource.Novel
type Chapter = novelsource.Chapter
type ChapterContent = novelsource.ChapterContent

func NewClient(client novelsource.HTTPClient) *Client { return novelsource.NewClient(Source, client) }
func NewClientWithOptions(client novelsource.HTTPClient, cookie, userAgent string) *Client {
	return novelsource.NewClientWithOptions(Source, client, cookie, userAgent)
}
func CanParse(rawURL string) bool            { return Source.CanParse(rawURL) }
func ParseURL(rawURL string) (PageURL, bool) { return Source.ParseURL(rawURL) }
func ParseNovelHTML(pageURL string, htmlText string) (*Novel, error) {
	return Source.ParseNovelHTML(pageURL, htmlText)
}
func ParseChapterHTML(htmlText string) (*ChapterContent, error) {
	return Source.ParseChapterHTML(htmlText)
}
