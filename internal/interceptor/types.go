package interceptor

import "encoding/json"

type ChannelMediaSpec struct {
	FileFormat       string  `json:"file_format"`
	FirstLoadBytes   int     `json:"first_load_bytes"`
	BitRate          int     `json:"bit_rate"`
	CodingFormat     string  `json:"coding_format"`
	DynamicRangeType int     `json:"dynamic_range_type"`
	Vfps             float64 `json:"vfps"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	DurationMs       int     `json:"duration_ms"`
	QualityScore     float64 `json:"quality_score"`
	VideoBitrate     int     `json:"video_bitrate"`
	AudioBitrate     int     `json:"audio_bitrate"`
	LevelOrder       int     `json:"level_order"`
	Bypass           string  `json:"bypass"`
	Is3az            int     `json:"is3az"`
}
type ChannelPicture struct {
	URL string `json:"url"`
}
type ChannelContact struct {
	Id        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type ChannelMediaProfile struct {
	Type     json.RawMessage `json:"type"` // media | picture | live
	Id       json.RawMessage `json:"id"`
	NonceId  json.RawMessage `json:"nonce_id"`
	Title    json.RawMessage `json:"title"`
	URL      json.RawMessage `json:"url"`
	Key      json.RawMessage `json:"key"`
	CoverURL json.RawMessage `json:"cover_url"`
	Contact  json.RawMessage `json:"contact"`
	Spec     json.RawMessage `json:"spec"`
	Files    json.RawMessage `json:"files"`
}

type FrontendTip struct {
	End          int     `json:"end"`
	Replace      int     `json:"replace"`
	IgnorePrefix int     `json:"ignore_prefix"`
	Prefix       *string `json:"prefix"`
	Msg          string  `json:"msg"`
}
type FrontendErrorTip struct {
	Alert int    `json:"alert"`
	Msg   string `json:"msg"`
}
