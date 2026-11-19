// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
package vqq

import (
	"errors"
	"strconv"
	"strings"
)

// Affected Tencent CDN streams stall after the first 8183-byte DATA payload.
const tencent_range_size int64 = 8183

func parse_content_range(value string) (int64, int64, int64, error) {
	invalid := errors.New("视频 Content-Range 无效")
	value, ok := strings.CutPrefix(strings.TrimSpace(value), "bytes ")
	if !ok {
		return 0, 0, 0, invalid
	}
	span, total_text, ok := strings.Cut(value, "/")
	if !ok {
		return 0, 0, 0, invalid
	}
	start_text, end_text, ok := strings.Cut(span, "-")
	if !ok {
		return 0, 0, 0, invalid
	}
	start, start_err := strconv.ParseInt(start_text, 10, 64)
	end, end_err := strconv.ParseInt(end_text, 10, 64)
	total, total_err := strconv.ParseInt(total_text, 10, 64)
	if start_err != nil || end_err != nil || total_err != nil || start < 0 || end < start || total <= end {
		return 0, 0, 0, invalid
	}
	return start, end, total, nil
}
