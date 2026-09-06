package xiaohongshu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/minib"
)

const (
	xhs_home_api_url         = "https://edith.xiaohongshu.com/api/sns/web/v1/user_posted"
	xhs_home_api_timeout     = 35 * time.Second
	xhs_home_api_count       = "30"
	xhs_home_api_source      = "pc_feed"
	xhs_home_security_sdk    = "4.4.3"
	xhs_home_web_build       = "6.47.2"
	xhs_custom_base64_source = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	xhs_custom_base64_target = "ZmserbBoHQtNP+wOcza/LpngG8yJq42KWYj0DSfdikx3VT16IlUAFM97hECvuRX5"
)

type xhs_home_api_response struct {
	Success bool              `json:"success"`
	Code    int               `json:"code"`
	Message string            `json:"msg"`
	Data    xhs_home_api_data `json:"data"`
}

type xhs_home_api_data struct {
	Cursor  string              `json:"cursor"`
	HasMore bool                `json:"has_more"`
	Notes   []xhs_home_api_note `json:"notes"`
}

type xhs_home_api_note struct {
	ID           string                   `json:"id"`
	NoteID       string                   `json:"note_id"`
	XSecToken    string                   `json:"xsec_token"`
	DisplayTitle string                   `json:"display_title"`
	Type         string                   `json:"type"`
	Time         int64                    `json:"time"`
	User         xhs_home_api_user        `json:"user"`
	InteractInfo xhs_home_api_interaction `json:"interact_info"`
	Cover        xhs_home_api_image       `json:"cover"`
	NoteCard     *xhs_home_api_note       `json:"note_card"`
}

type xhs_home_api_user struct {
	UserID    string `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar"`
	XSecToken string `json:"xsec_token"`
}

type xhs_home_api_interaction struct {
	LikedCount     FlexibleInt64 `json:"liked_count"`
	CommentCount   FlexibleInt64 `json:"comment_count"`
	CollectedCount FlexibleInt64 `json:"collected_count"`
	ShareCount     FlexibleInt64 `json:"share_count"`
}

type xhs_home_api_image struct {
	FileID     string      `json:"file_id"`
	URL        string      `json:"url"`
	URLPreview string      `json:"url_pre"`
	URLDefault string      `json:"url_default"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	InfoList   []ImageInfo `json:"info_list"`
}

type xhs_home_runtime_state struct {
	A1     string `json:"a1"`
	B1     string `json:"b1"`
	B1B1   string `json:"b1b1"`
	DSLLT  string `json:"dsllt"`
	DSL    string `json:"dsl"`
	WebApp string `json:"web_build"`
}

type xhs_common_payload struct {
	S0  int    `json:"s0"`
	S1  string `json:"s1"`
	X0  string `json:"x0"`
	X1  string `json:"x1"`
	X2  string `json:"x2"`
	X3  string `json:"x3"`
	X4  string `json:"x4"`
	X5  string `json:"x5"`
	X6  string `json:"x6"`
	X7  string `json:"x7"`
	X8  string `json:"x8"`
	X9  int32  `json:"x9"`
	X10 int    `json:"x10"`
	X11 string `json:"x11"`
	X12 string `json:"x12"`
}

func (c *Client) FetchHomeContentsPage(raw_url string, scope string, page_marker string) (*HomeContentList, error) {
	return c.FetchHomeContentsPageContext(context.Background(), raw_url, scope, page_marker)
}

func (c *Client) FetchHomeContentsPageContext(fetch_context context.Context, raw_url string, scope string, page_marker string) (*HomeContentList, error) {
	page_marker = strings.TrimSpace(page_marker)
	if page_marker == "" {
		return c.FetchHomeContentsContext(fetch_context, raw_url, scope)
	}
	normalized_scope, _, err := normalize_home_scope(scope)
	if err != nil {
		return nil, err
	}
	if normalized_scope != "notes" {
		return nil, fmt.Errorf("小红书收藏列表暂不支持翻页")
	}
	profile_url, err := normalize_profile_url(raw_url)
	if err != nil {
		return nil, err
	}
	return c.fetch_home_api_page(fetch_context, profile_url, page_marker)
}

func (c *Client) fetch_home_api_page(fetch_context context.Context, profile_url string, page_marker string) (*HomeContentList, error) {
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	fetch_context, cancel_fetch := context.WithTimeout(fetch_context, xhs_home_api_timeout)
	defer cancel_fetch()
	profile, _ := url.Parse(profile_url)
	path_parts := strings.Split(strings.Trim(profile.Path, "/"), "/")
	if len(path_parts) < 3 || strings.TrimSpace(path_parts[len(path_parts)-1]) == "" {
		return nil, fmt.Errorf("xiaohongshu home API: profile user_id is missing")
	}
	user_id := strings.TrimSpace(path_parts[len(path_parts)-1])
	xsec_token := strings.TrimSpace(profile.Query().Get("xsec_token"))
	if xsec_token == "" {
		return nil, fmt.Errorf("xiaohongshu home API: profile xsec_token is missing")
	}
	page_query := xhs_home_api_query(user_id, page_marker, xsec_token, xhs_home_api_source)
	request_path := "/api/sns/web/v1/user_posted?" + page_query
	request_url := xhs_home_api_url + "?" + page_query

	browser, err := minib.NewMiniBrowser(xhs_home_api_timeout, c.cookie_reader)
	if err != nil {
		return nil, fmt.Errorf("xiaohongshu home API: create browser: %w", err)
	}
	defer browser.Close()
	page, err := browser.Navigate(fetch_context, profile_url, nil, minib.NavigateOptions{
		DisableImages: true, DisableMedia: true, DisableCSS: true,
		JavaScriptTimeout: 12 * time.Second, ResourceTimeout: 12 * time.Second,
		WaitUntil: minib.WaitUntilDOMContentLoaded,
	})
	if err != nil {
		return nil, fmt.Errorf("xiaohongshu home API: initialize signature runtime: %w", err)
	}
	if page.StatusCode < http.StatusOK || page.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("xiaohongshu home API: profile returned HTTP %d", page.StatusCode)
	}
	runtime_state, err := xhs_home_runtime(browser, fetch_context)
	if err != nil {
		return nil, err
	}
	a1_value := runtime_state.A1
	if a1_value == "" {
		return nil, fmt.Errorf("xiaohongshu home API: a1 cookie is missing")
	}
	timestamp_ms := time.Now().UnixMilli()
	x_s, err := xhs_xyw_signature(request_path, a1_value, timestamp_ms)
	if err != nil {
		return nil, err
	}
	x_s_common, err := xhs_home_common_signature(runtime_state, a1_value)
	if err != nil {
		return nil, err
	}
	x_rap, err := xhs_rap_param("//edith.xiaohongshu.com/api/sns/web/v1/user_posted", timestamp_ms)
	if err != nil {
		return nil, fmt.Errorf("xiaohongshu home API: generate x-rap-param: %w", err)
	}
	headers := make(http.Header)
	headers.Set("Accept", "application/json, text/plain, */*")
	headers.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	headers.Set("Origin", "https://www.xiaohongshu.com")
	headers.Set("Referer", profile_url)
	headers.Set("User-Agent", default_user_agent)
	headers.Set("X-S", x_s)
	headers.Set("X-T", strconv.FormatInt(timestamp_ms, 10))
	headers.Set("X-S-Common", x_s_common)
	headers.Set("X-Rap-Param", x_rap)
	headers.Set("X-B3-Traceid", xhs_random_hex(8))
	headers.Set("X-Xray-Traceid", xhs_random_hex(16))
	response, err := browser.Request(fetch_context, http.MethodGet, request_url, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("xiaohongshu home API: request: %w", err)
	}
	body, err := response.Text()
	if err != nil {
		return nil, fmt.Errorf("xiaohongshu home API: read response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("xiaohongshu home API: HTTP %d body=%q", response.StatusCode, response_preview([]byte(body)))
	}
	return decode_xhs_home_api(body, profile_url)
}

func xhs_home_api_query(user_id string, cursor string, xsec_token string, xsec_source string) string {
	values := []string{
		"num=" + xhs_home_query_escape(xhs_home_api_count),
		"cursor=" + xhs_home_query_escape(cursor),
		"user_id=" + xhs_home_query_escape(user_id),
		"image_formats=jpg,webp,avif",
		"xsec_token=" + xhs_home_query_escape(xsec_token),
		"xsec_source=" + xhs_home_query_escape(xsec_source),
	}
	return strings.Join(values, "&")
}

func xhs_home_query_escape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func xhs_home_runtime(browser *minib.MiniBrowser, fetch_context context.Context) (xhs_home_runtime_state, error) {
	value, err := browser.ExecuteJSImmediate(fetch_context, `JSON.stringify({a1:decodeURIComponent((document.cookie.match(/(?:^|; )a1=([^;]*)/)||[])[1]||''),b1:localStorage.getItem('b1')||'',b1b1:localStorage.getItem('b1b1')||'1',dsllt:localStorage.getItem('dsllt')||'',dsl:String(window._dsl||''),web_build:decodeURIComponent((document.cookie.match(/(?:^|; )webBuild=([^;]*)/)||[])[1]||'')})`)
	if err != nil {
		return xhs_home_runtime_state{}, fmt.Errorf("xiaohongshu home API: read signature runtime: %w", err)
	}
	var state xhs_home_runtime_state
	if err := json.Unmarshal([]byte(value.String()), &state); err != nil {
		return state, fmt.Errorf("xiaohongshu home API: decode signature runtime: %w", err)
	}
	timestamp_ms := time.Now().UnixMilli()
	if state.B1 == "" {
		state.B1, err = xhs_generate_b1(timestamp_ms)
		if err != nil {
			return state, fmt.Errorf("xiaohongshu home API: generate b1 fingerprint: %w", err)
		}
	}
	if state.DSLLT == "" {
		state.DSLLT = strconv.FormatInt(timestamp_ms, 10)
	}
	if state.DSL == "" {
		state.DSL = state.DSLLT
	}
	return state, nil
}

func xhs_home_common_signature(state xhs_home_runtime_state, a1_value string) (string, error) {
	web_build := strings.TrimSpace(state.WebApp)
	if web_build == "" {
		web_build = xhs_home_web_build
	}
	payload := xhs_common_payload{
		S0: 3, S1: "", X0: first_non_empty(state.B1B1, "1"), X1: xhs_home_security_sdk,
		X2: "Mac OS", X3: "xhs-pc-web", X4: web_build, X5: a1_value,
		X6: "", X7: "", X8: state.B1, X9: int32(crc32.ChecksumIEEE([]byte(state.B1))),
		X10: 0, X11: "normal", X12: state.DSLLT + ";" + state.DSL,
	}
	payload_data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("xiaohongshu home API: encode X-S-Common payload: %w", err)
	}
	return xhs_custom_base64(payload_data), nil
}

func xhs_random_hex(byte_count int) string {
	value := make([]byte, byte_count)
	if _, err := rand.Read(value); err != nil {
		return strings.Repeat("0", byte_count*2)
	}
	return hex.EncodeToString(value)
}

func decode_xhs_home_api(body string, source string) (*HomeContentList, error) {
	var response xhs_home_api_response
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, fmt.Errorf("xiaohongshu home API: decode response: %w", err)
	}
	if !response.Success || response.Code != 0 {
		return nil, fmt.Errorf("xiaohongshu home API: code %d: %s", response.Code, strings.TrimSpace(response.Message))
	}
	items := make([]HomeNoteItem, 0, len(response.Data.Notes))
	for _, api_note := range response.Data.Notes {
		items = append(items, xhs_home_api_item(api_note))
	}
	next_marker := ""
	if response.Data.HasMore {
		next_marker = strings.TrimSpace(response.Data.Cursor)
	}
	return &HomeContentList{Source: source, Scope: "notes", Items: items, NextMarker: next_marker}, nil
}

func xhs_home_api_item(api_note xhs_home_api_note) HomeNoteItem {
	card := api_note
	if api_note.NoteCard != nil {
		card = *api_note.NoteCard
	}
	note_id := first_non_empty(card.NoteID, card.ID, api_note.NoteID, api_note.ID)
	xsec_token := first_non_empty(card.XSecToken, api_note.XSecToken)
	return HomeNoteItem{ID: note_id, XSecToken: xsec_token, NoteCard: HomeNoteCard{
		NoteID: note_id, XSecToken: xsec_token, DisplayTitle: card.DisplayTitle,
		Type: card.Type, Time: card.Time,
		User:         NoteUser{UserID: card.User.UserID, Nickname: card.User.Nickname, AvatarURL: card.User.AvatarURL, XSecToken: card.User.XSecToken},
		InteractInfo: Interaction{LikedCount: card.InteractInfo.LikedCount, CommentCount: card.InteractInfo.CommentCount, CollectedCount: card.InteractInfo.CollectedCount, ShareCount: card.InteractInfo.ShareCount},
		Cover:        Image{FileID: card.Cover.FileID, URL: card.Cover.URL, URLPreview: card.Cover.URLPreview, URLDefault: card.Cover.URLDefault, Width: card.Cover.Width, Height: card.Cover.Height, InfoList: card.Cover.InfoList},
	}}
}
