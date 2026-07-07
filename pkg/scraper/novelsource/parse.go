package novelsource

import (
	"fmt"
	stdhtml "html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	spaceRE       = regexp.MustCompile(`\s+`)
	tagRE         = regexp.MustCompile(`<[^>]+>`)
	titleJunkRE   = regexp.MustCompile(`(?i)(最新章节.*|全文阅读.*|无弹窗.*|_.*小说.*| - .*)$`)
	wordCountRE   = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?\s*万?字)`)
	chapterTextRE = regexp.MustCompile(`^(第[0-9一二三四五六七八九十百千万零〇两]+[章节回卷集部]|序章|楔子|番外|正文|Chapter\s+\d+)`)
)

var defaultCatalogSelectors = []string{
	"#catalog li a",
	"#catalog a",
	"#list dl dd a",
	"#list dd a",
	"#list li a",
	"#chapterList li a",
	"#chapterList a",
	"#chapterlist a",
	"#chapters a",
	"#content_1 a",
	".full_chapters a",
	".catalog li a",
	".catalog a",
	".mulu_list li a",
	".mulu a",
	".booklist a",
	".book_list a",
	".listmain dd a",
	".listmain li a",
	"dl dd a",
	"ol li a",
	"ul li a",
}

var defaultChapterTitleSelectors = []string{
	".bookname > h1",
	".txtnav > h1",
	".content > h1",
	".title > h1",
	".submenu > h1",
	"#chapterTitle",
	"#chapter-name > h2",
	"#wrapper > article > h1",
	"h1",
	"title",
}

var defaultChapterContentSelectors = []string{
	"#htmlContent",
	"#chaptercontent",
	"#content",
	"#booktxt",
	"#txtcontent",
	"#txt",
	"#Lab_Contents",
	"div.content",
	".content",
	".con",
	".txtnav",
	"article",
}

func (s Source) ParseNovelHTML(pageURL string, htmlText string) (*Novel, error) {
	s = s.normalized()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	parts, _ := s.ParseURL(pageURL)
	novel := &Novel{
		URL:    firstNonEmpty(parts.Canonical, pageURL),
		BookID: parts.BookID,
	}
	novel.Title = cleanBookTitle(firstNonEmpty(
		metaAny(doc, "og:novel:book_name"),
		metaAny(doc, "og:book_name"),
		metaAny(doc, "og:title"),
		strings.TrimSpace(doc.Find("#bookName").First().Text()),
		strings.TrimSpace(doc.Find(".book-info h1, .bookinfo h1, .info h1, .item h1, h1").First().Text()),
		strings.TrimSpace(doc.Find("title").First().Text()),
	))
	novel.Author = cleanLabel(firstNonEmpty(
		metaAny(doc, "og:novel:author"),
		metaAny(doc, "author"),
		textByLabel(doc, "作者"),
		strings.TrimSpace(doc.Find(".author a, .author, .book-author, .writer").First().Text()),
	), "作者")
	novel.Category = cleanLabel(firstNonEmpty(
		metaAny(doc, "og:novel:category"),
		metaAny(doc, "category"),
		textByLabel(doc, "分类"),
		textByLabel(doc, "类型"),
	), "分类", "类型")
	novel.Status = cleanLabel(firstNonEmpty(
		metaAny(doc, "og:novel:status"),
		textByLabel(doc, "状态"),
	), "状态")
	novel.UpdateTime = cleanLabel(firstNonEmpty(
		metaAny(doc, "og:novel:update_time"),
		metaAny(doc, "og:novel:latest_chapter_time"),
		textByLabel(doc, "更新"),
	), "更新")
	novel.LatestChapter = firstNonEmpty(
		metaAny(doc, "og:novel:latest_chapter_name"),
		metaAny(doc, "og:novel:lastest_chapter_name"),
		textByLabel(doc, "最新章节"),
	)
	novel.CoverURL = normalizeURL(firstNonEmpty(
		metaAny(doc, "og:image"),
		attr(doc.Find(".bookimg img, .book-img img, .cover img, .pic img, .item img").First(), "src"),
	), pageURL, s.BaseURL)
	novel.Description = cleanDescription(firstNonEmpty(
		metaAny(doc, "og:description"),
		metaAny(doc, "description"),
		selectionText(doc.Find("#intro, .intro, .description, .desc").First()),
	))
	parseCountMetadata(doc, novel)
	novel.FullCatalogURL = firstNonEmpty(findCatalogURL(s, doc, pageURL, novel.BookID), s.CatalogURL(novel.BookID))
	novel.Chapters = s.parseChapterLinks(doc, pageURL)
	if novel.ChapterCount == 0 {
		novel.ChapterCount = len(novel.Chapters)
	}
	if novel.Title == "" && novel.BookID != "" {
		novel.Title = s.ID + "_" + strings.ReplaceAll(novel.BookID, "/", "_")
	}
	if novel.Title == "" {
		return nil, fmt.Errorf("%s 未找到书名", s.Name)
	}
	return novel, nil
}

func (s Source) ParseChapterHTML(htmlText string) (*ChapterContent, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	content := &ChapterContent{
		Title: cleanChapterTitle(firstNonEmptyText(doc, append(s.ChapterTitleSelectors, defaultChapterTitleSelectors...)...)),
	}
	for _, selector := range append(s.ChapterContentSelectors, defaultChapterContentSelectors...) {
		sel := doc.Find(selector).First()
		if sel.Length() == 0 {
			continue
		}
		text := cleanChapterContent(selectionText(sel))
		if text != "" {
			content.Content = text
			break
		}
	}
	if strings.TrimSpace(content.Content) == "" {
		return nil, fmt.Errorf("%s 未找到章节内容", s.Name)
	}
	if strings.TrimSpace(content.Title) == "" {
		content.Title = "chapter"
	}
	return content, nil
}

func (s Source) parseChapterLinks(doc *goquery.Document, pageURL string) []Chapter {
	selectors := append([]string{}, s.CatalogSelectors...)
	selectors = append(selectors, defaultCatalogSelectors...)
	seenSelectors := map[string]bool{}
	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if selector == "" || seenSelectors[selector] {
			continue
		}
		seenSelectors[selector] = true
		var chapters []Chapter
		seenURLs := map[string]bool{}
		doc.Find(selector).Each(func(_ int, item *goquery.Selection) {
			chapter, ok := s.chapterFromLink(item, pageURL)
			if !ok || seenURLs[chapter.URL] {
				return
			}
			chapter.Index = len(chapters) + 1
			seenURLs[chapter.URL] = true
			chapters = append(chapters, chapter)
		})
		if len(chapters) > 0 {
			return chapters
		}
	}
	var chapters []Chapter
	seenURLs := map[string]bool{}
	doc.Find("a[href]").Each(func(_ int, item *goquery.Selection) {
		chapter, ok := s.chapterFromLink(item, pageURL)
		if !ok || seenURLs[chapter.URL] {
			return
		}
		chapter.Index = len(chapters) + 1
		seenURLs[chapter.URL] = true
		chapters = append(chapters, chapter)
	})
	return chapters
}

func (s Source) chapterFromLink(item *goquery.Selection, pageURL string) (Chapter, bool) {
	href := strings.TrimSpace(attr(item, "href"))
	if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") {
		return Chapter{}, false
	}
	chapterURL := normalizeURL(href, pageURL, s.BaseURL)
	parts, ok := s.ParseURL(chapterURL)
	if !ok || parts.Kind != ContentTypeChapter {
		return Chapter{}, false
	}
	title := cleanChapterTitle(linkText(item))
	if title == "" || isNavChapterTitle(title) {
		return Chapter{}, false
	}
	if !chapterTextRE.MatchString(title) && !strings.Contains(title, "章") && !strings.Contains(title, "阅读") {
		return Chapter{}, false
	}
	return Chapter{
		Title:     title,
		URL:       parts.Canonical,
		UpdatedAt: strings.TrimSpace(firstNonEmpty(attr(item.Parent(), "data-etime"), strings.TrimSpace(item.Find("small").First().Text()))),
	}, true
}

func shouldFetchCatalog(sourceURL string, catalogURL string, novel *Novel) bool {
	catalogURL = strings.TrimSpace(catalogURL)
	if catalogURL == "" || novel == nil {
		return false
	}
	if strings.TrimRight(catalogURL, "/") == strings.TrimRight(sourceURL, "/") {
		return false
	}
	return len(novel.Chapters) == 0 || (novel.ChapterCount > 0 && len(novel.Chapters) < novel.ChapterCount)
}

func mergeFullCatalog(dst *Novel, full *Novel) {
	if dst == nil || full == nil {
		return
	}
	dst.BookID = firstNonEmpty(dst.BookID, full.BookID)
	dst.Title = firstNonEmpty(dst.Title, full.Title)
	dst.Author = firstNonEmpty(dst.Author, full.Author)
	dst.Category = firstNonEmpty(dst.Category, full.Category)
	dst.Status = firstNonEmpty(dst.Status, full.Status)
	dst.Description = firstNonEmpty(dst.Description, full.Description)
	dst.CoverURL = firstNonEmpty(dst.CoverURL, full.CoverURL)
	dst.WordCount = firstNonEmpty(dst.WordCount, full.WordCount)
	dst.UpdateTime = firstNonEmpty(dst.UpdateTime, full.UpdateTime)
	dst.LatestChapter = firstNonEmpty(dst.LatestChapter, full.LatestChapter)
	if dst.ChapterCount == 0 {
		dst.ChapterCount = full.ChapterCount
	}
	if len(full.Chapters) > len(dst.Chapters) {
		dst.Chapters = full.Chapters
	}
}

func cloneNovel(in *Novel) *Novel {
	if in == nil {
		return nil
	}
	out := *in
	if len(in.Chapters) > 0 {
		out.Chapters = append([]Chapter(nil), in.Chapters...)
	}
	return &out
}

func findCatalogURL(source Source, doc *goquery.Document, pageURL string, bookID string) string {
	if bookID != "" {
		if catalogURL := source.CatalogURL(bookID); catalogURL != "" {
			return catalogURL
		}
	}
	var out string
	doc.Find("a[href]").EachWithBreak(func(_ int, item *goquery.Selection) bool {
		text := linkText(item)
		if strings.Contains(text, "完整目录") || strings.Contains(text, "章节目录") || strings.Contains(text, "点击阅读") {
			out = normalizeURL(attr(item, "href"), pageURL, source.BaseURL)
			return false
		}
		return true
	})
	return out
}

func parseCountMetadata(doc *goquery.Document, novel *Novel) {
	bodyText := compactText(doc.Find("body").Text())
	if novel.WordCount == "" {
		if match := wordCountRE.FindString(bodyText); match != "" {
			novel.WordCount = strings.TrimSpace(match)
		}
	}
	for _, label := range []string{"章节数", "总章节"} {
		value := textByLabel(doc, label)
		if value == "" {
			continue
		}
		if count := parseInt(value); count > 0 {
			novel.ChapterCount = count
			return
		}
	}
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

func metaAny(doc *goquery.Document, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	for _, selector := range []string{
		fmt.Sprintf(`meta[property="%s"]`, key),
		fmt.Sprintf(`meta[name="%s"]`, key),
		fmt.Sprintf(`meta[itemprop="%s"]`, key),
	} {
		if value := attr(doc.Find(selector).First(), "content"); value != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func attr(sel *goquery.Selection, key string) string {
	if sel == nil || sel.Length() == 0 {
		return ""
	}
	value, ok := sel.Attr(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func firstNonEmptyText(doc *goquery.Document, selectors ...string) string {
	seen := map[string]bool{}
	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if selector == "" || seen[selector] {
			continue
		}
		seen[selector] = true
		if text := strings.TrimSpace(doc.Find(selector).First().Text()); text != "" {
			return text
		}
	}
	return ""
}

func textByLabel(doc *goquery.Document, label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	var out string
	doc.Find("p, span, li, div").EachWithBreak(func(_ int, item *goquery.Selection) bool {
		text := compactText(item.Text())
		if !strings.Contains(text, label) {
			return true
		}
		value := afterAnyLabel(text, label+"：", label+":")
		if value == "" || len([]rune(value)) > 80 {
			return true
		}
		out = value
		return false
	})
	return out
}

func afterAnyLabel(text string, labels ...string) string {
	for _, label := range labels {
		if idx := strings.Index(text, label); idx >= 0 {
			return strings.TrimSpace(text[idx+len(label):])
		}
	}
	return ""
}

func selectionText(sel *goquery.Selection) string {
	if sel == nil || sel.Length() == 0 {
		return ""
	}
	clone := sel.Clone()
	clone.Find("script, style, iframe, ins, .ads, .ad, #ads, .txtright, #txtright, .bottom-ad, .bottem, .bottem1, .bottem2, .page, .pager, .prenext, nav, h1").Remove()
	htmlText, _ := clone.Html()
	return htmlToText(htmlText)
}

func linkText(sel *goquery.Selection) string {
	if sel == nil || sel.Length() == 0 {
		return ""
	}
	clone := sel.Clone()
	clone.Find("small, span.update, span.time").Remove()
	return cleanChapterTitle(clone.Text())
}

func htmlToText(s string) string {
	replacements := map[string]string{
		"<br>":       "\n",
		"<br/>":      "\n",
		"<br />":     "\n",
		"</p>":       "\n",
		"</div>":     "\n",
		"</dd>":      "\n",
		"&nbsp;":     " ",
		"&#160;":     " ",
		"\r\n":       "\n",
		"\u00a0":     " ",
		"&amp;nbsp;": " ",
	}
	for old, replacement := range replacements {
		s = strings.ReplaceAll(s, old, replacement)
	}
	return strings.TrimSpace(stdhtml.UnescapeString(tagRE.ReplaceAllString(s, "")))
}

func cleanChapterContent(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = compactText(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(line, "本章完") ||
			strings.Contains(line, "请收藏本站") ||
			strings.Contains(line, "最新网址") ||
			strings.Contains(line, "手机用户") ||
			strings.Contains(line, "无弹窗") ||
			strings.Contains(lower, "www.") && strings.Contains(line, "小说") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

func cleanBookTitle(s string) string {
	s = cleanChapterTitle(s)
	s = strings.Trim(s, "《》")
	s = titleJunkRE.ReplaceAllString(s, "")
	if idx := strings.Index(s, ","); idx > 0 && strings.Contains(s, "最新章节") {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func cleanChapterTitle(s string) string {
	s = compactText(s)
	s = strings.TrimSpace(strings.Trim(s, " -_|\t\r\n"))
	for _, prefix := range []string{"最新章节：", "最新章节:", "章节目录", "正文 "} {
		s = strings.TrimPrefix(s, prefix)
	}
	return strings.TrimSpace(s)
}

func cleanLabel(s string, labels ...string) string {
	s = compactText(s)
	for _, label := range labels {
		s = strings.TrimPrefix(s, label+"：")
		s = strings.TrimPrefix(s, label+":")
	}
	return strings.TrimSpace(s)
}

func cleanDescription(s string) string {
	lines := strings.Split(htmlToText(s), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = compactText(line)
		if line == "" || strings.Contains(line, "最新章节") && strings.Contains(line, "免费") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func compactText(s string) string {
	s = stdhtml.UnescapeString(strings.ReplaceAll(s, "\u00a0", " "))
	return strings.TrimSpace(spaceRE.ReplaceAllString(s, " "))
}

func isNavChapterTitle(title string) bool {
	switch strings.TrimSpace(title) {
	case "上一章", "下一章", "目录", "返回目录", "最新章节", "TXT下载":
		return true
	default:
		return false
	}
}

func parseInt(value string) int {
	digits := regexp.MustCompile(`\d+`).FindString(value)
	if digits == "" {
		return 0
	}
	n, _ := strconv.Atoi(digits)
	return n
}

func normalizeURL(rawURL string, pageURL string, baseURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "//") {
		if parsed, err := url.Parse(pageURL); err == nil && parsed.Scheme != "" {
			return parsed.Scheme + ":" + rawURL
		}
		return "https:" + rawURL
	}
	if parsed, ok := parseHTTPURL(rawURL); ok {
		return parsed.String()
	}
	base := pageURL
	if strings.TrimSpace(base) == "" {
		base = baseURL + "/"
	}
	parsedBase, err := url.Parse(base)
	if err != nil || parsedBase == nil {
		return joinBaseURL(baseURL, rawURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil {
		return ""
	}
	return parsedBase.ResolveReference(parsed).String()
}
