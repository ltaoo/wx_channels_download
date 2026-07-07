package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

const (
	defaultAPIBaseURL = "https://api.bilibili.com"
	defaultWebBaseURL = "https://www.bilibili.com"
	defaultUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

	bilibiliHTTPChunkSize = 10 << 20
)

type Client struct {
	HTTPClient *http.Client
	APIBaseURL string
	WebBaseURL string
	UserAgent  string
	Cookie     string
}

type Handler struct {
	Resolver *Client
}

func New(resolver *Client) *Handler {
	if resolver == nil {
		resolver = NewClient(nil)
	} else {
		resolver.ensureDefaults()
	}
	return &Handler{Resolver: resolver}
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		HTTPClient: httpClient,
		APIBaseURL: defaultAPIBaseURL,
		WebBaseURL: defaultWebBaseURL,
		UserAgent:  defaultUserAgent,
	}
}

func (c *Client) ensureDefaults() {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if strings.TrimSpace(c.APIBaseURL) == "" {
		c.APIBaseURL = defaultAPIBaseURL
	}
	if strings.TrimSpace(c.WebBaseURL) == "" {
		c.WebBaseURL = defaultWebBaseURL
	}
	if strings.TrimSpace(c.UserAgent) == "" {
		c.UserAgent = defaultUserAgent
	}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	if _, ok := ExtractOpusID(rawURL); ok {
		return true
	}
	_, ok := ExtractVideoKey(rawURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	if h.Resolver == nil {
		return nil, contentdownload.ErrResolveUnavailable
	}
	return h.Resolver.Probe(ctx, input)
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	if h.Resolver == nil {
		return nil, contentdownload.ErrResolveUnavailable
	}
	resolved, err := h.Resolver.Resolve(ctx, input)
	if err != nil {
		return nil, err
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	nodes := []contentdownload.PipelineNode{
		{ID: "download", Type: "download_asset", Stage: "download"},
	}
	if resolved != nil && resolved.Download.Protocol == contentdownload.ProtocolMultiHTTP {
		nodes[0].Args = map[string]any{"merge": "ffmpeg", "sources": "multi_http"}
	}
	if resolved != nil && resolved.Suffix == ".mp3" {
		nodes = append(nodes, contentdownload.PipelineNode{
			ID:        "transcode_mp3",
			Type:      "ffmpeg_extract_mp3",
			Stage:     "post",
			DependsOn: []string{"download"},
			Args:      map[string]any{"bitrate": "192k"},
		})
	}
	nodes = append(nodes, contentdownload.PipelineNode{ID: "persist", Type: "persist_artifacts", Stage: "persist"})
	return &contentdownload.PipelinePlan{Platform: PlatformID, Nodes: nodes}, nil
}

func (c *Client) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	if _, ok := ExtractOpusID(input.URL); ok {
		return c.ProbeOpus(ctx, input)
	}
	info, err := c.Extract(ctx, input.URL)
	if err != nil {
		return nil, err
	}
	variants, defaults := buildVariants(info)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: info.WebpageURL,
		ContentID:    firstNonEmpty(info.BVID, strconv.FormatInt(info.AID, 10)),
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "video",
			ID:              firstNonEmpty(info.BVID, strconv.FormatInt(info.AID, 10)),
			Title:           firstNonEmpty(info.Title, "bilibili_"+firstNonEmpty(info.BVID, strconv.FormatInt(info.AID, 10))),
			Description:     info.Description,
			Author:          info.Owner.Name,
			URL:             info.WebpageURL,
			SourceURL:       info.WebpageURL,
			AuthorNickname:  info.Owner.Name,
			AuthorAvatarURL: info.Owner.Face,
			CoverURL:        info.Pic,
			Duration:        info.Duration,
		}, info, map[string]any{
			"aid":                 info.AID,
			"bvid":                info.BVID,
			"cid":                 info.CID,
			"page":                info.Page.Page,
			"part":                info.Page.Part,
			"account_external_id": strconv.FormatInt(info.Owner.MID, 10),
			"author_id":           strconv.FormatInt(info.Owner.MID, 10),
			"author_homepage_url": bilibiliSpaceURL(c.webBaseURL(), info.Owner.MID),
		}, ProbeOutput(info).Map()),
		Variants: variants,
		Defaults: defaults,
		Internal: map[string]any{
			"video_info": info,
			"pagejson":   info.RawView,
			"playurl":    info.RawPlayURL,
		},
		Warnings: info.Warnings,
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
		if opusInfoFromProbe(probe) != nil || contentdownload.ContentType(probe.Content) == "article" {
			return c.resolveOpus(ctx, input, probe)
		}
	}
	if info == nil {
		return nil, contentdownload.ErrResolveUnavailable
	}
	variant, err := contentdownload.SelectVariant(probe, input.Options)
	if err != nil {
		return nil, err
	}

	headers := c.downloadHeaders(info.WebpageURL)
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".mp4")
	metadata := map[string]any{
		"variant_id": variant.ID,
		"aid":        info.AID,
		"bvid":       info.BVID,
		"cid":        info.CID,
		"page":       info.Page.Page,
		"part":       info.Page.Part,
	}
	downloadSpec := contentdownload.DownloadSpec{}

	switch {
	case variant.ID == "cover":
		if info.Pic == "" {
			return nil, fmt.Errorf("bilibili cover is unavailable")
		}
		suffix = firstNonEmpty(input.Options.Suffix, suffixFromURL(info.Pic), ".jpg")
		metadata["format_type"] = "cover"
		downloadSpec = bilibiliHTTPDownloadSpec(info.Pic, headers)
	case variant.ID == "audio_m4a" || variant.ID == "audio_mp3":
		audio := selectBestAudio(info.PlayURL.DASH)
		if audio == nil || audio.URL() == "" {
			return nil, fmt.Errorf("bilibili audio stream is unavailable")
		}
		if variant.ID == "audio_mp3" {
			suffix = ".mp3"
			metadata["requires_ffmpeg"] = true
		} else {
			suffix = firstNonEmpty(input.Options.Suffix, ".m4a")
		}
		metadata["format_type"] = "audio"
		metadata["audio_id"] = audio.ID
		metadata["audio_codec"] = audio.Codecs
		metadata["direct_url"] = audio.URL()
		downloadSpec = bilibiliHTTPDownloadSpec(audio.URL(), headers)
	default:
		if isDASHVariant(variant) {
			qn := qualityOfVariant(variant)
			video := selectBestVideoByQuality(info.PlayURL.DASH, qn)
			if video == nil || video.URL() == "" {
				return nil, fmt.Errorf("bilibili quality %q stream is unavailable", variant.Spec)
			}
			audio := selectBestAudio(info.PlayURL.DASH)
			sources := []contentdownload.MultiSourceSpec{dashSource("video", *video, headers, true, false)}
			formatIDs := []string{strconv.Itoa(video.ID)}
			if audio != nil && audio.URL() != "" {
				sources = append(sources, dashSource("audio", *audio, headers, false, true))
				formatIDs = append(formatIDs, strconv.Itoa(audio.ID))
			}
			suffix = firstNonEmpty(input.Options.Suffix, variant.Suffix, ".mp4")
			metadata["format_id"] = strings.Join(formatIDs, "+")
			metadata["format_type"] = "dash"
			metadata["quality"] = video.ID
			metadata["video_codec"] = video.Codecs
			metadata["direct_urls"] = directURLs(sources)
			if len(sources) > 1 {
				metadata["sources"] = sources
				metadata["requires_ffmpeg"] = true
				downloadSpec = bilibiliMergedDownloadSpec(info.BVID, formatIDs, sources)
			} else {
				metadata["direct_url"] = video.URL()
				downloadSpec = bilibiliHTTPDownloadSpec(video.URL(), headers)
			}
		} else {
			durl := selectDURL(info.PlayURL.DURL)
			if durl == nil || durl.FirstURL() == "" {
				return nil, fmt.Errorf("bilibili video stream is unavailable")
			}
			metadata["format_type"] = "durl"
			metadata["quality"] = info.PlayURL.Quality
			metadata["direct_url"] = durl.FirstURL()
			downloadSpec = bilibiliHTTPDownloadSpec(durl.FirstURL(), headers)
		}
	}

	contentID := firstNonEmpty(info.BVID, strconv.FormatInt(info.AID, 10))
	filename := firstNonEmpty(input.Options.Filename, info.Title, contentID)
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    firstNonEmpty(probe.SourceURL, input.URL),
		CanonicalURL: info.WebpageURL,
		ContentID:    contentID,
		Title:        info.Title,
		Filename:     filename,
		Suffix:       suffix,
		Download:     downloadSpec,
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"title":        info.Title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   info.WebpageURL,
			"content_type": "video",
		},
		Metadata: metadata,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "video",
			ID:              contentID,
			Title:           info.Title,
			Description:     info.Description,
			URL:             info.WebpageURL,
			SourceURL:       info.WebpageURL,
			Author:          info.Owner.Name,
			AuthorNickname:  info.Owner.Name,
			AuthorAvatarURL: info.Owner.Face,
			CoverURL:        info.Pic,
			Duration:        info.Duration,
		}, info, contentdownload.ContentMetadataOf(probe.Content), nil),
		Internal: map[string]any{
			"video_info": info,
		},
	}
	return resolved, nil
}

func (c *Client) Extract(ctx context.Context, rawURL string) (*VideoInfo, error) {
	key, ok := ExtractVideoKey(rawURL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	view, err := c.fetchView(ctx, key)
	if err != nil {
		return nil, err
	}
	info := videoInfoFromView(c.webBaseURL(), rawURL, key, view)
	play, rawPlay, err := c.fetchBestPlayURL(ctx, info)
	if err != nil {
		return nil, err
	}
	info.PlayURL = play
	info.RawPlayURL = rawPlay
	if info.Duration == 0 {
		info.Duration = play.Timelength / 1000
	}
	return info, nil
}

func (c *Client) fetchView(ctx context.Context, key VideoKey) (*viewResponse, error) {
	values := url.Values{}
	if key.BVID != "" {
		values.Set("bvid", key.BVID)
	} else if key.AID > 0 {
		values.Set("aid", strconv.FormatInt(key.AID, 10))
	}
	apiURL, err := c.apiURL("/x/web-interface/view", values)
	if err != nil {
		return nil, err
	}
	req, err := c.newAPIRequest(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bilibili view request failed: %s", resp.Status)
	}
	var view viewResponse
	raw, err := decodeJSONWithRaw(resp.Body, &view)
	if err != nil {
		return nil, err
	}
	view.Raw = raw
	if view.Code != 0 {
		return nil, fmt.Errorf("bilibili view failed: %s", firstNonEmpty(view.Message, fmt.Sprint(view.Code)))
	}
	if view.Data.CID == 0 && len(view.Data.Pages) == 0 {
		return nil, fmt.Errorf("bilibili view missing cid")
	}
	return &view, nil
}

func (c *Client) fetchBestPlayURL(ctx context.Context, info *VideoInfo) (PlayURLData, json.RawMessage, error) {
	play, raw, err := c.fetchPlayURL(ctx, info, 127, 16)
	if err == nil && playHasMedia(play) {
		return play, raw, nil
	}
	if err != nil && info != nil {
		info.Warnings = append(info.Warnings, "B站 WBI playurl 失败，已尝试普通 playurl："+err.Error())
	}
	play, raw, plainErr := c.fetchPlainPlayURL(ctx, info, 127, 16)
	if plainErr == nil && playHasMedia(play) {
		return play, raw, nil
	}
	play, raw, durlErr := c.fetchPlainPlayURL(ctx, info, 127, 0)
	if durlErr == nil && playHasMedia(play) {
		return play, raw, nil
	}
	if plainErr != nil {
		return PlayURLData{}, nil, plainErr
	}
	return PlayURLData{}, nil, durlErr
}

func (c *Client) fetchPlayURL(ctx context.Context, info *VideoInfo, qn int, fnval int) (PlayURLData, json.RawMessage, error) {
	values := playURLValues(info, qn, fnval)
	signed, err := c.signWBIQuery(ctx, values)
	if err != nil {
		return PlayURLData{}, nil, err
	}
	return c.doPlayURL(ctx, "/x/player/wbi/playurl", signed)
}

func (c *Client) fetchPlainPlayURL(ctx context.Context, info *VideoInfo, qn int, fnval int) (PlayURLData, json.RawMessage, error) {
	return c.doPlayURL(ctx, "/x/player/playurl", playURLValues(info, qn, fnval))
}

func (c *Client) doPlayURL(ctx context.Context, apiPath string, values url.Values) (PlayURLData, json.RawMessage, error) {
	apiURL, err := c.apiURL(apiPath, values)
	if err != nil {
		return PlayURLData{}, nil, err
	}
	req, err := c.newAPIRequest(ctx, apiURL)
	if err != nil {
		return PlayURLData{}, nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return PlayURLData{}, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PlayURLData{}, nil, fmt.Errorf("bilibili playurl request failed: %s", resp.Status)
	}
	var parsed playURLResponse
	raw, err := decodeJSONWithRaw(resp.Body, &parsed)
	if err != nil {
		return PlayURLData{}, nil, err
	}
	if parsed.Code != 0 {
		return PlayURLData{}, raw, fmt.Errorf("bilibili playurl failed: %s", firstNonEmpty(parsed.Message, fmt.Sprint(parsed.Code)))
	}
	data := parsed.Data
	if data.Quality == 0 && (parsed.Result.Quality != 0 || parsed.Result.DASH != nil || len(parsed.Result.DURL) > 0) {
		data = parsed.Result
	}
	return data, raw, nil
}

func playURLValues(info *VideoInfo, qn int, fnval int) url.Values {
	values := url.Values{}
	if info.BVID != "" {
		values.Set("bvid", info.BVID)
	} else if info.AID > 0 {
		values.Set("avid", strconv.FormatInt(info.AID, 10))
	}
	values.Set("cid", strconv.FormatInt(info.CID, 10))
	values.Set("qn", strconv.Itoa(qn))
	values.Set("fnval", strconv.Itoa(fnval))
	values.Set("fourk", "1")
	values.Set("otype", "json")
	return values
}

func (c *Client) apiURL(apiPath string, values url.Values) (string, error) {
	base, err := url.Parse(c.apiBaseURL())
	if err != nil {
		return "", err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(apiPath, "/")
	base.RawQuery = values.Encode()
	return base.String(), nil
}

func (c *Client) newAPIRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range c.apiHeaders() {
		req.Header.Set(key, value)
	}
	return req, nil
}

func (c *Client) apiHeaders() map[string]string {
	headers := map[string]string{
		"User-Agent":      firstNonEmpty(c.UserAgent, defaultUserAgent),
		"Referer":         c.webBaseURL() + "/",
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	}
	if cookie := strings.TrimSpace(c.Cookie); cookie != "" {
		headers["Cookie"] = cookie
	}
	return headers
}

func (c *Client) downloadHeaders(referer string) map[string]string {
	headers := map[string]string{
		"User-Agent":      firstNonEmpty(c.UserAgent, defaultUserAgent),
		"Referer":         firstNonEmpty(referer, c.webBaseURL()+"/"),
		"Accept":          "*/*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Origin":          c.webBaseURL(),
		"Range":           "bytes=0-",
	}
	if cookie := strings.TrimSpace(c.Cookie); cookie != "" {
		headers["Cookie"] = cookie
	}
	return headers
}

func (c *Client) apiBaseURL() string {
	return strings.TrimRight(firstNonEmpty(c.APIBaseURL, defaultAPIBaseURL), "/")
}

func (c *Client) webBaseURL() string {
	return strings.TrimRight(firstNonEmpty(c.WebBaseURL, defaultWebBaseURL), "/")
}

func videoInfoFromView(webBaseURL string, rawURL string, key VideoKey, view *viewResponse) *VideoInfo {
	data := view.Data
	page := selectPage(data.Pages, data.CID, key.Page)
	cid := page.CID
	if cid == 0 {
		cid = data.CID
	}
	bvid := firstNonEmpty(data.BVID, key.BVID)
	info := &VideoInfo{
		BVID:        bvid,
		AID:         firstNonZeroInt64(data.AID, key.AID),
		CID:         cid,
		Title:       data.Title,
		Description: data.Description,
		Pic:         data.Pic,
		Owner:       data.Owner,
		Pages:       data.Pages,
		Page:        page,
		Duration:    firstNonZeroInt64(page.Duration, data.Duration),
		RawView:     view.Raw,
	}
	if info.Page.Page > 1 && info.Page.Part != "" && !strings.Contains(info.Title, info.Page.Part) {
		info.Title = info.Title + " - " + info.Page.Part
	}
	if bvid != "" {
		info.WebpageURL = strings.TrimRight(webBaseURL, "/") + "/video/" + pathEscape(bvid)
		if info.Page.Page > 1 {
			info.WebpageURL += "?p=" + strconv.Itoa(info.Page.Page)
		}
	} else {
		info.WebpageURL = rawURL
	}
	return info
}

func selectPage(pages []Page, defaultCID int64, requested int) Page {
	if requested <= 0 {
		requested = 1
	}
	for _, page := range pages {
		if page.Page == requested {
			return page
		}
	}
	if len(pages) > 0 {
		return pages[0]
	}
	return Page{CID: defaultCID, Page: requested}
}

func buildVariants(info *VideoInfo) ([]contentdownload.Variant, contentdownload.Defaults) {
	variants := make([]contentdownload.Variant, 0)
	defaults := contentdownload.Defaults{VariantID: "video", Suffix: ".mp4"}
	qualityLabels := qualityLabelMap(info.PlayURL)
	if info.PlayURL.DASH != nil && len(info.PlayURL.DASH.Video) > 0 {
		streams := bestVideosByQuality(info.PlayURL.DASH)
		qualities := make([]int, 0, len(streams))
		for qn := range streams {
			qualities = append(qualities, qn)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(qualities)))
		for _, qn := range qualities {
			stream := streams[qn]
			id := fmt.Sprintf("video_qn_%d", qn)
			if defaults.VariantID == "video" {
				defaults.VariantID = id
				defaults.Spec = strconv.Itoa(qn)
			}
			variants = append(variants, contentdownload.Variant{
				ID:       id,
				Type:     "video",
				Label:    "视频 " + qualityLabel(qn, qualityLabels),
				Spec:     strconv.Itoa(qn),
				Suffix:   ".mp4",
				Width:    stream.Width,
				Height:   stream.Height,
				Bitrate:  int(stream.Bandwidth),
				Requires: []string{"ffmpeg"},
				Metadata: map[string]any{
					"format_type": "dash",
					"quality":     qn,
					"codecs":      stream.Codecs,
				},
			})
		}
		if audio := selectBestAudio(info.PlayURL.DASH); audio != nil && audio.URL() != "" {
			variants = append(variants,
				contentdownload.Variant{ID: "audio_m4a", Type: "audio", Label: "音频 M4A", Suffix: ".m4a", Bitrate: int(audio.Bandwidth), Metadata: map[string]any{"format_type": "audio", "audio_id": audio.ID}},
				contentdownload.Variant{ID: "audio_mp3", Type: "audio", Label: "音频 MP3", Suffix: ".mp3", Bitrate: int(audio.Bandwidth), Requires: []string{"ffmpeg"}, Metadata: map[string]any{"format_type": "audio", "audio_id": audio.ID}},
			)
		}
	} else if len(info.PlayURL.DURL) > 0 {
		label := qualityLabel(info.PlayURL.Quality, qualityLabels)
		variants = append(variants, contentdownload.Variant{
			ID:     "video",
			Type:   "video",
			Label:  "视频 " + label,
			Spec:   strconv.Itoa(info.PlayURL.Quality),
			Suffix: ".mp4",
			Size:   durlSize(info.PlayURL.DURL),
			Metadata: map[string]any{
				"format_type": "durl",
				"quality":     info.PlayURL.Quality,
			},
		})
	}
	if info.Pic != "" {
		variants = append(variants, contentdownload.Variant{ID: "cover", Type: "image", Label: "封面", Suffix: firstNonEmpty(suffixFromURL(info.Pic), ".jpg")})
	}
	if len(variants) == 0 {
		defaults = contentdownload.Defaults{}
	}
	return variants, defaults
}

func ProbeOutput(info *VideoInfo) probeOutput {
	return probeOutput{
		ContentType:  "video",
		Title:        info.Title,
		SourceURL:    info.WebpageURL,
		BVID:         info.BVID,
		AID:          info.AID,
		CID:          info.CID,
		Page:         info.Page.Page,
		Part:         info.Page.Part,
		Duration:     info.Duration,
		UPName:       info.Owner.Name,
		UPMID:        info.Owner.MID,
		QualityCount: len(bestVideosByQuality(info.PlayURL.DASH)),
	}
}

type probeOutput struct {
	ContentType  string `json:"content_type,omitempty"`
	Title        string `json:"title,omitempty"`
	SourceURL    string `json:"source_url,omitempty"`
	BVID         string `json:"bvid,omitempty"`
	AID          int64  `json:"aid,omitempty"`
	CID          int64  `json:"cid,omitempty"`
	Page         int    `json:"page,omitempty"`
	Part         string `json:"part,omitempty"`
	Duration     int64  `json:"duration,omitempty"`
	UPName       string `json:"up_name,omitempty"`
	UPMID        int64  `json:"up_mid,omitempty"`
	QualityCount int    `json:"quality_count,omitempty"`
}

func (o probeOutput) Map() map[string]any {
	raw, _ := json.Marshal(o)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func ExtractVideoKey(rawURL string) (VideoKey, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return VideoKey{}, false
	}
	if isLikelyBVID(rawURL) {
		return VideoKey{BVID: rawURL, Page: 1}, true
	}
	if aid, ok := parseAID(rawURL); ok {
		return VideoKey{AID: aid, Page: 1}, true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return VideoKey{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if !isBilibiliHost(host) {
		return VideoKey{}, false
	}
	page := parsePositiveInt(parsed.Query().Get("p"), 1)
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(segments) > 0 {
		first, _ := url.PathUnescape(segments[0])
		if strings.EqualFold(first, "opus") {
			return VideoKey{}, false
		}
	}
	for i, segment := range segments {
		value, _ := url.PathUnescape(segment)
		if strings.EqualFold(value, "video") && i+1 < len(segments) {
			next, _ := url.PathUnescape(segments[i+1])
			if isLikelyBVID(next) {
				return VideoKey{BVID: next, Page: page}, true
			}
			if aid, ok := parseAID(next); ok {
				return VideoKey{AID: aid, Page: page}, true
			}
		}
		if isLikelyBVID(value) {
			return VideoKey{BVID: value, Page: page}, true
		}
		if aid, ok := parseAID(value); ok {
			return VideoKey{AID: aid, Page: page}, true
		}
	}
	return VideoKey{}, false
}

func isBilibiliHost(host string) bool {
	return host == "bilibili.com" || strings.HasSuffix(host, ".bilibili.com")
}

func isLikelyBVID(value string) bool {
	if len(value) < 10 || len(value) > 20 || !strings.HasPrefix(value, "BV") {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') {
			continue
		}
		return false
	}
	return true
}

func parseAID(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "av") {
		value = value[2:]
	}
	aid, err := strconv.ParseInt(value, 10, 64)
	return aid, err == nil && aid > 0
}

func parsePositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func videoInfoFromProbe(probe *contentdownload.Probe) *VideoInfo {
	if probe == nil {
		return nil
	}
	if probe.Internal != nil {
		if info, ok := probe.Internal["video_info"].(*VideoInfo); ok {
			return info
		}
	}
	if info, ok := contentdownload.ContentDataOf(probe.Content).(*VideoInfo); ok {
		return info
	}
	if info, ok := contentdownload.ContentDataOf(probe.Content).(VideoInfo); ok {
		return &info
	}
	return nil
}

func playHasMedia(play PlayURLData) bool {
	return (play.DASH != nil && (len(play.DASH.Video) > 0 || len(play.DASH.Audio) > 0)) || len(play.DURL) > 0
}

func bestVideosByQuality(dash *DASH) map[int]DASHStream {
	out := map[int]DASHStream{}
	if dash == nil {
		return out
	}
	for _, stream := range dash.Video {
		if stream.ID == 0 || stream.URL() == "" {
			continue
		}
		existing, ok := out[stream.ID]
		if !ok || preferVideoStream(stream, existing) {
			out[stream.ID] = stream
		}
	}
	return out
}

func selectBestVideoByQuality(dash *DASH, qn int) *DASHStream {
	if dash == nil {
		return nil
	}
	var best *DASHStream
	for i := range dash.Video {
		stream := dash.Video[i]
		if stream.URL() == "" || (qn > 0 && stream.ID != qn) {
			continue
		}
		if best == nil || preferVideoStream(stream, *best) {
			copy := stream
			best = &copy
		}
	}
	return best
}

func preferVideoStream(a, b DASHStream) bool {
	aAVC := strings.Contains(strings.ToLower(a.Codecs), "avc1")
	bAVC := strings.Contains(strings.ToLower(b.Codecs), "avc1")
	if aAVC != bAVC {
		return aAVC
	}
	if a.Bandwidth != b.Bandwidth {
		return a.Bandwidth > b.Bandwidth
	}
	return a.Width*a.Height > b.Width*b.Height
}

func selectBestAudio(dash *DASH) *DASHStream {
	if dash == nil {
		return nil
	}
	candidates := append([]DASHStream(nil), dash.Audio...)
	if dash.Dolby != nil {
		candidates = append(candidates, dash.Dolby.Audio...)
	}
	if dash.Flac != nil && dash.Flac.Audio != nil {
		candidates = append(candidates, *dash.Flac.Audio)
	}
	var best *DASHStream
	for i := range candidates {
		stream := candidates[i]
		if stream.URL() == "" {
			continue
		}
		if best == nil || stream.Bandwidth > best.Bandwidth {
			copy := stream
			best = &copy
		}
	}
	return best
}

func isDASHVariant(variant *contentdownload.Variant) bool {
	if variant == nil {
		return false
	}
	if variant.Metadata != nil {
		if typ, ok := variant.Metadata["format_type"].(string); ok && typ == "dash" {
			return true
		}
	}
	return strings.HasPrefix(variant.ID, "video_qn_")
}

func qualityOfVariant(variant *contentdownload.Variant) int {
	if variant == nil {
		return 0
	}
	if variant.Spec != "" {
		return parsePositiveInt(variant.Spec, 0)
	}
	if variant.Metadata != nil {
		switch v := variant.Metadata["quality"].(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			return parsePositiveInt(v, 0)
		}
	}
	return 0
}

func dashSource(kind string, stream DASHStream, headers map[string]string, hasVideo bool, hasAudio bool) contentdownload.MultiSourceSpec {
	ext := "m4s"
	if hasAudio {
		ext = "m4a"
	}
	return contentdownload.MultiSourceSpec{
		ID:        fmt.Sprintf("%s_%d", kind, stream.ID),
		URL:       stream.URL(),
		Method:    http.MethodGet,
		Headers:   cloneStringMap(headers),
		Ext:       ext,
		ChunkSize: bilibiliHTTPChunkSize,
		MimeType:  stream.Mime(),
		HasVideo:  hasVideo,
		HasAudio:  hasAudio,
	}
}

func bilibiliHTTPDownloadSpec(rawURL string, headers map[string]string) contentdownload.DownloadSpec {
	return contentdownload.DownloadSpec{
		URL:         rawURL,
		Method:      http.MethodGet,
		Protocol:    "http",
		Connections: 4,
		Headers:     cloneStringMap(headers),
		ChunkSize:   bilibiliHTTPChunkSize,
	}
}

func bilibiliMergedDownloadSpec(bvid string, formatIDs []string, sources []contentdownload.MultiSourceSpec) contentdownload.DownloadSpec {
	body, _ := json.Marshal(sources)
	return contentdownload.DownloadSpec{
		URL:         "multi-http://bilibili/" + url.PathEscape(firstNonEmpty(bvid, "video")) + "?formats=" + url.QueryEscape(strings.Join(formatIDs, "+")),
		Method:      http.MethodGet,
		Protocol:    contentdownload.ProtocolMultiHTTP,
		Body:        body,
		Connections: len(sources),
	}
}

func selectDURL(values []DURL) *DURL {
	if len(values) == 0 {
		return nil
	}
	best := values[0]
	for _, value := range values[1:] {
		if value.Size > best.Size {
			best = value
		}
	}
	return &best
}

func qualityLabelMap(play PlayURLData) map[int]string {
	out := map[int]string{}
	for i, quality := range play.AcceptQuality {
		if i < len(play.AcceptDescription) && play.AcceptDescription[i] != "" {
			out[quality] = play.AcceptDescription[i]
		}
	}
	for _, format := range play.SupportFormats {
		label := firstNonEmpty(format.NewDescription, format.DisplayDesc, format.Format)
		if format.Quality != 0 && label != "" {
			out[format.Quality] = label
		}
	}
	return out
}

func qualityLabel(qn int, labels map[int]string) string {
	if label := strings.TrimSpace(labels[qn]); label != "" {
		return label
	}
	switch qn {
	case 127:
		return "8K"
	case 126:
		return "杜比视界"
	case 125:
		return "HDR 真彩"
	case 120:
		return "4K"
	case 116:
		return "1080P60"
	case 112:
		return "1080P+"
	case 80:
		return "1080P"
	case 74:
		return "720P60"
	case 64:
		return "720P"
	case 32:
		return "480P"
	case 16:
		return "360P"
	case 6:
		return "240P"
	default:
		if qn > 0 {
			return strconv.Itoa(qn)
		}
		return "默认"
	}
}

func durlSize(values []DURL) int64 {
	var total int64
	for _, value := range values {
		total += value.Size
	}
	return total
}

func directURLs(sources []contentdownload.MultiSourceSpec) []string {
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		out = append(out, source.URL)
	}
	return out
}

func bilibiliSpaceURL(webBaseURL string, mid int64) string {
	if mid <= 0 {
		return ""
	}
	parsed, err := url.Parse(webBaseURL)
	if err != nil {
		return ""
	}
	parsed.Path = "/space/" + strconv.FormatInt(mid, 10)
	parsed.RawQuery = ""
	return parsed.String()
}

func suffixFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".mp4", ".m4a", ".mp3":
		return ext
	default:
		return ""
	}
}

func pathEscape(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), "%2F", "/")
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
