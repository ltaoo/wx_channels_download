package shuba69

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
	shubapkg "wx_channel/pkg/scraper/69shuba"
)

const PlatformID = shubapkg.PlatformID

const (
	ArchiveProtocol          = "69shuba_archive"
	LocalPDFProtocol         = "69shuba_local_pdf"
	archiveVariantID         = "html"
	localPDFVariantID        = "pdf"
	archiveConcurrency       = 5
	metadataNovelFetchResult = "_novel_fetch_result"
	metadataLocalArchive     = "_local_archive"
)

type Fetcher interface {
	FetchNovelChapters(url string) (*Novel, error)
	FetchChapterContent(url string) (*ChapterContent, error)
}

type novelFetcher interface {
	FetchNovel(url string) (*NovelFetchResult, error)
}

type Handler struct {
	Fetcher Fetcher
}

type parsedURL struct {
	Kind      string
	BookID    string
	ChapterID string
	Canonical string
}

func New(fetcher Fetcher) *Handler {
	if fetcher == nil {
		fetcher = NewClient()
	}
	return &Handler{Fetcher: fetcher}
}

func NewClient() *Client {
	return shubapkg.NewClient(nil)
}

func NewClientWithHTTP(client HTTPClient) *Client {
	return shubapkg.NewHTTPClientWithOptions(client, "", "")
}

func NewClientWithOptions(cookie, userAgent string) *Client {
	return shubapkg.NewClientWithOptions(nil, cookie, userAgent)
}

func NewHTTPClientWithOptions(client HTTPClient, cookie, userAgent string) *Client {
	return shubapkg.NewHTTPClientWithOptions(client, cookie, userAgent)
}

func NewClawreqFetcher() *ClawreqFetcher {
	return shubapkg.NewClawreqFetcher()
}

func NewClientWithHTMLFetcher(fetcher HTMLFetcher, cookie, userAgent string) *Client {
	return shubapkg.NewClientWithHTMLFetcher(fetcher, cookie, userAgent)
}

func NewCDPFetcher(endpoint string) *CDPFetcher {
	return shubapkg.NewCDPFetcher(endpoint)
}

func NewSandboxCDPFetcher(apiBaseURL string, sandboxID string) *SandboxCDPFetcher {
	return shubapkg.NewSandboxCDPFetcher(apiBaseURL, sandboxID)
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	if shubapkg.IsLocalArchiveDir(rawURL) {
		return true
	}
	_, ok := ParseURL(rawURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	if shubapkg.IsLocalArchiveDir(input.URL) {
		return h.probeLocalArchive(input.URL)
	}
	parts, ok := ParseURL(input.URL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	if parts.Kind == shubapkg.ContentTypeChapter {
		return h.probeChapter(input.URL, parts)
	}
	return h.probeNovel(input.URL, parts)
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	if shubapkg.IsLocalArchiveDir(input.URL) {
		return h.resolveLocalArchivePDF(ctx, input)
	}
	sourceURL := novelutil.FirstNonEmpty(input.URL)
	if input.Probe != nil {
		sourceURL = novelutil.FirstNonEmpty(sourceURL, input.Probe.SourceURL, input.Probe.CanonicalURL)
	}
	parts, ok := ParseURL(sourceURL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	if parts.Kind == shubapkg.ContentTypeChapter {
		return novelutil.ResolveInlineHTML(ctx, PlatformID, input, h.Probe)
	}
	return h.resolveNovelArchive(ctx, input, parts)
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	if resolved != nil && strings.EqualFold(resolved.Download.Protocol, LocalPDFProtocol) {
		return shubaLocalPDFPlan(), nil
	}
	if resolved != nil && strings.EqualFold(resolved.Download.Protocol, ArchiveProtocol) {
		return shubaArchivePlan(), nil
	}
	return novelutil.HTMLPlan(PlatformID), nil
}

func (h *Handler) probeLocalArchive(root string) (*contentdownload.Probe, error) {
	result, err := shubapkg.LoadLocalArchive(root)
	if err != nil {
		return nil, err
	}
	novel := result.Novel
	contentID := novelutil.FirstNonEmpty(novel.BookID, filepathBase(root))
	title := novelutil.FirstNonEmpty(novel.Title, filepathBase(root), contentID)
	bodyHTML := shubapkg.BuildNovelHTML(novel)
	warnings := []string{}
	if result.SkippedChapterFiles > 0 {
		warnings = append(warnings, fmt.Sprintf("跳过重复或未匹配章节文件 %d 个", result.SkippedChapterFiles))
	}
	if len(result.MissingChapters) > 0 {
		warnings = append(warnings, fmt.Sprintf("目录中有 %d 个章节未找到本地正文", len(result.MissingChapters)))
	}
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    root,
		CanonicalURL: root,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           shubapkg.ContentTypeNovel,
			ID:             contentID,
			Title:          title,
			Description:    novel.Description,
			Author:         novel.Author,
			URL:            root,
			SourceURL:      root,
			AuthorNickname: novel.Author,
			CoverURL:       novel.CoverURL,
		}, novel, map[string]any{
			"book_id":               contentID,
			"category":              novel.Category,
			"status":                novel.Status,
			"word_count":            novel.WordCount,
			"update_time":           novel.UpdateTime,
			"latest_chapter":        novel.LatestChapter,
			"chapter_count":         len(novel.Chapters),
			"local_root":            root,
			"skipped_chapter_files": result.SkippedChapterFiles,
			"missing_chapter_count": len(result.MissingChapters),
		}, ProbeOutput{
			Format:       "html",
			ContentType:  shubapkg.ContentTypeNovel,
			Title:        title,
			SourceURL:    root,
			CanonicalURL: root,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{shubaLocalPDFVariant()},
		Defaults: contentdownload.Defaults{VariantID: localPDFVariantID, Suffix: ".pdf"},
		Internal: map[string]any{
			metadataLocalArchive:     result,
			metadataNovelFetchResult: result.Fetch,
		},
		Warnings: warnings,
	}, nil
}

func (h *Handler) resolveLocalArchivePDF(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
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
	local := shubaProbeLocalArchive(probe)
	if local == nil {
		return nil, fmt.Errorf("69shuba local archive probe result is missing")
	}
	novel := local.Novel
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := novelutil.FirstNonEmpty(probe.ContentID, summary.ID, novel.BookID, filepathBase(input.URL))
	title := novelutil.FirstNonEmpty(summary.Title, novel.Title, filepathBase(input.URL), contentID)
	filename := novelutil.FirstNonEmpty(input.Options.Filename, title, contentID)
	suffix := novelutil.FirstNonEmpty(input.Options.Suffix, variant.Suffix, ".pdf")
	metadata := cloneAnyMap(contentdownload.ContentMetadataOf(probe.Content))
	metadata["variant_id"] = variant.ID
	metadata["content_type"] = shubapkg.ContentTypeNovel
	metadata["local_root"] = local.RootDir
	metadata["chapter_count"] = len(novel.Chapters)
	metadata["skipped_chapter_files"] = local.SkippedChapterFiles
	metadata["missing_chapter_count"] = len(local.MissingChapters)
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    local.RootDir,
		CanonicalURL: local.RootDir,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:      local.RootDir,
			Method:   "GET",
			Protocol: LocalPDFProtocol,
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   local.RootDir,
			"content_type": shubapkg.ContentTypeNovel,
		},
		Metadata: metadata,
		Content:  probe.Content,
		Internal: map[string]any{
			metadataLocalArchive:     local,
			metadataNovelFetchResult: local.Fetch,
		},
	}
	resolved.Pipeline = shubaLocalPDFPlan()
	return resolved, nil
}

func (h *Handler) resolveNovelArchive(ctx context.Context, input contentdownload.ResolveInput, parts parsedURL) (*contentdownload.ResolvedRequest, error) {
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
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := novelutil.FirstNonEmpty(probe.ContentID, summary.ID, parts.BookID)
	title := novelutil.FirstNonEmpty(summary.Title, contentID)
	sourceURL := novelutil.FirstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL, parts.Canonical)
	canonicalURL := novelutil.FirstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL, parts.Canonical)
	filename := novelutil.FirstNonEmpty(input.Options.Filename, title, contentID)
	suffix := ""
	contentMetadata := cloneAnyMap(contentdownload.ContentMetadataOf(probe.Content))
	contentOutput := contentdownload.ContentOutputOf(probe.Content)
	metadata := cloneAnyMap(contentMetadata)
	metadata["variant_id"] = variant.ID
	metadata["content_type"] = shubapkg.ContentTypeNovel
	metadata["source_url"] = sourceURL
	metadata["canonical_url"] = canonicalURL
	metadata["archive_concurrency"] = archiveConcurrency
	if bodyHTML, _ := contentOutput["body_html"].(string); strings.TrimSpace(bodyHTML) != "" {
		metadata["body_html"] = bodyHTML
	}
	if result := shubaProbeNovelFetchResult(probe); result != nil {
		metadata["source_html_size"] = len(result.SourceHTML)
		metadata["full_catalog_html_size"] = len(result.FullCatalogHTML)
		if result.Novel != nil {
			metadata["chapter_count"] = len(result.Novel.Chapters)
		}
	}
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         "69shuba-archive://" + contentID,
			Method:      "GET",
			Protocol:    ArchiveProtocol,
			Connections: archiveConcurrency,
		},
		Files: shubaArchiveFilesFromProbe(probe),
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": shubapkg.ContentTypeNovel,
		},
		Metadata: metadata,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           shubapkg.ContentTypeNovel,
			ID:             contentID,
			Title:          title,
			Description:    summary.Description,
			Author:         novelutil.FirstNonEmpty(summary.Author, summary.AuthorNickname),
			URL:            novelutil.FirstNonEmpty(summary.URL, canonicalURL),
			SourceURL:      novelutil.FirstNonEmpty(summary.SourceURL, canonicalURL, sourceURL),
			AuthorNickname: summary.AuthorNickname,
			CoverURL:       summary.CoverURL,
		}, contentdownload.ContentDataOf(probe.Content), contentMetadata, contentOutput),
		Internal: map[string]any{
			metadataNovelFetchResult: shubaProbeNovelFetchResult(probe),
		},
	}
	resolved.Pipeline = shubaArchivePlan()
	return resolved, nil
}

func (h *Handler) probeNovel(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	var result *NovelFetchResult
	var novel *Novel
	var err error
	if fetcher, ok := h.Fetcher.(novelFetcher); ok {
		result, err = fetcher.FetchNovel(parts.Canonical)
		if err != nil {
			return nil, fmt.Errorf("fetch 69shuba novel: %w", err)
		}
		novel = result.Novel
	} else {
		novel, err = h.Fetcher.FetchNovelChapters(parts.Canonical)
		if err != nil {
			return nil, fmt.Errorf("fetch 69shuba novel: %w", err)
		}
	}
	novel.BookID = novelutil.FirstNonEmpty(novel.BookID, parts.BookID)
	novel.URL = novelutil.FirstNonEmpty(novel.URL, parts.Canonical)
	contentID := novelutil.FirstNonEmpty(novel.BookID, parts.BookID)
	title := novelutil.FirstNonEmpty(novel.Title, "69shuba_"+contentID)
	description := novelutil.FirstNonEmpty(novel.Description, novelutil.Description(novel.Category, novel.Status))
	bodyHTML := novelutil.RenderBookHTML("69书吧", novelutil.Book{
		Title:       title,
		URL:         novelutil.FirstNonEmpty(novel.URL, parts.Canonical),
		Author:      novel.Author,
		Category:    novel.Category,
		Status:      novel.Status,
		BookID:      contentID,
		Description: description,
		CoverURL:    novel.CoverURL,
		Tags:        novel.Tags,
		Chapters:    shubaChapters(novel.Chapters),
	})
	internal := map[string]any{"novel": novel}
	if result != nil {
		internal[metadataNovelFetchResult] = result
		internal["probe_pipeline"] = shubaProbePipeline(result)
	}
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           shubapkg.ContentTypeNovel,
			ID:             contentID,
			Title:          title,
			Description:    description,
			Author:         novel.Author,
			URL:            parts.Canonical,
			SourceURL:      parts.Canonical,
			AuthorNickname: novel.Author,
			CoverURL:       novel.CoverURL,
		}, novel, map[string]any{
			"book_id":          contentID,
			"category":         novel.Category,
			"status":           novel.Status,
			"word_count":       novel.WordCount,
			"update_time":      novel.UpdateTime,
			"latest_chapter":   novel.LatestChapter,
			"chapter_count":    len(novel.Chapters),
			"full_catalog_url": novel.FullCatalogURL,
			"source_url":       parts.Canonical,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  shubapkg.ContentTypeNovel,
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{
			shubaArchiveVariant(),
		},
		Defaults: contentdownload.Defaults{VariantID: archiveVariantID},
		Internal: internal,
	}, nil
}

func (h *Handler) probeChapter(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	chapter, err := h.Fetcher.FetchChapterContent(parts.Canonical)
	if err != nil {
		return nil, fmt.Errorf("fetch 69shuba chapter: %w", err)
	}
	contentID := novelutil.FirstNonEmpty(joinID(parts.BookID, parts.ChapterID), parts.ChapterID)
	title := novelutil.FirstNonEmpty(chapter.Title, "69shuba_"+contentID)
	bodyHTML := novelutil.RenderChapterHTML("69书吧", title, parts.Canonical, chapter.Content)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:  PlatformID,
			Type:      shubapkg.ContentTypeChapter,
			ID:        contentID,
			Title:     title,
			URL:       parts.Canonical,
			SourceURL: parts.Canonical,
		}, chapter, map[string]any{
			"book_id":    parts.BookID,
			"chapter_id": parts.ChapterID,
			"source_url": parts.Canonical,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  shubapkg.ContentTypeChapter,
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{
			novelutil.HTMLVariant("章节 HTML", shubapkg.ContentTypeChapter),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{"chapter": chapter},
	}, nil
}

func ParseURL(rawURL string) (parsedURL, bool) {
	parts, ok := shubapkg.ParseURL(rawURL)
	if !ok {
		return parsedURL{}, false
	}
	return parsedURL{
		Kind:      parts.Kind,
		BookID:    parts.BookID,
		ChapterID: parts.ChapterID,
		Canonical: parts.Canonical,
	}, true
}

func shubaChapters(chapters []Chapter) []novelutil.Chapter {
	out := make([]novelutil.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		out = append(out, novelutil.Chapter{Index: chapter.Index, Title: chapter.Title, URL: chapter.URL})
	}
	return out
}

func shubaProbePipeline(result *NovelFetchResult) []map[string]any {
	if result == nil {
		return nil
	}
	nodes := make([]map[string]any, 0, 2)
	if result.SourceNovel != nil {
		nodes = append(nodes, map[string]any{
			"id":     "fetch_source_html",
			"type":   "fetch_html",
			"label":  "获取书籍页面 HTML",
			"status": "completed",
			"output": map[string]any{
				"url":           result.SourceURL,
				"html_size":     len(result.SourceHTML),
				"title":         result.SourceNovel.Title,
				"chapter_count": len(result.SourceNovel.Chapters),
				"body_html":     result.SourceParsedHTML,
			},
		})
	}
	if result.FullCatalogNovel != nil {
		nodes = append(nodes, map[string]any{
			"id":     "fetch_full_catalog",
			"type":   "fetch_full_catalog",
			"label":  "获取完整目录页面 HTML",
			"status": "completed",
			"output": map[string]any{
				"url":           result.FullCatalogURL,
				"html_size":     len(result.FullCatalogHTML),
				"title":         result.FullCatalogNovel.Title,
				"chapter_count": len(result.FullCatalogNovel.Chapters),
				"body_html":     result.FullCatalogParsedHTML,
			},
		})
	}
	return nodes
}

func shubaArchiveVariant() contentdownload.Variant {
	return contentdownload.Variant{
		ID:    archiveVariantID,
		Type:  "archive",
		Label: "整本 HTML 文件夹",
		Metadata: map[string]any{
			"format":       "directory",
			"content_type": shubapkg.ContentTypeNovel,
			"concurrency":  archiveConcurrency,
		},
	}
}

func shubaLocalPDFVariant() contentdownload.Variant {
	return contentdownload.Variant{
		ID:     localPDFVariantID,
		Type:   "pdf",
		Label:  "PDF",
		Suffix: ".pdf",
		Metadata: map[string]any{
			"format":       "pdf",
			"content_type": shubapkg.ContentTypeNovel,
			"source":       "local_directory",
		},
	}
}

func shubaLocalPDFPlan() *contentdownload.PipelinePlan {
	return &contentdownload.PipelinePlan{
		Platform: PlatformID,
		Nodes: []contentdownload.PipelineNode{
			{
				ID:    "local_directory",
				Type:  "local_directory",
				Stage: "input",
				Args:  map[string]any{"source": "archive_dir"},
			},
			{
				ID:        "clean_html",
				Type:      "clean_69shuba_html",
				Stage:     "process",
				DependsOn: []string{"local_directory"},
				Args:      map[string]any{"source": "69shuba_archive"},
			},
			{
				ID:        "render_pdf",
				Type:      "render_pdf",
				Stage:     "export",
				DependsOn: []string{"clean_html"},
				Args: map[string]any{
					"format": "pdf",
				},
			},
			{
				ID:        "persist",
				Type:      "persist_artifacts",
				Stage:     "persist",
				DependsOn: []string{"render_pdf"},
			},
		},
		Metadata: map[string]any{
			"protocol": LocalPDFProtocol,
		},
	}
}

func shubaArchivePlan() *contentdownload.PipelinePlan {
	return &contentdownload.PipelinePlan{
		Platform: PlatformID,
		Nodes: []contentdownload.PipelineNode{
			{
				ID:    "fetch_book_html",
				Type:  "fetch_html",
				Stage: "fetch",
				Args:  map[string]any{"source": "book"},
			},
			{
				ID:        "fetch_full_catalog",
				Type:      "fetch_full_catalog",
				Stage:     "fetch",
				DependsOn: []string{"fetch_book_html"},
				Args:      map[string]any{"source": "full_catalog"},
			},
			{
				ID:        "download",
				Type:      "download_69shuba_archive",
				Stage:     "download",
				DependsOn: []string{"fetch_full_catalog"},
				Args: map[string]any{
					"chapter_source": "fetch_full_catalog",
					"concurrency":    archiveConcurrency,
				},
			},
			{
				ID:        "persist",
				Type:      "persist_artifacts",
				Stage:     "persist",
				DependsOn: []string{"download"},
			},
		},
		Metadata: map[string]any{
			"archive_protocol": ArchiveProtocol,
			"concurrency":      archiveConcurrency,
		},
	}
}

func shubaProbeNovelFetchResult(probe *contentdownload.Probe) *NovelFetchResult {
	if probe == nil || probe.Internal == nil {
		return nil
	}
	result, _ := probe.Internal[metadataNovelFetchResult].(*NovelFetchResult)
	return result
}

func shubaProbeLocalArchive(probe *contentdownload.Probe) *shubapkg.LocalArchiveLoadResult {
	if probe == nil || probe.Internal == nil {
		return nil
	}
	result, _ := probe.Internal[metadataLocalArchive].(*shubapkg.LocalArchiveLoadResult)
	return result
}

func shubaArchiveFilesFromProbe(probe *contentdownload.Probe) []contentdownload.FileNode {
	if result := shubaProbeNovelFetchResult(probe); result != nil {
		return shubaArchiveFilesFromSeed(result, contentdownload.FileNodeStatusPending)
	}
	if probe == nil || probe.Internal == nil {
		return nil
	}
	novel, _ := probe.Internal["novel"].(*Novel)
	return shubaArchiveFilesFromNovel(novel, nil, contentdownload.FileNodeStatusPending)
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func joinID(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return strings.Join(out, "_")
}

func filepathBase(path string) string {
	path = strings.TrimSpace(strings.TrimPrefix(path, "file://"))
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
