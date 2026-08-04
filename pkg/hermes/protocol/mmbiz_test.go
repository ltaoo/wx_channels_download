package protocol

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wx_channel/pkg/hermes"
)

func TestMMBizDownloadAndVerify(t *testing.T) {
	imgURL := "https://mmbiz.qpic.cn/sz_mmbiz_jpg/7y9QHQ3Lll7vjYAdgoiazd4dUc12hHEKialHEApP76bxtVAicBGhnsUsBgtsPxF1xkIIG73PTWQxLa1QpRL4aLAX0hWuvxtYcfJOOmgtJGYfNA/0?wx_fmt=jpeg"

	driver := NewHTTPDriver()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	endpoint := hermes.Endpoint{
		URL: imgURL,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.50(0x1800322f) NetType/WIFI Language/zh_CN",
			"Referer":    "https://mp.weixin.qq.com/",
		},
	}

	// 1. Prepare
	prepared, err := driver.Prepare(ctx, endpoint)
	if err != nil {
		t.Fatal("Prepare failed:", err)
	}

	// 2. Open (full download)
	reader, err := driver.Open(ctx, endpoint, hermes.ReadRequest{UseRange: false})
	if err != nil {
		t.Fatal("Open failed:", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal("ReadAll failed:", err)
	}

	// 3. Verify
	isJPEG := len(data) > 2 && data[0] == 0xFF && data[1] == 0xD8
	isPNG := len(data) > 4 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G'
	isGIF := len(data) > 4 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F'
	isWEBP := len(data) > 12 && data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F'

	fmt.Printf("\n=== Download result ===\n")
	fmt.Printf("Content-Type: %s\n", prepared.ContentType)
	fmt.Printf("Expected size: %d bytes\n", prepared.Size)
	fmt.Printf("Actual body size: %d bytes\n", len(data))
	fmt.Printf("Magic bytes: %02X %02X %02X %02X\n", data[0], data[1], data[2], data[3])
	fmt.Printf("Is JPEG: %v\n", isJPEG)
	fmt.Printf("Is PNG:  %v\n", isPNG)
	fmt.Printf("Is GIF:  %v\n", isGIF)
	fmt.Printf("Is WEBP: %v\n", isWEBP)

	// Save to temp file
	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "test_image.jpg")
	if err := os.WriteFile(savePath, data, 0644); err != nil {
		t.Fatal("WriteFile failed:", err)
	}
	fmt.Printf("Saved: %s\n", savePath)

	if !isJPEG && !isPNG && !isGIF {
		t.Errorf("Expected JPEG/PNG/GIF, got magic: %02X %02X %02X %02X", data[0], data[1], data[2], data[3])
	}
	if isWEBP {
		t.Error("Got WEBP format")
	}
	if prepared.Size != int64(len(data)) {
		t.Errorf("Size mismatch: Prepare said %d, downloaded %d", prepared.Size, len(data))
	}
}
