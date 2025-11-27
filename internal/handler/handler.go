package handler

type ChannelInjectedFiles struct {
	JSFileSaver []byte
	JSZip       []byte
	JSPageSpy   []byte
	JSDebug     []byte
	JSUtils     []byte
	JSError     []byte
	JSMain      []byte
	JSLiveMain  []byte
}

type ChannelMediaSpec struct {
	FileFormat       string  `json:"file_format"`
	FirstLoadBytes   int     `json:"first_load_bytes"`
	BitRate          int     `json:"bit_rate"`
	CodingFormat     string  `json:"coding_format"`
	DynamicRangeType int     `json:"dynamic_range_type"`
	Vfps             int     `json:"vfps"`
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
type ChannelContact struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	HeadURL  string `json:"head_url"`
}
type ChannelMediaProfile struct {
	Title    string             `json:"title"`
	CoverURL string             `json:"cover_url"`
	URL      string             `json:"url"`
	Size     int                `json:"size"`
	Key      string             `json:"key"`
	NonceId  string             `json:"nonce_id"`
	Nickname string             `json:"nickname"`
	Type     string             `json:"type"`
	Contact  ChannelContact     `json:"contact"`
	Spec     []ChannelMediaSpec `json:"spec"`
}
type FrontendTip struct {
	End          int     `json:"end"`
	Replace      int     `json:"replace"`
	IgnorePrefix int     `json:"ignore_prefix"`
	Prefix       *string `json:"prefix"`
	Msg          string  `json:"msg"`
}
