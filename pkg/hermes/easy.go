package hermes

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

// Task is the high-level handle returned by CreateTask. CreateTask starts the
// download immediately; Wait blocks until it finishes or fails.
type Task struct {
	ID  int
	URL string

	engine *HermesEngine
	state  *memoryTaskState
}

// TaskOption customizes a task created by CreateTask before it is submitted to
// the scheduler.
type TaskOption func(*TaskJob)

// WithFilename overrides the filename derived from the URL.
func WithFilename(filename string) TaskOption {
	return func(job *TaskJob) {
		if job == nil || len(job.Resources) == 0 {
			return
		}
		job.Name = filename
		job.Resources[0].UniqueID = filename
	}
}

// WithSavePath sets the task's output directory relative to the engine base
// path.
func WithSavePath(savePath string) TaskOption {
	return func(job *TaskJob) {
		if job != nil {
			job.SavePath = savePath
		}
	}
}

// WithProxyServer routes the task through the given proxy server.
func WithProxyServer(proxyServer ProxyServer) TaskOption {
	return func(job *TaskJob) {
		if job != nil {
			job.ProxyServer = proxyServer
		}
	}
}

// WithHeaders adds HTTP headers to the task's primary endpoint.
func WithHeaders(headers map[string]string) TaskOption {
	return func(job *TaskJob) {
		if job == nil || len(job.Resources) == 0 || len(job.Resources[0].Endpoints) == 0 {
			return
		}
		job.Resources[0].Endpoints[0].Headers = cloneStringMap(headers)
	}
}

// WithCookies adds a Cookie header to the task's primary endpoint.
func WithCookies(cookies string) TaskOption {
	return func(job *TaskJob) {
		if job == nil || len(job.Resources) == 0 || len(job.Resources[0].Endpoints) == 0 {
			return
		}
		job.Resources[0].Endpoints[0].Cookies = cookies
	}
}

// OnEvent installs the high-level event callback. When the default in-memory
// store is active, lifecycle events emitted before registration are replayed.
func (d *HermesEngine) OnEvent(handler EventHandler) {
	if d == nil {
		return
	}
	if handler == nil || !d.replayEvents {
		d.SetEventHandler(handler)
		return
	}

	gate := make(chan struct{})
	wrapped := func(taskID int, event EventType, progress *TaskProgress) {
		<-gate
		handler(taskID, event, progress)
	}
	type recordedEvent struct {
		taskID int
		event  EventType
	}
	d.eventMu.Lock()
	d.onEvent = wrapped
	taskIDs := make([]int, 0, len(d.eventHistory))
	for taskID := range d.eventHistory {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Ints(taskIDs)
	recorded := make([]recordedEvent, 0)
	for _, taskID := range taskIDs {
		for _, event := range d.eventHistory[taskID] {
			recorded = append(recorded, recordedEvent{taskID: taskID, event: event})
		}
	}
	d.eventMu.Unlock()

	go func() {
		defer close(gate)
		for _, item := range recorded {
			handler(item.taskID, item.event, nil)
		}
	}()
}

// CreateTask creates and immediately starts a single-resource download task.
// A zero-value HermesNewConfig supplies the in-memory store and HTTP/HTTPS
// driver required by this method.
func (d *HermesEngine) CreateTask(rawURL string, options ...TaskOption) *Task {
	if d == nil {
		return failedTask(rawURL, errors.New("Hermes engine is nil"))
	}
	store, ok := d.store.(*memoryStore)
	if !ok {
		return failedTask(rawURL, errors.New("CreateTask requires the default in-memory store"))
	}

	cleanURL := strings.TrimSpace(rawURL)
	parsedURL, err := url.Parse(cleanURL)
	if err != nil || parsedURL.Scheme == "" {
		return failedTask(rawURL, errors.New("invalid download URL"))
	}
	protocol := strings.ToLower(parsedURL.Scheme)
	if (protocol == "http" || protocol == "https") && parsedURL.Host == "" {
		return failedTask(rawURL, errors.New("invalid download URL"))
	}

	filename := defaultTaskFilename(parsedURL)
	job := &TaskJob{
		Name: filename,
		Resources: []ResourceJob{{
			Type:     ResourceTypeFile,
			UniqueID: filename,
			Endpoints: []Endpoint{{
				Protocol: protocol,
				URL:      cleanURL,
			}},
		}},
		Config:   make(map[string]any),
		Metadata: make(map[string]any),
	}
	for _, option := range options {
		if option != nil {
			option(job)
		}
	}
	if len(job.Resources) == 0 {
		return failedTask(rawURL, errors.New("task has no downloadable resources"))
	}

	filename = filepath.Base(strings.TrimSpace(job.Resources[0].UniqueID))
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return failedTask(rawURL, errors.New("unable to determine download filename"))
	}
	processor := NewFilenameProcessor("", nil)
	filename, err = processor.SanitizeFilename(filename)
	if err != nil {
		return failedTask(rawURL, fmt.Errorf("invalid download filename: %w", err))
	}
	job.Resources[0].UniqueID = filename
	if strings.TrimSpace(job.Name) == "" {
		job.Name = filename
	}

	state, err := store.createTask(job)
	if err != nil {
		return failedTask(rawURL, err)
	}
	task := &Task{
		ID:     job.ID,
		URL:    cleanURL,
		engine: d,
		state:  state,
	}
	if err := d.StartTask(task.ID); err != nil {
		_ = store.RecordError(task.ID, err.Error())
	}
	return task
}

// Done is closed when the task finishes, fails, or is cancelled.
func (t *Task) Done() <-chan struct{} {
	if t == nil || t.state == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return t.state.done
}

// Wait blocks until the task reaches a terminal state.
func (t *Task) Wait() error {
	if t == nil {
		return errors.New("task is nil")
	}
	<-t.Done()
	return t.Err()
}

// Err returns the task failure after completion. It returns nil while the task
// is still running or when it finished successfully.
func (t *Task) Err() error {
	if t == nil {
		return errors.New("task is nil")
	}
	if t.state == nil {
		return errors.New("task state is unavailable")
	}
	t.state.mu.RLock()
	defer t.state.mu.RUnlock()
	if t.state.errorText == "" {
		return nil
	}
	return errors.New(t.state.errorText)
}

// FilePath returns the expected output path for this single-resource task.
func (t *Task) FilePath() string {
	if t == nil || t.state == nil || t.engine == nil {
		return ""
	}
	t.state.mu.RLock()
	defer t.state.mu.RUnlock()
	if t.state.job == nil || len(t.state.job.Resources) == 0 {
		return ""
	}
	resource := t.state.job.Resources[0]
	if resource.FilePath != "" {
		return resource.FilePath
	}
	name := strings.TrimSpace(resource.Name)
	if name == "" {
		name = resource.UniqueID
	}
	return t.engine.absFilePath(t.state.job.SavePath, name)
}

func failedTask(rawURL string, err error) *Task {
	return &Task{
		URL:   rawURL,
		state: newFailedMemoryTaskState(err),
	}
}

func defaultTaskFilename(parsedURL *url.URL) string {
	if parsedURL != nil {
		filename := filepath.Base(parsedURL.Path)
		if filename != "" && filename != "." && filename != "/" {
			if decoded, err := url.PathUnescape(filename); err == nil {
				return decoded
			}
			return filename
		}
	}
	return "download"
}
