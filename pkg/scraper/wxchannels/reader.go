package wxchannels

import (
	"encoding/binary"
	"io"
)

// Decrypt reader
type DecryptReader struct {
	reader   io.Reader
	ctx      *RandCtx64
	limit    uint64
	consumed uint64
	ks       [8]byte
	ksPos    int
}

func NewDecryptReader(reader io.Reader, key uint64, offset uint64, limit uint64) *DecryptReader {
	ctx := CreateISAacInst(key)
	dr := &DecryptReader{
		reader:   reader,
		ctx:      ctx,
		limit:    limit,
		consumed: 0,
		ksPos:    8,
	}
	if limit > 0 {
		// Align consumed to file offset; if beyond the encrypted region, set to region end
		if offset >= limit {
			dr.consumed = limit
		} else {
			dr.consumed = offset
			// Skip complete 8-byte blocks
			skipBlocks := offset / 8
			for i := uint64(0); i < skipBlocks; i++ {
				_ = dr.ctx.ISAacRandom()
			}
			// Generate current block and set start position
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
	// Byte-by-byte XOR, maintaining keystream position
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
