package youku

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const playback_api = "https://acs.youku.com/h5/mtop.youku.play.ups.appinfo.get/1.1/"
const playback_app_key = "24679788"

func (client *backend) new_mtop_session(page, utid string) (*backend, error) {
	page_url, _ := url.Parse(page)
	api_url, _ := url.Parse(playback_api)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, errors.New("优酷会话初始化失败")
	}
	// Do not mutate or reuse the caller's jar. A refreshed MTOP token must
	// replace the supplied token instead of sending duplicate Cookie values.
	cookies := (&http.Request{Header: http.Header{"Cookie": {client.Cookie}}}).Cookies()
	for _, cookie := range cookies {
		cookie.Domain, cookie.Path, cookie.Secure = ".youku.com", "/", true
	}
	jar.SetCookies(page_url, cookies)
	if cookie_value(jar, api_url, "cna") == "" && utid != "" {
		jar.SetCookies(page_url, []*http.Cookie{{Name: "cna", Value: utid, Domain: ".youku.com", Path: "/", Secure: true}})
	}
	http_client := *client.HTTPClient
	http_client.Jar = jar
	request_client := *client.RequestClient
	request_client.Cookie = ""
	request_client.HTTPClient = &http_client
	request_client.Referer = page
	return &backend{RequestClient: &request_client}, nil
}

func (client *backend) load_mtop_playback(ctx context.Context, id, utid string) (youku_playback, error) {
	page := "https://v.youku.com/v_show/id_" + id + ".html"
	page_url, _ := url.Parse(page)
	api_url, _ := url.Parse(playback_api)
	session, err := client.new_mtop_session(page, utid)
	if err != nil {
		return youku_playback{}, err
	}
	jar := session.HTTPClient.Jar
	session.Referer = "https://www.youku.com/"
	text, err := session.GetText(ctx, page)
	if ctx.Err() != nil {
		return youku_playback{}, ctx.Err()
	}
	awsc_address, player_address, err := player_script_addresses(page, text)
	if err != nil {
		// Signature libraries are shared public assets. The homepage also
		// publishes their current URLs when a play page contains only a gate.
		discovery_http := *session.HTTPClient
		discovery_http.Jar = nil
		discovery_client := *session.RequestClient
		discovery_client.HTTPClient = &discovery_http
		text, err = discovery_client.GetText(ctx, "https://www.youku.com/")
		if ctx.Err() != nil {
			return youku_playback{}, ctx.Err()
		}
		if err == nil {
			awsc_address, player_address, err = player_script_addresses("https://www.youku.com/", text)
		}
		if err != nil {
			awsc_address, player_address = fallback_awsc_address, fallback_player_address
		}
	}
	session.Referer = page
	player_js, err := session.fetch_script(ctx, player_address)
	if err != nil {
		return youku_playback{}, err
	}
	version := player_version_pattern.FindStringSubmatch(player_js)
	ccode := player_ccode_pattern.FindStringSubmatch(player_js)
	if len(version) != 2 || len(ccode) != 2 {
		return youku_playback{}, errors.New("优酷播放器 JS 缺少版本或客户端参数")
	}
	bootstrap, err := session.fetch_script(ctx, awsc_address)
	if err != nil {
		return youku_playback{}, err
	}
	module := uab_module_path.FindStringSubmatch(bootstrap)
	if len(module) != 2 {
		return youku_playback{}, errors.New("优酷官网未提供 UAB 签名脚本")
	}
	sdk, err := session.fetch_script(ctx, "https://g.alicdn.com/"+module[1])
	if err != nil {
		return youku_playback{}, err
	}
	cookie_header := &http.Request{Header: make(http.Header)}
	for _, cookie := range jar.Cookies(page_url) {
		cookie_header.AddCookie(cookie)
	}
	ckey, err := calculate_ckey(ctx, sdk, page, cookie_header.Header.Get("Cookie"))
	if err != nil {
		return youku_playback{}, err
	}
	utid = cookie_value(jar, api_url, "cna")
	if utid == "" {
		return youku_playback{}, errors.New("优酷播放会话缺少 cna")
	}
	steal, _ := json.Marshal(map[string]any{"ccode": ccode[1], "utid": utid, "version": version[1], "ckey": ckey, "client_ip": "192.168.1.1", "client_ts": time.Now().Unix()})
	business, _ := json.Marshal(map[string]any{"vid": id, "h265": 0, "media_type": "standard", "app_ver": version[1], "extag": "EXT-X-PRIVINF", "master_m3u8": 0, "drm_type": 0, "play_ability": 0, "close_ability": 0, "wdm": 1, "pn": "com.youku.pcweb", "playMode": "normal", "enc_ver": 0})
	data, _ := json.Marshal(map[string]string{"steal_params": string(steal), "biz_params": string(business), "ad_params": "{}"})
	raw, err := session.request_mtop(ctx, "mtop.youku.play.ups.appinfo.get", "1.1", data)
	if err != nil {
		return youku_playback{}, err
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return youku_playback{}, errors.New("优酷 MTOP 播放数据无效")
	}
	body, err := mtop_json_object(envelope.Data)
	if err != nil {
		return youku_playback{}, err
	}
	var payload youku_playback
	if json.Unmarshal([]byte(`{"data":`+string(body)+`}`), &payload) != nil {
		return youku_playback{}, errors.New("优酷 MTOP 播放数据无效")
	}
	if code := youku_error_code(payload.Data.Error); code != "" {
		return youku_playback{}, fmt.Errorf("优酷播放接口返回错误（code=%s），请检查官网播放状态及 Cookie", code)
	}
	return payload, nil
}

// Both official playback and catalogue requests use the same session/token
// rules. Only the two explicitly supported APIs can be selected here.
func (session *backend) request_mtop(ctx context.Context, api, version string, data json.RawMessage) (json.RawMessage, error) {
	if (api != "mtop.youku.play.ups.appinfo.get" || version != "1.1") && (api != "mtop.youku.columbus.gateway.new.execute" || version != "1.0") {
		return nil, errors.New("不支持的优酷 MTOP 接口")
	}
	api_url, _ := url.Parse("https://acs.youku.com/h5/" + api + "/" + version + "/")
	for attempt := 0; attempt < 3; attempt++ {
		token := strings.SplitN(cookie_value(session.HTTPClient.Jar, api_url, "_m_h5_tk"), "_", 2)[0]
		timestamp := fmt.Sprint(time.Now().UnixMilli())
		sign := fmt.Sprintf("%x", md5.Sum([]byte(token+"&"+timestamp+"&"+playback_app_key+"&"+string(data))))
		params := url.Values{"jsv": {"2.7.5"}, "appKey": {playback_app_key}, "t": {timestamp}, "sign": {sign}, "api": {api}, "v": {version}, "type": {"originaljson"}, "dataType": {"json"}, "data": {string(data)}}
		raw, err := session.GetText(ctx, api_url.String()+"?"+params.Encode())
		if err != nil {
			return nil, err
		}
		var envelope struct {
			Ret  []string        `json:"ret"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal([]byte(raw), &envelope) != nil || len(envelope.Ret) == 0 {
			return nil, errors.New("优酷 MTOP 响应无效")
		}
		if token_retry(envelope.Ret) {
			continue
		}
		for _, status := range envelope.Ret {
			if strings.SplitN(status, "::", 2)[0] != "SUCCESS" {
				return nil, errors.New("优酷 MTOP 请求未成功，请检查官网状态及 Cookie")
			}
		}
		return mtop_json_object(envelope.Data)
	}
	return nil, errors.New("优酷 MTOP 令牌刷新失败，请更新优酷 Cookie")
}

func mtop_json_object(raw []byte) (json.RawMessage, error) {
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, `"`) {
		var decoded string
		if json.Unmarshal([]byte(text), &decoded) != nil {
			return nil, errors.New("优酷 MTOP JSON 数据无效")
		}
		text = strings.TrimSpace(decoded)
	}
	if !strings.HasPrefix(text, "{") || !json.Valid([]byte(text)) {
		return nil, errors.New("优酷 MTOP JSON 数据无效")
	}
	return json.RawMessage(text), nil
}

func cookie_value(jar http.CookieJar, address *url.URL, name string) string {
	for _, cookie := range jar.Cookies(address) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func token_retry(statuses []string) bool {
	for _, status := range statuses {
		switch strings.SplitN(status, "::", 2)[0] {
		case "FAIL_SYS_TOKEN_EMPTY", "FAIL_SYS_TOKEN_EXPIRED", "FAIL_SYS_TOKEN_EXOIRED":
			return true
		}
	}
	return false
}
