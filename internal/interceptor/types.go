package interceptor

type ChannelInjectedFiles struct {
	JSFileSaver      []byte
	JSZip            []byte
	JSRecorder       []byte
	JSPageSpy        []byte
	JSFloatingUICore []byte
	JSFloatingUIDOM  []byte
	JSWeui           []byte
	JSMitt           []byte
	JSDebug          []byte
	JSEventBus       []byte
	JSComponents     []byte
	JSUtils          []byte
	JSError          []byte
	JSMain           []byte
	JSLiveMain       []byte
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
type ChannelPicture struct {
	URL string `json:"url"`
}
type ChannelContact struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	HeadURL  string `json:"head_url"`
}
type ChannelMediaProfile struct {
	Type     string             `json:"type"` // media | picture | live
	Id       string             `json:"id"`
	NonceId  string             `json:"nonce_id"`
	Title    string             `json:"title"`
	URL      string             `json:"url"`
	Key      string             `json:"key"`
	CoverURL string             `json:"cover_url"`
	Contact  ChannelContact     `json:"contact"`
	Spec     []ChannelMediaSpec `json:"spec"`
	Files    []ChannelPicture   `json:"files"`
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
