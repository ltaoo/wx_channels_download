package x

// X now streams its SSR payload as a flat reference table instead of the nested
// `__id:`/`__ref` record graph it used to inline. Each `$R[<n>]=<js literal>`
// binding is a slot; records point at each other by slot index, so a post has to
// be decoded back into plain Go values before it can be read. The literals are
// real JavaScript (unquoted keys, `!0`/`!1`, `void 0`, inline `$R[<n>]` refs),
// which is what this file implements.

import (
	"encoding/json"
	"strconv"
	"strings"
)

// tsr_slot_depth_limit stops a self-referential slot table from recursing forever.
const tsr_slot_depth_limit = 64

// tsr_literals are the JavaScript keywords the payload uses for scalar values.
var tsr_literals = []struct {
	text  string
	value any
}{
	{"void 0", nil},
	{"undefined", nil},
	{"null", nil},
	{"true", true},
	{"false", false},
}

type tsr_reader struct {
	source string
	// slots maps a slot index to the offset of its literal in source.
	slots map[int]int
	// cache memoizes decoded slots; the table shares subtrees heavily and a slot
	// reaches itself through the tweet/quoted-tweet graph.
	cache map[int]any
	ends  map[int]int
	depth int
}

func new_tsr_reader(source string) *tsr_reader {
	reader := &tsr_reader{
		source: source,
		slots:  make(map[int]int),
		cache:  make(map[int]any),
		ends:   make(map[int]int),
	}
	search_index := 0
	for search_index < len(source) {
		slot_index := strings.Index(source[search_index:], "$R[")
		if slot_index < 0 {
			return reader
		}
		slot_index += search_index
		digit_start := slot_index + len("$R[")
		digit_end := digit_start
		for digit_end < len(source) && source[digit_end] >= '0' && source[digit_end] <= '9' {
			digit_end++
		}
		if digit_end == digit_start || digit_end+1 >= len(source) || source[digit_end] != ']' || source[digit_end+1] != '=' || (digit_end+2 < len(source) && source[digit_end+2] == '=') {
			search_index = digit_start
			continue
		}
		index, err := strconv.Atoi(source[digit_start:digit_end])
		if err != nil {
			search_index = digit_end
			continue
		}
		// The stream re-sends a slot when it updates it; the last binding wins.
		reader.slots[index] = digit_end + 2
		search_index = digit_end + 2
	}
	return reader
}

// tsr_find_tweet returns the focal post's record. The scroll-free payload can
// carry the same tweet more than once (the focused-tweet query and the
// conversation timeline both hold a copy) and the stream can send a partial
// update, so the richest copy wins.
func tsr_find_tweet(source string, status_id string) map[string]any {
	reader := new_tsr_reader(source)
	var best map[string]any
	best_score := -1
	best_index := 0
	for index := range reader.slots {
		record, ok := reader.slot_value(index).(map[string]any)
		if !ok || tsr_string(record["rest_id"]) != status_id {
			continue
		}
		score := tsr_tweet_score(record)
		if score > best_score || (score == best_score && index < best_index) {
			best, best_score, best_index = record, score, index
		}
	}
	return best
}

func tsr_tweet_score(record map[string]any) int {
	score := 0
	if len(tsr_array(record["media_entities2"])) > 0 {
		score += 2
	}
	if tsr_string(tsr_object(record["details"])["full_text"]) != "" {
		score++
	}
	return score
}

// tsr_extract_post builds a result from a streamed payload. It reports false
// when the requested post is not in the table, which is how a deleted or
// protected post shows up.
func tsr_extract_post(source string, metadata map[string]string, status_id string, fallback_url string) (*FetchResult, bool) {
	record := tsr_find_tweet(source, status_id)
	if record == nil {
		return nil, false
	}
	details := tsr_object(record["details"])
	counts := tsr_object(record["counts"])
	views := tsr_object(record["views"])
	result := &FetchResult{
		SourceURL:    first_non_empty(metadata["og:url"], metadata["canonical"], fallback_url),
		ExternalID:   status_id,
		BodyText:     first_non_empty(tsr_string(details["full_text"]), metadata["og:description"]),
		CoverURL:     metadata["og:image"],
		PublishTime:  tsr_int64(details["created_at_ms"]),
		ViewCount:    tsr_int64(views["count"]),
		LikeCount:    tsr_int64(counts["favorite_count"]),
		CommentCount: tsr_int64(counts["reply_count"]),
		ShareCount:   tsr_int64(counts["retweet_count"]),
	}
	tsr_populate_author(record, result)
	tsr_populate_media(record, result)
	return result, true
}

func tsr_populate_author(record map[string]any, result *FetchResult) {
	user_results := tsr_object(tsr_object(record["core"])["user_results"])
	user := tsr_object(user_results["result"])
	user_core := tsr_object(user["core"])
	result.AuthorID = tsr_string(user["rest_id"])
	result.AuthorName = tsr_string(user_core["name"])
	result.AuthorUsername = first_non_empty(result.AuthorUsername, tsr_string(user_core["screen_name"]))
	result.AuthorAvatar = tsr_string(tsr_object(user["avatar"])["image_url"])
}

func tsr_populate_media(record map[string]any, result *FetchResult) {
	for _, item := range tsr_array(record["media_entities2"]) {
		media := tsr_object(item)
		original_info := tsr_object(media["original_info"])
		switch tsr_string(media["type"]) {
		case "photo":
			result.Images = append(result.Images, Image{
				ID:      tsr_string(media["id_str"]),
				URL:     tsr_string(media["media_url_https"]),
				Width:   int(tsr_int64(original_info["width"])),
				Height:  int(tsr_int64(original_info["height"])),
				AltText: tsr_string(media["ext_alt_text"]),
			})
		case "video", "animated_gif":
			video := Video{
				ID:       tsr_string(media["id_str"]),
				Type:     tsr_string(media["type"]),
				CoverURL: tsr_string(media["media_url_https"]),
				Width:    int(tsr_int64(original_info["width"])),
				Height:   int(tsr_int64(original_info["height"])),
			}
			video_info := tsr_object(media["video_info"])
			video.DurationMillis = tsr_int64(video_info["duration_millis"])
			for _, variant_item := range tsr_array(video_info["variants"]) {
				variant := tsr_object(variant_item)
				variant_url := tsr_string(variant["url"])
				if variant_url == "" {
					continue
				}
				video.Variants = append(video.Variants, VideoVariant{
					Bitrate:     int(tsr_int64(variant["bitrate"])),
					ContentType: tsr_string(variant["content_type"]),
					URL:         variant_url,
				})
			}
			result.Videos = append(result.Videos, video)
		}
	}
}

// slot_value decodes one slot. The value is cached, and the slot is stamped as
// in-progress first so a reference cycle decodes to nil instead of recursing.
func (r *tsr_reader) slot_value(index int) any {
	value, _, ok := r.slot(index)
	if !ok {
		return nil
	}
	return value
}

func (r *tsr_reader) slot(index int) (any, int, bool) {
	if value, cached := r.cache[index]; cached {
		return value, r.ends[index], true
	}
	position, defined := r.slots[index]
	if !defined || r.depth >= tsr_slot_depth_limit {
		return nil, position, false
	}
	r.cache[index], r.ends[index] = nil, position
	r.depth++
	value, end := r.value_at(position)
	r.depth--
	r.cache[index], r.ends[index] = value, end
	return value, end, true
}

// value_at decodes the literal at position and returns where it ended. Values
// the reader has no use for (function expressions, identifiers) decode to nil,
// but the cursor still advances so the caller cannot loop.
func (r *tsr_reader) value_at(position int) (any, int) {
	position = r.skip_space(position)
	if position >= len(r.source) {
		return nil, position
	}
	switch r.source[position] {
	case '{':
		return r.object_at(position)
	case '[':
		return r.array_at(position)
	case '"':
		return r.string_at(position)
	case '$':
		return r.reference_at(position)
	case '!':
		if strings.HasPrefix(r.source[position:], "!0") {
			return false, position + 2
		}
		if strings.HasPrefix(r.source[position:], "!1") {
			return true, position + 2
		}
	}
	for _, literal := range tsr_literals {
		if strings.HasPrefix(r.source[position:], literal.text) {
			return literal.value, position + len(literal.text)
		}
	}
	if character := r.source[position]; character == '-' || (character >= '0' && character <= '9') {
		return r.number_at(position)
	}
	_, end := r.identifier_at(position)
	if end == position {
		end++
	}
	return nil, end
}

func (r *tsr_reader) object_at(position int) (map[string]any, int) {
	object := make(map[string]any)
	position = r.skip_space(position) + 1
	for position < len(r.source) {
		position = r.skip_space(position)
		if position >= len(r.source) {
			break
		}
		switch r.source[position] {
		case '}':
			return object, position + 1
		case ',':
			position++
			continue
		}
		var key string
		if r.source[position] == '"' {
			key, position = r.string_at(position)
		} else {
			key, position = r.identifier_at(position)
		}
		if key == "" {
			position++
			continue
		}
		position = r.skip_space(position)
		if position < len(r.source) && r.source[position] == ':' {
			position++
		}
		var value any
		value, position = r.value_at(position)
		object[key] = value
	}
	return object, position
}

func (r *tsr_reader) array_at(position int) ([]any, int) {
	items := make([]any, 0, 4)
	position = r.skip_space(position) + 1
	for position < len(r.source) {
		position = r.skip_space(position)
		if position >= len(r.source) {
			break
		}
		switch r.source[position] {
		case ']':
			return items, position + 1
		case ',':
			position++
			continue
		}
		var value any
		value, position = r.value_at(position)
		items = append(items, value)
	}
	return items, position
}

// reference_at resolves `$R[<n>]`. X usually binds the slot inline
// (`media_entities2:$R[45]=[...]`), which both defines the slot and yields its
// value, so the slot is read through the cache when the binding is the slot's
// current definition.
func (r *tsr_reader) reference_at(position int) (any, int) {
	digit_start := position + len("$R[")
	digit_end := digit_start
	for digit_end < len(r.source) && r.source[digit_end] >= '0' && r.source[digit_end] <= '9' {
		digit_end++
	}
	if digit_end == digit_start || digit_end >= len(r.source) || r.source[digit_end] != ']' {
		return nil, position + 1
	}
	index, err := strconv.Atoi(r.source[digit_start:digit_end])
	if err != nil {
		return nil, digit_end + 1
	}
	after_bracket := digit_end + 1
	assignment := r.skip_space(after_bracket)
	if assignment >= len(r.source) || r.source[assignment] != '=' || (assignment+1 < len(r.source) && r.source[assignment+1] == '=') {
		return r.slot_value(index), after_bracket
	}
	if value, end, ok := r.slot(index); ok && r.slots[index] == assignment+1 {
		return value, end
	}
	return r.value_at(assignment + 1)
}

func (r *tsr_reader) string_at(position int) (string, int) {
	end := position + 1
	for end < len(r.source) {
		switch r.source[end] {
		case '\\':
			end += 2
			continue
		case '"':
			var value string
			if json.Unmarshal([]byte(r.source[position:end+1]), &value) == nil {
				return value, end + 1
			}
			return r.source[position+1 : end], end + 1
		}
		end++
	}
	return "", len(r.source)
}

func (r *tsr_reader) number_at(position int) (any, int) {
	end := position
	for end < len(r.source) {
		character := r.source[end]
		if (character >= '0' && character <= '9') || character == '-' || character == '+' || character == '.' || character == 'e' || character == 'E' {
			end++
			continue
		}
		break
	}
	text := r.source[position:end]
	if strings.ContainsAny(text, ".eE") {
		if value, err := strconv.ParseFloat(text, 64); err == nil {
			return value, end
		}
		return nil, end
	}
	if value, err := strconv.ParseInt(text, 10, 64); err == nil {
		return value, end
	}
	if value, err := strconv.ParseFloat(text, 64); err == nil {
		return value, end
	}
	return nil, end
}

func (r *tsr_reader) identifier_at(position int) (string, int) {
	end := position
	for end < len(r.source) {
		character := r.source[end]
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '$' {
			end++
			continue
		}
		break
	}
	return r.source[position:end], end
}

func (r *tsr_reader) skip_space(position int) int {
	for position < len(r.source) {
		switch r.source[position] {
		case ' ', '\t', '\n', '\r':
			position++
		default:
			return position
		}
	}
	return position
}

func tsr_object(value any) map[string]any {
	object, _ := value.(map[string]any)
	if object == nil {
		return map[string]any{}
	}
	return object
}

func tsr_array(value any) []any {
	items, _ := value.([]any)
	return items
}

func tsr_string(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func tsr_int64(value any) int64 {
	switch number := value.(type) {
	case int64:
		return number
	case float64:
		return int64(number)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(number), 10, 64)
		return parsed
	}
	return 0
}
