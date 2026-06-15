package qidian

import qidianpkg "wx_channel/pkg/qidian"

type BookVolume = qidianpkg.BookVolume
type Chapter = qidianpkg.Chapter
type Author = qidianpkg.Author
type BookProfile = qidianpkg.BookProfile
type Client = qidianpkg.Client

// ProbeOutput is the Qidian novel output extracted during Probe and exposed
// through the shared content envelope.
type ProbeOutput struct {
	// Format is the rendered output format, usually html for text content.
	Format string `json:"format,omitempty"`
	// ContentType classifies the probed Qidian content, such as novel.
	ContentType string `json:"content_type,omitempty"`
	// Title is the novel title.
	Title string `json:"title,omitempty"`
	// SourceURL is the original URL submitted by the user.
	SourceURL string `json:"source_url,omitempty"`
	// CanonicalURL is the normalized Qidian content URL.
	CanonicalURL string `json:"canonical_url,omitempty"`
	// BodyHTML is the sanitized HTML body generated from the page content.
	BodyHTML string `json:"body_html,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	return map[string]any{
		"format":        o.Format,
		"content_type":  o.ContentType,
		"title":         o.Title,
		"source_url":    o.SourceURL,
		"canonical_url": o.CanonicalURL,
		"body_html":     o.BodyHTML,
	}
}
