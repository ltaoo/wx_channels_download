package hermes

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// Task is the high-level handle returned by CreateTask. CreateTask starts the
// download immediately; Wait blocks until it finishes or fails.
type Task struct {
	ID  int
	URL string

	engine *HermesEngine
	state  *memory_task_state
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

// WithDownloadDir sets the task's output directory relative to the engine base
// path.
func WithDownloadDir(download_dir string) TaskOption {
	return func(job *TaskJob) {
		if job != nil {
			job.DownloadDir = download_dir
		}
	}
}

// WithProxyServer routes the task through the given proxy server.
func WithProxyServer(proxy_server ProxyServer) TaskOption {
	return func(job *TaskJob) {
		if job != nil {
			job.ProxyServer = proxy_server
		}
	}
}

// WithHeaders adds HTTP headers to the task's primary endpoint.
func WithHeaders(headers map[string]string) TaskOption {
	return func(job *TaskJob) {
		if job == nil || len(job.Resources) == 0 || len(job.Resources[0].Endpoints) == 0 {
			return
		}
		job.Resources[0].Endpoints[0].Headers = clone_string_map(headers)
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
// store is active, replayable events emitted before registration are replayed.
func (d *HermesEngine) OnEvent(handler EventHandler) {
	if d == nil {
		return
	}
	if handler == nil || !d.replay_events {
		d.event_mu.Lock()
		d.on_event = handler
		d.event_mu.Unlock()
		return
	}

	gate := make(chan struct{})
	wrapped := func(event EventType, data EventData) {
		<-gate
		handler(event, data)
	}
	d.event_mu.Lock()
	d.on_event = wrapped
	recorded := append([]event_record(nil), d.event_history...)
	d.event_mu.Unlock()

	go func() {
		defer close(gate)
		for _, item := range recorded {
			handler(item.event, item.data)
		}
	}()
}

// CreateTask creates and immediately starts a single-resource download task.
// A zero-value HermesNewConfig supplies the in-memory store and HTTP/HTTPS
// driver required by this method.
func (d *HermesEngine) CreateTask(raw_url string, options ...TaskOption) *Task {
	if d == nil {
		return failed_task(raw_url, errors.New("Hermes engine is nil"))
	}
	store, ok := d.store.(*memory_store)
	if !ok {
		return failed_task(raw_url, errors.New("CreateTask requires the default in-memory store"))
	}

	clean_url := strings.TrimSpace(raw_url)
	parsed_url, err := url.Parse(clean_url)
	if err != nil || parsed_url.Scheme == "" {
		return failed_task(raw_url, errors.New("invalid download URL"))
	}
	protocol := strings.ToLower(parsed_url.Scheme)
	if (protocol == "http" || protocol == "https") && parsed_url.Host == "" {
		return failed_task(raw_url, errors.New("invalid download URL"))
	}

	filename := default_task_filename(parsed_url)
	job := &TaskJob{
		Name: filename,
		Resources: []ResourceJob{{
			Type:     ResourceTypeFile,
			UniqueID: filename,
			Endpoints: []Endpoint{{
				Protocol: protocol,
				URL:      clean_url,
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
		return failed_task(raw_url, errors.New("task has no downloadable resources"))
	}

	filename = filepath.Base(strings.TrimSpace(job.Resources[0].UniqueID))
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return failed_task(raw_url, errors.New("unable to determine download filename"))
	}
	processor := NewFilenameProcessor("", nil)
	filename, err = processor.SanitizeFilename(filename)
	if err != nil {
		return failed_task(raw_url, fmt.Errorf("invalid download filename: %w", err))
	}
	job.Resources[0].UniqueID = filename
	if strings.TrimSpace(job.Name) == "" {
		job.Name = filename
	}

	state, err := store.create_task(job)
	if err != nil {
		return failed_task(raw_url, err)
	}
	task := &Task{
		ID:     job.ID,
		URL:    clean_url,
		engine: d,
		state:  state,
	}
	if err := d.StartCreatedTask(task.ID); err != nil {
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
	if t.state.error_text == "" {
		return nil
	}
	return errors.New(t.state.error_text)
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
	return t.engine.abs_file_path(resource_download_dir(t.state.job, &resource), name)
}

func failed_task(raw_url string, err error) *Task {
	return &Task{
		URL:   raw_url,
		state: new_failed_memory_task_state(err),
	}
}

func default_task_filename(parsed_url *url.URL) string {
	if parsed_url != nil {
		filename := filepath.Base(parsed_url.Path)
		if filename != "" && filename != "." && filename != "/" {
			if decoded, err := url.PathUnescape(filename); err == nil {
				return decoded
			}
			return filename
		}
	}
	return "download"
}
