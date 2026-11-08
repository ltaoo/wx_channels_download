// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
// SPDX-License-Identifier: GPL-3.0-only
// Adapted from jackotom/hongguo-drama AesV3; see ../NOTICE.
package hongguo

import (
	"encoding/base64"
	"encoding/binary"
)

type medusa_cipher struct {
	box         []byte
	permutation [4]int
	order       [16]int
	keys        [48]byte
}

func new_medusa_cipher(timestamp uint32) medusa_cipher {
	word_size := timestamp & 3
	block := medusa_cipher{
		box:         medusa_sbox[word_size*256 : (word_size+1)*256],
		permutation: [4][4]int{{1, 0, 2, 3}, {2, 0, 3, 1}, {0, 1, 3, 2}, {1, 0, 2, 3}}[word_size],
		order:       [4][16]int{{0, 9, 14, 11, 4, 13, 2, 7, 8, 1, 6, 15, 12, 5, 10, 3}, {0, 9, 14, 15, 4, 13, 2, 7, 8, 1, 6, 3, 12, 5, 10, 11}, {0, 9, 14, 7, 4, 13, 2, 11, 8, 1, 6, 3, 12, 5, 10, 15}, {0, 9, 14, 11, 4, 13, 2, 7, 8, 1, 6, 15, 12, 5, 10, 3}}[word_size],
	}
	initial := [4]uint32{0xca025ddc, 0x823dc546, 0xc9420583, 0xc298225f}[word_size]
	key := hex_bytes("f1593376766ea98d34f31b057a9d5be4")
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(block.keys[i*4:], initial)
	}
	for i := 0; i < 16; i++ {
		block.keys[i] ^= key[i]
	}
	rounds := uint32(8)
	for i := 4; i < 12; i++ {
		at := 4 * (i - 1)
		a, b, c, d := block.keys[at], block.keys[at+1], block.keys[at+2], block.keys[at+3]
		if i&3 == 0 {
			a, b, c, d = byte(initial>>(rounds&24))^block.box[b], block.box[c], block.box[d], block.box[a]
		}
		rounds += 2
		block.keys[at+4], block.keys[at+5], block.keys[at+6], block.keys[at+7] = a^block.keys[at-12], b^block.keys[at-11], c^block.keys[at-10], d^block.keys[at-9]
	}
	return block
}

func (block *medusa_cipher) add_key(rows *[16]byte, at int) {
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			rows[i*4+j] ^= block.keys[at+i*4+block.permutation[j]]
		}
	}
}

func medusa_xt(value byte) byte {
	if value&128 != 0 {
		return value<<1 ^ 0x1b
	}
	return value << 1
}

func (block *medusa_cipher) encrypt_block(rows [16]byte) [16]byte {
	block.add_key(&rows, 0)
	for round := 1; round < 3; round++ {
		for i := range rows {
			rows[i] = block.box[rows[i]]
		}
		old := rows
		for i := 0; i < 4; i++ {
			copy(rows[i*4:(i+1)*4], old[block.permutation[i]*4:(block.permutation[i]+1)*4])
		}
		old = rows
		for i := range rows {
			rows[i] = old[block.order[i]]
		}
		if round == 1 {
			old = rows
			for i := 0; i < 4; i++ {
				for j := 0; j < 4; j++ {
					rows[i*4+j] = old[i*4+block.permutation[j]]
				}
			}
			for i := 0; i < 4; i++ {
				t, u := rows[i]^rows[4+i]^rows[8+i]^rows[12+i], rows[i]
				rows[i] ^= t ^ medusa_xt(rows[i]^rows[4+i])
				rows[4+i] ^= t ^ medusa_xt(rows[4+i]^rows[8+i])
				rows[8+i] ^= t ^ medusa_xt(rows[8+i]^rows[12+i])
				rows[12+i] ^= t ^ medusa_xt(rows[12+i]^u)
			}
		}
		block.add_key(&rows, round*16)
	}
	for i := range rows {
		rows[i] ^= block.keys[16+i]
	}
	return rows
}

func medusa_encrypt(source []byte, timestamp uint32) []byte {
	var plaintext [32]byte
	for i := 0; i < 31; i++ {
		at := i * 8
		plaintext[i] = ((source[at] >> 4) & 2) | (source[at+1] & 64) | ((source[at+2] >> 2) & 1) | ((source[at+3] << 3) & 128) | ((source[at+4] >> 1) & 4) | ((source[at+5] << 3) & 16) | ((source[at+6] << 5) & 32) | ((source[at+7] >> 4) & 8)
	}
	plaintext[31] = 1
	block := new_medusa_cipher(timestamp)
	var previous [16]byte
	copy(previous[:], hex_bytes("1fe109a4125283f418de9e051a969e12"))
	var key [32]byte
	for at := 0; at < 32; at += 16 {
		for i := range previous {
			previous[i] ^= plaintext[at+i]
		}
		previous = block.encrypt_block(previous)
		copy(key[at:], previous[:])
	}
	out := append([]byte{key[31]}, source...)
	for i := 0; i < 31; i++ {
		at, k := 1+i*8, key[i]
		out[at] = (out[at] & 0xdf) | ((k << 4) & 32)
		out[at+1] = (out[at+1] & 0xbf) | (k & 64)
		out[at+2] = (out[at+2] & 0xfb) | ((k << 2) & 4)
		out[at+3] = (out[at+3] & 0xef) | ((k >> 3) & 16)
		out[at+4] = (out[at+4] & 0xf7) | ((k << 1) & 8)
		out[at+5] = (out[at+5] & 0xfd) | ((k >> 3) & 2)
		out[at+6] = (out[at+6] & 0xfe) | ((k >> 5) & 1)
		out[at+7] = (out[at+7] & 0x7f) | ((k << 4) & 128)
	}
	return out
}

var medusa_sbox, _ = base64.StdEncoding.DecodeString("+n0Ia5xZs0sEXznQOEqRmQBnpiCf9U2Ccybu3xhmgzOAAxn72f6uqqmwUsYL83klTni0NqxdGieeiNu9PGPsSRXBMB/cuFbUbM3KCUPINaPvHvSW0vwOcnuUhNHqRVpiAj/TEoE0K91+5ijypUYTATsh9mE3KSoN7Yyvv51cuyR2D3XkU4nhmI2xmmVwT1RMWKtub4sjxAcRDLrPoKSO2AU9FLLadMPX577Wf95IFj6FkKFVt3dCIsmGUC4X+WQxLJvxbRxEaOPpqJOXyzJX6+Vxaq3AzMfF/WAdoi1Hp+JRaV56zgpBtpWP97mH4DoGEIq1+FvV8LyS/3wvwugbQOwb2r26mJEMsiuDQTRn+wrYdrVGBVlhI3WQhyrjUBVMrLF5667llUcEaPCGPVGLD8qO5LlO8hKCvA7V9+8oJc9bXelqVQLhM76T5/WtnT45JKji+hdX0HoNCDDWuKON/QeaxB5uImSX0h2wv0VmP2zd2yeApxHcpsVS+MC2yFwAc2B7oBkTqsk1SEvTpM2fmfMQREBUfin0Bh+iq6EvPPavhWI2IX9e3yAas7Tm/3KEj2UmlFp36kN4x0rMLBRrxuh0U/zUHM4xcAMYjJY4MonxOl/X+alpt2M3WMI7w3HLnpIBigtNiJu7T21v4P6lSd5WFgntnMEt7oF92XzRLkJbTcGmXepE/UVOG6E/0YnhfS+q26utWcuxzpooyeD2cDlK1/8w9d28VzsRjbLuALbmGlp8+d7EzS6Au7lMpZ+ECMZvQmzwJ+eLOpxR+2chdUExp8ogQyq3v9l68rX4jCwjg0+PYKAEEzcU4wHFY2ZcdIHfWL1okD3SszT0GZMyKdZJrg1L2AeerB4tC0C4crp2EHGo5FYdSP7lwkeR2ocmnR+Ia8CYviUJlzOjhRZef9xuVOn3qcjow3fQgivsAmKKkg4+sA8F8/GWeDiGNhg8JM8KtFPMYWWkx5TVFX5t73kiNRJqjlIGVXtGZFCV4gzt0xcDopuZ6xz8r9Rzafpf9ywev8jh8592gHFIqpStZPuJxmDDMrNN0uBE3V+oscdoIzTJbRJ/t+sVvqnReJOgDJKk10fjisJwqyZBmnmn2BSFj8BvVtCMEbkuPOKdzw7eA11GPs04Qw8zWtkaZWwiO/wwpojqN6K0jY5RnNZA7vn4hPSul+nKCkVnVwQvg1zVxcSCtqORmB9KrJaBbssbCQivGJVJfVTt+hYxOtq4ZvWl8f4QAQZ0zGPffCgl9s6yT4vlvIdpu4YhBwA25wtQWZsc6GJYGWHyvSdeuh3mmUI9DSq13Fsp8C1MU3tqc04/df9LoTUXVXI5INOw/e8C7Hd+5CvbkMEFnnrUUmskEw==")
