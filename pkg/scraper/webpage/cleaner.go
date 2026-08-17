package webpage

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

const removable_content_selector = "script,style,noscript,template,iframe,object,embed,svg,math,link,meta,base,nav,footer,aside,form,input,button,select,textarea,[hidden]"

var (
	code_language_pattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_+.-]*$`)
	code_class_pattern    = regexp.MustCompile(`^language-[a-zA-Z0-9][a-zA-Z0-9_+.-]*$`)
	hidden_style_pattern  = regexp.MustCompile(`(?i)(?:^|;)\s*(?:display\s*:\s*none|visibility\s*:\s*hidden|opacity\s*:\s*0(?:\.0*)?)\s*(?:!important\s*)?(?:;|$)`)
	archive_html_policy   = new_archive_html_policy()
)

func new_archive_html_policy() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	// Defuddle keeps only the normalized language-* class on code blocks. It is
	// semantic input for fenced Markdown, not a dependency on the source site.
	policy.AllowAttrs("class").Matching(code_class_pattern).OnElements("code")
	return policy
}

// clean_page_content follows the same broad separation used by Obsidian's
// Defuddle pipeline: remove page chrome, standardize useful elements, resolve
// resources, then run untrusted HTML through a semantic allowlist.
func clean_page_content(selection *goquery.Selection, base_url string) (string, error) {
	if selection == nil || selection.Length() == 0 {
		return "", nil
	}
	selection.Find(removable_content_selector).Remove()
	remove_hidden_content(selection)
	standardize_code_blocks(selection)
	standardize_heading_links(selection)
	normalize_content_urls(selection, base_url)

	dirty_html, err := selection.Html()
	if err != nil {
		return "", fmt.Errorf("extract webpage HTML: %w", err)
	}
	cleaned_html := strings.TrimSpace(archive_html_policy.Sanitize(dirty_html))
	if cleaned_html == "" {
		return "", nil
	}
	cleaned_document, err := goquery.NewDocumentFromReader(strings.NewReader(cleaned_html))
	if err != nil {
		return "", fmt.Errorf("parse sanitized webpage HTML: %w", err)
	}
	strip_unreferenced_ids(cleaned_document)
	body := cleaned_document.Find("body").First()
	if body.Length() == 0 {
		return cleaned_html, nil
	}
	cleaned_html, err = body.Html()
	if err != nil {
		return "", fmt.Errorf("serialize sanitized webpage HTML: %w", err)
	}
	return strings.TrimSpace(cleaned_html), nil
}

func remove_hidden_content(selection *goquery.Selection) {
	selection.Find("*").Each(func(_ int, item *goquery.Selection) {
		if hidden, exists := item.Attr("aria-hidden"); exists && strings.EqualFold(strings.TrimSpace(hidden), "true") {
			item.Remove()
			return
		}
		if style, exists := item.Attr("style"); exists && hidden_style_pattern.MatchString(style) {
			item.Remove()
		}
	})
}

func standardize_code_blocks(selection *goquery.Selection) {
	selection.Find("pre").Each(func(_ int, pre *goquery.Selection) {
		code := pre.Find("code").First()
		if code.Length() == 0 {
			return
		}
		language := code_language(code)
		if language == "" {
			language = code_language(pre)
		}
		code_text := code.Text()
		code.Empty().SetText(code_text)
		code.RemoveAttr("class")
		code.RemoveAttr("id")
		code.RemoveAttr("style")
		if language != "" {
			code.SetAttr("class", "language-"+language)
		}
	})
}

func code_language(selection *goquery.Selection) string {
	for _, attribute := range []string{"data-lang", "data-language", "lang"} {
		if value, exists := selection.Attr(attribute); exists {
			value = strings.TrimSpace(value)
			if code_language_pattern.MatchString(value) {
				return value
			}
		}
	}
	class_name, _ := selection.Attr("class")
	for _, class_token := range strings.Fields(class_name) {
		for _, prefix := range []string{"language-", "lang-"} {
			if strings.HasPrefix(class_token, prefix) {
				language := strings.TrimPrefix(class_token, prefix)
				if code_language_pattern.MatchString(language) {
					return language
				}
			}
		}
	}
	return ""
}

func standardize_heading_links(selection *goquery.Selection) {
	selection.Find("h1 a,h2 a,h3 a,h4 a,h5 a,h6 a").Each(func(_ int, link *goquery.Selection) {
		link.ReplaceWithSelection(link.Contents())
	})
}

func normalize_content_urls(selection *goquery.Selection, base_url string) {
	selection.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
		normalize_selection_url(link, "href", base_url, true)
	})
	selection.Find("blockquote[cite],q[cite],del[cite],ins[cite]").Each(func(_ int, item *goquery.Selection) {
		normalize_selection_url(item, "cite", base_url, false)
	})
	selection.Find("img").Each(func(_ int, image *goquery.Selection) {
		normalize_image_source(image, base_url)
	})
}

func normalize_selection_url(selection *goquery.Selection, attribute string, base_url string, allow_fragment bool) {
	value, exists := selection.Attr(attribute)
	if !exists {
		return
	}
	normalized_url := normalized_content_url(value, base_url, allow_fragment)
	if normalized_url == "" {
		selection.RemoveAttr(attribute)
		return
	}
	selection.SetAttr(attribute, normalized_url)
}

func normalized_content_url(reference string, base_url string, allow_fragment bool) string {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return ""
	}
	if allow_fragment && strings.HasPrefix(reference, "#") {
		return reference
	}
	parsed_reference, err := url.Parse(reference)
	if err != nil {
		return ""
	}
	if !parsed_reference.IsAbs() {
		parsed_base, parse_err := url.Parse(strings.TrimSpace(base_url))
		if parse_err != nil || parsed_base.Scheme == "" || parsed_base.Hostname() == "" {
			return ""
		}
		parsed_reference = parsed_base.ResolveReference(parsed_reference)
	}
	parsed_reference.Scheme = strings.ToLower(parsed_reference.Scheme)
	switch parsed_reference.Scheme {
	case "http", "https":
		return parsed_reference.String()
	case "mailto":
		if allow_fragment {
			return parsed_reference.String()
		}
	}
	return ""
}

func normalize_image_source(image *goquery.Selection, base_url string) {
	width_value, _ := image.Attr("width")
	height_value, _ := image.Attr("height")
	width := parse_positive_dimension(width_value)
	height := parse_positive_dimension(height_value)
	if width > 0 && height > 0 && width <= 2 && height <= 2 {
		image.Remove()
		return
	}
	source, _ := image.Attr("src")
	if source == "" || is_placeholder_image(source) {
		for _, attribute := range []string{"data-src", "data-original", "data-lazy-src"} {
			if candidate, exists := image.Attr(attribute); exists && strings.TrimSpace(candidate) != "" {
				source = candidate
				break
			}
		}
	}
	if source == "" || is_placeholder_image(source) {
		for _, attribute := range []string{"srcset", "data-srcset"} {
			if source_set, exists := image.Attr(attribute); exists {
				if candidate := best_srcset_source(source_set); candidate != "" {
					source = candidate
					break
				}
			}
		}
	}
	source = normalized_content_url(source, base_url, false)
	if source != "" {
		source = unwrap_image_proxy_url(source, base_url)
		image.SetAttr("src", source)
	} else {
		image.RemoveAttr("src")
	}
	// Responsive and lazy-loading attributes belong to the source page's
	// presentation/runtime. A portable archive keeps one canonical image URL.
	for _, attribute := range []string{"srcset", "sizes", "style", "class", "decoding", "loading", "fetchpriority", "data-nimg", "data-src", "data-srcset", "data-original", "data-lazy-src"} {
		image.RemoveAttr(attribute)
	}
}

func best_srcset_source(source_set string) string {
	best_source := ""
	best_score := float64(-1)
	for source_index, source_candidate := range strings.Split(source_set, ",") {
		parts := strings.Fields(strings.TrimSpace(source_candidate))
		if len(parts) == 0 {
			continue
		}
		score := float64(source_index)
		if len(parts) > 1 {
			descriptor := strings.ToLower(parts[len(parts)-1])
			switch {
			case strings.HasSuffix(descriptor, "w"):
				if width, err := strconv.ParseFloat(strings.TrimSuffix(descriptor, "w"), 64); err == nil {
					score = width
				}
			case strings.HasSuffix(descriptor, "x"):
				if density, err := strconv.ParseFloat(strings.TrimSuffix(descriptor, "x"), 64); err == nil {
					score = density * 10000
				}
			}
		}
		if score >= best_score {
			best_source = parts[0]
			best_score = score
		}
	}
	return best_source
}

func is_placeholder_image(source string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	return source == "" || source == "about:blank" || strings.HasPrefix(source, "data:image/gif;base64,r0lgodlhaqaba")
}

func unwrap_image_proxy_url(raw_url string, base_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || !strings.HasSuffix(strings.TrimRight(parsed_url.Path, "/"), "/_next/image") {
		return raw_url
	}
	original_url := strings.TrimSpace(parsed_url.Query().Get("url"))
	if original_url == "" {
		return raw_url
	}
	normalized_url := normalized_content_url(original_url, base_url, false)
	if normalized_url == "" {
		return raw_url
	}
	return normalized_url
}

func strip_unreferenced_ids(document *goquery.Document) {
	referenced_ids := make(map[string]struct{})
	document.Find(`a[href^="#"]`).Each(func(_ int, link *goquery.Selection) {
		href, _ := link.Attr("href")
		if referenced_id, err := url.PathUnescape(strings.TrimPrefix(href, "#")); err == nil && referenced_id != "" {
			referenced_ids[referenced_id] = struct{}{}
		}
	})
	document.Find("[id]").Each(func(_ int, item *goquery.Selection) {
		id, _ := item.Attr("id")
		if _, referenced := referenced_ids[id]; !referenced {
			item.RemoveAttr("id")
		}
	})
}

func parse_positive_dimension(value string) int {
	dimension, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(value, "px")))
	if err != nil || dimension < 0 {
		return 0
	}
	return dimension
}
