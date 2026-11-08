// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
// SPDX-License-Identifier: GPL-3.0-only
// Adapted from jackotom/hongguo-drama src/native/sm3.js; see ../NOTICE.
package hongguo

import (
	"encoding/binary"
	"math/bits"
)

func sm3(source []byte) []byte {
	data := make([]byte, ((len(source)+9+63)/64)*64)
	copy(data, source)
	data[len(source)] = 0x80
	binary.BigEndian.PutUint64(data[len(data)-8:], uint64(len(source))*8)
	state := [8]uint32{0x7380166f, 0x4914b2b9, 0x172442d7, 0xda8a0600, 0xa96f30bc, 0x163138aa, 0xe38dee4d, 0xb0fb0e4e}
	for offset := 0; offset < len(data); offset += 64 {
		var words [68]uint32
		for i := 0; i < 16; i++ {
			words[i] = binary.BigEndian.Uint32(data[offset+i*4:])
		}
		for i := 16; i < 68; i++ {
			x := words[i-16] ^ words[i-9] ^ bits.RotateLeft32(words[i-3], 15)
			words[i] = x ^ bits.RotateLeft32(x, 15) ^ bits.RotateLeft32(x, 23) ^ bits.RotateLeft32(words[i-13], 7) ^ words[i-6]
		}
		a, b, c, d, e, f, g, h := state[0], state[1], state[2], state[3], state[4], state[5], state[6], state[7]
		for i := 0; i < 64; i++ {
			constant := uint32(0x79cc4519)
			ff, gg := a^b^c, e^f^g
			if i >= 16 {
				constant = 0x7a879d8a
				ff = (a & b) | (a & c) | (b & c)
				gg = (e & f) | (^e & g)
			}
			a12 := bits.RotateLeft32(a, 12)
			s1 := bits.RotateLeft32(a12+e+bits.RotateLeft32(constant, i), 7)
			t1 := ff + d + (s1 ^ a12) + (words[i] ^ words[i+4])
			t2 := gg + h + s1 + words[i]
			d, c, b, a = c, bits.RotateLeft32(b, 9), a, t1
			h, g, f, e = g, bits.RotateLeft32(f, 19), e, t2^bits.RotateLeft32(t2, 9)^bits.RotateLeft32(t2, 17)
		}
		next := [8]uint32{a, b, c, d, e, f, g, h}
		for i := range state {
			state[i] ^= next[i]
		}
	}
	digest := make([]byte, 32)
	for i, value := range state {
		binary.BigEndian.PutUint32(digest[i*4:], value)
	}
	return digest
}
