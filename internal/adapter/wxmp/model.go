package wxmpadapter

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxmp"
	"wx_channel/pkg/util"
)

const platform_id_wx_mp = "wxmp"

// PlatformID is the platform identifier for WeChat official accounts.
const PlatformID = platform_id_wx_mp

var wechat_headers string

func init() {
	h := map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN",
		"Referer":    "https://mp.weixin.qq.com/",
	}
	b, _ := json.Marshal(h)
	wechat_headers = string(b)
}

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(external_id string) string {
	return PlatformID + ":" + external_id
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(external_id string) string {
	return PlatformID + ":" + external_id
}

// ArticleExternalID builds a unique external identifier for an official account article.
func ArticleExternalID(data *wxmp.ArticleCgiDataNew) string {
	if data == nil || strings.TrimSpace(data.Bizuin) == "" || data.Mid <= 0 || data.Idx <= 0 {
		return ""
	}
	return fmt.Sprintf("%s_%d_%d", strings.TrimSpace(data.Bizuin), data.Mid, data.Idx)
}

// article_cover_url picks the best cover image URL from the article data.
func article_cover_url(data *wxmp.ArticleCgiDataNew) string {
	return strings.TrimSpace(data.CdnURL)
}

func build_source_url(data *wxmp.ArticleCgiDataNew) string {
	if data == nil {
		return ""
	}
	if strings.TrimSpace(data.UserInfo.ShortLink) != "" {
		return strings.TrimSpace(data.UserInfo.ShortLink)
	}
	return strings.TrimSpace(data.SourceURL)
}

// article_avatar_url picks the best avatar URL for the publisher account.
func article_avatar_url(data *wxmp.ArticleCgiDataNew) string {
	return first_non_empty_str(
		strings.TrimSpace(data.RoundHeadImg),
		strings.TrimSpace(data.OriHeadImgURL),
		strings.TrimSpace(data.HdHeadImg),
	)
}

// article_publish_time returns the publish timestamp from the article data.
func article_publish_time(data *wxmp.ArticleCgiDataNew) *int64 {
	if data.OriCreateTime > 0 {
		t := int64(data.OriCreateTime) * 1000
		return &t
	}
	if data.CreateTimestamp > 0 {
		t := int64(data.CreateTimestamp)
		return &t
	}
	return nil
}

// ToContent converts an ArticleCgiData into a slim model.Content and its type-specific extension.
func ToContent(data *wxmp.ArticleCgiDataNew) (*model.Content, any, error) {
	if data == nil {
		return nil, nil, errors.New("article data is nil")
	}
	external_id := ArticleExternalID(data)
	if external_id == "" {
		return nil, nil, errors.New("missing bizuin/mid/idx in article data")
	}

	now := util.NowMillis()
	content_type := "article"
	if is_album(data) {
		content_type = "album"
	}
	c := &model.Content{
		Id:          BuildContentID(external_id),
		PlatformId:  PlatformID,
		Type:        content_type,
		ExternalId:  external_id,
		ExternalId2: strconv.Itoa(data.Mid),
		Title:       strings.TrimSpace(data.Title),
		Description: strings.TrimSpace(data.Desc),
		URL:         strings.TrimSpace(data.Link),
		SourceURL:   build_source_url(data),
		CoverURL:    article_cover_url(data),
		PublishTime: article_publish_time(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if is_album(data) {
		album, images := build_content_album(data, c.Id)
		return c, &ContentAlbumExt{Album: album, Images: images}, nil
	}

	return c, build_content_article(data, c.Id), nil
}

// ToAccount converts an ArticleCgiData publisher into a model.Account.
func ToAccount(data *wxmp.ArticleCgiDataNew) (*model.Account, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	external_id := data.UserName
	if external_id == "" {
		return nil, errors.New("missing bizuin in article data")
	}

	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(external_id),
		PlatformId: PlatformID,
		ExternalId: data.UserName,
		Alias:      strings.TrimSpace(data.Alias),
		Nickname:   strings.TrimSpace(data.NickName),
		Signature:  strings.TrimSpace(data.Signature),
		AvatarURL:  article_avatar_url(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ArticleToHistory converts an ArticleCgiData into a model.BrowseHistory.
func ArticleToHistory(data *wxmp.ArticleCgiDataNew) (*model.BrowseHistory, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	external_id := ArticleExternalID(data)
	if external_id == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	now := util.NowMillis()
	content_type := "article"
	if is_album(data) {
		content_type = "album"
	}

	return &model.BrowseHistory{
		PlatformId:   PlatformID,
		VisitedTimes: 1,
		Type:         content_type,
		ExternalId:   external_id,
		Title:        strings.TrimSpace(data.Title),
		URL:          strings.TrimSpace(data.Link),
		SourceURL:    build_source_url(data),
		CoverURL:     article_cover_url(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func is_album(data *wxmp.ArticleCgiDataNew) bool {
	if data == nil {
		return false
	}
	return data.PageType == 2
}

// BuildBrowseHistory converts intercepted article CGI data into the standard
// browse history result.
func (a *OfficialAccountAdapter) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	var data wxmp.ArticleCgiDataNew
	if err := json.Unmarshal(content_json, &data); err != nil {
		return nil, fmt.Errorf("解析文章数据失败: %w", err)
	}

	browse_history, err := ArticleToHistory(&data)
	if err != nil {
		return nil, err
	}
	account, err := ToAccount(&data)
	if err != nil {
		return nil, err
	}

	return &adapter.BrowseHistoryResult{
		BrowseHistory: browse_history,
		Account:       account,
	}, nil
}

// ArticleToContentArticle converts an ArticleCgiData into a model.ContentArticle with the HTML body.
func ArticleToContentArticle(data *wxmp.ArticleCgiDataNew) (*model.ContentArticle, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	external_id := ArticleExternalID(data)
	if external_id == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	return build_content_article(data, BuildContentID(external_id)), nil
}

func build_content_article(data *wxmp.ArticleCgiDataNew, id string) *model.ContentArticle {
	article := &model.ContentArticle{
		Id:        id,
		Type:      model.ContentArticleTypeHTML,
		WordCount: article_word_count(data.ContentNoencode),
		HTML:      data.ContentNoencode,
	}
	if data.CopyrightInfo.CopyrightStat > 0 {
		article.IsOriginal = 1
	}
	return article
}

// ContentAlbumExt wraps ContentAlbum with its child images for passing through
// the ToContent any return value.
type ContentAlbumExt struct {
	Album  *model.ContentAlbum
	Images []*model.ContentImage
}

func build_content_album(data *wxmp.ArticleCgiDataNew, content_id string) (*model.ContentAlbum, []*model.ContentImage) {
	album_images := make([]*model.ContentImage, 0, len(data.PicturePageInfoList))
	for i, picture := range data.PicturePageInfoList {
		image_url := normalize_image_url(picture.CdnUrl)
		live_photo := build_content_image_live_photo(picture.LivePhoto)
		if image_url == "" && live_photo == nil {
			continue
		}
		image_type := model.ContentImageTypeStill
		if live_photo != nil {
			image_type = model.ContentImageTypeLivePhoto
		}
		album_images = append(album_images, &model.ContentImage{
			AlbumId:   content_id,
			ImageKey:  model.BuildContentAlbumImageKey("", "", i),
			SortOrder: i,
			URL:       image_url,
			Width:     picture.Width,
			Height:    picture.Height,
			ImageType: image_type,
			LivePhoto: live_photo,
		})
	}

	album := &model.ContentAlbum{
		Id:          content_id,
		ImageCount:  len(album_images),
		Format:      strings.TrimSpace(data.ImgFormat),
		Description: strings.TrimSpace(data.Desc),
		Images:      content_image_values(album_images, content_id),
	}
	if len(album_images) > 0 {
		album.CoverWidth = album_images[0].Width
		album.CoverHeight = album_images[0].Height
	}
	return album, album_images
}

func build_content_image_live_photo(input wxmp.PictureLivePhoto) *model.ContentImageLivePhoto {
	formats := make([]model.ContentImageLivePhotoFormat, 0, len(input.FormatInfo))
	selected := -1
	for _, source := range input.FormatInfo {
		video_url := normalize_image_url(source.URL)
		if video_url == "" {
			continue
		}
		format := model.ContentImageLivePhotoFormat{
			FormatId:   source.FormatID,
			URL:        video_url,
			Size:       int64(source.FileSize),
			DurationMs: source.Duration,
			Width:      source.Width,
			Height:     source.Height,
		}
		formats = append(formats, format)
		candidate := len(formats) - 1
		if selected < 0 || better_live_photo_format(formats[candidate], formats[selected]) {
			selected = candidate
		}
	}

	vid := strings.TrimSpace(input.Vid)
	if selected < 0 {
		if vid == "" && input.Type == 0 {
			return nil
		}
		return &model.ContentImageLivePhoto{Vid: vid, Type: input.Type}
	}

	best := formats[selected]
	return &model.ContentImageLivePhoto{
		Vid:        vid,
		Type:       input.Type,
		URL:        best.URL,
		FormatId:   best.FormatId,
		Width:      best.Width,
		Height:     best.Height,
		Size:       best.Size,
		DurationMs: best.DurationMs,
		Formats:    formats,
	}
}

func better_live_photo_format(candidate, current model.ContentImageLivePhotoFormat) bool {
	if candidate.FormatId == 0 && current.FormatId != 0 {
		return true
	}
	if current.FormatId == 0 && candidate.FormatId != 0 {
		return false
	}
	candidate_pixels := int64(candidate.Width) * int64(candidate.Height)
	current_pixels := int64(current.Width) * int64(current.Height)
	if candidate_pixels != current_pixels {
		return candidate_pixels > current_pixels
	}
	return candidate.Size > current.Size
}

func article_word_count(content_html string) int {
	text := content_html
	if doc, err := goquery.NewDocumentFromReader(strings.NewReader(content_html)); err == nil {
		text = doc.Text()
	}

	count := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}

// ArticleToContentAccount creates a model.ContentAccount linking content to its publisher account.
func ArticleToContentAccount(data *wxmp.ArticleCgiDataNew) (*model.ContentAccount, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	external_id := ArticleExternalID(data)
	if external_id == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	return &model.ContentAccount{
		ContentId: BuildContentID(external_id),
		AccountId: BuildAccountID(strings.TrimSpace(data.Bizuin)),
		Role:      "publisher",
		CreatedAt: util.NowMillis(),
	}, nil
}

// first_non_empty_str returns the first non-empty string from the given values.
func first_non_empty_str(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (a *OfficialAccountAdapter) BuildDownloadTask(content_json json.RawMessage, config_raw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(config_raw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	var data wxmp.ArticleCgiDataNew
	if err := json.Unmarshal(content_json, &data); err != nil {
		return nil, fmt.Errorf("解析文章数据失败: %w", err)
	}

	content, ext, err := ToContent(&data)
	if err != nil {
		return nil, err
	}
	account, err := ToAccount(&data)
	if err != nil {
		return nil, err
	}

	external_id := ArticleExternalID(&data)
	if external_id == "" {
		return nil, fmt.Errorf("无法生成文章唯一标识")
	}

	title, _ := config["filename"].(string)
	if title == "" {
		title = strings.TrimSpace(data.Title)
	}

	config_json, _ := json.Marshal(build_config_json(config))
	biz_type := content_biz_type(&data)
	metadata_json, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"external_id": external_id,
		"author":      strings.TrimSpace(data.NickName),
		"created_at":  article_publish_time_val(&data),
		"biz_type":    biz_type,
	})

	extra_json := build_extra_json(external_id, title, strings.TrimSpace(data.NickName), article_publish_time_val(&data))
	content_id := content.Id

	var image_resources []*adapter.ResourceInfo
	if album_ext, ok := ext.(*ContentAlbumExt); ok {
		image_resources = parse_album_images(album_ext.Images, content_id, external_id, extra_json)
		album_ext.Album.Images = content_image_values(album_ext.Images, content_id)
		ext = album_ext.Album
	} else if biz_type == 2 {
		return nil, fmt.Errorf("图集内容缺少图集详情")
	} else {
		image_resources = parse_content_images(data.ContentNoencode, content_id, external_id, extra_json)
	}

	cover_url := strings.TrimSpace(data.CdnURL)
	source_url := strings.TrimSpace(data.SourceURL)
	if source_url == "" {
		source_url = strings.TrimSpace(data.Link)
	}

	html_name := title
	html_resource := model.DownloadResource{
		ContentId:  &content_id,
		Name:       html_name,
		Kind:       "html",
		UniqueID:   external_id + "_html",
		MergeOrder: 0,
		Extra:      extra_json,
	}
	html_endpoint := model.DownloadEndpoint{
		Protocol: "inline",
		URL:      data.ContentNoencode,
		Enabled:  1,
	}

	resources := make([]*adapter.ResourceInfo, 0, len(image_resources)+1)
	resources = append(resources, &adapter.ResourceInfo{
		Resource:  html_resource,
		Endpoints: []model.DownloadEndpoint{html_endpoint},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindText,
			Role:     model.ContentAssetRoleArticleBody,
			AssetKey: "body:html",
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	})
	for _, r := range image_resources {
		resources = append(resources, r)
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     build_download_task_unique_id(external_id, config_string(config, "suffix")),
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    source_url,
			CoverURL:     cover_url,
			ConfigJSON:   string(config_json),
			MetadataJSON: string(metadata_json),
		},
		Resources:     resources,
		ContentDetail: ext,
		Account:       account,
		Content:       content,
	}, nil
}

func build_download_task_unique_id(external_id, suffix string) string {
	suffix = strings.TrimSpace(suffix)
	suffix = strings.TrimPrefix(suffix, ".")
	if suffix == "" {
		return external_id + "_html"
	}
	return external_id + "_" + suffix
}

// content_biz_type normalizes WeChat's picture-message markers for downstream processing.
func content_biz_type(data *wxmp.ArticleCgiDataNew) int {
	if is_album(data) {
		return 2
	}
	return 1
}

// parse_content_images parses ContentNoencode HTML and creates a DownloadResource for each inline image.
func parse_content_images(content_html, content_id, external_id, extra_json string) []*adapter.ResourceInfo {
	if content_html == "" {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content_html))
	if err != nil {
		return nil
	}

	var resources []*adapter.ResourceInfo
	merge_base := 100

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		img_url := s.AttrOr("data-src", "")
		if img_url == "" {
			img_url = s.AttrOr("src", "")
		}
		img_url = normalize_image_url(img_url)
		if img_url == "" {
			return
		}

		hash := md5.Sum([]byte(img_url))
		filename := hex.EncodeToString(hash[:])

		res := model.DownloadResource{
			ContentId:  &content_id,
			Name:       filename,
			Kind:       "image",
			UniqueID:   fmt.Sprintf("%s_img_%d", external_id, i),
			MergeOrder: merge_base + i,
			Extra:      extra_json,
		}
		ep := model.DownloadEndpoint{
			Protocol: "https",
			URL:      img_url,
			Enabled:  1,
			Headers:  wechat_headers,
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource:  res,
			Endpoints: []model.DownloadEndpoint{ep},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindImage,
				Role:     model.ContentAssetRoleAttachment,
				AssetKey: "inline_image:" + filename,
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	})

	return resources
}

// content_image_values copies album images into model values.
func content_image_values(images []*model.ContentImage, album_id string) []model.ContentImage {
	if len(images) == 0 {
		return nil
	}
	values := make([]model.ContentImage, 0, len(images))
	for i, image := range images {
		if image == nil {
			continue
		}
		value := *image
		value.Id = 0
		value.AlbumId = album_id
		value.SortOrder = i
		if image.LivePhoto != nil {
			live_photo := *image.LivePhoto
			live_photo.Formats = append([]model.ContentImageLivePhotoFormat(nil), image.LivePhoto.Formats...)
			value.LivePhoto = &live_photo
		}
		values = append(values, value)
	}
	return values
}

func parse_album_images(images []*model.ContentImage, content_id, external_id, extra_json string) []*adapter.ResourceInfo {
	if len(images) == 0 {
		return nil
	}

	resources := make([]*adapter.ResourceInfo, 0, len(images)*2)
	merge_order := 100

	for i, image := range images {
		if image == nil {
			continue
		}
		img_url := normalize_image_url(image.URL)
		image_key := strings.TrimSpace(image.ImageKey)
		if image_key == "" {
			image_key = model.BuildContentAlbumImageKey("", "", i)
		}
		filename := ""
		if img_url != "" {
			hash := md5.Sum([]byte(img_url))
			filename = hex.EncodeToString(hash[:])
			res := model.DownloadResource{
				ContentId:  &content_id,
				Name:       filename,
				Kind:       "image",
				UniqueID:   fmt.Sprintf("%s_album_%d", external_id, i),
				MergeOrder: merge_order,
				Extra:      extra_json,
			}
			ep := model.DownloadEndpoint{
				Protocol: "https",
				URL:      img_url,
				Enabled:  1,
				Headers:  wechat_headers,
			}
			resources = append(resources, &adapter.ResourceInfo{
				Resource:  res,
				Endpoints: []model.DownloadEndpoint{ep},
				ContentAssets: []adapter.ContentAssetReference{{
					Kind:            model.ContentAssetKindImage,
					Role:            model.ContentAssetRolePrimary,
					AssetKey:        model.BuildContentAlbumImageAssetKey(image_key, "original"),
					Relation:        model.DownloadResourceAssetRelationSource,
					SubjectType:     model.ContentAssetSubjectAlbumImage,
					SubjectKey:      image_key,
					SubjectRelation: model.ContentAssetSubjectRelationRepresentation,
				}},
			})
			merge_order++
		}

		if image.LivePhoto == nil {
			continue
		}
		video_url := normalize_image_url(image.LivePhoto.URL)
		if video_url == "" {
			continue
		}
		if filename == "" {
			hash := md5.Sum([]byte(video_url))
			filename = hex.EncodeToString(hash[:])
		}
		video_resource := model.DownloadResource{
			ContentId:  &content_id,
			Name:       filename,
			Kind:       "video/mp4",
			UniqueID:   fmt.Sprintf("%s_album_%d_live", external_id, i),
			Size:       image.LivePhoto.Size,
			Duration:   duration_milliseconds_to_seconds(image.LivePhoto.DurationMs),
			MergeOrder: merge_order,
			Extra:      extra_json,
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource: video_resource,
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      video_url,
				Enabled:  1,
				Headers:  wechat_headers,
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:            model.ContentAssetKindVideo,
				Role:            model.ContentAssetRoleLivePhoto,
				AssetKey:        model.BuildContentAlbumImageAssetKey(image_key, "live_photo"),
				Relation:        model.DownloadResourceAssetRelationSource,
				SubjectType:     model.ContentAssetSubjectAlbumImage,
				SubjectKey:      image_key,
				SubjectRelation: model.ContentAssetSubjectRelationRepresentation,
			}},
		})
		merge_order++
	}

	return resources
}

func duration_milliseconds_to_seconds(duration_ms int64) int64 {
	if duration_ms <= 0 {
		return 0
	}
	seconds := duration_ms / 1000
	if seconds == 0 {
		return 1
	}
	return seconds
}

// normalize_image_url cleans image URLs: handles HTML entities, protocol prefix, enforces HTTPS.
func normalize_image_url(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	u = strings.ReplaceAll(u, "&amp;amp;", "&")
	u = strings.ReplaceAll(u, "&amp;", "&")
	u = html.UnescapeString(u)
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	if strings.HasPrefix(u, "http://mmbiz.qpic.cn/") {
		u = "https://" + strings.TrimPrefix(u, "http://")
	}
	return u
}

// article_publish_time_val returns the publish timestamp as int64, or 0 if unavailable.
func article_publish_time_val(data *wxmp.ArticleCgiDataNew) int64 {
	if pt := article_publish_time(data); pt != nil {
		return *pt
	}
	if data.CreateTime != "" {
		if ts, err := strconv.ParseInt(data.CreateTime, 10, 64); err == nil && ts > 0 {
			return ts
		}
	}
	return 0
}

// build_extra_json builds the resource.Extra JSON string.
func build_extra_json(id, title, author string, created_at int64) string {
	data, _ := json.Marshal(map[string]string{
		"id":         id,
		"title":      title,
		"author":     author,
		"created_at": strconv.FormatInt(created_at, 10),
	})
	return string(data)
}

// build_config_json returns a map containing only the non-empty config fields.
func build_config_json(config map[string]any) map[string]any {
	m := make(map[string]any, len(config))
	for key, value := range config {
		m[key] = value
	}
	return m
}

func config_string(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}
