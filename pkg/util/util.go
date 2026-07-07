package util

import (
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"strconv"
	"strings"
)

func Includes(str, substr string) bool {
	return strings.Contains(str, substr)
}

func EncodeUint64ToBase64(decimalUint64 string) string {
	n := new(big.Int)
	if _, ok := n.SetString(decimalUint64, 10); !ok {
		return ""
	}
	if n.Sign() < 0 || n.BitLen() > 64 {
		return ""
	}

	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n.Uint64())

	b64 := base64.StdEncoding.EncodeToString(b[:])
	return base64URLEncodeNoPad(b64)
}

func DecodeBase64ToUint64String(base64URLNoPad string) string {
	b64Std := base64URLDecodeToStd(base64URLNoPad)
	if b64Std == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(b64Std)
	if err != nil {
		return ""
	}
	if len(raw) == 0 {
		return ""
	}
	if len(raw) < 8 {
		padded := make([]byte, 8)
		copy(padded, raw)
		raw = padded
	} else if len(raw) > 8 {
		raw = raw[:8]
	}
	v := binary.BigEndian.Uint64(raw)
	return strconv.FormatUint(v, 10)
}

func base64URLEncodeNoPad(base64Std string) string {
	base64Std = strings.ReplaceAll(base64Std, "+", "-")
	base64Std = strings.ReplaceAll(base64Std, "/", "_")
	return strings.TrimRight(base64Std, "=")
}

func base64URLDecodeToStd(base64URLNoPad string) string {
	s := strings.ReplaceAll(base64URLNoPad, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	for len(s)%4 != 0 {
		s += "="
	}
	return s
}
