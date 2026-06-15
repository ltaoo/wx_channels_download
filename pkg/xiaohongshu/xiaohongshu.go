package xiaohongshu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	errUnsupportedURL = errors.New("unsupported xiaohongshu url")
	shareURLRE        = regexp.MustCompile(`https?://[^\s"'<>]+`)
	noteIDRE          = regexp.MustCompile(`^[0-9A-Za-z]+$`)
	unsafeFilenameRE  = regexp.MustCompile(`[\\/:*?"<>|#\n\r]`)
	dotsRE            = regexp.MustCompile(`\.{2,}`)
)

type Client struct {
	HTTPClient *http.Client
	UserAgent  string
	Cookie     string
}

func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  DefaultUserAgent(),
	}
}

func CanParse(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "xiaohongshu.com" ||
		strings.HasSuffix(host, ".xiaohongshu.com") ||
		host == "xhslink.com" ||
		strings.HasSuffix(host, ".xhslink.com")
}

func ExtractShareURL(text string) string {
	text = strings.TrimSpace(text)
	for _, match := range shareURLRE.FindAllString(text, -1) {
		match = strings.Trim(match, " \t\r\n，。；;、.,!?！？")
		if CanParse(match) {
			return match
		}
	}
	if CanParse(text) {
		return text
	}
	return ""
}

func ParseNoteURL(rawURL string) (NoteURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return NoteURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "xiaohongshu.com" && !strings.HasSuffix(host, ".xiaohongshu.com") {
		return NoteURL{}, false
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) < 2 {
		return NoteURL{}, false
	}
	id := ""
	switch parts[0] {
	case "explore":
		id, _ = url.PathUnescape(parts[1])
	case "discovery":
		if len(parts) >= 3 && parts[1] == "item" {
			id, _ = url.PathUnescape(parts[2])
		}
	}
	id = strings.TrimSpace(id)
	if id == "" || !noteIDRE.MatchString(id) {
		return NoteURL{}, false
	}
	return NoteURL{
		NoteID:    id,
		Canonical: CanonicalNoteURL(id),
		XSecToken: parsed.Query().Get("xsec_token"),
	}, true
}

func CanonicalNoteURL(noteID string) string {
	noteID = strings.TrimSpace(noteID)
	if noteID == "" {
		return ""
	}
	return "https://www.xiaohongshu.com/explore/" + url.PathEscape(noteID)
}

func FetchNotePage(ctx context.Context, rawURL string) (*NotePage, error) {
	return NewClient().FetchNotePage(ctx, rawURL)
}

func ParseNotePage(body []byte, noteURL NoteURL) (*NotePage, error) {
	initialState, err := ParseInitialState(body)
	if err != nil {
		return nil, err
	}
	note, ok := NoteFromInitialState(initialState, noteURL.NoteID)
	if !ok {
		return nil, fmt.Errorf("missing xiaohongshu note entity")
	}
	if noteURL.NoteID == "" {
		noteURL.NoteID = note.NoteID
	}
	if noteURL.Canonical == "" {
		noteURL.Canonical = CanonicalNoteURL(noteURL.NoteID)
	}
	if noteURL.XSecToken == "" {
		noteURL.XSecToken = note.XSecToken
	}
	return &NotePage{
		URL:              noteURL,
		Source:           noteURL.Canonical,
		PageHTML:         string(body),
		InitialState:     initialState,
		InitialStateJSON: initialState.Raw,
		Note:             note,
	}, nil
}

func NoteFromInitialState(state *InitialState, noteID string) (Note, bool) {
	if state == nil || len(state.Note.NoteDetailMap) == 0 {
		return Note{}, false
	}
	noteID = strings.TrimSpace(noteID)
	if noteID != "" {
		if item, ok := state.Note.NoteDetailMap[noteID]; ok && strings.TrimSpace(item.Note.NoteID) != "" {
			return item.Note, true
		}
	}
	for _, candidate := range []string{state.Note.CurrentNoteID, state.Note.FirstNoteID} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if item, ok := state.Note.NoteDetailMap[candidate]; ok && strings.TrimSpace(item.Note.NoteID) != "" {
			return item.Note, true
		}
	}
	keys := make([]string, 0, len(state.Note.NoteDetailMap))
	for key := range state.Note.NoteDetailMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		item := state.Note.NoteDetailMap[key]
		if strings.TrimSpace(item.Note.NoteID) != "" {
			return item.Note, true
		}
	}
	return Note{}, false
}

func (c *Client) FetchNotePage(ctx context.Context, rawURL string) (*NotePage, error) {
	rawURL = ExtractShareURL(rawURL)
	if rawURL == "" {
		return nil, errUnsupportedURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	setHeaders(req, c.userAgent(), c.Cookie)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch xiaohongshu page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("fetch xiaohongshu page: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	noteURL, _ := ParseNoteURL(finalURL)
	if noteURL.NoteID == "" {
		noteURL, _ = ParseNoteURL(rawURL)
	}
	page, err := ParseNotePage(body, noteURL)
	if err != nil {
		return nil, err
	}
	page.Source = finalURL
	return page, nil
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return NewClient().HTTPClient
}

func (c *Client) userAgent() string {
	if c != nil && strings.TrimSpace(c.UserAgent) != "" {
		return c.UserAgent
	}
	return DefaultUserAgent()
}

func DefaultUserAgent() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
}

func setHeaders(req *http.Request, ua, cookie string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", SourceURL)
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", cookie)
	}
}

func (n Note) ImageURLs() []string {
	urls := make([]string, 0, len(n.ImageList))
	seen := make(map[string]bool)
	for _, image := range n.ImageList {
		rawURL := bestImageURL(image)
		if rawURL == "" {
			continue
		}
		rawURL = NormalizeAssetURL(rawURL)
		if rawURL == "" || seen[rawURL] {
			continue
		}
		seen[rawURL] = true
		urls = append(urls, rawURL)
	}
	return urls
}

func (n Note) CoverURL() string {
	if len(n.ImageList) == 0 {
		return ""
	}
	return NormalizeAssetURL(bestImageURL(n.ImageList[0]))
}

func (n Note) VideoStreams() []VideoStreamInfo {
	streams := make([]VideoStreamInfo, 0)
	streams = append(streams, n.Video.Media.Stream.H264...)
	streams = append(streams, n.Video.Media.Stream.H265...)
	streams = append(streams, n.Video.Media.Stream.H266...)
	streams = append(streams, n.Video.Media.Stream.AV1...)
	streams = append(streams, mediaV2Streams(n.Video.MediaV2)...)
	for i := range streams {
		streams[i].MasterURL = NormalizeAssetURL(streams[i].MasterURL)
		for j := range streams[i].BackupURLs {
			streams[i].BackupURLs[j] = NormalizeAssetURL(streams[i].BackupURLs[j])
		}
	}
	return streams
}

func (n Note) BestVideoStream() (VideoStreamInfo, bool) {
	streams := n.VideoStreams()
	if len(streams) == 0 {
		return VideoStreamInfo{}, false
	}
	for _, stream := range streams {
		if strings.TrimSpace(stream.MasterURL) != "" && stream.DefaultStream == 1 {
			return stream, true
		}
	}
	for _, stream := range streams {
		if strings.TrimSpace(stream.MasterURL) != "" {
			return stream, true
		}
	}
	return VideoStreamInfo{}, false
}

func NormalizeAssetURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	return rawURL
}

func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = unsafeFilenameRE.ReplaceAllString(name, "_")
	name = dotsRE.ReplaceAllString(name, ".")
	name = strings.Trim(name, ". ")
	if len([]rune(name)) > 120 {
		runes := []rune(name)
		name = string(runes[:120])
	}
	return name
}

func bestImageURL(image Image) string {
	for _, scene := range []string{"WB_DFT", "WB_PRV"} {
		for _, info := range image.InfoList {
			if strings.EqualFold(info.ImageScene, scene) && strings.TrimSpace(info.URL) != "" {
				return info.URL
			}
		}
	}
	for _, value := range []string{image.URLDefault, image.URL, image.URLPre} {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	for _, info := range image.InfoList {
		if strings.TrimSpace(info.URL) != "" {
			return info.URL
		}
	}
	return ""
}

type mediaV2Payload struct {
	Stream struct {
		H264 []mediaV2StreamInfo `json:"h264"`
		H265 []mediaV2StreamInfo `json:"h265"`
		H266 []mediaV2StreamInfo `json:"h266"`
		AV1  []mediaV2StreamInfo `json:"av1"`
	} `json:"stream"`
}

type mediaV2StreamInfo struct {
	StreamType    int      `json:"stream_type"`
	StreamDesc    string   `json:"stream_desc"`
	DefaultStream int      `json:"default_stream"`
	Format        string   `json:"format"`
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	Duration      int64    `json:"duration"`
	Size          int64    `json:"size"`
	AvgBitrate    int      `json:"avg_bitrate"`
	FPS           int      `json:"fps"`
	VideoCodec    string   `json:"video_codec"`
	VideoBitrate  int      `json:"video_bitrate"`
	VideoDuration int64    `json:"video_duration"`
	AudioCodec    string   `json:"audio_codec"`
	AudioBitrate  int      `json:"audio_bitrate"`
	AudioDuration int64    `json:"audio_duration"`
	Rotate        int      `json:"rotate"`
	MasterURL     string   `json:"master_url"`
	BackupURLs    []string `json:"backup_urls"`
	QualityType   string   `json:"quality_type"`
}

func mediaV2Streams(raw string) []VideoStreamInfo {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var payload mediaV2Payload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	var out []VideoStreamInfo
	for _, items := range [][]mediaV2StreamInfo{payload.Stream.H264, payload.Stream.H265, payload.Stream.H266, payload.Stream.AV1} {
		for _, item := range items {
			out = append(out, item.toVideoStreamInfo())
		}
	}
	return out
}

func (s mediaV2StreamInfo) toVideoStreamInfo() VideoStreamInfo {
	return VideoStreamInfo{
		AudioCodec:    s.AudioCodec,
		MasterURL:     s.MasterURL,
		Format:        s.Format,
		Height:        s.Height,
		Width:         s.Width,
		Size:          s.Size,
		StreamType:    s.StreamType,
		StreamDesc:    s.StreamDesc,
		QualityType:   s.QualityType,
		DefaultStream: s.DefaultStream,
		AvgBitrate:    s.AvgBitrate,
		VideoBitrate:  s.VideoBitrate,
		AudioBitrate:  s.AudioBitrate,
		Duration:      s.Duration,
		VideoDuration: s.VideoDuration,
		AudioDuration: s.AudioDuration,
		VideoCodec:    s.VideoCodec,
		BackupURLs:    s.BackupURLs,
		FPS:           s.FPS,
		Rotate:        s.Rotate,
	}
}

func ImageFileSuffix(rawURL string) string {
	lower := strings.ToLower(rawURL)
	switch {
	case strings.Contains(lower, "webp"):
		return ".webp"
	case strings.Contains(lower, ".png"):
		return ".png"
	case strings.Contains(lower, ".gif"):
		return ".gif"
	case strings.Contains(lower, ".jpeg"):
		return ".jpeg"
	case strings.Contains(lower, ".jpg"):
		return ".jpg"
	default:
		return ".jpg"
	}
}

func VideoFileSuffix(stream VideoStreamInfo) string {
	format := strings.Trim(strings.ToLower(stream.Format), ". ")
	if format == "" {
		return ".mp4"
	}
	return "." + format
}

func StreamLabel(stream VideoStreamInfo) string {
	for _, value := range []string{stream.QualityType, stream.StreamDesc, strconv.Itoa(stream.StreamType)} {
		value = strings.TrimSpace(value)
		if value != "" && value != "0" {
			return value
		}
	}
	return "默认/原始"
}
