package wxchannels

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"wx_channel/internal/pipeline"
	"wx_channel/pkg/hermes"
)

func TestBuildPostprocessPipelineFromJSON(t *testing.T) {
	p, err := buildPostprocessPipeline(wxchannelsPostprocessPipelineJSON)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	if p.Name != "wxchannels_postprocess" {
		t.Fatalf("pipeline name = %q", p.Name)
	}
	if p.StartNodeID != "route_resource" {
		t.Fatalf("pipeline start = %q", p.StartNodeID)
	}
	if len(p.Nodes) != 10 {
		t.Fatalf("pipeline node count = %d", len(p.Nodes))
	}
}

func TestPostprocessPipelineJSONDescribesDataAndControlFlow(t *testing.T) {
	var config postprocessPipelineConfig
	if err := json.Unmarshal([]byte(wxchannelsPostprocessPipelineJSON), &config); err != nil {
		t.Fatalf("unmarshal pipeline: %v", err)
	}
	if config.Inputs["input_file"] != "resource.file_path" {
		t.Fatalf("input_file source = %q", config.Inputs["input_file"])
	}
	if config.Outputs["output_file"] != "resource.file_path" {
		t.Fatalf("output_file target = %q", config.Outputs["output_file"])
	}

	nodes := make(map[string]postprocessNodeConfig, len(config.Nodes))
	for _, node := range config.Nodes {
		nodes[node.ID] = node
	}
	resourceSwitch := nodes["route_resource"]
	if resourceSwitch.Type != "switch" || len(resourceSwitch.Parameters.Rules) != 2 {
		t.Fatalf("resource switch = %#v", resourceSwitch)
	}
	streamRule := resourceSwitch.Parameters.Rules[0]
	if streamRule.Condition.Field != "resource.type" || streamRule.Condition.Operator != "equals" || streamRule.Output != "stream" {
		t.Fatalf("stream rule = %#v", streamRule)
	}
	connections := config.Connections["route_resource"]["stream"]
	if len(connections) != 1 || connections[0].Node != "stream_convert" {
		t.Fatalf("stream connections = %#v", connections)
	}
	if nodes["convert_mp3"].Inputs["decrypted_file"] != "pipeline.decrypted_file" {
		t.Fatalf("convert_mp3 input mapping missing")
	}
	if nodes["convert_mp3"].Outputs["mp3_file"] != "pipeline.mp3_file" {
		t.Fatalf("convert_mp3 output mapping missing")
	}
	zipConnections := config.Connections["route_output_format"]["zip"]
	if len(zipConnections) != 1 || zipConnections[0].Node != "zip_resources" {
		t.Fatalf("zip connections = %#v", zipConnections)
	}
}

func TestOutputPipelineCompressesResourcesIntoZIP(t *testing.T) {
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
		Config:   map[string]any{"suffix": ".zip"},
		Resources: []hermes.ResourceJob{
			{ID: 11, Name: "same", Kind: "text/plain", Extension: ".txt", FilePath: firstPath},
			{ID: 12, Name: "same", Kind: "text/plain", Extension: ".txt", FilePath: secondPath},
		},
	}
	p := mustBuildPostprocessPipelineAt(wxchannelsPostprocessPipelineJSON, "route_output_format")
	pc := pipeline.NewContext()
	pc.Values["postprocess_run"] = &postprocessRun{task: task, basePath: dir}

	if _, err := p.Run(context.Background(), pc); err != nil {
		t.Fatalf("run ZIP pipeline: %v", err)
	}
	if len(task.Resources) != 1 {
		t.Fatalf("resource count = %d", len(task.Resources))
	}
	archive := task.Resources[0]
	if archive.Kind != "application/zip" {
		t.Fatalf("archive resource = %#v", archive)
	}
	if archive.Extension != ".zip" || archive.Size <= 0 {
		t.Fatalf("archive persisted fields = extension %q size %d", archive.Extension, archive.Size)
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
	content := []byte("postprocessed-content")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}
	task := &hermes.TaskJob{Resources: []hermes.ResourceJob{{
		ID: 3, UniqueID: "resource-3", Name: "before", Extension: ".bin", Size: 1,
	}}}
	processed := task.Resources[0]
	processed.Name = "after"
	processed.FilePath = filePath
	processed.Kind = "audio/mpeg"
	pc := pipeline.NewContext()
	pc.Values["postprocess_run"] = &postprocessRun{task: task, resource: &processed}

	if _, err := TaskJobUpdateNode.Execute(context.Background(), pc); err != nil {
		t.Fatalf("update TaskJob: %v", err)
	}
	got := task.Resources[0]
	if got.Name != "after" || got.Extension != ".mp3" || got.Size != int64(len(content)) {
		t.Fatalf("updated resource = %#v", got)
	}
}

func TestCanonicalExtensionForMIMEType(t *testing.T) {
	tests := map[string]string{
		"image/jpeg":                   ".jpg",
		"image/jpeg; charset=binary":   ".jpg",
		"audio/mpeg":                   ".mp3",
		"video/x-matroska":             ".mkv",
		"application/zip":              ".zip",
		"application/octet-stream":     "",
		"application/x-unknown-format": "",
		"video":                        "",
	}
	for mimeType, want := range tests {
		if got := hermes.CanonicalExtensionForMIMEType(mimeType); got != want {
			t.Errorf("CanonicalExtensionForMIMEType(%q) = %q, want %q", mimeType, got, want)
		}
	}
}

func TestPostprocessPipelineSkipsPlainFile(t *testing.T) {
	p, err := buildPostprocessPipeline(wxchannelsPostprocessPipelineJSON)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	r := &hermes.ResourceJob{Type: "FILE", Extra: map[string]string{}}
	pc := newPostprocessTestContext(&hermes.TaskJob{}, r)

	if _, err := p.Run(context.Background(), pc); err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if state := pc.GetNodeState("done"); state != pipeline.StateCompleted {
		t.Fatalf("done state = %q", state)
	}
	if state := pc.GetNodeState("decrypt"); state != "" {
		t.Fatalf("decrypt should not run, state = %q", state)
	}
}

func TestPostprocessPipelinePassesThroughMP4Stream(t *testing.T) {
	p, err := buildPostprocessPipeline(wxchannelsPostprocessPipelineJSON)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	filePath := filepath.Join(t.TempDir(), "already-playable.mp4")
	if err := os.WriteFile(filePath, []byte("mp4"), 0644); err != nil {
		t.Fatal(err)
	}
	r := &hermes.ResourceJob{
		Type:     "STREAM",
		FilePath: filePath,
		Extra:    map[string]string{},
	}
	pc := newPostprocessTestContext(&hermes.TaskJob{}, r)

	if _, err := p.Run(context.Background(), pc); err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if state := pc.GetNodeState("stream_convert"); state != pipeline.StateCompleted {
		t.Fatalf("stream_convert state = %q", state)
	}
	if state := pc.GetNodeState("finalize_stream"); state != pipeline.StateCompleted {
		t.Fatalf("finalize_stream state = %q", state)
	}
	if state := pc.GetNodeState("decrypt"); state != "" {
		t.Fatalf("decrypt should not run, state = %q", state)
	}
}

func newPostprocessTestContext(task *hermes.TaskJob, resource *hermes.ResourceJob) *pipeline.Context {
	if len(task.Resources) == 0 {
		task.Resources = append(task.Resources, *resource)
		resource = &task.Resources[0]
	}
	pc := pipeline.NewContext()
	pc.Values["input_file"] = resource.FilePath
	pc.Values["decode_key"] = resource.Extra["decode_key"]
	pc.Values["postprocess_run"] = &postprocessRun{
		task:        task,
		resource:    resource,
		originalExt: ".mp4",
		log:         func(string, ...interface{}) {},
	}
	return pc
}
