package officialaccount

import "encoding/xml"

type AtomAuthor struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}
type AtomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}
type AtomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}
type MediaThumbnail struct {
	XMLName    xml.Name `xml:"media:thumbnail"`
	XMLNSMedia string   `xml:"xmlns:media,attr"`
	URL        string   `xml:"url,attr"`
	Width      int      `xml:"width,attr,omitempty"`
	Height     int      `xml:"height,attr,omitempty"`
}
type AtomEntry struct {
	ID             string          `xml:"id"`
	Title          string          `xml:"title"`
	Updated        string          `xml:"updated"`
	Published      string          `xml:"published"`
	Author         AtomAuthor      `xml:"author"`
	Link           []AtomLink      `xml:"link"`
	Content        AtomContent     `xml:"content"`
	Summary        AtomContent     `xml:"summary"`
	MediaThumbnail *MediaThumbnail `xml:"media:thumbnail"`
}
type AtomCategory struct {
	Term string `xml:"term,attr"`
}
type AtomFeed struct {
	XMLName   xml.Name       `xml:"http://www.w3.org/2005/Atom feed"`
	Title     string         `xml:"title"`
	ID        string         `xml:"id"`
	Updated   string         `xml:"updated"`
	Generator string         `xml:"generator"`
	Icon      string         `xml:"icon"`
	Category  []AtomCategory `xml:"category"`
	Link      []AtomLink     `xml:"link"`
	Author    AtomAuthor     `xml:"author"`
	Entry     []AtomEntry    `xml:"entry"`
}
