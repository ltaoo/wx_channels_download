package douyin

import (
	"context"
	"fmt"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	douyinpkg "wx_channel/pkg/douyin"
)

const PlatformID = "douyin"

type Parser interface {
	Parse(ctx context.Context, rawURL string) (*douyinpkg.VideoInfo, error)
}

type defaultParser struct{}

func (defaultParser) Parse(ctx context.Context, rawURL string) (*douyinpkg.VideoInfo, error) {
	return douyinpkg.Parse(ctx, rawURL)
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
	return douyinpkg.ExtractShareURL(rawURL) != ""
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
