package wxchannels

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ParseResponse represents the response from tencent yuanbao API
type ParseResponse struct {
	Code int       `json:"code"`
	Msg  string    `json:"msg"`
	Data ParseData `json:"data"`
}

type ParseData struct {
	WxExportId              string `json:"wx_export_id"`
	CoverUrl                string `json:"cover_url"`
	AuthorCertificationIcon string `json:"author_certification_icon"`
	Author                  string `json:"author"`
	AuthorIcon              string `json:"author_icon"`
	Desc                    string `json:"desc"`
	PlayableUrl             string `json:"playable_url"`
}

func generate_rid() string {
	timestamp_hex := fmt.Sprintf("%x", time.Now().Unix())
	random_hex := make([]byte, 8)
	for i := range random_hex {
		random_hex[i] = "0123456789abcdef"[rand.Intn(16)]
	}
	return timestamp_hex + "-" + string(random_hex)
}

func ParseShareUrl(share_url string, cookie string) (*ParseResponse, error) {
	log.Println("[parseShareUrl] start, url:", share_url)
	client := &http.Client{}
	payload := fmt.Sprintf(`{"type":"video_channel_url","url":"%s","scene":1}`, share_url)
	req, err := http.NewRequest("POST", "https://yuanbao.tencent.com/api/weixin/get_parse_result", strings.NewReader(payload))
	if err != nil {
		log.Println("[parseShareUrl] create request failed:", err)
		return nil, err
	}
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("origin", "https://yuanbao.tencent.com")
	req.Header.Set("referer", "https://yuanbao.tencent.com/chat/naQivTmsDa/cf4d0079-ed1b-4c55-a3f3-2ca1379727d1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	req.Header.Set("sec-ch-ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("t-userid", "b9575f6b0a8c4a55a08096904a5ef20a")
	req.Header.Set("x-agentid", "naQivTmsDa/cf4d0079-ed1b-4c55-a3f3-2ca1379727d1")
	req.Header.Set("x-commit-tag", "72282a0d")
	req.Header.Set("x-device-id", "1921b001708100d7fa31002b9646bd0cc15a3e2e1f")
	req.Header.Set("x-hy106", "")
	req.Header.Set("x-hy92", "e963067ffa31002b9646bd0c03000008b1951a")
	req.Header.Set("x-hy93", "1921b001708100d7fa31002b9646bd0cc15a3e2e1f")
	req.Header.Set("x-id", "b9575f6b0a8c4a55a08096904a5ef20a")
	req.Header.Set("x-instance-id", "5")
	req.Header.Set("x-language", "zh-CN")
	req.Header.Set("x-os_version", "Mac OS(10.15.7)-Blink")
	req.Header.Set("x-platform", "mac")
	req.Header.Set("x-requested-with", "XMLHttpRequest")
	req.Header.Set("x-source", "web")
	req.Header.Set("x-web-third-source", "main")
	req.Header.Set("x-webdriver", "0")
	req.Header.Set("x-webversion", "2.69.0")
	req.Header.Set("x-ybuitest", "0")
	req.Header.Set("cookie", cookie)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("[parseShareUrl] http request failed:", err)
		return nil, err
	}
	defer resp.Body.Close()

	body_text, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("[parseShareUrl] read body failed:", err)
		return nil, err
	}
	var result ParseResponse
	if err := json.Unmarshal(body_text, &result); err != nil {
		log.Println("[parseShareUrl] unmarshal failed:", err)
		return nil, err
	}
	log.Println("[parseShareUrl] success, exportId:", result.Data.WxExportId)
	return &result, nil
}

func get_feed_info(export_id, general_token string) (json.RawMessage, error) {
	log.Println("[getFeedInfo] start, exportId:", export_id, "generalToken:", general_token)
	client := &http.Client{}
	rid := generate_rid()
	payload := fmt.Sprintf(`{"baseReq":{"generalToken":"%s"},"exportId":"%s"}`, general_token, export_id)
	api_url := fmt.Sprintf("https://channels.weixin.qq.com/finder-preview/api/feed/get_feed_info?_rid=%s&_pageUrl=https:%%2F%%2Fchannels.weixin.qq.com%%2Ffinder-preview%%2Fpages%%2Ffeed", rid)

	req, err := http.NewRequest("POST", api_url, strings.NewReader(payload))
	if err != nil {
		log.Println("[getFeedInfo] create request failed:", err)
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://channels.weixin.qq.com")
	referer := fmt.Sprintf("https://channels.weixin.qq.com/finder-preview/pages/feed?entry_card_type=48&comment_scene=39&appid=0&token=%s&entry_scene=0&eid=%s", url.QueryEscape(general_token), url.QueryEscape(export_id))
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	req.Header.Set("sec-ch-ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("[getFeedInfo] http request failed:", err)
		return nil, err
	}
	defer resp.Body.Close()

	body_text, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("[getFeedInfo] read body failed:", err)
		return nil, err
	}
	log.Println("[getFeedInfo] success")
	return body_text, nil
}

func FetchVideoProfileWithShareUrl(share_url string, cookie string) (json.RawMessage, error) {
	log.Println("[fetch] start, shareUrl:", share_url)

	log.Println("[fetch] step 1/2: parseShareUrl...")
	parse_result, err := ParseShareUrl(share_url, cookie)
	if err != nil {
		log.Println("[fetch] step 1/2 failed:", err)
		return nil, fmt.Errorf("parse share url: %w", err)
	}
	log.Println("[fetch] step 1/2 done, exportId:", parse_result.Data.WxExportId)

	// extract generalToken and exportId from playable_url query params
	general_token := ""
	export_id := ""
	if u, err := url.Parse(parse_result.Data.PlayableUrl); err == nil {
		general_token = u.Query().Get("token")
		export_id = u.Query().Get("eid")
	}
	if general_token == "" {
		log.Println("[fetch] warn: generalToken is empty in playable_url")
	}
	if export_id == "" {
		log.Println("[fetch] warn: exportId (eid) is empty in playable_url")
	}
	log.Println("[fetch] generalToken:", general_token, "exportId:", export_id)

	log.Println("[fetch] step 2/2: getFeedInfo...")
	feed_result, err := get_feed_info(export_id, general_token)
	if err != nil {
		log.Println("[fetch] step 2/2 failed:", err)
		return nil, fmt.Errorf("get feed info: %w", err)
	}
	log.Println("[fetch] step 2/2 done")
	log.Println("[fetch] all done")
	return feed_result, nil
}

func CleanVideoURL(video_url string) string {
	u, err := url.Parse(video_url)
	if err != nil {
		return ""
	}
	filekey := u.Query().Get("encfilekey")
	token := u.Query().Get("token")
	if filekey != "" && token != "" {
		new_url := u.Scheme + "://" + u.Host + u.Path
		new_url += "?encfilekey=" + filekey + "&token=" + token
		return new_url
	}
	return ""
}
