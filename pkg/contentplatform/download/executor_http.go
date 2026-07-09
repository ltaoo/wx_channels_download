package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPChunkSize    = 10 << 20
	defaultHTTPChunkRetries = 10
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
	if strings.TrimSpace(req.Source.URL) == "" {
		return fmt.Errorf("http download URL is empty")
	}
	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return err
	}
	if req.Source.ChunkSize > 0 && strings.EqualFold(method, http.MethodGet) {
		return e.executeChunked(ctx, req, method)
	}

	partPath := req.DestPath + ".part"
	resumeFrom := existingFileSize(partPath)
	httpReq, err := http.NewRequestWithContext(ctx, method, req.Source.URL, nil)
	if err != nil {
		return err
	}
	for k, v := range req.Source.Headers {
		httpReq.Header.Set(k, v)
	}
	if httpReq.Header.Get("Accept-Encoding") == "" {
		httpReq.Header.Set("Accept-Encoding", "identity")
	}
	if resumeFrom > 0 {
		httpReq.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
	}

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http download failed: %s", resp.Status)
	}
	appendPart := resumeFrom > 0 && resp.StatusCode == http.StatusPartialContent
	if resumeFrom > 0 && !appendPart {
		resumeFrom = 0
	}

	flags := os.O_CREATE | os.O_WRONLY
	if appendPart {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(partPath, flags, 0o644)
	if err != nil {
		return err
	}

	total := resp.ContentLength
	if total > 0 && resumeFrom > 0 {
		total += resumeFrom
	}
	if err := copyWithProgressFrom(file, resp.Body, total, resumeFrom, req.OnProgress); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := renameReplace(partPath, req.DestPath); err != nil {
		return err
	}
	return nil
}

func (e *HTTPExecutor) executeChunked(ctx context.Context, req ExecuteRequest, method string) error {
	partPath := req.DestPath + ".part"
	chunkSize := req.Source.ChunkSize
	if chunkSize <= 0 {
		chunkSize = defaultHTTPChunkSize
	}
	resumeFrom := existingFileSize(partPath)
	flags := os.O_CREATE | os.O_WRONLY
	if resumeFrom > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(partPath, flags, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	downloaded := resumeFrom
	total := int64(0)
	retries := 0
	for {
		start := downloaded
		end := start + chunkSize - 1
		err := e.downloadChunk(ctx, file, req, method, start, end, &downloaded, &total)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if !retryableHTTPDownloadError(err) || retries >= defaultHTTPChunkRetries {
				return err
			}
			retries++
			if info, statErr := file.Stat(); statErr == nil {
				downloaded = info.Size()
			}
			if seekErr := seekFileEnd(file); seekErr != nil {
				return seekErr
			}
			if sleepErr := sleepBeforeRetry(ctx, retries); sleepErr != nil {
				return sleepErr
			}
			continue
		}
		retries = 0
		if downloaded == start {
			break
		}
		if total > 0 && downloaded >= total {
			break
		}
		if downloaded-start < chunkSize {
			break
		}
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := renameReplace(partPath, req.DestPath); err != nil {
		return err
	}
	return nil
}

func (e *HTTPExecutor) downloadChunk(ctx context.Context, file *os.File, req ExecuteRequest, method string, start int64, end int64, downloaded *int64, total *int64) error {
	httpReq, err := http.NewRequestWithContext(ctx, method, req.Source.URL, nil)
	if err != nil {
		return err
	}
	for k, v := range req.Source.Headers {
		httpReq.Header.Set(k, v)
	}
	if httpReq.Header.Get("Accept-Encoding") == "" {
		httpReq.Header.Set("Accept-Encoding", "identity")
	}
	httpReq.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return io.EOF
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpStatusError{status: resp.Status, code: resp.StatusCode}
	}
	if resp.StatusCode == http.StatusOK && start > 0 {
		return fmt.Errorf("http server ignored range request while resuming from %d bytes", start)
	}
	if resp.StatusCode == http.StatusOK {
		if err := file.Truncate(0); err != nil {
			return err
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		*downloaded = 0
		start = 0
	}
	if parsedTotal := totalFromContentRange(resp.Header.Get("Content-Range")); parsedTotal > 0 {
		*total = parsedTotal
	} else if resp.ContentLength > 0 && resp.StatusCode == http.StatusOK {
		*total = resp.ContentLength
	}
	progressTotal := *total
	if progressTotal == 0 && resp.ContentLength > 0 {
		progressTotal = *downloaded + resp.ContentLength
	}
	if err := copyWithProgressFrom(file, resp.Body, progressTotal, *downloaded, req.OnProgress); err != nil {
		if info, statErr := file.Stat(); statErr == nil {
			*downloaded = info.Size()
		}
		return err
	}
	if info, statErr := file.Stat(); statErr == nil {
		*downloaded = info.Size()
	}
	return nil
}

type httpStatusError struct {
	status string
	code   int
}

func (e httpStatusError) Error() string {
	return "http download failed: " + e.status
}

func retryableHTTPDownloadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) {
		return false
	}
	var statusErr httpStatusError
	if errors.As(err, &statusErr) {
		return statusErr.code == http.StatusTooManyRequests || statusErr.code >= 500
	}
	return true
}

func seekFileEnd(file *os.File) error {
	_, err := file.Seek(0, io.SeekEnd)
	return err
}

func sleepBeforeRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt) * 500 * time.Millisecond
	if delay > 5*time.Second {
		delay = 5 * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func totalFromContentRange(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	_, after, ok := strings.Cut(value, "/")
	if !ok || after == "*" {
		return 0
	}
	total, _ := strconv.ParseInt(after, 10, 64)
	return total
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, onProgress func(Progress)) error {
	return copyWithProgressFrom(dst, src, total, 0, onProgress)
}

func copyWithProgressFrom(dst io.Writer, src io.Reader, total int64, initial int64, onProgress func(Progress)) error {
	buf := make([]byte, 32*1024)
	downloaded := initial
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

func existingFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

func renameReplace(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	_ = os.Remove(dst)
	return os.Rename(src, dst)
}
