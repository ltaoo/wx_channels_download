package soundgasm

import soundgasmpkg "wx_channel/pkg/scraper/soundgasm"

const (
	PlatformID = soundgasmpkg.PlatformID

	audioVariantID = "audio"
	htmlVariantID  = "html"
	jsonVariantID  = "json"
)

// ProbeOutput is the Soundgasm audio metadata exposed through the shared
// content envelope.
type ProbeOutput struct {
	Format       string   `json:"format,omitempty"`
	ContentType  string   `json:"content_type,omitempty"`
	AudioID      string   `json:"audio_id,omitempty"`
	Title        string   `json:"title,omitempty"`
	SourceURL    string   `json:"source_url,omitempty"`
	CanonicalURL string   `json:"canonical_url,omitempty"`
	AudioURL     string   `json:"audio_url,omitempty"`
	AudioType    string   `json:"audio_type,omitempty"`
	BodyHTML     string   `json:"body_html,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

func (o ProbeOutput) Map() map[string]any {
	return map[string]any{
		"format":        o.Format,
		"content_type":  o.ContentType,
		"audio_id":      o.AudioID,
		"title":         o.Title,
		"source_url":    o.SourceURL,
		"canonical_url": o.CanonicalURL,
		"audio_url":     o.AudioURL,
		"audio_type":    o.AudioType,
		"body_html":     o.BodyHTML,
		"tags":          o.Tags,
	}
}
