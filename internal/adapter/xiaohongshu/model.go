package xiaohongshuadapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

const (
	platform_id_xiaohongshu = "xiaohongshu"
	xiaohongshu_user_agent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"
)

// PlatformID is the Xiaohongshu platform identifier.
const PlatformID = platform_id_xiaohongshu

var initial_state_pattern = regexp.MustCompile(`(?is)window\.__INITIAL_STATE__\s*=\s*(.*?)</script\s*>`)

type fetch_result struct {
	SourceURL string     `json:"source_url"`
	HTML      string     `json:"html"`
	Note      *note_data `json:"note"`
}

type initial_state struct {
	Note note_state `json:"note"`
}

type note_state struct {
	CurrentNoteID string                 `json:"currentNoteId"`
	FirstNoteID   string                 `json:"firstNoteId"`
	NoteDetailMap map[string]note_detail `json:"noteDetailMap"`
}

type note_detail struct {
	Note note_data `json:"note"`
}

type note_data struct {
	XSecToken      string           `json:"xsecToken"`
	Title          string           `json:"title"`
	Description    string           `json:"desc"`
	Type           string           `json:"type"`
	NoteID         string           `json:"noteId"`
	Time           int64            `json:"time"`
	LastUpdateTime int64            `json:"lastUpdateTime"`
	User           note_user        `json:"user"`
	InteractInfo   note_interaction `json:"interactInfo"`
	ImageList      []note_image     `json:"imageList"`
	TagList        []note_tag       `json:"tagList"`
	Video          note_video       `json:"video"`
}

type note_user struct {
	UserID    string `json:"userId"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar"`
	XSecToken string `json:"xsecToken"`
}

type note_interaction struct {
	LikedCount     flexible_int64 `json:"likedCount"`
	CommentCount   flexible_int64 `json:"commentCount"`
	CollectedCount flexible_int64 `json:"collectedCount"`
	ShareCount     flexible_int64 `json:"shareCount"`
}

type flexible_int64 int64

func (v *flexible_int64) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "" || value == "null" || value == `""` {
		*v = 0
		return nil
	}
	value = strings.Trim(value, `"`)
	parsed_value, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		parsed_float, float_err := strconv.ParseFloat(value, 64)
		if float_err != nil {
			return fmt.Errorf("invalid Xiaohongshu count %q", value)
		}
		parsed_value = int64(parsed_float)
	}
	*v = flexible_int64(parsed_value)
	return nil
}

type note_tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type note_image struct {
	FileID     string            `json:"fileId"`
	URL        string            `json:"url"`
	URLPreview string            `json:"urlPre"`
	URLDefault string            `json:"urlDefault"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	InfoList   []note_image_info `json:"infoList"`
}

type note_image_info struct {
	ImageScene string `json:"imageScene"`
	URL        string `json:"url"`
}

type note_video struct {
	Media video_media `json:"media"`
	Capa  video_capa  `json:"capa"`
}

type video_capa struct {
	Duration int64 `json:"duration"`
}

type video_media struct {
	Video  video_metadata `json:"video"`
	Stream video_streams  `json:"stream"`
}

type video_metadata struct {
	Duration int64  `json:"duration"`
	MD5      string `json:"md5"`
	BizID    string `json:"bizId"`
}

type video_streams struct {
	H264 []video_stream `json:"h264"`
	H265 []video_stream `json:"h265"`
	H266 []video_stream `json:"h266"`
	AV1  []video_stream `json:"av1"`
}

type video_stream struct {
	StreamType     int      `json:"streamType"`
	StreamDesc     string   `json:"streamDesc"`
	DefaultStream  int      `json:"defaultStream"`
	Format         string   `json:"format"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	Duration       int64    `json:"duration"`
	Size           int64    `json:"size"`
	AverageBitrate int      `json:"avgBitrate"`
	FPS            int      `json:"fps"`
	VideoCodec     string   `json:"videoCodec"`
	AudioCodec     string   `json:"audioCodec"`
	AudioChannels  int      `json:"audioChannels"`
	MasterURL      string   `json:"masterUrl"`
	BackupURLs     []string `json:"backupUrls"`
	QualityType    string   `json:"qualityType"`
}

func parse_fetch_result(source_url string, html_text string) (*fetch_result, error) {
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
	result := &fetch_result{
		SourceURL: strings.TrimSpace(source_url),
		HTML:      html_text,
		Note:      note,
	}
	return validate_fetch_result(result)
}

func extract_initial_state(html_text string) (*initial_state, error) {
	matches := initial_state_pattern.FindStringSubmatch(html_text)
	if len(matches) != 2 {
		return nil, fmt.Errorf("小红书页面缺少 window.__INITIAL_STATE__")
	}
	state_text := strings.TrimSpace(matches[1])
	state_text = strings.TrimSpace(strings.TrimSuffix(state_text, ";"))
	state_text = replace_undefined_identifiers(state_text)
	decoder := json.NewDecoder(bytes.NewBufferString(state_text))
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

func select_note(state note_state) (*note_data, error) {
	preferred_note_ids := []string{
		strings.TrimSpace(state.FirstNoteID),
		strings.TrimSpace(state.CurrentNoteID),
	}
	for _, note_id := range preferred_note_ids {
		if note_id == "" {
			continue
		}
		if detail, exists := state.NoteDetailMap[note_id]; exists && strings.TrimSpace(detail.Note.NoteID) != "" {
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
		if strings.TrimSpace(detail.Note.NoteID) == "" {
			continue
		}
		note := detail.Note
		return &note, nil
	}
	return nil, fmt.Errorf("小红书 INITIAL_STATE 中没有笔记详情")
}

func validate_fetch_result(result *fetch_result) (*fetch_result, error) {
	if result == nil || result.Note == nil {
		return nil, fmt.Errorf("小红书抓取结果为空")
	}
	if strings.TrimSpace(result.Note.NoteID) == "" {
		return nil, fmt.Errorf("小红书笔记 ID 为空")
	}
	if !strings.EqualFold(strings.TrimSpace(result.Note.Type), "video") {
		return nil, fmt.Errorf("小红书笔记 %s 不是视频笔记", result.Note.NoteID)
	}
	if len(note_streams(result.Note)) == 0 {
		return nil, fmt.Errorf("小红书视频 %s 没有可用视频流", result.Note.NoteID)
	}
	return result, nil
}

func fetch_result_from_data(data any) (*fetch_result, error) {
	switch value := data.(type) {
	case *fetch_result:
		return validate_fetch_result(value)
	case fetch_result:
		result := value
		return validate_fetch_result(&result)
	case string:
		if strings.Contains(value, "window.__INITIAL_STATE__") {
			return parse_fetch_result("", value)
		}
		return fetch_result_from_json([]byte(value))
	case []byte:
		if strings.Contains(string(value), "window.__INITIAL_STATE__") {
			return parse_fetch_result("", string(value))
		}
		return fetch_result_from_json(value)
	case json.RawMessage:
		return fetch_result_from_json(value)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("编码小红书抓取结果失败: %w", err)
	}
	return fetch_result_from_json(encoded)
}

func fetch_result_from_json(raw_json []byte) (*fetch_result, error) {
	if len(strings.TrimSpace(string(raw_json))) == 0 {
		return nil, fmt.Errorf("小红书抓取 JSON 为空")
	}
	var html_text string
	if err := json.Unmarshal(raw_json, &html_text); err == nil && strings.Contains(html_text, "window.__INITIAL_STATE__") {
		return parse_fetch_result("", html_text)
	}
	var result fetch_result
	if err := json.Unmarshal(raw_json, &result); err != nil {
		return nil, fmt.Errorf("解析小红书抓取 JSON 失败: %w", err)
	}
	if result.Note == nil && strings.TrimSpace(result.HTML) != "" {
		return parse_fetch_result(result.SourceURL, result.HTML)
	}
	return validate_fetch_result(&result)
}

func note_streams(note *note_data) []video_stream {
	if note == nil {
		return nil
	}
	streams := make([]video_stream, 0,
		len(note.Video.Media.Stream.H264)+len(note.Video.Media.Stream.H265)+
			len(note.Video.Media.Stream.H266)+len(note.Video.Media.Stream.AV1))
	append_streams := func(codec string, source []video_stream) {
		for _, stream := range source {
			stream.MasterURL = normalize_media_url(stream.MasterURL)
			for backup_index, backup_url := range stream.BackupURLs {
				stream.BackupURLs[backup_index] = normalize_media_url(backup_url)
			}
			if strings.TrimSpace(stream.VideoCodec) == "" {
				stream.VideoCodec = codec
			}
			if stream.MasterURL != "" {
				streams = append(streams, stream)
			}
		}
	}
	append_streams("h264", note.Video.Media.Stream.H264)
	append_streams("h265", note.Video.Media.Stream.H265)
	append_streams("h266", note.Video.Media.Stream.H266)
	append_streams("av1", note.Video.Media.Stream.AV1)
	return streams
}

func preferred_stream(note *note_data) (*video_stream, error) {
	streams := note_streams(note)
	if len(streams) == 0 {
		return nil, fmt.Errorf("小红书视频没有可用视频流")
	}
	for stream_index := range streams {
		if streams[stream_index].DefaultStream == 1 {
			return &streams[stream_index], nil
		}
	}
	return &streams[0], nil
}

func normalize_media_url(raw_url string) string {
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

func note_cover(note *note_data) (string, int, int) {
	if note == nil || len(note.ImageList) == 0 {
		return "", 0, 0
	}
	image := note.ImageList[0]
	cover_url := first_non_empty(image.URLDefault, image.URLPreview, image.URL)
	if cover_url == "" {
		for _, image_info := range image.InfoList {
			if cover_url = strings.TrimSpace(image_info.URL); cover_url != "" {
				break
			}
		}
	}
	return normalize_media_url(cover_url), image.Width, image.Height
}

func note_title(note *note_data) string {
	if note == nil {
		return "小红书视频"
	}
	return first_non_empty(note.Description, note.Title, "小红书视频_"+note.NoteID)
}

func canonical_note_url(note *note_data) string {
	if note == nil || strings.TrimSpace(note.NoteID) == "" {
		return "https://www.xiaohongshu.com/"
	}
	note_url := &url.URL{
		Scheme: "https",
		Host:   "www.xiaohongshu.com",
		Path:   "/discovery/item/" + strings.TrimSpace(note.NoteID),
	}
	query := note_url.Query()
	if xsec_token := strings.TrimSpace(note.XSecToken); xsec_token != "" {
		query.Set("xsec_token", xsec_token)
		query.Set("xsec_source", "pc_share")
	}
	note_url.RawQuery = query.Encode()
	return note_url.String()
}

func note_profile_url(user note_user) string {
	if strings.TrimSpace(user.UserID) == "" {
		return "https://www.xiaohongshu.com/"
	}
	profile_url := &url.URL{
		Scheme: "https",
		Host:   "www.xiaohongshu.com",
		Path:   "/user/profile/" + strings.TrimSpace(user.UserID),
	}
	query := profile_url.Query()
	if xsec_token := strings.TrimSpace(user.XSecToken); xsec_token != "" {
		query.Set("xsec_token", xsec_token)
		query.Set("xsec_source", "pc_note")
	}
	profile_url.RawQuery = query.Encode()
	return profile_url.String()
}

// BuildContentID builds the stable shared content ID for a Xiaohongshu note.
func BuildContentID(note_id string) string {
	return PlatformID + ":" + strings.TrimSpace(note_id)
}

// BuildAccountID builds the stable shared account ID for a Xiaohongshu user.
func BuildAccountID(user_id string) string {
	return PlatformID + ":" + strings.TrimSpace(user_id)
}

func to_content(result *fetch_result) (*model.Content, error) {
	result, err := validate_fetch_result(result)
	if err != nil {
		return nil, err
	}
	note := result.Note
	stream, err := preferred_stream(note)
	if err != nil {
		return nil, err
	}
	cover_url, cover_width, cover_height := note_cover(note)
	tag_names := make([]string, 0, len(note.TagList))
	for _, tag := range note.TagList {
		if tag_name := strings.TrimSpace(tag.Name); tag_name != "" {
			tag_names = append(tag_names, tag_name)
		}
	}
	tags_json, _ := json.Marshal(tag_names)
	metadata_json, _ := json.Marshal(map[string]any{
		"note_type":        note.Type,
		"last_update_time": note.LastUpdateTime,
		"video_biz_id":     note.Video.Media.Video.BizID,
		"video_md5":        note.Video.Media.Video.MD5,
		"stream_count":     len(note_streams(note)),
	})
	now := util.NowMillis()
	publish_time := note.Time
	if publish_time <= 0 {
		publish_time = note.LastUpdateTime
	}
	var publish_time_pointer *int64
	if publish_time > 0 {
		publish_time_pointer = &publish_time
	}
	source_url := first_non_empty(result.SourceURL, canonical_note_url(note))
	return &model.Content{
		Id:           BuildContentID(note.NoteID),
		PlatformId:   PlatformID,
		Type:         model.ContentTypeVideo,
		Subtype:      model.ContentSubtypeShortVideo,
		ExternalId:   strings.TrimSpace(note.NoteID),
		ExternalId2:  strings.TrimSpace(note.Video.Media.Video.BizID),
		Title:        note_title(note),
		Description:  strings.TrimSpace(note.Description),
		URL:          stream.MasterURL,
		SourceURL:    source_url,
		CoverURL:     cover_url,
		CoverWidth:   positive_int_string(cover_width),
		CoverHeight:  positive_int_string(cover_height),
		PublishTime:  publish_time_pointer,
		LikeCount:    int64(note.InteractInfo.LikedCount),
		CommentCount: int64(note.InteractInfo.CommentCount),
		ShareCount:   int64(note.InteractInfo.ShareCount),
		CollectCount: int64(note.InteractInfo.CollectedCount),
		Tags:         string(tags_json),
		Metadata:     string(metadata_json),
		Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, nil
}

func to_account(result *fetch_result) (*model.Account, error) {
	result, err := validate_fetch_result(result)
	if err != nil {
		return nil, err
	}
	user := result.Note.User
	if strings.TrimSpace(user.UserID) == "" {
		return nil, fmt.Errorf("小红书笔记 %s 缺少发布者 ID", result.Note.NoteID)
	}
	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(user.UserID),
		PlatformId: PlatformID,
		ExternalId: strings.TrimSpace(user.UserID),
		Nickname:   first_non_empty(user.Nickname, user.UserID),
		AvatarURL:  normalize_media_url(user.AvatarURL),
		ProfileURL: note_profile_url(user),
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, nil
}

func to_content_video(result *fetch_result) (*model.ContentVideo, error) {
	result, err := validate_fetch_result(result)
	if err != nil {
		return nil, err
	}
	streams := note_streams(result.Note)
	selected_stream, err := preferred_stream(result.Note)
	if err != nil {
		return nil, err
	}
	content_id := BuildContentID(result.Note.NoteID)
	now := util.NowMillis()
	variants := make([]model.ContentVideoVariant, 0, len(streams))
	variant_keys := make(map[string]int)
	for stream_index, stream := range streams {
		variant_key := video_variant_key(stream, stream_index)
		variant_keys[variant_key]++
		if variant_keys[variant_key] > 1 {
			variant_key += ":" + strconv.Itoa(variant_keys[variant_key])
		}
		metadata_json, _ := json.Marshal(map[string]any{
			"stream_type":    stream.StreamType,
			"stream_desc":    stream.StreamDesc,
			"duration_ms":    stream.Duration,
			"audio_codec":    stream.AudioCodec,
			"audio_channels": stream.AudioChannels,
			"backup_urls":    stream.BackupURLs,
		})
		variant := model.ContentVideoVariant{
			VideoId:      content_id,
			VariantKey:   variant_key,
			Spec:         strconv.Itoa(stream.StreamType),
			Quality:      first_non_empty(stream.QualityType, stream.StreamDesc),
			Size:         stream.Size,
			Codec:        strings.TrimSpace(stream.VideoCodec),
			Format:       first_non_empty(stream.Format, "mp4"),
			StreamType:   model.ContentVideoVariantStreamTypeProgressive,
			HasVideo:     1,
			HasAudio:     bool_to_int(strings.TrimSpace(stream.AudioCodec) != "" || stream.AudioChannels > 0),
			IsDefault:    bool_to_int(stream.MasterURL == selected_stream.MasterURL),
			URL:          stream.MasterURL,
			URLExpiresAt: video_url_expires_at(stream.MasterURL),
			Metadata:     string(metadata_json),
			Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		variant.Width = positive_int_pointer(stream.Width)
		variant.Height = positive_int_pointer(stream.Height)
		variant.FPS = positive_int_pointer(stream.FPS)
		variant.Bitrate = positive_int_pointer(stream.AverageBitrate)
		variants = append(variants, variant)
	}
	duration := milliseconds_to_seconds(selected_stream.Duration)
	if duration == 0 {
		duration = first_positive_int64(result.Note.Video.Media.Video.Duration, result.Note.Video.Capa.Duration)
	}
	return &model.ContentVideo{
		Id:              content_id,
		Duration:        duration,
		Width:           selected_stream.Width,
		Height:          selected_stream.Height,
		FPS:             selected_stream.FPS,
		Bitrate:         selected_stream.AverageBitrate,
		Size:            selected_stream.Size,
		Codec:           strings.TrimSpace(selected_stream.VideoCodec),
		Format:          first_non_empty(selected_stream.Format, "mp4"),
		AudioTrackCount: bool_to_int(strings.TrimSpace(selected_stream.AudioCodec) != "" || selected_stream.AudioChannels > 0),
		URL:             selected_stream.MasterURL,
		Variants:        variants,
	}, nil
}

func to_content_details(result *fetch_result) ([]adapter.ContentDetail, error) {
	content, err := to_content(result)
	if err != nil {
		return nil, err
	}
	video, err := to_content_video(result)
	if err != nil {
		return nil, err
	}
	account, err := to_account(result)
	if err != nil {
		return nil, err
	}
	return []adapter.ContentDetail{{
		Type:    model.ContentTypeVideo,
		Key:     content.Id,
		Content: content,
		Data:    video,
		Accounts: []adapter.ContentAccountReference{{
			Account: account,
			Role:    "owner",
		}},
	}}, nil
}

func video_variant_key(stream video_stream, stream_index int) string {
	codec := strings.ToLower(strings.TrimSpace(stream.VideoCodec))
	if codec == "" {
		codec = "video"
	}
	if stream.StreamType > 0 {
		return codec + ":" + strconv.Itoa(stream.StreamType)
	}
	return codec + ":" + strconv.Itoa(stream_index+1)
}

func video_url_expires_at(raw_url string) *int64 {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return nil
	}
	expires_text := strings.TrimSpace(parsed_url.Query().Get("t"))
	if expires_text == "" {
		return nil
	}
	expires_at, err := strconv.ParseInt(expires_text, 10, 64)
	if err != nil {
		expires_at, err = strconv.ParseInt(expires_text, 16, 64)
	}
	if err != nil || expires_at <= 0 {
		return nil
	}
	expires_at *= 1000
	return &expires_at
}

func positive_int_pointer(value int) *int {
	if value <= 0 {
		return nil
	}
	result := value
	return &result
}

func positive_int_string(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func milliseconds_to_seconds(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return (value + 500) / 1000
}

func first_positive_int64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func bool_to_int(value bool) int {
	if value {
		return 1
	}
	return 0
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
