package youtubeadapter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/youtube"
	"wx_channel/pkg/util"
)

// ToContent converts a complete YouTube fetch result into the common content
// model. Media representations and captions are returned by ToContentVideo.
func ToContent(info *youtube.VideoInfo) (*model.Content, error) {
	if info == nil {
		return nil, fmt.Errorf("youtube video info is nil")
	}
	if strings.TrimSpace(info.ID) == "" {
		return nil, fmt.Errorf("youtube video id is empty")
	}

	content_type := model.ContentTypeVideo
	subtype := model.ContentSubtypeLongVideo
	switch info.MediaType {
	case "short":
		subtype = model.ContentSubtypeShortVideo
	case "livestream":
		content_type = model.ContentTypeLive
		subtype = model.ContentSubtypeLivestream
	}
	metadata, _ := json.Marshal(map[string]any{
		"channel_id":                    info.ChannelID,
		"channel_url":                   info.ChannelURL,
		"channel_avatar_url":            info.ChannelAvatarURL,
		"uploader":                      info.Uploader,
		"uploader_url":                  info.UploaderURL,
		"uploader_avatar_url":           info.UploaderAvatarURL,
		"live_status":                   info.LiveStatus,
		"age_limit":                     info.AgeLimit,
		"format_count":                  len(info.Formats),
		"caption_track_count":           len(info.CaptionTracks),
		"caption_audio_track_count":     len(info.CaptionAudioTracks),
		"caption_translation_languages": info.CaptionTranslationLanguages,
		"warnings":                      info.Warnings,
	})
	now := util.NowMillis()
	content := &model.Content{
		Id:          BuildContentID(info.ID),
		PlatformId:  PlatformID,
		Type:        content_type,
		Subtype:     subtype,
		ExternalId:  info.ID,
		Title:       first_non_empty(info.Title, "youtube_"+info.ID),
		Description: info.Description,
		URL:         best_content_url(info),
		SourceURL:   first_non_empty(info.WebpageURL, canonical_watch_url(info.ID)),
		CoverURL:    info.Thumbnail,
		PublishTime: youtube_publish_time(info),
		ViewCount:   info.ViewCount,
		Tags:        json_string_array(info.Tags),
		Category:    strings.Join(info.Categories, ","),
		Metadata:    string(metadata),
		TextTracks:  ToContentTextTracks(info),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	for _, thumbnail := range info.Thumbnails {
		if thumbnail.URL != info.Thumbnail {
			continue
		}
		content.CoverWidth = positive_int_string(thumbnail.Width)
		content.CoverHeight = positive_int_string(thumbnail.Height)
		break
	}
	return content, nil
}

func youtube_publish_time(info *youtube.VideoInfo) *int64 {
	if info == nil {
		return nil
	}
	for _, value := range []string{info.PublishDate, info.UploadDate} {
		if publish_time := parse_youtube_publish_time(value); publish_time != nil {
			return publish_time
		}
	}

	page_html := strings.TrimSpace(info.PageHTML)
	if page_html == "" {
		return nil
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(page_html))
	if err != nil {
		return nil
	}
	for _, itemprop := range []string{"datePublished", "uploadDate"} {
		value, exists := document.Find(`meta[itemprop="` + itemprop + `"]`).First().Attr("content")
		if !exists {
			continue
		}
		if publish_time := parse_youtube_publish_time(value); publish_time != nil {
			return publish_time
		}
	}
	return nil
}

func parse_youtube_publish_time(value string) *int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02"} {
		parsed_time, err := time.Parse(layout, value)
		if err != nil {
			continue
		}
		publish_time := parsed_time.UnixMilli()
		return &publish_time
	}
	return nil
}

// ToAccount converts YouTube channel/uploader data into the common account
// model.
func ToAccount(info *youtube.VideoInfo) (*model.Account, error) {
	if info == nil {
		return nil, fmt.Errorf("youtube video info is nil")
	}
	external_id := strings.TrimSpace(first_non_empty(info.ChannelID, info.UploaderID, info.ID))
	if external_id == "" {
		return nil, fmt.Errorf("youtube account external id is empty")
	}
	nickname := strings.TrimSpace(first_non_empty(info.Channel, info.Uploader, "YouTube"))
	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(external_id),
		PlatformId: PlatformID,
		ExternalId: external_id,
		Nickname:   nickname,
		AvatarURL:  first_non_empty(info.ChannelAvatarURL, info.UploaderAvatarURL),
		ProfileURL: first_non_empty(info.ChannelURL, info.UploaderURL),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToContentVideo converts all playable representations and caption tracks into
// the normalized video detail model.
func ToContentVideo(info *youtube.VideoInfo) (*model.ContentVideo, error) {
	if info == nil {
		return nil, fmt.Errorf("youtube video info is nil")
	}
	if strings.TrimSpace(info.ID) == "" {
		return nil, fmt.Errorf("youtube video id is empty")
	}
	selected := best_video_detail_format(info)
	video := &model.ContentVideo{
		Id:              BuildContentID(info.ID),
		Duration:        info.Duration,
		PlayTimes:       info.ViewCount,
		AudioTrackCount: len(info.CaptionAudioTracks),
	}
	if selected != nil {
		video.Width = selected.Width
		video.Height = selected.Height
		video.FPS = selected.FPS
		video.Bitrate = first_non_zero_int(selected.AverageBitrate, selected.Bitrate)
		video.Size = selected.ContentLength
		video.Codec = first_non_empty(selected.VideoCodec, selected.AudioCodec)
		video.Format = selected.Ext
		video.URL = selected.URL
	}
	video.Variants = youtube_video_variants(info, selected)
	return video, nil
}

// ToContentTextTracks maps YouTube caption tracks to content-level logical
// text tracks and their concrete timedtext representations.
func ToContentTextTracks(info *youtube.VideoInfo) []model.ContentTextTrack {
	if info == nil || strings.TrimSpace(info.ID) == "" {
		return nil
	}
	content_id := BuildContentID(info.ID)
	tracks := make([]model.ContentTextTrack, 0, len(info.CaptionTracks))
	for _, caption := range info.CaptionTracks {
		base_url := strings.TrimSpace(caption.BaseURL)
		if base_url == "" {
			continue
		}
		language_code := first_non_empty(caption.LanguageCode, "und")
		track_type := model.ContentTextTrackTypeSubtitle
		is_auto_generated := strings.EqualFold(caption.Kind, "asr") || strings.HasPrefix(strings.ToLower(caption.VssID), "a.")
		if is_auto_generated {
			track_type = model.ContentTextTrackTypeCaption
		}
		track_key := youtube_caption_track_key(caption)
		track_metadata, _ := json.Marshal(map[string]any{
			"vss_id":          caption.VssID,
			"youtube_kind":    caption.Kind,
			"is_translatable": caption.IsTranslatable,
		})
		sources := make([]model.ContentTextTrackSource, 0, 4)
		for _, format := range []string{"vtt", "json3", "srv3", "ttml"} {
			source_url := youtube_caption_format_url(base_url, format)
			sources = append(sources, model.ContentTextTrackSource{
				SourceKey:    format,
				Format:       format,
				URL:          source_url,
				URLExpiresAt: youtube_url_expires_at(source_url),
				Encoding:     "utf-8",
				Metadata:     string(track_metadata),
			})
		}
		tracks = append(tracks, model.ContentTextTrack{
			ContentId:         content_id,
			TrackKey:          track_key,
			Type:              track_type,
			LanguageCode:      language_code,
			LanguageName:      caption.Name,
			Label:             caption.Name,
			IsDefault:         caption.IsDefault,
			IsAutoGenerated:   is_auto_generated,
			IsHearingImpaired: is_hearing_impaired_caption(caption),
			Sources:           sources,
			Metadata:          string(track_metadata),
		})
	}
	return tracks
}

// ToContentDetails returns the complete normalized detail graph produced from
// one YouTube player response.
func ToContentDetails(info *youtube.VideoInfo) ([]adapter.ContentDetail, error) {
	content, err := ToContent(info)
	if err != nil {
		return nil, err
	}
	video, err := ToContentVideo(info)
	if err != nil {
		return nil, err
	}
	return []adapter.ContentDetail{{
		Type: content.Type,
		Key:  content.Id,
		Data: video,
	}}, nil
}

func youtube_caption_track_key(caption youtube.CaptionTrack) string {
	if vss_id := strings.TrimSpace(caption.VssID); vss_id != "" {
		return "vss:" + vss_id
	}
	return strings.ToLower(first_non_empty(caption.LanguageCode, "und")) + ":" + first_non_empty(caption.Kind, "subtitle")
}

func youtube_caption_format_url(base_url string, format string) string {
	parsed, err := url.Parse(strings.TrimSpace(base_url))
	if err != nil {
		return base_url
	}
	query := parsed.Query()
	query.Set("fmt", format)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func youtube_url_expires_at(raw_url string) *int64 {
	parsed, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return nil
	}
	expires_at, err := strconv.ParseInt(parsed.Query().Get("expire"), 10, 64)
	if err != nil || expires_at <= 0 {
		return nil
	}
	expires_at *= 1000
	return &expires_at
}

func is_hearing_impaired_caption(caption youtube.CaptionTrack) bool {
	label := strings.ToLower(caption.Name + " " + caption.VssID)
	return strings.Contains(label, "[cc]") ||
		strings.Contains(label, "(cc)") ||
		strings.Contains(label, "hearing impaired") ||
		strings.Contains(label, "sdh")
}

func positive_int_string(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}
