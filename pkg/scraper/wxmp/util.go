package wxmp

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
)

const wechat_user_agent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN"

type WebsocketClient struct {
	conn      *websocket.Conn
	send      chan []byte
	title     string
	available bool
	last_ping int64
}

func (c *WebsocketClient) write_pump() {
	ticker := time.NewTicker(5 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func set_wechat_headers(req *http.Request, referer string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", wechat_user_agent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

func normalize_media_url(raw string) string {
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

func is_verification_page(body string) bool {
	return strings.Contains(body, "环境异常") && strings.Contains(body, "完成验证后即可继续访问")
}

func parse_cgi_datanew(html_content string) (*ArticleCgiDataNew, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html_content))
	if err != nil {
		return nil, err
	}

	var scripts []string
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "window.cgiDataNew =") ||
			strings.Contains(text, "var videoPageInfos =") ||
			strings.Contains(text, "picture_page_info_list") {
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
	// are often just string literals. If they contain escape sequences,
	// the JS parser handles them.
	// We'll treat it as an identity function for now.
	vm.Set("JsDecode", func(call goja.FunctionCall) goja.Value {
		return call.Argument(0)
	})

	// Run the scripts
	var script_errs []error
	for _, script := range scripts {
		_, err = vm.RunString(script)
		if err != nil {
			script_errs = append(script_errs, err)
		}
	}

	// Extract cgiDataNew
	val := vm.Get("window").ToObject(vm).Get("cgiDataNew")
	if val == nil {
		if len(script_errs) > 0 {
			for _, err := range script_errs {
				fmt.Printf("failed to run script: %v\n", err)
			}
		}
		return nil, fmt.Errorf("window.cgiDataNew is nil")
	}

	// Convert to JSON string
	json_text, err := vm.RunString("JSON.stringify(window.cgiDataNew)")
	if err != nil {
		return nil, fmt.Errorf("failed to stringify cgiDataNew: %v", err)
	}

	data := &ArticleCgiDataNew{}
	if err := json.Unmarshal([]byte(json_text.String()), data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cgiDataNew: %v", err)
	}

	// Check if PicturePageInfoList is empty, if so check global picture_page_info_list.
	if len(data.PicturePageInfoList) == 0 {
		data.PicturePageInfoList = read_picture_page_info_list(vm, "window.picture_page_info_list")
	}
	if len(data.PicturePageInfoList) == 0 {
		data.PicturePageInfoList = read_picture_page_info_list(vm, "picture_page_info_list")
	}

	// Check for videoPageInfos
	video_value := vm.Get("videoPageInfos")
	if video_value != nil && !goja.IsNull(video_value) && !goja.IsUndefined(video_value) {
		video_json, err := vm.RunString("JSON.stringify(videoPageInfos)")
		if err == nil {
			var video_infos []VideoPageInfoItem
			if err := json.Unmarshal([]byte(video_json.String()), &video_infos); err == nil {
				data.VideoPageInfos = video_infos
			}
		}
	}

	return data, nil
}

func read_picture_page_info_list(vm *goja.Runtime, expr string) []PicturePageInfo {
	val, err := vm.RunString(expr)
	if err != nil || val == nil || goja.IsNull(val) || goja.IsUndefined(val) {
		return nil
	}
	json_list, err := vm.RunString("JSON.stringify(" + expr + ")")
	if err != nil {
		return nil
	}
	var list []PicturePageInfo
	if err := json.Unmarshal([]byte(json_list.String()), &list); err != nil {
		return nil
	}
	return list
}

func parse_article_from_dom(html_content string) (*ArticleCgiDataNew, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html_content))
	if err != nil {
		return nil, err
	}

	article := &ArticleCgiDataNew{}

	// Extract title
	title := doc.Find("meta[property=\"og:title\"]").AttrOr("content", "")
	if title == "" {
		title = doc.Find("h1.rich_media_title").Text()
	}
	if title == "" {
		title = doc.Find("title").Text()
	}
	article.Title = strings.TrimSpace(title)

	// Extract content from rich_media_content or js_content
	content_html := ""
	doc.Find("#js_content, .rich_media_content").Each(func(i int, s *goquery.Selection) {
		if content_html == "" {
			content_html, _ = s.Html()
		}
	})
	article.ContentNoencode = content_html

	// Extract author nickname
	nickname := doc.Find("meta[name=\"author\"]").AttrOr("content", "")
	if nickname == "" {
		nickname = doc.Find(".rich_media_meta_nickname").First().Text()
	}
	if nickname == "" {
		nickname = doc.Find(".profile_nickname").First().Text()
	}
	article.NickName = strings.TrimSpace(nickname)

	// Extract author avatar
	avatar := doc.Find(".rich_media_meta_avatar, .js_wx_tap_highlight img").First().AttrOr("src", "")
	article.RoundHeadImg = normalize_media_url(avatar)

	// Extract publish time from script or meta.
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		if article.CreateTime != "" {
			return
		}
		text := s.Text()
		if strings.Contains(text, "createTime") {
			re := regexp.MustCompile(`var\s+createTime\s*=\s*'([^']+)'`)
			matches := re.FindStringSubmatch(text)
			if len(matches) > 1 {
				article.CreateTime = strings.TrimSpace(matches[1])
			}
		}
	})

	// Extract image URLs
	doc.Find("#js_content img, .rich_media_content img").Each(func(i int, s *goquery.Selection) {
		image_url := s.AttrOr("data-src", "")
		if image_url == "" {
			image_url = s.AttrOr("src", "")
		}
		image_url = normalize_media_url(image_url)
		if image_url != "" {
			article.PicturePageInfoList = append(article.PicturePageInfoList, PicturePageInfo{
				CdnUrl: image_url,
			})
		}
	})

	return article, nil
}

func ValidateTokenFilepath(file_path string, root_dir string) (string, error) {
	token_filepath := file_path
	if file_path == "" {
		return "", nil
	}
	if !filepath.IsAbs(file_path) {
		token_filepath = filepath.Join(root_dir, file_path)
	}
	if _, err := os.Stat(token_filepath); err != nil {
		return "", err
	}
	return token_filepath, nil
}

type APIClientWSMessage struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

type ClientWSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}
type ClientWebsocketRequestBody struct {
	ID   string      `json:"id"`
	Key  string      `json:"key"`
	Body interface{} `json:"data"`
}
type ClientWebsocketResponse struct {
	Id string `json:"id"`
	// Original response from wx api call
	Data json.RawMessage `json:"data"`
}

type OfficialMsgListResp struct {
	Ret        int    `json:"ret"`
	ErrMsg     string `json:"errmsg"`
	MsgList    string `json:"general_msg_list"`
	HasMore    int    `json:"can_msg_continue"`
	MsgCount   int    `json:"msg_count"`
	NextOffset int    `json:"next_offset"`
}
type CommonMsgInfo struct {
	Id       int    `json:"id"`
	Type     int    `json:"type"`
	Datetime int    `json:"datetime"`
	Fakeid   string `json:"fakeid"`
	Status   int    `json:"status"`
	Content  string `json:"content"`
}
type OfficialAccountMsgListRespItem struct {
	MsgExtInfo    OfficialAccountMsg `json:"app_msg_ext_info"`
	CommonMsgInfo CommonMsgInfo      `json:"comm_msg_info"`
}
type OfficialArticle struct {
	Title                  string `json:"title"`
	Digest                 string `json:"digest"`
	Content                string `json:"content"`
	Fileid                 int    `json:"fileid"`
	ContentUrl             string `json:"content_url"`
	SourceUrl              string `json:"source_url"`
	Cover                  string `json:"cover"`
	Author                 string `json:"author"`
	CopyrightStat          int    `json:"copyright_stat"`
	DelFlag                int    `json:"del_flag"`
	ItemShowType           int    `json:"item_show_type"`
	AudioFileid            int    `json:"audio_fileid"`
	Duration               int    `json:"duration"`
	PlayUrl                string `json:"play_url"`
	MaliciousTitleReasonId int    `json:"malicious_title_reason_id"`
	MaliciousContentType   int    `json:"malicious_content_type"`
}

// Push message — same as OfficialArticle but with IsMulti and MultiAppMsgItemList fields added
type OfficialAccountMsg struct {
	Title                  string            `json:"title"`
	Digest                 string            `json:"digest"`
	Content                string            `json:"content"`
	Fileid                 int               `json:"fileid"`
	ContentUrl             string            `json:"content_url"`
	SourceUrl              string            `json:"source_url"`
	Cover                  string            `json:"cover"`
	Subtype                int               `json:"subtype"`
	IsMulti                int               `json:"is_multi"`
	MultiAppMsgItemList    []OfficialArticle `json:"multi_app_msg_item_list"`
	Author                 string            `json:"author"`
	CopyrightStat          int               `json:"copyright_stat"`
	Duration               int               `json:"duration"`
	DelFlag                int               `json:"del_flag"`
	ItemShowType           int               `json:"item_show_type"`
	AudioFileid            int               `json:"audio_fileid"`
	PlayUrl                string            `json:"play_url"`
	MaliciousTitleReasonId int               `json:"malicious_title_reason_id"`
	MaliciousContentType   int               `json:"malicious_content_type"`
}
