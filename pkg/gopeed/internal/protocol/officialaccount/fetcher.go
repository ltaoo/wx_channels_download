package officialaccountdownload

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/internal/controller"
	"github.com/GopeedLab/gopeed/internal/fetcher"
	"github.com/GopeedLab/gopeed/pkg/base"

	officialaccountdownload "github.com/GopeedLab/gopeed/pkg/officialaccount"
)

type Fetcher struct {
	fetcher.DefaultFetcher
	mu     sync.Mutex
	closed bool
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
	if f.DefaultFetcher.Meta == nil {
		f.DefaultFetcher.Meta = &fetcher.FetcherMeta{}
	}
	f.DefaultFetcher.Meta.Req = req

	oa := &officialaccountdownload.OfficialAccountDownload{}

	// Handle custom scheme
	real_url := req.URL
	if strings.HasPrefix(strings.ToLower(real_url), "officialaccount://") {
		real_url = real_url[len("officialaccount://"):]
		if !strings.HasPrefix(real_url, "http") {
			real_url = "https://" + real_url
		}
		// Create a copy of request with real URL
		req_copy := *req
		req_copy.URL = real_url
		f.DefaultFetcher.Meta.Req = &req_copy
	}

	// Fetch the article to get the title
	article, err := oa.FetchArticle(f.DefaultFetcher.Meta.Req.URL)
	if err != nil {
		return err
	}

	// Sanitize filename
	filename := strings.ReplaceAll(article.Title, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = filename + ".html"
	size := int64(article.ContentLength)
	if len(article.Images) > 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, imgURL := range article.Images {
			if imgURL == "" {
				continue
			}
			wg.Add(1)
			go func(u string) {
				defer wg.Done()
				client := &http.Client{
					Timeout: 10 * time.Second,
				}
				resp, err := client.Head(u)
				if err != nil {
					return
				}
				defer resp.Body.Close()
				if resp.ContentLength > 0 {
					mu.Lock()
					size += resp.ContentLength
					mu.Unlock()
				}
			}(imgURL)
		}
		wg.Wait()
	}

	f.DefaultFetcher.Meta.Res = &base.Resource{
		Name: filename,
		// Size is unknown/variable because of images, but we can set 0 or estimate
		Size: size,
		Files: []*base.FileInfo{
			{
				Name: filename,
				Path: "",
				Size: size,
			},
		},
	}

	return nil
}

func (f *Fetcher) Create(opts *base.Options) error {
	f.DefaultFetcher.Meta.Opts = opts
	return nil
}

func (f *Fetcher) Start() error {
	go func() {
		oa := &officialaccountdownload.OfficialAccountDownload{}
		// Use the URL from the request and Path from options
		destPath := filepath.Join(f.DefaultFetcher.Meta.Opts.Path, f.DefaultFetcher.Meta.Opts.Name)
		needCompress := false
		if f.DefaultFetcher.Meta.Req.Labels != nil && f.DefaultFetcher.Meta.Req.Labels["compress"] == "true" {
			needCompress = true
		}
		content, err := oa.BuildHTMLFromURL(f.DefaultFetcher.Meta.Req.URL, needCompress)
		if err != nil {
			f.DoneCh <- err
			return
		}

		// Save content to file
		file, err := f.DefaultFetcher.Ctl.Touch(destPath, int64(len(content)))
		if err != nil {
			f.DoneCh <- err
			return
		}
		defer file.Close()

		_, err = file.WriteString(content)
		f.DoneCh <- err
	}()
	return nil
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
	// Progress tracking might be difficult with current SaveURLAsHTML implementation
	// as it doesn't report progress. We return 0 or maybe implemented later.
	return fetcher.Progress{0}
}

// Manager
type FetcherManager struct{}

func (fm *FetcherManager) Name() string {
	return "officialaccount"
}

func (fm *FetcherManager) Filters() []*fetcher.SchemeFilter {
	return []*fetcher.SchemeFilter{
		{
			Type:    fetcher.FilterTypeUrl,
			Pattern: "officialaccount",
		},
	}
}

func (fm *FetcherManager) Build() fetcher.Fetcher {
	return &Fetcher{}
}

func (fm *FetcherManager) ParseName(u string) string {
	// Simple parsing, actual name resolution happens in Resolve
	parsed, err := url.Parse(u)
	if err != nil {
		return "article.html"
	}
	// Maybe extract something from URL if possible, but Resolve will do better
	return parsed.Path
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
