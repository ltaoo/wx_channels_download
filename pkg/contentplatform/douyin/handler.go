package douyin

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/internal/jsonartifact"
	douyinpkg "wx_channel/pkg/scraper/douyin"
)

const PlatformID = "douyin"

type Parser interface {
	Parse(ctx context.Context, rawURL string) (*douyinpkg.VideoInfo, error)
}

type ProfileFetcher interface {
	FetchUserProfile(ctx context.Context, rawURL string, opts douyinpkg.ProfileOptions) (*douyinpkg.ProfilePage, error)
}

type defaultParser struct{}

func (defaultParser) Parse(ctx context.Context, rawURL string) (*douyinpkg.VideoInfo, error) {
	return douyinpkg.Parse(ctx, rawURL)
}

func (defaultParser) FetchUserProfile(ctx context.Context, rawURL string, opts douyinpkg.ProfileOptions) (*douyinpkg.ProfilePage, error) {
	return douyinpkg.FetchUserProfile(ctx, rawURL, opts)
}

type Handler struct {
	Parser Parser
}

func New(parser Parser) *Handler {
	if parser == nil {
		parser = defaultParser{}
	}
	return &Handler{Parser: parser}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return douyinpkg.ExtractProfileURL(rawURL) != "" || douyinpkg.ExtractShareURL(rawURL) != ""
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	if profileURL := douyinpkg.ExtractProfileURL(input.URL); profileURL != "" {
		return h.probeProfile(ctx, input, profileURL)
	}
	shareURL := douyinpkg.ExtractShareURL(input.URL)
	if shareURL == "" {
		return nil, contentdownload.ErrUnsupportedURL
	}
	info, err := h.Parser.Parse(ctx, shareURL)
	if err != nil {
		return nil, fmt.Errorf("parse douyin: %w", err)
	}
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: shareURL,
		ContentID:    info.VideoID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "video",
			ID:              info.VideoID,
			Title:           info.Title,
			Author:          firstNonEmpty(info.AuthorNickname, info.AuthorUsername, info.AuthorID),
			URL:             info.URL,
			SourceURL:       shareURL,
			AuthorNickname:  firstNonEmpty(info.AuthorNickname, info.AuthorUsername, info.AuthorID),
			AuthorAvatarURL: info.AuthorAvatarURL,
			CoverURL:        info.CoverURL,
		}, info, map[string]any{
			"author_id":       info.AuthorID,
			"author_username": info.AuthorUsername,
			"author_sec_id":   info.AuthorSecID,
		}, ProbeOutput{}.Map()),
		Variants: []contentdownload.Variant{
			{ID: "video", Type: "video", Label: "视频", Suffix: ".mp4"},
			{ID: "audio_mp3", Type: "audio", Label: "MP3", Suffix: ".mp3", Requires: []string{"ffmpeg"}},
			{ID: "cover", Type: "image", Label: "封面", Suffix: ".jpg"},
		},
		Defaults: contentdownload.Defaults{VariantID: "video", Suffix: ".mp4"},
		Internal: map[string]any{
			"user_agent": info.UserAgent,
			"pagejson":   info,
		},
	}, nil
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	probe := input.Probe
	if probe == nil {
		var err error
		probe, err = h.Probe(ctx, contentdownload.ProbeInput{URL: input.URL, Extra: input.Extra})
		if err != nil {
			return nil, err
		}
	}
	variant, err := contentdownload.SelectVariant(probe, input.Options)
	if err != nil {
		return nil, err
	}
	if contentdownload.ContentType(probe.Content) == douyinpkg.ContentTypeUserProfile {
		resolved, err := jsonartifact.Resolve(ctx, PlatformID, input, probe, variant)
		if err != nil {
			return nil, err
		}
		resolved.Filename = douyinpkg.SanitizeFilename(resolved.Filename)
		return resolved, nil
	}
	info, _ := contentdownload.ContentDataOf(probe.Content).(*douyinpkg.VideoInfo)
	if info == nil {
		return nil, contentdownload.ErrResolveUnavailable
	}

	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".mp4")
	downloadURL := info.URL
	if variant.ID == "cover" {
		suffix = ".jpg"
		downloadURL = info.CoverURL
	}
	filename := firstNonEmpty(input.Options.Filename, info.Title, info.VideoID)

	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    probe.SourceURL,
		CanonicalURL: probe.CanonicalURL,
		ContentID:    info.VideoID,
		Title:        info.Title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         downloadURL,
			Method:      "GET",
			Protocol:    "http",
			Connections: 4,
			Headers: map[string]string{
				"User-Agent":      info.UserAgent,
				"Referer":         "https://www.douyin.com/",
				"Accept":          "*/*",
				"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			},
		},
		Labels: map[string]string{
			"platform":   PlatformID,
			"id":         info.VideoID,
			"title":      info.Title,
			"key":        "0",
			"spec":       variant.Spec,
			"suffix":     suffix,
			"source_url": probe.CanonicalURL,
		},
		Metadata: map[string]any{
			"variant_id": variant.ID,
			"author_id":  info.AuthorID,
			"author":     firstNonEmpty(info.AuthorNickname, info.AuthorUsername),
		},
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "video",
			ID:              info.VideoID,
			Title:           info.Title,
			URL:             info.URL,
			SourceURL:       probe.CanonicalURL,
			Author:          contentdownload.ContentAuthor(probe.Content),
			AuthorNickname:  contentdownload.ContentAuthorNickname(probe.Content),
			AuthorAvatarURL: contentdownload.ContentAuthorAvatarURL(probe.Content),
			CoverURL:        info.CoverURL,
		}, info, contentdownload.ContentMetadataOf(probe.Content), nil),
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	if resolved != nil && strings.EqualFold(resolved.Download.Protocol, "inline_json") {
		return jsonartifact.Plan(PlatformID), nil
	}
	nodes := []contentdownload.PipelineNode{
		{ID: "download", Type: "download_asset", Stage: "download"},
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

func (h *Handler) probeProfile(ctx context.Context, input contentdownload.ProbeInput, profileURL string) (*contentdownload.Probe, error) {
	target, ok := douyinpkg.ParseProfileURL(profileURL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	fetcher, _ := h.Parser.(ProfileFetcher)
	if fetcher == nil {
		fetcher = defaultParser{}
	}
	page, err := fetcher.FetchUserProfile(ctx, profileURL, profileOptionsFromExtra(input.Extra))
	if err != nil {
		return nil, fmt.Errorf("fetch douyin profile: %w", err)
	}
	if page == nil {
		return nil, fmt.Errorf("fetch douyin profile: empty page")
	}
	if strings.TrimSpace(page.URL.SecUserID) == "" {
		page.URL = target
	}
	if strings.TrimSpace(page.URL.Canonical) == "" {
		page.URL.Canonical = target.Canonical
	}
	if strings.TrimSpace(page.User.SecUID) == "" {
		page.User.SecUID = target.SecUserID
	}
	contentID := firstNonEmpty(page.User.UID, page.User.SecUID, target.SecUserID)
	authorName := firstNonEmpty(page.User.Nickname, page.User.UniqueID, page.User.UID, target.SecUserID)
	title := firstNonEmpty(authorName+" 的抖音主页", "douyin_"+target.SecUserID)
	description := profileDescription(page)
	previewImages := profilePreviewImages(page.Posts, 9)
	coverURL := firstNonEmpty(page.User.Avatar300URL, page.User.AvatarURL, page.User.CoverURL, firstString(previewImages))
	output := map[string]any{
		"format":              "json",
		"content_type":        douyinpkg.ContentTypeUserProfile,
		"id":                  contentID,
		"uid":                 page.User.UID,
		"sec_user_id":         page.User.SecUID,
		"unique_id":           page.User.UniqueID,
		"title":               title,
		"description":         description,
		"text":                description,
		"source_url":          input.URL,
		"canonical_url":       page.URL.Canonical,
		"api_url":             page.APIURL,
		"author_homepage_url": page.URL.Canonical,
		"author_avatar_url":   coverURL,
		"account_nickname":    authorName,
		"profile":             page.User,
		"posts":               page.Posts,
		"post_count":          len(page.Posts),
		"content_count":       page.User.AwemeCount,
		"followers_count":     page.User.FollowerCount,
		"following_count":     page.User.FollowingCount,
		"has_more":            page.HasMore,
		"max_cursor":          page.MaxCursor,
		"images":              previewImages,
		"image_count":         len(previewImages),
		"warnings":            page.Warnings,
	}
	content := contentdownload.NewContent(contentdownload.ContentSummary{
		Platform:          PlatformID,
		Type:              douyinpkg.ContentTypeUserProfile,
		ID:                contentID,
		Title:             title,
		Description:       description,
		URL:               page.URL.Canonical,
		SourceURL:         input.URL,
		Author:            authorName,
		AuthorNickname:    authorName,
		AuthorAvatarURL:   coverURL,
		AuthorHomepageURL: page.URL.Canonical,
		CoverURL:          coverURL,
	}, page, map[string]any{
		"uid":                 page.User.UID,
		"sec_user_id":         page.User.SecUID,
		"unique_id":           page.User.UniqueID,
		"account_external_id": contentID,
		"account_username":    firstNonEmpty(page.User.UniqueID, page.User.ShortID),
		"author_id":           contentID,
		"author_homepage_url": page.URL.Canonical,
		"author_avatar_url":   coverURL,
		"post_count":          len(page.Posts),
		"content_count":       page.User.AwemeCount,
		"followers_count":     page.User.FollowerCount,
		"following_count":     page.User.FollowingCount,
	}, output)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: page.URL.Canonical,
		ContentID:    contentID,
		Content:      content,
		Variants:     []contentdownload.Variant{jsonartifact.Variant(douyinpkg.ContentTypeUserProfile)},
		Defaults:     jsonartifact.Defaults(),
		Internal: map[string]any{
			"profile": page,
		},
		Warnings: page.Warnings,
	}, nil
}

func profileOptionsFromExtra(extra map[string]any) douyinpkg.ProfileOptions {
	if extra == nil {
		return douyinpkg.ProfileOptions{Count: 18}
	}
	return douyinpkg.ProfileOptions{
		Count:     extraInt(extra, "count", 18),
		MaxCursor: extraInt64(extra, "max_cursor", 0),
		Cookie:    extraString(extra, "cookie"),
		UserAgent: extraString(extra, "user_agent"),
		APIURL:    extraString(extra, "api_url"),
		SkipPage:  extraBool(extra, "skip_page"),
		Extra:     extraStringMap(extra, "query"),
	}
}

func profileDescription(page *douyinpkg.ProfilePage) string {
	if page == nil {
		return ""
	}
	if strings.TrimSpace(page.User.Signature) != "" {
		return strings.TrimSpace(page.User.Signature)
	}
	switch {
	case len(page.Posts) > 0 && page.User.AwemeCount > 0:
		return fmt.Sprintf("已获取 %d 条抖音作品，共 %d 条", len(page.Posts), page.User.AwemeCount)
	case len(page.Posts) > 0:
		return fmt.Sprintf("已获取 %d 条抖音作品", len(page.Posts))
	case page.User.AwemeCount > 0:
		return fmt.Sprintf("共 %d 条抖音作品", page.User.AwemeCount)
	default:
		return "抖音主页"
	}
}

func profilePreviewImages(posts []douyinpkg.AwemeSummary, limit int) []string {
	if limit <= 0 {
		limit = 9
	}
	out := make([]string, 0, limit)
	seen := map[string]bool{}
	add := func(rawURL string) bool {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" || seen[rawURL] {
			return false
		}
		seen[rawURL] = true
		out = append(out, rawURL)
		return len(out) >= limit
	}
	for _, post := range posts {
		if add(post.CoverURL) {
			return out
		}
		for _, imageURL := range post.ImageURLs {
			if add(imageURL) {
				return out
			}
		}
	}
	return out
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func extraString(extra map[string]any, key string) string {
	if extra == nil {
		return ""
	}
	value := extra[key]
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func extraInt(extra map[string]any, key string, fallback int) int {
	value := extraInt64(extra, key, int64(fallback))
	return int(value)
}

func extraInt64(extra map[string]any, key string, fallback int64) int64 {
	if extra == nil {
		return fallback
	}
	value := extra[key]
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case jsonNumber:
		if out, err := strconv.ParseInt(strings.TrimSpace(v.String()), 10, 64); err == nil {
			return out
		}
	case string:
		if out, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return out
		}
	}
	return fallback
}

func extraBool(extra map[string]any, key string) bool {
	if extra == nil {
		return false
	}
	value := extra[key]
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
	default:
		return false
	}
}

func extraStringMap(extra map[string]any, key string) map[string]string {
	if extra == nil {
		return nil
	}
	value, ok := extra[key]
	if !ok {
		return nil
	}
	out := map[string]string{}
	switch typed := value.(type) {
	case map[string]string:
		for k, v := range typed {
			if strings.TrimSpace(k) != "" {
				out[k] = v
			}
		}
	case map[string]any:
		for k, v := range typed {
			if strings.TrimSpace(k) != "" {
				out[k] = strings.TrimSpace(fmt.Sprint(v))
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type jsonNumber interface {
	String() string
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
