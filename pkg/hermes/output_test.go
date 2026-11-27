package hermes

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type collision_driver struct {
	opened atomic.Int32
	ready  chan struct{}
	closed sync.WaitGroup
}

func (d *collision_driver) Protocols() []string { return []string{"collision"} }
func (d *collision_driver) Prepare(context.Context, Endpoint) (PreparedResource, error) {
	return PreparedResource{Size: 4, SupportsRange: true, ContentType: "video/mp4"}, nil
}
func (d *collision_driver) Open(ctx context.Context, _ Endpoint, _ ReadRequest) (io.ReadCloser, error) {
	if d.opened.Add(1) == 2 {
		close(d.ready)
	}
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case <-d.ready:
	}
	return &collision_reader{Reader: bytes.NewReader([]byte("abcd")), driver: d}, nil
}

type collision_reader struct {
	*bytes.Reader
	driver *collision_driver
	once   sync.Once
}

func (r *collision_reader) Close() error {
	r.once.Do(func() { r.driver.closed.Done() })
	r.driver.closed.Wait()
	return nil
}

func TestConcurrentDuplicateDownloadsKeepIndependentBytesAndNames(t *testing.T) {
	root := t.TempDir()
	driver := &collision_driver{ready: make(chan struct{})}
	driver.closed.Add(2)
	engine := New(HermesNewConfig{Config: HermesEngineConfig{BasePath: root, MaxConcurrent: 2}})
	engine.RegisterProtocol(driver)
	options := []TaskOption{WithFilename("same"), func(job *TaskJob) { job.Config["duplicate"] = true }}
	first := engine.CreateTask("collision://media/file", options...)
	second := engine.CreateTask("collision://media/file", options...)
	for _, task := range []*Task{first, second} {
		select {
		case <-task.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent downloads stalled")
		}
		if err := task.Err(); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(task.FilePath())
		if err != nil || string(body) != "abcd" {
			t.Fatalf("path=%s body=%q err=%v", task.FilePath(), body, err)
		}
	}
	if first.FilePath() == second.FilePath() {
		t.Fatal("duplicate tasks share a final output")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("leftover transfer files: %v", entries)
	}
}

func TestFinalizeFailurePreservesCompletedResourceAndRetry(t *testing.T) {
	root := t.TempDir()
	store := new_memory_store()
	job := &TaskJob{DownloadDir: root, Name: "task", Resources: []ResourceJob{
		{UniqueID: "raw-a", Name: "first", Kind: "video/mp4", Size: 4, Downloaded: 4},
		{UniqueID: "raw-b", Name: "second", Kind: "video/mp4", Size: 4, Downloaded: 4},
	}}
	if _, err := store.create_task(job); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "raw-a"), []byte("abcd"), 0600); err != nil {
		t.Fatal(err)
	}
	engine := New(HermesNewConfig{Store: store})
	if err := engine.finish_task(job); err == nil {
		t.Fatal("expected missing second resource to fail")
	}
	persisted, err := store.LoadTask(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	first := &persisted.Resources[0]
	if first.Name != "first.mp4" || first.Extra["_hermes_finalized"] != "1" {
		t.Fatalf("first resource not persisted: %+v", first)
	}
	// No endpoints are available. A finalized resource must be usable without another source request.
	path, err := engine.download_resource(context.Background(), persisted, first)
	if err != nil || path != filepath.Join(root, "first.mp4") {
		t.Fatalf("completed resource cannot be recovered: path=%q err=%v", path, err)
	}
	if err := os.WriteFile(filepath.Join(root, "raw-b"), []byte("efgh"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := engine.finish_task(persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Resources[0].Name != "first.mp4" {
		t.Fatalf("retry reapplied final filename: %q", persisted.Resources[0].Name)
	}
}

type failing_output_store struct{ *memory_store }

func (s *failing_output_store) UpdateResourceOutput(ResourceOutputUpdate) error {
	return errors.New("database unavailable")
}

func TestFinalizeRollsBackMoveWhenPersistenceFails(t *testing.T) {
	root := t.TempDir()
	store := &failing_output_store{new_memory_store()}
	job := &TaskJob{DownloadDir: root, Name: "task", Resources: []ResourceJob{{UniqueID: "raw", Name: "final", Kind: "video/mp4"}}}
	if _, err := store.create_task(job); err != nil {
		t.Fatal(err)
	}
	old_path := filepath.Join(root, "raw")
	if err := os.WriteFile(old_path, []byte("abcd"), 0600); err != nil {
		t.Fatal(err)
	}
	engine := New(HermesNewConfig{Store: store})
	if err := engine.finish_task(job); err == nil {
		t.Fatal("expected persistence failure")
	}
	if body, err := os.ReadFile(old_path); err != nil || string(body) != "abcd" {
		t.Fatalf("original output lost: %q %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(root, "final.mp4")); !os.IsNotExist(err) {
		t.Fatalf("move not rolled back: %v", err)
	}
	if job.Resources[0].Extra["_hermes_finalized"] != "" {
		t.Fatal("failed output marked finalized")
	}
}

type postprocessor_func func(context.Context, *TaskJob) error

func (f postprocessor_func) Process(ctx context.Context, job *TaskJob) error { return f(ctx, job) }

type fail_second_finalize_store struct {
	*memory_store
	failed bool
}

type stale_source_driver struct{ probes atomic.Int32 }

func (d *stale_source_driver) Protocols() []string { return []string{"source"} }
func (d *stale_source_driver) Prepare(context.Context, Endpoint) (PreparedResource, error) {
	d.probes.Add(1)
	return PreparedResource{Size: 4, ContentType: "video/mp4"}, nil
}
func (d *stale_source_driver) Open(context.Context, Endpoint, ReadRequest) (io.ReadCloser, error) {
	return nil, errors.New("source is no longer available")
}

func (s *fail_second_finalize_store) UpdateResourceOutput(update ResourceOutputUpdate) error {
	if update.ResourceID == 2 && update.ResourceExtra["_hermes_finalized"] == "1" && !s.failed {
		s.failed = true
		return errors.New("temporary write failure")
	}
	return s.memory_store.UpdateResourceOutput(update)
}

func TestRetryDoesNotRepeatPostprocessing(t *testing.T) {
	root := t.TempDir()
	store := &fail_second_finalize_store{memory_store: new_memory_store()}
	job := &TaskJob{DownloadDir: root, Name: "task", Resources: []ResourceJob{
		{UniqueID: "raw-a", Name: "first", Kind: "video/mp4", Size: 4, Downloaded: 4},
		{UniqueID: "raw-b", Name: "second", Kind: "video/mp4", Size: 4, Downloaded: 4},
	}}
	for i := range job.Resources {
		job.Resources[i].Endpoints = []Endpoint{{Protocol: "source", URL: "source://expired"}}
	}
	if _, err := store.create_task(job); err != nil {
		t.Fatal(err)
	}
	for _, resource := range job.Resources {
		if err := os.WriteFile(filepath.Join(root, resource.UniqueID), []byte("abcd"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	engine := New(HermesNewConfig{Store: store})
	driver := &stale_source_driver{}
	engine.RegisterProtocol(driver)
	processed := 0
	engine.SetPostprocessor(postprocessor_func(func(_ context.Context, task *TaskJob) error {
		processed++
		for i := range task.Resources {
			resource := &task.Resources[i]
			body, err := os.ReadFile(resource.FilePath)
			if err != nil {
				return err
			}
			path := resource.FilePath + ".processed"
			if err := os.WriteFile(path, append(body, 'p'), 0600); err != nil {
				return err
			}
			if err := os.Remove(resource.FilePath); err != nil {
				return err
			}
			resource.FilePath, resource.Size, resource.Downloaded = path, 5, 5
		}
		return nil
	}))
	if err := engine.finish_task(job); err == nil {
		t.Fatal("expected second output persistence failure")
	}
	reloaded, err := store.LoadTask(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.ctx = context.Background()
	if err := engine.run(reloaded); err != nil {
		t.Fatal(err)
	}
	if driver.probes.Load() != 0 {
		t.Fatal("retry probed an already processed resource and replaced its output size")
	}
	if processed != 1 {
		t.Fatalf("postprocessor ran %d times", processed)
	}
	for _, resource := range reloaded.Resources {
		body, err := os.ReadFile(resource.FilePath)
		if err != nil || string(body) != "abcdp" {
			t.Fatalf("output=%q err=%v", body, err)
		}
	}
}

func TestOutputOwnershipRejectsConflictingTransfers(t *testing.T) {
	engine := New(HermesNewConfig{})
	path := filepath.Join(t.TempDir(), "shared")
	if err := engine.claim_output_path(1, 1, path); err != nil {
		t.Fatal(err)
	}
	if err := engine.claim_output_path(2, 2, path); err == nil {
		t.Fatal("another task can overwrite an active transfer")
	}
	name := ResourceDownloadName(2, 2, strings.Repeat("界", 100), true)
	if len(name)+len(partial_file_suffix) > 255 {
		t.Fatal("duplicate transfer name exceeds filesystem limit")
	}
}
