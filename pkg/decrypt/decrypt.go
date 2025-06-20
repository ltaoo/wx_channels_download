package decrypt

// 代码来自 https://github.com/Hanson/WechatSphDecrypt/blob/main/decrypt.go

import (
	"encoding/binary"
)

type RandCtx64 struct {
	RandCnt uint64
	Seed    [256]uint64
	MM      [256]uint64
	AA      uint64
	BB      uint64
	CC      uint64
}

func CreateISAacInst(encKey uint64) *RandCtx64 {
	ctx := &RandCtx64{
		RandCnt: 255,
		AA:      0,
		BB:      0,
		CC:      0,
	}
	rand64Init(ctx, encKey)
	return ctx
}

func (ctx *RandCtx64) ISAacRandom() uint64 {
	result := ctx.Seed[ctx.RandCnt]
	if ctx.RandCnt == 0 {
		ctx.isAAC64()
		ctx.RandCnt = 255
	} else {
		ctx.RandCnt--
	}
	return result
}

func rand64Init(ctx *RandCtx64, encKey uint64) {
	const golden = uint64(0x9e3779b97f4a7c13)
	a, b, c, d := golden, golden, golden, golden
	e, f, g, h := golden, golden, golden, golden

	ctx.Seed[0] = encKey
	for i := 1; i < 256; i++ {
		ctx.Seed[i] = 0
	}

	for i := 0; i < 4; i++ {
		mix(&a, &b, &c, &d, &e, &f, &g, &h)
	}

	for i := 0; i < 256; i += 8 {
		a += ctx.Seed[i]
		b += ctx.Seed[i+1]
		c += ctx.Seed[i+2]
		d += ctx.Seed[i+3]
		e += ctx.Seed[i+4]
		f += ctx.Seed[i+5]
		g += ctx.Seed[i+6]
		h += ctx.Seed[i+7]
		mix(&a, &b, &c, &d, &e, &f, &g, &h)
		ctx.MM[i] = a
		ctx.MM[i+1] = b
		ctx.MM[i+2] = c
		ctx.MM[i+3] = d
		ctx.MM[i+4] = e
		ctx.MM[i+5] = f
		ctx.MM[i+6] = g
		ctx.MM[i+7] = h
	}

	for i := 0; i < 256; i += 8 {
		a += ctx.MM[i]
		b += ctx.MM[i+1]
		c += ctx.MM[i+2]
		d += ctx.MM[i+3]
		e += ctx.MM[i+4]
		f += ctx.MM[i+5]
		g += ctx.MM[i+6]
		h += ctx.MM[i+7]
		mix(&a, &b, &c, &d, &e, &f, &g, &h)
		ctx.MM[i] = a
		ctx.MM[i+1] = b
		ctx.MM[i+2] = c
		ctx.MM[i+3] = d
		ctx.MM[i+4] = e
		ctx.MM[i+5] = f
		ctx.MM[i+6] = g
		ctx.MM[i+7] = h
	}

	ctx.isAAC64()
}

func (ctx *RandCtx64) isAAC64() {
	ctx.CC++
	ctx.BB += ctx.CC

	for i := 0; i < 256; i++ {
		switch i % 4 {
		case 0:
			ctx.AA = ^(ctx.AA ^ (ctx.AA << 21))
		case 1:
			ctx.AA ^= ctx.AA >> 5
		case 2:
			ctx.AA ^= ctx.AA << 12
		case 3:
			ctx.AA ^= ctx.AA >> 33
		}

		ctx.AA += ctx.MM[(i+128)%256]
		x := ctx.MM[i]
		y := ctx.MM[(x>>3)%256] + ctx.AA + ctx.BB
		ctx.MM[i] = y
		ctx.BB = ctx.MM[(y>>11)%256] + x
		ctx.Seed[i] = ctx.BB
	}
}

func mix(a, b, c, d, e, f, g, h *uint64) {
	*a -= *e
	*f ^= *h >> 9
	*h += *a
	*b -= *f
	*g ^= *a << 9
	*a += *b
	*c -= *g
	*h ^= *b >> 23
	*b += *c
	*d -= *h
	*a ^= *c << 15
	*c += *d
	*e -= *a
	*b ^= *d >> 14
	*d += *e
	*f -= *b
	*c ^= *e << 20
	*e += *f
	*g -= *c
	*d ^= *f >> 17
	*f += *g
	*h -= *d
	*e ^= *g << 14
	*g += *h
}

func DecryptData(data []byte, encLen uint32, key uint64) {
	if len(data) == 0 || uint32(len(data)) < encLen {
		return
	}

	aaInst := CreateISAacInst(key)

	for i := uint32(0); i < encLen; i += 8 {
		randNumber := aaInst.ISAacRandom()
		tempNumber := make([]byte, 8)
		binary.BigEndian.PutUint64(tempNumber, randNumber)

		for j := 0; j < 8; j++ {
			realIndex := i + uint32(j)
			if realIndex >= encLen {
				return
			}
			data[realIndex] ^= tempNumber[j]
		}
	}

}
