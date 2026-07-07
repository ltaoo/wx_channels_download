package v2ex

// ProbeOutput is the V2EX topic output extracted during Probe and exposed
// through the shared content envelope.
type ProbeOutput struct {
	// Format is the rendered output format, currently html.
	Format string `json:"format,omitempty"`
	// ContentType classifies the probed V2EX content, currently topic.
	ContentType string `json:"content_type,omitempty"`
	// TopicID is the V2EX topic id.
	TopicID string `json:"topic_id,omitempty"`
	// Title is the topic title.
	Title string `json:"title,omitempty"`
	// SourceURL is the original URL submitted by the user.
	SourceURL string `json:"source_url,omitempty"`
	// CanonicalURL is the normalized V2EX topic URL.
	CanonicalURL string `json:"canonical_url,omitempty"`
	// BodyHTML is the rendered full HTML document generated from the topic page.
	BodyHTML string `json:"body_html,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	return map[string]any{
		"format":        o.Format,
		"content_type":  o.ContentType,
		"topic_id":      o.TopicID,
		"title":         o.Title,
		"source_url":    o.SourceURL,
		"canonical_url": o.CanonicalURL,
		"body_html":     o.BodyHTML,
	}
}
