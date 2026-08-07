package zhihu

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
)

var zk = []uint32{
	1170614578, 1024848638, 1413669199, 3951632832, 3528873006, 2921909214, 4151847688, 3997739139, 1933479194, 3323781115, 3888513386, 460404854, 3747539722, 2403641034, 2615871395,
	2119585428, 2265697227, 2035090028, 2773447226, 4289380121, 4217216195, 2200601443, 3051914490, 1579901135, 1321810770, 456816404, 2903323407, 4065664991, 330002838, 3506006750,
	363569021, 2347096187,
}

var zb = []uint8{
	20, 223, 245, 7, 248, 2, 194, 209, 87, 6, 227, 253, 240, 128, 222, 91, 237, 9, 125, 157, 230, 93, 252, 205, 90, 79, 144, 199, 159, 197, 186, 167, 39, 37, 156, 198, 38, 42, 43, 168, 217, 153, 15, 103, 80, 189, 71, 191, 97, 84,
	247, 95, 36, 69, 14, 35, 12, 171, 28, 114, 178, 148, 86, 182, 32, 83, 158, 109, 22, 255, 94, 238, 151, 85, 77, 124, 254, 18, 4, 26, 123, 176, 232, 193, 131, 172, 143, 142, 150, 30, 10, 146, 162, 62, 224, 218, 196, 229, 1,
	192, 213, 27, 110, 56, 231, 180, 138, 107, 242, 187, 54, 120, 19, 44, 117, 228, 215, 203, 53, 239, 251, 127, 81, 11, 133, 96, 204, 132, 41, 115, 73, 55, 249, 147, 102, 48, 122, 145, 106, 118, 74, 190, 29, 16, 174, 5, 177,
	129, 63, 113, 99, 31, 161, 76, 246, 34, 211, 13, 60, 68, 207, 160, 65, 111, 82, 165, 67, 169, 225, 57, 112, 244, 155, 51, 236, 200, 233, 58, 61, 47, 100, 137, 185, 64, 17, 70, 234, 163, 219, 108, 170, 166, 59, 149, 52, 105,
	24, 212, 78, 173, 45, 0, 116, 226, 119, 136, 206, 135, 175, 195, 25, 92, 121, 208, 126, 139, 3, 75, 141, 21, 130, 98, 241, 40, 154, 66, 184, 49, 181, 46, 243, 88, 101, 183, 8, 23, 72, 188, 104, 179, 210, 134, 250, 201, 164,
	89, 216, 202, 220, 50, 221, 152, 140, 33, 235, 214,
}

func buildSignedHeader(apiPath, dc0 string) map[string]string {
	xzse93 := "101_3_3.0"
	sum := md5.Sum([]byte(fmt.Sprintf("%s+%s+%s", xzse93, apiPath, dc0)))
	return map[string]string{
		"x-zse-96": "2.0_" + encrypt(hex.EncodeToString(sum[:])),
		"x-zse-93": xzse93,
		"x-app-za": "OS=Web",
	}
}

func encrypt(md5Str string) string {
	processed := preProcess(md5Str)
	var current uint32
	var result string
	for i := 0; i < len(processed); i++ {
		pop := processed[len(processed)-i-1]
		d := pop ^ uint8((uint32(58)>>(8*uint(i%4)))&255)
		current |= uint32(d) << (8 * uint(i%3))
		if i%3 == 2 {
			result += encode(current)
			current = 0
		}
	}
	return result
}

func preProcess(md5Str string) []uint8 {
	var data []uint8
	for i := 0; i < len(md5Str); i++ {
		data = append(data, md5Str[i])
	}
	data = append([]uint8{uint8(rand.Intn(127)), 0}, data...)
	for i := 0; i < 15; i++ {
		data = append(data, 14)
	}
	first := data[:16]
	fix := []uint8{48, 53, 57, 48, 53, 51, 102, 55, 100, 49, 53, 101, 48, 49, 100, 55}
	block := make([]uint8, 16)
	for i, value := range first {
		block[i] = value ^ fix[i] ^ 42
	}
	head := gr(block)
	return append(head, gx(data[16:48], head)...)
}

func encode(param uint32) string {
	salt := "6fpLRqJO8M/c3jnYxFkUVC4ZIG12SiH=5v0mXDazWBTsuw7QetbKdoPyAl+hN9rgE"
	var result string
	for _, shift := range []uint{0, 6, 12, 18} {
		result += string(salt[(param>>shift)&63])
	}
	return result
}

func gr(input []uint8) []uint8 {
	out := make([]uint8, 16)
	words := make([]uint32, 36)
	words[0] = bGet(input, 0)
	words[1] = bGet(input, 4)
	words[2] = bGet(input, 8)
	words[3] = bGet(input, 12)
	for i := 0; i < 32; i++ {
		words[i+4] = words[i] ^ g(words[i+1]^words[i+2]^words[i+3]^zk[i])
	}
	iPut(words[35], out, 0)
	iPut(words[34], out, 4)
	iPut(words[33], out, 8)
	iPut(words[32], out, 12)
	return out
}

func gx(input []uint8, seed []uint8) []uint8 {
	var out []uint8
	for remaining, blockIndex := len(input), 0; remaining > 0; remaining -= 16 {
		block := make([]uint8, 16)
		part := input[16*blockIndex : 16*(blockIndex+1)]
		for i := 0; i < 16; i++ {
			block[i] = part[i] ^ seed[i]
		}
		seed = gr(block)
		out = append(out, seed...)
		blockIndex++
	}
	return out
}

func g(value uint32) uint32 {
	in := make([]uint8, 4)
	out := make([]uint8, 4)
	iPut(value, in, 0)
	out[0] = zb[in[0]]
	out[1] = zb[in[1]]
	out[2] = zb[in[2]]
	out[3] = zb[in[3]]
	result := bGet(out, 0)
	return result ^ rotate(result, 2) ^ rotate(result, 10) ^ rotate(result, 18) ^ rotate(result, 24)
}

func iPut(value uint32, out []uint8, index int) {
	out[index] = uint8(value >> 24)
	out[index+1] = uint8(value >> 16)
	out[index+2] = uint8(value >> 8)
	out[index+3] = uint8(value)
}

func bGet(input []uint8, index int) uint32 {
	return uint32(input[index])<<24 | uint32(input[index+1])<<16 | uint32(input[index+2])<<8 | uint32(input[index+3])
}

func rotate(value uint32, shift uint) uint32 {
	return (value << shift) | (value >> (32 - shift))
}

func getCookieValue(cookieStr, key string) string {
	parts := strings.Split(cookieStr, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, key+"=") {
			return strings.TrimPrefix(part, key+"=")
		}
	}
	return ""
}
