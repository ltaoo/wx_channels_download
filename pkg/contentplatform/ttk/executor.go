package ttk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
)

var unsafeArchiveNameRE = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]+`)

type ArchiveFetcher interface {
	FetchNovelArchive(rawURL string, seed *NovelFetchResult, options NovelArchiveOptions) (*NovelArchiveResult, error)
}

type Executor struct {
	Fetcher     ArchiveFetcher
	Concurrency int
}

func NewExecutor(fetcher any) *Executor {
	archiveFetcher, _ := fetcher.(ArchiveFetcher)
	if archiveFetcher == nil {
		archiveFetcher = NewClient(nil)
	}
	return &Executor{Fetcher: archiveFetcher, Concurrency: archiveConcurrency}
}

func (e *Executor) Name() string {
	return ArchiveProtocol
}

func (e *Executor) CanHandle(source contentdownload.DownloadSpec) bool {
	return strings.EqualFold(source.Protocol, ArchiveProtocol) ||
		strings.HasPrefix(strings.ToLower(source.URL), "ttk-archive://")
}

func (e *Executor) Execute(ctx context.Context, req contentdownload.ExecuteRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if req.Resolved == nil {
		return fmt.Errorf("ttk archive resolved request is nil")
	}
	fetcher := e.Fetcher
	if fetcher == nil {
		fetcher = NewClient(nil)
	}
	concurrency := e.Concurrency
	if concurrency <= 0 {
		concurrency = archiveConcurrency
	}
	if req.Source.Connections > 0 {
		concurrency = req.Source.Connections
	}

	seed := archiveSeedFromResolved(req.Resolved)
	chapterPaths := archiveChapterPathLookup(seed)
	files := contentdownload.CloneFileNodes(req.Resolved.Files)
	if len(files) == 0 {
		files = ttkArchiveFilesFromSeed(seed, contentdownload.FileNodeStatusPending)
	}
	if err := prepareNovelArchiveDirectory(req.DestPath); err != nil {
		return err
	}
	var writeErr error
	sourceWritten := false
	if seed != nil {
		if html := archiveBookHTMLFromFetch(seed); strings.TrimSpace(html) != "" {
			if err := writeArchiveFile(ctx, req.DestPath, &files, "source/book.html", html); err != nil && writeErr == nil {
				writeErr = err
			}
			sourceWritten = true
		}
		if html := archiveCatalogHTMLFromFetch(seed); strings.TrimSpace(html) != "" {
			if err := writeArchiveFile(ctx, req.DestPath, &files, "source/full_catalog.html", html); err != nil && writeErr == nil {
				writeErr = err
			}
			sourceWritten = true
		}
	}
	refreshArchiveDirStatuses(files)
	reportArchiveFiles(files, req.OnFiles)

	totalFiles := 2 + archiveSeedChapterCount(seed)
	archive, err := fetcher.FetchNovelArchive(archiveSourceURL(req.Resolved, seed), seed, NovelArchiveOptions{
		Concurrency:  concurrency,
		AllowPartial: true,
		OnChapter: func(done int, total int, chapter ChapterFetchResult) {
			if totalFiles < total+2 {
				totalFiles = total + 2
			}
			if len(files) == 0 {
				return
			}
			chapterPath := archiveChapterFilename(done-1, chapter, chapterPaths)
			if writeErr == nil {
				if err := writeArchiveChapter(ctx, req.DestPath, &files, chapterPath, chapter); err != nil {
					writeErr = err
				}
			}
			refreshArchiveDirStatuses(files)
			reportArchiveFiles(files, req.OnFiles)
			if req.OnProgress != nil {
				completed := done + 2
				progress := contentdownload.Progress{
					DownloadedBytes: int64(completed),
					TotalBytes:      int64(totalFiles),
				}
				if totalFiles > 0 {
					progress.Percent = float64(completed) * 100 / float64(totalFiles)
				}
				req.OnProgress(progress)
			}
		},
	})
	if err != nil {
		return err
	}
	if writeErr != nil {
		return writeErr
	}
	if archive == nil || archive.Fetch == nil {
		return fmt.Errorf("ttk archive result is empty")
	}
	if len(files) == 0 {
		files = ttkArchiveFilesFromNovel(archive.Novel, archive.Fetch, contentdownload.FileNodeStatusPending)
	}
	if len(chapterPaths) == 0 {
		chapterPaths = archiveChapterPathLookup(archive.Fetch)
	}
	if !sourceWritten {
		if err := writeArchiveFile(ctx, req.DestPath, &files, "source/book.html", archiveBookHTML(archive)); err != nil {
			return err
		}
		if err := writeArchiveFile(ctx, req.DestPath, &files, "source/full_catalog.html", archiveCatalogHTML(archive)); err != nil {
			return err
		}
	}
	for i, chapter := range archive.Chapters {
		path := archiveChapterFilename(i, chapter, chapterPaths)
		if node := findArchiveFileNode(files, path); node != nil && node.Status == contentdownload.FileNodeStatusDone {
			continue
		}
		if err := writeArchiveChapter(ctx, req.DestPath, &files, path, chapter); err != nil {
			return err
		}
	}
	refreshArchiveDirStatuses(files)
	reportArchiveFiles(files, req.OnFiles)
	if req.OnProgress != nil {
		totalBytes := contentdownload.FileNodesSize(files)
		req.OnProgress(contentdownload.Progress{
			DownloadedBytes: totalBytes,
			TotalBytes:      totalBytes,
			Percent:         100,
		})
	}
	return nil
}

func archiveSeedFromResolved(resolved *contentdownload.ResolvedRequest) *NovelFetchResult {
	if resolved == nil {
		return nil
	}
	for _, values := range []map[string]any{resolved.Internal, resolved.Metadata} {
		switch seed := values[metadataNovelFetchResult].(type) {
		case *NovelFetchResult:
			return seed
		case NovelFetchResult:
			return &seed
		}
	}
	return nil
}

func archiveSeedChapterCount(seed *NovelFetchResult) int {
	if seed == nil || seed.Novel == nil {
		return 0
	}
	return len(seed.Novel.Chapters)
}

func archiveSourceURL(resolved *contentdownload.ResolvedRequest, seed *NovelFetchResult) string {
	if seed != nil && seed.Novel != nil && strings.TrimSpace(seed.Novel.URL) != "" {
		return seed.Novel.URL
	}
	return novelutil.FirstNonEmpty(resolved.CanonicalURL, resolved.SourceURL)
}

func prepareNovelArchiveDirectory(destPath string) error {
	if info, err := os.Stat(destPath); err == nil && !info.IsDir() {
		return fmt.Errorf("ttk archive destination exists and is not a directory: %s", destPath)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, dir := range []string{destPath, filepath.Join(destPath, "source"), filepath.Join(destPath, "chapters")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func archiveBookHTML(archive *NovelArchiveResult) string {
	if archive == nil || archive.Fetch == nil {
		return ""
	}
	return novelutil.FirstNonEmpty(
		archive.Fetch.SourceHTML,
		archive.Fetch.SourceParsedHTML,
		BuildNovelHTML(firstNonNilNovel(archive.Fetch.SourceNovel, archive.Novel)),
	)
}

func archiveCatalogHTML(archive *NovelArchiveResult) string {
	if archive == nil || archive.Fetch == nil {
		return ""
	}
	return archiveCatalogHTMLFromFetch(archive.Fetch)
}

func firstNonNilNovel(values ...*Novel) *Novel {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func writeArchiveChapter(ctx context.Context, root string, files *[]contentdownload.FileNode, relPath string, chapter ChapterFetchResult) error {
	if strings.TrimSpace(relPath) == "" {
		relPath = archiveChapterFilename(0, chapter, nil)
	}
	if strings.TrimSpace(chapter.Error) != "" {
		setArchiveFileNode(*files, relPath, 0, contentdownload.FileNodeStatusError, chapter.Error)
		return nil
	}
	html := novelutil.FirstNonEmpty(chapter.ParsedHTML, chapter.HTML)
	if html == "" && chapter.Content != nil {
		html = BuildChapterHTML(chapter.Content, novelutil.FirstNonEmpty(chapter.URL, chapter.Chapter.URL))
	}
	return writeArchiveFile(ctx, root, files, relPath, html)
}

func writeArchiveFile(ctx context.Context, root string, files *[]contentdownload.FileNode, relPath string, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("archive item %s is empty", relPath)
	}
	destPath := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	data := []byte(body)
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return err
	}
	setArchiveFileNode(*files, relPath, int64(len(data)), contentdownload.FileNodeStatusDone, "")
	return nil
}

func setArchiveFileNode(files []contentdownload.FileNode, relPath string, size int64, status string, errText string) {
	dirPath := strings.Trim(filepath.ToSlash(filepath.Dir(relPath)), ".")
	for dirIndex := range files {
		if files[dirIndex].Path != dirPath {
			continue
		}
		files[dirIndex].Status = contentdownload.FileNodeStatusDownloading
		if status == contentdownload.FileNodeStatusError {
			files[dirIndex].Status = contentdownload.FileNodeStatusError
		}
		for childIndex := range files[dirIndex].Children {
			if files[dirIndex].Children[childIndex].Path != relPath {
				continue
			}
			files[dirIndex].Children[childIndex].Size = size
			files[dirIndex].Children[childIndex].Status = status
			files[dirIndex].Children[childIndex].Error = errText
			return
		}
		files[dirIndex].Children = append(files[dirIndex].Children, contentdownload.FileNode{
			Name:   filepath.Base(relPath),
			Path:   relPath,
			Type:   contentdownload.FileNodeTypeFile,
			Size:   size,
			Status: status,
			Error:  errText,
		})
		return
	}
}

func refreshArchiveDirStatuses(files []contentdownload.FileNode) {
	for i := range files {
		if len(files[i].Children) == 0 {
			continue
		}
		refreshArchiveDirStatuses(files[i].Children)
		allDone := true
		anyError := false
		for _, child := range files[i].Children {
			switch strings.ToLower(strings.TrimSpace(child.Status)) {
			case contentdownload.FileNodeStatusDone:
			case contentdownload.FileNodeStatusError, "failed", "fail":
				allDone = false
				anyError = true
			default:
				allDone = false
			}
		}
		switch {
		case anyError:
			files[i].Status = contentdownload.FileNodeStatusError
		case allDone:
			files[i].Status = contentdownload.FileNodeStatusDone
		default:
			files[i].Status = contentdownload.FileNodeStatusPending
		}
	}
}

func reportArchiveFiles(files []contentdownload.FileNode, onFiles func([]contentdownload.FileNode)) {
	if onFiles != nil {
		onFiles(contentdownload.CloneFileNodes(files))
	}
}

func archiveChapterFilename(index int, chapter ChapterFetchResult, chapterPaths map[string]string) string {
	if path := archiveChapterPathFromLookup(chapter.Chapter, chapterPaths); path != "" {
		return path
	}
	title := novelutil.FirstNonEmpty(chapter.Chapter.Title)
	if title == "" && chapter.Content != nil {
		title = chapter.Content.Title
	}
	return archiveChapterFilePath(index, chapter.Chapter, title)
}

func archiveChapterFilePath(index int, chapter Chapter, title string) string {
	chapterIndex := chapter.Index
	if chapterIndex <= 0 {
		chapterIndex = index + 1
	}
	title = safeArchiveName(novelutil.FirstNonEmpty(title, fmt.Sprintf("chapter_%04d", chapterIndex)))
	return fmt.Sprintf("chapters/%s.html", title)
}

func ttkArchiveFilesFromSeed(seed *NovelFetchResult, chapterStatus string) []contentdownload.FileNode {
	if seed == nil || seed.Novel == nil {
		return nil
	}
	return ttkArchiveFilesFromNovel(seed.Novel, seed, chapterStatus)
}

func ttkArchiveFilesFromNovel(novel *Novel, fetch *NovelFetchResult, chapterStatus string) []contentdownload.FileNode {
	if novel == nil {
		return nil
	}
	if chapterStatus == "" {
		chapterStatus = contentdownload.FileNodeStatusPending
	}
	sourceStatus := contentdownload.FileNodeStatusDone
	bookHTML := archiveBookHTMLFromFetch(fetch)
	if bookHTML == "" {
		bookHTML = BuildNovelHTML(novel)
	}
	catalogHTML := archiveCatalogHTMLFromFetch(fetch)
	if catalogHTML == "" {
		catalogHTML = BuildNovelHTML(novel)
	}
	sourceChildren := []contentdownload.FileNode{
		{
			Name:   "book.html",
			Path:   "source/book.html",
			Type:   contentdownload.FileNodeTypeFile,
			Size:   int64(len(bookHTML)),
			Status: sourceStatus,
		},
		{
			Name:   "full_catalog.html",
			Path:   "source/full_catalog.html",
			Type:   contentdownload.FileNodeTypeFile,
			Size:   int64(len(catalogHTML)),
			Status: sourceStatus,
		},
	}
	chapterChildren := make([]contentdownload.FileNode, 0, len(novel.Chapters))
	chapterPaths := archiveChapterFilePaths(novel.Chapters)
	for i := range novel.Chapters {
		path := chapterPaths[i]
		chapterChildren = append(chapterChildren, contentdownload.FileNode{
			Name:   filepath.Base(path),
			Path:   path,
			Type:   contentdownload.FileNodeTypeFile,
			Status: chapterStatus,
		})
	}
	return []contentdownload.FileNode{
		{Name: "source", Path: "source", Type: contentdownload.FileNodeTypeDir, Status: sourceStatus, Children: sourceChildren},
		{Name: "chapters", Path: "chapters", Type: contentdownload.FileNodeTypeDir, Status: chapterStatus, Children: chapterChildren},
	}
}

func archiveChapterFilePaths(chapters []Chapter) []string {
	paths := make([]string, 0, len(chapters))
	used := map[string]int{}
	for i, chapter := range chapters {
		base := archiveChapterFilePath(i, chapter, chapter.Title)
		dir := filepath.ToSlash(filepath.Dir(base))
		name := filepath.Base(base)
		ext := filepath.Ext(name)
		nameWithoutExt := strings.TrimSuffix(name, ext)
		key := filepath.ToSlash(filepath.Join(dir, name))
		if count, exists := used[key]; exists {
			for {
				count++
				nextName := fmt.Sprintf("%s (%d)%s", nameWithoutExt, count, ext)
				nextKey := filepath.ToSlash(filepath.Join(dir, nextName))
				if _, ok := used[nextKey]; !ok {
					name = nextName
					key = nextKey
					used[base] = count
					break
				}
			}
		}
		used[key] = 0
		paths = append(paths, filepath.ToSlash(filepath.Join(dir, name)))
	}
	return paths
}

func archiveChapterPathLookup(seed *NovelFetchResult) map[string]string {
	if seed == nil || seed.Novel == nil {
		return nil
	}
	paths := archiveChapterFilePaths(seed.Novel.Chapters)
	out := make(map[string]string, len(paths)*3)
	for i, chapter := range seed.Novel.Chapters {
		if i >= len(paths) {
			continue
		}
		for _, key := range archiveChapterLookupKeys(chapter) {
			out[key] = paths[i]
		}
	}
	return out
}

func archiveChapterPathFromLookup(chapter Chapter, lookup map[string]string) string {
	for _, key := range archiveChapterLookupKeys(chapter) {
		if path := strings.TrimSpace(lookup[key]); path != "" {
			return path
		}
	}
	return ""
}

func archiveChapterLookupKeys(chapter Chapter) []string {
	keys := make([]string, 0, 3)
	if url := strings.TrimSpace(chapter.URL); url != "" {
		keys = append(keys, "url:"+url)
	}
	if chapter.Index > 0 {
		keys = append(keys, fmt.Sprintf("index:%d", chapter.Index))
	}
	if title := strings.TrimSpace(chapter.Title); title != "" {
		keys = append(keys, "title:"+title)
	}
	return keys
}

func archiveBookHTMLFromFetch(fetch *NovelFetchResult) string {
	if fetch == nil {
		return ""
	}
	return novelutil.FirstNonEmpty(
		fetch.SourceHTML,
		fetch.SourceParsedHTML,
		BuildNovelHTML(firstNonNilNovel(fetch.SourceNovel, fetch.Novel)),
	)
}

func archiveCatalogHTMLFromFetch(fetch *NovelFetchResult) string {
	if fetch == nil {
		return ""
	}
	if strings.TrimSpace(fetch.FullCatalogHTML) == "" && archiveFetchSourceIsFullCatalog(fetch) {
		return novelutil.FirstNonEmpty(
			fetch.SourceHTML,
			fetch.SourceParsedHTML,
			BuildNovelHTML(firstNonNilNovel(fetch.SourceNovel, fetch.Novel)),
		)
	}
	return novelutil.FirstNonEmpty(
		fetch.FullCatalogHTML,
		fetch.FullCatalogParsedHTML,
		BuildNovelHTML(firstNonNilNovel(fetch.FullCatalogNovel, fetch.Novel)),
	)
}

func archiveFetchSourceIsFullCatalog(fetch *NovelFetchResult) bool {
	if fetch == nil {
		return false
	}
	sourceURL := strings.TrimRight(strings.TrimSpace(fetch.SourceURL), "/")
	fullURL := ""
	if fetch.Novel != nil {
		fullURL = strings.TrimRight(strings.TrimSpace(fetch.Novel.FullCatalogURL), "/")
	}
	return fullURL != "" && sourceURL == fullURL
}

func findArchiveFileNode(files []contentdownload.FileNode, path string) *contentdownload.FileNode {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
		if found := findArchiveFileNode(files[i].Children, path); found != nil {
			return found
		}
	}
	return nil
}

func safeArchiveName(name string) string {
	name = unsafeArchiveNameRE.ReplaceAllString(strings.TrimSpace(name), "_")
	name = strings.Trim(name, " ._")
	if name == "" {
		return "untitled"
	}
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	return name
}
