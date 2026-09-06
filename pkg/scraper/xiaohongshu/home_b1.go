package xiaohongshu

// The b1 fingerprint encoding is compatible with the MIT-licensed xhshow
// project: https://github.com/Cloxl/xhshow (Copyright 2024 Cloxl).

import (
	"crypto/rand"
	"crypto/rc4"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type xhs_b1_payload struct {
	X33 string `json:"x33"`
	X34 string `json:"x34"`
	X35 string `json:"x35"`
	X36 string `json:"x36"`
	X37 string `json:"x37"`
	X38 string `json:"x38"`
	X39 int    `json:"x39"`
	X42 string `json:"x42"`
	X43 string `json:"x43"`
	X44 string `json:"x44"`
	X45 string `json:"x45"`
	X46 string `json:"x46"`
	X48 string `json:"x48"`
	X49 string `json:"x49"`
	X50 string `json:"x50"`
	X51 string `json:"x51"`
	X52 string `json:"x52"`
	X82 string `json:"x82"`
}

func xhs_generate_b1(timestamp_ms int64) (string, error) {
	random_byte := []byte{0}
	if _, err := rand.Read(random_byte); err != nil {
		return "", err
	}
	payload := xhs_b1_payload{
		X33: "0", X34: "0", X35: "0", X36: fmt.Sprintf("%d", int(random_byte[0])%20+1),
		X37: "0|0|0|0|0|0|0|0|0|1|0|0|0|0|0|0|0|0|1|0|0|0|0|0",
		X38: "0|0|1|0|1|0|0|0|0|0|1|0|1|0|1|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0",
		X39: 0, X42: "3.4.4", X43: "742cc32c", X44: fmt.Sprintf("%d", timestamp_ms),
		X45: "__SEC_CAV__1-1-1-1-1|__SEC_WSA__|", X46: "false", X48: "", X49: "{list:[],type:}",
		X50: "", X51: "", X52: "", X82: "_0x17a2|_0x1954",
	}
	payload_data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	cipher, err := rc4.NewCipher([]byte("xhswebmplfbt"))
	if err != nil {
		return "", err
	}
	ciphertext := make([]byte, len(payload_data))
	cipher.XORKeyStream(ciphertext, payload_data)
	quoted_bytes := xhs_python_quote_latin1_bytes(ciphertext)
	return xhs_custom_base64(quoted_bytes), nil
}

func xhs_python_quote_latin1_bytes(value []byte) []byte {
	encoded := make([]byte, 0, len(value)*3)
	// Python converts the latin-1 string back through UTF-8 before quoting.
	for _, current := range value {
		var sequence []byte
		if current < 0x80 {
			sequence = []byte{current}
		} else {
			sequence = []byte{0xc0 | current>>6, 0x80 | current&0x3f}
		}
		for _, byte_value := range sequence {
			if xhs_quote_safe(byte_value) {
				encoded = append(encoded, byte_value)
			} else {
				encoded = append(encoded, '%', "0123456789ABCDEF"[byte_value>>4], "0123456789ABCDEF"[byte_value&15])
			}
		}
	}
	// Match split("%")[1:]: discard any leading unescaped bytes, then parse
	// the first byte in each percent-delimited segment and retain its suffix.
	segments := make([][]byte, 0)
	for index := 0; index < len(encoded); {
		if encoded[index] != '%' {
			index++
			continue
		}
		start := index + 1
		end := start
		for end < len(encoded) && encoded[end] != '%' {
			end++
		}
		segments = append(segments, encoded[start:end])
		index = end
	}
	result := make([]byte, 0, len(value)*2)
	for _, segment := range segments {
		if len(segment) < 2 {
			continue
		}
		result = append(result, xhs_hex_byte(segment[0], segment[1]))
		result = append(result, segment[2:]...)
	}
	return result
}

func xhs_quote_safe(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' ||
		value == '_' || value == '-' || value == '.' || value == '~' || value == '!' || value == '*' || value == '\'' || value == '(' || value == ')'
}

func xhs_hex_byte(left byte, right byte) byte {
	decode := func(value byte) byte {
		if value >= '0' && value <= '9' {
			return value - '0'
		}
		if value >= 'A' && value <= 'F' {
			return value - 'A' + 10
		}
		return value - 'a' + 10
	}
	return decode(left)<<4 | decode(right)
}

func xhs_custom_base64(value []byte) string {
	encoded := []byte(base64.StdEncoding.EncodeToString(value))
	for index, current := range encoded {
		alphabet_index := strings.IndexByte(xhs_custom_base64_source, current)
		if alphabet_index >= 0 {
			encoded[index] = xhs_custom_base64_target[alphabet_index]
		}
	}
	return string(encoded)
}
