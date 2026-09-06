package xiaohongshuadapter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/xiaohongshu"
	"wx_channel/pkg/util"
)

const xiaohongshu_user_agent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

// PlatformID is the Xiaohongshu platform identifier.
const PlatformID = xiaohongshu.PlatformID

type fetch_result = xiaohongshu.FetchResult
type note_data = xiaohongshu.Note
type note_user = xiaohongshu.NoteUser
type video_stream = xiaohongshu.VideoStream

func validate_fetch_result(result *fetch_result) (*fetch_result, error) {
	return xiaohongshu.ValidateFetchResult(result)
}

func is_video_note(note *note_data) bool {
	return xiaohongshu.IsVideoNote(note)
}

func fetch_result_from_data(data any) (*fetch_result, error) {
	switch value := data.(type) {
	case *fetch_result:
		return validate_fetch_result(value)
	case fetch_result:
		result := value
		return validate_fetch_result(&result)
	case string:
		return fetch_result_from_json([]byte(value))
	case []byte:
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
	var result fetch_result
	if err := json.Unmarshal(raw_json, &result); err != nil {
		return nil, fmt.Errorf("解析小红书抓取 JSON 失败: %w", err)
	}
	return validate_fetch_result(&result)
}

func note_streams(note *note_data) []video_stream {
	return xiaohongshu.NoteStreams(note)
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
	return xiaohongshu.NormalizeMediaURL(raw_url)
}

func note_cover(note *note_data) (string, int, int) {
	if note == nil || len(note.ImageList) == 0 {
		return "", 0, 0
	}
	image := note.ImageList[0]
	return xiaohongshu.ImageURL(image), image.Width, image.Height
}

func note_images(note *note_data) []model.ContentImage {
	if note == nil {
		return nil
	}
	content_id := BuildContentID(note.NoteID)
	images := make([]model.ContentImage, 0, len(note.ImageList))
	for image_index, image := range xiaohongshu.NoteImages(note) {
		image_url := xiaohongshu.ImageURL(image)
		if image_url == "" {
			continue
		}
		images = append(images, model.ContentImage{
			AlbumId:   content_id,
			ImageKey:  model.BuildContentAlbumImageKey(image.FileID, image_url, image_index),
			SortOrder: image_index,
			URL:       image_url,
			Width:     image.Width,
			Height:    image.Height,
			ImageType: model.ContentImageTypeStill,
		})
	}
	return images
}

func note_title(note *note_data) string {
	if note == nil {
		return "小红书笔记"
	}
	return first_non_empty(note.Description, note.Title, "小红书笔记_"+note.NoteID)
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
	content_type := model.ContentTypeAlbum
	content_subtype := model.ContentSubtypePhotoAlbum
	content_url := canonical_note_url(note)
	if is_video_note(note) {
		stream, stream_err := preferred_stream(note)
		if stream_err != nil {
			return nil, stream_err
		}
		content_type = model.ContentTypeVideo
		content_subtype = model.ContentSubtypeShortVideo
		content_url = stream.MasterURL
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
		"image_count":      len(note_images(note)),
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
		Type:         content_type,
		Subtype:      content_subtype,
		ExternalId:   strings.TrimSpace(note.NoteID),
		ExternalId2:  strings.TrimSpace(note.Video.Media.Video.BizID),
		Title:        note_title(note),
		Description:  strings.TrimSpace(note.Description),
		URL:          content_url,
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

func to_content_album(result *fetch_result) (*model.ContentAlbum, error) {
	result, err := validate_fetch_result(result)
	if err != nil {
		return nil, err
	}
	images := note_images(result.Note)
	album := &model.ContentAlbum{
		Id:          BuildContentID(result.Note.NoteID),
		ImageCount:  len(images),
		Description: strings.TrimSpace(result.Note.Description),
		Images:      images,
	}
	if len(images) > 0 {
		album.CoverWidth = images[0].Width
		album.CoverHeight = images[0].Height
	}
	return album, nil
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
	account, err := to_account(result)
	if err != nil {
		return nil, err
	}
	var detail any
	if is_video_note(result.Note) {
		detail, err = to_content_video(result)
	} else {
		detail, err = to_content_album(result)
	}
	if err != nil {
		return nil, err
	}
	return []adapter.ContentDetail{{
		Type:    content.Type,
		Key:     content.Id,
		Content: content,
		Data:    detail,
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
