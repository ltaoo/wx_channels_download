package xdownloader

import (
	"bytes"
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

const defaultHTTPBufferSize = 32 * 1024

type HTTPDownloader struct {
	client           *http.Client
	bufferSize       int
	progressInterval time.Duration
}

func NewHTTPDownloader(client *http.Client) *HTTPDownloader {
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	return &HTTPDownloader{
		client:           client,
		bufferSize:       defaultHTTPBufferSize,
		progressInterval: 200 * time.Millisecond,
	}
}

func (d *HTTPDownloader) Name() string {
	return "http"
}

func (d *HTTPDownloader) CanDownload(req Request) bool {
	switch requestProtocol(req) {
	case ProtocolHTTP, ProtocolHTTPS:
		return true
	default:
		return false
	}
}

func (d *HTTPDownloader) Download(ctx context.Context, req Request) (*Result, error) {
	if strings.TrimSpace(req.URL) == "" || strings.TrimSpace(req.DestPath) == "" {
		return nil, ErrInvalidRequest
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}

	tmpPath := tempPath(req)
	resumeLen, err := resumeLength(tmpPath, req.Resume)
	if err != nil {
		return nil, err
	}

	resp, err := d.open(ctx, req, resumeLen)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	openFlag := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent && resumeLen > 0 {
		openFlag |= os.O_APPEND
	} else {
		openFlag |= os.O_TRUNC
		resumeLen = 0
	}

	total := resp.ContentLength
	if total >= 0 {
		total += resumeLen
	}
	if err := validateSize(total, req.MinSize, req.MaxSize); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(tmpPath, openFlag, 0o644)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	downloaded, copyErr := d.copy(ctx, file, resp.Body, copyState{
		filename:    req.DestPath,
		tmpFilename: tmpPath,
		startedAt:   startedAt,
		startOffset: resumeLen,
		total:       total,
		rateLimit:   req.RateLimitBytesPerSec,
		resumed:     resumeLen > 0,
		progress:    req.Progress,
	})
	closeErr := file.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if total > 0 && downloaded < total {
		return nil, fmt.Errorf("%w: got %d of %d bytes", ErrContentTooShort, downloaded, total)
	}
	if tmpPath != req.DestPath {
		if err := os.Rename(tmpPath, req.DestPath); err != nil {
			return nil, err
		}
	}

	reportProgress(req.Progress, Progress{
		Status:            StatusFinished,
		Filename:          req.DestPath,
		TemporaryFilename: tmpPath,
		DownloadedBytes:   downloaded,
		TotalBytes:        total,
		Percent:           percent(downloaded, total),
		StartedAt:         startedAt,
		UpdatedAt:         time.Now(),
		Resumed:           resumeLen > 0,
	})
	return &Result{Path: req.DestPath, Bytes: downloaded, Resumed: resumeLen > 0}, nil
}

func (d *HTTPDownloader) open(ctx context.Context, req Request, resumeLen int64) (*http.Response, error) {
	body := io.Reader(nil)
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return nil, err
	}
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}
	if httpReq.Header.Get("Accept-Encoding") == "" {
		httpReq.Header.Set("Accept-Encoding", "identity")
	}
	if resumeLen > 0 {
		httpReq.Header.Set("Range", "bytes="+strconv.FormatInt(resumeLen, 10)+"-")
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		resp.Body.Close()
		return nil, ErrRangeNotSatisfiable
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status := resp.Status
		resp.Body.Close()
		return nil, fmt.Errorf("xdownloader: http download failed: %s", status)
	}
	return resp, nil
}

type copyState struct {
	filename    string
	tmpFilename string
	startedAt   time.Time
	startOffset int64
	total       int64
	rateLimit   int64
	resumed     bool
	progress    Hook
}

func (d *HTTPDownloader) copy(ctx context.Context, dst io.Writer, src io.Reader, state copyState) (int64, error) {
	bufferSize := d.bufferSize
	if bufferSize <= 0 {
		bufferSize = defaultHTTPBufferSize
	}
	buf := make([]byte, bufferSize)
	downloaded := state.startOffset
	lastReportAt := time.Now()
	lastReportBytes := downloaded

	reportProgress(state.progress, Progress{
		Status:            StatusConnecting,
		Filename:          state.filename,
		TemporaryFilename: state.tmpFilename,
		DownloadedBytes:   downloaded,
		TotalBytes:        state.total,
		Percent:           percent(downloaded, state.total),
		StartedAt:         state.startedAt,
		UpdatedAt:         lastReportAt,
		Resumed:           state.resumed,
	})

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			if writeErr != nil {
				return downloaded, writeErr
			}
			downloaded += int64(written)

			if err := sleepForRateLimit(ctx, state.startedAt, downloaded-state.startOffset, state.rateLimit); err != nil {
				return downloaded, err
			}

			now := time.Now()
			if now.Sub(lastReportAt) >= d.progressInterval || (state.total > 0 && downloaded >= state.total) {
				elapsed := now.Sub(lastReportAt).Seconds()
				speed := int64(0)
				if elapsed > 0 {
					speed = int64(float64(downloaded-lastReportBytes) / elapsed)
				}
				reportProgress(state.progress, Progress{
					Status:            StatusDownloading,
					Filename:          state.filename,
					TemporaryFilename: state.tmpFilename,
					DownloadedBytes:   downloaded,
					TotalBytes:        state.total,
					Percent:           percent(downloaded, state.total),
					SpeedBytesPerSec:  speed,
					StartedAt:         state.startedAt,
					UpdatedAt:         now,
					Resumed:           state.resumed,
				})
				lastReportAt = now
				lastReportBytes = downloaded
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return downloaded, nil
			}
			return downloaded, readErr
		}
	}
}

func tempPath(req Request) string {
	ext := req.TempExt
	if ext == "" {
		ext = ".part"
	}
	if ext == "-" {
		return req.DestPath
	}
	return req.DestPath + ext
}

func resumeLength(path string, resume bool) (int64, error) {
	if !resume {
		return 0, nil
	}
	info, err := os.Stat(path)
	if err == nil {
		return info.Size(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return 0, err
}

func validateSize(total, minSize, maxSize int64) error {
	if total <= 0 {
		return nil
	}
	if minSize > 0 && total < minSize {
		return fmt.Errorf("xdownloader: file is smaller than min size: %d < %d", total, minSize)
	}
	if maxSize > 0 && total > maxSize {
		return fmt.Errorf("xdownloader: file is larger than max size: %d > %d", total, maxSize)
	}
	return nil
}

func sleepForRateLimit(ctx context.Context, startedAt time.Time, bytes int64, rateLimit int64) error {
	if rateLimit <= 0 || bytes <= 0 {
		return nil
	}
	expected := time.Duration(float64(bytes) / float64(rateLimit) * float64(time.Second))
	if wait := expected - time.Since(startedAt); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func percent(downloaded, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(downloaded) * 100 / float64(total)
}

func reportProgress(hook Hook, progress Progress) {
	if hook != nil {
		hook(progress)
	}
}
