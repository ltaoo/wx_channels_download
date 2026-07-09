package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

const (
	opusHTMLVariantID             = "html"
	opusInitialStateJSONVariantID = "initial_state_json"
)

type OpusInfo struct {
	ID                string          `json:"id,omitempty"`
	Title             string          `json:"title,omitempty"`
	Description       string          `json:"description,omitempty"`
	AuthorName        string          `json:"author_name,omitempty"`
	AuthorID          string          `json:"author_id,omitempty"`
	AuthorAvatarURL   string          `json:"author_avatar_url,omitempty"`
	AuthorHomepageURL string          `json:"author_homepage_url,omitempty"`
	CoverURL          string          `json:"cover_url,omitempty"`
	WebpageURL        string          `json:"webpage_url,omitempty"`
	ArticleID         string          `json:"article_id,omitempty"`
	ArticleType       int             `json:"article_type,omitempty"`
	PublishedAt       string          `json:"published_at,omitempty"`
	PageHTML          string          `json:"-"`
	InitialState      json.RawMessage `json:"-"`
}

type opusInitialState struct {
	ID     string `json:"id"`
	Detail struct {
		IDStr string `json:"id_str"`
		Basic struct {
			Title       string `json:"title"`
			UID         any    `json:"uid"`
			RIDStr      string `json:"rid_str"`
			ArticleType int    `json:"article_type"`
		} `json:"basic"`
		Modules []opusModule `json:"modules"`
	} `json:"detail"`
}

type opusModule struct {
	ModuleType   string            `json:"module_type"`
	ModuleTop    *opusModuleTop    `json:"module_top"`
	ModuleTitle  *opusModuleTitle  `json:"module_title"`
	ModuleAuthor *opusModuleAuthor `json:"module_author"`
	ModuleBottom *opusModuleBottom `json:"module_bottom"`
}

type opusModuleTop struct {
	Display *opusTopDisplay `json:"display"`
}

type opusTopDisplay struct {
	Album *opusAlbum `json:"album"`
}

type opusAlbum struct {
	Pics []opusPicture `json:"pics"`
}

type opusPicture struct {
	URL string `json:"url"`
}

type opusModuleTitle struct {
	Text string `json:"text"`
}

type opusModuleAuthor struct {
	Face    string `json:"face"`
	Name    string `json:"name"`
	MID     any    `json:"mid"`
	JumpURL string `json:"jump_url"`
	PubTime string `json:"pub_time"`
	PubTS   any    `json:"pub_ts"`
}

type opusModuleBottom struct {
	ShareInfo *opusShareInfo `json:"share_info"`
}

type opusShareInfo struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Pic     string `json:"pic"`
}

func (c *Client) ProbeOpus(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	info, err := c.ExtractOpus(ctx, input.URL)
	if err != nil {
		return nil, err
	}
	title := firstNonEmpty(info.Title, "bilibili_opus_"+info.ID)
	metadata := map[string]any{
		"opus_id":             info.ID,
		"article_id":          info.ArticleID,
		"article_type":        info.ArticleType,
		"author_id":           info.AuthorID,
		"author_homepage_url": info.AuthorHomepageURL,
		"source_url":          info.WebpageURL,
	}
	output := map[string]any{
		"format":        "html",
		"content_type":  "article",
		"opus_id":       info.ID,
		"article_id":    info.ArticleID,
		"title":         title,
		"source_url":    info.WebpageURL,
		"canonical_url": info.WebpageURL,
	}
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: info.WebpageURL,
		ContentID:    info.ID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "article",
			ID:              info.ID,
			Title:           title,
			Description:     info.Description,
			Author:          info.AuthorName,
			URL:             info.WebpageURL,
			SourceURL:       info.WebpageURL,
			AuthorNickname:  info.AuthorName,
			AuthorAvatarURL: info.AuthorAvatarURL,
			CoverURL:        info.CoverURL,
		}, info, metadata, output),
		Variants: buildOpusVariants(info.InitialState),
		Defaults: contentdownload.Defaults{VariantID: opusHTMLVariantID, Suffix: ".html"},
		Internal: map[string]any{
			"article_info": info,
			"pagehtml":     info.PageHTML,
			"pagejson":     json.RawMessage(append([]byte(nil), info.InitialState...)),
		},
	}, nil
}

func (c *Client) ExtractOpus(ctx context.Context, rawURL string) (*OpusInfo, error) {
	opusID, ok := ExtractOpusID(rawURL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	pageHTML, err := c.fetchOpusPage(ctx, opusID)
	if err != nil {
		return nil, err
	}
	stateJSON, err := extractOpusInitialStateJSON(pageHTML)
	if err != nil {
		return nil, fmt.Errorf("extract bilibili opus initial state: %w", err)
	}
	info, err := opusInfoFromInitialState(c.webBaseURL(), opusID, pageHTML, stateJSON)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (c *Client) fetchOpusPage(ctx context.Context, opusID string) (string, error) {
	pageURL := c.opusURL(opusID)
	req, err := c.newWebPageRequest(ctx, pageURL)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("bilibili opus page request failed: %s", resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (c *Client) newWebPageRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{
		"User-Agent":      firstNonEmpty(c.UserAgent, defaultUserAgent),
		"Referer":         c.webBaseURL() + "/",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Cache-Control":   "no-cache",
	}
	if cookie := strings.TrimSpace(c.Cookie); cookie != "" {
		headers["Cookie"] = cookie
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return req, nil
}

func (c *Client) opusURL(opusID string) string {
	base, err := url.Parse(c.webBaseURL())
	if err != nil {
		return strings.TrimRight(defaultWebBaseURL, "/") + "/opus/" + pathEscape(opusID)
	}
	base.Path = "/opus/" + pathEscape(opusID)
	base.RawQuery = ""
	base.Fragment = ""
	return base.String()
}

func (c *Client) resolveOpus(ctx context.Context, input contentdownload.ResolveInput, probe *contentdownload.Probe) (*contentdownload.ResolvedRequest, error) {
	_ = ctx
	variant, err := contentdownload.SelectVariant(probe, input.Options)
	if err != nil {
		return nil, err
	}
	info := opusInfoFromProbe(probe)
	if info == nil {
		info = opusInfoFromSummary(probe)
	}
	if isOpusInitialStateJSONVariant(variant) || variant.ID == "json" {
		return c.resolveOpusInitialStateJSON(input, probe, info, variant)
	}
	pageHTML := opusPageHTMLFromProbe(probe, info)
	if strings.TrimSpace(pageHTML) == "" {
		return nil, fmt.Errorf("missing bilibili opus page html")
	}
	summary := opusResolvedSummary(probe, info)
	contentID := firstNonEmpty(probe.ContentID, summary.ID, info.ID)
	title := firstNonEmpty(summary.Title, contentID, "bilibili_opus")
	sourceURL := firstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, info.WebpageURL, sourceURL)
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".html")
	filename := firstNonEmpty(input.Options.Filename, title, contentID)
	contentMetadata := cloneAnyMap(contentdownload.ContentMetadataOf(probe.Content))
	contentOutput := cloneAnyMap(contentdownload.ContentOutputOf(probe.Content))
	contentOutput["body_html"] = pageHTML
	metadata := cloneAnyMap(contentMetadata)
	metadata["variant_id"] = variant.ID
	metadata["content_type"] = "article"
	metadata["source_url"] = sourceURL
	metadata["canonical_url"] = canonicalURL
	metadata["body_html"] = pageHTML
	return &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         "inline-html://bilibili/opus/" + url.PathEscape(contentID),
			Method:      http.MethodGet,
			Protocol:    "inline_html",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"opus_id":      contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": "article",
		},
		Metadata: metadata,
		Content:  contentdownload.NewContent(summary, info, contentMetadata, contentOutput),
		Internal: map[string]any{
			"article_info": info,
		},
	}, nil
}

func (c *Client) resolveOpusInitialStateJSON(input contentdownload.ResolveInput, probe *contentdownload.Probe, info *OpusInfo, variant *contentdownload.Variant) (*contentdownload.ResolvedRequest, error) {
	raw := opusInitialStateJSONFromProbe(probe, info)
	if len(raw) == 0 {
		return nil, fmt.Errorf("missing bilibili opus initial state json")
	}
	summary := opusResolvedSummary(probe, info)
	contentID := firstNonEmpty(probe.ContentID, summary.ID, info.ID)
	title := firstNonEmpty(summary.Title, contentID, "bilibili_opus")
	sourceURL := firstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, info.WebpageURL, sourceURL)
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".json")
	filename := firstNonEmpty(input.Options.Filename, title, contentID)
	metadata := cloneAnyMap(contentdownload.ContentMetadataOf(probe.Content))
	metadata["variant_id"] = variant.ID
	metadata["content_type"] = "article"
	metadata["source_url"] = sourceURL
	metadata["canonical_url"] = canonicalURL
	metadata["json"] = json.RawMessage(append([]byte(nil), raw...))
	return &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         "inline-json://bilibili/opus/" + url.PathEscape(contentID) + "/initial-state",
			Method:      http.MethodGet,
			Protocol:    "inline_json",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"opus_id":      contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": "article",
		},
		Metadata: metadata,
		Content:  contentdownload.NewContent(summary, info, contentdownload.ContentMetadataOf(probe.Content), contentdownload.ContentOutputOf(probe.Content)),
		Internal: map[string]any{
			"article_info": info,
		},
	}, nil
}

func ExtractOpusID(rawURL string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if !isBilibiliHost(host) {
		return "", false
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	for i, segment := range segments {
		value, _ := url.PathUnescape(segment)
		if !strings.EqualFold(value, "opus") || i+1 >= len(segments) {
			continue
		}
		next, _ := url.PathUnescape(segments[i+1])
		if isDigits(next) {
			return next, true
		}
		return "", false
	}
	return "", false
}

func extractOpusInitialStateJSON(pageHTML string) (json.RawMessage, error) {
	idx := strings.Index(pageHTML, "window.__INITIAL_STATE__")
	if idx < 0 {
		return nil, fmt.Errorf("window.__INITIAL_STATE__ not found")
	}
	rest := pageHTML[idx:]
	eq := strings.Index(rest, "=")
	if eq < 0 {
		return nil, fmt.Errorf("window.__INITIAL_STATE__ assignment not found")
	}
	raw, err := extractJSONValue([]byte(rest[eq+1:]))
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("window.__INITIAL_STATE__ is not valid json")
	}
	return json.RawMessage(append([]byte(nil), raw...)), nil
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

func opusInfoFromInitialState(webBaseURL string, requestedID string, pageHTML string, raw json.RawMessage) (*OpusInfo, error) {
	var state opusInitialState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	var moduleTitle, shareTitle, summary, coverURL string
	var author opusModuleAuthor
	for _, module := range state.Detail.Modules {
		if module.ModuleTitle != nil && strings.TrimSpace(module.ModuleTitle.Text) != "" && moduleTitle == "" {
			moduleTitle = module.ModuleTitle.Text
		}
		if module.ModuleAuthor != nil && author.Name == "" && opusMIDString(author.MID) == "" {
			author = *module.ModuleAuthor
		}
		if module.ModuleTop != nil && module.ModuleTop.Display != nil && module.ModuleTop.Display.Album != nil && coverURL == "" {
			for _, pic := range module.ModuleTop.Display.Album.Pics {
				if strings.TrimSpace(pic.URL) != "" {
					coverURL = pic.URL
					break
				}
			}
		}
		if module.ModuleBottom != nil && module.ModuleBottom.ShareInfo != nil {
			if shareTitle == "" {
				shareTitle = module.ModuleBottom.ShareInfo.Title
			}
			if summary == "" {
				summary = module.ModuleBottom.ShareInfo.Summary
			}
			if coverURL == "" {
				coverURL = module.ModuleBottom.ShareInfo.Pic
			}
		}
	}
	id := firstNonEmpty(requestedID, state.Detail.IDStr, state.ID)
	title := cleanOpusTitle(firstNonEmpty(moduleTitle, shareTitle, state.Detail.Basic.Title, extractHTMLTitle(pageHTML), "bilibili_opus_"+id))
	authorID := firstNonEmpty(opusMIDString(author.MID), opusMIDString(state.Detail.Basic.UID))
	authorHomepage := normalizeBilibiliURL(author.JumpURL, webBaseURL)
	if authorHomepage == "" && authorID != "" {
		authorHomepage = bilibiliSpaceURLString(webBaseURL, authorID)
	}
	info := &OpusInfo{
		ID:                id,
		Title:             title,
		Description:       shortenText(normalizeText(summary), 500),
		AuthorName:        normalizeText(author.Name),
		AuthorID:          authorID,
		AuthorAvatarURL:   normalizeBilibiliURL(author.Face, webBaseURL),
		AuthorHomepageURL: authorHomepage,
		CoverURL:          normalizeBilibiliURL(coverURL, webBaseURL),
		WebpageURL:        strings.TrimRight(webBaseURL, "/") + "/opus/" + pathEscape(id),
		ArticleID:         state.Detail.Basic.RIDStr,
		ArticleType:       state.Detail.Basic.ArticleType,
		PublishedAt:       firstNonEmpty(author.PubTime, opusMIDString(author.PubTS)),
		PageHTML:          pageHTML,
		InitialState:      json.RawMessage(append([]byte(nil), raw...)),
	}
	return info, nil
}

func buildOpusVariants(initialState json.RawMessage) []contentdownload.Variant {
	variants := []contentdownload.Variant{
		{
			ID:     opusHTMLVariantID,
			Type:   "html",
			Label:  "HTML",
			Suffix: ".html",
			Metadata: map[string]any{
				"format":       "html",
				"content_type": "article",
			},
		},
	}
	if len(initialState) > 0 {
		variants = append(variants, contentdownload.Variant{
			ID:     opusInitialStateJSONVariantID,
			Type:   "json",
			Label:  "INITIAL_STATE JSON",
			Suffix: ".json",
			Metadata: map[string]any{
				"format": "json",
				"source": "window.__INITIAL_STATE__",
			},
		})
	}
	return variants
}

func isOpusInitialStateJSONVariant(variant *contentdownload.Variant) bool {
	return variant != nil && variant.ID == opusInitialStateJSONVariantID
}

func opusInfoFromProbe(probe *contentdownload.Probe) *OpusInfo {
	if probe == nil {
		return nil
	}
	if probe.Internal != nil {
		if info, ok := probe.Internal["article_info"].(*OpusInfo); ok {
			return info
		}
	}
	if info, ok := contentdownload.ContentDataOf(probe.Content).(*OpusInfo); ok {
		return info
	}
	if info, ok := contentdownload.ContentDataOf(probe.Content).(OpusInfo); ok {
		return &info
	}
	return nil
}

func opusInfoFromSummary(probe *contentdownload.Probe) *OpusInfo {
	summary := contentdownload.ContentSummaryOf(probe.Content)
	return &OpusInfo{
		ID:              firstNonEmpty(probe.ContentID, summary.ID),
		Title:           summary.Title,
		Description:     summary.Description,
		AuthorName:      firstNonEmpty(summary.AuthorNickname, summary.Author),
		AuthorAvatarURL: summary.AuthorAvatarURL,
		CoverURL:        summary.CoverURL,
		WebpageURL:      firstNonEmpty(probe.CanonicalURL, summary.URL, probe.SourceURL),
	}
}

func opusPageHTMLFromProbe(probe *contentdownload.Probe, info *OpusInfo) string {
	if info != nil && strings.TrimSpace(info.PageHTML) != "" {
		return info.PageHTML
	}
	if probe != nil && probe.Internal != nil {
		if pageHTML, _ := probe.Internal["pagehtml"].(string); strings.TrimSpace(pageHTML) != "" {
			return pageHTML
		}
	}
	return ""
}

func opusInitialStateJSONFromProbe(probe *contentdownload.Probe, info *OpusInfo) json.RawMessage {
	if info != nil && len(info.InitialState) > 0 {
		return info.InitialState
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

func opusResolvedSummary(probe *contentdownload.Probe, info *OpusInfo) contentdownload.ContentSummary {
	summary := contentdownload.ContentSummaryOf(probe.Content)
	if info == nil {
		return summary
	}
	return contentdownload.ContentSummary{
		Platform:        PlatformID,
		Type:            "article",
		ID:              firstNonEmpty(summary.ID, probe.ContentID, info.ID),
		Title:           firstNonEmpty(summary.Title, info.Title),
		Description:     firstNonEmpty(summary.Description, info.Description),
		Author:          firstNonEmpty(summary.Author, summary.AuthorNickname, info.AuthorName),
		URL:             firstNonEmpty(summary.URL, probe.CanonicalURL, info.WebpageURL),
		SourceURL:       firstNonEmpty(summary.SourceURL, probe.CanonicalURL, info.WebpageURL, probe.SourceURL),
		AuthorNickname:  firstNonEmpty(summary.AuthorNickname, info.AuthorName),
		AuthorAvatarURL: firstNonEmpty(summary.AuthorAvatarURL, info.AuthorAvatarURL),
		CoverURL:        firstNonEmpty(summary.CoverURL, info.CoverURL),
	}
}

func normalizeBilibiliURL(rawURL string, webBaseURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	if strings.HasPrefix(rawURL, "/") {
		return strings.TrimRight(webBaseURL, "/") + rawURL
	}
	return rawURL
}

func bilibiliSpaceURLString(webBaseURL string, mid string) string {
	mid = strings.TrimSpace(mid)
	if mid == "" {
		return ""
	}
	if value, err := strconv.ParseInt(mid, 10, 64); err == nil {
		return bilibiliSpaceURL(webBaseURL, value)
	}
	return strings.TrimRight(webBaseURL, "/") + "/space/" + pathEscape(mid)
}

func opusMIDString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		if v <= 0 {
			return ""
		}
		return strconv.FormatInt(int64(v), 10)
	case int64:
		if v <= 0 {
			return ""
		}
		return strconv.FormatInt(v, 10)
	case int:
		if v <= 0 {
			return ""
		}
		return strconv.Itoa(v)
	default:
		return fmt.Sprint(v)
	}
}

func cleanOpusTitle(value string) string {
	value = normalizeText(value)
	for _, suffix := range []string{" - 哔哩哔哩", "_哔哩哔哩_bilibili"} {
		value = strings.TrimSuffix(value, suffix)
	}
	return strings.TrimSpace(value)
}

func extractHTMLTitle(pageHTML string) string {
	lower := strings.ToLower(pageHTML)
	start := strings.Index(lower, "<title")
	if start < 0 {
		return ""
	}
	tagEnd := strings.Index(lower[start:], ">")
	if tagEnd < 0 {
		return ""
	}
	contentStart := start + tagEnd + 1
	end := strings.Index(lower[contentStart:], "</title>")
	if end < 0 {
		return ""
	}
	return pageHTML[contentStart : contentStart+end]
}

func normalizeText(value string) string {
	value = htmlpkg.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}

func shortenText(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func cloneAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
