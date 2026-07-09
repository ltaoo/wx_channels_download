package qidian

import qidianpkg "wx_channel/pkg/scraper/qidian"

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
	// Description is the novel synopsis parsed from the Qidian intro block.
	Description string `json:"description,omitempty"`
	// Author is the novel author display name.
	Author string `json:"author,omitempty"`
	// AuthorAvatarURL is the novel author's avatar image URL.
	AuthorAvatarURL string `json:"author_avatar_url,omitempty"`
	// Category is the novel category shown by Qidian.
	Category string `json:"category,omitempty"`
	// Status is the serialization status, such as 连载.
	Status string `json:"status,omitempty"`
	// ChapterCount is the total chapter count shown by Qidian.
	ChapterCount int `json:"chapter_count,omitempty"`
	// WordCount is the numeric word count when it can be parsed.
	WordCount int64 `json:"word_count,omitempty"`
	// DisplayWordCount is the Qidian-formatted word count, such as 588.63万.
	DisplayWordCount string `json:"display_word_count,omitempty"`
	// LatestChapter is the newest chapter parsed from the page.
	LatestChapter Chapter `json:"latest_chapter,omitempty"`
	// Volumes preserves Qidian's volume -> chapter catalog structure.
	Volumes []BookVolume `json:"volumes,omitempty"`
	// SourceURL is the original URL submitted by the user.
	SourceURL string `json:"source_url,omitempty"`
	// CanonicalURL is the normalized Qidian content URL.
	CanonicalURL string `json:"canonical_url,omitempty"`
	// BodyHTML is the sanitized HTML body generated from the page content.
	BodyHTML string `json:"body_html,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	return map[string]any{
		"format":             o.Format,
		"content_type":       o.ContentType,
		"title":              o.Title,
		"description":        o.Description,
		"author":             o.Author,
		"author_avatar_url":  o.AuthorAvatarURL,
		"category":           o.Category,
		"status":             o.Status,
		"chapter_count":      o.ChapterCount,
		"word_count":         o.WordCount,
		"display_word_count": o.DisplayWordCount,
		"latest_chapter":     o.LatestChapter,
		"volumes":            o.Volumes,
		"source_url":         o.SourceURL,
		"canonical_url":      o.CanonicalURL,
		"body_html":          o.BodyHTML,
	}
}
