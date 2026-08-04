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

// Task status values maintain stable mapping with the persistence layer's download_task_v1 status values.
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

// Task contains the task information needed by the downloader (provided externally via LoadTask).
// ResourceID identifies the currently processed resource, carrying resource-level context during multi-resource downloads.
type Task struct {
	ID               int
	Name             string
	SavePath         string
	FilenameTemplate string
	ResourceID       int
	Platform         string // PlatformId from DB, used by postprocessor for platform routing
	Endpoints        []Endpoint
	Resources        []Resource
	Config           string // ConfigJSON from DB, download configuration for hooks
	Metadata         string // MetadataJSON from DB, content metadata for hooks
}

// Resource is an independently downloadable file resource within a Task.
type Resource struct {
	ID        int
	Name      string
	Kind      string // "html", "image", "video", etc.
	Type      string // "FILE" | "STREAM"
	UniqueID  string // Platform-level unique identifier, used as download filename
	Endpoints []Endpoint
	Extension string            // User-specified suffix, used when both Content-Type and magic bytes are unavailable (e.g., ".mp4")
	Extra     map[string]string // User-defined fields, irrelevant to download, passed through to hooks
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
	LoadTask(taskID int) (*Task, error)
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

// Postprocessor defines platform-specific post-download processing.
// Called by HermesEngine after all resources download, before .tmp renaming and DB updates.
type Postprocessor interface {
	Process(ctx context.Context, info *PostprocessInfo) error
}

// PostprocessInfo provides postprocessing context.
type PostprocessInfo struct {
	TaskID    int
	TaskName  string
	SavePath  string
	Config    map[string]any
	Metadata  map[string]any
	Resources []PostprocessResource
}

// PostprocessResource describes one downloaded resource for postprocessing.
type PostprocessResource struct {
	ID        int
	Name      string            // Current filename
	Kind      string            // "video", "image", "html", etc.
	Type      string            // "FILE", "STREAM"
	Extra     map[string]string // Platform metadata (decode_key, etc.)
	TargetExt string            // Correct final extension (e.g., ".mp4")
	FilePath  string            // Absolute path to downloaded file
}

// HermesEngine is a protocol-agnostic finite resource download scheduler.
// FILE and COLLECTION are scheduled by the same task, STREAM is handled by the recording scheduler.
type HermesEngine struct {
	mu            sync.Mutex
	eventMu       sync.RWMutex
	sem           chan struct{}
	jobs          map[int]*job
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

type job struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu     sync.Mutex
	reason cancellationReason
}

func (j *job) stop(reason cancellationReason) {
	j.mu.Lock()
	if reason > j.reason {
		j.reason = reason
	}
	j.mu.Unlock()
	j.cancel()
}

func (j *job) cancellationReason() cancellationReason {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.reason
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
		jobs:          make(map[int]*job),
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
	job := &job{ctx: ctx, cancel: cancel, done: make(chan struct{})}
	d.jobs[taskID] = job
	d.mu.Unlock()

	if err := d.store.UpdateStatus(taskID, TaskStatusPreparing); err != nil {
		cancel()
		d.mu.Lock()
		if d.jobs[taskID] == job {
			delete(d.jobs, taskID)
		}
		d.mu.Unlock()
		close(job.done)
		return fmt.Errorf("failed to update task status to preparing: %w", err)
	}
	d.emit(taskID, EventCreated)
	d.logger.Info().Int("taskID", taskID).Msg("task started")
	go d.schedule(taskID, job)
	return nil
}

// PauseTask cancels and waits for the current execution instance to exit, ensuring subsequent Resume
// will not write files concurrently with the old Writer.
func (d *HermesEngine) PauseTask(taskID int) {
	if job := d.findJob(taskID); job != nil {
		job.stop(cancelPause)
		<-job.done
	}
}

// PauseAllTask pauses all in-progress or queued download tasks.
func (d *HermesEngine) PauseAllTask() {
	d.logger.Info().Msg("PauseAllTask")
	d.mu.Lock()
	jobs := make([]*job, 0, len(d.jobs))
	for _, job := range d.jobs {
		jobs = append(jobs, job)
	}
	d.mu.Unlock()
	for _, job := range jobs {
		job.stop(cancelPause)
	}
	for _, job := range jobs {
		<-job.done
	}
}

// DeleteTask stops the execution instance and marks the task as cancelled.
// Soft deletion of database entities is still handled by the API handler.
func (d *HermesEngine) DeleteTask(taskID int) {
	d.logger.Info().Int("taskID", taskID).Msg("DeleteTask")
	if job := d.findJob(taskID); job != nil {
		job.stop(cancelDelete)
		<-job.done
		_ = d.store.UpdateStatus(taskID, TaskStatusCancelled)
		d.logger.Info().Int("taskID", taskID).Msg("task deleted")
		d.emit(taskID, EventDeleted)
		d.deleteTracker(taskID)
	}
}

func (d *HermesEngine) findJob(taskID int) *job {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.jobs[taskID]
}

func (d *HermesEngine) schedule(taskID int, job *job) {
	d.logger.Info().
		Int("taskID", taskID).
		Msg("schedule")
	acquired := false
	defer func() {
		if acquired {
			<-d.sem
		}
		d.mu.Lock()
		if d.jobs[taskID] == job {
			delete(d.jobs, taskID)
		}
		d.mu.Unlock()
		close(job.done)
	}()

	select {
	case d.sem <- struct{}{}:
		acquired = true
	case <-job.ctx.Done():
		d.handleCancellation(taskID, job)
		return
	}

	var runErr error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				runErr = fmt.Errorf("download goroutine exited unexpectedly: %v", recovered)
				d.logger.Error().
					Int("taskID", taskID).
					Interface("panic", recovered).
					Str("stack", string(debug.Stack())).
					Msg("download goroutine panicked")
			}
		}()
		d.logger.Info().
			Int("taskID", taskID).
			Msg("before d.run")
		runErr = d.run(taskID, job.ctx)
	}()

	if errors.Is(runErr, context.Canceled) || job.ctx.Err() != nil {
		d.handleCancellation(taskID, job)
		return
	}
	if runErr != nil {
		d.failTask(taskID, runErr.Error())
	}
}

func (d *HermesEngine) handleCancellation(taskID int, job *job) {
	if job.cancellationReason() == cancelPause {
		d.pauseTask(taskID)
	}
}

func (d *HermesEngine) run(taskID int, ctx context.Context) error {
	info, err := d.store.LoadTask(taskID)
	if err != nil {
		return fmt.Errorf("failed to load task information: %w", err)
	}
	if info == nil {
		return errors.New("failed to load task information: task is nil")
	}
	if len(info.Resources) == 0 {
		return errors.New("task has no downloadable resources")
	}
	resources := info.Resources

	// Task start log: record basic task information
	d.logger.Info().
		Int("taskID", taskID).
		Str("taskName", info.Name).
		Str("savePath", info.SavePath).
		Int("resourceCount", len(resources)).
		Msg("run - the task info")

	// Log detailed information for each resource
	for i, r := range resources {
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceID", r.ID).
			Int("resourceIndex", i+1).
			Str("resourceName", r.Name).
			Str("resourceKind", r.Kind).
			Str("resourceType", r.Type).
			Int("endpointCount", len(r.Endpoints)).
			Msg("run - the resource info")
	}

	if err := d.store.UpdateStatus(taskID, TaskStatusDownloading); err != nil {
		return fmt.Errorf("failed to update task status to downloading: %w", err)
	}
	if err := d.store.ActivateTask(taskID); err != nil {
		return fmt.Errorf("failed to activate task: %w", err)
	}
	d.emit(taskID, EventStarted)

	config, _ := parseConfigAndMetadata(info.Config, info.Metadata)

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
				Int("taskID", taskID).
				Int("resourceID", r.ID).
				Str("resourceName", r.Name).
				Int64("resourceSize", sz).
				Str("resourceSizeReadable", formatSize(sz)).
				Msg("run - resource size probed")
		} else {
			unknownSizes++
		}
	}
	hasSizes := totalTaskSize > 0 && len(resourceSizes) > 1

	if totalTaskSize > 0 {
		d.logger.Info().
			Int("taskID", taskID).
			Int64("totalSize", totalTaskSize).
			Str("totalSizeReadable", formatSize(totalTaskSize)).
			Int("resourceCount", len(resources)).
			Int("knownSizeCount", knownSizes).
			Int("unknownSizeCount", unknownSizes).
			Str("probeElapsed", probeElapsed.String()).
			Msg("run - task size summary")
	} else {
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceCount", len(resources)).
			Int("unknownSizeCount", unknownSizes).
			Str("probeElapsed", probeElapsed.String()).
			Msg("run - task total size unknown, downloading resource by resource")
	}

	// Concurrently download all resources
	downloadStart := time.Now()
	var extensionsMu sync.Mutex
	resourceExtensions := make(map[int]string)
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

	for i, resource := range resources {
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceID", resource.ID).
			Int("resourceIndex", i+1).
			Int("totalResources", len(resources)).
			Str("resourceName", resource.Name).
			Msg("downloading resource")

		wg.Add(1)
		go func(idx int, res Resource) {
			defer wg.Done()
			resStart := time.Now()
			localExts := make(map[int]string)
			filePath, err := d.downloadResource(downloadCtx, taskID, info.SavePath, res.Type, res, config, localExts)
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
					Int("taskID", taskID).
					Int("resourceID", res.ID).
					Int("resourceIndex", idx+1).
					Str("resourceName", res.Name).
					Str("elapsed", elapsed.String()).
					Err(err).
					Msg("resource download failed")
				return
			}

			if sz, ok := resourceSizes[res.ID]; ok {
				completedSize.Add(sz)
			}
			extensionsMu.Lock()
			for k, v := range localExts {
				resourceExtensions[k] = v
			}
			extensionsMu.Unlock()

			d.logger.Info().
				Int("taskID", taskID).
				Int("resourceID", res.ID).
				Int("totalResources", len(resources)).
				Str("elapsed", elapsed.String()).
				Msg("resource downloaded")
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
			Int("taskID", taskID).
			Int64("downloadedSize", downloaded).
			Int64("totalSize", totalTaskSize).
			Float64("progressPercent", pct).
			Int("completedResources", totalRes).
			Int("totalResources", len(resources)).
			Msg("overall concurrent download progress")
	}

	// All resources downloaded, log summary information
	downloadElapsed := time.Since(downloadStart).Round(time.Millisecond)
	logEvent := d.logger.Info().
		Int("taskID", taskID).
		Int("resourceCount", len(resources)).
		Str("downloadElapsed", downloadElapsed.String())
	if totalTaskSize > 0 && downloadElapsed.Seconds() > 0 {
		avgSpeed := int64(float64(totalTaskSize) / downloadElapsed.Seconds())
		logEvent.
			Int64("totalSize", totalTaskSize).
			Str("totalSizeReadable", formatSize(totalTaskSize)).
			Int64("avgSpeedBytes", avgSpeed).
			Str("avgSpeed", formatSpeed(avgSpeed))
	}
	logEvent.Msg("all resources downloaded, starting task finalization")

	return d.finishTask(taskID, strings.Join(filePaths, ", "), resourceExtensions)
}
