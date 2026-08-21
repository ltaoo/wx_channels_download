package hermes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// Task status values maintain stable mapping with the persistence layer's download_task status values.
const (
	TaskStatusWaiting     = 0
	TaskStatusPreparing   = 1
	TaskStatusDownloading = 2
	TaskStatusPaused      = 3
	TaskStatusMerging     = 4
	TaskStatusFinished    = 5
	TaskStatusFailed      = 6
	TaskStatusCancelled   = 7
)

const (
	ResourceTypeFile       = "FILE"
	ResourceTypeCollection = "COLLECTION"
	ResourceTypeStream     = "STREAM"
)

const (
	default_segment_count = 32
	minimum_segment_size  = int64(25 * 1024 * 1024)
	partial_file_suffix   = ".part"
	progress_interval     = 1 * time.Second
	progress_log_interval = 3 * time.Second
	max_read_attempts     = 3
	default_read_timeout  = 10 * time.Second
	read_buffer_size      = 256 * 1024
)

// Endpoint contains download source information needed by protocol drivers. Headers and Cookies
// are only passed to drivers and will not be logged or included in events.
type Endpoint struct {
	ID       int
	Protocol string
	URL      string
	Priority int
	Headers  map[string]string
	Cookies  string
	// ProxyServer is copied from the owning TaskJob before protocol requests are
	// made. Drivers that perform network I/O should honor it for this endpoint.
	ProxyServer ProxyServer
}

// TaskJob is the single task model used by Hermes from Store.LoadTask until
// post-processing finishes. Task-level input and runtime state belong here;
// resource-level state belongs to ResourceJob.
type TaskJob struct {
	ID          int
	Name        string
	UniqueID    string // Task-level platform unique identifier
	DownloadDir string
	Platform    string // PlatformId from DB, used by postprocessor for platform routing
	// ProxyServer applies to every network endpoint in this task. When its
	// address is empty, Config["proxy_server"] is used.
	ProxyServer ProxyServer
	Resources   []ResourceJob
	Config      map[string]any // Parsed download configuration for hooks
	Metadata    map[string]any // Parsed content metadata for postprocessors and hooks

	ctx           context.Context
	cancel        context.CancelCauseFunc
	done          chan struct{}
	execution_mu  sync.Mutex
	cancel_reason cancellation_reason
}

// ResourceJob is the single resource model used from endpoint selection through
// probing, transfer, filename resolution and post-processing. Runtime fields are
// intentionally kept beside the immutable resource input so later phases mutate
// the same object instead of constructing parallel DTOs.
type ResourceJob struct {
	ID          int
	DownloadDir string // Resource-specific output container; falls back to TaskJob.DownloadDir.
	Name        string
	Kind        string // "html", "image", "video", etc.
	Type        string // "FILE" | "STREAM"
	UniqueID    string // Platform-level unique identifier
	Endpoints   []Endpoint
	Extra       map[string]string // User-defined fields, irrelevant to download, passed through to hooks

	// Live-stream recording configuration. Timestamps accept Unix seconds or
	// Unix milliseconds; Duration is expressed in seconds. RotateMinutes is the
	// preferred recording chunk duration and RotateSize is reserved for recorder
	// implementations that support size-based rotation.
	StreamURL     string
	RecordStart   *int64
	RecordEnd     *int64
	Duration      int64
	RotateMinutes int
	RotateSize    int64

	// Runtime/output state populated by Hermes.
	Size       int64
	FilePath   string
	Downloaded int64
	Speed      int64
	StartTime  time.Time
	FinishTime time.Time
	Error      error
}

// SegmentRange is a protocol-agnostic finite byte range, inclusive on both ends.
type SegmentRange struct {
	Index       int
	OffsetStart int64
	OffsetEnd   int64
	Size        int64
}

// Segment is a resumable unified segment state in the Store.
type Segment struct {
	ID          int
	Index       int
	URL         string
	OffsetStart int64
	OffsetEnd   int64
	Size        int64
	Downloaded  int64
}

type segment_progress struct {
	slot       int
	downloaded int64
	speed      int64
	done       bool
	err        error
}

// PreparedResource is protocol-agnostic resource information obtained from driver probing.
type PreparedResource struct {
	Size          int64
	SupportsRange bool
	ContentType   string
	ProbeData     []byte // First 512 bytes returned by probe request, used for magic bytes detection
}

// ReadRequest describes the byte range the Writer wants to read from the protocol driver.
type ReadRequest struct {
	OffsetStart int64
	OffsetEnd   int64
	UseRange    bool
}

type endpoint_candidate struct {
	endpoint Endpoint
	protocol string
	driver   ProtocolDriver
}

// ProtocolDriver only handles connection, authentication, probing, and reading; file layout,
// concurrency, and state machine are managed by the downloader. New protocols can register
// drivers to integrate with the scheduler without writing target files directly in the driver.
type ProtocolDriver interface {
	Protocols() []string
	Prepare(ctx context.Context, endpoint Endpoint) (PreparedResource, error)
	Open(ctx context.Context, endpoint Endpoint, request ReadRequest) (io.ReadCloser, error)
}

// StreamRecordRequest describes one live-stream recording execution. Stream
// recorders write directly to OutputPath because live rotation and crash-safe
// chunk handling cannot be expressed as a resumable byte-range Reader.
type StreamRecordRequest struct {
	OutputPath    string
	StopAt        time.Time
	Duration      time.Duration
	RotateMinutes int
	RotateSize    int64
}

// StreamSegmentState is the durable progress state of one recorder chunk.
// The local path is intentionally not persisted by the generic Store; recorder
// implementations derive it deterministically from OutputPath and Index.
type StreamSegmentState struct {
	Index      int
	Size       int64
	Downloaded int64
	Complete   bool
}

// StreamRecordProgress reports byte/time progress for a live stream. Live
// resources have no known final size, so clients should display Duration and
// Downloaded rather than a percentage.
type StreamRecordProgress struct {
	Downloaded int64
	Speed      int64
	Duration   time.Duration
	Segments   []StreamSegmentState
	Finalizing bool
}

// StreamRecordResult is returned after chunks have been finalized into the
// requested output file.
type StreamRecordResult struct {
	FilePath string
	Size     int64
	Duration time.Duration
}

// StreamRecorder is an optional capability implemented by protocol drivers
// that record endless media sources. ProtocolDriver remains unchanged for
// finite resources and for backwards-compatible registration.
type StreamRecorder interface {
	RecordStream(
		ctx context.Context,
		endpoint Endpoint,
		request StreamRecordRequest,
		on_progress func(StreamRecordProgress) error,
	) (StreamRecordResult, error)
}

// Store isolates the download execution layer from the database.
type Store interface {
	LoadTask(task_id int) (*TaskJob, error)
	UpdateStatus(task_id int, status int) error
	ActivateTask(task_id int) error
	UpdateProgress(task_id int, downloaded int64, speed int64) error
	UpdateResourceSize(task_id int, size int64) error
	DeactivateConnections(task_id int) error
	FinishTask(task_id int) error
	RecordError(task_id int, err_msg string) error
	CreateSegments(resource_id int, url string, ranges []SegmentRange) ([]int, error)
	LoadSegmentInfo(resource_id int) ([]Segment, error)
	UpdateSegmentProgress(seg_id int, downloaded int64) error
}

// SegmentProgressUpdate carries one durable segment-progress update.
type SegmentProgressUpdate struct {
	SegmentID  int
	Downloaded int64
}

// ResourceStore provides per-resource progress updates for multi-resource tasks.
// When not implemented, HermesEngine falls back to Store's task-level update methods.
type ResourceStore interface {
	UpdateResourceProgress(resource_id int, downloaded int64, speed int64) error
	UpdateResourceSizeByID(resource_id int, size int64) error
	FinishResource(resource_id int) error
}

// ProgressBatchStore lets a persistent store update segment/resource progress
// in one transaction. Stores that do not implement it use the base Store and
// ResourceStore methods.
type ProgressBatchStore interface {
	UpdateResourceSegmentProgress(resource_id int, segment_id int, downloaded int64, speed int64) error
	UpdateAggregateResourceProgress(resource_id int, segments []SegmentProgressUpdate, downloaded int64, speed int64) error
}

// StreamSegmentStore optionally persists the recorder's time-based chunks.
// Stores that do not implement it still receive aggregate resource progress.
type StreamSegmentStore interface {
	SyncStreamSegments(resource_id int, url string, segments []StreamSegmentState) error
}

// StreamResultStore optionally persists media-specific recording metadata that
// is not part of the generic finite-resource Store contract.
type StreamResultStore interface {
	UpdateStreamDuration(resource_id int, duration_seconds int64) error
}

// OutputNameUpdate keeps persisted download metadata aligned with the output
// path chosen from a response Content-Type before the first file write.
type OutputNameUpdate struct {
	TaskID       int
	ResourceID   int
	ResourceName string
	TaskName     string
}

// OutputNameStore is implemented by stores that persist task/resource output
// names. It is optional so non-persistent HermesEngine users remain supported.
type OutputNameStore interface {
	UpdateOutputName(update OutputNameUpdate) error
}

// ResourceOutputUpdate is the final resource state after post-processing and
// filename finalization. Kind is the persisted MIME source of truth;
// Extension is derived from it at runtime.
type ResourceOutputUpdate struct {
	TaskID       int // Optional task guard; zero supports standalone resources.
	ResourceID   int
	DownloadDir  string
	ResourceName string
	ResourceKind string
	ResourceSize int64
}

// ResourceOutputStore persists the final post-processed resource metadata.
// It is optional so non-persistent HermesEngine users remain supported.
type ResourceOutputStore interface {
	UpdateResourceOutput(update ResourceOutputUpdate) error
}

// ResourceCleanupStore removes resources (and their associated endpoints,
// connections and segments) that were removed by post-processing.
// It is optional so non-persistent HermesEngine users remain supported.
type ResourceCleanupStore interface {
	DeleteStaleResources(task_id int, keep_resource_ids []int) error
}

// Postprocessor defines platform-specific post-download processing.
// Called by HermesEngine after all resources download, before .tmp renaming and DB updates.
type Postprocessor interface {
	Process(ctx context.Context, task_job *TaskJob) error
}

// HermesEngine is a protocol-agnostic finite resource download scheduler.
// FILE and COLLECTION are scheduled by the same task, STREAM is handled by the recording scheduler.
type HermesEngine struct {
	mu             sync.Mutex
	event_mu       sync.RWMutex
	sem            chan struct{}
	resource_sem   chan struct{}
	connection_sem chan struct{}
	jobs           map[int]*TaskJob
	store          Store
	logger         zerolog.Logger
	on_event       EventHandler
	event_history  []event_record
	replay_events  bool
	drivers        map[string]ProtocolDriver
	hooks          *HookManager
	postprocessor  Postprocessor
	progress_mu    sync.RWMutex
	progress_cache map[int]*progress_tracker // keyed by task ID
	cfg            HermesEngineConfig
}

// progressTracker holds in-memory progress for all resources of a task.
type progress_tracker struct {
	mu                   sync.Mutex
	resources            map[int]*resource_tracker
	order                []int // resource IDs in merge_order
	total_size           int64
	total_downloaded     int64
	total_speed          int64
	last_emit_downloaded int64     // last emitted total downloaded, used to skip duplicate broadcasts
	last_emit_speed      int64     // last emitted total speed
	last_emit_time       time.Time // last emission time, used for keepalive when segments are connecting
}

// resourceTracker holds the current download progress and metadata of a single resource.
// resourceTracker holds the current download progress and metadata of a single resource.
// speed stores the real-time speed reported by the download loop (copyReader/downloadSegment),
// which is updated every progressInterval. snapshotProgress uses this value directly
// for the WS push rather than re-computing from deltas, ensuring speed is never stale.
type resource_tracker struct {
	size       int64
	downloaded int64
	speed      int64
	name       string
	kind       string
	typ        string
}

type cancellation_reason uint8

const (
	cancel_none cancellation_reason = iota
	cancel_pause
	cancel_stop
	cancel_delete
)

// ErrTaskStopRequested is the cancellation cause used when a live recording
// should stop accepting new media but still finalize its recorded chunks.
var ErrTaskStopRequested = errors.New("live stream stop requested")

func (j *TaskJob) stop(reason cancellation_reason) {
	j.execution_mu.Lock()
	if j.cancel_reason != cancel_none {
		// Deletion remains authoritative if it races with pause/stop. The first
		// cancellation cause still controls how an in-flight recorder exits.
		if reason == cancel_delete {
			j.cancel_reason = cancel_delete
		}
		j.execution_mu.Unlock()
		return
	}
	j.cancel_reason = reason
	cancel := j.cancel
	j.execution_mu.Unlock()
	if cancel == nil {
		return
	}
	if reason == cancel_stop {
		cancel(ErrTaskStopRequested)
		return
	}
	cancel(context.Canceled)
}

func (j *TaskJob) cancellation_reason() cancellation_reason {
	j.execution_mu.Lock()
	defer j.execution_mu.Unlock()
	return j.cancel_reason
}

// HermesEngineConfig contains the download scheduler's runtime configuration.
type HermesEngineConfig struct {
	MaxConcurrent         int
	ResourceConcurrency   int // Max concurrent resources across all tasks, <=0 uses default 5
	ConnectionConcurrency int // Optional overall safety cap; <=0 derives ResourceConcurrency*SegmentConcurrency
	FilenameTemplate      string
	BasePath              string        // Absolute download root directory.
	ProgressEmitInterval  time.Duration // Progress event emission interval, <=0 uses default 180ms
	SpeedLimit            int64         // Per-segment download speed limit (bytes/sec), 0 means unlimited
	SegmentConcurrency    int           // Max concurrent segments per resource, <=0 uses default 5
	ReadTimeout           time.Duration // Timeout for a single Read() call, <=0 uses default 10s
}

func (cfg HermesEngineConfig) with_defaults() HermesEngineConfig {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.ResourceConcurrency <= 0 {
		cfg.ResourceConcurrency = 5
	}
	if cfg.ProgressEmitInterval <= 0 {
		cfg.ProgressEmitInterval = 180 * time.Millisecond
	}
	if cfg.SegmentConcurrency <= 0 {
		cfg.SegmentConcurrency = 5
	}
	if cfg.ConnectionConcurrency <= 0 {
		// Match aria2's concurrency layers: active items multiplied by the
		// per-item split count. The derived guard prevents accidental growth but
		// does not reduce throughput permitted by those two primary limits.
		cfg.ConnectionConcurrency = cfg.ResourceConcurrency * cfg.SegmentConcurrency
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = default_read_timeout
	}
	return cfg
}

// HermesNewConfig contains HermesEngine dependencies and runtime configuration.
type HermesNewConfig struct {
	Store  Store
	Logger *zerolog.Logger
	Config HermesEngineConfig
}

// New creates a new HermesEngine from the given configuration.
func New(opt HermesNewConfig) *HermesEngine {
	cfg := opt.Config.with_defaults()
	if strings.TrimSpace(cfg.BasePath) == "" {
		if working_directory, err := os.Getwd(); err == nil {
			cfg.BasePath = working_directory
		} else {
			cfg.BasePath = "."
		}
	}
	logger := zerolog.Nop()
	if opt.Logger != nil {
		logger = *opt.Logger
	}
	logger = logger.With().Str("component", "hermes").Logger()
	store := opt.Store
	replay_events := store == nil
	if store == nil {
		store = new_memory_store()
	}
	e := &HermesEngine{
		sem:            make(chan struct{}, cfg.MaxConcurrent),
		resource_sem:   make(chan struct{}, cfg.ResourceConcurrency),
		connection_sem: make(chan struct{}, cfg.ConnectionConcurrency),
		jobs:           make(map[int]*TaskJob),
		store:          store,
		logger:         logger,
		event_history:  make([]event_record, 0),
		replay_events:  replay_events,
		drivers:        make(map[string]ProtocolDriver),
		progress_cache: make(map[int]*progress_tracker),
		cfg:            cfg,
	}
	e.RegisterProtocol(new_default_http_driver())
	return e
}

// SetHooks sets the JS hook manager. Pass nil to disable hooks.
func (d *HermesEngine) SetHooks(h *HookManager) {
	d.hooks = h
}

// SetPostprocessor registers platform-specific post-processing.
func (d *HermesEngine) SetPostprocessor(p Postprocessor) {
	d.postprocessor = p
}

// RegisterProtocol registers or replaces a protocol driver. Protocol names are case-insensitive.
func (d *HermesEngine) RegisterProtocol(driver ProtocolDriver) {
	if driver == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, protocol := range driver.Protocols() {
		protocol = strings.ToLower(strings.TrimSpace(protocol))
		if protocol != "" {
			d.drivers[protocol] = driver
		}
	}
}

// StartTask submits an existing task for start, resume, or retry. Concurrent
// slot acquisition happens in the background, so queuing does not block API
// requests.
func (d *HermesEngine) StartTask(task_id int) error {
	return d.start_task(task_id, false)
}

// StartCreatedTask submits a newly persisted task and emits EventCreated with
// the identity needed by consumers to load its complete record.
func (d *HermesEngine) StartCreatedTask(task_id int) error {
	return d.start_task(task_id, true)
}

func (d *HermesEngine) start_task(task_id int, created bool) error {
	if task_id <= 0 {
		return errors.New("taskID must be greater than 0")
	}
	if d.store == nil {
		return errors.New("download task store is nil")
	}

	d.mu.Lock()
	if _, exists := d.jobs[task_id]; exists {
		d.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	task_job := &TaskJob{ID: task_id, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	d.jobs[task_id] = task_job
	d.mu.Unlock()

	if err := d.store.UpdateStatus(task_id, TaskStatusPreparing); err != nil {
		cancel(context.Canceled)
		d.mu.Lock()
		if d.jobs[task_id] == task_job {
			delete(d.jobs, task_id)
		}
		d.mu.Unlock()
		close(task_job.done)
		return fmt.Errorf("failed to update task status to preparing: %w", err)
	}
	if created {
		d.emit(EventCreated, TaskCreatedEventData{TaskID: task_id})
	} else {
		d.emit(EventPreparing, TaskPreparingEventData{TaskID: task_id})
	}
	go d.schedule(task_job)
	return nil
}

// HasAvailableSlot returns true when at least one concurrent download slot is free.
func (d *HermesEngine) HasAvailableSlot() bool {
	return len(d.sem) < cap(d.sem)
}

// RunningTaskCount returns the number of currently occupied concurrent slots.
func (d *HermesEngine) RunningTaskCount() int {
	return len(d.sem)
}

// MaxConcurrent returns the maximum number of concurrent download slots.
func (d *HermesEngine) MaxConcurrent() int {
	return cap(d.sem)
}

// PauseTask cancels and waits for the current execution instance to exit, ensuring subsequent Resume
// will not write files concurrently with the old Writer.
func (d *HermesEngine) PauseTask(task_id int) {
	if task_job := d.find_job(task_id); task_job != nil {
		task_job.stop(cancel_pause)
		<-task_job.done
	}
}

// StopTask terminates a live recording and waits until its persisted chunks
// have been merged and the regular task post-processing pipeline has finished.
// Unlike PauseTask, a stopped live task cannot be resumed.
func (d *HermesEngine) StopTask(task_id int) error {
	if task_id <= 0 {
		return errors.New("taskID must be greater than 0")
	}
	if d.store == nil {
		return errors.New("download task store is nil")
	}

	d.mu.Lock()
	task_job := d.jobs[task_id]
	created := task_job == nil
	if created {
		ctx, cancel := context.WithCancelCause(context.Background())
		task_job = &TaskJob{ID: task_id, ctx: ctx, cancel: cancel, done: make(chan struct{})}
		d.jobs[task_id] = task_job
	}
	d.mu.Unlock()

	task_job.stop(cancel_stop)
	if created {
		go d.schedule(task_job)
	}
	<-task_job.done
	return nil
}

// PauseAllTask pauses all in-progress or queued download tasks.
func (d *HermesEngine) PauseAllTask() {
	d.logger.Info().Msg("PauseAllTask")
	jobs := d.request_pause_all_tasks()
	for _, task_job := range jobs {
		<-task_job.done
	}
}

// RequestPauseAllTask requests all in-progress or queued download tasks to
// pause without waiting for every task goroutine to exit.
func (d *HermesEngine) RequestPauseAllTask() {
	d.logger.Info().Msg("RequestPauseAllTask")
	d.request_pause_all_tasks()
}

func (d *HermesEngine) request_pause_all_tasks() []*TaskJob {
	d.mu.Lock()
	jobs := make([]*TaskJob, 0, len(d.jobs))
	for _, task_job := range d.jobs {
		jobs = append(jobs, task_job)
	}
	d.mu.Unlock()
	for _, task_job := range jobs {
		task_job.stop(cancel_pause)
	}
	return jobs
}

// DeleteTask stops the execution instance and marks the task as cancelled.
// Soft deletion of database entities is still handled by the API handler.
func (d *HermesEngine) DeleteTask(task_id int) {
	d.logger.Info().Int("task_id", task_id).Msg("DeleteTask")
	if task_job := d.find_job(task_id); task_job != nil {
		task_job.stop(cancel_delete)
		<-task_job.done
		_ = d.store.UpdateStatus(task_id, TaskStatusCancelled)
		d.logger.Info().Int("task_id", task_id).Msg("task deleted")
		d.emit(EventDeleted, TaskDeletedEventData{TaskID: task_id})
		d.delete_tracker(task_id)
	}
}

func (d *HermesEngine) find_job(task_id int) *TaskJob {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.jobs[task_id]
}

func (d *HermesEngine) schedule(task_job *TaskJob) {
	task_id := task_job.ID
	acquired := false
	defer func() {
		if acquired {
			<-d.sem
		}
		d.mu.Lock()
		if d.jobs[task_id] == task_job {
			delete(d.jobs, task_id)
		}
		d.mu.Unlock()
		close(task_job.done)
	}()

	if task_job.cancellation_reason() == cancel_stop {
		// A stop-only job may be recovering durable chunks after a restart, so
		// it still needs to run even though its recording context is cancelled.
		d.sem <- struct{}{}
		acquired = true
	} else {
		select {
		case d.sem <- struct{}{}:
			acquired = true
		case <-task_job.ctx.Done():
			d.handle_cancellation(task_id, task_job)
			return
		}
	}

	var run_err error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				run_err = fmt.Errorf("download goroutine exited unexpectedly: %v", recovered)
				d.logger.Error().
					Int("task_id", task_id).
					Interface("panic", recovered).
					Str("stack", string(debug.Stack())).
					Msg("download goroutine panicked")
			}
		}()
		run_err = d.run(task_job)
	}()

	if task_job.cancellation_reason() != cancel_stop &&
		(errors.Is(run_err, context.Canceled) || task_job.ctx.Err() != nil) {
		d.handle_cancellation(task_id, task_job)
		return
	}
	if run_err != nil {
		d.fail_task(task_id, run_err.Error())
	}
}

func (d *HermesEngine) handle_cancellation(task_id int, task_job *TaskJob) {
	if task_job.cancellation_reason() == cancel_pause {
		d.pause_task(task_id)
	}
}

func (d *HermesEngine) run(info *TaskJob) error {
	task_id := info.ID
	ctx := info.ctx
	loaded, err := d.store.LoadTask(task_id)
	if err != nil {
		return fmt.Errorf("failed to load task information: %w", err)
	}
	if loaded == nil {
		return errors.New("failed to load task information: task is nil")
	}
	// Hydrate the scheduler-owned TaskJob while preserving its execution control
	// fields. The same object now travels from queueing through finalization.
	info.Name = loaded.Name
	info.UniqueID = loaded.UniqueID
	info.DownloadDir = loaded.DownloadDir
	info.Platform = loaded.Platform
	info.ProxyServer = loaded.ProxyServer
	info.Resources = loaded.Resources
	info.Config = loaded.Config
	info.Metadata = loaded.Metadata
	apply_task_proxy(info)
	if len(info.Resources) == 0 {
		return errors.New("task has no downloadable resources")
	}
	resources := info.Resources

	// Task start log: record basic task information
	d.logger.Info().
		Int("task_id", task_id).
		Str("task_name", info.Name).
		Str("task_unique_id", info.UniqueID).
		Str("download_dir", info.DownloadDir).
		Interface("config", task_config_for_log(info.Config)).
		Int("resource_count", len(resources)).
		Msg("run - after d.store.LoadTask")

	// Log detailed information for each resource
	for i, r := range resources {
		d.logger.Info().
			Int("task_id", task_id).
			Int("resource_id", r.ID).
			Int("resource_index", i+1).
			Str("resource_name", r.Name).
			Str("resource_unique_id", r.UniqueID).
			Str("resource_kind", r.Kind).
			Str("resource_type", r.Type).
			Int("endpoint_count", len(r.Endpoints)).
			Msg("run - after d.store.LoadTask")
	}

	if err := d.store.UpdateStatus(task_id, TaskStatusDownloading); err != nil {
		return fmt.Errorf("failed to update task status to downloading: %w", err)
	}
	if err := d.store.ActivateTask(task_id); err != nil {
		return fmt.Errorf("failed to activate task: %w", err)
	}
	d.emit(EventStarted, TaskStartedEventData{TaskID: task_id})

	if info.Config == nil {
		info.Config = make(map[string]any)
	}
	if info.Metadata == nil {
		info.Metadata = make(map[string]any)
	}
	// Preserve the existing template/hook behavior: metadata supplies defaults,
	// while explicit download configuration wins on key conflicts.
	for key, value := range info.Metadata {
		if _, exists := info.Config[key]; !exists {
			info.Config[key] = value
		}
	}

	// Pre-probe all resource sizes, record time overhead
	probe_start := time.Now()
	resource_sizes := d.ensure_resource_sizes(ctx, task_id, resources)
	probe_elapsed := time.Since(probe_start).Round(time.Millisecond)

	d.init_tracker(task_id, resource_sizes, resources)

	// Start periodic progress emission
	progress_ticker := time.NewTicker(d.cfg.ProgressEmitInterval)
	defer progress_ticker.Stop()
	progress_done := make(chan struct{})
	emit_done := make(chan struct{})
	defer func() {
		close(progress_done) // signal goroutine to exit on normal completion
		<-emit_done
	}()
	go func() {
		defer close(emit_done)
		for {
			select {
			case <-progress_ticker.C:
				d.emit_progress(task_id)
			case <-progress_done:
				d.emit_progress(task_id) // Send final progress one last time
				return
			case <-ctx.Done():
				d.emit_progress(task_id) // Send final progress one last time
				return
			}
		}
	}()

	// Summarize and log resource size probe results
	var total_task_size int64
	known_sizes := 0
	unknown_sizes := 0
	for _, r := range resources {
		if sz, ok := resource_sizes[r.ID]; ok && sz > 0 {
			known_sizes++
			total_task_size += sz
			d.logger.Info().
				Int("task_id", task_id).
				Int("resource_id", r.ID).
				Str("resource_name", r.Name).
				Int64("resource_size", sz).
				Str("resource_size_readable", format_size(sz)).
				Msg("run - resource size probed")
		} else {
			unknown_sizes++
		}
	}
	has_sizes := total_task_size > 0 && len(resource_sizes) > 1

	if total_task_size > 0 {
		d.logger.Info().
			Int("task_id", task_id).
			Int64("total_size", total_task_size).
			Str("total_size_readable", format_size(total_task_size)).
			Int("resource_count", len(resources)).
			Int("known_size_count", known_sizes).
			Int("unknown_size_count", unknown_sizes).
			Str("probe_elapsed", probe_elapsed.String()).
			Msg("run - task size summary")
	} else {
		d.logger.Info().
			Int("task_id", task_id).
			Int("resource_count", len(resources)).
			Int("unknown_size_count", unknown_sizes).
			Str("probe_elapsed", probe_elapsed.String()).
			Msg("run - task total size unknown, downloading resource by resource")
	}

	// Concurrently download all resources
	download_start := time.Now()
	var completed_size atomic.Int64

	type resource_result struct {
		file_path string
		err       error
		elapsed   time.Duration
	}
	results := make([]*resource_result, len(resources))

	download_ctx, cancel_downloads := context.WithCancel(ctx)
	defer cancel_downloads()

	var first_err error
	var err_once sync.Once

	d.process_resources(download_ctx, resources, d.cfg.ResourceConcurrency, func(idx int, resource *ResourceJob) {
		d.logger.Info().
			Int("task_id", task_id).
			Int("resource_id", resource.ID).
			Int("resource_index", idx+1).
			Int("total_resources", len(resources)).
			Str("resource_name", resource.Name).
			Str("resource_unique_id", resource.UniqueID).
			Msg("run - before download resource")

		resource_start := time.Now()
		resource.StartTime = resource_start
		file_path, err := d.download_resource(download_ctx, info, resource)
		resource.FinishTime = time.Now()
		resource.Error = err
		resource.Speed = 0
		if err == nil && resource.Size > 0 {
			resource.Downloaded = resource.Size
		}
		elapsed := time.Since(resource_start).Round(time.Millisecond)

		results[idx] = &resource_result{file_path: file_path, err: err, elapsed: elapsed}

		if err != nil {
			if info.cancellation_reason() == cancel_stop ||
				(!errors.Is(err, context.Canceled) && download_ctx.Err() == nil) {
				err_once.Do(func() {
					first_err = fmt.Errorf("failed to download resource %s: %w", resource.Name, err)
					cancel_downloads()
				})
			}
			d.logger.Error().
				Int("task_id", task_id).
				Int("resource_id", resource.ID).
				Int("resource_index", idx+1).
				Str("resource_name", resource.Name).
				Str("elapsed", elapsed.String()).
				Err(err).
				Msg("run - resource download failed")
			return
		}

		if size, ok := resource_sizes[resource.ID]; ok {
			completed_size.Add(size)
		}
		d.logger.Info().
			Int("task_id", task_id).
			Int("resource_id", resource.ID).
			Int("total_resources", len(resources)).
			Str("elapsed", elapsed.String()).
			Msg("run - resource downloaded")
	})

	// Pause/delete must not finalize. A live stop is different: the stream
	// recorder has already merged its chunks, so continue through postprocess.
	if err := context.Cause(ctx); err != nil && info.cancellation_reason() != cancel_stop {
		return err
	}

	if first_err != nil {
		return first_err
	}
	if info.cancellation_reason() == cancel_stop {
		if err := d.store.UpdateStatus(task_id, TaskStatusMerging); err != nil {
			return fmt.Errorf("failed to update stopped live task status to merging: %w", err)
		}
	}

	// Collect file paths in original order
	file_paths := make([]string, 0, len(resources))
	for _, r := range results {
		if r != nil {
			file_paths = append(file_paths, r.file_path)
		}
	}

	if has_sizes && total_task_size > 0 {
		downloaded := completed_size.Load()
		total_res := len(file_paths)
		pct := float64(downloaded) * 100 / float64(total_task_size)
		d.logger.Info().
			Int("task_id", task_id).
			Int64("downloaded_size", downloaded).
			Int64("total_size", total_task_size).
			Float64("progress_percent", pct).
			Int("completed_resources", total_res).
			Int("total_resources", len(resources)).
			Msg("run - overall concurrent download progress")
	}

	// All resources downloaded, log summary information
	download_elapsed := time.Since(download_start).Round(time.Millisecond)
	log_event := d.logger.Info().
		Int("task_id", task_id).
		Int("resource_count", len(resources)).
		Str("download_elapsed", download_elapsed.String())
	if total_task_size > 0 && download_elapsed.Seconds() > 0 {
		avg_speed := int64(float64(total_task_size) / download_elapsed.Seconds())
		log_event.
			Int64("total_size", total_task_size).
			Str("total_size_readable", format_size(total_task_size)).
			Int64("avg_speed_bytes", avg_speed).
			Str("avg_speed", format_speed(avg_speed))
	}
	log_event.Msg("run - all resources downloaded, starting task finalization")

	return d.finish_task(info)
}
