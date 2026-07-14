package officialaccountdownload

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/internal/controller"
	"github.com/GopeedLab/gopeed/internal/fetcher"
	"github.com/GopeedLab/gopeed/pkg/base"

	officialaccountdownload "wx_channel/pkg/officialaccount"
)

type Fetcher struct {
	fetcher.DefaultFetcher
	oa         *officialaccountdownload.OfficialAccountDownload
	mu         sync.Mutex
	closed     bool
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

	f.oa = &officialaccountdownload.OfficialAccountDownload{}

	// Fetch the article to get the title
	article, err := f.oa.FetchArticle(resolveRealURL(req.URL))
	if err != nil {
		return err
	}

	// Build filename from template if available
	author := article.Creator
	if author == "" {
		author = article.AuthorNickname
	}
	rawName := buildArticleFilename(article.Title, author, req.Labels)
	if ext := strings.ToLower(filepath.Ext(rawName)); ext != ".html" && ext != ".htm" {
		rawName += ".html"
	}
	// Split into dir and filename, sanitize each part (same logic as processTaskFilename)
	dir, filename := normalizeArticlePath(rawName)
	if dir != "" && f.DefaultFetcher.Meta.Opts != nil {
		f.DefaultFetcher.Meta.Opts.Path = filepath.Join(f.DefaultFetcher.Meta.Opts.Path, dir)
	}
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
		if f.oa == nil {
			f.oa = &officialaccountdownload.OfficialAccountDownload{}
		}
		f.oa.OnProgress = func(downloaded int64) {
			f.addProgress(downloaded)
		}
		// Use the URL from the request and Path from options
		name := f.DefaultFetcher.Meta.Opts.Name
		if name == "" {
			name = f.DefaultFetcher.Meta.Res.Name
		}
		if name == "" && len(f.DefaultFetcher.Meta.Res.Files) > 0 {
			name = f.DefaultFetcher.Meta.Res.Files[0].Name
		}
		destPath := filepath.Join(f.DefaultFetcher.Meta.Opts.Path, name)
		needCompress := false
		if f.DefaultFetcher.Meta.Req.Labels != nil && f.DefaultFetcher.Meta.Req.Labels["compress"] == "true" {
			needCompress = true
		}
		content, err := f.oa.BuildHTMLFromURL(resolveRealURL(f.DefaultFetcher.Meta.Req.URL), needCompress)
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

func (f *Fetcher) addProgress(n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downloaded += n
}

func (f *Fetcher) Progress() fetcher.Progress {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fetcher.Progress{f.downloaded}
}

// Manager
type FetcherManager struct{}

// resolveRealURL strips the "officialaccount://" scheme prefix and returns a valid HTTP(S) URL.
func resolveRealURL(rawURL string) string {
	if strings.HasPrefix(strings.ToLower(rawURL), "officialaccount://") {
		rawURL = rawURL[len("officialaccount://"):]
		if !strings.HasPrefix(rawURL, "http") {
			rawURL = "https://" + rawURL
		}
	}
	return rawURL
}

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
	articleID := officialaccountdownload.ExtractArticleID(u)
	if articleID == "" {
		return "article.html"
	}
	return articleID + ".html"
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

// buildArticleFilename applies the filename template to generate the article filename.
// If no template is provided in labels, falls back to the article title.
func buildArticleFilename(title string, author string, labels map[string]string) string {
	template := ""
	if labels != nil {
		template = labels["filename_template"]
	}
	if template == "" {
		return title
	}

	defaultName := title
	if defaultName == "" {
		defaultName = "article"
	}

	params := map[string]string{
		"filename": defaultName,
		"title":    title,
		"author":   author,
	}
	if labels != nil {
		if v, ok := labels["article_id"]; ok {
			params["id"] = v
		}
		if v, ok := labels["created_at"]; ok {
			params["created_at"] = v
		}
	}

	result := template
	for k, v := range params {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}

// normalizeArticlePath splits a raw filename (which may contain "/" for subdirectories)
// into a directory part and a sanitized filename part.
// This replicates the same behavior as processTaskFilename in the API handler.
func normalizeArticlePath(rawName string) (dir string, filename string) {
	rawName = strings.ReplaceAll(rawName, "//", "_")
	dirPart, namePart := filepath.Split(rawName)
	filename = sanitizeFilenameComponent(namePart)
	if filename == "" {
		filename = "article.html"
	}
	if dirPart != "" {
		dirPart = strings.TrimSuffix(dirPart, string(filepath.Separator))
		components := strings.Split(dirPart, string(filepath.Separator))
		validDirs := make([]string, 0, len(components))
		for _, comp := range components {
			clean := sanitizeFilenameComponent(comp)
			if clean != "" {
				validDirs = append(validDirs, clean)
			}
		}
		dir = filepath.Join(validDirs...)
	}
	return
}

// sanitizeFilenameComponent removes invalid characters from a single filename component.
func sanitizeFilenameComponent(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '\\', '|', '?', '*':
			return -1
		}
		if r < 0x20 {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	name = strings.Trim(name, ".")
	return name
}
