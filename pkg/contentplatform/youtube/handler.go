package youtube

import (
	"context"
	"fmt"
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
	if resolver == nil {
		resolver = NewClient(nil)
	}
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
	if resolved == nil {
		return nil, fmt.Errorf("youtube resolver returned empty request")
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
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false
	}
	if isLikelyVideoID(rawURL) {
		return rawURL, true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.Trim(parsed.EscapedPath(), "/")
	switch {
	case host == "youtu.be" || host == "www.youtu.be":
		id := firstPathSegment(path)
		return id, isLikelyVideoID(id)
	case isYouTubeHost(host):
		if parsed.EscapedPath() == "/watch" {
			id := parsed.Query().Get("v")
			return id, isLikelyVideoID(id)
		}
		for _, prefix := range []string{"shorts/", "embed/", "v/", "live/"} {
			if strings.HasPrefix(path, prefix) {
				id := firstPathSegment(strings.TrimPrefix(path, prefix))
				return id, isLikelyVideoID(id)
			}
		}
		if parsed.Query().Get("video_id") != "" {
			id := parsed.Query().Get("video_id")
			return id, isLikelyVideoID(id)
		}
	}
	return "", false
}

func isYouTubeHost(host string) bool {
	return host == "youtube.com" ||
		host == "www.youtube.com" ||
		host == "m.youtube.com" ||
		host == "music.youtube.com" ||
		host == "youtube-nocookie.com" ||
		host == "www.youtube-nocookie.com"
}

func firstPathSegment(path string) string {
	if i := strings.Index(path, "/"); i >= 0 {
		return path[:i]
	}
	return path
}

func isLikelyVideoID(value string) bool {
	if len(value) != 11 || strings.ContainsAny(value, "/:?&=#") {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
