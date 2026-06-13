package api

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

	"github.com/gin-gonic/gin"
	result "wx_channel/internal/util"
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

// FeedResponse represents the response from channels feed API
type FeedResponse struct {
	Data    FeedResponseData `json:"data"`
	Errcode int              `json:"errCode"`
	Errmsg  string           `json:"errMsg"`
}

type FeedResponseData struct {
	Authorinfo Authorinfo `json:"authorInfo"`
	Feedinfo   Feedinfo   `json:"feedInfo"`
	Errmsg     Errmsg     `json:"errMsg"`
	Sceneinfo  Sceneinfo  `json:"sceneInfo"`
}

type Sceneinfo struct {
	Dynamicexportid string `json:"dynamicExportId"`
	Commentscene    int    `json:"commentScene"`
	Expiredtime     int    `json:"expiredTime"`
	Requestscene    int    `json:"requestScene"`
	Entryscene      int    `json:"entryScene"`
	Entrycardtype   int    `json:"entryCardType"`
}

type Errmsg struct {
	Type int `json:"type"`
}

type Feedinfo struct {
	Picinfo         []interface{} `json:"picInfo"`
	Videourl        string        `json:"videoUrl"`
	OriginVideoUrl  string        `json:"originVideoUrl"`
	Description     string        `json:"description"`
	Mediatype       int           `json:"mediaType"`
	Favcountfmt     string        `json:"favCountFmt"`
	Likecountfmt    string        `json:"likeCountFmt"`
	Forwardcountfmt string        `json:"forwardCountFmt"`
	Commentcountfmt string        `json:"commentCountFmt"`
	H264videoinfo   H264videoinfo `json:"h264VideoInfo"`
	H265videoinfo   H265videoinfo `json:"h265VideoInfo"`
	Createtime      int           `json:"createtime"`
	Scalinginfo     Scalinginfo   `json:"scalingInfo"`
	Ishardad        bool          `json:"isHardAd"`
	Coverurl        string        `json:"coverUrl"`
}

type Scalinginfo struct {
	Version               string  `json:"version"`
	Issplitscreen         bool    `json:"isSplitScreen"`
	Isdisablefollow       bool    `json:"isDisableFollow"`
	Uppercentposition     float64 `json:"upPercentPosition"`
	Downpercentposition   float64 `json:"downPercentPosition"`
	Aspectratioexceeds169 bool    `json:"aspectRatioExceeds169"`
}

type H265videoinfo struct {
	Videourl string `json:"videoUrl"`
}

type H264videoinfo struct {
	Videourl string `json:"videoUrl"`
}

type Authorinfo struct {
	Nickname    string `json:"nickname"`
	Headimgurl  string `json:"headImgUrl"`
	Authiconurl string `json:"authIconUrl"`
}

func generateRid() string {
	timestampHex := fmt.Sprintf("%x", time.Now().Unix())
	randomHex := make([]byte, 8)
	for i := range randomHex {
		randomHex[i] = "0123456789abcdef"[rand.Intn(16)]
	}
	return timestampHex + "-" + string(randomHex)
}

func parseShareUrl(shareUrl string, cookie string) (*ParseResponse, error) {
	log.Println("[parseShareUrl] start, url:", shareUrl)
	client := &http.Client{}
	payload := fmt.Sprintf(`{"type":"video_channel_url","url":"%s","scene":1}`, shareUrl)
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

	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("[parseShareUrl] read body failed:", err)
		return nil, err
	}
	var result ParseResponse
	if err := json.Unmarshal(bodyText, &result); err != nil {
		log.Println("[parseShareUrl] unmarshal failed:", err)
		return nil, err
	}
	log.Println("[parseShareUrl] success, exportId:", result.Data.WxExportId)
	return &result, nil
}

func getFeedInfo(exportId, generalToken string) (*FeedResponse, error) {
	log.Println("[getFeedInfo] start, exportId:", exportId, "generalToken:", generalToken)
	client := &http.Client{}
	rid := generateRid()
	payload := fmt.Sprintf(`{"baseReq":{"generalToken":"%s"},"exportId":"%s"}`, generalToken, exportId)
	apiUrl := fmt.Sprintf("https://channels.weixin.qq.com/finder-preview/api/feed/get_feed_info?_rid=%s&_pageUrl=https:%%2F%%2Fchannels.weixin.qq.com%%2Ffinder-preview%%2Fpages%%2Ffeed", rid)

	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(payload))
	if err != nil {
		log.Println("[getFeedInfo] create request failed:", err)
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://channels.weixin.qq.com")
	referer := fmt.Sprintf("https://channels.weixin.qq.com/finder-preview/pages/feed?entry_card_type=48&comment_scene=39&appid=0&token=%s&entry_scene=0&eid=%s", url.QueryEscape(generalToken), url.QueryEscape(exportId))
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

	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("[getFeedInfo] read body failed:", err)
		return nil, err
	}
	var feedResp FeedResponse
	if err := json.Unmarshal(bodyText, &feedResp); err != nil {
		log.Println("[getFeedInfo] unmarshal failed:", err)
		return nil, err
	}
	log.Println("[getFeedInfo] success, errCode:", feedResp.Errcode)
	return &feedResp, nil
}

func fetchVideoProfileWithShareUrl(shareUrl string, cookie string) (*FeedResponse, error) {
	log.Println("[fetch] start, shareUrl:", shareUrl)

	log.Println("[fetch] step 1/2: parseShareUrl...")
	parseResult, err := parseShareUrl(shareUrl, cookie)
	if err != nil {
		log.Println("[fetch] step 1/2 failed:", err)
		return nil, fmt.Errorf("parse share url: %w", err)
	}
	log.Println("[fetch] step 1/2 done, exportId:", parseResult.Data.WxExportId)

	// extract generalToken and exportId from playable_url query params
	generalToken := ""
	exportId := ""
	if u, err := url.Parse(parseResult.Data.PlayableUrl); err == nil {
		generalToken = u.Query().Get("token")
		exportId = u.Query().Get("eid")
	}
	if generalToken == "" {
		log.Println("[fetch] warn: generalToken is empty in playable_url")
	}
	if exportId == "" {
		log.Println("[fetch] warn: exportId (eid) is empty in playable_url")
	}
	log.Println("[fetch] generalToken:", generalToken, "exportId:", exportId)

	log.Println("[fetch] step 2/2: getFeedInfo...")
	feedResult, err := getFeedInfo(exportId, generalToken)
	if err != nil {
		log.Println("[fetch] step 2/2 failed:", err)
		return nil, fmt.Errorf("get feed info: %w", err)
	}
	log.Println("[fetch] step 2/2 done")
	log.Println("[fetch] all done")
	return feedResult, nil
}

func (c *APIClient) handleParseSph(ctx *gin.Context) {
	shareUrl := ctx.Query("url")
	if shareUrl == "" {
		result.Err(ctx, 400, "url parameter is required")
		return
	}

	// 从 APIClient 配置中获取 cookie
	cookie := c.cfg.CloudflareSphCookie
	if cookie == "" {
		result.Err(ctx, 400, "cloudflare.sphCookie not configured")
		return
	}

	feedResp, err := fetchVideoProfileWithShareUrl(shareUrl, cookie)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	// 处理 video URL：仅保留 encfilekey 和 token 参数，存储为 originVideoUrl
	if feedResp != nil && feedResp.Data.Feedinfo.Videourl != "" {
		feedResp.Data.Feedinfo.OriginVideoUrl = cleanVideoURL(feedResp.Data.Feedinfo.Videourl)
	}

	result.Ok(ctx, feedResp)
}

func cleanVideoURL(videoURL string) string {
	u, err := url.Parse(videoURL)
	if err != nil {
		return ""
	}
	filekey := u.Query().Get("encfilekey")
	token := u.Query().Get("token")
	if filekey != "" && token != "" {
		newURL := u.Scheme + "://" + u.Host + u.Path
		newURL += "?encfilekey=" + filekey + "&token=" + token
		return newURL
	}
	return ""
}
