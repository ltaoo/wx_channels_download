package x

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/internal/jsonartifact"
	xpkg "wx_channel/pkg/scraper/x"
)

const PlatformID = "x"

type TimelineFetcher interface {
	FetchUserTimeline(ctx context.Context, rawURL string, opts xpkg.TimelineOptions) (*xpkg.TimelinePage, error)
}

type Handler struct {
	Client TimelineFetcher
}

func New(client TimelineFetcher) *Handler {
	if client == nil {
		client = xpkg.NewClient()
	}
	return &Handler{Client: client}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return xpkg.CanParse(rawURL)
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	target, ok := xpkg.ParseProfileURL(xpkg.ExtractShareURL(input.URL))
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	page, err := h.Client.FetchUserTimeline(ctx, input.URL, timelineOptionsFromExtra(input.Extra))
	if err != nil {
		return nil, fmt.Errorf("fetch x timeline: %w", err)
	}
	if page == nil {
		return nil, fmt.Errorf("fetch x timeline: empty page")
	}
	if strings.TrimSpace(page.URL.Username) == "" {
		page.URL = target
	}
	if strings.TrimSpace(page.URL.Canonical) == "" {
		page.URL.Canonical = target.Canonical
	}
	if strings.TrimSpace(page.Profile.Username) == "" {
		page.Profile.Username = target.Username
	}
	contentID := firstNonEmpty(page.Profile.ID, page.Profile.Username, target.Username)
	authorName := firstNonEmpty(page.Profile.Name, page.Profile.Username, target.Username)
	title := firstNonEmpty(authorName+" X timeline", "x_"+target.Username)
	description := timelineDescription(page)
	previewImages := timelinePreviewImages(page, 9)
	coverURL := firstNonEmpty(page.Profile.AvatarURL, firstString(previewImages))
	output := map[string]any{
		"format":              "json",
		"content_type":        xpkg.ContentTypeUserTimeline,
		"id":                  contentID,
		"user_id":             page.Profile.ID,
		"username":            page.Profile.Username,
		"title":               title,
		"description":         description,
		"text":                description,
		"source_url":          input.URL,
		"canonical_url":       page.URL.Canonical,
		"api_url":             page.APIURL,
		"author_homepage_url": page.URL.Canonical,
		"author_avatar_url":   coverURL,
		"account_nickname":    authorName,
		"profile":             page.Profile,
		"posts":               page.Posts,
		"post_count":          len(page.Posts),
		"content_count":       page.Profile.StatusesCount,
		"followers_count":     page.Profile.FollowersCount,
		"following_count":     page.Profile.FollowingCount,
		"media_count":         page.Profile.MediaCount,
		"images":              previewImages,
		"image_count":         len(previewImages),
		"top_cursor":          page.TopCursor,
		"bottom_cursor":       page.BottomCursor,
		"warnings":            page.Warnings,
	}
	content := contentdownload.NewContent(contentdownload.ContentSummary{
		Platform:        PlatformID,
		Type:            xpkg.ContentTypeUserTimeline,
		ID:              contentID,
		Title:           title,
		Description:     description,
		URL:             page.URL.Canonical,
		SourceURL:       input.URL,
		Author:          authorName,
		AuthorNickname:  authorName,
		AuthorAvatarURL: coverURL,
		CoverURL:        coverURL,
	}, page, map[string]any{
		"user_id":             page.Profile.ID,
		"username":            page.Profile.Username,
		"account_external_id": contentID,
		"account_username":    page.Profile.Username,
		"author_id":           contentID,
		"author_homepage_url": page.URL.Canonical,
		"author_avatar_url":   coverURL,
		"post_count":          len(page.Posts),
		"content_count":       page.Profile.StatusesCount,
		"followers_count":     page.Profile.FollowersCount,
		"following_count":     page.Profile.FollowingCount,
		"media_count":         page.Profile.MediaCount,
		"top_cursor":          page.TopCursor,
		"bottom_cursor":       page.BottomCursor,
	}, output)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: page.URL.Canonical,
		ContentID:    contentID,
		Content:      content,
		Variants:     []contentdownload.Variant{jsonartifact.Variant(xpkg.ContentTypeUserTimeline)},
		Defaults:     jsonartifact.Defaults(),
		Internal: map[string]any{
			"timeline": page,
		},
		Warnings: page.Warnings,
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
	resolved, err := jsonartifact.Resolve(ctx, PlatformID, input, probe, variant)
	if err != nil {
		return nil, err
	}
	resolved.Filename = xpkg.SanitizeFilename(resolved.Filename)
	return resolved, nil
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return jsonartifact.Plan(PlatformID), nil
}

func timelineOptionsFromExtra(extra map[string]any) xpkg.TimelineOptions {
	if extra == nil {
		return xpkg.TimelineOptions{Count: 20}
	}
	return xpkg.TimelineOptions{
		Count:       extraInt(extra, "count", 20),
		Cursor:      extraString(extra, "cursor"),
		UserID:      extraString(extra, "user_id"),
		OperationID: extraString(extra, "operation_id"),
		Cookie:      extraString(extra, "cookie"),
		BearerToken: extraString(extra, "bearer_token"),
		GuestToken:  extraString(extra, "guest_token"),
		CSRFToken:   firstNonEmpty(extraString(extra, "csrf_token"), extraString(extra, "ct0")),
	}
}

func timelineDescription(page *xpkg.TimelinePage) string {
	if page == nil {
		return ""
	}
	if strings.TrimSpace(page.Profile.Description) != "" {
		return strings.TrimSpace(page.Profile.Description)
	}
	switch {
	case len(page.Posts) > 0 && page.Profile.StatusesCount > 0:
		return fmt.Sprintf("Fetched %d X posts, %d total", len(page.Posts), page.Profile.StatusesCount)
	case len(page.Posts) > 0:
		return fmt.Sprintf("Fetched %d X posts", len(page.Posts))
	case page.Profile.StatusesCount > 0:
		return fmt.Sprintf("%d X posts", page.Profile.StatusesCount)
	default:
		return "X timeline"
	}
}

func timelinePreviewImages(page *xpkg.TimelinePage, limit int) []string {
	if page == nil {
		return nil
	}
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
	for _, post := range page.Posts {
		if add(firstNonEmpty(post.CoverURL, firstString(post.ImageURLs))) {
			return out
		}
	}
	if add(page.Profile.AvatarURL) {
		return out
	}
	return out
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
	if extra == nil {
		return fallback
	}
	value := extra[key]
	switch v := value.(type) {
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case float64:
		if v > 0 {
			return int(v)
		}
	case jsonNumber:
		if n, err := strconv.Atoi(v.String()); err == nil && n > 0 {
			return n
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

type jsonNumber interface {
	String() string
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstString(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
