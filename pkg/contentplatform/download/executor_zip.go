package download

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ZipFileItem struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

type ZipExecutor struct {
	client *http.Client
}

func NewZipExecutor(client *http.Client) *ZipExecutor {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &ZipExecutor{client: client}
}

func (e *ZipExecutor) Name() string {
	return "zip"
}

func (e *ZipExecutor) CanHandle(source DownloadSpec) bool {
	if strings.EqualFold(source.Protocol, "zip") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(source.URL), "zip://")
}

func (e *ZipExecutor) Execute(ctx context.Context, req ExecuteRequest) error {
	files, err := parseZipFiles(req.Source.URL)
	if err != nil {
		return err
	}
	total := e.resolveTotalSize(ctx, files)

	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return err
	}
	destFile, err := os.Create(req.DestPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	zipWriter := zip.NewWriter(destFile)
	defer zipWriter.Close()

	var downloaded int64
	for _, item := range files {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
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
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return fmt.Errorf("download zip item %s failed: %s", item.URL, resp.Status)
		}

		header := &zip.FileHeader{Name: item.Filename, Method: zip.Deflate}
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			resp.Body.Close()
			return err
		}
		err = copyWithProgress(writer, resp.Body, resp.ContentLength, func(progress Progress) {
			if req.OnProgress == nil {
				return
			}
			p := Progress{
				DownloadedBytes: downloaded + progress.DownloadedBytes,
				TotalBytes:      total,
			}
			if total > 0 {
				p.Percent = float64(p.DownloadedBytes) * 100 / float64(total)
			}
			req.OnProgress(p)
		})
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.ContentLength > 0 {
			downloaded += resp.ContentLength
		}
	}
	return nil
}

func parseZipFiles(rawURL string) ([]ZipFileItem, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	filesJSON := u.Query().Get("files")
	if filesJSON == "" || !json.Valid([]byte(filesJSON)) {
		if idx := strings.Index(u.RawQuery, "files="); idx != -1 {
			encoded := u.RawQuery[idx+6:]
			if end := strings.Index(encoded, "&"); end != -1 {
				encoded = encoded[:end]
			}
			if decoded, err := url.QueryUnescape(encoded); err == nil {
				filesJSON = decoded
			}
		}
	}
	if filesJSON == "" {
		return nil, errors.New("missing files parameter")
	}
	var files []ZipFileItem
	if err := json.Unmarshal([]byte(filesJSON), &files); err != nil {
		return nil, fmt.Errorf("invalid files parameter: %w", err)
	}
	if len(files) == 0 {
		return nil, errors.New("no files to download")
	}
	return files, nil
}

func (e *ZipExecutor) resolveTotalSize(ctx context.Context, files []ZipFileItem) int64 {
	var total int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, file := range files {
		wg.Add(1)
		go func(rawURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
			if err != nil {
				return
			}
			resp, err := e.client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 && resp.ContentLength > 0 {
				mu.Lock()
				total += resp.ContentLength
				mu.Unlock()
			}
		}(file.URL)
	}
	wg.Wait()
	return total
}
