package api

import (
	"encoding/binary"
	"io"

	"wx_channel/pkg/decrypt"
)

// 解密读取器
type DecryptReader struct {
	reader   io.Reader
	ctx      *decrypt.RandCtx64
	limit    uint64
	consumed uint64
	ks       [8]byte
	ksPos    int
}

func NewDecryptReader(reader io.Reader, key uint64, offset uint64, limit uint64) *DecryptReader {
	ctx := decrypt.CreateISAacInst(key)
	dr := &DecryptReader{
		reader:   reader,
		ctx:      ctx,
		limit:    limit,
		consumed: 0,
		ksPos:    8,
	}
	if limit > 0 {
		// 将 consumed 对齐到文件偏移，超出加密区则设置为加密区末尾
		if offset >= limit {
			dr.consumed = limit
		} else {
			dr.consumed = offset
			// 跳过完整的 8 字节块
			skipBlocks := offset / 8
			for i := uint64(0); i < skipBlocks; i++ {
				_ = dr.ctx.ISAacRandom()
			}
			// 生成当前块并设置起始位置
			randNumber := dr.ctx.ISAacRandom()
			binary.BigEndian.PutUint64(dr.ks[:], randNumber)
			dr.ksPos = int(offset % 8)
		}
	}
	return dr
}

func (dr *DecryptReader) Read(p []byte) (int, error) {
	n, err := dr.reader.Read(p)
	if n <= 0 {
		return n, err
	}
	if dr.limit == 0 || dr.consumed >= dr.limit {
		return n, err
	}

	toDecrypt := uint64(n)
	remaining := dr.limit - dr.consumed
	if toDecrypt > remaining {
		toDecrypt = remaining
	}
	// 逐字节异或，维护 keystream 位置
	for i := uint64(0); i < toDecrypt; i++ {
		if dr.ksPos >= 8 {
			randNumber := dr.ctx.ISAacRandom()
			binary.BigEndian.PutUint64(dr.ks[:], randNumber)
			dr.ksPos = 0
		}
		p[i] ^= dr.ks[dr.ksPos]
		dr.ksPos++
	}
	dr.consumed += toDecrypt
	return n, err
}
