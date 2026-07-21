// Package fsmock provides comprehensive test data and mock infrastructure for
// the hermes download engine. It includes a virtual filesystem with patterned
// data generation, mock protocol drivers, a configurable mock Store, and
// pre-built test scenarios covering all resource types and edge cases.
package fsmock

import (
	"os"
)

// TestFile represents a file in the virtual filesystem with pre-generated
// content for download testing.
type TestFile struct {
	Name        string
	Data        []byte
	Size        int64
	ContentType string
}

// AssetBundle holds pre-generated test files keyed by name.
type AssetBundle struct {
	Files map[string]*TestFile
}

// DefaultAssets creates a standard set of test files of various types and
// sizes for comprehensive download testing.
func DefaultAssets() *AssetBundle {
	sizes := map[string]int64{
		"empty.bin":    0,
		"1b.bin":       1,
		"1kb.bin":      1024,
		"64kb.bin":     64 * 1024,
		"1mb.bin":      1024 * 1024,
		"5mb.bin":      5 * 1024 * 1024,
		"10mb.bin":     10 * 1024 * 1024,
		"100kb.bin":    100 * 1024,
		"video.mp4":    5 * 1024 * 1024,
		"audio.mp3":    2 * 1024 * 1024,
		"cover.png":    128 * 1024,
		"cover.jpg":    96 * 1024,
		"document.pdf": 512 * 1024,
		"readme.txt":   4096,
		"small.mp4":    1024*1024 + 1,
	}

	bundle := &AssetBundle{Files: make(map[string]*TestFile, len(sizes))}
	for name, size := range sizes {
		contentType := contentTypeForFile(name)
		bundle.Files[name] = &TestFile{
			Name:        name,
			Data:        GenerateData(size),
			Size:        size,
			ContentType: contentType,
		}
	}
	return bundle
}

func contentTypeForFile(name string) string {
	switch {
	case hasSuffix(name, ".mp4"):
		return "video/mp4"
	case hasSuffix(name, ".mp3"):
		return "audio/mpeg"
	case hasSuffix(name, ".png"):
		return "image/png"
	case hasSuffix(name, ".jpg"), hasSuffix(name, ".jpeg"):
		return "image/jpeg"
	case hasSuffix(name, ".pdf"):
		return "application/pdf"
	case hasSuffix(name, ".txt"):
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// GenerateData creates patterned test data of the given size. Each byte at
// offset i equals byte(i % 256), producing deterministic and verifiable
// content.
func GenerateData(size int64) []byte {
	if size <= 0 {
		return nil
	}
	data := make([]byte, size)
	for i := int64(0); i < size; i++ {
		data[i] = byte(i % 256)
	}
	return data
}

// VerifyData checks that data at each offset matches the pattern produced by
// GenerateData.
func VerifyData(data []byte, offset int64) bool {
	for i, b := range data {
		expected := byte((offset + int64(i)) % 256)
		if b != expected {
			return false
		}
	}
	return true
}

// ReadFile reads a file at path and returns its complete contents.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// GetFileSize returns the size of the file at the given path.
func GetFileSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}
