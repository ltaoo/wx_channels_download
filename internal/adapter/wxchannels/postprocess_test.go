package wxchannels

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"wx_channel/pkg/hermes"
)

func TestPostprocessFlowDefinitionsAreUsable(t *testing.T) {
	if wxchannelsPostprocessFlow.ID != "wxchannels_postprocess" {
		t.Fatalf("postprocess flow id = %q", wxchannelsPostprocessFlow.ID)
	}
	if wxchannelsOutputFlow.ID != "wxchannels_postprocess_output" {
		t.Fatalf("output flow id = %q", wxchannelsOutputFlow.ID)
	}
	for _, name := range []string{"route_resource", "route_output_format", "task_job_update", "done"} {
		if _, ok := wxchannelsPostprocessFlow.Nodes[name]; !ok {
			t.Fatalf("wxchannelsPostprocessFlow missing node %q", name)
		}
	}
}

func TestOutputFlowCompressesResourcesIntoZIP(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first")
	secondPath := filepath.Join(dir, "second")
	if err := os.WriteFile(firstPath, []byte("first-content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("second-content"), 0644); err != nil {
		t.Fatal(err)
	}
	task := &hermes.TaskJob{
		ID:       7,
		Name:     "测试压缩包",
		UniqueID: "bundle",
		Config:   map[string]any{"suffix": ".zip", "type": float64(-1)},
		Resources: []hermes.ResourceJob{
			{ID: 11, Name: "same", Kind: "text/plain", FilePath: firstPath},
			{ID: 12, Name: "same", Kind: "text/plain", FilePath: secondPath},
		},
	}

	err := runWXChannelsPostprocessFlow(wxchannelsOutputFlow, map[string]interface{}{
		wxchannelsPostprocessContextTaskType:   taskConfigType(task.Config),
		wxchannelsPostprocessContextTaskSuffix: taskConfigSuffix(task.Config),
		wxchannelsPostprocessRunKey: &postprocessRun{
			task:     task,
			basePath: dir,
		},
	})
	if err != nil {
		t.Fatalf("run output flow: %v", err)
	}

	if len(task.Resources) != 1 {
		t.Fatalf("resource count = %d", len(task.Resources))
	}
	archive := task.Resources[0]
	if archive.Kind != "application/zip" {
		t.Fatalf("archive resource = %#v", archive)
	}
	if archive.FilePath == "" {
		t.Fatalf("archive file path is empty")
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("first source was not removed: %v", err)
	}
	if _, err := os.Stat(secondPath); !os.IsNotExist(err) {
		t.Fatalf("second source was not removed: %v", err)
	}

	reader, err := zip.OpenReader(archive.FilePath)
	if err != nil {
		t.Fatalf("open ZIP: %v", err)
	}
	defer reader.Close()
	if len(reader.File) != 2 {
		t.Fatalf("ZIP entry count = %d", len(reader.File))
	}
	want := map[string]string{"same.txt": "first-content", "same_2.txt": "second-content"}
	for _, entry := range reader.File {
		file, err := entry.Open()
		if err != nil {
			t.Fatalf("open entry %q: %v", entry.Name, err)
		}
		data, readErr := io.ReadAll(file)
		_ = file.Close()
		if readErr != nil {
			t.Fatalf("read entry %q: %v", entry.Name, readErr)
		}
		if string(data) != want[entry.Name] {
			t.Fatalf("entry %q content = %q", entry.Name, data)
		}
		delete(want, entry.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing ZIP entries: %#v", want)
	}
}

func TestTaskJobUpdateNodeMutatesInputTaskJob(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "converted")
	if err := os.WriteFile(filePath, []byte("postprocessed-content"), 0644); err != nil {
		t.Fatal(err)
	}
	task := &hermes.TaskJob{Resources: []hermes.ResourceJob{{
		ID: 3, UniqueID: "resource-3", Name: "before", Kind: "audio/mpeg", FilePath: filePath,
	}}}
	processed := &task.Resources[0]
	processed.Name = "after"

	_, err := taskJobUpdateNode(map[string]interface{}{
		wxchannelsPostprocessRunKey: &postprocessRun{
			task:     task,
			resource: processed,
		},
	})
	if err != nil {
		t.Fatalf("update TaskJob: %v", err)
	}
	got := task.Resources[0]
	if got.Name != "after" {
		t.Fatalf("updated resource name = %q", got.Name)
	}
	if got.Size != int64(len("postprocessed-content")) {
		t.Fatalf("updated resource size=%d", got.Size)
	}
}

func TestPostprocessSkipsPlainFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("raw-content"), 0644); err != nil {
		t.Fatal(err)
	}
	task := &hermes.TaskJob{
		ID: 9,
		Config: map[string]any{"suffix": ".txt", "type": float64(0)},
		Resources: []hermes.ResourceJob{{
			ID: 33,
			Name: "plain",
			Type: "FILE",
			FilePath: filePath,
		}},
	}
	err := runWXChannelsPostprocessFlow(wxchannelsPostprocessFlow, map[string]interface{}{
		wxchannelsPostprocessContextInputFile:             filePath,
		wxchannelsPostprocessContextDecodeKey:             "",
		wxchannelsPostprocessContextResourceType:           "FILE",
		wxchannelsPostprocessContextResourceHasDecodeSecret: false,
		wxchannelsPostprocessContextTaskType:               taskConfigType(task.Config),
		wxchannelsPostprocessContextTaskSuffix:             taskConfigSuffix(task.Config),
		wxchannelsPostprocessRunKey: &postprocessRun{
			task:        task,
			resource:    &task.Resources[0],
			basePath:    dir,
			originalExt: ".txt",
		},
	})
	if err != nil {
		t.Fatalf("run flow: %v", err)
	}
	if got := task.Resources[0]; got.Name != "plain" {
		t.Fatalf("unexpected mutation: name=%q kind=%q", got.Name, got.Kind)
	}
}

func TestPostprocessStreamsPassThroughKeepsKind(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(filePath, []byte("stream-content"), 0644); err != nil {
		t.Fatal(err)
	}
	task := &hermes.TaskJob{
		ID: 12,
		Config: map[string]any{"type": float64(2), "suffix": ".mp4"},
		Resources: []hermes.ResourceJob{{
			ID: 44,
			Name: "stream",
			Type: "STREAM",
			FilePath: filePath,
		}},
	}
	err := runWXChannelsPostprocessFlow(wxchannelsPostprocessFlow, map[string]interface{}{
		wxchannelsPostprocessContextInputFile:             filePath,
		wxchannelsPostprocessContextDecodeKey:             "",
		wxchannelsPostprocessContextResourceType:           "STREAM",
		wxchannelsPostprocessContextResourceHasDecodeSecret: false,
		wxchannelsPostprocessContextTaskType:               taskConfigType(task.Config),
		wxchannelsPostprocessContextTaskSuffix:             taskConfigSuffix(task.Config),
		wxchannelsPostprocessRunKey: &postprocessRun{
			task:        task,
			resource:    &task.Resources[0],
			basePath:    dir,
			originalExt: filepath.Ext(filePath),
		},
	})
	if err != nil {
		t.Fatalf("run flow: %v", err)
	}
	if got := task.Resources[0]; got.Kind != "video/mp4" {
		t.Fatalf("resource kind=%q", got.Kind)
	}
}

func TestCanonicalExtensionForMIMEType(t *testing.T) {
	tests := map[string]string{
		"image/jpeg":                 ".jpg",
		"image/jpeg; charset=binary": ".jpg",
		"audio/mpeg":                 ".mp3",
		"video/x-matroska":           ".mkv",
		"application/zip":            ".zip",
		"application/octet-stream":   "",
		"application/x-unknown-format": "",
		"video":                      "",
	}
	for mimeType, want := range tests {
		if got := hermes.CanonicalExtensionForMIMEType(mimeType); got != want {
			t.Errorf("CanonicalExtensionForMIMEType(%q) = %q, want %q", mimeType, got, want)
		}
	}
}
