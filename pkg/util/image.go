package util

import (
	"bytes"
	"net/http"
)

// SafeImageContentType accepts raster images based on their bytes, never an upstream header.
func SafeImageContentType(body []byte) string {
	content_type := http.DetectContentType(body)
	switch content_type {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/x-icon":
		return content_type
	}
	if len(body) >= 16 && bytes.Equal(body[4:8], []byte("ftyp")) &&
		(bytes.Equal(body[8:12], []byte("avif")) || bytes.Equal(body[8:12], []byte("avis"))) {
		return "image/avif"
	}
	return ""
}
