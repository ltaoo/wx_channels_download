package wxmpadapter

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	stdhtml "html"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/image/draw"
	"golang.org/x/net/html"

	"wx_channel/pkg/scraper/wxmp"
)

var md_convert = htmltomarkdown.NewConverter("", true, nil)

const postprocess_wechat_user_agent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN"

type article_postprocessor struct {
	on_progress func(downloaded int64)
}

func (c *article_postprocessor) report_progress(n int64) {
	if c.on_progress != nil {
		c.on_progress(n)
	}
}

func (c *article_postprocessor) convert_html_to_markdown(article *wxmp.ArticleCgiDataNew, dir_path string) error {
	// Update the receiver with the fetched article data
	// Sanitize filename for the markdown file
	filename := strings.ReplaceAll(article.Title, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Create images directory
	images_dir_name := "images"
	images_dir_path := filepath.Join(dir_path, images_dir_name)
	if err := os.MkdirAll(images_dir_path, 0755); err != nil {
		return err
	}

	// Process HTML content to download images and replace links
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(article.ContentNoencode))
	if err != nil {
		return err
	}

	// Preserve newlines in text nodes by replacing them with a placeholder
	// This is needed because HTML parsers and markdown converters often treat newlines as whitespace
	newline_placeholder := "WECHATNEWLINEHOLDER"
	var replace_newlines func(*html.Node)
	replace_newlines = func(n *html.Node) {
		if n.Type == html.TextNode {
			if strings.Contains(n.Data, "\n") {
				n.Data = strings.ReplaceAll(n.Data, "\n", newline_placeholder)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				tag := strings.ToLower(c.Data)
				// Skip pre-formatted blocks where newlines should be preserved naturally
				if tag == "pre" || tag == "code" || tag == "script" || tag == "style" {
					continue
				}
			}
			replace_newlines(c)
		}
	}

	for _, n := range doc.Nodes {
		replace_newlines(n)
	}

	doc.Find("mp-common-mpaudio").Each(func(i int, s *goquery.Selection) {
		voice_encode_file_id := s.AttrOr("voice_encode_fileid", "")
		if voice_encode_file_id != "" {
			audio_url := "https://res.wx.qq.com/voice/getvoice?mediaid=" + voice_encode_file_id
			s.AppendHtml(fmt.Sprintf(`<audio src="%s" controls="controls"></audio>`, audio_url))
		}
	})

	doc.Find("iframe.video_iframe").Each(func(i int, s *goquery.Selection) {
		vid := s.AttrOr("data-vid", "")
		if vid == "" {
			vid = s.AttrOr("vid", "")
		}
		if vid == "" {
			vid = s.AttrOr("data-mpvid", "")
		}
		if vid != "" {
			for _, video := range article.VideoPageInfos {
				if video.VideoID == vid {
					if len(video.MpVideoTransInfo) > 0 {
						video_url := video.MpVideoTransInfo[0].Url
						cover := s.AttrOr("data-cover", "")
						poster_attr := ""
						if cover != "" {
							if decoded_cover, err := url.QueryUnescape(cover); err == nil {
								cover = decoded_cover
							}
							poster_attr = fmt.Sprintf(` poster="%s"`, escape_html(cover))
						}
						video_html := fmt.Sprintf(`<video src="%s"%s controls="controls" style="width: 100%%; height: auto;"></video>`, video_url, poster_attr)
						s.ReplaceWithHtml(video_html)
					}
					break
				}
			}
		}
	})

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		image_url := s.AttrOr("data-src", "")
		if image_url == "" {
			image_url = s.AttrOr("src", "")
		}
		image_url = normalize_media_url(image_url)

		if image_url != "" {
			// Download image
			local_file_name, err := c.download_image(image_url, images_dir_path)
			if err == nil {
				// Replace src with local relative path
				relative_path := filepath.Join(images_dir_name, local_file_name)
				s.SetAttr("src", relative_path)
				// Remove data-src to ensure markdown converter uses src
				s.RemoveAttr("data-src")
			} else {
				fmt.Printf("Failed to download image %s: %v\n", image_url, err)
			}
		}
	})

	new_html, err := doc.Html()
	if err != nil {
		return err
	}

	// Workaround for <br> handling: Replace <br> with a placeholder to ensure it's preserved as a hard break
	// html-to-markdown/v2 might handle <br> differently depending on context or configuration.
	// We want explicit hard breaks (two spaces + newline) for every <br> tag.
	br_placeholder := "WECHATBRHOLDER"
	// Replace the newline placeholder with the break placeholder
	new_html = strings.ReplaceAll(new_html, "WECHATNEWLINEHOLDER", br_placeholder)

	// goquery normalizes to <br/> but we handle all cases just to be safe
	new_html = strings.ReplaceAll(new_html, "<br/>", br_placeholder)
	new_html = strings.ReplaceAll(new_html, "<br>", br_placeholder)
	new_html = strings.ReplaceAll(new_html, "<br />", br_placeholder)

	markdown, err := md_convert.ConvertString(new_html)
	if err != nil {
		return err
	}

	// Restore line breaks
	markdown = strings.ReplaceAll(markdown, br_placeholder, "  \n")

	// Process additional images from the article picture metadata.
	if len(article.PicturePageInfoList) > 0 {
		markdown += "\n\n"
		for _, picture := range article.PicturePageInfoList {
			image_url := picture.CdnUrl
			image_url = normalize_media_url(image_url)
			if image_url == "" {
				continue
			}
			local_file_name, err := c.download_image(image_url, images_dir_path)
			if err != nil {
				fmt.Printf("Failed to download attached image %s: %v\n", image_url, err)
				continue
			}
			relative_path := filepath.Join(images_dir_name, local_file_name)
			markdown += fmt.Sprintf("\n![image](%s)\n", relative_path)
		}
	}

	if err := os.MkdirAll(dir_path, 0755); err != nil {
		return err
	}

	file_path := filepath.Join(dir_path, filename+".md")

	if err := os.WriteFile(file_path, []byte(markdown), 0644); err != nil {
		return err
	}

	return nil
}

func (c *article_postprocessor) build_html_from_article(article *wxmp.ArticleCgiDataNew, need_compress_img bool) (string, error) {
	is_image_article := article.PageType == 2 && len(article.PicturePageInfoList) > 0
	body_max_width := "677px"
	if is_image_article {
		body_max_width = "1024px"
	}

	var html_content strings.Builder
	html_content.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>`)
	html_content.WriteString(escape_html(article.Title))
	html_content.WriteString(`</title>
    <style>
        html { height: 100%; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            max-width: ` + body_max_width + `;
            margin: 0 auto;
            padding: 20px;
            color: #333;`)
	if is_image_article {
		html_content.WriteString(`
            height: 100%;
            overflow: hidden;
            box-sizing: border-box;`)
	}
	html_content.WriteString(`
        }
        h1 { font-size: 1.8em; margin-bottom: 0.5em; }
        .author { color: #666; margin-bottom: 20px; }
        .author img { width: 24px; height: 24px; border-radius: 50%; vertical-align: middle; margin-right: 8px; }
        img { max-width: 100%; height: auto; }
	.rich_media_title {
		font-size: 22px;
		line-height: 1.4;
		margin-bottom: 14px;
		font-weight: 500;
	}
	.not_in_mm .rich_media_meta_list {
		position: relative;
		z-index: 1;
	}
	.rich_media_meta_list {
		margin-bottom: 22px;
		line-height: 20px;
		font-size: 0;
		word-wrap: break-word;
		-webkit-hyphens: auto;
		-ms-hyphens: auto;
		hyphens: auto;
	}
	.rich_media_meta {
		display: inline-block;
		vertical-align: middle;
		margin: 0 10px 10px 0;
		font-size: 15px;
		-webkit-tap-highlight-color: rgba(0, 0, 0, 0);
	}
	.rich_media_meta_avatar {
		display: inline-block;
		width: 24px;
		height: 24px;
		border-radius: 50%;
		object-fit: cover;
		vertical-align: middle;
		margin: 0 8px 10px 0;
	}
	.rich_media_meta_text.article_modify_tag, .rich_media_meta_nickname {
		position: relative;
	}
	.rich_media_meta_list em {
		font-style: normal;
	}
	.audio_card {
		display: flex;
		align-items: center;
		background-color: #f7f7f7;
		border-radius: 6px;
		padding: 12px;
		margin: 20px 0;
		border: 1px solid #ebebeb;
	}
	.audio_card_cover {
		width: 64px;
		height: 64px;
		border-radius: 4px;
		overflow: hidden;
		flex-shrink: 0;
		margin-right: 12px;
		position: relative;
	}
	.audio_card_cover img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	.audio_card_content {
		flex-grow: 1;
		overflow: hidden;
		margin-right: 12px;
	}
	.audio_card_title {
		font-size: 16px;
		font-weight: 500;
		color: #333;
		margin-bottom: 4px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.audio_card_meta {
		font-size: 13px;
		color: #999;
	}
	.audio_card audio {
		height: 32px;
	}
	.additional-images {
		margin-top: 0;
		padding-top: 0;
		border-top: none;
	}
	.additional-images img {
		display: block;
		width: 100%;
		height: auto;
		margin-bottom: 20px;
		border-radius: 6px;
		box-shadow: 0 2px 6px rgba(0,0,0,0.05);
	}
    /* Split layout styles */
    .split-container {
        display: flex;
        gap: 40px;
        align-items: flex-start;
        justify-content: center;
        height: 100%;
    }
    .split-left {
        width: 600px;
        flex: 0 0 600px;
        height: 100%;
        overflow-y: auto;
        scrollbar-width: thin;
    }
    .split-right {
        width: 344px;
        flex: 0 0 344px;
        height: 100%;
        overflow-y: auto;
        scrollbar-width: thin;
    }
    @media (max-width: 1000px) {
        html, body {
            height: auto !important;
            overflow: visible !important;
        }
        .split-container {
            display: block;
            height: auto;
        }
        .split-left, .split-right {
            width: 100%;
            flex: none;
            height: auto;
            overflow-y: visible;
        }
        .split-left {
            margin-bottom: 20px;
        }
    }
    </style>
</head>
<body>`)

	if is_image_article {
		html_content.WriteString(`<div class="split-container"><div class="split-left"><div class="additional-images">`)
		c.write_picture_article_media(&html_content, article, need_compress_img)
		html_content.WriteString(`</div></div><div class="split-right">`)
	}

	html_content.WriteString(`<h1 class="rich_media_title"><span>` + article.Title + "</span></h1>")
	creator_html := ""
	if article.Author != "" {
		creator_html = `<span class="rich_media_meta rich_media_meta_text">` + article.Author + `</span>`
	}
	avatar_html := ""
	avatar_url := article_avatar_url(article)
	if avatar_url != "" {
		if image_data, mime_type, err := download_image_bytes(avatar_url); err == nil {
			c.report_progress(int64(len(image_data)))
			avatar_html = `<img class="rich_media_meta_avatar" src="data:` + mime_type + `;base64,` + base64.StdEncoding.EncodeToString(image_data) + `" alt="` + escape_html(article.NickName) + `">`
		}
	}
	html_content.WriteString(`<div class="rich_media_meta_list">` + avatar_html + creator_html + `<span class="rich_media_meta rich_media_meta_nickname">` + article.NickName + `</span><span><em class="rich_media_meta rich_media_meta_text">` + article_publish_time_text(article) + "</em></span></div>")
	html_content.WriteString(`<div class="rich_media_content js_underline_content autoTypeSetting24psection fix_apple_default_style">`)
	// Process HTML content to handle newlines
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(article.ContentNoencode))
	if err != nil {
		html_content.WriteString(article.ContentNoencode)
	} else {
		newline_placeholder := "WECHATNEWLINEHOLDER"
		var replace_newlines func(*html.Node)
		replace_newlines = func(n *html.Node) {
			if n.Type == html.TextNode {
				if strings.Contains(n.Data, "\n") {
					n.Data = strings.ReplaceAll(n.Data, "\n", newline_placeholder)
				}
				return
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode {
					tag := strings.ToLower(c.Data)
					// Skip pre-formatted blocks
					if tag == "pre" || tag == "code" || tag == "script" || tag == "style" {
						continue
					}
				}
				replace_newlines(c)
			}
		}

		for _, n := range doc.Nodes {
			replace_newlines(n)
		}

		doc.Find("mp-common-mpaudio").Each(func(i int, s *goquery.Selection) {
			voice_encode_file_id := s.AttrOr("voice_encode_fileid", "")
			if voice_encode_file_id != "" {
				audio_url := "https://res.wx.qq.com/voice/getvoice?mediaid=" + voice_encode_file_id
				name := s.AttrOr("name", "音频")
				poster := s.AttrOr("poster", "")
				if poster == "" {
					poster = s.AttrOr("cover", "")
				}

				html := fmt.Sprintf(`
				<div class="audio_card">
					<div class="audio_card_cover">
						<img src="%s" alt="cover">
					</div>
					<div class="audio_card_content">
						<div class="audio_card_title">%s</div>
						<audio src="%s" controls></audio>
					</div>
				</div>`, escape_html(poster), escape_html(name), audio_url)

				s.ReplaceWithHtml(html)
			}
		})

		doc.Find("iframe.video_iframe").Each(func(i int, s *goquery.Selection) {
			vid := s.AttrOr("data-vid", "")
			if vid == "" {
				vid = s.AttrOr("vid", "")
			}
			if vid == "" {
				vid = s.AttrOr("data-mpvid", "")
			}
			if vid != "" {
				for _, video := range article.VideoPageInfos {
					if video.VideoID == vid {
						if len(video.MpVideoTransInfo) > 0 {
							video_url := video.MpVideoTransInfo[0].Url
							cover := s.AttrOr("data-cover", "")
							poster_attr := ""
							if cover != "" {
								if decoded_cover, err := url.QueryUnescape(cover); err == nil {
									cover = decoded_cover
								}
								poster_attr = fmt.Sprintf(` poster="%s"`, escape_html(cover))
							}
							video_html := fmt.Sprintf(`<video src="%s"%s controls="controls" style="width: 100%%; height: auto;"></video>`, video_url, poster_attr)
							s.ReplaceWithHtml(video_html)
						}
						break
					}
				}
			}
		})

		// Process images with data-src for base64 encoding
		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			image_url := s.AttrOr("data-src", "")
			image_url = normalize_media_url(image_url)
			if image_url != "" {
				image_data, mime_type, err := download_image_bytes(image_url)
				if err == nil {
					c.report_progress(int64(len(image_data)))
					if need_compress_img {
						// Compress image to reduce size
						compressed_data, compressed_mime, compress_err := compress_image(image_data)
						if compress_err == nil {
							fmt.Printf("Compressed image %s: %d -> %d bytes (%.2f%%)\n",
								image_url, len(image_data), len(compressed_data), float64(len(compressed_data))/float64(len(image_data))*100)
							image_data = compressed_data
							mime_type = compressed_mime
						} else {
							fmt.Printf("Failed to compress image %s: %v\n", image_url, compress_err)
						}
					}
					base64_str := base64.StdEncoding.EncodeToString(image_data)
					image_src := fmt.Sprintf("data:%s;base64,%s", mime_type, base64_str)
					s.SetAttr("src", image_src)
					s.RemoveAttr("data-src")
				} else {
					fmt.Printf("Failed to download image for base64 %s: %v\n", image_url, err)
				}
			}
		})

		// Get the content inside <body>
		new_html, err := doc.Find("body").Html()
		if err != nil {
			html_content.WriteString(article.ContentNoencode)
		} else {
			new_html = strings.ReplaceAll(new_html, newline_placeholder, "<br>")
			html_content.WriteString(new_html)
		}
	}

	if is_image_article {
		html_content.WriteString("    </div></div>")
	}

	html_content.WriteString(`</body>
</html>`)

	return html_content.String(), nil
}

func (c *article_postprocessor) write_picture_article_media(html_content *strings.Builder, article *wxmp.ArticleCgiDataNew, need_compress_img bool) {
	if html_content == nil || article == nil {
		return
	}
	if len(article.PicturePageInfoList) > 0 {
		for _, item := range article.PicturePageInfoList {
			if video_url := picture_page_info_live_photo_url(item); video_url != "" {
				poster_attr := ""
				if item.CdnUrl != "" {
					poster_attr = fmt.Sprintf(` poster="%s"`, escape_html(item.CdnUrl))
				}
				html_content.WriteString(fmt.Sprintf(`        <video src="%s"%s controls="controls" playsinline style="width: 100%%; height: auto;"></video>`+"\n", escape_html(video_url), poster_attr))
				continue
			}
			c.write_inline_image(html_content, item.CdnUrl, need_compress_img)
		}
		return
	}
}

func article_publish_time_text(article *wxmp.ArticleCgiDataNew) string {
	if article == nil {
		return ""
	}
	if create_time := strings.TrimSpace(article.CreateTime); create_time != "" {
		return create_time
	}
	timestamp := int64(article.OriCreateTime)
	if timestamp <= 0 {
		timestamp = int64(article.CreateTimestamp)
		if timestamp > 1_000_000_000_000 {
			timestamp /= 1000
		}
	}
	if timestamp <= 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Format("2006年01月02日 15:04")
}

func (c *article_postprocessor) write_inline_image(html_content *strings.Builder, image_url string, need_compress_img bool) {
	if html_content == nil || image_url == "" {
		return
	}
	image_data, mime_type, err := download_image_bytes(image_url)
	if err == nil {
		c.report_progress(int64(len(image_data)))
		if need_compress_img {
			compressed_data, compressed_mime, compress_err := compress_image(image_data)
			if compress_err == nil {
				fmt.Printf("Compressed image %s: %d -> %d bytes (%.2f%%)\n",
					image_url, len(image_data), len(compressed_data), float64(len(compressed_data))/float64(len(image_data))*100)
				image_data = compressed_data
				mime_type = compressed_mime
			} else {
				fmt.Printf("Failed to compress image %s: %v\n", image_url, compress_err)
			}
		}
		base64_str := base64.StdEncoding.EncodeToString(image_data)
		image_src := fmt.Sprintf("data:%s;base64,%s", mime_type, base64_str)
		html_content.WriteString(fmt.Sprintf("        <img src=\"%s\" alt=\"\">\n", image_src))
	} else {
		fmt.Printf("Failed to download image for base64 %s: %v\n", image_url, err)
	}
}

func picture_page_info_live_photo_url(item wxmp.PicturePageInfo) string {
	for _, format := range item.LivePhoto.FormatInfo {
		if video_url := strings.TrimSpace(format.URL); video_url != "" {
			return video_url
		}
	}
	return ""
}

func (c *article_postprocessor) download_image(image_url string, save_dir string) (string, error) {
	image_url = normalize_media_url(image_url)
	// Generate filename based on hash of URL
	hash := md5.Sum([]byte(image_url))
	hash_str := hex.EncodeToString(hash[:])

	// Try to guess extension
	ext := ".jpg" // Default
	if strings.Contains(image_url, "wx_fmt=png") {
		ext = ".png"
	} else if strings.Contains(image_url, "wx_fmt=gif") {
		ext = ".gif"
	} else if strings.Contains(image_url, "wx_fmt=jpeg") {
		ext = ".jpg"
	} else if strings.Contains(image_url, "wx_fmt=webp") {
		ext = ".webp"
	} else {
		// Try to parse from URL path if query param not present
		u, err := url.Parse(image_url)
		if err == nil {
			path_ext := filepath.Ext(u.Path)
			if path_ext != "" {
				ext = path_ext
			}
		}
	}

	filename := hash_str + ext
	file_path := filepath.Join(save_dir, filename)

	// Check if file already exists
	if _, err := os.Stat(file_path); err == nil {
		return filename, nil
	}

	client := &http.Client{Timeout: 25 * time.Second}
	req, err := http.NewRequest("GET", image_url, nil)
	if err != nil {
		return "", err
	}

	set_wechat_headers(req, "https://mp.weixin.qq.com/")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	file, err := os.Create(file_path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return filename, nil
}

func normalize_media_url(raw_url string) string {
	normalized_url := strings.TrimSpace(raw_url)
	if normalized_url == "" {
		return ""
	}
	normalized_url = strings.ReplaceAll(normalized_url, "&amp;amp;", "&")
	normalized_url = strings.ReplaceAll(normalized_url, "&amp;", "&")
	normalized_url = stdhtml.UnescapeString(normalized_url)
	if strings.HasPrefix(normalized_url, "//") {
		normalized_url = "https:" + normalized_url
	}
	if strings.HasPrefix(normalized_url, "http://mmbiz.qpic.cn/") {
		normalized_url = "https://" + strings.TrimPrefix(normalized_url, "http://")
	}
	return normalized_url
}

func escape_html(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, "\"", "&quot;")
	value = strings.ReplaceAll(value, "'", "&#39;")
	return value
}

func compress_image(data []byte) ([]byte, string, error) {
	decoded_image, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	bounds := decoded_image.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	max_width := 640
	var resized_image image.Image = decoded_image
	if width > max_width {
		new_height := height * max_width / width
		destination := image.NewRGBA(image.Rect(0, 0, max_width, new_height))
		draw.CatmullRom.Scale(destination, destination.Bounds(), decoded_image, bounds, draw.Over, nil)
		resized_image = destination
	}
	background := image.NewRGBA(resized_image.Bounds())
	draw.Draw(background, background.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(background, background.Bounds(), resized_image, resized_image.Bounds().Min, draw.Over)
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, background, &jpeg.Options{Quality: 60}); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), "image/jpeg", nil
}

func download_image_bytes(image_url string) ([]byte, string, error) {
	image_url = normalize_media_url(image_url)
	http_client := &http.Client{}
	request, err := http.NewRequest(http.MethodGet, image_url, nil)
	if err != nil {
		return nil, "", err
	}
	set_wechat_headers(request, "https://mp.weixin.qq.com/")
	response, err := http_client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("bad status: %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 20<<20))
	if err != nil {
		return nil, "", err
	}
	content_type := response.Header.Get("Content-Type")
	if content_type == "" {
		content_type = http.DetectContentType(data)
	}
	return data, content_type, nil
}

func set_wechat_headers(request *http.Request, referer string) {
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Pragma", "no-cache")
	request.Header.Set("Upgrade-Insecure-Requests", "1")
	request.Header.Set("User-Agent", postprocess_wechat_user_agent)
	if referer != "" {
		request.Header.Set("Referer", referer)
	}
}
