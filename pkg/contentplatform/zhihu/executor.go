package zhihu

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	zhihupkg "wx_channel/pkg/zhihu"
)

type HTMLBuilder interface {
	BuildHTMLFromURL(rawURL string) (string, error)
}

type Executor struct {
	Builder HTMLBuilder
}

func NewExecutor(builder HTMLBuilder) *Executor {
	if builder == nil {
		builder = &zhihupkg.Client{}
	}
	return &Executor{Builder: builder}
}

func (e *Executor) Name() string {
	return PlatformID
}

func (e *Executor) CanHandle(source contentdownload.DownloadSpec) bool {
	return strings.EqualFold(source.Protocol, "zhihu") ||
		strings.HasPrefix(strings.ToLower(source.URL), "zhihu://")
}

func (e *Executor) Execute(ctx context.Context, req contentdownload.ExecuteRequest) error {
	sourceURL := strings.TrimPrefix(req.Source.URL, "zhihu://")
	var html string
	if page, _ := req.Resolved.Metadata["page"].(*zhihupkg.AnswerPage); page != nil {
		html = zhihupkg.BuildHTML(page)
	} else if page, _ := req.Resolved.Metadata["page"].(*zhihupkg.QuestionPage); page != nil {
		html = zhihupkg.BuildQuestionHTML(page)
	} else if page, _ := req.Resolved.Metadata["page"].(*zhihupkg.ArticlePage); page != nil {
		html = zhihupkg.BuildArticleHTML(page)
	} else {
		var err error
		html, err = e.Builder.BuildHTMLFromURL(sourceURL)
		if err != nil {
			return err
		}
	}
	mediaClient := e.mediaClient()
	var err error
	html, err = mediaClient.InlineRemoteImages(html, sourceURL)
	if err != nil {
		return err
	}
	html, err = mediaClient.LocalizeRemoteVideos(ctx, html, sourceURL, req.DestPath)
	if err != nil {
		return err
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

func (e *Executor) mediaClient() *zhihupkg.Client {
	if client, ok := e.Builder.(*zhihupkg.Client); ok && client != nil {
		return client
	}
	return &zhihupkg.Client{}
}
