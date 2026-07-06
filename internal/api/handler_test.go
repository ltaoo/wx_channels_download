package api

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"

	"wx_channel/pkg/util"
)

func writeTestImage(t *testing.T, path string, width int, height int, encode func(io.Writer, image.Image) error) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image %s: %v", path, err)
	}
	defer file.Close()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y += 1 {
		for x := 0; x < width; x += 1 {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 120, A: 255})
		}
	}
	if err := encode(file, img); err != nil {
		t.Fatalf("encode image %s: %v", path, err)
	}
}

func TestProcessTaskFilenameAllowsSameBaseWithDifferentSuffix(t *testing.T) {
	c := &APIClient{
		formatter: util.NewFilenameProcessor("", make(map[string]int)),
	}

	name, dir, err := c.processTaskFilename("clip", ".mp4")
	if err != nil {
		t.Fatalf("process mp4 filename: %v", err)
	}
	if name != "clip.mp4" {
		t.Fatalf("first mp4 name = %q, want %q", name, "clip.mp4")
	}
	if dir != "" {
		t.Fatalf("first mp4 dir = %q, want empty", dir)
	}

	name, dir, err = c.processTaskFilename("clip", ".jpg")
	if err != nil {
		t.Fatalf("process jpg filename: %v", err)
	}
	if name != "clip.jpg" {
		t.Fatalf("jpg name = %q, want %q", name, "clip.jpg")
	}
	if dir != "" {
		t.Fatalf("jpg dir = %q, want empty", dir)
	}

	name, _, err = c.processTaskFilename("clip", ".mp4")
	if err != nil {
		t.Fatalf("process duplicate mp4 filename: %v", err)
	}
	if name != "clip(1).mp4" {
		t.Fatalf("duplicate mp4 name = %q, want %q", name, "clip(1).mp4")
	}
}

func TestCountDownloadTaskStatusesNormalizesFailureAliases(t *testing.T) {
	counts := countDownloadTaskStatuses([]*downloadpkg.Task{
		{Status: base.DownloadStatusError},
		{Status: base.Status("failed")},
		{Status: base.Status("fail")},
		{Status: base.Status("failure")},
		{Status: base.Status("errored")},
		{Status: base.Status("paused")},
		{Status: base.Status("pending")},
		{Status: base.Status("completed")},
	})

	if counts["total"] != 8 {
		t.Fatalf("total = %d, want 8", counts["total"])
	}
	if counts["error"] != 5 {
		t.Fatalf("error = %d, want 5", counts["error"])
	}
	if counts["pause"] != 1 {
		t.Fatalf("pause = %d, want 1", counts["pause"])
	}
	if counts["wait"] != 1 {
		t.Fatalf("wait = %d, want 1", counts["wait"])
	}
	if counts["done"] != 1 {
		t.Fatalf("done = %d, want 1", counts["done"])
	}
}

func TestDownloadTaskStatusFilterMapsWaitToWaitingStatuses(t *testing.T) {
	filter := downloadTaskStatusFilter("wait")
	if filter == nil {
		t.Fatal("filter is nil, want statuses")
	}
	if len(filter.Statuses) != 2 {
		t.Fatalf("len(statuses) = %d, want 2", len(filter.Statuses))
	}
	if filter.Statuses[0] != base.DownloadStatusReady {
		t.Fatalf("statuses[0] = %q, want %q", filter.Statuses[0], base.DownloadStatusReady)
	}
	if filter.Statuses[1] != base.DownloadStatusWait {
		t.Fatalf("statuses[1] = %q, want %q", filter.Statuses[1], base.DownloadStatusWait)
	}
}

func TestDownloadTaskPauseAllFilterDefaultsToPausableStatuses(t *testing.T) {
	filter := downloadTaskPauseAllFilter("")
	if filter == nil {
		t.Fatal("filter is nil, want default pause statuses")
	}
	want := []base.Status{
		base.DownloadStatusReady,
		base.DownloadStatusRunning,
		base.DownloadStatusWait,
	}
	if len(filter.Statuses) != len(want) {
		t.Fatalf("len(statuses) = %d, want %d", len(filter.Statuses), len(want))
	}
	for i := range want {
		if filter.Statuses[i] != want[i] {
			t.Fatalf("statuses[%d] = %q, want %q", i, filter.Statuses[i], want[i])
		}
	}
}

func TestParseDownloadTaskTextParsesLines(t *testing.T) {
	tasks := parseDownloadTaskText(" https://example.com/a.mp4 \n\nhttps://example.com/b.mp4", "", nil)
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
	if tasks[0].URL != "https://example.com/a.mp4" {
		t.Fatalf("tasks[0].URL = %q", tasks[0].URL)
	}
	if tasks[1].URL != "https://example.com/b.mp4" {
		t.Fatalf("tasks[1].URL = %q", tasks[1].URL)
	}
}

func TestParseDownloadTaskTextParsesJSONLineWithDefaults(t *testing.T) {
	tasks := parseDownloadTaskText(
		`{"url":"https://example.com/a.mp4","filename":"a.mp4","extra":{"source":"line"}}`,
		"batch",
		map[string]string{"source": "default", "type": "manual"},
	)
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.URL != "https://example.com/a.mp4" {
		t.Fatalf("task.URL = %q", task.URL)
	}
	if task.Filename != "a.mp4" {
		t.Fatalf("task.Filename = %q", task.Filename)
	}
	if task.Dir != "batch" {
		t.Fatalf("task.Dir = %q", task.Dir)
	}
	if task.Extra["source"] != "line" {
		t.Fatalf("task.Extra[source] = %q", task.Extra["source"])
	}
	if task.Extra["type"] != "manual" {
		t.Fatalf("task.Extra[type] = %q", task.Extra["type"])
	}
}

func TestListImageFilesReturnsRecursiveImages(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	writeTestImage(t, filepath.Join(root, "a.jpg"), 3, 2, func(w io.Writer, img image.Image) error {
		return jpeg.Encode(w, img, nil)
	})
	writeTestImage(t, filepath.Join(nested, "b.PNG"), 4, 5, png.Encode)
	files := map[string]string{
		filepath.Join(root, "ignore.txt"):   "txt",
		filepath.Join(nested, "ignore.mp4"): "mp4",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	images, err := (&APIClient{}).listImageFiles(root)
	if err != nil {
		t.Fatalf("listImageFiles: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("len(images) = %d, want 2: %#v", len(images), images)
	}
	if images[0].Name != "a.jpg" {
		t.Fatalf("images[0].Name = %q, want a.jpg", images[0].Name)
	}
	if images[0].Width != 3 || images[0].Height != 2 {
		t.Fatalf("images[0] dimensions = %dx%d, want 3x2", images[0].Width, images[0].Height)
	}
	if images[1].Name != "nested/b.PNG" {
		t.Fatalf("images[1].Name = %q, want nested/b.PNG", images[1].Name)
	}
	if images[1].Width != 4 || images[1].Height != 5 {
		t.Fatalf("images[1] dimensions = %dx%d, want 4x5", images[1].Width, images[1].Height)
	}
	for _, image := range images {
		if !filepath.IsAbs(image.Path) {
			t.Fatalf("image path is not absolute: %q", image.Path)
		}
		if image.URL == "" {
			t.Fatalf("image URL is empty for %q", image.Name)
		}
	}
}
