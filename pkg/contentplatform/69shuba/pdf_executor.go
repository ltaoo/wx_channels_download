package shuba69

import (
	"context"
	"fmt"
	"os"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/novelpdf"
	shubapkg "wx_channel/pkg/scraper/69shuba"
)

type PDFOptions struct {
	ForceClean bool
}

type PDFExecutor struct {
	ForceClean bool
}

func NewPDFExecutor() *PDFExecutor {
	return &PDFExecutor{}
}

func (e *PDFExecutor) Name() string {
	return LocalPDFProtocol
}

func (e *PDFExecutor) CanHandle(source contentdownload.DownloadSpec) bool {
	return strings.EqualFold(source.Protocol, LocalPDFProtocol)
}

func (e *PDFExecutor) Execute(ctx context.Context, req contentdownload.ExecuteRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rootDir := req.Source.URL
	if local := localArchiveFromResolved(req.Resolved); local != nil {
		rootDir = local.RootDir
	}
	cleaned, err := shubapkg.CleanLocalArchive(rootDir, shubapkg.CleanArchiveOptions{Force: e.ForceClean})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	novel, err := pdfNovelFromCleanArchive(cleaned)
	if err != nil {
		return err
	}
	if err := novelpdf.Export(req.DestPath, novel, novelpdf.Options{}); err != nil {
		return err
	}
	if req.OnProgress != nil {
		var size int64
		if info, statErr := os.Stat(req.DestPath); statErr == nil {
			size = info.Size()
		}
		req.OnProgress(contentdownload.Progress{
			DownloadedBytes: size,
			TotalBytes:      size,
			Percent:         100,
		})
	}
	return nil
}

func localArchiveFromResolved(resolved *contentdownload.ResolvedRequest) *shubapkg.LocalArchiveLoadResult {
	if resolved == nil {
		return nil
	}
	for _, values := range []map[string]any{resolved.Internal, resolved.Metadata} {
		if values == nil {
			continue
		}
		switch local := values[metadataLocalArchive].(type) {
		case *shubapkg.LocalArchiveLoadResult:
			return local
		case shubapkg.LocalArchiveLoadResult:
			return &local
		}
	}
	return nil
}

func pdfNovelFromLocalArchive(local *shubapkg.LocalArchiveLoadResult) *novelpdf.Novel {
	if local == nil || local.Novel == nil {
		return &novelpdf.Novel{}
	}
	chapters := make([]novelpdf.Chapter, 0, len(local.Chapters))
	for _, item := range local.Chapters {
		if strings.TrimSpace(item.Error) != "" || item.Content == nil {
			continue
		}
		title := item.Chapter.Title
		if title == "" {
			title = item.Content.Title
		}
		content := item.Content.Content
		if strings.TrimSpace(content) == "" {
			continue
		}
		chapters = append(chapters, novelpdf.Chapter{
			Title:   title,
			Content: content,
		})
	}
	if len(chapters) == 0 {
		return &novelpdf.Novel{Title: local.Novel.Title}
	}
	return &novelpdf.Novel{
		Title:       local.Novel.Title,
		Author:      local.Novel.Author,
		Category:    local.Novel.Category,
		Status:      local.Novel.Status,
		Description: local.Novel.Description,
		Chapters:    chapters,
	}
}

func pdfNovelFromCleanArchive(cleaned *shubapkg.CleanArchiveResult) (*novelpdf.Novel, error) {
	if cleaned == nil || cleaned.Catalog == nil {
		return nil, fmt.Errorf("clean archive is empty")
	}
	catalog := cleaned.Catalog
	chapters := make([]novelpdf.Chapter, 0, len(catalog.Chapters))
	for _, item := range catalog.Chapters {
		content, err := shubapkg.LoadCleanChapterText(cleaned.RootDir, item)
		if err != nil {
			return nil, fmt.Errorf("read cleaned chapter %d %q: %w", item.Index, item.Title, err)
		}
		if strings.TrimSpace(content) == "" {
			return nil, fmt.Errorf("cleaned chapter %d %q is empty", item.Index, item.Title)
		}
		chapters = append(chapters, novelpdf.Chapter{
			Title:   item.Title,
			Content: content,
		})
	}
	return &novelpdf.Novel{
		Title:       catalog.Title,
		Author:      catalog.Author,
		Category:    catalog.Category,
		Status:      catalog.Status,
		Description: catalog.Description,
		CoverImage:  "",
		Chapters:    chapters,
	}, nil
}

func GenerateLocalArchivePDF(ctx context.Context, rootDir string, outputPath string) error {
	return GenerateLocalArchivePDFWithOptions(ctx, rootDir, outputPath, PDFOptions{})
}

func GenerateLocalArchivePDFWithOptions(ctx context.Context, rootDir string, outputPath string, options PDFOptions) error {
	if strings.TrimSpace(rootDir) == "" {
		return fmt.Errorf("root directory is empty")
	}
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("output path is empty")
	}
	executor := NewPDFExecutor()
	executor.ForceClean = options.ForceClean
	return executor.Execute(ctx, contentdownload.ExecuteRequest{
		Source: contentdownload.DownloadSpec{
			URL:      rootDir,
			Protocol: LocalPDFProtocol,
		},
		DestPath: outputPath,
	})
}
