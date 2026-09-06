package xiaohongshu

// The RAP wire-format implementation follows the MIT-licensed xhshow project:
// https://github.com/Cloxl/xhshow (Copyright 2024 Cloxl).

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

const xhs_rap_sdk_version uint32 = 10300

var xhs_rap_round_keys = [10][4]uint32{
	{0x6b714931, 0x44546377, 0x4b583930, 0x5a744179},
	{0x89314c98, 0xcd652fef, 0x863d16df, 0xdc4957a6},
	{0xc205330c, 0x0f601ce3, 0x895d0a3c, 0x55145d9a},
	{0xd205006e, 0xdd651c8d, 0x543816b1, 0x012c4b2b},
	{0x770c2b6f, 0xaa6937e2, 0xfe512153, 0xff7d6a78},
	{0x7866fbf4, 0xd20fcc16, 0x2c5eed45, 0xd323873d},
	{0x90e9c67e, 0x42e60a68, 0x6eb8e72d, 0xbd9b6010},
	{0xe9c52bee, 0xab232186, 0xc59bc6ab, 0x7800a6bb},
	{0x13ba9a3e, 0xb899bbb8, 0x7d027d13, 0x0502dba8},
	{0x50613270, 0xe8f889c8, 0x95faf4db, 0x90f82f73},
}

var xhs_rap_last_round_key = [4]uint32{0xf396b44f, 0x1b6e3d87, 0x8e94c95c, 0x1e6ce62f}

var xhs_rap_sbox = [256]byte{
	0x7a, 0x01, 0x58, 0xe0, 0x50, 0x4e, 0x02, 0x79, 0x1d, 0x4b, 0x53, 0xda, 0x6b, 0x48, 0xd4, 0x52,
	0xed, 0x77, 0x12, 0x21, 0x14, 0x15, 0xec, 0x10, 0x18, 0xe5, 0xb9, 0xf1, 0x0c, 0x08, 0xfc, 0x7d,
	0xf9, 0xcd, 0xb5, 0xc8, 0xe6, 0x37, 0x26, 0x87, 0x56, 0xba, 0xb8, 0x2b, 0xad, 0xf0, 0x68, 0xf7,
	0x8b, 0x8d, 0xd3, 0x5e, 0x36, 0x4d, 0x2e, 0x92, 0x31, 0x82, 0xf2, 0x29, 0x70, 0x3d, 0x2d, 0xd7,
	0xb6, 0x40, 0xb2, 0x43, 0x44, 0x80, 0x78, 0xd2, 0x0d, 0x49, 0x4a, 0x09, 0x63, 0x6c, 0x07, 0x3a,
	0x9e, 0xd5, 0x06, 0xc6, 0xe1, 0x62, 0xf4, 0x34, 0x24, 0x59, 0xa9, 0x57, 0x2a, 0x00, 0x3e, 0x17,
	0x2c, 0x0a, 0x1a, 0x42, 0xfa, 0x93, 0xbe, 0xdc, 0xf5, 0xb3, 0x6a, 0x13, 0xe8, 0x03, 0xc7, 0x97,
	0xbb, 0x73, 0x76, 0x86, 0xe3, 0x46, 0x72, 0x47, 0xd0, 0x05, 0x4c, 0x38, 0x7c, 0x1f, 0x81, 0xab,
	0x75, 0x51, 0xeb, 0xf3, 0x32, 0x74, 0x11, 0x8f, 0x84, 0x89, 0x9c, 0x71, 0x22, 0x7e, 0x9d, 0xcf,
	0x3f, 0x91, 0x69, 0x65, 0x3c, 0x6d, 0x96, 0xa2, 0x98, 0x99, 0x33, 0x39, 0x9a, 0xca, 0xc3, 0x9f,
	0xa0, 0xbc, 0xe4, 0xa3, 0xa4, 0x54, 0x7f, 0xa7, 0xa8, 0x04, 0x6f, 0x5d, 0xac, 0xb7, 0x27, 0xaf,
	0xb0, 0x28, 0x41, 0xae, 0xb4, 0x6e, 0x0b, 0x1b, 0xdf, 0x8e, 0x30, 0xb1, 0xfe, 0x90, 0x61, 0x60,
	0xc0, 0xcb, 0x5c, 0x0e, 0xef, 0x16, 0x83, 0xea, 0x20, 0xe9, 0xc9, 0x55, 0xc4, 0x45, 0x85, 0xcc,
	0x1e, 0xaa, 0x67, 0x8a, 0x7b, 0x35, 0xd6, 0x19, 0xd8, 0xd9, 0xc2, 0xdb, 0x94, 0xdd, 0x1c, 0xde,
	0xa6, 0xff, 0xf8, 0xbf, 0x5b, 0x5a, 0x0f, 0xe7, 0xc1, 0xbd, 0xd1, 0x66, 0xc5, 0x25, 0xee, 0x8c,
	0xe2, 0x5f, 0x88, 0xa1, 0x3b, 0xa5, 0xf6, 0xce, 0x95, 0x2f, 0x64, 0x23, 0xfb, 0xfd, 0x4f, 0x9b,
}

var xhs_rap_default_trace, _ = hex.DecodeString(
	"0002000200004700000000000001ffd9ffa8005c23fff1ffff001501ff90fff9000022fff2ffff" +
		"001501ffb800770061a5ffe3ffff002a01fffcffe70000f0ffe4ffff002a01ffd30000003e01" +
		"ffd40000003e01ffc8ffff005301ffbdfffa006901ffbefffb006901ffb1fff4007d01ffa6ff" +
		"ee009301ffa8ffef009301ff9effe900a801ff96ffe600be01ff97ffe600be01ff93ffe500d3" +
		"01ff92ffe500ea01ff92ffe4010501ff92ffe3011b01ff92ffe402d101ff93ffe502f201ff94" +
		"ffe6040501",
)

var xhs_rap_default_environment, _ = hex.DecodeString(
	"000000010000000000000000fffeffff00000000000000390001ffff00bc00000000006e0002" +
		"0001013400000000012f00000002011a00000000016100000000019f000000000247000000000279" +
		"0000000002b1",
)

func xhs_rap_gf_double(value byte) byte {
	shifted := uint16(value) << 1
	if shifted&0x100 != 0 {
		shifted ^= 0x11b
	}
	return byte(shifted)
}

func xhs_rap_tables() [4][256]uint32 {
	var tables [4][256]uint32
	for index, substituted := range xhs_rap_sbox {
		doubled := xhs_rap_gf_double(substituted)
		tripled := doubled ^ substituted
		tables[0][index] = uint32(doubled)<<24 | uint32(substituted)<<16 | uint32(substituted)<<8 | uint32(tripled)
		tables[1][index] = uint32(tripled)<<24 | uint32(doubled)<<16 | uint32(substituted)<<8 | uint32(substituted)
		tables[2][index] = uint32(substituted)<<24 | uint32(tripled)<<16 | uint32(doubled)<<8 | uint32(substituted)
		tables[3][index] = uint32(substituted)<<24 | uint32(substituted)<<16 | uint32(tripled)<<8 | uint32(doubled)
	}
	return tables
}

func xhs_rap_encrypt_block(block []byte) ([]byte, error) {
	if len(block) > 16 {
		return nil, fmt.Errorf("xhs rap block exceeds 16 bytes")
	}
	padded := make([]byte, 16)
	copy(padded, block)
	state := [4]uint32{
		binary.BigEndian.Uint32(padded[0:4]) ^ xhs_rap_round_keys[0][0],
		binary.BigEndian.Uint32(padded[4:8]) ^ xhs_rap_round_keys[0][1],
		binary.BigEndian.Uint32(padded[8:12]) ^ xhs_rap_round_keys[0][2],
		binary.BigEndian.Uint32(padded[12:16]) ^ xhs_rap_round_keys[0][3],
	}
	tables := xhs_rap_tables()
	for round := 1; round < len(xhs_rap_round_keys); round++ {
		var next [4]uint32
		for column := 0; column < 4; column++ {
			next[column] = tables[0][byte(state[column]>>24)] ^ tables[1][byte(state[(column+1)&3]>>16)] ^ tables[2][byte(state[(column+2)&3]>>8)] ^ tables[3][byte(state[(column+3)&3])] ^ xhs_rap_round_keys[round][column]
		}
		state = next
	}
	last_key := make([]byte, 16)
	for index, value := range xhs_rap_last_round_key {
		binary.BigEndian.PutUint32(last_key[index*4:], value)
	}
	result := make([]byte, 16)
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			value := byte(state[(row+column)&3] >> (24 - 8*column))
			result[4*row+column] = xhs_rap_sbox[value] ^ last_key[4*row+column]
		}
	}
	return result, nil
}

func xhs_rap_random_ascii(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	random_bytes := make([]byte, length)
	if _, err := rand.Read(random_bytes); err != nil {
		return "", err
	}
	for index := range random_bytes {
		random_bytes[index] = alphabet[int(random_bytes[index])%len(alphabet)]
	}
	return string(random_bytes), nil
}

func xhs_rap_xxh32(data []byte) uint32 {
	const prime1 uint32 = 0x9e3779b1
	const prime2 uint32 = 0x85ebca77
	const prime3 uint32 = 0xc2b2ae3d
	const prime4 uint32 = 0x27d4eb2f
	const prime5 uint32 = 0x165667b1
	rotate := func(value uint32, shift uint) uint32 { return value<<shift | value>>(32-shift) }
	position := 0
	var digest uint32
	if len(data) >= 16 {
		first_accumulator := prime1
		first_accumulator += prime2
		fourth_accumulator := uint32(0)
		fourth_accumulator -= prime1
		accumulators := [4]uint32{first_accumulator, prime2, 0, fourth_accumulator}
		for position <= len(data)-16 {
			for index := range accumulators {
				word := binary.LittleEndian.Uint32(data[position:])
				position += 4
				accumulators[index] = rotate(accumulators[index]+word*prime2, 13) * prime1
			}
		}
		digest = rotate(accumulators[0], 1) + rotate(accumulators[1], 7) + rotate(accumulators[2], 12) + rotate(accumulators[3], 18)
	} else {
		digest = prime5
	}
	digest += uint32(len(data))
	for position+4 <= len(data) {
		digest = rotate(digest+binary.LittleEndian.Uint32(data[position:])*prime3, 17) * prime4
		position += 4
	}
	for position < len(data) {
		digest = rotate(digest+uint32(data[position])*prime5, 11) * prime1
		position++
	}
	digest ^= digest >> 15
	digest *= prime2
	digest ^= digest >> 13
	digest *= prime3
	digest ^= digest >> 16
	return digest
}

func xhs_rap_field_byte(buffer *bytes.Buffer, tag uint16, value byte) {
	_ = binary.Write(buffer, binary.BigEndian, tag)
	buffer.WriteByte(value)
}

func xhs_rap_field_u32(buffer *bytes.Buffer, tag uint16, value uint32) {
	_ = binary.Write(buffer, binary.BigEndian, tag)
	_ = binary.Write(buffer, binary.BigEndian, value)
}

func xhs_rap_field_u64(buffer *bytes.Buffer, tag uint16, value uint64) {
	_ = binary.Write(buffer, binary.BigEndian, tag)
	_ = binary.Write(buffer, binary.BigEndian, value)
}

func xhs_rap_field_blob(buffer *bytes.Buffer, tag uint16, value []byte) {
	_ = binary.Write(buffer, binary.BigEndian, tag)
	_ = binary.Write(buffer, binary.BigEndian, uint32(len(value)))
	buffer.Write(value)
}

func xhs_max_int64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func xhs_min_int(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func xhs_rap_body(api string, data string, timestamp_ms int64, session_key []byte, nonce uint32, mask byte) []byte {
	var buffer bytes.Buffer
	xhs_rap_field_u64(&buffer, 1000, uint64(timestamp_ms))
	xhs_rap_field_u32(&buffer, 1001, nonce)
	xhs_rap_field_blob(&buffer, 1002, session_key)
	xhs_rap_field_u32(&buffer, 1003, xhs_rap_xxh32([]byte(api+data)))
	for tag := uint16(1051); tag <= 1065; tag++ {
		xhs_rap_field_byte(&buffer, tag, 0)
	}
	xhs_rap_field_byte(&buffer, 1070, 0)
	for tag := uint16(1066); tag <= 1069; tag++ {
		xhs_rap_field_byte(&buffer, tag, 0)
	}
	xhs_rap_field_u32(&buffer, 1100, 0)
	for tag := uint16(1071); tag <= 1073; tag++ {
		xhs_rap_field_byte(&buffer, tag, 0)
	}
	xhs_rap_field_u32(&buffer, 1075, 0x564)
	xhs_rap_field_u32(&buffer, 1076, 0x2c)
	xhs_rap_field_u64(&buffer, 1077, uint64(xhs_max_int64(timestamp_ms-0x434, 0)))
	xhs_rap_field_blob(&buffer, 1078, xhs_rap_default_trace)
	xhs_rap_field_u32(&buffer, 1082, 0)
	xhs_rap_field_u32(&buffer, 1084, 0)
	xhs_rap_field_u32(&buffer, 1085, 0)
	xhs_rap_field_u32(&buffer, 1086, 100)
	xhs_rap_field_u64(&buffer, 1087, uint64(xhs_max_int64(timestamp_ms-0x2d7, 0)))
	xhs_rap_field_blob(&buffer, 1088, xhs_rap_default_environment)
	xhs_rap_field_u32(&buffer, 1090, 0)
	xhs_rap_field_u32(&buffer, 1097, 0)
	xhs_rap_field_u32(&buffer, 1092, 0x566)
	xhs_rap_field_u32(&buffer, 1094, 0x519)
	xhs_rap_field_u64(&buffer, 1095, uint64(xhs_max_int64(timestamp_ms-0x218c, 0)))
	xhs_rap_field_u32(&buffer, 1093, 0)
	xhs_rap_field_byte(&buffer, 1096, 0)
	xhs_rap_field_blob(&buffer, 1091, []byte{0, 0, 0xff, 0xff})
	for tag := uint16(1151); tag <= 1156; tag++ {
		xhs_rap_field_byte(&buffer, tag, 0)
	}
	raw := buffer.Bytes()
	for index := 16; index < len(raw); index++ {
		raw[index] ^= mask
	}
	return raw
}

func xhs_rap_compress(data []byte, timestamp_ms int64) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, 6)
	if err != nil {
		return nil, err
	}
	writer.Header.ModTime = time.Unix(timestamp_ms/1000, 0)
	writer.Header.OS = 3
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func xhs_rap_param(api string, timestamp_ms int64) (string, error) {
	session_key, err := xhs_rap_random_ascii(16)
	if err != nil {
		return "", err
	}
	encryption_key, err := xhs_rap_random_ascii(16)
	if err != nil {
		return "", err
	}
	salt, err := xhs_rap_random_ascii(5)
	if err != nil {
		return "", err
	}
	random_values := make([]byte, 9)
	if _, err := rand.Read(random_values); err != nil {
		return "", err
	}
	nonce := binary.BigEndian.Uint32(random_values[:4])
	mask := random_values[4]
	if mask == 0 {
		mask = 1
	}
	processing_time := uint32(60 + binary.BigEndian.Uint32(random_values[5:])%181)
	raw := xhs_rap_body(api, "{}", timestamp_ms, []byte(session_key), nonce, mask)
	compressed, err := xhs_rap_compress(raw, timestamp_ms)
	if err != nil {
		return "", err
	}
	encryption_key_bytes := []byte(encryption_key)
	xored := make([]byte, len(compressed))
	for index := range compressed {
		xored[index] = compressed[index] ^ encryption_key_bytes[index%len(encryption_key_bytes)]
	}
	encrypted := make([]byte, 0, (len(xored)+15)/16*16)
	for offset := 0; offset < len(xored); offset += 16 {
		end := xhs_min_int(offset+16, len(xored))
		block, block_err := xhs_rap_encrypt_block(xored[offset:end])
		if block_err != nil {
			return "", block_err
		}
		encrypted = append(encrypted, block...)
	}
	cipher_body := append(encrypted, make([]byte, 4)...)
	binary.BigEndian.PutUint32(cipher_body[len(cipher_body)-4:], uint32(len(compressed)))
	encrypted_key, err := xhs_rap_encrypt_block(encryption_key_bytes)
	if err != nil {
		return "", err
	}
	content := append([]byte(salt), encrypted_key...)
	key_length := make([]byte, 4)
	binary.BigEndian.PutUint32(key_length, 16)
	content = append(content, key_length...)
	content = append(content, cipher_body...)
	header := make([]byte, 36)
	copy(header, []byte{0x07, 0x24, 0x01, byte(len(salt))})
	binary.BigEndian.PutUint32(header[4:8], 1)
	binary.BigEndian.PutUint32(header[8:12], 20)
	binary.BigEndian.PutUint32(header[12:16], uint32(len(cipher_body)))
	binary.BigEndian.PutUint32(header[16:20], xhs_rap_xxh32(content))
	binary.BigEndian.PutUint32(header[20:24], xhs_rap_sdk_version)
	binary.BigEndian.PutUint32(header[24:28], processing_time)
	return base64.StdEncoding.EncodeToString(append(header, content...)), nil
}
