package douyinadapter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/douyin"
	"wx_channel/pkg/util"
)

const (
	douyin_mobile_video_page_key = "video_(id)/page"
	douyin_aweme_type_album      = 2
	douyin_content_type_video    = "video"
	douyin_content_type_album    = "album"
)

type douyin_mobile_router_data struct {
	LoaderData map[string]json.RawMessage `json:"loaderData"`
}

type douyin_mobile_page struct {
	Host         string                            `json:"host"`
	LastPath     string                            `json:"lastPath"`
	VideoInfoRes douyin_mobile_video_info_response `json:"videoInfoRes"`
}

type douyin_mobile_video_info_response struct {
	StatusCode int                  `json:"status_code"`
	ItemList   []douyin_mobile_item `json:"item_list"`
}

type douyin_mobile_item struct {
	AwemeID    string                   `json:"aweme_id"`
	Desc       string                   `json:"desc"`
	CreateTime int64                    `json:"create_time"`
	AwemeType  int                      `json:"aweme_type"`
	Author     douyin_mobile_author     `json:"author"`
	Video      douyin_mobile_video      `json:"video"`
	Images     []douyin_mobile_image    `json:"images"`
	Music      douyin_mobile_music      `json:"music"`
	Statistics douyin_mobile_statistics `json:"statistics"`
	TextExtra  []douyin_mobile_text     `json:"text_extra"`
}

type douyin_mobile_author struct {
	UID                     string                 `json:"uid"`
	ShortID                 string                 `json:"short_id"`
	UniqueID                string                 `json:"unique_id"`
	SecUID                  string                 `json:"sec_uid"`
	Nickname                string                 `json:"nickname"`
	Signature               string                 `json:"signature"`
	FollowerCount           int64                  `json:"follower_count"`
	MPlatformFollowersCount int64                  `json:"mplatform_followers_count"`
	AvatarThumb             douyin_mobile_url_data `json:"avatar_thumb"`
}

type douyin_mobile_video struct {
	Duration int64                  `json:"duration"`
	Width    int                    `json:"width"`
	Height   int                    `json:"height"`
	PlayAddr douyin_mobile_url_data `json:"play_addr"`
	Cover    douyin_mobile_url_data `json:"cover"`
}

type douyin_mobile_url_data struct {
	URI     string   `json:"uri"`
	URLList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

type douyin_mobile_image struct {
	URI             string   `json:"uri"`
	URLList         []string `json:"url_list"`
	DownloadURLList []string `json:"download_url_list"`
	Width           int      `json:"width"`
	Height          int      `json:"height"`
}

type douyin_mobile_music struct {
	MID      string `json:"mid"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Duration int64  `json:"duration"`
}

type douyin_mobile_statistics struct {
	PlayCount    int64 `json:"play_count"`
	DiggCount    int64 `json:"digg_count"`
	CommentCount int64 `json:"comment_count"`
	ShareCount   int64 `json:"share_count"`
	CollectCount int64 `json:"collect_count"`
}

type douyin_mobile_text struct {
	HashtagName string `json:"hashtag_name"`
}

type douyin_model_image struct {
	uri    string
	url    string
	width  int
	height int
	ext    string
}

type douyin_model_data struct {
	content_type   string
	video_id       string
	title          string
	description    string
	video_url      string
	video_uri      string
	cover_url      string
	source_url     string
	cover_width    int
	cover_height   int
	duration       int64
	width          int
	height         int
	view_count     int64
	like_count     int64
	comment_count  int64
	share_count    int64
	collect_count  int64
	publish_time   int64
	tags           []string
	aweme_type     int
	author_id      string
	author_alias   string
	author_name    string
	author_bio     string
	author_avatar  string
	author_profile string
	follower_count int64
	images         []douyin_model_image
	bgm_url        string
	bgm_id         string
	bgm_title      string
	bgm_author     string
	bgm_duration   int64
	source         string
}

func douyin_model_data_from_fetch(data any) (*douyin_model_data, error) {
	switch value := data.(type) {
	case json.RawMessage:
		return douyin_model_data_from_json(value)
	case *json.RawMessage:
		if value == nil {
			return nil, fmt.Errorf("douyin raw JSON is nil")
		}
		return douyin_model_data_from_json(*value)
	case []byte:
		return douyin_model_data_from_json(json.RawMessage(value))
	case *douyin.VideoInfo:
		if value == nil {
			return nil, fmt.Errorf("douyin video info is nil")
		}
		return douyin_model_data_from_video_info(value)
	case douyin.VideoInfo:
		return douyin_model_data_from_video_info(&value)
	default:
		return nil, fmt.Errorf("unsupported douyin fetch data type %T", data)
	}
}

func douyin_model_data_from_json(raw_json json.RawMessage) (*douyin_model_data, error) {
	if len(strings.TrimSpace(string(raw_json))) == 0 {
		return nil, fmt.Errorf("douyin mobile JSON is empty")
	}

	var router_data douyin_mobile_router_data
	if err := json.Unmarshal(raw_json, &router_data); err != nil {
		return nil, fmt.Errorf("parse douyin mobile JSON: %w", err)
	}
	if len(router_data.LoaderData) == 0 {
		return nil, fmt.Errorf("douyin mobile JSON is missing loaderData")
	}

	page, page_key, err := douyin_mobile_video_page(router_data.LoaderData)
	if err != nil {
		return nil, err
	}
	if page.VideoInfoRes.StatusCode != 0 {
		return nil, fmt.Errorf("douyin mobile video response status_code=%d", page.VideoInfoRes.StatusCode)
	}
	if len(page.VideoInfoRes.ItemList) == 0 {
		return nil, fmt.Errorf("douyin mobile page %q has no item_list", page_key)
	}

	item := page.VideoInfoRes.ItemList[0]
	video_id := strings.TrimSpace(item.AwemeID)
	if video_id == "" {
		video_id = strings.TrimSpace(page.LastPath)
	}
	if video_id == "" {
		return nil, fmt.Errorf("douyin mobile video id is empty")
	}

	title := strings.TrimSpace(item.Desc)
	if title == "" {
		title = "douyin_" + video_id
	}
	author_id := douyin_author_external_id(item.Author, video_id)
	follower_count := item.Author.FollowerCount
	if follower_count == 0 {
		follower_count = item.Author.MPlatformFollowersCount
	}
	content_type := douyin_content_type_video
	video_url := ""
	video_uri := ""
	images := make([]douyin_model_image, 0, len(item.Images))
	bgm_url := ""
	if len(item.Images) > 0 || item.AwemeType == douyin_aweme_type_album {
		content_type = douyin_content_type_album
		var image_err error
		images, image_err = douyin_model_images(item.Images)
		if image_err != nil {
			return nil, fmt.Errorf("douyin mobile album %s: %w", video_id, image_err)
		}
		if len(images) == 0 {
			return nil, fmt.Errorf("douyin mobile album %s has no images", video_id)
		}
		bgm_url = douyin_album_bgm_url(item.Video.PlayAddr.URI)
	} else {
		video_url = douyin_no_watermark_url(first_douyin_url(item.Video.PlayAddr.URLList))
		if video_url == "" {
			return nil, fmt.Errorf("douyin mobile video %s has no play URL", video_id)
		}
		video_uri = strings.TrimSpace(item.Video.PlayAddr.URI)
	}

	cover_url := first_douyin_url(item.Video.Cover.URLList)
	cover_width := item.Video.Cover.Width
	cover_height := item.Video.Cover.Height
	if content_type == douyin_content_type_album && len(images) > 0 {
		if cover_url == "" {
			cover_url = images[0].url
		}
		if cover_width <= 0 {
			cover_width = images[0].width
		}
		if cover_height <= 0 {
			cover_height = images[0].height
		}
	}

	return &douyin_model_data{
		content_type:   content_type,
		video_id:       video_id,
		title:          title,
		description:    strings.TrimSpace(item.Desc),
		video_url:      video_url,
		video_uri:      video_uri,
		cover_url:      cover_url,
		source_url:     douyin_content_source_url(content_type, video_id),
		cover_width:    cover_width,
		cover_height:   cover_height,
		duration:       item.Video.Duration,
		width:          item.Video.Width,
		height:         item.Video.Height,
		view_count:     item.Statistics.PlayCount,
		like_count:     item.Statistics.DiggCount,
		comment_count:  item.Statistics.CommentCount,
		share_count:    item.Statistics.ShareCount,
		collect_count:  item.Statistics.CollectCount,
		publish_time:   douyin_publish_time_millis(item.CreateTime),
		tags:           douyin_hashtags(item.TextExtra),
		aweme_type:     item.AwemeType,
		author_id:      author_id,
		author_alias:   strings.TrimSpace(item.Author.UniqueID),
		author_name:    strings.TrimSpace(item.Author.Nickname),
		author_bio:     strings.TrimSpace(item.Author.Signature),
		author_avatar:  first_douyin_url(item.Author.AvatarThumb.URLList),
		author_profile: douyin_author_profile_url(item.Author),
		follower_count: follower_count,
		images:         images,
		bgm_url:        bgm_url,
		bgm_id:         strings.TrimSpace(item.Music.MID),
		bgm_title:      strings.TrimSpace(item.Music.Title),
		bgm_author:     strings.TrimSpace(item.Music.Author),
		bgm_duration:   item.Music.Duration,
		source:         "mobile",
	}, nil
}

func douyin_mobile_video_page(loader_data map[string]json.RawMessage) (*douyin_mobile_page, string, error) {
	if raw_page, ok := loader_data[douyin_mobile_video_page_key]; ok {
		page, err := decode_douyin_mobile_page(raw_page)
		if err != nil {
			return nil, douyin_mobile_video_page_key, err
		}
		return page, douyin_mobile_video_page_key, nil
	}

	page_keys := make([]string, 0, len(loader_data))
	for page_key := range loader_data {
		page_keys = append(page_keys, page_key)
	}
	sort.Strings(page_keys)
	for _, page_key := range page_keys {
		page, err := decode_douyin_mobile_page(loader_data[page_key])
		if err == nil && len(page.VideoInfoRes.ItemList) > 0 {
			return page, page_key, nil
		}
	}
	return nil, "", fmt.Errorf("douyin mobile JSON has no video page")
}

func decode_douyin_mobile_page(raw_page json.RawMessage) (*douyin_mobile_page, error) {
	if string(raw_page) == "null" || len(raw_page) == 0 {
		return nil, fmt.Errorf("douyin mobile video page is empty")
	}
	var page douyin_mobile_page
	if err := json.Unmarshal(raw_page, &page); err != nil {
		return nil, fmt.Errorf("parse douyin mobile video page: %w", err)
	}
	return &page, nil
}

func douyin_model_data_from_video_info(video_info *douyin.VideoInfo) (*douyin_model_data, error) {
	video_id := strings.TrimSpace(video_info.VideoID)
	if video_id == "" {
		return nil, fmt.Errorf("douyin video id is empty")
	}
	title := strings.TrimSpace(video_info.Title)
	if title == "" {
		title = "douyin_" + video_id
	}
	return &douyin_model_data{
		content_type: douyin_content_type_video,
		video_id:     video_id,
		title:        title,
		description:  strings.TrimSpace(video_info.Title),
		video_url:    strings.TrimSpace(video_info.URL),
		cover_url:    strings.TrimSpace(video_info.CoverURL),
		source_url:   douyin_content_source_url(douyin_content_type_video, video_id),
		author_id:    video_id,
		author_name:  "抖音用户",
		source:       strings.TrimSpace(video_info.Source),
	}, nil
}

func douyin_content_from_data(data *douyin_model_data) *model.Content {
	now := util.NowMillis()
	content := &model.Content{
		Id:           BuildContentID(data.video_id),
		PlatformId:   PlatformID,
		ExternalId:   data.video_id,
		Type:         data.content_type,
		Title:        data.title,
		Description:  data.description,
		URL:          data.video_url,
		SourceURL:    data.source_url,
		CoverURL:     data.cover_url,
		CoverWidth:   douyin_dimension_string(data.cover_width),
		CoverHeight:  douyin_dimension_string(data.cover_height),
		ViewCount:    data.view_count,
		LikeCount:    data.like_count,
		CommentCount: data.comment_count,
		ShareCount:   data.share_count,
		CollectCount: data.collect_count,
		Tags:         douyin_tags_json(data.tags),
		Metadata:     douyin_content_metadata(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if data.publish_time > 0 {
		publish_time := data.publish_time
		content.PublishTime = &publish_time
	}
	return content
}

func douyin_account_from_data(data *douyin_model_data) *model.Account {
	now := util.NowMillis()
	author_id := strings.TrimSpace(data.author_id)
	if author_id == "" {
		author_id = data.video_id
	}
	nickname := strings.TrimSpace(data.author_name)
	if nickname == "" {
		nickname = "抖音用户"
	}
	return &model.Account{
		Id:            BuildAccountID(author_id),
		PlatformId:    PlatformID,
		ExternalId:    author_id,
		Alias:         data.author_alias,
		Nickname:      nickname,
		Signature:     data.author_bio,
		AvatarURL:     data.author_avatar,
		ProfileURL:    data.author_profile,
		FollowerCount: data.follower_count,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

func douyin_video_from_data(data *douyin_model_data) *model.ContentVideo {
	return &model.ContentVideo{
		Id:        BuildContentID(data.video_id),
		Duration:  data.duration,
		Width:     data.width,
		Height:    data.height,
		Format:    "mp4",
		URL:       data.video_url,
		PlayTimes: data.view_count,
	}
}

func douyin_album_from_data(data *douyin_model_data) *model.ContentAlbum {
	content_id := BuildContentID(data.video_id)
	images := make([]model.ContentImage, 0, len(data.images))
	for image_index, image := range data.images {
		images = append(images, model.ContentImage{
			AlbumId:   content_id,
			SortOrder: image_index,
			URL:       image.url,
			Width:     image.width,
			Height:    image.height,
			Ext:       image.ext,
			ImageType: model.ContentImageTypeStill,
		})
	}
	album := &model.ContentAlbum{
		Id:          content_id,
		ImageCount:  len(images),
		Description: data.description,
		Images:      images,
	}
	if len(images) > 0 {
		album.CoverWidth = images[0].Width
		album.CoverHeight = images[0].Height
		album.Format = images[0].Ext
	}
	return album
}

func douyin_content_detail_from_data(data *douyin_model_data) any {
	if data.content_type == douyin_content_type_album {
		return douyin_album_from_data(data)
	}
	return douyin_video_from_data(data)
}

func douyin_author_external_id(author douyin_mobile_author, fallback_id string) string {
	candidates := []string{author.SecUID, author.UniqueID, author.UID, author.ShortID, fallback_id}
	for _, candidate := range candidates {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			return candidate
		}
	}
	return fallback_id
}

func douyin_author_profile_url(author douyin_mobile_author) string {
	if sec_uid := strings.TrimSpace(author.SecUID); sec_uid != "" {
		return "https://www.douyin.com/user/" + url.PathEscape(sec_uid)
	}
	if unique_id := strings.TrimSpace(author.UniqueID); unique_id != "" {
		return "https://www.douyin.com/user/" + url.PathEscape(unique_id)
	}
	return ""
}

func douyin_content_source_url(content_type string, aweme_id string) string {
	path_name := "video"
	if content_type == douyin_content_type_album {
		path_name = "note"
	}
	return "https://www.douyin.com/" + path_name + "/" + url.PathEscape(strings.TrimSpace(aweme_id))
}

func douyin_publish_time_millis(create_time int64) int64 {
	if create_time <= 0 {
		return 0
	}
	if create_time < 100000000000 {
		return create_time * 1000
	}
	return create_time
}

func douyin_hashtags(text_extra []douyin_mobile_text) []string {
	tags := make([]string, 0, len(text_extra))
	seen := make(map[string]struct{}, len(text_extra))
	for _, text := range text_extra {
		tag := strings.TrimSpace(text.HashtagName)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func first_douyin_url(url_list []string) string {
	for _, candidate := range url_list {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			return candidate
		}
	}
	return ""
}

func douyin_model_images(images []douyin_mobile_image) ([]douyin_model_image, error) {
	model_images := make([]douyin_model_image, 0, len(images))
	for image_index, image := range images {
		image_url := first_douyin_url(image.URLList)
		if image_url == "" {
			return nil, fmt.Errorf("image %d has no non-watermarked URL", image_index+1)
		}
		model_images = append(model_images, douyin_model_image{
			uri:    strings.TrimSpace(image.URI),
			url:    image_url,
			width:  image.Width,
			height: image.Height,
			ext:    douyin_image_extension(image_url),
		})
	}
	return model_images, nil
}

func douyin_album_bgm_url(play_uri string) string {
	play_uri = strings.TrimSpace(play_uri)
	parsed_url, err := url.Parse(play_uri)
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return ""
	}
	if strings.ToLower(path.Ext(parsed_url.Path)) != ".mp3" {
		return ""
	}
	return play_uri
}

func douyin_image_extension(image_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(image_url))
	if err != nil {
		return ""
	}
	extension := strings.TrimPrefix(strings.ToLower(path.Ext(parsed_url.Path)), ".")
	switch extension {
	case "jpg", "jpeg", "png", "webp", "gif", "avif":
		return extension
	default:
		return ""
	}
}

func douyin_no_watermark_url(video_url string) string {
	return strings.Replace(strings.TrimSpace(video_url), "/playwm/", "/play/", 1)
}

func douyin_dimension_string(dimension int) string {
	if dimension <= 0 {
		return ""
	}
	return strconv.Itoa(dimension)
}

func douyin_tags_json(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	tags_json, err := json.Marshal(tags)
	if err != nil {
		return ""
	}
	return string(tags_json)
}

func douyin_content_metadata(data *douyin_model_data) string {
	metadata := map[string]any{
		"aweme_type":   data.aweme_type,
		"content_type": data.content_type,
		"source":       data.source,
	}
	if data.video_uri != "" {
		metadata["video_uri"] = data.video_uri
	}
	if len(data.images) > 0 {
		metadata["image_count"] = len(data.images)
	}
	if data.bgm_id != "" {
		metadata["music_id"] = data.bgm_id
	}
	if data.bgm_title != "" {
		metadata["music_title"] = data.bgm_title
	}
	if data.bgm_author != "" {
		metadata["music_author"] = data.bgm_author
	}
	metadata_json, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	return string(metadata_json)
}
