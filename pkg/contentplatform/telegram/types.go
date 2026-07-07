package telegram

// ProbeOutput is the Telegram page output extracted during Probe and exposed
// through the shared content envelope.
type ProbeOutput struct {
	// Format is the rendered output format, currently html.
	Format string `json:"format,omitempty"`
	// ContentType classifies the probed Telegram content, such as channel or message.
	ContentType string `json:"content_type,omitempty"`
	// Username is the public Telegram channel username.
	Username string `json:"username,omitempty"`
	// MessageID is set when the URL points to one public message.
	MessageID int `json:"message_id,omitempty"`
	// Title is the generated page title.
	Title string `json:"title,omitempty"`
	// SourceURL is the original URL submitted by the user.
	SourceURL string `json:"source_url,omitempty"`
	// CanonicalURL is the normalized public Telegram URL.
	CanonicalURL string `json:"canonical_url,omitempty"`
	// BodyHTML is the rendered full HTML document generated from the Telegram page.
	BodyHTML string `json:"body_html,omitempty"`
	// MessageCount is the number of public messages parsed from the page.
	MessageCount int `json:"message_count,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	return map[string]any{
		"format":        o.Format,
		"content_type":  o.ContentType,
		"username":      o.Username,
		"message_id":    o.MessageID,
		"title":         o.Title,
		"source_url":    o.SourceURL,
		"canonical_url": o.CanonicalURL,
		"body_html":     o.BodyHTML,
		"message_count": o.MessageCount,
	}
}
