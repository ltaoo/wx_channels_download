// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
// SPDX-License-Identifier: GPL-3.0-only
// Anonymous registration ported from jackotom/hongguo-drama; see ../NOTICE.
package hongguo

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const register_url = "https://log5-applog-lf.fqnovel.com/service/2/device_register/?tt_data=a&cronet_version=8d40f833_2026-03-03&ttnet_version=4.2.243.31-douyin&use_store_region_cookie=1"
const register_user_agent = "com.phoenix.read/72232 (Linux; U; Android 16; zh_CN; 25053RT47C; Build/BP2A.250605.031.A3)"
const app_user_agent = "com.phoenix.read/71532 (Linux; U; Android 9; SM-N9860; Build/PQ3A.190705.10241111;tt-ok/3.12.13.20)"

var err_empty_response = errors.New("App 接口返回空响应，可能是签名或匿名设备鉴权问题")

type device_identity struct {
	device_id string
	iid       string
	cdid      string
}

func random_uuid() string {
	var data [16]byte
	_, _ = rand.Read(data[:])
	data[6], data[8] = (data[6]&15)|64, (data[8]&63)|128
	return fmt.Sprintf("%x-%x-%x-%x-%x", data[:4], data[4:6], data[6:8], data[8:10], data[10:])
}

func derive_aes_material(seed []byte) []byte {
	salt := hex_bytes("4dd4c2e6b83162090e52b3c7a6733ba41cb2462b829ab58a196b39db57177524f49baf7f08e8d68d26a72e37c1a95a2f1f05a51892aef2949732b62a38aadd58")
	digest := sha512.Sum512(seed)
	derived := sha512.Sum512(append(digest[:], salt...))
	return derived[:]
}

func encode_registration(payload, random []byte) ([]byte, error) {
	if len(random) != 32 {
		return nil, errors.New("匿名注册随机数据长度错误")
	}
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(payload); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	compressed := buffer.Bytes()
	digest := sha512.Sum512(compressed)
	plaintext := append(digest[:], compressed...)
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	plaintext = append(plaintext, bytes.Repeat([]byte{byte(padding)}, padding)...)
	material := derive_aes_material(random)
	block, err := aes.NewCipher(material[:16])
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, material[16:32]).CryptBlocks(ciphertext, plaintext)
	return proto(hex_bytes("746305100000"), random, ciphertext), nil
}

func checked_url(address string, domains ...string) error {
	link, err := url.Parse(address)
	if err == nil && link.Scheme == "https" && link.User == nil && (link.Port() == "" || link.Port() == "443") && link.Fragment == "" {
		for _, domain := range domains {
			if link.Hostname() == domain || strings.HasSuffix(link.Hostname(), "."+domain) {
				return nil
			}
		}
	}
	return errors.New("App 返回了不受支持的服务地址")
}

func (client *api_client) app_json(ctx context.Context, req *http.Request) ([]byte, error) {
	if err := checked_url(req.URL.String(), "fqnovel.com", "snssdk.com"); err != nil {
		return nil, err
	}
	http_client := *client.HTTPClient
	http_client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	response, err := http_client.Do(req.WithContext(ctx))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("App 接口网络请求失败，请稍后重试")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("App 接口返回 HTTP %d", response.StatusCode)
	}
	const max_size = 2 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, max_size+1))
	if err != nil {
		return nil, errors.New("App 接口响应读取失败")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, err_empty_response
	}
	if len(data) > max_size || !json.Valid(data) {
		return nil, errors.New("App 接口返回无效 JSON 或响应过大")
	}
	return data, nil
}

func (client *api_client) register_device(ctx context.Context) (device_identity, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	now, cdid := time.Now().UnixMilli(), random_uuid()
	var random [32]byte
	_, _ = rand.Read(random[:])
	header := map[string]any{
		"display_name": "红果免费短剧", "update_version_code": 73932, "manifest_version_code": 73932,
		"app_version_minor": "7.3.9.32", "aid": 8662, "channel": "update_64", "package": "com.phoenix.read",
		"app_version": "7.3.9.32", "version_code": 73932, "sdk_version": "3.7.0-rc.34-fanqie-xiaoshuo",
		"sdk_target_version": 29, "git_hash": "3a88f4e", "os": "Android", "os_version": "16", "os_api": 36,
		"device_model": "25053RT47C", "device_brand": "Redmi", "device_manufacturer": "Redmi", "cpu_abi": "arm64-v8a",
		"release_build": "b5e187f", "density_dpi": 520, "display_density": "mdpi", "resolution": "1280x2772",
		"language": "zh", "timezone": 8, "access": "wifi", "not_request_sender": 0, "carrier": "CHINA MOBILE", "mcc_mnc": "46000",
		"rom": "eng.root.20231116.160340", "rom_version": "W528JS release-keys", "cdid": cdid, "sig_hash": "56a962410c494bbaf0b58dba20cae56f",
		"openudid": hex.EncodeToString(random[:8]), "clientudid": random_uuid(), "ipv6_list": []string{}, "region": "CN",
		"tz_name": "Asia/Shanghai", "tz_offset": 28800, "sim_region": "cn", "sim_serial_number": []string{}, "oaid_may_support": false,
		"req_id": random_uuid(), "device_platform": "android", "pre_installed_channel": "", "apk_first_install_time": now,
		"is_system_app": 0, "sdk_flavor": "china", "guest_mode": 0,
		"custom": map[string]any{"account_region_src": "fallback", "account_region": "cn", "host_bit": 64, "dragon_device_type": "phone"},
	}
	payload, err := json.Marshal(map[string]any{"magic_tag": "ss_app_log", "header": header, "_gen_time": now})
	if err != nil {
		return device_identity{}, err
	}
	data, err := encode_registration(payload, random[:])
	if err != nil {
		return device_identity{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, register_url, bytes.NewReader(data))
	if err != nil {
		return device_identity{}, err
	}
	digest := md5.Sum(data)
	for key, value := range map[string]string{
		"User-Agent": register_user_agent, "Accept": "application/json", "Content-Type": "application/octet-stream;tt-data=a",
		"log-encode-type": "gzip", "x-ss-dp": "8662", "x-ss-req-ticket": fmt.Sprint(now), "x-vc-bdturing-sdk-version": "4.0.3.cn",
		"lc": "101", "sdk-version": "2", "passport-sdk-version": "5051452", "x-neptune": "-8|50:51:59:30:40:47:49:39",
		"x-ss-stub": strings.ToUpper(hex.EncodeToString(digest[:])),
	} {
		req.Header.Set(key, value)
	}
	raw, err := client.app_json(ctx, req)
	if err != nil {
		return device_identity{}, fmt.Errorf("匿名播放设备注册失败：%w", err)
	}
	var response struct {
		Code         sourceutil.TextValue `json:"code"`
		UpperCode    sourceutil.TextValue `json:"Code"`
		DeviceID     sourceutil.TextValue `json:"device_id"`
		DeviceIDStr  sourceutil.TextValue `json:"device_id_str"`
		InstallID    sourceutil.TextValue `json:"install_id"`
		InstallIDStr sourceutil.TextValue `json:"install_id_str"`
	}
	if json.Unmarshal(raw, &response) != nil || (response.Code != "" && response.Code != "0") || (response.UpperCode != "" && response.UpperCode != "0") {
		return device_identity{}, errors.New("匿名设备注册被拒绝或返回数据无效")
	}
	device_id, iid := first_text(string(response.DeviceIDStr), string(response.DeviceID)), first_text(string(response.InstallIDStr), string(response.InstallID))
	if !id_pattern.MatchString(device_id) || !id_pattern.MatchString(iid) || strings.Trim(device_id, "0") == "" || strings.Trim(iid, "0") == "" {
		return device_identity{}, errors.New("匿名设备注册未返回有效设备编号")
	}
	return device_identity{device_id: device_id, iid: iid, cdid: cdid}, nil
}
