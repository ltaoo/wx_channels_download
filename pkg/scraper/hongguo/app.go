// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
// SPDX-License-Identifier: GPL-3.0-only
// Go adaptation of jackotom/hongguo-drama App playback and Spade; see ../NOTICE.
package hongguo

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/bits"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"wx_channel/pkg/scraper"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

var err_missing_encryption = errors.New("App 片源缺少明确的加密信息")

type encryption_info struct {
	SpadeA  string          `json:"spade_a"`
	Encrypt json.RawMessage `json:"encrypt"`
}

type video_stream struct {
	MainURL       string               `json:"main_url"`
	BackupURL     string               `json:"backup_url"`
	BackupURLList []string             `json:"backup_url_list"`
	CodecType     string               `json:"codec_type"`
	Width         sourceutil.TextValue `json:"width"`
	Height        sourceutil.TextValue `json:"height"`
	VWidth        sourceutil.TextValue `json:"vwidth"`
	VHeight       sourceutil.TextValue `json:"vheight"`
	SpadeA        string               `json:"spade_a"`
	Encrypt       json.RawMessage      `json:"encrypt"`
	EncryptInfo   *encryption_info     `json:"encrypt_info"`
	VideoMeta     *video_stream        `json:"video_meta"`
}

type app_model struct {
	VideoID       string               `json:"video_id"`
	VideoDuration sourceutil.TextValue `json:"video_duration"`
	KeySeed       string               `json:"key_seed"`
	VideoList     json.RawMessage      `json:"video_list"`
	FallbackAPI   json.RawMessage      `json:"fallback_api"`
}

func decode_base64(value string) ([]byte, error) {
	value = strings.NewReplacer("-", "+", "_", "/").Replace(strings.TrimSpace(value))
	return base64.RawStdEncoding.DecodeString(strings.TrimRight(value, "="))
}

func derive_content_key(value string) ([]byte, error) {
	raw, err := decode_base64(value)
	if err != nil || len(raw) < 3 {
		return nil, errors.New("App 视频密钥编码无效")
	}
	length := len(raw) - int(raw[0]^raw[1]^raw[2]) + 47
	if length > len(raw)-1 {
		length = len(raw) - 1
	}
	if length < 33 {
		return nil, errors.New("App 视频密钥长度无效")
	}
	work := append([]byte(nil), raw[1:1+length]...)
	a, b := byte(85), byte(246)
	for i, old := range work {
		previous := b
		if i&1 != 0 {
			previous = a
			a = old
		} else {
			b = old
		}
		work[i] = byte(-21 - bits.OnesCount(uint(i)) + int(previous^old))
	}
	key, err := hex.DecodeString(string(work[1:33]))
	if err != nil || len(key) != 16 {
		return nil, errors.New("App 视频密钥格式无效")
	}
	return key, nil
}

func decrypt_spade_url(value string, seed []byte) (string, error) {
	raw, err := decode_base64(value)
	if err != nil || len(raw) < 20 || raw[0] != 0xa8 || raw[2] != 1 || raw[3] != 0 || (len(raw)-4)%16 != 0 {
		return "", errors.New("App 播放地址编码无效")
	}
	material := derive_aes_material(seed)
	block, err := aes.NewCipher(material[:16])
	if err != nil {
		return "", err
	}
	plaintext := make([]byte, len(raw)-4)
	cipher.NewCBCDecrypter(block, material[16:32]).CryptBlocks(plaintext, raw[4:])
	padding := int(plaintext[len(plaintext)-1])
	if padding > 0 && padding <= 16 && bytes.Equal(plaintext[len(plaintext)-padding:], bytes.Repeat([]byte{byte(padding)}, padding)) {
		plaintext = plaintext[:len(plaintext)-padding]
	}
	return strings.TrimRight(string(plaintext), "\x00"), nil
}

func numeric(value sourceutil.TextValue) float64 {
	number, err := strconv.ParseFloat(string(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0
	}
	return number
}

func read_model(raw json.RawMessage) (app_model, error) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		raw = json.RawMessage(text)
	}
	var model app_model
	if json.Unmarshal(raw, &model) != nil {
		return model, errors.New("App 播放模型无效")
	}
	return model, nil
}

func read_streams(raw json.RawMessage) []video_stream {
	var rows []video_stream
	if json.Unmarshal(raw, &rows) == nil {
		return rows
	}
	var entries map[string]video_stream
	if json.Unmarshal(raw, &entries) != nil {
		return nil
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rows = append(rows, entries[key])
	}
	return rows
}

func stream_metadata(row video_stream) (string, int, int) {
	codec := strings.ToLower(row.CodecType)
	pairs := [][2]sourceutil.TextValue{{row.VWidth, row.VHeight}, {row.Width, row.Height}}
	if row.VideoMeta != nil {
		if codec == "" {
			codec = strings.ToLower(row.VideoMeta.CodecType)
		}
		pairs = append(pairs, [2]sourceutil.TextValue{row.VideoMeta.VWidth, row.VideoMeta.VHeight}, [2]sourceutil.TextValue{row.VideoMeta.Width, row.VideoMeta.Height})
	}
	for _, pair := range pairs {
		width, height := int(numeric(pair[0])), int(numeric(pair[1]))
		if width > 0 && height > 0 && width <= 16384 && height <= 16384 {
			return codec, width, height
		}
	}
	return codec, 0, 0
}

func codec_rank(codec string) int {
	switch codec {
	case "h264", "avc", "avc1":
		return 0
	case "hevc", "h265", "hvc1", "hev1", "bytevc1":
		return 1
	default:
		return 2
	}
}

func short_edge(width, height int) int {
	if width < height {
		return width
	}
	return height
}

func choose_stream(model app_model, minimum_pixels int) (scraper.PlayInfo, error) {
	duration := numeric(model.VideoDuration)
	if duration <= 0 || strings.TrimSpace(model.VideoID) == "" {
		return scraper.PlayInfo{}, errors.New("App 片源缺少视频身份或完整时长")
	}
	rows := read_streams(model.VideoList)
	sort.SliceStable(rows, func(i, j int) bool {
		codec_i, width_i, height_i := stream_metadata(rows[i])
		codec_j, width_j, height_j := stream_metadata(rows[j])
		if codec_rank(codec_i) != codec_rank(codec_j) {
			return codec_rank(codec_i) < codec_rank(codec_j)
		}
		return short_edge(width_i, height_i) > short_edge(width_j, height_j)
	})
	var seed []byte
	if model.KeySeed != "" {
		var err error
		seed, err = decode_base64(model.KeySeed)
		if err != nil || len(seed) == 0 {
			return scraper.PlayInfo{}, errors.New("App 播放地址密钥无效")
		}
	}
	best_pixels := -1
	last_error := errors.New("本集未提供尺寸可确认的 H.264/HEVC 片源（ByteVC2 暂不支持）")
	for _, row := range rows {
		codec, width, height := stream_metadata(row)
		pixels := short_edge(width, height)
		if codec_rank(codec) >= 2 || pixels <= 0 || pixels < minimum_pixels {
			continue
		}
		if best_pixels == -1 {
			best_pixels = pixels
		}
		if pixels != best_pixels {
			continue
		}
		spade, encrypt := row.SpadeA, row.Encrypt
		if row.EncryptInfo != nil {
			spade = first_text(spade, row.EncryptInfo.SpadeA)
			if len(encrypt) == 0 {
				encrypt = row.EncryptInfo.Encrypt
			}
		}
		var key []byte
		if spade != "" {
			var err error
			key, err = derive_content_key(spade)
			if err != nil {
				last_error = err
				continue
			}
		} else if string(encrypt) != "false" && string(encrypt) != "0" && string(encrypt) != `"0"` && string(encrypt) != `"false"` {
			if len(encrypt) == 0 || string(encrypt) == "null" {
				last_error = err_missing_encryption
			} else {
				last_error = errors.New("App 加密片源未提供有效解密信息")
			}
			continue
		}
		candidates := append([]string{row.MainURL, row.BackupURL}, row.BackupURLList...)
		for _, address := range candidates {
			if address == "" {
				continue
			}
			if len(seed) > 0 {
				var err error
				address, err = decrypt_spade_url(address, seed)
				if err != nil {
					last_error = err
					continue
				}
			}
			if err := checked_url(address, "qznovelvod.com", "fqnovelvod.com", "douyinvod.com"); err != nil {
				last_error = err
				continue
			}
			return scraper.PlayInfo{Address: address, Key: key, Codec: codec, Width: width, Height: height, Duration: duration}, nil
		}
	}
	return scraper.PlayInfo{}, last_error
}

func check_api_status(raw []byte) error {
	var response struct {
		Code      sourceutil.TextValue `json:"code"`
		UpperCode sourceutil.TextValue `json:"Code"`
	}
	if json.Unmarshal(raw, &response) != nil || (response.Code != "" && response.Code != "0") || (response.UpperCode != "" && response.UpperCode != "0") {
		return errors.New("App 接口暂未提供本集播放信息")
	}
	return nil
}

func (client *api_client) fallback_model(ctx context.Context, raw json.RawMessage) (app_model, error) {
	var address string
	if json.Unmarshal(raw, &address) == nil {
		var embedded struct {
			FallbackAPI string `json:"fallback_api"`
		}
		if json.Unmarshal([]byte(address), &embedded) == nil && embedded.FallbackAPI != "" {
			address = embedded.FallbackAPI
		}
	} else {
		var addresses []string
		if json.Unmarshal(raw, &addresses) == nil && len(addresses) > 0 {
			address = addresses[0]
		} else {
			var embedded struct {
				FallbackAPI string `json:"fallback_api"`
			}
			if json.Unmarshal(raw, &embedded) == nil {
				address = embedded.FallbackAPI
			}
		}
	}
	if err := checked_url(address, "fqnovel.com", "snssdk.com"); err != nil {
		return app_model{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return app_model{}, errors.New("App 备用片源地址无效")
	}
	req.Header.Set("User-Agent", app_user_agent)
	response, err := client.app_json(ctx, req)
	if err != nil {
		return app_model{}, err
	}
	if err := check_api_status(response); err != nil {
		return app_model{}, err
	}
	var payload struct {
		VideoInfo json.RawMessage `json:"video_info"`
	}
	if json.Unmarshal(response, &payload) != nil || check_api_status(payload.VideoInfo) != nil {
		return app_model{}, errors.New("App 备用片源响应无效")
	}
	var inner struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(payload.VideoInfo, &inner) != nil {
		return app_model{}, errors.New("App 备用片源响应无效")
	}
	return read_model(inner.Data)
}

func (client *api_client) app_media(ctx context.Context, vid string) (scraper.PlayInfo, error) {
	if !id_pattern.MatchString(vid) {
		return scraper.PlayInfo{}, errors.New("分集编号无效")
	}
	identity, err := client.register_device(ctx)
	if err != nil {
		return scraper.PlayInfo{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	body, err := json.Marshal(map[string]any{
		"biz_param":          map[string]any{"detail_page_version": 0, "device_level": 3, "disable_digg_stat": false, "need_all_video_definition": true, "need_mp4_align": false, "use_os_player": false, "use_server_dns": false, "video_platform": 1024},
		"mixed_video_id_map": map[string][]string{"1004": {vid}},
	})
	if err != nil {
		return scraper.PlayInfo{}, err
	}
	var raw []byte
	for attempt, host := range []string{"api5-normal-sinfonlineb.fqnovel.com", "api5-normal-hl.fqnovel.com"} {
		now := time.Now()
		params, err := new_signature_params(now)
		if err != nil {
			return scraper.PlayInfo{}, err
		}
		req, err := playback_request(body, identity, host, now, params)
		if err != nil {
			return scraper.PlayInfo{}, err
		}
		raw, err = client.app_json(ctx, req)
		if err == nil {
			break
		}
		if attempt != 0 || !errors.Is(err, err_empty_response) {
			return scraper.PlayInfo{}, err
		}
		// Playback identities may be single use; retry once on the official alternate host.
		if renewed, renew_err := client.register_device(ctx); renew_err == nil {
			identity = renewed
		}
	}
	if err := check_api_status(raw); err != nil {
		return scraper.PlayInfo{}, err
	}
	var payload struct {
		Data map[string]struct {
			VideoModel json.RawMessage `json:"video_model"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &payload) != nil || len(payload.Data[vid].VideoModel) == 0 {
		return scraper.PlayInfo{}, errors.New("App 接口未返回请求的分集模型")
	}
	model, err := read_model(payload.Data[vid].VideoModel)
	if err != nil {
		return scraper.PlayInfo{}, err
	}
	if len(read_streams(model.VideoList)) == 0 {
		fallback, err := client.fallback_model(ctx, model.FallbackAPI)
		if err != nil {
			return scraper.PlayInfo{}, err
		}
		return choose_stream(fallback, 0)
	}
	info, err := choose_stream(model, 0)
	if !errors.Is(err, err_missing_encryption) {
		return info, err
	}
	if model.VideoID == "" || numeric(model.VideoDuration) <= 0 || len(model.FallbackAPI) == 0 {
		return scraper.PlayInfo{}, err
	}
	fallback, fallback_err := client.fallback_model(ctx, model.FallbackAPI)
	if fallback_err != nil {
		return scraper.PlayInfo{}, fallback_err
	}
	if fallback.VideoID != model.VideoID || math.Abs(numeric(fallback.VideoDuration)-numeric(model.VideoDuration)) > 0.5 {
		return scraper.PlayInfo{}, errors.New("App 备用片源身份或完整时长不匹配")
	}
	minimum_pixels := 0
	for _, row := range read_streams(model.VideoList) {
		_, width, height := stream_metadata(row)
		if pixels := short_edge(width, height); pixels > minimum_pixels {
			minimum_pixels = pixels
		}
	}
	return choose_stream(fallback, minimum_pixels)
}
