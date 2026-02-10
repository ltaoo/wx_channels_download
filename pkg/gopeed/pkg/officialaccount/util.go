package officialaccountdownload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/chai2010/webp"
	"github.com/dop251/goja"
	"golang.org/x/image/draw"
)

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
	maxWidth := 640 // Reduced from 800 to 640

	var newImg image.Image = img
	if width > maxWidth {
		newHeight := height * maxWidth / width
		dst := image.NewRGBA(image.Rect(0, 0, maxWidth, newHeight))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		newImg = dst
	}

	// Encode to WebP
	var buf bytes.Buffer
	// Quality 50 provides a good balance between size and visual quality for web usage
	err = webp.Encode(&buf, newImg, &webp.Options{Lossless: false, Quality: 50})
	if err != nil {
		// Fallback to JPEG if WebP encoding fails (though unlikely)
		fmt.Printf("WebP encoding failed, falling back to JPEG: %v\n", err)
		buf.Reset()

		// Create white background for JPEG
		bg := image.NewRGBA(newImg.Bounds())
		draw.Draw(bg, bg.Bounds(), &image.Uniform{image.White}, image.Point{}, draw.Src)
		draw.Draw(bg, bg.Bounds(), newImg, newImg.Bounds().Min, draw.Over)
		finalImg := bg

		err = jpeg.Encode(&buf, finalImg, &jpeg.Options{Quality: 40})
		if err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	}

	return buf.Bytes(), "image/webp", nil
}

func downloadImageBytes(imgURL string) ([]byte, string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://mp.weixin.qq.com/")

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
