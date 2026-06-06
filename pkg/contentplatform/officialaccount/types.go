package officialaccount

type ProbeOutput struct {
	Format      string `json:"format,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Title       string `json:"title,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
	BodyHTML    string `json:"body_html,omitempty"`
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
