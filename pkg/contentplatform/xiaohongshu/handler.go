package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	xhspkg "wx_channel/pkg/scraper/xiaohongshu"
)

type NotePageFetcher interface {
	FetchNotePage(ctx context.Context, rawURL string) (*xhspkg.NotePage, error)
}

type Handler struct {
	Client NotePageFetcher
}

func New(client NotePageFetcher) *Handler {
	if client == nil {
		client = xhspkg.NewClient()
	}
	return &Handler{Client: client}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return xhspkg.ExtractShareURL(rawURL) != ""
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	shareURL := xhspkg.ExtractShareURL(input.URL)
	if shareURL == "" {
		return nil, contentdownload.ErrUnsupportedURL
	}
	page, err := h.Client.FetchNotePage(ctx, shareURL)
	if err != nil {
		return nil, fmt.Errorf("fetch xiaohongshu note: %w", err)
	}
	if page == nil || strings.TrimSpace(page.Note.NoteID) == "" {
		return nil, fmt.Errorf("fetch xiaohongshu note: empty note")
	}

	note := page.Note
	noteID := firstNonEmpty(page.URL.NoteID, note.NoteID)
	canonicalURL := firstNonEmpty(page.URL.Canonical, xhspkg.CanonicalNoteURL(noteID))
	sourceURL := firstNonEmpty(page.Source, canonicalURL, shareURL, input.URL)
	title := firstNonEmpty(note.Title, note.Desc, "xiaohongshu_"+noteID)
	contentType := noteContentType(note)
	coverURL := note.CoverURL()
	stream, hasVideo := note.BestVideoStream()
	videoURL := ""
	if hasVideo {
		videoURL = stream.MasterURL
	}
	imageURLs := note.ImageURLs()

	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: canonicalURL,
		ContentID:    noteID,
		Content: NewNoteContentEnvelope(
			contentdownload.ContentSummary{
				Platform:        PlatformID,
				Type:            contentType,
				ID:              noteID,
				Title:           title,
				Description:     note.Desc,
				Author:          firstNonEmpty(note.User.Nickname, note.User.UserID),
				URL:             canonicalURL,
				SourceURL:       sourceURL,
				AuthorNickname:  note.User.Nickname,
				AuthorAvatarURL: note.User.Avatar,
				CoverURL:        coverURL,
				Duration:        noteDuration(note),
			},
			note,
			NoteMetadata{
				NoteID:          noteID,
				XSecToken:       firstNonEmpty(page.URL.XSecToken, note.XSecToken),
				AuthorID:        note.User.UserID,
				AuthorXSecToken: note.User.XSecToken,
				SourceURL:       sourceURL,
				CanonicalURL:    canonicalURL,
				PublishedAt:     note.Time,
				LastUpdateTime:  note.LastUpdateTime,
			},
			NoteOutput{
				Format:       OutputFormatJSON,
				ContentType:  contentType,
				NoteID:       noteID,
				Title:        title,
				SourceURL:    sourceURL,
				CanonicalURL: canonicalURL,
				VideoURL:     videoURL,
				ImageURLs:    imageURLs,
				CoverURL:     coverURL,
			},
		),
		Variants: xiaohongshuVariants(note, page.InitialStateJSON),
		Defaults: xiaohongshuDefaults(note, page.InitialStateJSON),
		Internal: map[string]any{
			"page":     page,
			"pagejson": page.InitialStateJSON,
			"pagehtml": page.PageHTML,
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

	page := notePageFromProbe(probe)
	note := noteFromProbe(probe)
	summary := contentdownload.ContentSummaryOf(probe.Content)
	noteID := firstNonEmpty(probe.ContentID, summary.ID, note.NoteID)
	title := firstNonEmpty(summary.Title, note.Title, note.Desc, noteID)
	filename := xhspkg.SanitizeFilename(firstNonEmpty(input.Options.Filename, title, noteID, "xiaohongshu"))
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, xhspkg.CanonicalNoteURL(noteID))
	sourceURL := firstNonEmpty(probe.SourceURL, summary.SourceURL, input.URL, canonicalURL)
	contentType := firstNonEmpty(summary.Type, noteContentType(note))
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".json")
	download := contentdownload.DownloadSpec{
		URL:         "inline-json://xiaohongshu/" + noteID + "/initial-state",
		Method:      "GET",
		Protocol:    "inline_json",
		Connections: 1,
	}

	metadata := map[string]any{
		"variant_id":        variant.ID,
		"content_type":      contentType,
		"note_id":           noteID,
		"author_id":         note.User.UserID,
		"author_nickname":   note.User.Nickname,
		"author_avatar_url": note.User.Avatar,
		"source_url":        sourceURL,
		"canonical_url":     canonicalURL,
	}
	if isInitialStateJSONVariant(variant) {
		raw := initialStateJSONFromPage(page)
		if len(raw) == 0 {
			return nil, fmt.Errorf("missing xiaohongshu initial state json")
		}
		metadata["json"] = json.RawMessage(append([]byte(nil), raw...))
		suffix = firstNonEmpty(input.Options.Suffix, variant.Suffix, ".json")
	} else {
		download, suffix, err = downloadSpecForVariant(note, noteID, canonicalURL, variant, input.Options)
		if err != nil {
			return nil, err
		}
	}

	resolvedSummary := contentdownload.ContentSummary{
		Platform:        PlatformID,
		Type:            contentType,
		ID:              noteID,
		Title:           title,
		Description:     summary.Description,
		URL:             canonicalURL,
		SourceURL:       sourceURL,
		Author:          firstNonEmpty(summary.Author, summary.AuthorNickname, note.User.Nickname, note.User.UserID),
		AuthorNickname:  firstNonEmpty(summary.AuthorNickname, note.User.Nickname),
		AuthorAvatarURL: firstNonEmpty(summary.AuthorAvatarURL, note.User.Avatar),
		CoverURL:        firstNonEmpty(summary.CoverURL, note.CoverURL()),
		Duration:        contentdownload.ContentDuration(probe.Content),
	}
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    noteID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download:     download,
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           noteID,
			"note_id":      noteID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": contentType,
		},
		Metadata: metadata,
		Content:  xiaohongshuContentWithSummary(probe.Content, resolvedSummary),
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func downloadSpecForVariant(note xhspkg.Note, noteID, referer string, variant *contentdownload.Variant, options contentdownload.Options) (contentdownload.DownloadSpec, string, error) {
	suffix := firstNonEmpty(options.Suffix, variant.Suffix, ".mp4")
	headers := map[string]string{
		"User-Agent":      xhspkg.DefaultUserAgent(),
		"Referer":         firstNonEmpty(referer, xhspkg.SourceURL),
		"Accept":          "*/*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	}
	switch variant.ID {
	case "video", "audio_mp3":
		stream, ok := note.BestVideoStream()
		if !ok || strings.TrimSpace(stream.MasterURL) == "" {
			return contentdownload.DownloadSpec{}, "", contentdownload.ErrResolveUnavailable
		}
		if variant.ID == "audio_mp3" {
			suffix = firstNonEmpty(options.Suffix, variant.Suffix, ".mp3")
		} else {
			suffix = firstNonEmpty(options.Suffix, variant.Suffix, xhspkg.VideoFileSuffix(stream))
		}
		return contentdownload.DownloadSpec{
			URL:         stream.MasterURL,
			Method:      "GET",
			Protocol:    "http",
			Connections: 4,
			Headers:     headers,
		}, suffix, nil
	case "cover", "image":
		imageURL := note.CoverURL()
		if variant.ID == "image" {
			imageURLs := note.ImageURLs()
			if len(imageURLs) > 0 {
				imageURL = imageURLs[0]
			}
		}
		if imageURL == "" {
			return contentdownload.DownloadSpec{}, "", contentdownload.ErrResolveUnavailable
		}
		suffix = firstNonEmpty(options.Suffix, variant.Suffix, xhspkg.ImageFileSuffix(imageURL))
		return contentdownload.DownloadSpec{
			URL:         imageURL,
			Method:      "GET",
			Protocol:    "http",
			Connections: 1,
			Headers:     headers,
		}, suffix, nil
	case "pictures":
		files := zipFilesForImages(noteID, note.ImageURLs())
		if len(files) == 0 {
			return contentdownload.DownloadSpec{}, "", contentdownload.ErrResolveUnavailable
		}
		data, err := json.Marshal(files)
		if err != nil {
			return contentdownload.DownloadSpec{}, "", err
		}
		suffix = firstNonEmpty(options.Suffix, variant.Suffix, ".zip")
		return contentdownload.DownloadSpec{
			URL:         "zip://xiaohongshu/" + url.PathEscape(noteID) + "?files=" + url.QueryEscape(string(data)),
			Method:      "GET",
			Protocol:    "zip",
			Connections: 4,
			Headers:     headers,
		}, suffix, nil
	default:
		return contentdownload.DownloadSpec{}, "", contentdownload.ErrVariantNotFound
	}
}

func zipFilesForImages(noteID string, imageURLs []string) []contentdownload.ZipFileItem {
	files := make([]contentdownload.ZipFileItem, 0, len(imageURLs))
	for i, imageURL := range imageURLs {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			continue
		}
		filename := fmt.Sprintf("%s_%02d%s", firstNonEmpty(noteID, "image"), i+1, xhspkg.ImageFileSuffix(imageURL))
		files = append(files, contentdownload.ZipFileItem{URL: imageURL, Filename: filename})
	}
	return files
}

func xiaohongshuVariants(note xhspkg.Note, initialStateJSON json.RawMessage) []contentdownload.Variant {
	var variants []contentdownload.Variant
	if len(initialStateJSON) > 0 {
		variants = append(variants, initialStateJSONVariant())
	}
	if stream, ok := note.BestVideoStream(); ok && strings.TrimSpace(stream.MasterURL) != "" {
		variants = append(variants,
			contentdownload.Variant{
				ID:      "video",
				Type:    "video",
				Label:   firstNonEmpty(xhspkg.StreamLabel(stream), "视频"),
				Spec:    strconv.Itoa(stream.StreamType),
				Suffix:  xhspkg.VideoFileSuffix(stream),
				Size:    stream.Size,
				Width:   stream.Width,
				Height:  stream.Height,
				Bitrate: stream.AvgBitrate,
			},
			contentdownload.Variant{ID: "audio_mp3", Type: "audio", Label: "MP3", Suffix: ".mp3", Requires: []string{"ffmpeg"}},
		)
		if note.CoverURL() != "" {
			variants = append(variants, contentdownload.Variant{ID: "cover", Type: "image", Label: "封面", Suffix: xhspkg.ImageFileSuffix(note.CoverURL())})
		}
		return variants
	}
	imageURLs := note.ImageURLs()
	if len(imageURLs) == 1 {
		variants = append(variants, contentdownload.Variant{ID: "image", Type: "image", Label: "图片", Suffix: xhspkg.ImageFileSuffix(imageURLs[0])})
	} else if len(imageURLs) > 1 {
		variants = append(variants, contentdownload.Variant{ID: "pictures", Type: "archive", Label: "图集", Suffix: ".zip", Metadata: map[string]any{"count": len(imageURLs)}})
	}
	return variants
}

func xiaohongshuDefaults(note xhspkg.Note, initialStateJSON json.RawMessage) contentdownload.Defaults {
	if stream, ok := note.BestVideoStream(); ok && strings.TrimSpace(stream.MasterURL) != "" {
		return contentdownload.Defaults{VariantID: "video", Spec: strconv.Itoa(stream.StreamType), Suffix: xhspkg.VideoFileSuffix(stream)}
	}
	imageURLs := note.ImageURLs()
	if len(imageURLs) == 1 {
		return contentdownload.Defaults{VariantID: "image", Suffix: xhspkg.ImageFileSuffix(imageURLs[0])}
	}
	if len(imageURLs) > 1 {
		return contentdownload.Defaults{VariantID: "pictures", Suffix: ".zip"}
	}
	if len(initialStateJSON) > 0 {
		return contentdownload.Defaults{VariantID: "initial_state_json", Suffix: ".json"}
	}
	return contentdownload.Defaults{}
}

func initialStateJSONVariant() contentdownload.Variant {
	return contentdownload.Variant{
		ID:     "initial_state_json",
		Type:   OutputFormatJSON,
		Label:  "INITIAL_STATE JSON",
		Suffix: ".json",
		Metadata: map[string]any{
			"format": OutputFormatJSON,
			"source": "window.__INITIAL_STATE__",
		},
	}
}

func isInitialStateJSONVariant(variant *contentdownload.Variant) bool {
	return variant != nil && variant.ID == "initial_state_json"
}

func initialStateJSONFromPage(page *xhspkg.NotePage) json.RawMessage {
	if page == nil || len(page.InitialStateJSON) == 0 {
		return nil
	}
	return page.InitialStateJSON
}

func notePageFromProbe(probe *contentdownload.Probe) *xhspkg.NotePage {
	if probe == nil || probe.Internal == nil {
		return nil
	}
	page, _ := probe.Internal["page"].(*xhspkg.NotePage)
	return page
}

func noteFromProbe(probe *contentdownload.Probe) xhspkg.Note {
	if page := notePageFromProbe(probe); page != nil {
		return page.Note
	}
	note, _ := contentdownload.ContentDataOf(probe.Content).(xhspkg.Note)
	return note
}

func noteContentType(note xhspkg.Note) string {
	if strings.EqualFold(note.Type, "video") {
		return ContentTypeVideo
	}
	if stream, ok := note.BestVideoStream(); ok && strings.TrimSpace(stream.MasterURL) != "" {
		return ContentTypeVideo
	}
	if len(note.ImageURLs()) > 1 {
		return ContentTypeImageAlbum
	}
	return ContentTypeImage
}

func noteDuration(note xhspkg.Note) int64 {
	if note.Video.Capa.Duration > 0 {
		return note.Video.Capa.Duration
	}
	if stream, ok := note.BestVideoStream(); ok {
		if stream.VideoDuration > 0 {
			return stream.VideoDuration / 1000
		}
		if stream.Duration > 0 {
			return stream.Duration / 1000
		}
	}
	return int64(note.Video.Media.Video.Duration)
}

func xiaohongshuContentWithSummary(content any, summary contentdownload.ContentSummary) any {
	switch c := content.(type) {
	case *NoteContentEnvelope:
		next := *c
		next.Summary = summary
		return &next
	default:
		return contentdownload.NewContent(summary, contentdownload.ContentDataOf(content), contentdownload.ContentMetadataOf(content), contentdownload.ContentOutputOf(content))
	}
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	nodes := []contentdownload.PipelineNode{
		{ID: "download", Type: "download_asset", Stage: "download"},
	}
	persistDependsOn := []string{"download"}
	if resolved != nil && resolved.Suffix == ".mp3" {
		nodes = append(nodes, contentdownload.PipelineNode{
			ID:        "transcode_mp3",
			Type:      "ffmpeg_extract_mp3",
			Stage:     "post",
			DependsOn: []string{"download"},
			Args:      map[string]any{"bitrate": "192k"},
		})
		persistDependsOn = []string{"transcode_mp3"}
	}
	nodes = append(nodes, contentdownload.PipelineNode{ID: "persist", Type: "persist_artifacts", Stage: "persist", DependsOn: persistDependsOn})
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
