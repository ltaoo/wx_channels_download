package wxmp

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	"wx_channel/internal/download/types"
	scraper "wx_channel/pkg/scraper/wxmp"
)

var wechatHeaders string

func init() {
	registry.Register(&handler{})
	h := map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN",
		"Referer":    "https://mp.weixin.qq.com/",
	}
	b, _ := json.Marshal(h)
	wechatHeaders = string(b)
}

type handler struct{}

func (h *handler) PlatformID() string { return PlatformID }

// DownloadConfig holds WeChat Official Account download configuration.
type DownloadConfig struct {
	Filename  string `json:"filename"`
	Suffix    string `json:"suffix"`
	Overwrite bool   `json:"overwrite"`
	Duplicate bool   `json:"duplicate"`
}

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, configRaw json.RawMessage) (*types.DownloadTaskResult, error) {
	var config DownloadConfig
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	var data scraper.ArticleCgiDataNew
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

	title := config.Filename
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

	// Parse images based on content type
	var imageResources []*types.ResourceInfo
	var albumImages []*model.ContentImage
	if bizType == 2 {
		// Album: images from the platform-neutral ContentAlbum representation.
		albumExt, ok := ext.(*ContentAlbumExt)
		if !ok {
			return nil, fmt.Errorf("图集内容缺少图集详情")
		}
		imageResources = parseAlbumImages(albumExt.Images, externalID, extraJSON)
		albumImages = albumExt.Images
		ext = albumExt.Album
	} else {
		// Article: parse images from ContentNoencode HTML
		imageResources = parseContentImages(data.ContentNoencode, externalID, extraJSON)
	}

	// Cover image resource (placed after content images in merge order)
	coverURL := strings.TrimSpace(data.CdnURL)
	sourceURL := strings.TrimSpace(data.SourceURL)
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(data.Link)
	}
	coverMergeOrder := 100 + len(imageResources)
	coverResource := model.DownloadResource{
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

	// HTML content resource (content_noencode saved as .html file)
	htmlName := title
	htmlResource := model.DownloadResource{
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

	// Build resources list: html first, then content images, then cover
	resources := make([]*types.ResourceInfo, 0, len(imageResources)+2)
	resources = append(resources, &types.ResourceInfo{
		DownloadResource: htmlResource,
		Endpoints:        []model.DownloadEndpoint{htmlEndpoint},
	})
	for _, r := range imageResources {
		resources = append(resources, r)
	}
	resources = append(resources, &types.ResourceInfo{
		DownloadResource: coverResource,
		Endpoints:        []model.DownloadEndpoint{coverEndpoint},
	})

	return &types.DownloadTaskResult{
		Task: &model.DownloadTaskV1{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     buildDownloadTaskUniqueID(externalID, config.Suffix),
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

// contentBizType normalizes WeChat's picture-message markers for downstream
// processing. Real album responses commonly keep biz_type=1 and identify the
// content through item_show_type=8.
func contentBizType(data *scraper.ArticleCgiDataNew) int {
	if data.ItemShowType == 8 || data.RealItemShowType == 8 {
		return 2
	}
	return data.BizType
}

// parseContentImages parses ContentNoencode HTML and creates a DownloadResource for each inline image.
func parseContentImages(contentHTML, externalID, extraJSON string) []*types.ResourceInfo {
	if contentHTML == "" {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(contentHTML))
	if err != nil {
		return nil
	}

	var resources []*types.ResourceInfo
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
		resources = append(resources, &types.ResourceInfo{
			DownloadResource: res,
			Endpoints:        []model.DownloadEndpoint{ep},
		})
	})

	return resources
}

// parseAlbumImages creates a DownloadResource for each image in the album.
func parseAlbumImages(images []*model.ContentImage, externalID, extraJSON string) []*types.ResourceInfo {
	if len(images) == 0 {
		return nil
	}

	var resources []*types.ResourceInfo
	mergeBase := 100

	for i, image := range images {
		imgURL := normalizeImageURL(image.URL)
		if imgURL == "" {
			continue
		}

		hash := md5.Sum([]byte(imgURL))
		filename := hex.EncodeToString(hash[:])

		res := model.DownloadResource{
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
		resources = append(resources, &types.ResourceInfo{
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
func articlePublishTimeVal(data *scraper.ArticleCgiDataNew) int64 {
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

// buildExtraJSON builds the resource.Extra JSON string, used by filenameTemplate and onFilename hook meta parameters.
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
func buildConfigJSON(config DownloadConfig) map[string]any {
	m := make(map[string]any)
	if config.Filename != "" {
		m["filename"] = config.Filename
	}
	if config.Suffix != "" {
		m["suffix"] = config.Suffix
	}
	if config.Overwrite {
		m["overwrite"] = true
	}
	if config.Duplicate {
		m["duplicate"] = true
	}
	return m
}
