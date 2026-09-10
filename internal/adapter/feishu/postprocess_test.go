package feishuadapter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
)

func postprocess_test_document_html() string {
	return `<!doctype html><html><head><title>Doc</title></head><body>` +
		`<figure class="image-block"><img class="image-content" src="images/IMG1.png" alt="one"></figure>` +
		`<a class="file-card" href="files/PDF1.pdf" download><strong>report.pdf</strong></a>` +
		`<section data-n="view-block" class="view-block"><header class="view-header"><strong>clip.mp4</strong>` +
		`<a data-n="view-address-link" href="files/MP41.mp4" download>下载文件</a></header>` +
		`<div data-n="file-address-placeholder" class="view-placeholder">视频作为独立资源下载</div></section>` +
		`</body></html>`
}

func postprocess_test_resources(t *testing.T, download_dir string) *hermes.TaskJob {
	t.Helper()
	write_file := func(name string, data []byte) string {
		path := filepath.Join(download_dir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	png_header := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	return &hermes.TaskJob{
		ID:       7,
		Name:     "Feishu document",
		Platform: PlatformID,
		Resources: []hermes.ResourceJob{
			{
				ID:       1,
				Name:     "document",
				Kind:     "text/html",
				Type:     "FILE",
				UniqueID: "DOCTOKEN_body",
				Extra:    map[string]string{feishu_resource_extra_key: feishu_resource_document},
				FilePath: write_file("DOCTOKEN_body", []byte(postprocess_test_document_html())),
			},
			{
				ID:       2,
				Name:     "images/IMG1",
				Kind:     "image/png",
				Type:     "FILE",
				UniqueID: "DOCTOKEN_asset_IMG1",
				Extra: map[string]string{
					feishu_resource_extra_key: feishu_resource_asset,
					feishu_asset_path_key:     "images/IMG1.png",
					feishu_asset_name_key:     "screenshot.png",
				},
				FilePath: write_file("DOCTOKEN_asset_IMG1", png_header),
			},
			{
				ID:       3,
				Name:     "files/PDF1",
				Kind:     "application/pdf",
				Type:     "FILE",
				UniqueID: "DOCTOKEN_asset_PDF1",
				Extra: map[string]string{
					feishu_resource_extra_key: feishu_resource_asset,
					feishu_asset_path_key:     "files/PDF1.pdf",
					feishu_asset_name_key:     "report.pdf",
				},
				FilePath: write_file("DOCTOKEN_asset_PDF1", []byte("%PDF-1.4")),
			},
			{
				ID:       4,
				Name:     "files/MP41",
				Kind:     "video/mp4",
				Type:     "FILE",
				UniqueID: "DOCTOKEN_asset_MP41",
				Extra: map[string]string{
					feishu_resource_extra_key: feishu_resource_asset,
					feishu_asset_path_key:     "files/MP41.mp4",
					feishu_asset_name_key:     "clip.mp4",
				},
				FilePath: write_file("DOCTOKEN_asset_MP41", []byte("mp4")),
			},
		},
	}
}

func Test_postprocess_embeds_images_and_links_files(t *testing.T) {
	download_dir := t.TempDir()
	task := postprocess_test_resources(t, download_dir)
	platform_adapter := NewFeishuAdapter()
	platform_adapter.filename_template = "{{filename}}_{{spec}}"

	err := platform_adapter.Postprocess(context.Background(), task, adapter.PostprocessDeps{Logger: zerolog.Nop()})
	if err != nil {
		t.Fatalf("Postprocess() error = %v", err)
	}

	processed, err := os.ReadFile(task.Resources[0].FilePath)
	if err != nil {
		t.Fatal(err)
	}
	html := string(processed)
	if !strings.Contains(html, `src="data:image/png;base64,`) {
		t.Fatal("image was not embedded as a data URI")
	}
	if strings.Contains(html, `src="images/IMG1.png"`) {
		t.Fatal("original image reference still present")
	}
	if !strings.Contains(html, `href="files_PDF1_.pdf"`) {
		t.Fatalf("pdf link was not rewritten to the final filename: %s", html)
	}
	if !strings.Contains(html, `href="files_MP41_.mp4"`) {
		t.Fatalf("mp4 link was not rewritten to the final filename: %s", html)
	}
	if !strings.Contains(html, `<video data-n="view-video" class="view-video" controls="" preload="metadata" src="files_MP41_.mp4"`) {
		t.Fatalf("video placeholder was not upgraded to a video element: %s", html)
	}
	if !strings.Contains(html, `download="report.pdf"`) {
		t.Fatal("pdf download attribute does not keep the original file name")
	}

	if len(task.Resources) != 3 {
		t.Fatalf("resource count = %d, want 3 after removing the embedded image", len(task.Resources))
	}
	if _, err := os.Stat(filepath.Join(download_dir, "DOCTOKEN_asset_IMG1")); !os.IsNotExist(err) {
		t.Fatal("embedded image file was not removed from disk")
	}
	document_resource := task.Resources[0]
	if document_resource.Extra[feishu_postprocessed_key] != "true" || document_resource.Size != int64(len(processed)) {
		t.Fatalf("document resource was not updated: %+v", document_resource)
	}
}

func Test_postprocess_is_idempotent(t *testing.T) {
	download_dir := t.TempDir()
	task := postprocess_test_resources(t, download_dir)
	platform_adapter := NewFeishuAdapter()
	deps := adapter.PostprocessDeps{Logger: zerolog.Nop()}
	if err := platform_adapter.Postprocess(context.Background(), task, deps); err != nil {
		t.Fatalf("first Postprocess() error = %v", err)
	}
	if err := platform_adapter.Postprocess(context.Background(), task, deps); err != nil {
		t.Fatalf("second Postprocess() error = %v", err)
	}
	if len(task.Resources) != 3 {
		t.Fatalf("resource count = %d, want 3 after the second run", len(task.Resources))
	}
}

func Test_final_resource_filename_matches_hermes_finalize(t *testing.T) {
	platform_adapter := NewFeishuAdapter()
	platform_adapter.filename_template = "{{filename}}_{{spec}}"
	resource := &hermes.ResourceJob{
		Name: "files/MP41",
		Kind: "video/mp4",
		Extra: map[string]string{
			feishu_resource_extra_key: feishu_resource_asset,
			feishu_asset_path_key:     "files/MP41.mp4",
		},
	}
	if name := platform_adapter.final_resource_filename(resource); name != "files_MP41_.mp4" {
		t.Fatalf("final_resource_filename() = %q, want files_MP41_.mp4", name)
	}
	platform_adapter.filename_template = ""
	if name := platform_adapter.final_resource_filename(resource); name != "files_MP41.mp4" {
		t.Fatalf("final_resource_filename() = %q, want files_MP41.mp4", name)
	}
}
