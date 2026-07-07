package officialaccount

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	officialaccountpkg "wx_channel/pkg/scraper/officialaccount"
)

type HTMLBuilder interface {
	BuildHTMLFromURL(url string, needCompressImg bool) (string, error)
}

type Executor struct {
	Builder         HTMLBuilder
	NeedCompressImg bool
}

func NewExecutor(builder HTMLBuilder) *Executor {
	if builder == nil {
		builder = &officialaccountpkg.OfficialAccountDownload{}
	}
	return &Executor{Builder: builder}
}

func (e *Executor) Name() string {
	return PlatformID
}

func (e *Executor) CanHandle(source contentdownload.DownloadSpec) bool {
	return strings.EqualFold(source.Protocol, "officialaccount") ||
		strings.HasPrefix(strings.ToLower(source.URL), "officialaccount://")
}

func (e *Executor) Execute(ctx context.Context, req contentdownload.ExecuteRequest) error {
	sourceURL := strings.TrimPrefix(req.Source.URL, "officialaccount://")
	var html string
	if article, _ := req.Resolved.Metadata["article"].(*officialaccountpkg.WechatOfficialArticle); article != nil {
		builder := &officialaccountpkg.OfficialAccountDownload{}
		var err error
		html, err = builder.BuildHTMLFromArticle(article, e.NeedCompressImg)
		if err != nil {
			return err
		}
	} else {
		var err error
		html, err = e.Builder.BuildHTMLFromURL(sourceURL, e.NeedCompressImg)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(req.DestPath, []byte(html), 0o644); err != nil {
		return err
	}
	if req.OnProgress != nil {
		req.OnProgress(contentdownload.Progress{
			DownloadedBytes: int64(len(html)),
			TotalBytes:      int64(len(html)),
			Percent:         100,
		})
	}
	return nil
}
