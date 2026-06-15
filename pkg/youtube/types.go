package youtube

// ProbeOutput is the YouTube video metadata extracted during Probe and exposed
// through the shared content envelope.
type ProbeOutput struct {
	// ContentType classifies the probed YouTube content, such as video.
	ContentType string `json:"content_type,omitempty"`
	// Title is the video title.
	Title string `json:"title,omitempty"`
	// SourceURL is the canonical YouTube watch page URL.
	SourceURL string `json:"source_url,omitempty"`
	// VideoID is the stable YouTube video identifier.
	VideoID string `json:"video_id,omitempty"`
	// Duration is the video duration in seconds.
	Duration int64 `json:"duration,omitempty"`
	// FormatCount is the number of available media formats reported by YouTube.
	FormatCount int `json:"format_count,omitempty"`
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
