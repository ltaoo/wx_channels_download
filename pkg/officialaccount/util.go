package officialaccount

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"golang.org/x/image/draw"
)

const wechatUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN"

func setWechatHeaders(req *http.Request, referer string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", wechatUserAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

func normalizeMediaURL(raw string) string {
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

func formatPublishTime(createTime string, unixTime int) string {
	createTime = strings.TrimSpace(createTime)
	if createTime != "" {
		if t, err := time.Parse("2006-01-02 15:04", createTime); err == nil {
			return t.Format("2006年01月02日 15:04")
		}
		return createTime
	}
	if unixTime > 0 {
		return time.Unix(int64(unixTime), 0).Format("2006年01月02日 15:04")
	}
	return ""
}

func isVerificationPage(body string) bool {
	return strings.Contains(body, "环境异常") && strings.Contains(body, "完成验证后即可继续访问")
}

func parse_cgi_datanew(htmlContent string) (*CgiDataNew, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var scripts []string
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "window.cgiDataNew =") || strings.Contains(text, "var videoPageInfos =") {
			scripts = append(scripts, text)
		}
	})

	if len(scripts) == 0 {
		return nil, fmt.Errorf("cgiDataNew script not found")
	}

	vm := goja.New()

	// Mock window
	vm.RunString("var window = {};")
	// Mock document and basic DOM/jQuery environment to prevent script errors
	vm.RunString(`
		var document = { 
			getElementById: function() { return {}; }, 
			getElementsByTagName: function() { return []; },
			createElement: function() { return {}; },
			head: {}
		};
		var console = { log: function() {}, warn: function() {}, error: function() {} };
		var mockDom = {
			html: function() { return mockDom; },
			text: function() { return mockDom; },
			val: function() { return mockDom; },
			attr: function() { return mockDom; },
			find: function() { return mockDom; },
			css: function() { return mockDom; },
			addClass: function() { return mockDom; },
			removeClass: function() { return mockDom; },
			show: function() { return mockDom; },
			hide: function() { return mockDom; },
			append: function() { return mockDom; }
		};
		var $ = function() { return mockDom; };
		var jQuery = $;
	`)

	// Mock JsDecode
	// In the browser, JsDecode seems to decode strings, but the strings in the script
	// are often already just string literals. If they contain escape sequences,
	// the JS parser handles them.
	// We'll treat it as an identity function for now.
	vm.Set("JsDecode", func(call goja.FunctionCall) goja.Value {
		return call.Argument(0)
	})

	// Run the scripts
	var scriptErrs []error
	for _, script := range scripts {
		_, err = vm.RunString(script)
		if err != nil {
			scriptErrs = append(scriptErrs, err)
		}
	}

	// Extract cgiDataNew
	val := vm.Get("window").ToObject(vm).Get("cgiDataNew")
	if val == nil {
		if len(scriptErrs) > 0 {
			for _, err := range scriptErrs {
				fmt.Printf("failed to run script: %v\n", err)
			}
		}
		return nil, fmt.Errorf("window.cgiDataNew is nil")
	}

	// Convert to JSON string
	jsonStr, err := vm.RunString("JSON.stringify(window.cgiDataNew)")
	if err != nil {
		return nil, fmt.Errorf("failed to stringify cgiDataNew: %v", err)
	}

	data := &CgiDataNew{}
	if err := json.Unmarshal([]byte(jsonStr.String()), data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cgiDataNew: %v", err)
	}

	// Check if PicturePageInfoList is empty, if so check global window.picture_page_info_list
	if len(data.PicturePageInfoList) == 0 {
		valList := vm.Get("window").ToObject(vm).Get("picture_page_info_list")
		if valList != nil && !goja.IsNull(valList) && !goja.IsUndefined(valList) {
			jsonList, err := vm.RunString("JSON.stringify(window.picture_page_info_list)")
			if err == nil {
				var list []PicturePageInfo
				if err := json.Unmarshal([]byte(jsonList.String()), &list); err == nil {
					data.PicturePageInfoList = list
				}
			}
		}
	}

	// Check for videoPageInfos
	valVideo := vm.Get("videoPageInfos")
	if valVideo != nil && !goja.IsNull(valVideo) && !goja.IsUndefined(valVideo) {
		jsonVideo, err := vm.RunString("JSON.stringify(videoPageInfos)")
		if err == nil {
			var videoInfos []VideoPageInfoItem
			if err := json.Unmarshal([]byte(jsonVideo.String()), &videoInfos); err == nil {
				data.VideoPageInfos = videoInfos
			}
		}
	}

	return data, nil
}

func parseArticleFromDOM(htmlContent string) (*WechatOfficialArticle, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	contentSel := doc.Find("#js_content").First()
	if contentSel.Length() == 0 {
		return nil, fmt.Errorf("article content not found")
	}

	content, err := contentSel.Html()
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(doc.Find("#activity-name").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find(".rich_media_title").First().Text())
	}
	if title == "" {
		if ogTitle, ok := doc.Find(`meta[property="og:title"]`).Attr("content"); ok {
			title = strings.TrimSpace(ogTitle)
		}
	}
	if title == "" {
		return nil, fmt.Errorf("article title not found")
	}

	var images []string
	contentSel.Find("img").Each(func(i int, s *goquery.Selection) {
		imgURL := s.AttrOr("data-src", "")
		if imgURL == "" {
			imgURL = s.AttrOr("src", "")
		}
		imgURL = normalizeMediaURL(imgURL)
		if imgURL != "" {
			images = append(images, imgURL)
		}
	})

	publishTime := strings.TrimSpace(doc.Find("#publish_time").First().Text())
	return &WechatOfficialArticle{
		Title:          title,
		Content:        content,
		ContentLength:  len(content),
		Images:         images,
		Creator:        strings.TrimSpace(doc.Find("#js_author_name").First().Text()),
		AuthorNickname: strings.TrimSpace(doc.Find("#js_name").First().Text()),
		PublishTimeStr: publishTime,
	}, nil
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

func compressImage(data []byte) ([]byte, string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}

	// Resize if needed
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	maxWidth := 640

	var newImg image.Image = img
	if width > maxWidth {
		newHeight := height * maxWidth / width
		dst := image.NewRGBA(image.Rect(0, 0, maxWidth, newHeight))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		newImg = dst
	}

	// Always encode to JPEG with a white background to handle alpha
	bg := image.NewRGBA(newImg.Bounds())
	draw.Draw(bg, bg.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(bg, bg.Bounds(), newImg, newImg.Bounds().Min, draw.Over)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, bg, &jpeg.Options{Quality: 60}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/jpeg", nil
}

func downloadImageBytes(imgURL string) ([]byte, string, error) {
	imgURL = normalizeMediaURL(imgURL)
	client := &http.Client{}
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return nil, "", err
	}

	setWechatHeaders(req, "https://mp.weixin.qq.com/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("bad status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	return data, contentType, nil
}
