package bilibili

import "encoding/json"

const PlatformID = "bilibili"

type VideoKey struct {
	BVID string
	AID  int64
	Page int
}

type VideoInfo struct {
	BVID        string          `json:"bvid,omitempty"`
	AID         int64           `json:"aid,omitempty"`
	CID         int64           `json:"cid,omitempty"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Pic         string          `json:"pic,omitempty"`
	Owner       Owner           `json:"owner,omitempty"`
	Pages       []Page          `json:"pages,omitempty"`
	Page        Page            `json:"page,omitempty"`
	Duration    int64           `json:"duration,omitempty"`
	WebpageURL  string          `json:"webpage_url,omitempty"`
	PlayURL     PlayURLData     `json:"play_url,omitempty"`
	RawView     json.RawMessage `json:"raw_view,omitempty"`
	RawPlayURL  json.RawMessage `json:"raw_play_url,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
}

type Owner struct {
	MID  int64  `json:"mid,omitempty"`
	Name string `json:"name,omitempty"`
	Face string `json:"face,omitempty"`
}

type Page struct {
	CID      int64  `json:"cid,omitempty"`
	Page     int    `json:"page,omitempty"`
	From     string `json:"from,omitempty"`
	Part     string `json:"part,omitempty"`
	Duration int64  `json:"duration,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type viewResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	TTL     int             `json:"ttl"`
	Data    viewData        `json:"data"`
	Raw     json.RawMessage `json:"-"`
}

type viewData struct {
	BVID        string `json:"bvid"`
	AID         int64  `json:"aid"`
	CID         int64  `json:"cid"`
	Title       string `json:"title"`
	Description string `json:"desc"`
	Pic         string `json:"pic"`
	Owner       Owner  `json:"owner"`
	Pages       []Page `json:"pages"`
	Duration    int64  `json:"duration"`
}

type playURLResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    PlayURLData     `json:"data"`
	Result  PlayURLData     `json:"result"`
	Raw     json.RawMessage `json:"-"`
}

type PlayURLData struct {
	Quality           int             `json:"quality,omitempty"`
	Format            string          `json:"format,omitempty"`
	Timelength        int64           `json:"timelength,omitempty"`
	AcceptQuality     []int           `json:"accept_quality,omitempty"`
	AcceptDescription []string        `json:"accept_description,omitempty"`
	SupportFormats    []SupportFormat `json:"support_formats,omitempty"`
	DASH              *DASH           `json:"dash,omitempty"`
	DURL              []DURL          `json:"durl,omitempty"`
}

type SupportFormat struct {
	Quality        int    `json:"quality,omitempty"`
	Format         string `json:"format,omitempty"`
	NewDescription string `json:"new_description,omitempty"`
	DisplayDesc    string `json:"display_desc,omitempty"`
	Superscript    string `json:"superscript,omitempty"`
}

type DASH struct {
	Duration int64        `json:"duration,omitempty"`
	Video    []DASHStream `json:"video,omitempty"`
	Audio    []DASHStream `json:"audio,omitempty"`
	Dolby    *Dolby       `json:"dolby,omitempty"`
	Flac     *Flac        `json:"flac,omitempty"`
}

type Dolby struct {
	Type  int          `json:"type,omitempty"`
	Audio []DASHStream `json:"audio,omitempty"`
}

type Flac struct {
	Display bool        `json:"display,omitempty"`
	Audio   *DASHStream `json:"audio,omitempty"`
}

type DASHStream struct {
	ID           int      `json:"id,omitempty"`
	BaseURL      string   `json:"baseUrl,omitempty"`
	BaseURLAlt   string   `json:"base_url,omitempty"`
	BackupURL    []string `json:"backupUrl,omitempty"`
	BackupURLAlt []string `json:"backup_url,omitempty"`
	Bandwidth    int64    `json:"bandwidth,omitempty"`
	MimeType     string   `json:"mimeType,omitempty"`
	MimeTypeAlt  string   `json:"mime_type,omitempty"`
	Codecs       string   `json:"codecs,omitempty"`
	Width        int      `json:"width,omitempty"`
	Height       int      `json:"height,omitempty"`
	FrameRate    string   `json:"frameRate,omitempty"`
	FrameRateAlt string   `json:"frame_rate,omitempty"`
}

func (s DASHStream) URL() string {
	if s.BaseURL != "" {
		return s.BaseURL
	}
	return s.BaseURLAlt
}

func (s DASHStream) Mime() string {
	if s.MimeType != "" {
		return s.MimeType
	}
	return s.MimeTypeAlt
}

type DURL struct {
	Order        int      `json:"order,omitempty"`
	Length       int64    `json:"length,omitempty"`
	Size         int64    `json:"size,omitempty"`
	URL          string   `json:"url,omitempty"`
	BackupURL    []string `json:"backup_url,omitempty"`
	BackupURLAlt []string `json:"backupUrl,omitempty"`
}

func (d DURL) FirstURL() string {
	if d.URL != "" {
		return d.URL
	}
	if len(d.BackupURL) > 0 {
		return d.BackupURL[0]
	}
	if len(d.BackupURLAlt) > 0 {
		return d.BackupURLAlt[0]
	}
	return ""
}
