// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
// SPDX-License-Identifier: GPL-3.0-only
// Go port of the playback signature in jackotom/hongguo-drama; see ../NOTICE.
package hongguo

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type signature_params struct {
	nonce          string
	started        int64
	gorgon_random  uint16
	helios_random  uint32
	message_random uint32
	env_launch     uint32
	env_pid        uint32
	medusa_random  uint32
	xm_random      uint32
}

func new_signature_params(now time.Time) (signature_params, error) {
	data := make([]byte, 44)
	if _, err := rand.Read(data); err != nil {
		return signature_params{}, err
	}
	return signature_params{
		nonce: base64.RawURLEncoding.EncodeToString(data[:18]), started: now.UnixMilli(),
		gorgon_random: binary.LittleEndian.Uint16(data[18:]), helios_random: binary.LittleEndian.Uint32(data[20:]),
		message_random: binary.LittleEndian.Uint32(data[24:]), env_launch: 100 + binary.LittleEndian.Uint32(data[28:])%21,
		env_pid:       10001 + binary.LittleEndian.Uint32(data[32:])%2000,
		medusa_random: binary.LittleEndian.Uint32(data[36:]), xm_random: binary.LittleEndian.Uint32(data[40:]),
	}, nil
}

func hex_bytes(value string) []byte {
	data, err := hex.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return data
}

func le32(value uint32) []byte { return binary.LittleEndian.AppendUint32(nil, value) }
func be32(value uint32) []byte { return binary.BigEndian.AppendUint32(nil, value) }

func proto(fields ...[]byte) []byte { return bytes.Join(fields, nil) }

func proto_bytes(tag int, value []byte) []byte {
	data := binary.AppendUvarint(nil, uint64(tag<<3|2))
	data = binary.AppendUvarint(data, uint64(len(value)))
	return append(data, value...)
}

func proto_string(tag int, value string) []byte {
	if value == "" {
		return nil
	}
	return proto_bytes(tag, []byte(value))
}

func proto_sint(tag int, value int64) []byte {
	data := binary.AppendUvarint(nil, uint64(tag<<3))
	return binary.AppendUvarint(data, uint64(value<<1)^uint64(value>>63))
}

func get_iv(iv uint32, data []byte) uint32 {
	for i, value := range data {
		if i&1 == 0 {
			iv = (iv >> 4) ^ iv ^ (iv << 6) ^ uint32(value)
		} else {
			iv = ^((iv >> 7) ^ iv ^ (uint32(value) | (iv << 12)))
		}
	}
	return iv
}

func sum_md5(data []byte) uint32 {
	check := uint32(0x20220420)
	for i := 0; i < 12; i++ {
		if i&1 == 0 {
			check = (uint32(data[i]) ^ (check << 7)) ^ ((check >> 3) ^ check)
		} else {
			check = ^(uint32(data[i]) | (check << 11)) ^ ((check >> 5) ^ check)
		}
	}
	return (check | 4) ^ 0x1000000
}

func signature_branch(query string, body []byte, timestamp uint32) int {
	body_md5 := md5.Sum(body)
	iv := get_iv(get_iv(get_iv(0x20230928, sm3([]byte(query))), body_md5[:]), le32(timestamp))
	return int(iv&15) % 3
}

func hash_f13(query_sm3, body_md5 []byte, timestamp uint32) []byte {
	iv := get_iv(get_iv(get_iv(0x20230928, query_sm3), body_md5), le32(timestamp))
	index := int(iv&15) / 3
	iv_v1 := [...]uint32{0xc4a78580, 0xb3c0fd39, 0xc58c5686, 0xc9aa3ba7, 0xf5a7adf2, 0x963c2ed1}[index]
	count_v2 := iv_v1 + timestamp
	constants := []uint32{0xebb64faf, 0x7aadcc2, 0xcf3187bf, 0xe01138ff, 0x6d0bfcff, 0x5a30a3be, 0xb41ad638, 0x34180eb8, 0xf233eb6f, 0xb1a584cc, 0xccc30dc7, 0x47d1db51, 0xd55653de, 0x70a84fa1, 0x57473c12, 0xf76f0288, 0x2c077f0a, 0xda0dcad0, 0xfbb86f6c, 0xfdc4cf00, 0x688a020d, 0xe676c6a6, 0x8cd6338b, 0x1a3c8d0e, 0xcce8b06b, 0x6ad0ed0b, 0xa0522717, 0xdc71ac83, 0x2285db71, 0xd5b4dda6, 0x736f8650, 0x6560306c, 0x617ce2a6, 0xe423417e, 0xa40e143, 0x544e4032, 0x88dffb2a, 0x716c1ae0, 0x4c467a88, 0x5b23bb3, 0xe1d0b866, 0xbaa3dcb8, 0xae3374d3, 0xc3381a50, 0x1702f75b, 0xfe6da368, 0xf0b4cf48, 0x4e0ffbb8, 0x72aad10d, 0x26c53a3d, 0xf2bce0f6, 0xb4557581, 0x4a257fdd, 0x8c3182a2, 0xab0b3b86, 0x3d5dfb14, 0x4f103634, 0xd37b52d7, 0x444eff16, 0xeb0a33d1, 0x6ca86f6e, 0x284ba7, 0x8387cfa, 0x5fb37586}
	for i := range constants {
		constants[i] = bits.RotateLeft32(constants[i], -int((count_v2+1)&31))
	}
	pad := []byte{0xfa, 0x45, 0x61, 0xd7}
	for i, value := range pad {
		pad[i] = byte((uint32(value) | uint32(value)<<8) >> ((count_v2 + 2) & 7))
	}
	data := proto(query_sm3, body_md5, le32(timestamp), pad, hex_bytes("00000000000001a0"))
	var words [128]uint32
	for i := 0; i < 16; i++ {
		words[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	for i := 0; i < 112; i++ {
		r1 := bits.RotateLeft32(words[i+1], 14) ^ bits.RotateLeft32(words[i+1], 25) ^ (words[i+1] >> 3)
		r2 := bits.RotateLeft32(words[i+14], 13) ^ bits.RotateLeft32(words[i+14], 15) ^ (words[i+14] >> 10)
		words[i+16] = words[i] + words[i+9] + r1 + r2
	}
	initial := [8]uint32{0x7aba4fc8, 0x67166507, 0x6403fa00, 0x340f512f, 984304912, 3005047866, 2874125293, 2152413264}
	for i := range initial {
		initial[i] = bits.RotateLeft32(initial[i], -int(count_v2&31))
	}
	chosen := [6][11]int{{101, 5, 7, 6, 3, 2, 1, 0, 5, 4, 3}, {96, 0, 6, 7, 5, 3, 2, 1, 5, 4, 4}, {96, 7, 6, 2, 1, 4, 0, 5, 4, 3, 5}, {99, 3, 6, 2, 4, 5, 1, 0, 0, 7, 6}, {96, 0, 5, 6, 7, 3, 1, 2, 5, 4, 4}, {100, 2, 0, 3, 5, 4, 6, 7, 2, 1, 5}}[index]
	d := initial
	for i := 0; i < chosen[0]; i++ {
		base := iv_v1 + uint32(i)
		n1 := ((d[chosen[3]] ^ d[chosen[4]]) & d[chosen[1]]) ^ d[chosen[3]]
		n2 := bits.RotateLeft32(d[chosen[1]], 26) ^ bits.RotateLeft32(d[chosen[1]], 21) ^ bits.RotateLeft32(d[chosen[1]], 7)
		n4 := words[base&127] + n1 + n2 + constants[base&63] + d[chosen[5]]
		n5 := bits.RotateLeft32(d[chosen[2]], 30) ^ bits.RotateLeft32(d[chosen[2]], 19) ^ bits.RotateLeft32(d[chosen[2]], 10)
		n6 := (d[chosen[2]] & d[chosen[6]]) | ((d[chosen[2]] | d[chosen[6]]) & d[chosen[7]])
		old, last := d[chosen[9]], d[7]
		copy(d[1:], d[:7])
		d[0] = last
		d[chosen[10]], d[chosen[8]] = n5+n6+n4, old+n4
	}
	ret := make([]byte, 32)
	for i := range d {
		binary.BigEndian.PutUint32(ret[i*4:], d[i]+initial[i])
	}
	for i := 0; i < 16; i++ {
		ret[i] ^= ret[i+16]
	}
	return append(ret[:16:16], le32(sum_md5(ret[:16]))...)
}

func x_gorgon(query string, body []byte, timestamp uint32, random uint16) string {
	query_md5, body_md5 := md5.Sum([]byte(query)), md5.Sum(body)
	input := proto(query_md5[:4], body_md5[:4], make([]byte, 4), le32(67503104), be32(timestamp))
	key := []byte{0x4a, 0x40, 0x16, byte(random >> 8), 0x47, 0x6c, 0x01, byte(random)}
	var table [256]byte
	for i := range table {
		table[i] = byte(i)
	}
	j := 0
	for i := range table {
		j = (j + int(table[i]) + int(key[i%len(key)])) & 255
		table[i] = table[j]
	}
	j = 0
	out := make([]byte, len(input))
	for i, value := range input {
		x := table[i+1]
		j += int(x)
		y := table[j&255]
		table[i+1] = y
		out[i] = value ^ table[(int(y)+int(y))&255]
	}
	for i, value := range out {
		value = value>>4 | value<<4
		next := out[0]
		if i+1 < len(out) {
			next = out[i+1]
		}
		out[i] = ^(bits.Reverse8(next^value) ^ 20)
	}
	return hex.EncodeToString(proto([]byte{0x84, 0x04, byte(random), byte(random >> 8), 0x40, 0x01}, out))
}

func helios(timestamp, random uint32) string {
	digest := md5.Sum(append(le32(random), []byte("8662")...))
	ascii := []byte(hex.EncodeToString(digest[:]))
	words := []uint64{binary.LittleEndian.Uint64(ascii[16:]), binary.LittleEndian.Uint64(ascii[24:])}
	b0, b8 := binary.LittleEndian.Uint64(ascii), binary.LittleEndian.Uint64(ascii[8:])
	table := []uint64{b0}
	for i := 0; i < 34; i++ {
		x8 := (bits.RotateLeft64(b8, -8) + b0) ^ uint64(i)
		words = append(words, x8)
		x8 ^= bits.RotateLeft64(b0, -61)
		table = append(table, x8)
		b0, b8, words = x8, words[0], words[1:]
	}
	text := []byte(fmt.Sprintf("%d-1588093228-8662", timestamp))
	padding := bytes.Repeat([]byte{byte(16 - len(text)%16)}, 16-len(text)%16)
	data := append(text, padding...)
	output := le32(random)
	for at := 0; at < len(data); at += 16 {
		a, b := binary.LittleEndian.Uint64(data[at:]), binary.LittleEndian.Uint64(data[at+8:])
		for i := 0; i < 34; i++ {
			b = table[i] ^ (a + bits.RotateLeft64(b, -8))
			a = b ^ bits.RotateLeft64(a, -61)
		}
		output = binary.LittleEndian.AppendUint64(output, a)
		output = binary.LittleEndian.AppendUint64(output, b)
	}
	return base64.StdEncoding.EncodeToString(output)
}

func device_proto(device_id string, params signature_params) []byte {
	fields := [][]byte{proto_sint(1, 1), proto_sint(2, 2), proto_string(3, "8662"), proto_string(4, device_id), proto_string(5, params.nonce),
		proto_string(6, "!noperm!"), proto_sint(7, -888888), proto_sint(8, -888888), proto_sint(9, 3), proto_sint(10, -888888),
		proto_string(11, "!notset!"), proto_string(12, "Asia/Shanghai,8"), proto_string(13, "zh_CN"), proto_sint(14, 4)}
	for tag := 16; tag <= 21; tag++ {
		fields = append(fields, append(binary.AppendUvarint(nil, uint64(tag<<3|5)), make([]byte, 4)...))
	}
	fields = append(fields, proto_string(22, "16"), proto_sint(23, 41), proto_sint(24, 36))
	for tag := 25; tag <= 28; tag++ {
		fields = append(fields, proto_sint(tag, params.started))
	}
	fields = append(fields, proto_sint(29, -1), proto_string(30, "25053RT47C"), proto_string(31, "Redmi"), proto_string(32, "25053RT47C"),
		proto_string(33, "25053RT47C"), proto_string(34, "Xiaomi"), proto_string(35, "Redmi"), proto_string(36, "Redmi"), proto_sint(38, 31))
	return proto(fields...)
}

func xm_xor(data, key []byte) []byte {
	encoded := make([]byte, len(data))
	for i, value := range data {
		at := (i * 4) & 28
		d0, d1 := key[at], key[at+1]
		d2 := ^((value<<4 | value>>4) + d0) ^ d1
		d2 = bits.RotateLeft8(d2, 3)
		d2 = (d2 + d1) ^ d0
		encoded[len(data)-i-1] = ^d2
	}
	last, first := encoded[len(encoded)-1]^encoded[len(encoded)-2], encoded[0]
	encoded[0] = ^last + first
	encoded[1] += encoded[0] ^ encoded[len(encoded)-1] ^ 254
	encoded[2] += (last - first) ^ bits.RotateLeft8(encoded[1], 3) ^ 2
	for i := 0; i < len(encoded)-4; i++ {
		temp := bits.RotateLeft8(encoded[i+2], 3) ^ encoded[i+1] ^ byte(i+3)
		encoded[i+3] += ^temp
	}
	encoded[len(encoded)-1] ^= encoded[len(encoded)-2]
	sum := byte(0)
	for _, value := range encoded[1:] {
		sum += value
	}
	encoded[0] = (encoded[0] ^ encoded[1]) + sum
	return encoded
}

func build_medusa(query string, body []byte, timestamp uint32, device_id string, params signature_params) string {
	body_md5 := md5.Sum(body)
	query_sm3 := sm3([]byte(query))
	query_body_ts := hash_f13(query_sm3, body_md5[:], timestamp)
	nested := proto(proto_sint(1, 111), proto_sint(2, 10), proto_sint(3, 694367), proto_sint(5, 586952199))
	env := proto(proto_sint(1, int64(params.env_launch)), proto_sint(2, 146331399), proto_sint(3, 146331396), proto_sint(5, 7),
		proto_string(6, "v04.06.04.03-bugfix"), proto_sint(7, int64(params.env_pid)), proto_bytes(12, device_proto(device_id, params)),
		proto_bytes(13, proto(proto_sint(1, int64(timestamp)), proto_sint(2, -2), proto_sint(4, 200))), proto_string(14, "7.1.3.32"))
	query_hash := sm3(proto([]byte(query), body_md5[:], []byte("none")))
	message := proto(proto_bytes(1, hex_bytes("f7e85ffad7d7dc3bd62ac87057cf6118")), proto_sint(2, 3), proto_sint(3, int64(params.message_random)),
		proto_string(4, "8662"), proto_string(5, device_id), proto_string(6, "1588093228"), proto_string(7, "7.1.3.32"),
		proto_string(8, "v04.06.04-ml-android"), proto_sint(9, 67503104), proto_bytes(10, hex_bytes("4001000000000000")), proto_sint(12, int64(timestamp)),
		proto_bytes(13, query_body_ts), proto_bytes(14, query_sm3[:6]), proto_bytes(15, nested), proto_string(16, params.nonce),
		proto_sint(17, int64(timestamp)), proto_bytes(19, query_hash), proto_string(20, "none"), proto_sint(21, 312), proto_bytes(23, env),
		proto_string(24, `{"cmr":16777216,"cmr2":16777216,"un_h":1879194040,"vpn":0,"kd":0,"fkd":3672518972,"pd":-1872573247,"dyn":"","do":0,"tk":true}`))
	sign_key := hex_bytes("8ebdfa3806ecc5cee79423e6029ed82540bc2218bb7eaef71cb691f7aa8aa2f5")
	hash := sm3(proto(sign_key, le32(params.medusa_random), sign_key))
	d1 := (params.medusa_random >> 16) & 255
	seed := le32(^((d1 << 11) | (params.medusa_random >> 24) ^ (d1 >> 5) ^ d1))
	transformed := proto(hex_bytes("4001000000000000"), xm_xor(message, hash))
	for i, j := 0, len(transformed)-1; i < j; i, j = i+1, j-1 {
		transformed[i], transformed[j] = transformed[j], transformed[i]
	}
	for i := range transformed {
		transformed[i] ^= seed[(^i)&3]
	}
	check := (uint32(query_sm3[0]&63) << 14) | 0x18000001 | (uint32(query_body_ts[0]&63) << 8)
	packed := proto([]byte{0x35}, le32(params.xm_random), le32(check), transformed, []byte{byte(params.medusa_random >> 16), byte(params.medusa_random >> 24)})
	encrypted := medusa_encrypt(packed, timestamp)
	prefix := hex_bytes("03000000f7e85ffad7d7dc3bd62ac87057cf6118")
	for i := 0; i < len(prefix); i += 4 {
		binary.LittleEndian.PutUint32(prefix[i:], binary.LittleEndian.Uint32(prefix[i:])^timestamp)
	}
	return base64.StdEncoding.EncodeToString(proto(prefix, []byte{byte(params.medusa_random), byte(params.medusa_random >> 8), 0, 1}, encrypted))
}

func query_quote(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(value), "+", "%20"), "%2A", "*")
}

func playback_request(body []byte, identity device_identity, host string, now time.Time, params signature_params) (*http.Request, error) {
	timestamp := uint32(now.Unix())
	var query string
	var ticket int64
	for attempt := 0; attempt < 128; attempt++ {
		ticket = now.UnixMilli() + int64(attempt)
		fields := [][2]string{
			{"ac", "wifi"}, {"channel", "update_64"}, {"aid", "8662"}, {"app_name", "novelread"},
			{"version_code", "71332"}, {"version_name", "7.1.3.32"}, {"device_platform", "android"}, {"os", "android"},
			{"ssmix", "a"}, {"device_type", "25053RT47C"}, {"device_brand", "Redmi"}, {"language", "zh"}, {"os_api", "36"},
			{"os_version", "16"}, {"manifest_version_code", "71332"}, {"resolution", "1280*2772"}, {"dpi", "520"},
			{"update_version_code", "71332"}, {"host_abi", "arm64-v8a"}, {"dragon_device_type", "phone"}, {"pv_player", "71332"},
			{"compliance_status", "0"}, {"need_personal_recommend", "1"}, {"player_so_load", "1"}, {"is_android_pad_screen", "0"},
			{"device_id", identity.device_id}, {"iid", identity.iid}, {"cdid", identity.cdid},
			{"ts", strconv.FormatUint(uint64(timestamp), 10)}, {"_rticket", strconv.FormatInt(ticket, 10)},
		}
		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			parts = append(parts, query_quote(field[0])+"="+query_quote(field[1]))
		}
		query = strings.Join(parts, "&")
		if signature_branch(query, body, timestamp) == 0 {
			break
		}
		if attempt == 127 {
			return nil, errors.New("无法生成播放请求签名")
		}
	}
	req, err := http.NewRequest(http.MethodPost, "https://"+host+"/novel/player/multi_video_model/v1/?"+query, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range map[string]string{
		"User-Agent": app_user_agent, "Accept": "application/json; charset=utf-8,application/x-protobuf", "Content-Type": "application/json; charset=UTF-8",
		"x-ss-req-ticket": strconv.FormatInt(ticket, 10), "sdk-version": "2", "x-xs-from-web": "0", "x-khronos": fmt.Sprint(timestamp),
		"x-gorgon": x_gorgon(query, body, timestamp, params.gorgon_random), "x-ladon": base64.StdEncoding.EncodeToString(be32(timestamp)),
		"x-argus": base64.StdEncoding.EncodeToString(le32(timestamp)), "x-helios": helios(timestamp, params.helios_random),
		"x-medusa": build_medusa(query, body, timestamp, identity.device_id, params), "x-tt-dt": "", "x-tt-store-region": "cn", "x-tt-store-region-src": "did",
	} {
		req.Header.Set(key, value)
	}
	return req, nil
}
