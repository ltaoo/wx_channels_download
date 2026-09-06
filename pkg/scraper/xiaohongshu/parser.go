package xiaohongshu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

var initial_state_pattern = regexp.MustCompile(`(?is)window\.__INITIAL_STATE__\s*=\s*(.*?)</script\s*>`)
var xiaohongshu_note_id_pattern = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

type initial_state struct {
	Note note_state `json:"note"`
	User user_state `json:"user"`
}

type note_state struct {
	CurrentNoteID string                 `json:"currentNoteId"`
	FirstNoteID   string                 `json:"firstNoteId"`
	NoteDetailMap map[string]note_detail `json:"noteDetailMap"`
}

type note_detail struct {
	Note Note `json:"note"`
}

type user_state struct {
	ActiveTab   active_tab        `json:"activeTab"`
	Notes       [][]HomeNoteItem  `json:"notes"`
	NoteQueries []home_note_query `json:"noteQueries"`
}

type home_note_query struct {
	Cursor  string `json:"cursor"`
	HasMore bool   `json:"hasMore"`
}

type active_tab struct {
	Index int    `json:"index"`
	Query string `json:"query"`
}

// ParseFetchResult parses note data from a server-rendered Xiaohongshu page.
func ParseFetchResult(source_url string, page_url string, html_text string) (*FetchResult, error) {
	html_text = strings.TrimSpace(html_text)
	if html_text == "" {
		return nil, fmt.Errorf("小红书页面 HTML 为空")
	}
	state, err := extract_initial_state(html_text)
	if err != nil {
		return nil, err
	}
	note, err := select_note(state.Note)
	if err != nil {
		return nil, err
	}
	return ValidateFetchResult(&FetchResult{
		SourceURL: strings.TrimSpace(source_url),
		PageURL:   strings.TrimSpace(page_url),
		HTML:      html_text,
		Note:      note,
	})
}

// ParseHomeContents parses the active profile tab from Xiaohongshu's initial
// state. scope must be notes or collections.
func ParseHomeContents(document_html string, scope string) (*HomeContentList, error) {
	scope, expected_query, err := normalize_home_scope(scope)
	if err != nil {
		return nil, err
	}
	state, err := extract_initial_state(document_html)
	if err != nil {
		return nil, err
	}
	active_query := strings.TrimSpace(state.User.ActiveTab.Query)
	if active_query != "" && active_query != expected_query {
		return nil, fmt.Errorf("小红书主页 tab 类型不匹配: want %s, got %s", expected_query, active_query)
	}
	tab_index := state.User.ActiveTab.Index
	if tab_index < 0 || tab_index >= len(state.User.Notes) {
		return nil, fmt.Errorf("小红书主页 activeTab.index 越界: %d", tab_index)
	}
	items := append([]HomeNoteItem(nil), state.User.Notes[tab_index]...)
	merge_home_card_links(document_html, items)
	redacted := home_items_are_redacted(items)
	next_marker := ""
	if !redacted && tab_index < len(state.User.NoteQueries) && state.User.NoteQueries[tab_index].HasMore {
		next_marker = strings.TrimSpace(state.User.NoteQueries[tab_index].Cursor)
	}
	return &HomeContentList{Scope: scope, Items: items, NextMarker: next_marker, Redacted: redacted, HTML: document_html}, nil
}

func merge_home_card_links(document_html string, items []HomeNoteItem) {
	if len(items) == 0 || strings.TrimSpace(document_html) == "" {
		return
	}
	card_links := extract_home_card_links(document_html)
	for item_index := range items {
		item := &items[item_index]
		card_url := strings.TrimSpace(card_links[item.Index])
		if card_url == "" {
			continue
		}
		item.URL = card_url
		if note_id := home_note_id_from_url(card_url); note_id != "" {
			if strings.TrimSpace(item.ID) == "" {
				item.ID = note_id
			}
			if strings.TrimSpace(item.NoteCard.NoteID) == "" {
				item.NoteCard.NoteID = note_id
			}
		}
		if parsed_url, err := url.Parse(card_url); err == nil {
			xsec_token := strings.TrimSpace(parsed_url.Query().Get("xsec_token"))
			if xsec_token != "" {
				if strings.TrimSpace(item.XSecToken) == "" {
					item.XSecToken = xsec_token
				}
				if strings.TrimSpace(item.NoteCard.XSecToken) == "" {
					item.NoteCard.XSecToken = xsec_token
				}
			}
		}
	}
}

func extract_home_card_links(document_html string) map[int]string {
	document, err := xhtml.Parse(strings.NewReader(document_html))
	if err != nil {
		return nil
	}
	card_links := make(map[int]string)
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "section" && html_node_has_class(node, "note-item") {
			index_value := html_node_attribute(node, "data-index")
			item_index, index_err := strconv.Atoi(strings.TrimSpace(index_value))
			if index_err == nil {
				if href := find_home_cover_href(node); href != "" {
					card_links[item_index] = normalize_home_card_url(href)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	return card_links
}

func find_home_cover_href(node *xhtml.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == xhtml.ElementNode && node.Data == "a" &&
		html_node_has_class(node, "cover") && html_node_has_class(node, "mask") && html_node_has_class(node, "ld") {
		return strings.TrimSpace(html_node_attribute(node, "href"))
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if href := find_home_cover_href(child); href != "" {
			return href
		}
	}
	return ""
}

func html_node_attribute(node *xhtml.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val
		}
	}
	return ""
}

func html_node_has_class(node *xhtml.Node, class_name string) bool {
	for _, current_class := range strings.Fields(html_node_attribute(node, "class")) {
		if current_class == class_name {
			return true
		}
	}
	return false
}

func normalize_home_card_url(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return strings.TrimSpace(raw_url)
	}
	base_url, _ := url.Parse("https://www.xiaohongshu.com/")
	return base_url.ResolveReference(parsed_url).String()
}

func home_note_id_from_url(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ""
	}
	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	note_id := ""
	switch {
	case len(path_parts) == 4 && path_parts[0] == "user" && path_parts[1] == "profile":
		note_id = strings.TrimSpace(path_parts[3])
	case len(path_parts) == 2 && path_parts[0] == "explore":
		note_id = strings.TrimSpace(path_parts[1])
	case len(path_parts) == 3 && path_parts[0] == "discovery" && path_parts[1] == "item":
		note_id = strings.TrimSpace(path_parts[2])
	}
	if !xiaohongshu_note_id_pattern.MatchString(note_id) {
		return ""
	}
	return note_id
}

// ValidateFetchResult verifies that a decoded scraper result contains usable
// note media.
func ValidateFetchResult(result *FetchResult) (*FetchResult, error) {
	if result == nil || result.Note == nil {
		return nil, fmt.Errorf("小红书抓取结果为空")
	}
	if strings.TrimSpace(result.Note.NoteID) == "" {
		return nil, fmt.Errorf("小红书笔记 ID 为空")
	}
	if IsVideoNote(result.Note) && len(NoteStreams(result.Note)) == 0 {
		return nil, fmt.Errorf("小红书视频 %s 没有可用视频流", result.Note.NoteID)
	}
	if !IsVideoNote(result.Note) && len(NoteImages(result.Note)) == 0 {
		return nil, fmt.Errorf("小红书图文笔记 %s 没有可用图片", result.Note.NoteID)
	}
	return result, nil
}

// IsVideoNote reports whether note represents a video.
func IsVideoNote(note *Note) bool {
	return note != nil && strings.EqualFold(strings.TrimSpace(note.Type), "video")
}

// NoteStreams returns normalized, usable streams in codec preference order.
func NoteStreams(note *Note) []VideoStream {
	if note == nil {
		return nil
	}
	stream_groups := note.Video.Media.Stream
	stream_count := 0
	for _, group := range stream_groups {
		stream_count += len(group)
	}
	streams := make([]VideoStream, 0, stream_count)
	append_streams := func(codec string, source []VideoStream) {
		for _, stream := range source {
			stream.MasterURL = NormalizeMediaURL(stream.MasterURL)
			for backup_index, backup_url := range stream.BackupURLs {
				stream.BackupURLs[backup_index] = NormalizeMediaURL(backup_url)
			}
			if strings.TrimSpace(stream.VideoCodec) == "" {
				stream.VideoCodec = codec
			}
			if stream.MasterURL != "" {
				streams = append(streams, stream)
			}
		}
	}
	stream_keys := make([]string, 0, len(stream_groups))
	for stream_key := range stream_groups {
		stream_keys = append(stream_keys, stream_key)
	}
	sort.Slice(stream_keys, func(left_index int, right_index int) bool {
		left_key := stream_keys[left_index]
		right_key := stream_keys[right_index]
		left_priority := video_stream_key_priority(left_key)
		right_priority := video_stream_key_priority(right_key)
		if left_priority != right_priority {
			return left_priority < right_priority
		}
		return strings.ToLower(left_key) < strings.ToLower(right_key)
	})
	for _, stream_key := range stream_keys {
		append_streams(stream_key, stream_groups[stream_key])
	}
	return streams
}

func video_stream_key_priority(stream_key string) int {
	switch strings.ToLower(strings.TrimSpace(stream_key)) {
	case "h264", "ef4":
		return 0
	case "h265", "hevc", "ef5":
		return 1
	case "h266", "vvc", "ef6":
		return 2
	case "av1", "ef7":
		return 3
	default:
		return 4
	}
}

// NoteImages returns note images with normalized URLs. Images without a URL
// are omitted.
func NoteImages(note *Note) []Image {
	if note == nil {
		return nil
	}
	images := make([]Image, 0, len(note.ImageList))
	for _, image := range note.ImageList {
		image.URL = ImageURL(image)
		if image.URL != "" {
			images = append(images, image)
		}
	}
	return images
}

// ImageURL selects and normalizes the best URL in an image object.
func ImageURL(image Image) string {
	image_url := first_non_empty(image.URL, image.URLDefault, image.URLPreview)
	if image_url == "" {
		for _, image_info := range image.InfoList {
			if image_url = strings.TrimSpace(image_info.URL); image_url != "" {
				break
			}
		}
	}
	return NormalizeMediaURL(image_url)
}

// NormalizeMediaURL upgrades Xiaohongshu CDN URLs to HTTPS when possible.
func NormalizeMediaURL(raw_url string) string {
	raw_url = strings.TrimSpace(raw_url)
	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Hostname() == "" {
		return raw_url
	}
	if strings.EqualFold(parsed_url.Scheme, "http") {
		host := strings.ToLower(parsed_url.Hostname())
		if host == "xhscdn.com" || strings.HasSuffix(host, ".xhscdn.com") {
			parsed_url.Scheme = "https"
		}
	}
	return parsed_url.String()
}

func extract_initial_state(html_text string) (*initial_state, error) {
	matches := initial_state_pattern.FindStringSubmatch(html_text)
	if len(matches) != 2 {
		return nil, fmt.Errorf("小红书页面缺少 window.__INITIAL_STATE__")
	}
	state_text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(matches[1]), ";"))
	decoder := json.NewDecoder(bytes.NewBufferString(replace_undefined_identifiers(state_text)))
	decoder.UseNumber()
	var state initial_state
	if err := decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("解析小红书 INITIAL_STATE 失败: %w", err)
	}
	return &state, nil
}

func replace_undefined_identifiers(value string) string {
	const undefined_value = "undefined"
	var result strings.Builder
	result.Grow(len(value))
	inside_string := false
	escaped := false
	for index := 0; index < len(value); {
		current := value[index]
		if inside_string {
			result.WriteByte(current)
			index++
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == '"' {
				inside_string = false
			}
			continue
		}
		if current == '"' {
			inside_string = true
			result.WriteByte(current)
			index++
			continue
		}
		if strings.HasPrefix(value[index:], undefined_value) &&
			(index == 0 || !is_javascript_identifier_byte(value[index-1])) &&
			(index+len(undefined_value) == len(value) || !is_javascript_identifier_byte(value[index+len(undefined_value)])) {
			result.WriteString("null")
			index += len(undefined_value)
			continue
		}
		result.WriteByte(current)
		index++
	}
	return result.String()
}

func is_javascript_identifier_byte(value byte) bool {
	return value == '_' || value == '$' ||
		(value >= 'a' && value <= 'z') ||
		(value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9')
}

func select_note(state note_state) (*Note, error) {
	preferred_note_ids := []string{
		strings.TrimSpace(state.FirstNoteID),
		strings.TrimSpace(state.CurrentNoteID),
	}
	for _, note_id := range preferred_note_ids {
		if detail, exists := state.NoteDetailMap[note_id]; note_id != "" && exists && strings.TrimSpace(detail.Note.NoteID) != "" {
			note := detail.Note
			return &note, nil
		}
	}
	keys := make([]string, 0, len(state.NoteDetailMap))
	for key := range state.NoteDetailMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		detail := state.NoteDetailMap[key]
		if strings.TrimSpace(detail.Note.NoteID) != "" {
			note := detail.Note
			return &note, nil
		}
	}
	return nil, fmt.Errorf("小红书 INITIAL_STATE 中没有笔记详情")
}

func normalize_home_scope(scope string) (string, string, error) {
	scope = strings.TrimSpace(scope)
	switch scope {
	case "notes":
		return scope, "note", nil
	case "collections":
		return scope, "collect", nil
	default:
		return "", "", fmt.Errorf("小红书不支持主页 scope: %s", scope)
	}
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
