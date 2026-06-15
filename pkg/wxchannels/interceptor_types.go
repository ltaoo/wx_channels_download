package wxchannels

import "encoding/json"

type InterceptorMediaSpec struct {
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
type InterceptorPicture struct {
	URL string `json:"url"`
}
type InterceptorContact struct {
	Id        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type MediaProfile struct {
	Type     string             `json:"type"` // media | picture | live
	Id       string             `json:"id"`
	NonceId  string             `json:"nonce_id"`
	Title    string             `json:"title"`
	URL      string             `json:"url"`
	Key      string             `json:"key"`
	CoverURL string             `json:"cover_url"`
	Contact  InterceptorContact `json:"contact"`
	Spec     json.RawMessage    `json:"spec"`
	Files    json.RawMessage    `json:"files"`
	Pageurl  string             `json:"pageurl"`
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

type ChannelMediaSpec = InterceptorMediaSpec
type ChannelPicture = InterceptorPicture
type ChannelContact = InterceptorContact
type ChannelMediaProfile = MediaProfile
