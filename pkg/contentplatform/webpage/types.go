package webpage

const (
	PlatformID         = "webpage"
	ContentTypeArticle = "article"
	OutputFormatHTML   = "html"
)

type FetchedPage struct {
	URL         string
	StatusCode  int
	ContentType string
	HTML        string
}

type ArticlePage struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Description     string  `json:"description,omitempty"`
	Author          string  `json:"author,omitempty"`
	SiteName        string  `json:"site_name,omitempty"`
	Language        string  `json:"language,omitempty"`
	SourceURL       string  `json:"source_url"`
	CanonicalURL    string  `json:"canonical_url,omitempty"`
	CoverURL        string  `json:"cover_url,omitempty"`
	PublishedTime   string  `json:"published_time,omitempty"`
	ModifiedTime    string  `json:"modified_time,omitempty"`
	BodyHTML        string  `json:"body_html"`
	ExtractedHTML   string  `json:"extracted_html,omitempty"`
	BodyText        string  `json:"body_text,omitempty"`
	Extractor       string  `json:"extractor,omitempty"`
	ExtractorReason string  `json:"extractor_reason,omitempty"`
	Quality         float64 `json:"quality,omitempty"`
	RawHTMLLength   int     `json:"raw_html_length,omitempty"`
	ContentLength   int     `json:"content_length,omitempty"`
}

type ProbeOutput struct {
	Format          string  `json:"format,omitempty"`
	ContentType     string  `json:"content_type,omitempty"`
	ArticleID       string  `json:"article_id,omitempty"`
	Title           string  `json:"title,omitempty"`
	SourceURL       string  `json:"source_url,omitempty"`
	CanonicalURL    string  `json:"canonical_url,omitempty"`
	Author          string  `json:"author,omitempty"`
	PublishedTime   string  `json:"published_time,omitempty"`
	ModifiedTime    string  `json:"modified_time,omitempty"`
	BodyHTML        string  `json:"body_html,omitempty"`
	Extractor       string  `json:"extractor,omitempty"`
	ExtractorReason string  `json:"extractor_reason,omitempty"`
	Quality         float64 `json:"quality,omitempty"`
	ContentLength   int     `json:"content_length,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	return map[string]any{
		"format":           o.Format,
		"content_type":     o.ContentType,
		"article_id":       o.ArticleID,
		"title":            o.Title,
		"source_url":       o.SourceURL,
		"canonical_url":    o.CanonicalURL,
		"author":           o.Author,
		"published_time":   o.PublishedTime,
		"modified_time":    o.ModifiedTime,
		"body_html":        o.BodyHTML,
		"extractor":        o.Extractor,
		"extractor_reason": o.ExtractorReason,
		"quality":          o.Quality,
		"content_length":   o.ContentLength,
	}
}
