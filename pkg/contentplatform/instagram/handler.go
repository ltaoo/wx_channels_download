package instagram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/internal/jsonartifact"
	instagrampkg "wx_channel/pkg/scraper/instagram"
)

const PlatformID = "instagram"

type ProfileFetcher interface {
	FetchUserProfile(ctx context.Context, rawURL string, opts instagrampkg.ProfileOptions) (*instagrampkg.ProfilePage, error)
}

type Handler struct {
	Client ProfileFetcher
}

func New(client ProfileFetcher) *Handler {
	if client == nil {
		client = instagrampkg.NewClient()
	}
	return &Handler{Client: client}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return instagrampkg.CanParse(rawURL)
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	target, ok := instagrampkg.ParseProfileURL(instagrampkg.ExtractShareURL(input.URL))
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	page, err := h.Client.FetchUserProfile(ctx, input.URL, profileOptionsFromExtra(input.Extra))
	if err != nil {
		return nil, fmt.Errorf("fetch instagram profile: %w", err)
	}
	if page == nil {
		return nil, fmt.Errorf("fetch instagram profile: empty page")
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
	authorName := firstNonEmpty(page.Profile.FullName, page.Profile.Username, target.Username)
	title := firstNonEmpty(authorName+" Instagram profile", "instagram_"+target.Username)
	description := profileDescription(page)
	previewImages := profilePreviewImages(page, 9)
	coverURL := firstNonEmpty(page.Profile.ProfilePicURLHD, page.Profile.ProfilePicURL, firstString(previewImages))
	output := map[string]any{
		"format":              "json",
		"content_type":        instagrampkg.ContentTypeUserProfile,
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
		"content_count":       page.Profile.MediaCount,
		"followers_count":     page.Profile.FollowersCount,
		"following_count":     page.Profile.FollowingCount,
		"images":              previewImages,
		"image_count":         len(previewImages),
		"warnings":            page.Warnings,
	}
	content := contentdownload.NewContent(contentdownload.ContentSummary{
		Platform:        PlatformID,
		Type:            instagrampkg.ContentTypeUserProfile,
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
		"content_count":       page.Profile.MediaCount,
		"followers_count":     page.Profile.FollowersCount,
		"following_count":     page.Profile.FollowingCount,
	}, output)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: page.URL.Canonical,
		ContentID:    contentID,
		Content:      content,
		Variants:     []contentdownload.Variant{jsonartifact.Variant(instagrampkg.ContentTypeUserProfile)},
		Defaults:     jsonartifact.Defaults(),
		Internal: map[string]any{
			"profile": page,
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
	resolved.Filename = instagrampkg.SanitizeFilename(resolved.Filename)
	return resolved, nil
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return jsonartifact.Plan(PlatformID), nil
}

func profileOptionsFromExtra(extra map[string]any) instagrampkg.ProfileOptions {
	if extra == nil {
		return instagrampkg.ProfileOptions{Count: 12}
	}
	return instagrampkg.ProfileOptions{
		Count:  extraInt(extra, "count", 12),
		Cookie: extraString(extra, "cookie"),
		AppID:  extraString(extra, "app_id"),
	}
}

func profileDescription(page *instagrampkg.ProfilePage) string {
	if page == nil {
		return ""
	}
	if strings.TrimSpace(page.Profile.Biography) != "" {
		return strings.TrimSpace(page.Profile.Biography)
	}
	switch {
	case len(page.Posts) > 0 && page.Profile.MediaCount > 0:
		return fmt.Sprintf("Fetched %d Instagram posts, %d total", len(page.Posts), page.Profile.MediaCount)
	case len(page.Posts) > 0:
		return fmt.Sprintf("Fetched %d Instagram posts", len(page.Posts))
	case page.Profile.MediaCount > 0:
		return fmt.Sprintf("%d Instagram posts", page.Profile.MediaCount)
	default:
		return "Instagram profile"
	}
}

func profilePreviewImages(page *instagrampkg.ProfilePage, limit int) []string {
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
		if add(firstNonEmpty(post.ThumbnailURL, post.DisplayURL, post.VideoURL)) {
			return out
		}
	}
	if add(firstNonEmpty(page.Profile.ProfilePicURLHD, page.Profile.ProfilePicURL)) {
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
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
