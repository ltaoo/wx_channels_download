package shuba69

import shubapkg "wx_channel/pkg/scraper/69shuba"

type Novel = shubapkg.Novel
type NovelFetchResult = shubapkg.NovelFetchResult
type Chapter = shubapkg.Chapter
type ChapterContent = shubapkg.ChapterContent
type ChapterFetchResult = shubapkg.ChapterFetchResult
type NovelArchiveResult = shubapkg.NovelArchiveResult
type NovelArchiveOptions = shubapkg.NovelArchiveOptions
type Client = shubapkg.Client
type HTTPClient = shubapkg.HTTPClient
type HTMLFetcher = shubapkg.HTMLFetcher
type ClawreqFetcher = shubapkg.ClawreqFetcher
type CDPFetcher = shubapkg.CDPFetcher
type SandboxCDPFetcher = shubapkg.SandboxCDPFetcher

// ProbeOutput is the 69shuba text output extracted during Probe and exposed
// through the shared content envelope.
type ProbeOutput struct {
	// Format is the rendered output format, usually html for text content.
	Format string `json:"format,omitempty"`
	// ContentType classifies the probed 69shuba content, such as novel or chapter.
	ContentType string `json:"content_type,omitempty"`
	// Title is the novel or chapter title.
	Title string `json:"title,omitempty"`
	// SourceURL is the original URL submitted by the user.
	SourceURL string `json:"source_url,omitempty"`
	// CanonicalURL is the normalized 69shuba content URL.
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
