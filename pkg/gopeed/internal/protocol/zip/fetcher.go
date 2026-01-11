package zip

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/internal/controller"
	"github.com/GopeedLab/gopeed/internal/fetcher"
	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/util"
)

type FileItem struct {
	Url      string `json:"url"`
	Filename string `json:"filename"`
}

type Fetcher struct {
	fetcher.DefaultFetcher
	files      []FileItem
	downloaded int64
	mu         sync.Mutex
	closed     bool
}

func (f *Fetcher) Setup(ctl *controller.Controller) {
	if err := f.DefaultFetcher.Setup(ctl); err != nil {
		panic(err)
	}
	if f.DefaultFetcher.Meta == nil {
		f.DefaultFetcher.Meta = &fetcher.FetcherMeta{}
	}
}

func (f *Fetcher) Meta() *fetcher.FetcherMeta {
	return f.DefaultFetcher.Meta
}

func (f *Fetcher) Resolve(req *base.Request) error {
	u, err := url.Parse(req.URL)
	if err != nil {
		return err
	}

	filesJson := u.Query().Get("files")
	// Fallback to manual parsing if Query().Get() fails or returns truncated/invalid data
	// This can happen with complex JSON in query params sometimes
	if filesJson == "" || !json.Valid([]byte(filesJson)) {
		if idx := strings.Index(u.RawQuery, "files="); idx != -1 {
			// Extract everything after files=
			encoded := u.RawQuery[idx+6:]
			// If there are other parameters, cut them off (assuming & is separator)
			if end := strings.Index(encoded, "&"); end != -1 {
				encoded = encoded[:end]
			}
			if decoded, err := url.QueryUnescape(encoded); err == nil {
				filesJson = decoded
			}
		}
	}

	if filesJson == "" {
		return errors.New("missing files parameter")
	}

	var files []FileItem
	if err := json.Unmarshal([]byte(filesJson), &files); err != nil {
		return fmt.Errorf("invalid files parameter: %w", err)
	}

	if len(files) == 0 {
		return errors.New("no files to download")
	}

	f.files = files

	// Fetch file sizes to calculate total size
	var totalSize int64
	var wg sync.WaitGroup
	var sizeMu sync.Mutex
	// Limit concurrency
	sem := make(chan struct{}, 16)
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, file := range files {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			resp, err := client.Head(u)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
					sizeMu.Lock()
					totalSize += resp.ContentLength
					sizeMu.Unlock()
				}
			}
		}(file.Url)
	}
	wg.Wait()

	if f.DefaultFetcher.Meta == nil {
		f.DefaultFetcher.Meta = &fetcher.FetcherMeta{}
	}
	f.DefaultFetcher.Meta.Req = req
	f.DefaultFetcher.Meta.Res = &base.Resource{
		Name:  "archive.zip",
		Size:  totalSize,
		Files: []*base.FileInfo{{Name: "archive.zip", Path: "", Size: totalSize}},
	}

	if u.Hostname() != "" {
		f.DefaultFetcher.Meta.Res.Name = u.Hostname() + ".zip"
		f.DefaultFetcher.Meta.Res.Files[0].Name = f.DefaultFetcher.Meta.Res.Name
	}

	return nil
}

func (f *Fetcher) Create(opts *base.Options) error {
	return nil
}

func (f *Fetcher) Start() error {
	go func() {
		err := f.downloadAndZip()
		f.DoneCh <- err
	}()
	return nil
}

func (f *Fetcher) downloadAndZip() error {
	// Ensure directory exists
	if err := util.CreateDirIfNotExist(f.DefaultFetcher.Meta.Opts.Path); err != nil {
		return err
	}

	destPath := f.DefaultFetcher.Meta.SingleFilepath()
	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	zipWriter := zip.NewWriter(destFile)
	defer zipWriter.Close()

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for _, item := range f.files {
		if f.isClosed() {
			return errors.New("fetcher closed")
		}

		req, err := http.NewRequest("GET", item.Url, nil)
		if err != nil {
			return err
		}

		// Copy headers from original request if available
		// Assuming we might want to pass some user-agent or cookies
		// For now, we keep it simple.

		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("failed to download %s: %s", item.Url, resp.Status)
		}

		header := &zip.FileHeader{
			Name:   item.Filename,
			Method: zip.Deflate,
		}
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			resp.Body.Close()
			return err
		}

		buf := make([]byte, 32*1024)
		for {
			if f.isClosed() {
				resp.Body.Close()
				return errors.New("fetcher closed")
			}
			n, err := resp.Body.Read(buf)
			if n > 0 {
				_, wErr := writer.Write(buf[:n])
				if wErr != nil {
					resp.Body.Close()
					return wErr
				}
				f.addProgress(int64(n))
			}
			if err != nil {
				resp.Body.Close()
				if err == io.EOF {
					break
				}
				return err
			}
		}
	}
	return nil
}

func (f *Fetcher) addProgress(n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downloaded += n
}

func (f *Fetcher) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

func (f *Fetcher) Pause() error {
	return nil
}

func (f *Fetcher) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *Fetcher) Stats() any {
	return nil
}

func (f *Fetcher) Progress() fetcher.Progress {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fetcher.Progress{f.downloaded}
}

// Manager
type FetcherManager struct{}

func (fm *FetcherManager) Name() string {
	return "zip"
}

func (fm *FetcherManager) Filters() []*fetcher.SchemeFilter {
	return []*fetcher.SchemeFilter{
		{
			Type:    fetcher.FilterTypeUrl,
			Pattern: "zip",
		},
	}
}

func (fm *FetcherManager) Build() fetcher.Fetcher {
	return &Fetcher{}
}

func (fm *FetcherManager) ParseName(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return "archive.zip"
	}
	if parsed.Hostname() != "" {
		return parsed.Hostname() + ".zip"
	}
	return "archive.zip"
}

func (fm *FetcherManager) AutoRename() bool {
	return true
}

func (fm *FetcherManager) DefaultConfig() any {
	return nil
}

func (fm *FetcherManager) Store(f fetcher.Fetcher) (any, error) {
	return nil, nil
}

func (fm *FetcherManager) Restore() (v any, f func(meta *fetcher.FetcherMeta, v any) fetcher.Fetcher) {
	return nil, nil
}

func (fm *FetcherManager) Close() error {
	return nil
}
