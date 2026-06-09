package officialaccount

// ProbeOutput is the WeChat official account article output extracted during
// Probe and exposed through the shared content envelope.
type ProbeOutput struct {
	// Format is the rendered output format, usually html for article content.
	Format string `json:"format,omitempty"`
	// ContentType classifies the probed official account content, such as article.
	ContentType string `json:"content_type,omitempty"`
	// Title is the article title.
	Title string `json:"title,omitempty"`
	// SourceURL is the original official account article URL.
	SourceURL string `json:"source_url,omitempty"`
	// BodyHTML is the sanitized article HTML body.
	BodyHTML string `json:"body_html,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	return map[string]any{
		"format":       o.Format,
		"content_type": o.ContentType,
		"title":        o.Title,
		"source_url":   o.SourceURL,
		"body_html":    o.BodyHTML,
	}
}
