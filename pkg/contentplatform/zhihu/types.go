package zhihu

// ProbeOutput is the Zhihu text output extracted during Probe and exposed
// through the shared content envelope.
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
