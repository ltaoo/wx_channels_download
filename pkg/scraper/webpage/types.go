// Package webpage fetches and extracts content from generic web pages.
package webpage

// PlatformID is the stable platform identifier used by the fallback adapter.
const PlatformID = "webpage"

// Page is the normalized result of fetching and extracting one web page.
type Page struct {
	URL          string `json:"url"`
	RequestedURL string `json:"requested_url"`
	FinalURL     string `json:"final_url"`
	CanonicalURL string `json:"canonical_url,omitempty"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	Author       string `json:"author,omitempty"`
	SiteName     string `json:"site_name,omitempty"`
	Language     string `json:"language,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
	FaviconURL   string `json:"favicon_url,omitempty"`
	PublishTime  *int64 `json:"publish_time,omitempty"`
	HTML         string `json:"html"`
	Text         string `json:"text"`
	Markdown     string `json:"markdown"`
	StatusCode   int    `json:"status_code"`
	ContentType  string `json:"content_type,omitempty"`
}

// ArchiveURL returns the best archive URL exposed by the page.
func (p *Page) ArchiveURL() string {
	if p == nil {
		return ""
	}
	if p.URL != "" {
		return p.URL
	}
	if p.CanonicalURL != "" {
		return p.CanonicalURL
	}
	if p.FinalURL != "" {
		return p.FinalURL
	}
	return p.RequestedURL
}
