package zhihu

type ProbeOutput struct {
	Format       string `json:"format,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	QuestionID   string `json:"question_id,omitempty"`
	AnswerID     string `json:"answer_id,omitempty"`
	ArticleID    string `json:"article_id,omitempty"`
	Title        string `json:"title,omitempty"`
	SourceURL    string `json:"source_url,omitempty"`
	CanonicalURL string `json:"canonical_url,omitempty"`
	BodyHTML     string `json:"body_html,omitempty"`
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
