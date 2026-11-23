package x

import (
	"html"
	"net/url"
	"strings"
)

func article_from_object(entity_record map[string]any) *Article {
	return normalize_article(&Article{
		ID:       tsr_string(entity_record["rest_id"]),
		Title:    tsr_string(entity_record["title"]),
		Text:     first_non_empty(tsr_string(entity_record["plain_text"]), tsr_string(entity_record["preview_text"])),
		HTML:     article_html(entity_record),
		CoverURL: article_cover_url(entity_record),
	})
}

// article_value resolves the legacy record graph into the same nested values
// the streamed JSON supplies, so both formats use the same article renderer.
func (r *tsr_reader) article_value(value any, depth int) any {
	if depth >= tsr_slot_depth_limit {
		return nil
	}
	switch record := value.(type) {
	case map[string]any:
		if ref := tsr_string(record["__ref"]); ref != "" {
			source_record := extract_record(r.source, ref)
			if source_record == "" {
				return nil
			}
			resolved, _ := r.value_at(strings.Index(r.source, source_record))
			return r.article_value(resolved, depth+1)
		}
		if refs, ok := record["__refs"].([]any); ok {
			items := make([]any, 0, len(refs))
			for _, ref := range refs {
				items = append(items, r.article_value(map[string]any{"__ref": ref}, depth+1))
			}
			return items
		}
		resolved := make(map[string]any, len(record))
		for key, child := range record {
			resolved[key] = r.article_value(child, depth+1)
		}
		return resolved
	case []any:
		items := make([]any, 0, len(record))
		for _, child := range record {
			items = append(items, r.article_value(child, depth+1))
		}
		return items
	}
	return value
}

// Draft.js blocks and entities retain the article's layout and media, unlike plain_text.
func article_html(entity_record map[string]any) string {
	state_record := tsr_object(entity_record["content_state"])
	media_urls := article_media_urls(entity_record)
	entities := article_entities(state_record)
	parts := make([]string, 0, 64)
	if title := tsr_string(entity_record["title"]); title != "" {
		parts = append(parts, `<h1 data-n="article-title">`+html.EscapeString(title)+`</h1>`)
	}
	if cover_url := normalize_media_url(article_cover_url(entity_record), "pbs.twimg.com"); cover_url != "" {
		parts = append(parts, `<img data-n="article-cover" src="`+html.EscapeString(cover_url)+`" alt="">`)
	}
	blocks := tsr_array(state_record["blocks"])
	if len(blocks) == 0 {
		if text := first_non_empty(tsr_string(entity_record["plain_text"]), tsr_string(entity_record["preview_text"])); text != "" {
			parts = append(parts, `<p data-n="article-paragraph">`+article_text_html(text)+`</p>`)
		}
	}
	list_tag := ""
	for _, block_value := range blocks {
		block := tsr_object(block_value)
		next_list_tag := ""
		switch tsr_string(block["type"]) {
		case "ordered-list-item":
			next_list_tag = "ol"
		case "unordered-list-item":
			next_list_tag = "ul"
		}
		if list_tag != next_list_tag {
			if list_tag != "" {
				parts = append(parts, "</"+list_tag+">")
			}
			if next_list_tag != "" {
				parts = append(parts, "<"+next_list_tag+` data-n="article-list">`)
			}
			list_tag = next_list_tag
		}
		if block_html := article_block_html(block, entities, media_urls); block_html != "" {
			parts = append(parts, block_html)
		}
	}
	if list_tag != "" {
		parts = append(parts, "</"+list_tag+">")
	}
	if len(parts) == 0 {
		return ""
	}
	return `<article data-n="x-article">` + strings.Join(parts, "\n") + `</article>`
}

// article_cover_url is the article's own cover image, which X keeps in a
// dedicated media result rather than in the tweet's media entities.
func article_cover_url(entity_record map[string]any) string {
	results_record := tsr_object(entity_record["cover_media_results"])
	return article_media_image_url(tsr_object(results_record["result"]))
}

// article_entity is one entry of the content_state entity map.
type article_entity struct {
	kind string
	data map[string]any
}

// inline_style_markers maps Draft.js inline styles to HTML tags.
var inline_style_markers = map[string][2]string{
	"Bold":          {`<strong data-n="article-bold">`, "</strong>"},
	"Italic":        {`<em data-n="article-italic">`, "</em>"},
	"Strikethrough": {`<del data-n="article-strikethrough">`, "</del>"},
}

// article_media_urls maps each article media id to its full-size image URL.
func article_media_urls(entity_record map[string]any) map[string]string {
	urls := make(map[string]string)
	for _, media_value := range tsr_array(entity_record["media_entities"]) {
		media_record := tsr_object(media_value)
		media_id := tsr_string(media_record["media_id"])
		if media_id == "" {
			continue
		}
		if image_url := normalize_media_url(article_media_image_url(media_record), "pbs.twimg.com"); image_url != "" {
			urls[media_id] = image_url
		}
	}
	return urls
}

func article_media_image_url(media_record map[string]any) string {
	info_record := tsr_object(media_record["media_info"])
	return tsr_string(info_record["original_img_url"])
}

func article_entities(state_record map[string]any) map[int]article_entity {
	entities := make(map[int]article_entity)
	for _, entry_value := range tsr_array(state_record["entity_map"]) {
		entry_record := tsr_object(entry_value)
		value_record := tsr_object(entry_record["value"])
		if len(value_record) == 0 {
			continue
		}
		entities[int(tsr_int64(entry_record["key"]))] = article_entity{
			kind: tsr_string(value_record["type"]),
			data: tsr_object(value_record["data"]),
		}
	}
	return entities
}

func article_block_html(block map[string]any, entities map[int]article_entity, media_urls map[string]string) string {
	block_type := tsr_string(block["type"])
	if block_type == "atomic" {
		return article_atomic_html(block, entities, media_urls)
	}
	text := article_inline_html(block, entities)
	if text == "" {
		return ""
	}
	tag, name := "p", "article-paragraph"
	switch block_type {
	case "header-one":
		tag, name = "h1", "article-heading"
	case "header-two":
		tag, name = "h2", "article-heading"
	case "header-three":
		tag, name = "h3", "article-heading"
	case "blockquote":
		tag, name = "blockquote", "article-quote"
	case "unordered-list-item", "ordered-list-item":
		tag, name = "li", "article-list-item"
	case "code-block":
		plain_text, _ := block["text"].(string)
		return `<pre data-n="article-code-block"><code data-n="article-code">` + html.EscapeString(plain_text) + `</code></pre>`
	}
	return "<" + tag + ` data-n="` + name + `">` + text + "</" + tag + ">"
}

func article_atomic_html(block map[string]any, entities map[int]article_entity, media_urls map[string]string) string {
	entity, ok := article_block_entity(block, entities)
	if !ok {
		return ""
	}
	switch entity.kind {
	case "MEDIA":
		return article_media_html(entity.data, media_urls)
	case "MARKDOWN":
		// shortcut: Markdown embeds retain source text; add a renderer when formatted embeds are needed.
		return `<pre data-n="article-markdown">` + html.EscapeString(tsr_string(entity.data["markdown"])) + `</pre>`
	case "DIVIDER":
		return `<hr data-n="article-divider">`
	case "TWEET":
		if tweet_id := tsr_string(entity.data["tweet_id"]); decimal_string(tweet_id) {
			return `<p data-n="article-embedded-post"><a data-n="article-post-link" href="https://x.com/i/status/` + tweet_id + `">https://x.com/i/status/` + tweet_id + `</a></p>`
		}
	}
	return ""
}

func article_block_entity(block map[string]any, entities map[int]article_entity) (article_entity, bool) {
	for _, range_value := range tsr_array(block["entity_ranges"]) {
		range_record := tsr_object(range_value)
		if entity, ok := entities[int(tsr_int64(range_record["key"]))]; ok {
			return entity, true
		}
	}
	return article_entity{}, false
}

func article_media_html(data map[string]any, media_urls map[string]string) string {
	images := make([]string, 0, 2)
	for _, item_value := range tsr_array(data["media_items"]) {
		media_id := tsr_string(tsr_object(item_value)["media_id"])
		if image_url := media_urls[media_id]; image_url != "" {
			images = append(images, `<img data-n="article-image" src="`+html.EscapeString(image_url)+`" alt="">`)
		}
	}
	return strings.Join(images, "\n\n")
}

type article_inline_range struct {
	start int
	end   int
	open  string
	close string
}

func article_text_html(text string) string {
	return strings.ReplaceAll(html.EscapeString(text), "\n", `<br data-n="article-line-break">`)
}

// Draft.js offsets count UTF-16 units. Reopen overlapping ranges as needed to
// keep nested HTML valid even when a link and an inline style cross boundaries.
func article_inline_html(block map[string]any, entities map[int]article_entity) string {
	text, _ := block["text"].(string)
	if strings.TrimSpace(text) == "" {
		return ""
	}
	units := utf16_units(text)
	ranges := make([]article_inline_range, 0)
	add_range := func(range_record map[string]any, open string, close string) {
		offset := int(tsr_int64(range_record["offset"]))
		length := int(tsr_int64(range_record["length"]))
		if offset < 0 || length <= 0 || offset >= len(units) {
			return
		}
		if length > len(units)-offset {
			length = len(units) - offset
		}
		ranges = append(ranges, article_inline_range{start: offset, end: offset + length, open: open, close: close})
	}
	for _, range_value := range tsr_array(block["inline_style_ranges"]) {
		range_record := tsr_object(range_value)
		if markers, ok := inline_style_markers[tsr_string(range_record["style"])]; ok {
			add_range(range_record, markers[0], markers[1])
		}
	}
	for _, range_value := range tsr_array(block["entity_ranges"]) {
		range_record := tsr_object(range_value)
		entity, ok := entities[int(tsr_int64(range_record["key"]))]
		if !ok || entity.kind != "LINK" {
			continue
		}
		link_url := tsr_string(entity.data["url"])
		parsed_url, err := url.Parse(link_url)
		if err != nil || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") || parsed_url.Hostname() == "" {
			continue
		}
		add_range(range_record, `<a data-n="article-link" href="`+html.EscapeString(link_url)+`">`, "</a>")
	}
	var builder strings.Builder
	var active []article_inline_range
	for index := 0; index <= len(units); index++ {
		var next []article_inline_range
		for _, text_range := range ranges {
			if index >= text_range.start && index < text_range.end {
				next = append(next, text_range)
			}
		}
		common := 0
		for common < len(active) && common < len(next) && active[common] == next[common] {
			common++
		}
		for closing := len(active) - 1; closing >= common; closing-- {
			builder.WriteString(active[closing].close)
		}
		for _, opening := range next[common:] {
			builder.WriteString(opening.open)
		}
		if index < len(units) {
			builder.WriteString(article_text_html(units[index]))
		}
		active = next
	}
	return builder.String()
}

// utf16_units splits text into one entry per UTF-16 code unit so Draft.js
// offsets can address it directly. A non-BMP character fills two entries; the
// trailing one is empty and writes nothing.
func utf16_units(text string) []string {
	units := make([]string, 0, len(text))
	for _, character := range text {
		if character > 0xFFFF {
			units = append(units, string(character), "")
			continue
		}
		units = append(units, string(character))
	}
	return units
}
