package webpage

import (
	"fmt"
	stdhtml "html"
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	xhtml "golang.org/x/net/html"
)

const removeSelectors = `script, style, noscript, template, iframe,
nav, header, footer, aside, form,
[role="navigation"], [role="banner"], [role="contentinfo"], [role="search"],
.nav, .navbar, .menu, .sidebar, .breadcrumb, .breadcrumbs,
.search, .search-form, .site-header, .site-footer, .site-nav,
.mega-menu, .megamenu, [class*="mega-menu"], [class*="megamenu"],
[class*="global-nav"], [class*="search-panel"],
#nav, #header, #footer, #sidebar, #menu,
.cookie-banner, .cookie-notice, .share-buttons, .social-share, [class*="share"],
.related-posts, .recommended, .blog-sidebar, .mobile-credits, [id*="blog-name"],
.post-author, .post-date, .category-link, .advertisement, .ad, .ads, [class*="advert"]`

var (
	widgetTokenRE     = regexp.MustCompile(`\b(?:flw-widget-newsletter|flw-article-recirculation(?:-v\d+)?|widget-newsletter|recirculation)\b`)
	positiveClassIDRE = regexp.MustCompile(`(?i)(article|body|content|entry|hentry|main|page|pagination|post|story|text|blog|rich_media_content|js_content)`)
	negativeClassIDRE = regexp.MustCompile(`(?i)(comment|combx|contact|foot|footer|footnote|masthead|media|meta|outbrain|promo|related|scroll|share|shoutbox|sidebar|sponsor|shopping|tags|tool|widget|nav|menu|breadcrumb|advert|ads|banner|cookie|newsletter|recirculation)`)
	articleMainRE     = regexp.MustCompile(`(?i)<\s*(article|main)\b`)
)

type pageMetadata struct {
	Title         string
	Description   string
	CanonicalURL  string
	Author        string
	SiteName      string
	Language      string
	CoverURL      string
	PublishedTime string
	ModifiedTime  string
}

type candidate struct {
	Selection *goquery.Selection
	Score     float64
	TextLen   int
	Reason    string
}

func ExtractReadableArticle(rawHTML string, sourceURL string) (*ArticlePage, error) {
	rawHTML = strings.TrimSpace(rawHTML)
	if rawHTML == "" {
		return nil, fmt.Errorf("extract readable article: empty html")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil, fmt.Errorf("parse webpage html: %w", err)
	}
	preprocessDocument(doc)
	absolutifyDocumentURLs(doc, sourceURL)
	metadata := extractPageMetadata(doc, sourceURL)
	cleanDocument(doc)

	best := bestArticleCandidate(doc)
	if best == nil || best.Selection == nil || best.Selection.Length() == 0 {
		best = &candidate{
			Selection: doc.Find("body").First(),
			Reason:    "fallback body",
		}
	}
	cleanCandidate(best.Selection)
	innerHTML, err := best.Selection.Html()
	if err != nil {
		return nil, fmt.Errorf("extract readable article html: %w", err)
	}
	extractedHTML := sanitizeHTMLFragment(innerHTML)
	bodyText := normalizeText(htmlText(extractedHTML))
	if runeLen(bodyText) < 40 {
		return nil, fmt.Errorf("extract readable article: body is too thin")
	}
	title := firstNonEmpty(metadata.Title, firstHeadingText(best.Selection), hostDisplayName(sourceURL), "webpage")
	article := &ArticlePage{
		ID:              articleID(firstNonEmpty(metadata.CanonicalURL, sourceURL), title),
		Title:           title,
		Description:     metadata.Description,
		Author:          metadata.Author,
		SiteName:        metadata.SiteName,
		Language:        metadata.Language,
		SourceURL:       sourceURL,
		CanonicalURL:    firstNonEmpty(metadata.CanonicalURL, sourceURL),
		CoverURL:        metadata.CoverURL,
		PublishedTime:   metadata.PublishedTime,
		ModifiedTime:    metadata.ModifiedTime,
		ExtractedHTML:   extractedHTML,
		BodyText:        bodyText,
		Extractor:       "readability",
		ExtractorReason: best.Reason,
		RawHTMLLength:   len(rawHTML),
		ContentLength:   runeLen(bodyText),
	}
	article.Quality = qualityScore(article, rawHTML, extractedHTML)
	article.BodyHTML = renderArticleHTML(article)
	return article, nil
}

func preprocessDocument(doc *goquery.Document) {
	doc.Find(`p.paywall, p[aria-hidden="true"]`).Each(func(_ int, s *goquery.Selection) {
		if attr, _ := s.Attr("aria-hidden"); strings.EqualFold(attr, "true") {
			s.RemoveAttr("aria-hidden")
		}
		if className, ok := s.Attr("class"); ok {
			next := strings.Join(removeClassTokens(strings.Fields(className), "paywall"), " ")
			if next == "" {
				s.RemoveAttr("class")
			} else {
				s.SetAttr("class", next)
			}
		}
	})
	doc.Find("div").Each(func(_ int, s *goquery.Selection) {
		className, ok := s.Attr("class")
		if !ok || !widgetTokenRE.MatchString(className) {
			return
		}
		substantial := 0
		s.Find("p").Each(func(_ int, p *goquery.Selection) {
			if runeLen(normalizeText(p.Text())) >= 80 {
				substantial++
			}
		})
		if substantial < 5 {
			return
		}
		next := strings.TrimSpace(widgetTokenRE.ReplaceAllString(className, ""))
		next = strings.Join(strings.Fields(next), " ")
		if next == "" {
			s.RemoveAttr("class")
		} else {
			s.SetAttr("class", next)
		}
	})
	doc.Find(`input[type="text"][readonly]`).Each(func(_ int, s *goquery.Selection) {
		value, _ := s.Attr("value")
		if strings.TrimSpace(value) == "" {
			return
		}
		s.ReplaceWithHtml("<code>" + stdhtml.EscapeString(value) + "</code>")
	})
	doc.Find(`img[src^="data:"]`).Each(func(_ int, s *goquery.Selection) {
		alt, _ := s.Attr("alt")
		if strings.TrimSpace(alt) != "" {
			s.ReplaceWithHtml(stdhtml.EscapeString(alt))
			return
		}
		s.Remove()
	})
}

func cleanDocument(doc *goquery.Document) {
	doc.Find(removeSelectors).Remove()
}

func cleanCandidate(s *goquery.Selection) {
	s.Find(removeSelectors).Remove()
	s.Find("*").Each(func(_ int, child *goquery.Selection) {
		tag := strings.ToLower(goquery.NodeName(child))
		if tag == "p" || tag == "li" || tag == "blockquote" || strings.HasPrefix(tag, "h") {
			return
		}
		textLen := runeLen(normalizeText(child.Text()))
		if textLen == 0 && child.Find("img, video, audio, iframe").Length() == 0 {
			child.Remove()
			return
		}
		classID := classIDText(child)
		if textLen < 350 && negativeClassIDRE.MatchString(classID) && !positiveClassIDRE.MatchString(classID) {
			child.Remove()
			return
		}
		if textLen < 600 && linkDensity(child) > 0.55 {
			child.Remove()
		}
	})
	s.Find("p, div").Each(func(_ int, child *goquery.Selection) {
		if runeLen(normalizeText(child.Text())) == 0 && child.Find("img, video, audio").Length() == 0 {
			child.Remove()
		}
	})
}

func bestArticleCandidate(doc *goquery.Document) *candidate {
	var best *candidate
	selector := `article, main, [role="main"], #js_content, .rich_media_content,
.article, .article-content, .post-content, .entry-content, .content, #content,
section, div, td, body`
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		c := scoreCandidate(s)
		if c == nil {
			return
		}
		if best == nil || c.Score > best.Score {
			best = c
		}
	})
	return best
}

func scoreCandidate(s *goquery.Selection) *candidate {
	text := normalizeText(s.Text())
	textLen := runeLen(text)
	tag := strings.ToLower(goquery.NodeName(s))
	if textLen < 80 && tag != "body" && tag != "article" && tag != "main" {
		return nil
	}
	pCount := 0
	s.Find("p, li").Each(func(_ int, p *goquery.Selection) {
		if runeLen(normalizeText(p.Text())) >= 45 {
			pCount++
		}
	})
	headings := s.Find("h1, h2, h3").Length()
	images := s.Find("img[src]").Length()
	punct := punctuationCount(text)
	score := float64(textLen) + float64(pCount)*140 + float64(headings)*80 + float64(punct)*8 + float64(images)*30
	classID := classIDText(s)
	if positiveClassIDRE.MatchString(classID) {
		score += 500
	}
	if negativeClassIDRE.MatchString(classID) && !positiveClassIDRE.MatchString(classID) {
		score -= 700
	}
	switch tag {
	case "article", "main":
		score += 500
	case "body":
		score -= 250
	}
	density := linkDensity(s)
	if density > 0.18 {
		score *= math.Max(0.15, 1-density)
	}
	listItems := s.Find("li").Length()
	if pCount == 0 && listItems > 12 {
		score *= 0.45
	}
	return &candidate{
		Selection: s,
		Score:     score,
		TextLen:   textLen,
		Reason:    fmt.Sprintf("best candidate %s: score %.0f, text %d, p %d, link density %.2f", tag, score, textLen, pCount, density),
	}
}

func extractPageMetadata(doc *goquery.Document, sourceURL string) pageMetadata {
	title := firstNonEmpty(
		strings.TrimSpace(doc.Find("title").First().Text()),
		metaContent(doc, "property", "og:title"),
		metaContent(doc, "name", "twitter:title"),
		strings.TrimSpace(doc.Find("h1").First().Text()),
	)
	description := firstNonEmpty(
		metaContent(doc, "name", "description"),
		metaContent(doc, "property", "og:description"),
		metaContent(doc, "name", "twitter:description"),
	)
	canonical := firstNonEmpty(
		linkHref(doc, "canonical"),
		metaContent(doc, "property", "og:url"),
		sourceURL,
	)
	author := firstNonEmpty(
		metaContent(doc, "name", "author"),
		metaContent(doc, "property", "article:author"),
		doc.Find(`[rel="author"]`).First().Text(),
	)
	cover := firstNonEmpty(
		metaContent(doc, "property", "og:image"),
		metaContent(doc, "name", "twitter:image"),
		doc.Find("article img[src], main img[src], img[src]").First().AttrOr("src", ""),
	)
	language := firstNonEmpty(
		strings.TrimSpace(doc.Find("html").First().AttrOr("lang", "")),
		metaContent(doc, "property", "og:locale"),
	)
	return pageMetadata{
		Title:         title,
		Description:   description,
		CanonicalURL:  absolutifyURL(canonical, sourceURL),
		Author:        strings.TrimSpace(author),
		SiteName:      metaContent(doc, "property", "og:site_name"),
		Language:      language,
		CoverURL:      absolutifyURL(cover, sourceURL),
		PublishedTime: firstNonEmpty(metaContent(doc, "property", "article:published_time"), metaContent(doc, "name", "date"), metaContent(doc, "name", "publish_date"), doc.Find("time[datetime]").First().AttrOr("datetime", "")),
		ModifiedTime:  firstNonEmpty(metaContent(doc, "property", "article:modified_time"), metaContent(doc, "name", "lastmod")),
	}
}

func metaContent(doc *goquery.Document, attr string, value string) string {
	out := ""
	doc.Find("meta").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		attrValue, _ := s.Attr(attr)
		if !strings.EqualFold(strings.TrimSpace(attrValue), value) {
			return true
		}
		content, _ := s.Attr("content")
		out = strings.TrimSpace(content)
		return out == ""
	})
	return out
}

func linkHref(doc *goquery.Document, rel string) string {
	out := ""
	doc.Find("link[rel]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		attrValue, _ := s.Attr("rel")
		if !strings.EqualFold(strings.TrimSpace(attrValue), rel) {
			return true
		}
		out = strings.TrimSpace(s.AttrOr("href", ""))
		return out == ""
	})
	return out
}

func absolutifyDocumentURLs(doc *goquery.Document, baseURL string) {
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		if href := absolutifyURL(s.AttrOr("href", ""), baseURL); href != "" {
			s.SetAttr("href", href)
		}
	})
	doc.Find("img[src], source[src], video[src], audio[src]").Each(func(_ int, s *goquery.Selection) {
		if src := absolutifyURL(s.AttrOr("src", ""), baseURL); src != "" {
			s.SetAttr("src", src)
		}
	})
	doc.Find("img[srcset], source[srcset]").Each(func(_ int, s *goquery.Selection) {
		if srcset := absolutifySrcset(s.AttrOr("srcset", ""), baseURL); srcset != "" {
			s.SetAttr("srcset", srcset)
		}
	})
	doc.Find("video[poster]").Each(func(_ int, s *goquery.Selection) {
		if poster := absolutifyURL(s.AttrOr("poster", ""), baseURL); poster != "" {
			s.SetAttr("poster", poster)
		}
	})
}

func absolutifyURL(value string, baseURL string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "#") || nonResolvableURL(value) {
		return value
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.Scheme != "" {
		return parsed.String()
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return value
	}
	return base.ResolveReference(parsed).String()
}

func absolutifySrcset(value string, baseURL string) string {
	parts := strings.Split(value, ",")
	for i, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 {
			continue
		}
		fields[0] = absolutifyURL(fields[0], baseURL)
		parts[i] = strings.Join(fields, " ")
	}
	return strings.Join(parts, ", ")
}

func nonResolvableURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "data:") ||
		strings.HasPrefix(value, "mailto:") ||
		strings.HasPrefix(value, "javascript:") ||
		strings.HasPrefix(value, "tel:")
}

func sanitizeHTMLFragment(fragment string) string {
	const rootID = "wx-readable-root"
	doc, err := xhtml.Parse(strings.NewReader(`<html><body><div id="` + rootID + `">` + fragment + `</div></body></html>`))
	if err != nil {
		return stdhtml.EscapeString(htmlText(fragment))
	}
	root := findElementByID(doc, rootID)
	if root == nil {
		root = findFirstElement(doc, "body")
	}
	if root == nil {
		return stdhtml.EscapeString(htmlText(fragment))
	}
	var b strings.Builder
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		renderSanitizedNode(&b, node)
	}
	return strings.TrimSpace(b.String())
}

func findElementByID(node *xhtml.Node, id string) *xhtml.Node {
	if node == nil {
		return nil
	}
	if node.Type == xhtml.ElementNode {
		for _, attr := range node.Attr {
			if attr.Key == "id" && attr.Val == id {
				return node
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findElementByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

func findFirstElement(node *xhtml.Node, tag string) *xhtml.Node {
	if node == nil {
		return nil
	}
	if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, tag) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirstElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func renderSanitizedNode(b *strings.Builder, node *xhtml.Node) {
	switch node.Type {
	case xhtml.TextNode:
		b.WriteString(stdhtml.EscapeString(node.Data))
	case xhtml.ElementNode:
		tag := strings.ToLower(node.Data)
		if dropElement(tag) {
			return
		}
		if !allowedElement(tag) {
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				renderSanitizedNode(b, child)
			}
			return
		}
		attrs := sanitizedAttrs(tag, node.Attr)
		if tag == "img" && attrValue(attrs, "src") == "" {
			if alt := attrValue(attrs, "alt"); alt != "" {
				b.WriteString(stdhtml.EscapeString(alt))
			}
			return
		}
		b.WriteByte('<')
		b.WriteString(tag)
		for _, attr := range attrs {
			b.WriteByte(' ')
			b.WriteString(attr.Key)
			b.WriteString(`="`)
			b.WriteString(stdhtml.EscapeString(attr.Val))
			b.WriteByte('"')
		}
		if tag == "audio" || tag == "video" {
			b.WriteString(` controls`)
		}
		b.WriteByte('>')
		if voidElement(tag) {
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			renderSanitizedNode(b, child)
		}
		b.WriteString("</")
		b.WriteString(tag)
		b.WriteByte('>')
	default:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			renderSanitizedNode(b, child)
		}
	}
}

func dropElement(tag string) bool {
	switch tag {
	case "script", "style", "noscript", "template", "iframe", "object", "embed":
		return true
	default:
		return false
	}
}

func allowedElement(tag string) bool {
	switch tag {
	case "a", "abbr", "article", "audio", "b", "blockquote", "br", "caption", "code", "del", "details", "div", "em", "figcaption", "figure", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "i", "img", "li", "ol", "p", "pre", "section", "source", "span", "strong", "sub", "summary", "sup", "table", "tbody", "td", "th", "thead", "tr", "u", "ul", "video":
		return true
	default:
		return false
	}
}

func voidElement(tag string) bool {
	switch tag {
	case "br", "hr", "img", "source":
		return true
	default:
		return false
	}
}

func sanitizedAttrs(tag string, attrs []xhtml.Attribute) []xhtml.Attribute {
	out := make([]xhtml.Attribute, 0, 3)
	add := func(key, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		out = append(out, xhtml.Attribute{Key: key, Val: value})
	}
	for _, attr := range attrs {
		key := strings.ToLower(strings.TrimSpace(attr.Key))
		value := strings.TrimSpace(attr.Val)
		switch tag {
		case "a":
			if key == "href" && allowedLinkURL(value) {
				add(key, value)
			} else if key == "title" {
				add(key, value)
			}
		case "img":
			if key == "src" && allowedMediaURL(value) {
				add(key, value)
			} else if key == "srcset" {
				add(key, sanitizeSrcset(value))
			} else if key == "alt" || key == "title" {
				add(key, value)
			}
		case "video":
			if (key == "src" || key == "poster") && allowedMediaURL(value) {
				add(key, value)
			}
		case "audio", "source":
			if key == "src" && allowedMediaURL(value) {
				add(key, value)
			} else if key == "type" {
				add(key, value)
			}
		case "td", "th":
			if key == "colspan" || key == "rowspan" {
				add(key, value)
			}
		}
	}
	return out
}

func allowedLinkURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "#") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" {
		return true
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "mailto", "tel":
		return true
	default:
		return false
	}
}

func allowedMediaURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" {
		return true
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

func sanitizeSrcset(value string) string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 || !allowedMediaURL(fields[0]) {
			continue
		}
		out = append(out, strings.Join(fields, " "))
	}
	return strings.Join(out, ", ")
}

func attrValue(attrs []xhtml.Attribute, key string) string {
	for _, attr := range attrs {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func renderArticleHTML(article *ArticlePage) string {
	lang := firstNonEmpty(article.Language, "zh-CN")
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"")
	b.WriteString(stdhtml.EscapeString(lang))
	b.WriteString("\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>")
	b.WriteString(stdhtml.EscapeString(firstNonEmpty(article.Title, "webpage")))
	b.WriteString("</title>\n")
	if article.SourceURL != "" {
		b.WriteString("<meta name=\"source_url\" content=\"")
		b.WriteString(stdhtml.EscapeString(article.SourceURL))
		b.WriteString("\">\n")
	}
	b.WriteString("<style>body{font-family:-apple-system,BlinkMacSystemFont,\"Segoe UI\",sans-serif;line-height:1.75;margin:0;background:#f7f7f5;color:#1f2328}main{max-width:860px;margin:0 auto;padding:32px 20px 56px;background:#fff;min-height:100vh}h1{font-size:30px;line-height:1.3;margin:0 0 16px}h2{font-size:22px;margin:30px 0 12px}h3{font-size:18px;margin:24px 0 10px}.meta{display:grid;grid-template-columns:max-content 1fr;gap:6px 14px;margin:0 0 24px;color:#4f5762;font-size:14px}.lead{color:#4f5762;margin:0 0 24px}dt{font-weight:600}dd{margin:0;word-break:break-all}p{margin:0 0 14px}ol,ul{padding-left:1.6em}li{margin:6px 0}a{color:#0b65c2;text-decoration:none}a:hover{text-decoration:underline}img,video{max-width:100%;height:auto}blockquote{border-left:4px solid #d0d7de;margin:16px 0;padding:0 0 0 14px;color:#4f5762}pre{overflow:auto;background:#f6f8fa;padding:12px;border-radius:6px}table{border-collapse:collapse;width:100%;margin:16px 0}td,th{border:1px solid #d0d7de;padding:6px 8px}</style>\n")
	b.WriteString("</head>\n<body>\n<main>\n<header>\n<h1>")
	b.WriteString(stdhtml.EscapeString(firstNonEmpty(article.Title, "webpage")))
	b.WriteString("</h1>\n<dl class=\"meta\">\n")
	writeTerm(&b, "来源", article.CanonicalURL)
	writeTerm(&b, "作者", article.Author)
	writeTerm(&b, "站点", article.SiteName)
	writeTerm(&b, "发布时间", article.PublishedTime)
	writeTerm(&b, "更新时间", article.ModifiedTime)
	b.WriteString("</dl>\n")
	if strings.TrimSpace(article.Description) != "" {
		b.WriteString("<p class=\"lead\">")
		b.WriteString(stdhtml.EscapeString(article.Description))
		b.WriteString("</p>\n")
	}
	b.WriteString("</header>\n<article>\n")
	b.WriteString(article.ExtractedHTML)
	b.WriteString("\n</article>\n</main>\n</body>\n</html>\n")
	return b.String()
}

func writeTerm(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString("<dt>")
	b.WriteString(stdhtml.EscapeString(key))
	b.WriteString("</dt><dd>")
	b.WriteString(stdhtml.EscapeString(value))
	b.WriteString("</dd>\n")
}

func qualityScore(article *ArticlePage, rawHTML string, extractedHTML string) float64 {
	if article == nil || article.ContentLength == 0 {
		return 0
	}
	score := 0.0
	if article.ContentLength > 500 {
		score += 0.3
	}
	if len(rawHTML) > 0 && float64(len(extractedHTML))/float64(len(rawHTML)) > 0.02 {
		score += 0.2
	}
	if articleMainRE.MatchString(rawHTML) {
		score += 0.2
	}
	if countHTMLTags(extractedHTML, "h1", "h2", "h3", "h4", "h5", "h6") >= 1 {
		score += 0.15
	}
	if countHTMLTags(extractedHTML, "p") >= 3 {
		score += 0.15
	}
	return math.Min(math.Round(score*100)/100, 1)
}

func countHTMLTags(fragment string, tags ...string) int {
	count := 0
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fragment))
	if err != nil {
		return 0
	}
	for _, tag := range tags {
		count += doc.Find(tag).Length()
	}
	return count
}

func linkDensity(s *goquery.Selection) float64 {
	textLen := runeLen(normalizeText(s.Text()))
	if textLen == 0 {
		return 0
	}
	linkLen := 0
	s.Find("a").Each(func(_ int, a *goquery.Selection) {
		linkLen += runeLen(normalizeText(a.Text()))
	})
	return math.Min(float64(linkLen)/float64(textLen), 1)
}

func classIDText(s *goquery.Selection) string {
	parts := make([]string, 0, 2)
	if id, ok := s.Attr("id"); ok {
		parts = append(parts, id)
	}
	if className, ok := s.Attr("class"); ok {
		parts = append(parts, className)
	}
	return strings.Join(parts, " ")
}

func removeClassTokens(tokens []string, remove ...string) []string {
	blocked := make(map[string]bool, len(remove))
	for _, token := range remove {
		blocked[strings.ToLower(token)] = true
	}
	out := tokens[:0]
	for _, token := range tokens {
		if !blocked[strings.ToLower(token)] {
			out = append(out, token)
		}
	}
	return out
}

func firstHeadingText(s *goquery.Selection) string {
	return strings.TrimSpace(s.Find("h1, h2, h3").First().Text())
}

func htmlText(fragment string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fragment))
	if err != nil {
		return fragment
	}
	return doc.Text()
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func runeLen(value string) int {
	return len([]rune(strings.TrimSpace(value)))
}

func punctuationCount(value string) int {
	count := 0
	for _, r := range value {
		if unicode.IsPunct(r) {
			count++
			continue
		}
		switch r {
		case '，', '。', '、', '：', '；', '！', '？', '「', '」', '“', '”':
			count++
		}
	}
	return count
}
