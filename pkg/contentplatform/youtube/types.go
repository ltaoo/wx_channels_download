package youtube

type ProbeOutput struct {
	ContentType string `json:"content_type,omitempty"`
	Title       string `json:"title,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
	VideoID     string `json:"video_id,omitempty"`
	Duration    int64  `json:"duration,omitempty"`
	FormatCount int    `json:"format_count,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	return map[string]any{
		"content_type": o.ContentType,
		"title":        o.Title,
		"source_url":   o.SourceURL,
		"video_id":     o.VideoID,
		"duration":     o.Duration,
		"format_count": o.FormatCount,
	}
}
