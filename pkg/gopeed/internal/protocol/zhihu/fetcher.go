package zhihu

import (
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/GopeedLab/gopeed/internal/controller"
	"github.com/GopeedLab/gopeed/internal/fetcher"
	"github.com/GopeedLab/gopeed/pkg/base"

	zhihudownload "wx_channel/pkg/scraper/zhihu"
)

type Fetcher struct {
	fetcher.DefaultFetcher
	mu         sync.Mutex
	downloaded int64
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

	client := &zhihudownload.Client{}
	page, err := client.FetchAnswerPage(resolveRealURL(req.URL))
	if err != nil {
		return err
	}
	content := zhihudownload.BuildHTML(page)
	filename := sanitizeFilename(page.Question.Title)
	if filename == "" {
		filename = "zhihu_" + page.URL.QuestionID + "_" + page.URL.AnswerID
	}
	filename += ".html"
	size := int64(len(content))
	f.DefaultFetcher.Meta.Res = &base.Resource{
		Name: filename,
		Size: size,
		Files: []*base.FileInfo{
			{Name: filename, Path: "", Size: size},
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
		client := &zhihudownload.Client{
			OnProgress: func(downloaded int64) {
				f.addProgress(downloaded)
			},
		}
		content, err := client.BuildHTMLFromURL(resolveRealURL(f.DefaultFetcher.Meta.Req.URL))
		if err != nil {
			f.DoneCh <- err
			return
		}
		destPath := filepath.Join(f.DefaultFetcher.Meta.Opts.Path, f.DefaultFetcher.Meta.Opts.Name)
		file, err := f.DefaultFetcher.Ctl.Touch(destPath, int64(len(content)))
		if err != nil {
			f.DoneCh <- err
			return
		}
		defer file.Close()
		n, err := file.WriteString(content)
		f.addProgress(int64(n))
		f.DoneCh <- err
	}()
	return nil
}

func (f *Fetcher) Pause() error {
	return nil
}

func (f *Fetcher) Close() error {
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

func (f *Fetcher) addProgress(n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downloaded += n
}

type FetcherManager struct{}

func resolveRealURL(rawURL string) string {
	return zhihudownload.ResolveRealURL(rawURL)
}

func (fm *FetcherManager) Name() string {
	return "zhihu"
}

func (fm *FetcherManager) Filters() []*fetcher.SchemeFilter {
	return []*fetcher.SchemeFilter{
		{Type: fetcher.FilterTypeUrl, Pattern: "zhihu"},
	}
}

func (fm *FetcherManager) Build() fetcher.Fetcher {
	return &Fetcher{}
}

func (fm *FetcherManager) ParseName(u string) string {
	parsed, err := url.Parse(resolveRealURL(u))
	if err != nil {
		return "zhihu.html"
	}
	return strings.Trim(parsed.Path, "/") + ".html"
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

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	name = replacer.Replace(name)
	if len([]rune(name)) > 80 {
		runes := []rune(name)
		name = string(runes[:80])
	}
	return strings.TrimSpace(name)
}
