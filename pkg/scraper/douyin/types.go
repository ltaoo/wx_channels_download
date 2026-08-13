package douyin

// VideoInfo holds Douyin video information.
type VideoInfo struct {
	URL      string // video playback URL
	Title    string // video title
	VideoID  string // video ID
	CoverURL string // cover image URL
	Source   string // source identifier (mobile/web)
}

// DouyinWebVideoProfileResp is the web API response.
type DouyinWebVideoProfileResp struct {
	AwemeDetail AwemeDetail `json:"aweme_detail"`
	StatusCode  int         `json:"status_code"`
	raw_body    []byte
}

// AwemeDetail holds video detail data.
type AwemeDetail struct {
	AwemeId   string    `json:"aweme_id"`
	Desc      string    `json:"desc"`
	Author    Author    `json:"author"`
	Video     Video     `json:"video"`
	Music     Music     `json:"music"`
	Duration  int       `json:"duration"`
	ShareInfo ShareInfo `json:"share_info"`
}

// Author holds author information.
type Author struct {
	Nickname string `json:"nickname"`
	Uid      string `json:"uid"`
	SecUid   string `json:"sec_uid"`
}

// Video holds video information.
type Video struct {
	PlayAddr     PlayAddr     `json:"play_addr"`
	PlayAddrH264 PlayAddrH264 `json:"play_addr_h264"`
	PlayAddr265  PlayAddr265  `json:"play_addr_265"`
	Cover        Cover        `json:"cover"`
	OriginCover  OriginCover  `json:"origin_cover"`
	Duration     int          `json:"duration"`
	Width        int          `json:"width"`
	Height       int          `json:"height"`
	BitRate      []BitRate    `json:"bit_rate"`
}

// BitRate holds bitrate information.
type BitRate struct {
	BitRate  int      `json:"bit_rate"`
	PlayAddr PlayAddr `json:"play_addr"`
	GearName string   `json:"gear_name"`
}

// PlayAddr holds playback URL information.
type PlayAddr struct {
	Uri     string   `json:"uri"`
	UrlList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

// PlayAddrH264 holds H264 playback URL information.
type PlayAddrH264 struct {
	Uri     string   `json:"uri"`
	UrlList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

// PlayAddr265 holds H265 playback URL information.
type PlayAddr265 struct {
	Uri     string   `json:"uri"`
	UrlList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

// Cover holds cover image information.
type Cover struct {
	Uri     string   `json:"uri"`
	UrlList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

// OriginCover holds original cover image information.
type OriginCover struct {
	Uri     string   `json:"uri"`
	UrlList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

// Music holds music information.
type Music struct {
	Title   string  `json:"title"`
	Author  string  `json:"author"`
	PlayUrl PlayUrl `json:"play_url"`
}

// PlayUrl holds music playback URL information.
type PlayUrl struct {
	Uri     string   `json:"uri"`
	UrlList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

// ShareInfo holds share information.
type ShareInfo struct {
	ShareUrl  string `json:"share_url"`
	ShareDesc string `json:"share_desc"`
}
