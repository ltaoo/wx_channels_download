package wxmp

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
	wxmp "wx_channel/pkg/scraper/wxmp"
	"wx_channel/pkg/util"
)

const platformIDWxMP = "wxmp"

// PlatformID is the platform identifier for WeChat official accounts.
const PlatformID = platformIDWxMP

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(externalID string) string {
	return PlatformID + ":" + externalID
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(externalID string) string {
	return PlatformID + ":" + externalID
}

// ArticleExternalID builds a unique external identifier for an official account article.
func ArticleExternalID(data *wxmp.ArticleCgiDataNew) string {
	if data == nil || strings.TrimSpace(data.Bizuin) == "" {
		return ""
	}
	return strings.TrimSpace(data.Bizuin)
}

// articleCoverURL picks the best cover image URL from the article data.
func articleCoverURL(data *wxmp.ArticleCgiDataNew) string {
	return strings.TrimSpace(data.CdnURL)
}

func buildSourceURL(data *wxmp.ArticleCgiDataNew) string {
	if data == nil {
		return ""
	}
	if strings.TrimSpace(data.UserInfo.ShortLink) != "" {
		return strings.TrimSpace(data.UserInfo.ShortLink)
	}
	return strings.TrimSpace(data.SourceURL)
}

// articleAvatarURL picks the best avatar URL for the publisher account.
func articleAvatarURL(data *wxmp.ArticleCgiDataNew) string {
	return firstNonEmptyStr(
		strings.TrimSpace(data.RoundHeadImg),
		strings.TrimSpace(data.OriHeadImgURL),
		strings.TrimSpace(data.HdHeadImg),
	)
}

// articlePublishTime returns the publish timestamp from the article data.
func articlePublishTime(data *wxmp.ArticleCgiDataNew) *int64 {
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
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, nil, errors.New("missing bizuin/mid/idx in article data")
	}

	now := util.NowMillis()
	contentType := "article"
	if isAlbum(data) {
		contentType = "album"
	}
	c := &model.Content{
		Id:          BuildContentID(externalID),
		PlatformId:  PlatformID,
		Type:        contentType,
		ExternalId:  externalID,
		ExternalId2: strconv.Itoa(data.Mid),
		Title:       strings.TrimSpace(data.Title),
		Description: strings.TrimSpace(data.Desc),
		URL:         strings.TrimSpace(data.Link),
		SourceURL:   buildSourceURL(data),
		CoverURL:    articleCoverURL(data),
		PublishTime: articlePublishTime(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if isAlbum(data) {
		album, images := buildContentAlbum(data, c.Id)
		return c, &ContentAlbumExt{Album: album, Images: images}, nil
	}

	return c, buildContentArticle(data, c.Id), nil
}

// ToAccount converts an ArticleCgiData publisher into a model.Account.
func ToAccount(data *wxmp.ArticleCgiDataNew) (*model.Account, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := data.UserName
	if externalID == "" {
		return nil, errors.New("missing bizuin in article data")
	}

	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(externalID),
		PlatformId: PlatformID,
		ExternalId: data.UserName,
		Alias:      strings.TrimSpace(data.Alias),
		Nickname:   strings.TrimSpace(data.NickName),
		Signature:  strings.TrimSpace(data.Signature),
		AvatarURL:  articleAvatarURL(data),
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
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	accountID := BuildAccountID(strings.TrimSpace(data.Bizuin))
	now := util.NowMillis()
	contentType := "article"
	if isAlbum(data) {
		contentType = "album"
	}

	return &model.BrowseHistory{
		PlatformId:        PlatformID,
		VisitedTimes:      1,
		AccountId:         &accountID,
		AccountExternalId: strings.TrimSpace(data.Bizuin),
		AccountNickname:   strings.TrimSpace(data.NickName),
		AccountAvatarURL:  articleAvatarURL(data),
		Type:              contentType,
		ExternalId:        externalID,
		Title:             strings.TrimSpace(data.Title),
		URL:               strings.TrimSpace(data.Link),
		SourceURL:         buildSourceURL(data),
		CoverURL:          articleCoverURL(data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func isAlbum(data *wxmp.ArticleCgiDataNew) bool {
	if data == nil {
		return false
	}
	if data.PageType == 2 {
		return true
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return len(data.PicturePageInfoList) >= 4
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return len(data.PicturePageInfoList) >= 4
	}

	if _, ok := payload["appmsgalbuminfo"]; ok {
		return false
	}
	if _, ok := payload["public_tag_info"]; ok {
		return false
	}

	return len(data.PicturePageInfoList) >= 4
}

// BuildBrowseRecordFromObject converts article CGI data into a model.BrowseHistory.
func BuildBrowseRecordFromObject(data *wxmp.ArticleCgiDataNew) *model.BrowseHistory {
	record, _ := ArticleToHistory(data)
	return record
}

// ArticleToContentArticle converts an ArticleCgiData into a model.ContentArticle with the HTML body.
func ArticleToContentArticle(data *wxmp.ArticleCgiDataNew) (*model.ContentArticle, error) {
	if data == nil {
		return nil, errors.New("article data is nil")
	}
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	return buildContentArticle(data, BuildContentID(externalID)), nil
}

func buildContentArticle(data *wxmp.ArticleCgiDataNew, id string) *model.ContentArticle {
	article := &model.ContentArticle{
		Id:        id,
		Type:      model.ContentArticleTypeHTML,
		WordCount: articleWordCount(data.ContentNoencode),
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

func buildContentAlbum(data *wxmp.ArticleCgiDataNew, contentID string) (*model.ContentAlbum, []*model.ContentImage) {
	albumImages := make([]*model.ContentImage, 0, len(data.PicturePageInfoList))
	for i, picture := range data.PicturePageInfoList {
		imageURL := normalizeImageURL(picture.CdnUrl)
		if imageURL == "" {
			continue
		}
		albumImages = append(albumImages, &model.ContentImage{
			AlbumId:   contentID,
			SortOrder: i,
			URL:       imageURL,
			Width:     picture.Width,
			Height:    picture.Height,
		})
	}

	album := &model.ContentAlbum{
		Id:          contentID,
		ImageCount:  len(albumImages),
		Format:      strings.TrimSpace(data.ImgFormat),
		Description: strings.TrimSpace(data.Desc),
	}
	if len(albumImages) > 0 {
		album.CoverWidth = albumImages[0].Width
		album.CoverHeight = albumImages[0].Height
	}
	return album, albumImages
}

func articleWordCount(contentHTML string) int {
	text := contentHTML
	if doc, err := goquery.NewDocumentFromReader(strings.NewReader(contentHTML)); err == nil {
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
	externalID := ArticleExternalID(data)
	if externalID == "" {
		return nil, errors.New("missing bizuin/mid/idx in article data")
	}

	return &model.ContentAccount{
		ContentId: BuildContentID(externalID),
		AccountId: BuildAccountID(strings.TrimSpace(data.Bizuin)),
		Role:      "publisher",
		CreatedAt: util.NowMillis(),
	}, nil
}

// firstNonEmptyStr returns the first non-empty string from the given values.
func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, configRaw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	var data wxmp.ArticleCgiDataNew
	if err := json.Unmarshal(contentJSON, &data); err != nil {
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

	externalID := ArticleExternalID(&data)
	if externalID == "" {
		return nil, fmt.Errorf("无法生成文章唯一标识")
	}

	title, _ := config["filename"].(string)
	if title == "" {
		title = strings.TrimSpace(data.Title)
	}

	configJSON, _ := json.Marshal(buildConfigJSON(config))
	bizType := contentBizType(&data)
	metadataJSON, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"external_id": externalID,
		"author":      strings.TrimSpace(data.NickName),
		"created_at":  articlePublishTimeVal(&data),
		"biz_type":    bizType,
	})

	extraJSON := buildExtraJSON(externalID, title, strings.TrimSpace(data.NickName), articlePublishTimeVal(&data))
	contentID := content.Id

	var imageResources []*adapter.ResourceInfo
	var albumImages []*model.ContentImage
	if bizType == 2 {
		albumExt, ok := ext.(*ContentAlbumExt)
		if !ok {
			return nil, fmt.Errorf("图集内容缺少图集详情")
		}
		imageResources = parseAlbumImages(albumExt.Images, contentID, externalID, extraJSON)
		albumImages = albumExt.Images
		ext = albumExt.Album
	} else {
		imageResources = parseContentImages(data.ContentNoencode, contentID, externalID, extraJSON)
	}

	coverURL := strings.TrimSpace(data.CdnURL)
	sourceURL := strings.TrimSpace(data.SourceURL)
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(data.Link)
	}
	coverMergeOrder := 100 + len(imageResources)
	coverResource := model.DownloadResource{
		ContentId:  &contentID,
		Name:       title,
		Kind:       "image",
		UniqueID:   externalID + "_cover",
		MergeOrder: coverMergeOrder,
		Extra:      extraJSON,
	}
	coverEndpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      coverURL,
		Enabled:  1,
		Headers:  wechatHeaders,
	}

	htmlName := title
	htmlResource := model.DownloadResource{
		ContentId:  &contentID,
		Name:       htmlName,
		Kind:       "html",
		UniqueID:   externalID + "_html",
		MergeOrder: 0,
		Extra:      extraJSON,
	}
	htmlEndpoint := model.DownloadEndpoint{
		Protocol: "inline",
		URL:      data.ContentNoencode,
		Enabled:  1,
	}

	resources := make([]*adapter.ResourceInfo, 0, len(imageResources)+2)
	resources = append(resources, &adapter.ResourceInfo{
		DownloadResource: htmlResource,
		Endpoints:        []model.DownloadEndpoint{htmlEndpoint},
	})
	for _, r := range imageResources {
		resources = append(resources, r)
	}
	resources = append(resources, &adapter.ResourceInfo{
		DownloadResource: coverResource,
		Endpoints:        []model.DownloadEndpoint{coverEndpoint},
	})

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     buildDownloadTaskUniqueID(externalID, configString(config, "suffix")),
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    sourceURL,
			CoverURL:     coverURL,
			ConfigJSON:   string(configJSON),
			MetadataJSON: string(metadataJSON),
		},
		Resources:     resources,
		ContentDetail: ext,
		AlbumImages:   albumImages,
		Account:       account,
		Content:       content,
	}, nil
}

func buildDownloadTaskUniqueID(externalID, suffix string) string {
	suffix = strings.TrimSpace(suffix)
	suffix = strings.TrimPrefix(suffix, ".")
	if suffix == "" {
		return externalID + "_html"
	}
	return externalID + "_" + suffix
}

// contentBizType normalizes WeChat's picture-message markers for downstream processing.
func contentBizType(data *wxmp.ArticleCgiDataNew) int {
	if data.ItemShowType == 8 || data.RealItemShowType == 8 {
		return 2
	}
	return data.BizType
}

// parseContentImages parses ContentNoencode HTML and creates a DownloadResource for each inline image.
func parseContentImages(contentHTML, contentID, externalID, extraJSON string) []*adapter.ResourceInfo {
	if contentHTML == "" {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(contentHTML))
	if err != nil {
		return nil
	}

	var resources []*adapter.ResourceInfo
	mergeBase := 100

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		imgURL := s.AttrOr("data-src", "")
		if imgURL == "" {
			imgURL = s.AttrOr("src", "")
		}
		imgURL = normalizeImageURL(imgURL)
		if imgURL == "" {
			return
		}

		hash := md5.Sum([]byte(imgURL))
		filename := hex.EncodeToString(hash[:])

		res := model.DownloadResource{
			ContentId:  &contentID,
			Name:       filename,
			Kind:       "image",
			UniqueID:   fmt.Sprintf("%s_img_%d", externalID, i),
			MergeOrder: mergeBase + i,
			Extra:      extraJSON,
		}
		ep := model.DownloadEndpoint{
			Protocol: "https",
			URL:      imgURL,
			Enabled:  1,
			Headers:  wechatHeaders,
		}
		resources = append(resources, &adapter.ResourceInfo{
			DownloadResource: res,
			Endpoints:        []model.DownloadEndpoint{ep},
		})
	})

	return resources
}

// parseAlbumImages creates a DownloadResource for each image in the album.
func parseAlbumImages(images []*model.ContentImage, contentID, externalID, extraJSON string) []*adapter.ResourceInfo {
	if len(images) == 0 {
		return nil
	}

	var resources []*adapter.ResourceInfo
	mergeBase := 100

	for i, image := range images {
		imgURL := normalizeImageURL(image.URL)
		if imgURL == "" {
			continue
		}

		hash := md5.Sum([]byte(imgURL))
		filename := hex.EncodeToString(hash[:])

		res := model.DownloadResource{
			ContentId:  &contentID,
			Name:       filename,
			Kind:       "image",
			UniqueID:   fmt.Sprintf("%s_album_%d", externalID, i),
			MergeOrder: mergeBase + i,
			Extra:      extraJSON,
		}
		ep := model.DownloadEndpoint{
			Protocol: "https",
			URL:      imgURL,
			Enabled:  1,
			Headers:  wechatHeaders,
		}
		resources = append(resources, &adapter.ResourceInfo{
			DownloadResource: res,
			Endpoints:        []model.DownloadEndpoint{ep},
		})
	}

	return resources
}

// normalizeImageURL cleans image URLs: handles HTML entities, protocol prefix, enforces HTTPS.
func normalizeImageURL(raw string) string {
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

// articlePublishTimeVal returns the publish timestamp as int64, or 0 if unavailable.
func articlePublishTimeVal(data *wxmp.ArticleCgiDataNew) int64 {
	if pt := articlePublishTime(data); pt != nil {
		return *pt
	}
	if data.CreateTime != "" {
		if ts, err := strconv.ParseInt(data.CreateTime, 10, 64); err == nil && ts > 0 {
			return ts
		}
	}
	return 0
}

// buildExtraJSON builds the resource.Extra JSON string.
func buildExtraJSON(id, title, author string, createdAt int64) string {
	data, _ := json.Marshal(map[string]string{
		"id":         id,
		"title":      title,
		"author":     author,
		"created_at": strconv.FormatInt(createdAt, 10),
	})
	return string(data)
}

// buildConfigJSON returns a map containing only the non-empty config fields.
func buildConfigJSON(config map[string]any) map[string]any {
	m := make(map[string]any, len(config))
	for key, value := range config {
		m[key] = value
	}
	return m
}

func configString(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}
