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
func (t *EventTracker) Record(taskID int, event hermes.EventType) {
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
	Engine   *hermes.Engine
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
	b.Engine = hermes.New(b.Store, b.Tracker.Record, maxConcurrent)
	return b
}

// WithTask sets the task to be loaded by the store.
func (b *ScenarioBuilder) WithTask(task *hermes.Task) *ScenarioBuilder {
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
	return b.Engine.Start(taskID)
}

// Pause pauses a running task.
func (b *ScenarioBuilder) Pause(taskID int) {
	b.Engine.Pause(taskID)
}

// WaitFor blocks until the given event occurs or timeout.
func (b *ScenarioBuilder) WaitFor(event hermes.EventType, timeout time.Duration) bool {
	return b.Tracker.WaitFor(event, timeout)
}

// BuildFilePath computes the expected output path for a task+URL.
func BuildFilePath(task *hermes.Task, url string) (string, error) {
	return taskFilePath(task, url)
}

// taskFilePath mirrors the hermes package's taskFilePath logic.
func taskFilePath(info *hermes.Task, endpointURL string) (string, error) {
	if info == nil || info.SavePath == "" {
		return "", fmt.Errorf("保存路径不能为空")
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
		return "", fmt.Errorf("无法确定下载文件名")
	}
	if info.ResourceType == hermes.ResourceTypeFile && filepath.Base(info.SavePath) == name {
		return info.SavePath, nil
	}
	return filepath.Join(info.SavePath, name), nil
}

// ---------------------------------------------------------------------------
// Task constructors
// ---------------------------------------------------------------------------

// SingleFileHTTPTask creates a Task for a single-file HTTP download.
func SingleFileHTTPTask(id int, name string, saveDir string, url string) *hermes.Task {
	return &hermes.Task{
		ID:           id,
		Name:         name,
		SavePath:     saveDir,
		ResourceType: hermes.ResourceTypeFile,
		ResourceID:   id * 100,
		URL:          url,
	}
}

// CollectionTask creates a Task with multiple resources.
func CollectionTask(id int, saveDir string, resources ...hermes.Resource) *hermes.Task {
	return &hermes.Task{
		ID:           id,
		Name:         "collection",
		SavePath:     saveDir,
		ResourceType: hermes.ResourceTypeCollection,
		ResourceID:   0,
		Resources:    resources,
	}
}

// NewMemoryResource creates a Resource backed by a memory:// URL.
func NewMemoryResource(id int, name string) hermes.Resource {
	return hermes.Resource{
		ID:   id,
		Name: name,
		Endpoints: []hermes.Endpoint{
			{ID: id, Protocol: "memory", URL: "memory://" + name},
		},
	}
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
