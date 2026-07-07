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
	// Channel is the display name for the video's publisher.
	Channel string `json:"channel,omitempty"`
	// ChannelID is the stable YouTube channel identifier.
	ChannelID string `json:"channel_id,omitempty"`
	// ChannelURL is the publisher's YouTube channel/profile URL.
	ChannelURL string `json:"channel_url,omitempty"`
	// ChannelAvatarURL is the publisher's avatar image URL.
	ChannelAvatarURL string `json:"channel_avatar_url,omitempty"`
	// FormatCount is the number of available media formats reported by YouTube.
	FormatCount int `json:"format_count,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	return map[string]any{
		"content_type":        o.ContentType,
		"title":               o.Title,
		"source_url":          o.SourceURL,
		"video_id":            o.VideoID,
		"duration":            o.Duration,
		"channel":             o.Channel,
		"channel_id":          o.ChannelID,
		"author_homepage_url": o.ChannelURL,
		"channel_url":         o.ChannelURL,
		"channel_avatar_url":  o.ChannelAvatarURL,
		"author_avatar_url":   o.ChannelAvatarURL,
		"format_count":        o.FormatCount,
	}
}
