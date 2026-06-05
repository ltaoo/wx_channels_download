package download

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TaskStatus string

const (
	TaskStatusReady       TaskStatus = "ready"
	TaskStatusResolving   TaskStatus = "resolving"
	TaskStatusDownloading TaskStatus = "downloading"
	TaskStatusProcessing  TaskStatus = "processing"
	TaskStatusDone        TaskStatus = "done"
	TaskStatusError       TaskStatus = "error"
)

type Progress struct {
	DownloadedBytes int64   `json:"downloaded_bytes"`
	TotalBytes      int64   `json:"total_bytes"`
	Percent         float64 `json:"percent"`
}

type Task struct {
	ID        string           `json:"id"`
	Status    TaskStatus       `json:"status"`
	Resolved  *ResolvedRequest `json:"resolved"`
	FilePath  string           `json:"file_path"`
	Progress  Progress         `json:"progress"`
	Error     string           `json:"error,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type EventKind string

const (
	EventTaskCreated  EventKind = "task_created"
	EventTaskProgress EventKind = "task_progress"
	EventTaskDone     EventKind = "task_done"
	EventTaskError    EventKind = "task_error"
)

type Event struct {
	Kind EventKind `json:"kind"`
	Task *Task     `json:"task"`
}

type EventHandler func(Event)

type SourceExecutor interface {
	Name() string
	CanHandle(source DownloadSpec) bool
	Execute(ctx context.Context, req ExecuteRequest) error
}

type ExecuteRequest struct {
	Resolved   *ResolvedRequest
	Source     DownloadSpec
	DestPath   string
	OnProgress func(Progress)
}

type Downloader struct {
	router      *Router
	downloadDir string
	executors   []SourceExecutor
	onEvent     EventHandler

	mu    sync.RWMutex
	tasks map[string]*Task
}

type DownloaderOption func(*Downloader)

func WithExecutor(executor SourceExecutor) DownloaderOption {
	return func(d *Downloader) {
		if executor != nil {
			d.executors = append(d.executors, executor)
		}
	}
}

func WithEventHandler(fn EventHandler) DownloaderOption {
	return func(d *Downloader) {
		d.onEvent = fn
	}
}

func NewDownloader(router *Router, downloadDir string, opts ...DownloaderOption) *Downloader {
	d := &Downloader{
		router:      router,
		downloadDir: downloadDir,
		tasks:       make(map[string]*Task),
	}
	d.executors = append(d.executors, NewHTTPExecutor(nil), NewZipExecutor(nil), NewInlineHTMLExecutor())
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *Downloader) Create(ctx context.Context, input ResolveInput) (*Task, error) {
	if d.router == nil {
		return nil, fmt.Errorf("platform router is nil")
	}
	now := time.Now()
	task := &Task{
		ID:        newTaskID(),
		Status:    TaskStatusResolving,
		CreatedAt: now,
		UpdatedAt: now,
	}
	d.store(task)

	resolved, err := d.router.Resolve(ctx, input)
	if err != nil {
		d.fail(task, err)
		return task, err
	}
	task.Resolved = resolved
	task.Status = TaskStatusReady
	task.FilePath = d.resolveDestPath(resolved)
	task.UpdatedAt = time.Now()
	d.emit(EventTaskCreated, task)
	return task, nil
}

func (d *Downloader) CreateResolved(resolved *ResolvedRequest) (*Task, error) {
	if resolved == nil {
		return nil, fmt.Errorf("resolved request is nil")
	}
	now := time.Now()
	task := &Task{
		ID:        newTaskID(),
		Status:    TaskStatusReady,
		Resolved:  resolved,
		FilePath:  d.resolveDestPath(resolved),
		CreatedAt: now,
		UpdatedAt: now,
	}
	d.store(task)
	d.emit(EventTaskCreated, task)
	return task, nil
}

func (d *Downloader) Start(ctx context.Context, taskID string) error {
	task := d.GetTask(taskID)
	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if task.Resolved == nil {
		return fmt.Errorf("task has no resolved request: %s", taskID)
	}
	executor := d.executorFor(task.Resolved.Download)
	if executor == nil {
		err := fmt.Errorf("no executor for source protocol %q", task.Resolved.Download.Protocol)
		d.fail(task, err)
		return err
	}

	d.update(task, func(t *Task) {
		t.Status = TaskStatusDownloading
	})

	err := executor.Execute(ctx, ExecuteRequest{
		Resolved: task.Resolved,
		Source:   task.Resolved.Download,
		DestPath: task.FilePath,
		OnProgress: func(progress Progress) {
			d.update(task, func(t *Task) {
				t.Progress = progress
			})
			d.emit(EventTaskProgress, task)
		},
	})
	if err != nil {
		d.fail(task, err)
		return err
	}

	d.update(task, func(t *Task) {
		t.Status = TaskStatusDone
		if t.Progress.TotalBytes > 0 {
			t.Progress.DownloadedBytes = t.Progress.TotalBytes
			t.Progress.Percent = 100
		}
	})
	d.emit(EventTaskDone, task)
	return nil
}

func (d *Downloader) CreateAndStart(ctx context.Context, input ResolveInput) (*Task, error) {
	task, err := d.Create(ctx, input)
	if err != nil {
		return task, err
	}
	if err := d.Start(ctx, task.ID); err != nil {
		return task, err
	}
	return task, nil
}

func (d *Downloader) GetTask(id string) *Task {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.tasks[id]
}

func (d *Downloader) Tasks() []*Task {
	d.mu.RLock()
	defer d.mu.RUnlock()
	tasks := make([]*Task, 0, len(d.tasks))
	for _, task := range d.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

func (d *Downloader) executorFor(source DownloadSpec) SourceExecutor {
	for _, executor := range d.executors {
		if executor.CanHandle(source) {
			return executor
		}
	}
	return nil
}

func (d *Downloader) resolveDestPath(resolved *ResolvedRequest) string {
	filename := strings.TrimSpace(resolved.Filename)
	if filename == "" {
		filename = strings.TrimSpace(resolved.Title)
	}
	if filename == "" {
		filename = strings.TrimSpace(resolved.ContentID)
	}
	if filename == "" {
		filename = "download_" + newTaskID()
	}
	suffix := resolved.Suffix
	if suffix != "" && !strings.HasSuffix(strings.ToLower(filename), strings.ToLower(suffix)) {
		filename += suffix
	}
	return filepath.Join(d.downloadDir, filename)
}

func (d *Downloader) store(task *Task) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tasks[task.ID] = task
}

func (d *Downloader) update(task *Task, fn func(*Task)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fn(task)
	task.UpdatedAt = time.Now()
}

func (d *Downloader) fail(task *Task, err error) {
	d.update(task, func(t *Task) {
		t.Status = TaskStatusError
		t.Error = err.Error()
	})
	d.emit(EventTaskError, task)
}

func (d *Downloader) emit(kind EventKind, task *Task) {
	if d.onEvent != nil {
		d.onEvent(Event{Kind: kind, Task: task})
	}
}

func newTaskID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
