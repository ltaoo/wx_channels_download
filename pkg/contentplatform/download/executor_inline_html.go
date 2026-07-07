package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InlineHTMLExecutor struct{}

func NewInlineHTMLExecutor() *InlineHTMLExecutor {
	return &InlineHTMLExecutor{}
}

func (e *InlineHTMLExecutor) Name() string {
	return "inline_html"
}

func (e *InlineHTMLExecutor) CanHandle(source DownloadSpec) bool {
	return strings.EqualFold(source.Protocol, "inline_html") ||
		strings.HasPrefix(strings.ToLower(source.URL), "inline-html://")
}

func (e *InlineHTMLExecutor) Execute(ctx context.Context, req ExecuteRequest) error {
	html := inlineHTMLFromRequest(req)
	if strings.TrimSpace(html) == "" {
		return fmt.Errorf("inline html body is empty")
	}
	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(req.DestPath, []byte(html), 0o644); err != nil {
		return err
	}
	if req.OnProgress != nil {
		req.OnProgress(Progress{
			DownloadedBytes: int64(len(html)),
			TotalBytes:      int64(len(html)),
			Percent:         100,
		})
	}
	return nil
}

func inlineHTMLFromRequest(req ExecuteRequest) string {
	if req.Resolved == nil {
		return ""
	}
	for _, key := range []string{"body_html", "document_html", "html"} {
		if value, _ := req.Resolved.Metadata[key].(string); strings.TrimSpace(value) != "" {
			return value
		}
	}
	output := ContentOutputOf(req.Resolved.Content)
	for _, key := range []string{"body_html", "document_html", "html"} {
		if value, _ := output[key].(string); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
