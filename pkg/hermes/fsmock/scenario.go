package fsmock

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"wx_channel/pkg/hermes"
)

// EventTracker records events emitted by the hermes engine.
type EventTracker struct {
	mu     sync.Mutex
	Events []hermes.EventType
}

// Record stores an event.
func (t *EventTracker) Record(taskID int, event hermes.EventType, _ *hermes.TaskProgress) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Events = append(t.Events, event)
}

// Snapshot returns a copy of recorded events.
func (t *EventTracker) Snapshot() []hermes.EventType {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := make([]hermes.EventType, len(t.Events))
	copy(c, t.Events)
	return c
}

// Count returns how many times the given event was emitted.
func (t *EventTracker) Count(event hermes.EventType) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, e := range t.Events {
		if e == event {
			n++
		}
	}
	return n
}

// WaitFor blocks until the given event is seen or the timeout expires.
func (t *EventTracker) WaitFor(event hermes.EventType, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		t.mu.Lock()
		for _, e := range t.Events {
			if e == event {
				t.mu.Unlock()
				return true
			}
		}
		t.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// HasEvent checks if an event was recorded.
func (t *EventTracker) HasEvent(event hermes.EventType) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, e := range t.Events {
		if e == event {
			return true
		}
	}
	return false
}

// ScenarioBuilder provides a fluent API for constructing test engine
// setups.
type ScenarioBuilder struct {
	Engine   *hermes.HermesEngine
	Store    *MockStore
	Tracker  *EventTracker
	cleanups []func()
}

// NewScenario creates a new scenario builder.
func NewScenario(maxConcurrent int) *ScenarioBuilder {
	b := &ScenarioBuilder{
		Tracker: &EventTracker{},
	}
	b.Store = NewMockStore(nil)
	b.Engine = hermes.New(hermes.HermesNewConfig{
		Store:  b.Store,
		Config: hermes.HermesEngineConfig{MaxConcurrent: maxConcurrent},
	})
	b.Engine.SetEventHandler(b.Tracker.Record)
	return b
}

// WithTask sets the task to be loaded by the store.
func (b *ScenarioBuilder) WithTask(task *hermes.TaskJob) *ScenarioBuilder {
	b.Store.mu.Lock()
	b.Store.taskInfo = task
	b.Store.mu.Unlock()
	return b
}

// WithMemoryDriver registers a MemoryDriver with the given data.
func (b *ScenarioBuilder) WithMemoryDriver(data []byte) *ScenarioBuilder {
	b.Engine.RegisterProtocol(&MemoryDriver{Data: data})
	return b
}

// WithMemoryDriverCT registers a MemoryDriver with data and content type.
func (b *ScenarioBuilder) WithMemoryDriverCT(data []byte, ct string) *ScenarioBuilder {
	b.Engine.RegisterProtocol(&MemoryDriver{Data: data, ContentType: ct})
	return b
}

// WithFailingDriver registers a FailingDriver.
func (b *ScenarioBuilder) WithFailingDriver(size int64) *ScenarioBuilder {
	b.Engine.RegisterProtocol(&FailingDriver{Size: size})
	return b
}

// WithHTTPDriver registers a test HTTP driver (compatible with httptest.Server).
func (b *ScenarioBuilder) WithHTTPDriver() *ScenarioBuilder {
	b.Engine.RegisterProtocol(&testHTTPDriver{})
	return b
}

// WithCleanup adds a cleanup function.
func (b *ScenarioBuilder) WithCleanup(f func()) *ScenarioBuilder {
	b.cleanups = append(b.cleanups, f)
	return b
}

// Cleanup runs all cleanup functions in reverse order.
func (b *ScenarioBuilder) Cleanup() {
	for i := len(b.cleanups) - 1; i >= 0; i-- {
		b.cleanups[i]()
	}
}

// Start starts the engine.
func (b *ScenarioBuilder) Start(taskID int) error {
	return b.Engine.StartTask(taskID)
}

// Pause pauses a running task.
func (b *ScenarioBuilder) Pause(taskID int) {
	b.Engine.PauseTask(taskID)
}

// WaitFor blocks until the given event occurs or timeout.
func (b *ScenarioBuilder) WaitFor(event hermes.EventType, timeout time.Duration) bool {
	return b.Tracker.WaitFor(event, timeout)
}

// BuildFilePath computes the expected output path for a task+URL.
func BuildFilePath(task *hermes.TaskJob, url string) (string, error) {
	return taskFilePath(task, url)
}

// taskFilePath mirrors the hermes package's taskFilePath logic.
func taskFilePath(info *hermes.TaskJob, endpointURL string) (string, error) {
	if info == nil || info.SavePath == "" {
		return "", fmt.Errorf("save path cannot be empty")
	}
	name := info.Name
	if name == "" {
		if u := endpointURL; u != "" {
			for i := len(u) - 1; i >= 0; i-- {
				if u[i] == '/' {
					name = u[i+1:]
					break
				}
			}
		}
	}
	for len(name) >= 3 && name[:3] == "../" {
		name = name[3:]
	}
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("unable to determine download filename")
	}
	return filepath.Join(info.SavePath, name), nil
}

// ---------------------------------------------------------------------------
// TaskJob constructors
// ---------------------------------------------------------------------------

// SingleFileHTTPTask creates a TaskJob for a single-file HTTP download.
func SingleFileHTTPTask(id int, name string, saveDir string, url string) *hermes.TaskJob {
	return &hermes.TaskJob{
		ID:       id,
		Name:     name,
		SavePath: saveDir,
		Resources: []hermes.ResourceJob{{
			ID: id * 100, Name: name, UniqueID: name,
			Endpoints: []hermes.Endpoint{{URL: url}},
		}},
	}
}

// CollectionTask creates a TaskJob with multiple resources.
func CollectionTask(id int, saveDir string, resources ...hermes.ResourceJob) *hermes.TaskJob {
	return &hermes.TaskJob{
		ID:        id,
		Name:      "collection",
		SavePath:  saveDir,
		Resources: resources,
	}
}

// NewMemoryResource creates a ResourceJob backed by a memory:// URL.
func NewMemoryResource(id int, name string) hermes.ResourceJob {
	return hermes.ResourceJob{
		ID:       id,
		Name:     name,
		UniqueID: name,
		Endpoints: []hermes.Endpoint{
			{ID: id, Protocol: "memory", URL: "memory://" + name},
		},
	}
}

// NewMemoryResourceWithKind creates a ResourceJob with a fallback Kind (MIME type).
// Used when Content-Type is unavailable and Kind should be derived from a known file extension.
func NewMemoryResourceWithKind(id int, name, kind string) hermes.ResourceJob {
	r := NewMemoryResource(id, name)
	r.Kind = kind
	return r
}

// NewMemoryResourceWithExt creates a ResourceJob with a MIME type derived from the given extension.
func NewMemoryResourceWithExt(id int, name, ext string) hermes.ResourceJob {
	r := NewMemoryResource(id, name)
	r.Kind = hermes.MIMETypeForExtension(ext)
	return r
}

// NewEndpoint creates an Endpoint.
func NewEndpoint(id int, protocol, url string, priority int) hermes.Endpoint {
	return hermes.Endpoint{
		ID:       id,
		Protocol: protocol,
		URL:      url,
		Priority: priority,
	}
}

// ---------------------------------------------------------------------------
// testHTTPDriver – HTTP protocol driver compatible with httptest.Server
// ---------------------------------------------------------------------------

type testHTTPDriver struct{}

func (d *testHTTPDriver) Protocols() []string { return []string{"http", "https"} }

func (d *testHTTPDriver) Prepare(ctx context.Context, endpoint hermes.Endpoint) (hermes.PreparedResource, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Range", "bytes=0-0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return hermes.PreparedResource{}, err
	}
	defer resp.Body.Close()
	prepared := hermes.PreparedResource{ContentType: resp.Header.Get("Content-Type")}
	if resp.StatusCode == http.StatusPartialContent {
		var start, end, total int64
		if _, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total); err == nil && start == 0 && end == 0 && total > 0 {
			prepared.Size = total
			prepared.SupportsRange = true
			return prepared, nil
		}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if resp.ContentLength > 0 {
			prepared.Size = resp.ContentLength
		}
		return prepared, nil
	}
	return hermes.PreparedResource{}, fmt.Errorf("HTTP probe returned status %d", resp.StatusCode)
}

func (d *testHTTPDriver) Open(ctx context.Context, endpoint hermes.Endpoint, request hermes.ReadRequest) (io.ReadCloser, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
	req.Header.Set("Accept-Encoding", "identity")
	if request.UseRange {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", request.OffsetStart, request.OffsetEnd))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
