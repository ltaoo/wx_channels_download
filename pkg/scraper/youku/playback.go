package youku

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	sourceutil "wx_channel/pkg/scraper/internal/source"
)

func (client *backend) load_playback(ctx context.Context, id string) (youku_playback, error) {
	if err := ctx.Err(); err != nil {
		return youku_playback{}, err
	}
	response, err := client.Response(ctx, "https://log.mmstat.com/eg.js", true)
	if err != nil {
		return youku_playback{}, err
	}
	utid := strings.Trim(response.Header.Get("ETag"), `"`)
	response.Body.Close()
	if utid == "" || len(utid) > 1024 {
		return youku_playback{}, errors.New("优酷未返回有效 utid，无法获取播放信息")
	}
	params := url.Values{"vid": {id}, "ccode": {"0564"}, "client_ip": {"192.168.1.1"}, "utid": {utid}, "client_ts": {strconv.FormatInt(time.Now().Unix(), 10)}}
	body, err := client.GetText(ctx, "https://ups.youku.com/ups/get.json?"+params.Encode())
	if err != nil {
		return youku_playback{}, err
	}
	var payload youku_playback
	if json.Unmarshal([]byte(body), &payload) != nil {
		return youku_playback{}, errors.New("优酷播放响应不是有效 JSON")
	}
	code := sourceutil.FirstText(youku_error_code(payload.Data.Error), youku_error_code(payload.Error))
	if code != "" {
		if code == "-6004" {
			payload, err := client.load_mtop_playback(ctx, id, utid)
			if err != nil {
				return youku_playback{}, fmt.Errorf("优酷新版主动提取失败（旧接口 code=-6004）：%w", err)
			}
			return payload, nil
		}
		return youku_playback{}, fmt.Errorf("优酷播放接口返回错误（code=%s），请检查官网实际播放状态", code)
	}

	return payload, nil
}

func playback_matches(payload youku_playback, id string) bool {
	video := payload.Data.Video
	if video.EncodeID != "" {
		return video.EncodeID == id
	}
	if video.ID == "" {
		return false
	}
	raw, err := sourceutil.DecodeBase64(strings.TrimPrefix(id, "X"))
	number, number_err := strconv.ParseUint(string(raw), 10, 64)
	return err == nil && number_err == nil && number%4 == 0 && strconv.FormatUint(number/4, 10) == string(video.ID)
}
