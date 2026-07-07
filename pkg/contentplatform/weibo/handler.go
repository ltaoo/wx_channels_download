package weibo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/internal/jsonartifact"
	weibopkg "wx_channel/pkg/scraper/weibo"
)

const PlatformID = "weibo"

type TimelineFetcher interface {
	FetchUserTimeline(ctx context.Context, rawURL string, opts weibopkg.TimelineOptions) (*weibopkg.TimelinePage, error)
}

type Handler struct {
	Client TimelineFetcher
}

func New(client TimelineFetcher) *Handler {
	if client == nil {
		client = weibopkg.NewClient()
	}
	return &Handler{Client: client}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return weibopkg.CanParse(rawURL)
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	target, ok := weibopkg.ParseUserURL(input.URL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	page, err := h.Client.FetchUserTimeline(ctx, input.URL, timelineOptionsFromExtra(input.Extra))
	if err != nil {
		return nil, fmt.Errorf("fetch weibo timeline: %w", err)
	}
	if page == nil {
		return nil, fmt.Errorf("fetch weibo timeline: empty page")
	}
	if strings.TrimSpace(page.URL.UID) == "" {
		page.URL = target
	}
	if strings.TrimSpace(page.URL.Canonical) == "" {
		page.URL.Canonical = target.Canonical
	}
	user := page.User
	if user.IDStr == "" && user.ID > 0 {
		user.IDStr = strconv.FormatInt(user.ID, 10)
	}
	if user.IDStr == "" {
		user.IDStr = target.UID
	}
	userSummary := user.Summary()
	authorName := firstNonEmpty(user.ScreenName, userSummary.ScreenName, user.IDStr, target.UID)
	title := firstNonEmpty(authorName+" 的微博列表", "微博列表_"+target.UID)
	description := timelineDescription(page)
	previewImages := timelinePreviewImages(page.Posts, 9)
	coverURL := firstNonEmpty(userSummary.AvatarURL, firstString(previewImages))
	output := map[string]any{
		"format":              "json",
		"content_type":        weibopkg.ContentTypeUserTimeline,
		"id":                  target.UID,
		"uid":                 target.UID,
		"title":               title,
		"description":         description,
		"text":                description,
		"source_url":          input.URL,
		"canonical_url":       page.URL.Canonical,
		"author_homepage_url": page.URL.Canonical,
		"author_avatar_url":   userSummary.AvatarURL,
		"account_nickname":    authorName,
		"username":            user.IDStr,
		"posts":               page.Posts,
		"post_count":          len(page.Posts),
		"content_count":       firstPositive(page.Total, user.StatusesCount),
		"total":               page.Total,
		"page":                page.Request.Page,
		"since_id":            page.SinceID,
		"images":              previewImages,
		"image_count":         len(previewImages),
	}
	content := contentdownload.NewContent(contentdownload.ContentSummary{
		Platform:        PlatformID,
		Type:            weibopkg.ContentTypeUserTimeline,
		ID:              target.UID,
		Title:           title,
		Description:     description,
		URL:             page.URL.Canonical,
		SourceURL:       input.URL,
		Author:          authorName,
		AuthorNickname:  authorName,
		AuthorAvatarURL: userSummary.AvatarURL,
		CoverURL:        coverURL,
	}, page, map[string]any{
		"uid":                 target.UID,
		"account_external_id": target.UID,
		"account_username":    user.IDStr,
		"author_id":           user.IDStr,
		"author_homepage_url": page.URL.Canonical,
		"author_avatar_url":   userSummary.AvatarURL,
		"post_count":          len(page.Posts),
		"total":               page.Total,
		"page":                page.Request.Page,
		"since_id":            page.SinceID,
	}, output)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: page.URL.Canonical,
		ContentID:    target.UID,
		Content:      content,
		Variants:     []contentdownload.Variant{jsonartifact.Variant(weibopkg.ContentTypeUserTimeline)},
		Defaults:     jsonartifact.Defaults(),
		Internal: map[string]any{
			"timeline": page,
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
	resolved, err := jsonartifact.Resolve(ctx, PlatformID, input, probe, variant)
	if err != nil {
		return nil, err
	}
	resolved.Filename = weibopkg.SanitizeFilename(resolved.Filename)
	return resolved, nil
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return jsonartifact.Plan(PlatformID), nil
}

func timelineOptionsFromExtra(extra map[string]any) weibopkg.TimelineOptions {
	if extra == nil {
		return weibopkg.TimelineOptions{Page: 1}
	}
	return weibopkg.TimelineOptions{
		Page:    extraInt(extra, "page", 1),
		Feature: extraInt(extra, "feature", 0),
		Cookie:  extraString(extra, "cookie"),
	}
}

func timelineDescription(page *weibopkg.TimelinePage) string {
	if page == nil {
		return ""
	}
	count := len(page.Posts)
	total := page.Total
	switch {
	case count > 0 && total > 0:
		return fmt.Sprintf("已获取 %d 条微博，共 %d 条", count, total)
	case count > 0:
		return fmt.Sprintf("已获取 %d 条微博", count)
	case total > 0:
		return fmt.Sprintf("共 %d 条微博", total)
	default:
		return "微博列表"
	}
}

func timelinePreviewImages(posts []weibopkg.PostSummary, limit int) []string {
	if limit <= 0 {
		limit = 9
	}
	out := make([]string, 0, limit)
	seen := map[string]bool{}
	for _, post := range posts {
		for _, imageURL := range post.PicURLs {
			imageURL = strings.TrimSpace(imageURL)
			if imageURL == "" || seen[imageURL] {
				continue
			}
			seen[imageURL] = true
			out = append(out, imageURL)
			if len(out) >= limit {
				return out
			}
		}
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
		if v != 0 {
			return v
		}
	case int64:
		if v != 0 {
			return int(v)
		}
	case float64:
		if v != 0 {
			return int(v)
		}
	case jsonNumber:
		if i, err := strconv.Atoi(v.String()); err == nil && i != 0 {
			return i
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && i != 0 {
			return i
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

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
