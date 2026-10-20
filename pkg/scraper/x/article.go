package x

import (
	"encoding/json"
	"strconv"
	"strings"
)

// X stores a long-form article as a Draft.js document: an ordered list of
// blocks plus side tables for inline styles and embedded entities. The entity's
// plain_text flattens that structure away, so the markdown view is rebuilt here
// from the same graph to keep headings, lists, links, images and code blocks.
func article_markdown(source string, entity_record string) string {
	state_record := extract_record(source, extract_js_ref_field(entity_record, "content_state"))
	if state_record == "" {
		return ""
	}
	media_urls := article_media_urls(source, entity_record)
	entities := article_entities(source, state_record)
	lines := make([]string, 0, 64)
	ordered_index := 0
	for _, block_ref := range js_ref_list(state_record, "blocks") {
		block := extract_record(source, block_ref)
		if extract_js_string_field(block, "type") == "ordered-list-item" {
			ordered_index++
		} else {
			ordered_index = 0
		}
		if line := article_block_markdown(source, block, entities, media_urls, ordered_index); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n\n"))
}

// article_cover_url is the article's own cover image, which X keeps in a
// dedicated media result rather than in the tweet's media entities.
func article_cover_url(source string, entity_record string) string {
	results_record := extract_record(source, extract_js_ref_field(entity_record, "cover_media_results"))
	media_record := extract_record(source, extract_js_ref_field(results_record, "result"))
	return article_media_image_url(source, media_record)
}

// article_entity is one entry of the content_state entity map.
type article_entity struct {
	kind string
	data string
}

// inline_style_markers maps Draft.js inline styles to their markdown delimiters.
var inline_style_markers = map[string][2]string{
	"Bold":          {"**", "**"},
	"Italic":        {"*", "*"},
	"Strikethrough": {"~~", "~~"},
}

// article_media_urls maps each article media id to its full-size image URL.
func article_media_urls(source string, entity_record string) map[string]string {
	urls := make(map[string]string)
	for _, media_ref := range js_ref_list(entity_record, "media_entities") {
		media_record := extract_record(source, media_ref)
		media_id := extract_js_string_field(media_record, "media_id")
		if media_id == "" {
			continue
		}
		if image_url := article_media_image_url(source, media_record); image_url != "" {
			urls[media_id] = image_url
		}
	}
	return urls
}

func article_media_image_url(source string, media_record string) string {
	info_record := extract_record(source, extract_js_ref_field(media_record, "media_info"))
	return extract_js_string_field(info_record, "original_img_url")
}

func article_entities(source string, state_record string) map[int]article_entity {
	entities := make(map[int]article_entity)
	for _, entry_ref := range js_ref_list(state_record, "entity_map") {
		entry_record := extract_record(source, entry_ref)
		if entry_record == "" {
			continue
		}
		value_record := extract_record(source, extract_js_ref_field(entry_record, "value"))
		if value_record == "" {
			continue
		}
		entities[int(extract_js_int64_field(entry_record, "key"))] = article_entity{
			kind: extract_js_string_field(value_record, "type"),
			data: extract_record(source, extract_js_ref_field(value_record, "data")),
		}
	}
	return entities
}

func article_block_markdown(source string, block string, entities map[int]article_entity, media_urls map[string]string, ordered_index int) string {
	block_type := extract_js_string_field(block, "type")
	if block_type == "atomic" {
		return article_atomic_markdown(source, block, entities, media_urls)
	}
	text := article_inline_markdown(source, block, entities)
	text = strings.TrimRight(text, " \t")
	if strings.TrimSpace(text) == "" {
		return ""
	}
	switch block_type {
	case "header-one":
		return "# " + text
	case "header-two":
		return "## " + text
	case "header-three":
		return "### " + text
	case "blockquote":
		return "> " + text
	case "unordered-list-item":
		return "- " + text
	case "ordered-list-item":
		if ordered_index < 1 {
			ordered_index = 1
		}
		return strconv.Itoa(ordered_index) + ". " + text
	}
	return text
}

func article_atomic_markdown(source string, block string, entities map[int]article_entity, media_urls map[string]string) string {
	entity, ok := article_block_entity(source, block, entities)
	if !ok {
		return ""
	}
	switch entity.kind {
	case "MEDIA":
		return article_media_markdown(source, entity.data, media_urls)
	case "MARKDOWN":
		return strings.TrimSpace(extract_js_string_field(entity.data, "markdown"))
	case "DIVIDER":
		return "---"
	}
	return ""
}

func article_block_entity(source string, block string, entities map[int]article_entity) (article_entity, bool) {
	for _, range_ref := range js_ref_list(block, "entity_ranges") {
		range_record := extract_record(source, range_ref)
		if entity, ok := entities[int(extract_js_int64_field(range_record, "key"))]; ok {
			return entity, true
		}
	}
	return article_entity{}, false
}

func article_media_markdown(source string, data string, media_urls map[string]string) string {
	images := make([]string, 0, 2)
	for _, item_ref := range js_ref_list(data, "media_items") {
		media_id := extract_js_string_field(extract_record(source, item_ref), "media_id")
		if image_url := media_urls[media_id]; image_url != "" {
			images = append(images, "![]("+image_url+")")
		}
	}
	return strings.Join(images, "\n\n")
}

// article_inline_markdown wraps a text block's bold runs and link entities.
// Draft.js offsets count UTF-16 code units, so non-BMP characters occupy two
// slots and a boundary index always matches a slot.
func article_inline_markdown(source string, block string, entities map[int]article_entity) string {
	text := extract_js_string_field(block, "text")
	if text == "" {
		return ""
	}
	units := utf16_units(text)
	openings := make(map[int][]string)
	closings := make(map[int][]string)
	for _, range_ref := range js_ref_list(block, "inline_style_ranges") {
		range_record := extract_record(source, range_ref)
		markers, ok := inline_style_markers[extract_js_string_field(range_record, "style")]
		if !ok {
			continue
		}
		article_range_markers(openings, closings, range_record, markers[0], markers[1], len(units))
	}
	for _, range_ref := range js_ref_list(block, "entity_ranges") {
		range_record := extract_record(source, range_ref)
		entity, ok := entities[int(extract_js_int64_field(range_record, "key"))]
		if !ok || entity.kind != "LINK" {
			continue
		}
		url := extract_js_string_field(entity.data, "url")
		if url == "" {
			continue
		}
		article_range_markers(openings, closings, range_record, "[", "]("+url+")", len(units))
	}
	var builder strings.Builder
	for index, unit := range units {
		builder.WriteString(strings.Join(closings[index], ""))
		builder.WriteString(strings.Join(openings[index], ""))
		builder.WriteString(unit)
	}
	builder.WriteString(strings.Join(closings[len(units)], ""))
	builder.WriteString(strings.Join(openings[len(units)], ""))
	return builder.String()
}

func article_range_markers(openings map[int][]string, closings map[int][]string, range_record string, open string, close string, unit_count int) {
	offset := int(extract_js_int64_field(range_record, "offset"))
	length := int(extract_js_int64_field(range_record, "length"))
	if offset < 0 || length <= 0 || offset >= unit_count {
		return
	}
	end := offset + length
	if end > unit_count {
		end = unit_count
	}
	openings[offset] = append(openings[offset], open)
	closings[end] = append(closings[end], close)
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

// js_ref_list returns the string refs of a $R[n]={__refs:[...]} side table.
// The $R[n] label is skipped so its own brackets are not mistaken for the array.
func js_ref_list(record string, field string) []string {
	value_index := js_field_value_index(record, field)
	if value_index < 0 {
		return nil
	}
	tail := record[value_index:]
	refs_index := strings.Index(tail, "__refs:")
	if refs_index < 0 {
		return nil
	}
	array_index := strings.Index(tail[refs_index:], "=[")
	if array_index < 0 {
		return nil
	}
	array_index += refs_index + 1
	refs := make([]string, 0, 8)
	for index := array_index + 1; index < len(tail); index++ {
		switch tail[index] {
		case ']':
			return refs
		case '"':
			for end := index + 1; end < len(tail); end++ {
				if tail[end] == '\\' {
					end++
					continue
				}
				if tail[end] != '"' {
					continue
				}
				var ref string
				if json.Unmarshal([]byte(tail[index:end+1]), &ref) == nil && ref != "" {
					refs = append(refs, ref)
				}
				index = end
				break
			}
		}
	}
	return refs
}
