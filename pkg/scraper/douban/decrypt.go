package douban

import (
	"crypto/rc4"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"unicode/utf16"
)

func DecryptSearchData(text string) (*searchData, error) {
	buf, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, err
	}
	offset := (len(buf) - 32) / 3
	if offset < 0 {
		offset = 0
	}
	if offset+16 > len(buf) {
		return nil, fmt.Errorf("invalid encrypted douban data")
	}
	salt := append([]byte(nil), buf[offset:offset+16]...)
	payload := append([]byte{}, buf[:offset]...)
	payload = append(payload, buf[offset+16:]...)
	key := strconv.FormatUint(xxhash64(salt, 41405), 16)
	cipher, err := rc4.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(payload))
	cipher.XORKeyStream(plain, payload)
	root, err := parseBinaryPlist(plain)
	if err != nil {
		return nil, err
	}
	normalized := normalizeDoubanPlist(root)
	jsonBytes, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	var data searchData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

const (
	xxPrime1 uint64 = 11400714785074694791
	xxPrime2 uint64 = 14029467366897019727
	xxPrime3 uint64 = 1609587929392839161
	xxPrime4 uint64 = 9650029242287828579
	xxPrime5 uint64 = 2870177450012600261
)

func xxhash64(input []byte, seed uint64) uint64 {
	p := input
	var h uint64
	if len(p) >= 32 {
		v1 := seed + xxPrime1 + xxPrime2
		v2 := seed + xxPrime2
		v3 := seed
		v4 := seed - xxPrime1
		for len(p) >= 32 {
			v1 = xxRound(v1, binary.LittleEndian.Uint64(p[0:8]))
			v2 = xxRound(v2, binary.LittleEndian.Uint64(p[8:16]))
			v3 = xxRound(v3, binary.LittleEndian.Uint64(p[16:24]))
			v4 = xxRound(v4, binary.LittleEndian.Uint64(p[24:32]))
			p = p[32:]
		}
		h = bits.RotateLeft64(v1, 1) + bits.RotateLeft64(v2, 7) + bits.RotateLeft64(v3, 12) + bits.RotateLeft64(v4, 18)
		h = xxMergeRound(h, v1)
		h = xxMergeRound(h, v2)
		h = xxMergeRound(h, v3)
		h = xxMergeRound(h, v4)
	} else {
		h = seed + xxPrime5
	}
	h += uint64(len(input))
	for len(p) >= 8 {
		k1 := xxRound(0, binary.LittleEndian.Uint64(p[:8]))
		h ^= k1
		h = bits.RotateLeft64(h, 27)*xxPrime1 + xxPrime4
		p = p[8:]
	}
	if len(p) >= 4 {
		h ^= uint64(binary.LittleEndian.Uint32(p[:4])) * xxPrime1
		h = bits.RotateLeft64(h, 23)*xxPrime2 + xxPrime3
		p = p[4:]
	}
	for _, b := range p {
		h ^= uint64(b) * xxPrime5
		h = bits.RotateLeft64(h, 11) * xxPrime1
	}
	h ^= h >> 33
	h *= xxPrime2
	h ^= h >> 29
	h *= xxPrime3
	h ^= h >> 32
	return h
}

func xxRound(acc uint64, input uint64) uint64 {
	acc += input * xxPrime2
	acc = bits.RotateLeft64(acc, 31)
	acc *= xxPrime1
	return acc
}

func xxMergeRound(acc uint64, val uint64) uint64 {
	val = xxRound(0, val)
	acc ^= val
	acc = acc*xxPrime1 + xxPrime4
	return acc
}

type plistUID uint64

type plistParser struct {
	data        []byte
	offsets     []uint64
	objRefSize  int
	parsedCache map[uint64]any
}

func parseBinaryPlist(data []byte) (any, error) {
	if len(data) < 40 || string(data[:6]) != "bplist" {
		return nil, fmt.Errorf("invalid binary plist")
	}
	trailer := data[len(data)-32:]
	offsetIntSize := int(trailer[6])
	objRefSize := int(trailer[7])
	numObjects := binary.BigEndian.Uint64(trailer[8:16])
	topObject := binary.BigEndian.Uint64(trailer[16:24])
	offsetTableOffset := binary.BigEndian.Uint64(trailer[24:32])
	if offsetIntSize <= 0 || objRefSize <= 0 {
		return nil, fmt.Errorf("invalid binary plist trailer")
	}
	offsets := make([]uint64, numObjects)
	for i := uint64(0); i < numObjects; i++ {
		start := offsetTableOffset + i*uint64(offsetIntSize)
		end := start + uint64(offsetIntSize)
		if end > uint64(len(data)) {
			return nil, fmt.Errorf("invalid offset table")
		}
		offsets[i] = readBigEndian(data[start:end])
	}
	parser := plistParser{
		data:        data,
		offsets:     offsets,
		objRefSize:  objRefSize,
		parsedCache: map[uint64]any{},
	}
	return parser.parseObject(topObject)
}

func (p plistParser) parseObject(index uint64) (any, error) {
	if value, ok := p.parsedCache[index]; ok {
		return value, nil
	}
	if index >= uint64(len(p.offsets)) {
		return nil, fmt.Errorf("invalid object index %d", index)
	}
	offset := p.offsets[index]
	if offset >= uint64(len(p.data)) {
		return nil, fmt.Errorf("invalid object offset %d", offset)
	}
	marker := p.data[offset]
	objType := marker >> 4
	objInfo := marker & 0x0f
	var value any
	var err error
	switch objType {
	case 0x0:
		switch objInfo {
		case 0x0, 0xf:
			value = nil
		case 0x8:
			value = false
		case 0x9:
			value = true
		default:
			err = fmt.Errorf("unsupported simple object 0x%x", objInfo)
		}
	case 0x1:
		length := uint64(1) << objInfo
		value, err = p.readInt(offset+1, length)
	case 0x2:
		length := uint64(1) << objInfo
		value, err = p.readReal(offset+1, length)
	case 0x3:
		value, err = p.readReal(offset+1, 8)
	case 0x4:
		value, err = p.readData(offset, objInfo)
	case 0x5:
		value, err = p.readString(offset, objInfo, false)
	case 0x6:
		value, err = p.readString(offset, objInfo, true)
	case 0x8:
		length := uint64(objInfo)
		if length == 0 {
			value = plistUID(0)
			break
		}
		if offset+1+length > uint64(len(p.data)) {
			err = fmt.Errorf("invalid uid")
			break
		}
		value = plistUID(readBigEndian(p.data[offset+1 : offset+1+length]))
	case 0xa:
		value, err = p.readArray(offset, objInfo)
	case 0xd:
		value, err = p.readDict(offset, objInfo)
	default:
		err = fmt.Errorf("unsupported plist object type 0x%x", objType)
	}
	if err != nil {
		return nil, err
	}
	p.parsedCache[index] = value
	return value, nil
}

func (p plistParser) readLength(offset uint64, objInfo byte) (uint64, uint64, error) {
	if objInfo != 0x0f {
		return uint64(objInfo), offset + 1, nil
	}
	if offset+1 >= uint64(len(p.data)) {
		return 0, 0, fmt.Errorf("invalid length object")
	}
	marker := p.data[offset+1]
	if marker>>4 != 0x1 {
		return 0, 0, fmt.Errorf("invalid length marker")
	}
	lengthBytes := uint64(1) << (marker & 0x0f)
	start := offset + 2
	end := start + lengthBytes
	if end > uint64(len(p.data)) {
		return 0, 0, fmt.Errorf("invalid length bytes")
	}
	return readBigEndian(p.data[start:end]), end, nil
}

func (p plistParser) readInt(offset uint64, length uint64) (int64, error) {
	if offset+length > uint64(len(p.data)) || length > 8 {
		return 0, fmt.Errorf("invalid integer")
	}
	return int64(readBigEndian(p.data[offset : offset+length])), nil
}

func (p plistParser) readReal(offset uint64, length uint64) (float64, error) {
	if offset+length > uint64(len(p.data)) {
		return 0, fmt.Errorf("invalid real")
	}
	switch length {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(p.data[offset : offset+length]))), nil
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(p.data[offset : offset+length])), nil
	default:
		return 0, fmt.Errorf("invalid real length %d", length)
	}
}

func (p plistParser) readData(offset uint64, objInfo byte) ([]byte, error) {
	length, start, err := p.readLength(offset, objInfo)
	if err != nil {
		return nil, err
	}
	if start+length > uint64(len(p.data)) {
		return nil, fmt.Errorf("invalid data")
	}
	return append([]byte(nil), p.data[start:start+length]...), nil
}

func (p plistParser) readString(offset uint64, objInfo byte, utf16Text bool) (string, error) {
	length, start, err := p.readLength(offset, objInfo)
	if err != nil {
		return "", err
	}
	byteLength := length
	if utf16Text {
		byteLength = length * 2
	}
	if start+byteLength > uint64(len(p.data)) {
		return "", fmt.Errorf("invalid string")
	}
	raw := p.data[start : start+byteLength]
	if !utf16Text {
		return string(raw), nil
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		units = append(units, binary.BigEndian.Uint16(raw[i:i+2]))
	}
	return string(utf16.Decode(units)), nil
}

func (p plistParser) readArray(offset uint64, objInfo byte) ([]any, error) {
	length, start, err := p.readLength(offset, objInfo)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, length)
	for i := uint64(0); i < length; i++ {
		refStart := start + i*uint64(p.objRefSize)
		refEnd := refStart + uint64(p.objRefSize)
		if refEnd > uint64(len(p.data)) {
			return nil, fmt.Errorf("invalid array ref")
		}
		value, err := p.parseObject(readBigEndian(p.data[refStart:refEnd]))
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func (p plistParser) readDict(offset uint64, objInfo byte) (map[string]any, error) {
	length, start, err := p.readLength(offset, objInfo)
	if err != nil {
		return nil, err
	}
	keysStart := start
	valuesStart := start + length*uint64(p.objRefSize)
	out := make(map[string]any, length)
	for i := uint64(0); i < length; i++ {
		keyRefStart := keysStart + i*uint64(p.objRefSize)
		keyRefEnd := keyRefStart + uint64(p.objRefSize)
		valueRefStart := valuesStart + i*uint64(p.objRefSize)
		valueRefEnd := valueRefStart + uint64(p.objRefSize)
		if keyRefEnd > uint64(len(p.data)) || valueRefEnd > uint64(len(p.data)) {
			return nil, fmt.Errorf("invalid dict ref")
		}
		key, err := p.parseObject(readBigEndian(p.data[keyRefStart:keyRefEnd]))
		if err != nil {
			return nil, err
		}
		value, err := p.parseObject(readBigEndian(p.data[valueRefStart:valueRefEnd]))
		if err != nil {
			return nil, err
		}
		out[fmt.Sprint(key)] = value
	}
	return out, nil
}

func readBigEndian(data []byte) uint64 {
	var out uint64
	for _, b := range data {
		out = out<<8 | uint64(b)
	}
	return out
}

func normalizeDoubanPlist(root any) any {
	r := plistNormalizer{root: root}
	if _, ok := root.([]any); ok {
		return r.resolve(realUID(4))
	}
	return r.normalize(root)
}

type plistNormalizer struct {
	root any
}

func (n plistNormalizer) resolve(uid uint64) any {
	switch root := n.root.(type) {
	case []any:
		if uid < uint64(len(root)) {
			return n.normalize(root[uid])
		}
	case map[string]any:
		return n.normalize(root[strconv.FormatUint(uid, 10)])
	}
	return nil
}

func (n plistNormalizer) normalize(value any) any {
	switch v := value.(type) {
	case plistUID:
		return n.resolve(realUID(uint64(v)))
	case []byte:
		return strings.TrimRight(string(v), "\x00")
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, n.normalize(item))
		}
		return out
	case map[string]any:
		if len(v) == 1 {
			if rawUID, ok := v["j"]; ok {
				return n.resolve(realUID(toUint64(rawUID)))
			}
		}
		if vals, ok := v["k"]; ok {
			keys, hasKeys := v["z"]
			valList, _ := vals.([]any)
			if hasKeys {
				keyList, _ := keys.([]any)
				out := map[string]any{}
				for i, keyValue := range keyList {
					if i >= len(valList) {
						break
					}
					key := fmt.Sprint(n.normalize(keyValue))
					out[key] = n.normalize(valList[i])
				}
				return out
			}
			out := make([]any, 0, len(valList))
			for _, item := range valList {
				out = append(out, n.normalize(item))
			}
			return out
		}
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = n.normalize(item)
		}
		return out
	default:
		return v
	}
}

func realUID(uid uint64) uint64 {
	if uid >= 2 {
		diff := uint64(5)
		if uid < 7 {
			return uid + diff
		}
		if uid < 12 {
			return uid - diff
		}
	}
	return uid
}

func toUint64(value any) uint64 {
	switch v := value.(type) {
	case uint64:
		return v
	case int64:
		return uint64(v)
	case int:
		return uint64(v)
	case float64:
		return uint64(v)
	case plistUID:
		return uint64(v)
	default:
		u, _ := strconv.ParseUint(fmt.Sprint(v), 10, 64)
		return u
	}
}
