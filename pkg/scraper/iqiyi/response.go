package iqiyi

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	sourceutil "wx_channel/pkg/scraper/internal/source"
)

// PlaybackError preserves an upstream denial separately from malformed data or
// a mismatched episode. No signed URL, Cookie or arbitrary server text is kept.
type PlaybackError struct {
	Kind    string
	Code    string
	Status  int
	message string
}

func (e *PlaybackError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("爱奇艺%s（code=%s，st=%d）", e.message, e.Code, e.Status)
	}
	if e.Code != "" {
		return fmt.Sprintf("爱奇艺%s（code=%s）", e.message, e.Code)
	}
	return "爱奇艺" + e.message
}

type dash_video struct {
	Bid      int                  `json:"bid"`
	Format   string               `json:"ff"`
	M3U8     string               `json:"m3u8"`
	Duration sourceutil.TextValue `json:"duration"`
	Size     string               `json:"scrsz"`
}

var playback_code_pattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func read_dash_response(body, tvid string) ([]dash_video, error) {
	var payload struct {
		Code sourceutil.TextValue `json:"code"`
		Data struct {
			TVID    sourceutil.TextValue `json:"tvid"`
			Status  sourceutil.TextValue `json:"st"`
			Program json.RawMessage      `json:"program"`
			BossTS  json.RawMessage      `json:"boss_ts"`
			Boss    json.RawMessage      `json:"boss"`
			Content struct {
				BossStatus int `json:"bossStatus"`
			} `json:"content"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return nil, &PlaybackError{Kind: "invalid_response", message: "播放响应不是有效的 JSON 数据"}
	}
	code := string(payload.Code)
	if !playback_code_pattern.MatchString(code) {
		return nil, &PlaybackError{Kind: "invalid_response", message: "播放响应缺少有效接口状态码"}
	}
	if code != "A00000" {
		message, kind := "播放接口返回错误", "upstream"
		switch code {
		case "A00111":
			message, kind = "当前网络所在地区无法观看该视频", "region"
		case "A00006":
			message, kind = "未找到对应的视频编号", "identity"
		case "A00301", "A00302":
			message, kind = "视频为私密内容或需要访问密码", "authorization"
		}
		return nil, &PlaybackError{Kind: kind, Code: code, message: message}
	}
	if string(payload.Data.TVID) != tvid {
		return nil, &PlaybackError{Kind: "identity", Code: code, message: "播放响应的视频编号与请求的分集不匹配"}
	}
	status := 0
	if payload.Data.Status != "" {
		var err error
		status, err = strconv.Atoi(string(payload.Data.Status))
		if err != nil || status <= 0 {
			return nil, &PlaybackError{Kind: "invalid_response", Code: code, message: "播放响应的片源状态无效"}
		}
	}
	// Match the official PC player's status map. A00000 alone does not grant
	// playback: denials commonly return program:[] and an independent data.st.
	if status != 0 && (status < 100 || status >= 200) {
		kind, message := "unavailable", "未提供可用片源"
		switch status {
		case 502:
			kind, message = "region", "当前网络所在地区受版权限制，无法观看该视频"
		case 304, 401, 405, 406, 493:
			kind, message = "copyright", "因版权限制已停止提供该视频"
		case 312:
			kind, message = "unavailable", "该视频尚未上线"
		case 394, 411, 491, 501:
			kind, message = "unavailable", "该视频已失效或当前不可播放"
		case 452:
			kind, message = "authorization", "当前儿童模式不允许播放该视频"
		case 512, 601, 602, 701, 702, 706, 711:
			kind, message = "drm", "返回受 DRM 限制的片源，当前不支持下载"
		case 505, 506:
			kind, message = "authorization", "该视频仅允许在爱奇艺官方站点播放"
		case 800:
			kind, message = "authorization", "登录 Cookie 无效，请更新对应账号 Cookie"
		case 801, 802, 803:
			kind, message = "network", "播放服务暂时不可用"
		}
		return nil, &PlaybackError{Kind: kind, Code: code, Status: status, message: message}
	}
	boss_raw := payload.Data.BossTS
	if len(boss_raw) == 0 || string(boss_raw) == "null" {
		boss_raw = payload.Data.Boss
	}
	var boss struct {
		Code sourceutil.TextValue `json:"code"`
		Data struct {
			Preview sourceutil.TextValue `json:"prv"`
		} `json:"data"`
	}
	if len(boss_raw) > 0 && string(boss_raw) != "null" {
		if json.Unmarshal(boss_raw, &boss) != nil {
			return nil, &PlaybackError{Kind: "invalid_response", Code: code, Status: status, message: "播放授权响应无效"}
		}
	}
	if (boss.Code != "" && boss.Code != "A00000") || (payload.Data.Content.BossStatus > 0 && boss.Code == "") {
		if !playback_code_pattern.MatchString(string(boss.Code)) {
			boss.Code = "unknown"
		}
		return nil, &PlaybackError{Kind: "authorization", Code: string(boss.Code), Status: status, message: "账号未获完整播放授权，请检查登录、会员或对应内容权限"}
	}
	if sourceutil.Numeric(boss.Data.Preview) > 0 {
		return nil, &PlaybackError{Kind: "preview", Code: code, Status: status, message: "仅授权试看，不能作为完整分集下载"}
	}
	program := strings.TrimSpace(string(payload.Data.Program))
	if program == "" || program == "null" || program == "[]" {
		return nil, &PlaybackError{Kind: "unavailable", Code: code, Status: status, message: "播放接口未返回片源"}
	}
	var data struct {
		Video []dash_video `json:"video"`
	}
	if !strings.HasPrefix(program, "{") || json.Unmarshal([]byte(program), &data) != nil {
		return nil, &PlaybackError{Kind: "invalid_response", Code: code, Status: status, message: "播放片源结构无效"}
	}
	return data.Video, nil
}
