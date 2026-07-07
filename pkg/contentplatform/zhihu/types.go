package zhihu

import (
	contentdownload "wx_channel/pkg/contentplatform/download"
	zhihupkg "wx_channel/pkg/scraper/zhihu"
)

const (
	ContentTypeAnswer   = "answer"
	ContentTypeQuestion = "question"
	ContentTypeArticle  = "article"

	OutputFormatHTML = "html"
	OutputFormatJSON = "json"
)

// AnswerContentEnvelope is the complete shared content envelope returned for a
// Zhihu answer. Its Data payload contains the parent question and the answer;
// the answer author is stored at Data.Answer.Author and projected into Summary.
type AnswerContentEnvelope struct {
	Summary  contentdownload.ContentSummary `json:"summary,omitempty"`
	Data     AnswerContent                  `json:"data,omitempty"`
	Metadata AnswerMetadata                 `json:"metadata,omitempty"`
	Output   AnswerOutput                   `json:"output,omitempty"`
}

func NewAnswerContentEnvelope(summary contentdownload.ContentSummary, data AnswerContent, metadata AnswerMetadata, output AnswerOutput) *AnswerContentEnvelope {
	return &AnswerContentEnvelope{
		Summary:  summary,
		Data:     data,
		Metadata: metadata,
		Output:   output,
	}
}

func (c *AnswerContentEnvelope) ContentSummary() contentdownload.ContentSummary {
	if c == nil {
		return contentdownload.ContentSummary{}
	}
	return c.Summary
}

func (c *AnswerContentEnvelope) ContentData() any {
	if c == nil {
		return nil
	}
	return c.Data
}

func (c *AnswerContentEnvelope) ContentMetadata() map[string]any {
	if c == nil {
		return nil
	}
	return c.Metadata.Map()
}

func (c *AnswerContentEnvelope) ContentOutput() map[string]any {
	if c == nil {
		return nil
	}
	return c.Output.Map()
}

// AnswerContent is the JSON content.data payload for a Zhihu answer.
type AnswerContent struct {
	Question zhihupkg.Question `json:"question"`
	Answer   zhihupkg.Answer   `json:"answer"`
}

// AnswerMetadata is the JSON content.metadata payload for a Zhihu answer.
type AnswerMetadata struct {
	QuestionID        string `json:"question_id"`
	AnswerID          string `json:"answer_id"`
	QuestionTitle     string `json:"question_title"`
	AuthorID          string `json:"author_id"`
	AuthorURLToken    string `json:"author_url_token"`
	AuthorAvatarURL   string `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string `json:"author_homepage_url,omitempty"`
	CreatedTime       int64  `json:"created_time"`
	UpdatedTime       int64  `json:"updated_time"`
	CommentCount      int    `json:"comment_count"`
	SourceURL         string `json:"source_url"`
}

func (m AnswerMetadata) Map() map[string]any {
	return map[string]any{
		"question_id":         m.QuestionID,
		"answer_id":           m.AnswerID,
		"question_title":      m.QuestionTitle,
		"author_id":           m.AuthorID,
		"author_url_token":    m.AuthorURLToken,
		"author_avatar_url":   m.AuthorAvatarURL,
		"author_homepage_url": m.AuthorHomepageURL,
		"created_time":        m.CreatedTime,
		"updated_time":        m.UpdatedTime,
		"comment_count":       m.CommentCount,
		"source_url":          m.SourceURL,
	}
}

// AnswerOutput is the JSON content.output payload for a Zhihu answer.
type AnswerOutput struct {
	Format            string `json:"format"`
	ContentType       string `json:"content_type"`
	QuestionID        string `json:"question_id"`
	AnswerID          string `json:"answer_id"`
	Title             string `json:"title"`
	SourceURL         string `json:"source_url"`
	CanonicalURL      string `json:"canonical_url"`
	AuthorAvatarURL   string `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string `json:"author_homepage_url,omitempty"`
	BodyHTML          string `json:"body_html"`
	QuestionHTML      string `json:"question_html,omitempty"`
}

func (o AnswerOutput) Map() map[string]any {
	out := map[string]any{
		"format":              o.Format,
		"content_type":        o.ContentType,
		"question_id":         o.QuestionID,
		"answer_id":           o.AnswerID,
		"title":               o.Title,
		"source_url":          o.SourceURL,
		"canonical_url":       o.CanonicalURL,
		"author_avatar_url":   o.AuthorAvatarURL,
		"author_homepage_url": o.AuthorHomepageURL,
		"body_html":           o.BodyHTML,
	}
	if o.QuestionHTML != "" {
		out["question_html"] = o.QuestionHTML
	}
	return out
}

// QuestionContentEnvelope is the complete shared content envelope returned for
// a Zhihu question.
type QuestionContentEnvelope struct {
	Summary  contentdownload.ContentSummary `json:"summary,omitempty"`
	Data     QuestionContent                `json:"data,omitempty"`
	Metadata QuestionMetadata               `json:"metadata,omitempty"`
	Output   QuestionOutput                 `json:"output,omitempty"`
}

func NewQuestionContentEnvelope(summary contentdownload.ContentSummary, data QuestionContent, metadata QuestionMetadata, output QuestionOutput) *QuestionContentEnvelope {
	return &QuestionContentEnvelope{
		Summary:  summary,
		Data:     data,
		Metadata: metadata,
		Output:   output,
	}
}

func (c *QuestionContentEnvelope) ContentSummary() contentdownload.ContentSummary {
	if c == nil {
		return contentdownload.ContentSummary{}
	}
	return c.Summary
}

func (c *QuestionContentEnvelope) ContentData() any {
	if c == nil {
		return nil
	}
	return c.Data
}

func (c *QuestionContentEnvelope) ContentMetadata() map[string]any {
	if c == nil {
		return nil
	}
	return c.Metadata.Map()
}

func (c *QuestionContentEnvelope) ContentOutput() map[string]any {
	if c == nil {
		return nil
	}
	return c.Output.Map()
}

// QuestionContent is the JSON content.data payload for a Zhihu question.
type QuestionContent struct {
	Question zhihupkg.Question `json:"question"`
}

// QuestionMetadata is the JSON content.metadata payload for a Zhihu question.
type QuestionMetadata struct {
	QuestionID        string `json:"question_id"`
	AuthorID          string `json:"author_id"`
	AuthorURLToken    string `json:"author_url_token"`
	AuthorAvatarURL   string `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string `json:"author_homepage_url,omitempty"`
	SourceURL         string `json:"source_url"`
}

func (m QuestionMetadata) Map() map[string]any {
	return map[string]any{
		"question_id":         m.QuestionID,
		"author_id":           m.AuthorID,
		"author_url_token":    m.AuthorURLToken,
		"author_avatar_url":   m.AuthorAvatarURL,
		"author_homepage_url": m.AuthorHomepageURL,
		"source_url":          m.SourceURL,
	}
}

// QuestionOutput is the JSON content.output payload for a Zhihu question.
type QuestionOutput struct {
	Format            string `json:"format"`
	ContentType       string `json:"content_type"`
	QuestionID        string `json:"question_id"`
	Title             string `json:"title"`
	SourceURL         string `json:"source_url"`
	CanonicalURL      string `json:"canonical_url"`
	AuthorAvatarURL   string `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string `json:"author_homepage_url,omitempty"`
	BodyHTML          string `json:"body_html"`
}

func (o QuestionOutput) Map() map[string]any {
	return map[string]any{
		"format":              o.Format,
		"content_type":        o.ContentType,
		"question_id":         o.QuestionID,
		"title":               o.Title,
		"source_url":          o.SourceURL,
		"canonical_url":       o.CanonicalURL,
		"author_avatar_url":   o.AuthorAvatarURL,
		"author_homepage_url": o.AuthorHomepageURL,
		"body_html":           o.BodyHTML,
	}
}

// ArticleContentEnvelope is the complete shared content envelope returned for
// a Zhihu zhuanlan article.
type ArticleContentEnvelope struct {
	Summary  contentdownload.ContentSummary `json:"summary,omitempty"`
	Data     ArticleContent                 `json:"data,omitempty"`
	Metadata ArticleMetadata                `json:"metadata,omitempty"`
	Output   ArticleOutput                  `json:"output,omitempty"`
}

func NewArticleContentEnvelope(summary contentdownload.ContentSummary, data ArticleContent, metadata ArticleMetadata, output ArticleOutput) *ArticleContentEnvelope {
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

// ArticleContent is the JSON content.data payload for a Zhihu zhuanlan article.
type ArticleContent struct {
	Article zhihupkg.Article `json:"article"`
}

// ArticleMetadata is the JSON content.metadata payload for a Zhihu zhuanlan article.
type ArticleMetadata struct {
	ArticleID         string `json:"article_id"`
	AuthorID          string `json:"author_id"`
	AuthorURLToken    string `json:"author_url_token"`
	AuthorAvatarURL   string `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string `json:"author_homepage_url,omitempty"`
	CreatedTime       int64  `json:"created_time"`
	UpdatedTime       int64  `json:"updated_time"`
	SourceURL         string `json:"source_url"`
}

func (m ArticleMetadata) Map() map[string]any {
	return map[string]any{
		"article_id":          m.ArticleID,
		"author_id":           m.AuthorID,
		"author_url_token":    m.AuthorURLToken,
		"author_avatar_url":   m.AuthorAvatarURL,
		"author_homepage_url": m.AuthorHomepageURL,
		"created_time":        m.CreatedTime,
		"updated_time":        m.UpdatedTime,
		"source_url":          m.SourceURL,
	}
}

// ArticleOutput is the JSON content.output payload for a Zhihu zhuanlan article.
type ArticleOutput struct {
	Format            string `json:"format"`
	ContentType       string `json:"content_type"`
	ArticleID         string `json:"article_id"`
	Title             string `json:"title"`
	SourceURL         string `json:"source_url"`
	CanonicalURL      string `json:"canonical_url"`
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
		"canonical_url":       o.CanonicalURL,
		"author_avatar_url":   o.AuthorAvatarURL,
		"author_homepage_url": o.AuthorHomepageURL,
		"body_html":           o.BodyHTML,
	}
}

// ProbeOutput is a backwards-compatible union of the Zhihu output payloads.
// New code should prefer AnswerOutput, QuestionOutput, or ArticleOutput.
type ProbeOutput struct {
	// Format is the rendered output format, usually html for text content.
	Format string `json:"format,omitempty"`
	// ContentType classifies the probed Zhihu content, such as answer, question, or article.
	ContentType string `json:"content_type,omitempty"`
	// QuestionID is the Zhihu question identifier when the content belongs to a question.
	QuestionID string `json:"question_id,omitempty"`
	// AnswerID is the Zhihu answer identifier for answer pages.
	AnswerID string `json:"answer_id,omitempty"`
	// ArticleID is the Zhihu article identifier for zhuanlan articles.
	ArticleID string `json:"article_id,omitempty"`
	// Title is the question or article title.
	Title string `json:"title,omitempty"`
	// SourceURL is the original URL submitted by the user.
	SourceURL string `json:"source_url,omitempty"`
	// CanonicalURL is the normalized Zhihu content URL.
	CanonicalURL string `json:"canonical_url,omitempty"`
	// BodyHTML is the sanitized answer, question, or article HTML body.
	BodyHTML string `json:"body_html,omitempty"`
	// QuestionHTML is the sanitized question detail HTML attached to an answer.
	QuestionHTML string `json:"question_html,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	out := map[string]any{
		"format":        o.Format,
		"content_type":  o.ContentType,
		"title":         o.Title,
		"source_url":    o.SourceURL,
		"canonical_url": o.CanonicalURL,
		"body_html":     o.BodyHTML,
	}
	if o.QuestionID != "" {
		out["question_id"] = o.QuestionID
	}
	if o.AnswerID != "" {
		out["answer_id"] = o.AnswerID
	}
	if o.ArticleID != "" {
		out["article_id"] = o.ArticleID
	}
	if o.QuestionHTML != "" {
		out["question_html"] = o.QuestionHTML
	}
	return out
}
