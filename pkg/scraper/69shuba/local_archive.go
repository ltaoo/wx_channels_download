package shuba69

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	chapterFilePrefixRE = regexp.MustCompile(`^(\d+)_`)
	cleanFileNameRE     = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]+`)
)

const (
	cleanArchiveDirName = "clean"
	cleanCatalogRelPath = "clean/catalog.json"
)

type LocalArchiveLoadResult struct {
	RootDir             string
	Novel               *Novel
	Fetch               *NovelFetchResult
	Chapters            []ChapterFetchResult
	ChapterFiles        []string
	MissingChapters     []Chapter
	SkippedChapterFiles int
}

type CleanArchiveOptions struct {
	Force bool
}

type CleanArchiveResult struct {
	RootDir             string
	CatalogPath         string
	ChapterTextDir      string
	Catalog             *CleanCatalog
	CleanedChapters     int
	SkippedChapters     int
	MissingChapters     []CleanChapter
	SkippedChapterFiles int
}

type CleanCatalog struct {
	SchemaVersion       int            `json:"schema_version"`
	GeneratedAt         string         `json:"generated_at"`
	Title               string         `json:"title"`
	Author              string         `json:"author,omitempty"`
	Category            string         `json:"category,omitempty"`
	Status              string         `json:"status,omitempty"`
	Description         string         `json:"description,omitempty"`
	CoverURL            string         `json:"cover_url,omitempty"`
	BookID              string         `json:"book_id,omitempty"`
	SourceURL           string         `json:"source_url,omitempty"`
	FullCatalogURL      string         `json:"full_catalog_url,omitempty"`
	ChapterCount        int            `json:"chapter_count"`
	SkippedChapterFiles int            `json:"skipped_chapter_files,omitempty"`
	Chapters            []CleanChapter `json:"chapters"`
}

type CleanChapter struct {
	Index    int    `json:"index"`
	Title    string `json:"title"`
	URL      string `json:"url,omitempty"`
	HTMLPath string `json:"html_path,omitempty"`
	TextPath string `json:"text_path"`
}

type localChapterFile struct {
	Path      string
	RelPath   string
	Prefix    int
	Title     string
	SourceURL string
	Content   *ChapterContent
	HTML      string
	Error     error
}

func IsLocalArchiveDir(rawPath string) bool {
	root := normalizeLocalArchivePath(rawPath)
	if root == "" {
		return false
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return false
	}
	for _, rel := range []string{"source/book.html", "source/full_catalog.html", "chapters"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			return false
		}
	}
	return true
}

func LoadLocalArchive(rawPath string) (*LocalArchiveLoadResult, error) {
	root := normalizeLocalArchivePath(rawPath)
	if !IsLocalArchiveDir(root) {
		return nil, fmt.Errorf("not a 69shuba local archive directory: %s", rawPath)
	}
	bookHTML, err := readLocalHTML(filepath.Join(root, "source", "book.html"))
	if err != nil {
		return nil, fmt.Errorf("read source/book.html: %w", err)
	}
	bookNovel, err := ParseNovelHTML(BaseURL, bookHTML)
	if err != nil {
		return nil, fmt.Errorf("parse source/book.html: %w", err)
	}
	bookURL := localBookURL(bookNovel)
	if bookURL != "" {
		if parsed, parseErr := ParseNovelHTML(bookURL, bookHTML); parseErr == nil {
			bookNovel = parsed
		}
	}
	catalogHTML, err := readLocalHTML(filepath.Join(root, "source", "full_catalog.html"))
	if err != nil {
		return nil, fmt.Errorf("read source/full_catalog.html: %w", err)
	}
	catalogURL := localCatalogURL(bookNovel)
	catalogNovel, err := ParseNovelHTML(firstNonEmpty(catalogURL, bookURL, BaseURL), catalogHTML)
	if err != nil {
		return nil, fmt.Errorf("parse source/full_catalog.html: %w", err)
	}
	mergeFullCatalog(bookNovel, catalogNovel)
	if len(bookNovel.Chapters) == 0 {
		return nil, fmt.Errorf("local archive catalog has no chapters")
	}

	files, err := loadLocalChapterFiles(root)
	if err != nil {
		return nil, err
	}
	chapters, paths, missing, skipped := matchLocalChapterFiles(bookNovel.Chapters, files)
	if len(chapters) == 0 {
		return nil, fmt.Errorf("local archive has no readable chapter files")
	}
	if len(chapters) < len(bookNovel.Chapters) {
		novel := cloneNovel(bookNovel)
		kept := make([]Chapter, 0, len(chapters))
		for _, item := range chapters {
			kept = append(kept, item.Chapter)
		}
		novel.Chapters = kept
		bookNovel = novel
	}

	fetch := &NovelFetchResult{
		Novel:                 cloneNovel(bookNovel),
		SourceURL:             firstNonEmpty(bookURL, bookNovel.URL),
		SourceHTML:            bookHTML,
		SourceNovel:           cloneNovel(bookNovel),
		SourceParsedHTML:      BuildNovelHTML(bookNovel),
		FullCatalogURL:        firstNonEmpty(catalogURL, bookNovel.FullCatalogURL),
		FullCatalogHTML:       catalogHTML,
		FullCatalogNovel:      cloneNovel(catalogNovel),
		FullCatalogParsedHTML: BuildNovelHTML(catalogNovel),
	}
	return &LocalArchiveLoadResult{
		RootDir:             root,
		Novel:               cloneNovel(bookNovel),
		Fetch:               fetch,
		Chapters:            chapters,
		ChapterFiles:        paths,
		MissingChapters:     missing,
		SkippedChapterFiles: skipped,
	}, nil
}

func readLocalHTML(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return DecodeHTML(body, "")
}

func CleanLocalArchive(rawPath string, options CleanArchiveOptions) (*CleanArchiveResult, error) {
	root := normalizeLocalArchivePath(rawPath)
	if !IsLocalArchiveDir(root) {
		return nil, fmt.Errorf("not a 69shuba local archive directory: %s", rawPath)
	}
	cleanDir := filepath.Join(root, cleanArchiveDirName)
	textDir := filepath.Join(cleanDir, "chapters")
	if options.Force {
		if err := os.RemoveAll(textDir); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(textDir, 0o755); err != nil {
		return nil, err
	}

	catalogPath := filepath.Join(root, filepath.FromSlash(cleanCatalogRelPath))
	catalog, skippedFiles, err := loadOrBuildCleanCatalog(root, catalogPath, options.Force)
	if err != nil {
		return nil, err
	}
	result := &CleanArchiveResult{
		RootDir:             root,
		CatalogPath:         catalogPath,
		ChapterTextDir:      textDir,
		Catalog:             catalog,
		SkippedChapterFiles: skippedFiles,
	}
	for _, chapter := range catalog.Chapters {
		alreadyClean := !options.Force && fileNonEmpty(filepath.Join(root, filepath.FromSlash(chapter.TextPath)))
		if err := cleanLocalChapterText(root, chapter, options.Force); err != nil {
			result.MissingChapters = append(result.MissingChapters, chapter)
			return result, err
		}
		if alreadyClean {
			result.SkippedChapters++
			continue
		}
		result.CleanedChapters++
	}
	return result, nil
}

func LoadCleanArchive(rawPath string) (*CleanArchiveResult, error) {
	root := normalizeLocalArchivePath(rawPath)
	catalogPath := filepath.Join(root, filepath.FromSlash(cleanCatalogRelPath))
	catalog, err := readCleanCatalog(catalogPath)
	if err != nil {
		return nil, err
	}
	return &CleanArchiveResult{
		RootDir:             root,
		CatalogPath:         catalogPath,
		ChapterTextDir:      filepath.Join(root, cleanArchiveDirName, "chapters"),
		Catalog:             catalog,
		SkippedChapterFiles: catalog.SkippedChapterFiles,
	}, nil
}

func LoadCleanChapterText(root string, chapter CleanChapter) (string, error) {
	path := filepath.Join(normalizeLocalArchivePath(root), filepath.FromSlash(chapter.TextPath))
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func loadOrBuildCleanCatalog(root string, catalogPath string, force bool) (*CleanCatalog, int, error) {
	if !force && fileNonEmpty(catalogPath) {
		catalog, err := readCleanCatalog(catalogPath)
		if err != nil {
			return nil, 0, err
		}
		return catalog, catalog.SkippedChapterFiles, nil
	}
	bookHTML, err := readLocalHTML(filepath.Join(root, "source", "book.html"))
	if err != nil {
		return nil, 0, fmt.Errorf("read source/book.html: %w", err)
	}
	bookNovel, err := ParseNovelHTML(BaseURL, bookHTML)
	if err != nil {
		return nil, 0, fmt.Errorf("parse source/book.html: %w", err)
	}
	bookURL := localBookURL(bookNovel)
	if bookURL != "" {
		if parsed, parseErr := ParseNovelHTML(bookURL, bookHTML); parseErr == nil {
			bookNovel = parsed
		}
	}
	catalogHTML, err := readLocalHTML(filepath.Join(root, "source", "full_catalog.html"))
	if err != nil {
		return nil, 0, fmt.Errorf("read source/full_catalog.html: %w", err)
	}
	catalogURL := localCatalogURL(bookNovel)
	catalogNovel, err := ParseNovelHTML(firstNonEmpty(catalogURL, bookURL, BaseURL), catalogHTML)
	if err != nil {
		return nil, 0, fmt.Errorf("parse source/full_catalog.html: %w", err)
	}
	mergeFullCatalog(bookNovel, catalogNovel)
	if len(bookNovel.Chapters) == 0 {
		return nil, 0, fmt.Errorf("local archive catalog has no chapters")
	}
	files, err := loadLocalChapterFileIndex(root)
	if err != nil {
		return nil, 0, err
	}
	paths, skippedFiles := matchLocalChapterPaths(bookNovel.Chapters, files)
	catalog := cleanCatalogFromNovel(bookNovel, catalogURL, paths, skippedFiles)
	if err := writeJSONAtomic(catalogPath, catalog); err != nil {
		return nil, 0, err
	}
	return catalog, skippedFiles, nil
}

func cleanCatalogFromNovel(novel *Novel, catalogURL string, chapterPaths []string, skippedFiles int) *CleanCatalog {
	catalog := &CleanCatalog{
		SchemaVersion:       1,
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339),
		Title:               novel.Title,
		Author:              novel.Author,
		Category:            novel.Category,
		Status:              novel.Status,
		Description:         novel.Description,
		CoverURL:            novel.CoverURL,
		BookID:              novel.BookID,
		SourceURL:           novel.URL,
		FullCatalogURL:      firstNonEmpty(catalogURL, novel.FullCatalogURL),
		ChapterCount:        len(novel.Chapters),
		SkippedChapterFiles: skippedFiles,
		Chapters:            make([]CleanChapter, 0, len(novel.Chapters)),
	}
	for i, chapter := range novel.Chapters {
		htmlPath := ""
		if i < len(chapterPaths) {
			htmlPath = chapterPaths[i]
		}
		catalog.Chapters = append(catalog.Chapters, CleanChapter{
			Index:    chapter.Index,
			Title:    chapter.Title,
			URL:      chapter.URL,
			HTMLPath: htmlPath,
			TextPath: cleanChapterTextRelPath(i, chapter),
		})
	}
	return catalog
}

func cleanLocalChapterText(root string, chapter CleanChapter, force bool) error {
	textPath := filepath.Join(root, filepath.FromSlash(chapter.TextPath))
	if !force && fileNonEmpty(textPath) {
		return nil
	}
	if strings.TrimSpace(chapter.HTMLPath) == "" {
		return fmt.Errorf("chapter %d %q has no matched html file", chapter.Index, chapter.Title)
	}
	htmlText, err := readLocalHTML(filepath.Join(root, filepath.FromSlash(chapter.HTMLPath)))
	if err != nil {
		return fmt.Errorf("read %s: %w", chapter.HTMLPath, err)
	}
	content, err := ParseChapterHTML(htmlText)
	if err != nil {
		return fmt.Errorf("parse %s: %w", chapter.HTMLPath, err)
	}
	content.Title = firstNonEmpty(chapter.Title, content.Title)
	content.Content = trimRepeatedChapterTitle(content.Title, content.Content)
	if strings.TrimSpace(content.Content) == "" {
		return fmt.Errorf("chapter %d %q content is empty", chapter.Index, content.Title)
	}
	return writeTextAtomic(textPath, strings.TrimSpace(content.Content)+"\n")
}

func readCleanCatalog(path string) (*CleanCatalog, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var catalog CleanCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, err
	}
	if catalog.SchemaVersion == 0 || len(catalog.Chapters) == 0 {
		return nil, fmt.Errorf("invalid clean catalog: %s", path)
	}
	return &catalog, nil
}

func loadLocalChapterFiles(root string) ([]localChapterFile, error) {
	return loadLocalChapterFilesWithContent(root, true)
}

func loadLocalChapterFileIndex(root string) ([]localChapterFile, error) {
	return loadLocalChapterFilesWithContent(root, false)
}

func loadLocalChapterFilesWithContent(root string, parseContent bool) ([]localChapterFile, error) {
	chapterDir := filepath.Join(root, "chapters")
	entries, err := os.ReadDir(chapterDir)
	if err != nil {
		return nil, fmt.Errorf("read chapters dir: %w", err)
	}
	files := make([]localChapterFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".html") {
			continue
		}
		path := filepath.Join(chapterDir, entry.Name())
		htmlText, err := readLocalHTML(path)
		item := localChapterFile{
			Path:    path,
			RelPath: filepath.ToSlash(filepath.Join("chapters", entry.Name())),
			Prefix:  chapterFilePrefix(entry.Name()),
			HTML:    htmlText,
			Error:   err,
		}
		if err == nil {
			item.SourceURL = localChapterSourceURL(htmlText)
			if parseContent {
				content, parseErr := ParseChapterHTML(htmlText)
				if parseErr != nil {
					item.Error = parseErr
				} else {
					content.Content = trimRepeatedChapterTitle(content.Title, content.Content)
					item.Content = content
					item.Title = firstNonEmpty(content.Title, chapterTitleFromFilename(entry.Name()))
				}
			} else {
				item.Title = firstNonEmpty(localChapterTitle(htmlText), chapterTitleFromFilename(entry.Name()))
			}
		}
		if item.Title == "" {
			item.Title = chapterTitleFromFilename(entry.Name())
		}
		files = append(files, item)
	}
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].Prefix != files[j].Prefix {
			return files[i].Prefix < files[j].Prefix
		}
		return files[i].RelPath < files[j].RelPath
	})
	return files, nil
}

func matchLocalChapterFiles(catalog []Chapter, files []localChapterFile) ([]ChapterFetchResult, []string, []Chapter, int) {
	byURL := map[string][]int{}
	byTitle := map[string][]int{}
	byPrefix := map[int][]int{}
	for i, file := range files {
		if file.SourceURL != "" {
			byURL[normalizeChapterURLKey(file.SourceURL)] = append(byURL[normalizeChapterURLKey(file.SourceURL)], i)
		}
		if file.Title != "" {
			byTitle[file.Title] = append(byTitle[file.Title], i)
		}
		if file.Prefix > 0 {
			byPrefix[file.Prefix] = append(byPrefix[file.Prefix], i)
		}
	}
	used := map[int]bool{}
	out := make([]ChapterFetchResult, 0, len(catalog))
	paths := make([]string, 0, len(catalog))
	missing := make([]Chapter, 0)
	for _, chapter := range catalog {
		idx := chooseLocalChapterFile(chapter, files, used, byURL, byTitle, byPrefix)
		if idx < 0 {
			missing = append(missing, chapter)
			continue
		}
		used[idx] = true
		file := files[idx]
		item := ChapterFetchResult{
			Chapter:    chapter,
			URL:        firstNonEmpty(chapter.URL, file.SourceURL),
			HTML:       file.HTML,
			Content:    file.Content,
			ParsedHTML: "",
		}
		if file.Error != nil {
			item.Error = file.Error.Error()
		}
		if item.Content != nil {
			item.Content.Title = firstNonEmpty(chapter.Title, item.Content.Title)
			item.Content.Content = trimRepeatedChapterTitle(item.Content.Title, item.Content.Content)
			item.ParsedHTML = BuildChapterHTML(item.Content, item.URL)
		}
		out = append(out, item)
		paths = append(paths, file.RelPath)
	}
	if len(out) == 0 && len(files) > 0 {
		for _, file := range files {
			if file.Error != nil || file.Content == nil {
				continue
			}
			chapter := Chapter{Index: len(out) + 1, Title: file.Title, URL: file.SourceURL}
			out = append(out, ChapterFetchResult{
				Chapter:    chapter,
				URL:        file.SourceURL,
				HTML:       file.HTML,
				Content:    file.Content,
				ParsedHTML: BuildChapterHTML(file.Content, file.SourceURL),
			})
			paths = append(paths, file.RelPath)
		}
	}
	return out, paths, missing, len(files) - len(used)
}

func matchLocalChapterPaths(catalog []Chapter, files []localChapterFile) ([]string, int) {
	byURL := map[string][]int{}
	byTitle := map[string][]int{}
	byPrefix := map[int][]int{}
	for i, file := range files {
		if file.SourceURL != "" {
			byURL[normalizeChapterURLKey(file.SourceURL)] = append(byURL[normalizeChapterURLKey(file.SourceURL)], i)
		}
		if file.Title != "" {
			byTitle[file.Title] = append(byTitle[file.Title], i)
		}
		if file.Prefix > 0 {
			byPrefix[file.Prefix] = append(byPrefix[file.Prefix], i)
		}
	}
	used := map[int]bool{}
	paths := make([]string, len(catalog))
	for i, chapter := range catalog {
		idx := chooseLocalChapterFile(chapter, files, used, byURL, byTitle, byPrefix)
		if idx < 0 {
			continue
		}
		used[idx] = true
		paths[i] = files[idx].RelPath
	}
	return paths, len(files) - len(used)
}

func chooseLocalChapterFile(chapter Chapter, files []localChapterFile, used map[int]bool, byURL map[string][]int, byTitle map[string][]int, byPrefix map[int][]int) int {
	if chapter.URL != "" {
		if idx := bestLocalChapterCandidate(chapter.Index, files, used, byURL[normalizeChapterURLKey(chapter.URL)]); idx >= 0 {
			return idx
		}
	}
	if chapter.Title != "" {
		if idx := bestLocalChapterCandidate(chapter.Index, files, used, byTitle[chapter.Title]); idx >= 0 {
			return idx
		}
	}
	return bestLocalChapterCandidate(chapter.Index, files, used, byPrefix[chapter.Index])
}

func bestLocalChapterCandidate(chapterIndex int, files []localChapterFile, used map[int]bool, candidates []int) int {
	best := -1
	bestScore := int(^uint(0) >> 1)
	for _, idx := range candidates {
		if idx < 0 || idx >= len(files) || used[idx] || files[idx].Error != nil {
			continue
		}
		score := absInt(files[idx].Prefix - chapterIndex)
		if files[idx].Prefix <= 0 || chapterIndex <= 0 {
			score = 1000000 + idx
		}
		if score < bestScore {
			best = idx
			bestScore = score
		}
	}
	return best
}

func localChapterSourceURL(htmlText string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return ""
	}
	var out string
	doc.Find("dt").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if strings.TrimSpace(s.Text()) != "来源" {
			return true
		}
		out = strings.TrimSpace(s.NextFiltered("dd").Text())
		return false
	})
	return out
}

func localChapterTitle(htmlText string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Find("h1, .txtnav h1, .chapter-title").First().Text())
}

func localBookURL(novel *Novel) string {
	if novel == nil {
		return ""
	}
	if _, ok := ParseURL(novel.URL); ok {
		return novel.URL
	}
	if novel.BookID != "" {
		return BaseURL + "/book/" + novel.BookID + ".htm"
	}
	return ""
}

func localCatalogURL(novel *Novel) string {
	if novel == nil {
		return ""
	}
	if novel.FullCatalogURL != "" {
		return novel.FullCatalogURL
	}
	if novel.BookID != "" {
		return BaseURL + "/book/" + novel.BookID + "/"
	}
	return ""
}

func normalizeLocalArchivePath(rawPath string) string {
	rawPath = strings.TrimSpace(rawPath)
	rawPath = strings.TrimPrefix(rawPath, "file://")
	return rawPath
}

func normalizeChapterURLKey(rawURL string) string {
	return strings.TrimRight(strings.TrimSpace(rawURL), "/")
}

func chapterFilePrefix(name string) int {
	match := chapterFilePrefixRE.FindStringSubmatch(name)
	if len(match) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(match[1])
	return n
}

func chapterTitleFromFilename(name string) string {
	name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if idx := strings.Index(name, "_"); idx >= 0 {
		name = name[idx+1:]
	}
	return strings.TrimSpace(name)
}

func cleanChapterTextRelPath(index int, chapter Chapter) string {
	chapterIndex := chapter.Index
	if chapterIndex <= 0 {
		chapterIndex = index + 1
	}
	title := cleanArchiveFileName(firstNonEmpty(chapter.Title, fmt.Sprintf("chapter_%04d", chapterIndex)))
	return filepath.ToSlash(filepath.Join(cleanArchiveDirName, "chapters", fmt.Sprintf("%04d_%s.txt", chapterIndex, title)))
}

func cleanArchiveFileName(name string) string {
	name = cleanFileNameRE.ReplaceAllString(strings.TrimSpace(name), "_")
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

func fileNonEmpty(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func writeTextAtomic(path string, content string) error {
	return writeBytesAtomic(path, []byte(content))
}

func writeJSONAtomic(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return writeBytesAtomic(path, body)
}

func writeBytesAtomic(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func trimRepeatedChapterTitle(title, content string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return content
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == title {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
