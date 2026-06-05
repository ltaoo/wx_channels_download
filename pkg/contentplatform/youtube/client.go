package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

const (
	defaultBaseURL   = "https://www.youtube.com"
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.5 Safari/605.1.15"
)

var (
	initialPlayerResponseRE = regexp.MustCompile(`ytInitialPlayerResponse\s*=`)
	ytcfgSetRE              = regexp.MustCompile(`ytcfg\.set\s*\(`)
)

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	UserAgent  string
}

type VideoInfo struct {
	ID                string            `json:"id"`
	Title             string            `json:"title,omitempty"`
	Description       string            `json:"description,omitempty"`
	WebpageURL        string            `json:"webpage_url,omitempty"`
	Thumbnail         string            `json:"thumbnail,omitempty"`
	Thumbnails        []Thumbnail       `json:"thumbnails,omitempty"`
	Duration          int64             `json:"duration,omitempty"`
	ViewCount         int64             `json:"view_count,omitempty"`
	AgeLimit          int               `json:"age_limit,omitempty"`
	Channel           string            `json:"channel,omitempty"`
	ChannelID         string            `json:"channel_id,omitempty"`
	ChannelURL        string            `json:"channel_url,omitempty"`
	Uploader          string            `json:"uploader,omitempty"`
	UploaderID        string            `json:"uploader_id,omitempty"`
	UploaderURL       string            `json:"uploader_url,omitempty"`
	Categories        []string          `json:"categories,omitempty"`
	Tags              []string          `json:"tags,omitempty"`
	LiveStatus        string            `json:"live_status,omitempty"`
	MediaType         string            `json:"media_type,omitempty"`
	PlayableInEmbed   bool              `json:"playable_in_embed,omitempty"`
	Formats           []VideoFormat     `json:"formats,omitempty"`
	PlayabilityStatus PlayabilityStatus `json:"playability_status,omitempty"`
	Warnings          []string          `json:"warnings,omitempty"`
}

type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type PlayabilityStatus struct {
	Status string `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type VideoFormat struct {
	ID              string `json:"id"`
	Itag            int    `json:"itag,omitempty"`
	URL             string `json:"url,omitempty"`
	MimeType        string `json:"mime_type,omitempty"`
	Ext             string `json:"ext,omitempty"`
	Quality         string `json:"quality,omitempty"`
	QualityLabel    string `json:"quality_label,omitempty"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	FPS             int    `json:"fps,omitempty"`
	Bitrate         int    `json:"bitrate,omitempty"`
	AverageBitrate  int    `json:"average_bitrate,omitempty"`
	ContentLength   int64  `json:"content_length,omitempty"`
	AudioQuality    string `json:"audio_quality,omitempty"`
	AudioSampleRate int    `json:"audio_sample_rate,omitempty"`
	AudioChannels   int    `json:"audio_channels,omitempty"`
	AudioCodec      string `json:"audio_codec,omitempty"`
	VideoCodec      string `json:"video_codec,omitempty"`
	HasAudio        bool   `json:"has_audio,omitempty"`
	HasVideo        bool   `json:"has_video,omitempty"`
	Adaptive        bool   `json:"adaptive,omitempty"`
	NeedsSignature  bool   `json:"needs_signature,omitempty"`
	HasDRM          bool   `json:"has_drm,omitempty"`
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		HTTPClient: httpClient,
		BaseURL:    defaultBaseURL,
		UserAgent:  defaultUserAgent,
	}
}

func (c *Client) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	info, err := c.Extract(ctx, input.URL)
	if err != nil {
		return nil, err
	}
	variants, defaults, warnings := buildVariants(info)
	warnings = append(warnings, info.Warnings...)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: info.WebpageURL,
		ContentID:    info.ID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           firstNonEmpty(info.MediaType, "video"),
			ID:             info.ID,
			Title:          firstNonEmpty(info.Title, "youtube_"+info.ID),
			Description:    info.Description,
			Author:         firstNonEmpty(info.Channel, info.Uploader),
			URL:            info.WebpageURL,
			SourceURL:      info.WebpageURL,
			AuthorNickname: firstNonEmpty(info.Channel, info.Uploader),
			CoverURL:       info.Thumbnail,
			Duration:       info.Duration,
		}, info, map[string]any{
			"video_id":   info.ID,
			"channel_id": info.ChannelID,
			"uploader":   firstNonEmpty(info.Uploader, info.Channel),
		}, map[string]any{
			"content_type": "video",
			"title":        info.Title,
			"source_url":   info.WebpageURL,
			"video_id":     info.ID,
			"duration":     info.Duration,
			"format_count": len(info.Formats),
		}),
		Variants: variants,
		Defaults: defaults,
		Internal: map[string]any{"video_info": info},
		Warnings: warnings,
	}, nil
}

func (c *Client) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	probe := input.Probe
	if probe == nil {
		var err error
		probe, err = c.Probe(ctx, contentdownload.ProbeInput{URL: input.URL, Extra: input.Extra})
		if err != nil {
			return nil, err
		}
	}

	info := videoInfoFromProbe(probe)
	if info == nil {
		return nil, contentdownload.ErrResolveUnavailable
	}

	options := input.Options
	if options.VariantID == "" && strings.EqualFold(options.Suffix, ".mp3") {
		options.VariantID = firstVariantIDByType(probe, "audio")
	}
	variant, err := contentdownload.SelectVariant(probe, options)
	if err != nil {
		return nil, err
	}

	suffix := firstNonEmpty(options.Suffix, variant.Suffix)
	downloadURL := ""
	headers := c.downloadHeaders(info.WebpageURL)
	metadata := map[string]any{
		"variant_id": variant.ID,
		"video_id":   info.ID,
	}

	if variant.ID == "cover" {
		downloadURL = info.Thumbnail
		if downloadURL == "" {
			return nil, fmt.Errorf("youtube cover is unavailable")
		}
		suffix = firstNonEmpty(suffix, suffixFromURL(downloadURL), ".jpg")
		metadata["format_type"] = "cover"
	} else {
		formatID := formatIDOfVariant(variant)
		format := info.FindFormat(formatID)
		if format == nil {
			return nil, fmt.Errorf("youtube format %q not found", formatID)
		}
		if format.URL == "" || format.NeedsSignature {
			return nil, fmt.Errorf("youtube format %q does not have a direct downloadable URL", formatID)
		}
		downloadURL = format.URL
		if variant.Type == "audio" {
			suffix = firstNonEmpty(suffix, ".mp3")
		} else {
			suffix = firstNonEmpty(suffix, "."+firstNonEmpty(format.Ext, "mp4"))
		}
		metadata["format_id"] = format.ID
		metadata["itag"] = format.Itag
		metadata["format_type"] = formatType(*format)
		metadata["quality"] = firstNonEmpty(format.QualityLabel, format.Quality, format.AudioQuality)
		metadata["mime_type"] = format.MimeType
	}

	filename := firstNonEmpty(options.Filename, info.Title, "youtube_"+info.ID)
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    firstNonEmpty(probe.SourceURL, input.URL),
		CanonicalURL: info.WebpageURL,
		ContentID:    info.ID,
		Title:        info.Title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         downloadURL,
			Method:      http.MethodGet,
			Protocol:    "http",
			Connections: 4,
			Headers:     headers,
		},
		Labels: map[string]string{
			"platform":   PlatformID,
			"id":         info.ID,
			"title":      info.Title,
			"key":        "0",
			"spec":       variant.Spec,
			"suffix":     suffix,
			"source_url": info.WebpageURL,
		},
		Metadata: metadata,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           firstNonEmpty(info.MediaType, "video"),
			ID:             info.ID,
			Title:          info.Title,
			Description:    info.Description,
			Author:         firstNonEmpty(info.Channel, info.Uploader),
			URL:            info.WebpageURL,
			SourceURL:      info.WebpageURL,
			AuthorNickname: firstNonEmpty(info.Channel, info.Uploader),
			CoverURL:       info.Thumbnail,
			Duration:       info.Duration,
		}, info, contentdownload.ContentMetadataOf(probe.Content), contentdownload.ContentOutputOf(probe.Content)),
	}
	return resolved, nil
}

func (c *Client) Extract(ctx context.Context, rawURL string) (*VideoInfo, error) {
	videoID, ok := ExtractVideoID(rawURL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	watchURL := canonicalVideoURL(videoID)
	webpage, err := c.fetchWatchWebpage(ctx, videoID)
	if err != nil {
		return nil, err
	}

	playerResponse, hasPlayerResponse, err := parseInitialPlayerResponse(webpage)
	if err != nil {
		return nil, fmt.Errorf("parse youtube player response: %w", err)
	}
	ytcfg, _ := parseYTCfg(webpage)
	if !hasPlayerResponse || !playerResponse.hasStreamingData() {
		apiResponse, apiErr := c.fetchPlayerAPI(ctx, videoID, ytcfg, watchURL)
		if apiErr == nil {
			playerResponse = mergePlayerResponses(playerResponse, apiResponse)
			hasPlayerResponse = true
		} else if !hasPlayerResponse {
			return nil, apiErr
		}
	}
	if !hasPlayerResponse {
		return nil, fmt.Errorf("youtube player response not found")
	}

	info := buildVideoInfo(videoID, watchURL, playerResponse)
	if info.ID != videoID {
		return nil, fmt.Errorf("youtube player response video id %q does not match requested id %q", info.ID, videoID)
	}
	if info.PlayabilityStatus.Status != "" && info.PlayabilityStatus.Status != "OK" && len(info.Formats) == 0 {
		return nil, fmt.Errorf("youtube video unavailable: %s", firstNonEmpty(info.PlayabilityStatus.Reason, info.PlayabilityStatus.Status))
	}
	if len(info.Formats) == 0 {
		info.Warnings = append(info.Warnings, "未提取到可直接下载的 YouTube 格式")
	}
	return info, nil
}

func (info *VideoInfo) FindFormat(id string) *VideoFormat {
	id = strings.TrimSpace(id)
	for i := range info.Formats {
		if info.Formats[i].ID == id || strconv.Itoa(info.Formats[i].Itag) == id {
			return &info.Formats[i]
		}
	}
	return nil
}

func (c *Client) fetchWatchWebpage(ctx context.Context, videoID string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.watchURL(videoID), nil)
	if err != nil {
		return nil, err
	}
	c.setDefaultHeaders(req, "")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("youtube watch request failed: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) fetchPlayerAPI(ctx context.Context, videoID string, ytcfg map[string]any, referer string) (rawPlayerResponse, error) {
	apiKey := stringFromMap(ytcfg, "INNERTUBE_API_KEY")
	if apiKey == "" {
		return rawPlayerResponse{}, fmt.Errorf("youtube player response not found and INNERTUBE_API_KEY is missing")
	}
	contextValue, _ := ytcfg["INNERTUBE_CONTEXT"].(map[string]any)
	if contextValue == nil {
		contextValue = defaultInnertubeContext()
	}
	body, err := json.Marshal(map[string]any{
		"context": contextValue,
		"videoId": videoID,
	})
	if err != nil {
		return rawPlayerResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.playerAPIURL(apiKey), bytes.NewReader(body))
	if err != nil {
		return rawPlayerResponse{}, err
	}
	c.setDefaultHeaders(req, referer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-YouTube-Client-Name", "1")
	if client, ok := contextValue["client"].(map[string]any); ok {
		if version := stringFromMap(client, "clientVersion"); version != "" {
			req.Header.Set("X-YouTube-Client-Version", version)
		}
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return rawPlayerResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return rawPlayerResponse{}, fmt.Errorf("youtube player API request failed: %s", resp.Status)
	}
	var out rawPlayerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return rawPlayerResponse{}, err
	}
	return out, nil
}

func (c *Client) downloadHeaders(referer string) map[string]string {
	return map[string]string{
		"User-Agent":      c.userAgent(),
		"Accept":          "*/*",
		"Accept-Language": "en-US,en;q=0.9",
		"Referer":         referer,
	}
}

func (c *Client) setDefaultHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
		if parsed, err := url.Parse(referer); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			req.Header.Set("Origin", parsed.Scheme+"://"+parsed.Host)
		}
	}
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return NewClient(nil).HTTPClient
}

func (c *Client) userAgent() string {
	if c != nil && strings.TrimSpace(c.UserAgent) != "" {
		return c.UserAgent
	}
	return defaultUserAgent
}

func (c *Client) baseURL() string {
	if c != nil && strings.TrimSpace(c.BaseURL) != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return defaultBaseURL
}

func (c *Client) watchURL(videoID string) string {
	base, _ := url.Parse(c.baseURL())
	base.Path = "/watch"
	base.RawQuery = url.Values{"v": {videoID}}.Encode()
	return base.String()
}

func (c *Client) playerAPIURL(apiKey string) string {
	base, _ := url.Parse(c.baseURL())
	base.Path = "/youtubei/v1/player"
	base.RawQuery = url.Values{"key": {apiKey}}.Encode()
	return base.String()
}

func canonicalVideoURL(videoID string) string {
	return defaultBaseURL + "/watch?v=" + url.QueryEscape(videoID)
}

func parseInitialPlayerResponse(webpage []byte) (rawPlayerResponse, bool, error) {
	rawJSON, ok, err := extractJSONByRegexp(webpage, initialPlayerResponseRE)
	if err != nil || !ok {
		return rawPlayerResponse{}, ok, err
	}
	var out rawPlayerResponse
	if err := json.Unmarshal(rawJSON, &out); err != nil {
		return rawPlayerResponse{}, true, err
	}
	return out, true, nil
}

func parseYTCfg(webpage []byte) (map[string]any, bool) {
	rawJSON, ok, err := extractJSONByRegexp(webpage, ytcfgSetRE)
	if err != nil || !ok {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal(rawJSON, &out); err != nil {
		return nil, false
	}
	return out, true
}

func extractJSONByRegexp(data []byte, re *regexp.Regexp) ([]byte, bool, error) {
	loc := re.FindIndex(data)
	if loc == nil {
		return nil, false, nil
	}
	raw, err := extractJSONValue(data[loc[1]:])
	if err != nil {
		return nil, true, err
	}
	return raw, true, nil
}

func extractJSONValue(data []byte) ([]byte, error) {
	start := -1
	for i, b := range data {
		if b == '{' || b == '[' {
			start = i
			break
		}
		if b != ' ' && b != '\n' && b != '\t' && b != '\r' {
			continue
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("json object start not found")
	}

	var stack []byte
	inString := false
	escaped := false
	for i := start; i < len(data); i++ {
		b := data[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, b)
		case '}', ']':
			if len(stack) == 0 {
				return nil, fmt.Errorf("json object has unexpected closing delimiter")
			}
			open := stack[len(stack)-1]
			if (open == '{' && b != '}') || (open == '[' && b != ']') {
				return nil, fmt.Errorf("json object delimiters are unbalanced")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return data[start : i+1], nil
			}
		}
	}
	return nil, fmt.Errorf("json object end not found")
}

type rawPlayerResponse struct {
	PlayabilityStatus rawPlayabilityStatus `json:"playabilityStatus"`
	VideoDetails      rawVideoDetails      `json:"videoDetails"`
	StreamingData     rawStreamingData     `json:"streamingData"`
	Microformat       struct {
		Player rawMicroformat `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
}

type rawPlayabilityStatus struct {
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	PlayableInEmbed bool   `json:"playableInEmbed"`
}

type rawVideoDetails struct {
	VideoID          string       `json:"videoId"`
	Title            string       `json:"title"`
	ShortDescription string       `json:"shortDescription"`
	LengthSeconds    string       `json:"lengthSeconds"`
	ChannelID        string       `json:"channelId"`
	Author           string       `json:"author"`
	ViewCount        string       `json:"viewCount"`
	Keywords         []string     `json:"keywords"`
	Thumbnail        rawThumbnail `json:"thumbnail"`
	IsLiveContent    bool         `json:"isLiveContent"`
}

type rawMicroformat struct {
	Title             textRenderer `json:"title"`
	Description       textRenderer `json:"description"`
	LengthSeconds     string       `json:"lengthSeconds"`
	OwnerChannelName  string       `json:"ownerChannelName"`
	OwnerProfileURL   string       `json:"ownerProfileUrl"`
	ExternalChannelID string       `json:"externalChannelId"`
	ViewCount         string       `json:"viewCount"`
	Category          string       `json:"category"`
	IsFamilySafe      *bool        `json:"isFamilySafe"`
	IsShortsEligible  bool         `json:"isShortsEligible"`
	Thumbnail         rawThumbnail `json:"thumbnail"`
}

type textRenderer struct {
	SimpleText string `json:"simpleText"`
	Runs       []struct {
		Text string `json:"text"`
	} `json:"runs"`
}

type rawThumbnail struct {
	Thumbnails []Thumbnail `json:"thumbnails"`
}

type rawStreamingData struct {
	Formats          []rawFormat `json:"formats"`
	AdaptiveFormats  []rawFormat `json:"adaptiveFormats"`
	HLSManifestURL   string      `json:"hlsManifestUrl"`
	DASHManifestURL  string      `json:"dashManifestUrl"`
	ExpiresInSeconds string      `json:"expiresInSeconds"`
}

type rawFormat struct {
	Itag             int      `json:"itag"`
	URL              string   `json:"url"`
	SignatureCipher  string   `json:"signatureCipher"`
	Cipher           string   `json:"cipher"`
	MimeType         string   `json:"mimeType"`
	Bitrate          int      `json:"bitrate"`
	AverageBitrate   int      `json:"averageBitrate"`
	Width            int      `json:"width"`
	Height           int      `json:"height"`
	FPS              int      `json:"fps"`
	Quality          string   `json:"quality"`
	QualityLabel     string   `json:"qualityLabel"`
	AudioQuality     string   `json:"audioQuality"`
	AudioSampleRate  string   `json:"audioSampleRate"`
	AudioChannels    int      `json:"audioChannels"`
	ContentLength    string   `json:"contentLength"`
	ApproxDurationMS string   `json:"approxDurationMs"`
	DRMFamilies      []string `json:"drmFamilies"`
	Type             string   `json:"type"`
}

func (r rawPlayerResponse) hasStreamingData() bool {
	return len(r.StreamingData.Formats) > 0 ||
		len(r.StreamingData.AdaptiveFormats) > 0 ||
		r.StreamingData.HLSManifestURL != "" ||
		r.StreamingData.DASHManifestURL != ""
}

func mergePlayerResponses(base, next rawPlayerResponse) rawPlayerResponse {
	if next.VideoDetails.VideoID != "" || next.PlayabilityStatus.Status != "" {
		base = next
		return base
	}
	if len(next.StreamingData.Formats) > 0 || len(next.StreamingData.AdaptiveFormats) > 0 {
		base.StreamingData = next.StreamingData
	}
	return base
}

func (t textRenderer) text() string {
	if t.SimpleText != "" {
		return html.UnescapeString(t.SimpleText)
	}
	var b strings.Builder
	for _, run := range t.Runs {
		b.WriteString(run.Text)
	}
	return html.UnescapeString(b.String())
}

func buildVideoInfo(videoID string, webpageURL string, raw rawPlayerResponse) *VideoInfo {
	mf := raw.Microformat.Player
	vd := raw.VideoDetails
	if vd.VideoID != "" {
		videoID = vd.VideoID
	}
	duration := firstNonZeroInt64(parseInt64(vd.LengthSeconds), parseInt64(mf.LengthSeconds))
	viewCount := firstNonZeroInt64(parseInt64(vd.ViewCount), parseInt64(mf.ViewCount))
	title := firstNonEmpty(vd.Title, mf.Title.text(), "youtube_"+videoID)
	channel := firstNonEmpty(vd.Author, mf.OwnerChannelName)
	channelID := firstNonEmpty(vd.ChannelID, mf.ExternalChannelID)
	channelURL := mf.OwnerProfileURL
	if channelURL == "" && channelID != "" {
		channelURL = defaultBaseURL + "/channel/" + channelID
	}

	formats, warnings := extractFormats(raw.StreamingData)
	thumbnails := collectThumbnails(videoID, vd.Thumbnail.Thumbnails, mf.Thumbnail.Thumbnails)
	thumbnail := bestThumbnail(thumbnails)
	ageLimit := 0
	if mf.IsFamilySafe != nil && !*mf.IsFamilySafe {
		ageLimit = 18
	}
	mediaType := "video"
	if vd.IsLiveContent {
		mediaType = "livestream"
	} else if mf.IsShortsEligible {
		mediaType = "short"
	}
	liveStatus := "not_live"
	if vd.IsLiveContent {
		liveStatus = "is_live"
	}
	categories := []string(nil)
	if mf.Category != "" {
		categories = []string{mf.Category}
	}
	return &VideoInfo{
		ID:                videoID,
		Title:             title,
		Description:       firstNonEmpty(vd.ShortDescription, mf.Description.text()),
		WebpageURL:        canonicalVideoURL(videoID),
		Thumbnail:         thumbnail,
		Thumbnails:        thumbnails,
		Duration:          duration,
		ViewCount:         viewCount,
		AgeLimit:          ageLimit,
		Channel:           channel,
		ChannelID:         channelID,
		ChannelURL:        channelURL,
		Uploader:          channel,
		UploaderURL:       channelURL,
		Categories:        categories,
		Tags:              vd.Keywords,
		LiveStatus:        liveStatus,
		MediaType:         mediaType,
		PlayableInEmbed:   raw.PlayabilityStatus.PlayableInEmbed,
		Formats:           formats,
		PlayabilityStatus: PlayabilityStatus{Status: raw.PlayabilityStatus.Status, Reason: raw.PlayabilityStatus.Reason},
		Warnings:          warnings,
	}
}

func extractFormats(streaming rawStreamingData) ([]VideoFormat, []string) {
	var out []VideoFormat
	var warnings []string
	skippedSignature := 0
	skippedDRM := 0
	skippedOTF := 0
	nChallenge := 0
	videoOnly := 0

	add := func(raw rawFormat, adaptive bool) {
		format := formatFromRaw(raw, adaptive)
		if format.HasDRM {
			skippedDRM++
			return
		}
		if raw.Type == "FORMAT_STREAM_TYPE_OTF" {
			skippedOTF++
			return
		}
		format.URL, format.NeedsSignature = directFormatURL(raw)
		if format.NeedsSignature {
			skippedSignature++
			return
		}
		if format.URL == "" {
			return
		}
		if hasQueryParam(format.URL, "n") {
			nChallenge++
		}
		if format.HasVideo && !format.HasAudio {
			videoOnly++
		}
		out = append(out, format)
	}
	for _, raw := range streaming.Formats {
		add(raw, false)
	}
	for _, raw := range streaming.AdaptiveFormats {
		add(raw, true)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return formatScore(out[i]) > formatScore(out[j])
	})
	if skippedSignature > 0 {
		warnings = append(warnings, "部分 YouTube 格式需要 player JS 解签，已跳过")
	}
	if skippedDRM > 0 {
		warnings = append(warnings, "部分 YouTube 格式带 DRM，已跳过")
	}
	if skippedOTF > 0 {
		warnings = append(warnings, "部分 YouTube OTF 分片格式当前不支持，已跳过")
	}
	if nChallenge > 0 {
		warnings = append(warnings, "部分 URL 包含 YouTube n challenge，当前未解算，下载可能被限速")
	}
	if videoOnly > 0 {
		warnings = append(warnings, "已提取 video-only adaptive 格式，但当前下载管线不会自动合并音频")
	}
	if streaming.HLSManifestURL != "" || streaming.DASHManifestURL != "" {
		warnings = append(warnings, "已发现 HLS/DASH manifest；当前实现仅暴露 direct HTTPS 格式")
	}
	return out, warnings
}

func formatFromRaw(raw rawFormat, adaptive bool) VideoFormat {
	ext, audioCodec, videoCodec, hasAudio, hasVideo := parseMime(raw.MimeType, raw.AudioQuality, raw.AudioSampleRate)
	id := strconv.Itoa(raw.Itag)
	if raw.Itag == 0 {
		id = firstNonEmpty(raw.QualityLabel, raw.Quality, raw.AudioQuality)
	}
	return VideoFormat{
		ID:              id,
		Itag:            raw.Itag,
		MimeType:        raw.MimeType,
		Ext:             ext,
		Quality:         raw.Quality,
		QualityLabel:    raw.QualityLabel,
		Width:           raw.Width,
		Height:          raw.Height,
		FPS:             raw.FPS,
		Bitrate:         raw.Bitrate,
		AverageBitrate:  raw.AverageBitrate,
		ContentLength:   parseInt64(raw.ContentLength),
		AudioQuality:    strings.ToLower(raw.AudioQuality),
		AudioSampleRate: int(parseInt64(raw.AudioSampleRate)),
		AudioChannels:   raw.AudioChannels,
		AudioCodec:      audioCodec,
		VideoCodec:      videoCodec,
		HasAudio:        hasAudio,
		HasVideo:        hasVideo,
		Adaptive:        adaptive,
		HasDRM:          len(raw.DRMFamilies) > 0,
	}
}

func directFormatURL(raw rawFormat) (string, bool) {
	if raw.URL != "" {
		return raw.URL, false
	}
	cipher := firstNonEmpty(raw.SignatureCipher, raw.Cipher)
	if cipher == "" {
		return "", false
	}
	values, err := url.ParseQuery(cipher)
	if err != nil {
		return "", false
	}
	formatURL := values.Get("url")
	if formatURL == "" {
		return "", false
	}
	if values.Get("s") != "" {
		return "", true
	}
	return formatURL, false
}

func parseMime(mimeType string, audioQuality string, sampleRate string) (ext, audioCodec, videoCodec string, hasAudio, hasVideo bool) {
	mediaType, params, err := mime.ParseMediaType(mimeType)
	if err != nil {
		mediaType = strings.Split(mimeType, ";")[0]
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch mediaType {
	case "video/mp4":
		ext = "mp4"
	case "video/webm":
		ext = "webm"
	case "video/3gpp":
		ext = "3gp"
	case "audio/mp4":
		ext = "m4a"
	case "audio/webm":
		ext = "webm"
	default:
		if strings.Contains(mediaType, "mp4") {
			ext = "mp4"
		} else if strings.Contains(mediaType, "webm") {
			ext = "webm"
		}
	}
	codecs := splitCodecs(params["codecs"])
	for _, codec := range codecs {
		switch {
		case isAudioCodec(codec):
			if audioCodec == "" {
				audioCodec = codec
			}
		case isVideoCodec(codec):
			if videoCodec == "" {
				videoCodec = codec
			}
		}
	}
	hasVideo = strings.HasPrefix(mediaType, "video/") && videoCodec != "none"
	hasAudio = strings.HasPrefix(mediaType, "audio/") || audioCodec != "" || audioQuality != "" || sampleRate != ""
	if hasVideo && len(codecs) == 1 && audioQuality == "" && sampleRate == "" {
		hasAudio = false
	}
	return ext, audioCodec, videoCodec, hasAudio, hasVideo
}

func splitCodecs(codecs string) []string {
	if codecs == "" {
		return nil
	}
	parts := strings.Split(codecs, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), `"`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func isAudioCodec(codec string) bool {
	codec = strings.ToLower(codec)
	return strings.HasPrefix(codec, "mp4a") ||
		strings.HasPrefix(codec, "opus") ||
		strings.HasPrefix(codec, "vorbis") ||
		strings.HasPrefix(codec, "ac-3") ||
		strings.HasPrefix(codec, "ec-3")
}

func isVideoCodec(codec string) bool {
	codec = strings.ToLower(codec)
	return strings.HasPrefix(codec, "avc") ||
		strings.HasPrefix(codec, "vp") ||
		strings.HasPrefix(codec, "hev") ||
		strings.HasPrefix(codec, "hvc") ||
		strings.HasPrefix(codec, "av01")
}

func buildVariants(info *VideoInfo) ([]contentdownload.Variant, contentdownload.Defaults, []string) {
	var variants []contentdownload.Variant
	var warnings []string
	progressive := make([]VideoFormat, 0)
	audioOnly := make([]VideoFormat, 0)
	for _, format := range info.Formats {
		switch {
		case format.HasVideo && format.HasAudio:
			progressive = append(progressive, format)
		case format.HasAudio && !format.HasVideo:
			audioOnly = append(audioOnly, format)
		}
	}
	sort.SliceStable(progressive, func(i, j int) bool {
		return formatScore(progressive[i]) > formatScore(progressive[j])
	})
	sort.SliceStable(audioOnly, func(i, j int) bool {
		return audioScore(audioOnly[i]) > audioScore(audioOnly[j])
	})

	for _, format := range progressive {
		variants = append(variants, contentdownload.Variant{
			ID:       "format_" + format.ID,
			Type:     "video",
			Label:    videoFormatLabel(format),
			Spec:     format.ID,
			Suffix:   "." + firstNonEmpty(format.Ext, "mp4"),
			Size:     format.ContentLength,
			Width:    format.Width,
			Height:   format.Height,
			Bitrate:  firstNonZeroInt(format.AverageBitrate, format.Bitrate),
			Metadata: formatMetadata(format),
		})
	}
	if len(audioOnly) > 0 {
		format := audioOnly[0]
		variants = append(variants, contentdownload.Variant{
			ID:       "audio_mp3",
			Type:     "audio",
			Label:    audioFormatLabel(format),
			Spec:     format.ID,
			Suffix:   ".mp3",
			Size:     format.ContentLength,
			Bitrate:  firstNonZeroInt(format.AverageBitrate, format.Bitrate),
			Requires: []string{"ffmpeg"},
			Metadata: formatMetadata(format),
		})
	}
	if info.Thumbnail != "" {
		variants = append(variants, contentdownload.Variant{
			ID:     "cover",
			Type:   "image",
			Label:  "封面",
			Suffix: firstNonEmpty(suffixFromURL(info.Thumbnail), ".jpg"),
			Metadata: map[string]any{
				"url": info.Thumbnail,
			},
		})
	}

	defaults := contentdownload.Defaults{}
	if len(progressive) > 0 {
		best := progressive[0]
		defaults = contentdownload.Defaults{
			VariantID: "format_" + best.ID,
			Spec:      best.ID,
			Suffix:    "." + firstNonEmpty(best.Ext, "mp4"),
		}
	} else if len(audioOnly) > 0 {
		best := audioOnly[0]
		defaults = contentdownload.Defaults{
			VariantID: "audio_mp3",
			Spec:      best.ID,
			Suffix:    ".mp3",
		}
	} else if len(variants) > 0 {
		defaults = contentdownload.Defaults{
			VariantID: variants[0].ID,
			Spec:      variants[0].Spec,
			Suffix:    variants[0].Suffix,
		}
	}
	if len(progressive) == 0 {
		warnings = append(warnings, "未找到可单文件下载的 progressive 视频格式")
	}
	return variants, defaults, warnings
}

func videoInfoFromProbe(probe *contentdownload.Probe) *VideoInfo {
	if probe == nil {
		return nil
	}
	if info, ok := contentdownload.ContentDataOf(probe.Content).(*VideoInfo); ok {
		return info
	}
	if info, ok := contentdownload.ContentDataOf(probe.Content).(VideoInfo); ok {
		return &info
	}
	if info, ok := probe.Internal["video_info"].(*VideoInfo); ok {
		return info
	}
	return nil
}

func formatIDOfVariant(variant *contentdownload.Variant) string {
	if variant == nil {
		return ""
	}
	if variant.Metadata != nil {
		if v, ok := variant.Metadata["format_id"].(string); ok && v != "" {
			return v
		}
	}
	return variant.Spec
}

func firstVariantIDByType(probe *contentdownload.Probe, typ string) string {
	if probe == nil {
		return ""
	}
	for _, variant := range probe.Variants {
		if variant.Type == typ {
			return variant.ID
		}
	}
	return ""
}

func formatMetadata(format VideoFormat) map[string]any {
	return map[string]any{
		"format_id":       format.ID,
		"itag":            format.Itag,
		"mime_type":       format.MimeType,
		"quality":         format.Quality,
		"quality_label":   format.QualityLabel,
		"audio_quality":   format.AudioQuality,
		"audio_codec":     format.AudioCodec,
		"video_codec":     format.VideoCodec,
		"has_audio":       format.HasAudio,
		"has_video":       format.HasVideo,
		"adaptive":        format.Adaptive,
		"average_bitrate": format.AverageBitrate,
	}
}

func videoFormatLabel(format VideoFormat) string {
	parts := []string{"视频"}
	if format.QualityLabel != "" {
		parts = append(parts, format.QualityLabel)
	} else if format.Height > 0 {
		parts = append(parts, fmt.Sprintf("%dp", format.Height))
	} else if format.Quality != "" {
		parts = append(parts, format.Quality)
	}
	if format.Ext != "" {
		parts = append(parts, strings.ToUpper(format.Ext))
	}
	return strings.Join(parts, " ")
}

func audioFormatLabel(format VideoFormat) string {
	parts := []string{"MP3"}
	if bitrate := firstNonZeroInt(format.AverageBitrate, format.Bitrate); bitrate > 0 {
		parts = append(parts, fmt.Sprintf("%dk", bitrate/1000))
	}
	if format.Ext != "" {
		parts = append(parts, "from "+strings.ToUpper(format.Ext))
	}
	return strings.Join(parts, " ")
}

func formatType(format VideoFormat) string {
	switch {
	case format.HasVideo && format.HasAudio:
		return "progressive"
	case format.HasAudio:
		return "audio"
	case format.HasVideo:
		return "video"
	default:
		return "unknown"
	}
}

func formatScore(format VideoFormat) int {
	return format.Height*1_000_000 + format.FPS*10_000 + firstNonZeroInt(format.AverageBitrate, format.Bitrate)
}

func audioScore(format VideoFormat) int {
	return firstNonZeroInt(format.AverageBitrate, format.Bitrate) + int(format.ContentLength/1024)
}

func collectThumbnails(videoID string, groups ...[]Thumbnail) []Thumbnail {
	seen := map[string]bool{}
	var out []Thumbnail
	for _, group := range groups {
		for _, thumb := range group {
			thumb.URL = html.UnescapeString(thumb.URL)
			if thumb.URL == "" || seen[thumb.URL] {
				continue
			}
			seen[thumb.URL] = true
			out = append(out, thumb)
		}
	}
	for _, name := range []string{"maxresdefault", "hq720", "sddefault", "hqdefault"} {
		thumbURL := fmt.Sprintf("https://i.ytimg.com/vi/%s/%s.jpg", videoID, name)
		if !seen[thumbURL] {
			seen[thumbURL] = true
			out = append(out, Thumbnail{URL: thumbURL})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Width*out[i].Height > out[j].Width*out[j].Height
	})
	return out
}

func bestThumbnail(thumbnails []Thumbnail) string {
	if len(thumbnails) == 0 {
		return ""
	}
	return thumbnails[0].URL
}

func suffixFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path := strings.ToLower(parsed.Path)
	for _, suffix := range []string{".jpg", ".jpeg", ".png", ".webp", ".mp4", ".webm", ".m4a"} {
		if strings.HasSuffix(path, suffix) {
			return suffix
		}
	}
	return ""
}

func hasQueryParam(rawURL string, key string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.Query().Get(key) != ""
}

func defaultInnertubeContext() map[string]any {
	return map[string]any{
		"client": map[string]any{
			"clientName":    "WEB",
			"clientVersion": "2.20260114.08.00",
			"hl":            "en",
		},
	}
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return value
	}
	return ""
}

func parseInt64(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
