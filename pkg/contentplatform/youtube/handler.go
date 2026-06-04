package youtube

import (
	"context"
	"net/url"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

const PlatformID = "youtube"

type Resolver interface {
	Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error)
	Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error)
}

type Handler struct {
	Resolver Resolver
}

func New(resolver Resolver) *Handler {
	return &Handler{Resolver: resolver}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	_, ok := ExtractVideoID(rawURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	if h.Resolver != nil {
		return h.Resolver.Probe(ctx, input)
	}
	videoID, ok := ExtractVideoID(input.URL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	canonicalURL := "https://www.youtube.com/watch?v=" + videoID
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: canonicalURL,
		ContentID:    videoID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:  PlatformID,
			Type:      "video",
			ID:        videoID,
			Title:     "youtube_" + videoID,
			URL:       canonicalURL,
			SourceURL: canonicalURL,
		}, map[string]string{"video_id": videoID}, map[string]any{"video_id": videoID}, nil),
		Variants: []contentdownload.Variant{
			{
				ID:       "external_resolver",
				Type:     "video",
				Label:    "外部解析器",
				Suffix:   ".mp4",
				Requires: []string{"youtube_resolver"},
			},
		},
		Defaults: contentdownload.Defaults{VariantID: "external_resolver", Suffix: ".mp4"},
		Warnings: []string{"youtube resolver is not configured"},
	}, nil
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	if h.Resolver != nil {
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
	return nil, contentdownload.ErrResolveUnavailable
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
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

func ExtractVideoID(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "youtu.be":
		id := strings.Trim(parsed.EscapedPath(), "/")
		return id, id != ""
	case host == "youtube.com" || host == "www.youtube.com" || host == "m.youtube.com":
		if parsed.EscapedPath() == "/watch" {
			id := parsed.Query().Get("v")
			return id, id != ""
		}
		if strings.HasPrefix(parsed.EscapedPath(), "/shorts/") {
			id := strings.TrimPrefix(parsed.EscapedPath(), "/shorts/")
			id = strings.Trim(id, "/")
			return id, id != ""
		}
		if strings.HasPrefix(parsed.EscapedPath(), "/embed/") {
			id := strings.TrimPrefix(parsed.EscapedPath(), "/embed/")
			id = strings.Trim(id, "/")
			return id, id != ""
		}
	}
	return "", false
}
