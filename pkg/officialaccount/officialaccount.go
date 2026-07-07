package officialaccount

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var md_convert = htmltomarkdown.NewConverter("", true, nil)

type OfficialAccountDownload struct {
	article    *WechatOfficialArticle
	OnProgress func(downloaded int64) // callback after each image download, reports bytes downloaded
}

func (c *OfficialAccountDownload) reportProgress(n int64) {
	if c.OnProgress != nil {
		c.OnProgress(n)
	}
}

func (c *OfficialAccountDownload) SaveURLAsMarkdown(url string, dir_path string) error {
	article, err := c.FetchArticle(url)
	if err != nil {
		return err
	}
	return c.ConvertHtmlToMarkdown(article, dir_path)
}

func (c *OfficialAccountDownload) BuildHTMLFromURL(url string, need_compress_img bool) (string, error) {
	article := c.article
	if article == nil {
		r, err := c.FetchArticle(url)
		if err != nil {
			return "", err
		}
		article = r
	}
	return c.BuildHTMLFromArticle(article, need_compress_img)
}

func (c *OfficialAccountDownload) ConvertHtmlToMarkdown(article *WechatOfficialArticle, dir_path string) error {
	// Update the receiver with the fetched article data
	// Sanitize filename for the markdown file
	filename := strings.ReplaceAll(article.Title, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Create images directory
	imagesDirName := "images"
	imagesDirPath := filepath.Join(dir_path, imagesDirName)
	if err := os.MkdirAll(imagesDirPath, 0755); err != nil {
		return err
	}

	// Process HTML content to download images and replace links
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(article.Content))
	if err != nil {
		return err
	}

	// Preserve newlines in text nodes by replacing them with a placeholder
	// This is needed because HTML parsers and markdown converters often treat newlines as whitespace
	newlinePlaceholder := "WECHATNEWLINEHOLDER"
	var replaceNewlines func(*html.Node)
	replaceNewlines = func(n *html.Node) {
		if n.Type == html.TextNode {
			if strings.Contains(n.Data, "\n") {
				n.Data = strings.ReplaceAll(n.Data, "\n", newlinePlaceholder)
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
			replaceNewlines(c)
		}
	}

	for _, n := range doc.Nodes {
		replaceNewlines(n)
	}

	doc.Find("mp-common-mpaudio").Each(func(i int, s *goquery.Selection) {
		voiceEncodeFileId := s.AttrOr("voice_encode_fileid", "")
		if voiceEncodeFileId != "" {
			audioURL := "https://res.wx.qq.com/voice/getvoice?mediaid=" + voiceEncodeFileId
			s.AppendHtml(fmt.Sprintf(`<audio src="%s" controls="controls"></audio>`, audioURL))
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
			for _, video := range article.Videos {
				if video.VideoID == vid {
					if len(video.MpVideoTransInfo) > 0 {
						videoURL := video.MpVideoTransInfo[0].Url
						cover := s.AttrOr("data-cover", "")
						posterAttr := ""
						if cover != "" {
							if decodedCover, err := url.QueryUnescape(cover); err == nil {
								cover = decodedCover
							}
							posterAttr = fmt.Sprintf(` poster="%s"`, escapeHTML(cover))
						}
						videoHTML := fmt.Sprintf(`<video src="%s"%s controls="controls" style="width: 100%%; height: auto;"></video>`, videoURL, posterAttr)
						s.ReplaceWithHtml(videoHTML)
					}
					break
				}
			}
		}
	})

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		imgURL := s.AttrOr("data-src", "")
		if imgURL == "" {
			imgURL = s.AttrOr("src", "")
		}
		imgURL = normalizeMediaURL(imgURL)

		if imgURL != "" {
			// Download image
			localFileName, err := c.downloadImage(imgURL, imagesDirPath)
			if err == nil {
				// Replace src with local relative path
				relativePath := filepath.Join(imagesDirName, localFileName)
				s.SetAttr("src", relativePath)
				// Remove data-src to ensure markdown converter uses src
				s.RemoveAttr("data-src")
			} else {
				fmt.Printf("Failed to download image %s: %v\n", imgURL, err)
			}
		}
	})

	newHTML, err := doc.Html()
	if err != nil {
		return err
	}

	// Workaround for <br> handling: Replace <br> with a placeholder to ensure it's preserved as a hard break
	// html-to-markdown/v2 might handle <br> differently depending on context or configuration.
	// We want explicit hard breaks (two spaces + newline) for every <br> tag.
	brPlaceholder := "WECHATBRHOLDER"
	// Replace the newline placeholder with the break placeholder
	newHTML = strings.ReplaceAll(newHTML, "WECHATNEWLINEHOLDER", brPlaceholder)

	// goquery normalizes to <br/> but we handle all cases just to be safe
	newHTML = strings.ReplaceAll(newHTML, "<br/>", brPlaceholder)
	newHTML = strings.ReplaceAll(newHTML, "<br>", brPlaceholder)
	newHTML = strings.ReplaceAll(newHTML, "<br />", brPlaceholder)

	markdown, err := md_convert.ConvertString(newHTML)
	if err != nil {
		return err
	}

	// Restore line breaks
	markdown = strings.ReplaceAll(markdown, brPlaceholder, "  \n")

	// Process additional images from article.Images
	if len(article.Images) > 0 {
		markdown += "\n\n"
		for _, imgURL := range article.Images {
			imgURL = normalizeMediaURL(imgURL)
			localFileName, err := c.downloadImage(imgURL, imagesDirPath)
			if err != nil {
				fmt.Printf("Failed to download attached image %s: %v\n", imgURL, err)
				continue
			}
			relative_path := filepath.Join(imagesDirName, localFileName)
			markdown += fmt.Sprintf("\n![image](%s)\n", relative_path)
		}
	}

	if err := os.MkdirAll(dir_path, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dir_path, filename+".md")

	if err := os.WriteFile(filePath, []byte(markdown), 0644); err != nil {
		return err
	}

	return nil
}

func (c *OfficialAccountDownload) BuildHTMLFromArticle(article *WechatOfficialArticle, need_compress_img bool) (string, error) {
	isImageArticle := article.Type == 2 && len(article.Images) > 0
	bodyMaxWidth := "677px"
	if isImageArticle {
		bodyMaxWidth = "1024px"
	}

	var htmlContent strings.Builder
	htmlContent.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>`)
	htmlContent.WriteString(escapeHTML(article.Title))
	htmlContent.WriteString(`</title>
    <style>
        html { height: 100%; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            max-width: ` + bodyMaxWidth + `;
            margin: 0 auto;
            padding: 20px;
            color: #333;`)
	if isImageArticle {
		htmlContent.WriteString(`
            height: 100%;
            overflow: hidden;
            box-sizing: border-box;`)
	}
	htmlContent.WriteString(`
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

	if isImageArticle {
		htmlContent.WriteString(`<div class="split-container"><div class="split-left"><div class="additional-images">`)
		for _, imgURL := range article.Images {
			imgURL = normalizeMediaURL(imgURL)
			imgData, mimeType, err := downloadImageBytes(imgURL)
			if err == nil {
				c.reportProgress(int64(len(imgData)))
				if need_compress_img {
					// Compress image to reduce size
					compressedData, compressedMime, errCompress := compressImage(imgData)
					if errCompress == nil {
						fmt.Printf("Compressed image %s: %d -> %d bytes (%.2f%%)\n",
							imgURL, len(imgData), len(compressedData), float64(len(compressedData))/float64(len(imgData))*100)
						imgData = compressedData
						mimeType = compressedMime
					} else {
						fmt.Printf("Failed to compress image %s: %v\n", imgURL, errCompress)
					}
				}
				base64Str := base64.StdEncoding.EncodeToString(imgData)
				imgSrc := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str)
				htmlContent.WriteString(fmt.Sprintf("        <img src=\"%s\" alt=\"\">\n", imgSrc))
			} else {
				fmt.Printf("Failed to download image for base64 %s: %v\n", imgURL, err)
			}
		}
		htmlContent.WriteString(`</div></div><div class="split-right">`)
	}

	htmlContent.WriteString(`<h1 class="rich_media_title"><span>` + article.Title + "</span></h1>")
	creator_html := ""
	if article.Creator != "" {
		creator_html = `<span class="rich_media_meta rich_media_meta_text">` + article.Creator + `</span>`
	}
	htmlContent.WriteString(`<div class="rich_media_meta_list">` + creator_html + `<span class="rich_media_meta rich_media_meta_nickname">` + article.AuthorNickname + `</span><span><em class="rich_media_meta rich_media_meta_text">` + article.PublishTimeStr + "</em></span></div>")
	htmlContent.WriteString(`<div class="rich_media_content js_underline_content autoTypeSetting24psection fix_apple_default_style">`)
	// Process HTML content to handle newlines
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(article.Content))
	if err != nil {
		htmlContent.WriteString(article.Content)
	} else {
		newlinePlaceholder := "WECHATNEWLINEHOLDER"
		var replaceNewlines func(*html.Node)
		replaceNewlines = func(n *html.Node) {
			if n.Type == html.TextNode {
				if strings.Contains(n.Data, "\n") {
					n.Data = strings.ReplaceAll(n.Data, "\n", newlinePlaceholder)
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
				replaceNewlines(c)
			}
		}

		for _, n := range doc.Nodes {
			replaceNewlines(n)
		}

		doc.Find("mp-common-mpaudio").Each(func(i int, s *goquery.Selection) {
			voiceEncodeFileId := s.AttrOr("voice_encode_fileid", "")
			if voiceEncodeFileId != "" {
				audioURL := "https://res.wx.qq.com/voice/getvoice?mediaid=" + voiceEncodeFileId
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
				</div>`, escapeHTML(poster), escapeHTML(name), audioURL)

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
				for _, video := range article.Videos {
					if video.VideoID == vid {
						if len(video.MpVideoTransInfo) > 0 {
							videoURL := video.MpVideoTransInfo[0].Url
							cover := s.AttrOr("data-cover", "")
							posterAttr := ""
							if cover != "" {
								if decodedCover, err := url.QueryUnescape(cover); err == nil {
									cover = decodedCover
								}
								posterAttr = fmt.Sprintf(` poster="%s"`, escapeHTML(cover))
							}
							videoHTML := fmt.Sprintf(`<video src="%s"%s controls="controls" style="width: 100%%; height: auto;"></video>`, videoURL, posterAttr)
							s.ReplaceWithHtml(videoHTML)
						}
						break
					}
				}
			}
		})

		// Process images with data-src for base64 encoding
		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			imgURL := s.AttrOr("data-src", "")
			imgURL = normalizeMediaURL(imgURL)
			if imgURL != "" {
				imgData, mimeType, err := downloadImageBytes(imgURL)
				if err == nil {
					c.reportProgress(int64(len(imgData)))
					if need_compress_img {
						// Compress image to reduce size
						compressedData, compressedMime, errCompress := compressImage(imgData)
						if errCompress == nil {
							fmt.Printf("Compressed image %s: %d -> %d bytes (%.2f%%)\n",
								imgURL, len(imgData), len(compressedData), float64(len(compressedData))/float64(len(imgData))*100)
							imgData = compressedData
							mimeType = compressedMime
						} else {
							fmt.Printf("Failed to compress image %s: %v\n", imgURL, errCompress)
						}
					}
					base64Str := base64.StdEncoding.EncodeToString(imgData)
					imgSrc := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Str)
					s.SetAttr("src", imgSrc)
					s.RemoveAttr("data-src")
				} else {
					fmt.Printf("Failed to download image for base64 %s: %v\n", imgURL, err)
				}
			}
		})

		// Get the content inside <body>
		newHTML, err := doc.Find("body").Html()
		if err != nil {
			htmlContent.WriteString(article.Content)
		} else {
			newHTML = strings.ReplaceAll(newHTML, newlinePlaceholder, "<br>")
			htmlContent.WriteString(newHTML)
		}
	}

	if isImageArticle {
		htmlContent.WriteString("    </div></div>")
	}

	htmlContent.WriteString(`</body>
</html>`)

	return htmlContent.String(), nil
}

func (c *OfficialAccountDownload) downloadImage(imgURL string, save_dir string) (string, error) {
	imgURL = normalizeMediaURL(imgURL)
	// Generate filename based on hash of URL
	hash := md5.Sum([]byte(imgURL))
	hashStr := hex.EncodeToString(hash[:])

	// Try to guess extension
	ext := ".jpg" // Default
	if strings.Contains(imgURL, "wx_fmt=png") {
		ext = ".png"
	} else if strings.Contains(imgURL, "wx_fmt=gif") {
		ext = ".gif"
	} else if strings.Contains(imgURL, "wx_fmt=jpeg") {
		ext = ".jpg"
	} else if strings.Contains(imgURL, "wx_fmt=webp") {
		ext = ".webp"
	} else {
		// Try to parse from URL path if query param not present
		u, err := url.Parse(imgURL)
		if err == nil {
			pathExt := filepath.Ext(u.Path)
			if pathExt != "" {
				ext = pathExt
			}
		}
	}

	filename := hashStr + ext
	filePath := filepath.Join(save_dir, filename)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return filename, nil
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return "", err
	}

	setWechatHeaders(req, "https://mp.weixin.qq.com/")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	file, err := os.Create(filePath)
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

func (c *OfficialAccountDownload) FetchArticle(url string) (*WechatOfficialArticle, error) {
	content, err := c.Scrape(url)
	if err != nil {
		return nil, err
	}
	content_str := string(content)
	// Extract createTime
	var publish_time_str string
	re := regexp.MustCompile(`var\s+createTime\s*=\s*'([^']+)'`)
	matches := re.FindStringSubmatch(content_str)
	if len(matches) > 1 {
		publish_time_str = formatPublishTime(matches[1], 0)
	}
	data, err := parse_cgi_datanew(content_str)
	if err != nil {
		article, fallbackErr := parseArticleFromDOM(content_str)
		if fallbackErr != nil {
			return nil, err
		}
		c.article = article
		return article, nil
	}
	if publish_time_str == "" {
		publish_time_str = formatPublishTime(data.CreateTime, data.OriCreateTime)
	}
	article := &WechatOfficialArticle{
		Type:           data.PageType,
		Title:          data.Title,
		Content:        data.ContentNoEncode,
		PublishTimeStr: publish_time_str,
		ContentLength:  len(data.ContentNoEncode),
		Creator:        data.Author,
		AuthorNickname: data.NickName,
		AuthorAvatar:   data.RoundHeadImg,
		AuthorID:       data.UserName,
		Images:         make([]string, 0),
		Videos:         data.VideoPageInfos,
	}
	if article.Creator == "" {
		article.Creator = data.NickName
	}
	// isImageArticle := data.PageType == 2
	if len(data.PicturePageInfoList) > 1 {
		for _, img := range data.PicturePageInfoList {
			if img.CdnUrl != "" {
				article.Images = append(article.Images, normalizeMediaURL(img.CdnUrl))
			}
		}
	}
	c.article = article
	return article, nil
}

func (c *OfficialAccountDownload) Scrape(url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("url is empty")
	}
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	setWechatHeaders(req, "")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if isVerificationPage(string(body)) {
		return nil, fmt.Errorf("wechat verification page returned for %s", url)
	}
	return body, err
}

// ExtractArticleID extracts a unique article identifier from a WeChat official account URL.
// For short URLs like https://mp.weixin.qq.com/s/2kaR8z-xO_IAO9TPSUecsQ, returns the path suffix.
// For full URLs, returns mid+idx as the unique key.
// The rawURL may have an "officialaccount://" prefix.
func ExtractArticleID(rawURL string) string {
	u := rawURL
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "officialaccount://") {
		u = u[len("officialaccount://"):]
		if !strings.HasPrefix(u, "http") {
			u = "https://" + u
		}
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}

	if !strings.Contains(parsed.Host, "mp.weixin.qq.com") {
		return ""
	}

	// Short URL: /s/2kaR8z-xO_IAO9TPSUecsQ
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasPrefix(path, "/s/") {
		shortID := strings.TrimPrefix(path, "/s/")
		if shortID != "" && !strings.Contains(shortID, "/") {
			return shortID
		}
	}

	// Full URL: extract mid+idx
	q := parsed.Query()
	mid := q.Get("mid")
	idx := q.Get("idx")
	if mid != "" {
		if idx == "" {
			idx = "1"
		}
		return mid + "_" + idx
	}

	return ""
}
