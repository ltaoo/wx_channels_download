package weiboadapter

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
)

// Postprocess embeds downloaded Weibo images into the HTML resource.
func (h *handler) Postprocess(post_context context.Context, task *hermes.TaskJob, _ adapter.PostprocessDeps) error {
	if task == nil {
		return fmt.Errorf("weibo postprocess: task is nil")
	}
	if err := context.Cause(post_context); err != nil {
		return err
	}
	var html_resource *hermes.ResourceJob
	image_resources := make(map[string]*hermes.ResourceJob)
	for resource_index := range task.Resources {
		resource := &task.Resources[resource_index]
		kind := strings.ToLower(strings.TrimSpace(resource.Kind))
		if (kind == "html" || kind == "text/html") && resource.FilePath != "" {
			html_resource = resource
		}
		if kind == "image" || strings.HasPrefix(kind, "image/") {
			for _, endpoint := range resource.Endpoints {
				if image_url := strings.TrimSpace(endpoint.URL); image_url != "" {
					image_resources[image_url] = resource
				}
			}
		}
	}
	if html_resource == nil {
		return fmt.Errorf("weibo postprocess: task %d has no downloaded HTML resource", task.ID)
	}
	if len(image_resources) == 0 {
		return nil
	}
	html_data, err := os.ReadFile(html_resource.FilePath)
	if err != nil {
		return fmt.Errorf("weibo postprocess: read HTML: %w", err)
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(html_data)))
	if err != nil {
		return fmt.Errorf("weibo postprocess: parse HTML: %w", err)
	}
	embedded_count := 0
	embed_attribute := func(selection *goquery.Selection, attribute_name string) bool {
		if context.Cause(post_context) != nil {
			return false
		}
		resource := image_resources[strings.TrimSpace(selection.AttrOr(attribute_name, ""))]
		data_uri := image_data_uri(resource)
		if data_uri == "" {
			return true
		}
		selection.SetAttr(attribute_name, data_uri)
		selection.RemoveAttr("srcset")
		selection.RemoveAttr("data-src")
		embedded_count++
		return true
	}
	document.Find("img[src]").EachWithBreak(func(_ int, image *goquery.Selection) bool {
		return embed_attribute(image, "src")
	})
	document.Find("video[poster]").EachWithBreak(func(_ int, video *goquery.Selection) bool {
		return embed_attribute(video, "poster")
	})
	if err := context.Cause(post_context); err != nil {
		return err
	}
	if embedded_count == 0 {
		return nil
	}
	final_html, err := document.Html()
	if err != nil {
		return fmt.Errorf("weibo postprocess: render HTML: %w", err)
	}
	if err := os.WriteFile(html_resource.FilePath, []byte(final_html), 0644); err != nil {
		return fmt.Errorf("weibo postprocess: write HTML: %w", err)
	}
	html_resource.Kind = "text/html"
	html_resource.Size = int64(len(final_html))
	return nil
}

func render_detail_html(result *FetchResult) string {
	if result == nil {
		return ""
	}
	title := post_title(result.BodyText, result.AuthorName)
	body_html := strings.TrimSpace(result.BodyHTML)
	if body_html == "" {
		body_html = "<p>" + html.EscapeString(result.BodyText) + "</p>"
	}
	var document strings.Builder
	document.WriteString(`<!DOCTYPE html><html lang="zh-CN"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`)
	document.WriteString(html.EscapeString(title))
	document.WriteString(`</title><style>body{max-width:720px;margin:0 auto;padding:32px 20px;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;line-height:1.7;color:#1f2329}h1{font-size:1.65rem;line-height:1.35}.meta{display:flex;flex-wrap:wrap;gap:8px 16px;color:#646a73;font-size:.9rem;margin-bottom:24px}.content{font-size:1.05rem}.video{margin-top:24px}.video video{display:block;width:100%;max-height:80vh;background:#000;border-radius:8px}.images{display:grid;gap:16px;margin-top:24px}.images figure{margin:0}.images img{display:block;width:100%;height:auto;border-radius:8px}footer{margin-top:28px;padding-top:16px;border-top:1px solid #e5e6eb;color:#646a73;font-size:.9rem}a{color:#1677ff;text-decoration:none}</style></head><body><article data-n="weibo-detail"><h1 data-n="weibo-title">`)
	document.WriteString(html.EscapeString(title))
	document.WriteString(`</h1><div class="meta" data-n="weibo-meta"><a href="https://weibo.com/u/`)
	document.WriteString(html.EscapeString(result.AuthorID))
	document.WriteString(`">`)
	document.WriteString(html.EscapeString(result.AuthorName))
	document.WriteString(`</a>`)
	if result.PublishTime != nil {
		publish_time := time.UnixMilli(*result.PublishTime).In(time.FixedZone("Asia/Shanghai", 8*60*60))
		document.WriteString(`<time datetime="`)
		document.WriteString(publish_time.Format(time.RFC3339))
		document.WriteString(`">`)
		document.WriteString(publish_time.Format("2006-01-02 15:04"))
		document.WriteString(`</time>`)
	}
	if result.Region != "" {
		document.WriteString(`<span>发布于 ` + html.EscapeString(result.Region) + `</span>`)
	}
	if result.Client != "" {
		document.WriteString(`<span>来自 ` + html.EscapeString(result.Client) + `</span>`)
	}
	document.WriteString(`</div><section class="content" data-n="weibo-body">`)
	document.WriteString(body_html)
	document.WriteString(`</section>`)
	if result.Video != nil && result.Video.URL != "" {
		document.WriteString(`<section class="video" data-n="weibo-video"><video controls preload="metadata"`)
		if result.Video.CoverURL != "" {
			document.WriteString(` poster="`)
			document.WriteString(html.EscapeString(result.Video.CoverURL))
			document.WriteString(`"`)
		}
		document.WriteString(` src="`)
		document.WriteString(html.EscapeString(result.Video.URL))
		document.WriteString(`"><a href="`)
		document.WriteString(html.EscapeString(result.Video.URL))
		document.WriteString(`">下载视频</a></video></section>`)
	}
	if len(result.Images) > 0 {
		document.WriteString(`<section class="images" data-n="weibo-images">`)
		for image_index, image := range result.Images {
			document.WriteString(`<figure><img src="`)
			document.WriteString(html.EscapeString(image.URL))
			document.WriteString(`" alt="微博图片 `)
			document.WriteString(fmt.Sprint(image_index + 1))
			document.WriteString(`"></figure>`)
		}
		document.WriteString(`</section>`)
	}
	document.WriteString(`<footer data-n="weibo-footer"><span>`)
	fmt.Fprintf(&document, "转发 %d · 评论 %d · 赞 %d", result.ShareCount, result.CommentCount, result.LikeCount)
	document.WriteString(`</span> · <a href="`)
	document.WriteString(html.EscapeString(result.SourceURL))
	document.WriteString(`">查看原微博</a></footer></article></body></html>`)
	return document.String()
}

func image_data_uri(resource *hermes.ResourceJob) string {
	if resource == nil || strings.TrimSpace(resource.FilePath) == "" {
		return ""
	}
	image_data, err := os.ReadFile(resource.FilePath)
	if err != nil || len(image_data) == 0 {
		return ""
	}
	mime_type := http.DetectContentType(image_data)
	if !strings.HasPrefix(mime_type, "image/") {
		declared_type := strings.ToLower(strings.TrimSpace(resource.Kind))
		if mime_type != "application/octet-stream" || !strings.HasPrefix(declared_type, "image/") {
			return ""
		}
		mime_type = declared_type
	}
	return "data:" + mime_type + ";base64," + base64.StdEncoding.EncodeToString(image_data)
}

func image_mime_type(extension string) string {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(extension), ".")) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	default:
		return "image"
	}
}
