package qidian

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
	qidianpkg "wx_channel/pkg/scraper/qidian"
)

const PlatformID = qidianpkg.PlatformID

type Fetcher interface {
	FetchBookProfile(id string) (*BookProfile, error)
}

type Handler struct {
	Fetcher Fetcher
}

type parsedURL struct {
	BookID    string
	Canonical string
}

func New(fetcher Fetcher) *Handler {
	if fetcher == nil {
		fetcher = NewClient(nil)
	}
	return &Handler{Fetcher: fetcher}
}

func NewClient(client *http.Client) *Client {
	return qidianpkg.NewClient(client)
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	_, ok := ParseURL(rawURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	parts, ok := ParseURL(input.URL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	profile, err := h.Fetcher.FetchBookProfile(parts.BookID)
	if err != nil {
		return nil, fmt.Errorf("fetch qidian book profile: %w", err)
	}
	profile.URL = novelutil.FirstNonEmpty(profile.URL, parts.Canonical)
	title := novelutil.FirstNonEmpty(profile.Title, "qidian_"+parts.BookID)
	bodyHTML := novelutil.RenderBookHTML("起点中文网", novelutil.Book{
		Title:       title,
		URL:         parts.Canonical,
		Author:      profile.Author.Name,
		Category:    novelutil.FirstNonEmpty(profile.Category, profile.SubCategory),
		Status:      profile.Status,
		BookID:      parts.BookID,
		Description: profile.Description,
		CoverURL:    profile.CoverURL,
		Tags:        profile.Tags,
		Chapters:    qidianChapters(profile.Volumes),
	})
	variants := []contentdownload.Variant{novelutil.HTMLVariant("目录 HTML", "novel")}
	if len(profile.PageContextJSON) > 0 {
		variants = append(variants, pageContextJSONVariant())
	}
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: parts.Canonical,
		ContentID:    parts.BookID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "novel",
			ID:              parts.BookID,
			Title:           title,
			Description:     novelutil.FirstNonEmpty(profile.Description, profile.Slogan),
			Author:          profile.Author.Name,
			URL:             parts.Canonical,
			SourceURL:       parts.Canonical,
			AuthorNickname:  profile.Author.Name,
			AuthorAvatarURL: profile.Author.Avatar,
			CoverURL:        profile.CoverURL,
		}, profile, map[string]any{
			"book_id":            parts.BookID,
			"author_id":          profile.Author.ID,
			"author_url":         profile.Author.URL,
			"author_avatar_url":  profile.Author.Avatar,
			"chapter_count":      profile.ChapterCount,
			"word_count":         profile.WordCount,
			"display_word_count": profile.DisplayWordCount,
			"category":           profile.Category,
			"sub_category":       profile.SubCategory,
			"status":             profile.Status,
			"latest_chapter":     profile.LatestChapter.Title,
			"latest_update_at":   profile.LatestUpdateAt,
			"source_url":         parts.Canonical,
		}, ProbeOutput{
			Format:           "html",
			ContentType:      "novel",
			Title:            title,
			Description:      profile.Description,
			Author:           profile.Author.Name,
			AuthorAvatarURL:  profile.Author.Avatar,
			Category:         novelutil.FirstNonEmpty(profile.Category, profile.SubCategory),
			Status:           profile.Status,
			ChapterCount:     profile.ChapterCount,
			WordCount:        profile.WordCount,
			DisplayWordCount: profile.DisplayWordCount,
			LatestChapter:    profile.LatestChapter,
			Volumes:          profile.Volumes,
			SourceURL:        parts.Canonical,
			CanonicalURL:     parts.Canonical,
			BodyHTML:         bodyHTML,
		}.Map()),
		Variants: variants,
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"profile":  profile,
			"pagejson": profile.PageContextJSON,
			"pagehtml": profile.PageHTML,
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
	if isPageContextJSONVariant(variant) {
		return resolvePageContextJSON(ctx, probe, input.Options)
	}
	return novelutil.ResolveInlineHTML(ctx, PlatformID, contentdownload.ResolveInput{
		URL:     input.URL,
		Probe:   probe,
		Options: input.Options,
		Extra:   input.Extra,
	}, h.Probe)
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return novelutil.HTMLPlan(PlatformID), nil
}

func ParseURL(rawURL string) (parsedURL, bool) {
	parts, ok := qidianpkg.ParseURL(rawURL)
	if !ok {
		return parsedURL{}, false
	}
	return parsedURL{
		BookID:    parts.BookID,
		Canonical: parts.Canonical,
	}, true
}

func qidianChapters(volumes []BookVolume) []novelutil.Chapter {
	var out []novelutil.Chapter
	for _, volume := range volumes {
		for _, chapter := range volume.Chapters {
			out = append(out, novelutil.Chapter{Index: chapter.Idx, Title: chapter.Title, URL: chapter.URL})
		}
	}
	return out
}

func pageContextJSONVariant() contentdownload.Variant {
	return contentdownload.Variant{
		ID:     "page_context_json",
		Type:   "json",
		Label:  "PageContext JSON",
		Suffix: ".json",
		Metadata: map[string]any{
			"format": "json",
			"source": "g_data.pageJson",
		},
	}
}

func isPageContextJSONVariant(variant *contentdownload.Variant) bool {
	return variant != nil && variant.ID == "page_context_json"
}

func resolvePageContextJSON(ctx context.Context, probe *contentdownload.Probe, opts contentdownload.Options) (*contentdownload.ResolvedRequest, error) {
	raw := pageContextJSONFromProbe(probe)
	if len(raw) == 0 {
		return nil, fmt.Errorf("missing qidian page json")
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := novelutil.FirstNonEmpty(probe.ContentID, summary.ID)
	title := novelutil.FirstNonEmpty(summary.Title, contentID, "qidian")
	sourceURL := novelutil.FirstNonEmpty(probe.SourceURL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := novelutil.FirstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL)
	filename := novelutil.FirstNonEmpty(opts.Filename, title, contentID)
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       novelutil.FirstNonEmpty(opts.Suffix, ".json"),
		Download: contentdownload.DownloadSpec{
			URL:         "inline-json://qidian/" + contentID + "/page-json",
			Method:      "GET",
			Protocol:    "inline_json",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"book_id":      contentID,
			"title":        title,
			"key":          "0",
			"spec":         "",
			"suffix":       ".json",
			"source_url":   canonicalURL,
			"content_type": novelutil.FirstNonEmpty(summary.Type, "novel"),
		},
		Metadata: map[string]any{
			"variant_id":    "page_context_json",
			"content_type":  novelutil.FirstNonEmpty(summary.Type, "novel"),
			"book_id":       contentID,
			"source_url":    sourceURL,
			"canonical_url": canonicalURL,
			"json":          json.RawMessage(append([]byte(nil), raw...)),
		},
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            novelutil.FirstNonEmpty(summary.Type, "novel"),
			ID:              contentID,
			Title:           title,
			Description:     summary.Description,
			Author:          novelutil.FirstNonEmpty(summary.Author, summary.AuthorNickname),
			URL:             canonicalURL,
			SourceURL:       sourceURL,
			AuthorNickname:  summary.AuthorNickname,
			AuthorAvatarURL: summary.AuthorAvatarURL,
			CoverURL:        summary.CoverURL,
		}, contentdownload.ContentDataOf(probe.Content), contentdownload.ContentMetadataOf(probe.Content), contentdownload.ContentOutputOf(probe.Content)),
	}
	resolved.Pipeline = novelutil.HTMLPlan(PlatformID)
	return resolved, nil
}

func pageContextJSONFromProbe(probe *contentdownload.Probe) json.RawMessage {
	if probe == nil {
		return nil
	}
	if probe.Internal != nil {
		switch raw := probe.Internal["pagejson"].(type) {
		case json.RawMessage:
			return raw
		case []byte:
			return json.RawMessage(raw)
		}
	}
	if profile, _ := contentdownload.ContentDataOf(probe.Content).(*BookProfile); profile != nil {
		return profile.PageContextJSON
	}
	return nil
}
