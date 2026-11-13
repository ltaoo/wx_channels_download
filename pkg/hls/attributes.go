// Adapted from dramadl; see ../scraper/NOTICE and ../scraper/LICENSE.
package hls

import (
	"errors"
	"strings"
)

// ParseAttributes parses an HLS attribute list, preserving quoted commas and
// rejecting duplicate attributes and malformed quoting.
func ParseAttributes(value string) (map[string]string, error) {
	attributes := make(map[string]string)
	for value != "" {
		key, rest, ok := strings.Cut(value, "=")
		if !ok || key == "" || strings.ContainsAny(key, ", \t\r\n") {
			return nil, errors.New("HLS 属性格式无效")
		}
		if _, exists := attributes[key]; exists {
			return nil, errors.New("HLS 属性重复")
		}
		var item string
		if strings.HasPrefix(rest, `"`) {
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				return nil, errors.New("HLS 引号未闭合")
			}
			item, rest = rest[1:1+end], rest[end+2:]
			if rest != "" && !strings.HasPrefix(rest, ",") {
				return nil, errors.New("HLS 属性分隔无效")
			}
			rest = strings.TrimPrefix(rest, ",")
		} else {
			item, rest, _ = strings.Cut(rest, ",")
		}
		attributes[key] = item
		value = rest
	}
	return attributes, nil
}
