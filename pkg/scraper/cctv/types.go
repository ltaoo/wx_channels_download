// Package cctv retrieves video metadata and playable streams from CCTV video
// pages.
package cctv

const (
	// PlatformID is the stable platform identifier used by the adapter layer.
	PlatformID = "cctv"

	// VideoInfoAPIURL is CCTV's VDN video metadata endpoint.
	VideoInfoAPIURL = "https://vdn.apps.cntv.cn/api/getHttpVideoInfo.do"
	// PageInfoAPIURL is CCTV's media-page metadata endpoint.
	PageInfoAPIURL = "https://media.app.cctv.com/vapi/media/pageinfo.do"
	// DefaultVN is the VDN signature version used by CCTV's web player.
	DefaultVN = "2049"
	// DefaultUID is used when no browser-specific CCTV UID is available.
	DefaultUID = "00000000000000000000000000000000"
	// DefaultUserAgent matches a current desktop browser accepted by CCTV.
	DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"
)

// PageInfo contains metadata extracted directly from a CCTV video page.
type PageInfo struct {
	GUID        string `json:"guid"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Keywords    string `json:"keywords"`
	ContentID   string `json:"content_id"`
	CMSTID      string `json:"cmstid"`
}

// FetchResult combines page metadata with the VDN API response.
type FetchResult struct {
	PageURL         string           `json:"page_url"`
	PageTitle       string           `json:"page_title"`
	PageDescription string           `json:"page_description"`
	PageKeywords    string           `json:"page_keywords"`
	PageContentID   string           `json:"page_content_id"`
	CMSTID          string           `json:"cmstid"`
	PageInfoURL     string           `json:"page_info_url"`
	PageInfo        PageInfoResponse `json:"page_info"`
	PID             string           `json:"pid"`
	APIURL          string           `json:"api_url"`
	Data            VideoInfo        `json:"data"`
}

// PageInfoResponse is the JSONP payload returned by CCTV's media-page API.
type PageInfoResponse struct {
	Code   int           `json:"code"`
	Msg    string        `json:"msg"`
	Status string        `json:"status"`
	Data   MediaPageInfo `json:"data"`
}

// MediaPageInfo describes the account and media page that published a CCTV
// video.
type MediaPageInfo struct {
	BeSubscribed       int64  `json:"besubscribed"`
	BackgroundImage    string `json:"bgImg"`
	Brief              string `json:"biref"`
	CategoryID         string `json:"categoryId"`
	LogoImage          string `json:"logoImg"`
	MediaID            string `json:"mediaId"`
	MediaName          string `json:"mediaName"`
	MediaType          int    `json:"mediaType"`
	PageQRCodeURL      string `json:"pageQrimgUrl"`
	QRCodeURL          string `json:"qrImgUrl"`
	TVChannelID        string `json:"tvChId"`
	TVChannelName      string `json:"tvChName"`
	Verified           int    `json:"verified"`
	VerifiedDesc       string `json:"verifiedDesc"`
	VideoTotal         int64  `json:"vtotal"`
	WebURL             string `json:"webUrl"`
	WebBackgroundImage string `json:"webbgImg"`
}

// VideoInfo is the useful subset of the CCTV VDN video metadata response.
type VideoInfo struct {
	Ack                string   `json:"ack"`
	Status             string   `json:"status"`
	IsPreview          string   `json:"is_preview"`
	Tag                string   `json:"tag"`
	Title              string   `json:"title"`
	PlayChannel        string   `json:"play_channel"`
	Produce            string   `json:"produce"`
	EditorName         string   `json:"editer_name"`
	ProduceID          string   `json:"produce_id"`
	Column             string   `json:"column"`
	ProgramTime        string   `json:"f_pgmtime"`
	Image              string   `json:"image"`
	CDNInfo            CDNInfo  `json:"cdn_info"`
	Video              Video    `json:"video"`
	HLSCDNInfo         CDNInfo  `json:"hls_cdn_info"`
	HLSURL             string   `json:"hls_url"`
	Manifest           Manifest `json:"manifest"`
	DefaultStream      string   `json:"default_stream"`
	Public             string   `json:"public"`
	IsProtected        string   `json:"is_protected"`
	IsInvalidCopyright string   `json:"is_invalid_copyright"`
	Play               string   `json:"play"`
}

// CDNInfo identifies the content delivery network selected by VDN.
type CDNInfo struct {
	VIP  string `json:"cdn_vip"`
	Code string `json:"cdn_code"`
	Name string `json:"cdn_name"`
}

// Video contains the progressive stream groups advertised by VDN.
type Video struct {
	TotalLength     string    `json:"totalLength"`
	Chapters        []Chapter `json:"chapters"`
	Chapters2       []Chapter `json:"chapters2"`
	Chapters3       []Chapter `json:"chapters3"`
	Chapters4       []Chapter `json:"chapters4"`
	ValidChapterNum int       `json:"validChapterNum"`
	URL             string    `json:"url"`
}

// Chapter is one progressive video segment returned by VDN.
type Chapter struct {
	Duration string `json:"duration"`
	Image    string `json:"image"`
	URL      string `json:"url"`
}

// Manifest contains the additional HLS and audio manifests returned by VDN.
type Manifest struct {
	AudioMP3    string `json:"audio_mp3"`
	HLSAudioURL string `json:"hls_audio_url"`
	HLSEncURL   string `json:"hls_enc_url"`
	HLSH5EURL   string `json:"hls_h5e_url"`
	HLSEnc2URL  string `json:"hls_enc2_url"`
}
