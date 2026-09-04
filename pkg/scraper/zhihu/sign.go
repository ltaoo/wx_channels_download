package zhihu

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
)

var zse_round_keys = []uint32{
	1170614578, 1024848638, 1413669199, 3951632832, 3528873006, 2921909214, 4151847688, 3997739139, 1933479194, 3323781115, 3888513386, 460404854, 3747539722, 2403641034, 2615871395,
	2119585428, 2265697227, 2035090028, 2773447226, 4289380121, 4217216195, 2200601443, 3051914490, 1579901135, 1321810770, 456816404, 2903323407, 4065664991, 330002838, 3506006750,
	363569021, 2347096187,
}

var zse_s_box = []uint8{
	20, 223, 245, 7, 248, 2, 194, 209, 87, 6, 227, 253, 240, 128, 222, 91, 237, 9, 125, 157, 230, 93, 252, 205, 90, 79, 144, 199, 159, 197, 186, 167, 39, 37, 156, 198, 38, 42, 43, 168, 217, 153, 15, 103, 80, 189, 71, 191, 97, 84,
	247, 95, 36, 69, 14, 35, 12, 171, 28, 114, 178, 148, 86, 182, 32, 83, 158, 109, 22, 255, 94, 238, 151, 85, 77, 124, 254, 18, 4, 26, 123, 176, 232, 193, 131, 172, 143, 142, 150, 30, 10, 146, 162, 62, 224, 218, 196, 229, 1,
	192, 213, 27, 110, 56, 231, 180, 138, 107, 242, 187, 54, 120, 19, 44, 117, 228, 215, 203, 53, 239, 251, 127, 81, 11, 133, 96, 204, 132, 41, 115, 73, 55, 249, 147, 102, 48, 122, 145, 106, 118, 74, 190, 29, 16, 174, 5, 177,
	129, 63, 113, 99, 31, 161, 76, 246, 34, 211, 13, 60, 68, 207, 160, 65, 111, 82, 165, 67, 169, 225, 57, 112, 244, 155, 51, 236, 200, 233, 58, 61, 47, 100, 137, 185, 64, 17, 70, 234, 163, 219, 108, 170, 166, 59, 149, 52, 105,
	24, 212, 78, 173, 45, 0, 116, 226, 119, 136, 206, 135, 175, 195, 25, 92, 121, 208, 126, 139, 3, 75, 141, 21, 130, 98, 241, 40, 154, 66, 184, 49, 181, 46, 243, 88, 101, 183, 8, 23, 72, 188, 104, 179, 210, 134, 250, 201, 164,
	89, 216, 202, 220, 50, 221, 152, 140, 33, 235, 214,
}

// RefreshZSEHeaders replaces a page-generated Zhihu signature using the
// d_c0 cookie already carried by the request.
func RefreshZSEHeaders(request *http.Request) error {
	if request == nil || request.URL == nil || request.Header == nil {
		return fmt.Errorf("invalid request")
	}
	if !strings.EqualFold(request.URL.Hostname(), "www.zhihu.com") || request.Header.Get("X-Zse-96") == "" {
		return nil
	}
	d_c0, err := request.Cookie("d_c0")
	if err != nil {
		return fmt.Errorf("read d_c0 cookie: %w", err)
	}
	for name, value := range build_zse_headers(request.URL.RequestURI(), d_c0.Value) {
		request.Header.Set(name, value)
	}
	return nil
}

func build_zse_headers(api_path string, d_c0 string) map[string]string {
	const x_zse_93 = "101_3_3.0"
	digest := md5.Sum([]byte(x_zse_93 + "+" + api_path + "+" + d_c0))
	return map[string]string{
		"X-Zse-93": x_zse_93,
		"X-Zse-96": "2.0_" + encrypt_zse(hex.EncodeToString(digest[:])),
	}
}

func encrypt_zse(md5_text string) string {
	processed := preprocess_zse(md5_text)
	var current uint32
	var result string
	for index := 0; index < len(processed); index++ {
		value := processed[len(processed)-index-1]
		value ^= uint8((uint32(58) >> (8 * uint(index%4))) & 255)
		current |= uint32(value) << (8 * uint(index%3))
		if index%3 == 2 {
			result += encode_zse(current)
			current = 0
		}
	}
	return result
}

func preprocess_zse(md5_text string) []uint8 {
	data := append([]uint8{uint8(rand.Intn(127)), 0}, []byte(md5_text)...)
	for index := 0; index < 15; index++ {
		data = append(data, 14)
	}
	fixed := []uint8{48, 53, 57, 48, 53, 51, 102, 55, 100, 49, 53, 101, 48, 49, 100, 55}
	block := make([]uint8, 16)
	for index, value := range data[:16] {
		block[index] = value ^ fixed[index] ^ 42
	}
	head := encrypt_zse_block(block)
	return append(head, encrypt_zse_blocks(data[16:48], head)...)
}

func encode_zse(value uint32) string {
	const alphabet = "6fpLRqJO8M/c3jnYxFkUVC4ZIG12SiH=5v0mXDazWBTsuw7QetbKdoPyAl+hN9rgE"
	var result string
	for _, shift := range []uint{0, 6, 12, 18} {
		result += string(alphabet[(value>>shift)&63])
	}
	return result
}

func encrypt_zse_block(input []uint8) []uint8 {
	output := make([]uint8, 16)
	words := make([]uint32, 36)
	words[0] = get_zse_uint32(input, 0)
	words[1] = get_zse_uint32(input, 4)
	words[2] = get_zse_uint32(input, 8)
	words[3] = get_zse_uint32(input, 12)
	for index := 0; index < 32; index++ {
		words[index+4] = words[index] ^ transform_zse(words[index+1]^words[index+2]^words[index+3]^zse_round_keys[index])
	}
	put_zse_uint32(words[35], output, 0)
	put_zse_uint32(words[34], output, 4)
	put_zse_uint32(words[33], output, 8)
	put_zse_uint32(words[32], output, 12)
	return output
}

func encrypt_zse_blocks(input []uint8, seed []uint8) []uint8 {
	var output []uint8
	for block_index := 0; block_index*16 < len(input); block_index++ {
		block := make([]uint8, 16)
		part := input[block_index*16 : (block_index+1)*16]
		for index := 0; index < 16; index++ {
			block[index] = part[index] ^ seed[index]
		}
		seed = encrypt_zse_block(block)
		output = append(output, seed...)
	}
	return output
}

func transform_zse(value uint32) uint32 {
	input := make([]uint8, 4)
	output := make([]uint8, 4)
	put_zse_uint32(value, input, 0)
	for index := range input {
		output[index] = zse_s_box[input[index]]
	}
	result := get_zse_uint32(output, 0)
	return result ^ rotate_zse_left(result, 2) ^ rotate_zse_left(result, 10) ^ rotate_zse_left(result, 18) ^ rotate_zse_left(result, 24)
}

func put_zse_uint32(value uint32, output []uint8, index int) {
	output[index] = uint8(value >> 24)
	output[index+1] = uint8(value >> 16)
	output[index+2] = uint8(value >> 8)
	output[index+3] = uint8(value)
}

func get_zse_uint32(input []uint8, index int) uint32 {
	return uint32(input[index])<<24 | uint32(input[index+1])<<16 | uint32(input[index+2])<<8 | uint32(input[index+3])
}

func rotate_zse_left(value uint32, shift uint) uint32 {
	return value<<shift | value>>(32-shift)
}
