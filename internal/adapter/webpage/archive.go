package webpageadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/webpage"
)

const (
	webpage_postprocess_marker_key         = "webpage_postprocess"
	webpage_postprocess_marker_value       = "inline_markdown_images"
	webpage_postprocess_image_marker_key   = "webpage_postprocess_image"
	webpage_postprocess_image_marker_value = "inline"
	webpage_image_source_url_key           = "source_url"
	webpage_image_placeholder_key          = "placeholder"
)

type archive_image struct {
	source_url  string
	placeholder string
	hash        string
}

func build_archive_markdown(page *webpage.Page) (string, []archive_image, error) {
	if page == nil || strings.TrimSpace(page.HTML) == "" {
		if page == nil {
			return "", nil, nil
		}
		return strings.TrimSpace(page.Markdown), nil, nil
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(page.HTML))
	if err != nil {
		return "", nil, fmt.Errorf("解析网页归档 HTML 失败: %w", err)
	}
	images := make([]archive_image, 0)
	images_by_url := make(map[string]archive_image)
	document.Find("img[src]").Each(func(_ int, image *goquery.Selection) {
		source_url := strings.TrimSpace(image.AttrOr("src", ""))
		if !is_http_url(source_url) {
			return
		}
		archive_item, exists := images_by_url[source_url]
		if !exists {
			hash_data := sha256.Sum256([]byte(source_url))
			hash := hex.EncodeToString(hash_data[:])
			archive_item = archive_image{
				source_url:  source_url,
				placeholder: "webpage-image://" + hash,
				hash:        hash,
			}
			images_by_url[source_url] = archive_item
			images = append(images, archive_item)
		}
		image.SetAttr("src", archive_item.placeholder)
	})
	body := document.Find("body").First()
	archive_html := page.HTML
	if body.Length() > 0 {
		archive_html, err = body.Html()
		if err != nil {
			return "", nil, fmt.Errorf("序列化网页归档 HTML 失败: %w", err)
		}
	}
	converter := htmltomarkdown.NewConverter(page.ArchiveURL(), true, nil)
	markdown, err := converter.ConvertString(archive_html)
	if err != nil {
		return "", nil, fmt.Errorf("生成网页归档 Markdown 失败: %w", err)
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return strings.TrimSpace(page.Markdown), nil, nil
	}
	return markdown, images, nil
}

func build_archive_image_resources(page *webpage.Page, content *model.Content, images []archive_image, cookie_provider *cookies.Reader) ([]*adapter.ResourceInfo, error) {
	if page == nil || content == nil || len(images) == 0 {
		return nil, nil
	}
	headers_data, _ := json.Marshal(map[string]string{
		"Accept":  "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
		"Referer": page.ArchiveURL(),
	})
	resources := make([]*adapter.ResourceInfo, 0, len(images))
	for image_index, image := range images {
		parsed_url, err := url.Parse(image.source_url)
		if err != nil || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") || parsed_url.Hostname() == "" {
			return nil, fmt.Errorf("网页图片 URL 无效: %s", image.source_url)
		}
		cookie_header, err := archive_image_cookie(cookie_provider, parsed_url.Hostname())
		if err != nil {
			return nil, err
		}
		extra_data, _ := json.Marshal(map[string]string{
			webpage_postprocess_image_marker_key: webpage_postprocess_image_marker_value,
			webpage_image_source_url_key:         image.source_url,
			webpage_image_placeholder_key:        image.placeholder,
		})
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId:  &content.Id,
				Name:       fmt.Sprintf("inline_image_%03d", image_index+1),
				Kind:       "image",
				UniqueID:   content.ExternalId + "_image_" + image.hash[:16],
				MergeOrder: 100 + image_index,
				Extra:      string(extra_data),
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: parsed_url.Scheme,
				URL:      image.source_url,
				Enabled:  1,
				Headers:  string(headers_data),
				Cookies:  cookie_header,
			}},
		})
	}
	return resources, nil
}

func archive_image_cookie(cookie_provider *cookies.Reader, domain string) (string, error) {
	if cookie_provider == nil || strings.TrimSpace(domain) == "" {
		return "", nil
	}
	cookie_header, err := cookie_provider.HeaderForDomain(strings.ToLower(strings.TrimSpace(domain)))
	if errors.Is(err, cookies.ErrCookieNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取网页图片域名 %s 的 Cookie 失败: %w", domain, err)
	}
	return strings.TrimSpace(cookie_header), nil
}

func is_http_url(raw_url string) bool {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Hostname() == "" {
		return false
	}
	scheme := strings.ToLower(parsed_url.Scheme)
	return scheme == "http" || scheme == "https"
}
