package wxmpadapter

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
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

// ToContent converts ArticleCgiDataNew into a slim model.Content and its type-specific extension.
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

// ToAccount converts an ArticleCgiDataNew publisher into a model.Account.
func ToAccount(data *wxmp.ArticleCgiDataNew) (*model.Account, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	external_id := data.UserName
	if external_id == "" {
		return nil, errors.New("missing user_name in article data")
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

// ArticleToHistory converts an ArticleCgiDataNew into a model.BrowseHistory.
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

// ArticleToContentArticle converts ArticleCgiDataNew into a model.ContentArticle with the HTML body.
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
	account_external_id := strings.TrimSpace(data.UserName)
	if account_external_id == "" {
		return nil, errors.New("missing user_name in article data")
	}

	return &model.ContentAccount{
		ContentId: BuildContentID(external_id),
		AccountId: BuildAccountID(account_external_id),
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
	video_resources, video_details := build_wxmp_embedded_videos(
		&data,
		content,
		account,
		external_id,
		extra_json,
		config_string(config, "video_variant_key"),
		first_non_empty_str(
			config_string(config, "video_variant_spec"),
			config_string(config, "spec"),
		),
	)

	// Keep the cover URL as task metadata only. The wxmp adapter must not add a
	// separate cover DownloadResource by default.
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

	resources := make([]*adapter.ResourceInfo, 0, len(image_resources)+len(video_resources)+1)
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
	for _, r := range video_resources {
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
		Resources:      resources,
		ContentDetail:  ext,
		ContentDetails: wxmp_content_details(content, ext, video_details...),
		Account:        account,
		Content:        content,
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

// parse_content_videos creates one selected resource for each video embedded in
// the article body. The richer ContentVideo graph is built by
// build_wxmp_embedded_videos and exposed through ContentDetails.
func parse_content_videos(data *wxmp.ArticleCgiDataNew, content_id, external_id, extra_json string) []*adapter.ResourceInfo {
	resources, _ := build_wxmp_embedded_videos(
		data,
		&model.Content{Id: content_id},
		nil,
		external_id,
		extra_json,
		"",
		"",
	)
	return resources
}

func build_wxmp_embedded_videos(
	data *wxmp.ArticleCgiDataNew,
	root_content *model.Content,
	account *model.Account,
	external_id string,
	extra_json string,
	selected_variant_key string,
	selected_variant_spec string,
) ([]*adapter.ResourceInfo, []adapter.ContentDetail) {
	if data == nil || root_content == nil || len(data.VideoPageInfos) == 0 {
		return nil, nil
	}

	video_info_by_id := make(map[string]*wxmp.VideoPageInfoItem, len(data.VideoPageInfos))
	for video_index := range data.VideoPageInfos {
		video_info := &data.VideoPageInfos[video_index]
		video_id := wxmp_video_info_id(video_info)
		if video_id != "" {
			video_info_by_id[video_id] = video_info
		}
	}

	ordered_video_ids := embedded_wxmp_video_ids(data.ContentNoencode)
	if len(ordered_video_ids) == 0 {
		for video_index := range data.VideoPageInfos {
			if video_id := wxmp_video_info_id(&data.VideoPageInfos[video_index]); video_id != "" {
				ordered_video_ids = append(ordered_video_ids, video_id)
			}
		}
	}

	resources := make([]*adapter.ResourceInfo, 0, len(ordered_video_ids))
	details := make([]adapter.ContentDetail, 0, len(ordered_video_ids))
	seen_video_ids := make(map[string]bool, len(ordered_video_ids))
	for _, video_id := range ordered_video_ids {
		if seen_video_ids[video_id] {
			continue
		}
		seen_video_ids[video_id] = true
		video_info := video_info_by_id[video_id]
		if video_info == nil {
			continue
		}

		variants := wxmp_content_video_variants(
			video_id,
			video_info.MpVideoTransInfo,
			selected_variant_key,
			selected_variant_spec,
		)
		selected_variant := selected_wxmp_content_video_variant(variants)
		if selected_variant == nil {
			continue
		}

		video_number := len(resources) + 1
		video_content := wxmp_embedded_video_content(root_content, video_info, video_id, video_number)
		content_video := wxmp_embedded_content_video(video_content.Id, variants, selected_variant)
		resources = append(resources, wxmp_embedded_video_resource(
			video_content.Id,
			external_id,
			extra_json,
			video_id,
			video_number,
			selected_variant,
		))
		details = append(details, adapter.ContentDetail{
			Type:     "video",
			Key:      video_content.Id,
			Data:     content_video,
			Content:  video_content,
			Accounts: content_account_references(account),
			Relation: &model.ContentRelation{
				SourceContentId: root_content.Id,
				TargetContentId: video_content.Id,
				Type:            model.ContentRelationContains,
			},
		})
	}
	return resources, details
}

func content_account_references(account *model.Account) []adapter.ContentAccountReference {
	if account == nil {
		return nil
	}
	return []adapter.ContentAccountReference{{
		Account: account,
		Role:    "owner",
	}}
}

func wxmp_embedded_video_content(
	root_content *model.Content,
	video_info *wxmp.VideoPageInfoItem,
	video_id string,
	video_number int,
) *model.Content {
	video_title := fmt.Sprintf("视频 %d", video_number)
	if strings.TrimSpace(root_content.Title) != "" {
		video_title = fmt.Sprintf("%s - 视频 %d", strings.TrimSpace(root_content.Title), video_number)
	}
	video_url := strings.TrimSpace(video_info.SourceLink)
	if video_url == "" {
		video_url = strings.TrimSpace(root_content.URL)
	}
	source_url := strings.TrimSpace(root_content.SourceURL)
	if source_url == "" {
		source_url = strings.TrimSpace(root_content.URL)
	}
	return &model.Content{
		Id:         BuildContentID(video_id),
		PlatformId: PlatformID,
		Type:       "video",
		ExternalId: video_id,
		Title:      video_title,
		URL:        video_url,
		SourceURL:  source_url,
		CoverURL: normalize_image_url(first_non_empty_str(
			video_info.CoverUrl169,
			video_info.CoverUrl,
			video_info.CoverUrl11,
		)),
		PublishTime: root_content.PublishTime,
		Timestamps:  root_content.Timestamps,
	}
}

func wxmp_embedded_content_video(
	video_content_id string,
	variants []model.ContentVideoVariant,
	selected_variant *model.ContentVideoVariant,
) *model.ContentVideo {
	content_video := &model.ContentVideo{
		Id:       video_content_id,
		Duration: wxmp_variant_duration_seconds(selected_variant),
		Size:     selected_variant.Size,
		Format:   selected_variant.Format,
		URL:      selected_variant.URL,
		Variants: variants,
	}
	if selected_variant.Width != nil {
		content_video.Width = *selected_variant.Width
	}
	if selected_variant.Height != nil {
		content_video.Height = *selected_variant.Height
	}
	return content_video
}

func wxmp_embedded_video_resource(
	content_id string,
	external_id string,
	extra_json string,
	video_id string,
	video_number int,
	selected_variant *model.ContentVideoVariant,
) *adapter.ResourceInfo {
	resource := model.DownloadResource{
		ContentId:  &content_id,
		Name:       fmt.Sprintf("video_%02d", video_number),
		Kind:       "video/mp4",
		UniqueID:   fmt.Sprintf("%s_video_%s_%s", external_id, video_id, wxmp_video_resource_variant_id(selected_variant)),
		Size:       selected_variant.Size,
		MergeOrder: 200 + video_number,
		Duration:   wxmp_variant_duration_seconds(selected_variant),
		Extra:      extra_json,
	}
	return &adapter.ResourceInfo{
		Resource: resource,
		Endpoints: []model.DownloadEndpoint{{
			Protocol: download_endpoint_protocol(selected_variant.URL),
			URL:      selected_variant.URL,
			Enabled:  1,
			Headers:  wechat_headers,
		}},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindVideo,
			Role:     model.ContentAssetRoleVideoVariant,
			AssetKey: selected_variant.VariantKey,
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	}
}

func wxmp_video_resource_variant_id(variant *model.ContentVideoVariant) string {
	if variant != nil && strings.TrimSpace(variant.Spec) != "" {
		return "format_" + strings.TrimSpace(variant.Spec)
	}
	video_url := ""
	if variant != nil {
		video_url = variant.URL
	}
	hash := md5.Sum([]byte(video_url))
	return "url_" + hex.EncodeToString(hash[:8])
}

func embedded_wxmp_video_ids(content_html string) []string {
	if strings.TrimSpace(content_html) == "" {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content_html))
	if err != nil {
		return nil
	}

	video_ids := make([]string, 0)
	doc.Find("iframe.video_iframe, iframe[data-mpvid], iframe[data-vid]").Each(func(_ int, selection *goquery.Selection) {
		video_id := strings.TrimSpace(first_non_empty_str(
			selection.AttrOr("data-mpvid", ""),
			selection.AttrOr("data-vid", ""),
			selection.AttrOr("vid", ""),
		))
		if video_id == "" {
			video_id = wxmp_video_id_from_player_url(selection.AttrOr("data-src", ""))
		}
		if video_id != "" {
			video_ids = append(video_ids, video_id)
		}
	})
	return video_ids
}

func wxmp_video_id_from_player_url(raw_url string) string {
	player_url, err := url.Parse(html.UnescapeString(strings.TrimSpace(raw_url)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(player_url.Query().Get("vid"))
}

func wxmp_video_info_id(video_info *wxmp.VideoPageInfoItem) string {
	if video_info == nil {
		return ""
	}
	return strings.TrimSpace(first_non_empty_str(video_info.VideoID, video_info.HitVid))
}

func wxmp_content_video_variants(
	video_id string,
	trans_info []wxmp.MpVideoTransInfo,
	selected_variant_key string,
	selected_variant_spec string,
) []model.ContentVideoVariant {
	variants := make([]model.ContentVideoVariant, 0, len(trans_info))
	seen_urls := make(map[string]bool, len(trans_info))
	for trans_index := range trans_info {
		candidate := &trans_info[trans_index]
		video_url := normalize_image_url(candidate.Url)
		if video_url == "" || seen_urls[video_url] {
			continue
		}
		seen_urls[video_url] = true
		variant_key := wxmp_video_variant_key(video_id, candidate, video_url)
		spec := ""
		if candidate.FormatID > 0 {
			spec = strconv.Itoa(candidate.FormatID)
		}
		metadata, _ := json.Marshal(map[string]any{
			"duration_ms":         candidate.DurationMs,
			"format_id":           candidate.FormatID,
			"video_quality_level": candidate.VideoQualityLevel,
		})
		variant := model.ContentVideoVariant{
			VideoId:      BuildContentID(video_id),
			VariantKey:   variant_key,
			Spec:         spec,
			Quality:      strings.TrimSpace(candidate.VideoQualityWording),
			Size:         wxmp_video_file_size(candidate.Filesize),
			Format:       "mp4",
			StreamType:   model.ContentVideoVariantStreamTypeProgressive,
			HasVideo:     1,
			HasAudio:     1,
			URL:          video_url,
			URLExpiresAt: wxmp_video_url_expires_at(video_url),
			Metadata:     string(metadata),
		}
		variant.Width = positive_wxmp_int_pointer(candidate.Width)
		variant.Height = positive_wxmp_int_pointer(candidate.Height)
		variants = append(variants, variant)
	}

	selected_variant_key = strings.TrimSpace(selected_variant_key)
	selected_variant_spec = strings.TrimSpace(selected_variant_spec)
	selected_index := -1
	for variant_index := range variants {
		variant := &variants[variant_index]
		if selected_variant_key != "" && variant.VariantKey == selected_variant_key {
			selected_index = variant_index
			break
		}
		if selected_variant_key == "" && selected_variant_spec != "" && variant.Spec == selected_variant_spec {
			selected_index = variant_index
			break
		}
	}
	if selected_index < 0 && len(variants) > 0 {
		selected_index = 0
	}
	if selected_index >= 0 {
		variants[selected_index].IsDefault = 1
	}
	return variants
}

func wxmp_video_variant_key(video_id string, trans_info *wxmp.MpVideoTransInfo, video_url string) string {
	if trans_info != nil && trans_info.FormatID > 0 {
		return fmt.Sprintf("%s:format:%d", video_id, trans_info.FormatID)
	}
	hash := md5.Sum([]byte(video_url))
	return fmt.Sprintf("%s:url:%s", video_id, hex.EncodeToString(hash[:8]))
}

func selected_wxmp_content_video_variant(variants []model.ContentVideoVariant) *model.ContentVideoVariant {
	for variant_index := range variants {
		if variants[variant_index].IsDefault != 0 {
			return &variants[variant_index]
		}
	}
	if len(variants) == 0 {
		return nil
	}
	return &variants[0]
}

func positive_wxmp_int_pointer(value int) *int {
	if value <= 0 {
		return nil
	}
	result := value
	return &result
}

func wxmp_video_url_expires_at(raw_url string) *int64 {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return nil
	}
	expires_at, err := strconv.ParseInt(parsed_url.Query().Get("dis_t"), 10, 64)
	if err != nil || expires_at <= 0 {
		return nil
	}
	expires_at *= 1000
	return &expires_at
}

func wxmp_variant_duration_seconds(variant *model.ContentVideoVariant) int64 {
	if variant == nil || strings.TrimSpace(variant.Metadata) == "" {
		return 0
	}
	var metadata struct {
		DurationMs int64 `json:"duration_ms"`
	}
	if err := json.Unmarshal([]byte(variant.Metadata), &metadata); err != nil {
		return 0
	}
	return duration_milliseconds_to_seconds(metadata.DurationMs)
}

func download_endpoint_protocol(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
}

func wxmp_video_file_size(value any) int64 {
	switch size := value.(type) {
	case int:
		return int64(size)
	case int32:
		return int64(size)
	case int64:
		return size
	case float32:
		return int64(size)
	case float64:
		return int64(size)
	case json.Number:
		parsed_size, _ := size.Int64()
		return parsed_size
	case string:
		parsed_size, _ := strconv.ParseInt(strings.TrimSpace(size), 10, 64)
		return parsed_size
	default:
		return 0
	}
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
		indexed_extra_json := build_indexed_extra_json(extra_json, i+1)
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
				Extra:      indexed_extra_json,
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
			Extra:      indexed_extra_json,
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

// normalize_image_url cleans WeChat media URLs: handles HTML entities and protocol prefixes.
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
	if strings.HasPrefix(u, "http://mmbiz.qpic.cn/") || strings.HasPrefix(u, "http://mpvideo.qpic.cn/") {
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

// build_indexed_extra_json returns a copy of resource Extra with the
// one-based position of an image in an album. A Live Photo and its paired
// still image intentionally share the same index.
func build_indexed_extra_json(extra_json string, idx int) string {
	if idx <= 0 {
		return extra_json
	}
	extra := make(map[string]any)
	if err := json.Unmarshal([]byte(extra_json), &extra); err != nil {
		return extra_json
	}
	extra["idx"] = idx
	data, err := json.Marshal(extra)
	if err != nil {
		return extra_json
	}
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
