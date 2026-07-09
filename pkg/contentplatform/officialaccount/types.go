package officialaccount

import (
	contentdownload "wx_channel/pkg/contentplatform/download"
	officialaccountpkg "wx_channel/pkg/scraper/officialaccount"
)

const (
	ContentTypeArticle = "article"
	OutputFormatHTML   = "html"
)

// ArticleContentEnvelope is the complete shared content envelope returned for
// a WeChat official account article.
type ArticleContentEnvelope struct {
	Summary  contentdownload.ContentSummary            `json:"summary,omitempty"`
	Data     *officialaccountpkg.WechatOfficialArticle `json:"data,omitempty"`
	Metadata ArticleMetadata                           `json:"metadata,omitempty"`
	Output   ArticleOutput                             `json:"output,omitempty"`
}

func NewArticleContentEnvelope(summary contentdownload.ContentSummary, data *officialaccountpkg.WechatOfficialArticle, metadata ArticleMetadata, output ArticleOutput) *ArticleContentEnvelope {
	return &ArticleContentEnvelope{
		Summary:  summary,
		Data:     data,
		Metadata: metadata,
		Output:   output,
	}
}

func (c *ArticleContentEnvelope) ContentSummary() contentdownload.ContentSummary {
	if c == nil {
		return contentdownload.ContentSummary{}
	}
	return c.Summary
}

func (c *ArticleContentEnvelope) ContentData() any {
	if c == nil {
		return nil
	}
	return c.Data
}

func (c *ArticleContentEnvelope) ContentMetadata() map[string]any {
	if c == nil {
		return nil
	}
	return c.Metadata.Map()
}

func (c *ArticleContentEnvelope) ContentOutput() map[string]any {
	if c == nil {
		return nil
	}
	return c.Output.Map()
}

// ArticleMetadata is the JSON content.metadata payload for a WeChat official
// account article.
type ArticleMetadata struct {
	ArticleID         string `json:"article_id"`
	AuthorID          string `json:"author_id"`
	AuthorAvatarURL   string `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string `json:"author_homepage_url,omitempty"`
}

func (m ArticleMetadata) Map() map[string]any {
	return map[string]any{
		"article_id":          m.ArticleID,
		"author_id":           m.AuthorID,
		"author_avatar_url":   m.AuthorAvatarURL,
		"author_homepage_url": m.AuthorHomepageURL,
	}
}

// ArticleOutput is the JSON content.output payload for a WeChat official
// account article.
type ArticleOutput struct {
	Format            string `json:"format"`
	ContentType       string `json:"content_type"`
	ArticleID         string `json:"article_id"`
	Title             string `json:"title"`
	SourceURL         string `json:"source_url"`
	AuthorAvatarURL   string `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string `json:"author_homepage_url,omitempty"`
	BodyHTML          string `json:"body_html"`
}

func (o ArticleOutput) Map() map[string]any {
	return map[string]any{
		"format":              o.Format,
		"content_type":        o.ContentType,
		"article_id":          o.ArticleID,
		"title":               o.Title,
		"source_url":          o.SourceURL,
		"author_avatar_url":   o.AuthorAvatarURL,
		"author_homepage_url": o.AuthorHomepageURL,
		"body_html":           o.BodyHTML,
	}
}

// ProbeOutput is the WeChat official account article output extracted during
// Probe and exposed through the shared content envelope. New code should
// prefer ArticleOutput.
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
