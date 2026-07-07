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

	"github.com/dop251/goja"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

const (
	defaultBaseURL         = "https://www.youtube.com"
	defaultUserAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.5 Safari/605.1.15"
	defaultMergedVariantID = "best"
	youtubeHTTPChunkSize   = 10 << 20
)

var (
	initialPlayerResponseRE = regexp.MustCompile(`ytInitialPlayerResponse\s*=`)
	initialDataRE           = regexp.MustCompile(`ytInitialData\s*=`)
	ytcfgSetRE              = regexp.MustCompile(`ytcfg\.set\s*\(`)
	playerJSURLRE           = regexp.MustCompile(`(?:"PLAYER_JS_URL"|"jsUrl")\s*:\s*"([^"]*/s/player/[^"]+base\.js)"`)
	sigFuncNameREs          = []*regexp.Regexp{
		regexp.MustCompile(`(?m)([A-Za-z0-9_$]+)\s*=\s*function\(\s*a\s*\)\s*\{\s*a\s*=\s*a\.split\(\s*""\s*\)`),
		regexp.MustCompile(`(?m)function\s+([A-Za-z0-9_$]+)\(\s*a\s*\)\s*\{\s*a\s*=\s*a\.split\(\s*""\s*\)`),
	}
	nFuncNameREs = []*regexp.Regexp{
		regexp.MustCompile(`\.get\(\s*"n"\s*\)\s*\)\s*&&\s*\([^=]+=\s*([A-Za-z0-9_$]+)(?:\[\d+\])?\(`),
		regexp.MustCompile(`([A-Za-z0-9_$]+)\s*=\s*function\(\s*\w+\s*\)\s*\{\s*var\s+\w+\s*=\s*\w+\.split\(`),
	}
	nArrayCallRE         = regexp.MustCompile(`\.get\(\s*"n"\s*\)\s*\)\s*&&\s*\([^=]+=\s*([A-Za-z0-9_$]+)\[(\d+)\]\(`)
	signatureTimestampRE = regexp.MustCompile(`(?:signatureTimestamp|sts)\s*:\s*(\d{5})`)
)

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	UserAgent  string
	Cookie     string
	PoToken    string
}

type innertubeClient struct {
	Name       string
	HeaderID   string
	Host       string
	UserAgent  string
	Context    map[string]any
	RequireJS  bool
	UseCookies bool
	OmitAPIKey bool
}

func defaultInnertubeClients() []innertubeClient {
	return []innertubeClient{
		{
			Name:      "android_vr",
			HeaderID:  "28",
			Host:      "www.youtube.com",
			UserAgent: "com.google.android.apps.youtube.vr.oculus/1.65.10 (Linux; U; Android 12L; eureka-user Build/SQ3A.220605.009.A1) gzip",
			Context: map[string]any{
				"client": map[string]any{
					"clientName":        "ANDROID_VR",
					"clientVersion":     "1.65.10",
					"deviceMake":        "Oculus",
					"deviceModel":       "Quest 3",
					"androidSdkVersion": 32,
					"userAgent":         "com.google.android.apps.youtube.vr.oculus/1.65.10 (Linux; U; Android 12L; eureka-user Build/SQ3A.220605.009.A1) gzip",
					"osName":            "Android",
					"osVersion":         "12L",
					"hl":                "en",
					"timeZone":          "UTC",
					"utcOffsetMinutes":  0,
				},
			},
			RequireJS:  false,
			OmitAPIKey: true,
		},
		{
			Name:      "web_safari",
			HeaderID:  "1",
			Host:      "www.youtube.com",
			UserAgent: defaultUserAgent + ",gzip(gfe)",
			Context: map[string]any{
				"client": map[string]any{
					"clientName":       "WEB",
					"clientVersion":    "2.20260114.08.00",
					"userAgent":        defaultUserAgent + ",gzip(gfe)",
					"hl":               "en",
					"timeZone":         "UTC",
					"utcOffsetMinutes": 0,
				},
			},
			RequireJS:  true,
			UseCookies: true,
		},
	}
}

type VideoInfo struct {
	ID                        string            `json:"id"`
	Title                     string            `json:"title,omitempty"`
	Description               string            `json:"description,omitempty"`
	WebpageURL                string            `json:"webpage_url,omitempty"`
	Thumbnail                 string            `json:"thumbnail,omitempty"`
	Thumbnails                []Thumbnail       `json:"thumbnails,omitempty"`
	Duration                  int64             `json:"duration,omitempty"`
	ViewCount                 int64             `json:"view_count,omitempty"`
	AgeLimit                  int               `json:"age_limit,omitempty"`
	Channel                   string            `json:"channel,omitempty"`
	ChannelID                 string            `json:"channel_id,omitempty"`
	ChannelURL                string            `json:"channel_url,omitempty"`
	ChannelAvatarURL          string            `json:"channel_avatar_url,omitempty"`
	Uploader                  string            `json:"uploader,omitempty"`
	UploaderID                string            `json:"uploader_id,omitempty"`
	UploaderURL               string            `json:"uploader_url,omitempty"`
	UploaderAvatarURL         string            `json:"uploader_avatar_url,omitempty"`
	Categories                []string          `json:"categories,omitempty"`
	Tags                      []string          `json:"tags,omitempty"`
	LiveStatus                string            `json:"live_status,omitempty"`
	MediaType                 string            `json:"media_type,omitempty"`
	PlayableInEmbed           bool              `json:"playable_in_embed,omitempty"`
	Formats                   []VideoFormat     `json:"formats,omitempty"`
	PlayabilityStatus         PlayabilityStatus `json:"playability_status,omitempty"`
	Warnings                  []string          `json:"warnings,omitempty"`
	InitialPlayerResponseJSON json.RawMessage   `json:"-"`
	YTCfgJSON                 json.RawMessage   `json:"-"`
	PageHTML                  string            `json:"-"`
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
	Protocol        string `json:"protocol,omitempty"`
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
			AuthorAvatarURL: firstNonEmpty(
				info.ChannelAvatarURL,
				info.UploaderAvatarURL,
			),
			CoverURL: info.Thumbnail,
			Duration: info.Duration,
		}, info, map[string]any{
			"video_id":            info.ID,
			"channel_id":          info.ChannelID,
			"author_homepage_url": firstNonEmpty(info.ChannelURL, info.UploaderURL),
			"channel_url":         info.ChannelURL,
			"channel_avatar_url":  info.ChannelAvatarURL,
			"uploader":            firstNonEmpty(info.Uploader, info.Channel),
			"uploader_url":        info.UploaderURL,
			"uploader_avatar_url": info.UploaderAvatarURL,
		}, ProbeOutput{
			ContentType:      "video",
			Title:            info.Title,
			SourceURL:        info.WebpageURL,
			VideoID:          info.ID,
			Duration:         info.Duration,
			Channel:          info.Channel,
			ChannelID:        info.ChannelID,
			ChannelURL:       info.ChannelURL,
			ChannelAvatarURL: info.ChannelAvatarURL,
			FormatCount:      len(info.Formats),
		}.Map()),
		Variants: variants,
		Defaults: defaults,
		Internal: map[string]any{
			"video_info": info,
			"pagejson":   info.InitialPlayerResponseJSON,
			"pagehtml":   info.PageHTML,
			"ytcfg":      info.YTCfgJSON,
		},
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
	if isMergedVariant(variant) && !ffmpegAvailableFromOptions(options) {
		if fallback := firstProgressiveVideoVariant(probe); fallback != nil {
			variant = fallback
		}
	}
	if isPlayerResponseJSONVariant(variant) {
		return resolvePlayerResponseJSON(probe, info, options)
	}

	suffix := firstNonEmpty(options.Suffix, variant.Suffix)
	downloadURL := ""
	downloadSpec := contentdownload.DownloadSpec{}
	headers := c.downloadHeaders(info.WebpageURL)
	metadata := map[string]any{
		"variant_id":          variant.ID,
		"video_id":            info.ID,
		"channel_id":          info.ChannelID,
		"author_homepage_url": firstNonEmpty(info.ChannelURL, info.UploaderURL),
		"channel_url":         info.ChannelURL,
		"channel_avatar_url":  info.ChannelAvatarURL,
		"uploader":            firstNonEmpty(info.Uploader, info.Channel),
		"uploader_url":        info.UploaderURL,
		"uploader_avatar_url": info.UploaderAvatarURL,
	}

	if variant.ID == "cover" {
		downloadURL = info.Thumbnail
		if downloadURL == "" {
			return nil, fmt.Errorf("youtube cover is unavailable")
		}
		suffix = firstNonEmpty(suffix, suffixFromURL(downloadURL), ".jpg")
		metadata["format_type"] = "cover"
		downloadSpec = youtubeDownloadSpec(variant, info.WebpageURL, downloadURL, headers)
	} else if isMergedVariant(variant) {
		sources, formats, err := mergedFormatSources(info, variant, headers)
		if err != nil {
			return nil, err
		}
		formatIDs := make([]string, 0, len(formats))
		directURLs := make([]string, 0, len(formats))
		for _, format := range formats {
			formatIDs = append(formatIDs, format.ID)
			directURLs = append(directURLs, format.URL)
		}
		suffix = firstNonEmpty(suffix, variant.Suffix, "."+mergedOutputExt(formats[0], formats[1]))
		metadata["format_id"] = strings.Join(formatIDs, "+")
		metadata["format_type"] = "merged"
		metadata["variant_type"] = variant.Type
		metadata["requested_format_ids"] = formatIDs
		metadata["sources"] = sources
		metadata["direct_urls"] = directURLs
		metadata["requires_ffmpeg"] = true
		if poToken := c.PoToken; strings.TrimSpace(poToken) != "" {
			metadata["po_token"] = poToken
		}
		downloadSpec = youtubeMergedDownloadSpec(info.ID, formatIDs, sources)
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
		metadata["variant_type"] = variant.Type
		metadata["quality"] = firstNonEmpty(format.QualityLabel, format.Quality, format.AudioQuality)
		metadata["mime_type"] = format.MimeType
		metadata["direct_url"] = format.URL
		if poToken := c.PoToken; strings.TrimSpace(poToken) != "" {
			metadata["po_token"] = poToken
		}
		downloadSpec = youtubeDownloadSpec(variant, info.WebpageURL, downloadURL, headers)
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
		Download:     downloadSpec,
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
			AuthorAvatarURL: firstNonEmpty(
				info.ChannelAvatarURL,
				info.UploaderAvatarURL,
			),
			CoverURL: info.Thumbnail,
			Duration: info.Duration,
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
	initialPlayerResponseJSON, _, _ := ExtractInitialPlayerResponseJSON(webpage)
	initialDataOwner, _, _ := parseInitialDataOwner(webpage)
	ytcfgJSON, _, _ := ExtractYTCfgJSON(webpage)
	ytcfg, _ := parseYTCfg(webpage)
	player := c.newPlayerResolver(ctx, webpage, ytcfg)
	playerResponses := make([]rawPlayerResponse, 0, 4)
	if hasPlayerResponse {
		playerResponses = append(playerResponses, playerResponse)
	}
	if apiKey := stringFromMap(ytcfg, "INNERTUBE_API_KEY"); apiKey != "" {
		apiSuccess := false
		if hasPlayerResponse {
			playerResponses = []rawPlayerResponse{playerResponseWithoutStreamingData(playerResponse)}
		}
		visitorData := extractVisitorData(ytcfg, playerResponse)
		for _, client := range c.requestedInnertubeClients() {
			if client.RequireJS {
				if _, err := player.playerCode(); err != nil {
					player.warn(fmt.Sprintf("youtube %s player API skipped: player JS unavailable: %v", client.Name, err))
					continue
				}
			}
			apiResponse, apiErr := c.fetchPlayerAPIForClient(ctx, videoID, ytcfg, watchURL, client, player.signatureTimestamp(), visitorData)
			if apiErr != nil {
				player.warn(fmt.Sprintf("youtube %s player API failed: %v", client.Name, apiErr))
				continue
			}
			if prID := apiResponse.VideoDetails.VideoID; prID != "" && prID != videoID {
				player.warn(fmt.Sprintf("youtube %s player response video id %q does not match requested id %q", client.Name, prID, videoID))
				continue
			}
			playerResponses = append(playerResponses, apiResponse)
			apiSuccess = true
		}
		if !apiSuccess && hasPlayerResponse {
			playerResponses = []rawPlayerResponse{playerResponse}
		}
	}
	if len(playerResponses) > 0 {
		playerResponse = mergePlayerResponseList(playerResponses)
		hasPlayerResponse = true
	}
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

	info := buildVideoInfo(videoID, watchURL, playerResponse, initialDataOwner, player)
	info.InitialPlayerResponseJSON = initialPlayerResponseJSON
	info.YTCfgJSON = ytcfgJSON
	info.PageHTML = string(webpage)
	info.Warnings = append(info.Warnings, player.warnings...)
	if len(info.InitialPlayerResponseJSON) == 0 {
		if raw, err := json.Marshal(playerResponse); err == nil {
			info.InitialPlayerResponseJSON = raw
		}
	}
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

func (c *Client) fetchPlayerAPIForClient(ctx context.Context, videoID string, ytcfg map[string]any, referer string, client innertubeClient, sts string, visitorData string) (rawPlayerResponse, error) {
	apiKey := stringFromMap(ytcfg, "INNERTUBE_API_KEY")
	if apiKey == "" && !client.OmitAPIKey {
		return rawPlayerResponse{}, fmt.Errorf("INNERTUBE_API_KEY is missing")
	}
	if client.OmitAPIKey {
		apiKey = ""
	}
	contextValue := cloneMap(client.Context)
	if contextValue == nil {
		contextValue = defaultInnertubeContext()
	}
	query := map[string]any{
		"context":        contextValue,
		"videoId":        videoID,
		"contentCheckOk": true,
		"racyCheckOk":    true,
		"playbackContext": map[string]any{
			"contentPlaybackContext": map[string]any{
				"html5Preference": "HTML5_PREF_WANTS",
			},
		},
	}
	if sts != "" {
		if playbackContext, ok := query["playbackContext"].(map[string]any); ok {
			if contentPlaybackContext, ok := playbackContext["contentPlaybackContext"].(map[string]any); ok {
				contentPlaybackContext["signatureTimestamp"] = sts
			}
		}
	}
	body, err := json.Marshal(query)
	if err != nil {
		return rawPlayerResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.playerAPIURL(apiKey), bytes.NewReader(body))
	if err != nil {
		return rawPlayerResponse{}, err
	}
	c.setDefaultHeaders(req, referer)
	if client.UserAgent != "" {
		req.Header.Set("User-Agent", client.UserAgent)
	}
	if !client.UseCookies {
		req.Header.Del("Cookie")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-YouTube-Client-Name", client.HeaderID)
	if visitorData != "" {
		req.Header.Set("X-Goog-Visitor-Id", visitorData)
	}
	if clientMap, ok := contextValue["client"].(map[string]any); ok {
		if version := stringFromMap(clientMap, "clientVersion"); version != "" {
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

func (c *Client) requestedInnertubeClients() []innertubeClient {
	return defaultInnertubeClients()
}

func (c *Client) downloadHeaders(referer string) map[string]string {
	headers := map[string]string{
		"User-Agent":       c.userAgent(),
		"Accept":           "*/*",
		"Accept-Encoding":  "identity",
		"Accept-Language":  "en-US,en;q=0.9",
		"Referer":          referer,
		"Sec-Fetch-Dest":   "video",
		"Sec-Fetch-Mode":   "no-cors",
		"Sec-Fetch-Site":   "cross-site",
		"X-YouTube-Client": "web",
	}
	if cookie := c.cookie(); cookie != "" {
		headers["Cookie"] = cookie
	}
	return headers
}

func (c *Client) setDefaultHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if cookie := c.cookie(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	} else {
		req.Header.Set("Cookie", "PREF=hl=en&tz=UTC; SOCS=CAI")
	}
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

func (c *Client) cookie() string {
	if c != nil {
		return strings.TrimSpace(c.Cookie)
	}
	return ""
}

func (c *Client) poToken() string {
	if c == nil {
		return ""
	}
	token := strings.TrimSpace(c.PoToken)
	if token == "" {
		return ""
	}
	if before, after, ok := strings.Cut(token, "+"); ok {
		meta := strings.ToLower(strings.TrimSpace(before))
		if strings.Contains(meta, ".") && !strings.HasSuffix(meta, ".gvs") {
			return ""
		}
		token = after
	}
	token, _ = url.QueryUnescape(token)
	for _, sep := range []string{"?", "&", "#"} {
		if i := strings.Index(token, sep); i >= 0 {
			token = token[:i]
		}
	}
	return strings.TrimSpace(token)
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
	query := url.Values{"prettyPrint": {"false"}}
	if apiKey != "" {
		query.Set("key", apiKey)
	}
	base.RawQuery = query.Encode()
	return base.String()
}

func canonicalVideoURL(videoID string) string {
	return defaultBaseURL + "/watch?v=" + url.QueryEscape(videoID)
}

func parseInitialPlayerResponse(webpage []byte) (rawPlayerResponse, bool, error) {
	rawJSON, ok, err := ExtractInitialPlayerResponseJSON(webpage)
	if err != nil || !ok {
		return rawPlayerResponse{}, ok, err
	}
	var out rawPlayerResponse
	if err := json.Unmarshal(rawJSON, &out); err != nil {
		return rawPlayerResponse{}, true, err
	}
	return out, true, nil
}

func ExtractInitialPlayerResponseJSON(webpage []byte) (json.RawMessage, bool, error) {
	rawJSON, ok, err := extractJSONByRegexp(webpage, initialPlayerResponseRE)
	if err != nil || !ok {
		return nil, ok, err
	}
	return json.RawMessage(append([]byte(nil), rawJSON...)), true, nil
}

func ExtractInitialDataJSON(webpage []byte) (json.RawMessage, bool, error) {
	rawJSON, ok, err := extractJSONByRegexp(webpage, initialDataRE)
	if err != nil || !ok {
		return nil, ok, err
	}
	if !json.Valid(rawJSON) {
		return nil, true, fmt.Errorf("invalid youtube initial data json")
	}
	return json.RawMessage(append([]byte(nil), rawJSON...)), true, nil
}

func parseInitialDataOwner(webpage []byte) (rawInitialDataOwner, bool, error) {
	rawJSON, ok, err := ExtractInitialDataJSON(webpage)
	if err != nil || !ok {
		return rawInitialDataOwner{}, ok, err
	}
	var data rawInitialData
	if err := json.Unmarshal(rawJSON, &data); err != nil {
		return rawInitialDataOwner{}, true, err
	}
	owner := ownerFromInitialData(data)
	return owner, owner.hasAny(), nil
}

func ownerFromInitialData(data rawInitialData) rawInitialDataOwner {
	for _, content := range data.Contents.TwoColumnWatchNextResults.Results.Results.Contents {
		owner := ownerFromVideoOwnerRenderer(content.VideoSecondaryInfoRenderer.Owner.VideoOwnerRenderer)
		if owner.hasAny() {
			return owner
		}
	}
	return rawInitialDataOwner{}
}

func ownerFromVideoOwnerRenderer(renderer rawVideoOwnerRenderer) rawInitialDataOwner {
	endpoint := renderer.NavigationEndpoint
	if endpoint.empty() {
		for _, run := range renderer.Title.Runs {
			if !run.NavigationEndpoint.empty() {
				endpoint = run.NavigationEndpoint
				break
			}
		}
	}
	channelURL := firstNonEmpty(
		endpoint.CommandMetadata.WebCommandMetadata.URL,
		endpoint.BrowseEndpoint.CanonicalBaseURL,
	)
	return rawInitialDataOwner{
		Channel:          renderer.Title.text(),
		ChannelID:        endpoint.BrowseEndpoint.BrowseID,
		ChannelURL:       absoluteYouTubeURL(channelURL),
		ChannelAvatarURL: bestThumbnail(collectImageThumbnails(renderer.Thumbnail.Thumbnails)),
	}
}

func (o rawInitialDataOwner) hasAny() bool {
	return o.Channel != "" || o.ChannelID != "" || o.ChannelURL != "" || o.ChannelAvatarURL != ""
}

func (e rawNavigationEndpoint) empty() bool {
	return e.CommandMetadata.WebCommandMetadata.URL == "" &&
		e.BrowseEndpoint.BrowseID == "" &&
		e.BrowseEndpoint.CanonicalBaseURL == ""
}

func parseYTCfg(webpage []byte) (map[string]any, bool) {
	rawJSON, ok, err := ExtractYTCfgJSON(webpage)
	if err != nil || !ok {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal(rawJSON, &out); err != nil {
		return nil, false
	}
	return out, true
}

func ExtractYTCfgJSON(webpage []byte) (json.RawMessage, bool, error) {
	matches := ytcfgSetRE.FindAllIndex(webpage, -1)
	var foundInvalid bool
	var lastErr error
	for _, loc := range matches {
		rawJSON, err := extractJSONValue(webpage[loc[1]:])
		if err != nil {
			lastErr = err
			continue
		}
		if !json.Valid(rawJSON) {
			foundInvalid = true
			lastErr = fmt.Errorf("invalid youtube ytcfg json")
			continue
		}
		return json.RawMessage(append([]byte(nil), rawJSON...)), true, nil
	}
	if foundInvalid {
		return nil, true, lastErr
	}
	if lastErr != nil {
		return nil, true, lastErr
	}
	return nil, false, nil
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
	ResponseContext   rawResponseContext   `json:"responseContext"`
	Microformat       struct {
		Player rawMicroformat `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
}

type rawPlayabilityStatus struct {
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	PlayableInEmbed bool   `json:"playableInEmbed"`
}

type rawResponseContext struct {
	VisitorData               string `json:"visitorData"`
	MainAppWebResponseContext struct {
		DatasyncID string `json:"datasyncId"`
	} `json:"mainAppWebResponseContext"`
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
		Text               string                `json:"text"`
		NavigationEndpoint rawNavigationEndpoint `json:"navigationEndpoint"`
	} `json:"runs"`
}

type rawThumbnail struct {
	Thumbnails []Thumbnail `json:"thumbnails"`
}

type rawInitialData struct {
	Contents struct {
		TwoColumnWatchNextResults struct {
			Results struct {
				Results struct {
					Contents []rawWatchNextContent `json:"contents"`
				} `json:"results"`
			} `json:"results"`
		} `json:"twoColumnWatchNextResults"`
	} `json:"contents"`
}

type rawWatchNextContent struct {
	VideoSecondaryInfoRenderer struct {
		Owner struct {
			VideoOwnerRenderer rawVideoOwnerRenderer `json:"videoOwnerRenderer"`
		} `json:"owner"`
	} `json:"videoSecondaryInfoRenderer"`
}

type rawVideoOwnerRenderer struct {
	Thumbnail          rawThumbnail          `json:"thumbnail"`
	Title              textRenderer          `json:"title"`
	NavigationEndpoint rawNavigationEndpoint `json:"navigationEndpoint"`
}

type rawNavigationEndpoint struct {
	CommandMetadata struct {
		WebCommandMetadata struct {
			URL string `json:"url"`
		} `json:"webCommandMetadata"`
	} `json:"commandMetadata"`
	BrowseEndpoint struct {
		BrowseID         string `json:"browseId"`
		CanonicalBaseURL string `json:"canonicalBaseUrl"`
	} `json:"browseEndpoint"`
}

type rawInitialDataOwner struct {
	Channel          string
	ChannelID        string
	ChannelURL       string
	ChannelAvatarURL string
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

func playerResponseWithoutStreamingData(response rawPlayerResponse) rawPlayerResponse {
	response.StreamingData = rawStreamingData{}
	return response
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

func mergePlayerResponseList(responses []rawPlayerResponse) rawPlayerResponse {
	var out rawPlayerResponse
	for _, response := range responses {
		if out.VideoDetails.VideoID == "" && response.VideoDetails.VideoID != "" {
			out.VideoDetails = response.VideoDetails
		}
		if out.PlayabilityStatus.Status == "" && response.PlayabilityStatus.Status != "" {
			out.PlayabilityStatus = response.PlayabilityStatus
		}
		if out.ResponseContext.VisitorData == "" && response.ResponseContext.VisitorData != "" {
			out.ResponseContext = response.ResponseContext
		}
		if out.Microformat.Player.Title.SimpleText == "" && len(out.Microformat.Player.Title.Runs) == 0 {
			out.Microformat = response.Microformat
		}
		out.StreamingData.Formats = appendUniqueRawFormats(out.StreamingData.Formats, response.StreamingData.Formats...)
		out.StreamingData.AdaptiveFormats = appendUniqueRawFormats(out.StreamingData.AdaptiveFormats, response.StreamingData.AdaptiveFormats...)
		if out.StreamingData.HLSManifestURL == "" {
			out.StreamingData.HLSManifestURL = response.StreamingData.HLSManifestURL
		}
		if out.StreamingData.DASHManifestURL == "" {
			out.StreamingData.DASHManifestURL = response.StreamingData.DASHManifestURL
		}
		if out.StreamingData.ExpiresInSeconds == "" {
			out.StreamingData.ExpiresInSeconds = response.StreamingData.ExpiresInSeconds
		}
	}
	return out
}

func appendUniqueRawFormats(base []rawFormat, values ...rawFormat) []rawFormat {
	seen := make(map[string]bool, len(base)+len(values))
	for _, format := range base {
		seen[rawFormatKey(format)] = true
	}
	for _, format := range values {
		key := rawFormatKey(format)
		if seen[key] {
			continue
		}
		seen[key] = true
		base = append(base, format)
	}
	return base
}

func rawFormatKey(format rawFormat) string {
	return strings.Join([]string{
		strconv.Itoa(format.Itag),
		format.MimeType,
		firstNonEmpty(format.URL, format.SignatureCipher, format.Cipher),
		format.AudioQuality,
	}, "\x00")
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

func buildVideoInfo(videoID string, webpageURL string, raw rawPlayerResponse, owner rawInitialDataOwner, player *playerResolver) *VideoInfo {
	mf := raw.Microformat.Player
	vd := raw.VideoDetails
	if vd.VideoID != "" {
		videoID = vd.VideoID
	}
	duration := firstNonZeroInt64(parseInt64(vd.LengthSeconds), parseInt64(mf.LengthSeconds))
	viewCount := firstNonZeroInt64(parseInt64(vd.ViewCount), parseInt64(mf.ViewCount))
	title := firstNonEmpty(vd.Title, mf.Title.text(), "youtube_"+videoID)
	channel := firstNonEmpty(vd.Author, mf.OwnerChannelName, owner.Channel)
	channelID := firstNonEmpty(vd.ChannelID, mf.ExternalChannelID, owner.ChannelID)
	channelURL := firstNonEmpty(mf.OwnerProfileURL, owner.ChannelURL)
	if channelURL == "" && channelID != "" {
		channelURL = defaultBaseURL + "/channel/" + channelID
	}
	channelAvatarURL := owner.ChannelAvatarURL

	formats, warnings := extractFormats(raw.StreamingData, player)
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
		ChannelAvatarURL:  channelAvatarURL,
		Uploader:          channel,
		UploaderURL:       channelURL,
		UploaderAvatarURL: channelAvatarURL,
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

func extractFormats(streaming rawStreamingData, player *playerResolver) ([]VideoFormat, []string) {
	var out []VideoFormat
	var warnings []string
	skippedSignature := 0
	solvedSignature := 0
	skippedDRM := 0
	skippedOTF := 0
	nChallenge := 0
	solvedNChallenge := 0

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
		result := directFormatURL(raw, player)
		format.URL = result.URL
		format.NeedsSignature = result.NeedsSignature
		if result.NeedsSignature {
			skippedSignature++
			return
		}
		if format.URL == "" {
			return
		}
		if result.SolvedSignature {
			solvedSignature++
		}
		if result.HadNChallenge {
			nChallenge++
		}
		if result.SolvedNChallenge {
			solvedNChallenge++
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
	if solvedSignature > 0 {
		warnings = append(warnings, "已解算部分 YouTube player JS 签名格式")
	}
	if skippedDRM > 0 {
		warnings = append(warnings, "部分 YouTube 格式带 DRM，已跳过")
	}
	if skippedOTF > 0 {
		warnings = append(warnings, "部分 YouTube OTF 分片格式当前不支持，已跳过")
	}
	if nChallenge > solvedNChallenge {
		warnings = append(warnings, "部分 URL 包含 YouTube n challenge，当前未解算，下载可能被限速")
	} else if solvedNChallenge > 0 {
		warnings = append(warnings, "已解算 YouTube n challenge")
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
		Protocol:        "https",
		HasDRM:          len(raw.DRMFamilies) > 0,
	}
}

type formatURLResult struct {
	URL              string
	NeedsSignature   bool
	SolvedSignature  bool
	HadNChallenge    bool
	SolvedNChallenge bool
}

func directFormatURL(raw rawFormat, player *playerResolver) formatURLResult {
	if raw.URL != "" {
		return finalizeFormatURL(resolveNChallenge(raw.URL, player), player)
	}
	cipher := firstNonEmpty(raw.SignatureCipher, raw.Cipher)
	if cipher == "" {
		return formatURLResult{}
	}
	values, err := url.ParseQuery(cipher)
	if err != nil {
		return formatURLResult{}
	}
	formatURL := values.Get("url")
	if formatURL == "" {
		return formatURLResult{}
	}
	if encryptedSig := values.Get("s"); encryptedSig != "" {
		if player == nil {
			return formatURLResult{NeedsSignature: true}
		}
		sig, err := player.solveSignature(encryptedSig)
		if err != nil {
			player.warn("youtube player JS signature solving failed: " + err.Error())
			return formatURLResult{NeedsSignature: true}
		}
		sp := values.Get("sp")
		if sp == "" {
			sp = "signature"
		}
		formatURL = updateURLQuery(formatURL, map[string]string{sp: sig})
		result := resolveNChallenge(formatURL, player)
		result.SolvedSignature = true
		return finalizeFormatURL(result, player)
	}
	return finalizeFormatURL(resolveNChallenge(formatURL, player), player)
}

func resolveNChallenge(formatURL string, player *playerResolver) formatURLResult {
	result := formatURLResult{URL: formatURL}
	nValue := queryValue(formatURL, "n")
	if nValue == "" {
		return result
	}
	result.HadNChallenge = true
	if player == nil {
		return result
	}
	nResult, err := player.solveN(nValue)
	if err != nil {
		player.warn("youtube player JS n challenge solving failed: " + err.Error())
		return result
	}
	result.URL = updateURLQuery(formatURL, map[string]string{"n": nResult})
	result.SolvedNChallenge = true
	return result
}

func finalizeFormatURL(result formatURLResult, player *playerResolver) formatURLResult {
	if result.URL == "" || result.NeedsSignature || player == nil || player.client == nil {
		return result
	}
	if poToken := player.client.poToken(); poToken != "" {
		result.URL = updateURLQuery(result.URL, map[string]string{"pot": poToken})
	}
	return result
}

type playerResolver struct {
	client    *Client
	ctx       context.Context
	playerURL string
	playerJS  string
	fetchErr  error
	sigName   string
	nName     string
	sigSpecs  map[int][]int
	warnings  []string
	warned    map[string]bool
}

func (c *Client) newPlayerResolver(ctx context.Context, webpage []byte, ytcfg map[string]any) *playerResolver {
	return &playerResolver{
		client:    c,
		ctx:       ctx,
		playerURL: c.extractPlayerJSURL(webpage, ytcfg),
		sigSpecs:  map[int][]int{},
		warned:    map[string]bool{},
	}
}

func (c *Client) extractPlayerJSURL(webpage []byte, ytcfg map[string]any) string {
	for _, key := range []string{"PLAYER_JS_URL", "jsUrl"} {
		if value := stringFromMap(ytcfg, key); value != "" {
			return c.absYouTubeURL(value)
		}
	}
	if value := findPlayerJSURL(ytcfg); value != "" {
		return c.absYouTubeURL(value)
	}
	if match := playerJSURLRE.FindSubmatch(webpage); len(match) == 2 {
		value := strings.ReplaceAll(string(match[1]), `\/`, `/`)
		return c.absYouTubeURL(value)
	}
	return ""
}

func findPlayerJSURL(value any) string {
	switch v := value.(type) {
	case string:
		if strings.Contains(v, "/s/player/") && strings.Contains(v, "base.js") {
			return v
		}
	case map[string]any:
		for _, child := range v {
			if found := findPlayerJSURL(child); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range v {
			if found := findPlayerJSURL(child); found != "" {
				return found
			}
		}
	}
	return ""
}

func (c *Client) absYouTubeURL(raw string) string {
	return absoluteURL(raw, c.baseURL())
}

func absoluteYouTubeURL(raw string) string {
	return absoluteURL(raw, defaultBaseURL)
}

func absoluteURL(raw string, baseURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return raw
	}
	base, _ := url.Parse(strings.TrimRight(baseURL, "/") + "/")
	ref, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return base.ResolveReference(ref).String()
}

func (r *playerResolver) solveSignature(challenge string) (string, error) {
	code, err := r.playerCode()
	if err != nil {
		return "", err
	}
	if spec := r.sigSpecs[len(challenge)]; len(spec) > 0 {
		return applySignatureSpec(challenge, spec), nil
	}
	name := r.sigName
	if name == "" {
		name = findFirstFunctionName(code, sigFuncNameREs)
		r.sigName = name
	}
	if name != "" {
		if solved, err := solvePlayerFunction(code, name, challenge); err == nil {
			return solved, nil
		}
	}
	solved, err := solvePlayerChallengesWithNode(r.ctx, code, "sig", []string{challenge})
	if err != nil {
		if name == "" {
			return "", fmt.Errorf("signature function not found: %w", err)
		}
		return "", err
	}
	if out := solved[challenge]; out != "" {
		return out, nil
	}
	return "", fmt.Errorf("signature solver returned empty result")
}

func (r *playerResolver) solveN(challenge string) (string, error) {
	code, err := r.playerCode()
	if err != nil {
		return "", err
	}
	name := r.nName
	if name == "" {
		name = findNFunctionExpression(code)
		r.nName = name
	}
	if name != "" {
		if solved, err := solvePlayerFunction(code, name, challenge); err == nil {
			return solved, nil
		}
	}
	solved, err := solvePlayerChallengesWithNode(r.ctx, code, "n", []string{challenge})
	if err != nil {
		if name == "" {
			return "", fmt.Errorf("n function not found: %w", err)
		}
		return "", err
	}
	if out := solved[challenge]; out != "" {
		return out, nil
	}
	return "", fmt.Errorf("n solver returned empty result")
}

func (r *playerResolver) playerCode() (string, error) {
	if r.playerJS != "" {
		return r.playerJS, nil
	}
	if r.fetchErr != nil {
		return "", r.fetchErr
	}
	if r.playerURL == "" {
		r.fetchErr = fmt.Errorf("player JS URL not found")
		return "", r.fetchErr
	}
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.playerURL, nil)
	if err != nil {
		r.fetchErr = err
		return "", err
	}
	r.client.setDefaultHeaders(req, defaultBaseURL)
	resp, err := r.client.httpClient().Do(req)
	if err != nil {
		r.fetchErr = err
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.fetchErr = fmt.Errorf("player JS request failed: %s", resp.Status)
		return "", r.fetchErr
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		r.fetchErr = err
		return "", err
	}
	r.playerJS = string(body)
	return r.playerJS, nil
}

func (r *playerResolver) signatureTimestamp() string {
	code, err := r.playerCode()
	if err != nil {
		return ""
	}
	if match := signatureTimestampRE.FindStringSubmatch(code); len(match) == 2 {
		return match[1]
	}
	return ""
}

func (r *playerResolver) warn(message string) {
	if r == nil || message == "" {
		return
	}
	if r.warned == nil {
		r.warned = map[string]bool{}
	}
	if r.warned[message] {
		return
	}
	r.warned[message] = true
	r.warnings = append(r.warnings, message)
}

func findFirstFunctionName(code string, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(code); len(match) >= 2 {
			return match[1]
		}
	}
	return ""
}

func findNFunctionExpression(code string) string {
	if match := nArrayCallRE.FindStringSubmatch(code); len(match) == 3 {
		return match[1] + "[" + match[2] + "]"
	}
	return findFirstFunctionName(code, nFuncNameREs)
}

func solvePlayerFunction(playerJS string, functionName string, challenge string) (string, error) {
	if result, ok := solveSimplePlayerFunction(playerJS, functionName, challenge); ok {
		return result, nil
	}
	return solvePlayerFunctionWithGoja(playerJS, functionName, challenge)
}

func solvePlayerFunctionWithGoja(playerJS string, functionName string, challenge string) (string, error) {
	vm := goja.New()
	global := vm.GlobalObject()
	_ = global.Set("window", global)
	_ = global.Set("self", global)
	_ = global.Set("globalThis", global)
	_ = global.Set("navigator", map[string]any{"userAgent": defaultUserAgent})
	_ = global.Set("location", map[string]any{"href": defaultBaseURL + "/"})
	_ = global.Set("document", map[string]any{})
	_ = global.Set("console", map[string]func(...goja.Value){
		"log":   func(...goja.Value) {},
		"warn":  func(...goja.Value) {},
		"error": func(...goja.Value) {},
	})
	_ = global.Set("setTimeout", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
	_ = global.Set("clearTimeout", func(goja.FunctionCall) goja.Value { return goja.Undefined() })

	timer := time.AfterFunc(5*time.Second, func() {
		vm.Interrupt("youtube player JS execution timed out")
	})
	defer timer.Stop()

	if _, err := vm.RunString(playerJS); err != nil {
		return "", fmt.Errorf("run player JS: %w", err)
	}
	fn, ok := goja.AssertFunction(vm.Get(functionName))
	if !ok && strings.ContainsAny(functionName, ".[") {
		value, err := vm.RunString("(" + functionName + ")")
		if err != nil {
			return "", fmt.Errorf("evaluate player function %q: %w", functionName, err)
		}
		fn, ok = goja.AssertFunction(value)
	}
	if !ok {
		return "", fmt.Errorf("player function %q is not callable", functionName)
	}
	result, err := fn(goja.Undefined(), vm.ToValue(challenge))
	if err != nil {
		return "", fmt.Errorf("call player function %q: %w", functionName, err)
	}
	out := result.String()
	if out == "" {
		return "", fmt.Errorf("empty player function result")
	}
	return out, nil
}

func applySignatureSpec(value string, spec []int) string {
	if len(spec) == 0 {
		return value
	}
	runes := []rune(value)
	out := make([]rune, 0, len(spec))
	for _, index := range spec {
		if index >= 0 && index < len(runes) {
			out = append(out, runes[index])
		}
	}
	return string(out)
}

func solveSimplePlayerFunction(playerJS string, functionName string, challenge string) (string, bool) {
	body, ok := extractFunctionBody(playerJS, functionName)
	if !ok {
		return "", false
	}
	compact := strings.ReplaceAll(body, " ", "")
	compact = strings.ReplaceAll(compact, "\n", "")
	compact = strings.ReplaceAll(compact, "\t", "")
	switch {
	case strings.Contains(compact, `.split("").reverse().join("")`):
		return reverseString(challenge), true
	case strings.Contains(compact, `.reverse()`) && strings.Contains(compact, `.join("")`):
		return reverseString(challenge), true
	}
	return "", false
}

func extractFunctionBody(code string, name string) (string, bool) {
	if name == "" {
		return "", false
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`function\s+` + regexp.QuoteMeta(name) + `\s*\([^)]*\)\s*\{`),
		regexp.MustCompile(regexp.QuoteMeta(name) + `\s*=\s*function\s*\([^)]*\)\s*\{`),
	}
	for _, pattern := range patterns {
		loc := pattern.FindStringIndex(code)
		if loc == nil {
			continue
		}
		start := loc[1] - 1
		if end := matchingBraceIndex(code, start); end > start {
			return code[start+1 : end], true
		}
	}
	return "", false
}

func matchingBraceIndex(value string, start int) int {
	if start < 0 || start >= len(value) || value[start] != '{' {
		return -1
	}
	depth := 0
	inString := byte(0)
	escaped := false
	for i := start; i < len(value); i++ {
		ch := value[i]
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == inString {
				inString = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			inString = ch
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func reverseString(value string) string {
	runes := []rune(value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func queryValue(rawURL string, key string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Query().Get(key)
}

func updateURLQuery(rawURL string, values map[string]string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	for key, value := range values {
		if value == "" {
			continue
		}
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func cloneMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = cloneMap(typed)
		case []any:
			copied := make([]any, len(typed))
			copy(copied, typed)
			out[key] = copied
		default:
			out[key] = value
		}
	}
	return out
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
	videoOnly := make([]VideoFormat, 0)
	audioOnly := make([]VideoFormat, 0)
	for _, format := range info.Formats {
		switch {
		case format.HasVideo && format.HasAudio:
			progressive = append(progressive, format)
		case format.HasVideo && !format.HasAudio:
			videoOnly = append(videoOnly, format)
		case format.HasAudio && !format.HasVideo:
			audioOnly = append(audioOnly, format)
		}
	}
	sort.SliceStable(progressive, func(i, j int) bool {
		return formatScore(progressive[i]) > formatScore(progressive[j])
	})
	progressive = uniqueFormatsByID(progressive)
	sort.SliceStable(videoOnly, func(i, j int) bool {
		return formatScore(videoOnly[i]) > formatScore(videoOnly[j])
	})
	videoOnly = uniqueFormatsByID(videoOnly)
	sort.SliceStable(audioOnly, func(i, j int) bool {
		return audioScore(audioOnly[i]) > audioScore(audioOnly[j])
	})
	audioOnly = uniqueFormatsByID(audioOnly)

	if len(videoOnly) > 0 && len(audioOnly) > 0 {
		defaultVideo, defaultAudio := bestDefaultMergedPair(videoOnly, audioOnly)
		variants = append(variants, mergedVariant(defaultMergedVariantID, defaultVideo, defaultAudio, true))
		for _, video := range videoOnly {
			audio := bestAudioForVideo(video, audioOnly)
			variants = append(variants, mergedVariant(mergedVariantID(video, audio), video, audio, false))
		}
	}

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
	if len(info.InitialPlayerResponseJSON) > 0 {
		variants = append(variants, playerResponseJSONVariant())
	}

	defaults := contentdownload.Defaults{}
	if len(videoOnly) > 0 && len(audioOnly) > 0 {
		video, audio := bestDefaultMergedPair(videoOnly, audioOnly)
		mergedExt := mergedOutputExt(video, audio)
		defaults = contentdownload.Defaults{
			VariantID: defaultMergedVariantID,
			Spec:      video.ID + "+" + audio.ID,
			Suffix:    "." + mergedExt,
		}
	} else if len(progressive) > 0 {
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
	if len(progressive) == 0 && !(len(videoOnly) > 0 && len(audioOnly) > 0) {
		warnings = append(warnings, "未找到可单文件下载的 progressive 视频格式")
	}
	return variants, defaults, warnings
}

func playerResponseJSONVariant() contentdownload.Variant {
	return contentdownload.Variant{
		ID:     "player_response_json",
		Type:   "json",
		Label:  "PlayerResponse JSON",
		Suffix: ".json",
		Metadata: map[string]any{
			"format": "json",
			"source": "ytInitialPlayerResponse",
		},
	}
}

func mergedVariant(id string, video VideoFormat, audio VideoFormat, best bool) contentdownload.Variant {
	mergedExt := mergedOutputExt(video, audio)
	return contentdownload.Variant{
		ID:       id,
		Type:     "video",
		Label:    mergedFormatLabel(video, audio, best),
		Spec:     video.ID + "+" + audio.ID,
		Suffix:   "." + mergedExt,
		Size:     video.ContentLength + audio.ContentLength,
		Width:    video.Width,
		Height:   video.Height,
		Bitrate:  firstNonZeroInt(video.AverageBitrate, video.Bitrate) + firstNonZeroInt(audio.AverageBitrate, audio.Bitrate),
		Requires: []string{"ffmpeg"},
		Metadata: mergedFormatMetadata(video, audio, mergedExt),
	}
}

func mergedVariantID(video VideoFormat, audio VideoFormat) string {
	replacer := strings.NewReplacer("+", "_", "/", "_", " ", "_")
	return "merge_" + replacer.Replace(video.ID) + "_" + replacer.Replace(audio.ID)
}

func bestDefaultMergedPair(videoOnly []VideoFormat, audioOnly []VideoFormat) (VideoFormat, VideoFormat) {
	for _, video := range videoOnly {
		audio := bestAudioForVideo(video, audioOnly)
		if mergedOutputExt(video, audio) == "mp4" {
			return video, audio
		}
	}
	video := videoOnly[0]
	return video, bestAudioForVideo(video, audioOnly)
}

func bestAudioForVideo(video VideoFormat, audioOnly []VideoFormat) VideoFormat {
	preferredExt := strings.ToLower(video.Ext)
	for _, audio := range audioOnly {
		if mergedOutputExt(video, audio) == preferredExt {
			return audio
		}
	}
	return audioOnly[0]
}

func uniqueFormatsByID(formats []VideoFormat) []VideoFormat {
	if len(formats) < 2 {
		return formats
	}
	seen := make(map[string]bool, len(formats))
	out := make([]VideoFormat, 0, len(formats))
	for _, format := range formats {
		key := format.ID
		if key == "" {
			key = strconv.Itoa(format.Itag)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, format)
	}
	return out
}

func isPlayerResponseJSONVariant(variant *contentdownload.Variant) bool {
	return variant != nil && variant.ID == "player_response_json"
}

func resolvePlayerResponseJSON(probe *contentdownload.Probe, info *VideoInfo, options contentdownload.Options) (*contentdownload.ResolvedRequest, error) {
	raw := playerResponseJSONFromProbe(probe, info)
	if len(raw) == 0 {
		return nil, fmt.Errorf("missing youtube player response json")
	}
	contentID := firstNonEmpty(probe.ContentID, info.ID)
	title := firstNonEmpty(info.Title, contentID, "youtube")
	sourceURL := firstNonEmpty(probe.SourceURL, info.WebpageURL)
	canonicalURL := firstNonEmpty(probe.CanonicalURL, info.WebpageURL, sourceURL)
	filename := firstNonEmpty(options.Filename, title, contentID)
	suffix := firstNonEmpty(options.Suffix, ".json")
	return &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         "inline-json://youtube/" + contentID + "/player-response",
			Method:      http.MethodGet,
			Protocol:    "inline_json",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":   PlatformID,
			"id":         contentID,
			"title":      title,
			"key":        "0",
			"spec":       "",
			"suffix":     suffix,
			"source_url": canonicalURL,
		},
		Metadata: map[string]any{
			"variant_id":          "player_response_json",
			"video_id":            contentID,
			"author_homepage_url": firstNonEmpty(info.ChannelURL, info.UploaderURL),
			"channel_url":         info.ChannelURL,
			"channel_avatar_url":  info.ChannelAvatarURL,
			"source_url":          sourceURL,
			"canonical_url":       canonicalURL,
			"json":                json.RawMessage(append([]byte(nil), raw...)),
		},
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           firstNonEmpty(info.MediaType, "video"),
			ID:             contentID,
			Title:          title,
			Description:    info.Description,
			Author:         firstNonEmpty(info.Channel, info.Uploader),
			URL:            canonicalURL,
			SourceURL:      sourceURL,
			AuthorNickname: firstNonEmpty(info.Channel, info.Uploader),
			AuthorAvatarURL: firstNonEmpty(
				info.ChannelAvatarURL,
				info.UploaderAvatarURL,
			),
			CoverURL: info.Thumbnail,
			Duration: info.Duration,
		}, info, contentdownload.ContentMetadataOf(probe.Content), contentdownload.ContentOutputOf(probe.Content)),
	}, nil
}

func playerResponseJSONFromProbe(probe *contentdownload.Probe, info *VideoInfo) json.RawMessage {
	if info != nil && len(info.InitialPlayerResponseJSON) > 0 {
		return info.InitialPlayerResponseJSON
	}
	if probe != nil && probe.Internal != nil {
		switch raw := probe.Internal["pagejson"].(type) {
		case json.RawMessage:
			return raw
		case []byte:
			return json.RawMessage(raw)
		}
	}
	return nil
}

func youtubeDownloadSpec(variant *contentdownload.Variant, webpageURL string, directURL string, headers map[string]string) contentdownload.DownloadSpec {
	headers = cloneStringMap(headers)
	if variant != nil && variant.ID != "cover" {
		headers["Range"] = "bytes=0-"
		return contentdownload.DownloadSpec{
			URL:         directURL,
			Method:      http.MethodGet,
			Protocol:    "http",
			Connections: 4,
			Headers:     headers,
		}
	}
	return contentdownload.DownloadSpec{
		URL:         directURL,
		Method:      http.MethodGet,
		Protocol:    "http",
		Connections: 4,
		Headers:     headers,
	}
}

func youtubeMergedDownloadSpec(videoID string, formatIDs []string, sources []contentdownload.MultiSourceSpec) contentdownload.DownloadSpec {
	body, _ := json.Marshal(sources)
	return contentdownload.DownloadSpec{
		URL:         "multi-http://youtube/" + url.PathEscape(videoID) + "?formats=" + url.QueryEscape(strings.Join(formatIDs, "+")),
		Method:      http.MethodGet,
		Protocol:    contentdownload.ProtocolMultiHTTP,
		Body:        body,
		Connections: len(sources),
	}
}

func isMergedVariant(variant *contentdownload.Variant) bool {
	if variant == nil {
		return false
	}
	if variant.ID == defaultMergedVariantID {
		return true
	}
	if variant.Metadata != nil {
		if value, ok := variant.Metadata["format_type"].(string); ok && value == "merged" {
			return true
		}
	}
	return strings.Contains(variant.Spec, "+")
}

func mergedFormatSources(info *VideoInfo, variant *contentdownload.Variant, headers map[string]string) ([]contentdownload.MultiSourceSpec, []VideoFormat, error) {
	formatIDs := mergedVariantFormatIDs(variant)
	if len(formatIDs) < 2 {
		return nil, nil, fmt.Errorf("youtube merged variant %q does not contain video and audio formats", variant.ID)
	}
	formats := make([]VideoFormat, 0, len(formatIDs))
	sources := make([]contentdownload.MultiSourceSpec, 0, len(formatIDs))
	for _, id := range formatIDs {
		format := info.FindFormat(id)
		if format == nil {
			return nil, nil, fmt.Errorf("youtube format %q not found", id)
		}
		if format.URL == "" || format.NeedsSignature {
			return nil, nil, fmt.Errorf("youtube format %q does not have a direct downloadable URL", id)
		}
		formats = append(formats, *format)
		sourceHeaders := cloneStringMap(headers)
		sourceHeaders["Range"] = "bytes=0-"
		sources = append(sources, contentdownload.MultiSourceSpec{
			ID:        format.ID,
			URL:       format.URL,
			Method:    http.MethodGet,
			Headers:   sourceHeaders,
			Ext:       firstNonEmpty(format.Ext, "bin"),
			Size:      format.ContentLength,
			ChunkSize: youtubeHTTPChunkSize,
			MimeType:  format.MimeType,
			HasAudio:  format.HasAudio,
			HasVideo:  format.HasVideo,
		})
	}
	return sources, formats, nil
}

func mergedVariantFormatIDs(variant *contentdownload.Variant) []string {
	if variant == nil {
		return nil
	}
	if variant.Metadata != nil {
		switch value := variant.Metadata["requested_format_ids"].(type) {
		case []string:
			return append([]string(nil), value...)
		case []any:
			out := make([]string, 0, len(value))
			for _, item := range value {
				if text, ok := item.(string); ok && text != "" {
					out = append(out, text)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	parts := strings.Split(variant.Spec, "+")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
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

func firstProgressiveVideoVariant(probe *contentdownload.Probe) *contentdownload.Variant {
	if probe == nil {
		return nil
	}
	for i := range probe.Variants {
		variant := &probe.Variants[i]
		if variant.Type != "video" || isMergedVariant(variant) {
			continue
		}
		if variant.Metadata != nil {
			if formatType, _ := variant.Metadata["format_type"].(string); formatType != "" && formatType != "progressive" {
				continue
			}
		}
		return variant
	}
	return nil
}

func ffmpegAvailableFromOptions(options contentdownload.Options) bool {
	if options.Extra == nil {
		return true
	}
	value, ok := options.Extra["ffmpeg_available"]
	if !ok {
		return true
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(typed)
		return err != nil || parsed
	default:
		return true
	}
}

func formatMetadata(format VideoFormat) map[string]any {
	return map[string]any{
		"format_id":       format.ID,
		"itag":            format.Itag,
		"ext":             format.Ext,
		"mime_type":       format.MimeType,
		"quality":         format.Quality,
		"quality_label":   format.QualityLabel,
		"audio_quality":   format.AudioQuality,
		"audio_codec":     format.AudioCodec,
		"video_codec":     format.VideoCodec,
		"has_audio":       format.HasAudio,
		"has_video":       format.HasVideo,
		"adaptive":        format.Adaptive,
		"protocol":        format.Protocol,
		"average_bitrate": format.AverageBitrate,
		"content_length":  format.ContentLength,
	}
}

func mergedFormatMetadata(video VideoFormat, audio VideoFormat, ext string) map[string]any {
	return map[string]any{
		"format_id":            video.ID + "+" + audio.ID,
		"format_type":          "merged",
		"requested_format_ids": []string{video.ID, audio.ID},
		"output_ext":           ext,
		"video_format":         formatMetadata(video),
		"audio_format":         formatMetadata(audio),
	}
}

func mergedFormatLabel(video VideoFormat, audio VideoFormat, best bool) string {
	parts := []string{"视频"}
	if best {
		parts[0] = "最佳视频"
	}
	if video.QualityLabel != "" {
		parts = append(parts, video.QualityLabel)
	} else if video.Height > 0 {
		parts = append(parts, fmt.Sprintf("%dp", video.Height))
	} else if video.Quality != "" {
		parts = append(parts, video.Quality)
	}
	if video.Ext != "" {
		parts = append(parts, strings.ToUpper(video.Ext))
	}
	audioPart := "音频"
	if audio.Ext != "" {
		audioPart += " " + strings.ToUpper(audio.Ext)
	}
	return strings.Join(parts, " ") + " + " + audioPart
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

func mergedOutputExt(video VideoFormat, audio VideoFormat) string {
	videoExt := strings.ToLower(video.Ext)
	audioExt := strings.ToLower(audio.Ext)
	switch {
	case videoExt == "mp4" && (audioExt == "m4a" || audioExt == "mp4") && mp4CompatibleAudioCodec(audio.AudioCodec):
		return "mp4"
	case videoExt == "webm" && audioExt == "webm":
		return "webm"
	default:
		return "mkv"
	}
}

func mp4CompatibleAudioCodec(codec string) bool {
	codec = strings.ToLower(codec)
	return codec == "" ||
		strings.HasPrefix(codec, "mp4a") ||
		strings.HasPrefix(codec, "ac-3") ||
		strings.HasPrefix(codec, "ec-3")
}

func formatScore(format VideoFormat) int {
	return format.Height*1_000_000 + format.FPS*10_000 + firstNonZeroInt(format.AverageBitrate, format.Bitrate)
}

func audioScore(format VideoFormat) int {
	return firstNonZeroInt(format.AverageBitrate, format.Bitrate) + int(format.ContentLength/1024)
}

func collectImageThumbnails(groups ...[]Thumbnail) []Thumbnail {
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
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Width*out[i].Height > out[j].Width*out[j].Height
	})
	return out
}

func collectThumbnails(videoID string, groups ...[]Thumbnail) []Thumbnail {
	out := collectImageThumbnails(groups...)
	seen := make(map[string]bool, len(out)+4)
	for _, thumb := range out {
		seen[thumb.URL] = true
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

func extractVisitorData(ytcfg map[string]any, responses ...rawPlayerResponse) string {
	if visitorData := stringFromMap(ytcfg, "VISITOR_DATA"); visitorData != "" {
		return visitorData
	}
	if contextValue, ok := ytcfg["INNERTUBE_CONTEXT"].(map[string]any); ok {
		if client, ok := contextValue["client"].(map[string]any); ok {
			if visitorData := stringFromMap(client, "visitorData"); visitorData != "" {
				return visitorData
			}
		}
	}
	for _, response := range responses {
		if response.ResponseContext.VisitorData != "" {
			return response.ResponseContext.VisitorData
		}
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
