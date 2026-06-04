package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HTTPExecutor struct {
	client *http.Client
}

func NewHTTPExecutor(client *http.Client) *HTTPExecutor {
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	return &HTTPExecutor{client: client}
}

func (e *HTTPExecutor) Name() string {
	return "http"
}

func (e *HTTPExecutor) CanHandle(source DownloadSpec) bool {
	protocol := strings.ToLower(source.Protocol)
	if protocol == "" {
		return strings.HasPrefix(strings.ToLower(source.URL), "http://") || strings.HasPrefix(strings.ToLower(source.URL), "https://")
	}
	return protocol == "http" || protocol == "https"
}

func (e *HTTPExecutor) Execute(ctx context.Context, req ExecuteRequest) error {
	method := req.Source.Method
	if method == "" {
		method = http.MethodGet
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, req.Source.URL, nil)
	if err != nil {
		return err
	}
	for k, v := range req.Source.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http download failed: %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(req.DestPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return copyWithProgress(file, resp.Body, resp.ContentLength, req.OnProgress)
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, onProgress func(Progress)) error {
	buf := make([]byte, 32*1024)
	var downloaded int64
	lastReport := time.Now()
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			downloaded += int64(written)
			if onProgress != nil && (time.Since(lastReport) >= 200*time.Millisecond || downloaded == total) {
				lastReport = time.Now()
				progress := Progress{DownloadedBytes: downloaded, TotalBytes: total}
				if total > 0 {
					progress.Percent = float64(downloaded) * 100 / float64(total)
				}
				onProgress(progress)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				if onProgress != nil {
					progress := Progress{DownloadedBytes: downloaded, TotalBytes: total}
					if total > 0 {
						progress.Percent = float64(downloaded) * 100 / float64(total)
					}
					onProgress(progress)
				}
				return nil
			}
			return readErr
		}
	}
}
