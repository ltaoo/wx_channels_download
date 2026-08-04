package hermes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// EventType represents a downloader event type.
type EventType string

const (
	EventCreated  EventType = "created"
	EventStarted  EventType = "started"
	EventProgress EventType = "progress"
	EventPaused   EventType = "paused"
	EventFinished EventType = "finished"
	EventFailed   EventType = "failed"
	EventDeleted  EventType = "deleted"
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
	defaultSegmentCount = 32
	minimumSegmentSize  = int64(25 * 1024 * 1024)
	partialFileSuffix   = ".part"
	progressInterval    = 500 * time.Millisecond
	progressLogInterval = 3 * time.Second
	maxReadAttempts     = 3
	defaultReadTimeout  = 10 * time.Second
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
}

// TaskJob is the single task model used by Hermes from Store.LoadTask until
// post-processing finishes. Task-level input and runtime state belong here;
// resource-level state belongs to ResourceJob.
type TaskJob struct {
	ID               int
	Name             string
	UniqueID         string // Task-level platform unique identifier
	SavePath         string
	FilenameTemplate string
	Platform         string // PlatformId from DB, used by postprocessor for platform routing
	Resources        []ResourceJob
	Config           map[string]any // Parsed download configuration for hooks
	Metadata         map[string]any // Parsed content metadata for postprocessors and hooks

	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	executionMu  sync.Mutex
	cancelReason cancellationReason
}

// ResourceJob is the single resource model used from endpoint selection through
// probing, transfer, filename resolution and post-processing. Runtime fields are
// intentionally kept beside the immutable resource input so later phases mutate
// the same object instead of constructing parallel DTOs.
type ResourceJob struct {
	ID        int
	Name      string
	Kind      string // "html", "image", "video", etc.
	Type      string // "FILE" | "STREAM"
	UniqueID  string // Platform-level unique identifier
	Endpoints []Endpoint
	Extra     map[string]string // User-defined fields, irrelevant to download, passed through to hooks

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

// EventHandler receives task lifecycle and progress events.
// A nil progress indicates a non-progress event (e.g., started, finished, failed).
type EventHandler func(taskID int, event EventType, progress *TaskProgress)

// TaskProgress carries the current aggregate download progress, computed
// entirely from in-memory state without database queries.
type TaskProgress struct {
	TotalSize     int64              `json:"total_size"`
	Downloaded    int64              `json:"downloaded"`
	Speed         int64              `json:"speed"`
	ResourceCount int                `json:"resource_count"`
	Resources     []ResourceProgress `json:"resources"`
	Keepalive     bool               `json:"-"` // true when emitted as keepalive (no real progress change)
}

// ResourceProgress carries a single resource's download progress.
type ResourceProgress struct {
	ID         int    `json:"id"`
	Name       string `json:"name,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Type       string `json:"resource_type,omitempty"`
	Size       int64  `json:"size"`
	Downloaded int64  `json:"downloaded"`
	Speed      int64  `json:"speed"`
}

type segmentProgress struct {
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

type endpointCandidate struct {
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

// Store isolates the download execution layer from the database.
type Store interface {
	LoadTask(taskID int) (*TaskJob, error)
	UpdateStatus(taskID int, status int) error
	ActivateTask(taskID int) error
	UpdateProgress(taskID int, downloaded int64, speed int64) error
	UpdateResourceSize(taskID int, size int64) error
	DeactivateConnections(taskID int) error
	FinishTask(taskID int) error
	RecordError(taskID int, errMsg string) error
	CreateSegments(resourceID int, url string, ranges []SegmentRange) ([]int, error)
	LoadSegmentInfo(resourceID int) ([]Segment, error)
	UpdateSegmentProgress(segID int, downloaded int64) error
}

// ResourceStore provides per-resource progress updates for multi-resource tasks.
// When not implemented, HermesEngine falls back to Store's task-level update methods.
type ResourceStore interface {
	UpdateResourceProgress(resourceID int, downloaded int64, speed int64) error
	UpdateResourceSizeByID(resourceID int, size int64) error
	FinishResource(resourceID int) error
}

// OutputNameUpdate keeps persisted download metadata aligned with the output
// path chosen from a response Content-Type before the first file write.
type OutputNameUpdate struct {
	TaskID       int
	ResourceID   int
	ResourceName string
	TaskName     string
	SavePath     string
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
	TaskID       int
	ResourceID   int
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
	DeleteStaleResources(taskID int, keepResourceIDs []int) error
}

// Postprocessor defines platform-specific post-download processing.
// Called by HermesEngine after all resources download, before .tmp renaming and DB updates.
type Postprocessor interface {
	Process(ctx context.Context, taskJob *TaskJob) error
}

// HermesEngine is a protocol-agnostic finite resource download scheduler.
// FILE and COLLECTION are scheduled by the same task, STREAM is handled by the recording scheduler.
type HermesEngine struct {
	mu            sync.Mutex
	eventMu       sync.RWMutex
	sem           chan struct{}
	jobs          map[int]*TaskJob
	store         Store
	logger        zerolog.Logger
	onEvent       EventHandler
	drivers       map[string]ProtocolDriver
	hooks         *HookManager
	postprocessor Postprocessor
	progressMu    sync.Mutex
	progressCache map[int]*progressTracker // keyed by task ID
	cfg           HermesEngineConfig
}

// progressTracker holds in-memory progress for all resources of a task.
type progressTracker struct {
	mu                 sync.Mutex
	resources          map[int]*resourceTracker
	order              []int     // resource IDs in merge_order
	lastEmitDownloaded int64     // last emitted total downloaded, used to skip duplicate broadcasts
	lastEmitSpeed      int64     // last emitted total speed
	lastEmitTime       time.Time // last emission time, used for keepalive when segments are connecting
}

// resourceTracker holds the current download progress and metadata of a single resource.
// resourceTracker holds the current download progress and metadata of a single resource.
// speed stores the real-time speed reported by the download loop (copyReader/downloadSegment),
// which is updated every progressInterval (500ms). snapshotProgress uses this value directly
// for the WS push rather than re-computing from deltas, ensuring speed is never stale.
type resourceTracker struct {
	size       int64
	downloaded int64
	speed      int64
	name       string
	kind       string
	typ        string
}

type cancellationReason uint8

const (
	cancelNone cancellationReason = iota
	cancelPause
	cancelDelete
)

func (j *TaskJob) stop(reason cancellationReason) {
	j.executionMu.Lock()
	if reason > j.cancelReason {
		j.cancelReason = reason
	}
	j.executionMu.Unlock()
	j.cancel()
}

func (j *TaskJob) cancellationReason() cancellationReason {
	j.executionMu.Lock()
	defer j.executionMu.Unlock()
	return j.cancelReason
}

// HermesEngineConfig contains the download scheduler's runtime configuration.
type HermesEngineConfig struct {
	MaxConcurrent        int
	FilenameTemplate     string
	BasePath             string        // Absolute download root directory.
	ProgressEmitInterval time.Duration // Progress event emission interval, <=0 uses default 180ms
	SpeedLimit           int64         // Per-segment download speed limit (bytes/sec), 0 means unlimited
	SegmentConcurrency   int           // Max concurrent segments per resource, <=0 uses default 5
	ReadTimeout          time.Duration // Timeout for a single Read() call, <=0 uses default 10s
}

func (cfg HermesEngineConfig) withDefaults() HermesEngineConfig {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 3
	}
	if cfg.ProgressEmitInterval <= 0 {
		cfg.ProgressEmitInterval = 180 * time.Millisecond
	}
	if cfg.SegmentConcurrency <= 0 {
		cfg.SegmentConcurrency = 5
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = defaultReadTimeout
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
	cfg := opt.Config.withDefaults()
	logger := zerolog.Nop()
	if opt.Logger != nil {
		logger = *opt.Logger
	}
	logger = logger.With().Str("component", "hermes").Logger()
	e := &HermesEngine{
		sem:           make(chan struct{}, cfg.MaxConcurrent),
		jobs:          make(map[int]*TaskJob),
		store:         opt.Store,
		logger:        logger,
		drivers:       make(map[string]ProtocolDriver),
		progressCache: make(map[int]*progressTracker),
		cfg:           cfg,
	}
	return e
}

// SetEventHandler sets the task lifecycle and progress event handler. Pass nil to disable event callbacks.
func (d *HermesEngine) SetEventHandler(handler EventHandler) {
	d.eventMu.Lock()
	defer d.eventMu.Unlock()
	d.onEvent = handler
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

// StartTask submits a task to the scheduler. Concurrent slot acquisition happens in the background,
// so queuing does not block API requests.
func (d *HermesEngine) StartTask(taskID int) error {
	if taskID <= 0 {
		return errors.New("taskID must be greater than 0")
	}
	if d.store == nil {
		return errors.New("download task store is nil")
	}

	d.mu.Lock()
	if _, exists := d.jobs[taskID]; exists {
		d.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	taskJob := &TaskJob{ID: taskID, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	d.jobs[taskID] = taskJob
	d.mu.Unlock()

	if err := d.store.UpdateStatus(taskID, TaskStatusPreparing); err != nil {
		cancel()
		d.mu.Lock()
		if d.jobs[taskID] == taskJob {
			delete(d.jobs, taskID)
		}
		d.mu.Unlock()
		close(taskJob.done)
		return fmt.Errorf("failed to update task status to preparing: %w", err)
	}
	d.emit(taskID, EventCreated)
	go d.schedule(taskJob)
	return nil
}

// PauseTask cancels and waits for the current execution instance to exit, ensuring subsequent Resume
// will not write files concurrently with the old Writer.
func (d *HermesEngine) PauseTask(taskID int) {
	if taskJob := d.findJob(taskID); taskJob != nil {
		taskJob.stop(cancelPause)
		<-taskJob.done
	}
}

// PauseAllTask pauses all in-progress or queued download tasks.
func (d *HermesEngine) PauseAllTask() {
	d.logger.Info().Msg("PauseAllTask")
	d.mu.Lock()
	jobs := make([]*TaskJob, 0, len(d.jobs))
	for _, taskJob := range d.jobs {
		jobs = append(jobs, taskJob)
	}
	d.mu.Unlock()
	for _, taskJob := range jobs {
		taskJob.stop(cancelPause)
	}
	for _, taskJob := range jobs {
		<-taskJob.done
	}
}

// DeleteTask stops the execution instance and marks the task as cancelled.
// Soft deletion of database entities is still handled by the API handler.
func (d *HermesEngine) DeleteTask(taskID int) {
	d.logger.Info().Int("task_id", taskID).Msg("DeleteTask")
	if taskJob := d.findJob(taskID); taskJob != nil {
		taskJob.stop(cancelDelete)
		<-taskJob.done
		_ = d.store.UpdateStatus(taskID, TaskStatusCancelled)
		d.logger.Info().Int("task_id", taskID).Msg("task deleted")
		d.emit(taskID, EventDeleted)
		d.deleteTracker(taskID)
	}
}

func (d *HermesEngine) findJob(taskID int) *TaskJob {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.jobs[taskID]
}

func (d *HermesEngine) schedule(taskJob *TaskJob) {
	taskID := taskJob.ID
	acquired := false
	defer func() {
		if acquired {
			<-d.sem
		}
		d.mu.Lock()
		if d.jobs[taskID] == taskJob {
			delete(d.jobs, taskID)
		}
		d.mu.Unlock()
		close(taskJob.done)
	}()

	select {
	case d.sem <- struct{}{}:
		acquired = true
	case <-taskJob.ctx.Done():
		d.handleCancellation(taskID, taskJob)
		return
	}

	var runErr error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				runErr = fmt.Errorf("download goroutine exited unexpectedly: %v", recovered)
				d.logger.Error().
					Int("task_id", taskID).
					Interface("panic", recovered).
					Str("stack", string(debug.Stack())).
					Msg("download goroutine panicked")
			}
		}()
		runErr = d.run(taskJob)
	}()

	if errors.Is(runErr, context.Canceled) || taskJob.ctx.Err() != nil {
		d.handleCancellation(taskID, taskJob)
		return
	}
	if runErr != nil {
		d.failTask(taskID, runErr.Error())
	}
}

func (d *HermesEngine) handleCancellation(taskID int, taskJob *TaskJob) {
	if taskJob.cancellationReason() == cancelPause {
		d.pauseTask(taskID)
	}
}

func (d *HermesEngine) run(info *TaskJob) error {
	taskID := info.ID
	ctx := info.ctx
	loaded, err := d.store.LoadTask(taskID)
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
	info.SavePath = loaded.SavePath
	info.FilenameTemplate = loaded.FilenameTemplate
	info.Platform = loaded.Platform
	info.Resources = loaded.Resources
	info.Config = loaded.Config
	info.Metadata = loaded.Metadata
	if len(info.Resources) == 0 {
		return errors.New("task has no downloadable resources")
	}
	resources := info.Resources

	// Task start log: record basic task information
	d.logger.Info().
		Int("task_id", taskID).
		Str("task_name", info.Name).
		Str("task_unique_id", info.UniqueID).
		Str("save_path", info.SavePath).
		Interface("config", info.Config).
		Int("resource_count", len(resources)).
		Msg("run - after d.store.LoadTask")

	// Log detailed information for each resource
	for i, r := range resources {
		d.logger.Info().
			Int("task_id", taskID).
			Int("resource_id", r.ID).
			Int("resource_index", i+1).
			Str("resource_name", r.Name).
			Str("resource_unique_id", r.UniqueID).
			Str("resource_kind", r.Kind).
			Str("resource_type", r.Type).
			Int("endpoint_count", len(r.Endpoints)).
			Msg("run - after d.store.LoadTask")
	}

	if err := d.store.UpdateStatus(taskID, TaskStatusDownloading); err != nil {
		return fmt.Errorf("failed to update task status to downloading: %w", err)
	}
	if err := d.store.ActivateTask(taskID); err != nil {
		return fmt.Errorf("failed to activate task: %w", err)
	}
	d.emit(taskID, EventStarted)

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
	probeStart := time.Now()
	resourceSizes := d.ensureResourceSizes(ctx, taskID, resources)
	probeElapsed := time.Since(probeStart).Round(time.Millisecond)

	d.initTracker(taskID, resourceSizes, resources)

	// Start periodic progress emission
	progressTicker := time.NewTicker(d.cfg.ProgressEmitInterval)
	defer progressTicker.Stop()
	progressDone := make(chan struct{})
	emitDone := make(chan struct{})
	defer func() {
		close(progressDone) // signal goroutine to exit on normal completion
		<-emitDone
	}()
	go func() {
		defer close(emitDone)
		for {
			select {
			case <-progressTicker.C:
				d.emitProgress(taskID)
			case <-progressDone:
				d.emitProgress(taskID) // Send final progress one last time
				return
			case <-ctx.Done():
				d.emitProgress(taskID) // Send final progress one last time
				return
			}
		}
	}()

	// Summarize and log resource size probe results
	var totalTaskSize int64
	knownSizes := 0
	unknownSizes := 0
	for _, r := range resources {
		if sz, ok := resourceSizes[r.ID]; ok && sz > 0 {
			knownSizes++
			totalTaskSize += sz
			d.logger.Info().
				Int("task_id", taskID).
				Int("resource_id", r.ID).
				Str("resource_name", r.Name).
				Int64("resource_size", sz).
				Str("resource_size_readable", formatSize(sz)).
				Msg("run - resource size probed")
		} else {
			unknownSizes++
		}
	}
	hasSizes := totalTaskSize > 0 && len(resourceSizes) > 1

	if totalTaskSize > 0 {
		d.logger.Info().
			Int("task_id", taskID).
			Int64("total_size", totalTaskSize).
			Str("total_size_readable", formatSize(totalTaskSize)).
			Int("resource_count", len(resources)).
			Int("known_size_count", knownSizes).
			Int("unknown_size_count", unknownSizes).
			Str("probe_elapsed", probeElapsed.String()).
			Msg("run - task size summary")
	} else {
		d.logger.Info().
			Int("task_id", taskID).
			Int("resource_count", len(resources)).
			Int("unknown_size_count", unknownSizes).
			Str("probe_elapsed", probeElapsed.String()).
			Msg("run - task total size unknown, downloading resource by resource")
	}

	// Concurrently download all resources
	downloadStart := time.Now()
	var completedSize atomic.Int64

	type resourceResult struct {
		filePath string
		err      error
		elapsed  time.Duration
	}
	results := make([]*resourceResult, len(resources))

	downloadCtx, cancelDownloads := context.WithCancel(ctx)
	defer cancelDownloads()

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for i := range resources {
		resource := &resources[i]
		d.logger.Info().
			Int("task_id", taskID).
			Int("resource_id", resource.ID).
			Int("resource_index", i+1).
			Int("total_resources", len(resources)).
			Str("resource_name", resource.Name).
			Str("resource_unique_id", resource.UniqueID).
			Msg("run - before download resource")

		wg.Add(1)
		go func(idx int, res *ResourceJob) {
			defer wg.Done()
			resStart := time.Now()
			res.StartTime = resStart
			filePath, err := d.downloadResource(downloadCtx, info, res)
			res.FinishTime = time.Now()
			res.Error = err
			res.Speed = 0
			if err == nil && res.Size > 0 {
				res.Downloaded = res.Size
			}
			elapsed := time.Since(resStart).Round(time.Millisecond)

			results[idx] = &resourceResult{filePath: filePath, err: err, elapsed: elapsed}

			if err != nil {
				if !errors.Is(err, context.Canceled) && downloadCtx.Err() == nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("failed to download resource %s: %w", res.Name, err)
						cancelDownloads()
					})
				}
				d.logger.Error().
					Int("task_id", taskID).
					Int("resource_id", res.ID).
					Int("resource_index", idx+1).
					Str("resource_name", res.Name).
					Str("elapsed", elapsed.String()).
					Err(err).
					Msg("run - resource download failed")
				return
			}

			if sz, ok := resourceSizes[res.ID]; ok {
				completedSize.Add(sz)
			}
			d.logger.Info().
				Int("task_id", taskID).
				Int("resource_id", res.ID).
				Int("total_resources", len(resources)).
				Str("elapsed", elapsed.String()).
				Msg("run - resource downloaded")
		}(i, resource)
	}
	wg.Wait()

	// If parent context has been cancelled (pause/delete), return the cancellation cause directly.
	// Do not proceed with finishTask to avoid a Finished -> Paused state transition race.
	if err := context.Cause(ctx); err != nil {
		return err
	}

	if firstErr != nil {
		return firstErr
	}

	// Collect file paths in original order
	filePaths := make([]string, 0, len(resources))
	for _, r := range results {
		if r != nil {
			filePaths = append(filePaths, r.filePath)
		}
	}

	if hasSizes && totalTaskSize > 0 {
		downloaded := completedSize.Load()
		totalRes := len(filePaths)
		pct := float64(downloaded) * 100 / float64(totalTaskSize)
		d.logger.Info().
			Int("task_id", taskID).
			Int64("downloaded_size", downloaded).
			Int64("total_size", totalTaskSize).
			Float64("progress_percent", pct).
			Int("completed_resources", totalRes).
			Int("total_resources", len(resources)).
			Msg("run - overall concurrent download progress")
	}

	// All resources downloaded, log summary information
	downloadElapsed := time.Since(downloadStart).Round(time.Millisecond)
	logEvent := d.logger.Info().
		Int("task_id", taskID).
		Int("resource_count", len(resources)).
		Str("download_elapsed", downloadElapsed.String())
	if totalTaskSize > 0 && downloadElapsed.Seconds() > 0 {
		avgSpeed := int64(float64(totalTaskSize) / downloadElapsed.Seconds())
		logEvent.
			Int64("total_size", totalTaskSize).
			Str("total_size_readable", formatSize(totalTaskSize)).
			Int64("avg_speed_bytes", avgSpeed).
			Str("avg_speed", formatSpeed(avgSpeed))
	}
	logEvent.Msg("run - all resources downloaded, starting task finalization")

	return d.finishTask(info)
}
