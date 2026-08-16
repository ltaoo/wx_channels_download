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
	"github.com/rs/zerolog"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
)

const (
	default_base_url   = "https://www.youtube.com"
	default_user_agent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.5 Safari/605.1.15"
)

var (
	initial_player_response_re = regexp.MustCompile(`ytInitialPlayerResponse\s*=`)
	initial_data_re            = regexp.MustCompile(`ytInitialData\s*=`)
	ytcfg_set_re               = regexp.MustCompile(`ytcfg\.set\s*\(`)
	player_js_url_re           = regexp.MustCompile(`(?:"PLAYER_JS_URL"|"jsUrl")\s*:\s*"([^"]*/s/player/[^"]+base\.js)"`)
	sig_func_name_res          = []*regexp.Regexp{
		regexp.MustCompile(`(?m)([A-Za-z0-9_$]+)\s*=\s*function\(\s*a\s*\)\s*\{\s*a\s*=\s*a\.split\(\s*""\s*\)`),
		regexp.MustCompile(`(?m)function\s+([A-Za-z0-9_$]+)\(\s*a\s*\)\s*\{\s*a\s*=\s*a\.split\(\s*""\s*\)`),
	}
	n_func_name_res = []*regexp.Regexp{
		regexp.MustCompile(`\.get\(\s*"n"\s*\)\s*\)\s*&&\s*\([^=]+=\s*([A-Za-z0-9_$]+)(?:\[\d+\])?\(`),
		regexp.MustCompile(`([A-Za-z0-9_$]+)\s*=\s*function\(\s*\w+\s*\)\s*\{\s*var\s+\w+\s*=\s*\w+\.split\(`),
	}
	n_array_call_re        = regexp.MustCompile(`\.get\(\s*"n"\s*\)\s*\)\s*&&\s*\([^=]+=\s*([A-Za-z0-9_$]+)\[(\d+)\]\(`)
	signature_timestamp_re = regexp.MustCompile(`(?:signatureTimestamp|sts)\s*:\s*(\d{5})`)
)

type Client struct {
	HTTPClient   *http.Client
	BaseURL      string
	UserAgent    string
	Cookie       string
	PoToken      string
	CookieReader *cookies.Reader
	Cache        *cache.CacheProvider
	ForceRefresh bool
	Logger       *zerolog.Logger
}

type ClientOptions struct {
	HTTPClient   *http.Client
	BaseURL      string
	UserAgent    string
	Cookie       string
	PoToken      string
	CookieReader *cookies.Reader
	Cache        *cache.CacheProvider
	ForceRefresh bool
	Logger       *zerolog.Logger
}

type innertube_client struct {
	Name                 string
	HeaderID             string
	Host                 string
	UserAgent            string
	Context              map[string]any
	RequireJS            bool
	UseCookies           bool
	OmitAPIKey           bool
	UsePageClientVersion bool
	GVSRequiresPOT       bool
}

func default_innertube_clients() []innertube_client {
	return []innertube_client{
		{
			Name:      "web_embedded",
			HeaderID:  "56",
			Host:      "www.youtube.com",
			UserAgent: default_user_agent,
			Context: map[string]any{
				"client": map[string]any{
					"clientName":       "WEB_EMBEDDED_PLAYER",
					"clientVersion":    "2.20260708.00.00",
					"userAgent":        default_user_agent,
					"hl":               "en",
					"timeZone":         "UTC",
					"utcOffsetMinutes": 0,
				},
				"thirdParty": map[string]any{
					"embedUrl": "https://www.reddit.com/",
				},
			},
			RequireJS:            true,
			UseCookies:           true,
			UsePageClientVersion: true,
		},
		{
			Name:      "visionos",
			HeaderID:  "101",
			Host:      "www.youtube.com",
			UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_7_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15",
			Context: map[string]any{
				"client": map[string]any{
					"clientName":       "VISIONOS",
					"clientVersion":    "1.02",
					"deviceMake":       "Apple",
					"deviceModel":      "RealityDevice17,1",
					"userAgent":        "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_7_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15",
					"osName":           "visionOS",
					"osVersion":        "26.5.23O471",
					"hl":               "en",
					"timeZone":         "UTC",
					"utcOffsetMinutes": 0,
				},
			},
			RequireJS: false,
		},
		{
			Name:      "tv",
			HeaderID:  "7",
			Host:      "www.youtube.com",
			UserAgent: "Mozilla/5.0 (ChromiumStylePlatform) Cobalt/25.lts.30.1034943-gold (unlike Gecko), Unknown_TV_Unknown_0/Unknown (Unknown, Unknown)",
			Context: map[string]any{
				"client": map[string]any{
					"clientName":       "TVHTML5",
					"clientVersion":    "7.20260707.07.00",
					"userAgent":        "Mozilla/5.0 (ChromiumStylePlatform) Cobalt/25.lts.30.1034943-gold (unlike Gecko), Unknown_TV_Unknown_0/Unknown (Unknown, Unknown)",
					"hl":               "en",
					"timeZone":         "UTC",
					"utcOffsetMinutes": 0,
				},
			},
			RequireJS:  true,
			UseCookies: true,
		},
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
			RequireJS:      false,
			OmitAPIKey:     true,
			GVSRequiresPOT: true,
		},
		{
			Name:      "web_safari",
			HeaderID:  "1",
			Host:      "www.youtube.com",
			UserAgent: default_user_agent + ",gzip(gfe)",
			Context: map[string]any{
				"client": map[string]any{
					"clientName":       "WEB",
					"clientVersion":    "2.20260114.08.00",
					"userAgent":        default_user_agent + ",gzip(gfe)",
					"hl":               "en",
					"timeZone":         "UTC",
					"utcOffsetMinutes": 0,
				},
			},
			RequireJS:            true,
			UseCookies:           true,
			UsePageClientVersion: true,
			GVSRequiresPOT:       true,
		},
	}
}

type VideoInfo struct {
	ID                          string                       `json:"id"`
	Title                       string                       `json:"title,omitempty"`
	Description                 string                       `json:"description,omitempty"`
	WebpageURL                  string                       `json:"webpage_url,omitempty"`
	Thumbnail                   string                       `json:"thumbnail,omitempty"`
	Thumbnails                  []Thumbnail                  `json:"thumbnails,omitempty"`
	Duration                    int64                        `json:"duration,omitempty"`
	ViewCount                   int64                        `json:"view_count,omitempty"`
	AgeLimit                    int                          `json:"age_limit,omitempty"`
	Channel                     string                       `json:"channel,omitempty"`
	ChannelID                   string                       `json:"channel_id,omitempty"`
	ChannelURL                  string                       `json:"channel_url,omitempty"`
	ChannelAvatarURL            string                       `json:"channel_avatar_url,omitempty"`
	Uploader                    string                       `json:"uploader,omitempty"`
	UploaderID                  string                       `json:"uploader_id,omitempty"`
	UploaderURL                 string                       `json:"uploader_url,omitempty"`
	UploaderAvatarURL           string                       `json:"uploader_avatar_url,omitempty"`
	Categories                  []string                     `json:"categories,omitempty"`
	Tags                        []string                     `json:"tags,omitempty"`
	PublishDate                 string                       `json:"publish_date,omitempty"`
	UploadDate                  string                       `json:"upload_date,omitempty"`
	LiveStatus                  string                       `json:"live_status,omitempty"`
	MediaType                   string                       `json:"media_type,omitempty"`
	PlayableInEmbed             bool                         `json:"playable_in_embed,omitempty"`
	Formats                     []VideoFormat                `json:"formats,omitempty"`
	CaptionTracks               []CaptionTrack               `json:"caption_tracks,omitempty"`
	CaptionAudioTracks          []CaptionAudioTrack          `json:"caption_audio_tracks,omitempty"`
	CaptionTranslationLanguages []CaptionTranslationLanguage `json:"caption_translation_languages,omitempty"`
	PlayabilityStatus           PlayabilityStatus            `json:"playability_status,omitempty"`
	Warnings                    []string                     `json:"warnings,omitempty"`
	InitialPlayerResponseJSON   json.RawMessage              `json:"-"`
	YTCfgJSON                   json.RawMessage              `json:"-"`
	PageHTML                    string                       `json:"-"`
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

// CaptionTrack describes one subtitle/caption track advertised by YouTube's
// player response. BaseURL is a signed timedtext endpoint.
type CaptionTrack struct {
	BaseURL        string `json:"base_url"`
	Name           string `json:"name,omitempty"`
	VssID          string `json:"vss_id,omitempty"`
	LanguageCode   string `json:"language_code,omitempty"`
	Kind           string `json:"kind,omitempty"`
	IsDefault      bool   `json:"is_default,omitempty"`
	IsTranslatable bool   `json:"is_translatable,omitempty"`
}

// CaptionAudioTrack connects an audio track to its compatible caption tracks.
type CaptionAudioTrack struct {
	ID                       string `json:"id,omitempty"`
	DisplayName              string `json:"display_name,omitempty"`
	CaptionTrackIndices      []int  `json:"caption_track_indices,omitempty"`
	DefaultCaptionTrackIndex int    `json:"default_caption_track_index,omitempty"`
	IsDefault                bool   `json:"is_default,omitempty"`
}

// CaptionTranslationLanguage is one language supported by YouTube's timedtext
// translation endpoint for the source caption tracks.
type CaptionTranslationLanguage struct {
	LanguageCode string `json:"language_code"`
	Name         string `json:"name,omitempty"`
}

type VideoFormat struct {
	ID               string `json:"id"`
	Itag             int    `json:"itag,omitempty"`
	URL              string `json:"url,omitempty"`
	MimeType         string `json:"mime_type,omitempty"`
	Ext              string `json:"ext,omitempty"`
	Quality          string `json:"quality,omitempty"`
	QualityLabel     string `json:"quality_label,omitempty"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
	FPS              int    `json:"fps,omitempty"`
	Bitrate          int    `json:"bitrate,omitempty"`
	AverageBitrate   int    `json:"average_bitrate,omitempty"`
	ContentLength    int64  `json:"content_length,omitempty"`
	AudioQuality     string `json:"audio_quality,omitempty"`
	AudioSampleRate  int    `json:"audio_sample_rate,omitempty"`
	AudioChannels    int    `json:"audio_channels,omitempty"`
	AudioCodec       string `json:"audio_codec,omitempty"`
	VideoCodec       string `json:"video_codec,omitempty"`
	HasAudio         bool   `json:"has_audio,omitempty"`
	HasVideo         bool   `json:"has_video,omitempty"`
	Adaptive         bool   `json:"adaptive,omitempty"`
	Protocol         string `json:"protocol,omitempty"`
	NeedsSignature   bool   `json:"needs_signature,omitempty"`
	HasDRM           bool   `json:"has_drm,omitempty"`
	SourceClient     string `json:"source_client,omitempty"`
	SourceClientID   string `json:"source_client_id,omitempty"`
	SourceVersion    string `json:"source_client_version,omitempty"`
	SourceUserAgent  string `json:"source_user_agent,omitempty"`
	SourceCookies    bool   `json:"source_supports_cookies,omitempty"`
	RequiresPOT      bool   `json:"requires_pot,omitempty"`
	HadNChallenge    bool   `json:"had_n_challenge,omitempty"`
	SolvedNChallenge bool   `json:"solved_n_challenge,omitempty"`
}

func NewClient(cookie string) *Client {
	return NewClientWithOptions(ClientOptions{Cookie: cookie})
}

func NewClientWithOptions(options ClientOptions) *Client {
	http_client := options.HTTPClient
	if http_client == nil {
		http_client = &http.Client{Timeout: 30 * time.Second}
	}
	base_url := strings.TrimSpace(options.BaseURL)
	if base_url == "" {
		base_url = default_base_url
	}
	user_agent := strings.TrimSpace(options.UserAgent)
	if user_agent == "" {
		user_agent = default_user_agent
	}
	var logger *zerolog.Logger
	if options.Logger != nil {
		component_logger := options.Logger.With().Str("component", "youtube_scraper").Logger()
		logger = &component_logger
	}
	return &Client{
		HTTPClient:   http_client,
		BaseURL:      base_url,
		UserAgent:    user_agent,
		Cookie:       options.Cookie,
		PoToken:      options.PoToken,
		CookieReader: options.CookieReader,
		Cache:        options.Cache,
		ForceRefresh: options.ForceRefresh,
		Logger:       logger,
	}
}

func (c *Client) Fetch(raw_url string) (any, error) {
	return c.FetchContext(context.Background(), raw_url)
}

func (c *Client) FetchContext(ctx context.Context, raw_url string) (*VideoInfo, error) {
	return c.Extract(ctx, raw_url)
}

func (c *Client) Extract(ctx context.Context, raw_url string) (*VideoInfo, error) {
	started := time.Now()
	video_id, ok := ExtractVideoID(raw_url)
	if !ok {
		return nil, fmt.Errorf("unsupported youtube URL: %s", raw_url)
	}
	if c != nil && c.Logger != nil {
		c.Logger.Info().
			Str("video_id", video_id).
			Bool("force_refresh", c.ForceRefresh).
			Bool("configured_cookie", strings.TrimSpace(c.Cookie) != "").
			Bool("cookie_reader_available", c.CookieReader != nil).
			Bool("configured_po_token", strings.TrimSpace(c.PoToken) != "").
			Msg("youtube extraction started")
	}
	watch_url := canonical_video_url(video_id)
	webpage, err := c.fetch_watch_webpage(ctx, video_id)
	if err != nil {
		return nil, err
	}

	player_response, has_player_response, err := parse_initial_player_response(webpage)
	if err != nil {
		return nil, fmt.Errorf("parse youtube player response: %w", err)
	}
	initial_player_response_json, _, _ := ExtractInitialPlayerResponseJSON(webpage)
	initial_data_owner, _, _ := parse_initial_data_owner(webpage)
	ytcfg_json, _, _ := ExtractYTCfgJSON(webpage)
	ytcfg, _ := parse_yt_cfg(webpage)
	if has_player_response {
		page_client := innertube_client{
			Name:           "web",
			HeaderID:       "1",
			UserAgent:      c.user_agent(),
			UseCookies:     true,
			GVSRequiresPOT: true,
		}
		set_player_response_format_source(&player_response, page_client, page_client_version(ytcfg))
	}
	player := c.new_player_resolver(ctx, webpage, ytcfg)
	player_responses := make([]raw_player_response, 0, 4)
	if has_player_response {
		player_responses = append(player_responses, player_response)
	}
	if api_key := string_from_map(ytcfg, "INNERTUBE_API_KEY"); api_key != "" {
		api_success := false
		if has_player_response {
			player_responses = []raw_player_response{player_response_without_streaming_data(player_response)}
		}
		visitor_data := extract_visitor_data(ytcfg, player_response)
		for _, client := range c.requested_innertube_clients() {
			if client.RequireJS {
				if _, err := player.player_code(); err != nil {
					player.warn(fmt.Sprintf("youtube %s player API skipped: player JS unavailable: %v", client.Name, err))
					continue
				}
			}
			api_response, api_err := c.fetch_player_api_for_client(ctx, video_id, ytcfg, watch_url, client, player.signature_timestamp(), visitor_data)
			if api_err != nil {
				player.warn(fmt.Sprintf("youtube %s player API failed: %v", client.Name, api_err))
				continue
			}
			if pr_id := api_response.VideoDetails.VideoID; pr_id != "" && pr_id != video_id {
				player.warn(fmt.Sprintf("youtube %s player response video id %q does not match requested id %q", client.Name, pr_id, video_id))
				continue
			}
			player_responses = append(player_responses, api_response)
			api_success = true
		}
		if !api_success && has_player_response {
			player_responses = []raw_player_response{player_response}
		}
	}
	if len(player_responses) > 0 {
		player_response = merge_player_response_list(player_responses)
		has_player_response = true
	}
	if !has_player_response || !player_response.has_streaming_data() {
		api_response, api_err := c.fetch_player_api(ctx, video_id, ytcfg, watch_url)
		if api_err == nil {
			player_response = merge_player_responses(player_response, api_response)
			has_player_response = true
		} else if !has_player_response {
			return nil, api_err
		}
	}
	if !has_player_response {
		return nil, fmt.Errorf("youtube player response not found")
	}

	info := build_video_info(video_id, watch_url, player_response, initial_data_owner, player)
	info.InitialPlayerResponseJSON = initial_player_response_json
	info.YTCfgJSON = ytcfg_json
	info.PageHTML = string(webpage)
	info.Warnings = append(info.Warnings, player.warnings...)
	if len(info.InitialPlayerResponseJSON) == 0 {
		if raw, err := json.Marshal(player_response); err == nil {
			info.InitialPlayerResponseJSON = raw
		}
	}
	if info.ID != video_id {
		return nil, fmt.Errorf("youtube player response video id %q does not match requested id %q", info.ID, video_id)
	}
	if info.PlayabilityStatus.Status != "" && info.PlayabilityStatus.Status != "OK" && len(info.Formats) == 0 {
		return nil, fmt.Errorf("youtube video unavailable: %s", first_non_empty(info.PlayabilityStatus.Reason, info.PlayabilityStatus.Status))
	}
	if len(info.Formats) == 0 {
		info.Warnings = append(info.Warnings, "未提取到可直接下载的 YouTube 格式")
	}
	c.log_extraction_result(info, time.Since(started))
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

func (c *Client) fetch_watch_webpage(ctx context.Context, video_id string) ([]byte, error) {
	request_url := c.watch_url(video_id)
	cache_url := canonical_video_url(video_id)
	if c != nil && !c.ForceRefresh {
		cached_webpage, cached, err := c.read_cached_html(cache_url)
		if err != nil {
			return nil, fmt.Errorf("read cached youtube watch HTML response for %q: %w", cache_url, err)
		}
		if cached {
			if _, ok, parse_err := ExtractYTCfgJSON(cached_webpage); parse_err == nil && ok {
				return cached_webpage, nil
			}
			_ = c.remove_cached_html(cache_url)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request_url, nil)
	if err != nil {
		return nil, err
	}
	c.set_default_headers(req, "")
	resp, err := c.http_client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("youtube watch request failed: %s", resp.Status)
	}
	webpage, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if _, ok, parse_err := ExtractYTCfgJSON(webpage); parse_err == nil && ok {
		if err := c.write_cached_html(cache_url, webpage); err != nil {
			return nil, fmt.Errorf("cache youtube watch HTML response for %q: %w", cache_url, err)
		}
	}
	return webpage, nil
}

func (c *Client) fetch_player_api(ctx context.Context, video_id string, ytcfg map[string]any, referer string) (raw_player_response, error) {
	api_key := string_from_map(ytcfg, "INNERTUBE_API_KEY")
	if api_key == "" {
		return raw_player_response{}, fmt.Errorf("youtube player response not found and INNERTUBE_API_KEY is missing")
	}
	context_value, _ := ytcfg["INNERTUBE_CONTEXT"].(map[string]any)
	if context_value == nil {
		context_value = default_innertube_context()
	}
	body, err := json.Marshal(map[string]any{
		"context": context_value,
		"videoId": video_id,
	})
	if err != nil {
		return raw_player_response{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.player_api_url(api_key), bytes.NewReader(body))
	if err != nil {
		return raw_player_response{}, err
	}
	c.set_default_headers(req, referer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-YouTube-Client-Name", "1")
	if client, ok := context_value["client"].(map[string]any); ok {
		if version := string_from_map(client, "clientVersion"); version != "" {
			req.Header.Set("X-YouTube-Client-Version", version)
		}
	}

	resp, err := c.http_client().Do(req)
	if err != nil {
		return raw_player_response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return raw_player_response{}, fmt.Errorf("youtube player API request failed: %s", resp.Status)
	}
	var out raw_player_response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return raw_player_response{}, err
	}
	set_player_response_format_source(&out, innertube_client{
		Name:           "web",
		HeaderID:       "1",
		UserAgent:      c.user_agent(),
		UseCookies:     true,
		GVSRequiresPOT: true,
	}, context_client_version(context_value))
	return out, nil
}

func (c *Client) fetch_player_api_for_client(ctx context.Context, video_id string, ytcfg map[string]any, referer string, client innertube_client, sts string, visitor_data string) (raw_player_response, error) {
	api_key := string_from_map(ytcfg, "INNERTUBE_API_KEY")
	if api_key == "" && !client.OmitAPIKey {
		return raw_player_response{}, fmt.Errorf("INNERTUBE_API_KEY is missing")
	}
	if client.OmitAPIKey {
		api_key = ""
	}
	context_value := clone_map(client.Context)
	if context_value == nil {
		context_value = default_innertube_context()
	}
	if client.UsePageClientVersion {
		if version := page_client_version(ytcfg); version != "" {
			if client_map, ok := context_value["client"].(map[string]any); ok {
				client_map["clientVersion"] = version
			}
		}
	}
	query := map[string]any{
		"context":        context_value,
		"videoId":        video_id,
		"contentCheckOk": true,
		"racyCheckOk":    true,
		"playbackContext": map[string]any{
			"contentPlaybackContext": map[string]any{
				"html5Preference": "HTML5_PREF_WANTS",
			},
		},
	}
	if sts != "" {
		if playback_context, ok := query["playbackContext"].(map[string]any); ok {
			if content_playback_context, ok := playback_context["contentPlaybackContext"].(map[string]any); ok {
				content_playback_context["signatureTimestamp"] = sts
			}
		}
	}
	body, err := json.Marshal(query)
	if err != nil {
		return raw_player_response{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.player_api_url(api_key), bytes.NewReader(body))
	if err != nil {
		return raw_player_response{}, err
	}
	c.set_default_headers(req, referer)
	if client.UserAgent != "" {
		req.Header.Set("User-Agent", client.UserAgent)
	}
	if !client.UseCookies {
		req.Header.Del("Cookie")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-YouTube-Client-Name", client.HeaderID)
	if visitor_data != "" {
		req.Header.Set("X-Goog-Visitor-Id", visitor_data)
	}
	if client_map, ok := context_value["client"].(map[string]any); ok {
		if version := string_from_map(client_map, "clientVersion"); version != "" {
			req.Header.Set("X-YouTube-Client-Version", version)
		}
	}

	resp, err := c.http_client().Do(req)
	if err != nil {
		return raw_player_response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return raw_player_response{}, fmt.Errorf("youtube player API request failed: %s", resp.Status)
	}
	var out raw_player_response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return raw_player_response{}, err
	}
	set_player_response_format_source(&out, client, context_client_version(context_value))
	return out, nil
}

func (c *Client) requested_innertube_clients() []innertube_client {
	return default_innertube_clients()
}

func (c *Client) download_headers(referer string) map[string]string {
	cookie_url := strings.TrimSpace(referer)
	if cookie_url == "" {
		cookie_url = c.base_url()
	}
	headers := map[string]string{
		"User-Agent":      c.user_agent(),
		"Accept":          "*/*",
		"Accept-Encoding": "identity",
		"Accept-Language": "en-US,en;q=0.9",
		"Referer":         referer,
		"Sec-Fetch-Dest":  "video",
		"Sec-Fetch-Mode":  "no-cors",
		"Sec-Fetch-Site":  "cross-site",
	}
	if cookie := c.cookie(cookie_url); cookie != "" {
		headers["Cookie"] = cookie
	}
	return headers
}

func (c *Client) DownloadHeaders(referer string) map[string]string {
	return c.download_headers(referer)
}

// DownloadHeadersForFormat returns the headers for the Innertube client that
// produced a format URL. YouTube may reject a GVS request when an Android URL
// is replayed with web cookies/headers (or vice versa).
func (c *Client) DownloadHeadersForFormat(format VideoFormat, referer string) map[string]string {
	headers := c.download_headers(referer)
	if user_agent := strings.TrimSpace(format.SourceUserAgent); user_agent != "" {
		headers["User-Agent"] = user_agent
	}
	if client_id := strings.TrimSpace(format.SourceClientID); client_id != "" {
		headers["X-YouTube-Client-Name"] = client_id
	}
	if version := strings.TrimSpace(format.SourceVersion); version != "" {
		headers["X-YouTube-Client-Version"] = version
	}
	if format.SourceClient != "" && !format.SourceCookies {
		delete(headers, "Cookie")
	}
	if strings.HasPrefix(strings.ToLower(format.SourceClient), "android") {
		delete(headers, "Referer")
		delete(headers, "Sec-Fetch-Dest")
		delete(headers, "Sec-Fetch-Mode")
		delete(headers, "Sec-Fetch-Site")
	}
	return headers
}

func (c *Client) set_default_headers(req *http.Request, referer string) {
	req.Header.Set("User-Agent", c.user_agent())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if cookie := c.cookie(req.URL.String()); cookie != "" {
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

func (c *Client) http_client() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return NewClient("").HTTPClient
}

func (c *Client) user_agent() string {
	if c != nil && strings.TrimSpace(c.UserAgent) != "" {
		return c.UserAgent
	}
	return default_user_agent
}

func (c *Client) cookie(raw_url string) string {
	if c != nil && c.CookieReader != nil {
		if cookie_value, err := c.CookieReader.HeaderForURL(raw_url); err == nil {
			if cookie_value = strings.TrimSpace(cookie_value); cookie_value != "" {
				return cookie_value
			}
		}
	}
	if c != nil {
		return strings.TrimSpace(c.Cookie)
	}
	return ""
}

type po_token_spec struct {
	client  string
	context string
	token   string
}

func (c *Client) po_token_for(client_name string, token_context string) string {
	if c == nil {
		return ""
	}
	client_name = strings.ToLower(strings.TrimSpace(client_name))
	token_context = strings.ToLower(strings.TrimSpace(token_context))
	var fallback string
	for _, spec := range parse_po_token_specs(c.PoToken) {
		if spec.context != token_context {
			continue
		}
		if spec.client == client_name && client_name != "" {
			return spec.token
		}
		if spec.client == "" && fallback == "" {
			fallback = spec.token
		}
	}
	return fallback
}

func parse_po_token_specs(raw_value string) []po_token_spec {
	parts := strings.Split(strings.TrimSpace(raw_value), ",")
	specs := make([]po_token_spec, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		spec := po_token_spec{context: "gvs"}
		token := part
		if meta, value, ok := strings.Cut(part, "+"); ok {
			meta = strings.ToLower(strings.TrimSpace(meta))
			token = value
			if client, context_name, has_context := strings.Cut(meta, "."); has_context {
				spec.client = strings.TrimSpace(client)
				spec.context = strings.TrimSpace(context_name)
			} else if meta == "gvs" || meta == "player" || meta == "subs" {
				spec.context = meta
			} else {
				// A PO Token is base64url in normal use and therefore does not
				// contain '+'. Preserve unusual legacy bare values rather than
				// interpreting an unknown prefix as client metadata.
				token = part
			}
		}
		spec.token = sanitize_po_token(token)
		if spec.token != "" && (spec.context == "gvs" || spec.context == "player" || spec.context == "subs") {
			specs = append(specs, spec)
		}
	}
	return specs
}

func sanitize_po_token(token string) string {
	token, _ = url.QueryUnescape(token)
	for _, sep := range []string{"?", "&", "#"} {
		if i := strings.Index(token, sep); i >= 0 {
			token = token[:i]
		}
	}
	return strings.TrimSpace(token)
}

func (c *Client) base_url() string {
	if c != nil && strings.TrimSpace(c.BaseURL) != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return default_base_url
}

func (c *Client) watch_url(video_id string) string {
	base, _ := url.Parse(c.base_url())
	base.Path = "/watch"
	base.RawQuery = url.Values{"v": {video_id}}.Encode()
	return base.String()
}

func (c *Client) player_api_url(api_key string) string {
	base, _ := url.Parse(c.base_url())
	base.Path = "/youtubei/v1/player"
	query := url.Values{"prettyPrint": {"false"}}
	if api_key != "" {
		query.Set("key", api_key)
	}
	base.RawQuery = query.Encode()
	return base.String()
}

func canonical_video_url(video_id string) string {
	return default_base_url + "/watch?v=" + url.QueryEscape(video_id)
}

func parse_initial_player_response(webpage []byte) (raw_player_response, bool, error) {
	raw_json, ok, err := ExtractInitialPlayerResponseJSON(webpage)
	if err != nil || !ok {
		return raw_player_response{}, ok, err
	}
	var out raw_player_response
	if err := json.Unmarshal(raw_json, &out); err != nil {
		return raw_player_response{}, true, err
	}
	return out, true, nil
}

func ExtractInitialPlayerResponseJSON(webpage []byte) (json.RawMessage, bool, error) {
	raw_json, ok, err := extract_json_by_regexp(webpage, initial_player_response_re)
	if err != nil || !ok {
		return nil, ok, err
	}
	return json.RawMessage(append([]byte(nil), raw_json...)), true, nil
}

func ExtractInitialDataJSON(webpage []byte) (json.RawMessage, bool, error) {
	raw_json, ok, err := extract_json_by_regexp(webpage, initial_data_re)
	if err != nil || !ok {
		return nil, ok, err
	}
	if !json.Valid(raw_json) {
		return nil, true, fmt.Errorf("invalid youtube initial data json")
	}
	return json.RawMessage(append([]byte(nil), raw_json...)), true, nil
}

func parse_initial_data_owner(webpage []byte) (raw_initial_data_owner, bool, error) {
	raw_json, ok, err := ExtractInitialDataJSON(webpage)
	if err != nil || !ok {
		return raw_initial_data_owner{}, ok, err
	}
	var data raw_initial_data
	if err := json.Unmarshal(raw_json, &data); err != nil {
		return raw_initial_data_owner{}, true, err
	}
	owner := owner_from_initial_data(data)
	return owner, owner.has_any(), nil
}

func owner_from_initial_data(data raw_initial_data) raw_initial_data_owner {
	for _, content := range data.Contents.TwoColumnWatchNextResults.Results.Results.Contents {
		owner := owner_from_video_owner_renderer(content.VideoSecondaryInfoRenderer.Owner.VideoOwnerRenderer)
		if owner.has_any() {
			return owner
		}
	}
	return raw_initial_data_owner{}
}

func owner_from_video_owner_renderer(renderer raw_video_owner_renderer) raw_initial_data_owner {
	endpoint := renderer.NavigationEndpoint
	if endpoint.empty() {
		for _, run := range renderer.Title.Runs {
			if !run.NavigationEndpoint.empty() {
				endpoint = run.NavigationEndpoint
				break
			}
		}
	}
	channel_url := first_non_empty(
		endpoint.CommandMetadata.WebCommandMetadata.URL,
		endpoint.BrowseEndpoint.CanonicalBaseURL,
	)
	return raw_initial_data_owner{
		Channel:          renderer.Title.text(),
		ChannelID:        endpoint.BrowseEndpoint.BrowseID,
		ChannelURL:       absolute_youtube_url(channel_url),
		ChannelAvatarURL: best_thumbnail(collect_image_thumbnails(renderer.Thumbnail.Thumbnails)),
	}
}

func (o raw_initial_data_owner) has_any() bool {
	return o.Channel != "" || o.ChannelID != "" || o.ChannelURL != "" || o.ChannelAvatarURL != ""
}

func (e raw_navigation_endpoint) empty() bool {
	return e.CommandMetadata.WebCommandMetadata.URL == "" &&
		e.BrowseEndpoint.BrowseID == "" &&
		e.BrowseEndpoint.CanonicalBaseURL == ""
}

func parse_yt_cfg(webpage []byte) (map[string]any, bool) {
	raw_json, ok, err := ExtractYTCfgJSON(webpage)
	if err != nil || !ok {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal(raw_json, &out); err != nil {
		return nil, false
	}
	return out, true
}

func ExtractYTCfgJSON(webpage []byte) (json.RawMessage, bool, error) {
	matches := ytcfg_set_re.FindAllIndex(webpage, -1)
	var found_invalid bool
	var last_err error
	for _, loc := range matches {
		raw_json, err := extract_json_value(webpage[loc[1]:])
		if err != nil {
			last_err = err
			continue
		}
		if !json.Valid(raw_json) {
			found_invalid = true
			last_err = fmt.Errorf("invalid youtube ytcfg json")
			continue
		}
		return json.RawMessage(append([]byte(nil), raw_json...)), true, nil
	}
	if found_invalid {
		return nil, true, last_err
	}
	if last_err != nil {
		return nil, true, last_err
	}
	return nil, false, nil
}

func extract_json_by_regexp(data []byte, re *regexp.Regexp) ([]byte, bool, error) {
	loc := re.FindIndex(data)
	if loc == nil {
		return nil, false, nil
	}
	raw, err := extract_json_value(data[loc[1]:])
	if err != nil {
		return nil, true, err
	}
	return raw, true, nil
}

func extract_json_value(data []byte) ([]byte, error) {
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
	in_string := false
	escaped := false
	for i := start; i < len(data); i++ {
		b := data[i]
		if in_string {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				in_string = false
			}
			continue
		}
		switch b {
		case '"':
			in_string = true
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

type raw_player_response struct {
	PlayabilityStatus raw_playability_status `json:"playabilityStatus"`
	VideoDetails      raw_video_details      `json:"videoDetails"`
	StreamingData     raw_streaming_data     `json:"streamingData"`
	ResponseContext   raw_response_context   `json:"responseContext"`
	Captions          raw_captions           `json:"captions"`
	Microformat       struct {
		Player raw_microformat `json:"playerMicroformatRenderer"`
	} `json:"microformat"`
}

type raw_captions struct {
	Player struct {
		CaptionTracks          []raw_caption_track        `json:"captionTracks"`
		AudioTracks            []raw_caption_audio_track  `json:"audioTracks"`
		TranslationLanguages   []raw_translation_language `json:"translationLanguages"`
		DefaultAudioTrackIndex int                        `json:"defaultAudioTrackIndex"`
	} `json:"playerCaptionsTracklistRenderer"`
}

type raw_caption_track struct {
	BaseURL        string        `json:"baseUrl"`
	Name           text_renderer `json:"name"`
	VssID          string        `json:"vssId"`
	LanguageCode   string        `json:"languageCode"`
	Kind           string        `json:"kind"`
	IsTranslatable bool          `json:"isTranslatable"`
}

type raw_caption_audio_track struct {
	CaptionTrackIndices      []int  `json:"captionTrackIndices"`
	DefaultCaptionTrackIndex int    `json:"defaultCaptionTrackIndex"`
	AudioTrackID             string `json:"audioTrackId"`
	DisplayName              string `json:"displayName"`
	HasDefaultTrack          bool   `json:"hasDefaultTrack"`
}

type raw_translation_language struct {
	LanguageCode string        `json:"languageCode"`
	LanguageName text_renderer `json:"languageName"`
}

type raw_playability_status struct {
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	PlayableInEmbed bool   `json:"playableInEmbed"`
}

type raw_response_context struct {
	VisitorData               string `json:"visitorData"`
	MainAppWebResponseContext struct {
		DatasyncID string `json:"datasyncId"`
	} `json:"mainAppWebResponseContext"`
}

type raw_video_details struct {
	VideoID          string        `json:"videoId"`
	Title            string        `json:"title"`
	ShortDescription string        `json:"shortDescription"`
	LengthSeconds    string        `json:"lengthSeconds"`
	ChannelID        string        `json:"channelId"`
	Author           string        `json:"author"`
	ViewCount        string        `json:"viewCount"`
	Keywords         []string      `json:"keywords"`
	Thumbnail        raw_thumbnail `json:"thumbnail"`
	IsLiveContent    bool          `json:"isLiveContent"`
}

type raw_microformat struct {
	Title             text_renderer `json:"title"`
	Description       text_renderer `json:"description"`
	LengthSeconds     string        `json:"lengthSeconds"`
	OwnerChannelName  string        `json:"ownerChannelName"`
	OwnerProfileURL   string        `json:"ownerProfileUrl"`
	ExternalChannelID string        `json:"externalChannelId"`
	ViewCount         string        `json:"viewCount"`
	Category          string        `json:"category"`
	PublishDate       string        `json:"publishDate"`
	UploadDate        string        `json:"uploadDate"`
	IsFamilySafe      *bool         `json:"isFamilySafe"`
	IsShortsEligible  bool          `json:"isShortsEligible"`
	Thumbnail         raw_thumbnail `json:"thumbnail"`
}

type text_renderer struct {
	SimpleText string `json:"simpleText"`
	Runs       []struct {
		Text               string                  `json:"text"`
		NavigationEndpoint raw_navigation_endpoint `json:"navigationEndpoint"`
	} `json:"runs"`
}

type raw_thumbnail struct {
	Thumbnails []Thumbnail `json:"thumbnails"`
}

type raw_initial_data struct {
	Contents struct {
		TwoColumnWatchNextResults struct {
			Results struct {
				Results struct {
					Contents []raw_watch_next_content `json:"contents"`
				} `json:"results"`
			} `json:"results"`
		} `json:"twoColumnWatchNextResults"`
	} `json:"contents"`
}

type raw_watch_next_content struct {
	VideoSecondaryInfoRenderer struct {
		Owner struct {
			VideoOwnerRenderer raw_video_owner_renderer `json:"videoOwnerRenderer"`
		} `json:"owner"`
	} `json:"videoSecondaryInfoRenderer"`
}

type raw_video_owner_renderer struct {
	Thumbnail          raw_thumbnail           `json:"thumbnail"`
	Title              text_renderer           `json:"title"`
	NavigationEndpoint raw_navigation_endpoint `json:"navigationEndpoint"`
}

type raw_navigation_endpoint struct {
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

type raw_initial_data_owner struct {
	Channel          string
	ChannelID        string
	ChannelURL       string
	ChannelAvatarURL string
}

type raw_streaming_data struct {
	Formats          []raw_format `json:"formats"`
	AdaptiveFormats  []raw_format `json:"adaptiveFormats"`
	HLSManifestURL   string       `json:"hlsManifestUrl"`
	DASHManifestURL  string       `json:"dashManifestUrl"`
	ExpiresInSeconds string       `json:"expiresInSeconds"`
}

type raw_format struct {
	Itag              int      `json:"itag"`
	URL               string   `json:"url"`
	SignatureCipher   string   `json:"signatureCipher"`
	Cipher            string   `json:"cipher"`
	MimeType          string   `json:"mimeType"`
	Bitrate           int      `json:"bitrate"`
	AverageBitrate    int      `json:"averageBitrate"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	FPS               int      `json:"fps"`
	Quality           string   `json:"quality"`
	QualityLabel      string   `json:"qualityLabel"`
	AudioQuality      string   `json:"audioQuality"`
	AudioSampleRate   string   `json:"audioSampleRate"`
	AudioChannels     int      `json:"audioChannels"`
	ContentLength     string   `json:"contentLength"`
	ApproxDurationMS  string   `json:"approxDurationMs"`
	DRMFamilies       []string `json:"drmFamilies"`
	Type              string   `json:"type"`
	SourceClient      string   `json:"-"`
	SourceClientID    string   `json:"-"`
	SourceVersion     string   `json:"-"`
	SourceUserAgent   string   `json:"-"`
	SourceCookies     bool     `json:"-"`
	SourceRequiresPOT bool     `json:"-"`
}

func page_client_version(ytcfg map[string]any) string {
	if version := string_from_map(ytcfg, "INNERTUBE_CLIENT_VERSION"); version != "" {
		return version
	}
	context_value, _ := ytcfg["INNERTUBE_CONTEXT"].(map[string]any)
	return context_client_version(context_value)
}

func context_client_version(context_value map[string]any) string {
	if client_map, ok := context_value["client"].(map[string]any); ok {
		return string_from_map(client_map, "clientVersion")
	}
	return ""
}

func set_player_response_format_source(response *raw_player_response, client innertube_client, version string) {
	if response == nil {
		return
	}
	set_source := func(formats []raw_format) {
		for index := range formats {
			formats[index].SourceClient = client.Name
			formats[index].SourceClientID = client.HeaderID
			formats[index].SourceVersion = version
			formats[index].SourceUserAgent = client.UserAgent
			formats[index].SourceCookies = client.UseCookies
			formats[index].SourceRequiresPOT = client.GVSRequiresPOT
		}
	}
	set_source(response.StreamingData.Formats)
	set_source(response.StreamingData.AdaptiveFormats)
}

func (r raw_player_response) has_streaming_data() bool {
	return len(r.StreamingData.Formats) > 0 ||
		len(r.StreamingData.AdaptiveFormats) > 0 ||
		r.StreamingData.HLSManifestURL != "" ||
		r.StreamingData.DASHManifestURL != ""
}

func player_response_without_streaming_data(response raw_player_response) raw_player_response {
	response.StreamingData = raw_streaming_data{}
	return response
}

func merge_player_responses(base, next raw_player_response) raw_player_response {
	if next.VideoDetails.VideoID != "" || next.PlayabilityStatus.Status != "" {
		captions := merge_raw_captions(base.Captions, next.Captions)
		base = next
		base.Captions = captions
		return base
	}
	if len(next.StreamingData.Formats) > 0 || len(next.StreamingData.AdaptiveFormats) > 0 {
		base.StreamingData = next.StreamingData
	}
	base.Captions = merge_raw_captions(base.Captions, next.Captions)
	return base
}

func merge_player_response_list(responses []raw_player_response) raw_player_response {
	var out raw_player_response
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
		out.Captions = merge_raw_captions(out.Captions, response.Captions)
		out.StreamingData.Formats = append_unique_raw_formats(out.StreamingData.Formats, response.StreamingData.Formats...)
		out.StreamingData.AdaptiveFormats = append_unique_raw_formats(out.StreamingData.AdaptiveFormats, response.StreamingData.AdaptiveFormats...)
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

func merge_raw_captions(base, next raw_captions) raw_captions {
	seen_tracks := make(map[string]bool, len(base.Player.CaptionTracks)+len(next.Player.CaptionTracks))
	for _, track := range base.Player.CaptionTracks {
		seen_tracks[raw_caption_track_key(track)] = true
	}
	for _, track := range next.Player.CaptionTracks {
		key := raw_caption_track_key(track)
		if seen_tracks[key] {
			continue
		}
		seen_tracks[key] = true
		base.Player.CaptionTracks = append(base.Player.CaptionTracks, track)
	}
	if len(base.Player.AudioTracks) == 0 && len(next.Player.AudioTracks) > 0 {
		base.Player.AudioTracks = next.Player.AudioTracks
		base.Player.DefaultAudioTrackIndex = next.Player.DefaultAudioTrackIndex
	}
	seen_languages := make(map[string]bool, len(base.Player.TranslationLanguages)+len(next.Player.TranslationLanguages))
	for _, language := range base.Player.TranslationLanguages {
		seen_languages[language.LanguageCode] = true
	}
	for _, language := range next.Player.TranslationLanguages {
		if seen_languages[language.LanguageCode] {
			continue
		}
		seen_languages[language.LanguageCode] = true
		base.Player.TranslationLanguages = append(base.Player.TranslationLanguages, language)
	}
	return base
}

func raw_caption_track_key(track raw_caption_track) string {
	if vss_id := strings.TrimSpace(track.VssID); vss_id != "" {
		return strings.Join([]string{"vss", vss_id, track.LanguageCode, track.Kind}, "\x00")
	}
	return strings.Join([]string{"url", track.BaseURL, track.LanguageCode, track.Kind}, "\x00")
}

func append_unique_raw_formats(base []raw_format, values ...raw_format) []raw_format {
	seen := make(map[string]bool, len(base)+len(values))
	for _, format := range base {
		seen[raw_format_key(format)] = true
	}
	for _, format := range values {
		key := raw_format_key(format)
		if seen[key] {
			continue
		}
		seen[key] = true
		base = append(base, format)
	}
	return base
}

func raw_format_key(format raw_format) string {
	return strings.Join([]string{
		strconv.Itoa(format.Itag),
		format.MimeType,
		first_non_empty(format.URL, format.SignatureCipher, format.Cipher),
		format.AudioQuality,
	}, "\x00")
}

func (t text_renderer) text() string {
	if t.SimpleText != "" {
		return html.UnescapeString(t.SimpleText)
	}
	var b strings.Builder
	for _, run := range t.Runs {
		b.WriteString(run.Text)
	}
	return html.UnescapeString(b.String())
}

func build_video_info(video_id string, webpage_url string, raw raw_player_response, owner raw_initial_data_owner, player *player_resolver) *VideoInfo {
	mf := raw.Microformat.Player
	vd := raw.VideoDetails
	if vd.VideoID != "" {
		video_id = vd.VideoID
	}
	duration := first_non_zero_int64(parse_int64(vd.LengthSeconds), parse_int64(mf.LengthSeconds))
	view_count := first_non_zero_int64(parse_int64(vd.ViewCount), parse_int64(mf.ViewCount))
	title := first_non_empty(vd.Title, mf.Title.text(), "youtube_"+video_id)
	channel := first_non_empty(vd.Author, mf.OwnerChannelName, owner.Channel)
	channel_id := first_non_empty(vd.ChannelID, mf.ExternalChannelID, owner.ChannelID)
	channel_url := first_non_empty(mf.OwnerProfileURL, owner.ChannelURL)
	if channel_url == "" && channel_id != "" {
		channel_url = default_base_url + "/channel/" + channel_id
	}
	channel_avatar_url := owner.ChannelAvatarURL

	formats, warnings := extract_formats(raw.StreamingData, player)
	caption_tracks, caption_audio_tracks, caption_translation_languages := extract_captions(raw.Captions, player)
	thumbnails := collect_thumbnails(video_id, vd.Thumbnail.Thumbnails, mf.Thumbnail.Thumbnails)
	thumbnail := best_thumbnail(thumbnails)
	age_limit := 0
	if mf.IsFamilySafe != nil && !*mf.IsFamilySafe {
		age_limit = 18
	}
	media_type := "video"
	if vd.IsLiveContent {
		media_type = "livestream"
	} else if mf.IsShortsEligible {
		media_type = "short"
	}
	live_status := "not_live"
	if vd.IsLiveContent {
		live_status = "is_live"
	}
	categories := []string(nil)
	if mf.Category != "" {
		categories = []string{mf.Category}
	}
	return &VideoInfo{
		ID:                          video_id,
		Title:                       title,
		Description:                 first_non_empty(vd.ShortDescription, mf.Description.text()),
		WebpageURL:                  canonical_video_url(video_id),
		Thumbnail:                   thumbnail,
		Thumbnails:                  thumbnails,
		Duration:                    duration,
		ViewCount:                   view_count,
		AgeLimit:                    age_limit,
		Channel:                     channel,
		ChannelID:                   channel_id,
		ChannelURL:                  channel_url,
		ChannelAvatarURL:            channel_avatar_url,
		Uploader:                    channel,
		UploaderURL:                 channel_url,
		UploaderAvatarURL:           channel_avatar_url,
		Categories:                  categories,
		Tags:                        vd.Keywords,
		PublishDate:                 mf.PublishDate,
		UploadDate:                  mf.UploadDate,
		LiveStatus:                  live_status,
		MediaType:                   media_type,
		PlayableInEmbed:             raw.PlayabilityStatus.PlayableInEmbed,
		Formats:                     formats,
		CaptionTracks:               caption_tracks,
		CaptionAudioTracks:          caption_audio_tracks,
		CaptionTranslationLanguages: caption_translation_languages,
		PlayabilityStatus:           PlayabilityStatus{Status: raw.PlayabilityStatus.Status, Reason: raw.PlayabilityStatus.Reason},
		Warnings:                    warnings,
	}
}

func extract_captions(raw raw_captions, player *player_resolver) ([]CaptionTrack, []CaptionAudioTrack, []CaptionTranslationLanguage) {
	default_caption_index := -1
	default_audio_index := raw.Player.DefaultAudioTrackIndex
	if default_audio_index < 0 || default_audio_index >= len(raw.Player.AudioTracks) {
		default_audio_index = 0
	}
	if len(raw.Player.AudioTracks) > 0 {
		default_caption_index = raw.Player.AudioTracks[default_audio_index].DefaultCaptionTrackIndex
		if default_caption_index < 0 || default_caption_index >= len(raw.Player.CaptionTracks) {
			indices := raw.Player.AudioTracks[default_audio_index].CaptionTrackIndices
			if len(indices) > 0 {
				default_caption_index = indices[0]
			}
		}
	} else if len(raw.Player.CaptionTracks) > 0 {
		default_caption_index = 0
	}

	tracks := make([]CaptionTrack, 0, len(raw.Player.CaptionTracks))
	for index, track := range raw.Player.CaptionTracks {
		base_url := strings.TrimSpace(track.BaseURL)
		if base_url == "" {
			continue
		}
		if player != nil && player.client != nil {
			if po_token := player.client.po_token_for("web", "subs"); po_token != "" {
				base_url = update_url_query(base_url, map[string]string{"pot": po_token})
			}
		}
		tracks = append(tracks, CaptionTrack{
			BaseURL:        base_url,
			Name:           track.Name.text(),
			VssID:          track.VssID,
			LanguageCode:   track.LanguageCode,
			Kind:           track.Kind,
			IsDefault:      index == default_caption_index,
			IsTranslatable: track.IsTranslatable,
		})
	}

	audio_tracks := make([]CaptionAudioTrack, 0, len(raw.Player.AudioTracks))
	for index, track := range raw.Player.AudioTracks {
		audio_tracks = append(audio_tracks, CaptionAudioTrack{
			ID:                       track.AudioTrackID,
			DisplayName:              track.DisplayName,
			CaptionTrackIndices:      append([]int(nil), track.CaptionTrackIndices...),
			DefaultCaptionTrackIndex: track.DefaultCaptionTrackIndex,
			IsDefault:                track.HasDefaultTrack || index == default_audio_index,
		})
	}

	languages := make([]CaptionTranslationLanguage, 0, len(raw.Player.TranslationLanguages))
	for _, language := range raw.Player.TranslationLanguages {
		if strings.TrimSpace(language.LanguageCode) == "" {
			continue
		}
		languages = append(languages, CaptionTranslationLanguage{
			LanguageCode: language.LanguageCode,
			Name:         language.LanguageName.text(),
		})
	}
	return tracks, audio_tracks, languages
}

func extract_formats(streaming raw_streaming_data, player *player_resolver) ([]VideoFormat, []string) {
	var out []VideoFormat
	var warnings []string
	skipped_signature := 0
	solved_signature := 0
	skipped_drm := 0
	skipped_otf := 0
	skipped_missing_pot := 0
	skipped_unsolved_n := 0
	missing_pot_clients := make(map[string]bool)
	solved_n_challenge := 0

	add := func(raw raw_format, adaptive bool) {
		format := format_from_raw(raw, adaptive)
		if format.HasDRM {
			skipped_drm++
			return
		}
		if raw.Type == "FORMAT_STREAM_TYPE_OTF" {
			skipped_otf++
			return
		}
		result := direct_format_url(raw, player)
		format.URL = result.URL
		format.NeedsSignature = result.NeedsSignature
		format.HadNChallenge = result.HadNChallenge
		format.SolvedNChallenge = result.SolvedNChallenge
		if result.NeedsSignature {
			skipped_signature++
			return
		}
		if format.URL == "" {
			return
		}
		if result.HadNChallenge && !result.SolvedNChallenge {
			skipped_unsolved_n++
			return
		}
		if raw.SourceRequiresPOT && query_value(format.URL, "pot") == "" {
			skipped_missing_pot++
			missing_pot_clients[first_non_empty(raw.SourceClient, "unknown")] = true
			return
		}
		if result.SolvedSignature {
			solved_signature++
		}
		if result.SolvedNChallenge {
			solved_n_challenge++
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
		return format_score(out[i]) > format_score(out[j])
	})
	if skipped_signature > 0 {
		warnings = append(warnings, "部分 YouTube 格式需要 player JS 解签，已跳过")
	}
	if solved_signature > 0 {
		warnings = append(warnings, "已解算部分 YouTube player JS 签名格式")
	}
	if skipped_drm > 0 {
		warnings = append(warnings, "部分 YouTube 格式带 DRM，已跳过")
	}
	if skipped_otf > 0 {
		warnings = append(warnings, "部分 YouTube OTF 分片格式当前不支持，已跳过")
	}
	if skipped_missing_pot > 0 {
		clients := make([]string, 0, len(missing_pot_clients))
		for client_name := range missing_pot_clients {
			clients = append(clients, client_name)
		}
		sort.Strings(clients)
		warnings = append(warnings, fmt.Sprintf("已跳过 %d 个缺少匹配 GVS PO Token 的 YouTube HTTPS 格式（client: %s）", skipped_missing_pot, strings.Join(clients, ", ")))
	}
	if skipped_unsolved_n > 0 {
		warnings = append(warnings, fmt.Sprintf("已跳过 %d 个未能解算 n challenge 的 YouTube HTTPS 格式，以避免下载时 HTTP 403", skipped_unsolved_n))
	}
	if solved_n_challenge > 0 {
		warnings = append(warnings, "已解算 YouTube n challenge")
	}
	if streaming.HLSManifestURL != "" || streaming.DASHManifestURL != "" {
		warnings = append(warnings, "已发现 HLS/DASH manifest；当前实现仅暴露 direct HTTPS 格式")
	}
	return out, warnings
}

func format_from_raw(raw raw_format, adaptive bool) VideoFormat {
	ext, audio_codec, video_codec, has_audio, has_video := parse_mime(raw.MimeType, raw.AudioQuality, raw.AudioSampleRate)
	id := strconv.Itoa(raw.Itag)
	if raw.Itag == 0 {
		id = first_non_empty(raw.QualityLabel, raw.Quality, raw.AudioQuality)
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
		ContentLength:   parse_int64(raw.ContentLength),
		AudioQuality:    strings.ToLower(raw.AudioQuality),
		AudioSampleRate: int(parse_int64(raw.AudioSampleRate)),
		AudioChannels:   raw.AudioChannels,
		AudioCodec:      audio_codec,
		VideoCodec:      video_codec,
		HasAudio:        has_audio,
		HasVideo:        has_video,
		Adaptive:        adaptive,
		Protocol:        "https",
		HasDRM:          len(raw.DRMFamilies) > 0,
		SourceClient:    raw.SourceClient,
		SourceClientID:  raw.SourceClientID,
		SourceVersion:   raw.SourceVersion,
		SourceUserAgent: raw.SourceUserAgent,
		SourceCookies:   raw.SourceCookies,
		RequiresPOT:     raw.SourceRequiresPOT,
	}
}

type format_url_result struct {
	URL              string
	NeedsSignature   bool
	SolvedSignature  bool
	HadNChallenge    bool
	SolvedNChallenge bool
}

func direct_format_url(raw raw_format, player *player_resolver) format_url_result {
	if raw.URL != "" {
		return finalize_format_url(resolve_n_challenge(raw.URL, player), player, raw.SourceClient, raw.SourceRequiresPOT)
	}
	cipher := first_non_empty(raw.SignatureCipher, raw.Cipher)
	if cipher == "" {
		return format_url_result{}
	}
	values, err := url.ParseQuery(cipher)
	if err != nil {
		return format_url_result{}
	}
	format_url := values.Get("url")
	if format_url == "" {
		return format_url_result{}
	}
	if encrypted_sig := values.Get("s"); encrypted_sig != "" {
		if player == nil {
			return format_url_result{NeedsSignature: true}
		}
		sig, err := player.solve_signature(encrypted_sig)
		if err != nil {
			player.warn("youtube player JS signature solving failed: " + err.Error())
			return format_url_result{NeedsSignature: true}
		}
		sp := values.Get("sp")
		if sp == "" {
			sp = "signature"
		}
		format_url = update_url_query(format_url, map[string]string{sp: sig})
		result := resolve_n_challenge(format_url, player)
		result.SolvedSignature = true
		return finalize_format_url(result, player, raw.SourceClient, raw.SourceRequiresPOT)
	}
	return finalize_format_url(resolve_n_challenge(format_url, player), player, raw.SourceClient, raw.SourceRequiresPOT)
}

func resolve_n_challenge(format_url string, player *player_resolver) format_url_result {
	result := format_url_result{URL: format_url}
	n_value := query_value(format_url, "n")
	if n_value == "" {
		return result
	}
	result.HadNChallenge = true
	if player == nil {
		return result
	}
	n_result, err := player.solve_n(n_value)
	if err != nil {
		player.warn("youtube player JS n challenge solving failed: " + err.Error())
		return result
	}
	result.URL = update_url_query(format_url, map[string]string{"n": n_result})
	result.SolvedNChallenge = true
	return result
}

func finalize_format_url(result format_url_result, player *player_resolver, source_client string, requires_pot bool) format_url_result {
	if result.URL == "" || result.NeedsSignature || player == nil || player.client == nil {
		return result
	}
	if !requires_pot || query_value(result.URL, "pot") != "" {
		return result
	}
	if po_token := player.client.po_token_for(source_client, "gvs"); po_token != "" {
		result.URL = update_url_query(result.URL, map[string]string{"pot": po_token})
	}
	return result
}

type player_resolver struct {
	client     *Client
	ctx        context.Context
	player_url string
	player_js  string
	fetch_err  error
	sig_name   string
	n_name     string
	sig_specs  map[int][]int
	warnings   []string
	warned     map[string]bool
}

func (c *Client) new_player_resolver(ctx context.Context, webpage []byte, ytcfg map[string]any) *player_resolver {
	return &player_resolver{
		client:     c,
		ctx:        ctx,
		player_url: c.extract_player_js_url(webpage, ytcfg),
		sig_specs:  map[int][]int{},
		warned:     map[string]bool{},
	}
}

func (c *Client) extract_player_js_url(webpage []byte, ytcfg map[string]any) string {
	for _, key := range []string{"PLAYER_JS_URL", "jsUrl"} {
		if value := string_from_map(ytcfg, key); value != "" {
			return c.abs_youtube_url(value)
		}
	}
	if value := find_player_js_url(ytcfg); value != "" {
		return c.abs_youtube_url(value)
	}
	if match := player_js_url_re.FindSubmatch(webpage); len(match) == 2 {
		value := strings.ReplaceAll(string(match[1]), `\/`, `/`)
		return c.abs_youtube_url(value)
	}
	return ""
}

func find_player_js_url(value any) string {
	switch v := value.(type) {
	case string:
		if strings.Contains(v, "/s/player/") && strings.Contains(v, "base.js") {
			return v
		}
	case map[string]any:
		for _, child := range v {
			if found := find_player_js_url(child); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range v {
			if found := find_player_js_url(child); found != "" {
				return found
			}
		}
	}
	return ""
}

func (c *Client) abs_youtube_url(raw string) string {
	return absolute_url(raw, c.base_url())
}

func absolute_youtube_url(raw string) string {
	return absolute_url(raw, default_base_url)
}

func absolute_url(raw string, base_url string) string {
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
	base, _ := url.Parse(strings.TrimRight(base_url, "/") + "/")
	ref, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return base.ResolveReference(ref).String()
}

func (r *player_resolver) solve_signature(challenge string) (string, error) {
	code, err := r.player_code()
	if err != nil {
		return "", err
	}
	if spec := r.sig_specs[len(challenge)]; len(spec) > 0 {
		return apply_signature_spec(challenge, spec), nil
	}
	name := r.sig_name
	if name == "" {
		name = find_first_function_name(code, sig_func_name_res)
		r.sig_name = name
	}
	if name != "" {
		if solved, err := solve_player_function(code, name, challenge); err == nil {
			return solved, nil
		}
	}
	solved, err := r.solve_cached_player_challenges(code, "sig", []string{challenge})
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

func (r *player_resolver) solve_n(challenge string) (string, error) {
	code, err := r.player_code()
	if err != nil {
		return "", err
	}
	name := r.n_name
	if name == "" {
		name = find_n_function_expression(code)
		r.n_name = name
	}
	if name != "" {
		if solved, err := solve_player_function(code, name, challenge); err == nil {
			return solved, nil
		}
	}
	solved, err := r.solve_cached_player_challenges(code, "n", []string{challenge})
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

func (r *player_resolver) player_code() (string, error) {
	if r.player_js != "" {
		return r.player_js, nil
	}
	if r.fetch_err != nil {
		return "", r.fetch_err
	}
	if r.player_url == "" {
		r.fetch_err = fmt.Errorf("player JS URL not found")
		return "", r.fetch_err
	}
	fetch_started := time.Now()
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, r.player_url, nil)
	if err != nil {
		r.fetch_err = err
		return "", err
	}
	r.client.set_default_headers(req, default_base_url)
	resp, err := r.client.http_client().Do(req)
	if err != nil {
		r.fetch_err = err
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.fetch_err = fmt.Errorf("player JS request failed: %s", resp.Status)
		return "", r.fetch_err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		r.fetch_err = err
		return "", err
	}
	r.player_js = string(body)
	if r.client != nil && r.client.Logger != nil {
		r.client.Logger.Info().
			Str("player_id", youtube_player_id(r.player_url)).
			Int("player_js_bytes", len(body)).
			Dur("elapsed", time.Since(fetch_started)).
			Msg("youtube player JavaScript fetched")
	}
	return r.player_js, nil
}

func (r *player_resolver) signature_timestamp() string {
	code, err := r.player_code()
	if err != nil {
		return ""
	}
	if match := signature_timestamp_re.FindStringSubmatch(code); len(match) == 2 {
		return match[1]
	}
	return ""
}

func (r *player_resolver) warn(message string) {
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

func find_first_function_name(code string, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(code); len(match) >= 2 {
			return match[1]
		}
	}
	return ""
}

func find_n_function_expression(code string) string {
	if match := n_array_call_re.FindStringSubmatch(code); len(match) == 3 {
		return match[1] + "[" + match[2] + "]"
	}
	return find_first_function_name(code, n_func_name_res)
}

func solve_player_function(player_js string, function_name string, challenge string) (string, error) {
	if result, ok := solve_simple_player_function(player_js, function_name, challenge); ok {
		return result, nil
	}
	return solve_player_function_with_goja(player_js, function_name, challenge)
}

func solve_player_function_with_goja(player_js string, function_name string, challenge string) (string, error) {
	vm := goja.New()
	global := vm.GlobalObject()
	_ = global.Set("window", global)
	_ = global.Set("self", global)
	_ = global.Set("globalThis", global)
	_ = global.Set("navigator", map[string]any{"userAgent": default_user_agent})
	_ = global.Set("location", map[string]any{"href": default_base_url + "/"})
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

	if _, err := vm.RunString(player_js); err != nil {
		return "", fmt.Errorf("run player JS: %w", err)
	}
	fn, ok := goja.AssertFunction(vm.Get(function_name))
	if !ok && strings.ContainsAny(function_name, ".[") {
		value, err := vm.RunString("(" + function_name + ")")
		if err != nil {
			return "", fmt.Errorf("evaluate player function %q: %w", function_name, err)
		}
		fn, ok = goja.AssertFunction(value)
	}
	if !ok {
		return "", fmt.Errorf("player function %q is not callable", function_name)
	}
	result, err := fn(goja.Undefined(), vm.ToValue(challenge))
	if err != nil {
		return "", fmt.Errorf("call player function %q: %w", function_name, err)
	}
	out := result.String()
	if out == "" {
		return "", fmt.Errorf("empty player function result")
	}
	return out, nil
}

func apply_signature_spec(value string, spec []int) string {
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

func solve_simple_player_function(player_js string, function_name string, challenge string) (string, bool) {
	body, ok := extract_function_body(player_js, function_name)
	if !ok {
		return "", false
	}
	compact := strings.ReplaceAll(body, " ", "")
	compact = strings.ReplaceAll(compact, "\n", "")
	compact = strings.ReplaceAll(compact, "\t", "")
	switch {
	case strings.Contains(compact, `.split("").reverse().join("")`):
		return reverse_string(challenge), true
	case strings.Contains(compact, `.reverse()`) && strings.Contains(compact, `.join("")`):
		return reverse_string(challenge), true
	}
	return "", false
}

func extract_function_body(code string, name string) (string, bool) {
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
		if end := matching_brace_index(code, start); end > start {
			return code[start+1 : end], true
		}
	}
	return "", false
}

func matching_brace_index(value string, start int) int {
	if start < 0 || start >= len(value) || value[start] != '{' {
		return -1
	}
	depth := 0
	in_string := byte(0)
	escaped := false
	for i := start; i < len(value); i++ {
		ch := value[i]
		if in_string != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == in_string {
				in_string = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			in_string = ch
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

func reverse_string(value string) string {
	runes := []rune(value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func query_value(raw_url string, key string) string {
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	return parsed.Query().Get(key)
}

func update_url_query(raw_url string, values map[string]string) string {
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return raw_url
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

func clone_map(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = clone_map(typed)
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

func parse_mime(mime_type string, audio_quality string, sample_rate string) (ext, audio_codec, video_codec string, has_audio, has_video bool) {
	media_type, params, err := mime.ParseMediaType(mime_type)
	if err != nil {
		media_type = strings.Split(mime_type, ";")[0]
	}
	media_type = strings.ToLower(strings.TrimSpace(media_type))
	switch media_type {
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
		if strings.Contains(media_type, "mp4") {
			ext = "mp4"
		} else if strings.Contains(media_type, "webm") {
			ext = "webm"
		}
	}
	codecs := split_codecs(params["codecs"])
	for _, codec := range codecs {
		switch {
		case is_audio_codec(codec):
			if audio_codec == "" {
				audio_codec = codec
			}
		case is_video_codec(codec):
			if video_codec == "" {
				video_codec = codec
			}
		}
	}
	has_video = strings.HasPrefix(media_type, "video/") && video_codec != "none"
	has_audio = strings.HasPrefix(media_type, "audio/") || audio_codec != "" || audio_quality != "" || sample_rate != ""
	if has_video && len(codecs) == 1 && audio_quality == "" && sample_rate == "" {
		has_audio = false
	}
	return ext, audio_codec, video_codec, has_audio, has_video
}

func split_codecs(codecs string) []string {
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

func is_audio_codec(codec string) bool {
	codec = strings.ToLower(codec)
	return strings.HasPrefix(codec, "mp4a") ||
		strings.HasPrefix(codec, "opus") ||
		strings.HasPrefix(codec, "vorbis") ||
		strings.HasPrefix(codec, "ac-3") ||
		strings.HasPrefix(codec, "ec-3")
}

func is_video_codec(codec string) bool {
	codec = strings.ToLower(codec)
	return strings.HasPrefix(codec, "avc") ||
		strings.HasPrefix(codec, "vp") ||
		strings.HasPrefix(codec, "hev") ||
		strings.HasPrefix(codec, "hvc") ||
		strings.HasPrefix(codec, "av01")
}

func clone_string_map(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func format_metadata(format VideoFormat) map[string]any {
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

func merged_format_metadata(video VideoFormat, audio VideoFormat, ext string) map[string]any {
	return map[string]any{
		"format_id":            video.ID + "+" + audio.ID,
		"format_type":          "merged",
		"requested_format_ids": []string{video.ID, audio.ID},
		"output_ext":           ext,
		"video_format":         format_metadata(video),
		"audio_format":         format_metadata(audio),
	}
}

func merged_format_label(video VideoFormat, audio VideoFormat, best bool) string {
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
	audio_part := "音频"
	if audio.Ext != "" {
		audio_part += " " + strings.ToUpper(audio.Ext)
	}
	return strings.Join(parts, " ") + " + " + audio_part
}

func video_format_label(format VideoFormat) string {
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

func audio_format_label(format VideoFormat) string {
	parts := []string{"MP3"}
	if bitrate := first_non_zero_int(format.AverageBitrate, format.Bitrate); bitrate > 0 {
		parts = append(parts, fmt.Sprintf("%dk", bitrate/1000))
	}
	if format.Ext != "" {
		parts = append(parts, "from "+strings.ToUpper(format.Ext))
	}
	return strings.Join(parts, " ")
}

func format_type(format VideoFormat) string {
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

func merged_output_ext(video VideoFormat, audio VideoFormat) string {
	video_ext := strings.ToLower(video.Ext)
	audio_ext := strings.ToLower(audio.Ext)
	switch {
	case video_ext == "mp4" && (audio_ext == "m4a" || audio_ext == "mp4") && mp4_compatible_audio_codec(audio.AudioCodec):
		return "mp4"
	case video_ext == "webm" && audio_ext == "webm":
		return "webm"
	default:
		return "mkv"
	}
}

func mp4_compatible_audio_codec(codec string) bool {
	codec = strings.ToLower(codec)
	return codec == "" ||
		strings.HasPrefix(codec, "mp4a") ||
		strings.HasPrefix(codec, "ac-3") ||
		strings.HasPrefix(codec, "ec-3")
}

func format_score(format VideoFormat) int {
	return format.Height*1_000_000 + format.FPS*10_000 + first_non_zero_int(format.AverageBitrate, format.Bitrate)
}

func audio_score(format VideoFormat) int {
	return first_non_zero_int(format.AverageBitrate, format.Bitrate) + int(format.ContentLength/1024)
}

func collect_image_thumbnails(groups ...[]Thumbnail) []Thumbnail {
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

func collect_thumbnails(video_id string, groups ...[]Thumbnail) []Thumbnail {
	out := collect_image_thumbnails(groups...)
	seen := make(map[string]bool, len(out)+4)
	for _, thumb := range out {
		seen[thumb.URL] = true
	}
	for _, name := range []string{"maxresdefault", "hq720", "sddefault", "hqdefault"} {
		thumb_url := fmt.Sprintf("https://i.ytimg.com/vi/%s/%s.jpg", video_id, name)
		if !seen[thumb_url] {
			seen[thumb_url] = true
			out = append(out, Thumbnail{URL: thumb_url})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Width*out[i].Height > out[j].Width*out[j].Height
	})
	return out
}

func best_thumbnail(thumbnails []Thumbnail) string {
	if len(thumbnails) == 0 {
		return ""
	}
	return thumbnails[0].URL
}

func suffix_from_url(raw_url string) string {
	parsed, err := url.Parse(raw_url)
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

func has_query_param(raw_url string, key string) bool {
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return false
	}
	return parsed.Query().Get(key) != ""
}

func default_innertube_context() map[string]any {
	return map[string]any{
		"client": map[string]any{
			"clientName":    "WEB",
			"clientVersion": "2.20260114.08.00",
			"hl":            "en",
		},
	}
}

func string_from_map(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return value
	}
	return ""
}

func extract_visitor_data(ytcfg map[string]any, responses ...raw_player_response) string {
	if visitor_data := string_from_map(ytcfg, "VISITOR_DATA"); visitor_data != "" {
		return visitor_data
	}
	if context_value, ok := ytcfg["INNERTUBE_CONTEXT"].(map[string]any); ok {
		if client, ok := context_value["client"].(map[string]any); ok {
			if visitor_data := string_from_map(client, "visitorData"); visitor_data != "" {
				return visitor_data
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

func parse_int64(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func first_non_zero_int(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func first_non_zero_int64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
