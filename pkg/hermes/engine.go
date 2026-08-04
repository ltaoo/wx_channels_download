package hermes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
	"github.com/rs/zerolog"

	"wx_channel/pkg/util"
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
	Platform         string    // PlatformId from DB, used by postprocessor for platform routing
	Endpoints        []Endpoint
	Resources        []Resource
	Config           string // ConfigJSON from DB, download configuration for hooks
	Metadata         string // MetadataJSON from DB, content metadata for hooks
}

// Resource is an independently downloadable file resource within a Task.
type Resource struct {
	ID           int
	Name         string
	Kind         string // "html", "image", "video", etc.
	ResourceType string // "FILE" | "STREAM"
	UniqueID     string // Platform-level unique identifier, used as download filename
	Endpoints    []Endpoint
	Extra        map[string]string // User-defined fields, irrelevant to download, passed through to hooks
	Extension    string            // User-specified suffix, used when both Content-Type and magic bytes are unavailable (e.g., ".mp4")
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
	TotalSize      int64              `json:"total_size"`
	Downloaded     int64              `json:"downloaded"`
	Speed          int64              `json:"speed"`
	ResourceCount  int                `json:"resource_count"`
	Resources      []ResourceProgress `json:"resources"`
	Keepalive      bool               `json:"-"` // true when emitted as keepalive (no real progress change)
}

// ResourceProgress carries a single resource's download progress.
type ResourceProgress struct {
	ID           int    `json:"id"`
	Name         string `json:"name,omitempty"`
	Kind         string `json:"kind,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	Size         int64  `json:"size"`
	Downloaded   int64  `json:"downloaded"`
	Speed        int64  `json:"speed"`
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
// When not implemented, Engine falls back to Store's task-level update methods.
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
// names. It is optional so non-persistent Engine users remain supported.
type OutputNameStore interface {
	UpdateOutputName(update OutputNameUpdate) error
}

// Postprocessor defines platform-specific post-download processing.
// Called by Engine after all resources download, before .tmp renaming and DB updates.
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
	ID           int
	Name         string            // Current filename
	Kind         string            // "video", "image", "html", etc.
	ResourceType string            // "FILE", "STREAM"
	Extra        map[string]string // Platform metadata (decode_key, etc.)
	TargetExt    string            // Correct final extension (e.g., ".mp4")
	FilePath     string            // Absolute path to downloaded file
}

// Engine is a protocol-agnostic finite resource download scheduler.
// FILE and COLLECTION are scheduled by the same task, STREAM is handled by the recording scheduler.
type Engine struct {
	mu                  sync.Mutex
	eventMu             sync.RWMutex
	maxConcurrent       int
	sem                 chan struct{}
	jobs                map[int]*job
	store               Store
	logger              zerolog.Logger
	onEvent             EventHandler
	drivers             map[string]ProtocolDriver
	filenameTemplate    string
	hooks               *HookManager
	basePath            string // absolute download root directory
	postprocessor       Postprocessor
	progressMu          sync.Mutex
	progressCache       map[int]*progressTracker // keyed by task ID
	progressEmitInterval time.Duration           // interval for periodic progress emission
	speedLimit           int64                    // per-segment download speed limit (bytes/sec), 0 means no limit
	segmentConcurrency   int                      // max concurrent segments per resource, 0 means unlimited (legacy)
	readTimeout          time.Duration            // max wait for a single Read() call before retrying
}

// progressTracker holds in-memory progress for all resources of a task.
type progressTracker struct {
	mu                   sync.Mutex
	resources            map[int]*resourceTracker
	order                []int // resource IDs in merge_order
	lastEmitDownloaded   int64     // last emitted total downloaded, used to skip duplicate broadcasts
	lastEmitSpeed        int64     // last emitted total speed
	lastEmitTime         time.Time // last emission time, used for keepalive when segments are connecting
}

// resourceTracker holds the current download progress and metadata of a single resource.
// resourceTracker holds the current download progress and metadata of a single resource.
// speed stores the real-time speed reported by the download loop (copyReader/downloadSegment),
// which is updated every progressInterval (500ms). snapshotProgress uses this value directly
// for the WS push rather than re-computing from deltas, ensuring speed is never stale.
type resourceTracker struct {
	size         int64
	downloaded   int64
	speed        int64
	name         string
	kind         string
	resourceType string
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

// NewOpt is the configuration struct for Engine construction.
type NewOpt struct {
	Store               Store
	Logger              *zerolog.Logger
	MaxConcurrent       int
	FilenameTemplate    string
	BasePath            string
	ProgressEmitInterval time.Duration // Progress event emission interval, <=0 uses default 180ms
	SpeedLimit          int64          // Per-segment download speed limit (bytes/sec), 0 means unlimited
	SegmentConcurrency  int            // Max concurrent segments per resource, <=0 uses default 5
	ReadTimeout         time.Duration  // Timeout for a single Read() call, <=0 uses default 10s
}

// New creates a new Engine from the given configuration.
func New(opt NewOpt) *Engine {
	maxConcurrent := opt.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	progressEmitInterval := opt.ProgressEmitInterval
	if progressEmitInterval <= 0 {
		progressEmitInterval = 180 * time.Millisecond
	}
	segmentConcurrency := opt.SegmentConcurrency
	if segmentConcurrency <= 0 {
		segmentConcurrency = 5
	}
	readTimeout := opt.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = defaultReadTimeout
	}
	logger := zerolog.Nop()
	if opt.Logger != nil {
		logger = *opt.Logger
	}
	e := &Engine{
		maxConcurrent:        maxConcurrent,
		sem:                  make(chan struct{}, maxConcurrent),
		jobs:                 make(map[int]*job),
		store:                opt.Store,
		logger:               logger,
		drivers:              make(map[string]ProtocolDriver),
		filenameTemplate:     opt.FilenameTemplate,
		basePath:             opt.BasePath,
		progressCache:        make(map[int]*progressTracker),
		progressEmitInterval: progressEmitInterval,
		speedLimit:           opt.SpeedLimit,
		segmentConcurrency:   segmentConcurrency,
		readTimeout:          readTimeout,
	}
	return e
}

// SetEventHandler sets the task lifecycle and progress event handler. Pass nil to disable event callbacks.
func (d *Engine) SetEventHandler(handler EventHandler) {
	d.eventMu.Lock()
	defer d.eventMu.Unlock()
	d.onEvent = handler
}

// SetHooks sets the JS hook manager. Pass nil to disable hooks.
func (d *Engine) SetHooks(h *HookManager) {
	d.hooks = h
}

// SetPostprocessor registers platform-specific post-processing.
func (d *Engine) SetPostprocessor(p Postprocessor) {
	d.postprocessor = p
}

// RegisterProtocol registers or replaces a protocol driver. Protocol names are case-insensitive.
func (d *Engine) RegisterProtocol(driver ProtocolDriver) {
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

// Start submits a task to the scheduler. Concurrent slot acquisition happens in the background,
// so queuing does not block API requests.
func (d *Engine) Start(taskID int) error {
	if taskID <= 0 {
		return errors.New("taskID 必须大于 0")
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
		return fmt.Errorf("更新准备状态失败: %w", err)
	}
	d.emit(taskID, EventCreated)
	d.logger.Info().Int("taskID", taskID).Msg("task started")
	go d.schedule(taskID, job)
	return nil
}

// Pause cancels and waits for the current execution instance to exit, ensuring subsequent Resume
// will not write files concurrently with the old Writer.
func (d *Engine) Pause(taskID int) {
	if job := d.findJob(taskID); job != nil {
		job.stop(cancelPause)
		<-job.done
	}
}

// PauseAll pauses all in-progress or queued downloads.
func (d *Engine) PauseAll() {
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

// Delete stops the execution instance and marks the task as cancelled.
// Soft deletion of database entities is still handled by the API handler.
func (d *Engine) Delete(taskID int) {
	if job := d.findJob(taskID); job != nil {
		job.stop(cancelDelete)
		<-job.done
		_ = d.store.UpdateStatus(taskID, TaskStatusCancelled)
		d.logger.Info().Int("taskID", taskID).Msg("task deleted")
		d.emit(taskID, EventDeleted)
		d.deleteTracker(taskID)
	}
}

func (d *Engine) findJob(taskID int) *job {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.jobs[taskID]
}

func (d *Engine) schedule(taskID int, job *job) {
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
				runErr = fmt.Errorf("下载协程异常退出: %v", recovered)
				d.logger.Error().
					Int("taskID", taskID).
					Interface("panic", recovered).
					Str("stack", string(debug.Stack())).
					Msg("download goroutine panicked")
			}
		}()
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

func (d *Engine) handleCancellation(taskID int, job *job) {
	if job.cancellationReason() == cancelPause {
		d.pauseTask(taskID)
	}
}

func (d *Engine) run(taskID int, ctx context.Context) error {
	info, err := d.store.LoadTask(taskID)
	if err != nil {
		return fmt.Errorf("加载任务信息失败: %w", err)
	}
	if info == nil {
		return errors.New("加载任务信息失败: task is nil")
	}
	if len(info.Resources) == 0 {
		return errors.New("任务没有可下载资源")
	}
	resources := info.Resources

	// Task start log: record basic task information
	d.logger.Info().
		Int("taskID", taskID).
		Str("taskName", info.Name).
		Str("savePath", info.SavePath).
		Int("resourceCount", len(resources)).
		Msg("download task started")

	// Log detailed information for each resource
	for i, r := range resources {
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceID", r.ID).
			Int("resourceIndex", i+1).
			Str("resourceName", r.Name).
			Str("resourceKind", r.Kind).
			Str("resourceType", r.ResourceType).
			Int("endpointCount", len(r.Endpoints)).
			Msg("resource info")
	}

	if err := d.store.UpdateStatus(taskID, TaskStatusDownloading); err != nil {
		return fmt.Errorf("更新下载状态失败: %w", err)
	}
	if err := d.store.ActivateTask(taskID); err != nil {
		return fmt.Errorf("激活任务失败: %w", err)
	}
	d.emit(taskID, EventStarted)

	config, _ := parseConfigAndMetadata(info.Config, info.Metadata)

	// Pre-probe all resource sizes, record time overhead
	probeStart := time.Now()
	resourceSizes := d.ensureResourceSizes(ctx, taskID, resources)
	probeElapsed := time.Since(probeStart).Round(time.Millisecond)

	d.initTracker(taskID, resourceSizes, resources)

	// Start periodic progress emission
	progressTicker := time.NewTicker(d.progressEmitInterval)
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
				Msg("resource size probed")
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
			Msg("task size summary")
	} else {
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceCount", len(resources)).
			Int("unknownSizeCount", unknownSizes).
			Str("probeElapsed", probeElapsed.String()).
			Msg("task total size unknown, downloading resource by resource")
	}

	// Concurrently download all resources
	downloadStart := time.Now()
	var extensionsMu sync.Mutex
	resourceExtensions := make(map[int]string)
	var completedSize atomic.Int64

	type resourceResult struct {
		filePath   string
		err        error
		elapsed    time.Duration
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
			filePath, err := d.downloadResource(downloadCtx, taskID, info.SavePath, res.ResourceType, res, config, localExts)
			elapsed := time.Since(resStart).Round(time.Millisecond)

			results[idx] = &resourceResult{filePath: filePath, err: err, elapsed: elapsed}

			if err != nil {
				if !errors.Is(err, context.Canceled) && downloadCtx.Err() == nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("资源 %s 下载失败: %w", res.Name, err)
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

func (d *Engine) downloadResource(ctx context.Context, taskID int, savePath string, resourceType string, resource Resource, config map[string]any, resourceExtensions map[int]string) (string, error) {
	originalDBName := resource.Name
	// Use unique_id as download filename to avoid collisions between tasks
	// (e.g., mp3 conversion task vs normal download of the same video).
	// The display name is preserved in resource.Extra["title"] for the final rename.
	downloadName := resource.Name
	if resource.UniqueID != "" {
		downloadName = resource.UniqueID
	}
	configJSON, _ := json.Marshal(config)
	resourceTask := &Task{
		ID:               taskID,
		Name:             downloadName,
		SavePath:         savePath,
		FilenameTemplate: d.filenameTemplate,
		ResourceID:       resource.ID,
		Endpoints:        resource.Endpoints,
		Config:           string(configJSON),
	}
	candidates, err := d.endpointCandidates(resourceTask)
	if err != nil {
		return "", err
	}

	var endpointErrors []string
	var filePath string
	var expectedSize int64
	for _, candidate := range candidates {
		if err := context.Cause(ctx); err != nil {
			return "", err
		}
		if candidate.driver == nil {
			endpointErrors = append(endpointErrors, fmt.Sprintf("%s: 未注册协议驱动", candidate.protocol))
			continue
		}

		prepared, prepareErr := prepareWithRetry(ctx, candidate.driver, candidate.endpoint)
		if prepareErr != nil {
			if errors.Is(prepareErr, context.Canceled) {
				return "", prepareErr
			}
			endpointErrors = append(endpointErrors, fmt.Sprintf("%s: %v", candidate.protocol, prepareErr))
			continue
		}
		if prepared.Size < 0 {
			prepared.Size = 0
		}
		if expectedSize > 0 && prepared.Size > 0 && prepared.Size != expectedSize {
			endpointErrors = append(endpointErrors, fmt.Sprintf("%s: 镜像资源大小不一致", candidate.protocol))
			continue
		}
		if expectedSize == 0 && prepared.Size > 0 {
			expectedSize = prepared.Size
		}
		if prepared.Size > 0 {
			if err := d.updateResourceSize(taskID, resource.ID, prepared.Size); err != nil {
				return "", fmt.Errorf("更新资源大小失败: %w", err)
			}
			d.updateTrackerSize(taskID, resource.ID, prepared.Size)
		}

		// Once segment records exist, resource.Name is the canonical path that was
		// persisted when the download first started. Reapplying filename templates
		// or hooks after a restart can produce a different path, making the existing
		// .part file appear missing and causing downloadSegments to reset every
		// persisted offset to zero.
		existingSegments, segmentErr := d.store.LoadSegmentInfo(resource.ID)
		if segmentErr != nil {
			return "", fmt.Errorf("读取已有下载分片失败: %w", segmentErr)
		}
		resuming := len(existingSegments) > 0
		if resuming {
			d.logger.Info().
				Int("taskID", taskID).
				Int("resourceID", resource.ID).
				Int("segmentCount", len(existingSegments)).
				Str("resourceName", resourceTask.Name).
				Msg("existing segments found, preserving persisted filename")
		}

		if !resuming && resourceTask.FilenameTemplate != "" && downloadName == originalDBName {
			meta := buildTemplateMeta(resource.Extra, config, resourceTask.Name)
			rawName := resourceTask.Name
			if newName := d.applyFilenameTemplate(resourceTask, candidate.endpoint.URL, meta); newName != "" {
				resourceTask.Name = newName
				d.logger.Info().
					Int("taskID", taskID).
					Int("resourceID", resource.ID).
					Str("oldName", rawName).
					Str("newName", newName).
					Str("template", resourceTask.FilenameTemplate).
					Interface("meta", meta).
					Msg("filename template applied")
			}
		}

		// onFilename hook: user-defined final filename
		// Skip when using unique_id as download name; template and hook are
		// deferred to finishTask after the filename is restored to the display name.
		if !resuming && d.hooks != nil && d.hooks.HasFilenameHook() && downloadName == originalDBName {
			params := &FilenameParams{
				Meta: buildResourceMeta(resource.Extra, config),
				Task: TaskInfo{
					Name:     resourceTask.Name,
					SavePath: savePath,
					Config:   config,
				},
				Config: config,
			}
			rawName := resourceTask.Name
			if newName, err := d.hooks.InvokeFilenameHook(params, resourceTask.Name); err != nil {
				d.logger.Warn().Err(err).Msg("onFilename hook execution failed")
			} else if newName != "" {
				resourceTask.Name = newName
				d.logger.Info().
					Int("taskID", taskID).
					Int("resourceID", resource.ID).
					Str("oldName", rawName).
					Str("newName", newName).
					Msg("onFilename hook applied")
			}
		}

		_, err = d.processOutputFilename(resourceTask, candidate.endpoint.URL, prepared, resource.Extension, originalDBName, resourceExtensions)
		if err != nil {
			return "", err
		}
		filePath, err = d.filePathForResource(resourceTask, candidate.endpoint.URL)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return "", fmt.Errorf("创建下载目录失败: %w", err)
		}

		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceID", resource.ID).
			Str("endpoint", candidate.endpoint.URL).
			Str("filePath", d.relLogPath(filePath)).
			Msg("starting resource download")

		segmentCount := chooseSegmentCount(prepared)
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceID", resource.ID).
			Bool("segmented", segmentCount > 1).
			Int("segmentCount", segmentCount).
			Int64("segmentSize", minimumSegmentSize).
			Msg("download mode selected")
		if segmentCount > 1 {
			err = d.downloadSegments(ctx, candidate.driver, candidate.endpoint, filePath, resource.ID, prepared.Size, segmentCount, taskID)
		} else {
			err = d.downloadFile(ctx, candidate.driver, candidate.endpoint, filePath, resource.ID, prepared, taskID)
		}
		if err == nil {
			d.logger.Info().
				Int("taskID", taskID).
				Int("resourceID", resource.ID).
				Str("filePath", d.relLogPath(filePath)).
				Msg("data transfer completed")
			if prepared.Size <= 0 {
				if fileInfo, statErr := os.Stat(filePath); statErr == nil {
					if err := d.updateResourceSize(taskID, resource.ID, fileInfo.Size()); err != nil {
						return "", fmt.Errorf("更新资源最终大小失败: %w", err)
					}
					d.updateTrackerSize(taskID, resource.ID, fileInfo.Size())
				}
			}
			if store, ok := d.store.(ResourceStore); ok {
				d.logger.Info().
					Int("taskID", taskID).
					Int("resourceID", resource.ID).
					Msg("persisting resource state")
				if err := store.FinishResource(resource.ID); err != nil {
					return "", fmt.Errorf("完成资源持久化失败: %w", err)
				}
			}
			return filePath, nil
		}
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return "", context.Cause(ctx)
		}
		endpointErrors = append(endpointErrors, fmt.Sprintf("%s: %v", candidate.protocol, err))
		d.logger.Warn().
			Int("endpointID", candidate.endpoint.ID).
			Int("taskID", taskID).
			Int("resourceID", resource.ID).
			Err(err).
			Msg("endpoint failed, trying next mirror")
	}
	return "", fmt.Errorf("所有下载端点均不可用: %s", strings.Join(endpointErrors, "; "))
}

// processOutputFilename handles download resource output filenames uniformly.
// Called after filenameTemplate and onFilename hook processing; completes:
//  1. Determine file extension (Content-Type -> magic bytes -> user-specified fallback)
//  2. User input is always treated as a plain filename (ignoring any embedded extension); the system appends the extension
//  3. Clean and truncate the base filename (preserving directory portion)
//  4. Reconstruct the full path and update task/resource info in the database
//
// Each step outputs logs for easy troubleshooting.
func (d *Engine) processOutputFilename(task *Task, endpointURL string, prepared PreparedResource, extensionFallback string, originalDBName string, resourceExtensions map[int]string) (bool, error) {
	if task == nil || task.ResourceID <= 0 {
		return false, nil
	}

	rawName := strings.TrimSpace(task.Name)
	if rawName == "" {
		return false, nil
	}

	// Step 1: Separate directory and base filename (user input is a plain filename without extension)
	dir, baseName := filepath.Split(rawName)
	d.logger.Info().
		Int("taskID", task.ID).
		Int("resourceID", task.ResourceID).
		Str("rawName", rawName).
		Str("dir", dir).
		Str("baseName", baseName).
		Msg("output filename processing started")

	// Step 2: Determine extension
	// Priority: Content-Type -> magic bytes -> user-specified fallback suffix
	ext := extensionForContentType(prepared.ContentType)
	if ext != "" {
		d.logger.Info().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Str("extension", ext).
			Str("contentType", prepared.ContentType).
			Msg("extension from content type")
	}
	if ext == "" {
		if detectedType := detectContentTypeFromBytes(prepared.ProbeData); detectedType != "" {
			ext = extensionForContentType(detectedType)
			if ext != "" {
				d.logger.Info().
				Int("taskID", task.ID).
				Int("resourceID", task.ResourceID).
				Str("extension", ext).
				Str("detectedType", detectedType).
				Msg("extension from magic bytes")
			}
		}
	}
	if ext == "" {
		ext = extensionFallback
		if ext != "" {
			d.logger.Info().
				Int("taskID", task.ID).
				Int("resourceID", task.ResourceID).
				Str("extension", ext).
				Msg("using user-specified fallback extension")
		}
	}

	// Persist extension for file rename during finishTask
	if ext != "" && resourceExtensions != nil {
		resourceExtensions[task.ResourceID] = ext
	}

	// Step 3: Check for existing segments (resume skips filename processing)
	if ext != "" && task.ResourceID > 0 {
		segments, err := d.store.LoadSegmentInfo(task.ResourceID)
		if err != nil {
			return false, fmt.Errorf("读取已有下载分片失败: %w", err)
		}
		if len(segments) > 0 {
			persistedName := strings.TrimSpace(originalDBName)
			if persistedName != "" && task.Name != persistedName {
				d.logger.Warn().
					Int("taskID", task.ID).
					Int("resourceID", task.ResourceID).
					Str("derivedName", task.Name).
					Str("persistedName", persistedName).
					Msg("discarding derived filename while resuming")
				task.Name = persistedName
			}
			d.logger.Info().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Int("segmentCount", len(segments)).
			Msg("existing segments found, skipping filename processing (resume)")
			return false, nil
		}
	}

	// Step 4: Check if output file already exists (check .tmp and post-processed final files)
	if ext != "" {
		tmpExt := ".tmp"
		// Potential filenames to check: temp file .tmp and post-processed final file (with config suffix)
		candidateNames := []string{dir + baseName + tmpExt}
		if cfgSuffix := getConfigString(task.Config, "suffix"); cfgSuffix != "" && cfgSuffix != tmpExt {
			candidateNames = append(candidateNames, dir+baseName+cfgSuffix)
		}
		// Also check the actual detected extension (e.g. .jpg from magic bytes), so
		// that duplicate downloads can find files renamed by a prior completed task.
		if ext != "" && ext != tmpExt {
			candidateExt := dir + baseName + ext
			already := false
			for _, c := range candidateNames {
				if c == candidateExt {
					already = true
					break
				}
			}
			if !already {
				candidateNames = append(candidateNames, candidateExt)
			}
		}

		var currentPath string
		var fileExists bool
		for _, tryName := range candidateNames {
			tmpTask := &Task{
				ID: task.ID, Name: tryName,
				SavePath: task.SavePath, ResourceID: task.ResourceID,
			}
			if path, err := d.filePathForResource(tmpTask, endpointURL); err == nil {
				if info, statErr := os.Stat(path); statErr == nil && info.Size() > 0 {
					currentPath = path
					fileExists = true
					d.logger.Info().
						Int("taskID", task.ID).
						Int("resourceID", task.ResourceID).
						Str("filePath", currentPath).
						Msg("existing output file detected")
					break
				}
			}
		}

		if fileExists {
			d.logger.Info().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Str("filePath", currentPath).
			Str("config", task.Config).
			Msg("file exists with config")
			isDup := getConfigBool(task.Config, "duplicate")
			d.logger.Info().
				Int("taskID", task.ID).
				Int("resourceID", task.ResourceID).
				Bool("duplicate", isDup).
				Msg("duplicate config parsed")
			// duplicate=true: when temp file exists, auto-append numeric suffix (1), (2), ...
			if isDup {
				newName := d.findNextDuplicateName(task, currentPath, dir, baseName, tmpExt)
				d.logger.Info().
				Int("taskID", task.ID).
				Int("resourceID", task.ResourceID).
				Str("existingPath", currentPath).
				Str("newName", newName).
				Msg("file exists, duplicate enabled")
				task.Name = newName
				// Persist temp filename to DB; final extension written by finishTask
				if _, err := d.persistResourceName(task, newName, originalDBName, "duplicate"); err != nil {
					d.logger.Warn().
						Int("taskID", task.ID).
						Int("resourceID", task.ResourceID).
						Err(err).
						Msg("failed to update resource name")
				}
				return false, nil
			}
			// duplicate=false: file exists, skip download but update DB resource name for consistency
			task.Name = dir + baseName + tmpExt
			d.logger.Info().
				Int("taskID", task.ID).
				Int("resourceID", task.ResourceID).
				Str("existingPath", currentPath).
				Str("oldDBName", originalDBName).
				Str("newDBName", task.Name).
				Msg("file exists, duplicate disabled, resource name persisted to DB")
			if _, err := d.persistResourceName(task, task.Name, originalDBName, "overwrite"); err != nil {
				d.logger.Warn().
					Int("taskID", task.ID).
					Int("resourceID", task.ResourceID).
					Err(err).
					Msg("failed to update resource name")
			}
			return false, nil
		}
	}

	// Step 5: If no extension, abandon filename processing
	if ext == "" {
		d.logger.Info().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Msg("cannot determine extension, skipping filename processing")
		return false, nil
	}

	// Step 6: Sanitize and truncate base filename (keep directory portion unchanged)
	fp := NewFilenameProcessor("", nil)
	cleanBase, err := fp.SanitizeFilename(baseName)
	if err != nil {
		return false, fmt.Errorf("清理文件名失败: %w", err)
	}
	d.logger.Info().
		Int("taskID", task.ID).
		Int("resourceID", task.ResourceID).
		Str("oldName", baseName).
		Str("cleanName", cleanBase).
		Msg("filename sanitized")

	// Truncate overly long filenames (235 byte limit must include .tmp extension)
	tmpExt := ".tmp"
	maxBaseLen := fp.maxNameLength - len(tmpExt)
	if maxBaseLen > 0 && len(cleanBase) > maxBaseLen {
		truncated := fp.truncateString(cleanBase, maxBaseLen)
		d.logger.Info().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Int("oldLen", len(cleanBase)).
			Int("newLen", len(truncated)).
			Msg("filename truncated due to length")
		cleanBase = truncated
	}
	if cleanBase == "" {
		return false, fmt.Errorf("文件名仅包含无效字符")
	}

	// Step 7: Reconstruct full temp file path (.tmp suffix, final extension written by finishTask)
	resourceName := dir + cleanBase + tmpExt
	d.logger.Info().
		Int("taskID", task.ID).
		Int("resourceID", task.ResourceID).
		Str("resourceName", resourceName).
		Str("baseName", cleanBase).
		Str("tmpExt", tmpExt).
		Str("dir", dir).
		Msg("final temp output filename")

	// Step 8: Compare with original DB name; skip DB update if unchanged
	if resourceName == originalDBName {
		d.logger.Info().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Msg("filename matches DB, skipping DB update")
		task.Name = resourceName
		return false, nil
	}

	// Step 9: Update temp resource name in database
	task.Name = resourceName
	if updated, err := d.persistResourceName(task, resourceName, originalDBName, "new"); err != nil {
		return false, err
	} else {
		return updated, nil
	}
}

func extensionForContentType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType == "" {
		return ""
	}
	// Prefer exact mapping (handles special cases like image/jpeg -> .jpg)
	if ext, ok := contentTypeExtMap[mediaType]; ok {
		return ext
	}
	// Generic binary stream: server doesn't know the specific type; don't infer extension from Content-Type
	if mediaType == "application/octet-stream" {
		return ""
	}
	// Fallback: use Go standard library MIME extension table (return first matching extension)
	exts, err := mime.ExtensionsByType(mediaType)
	if err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ""
}

// contentTypeExtMap is a precise MIME type -> extension mapping.
// Takes priority over mime.ExtensionsByType for special cases (e.g., .jpg vs .jpe).
var contentTypeExtMap = map[string]string{
	"image/jpeg":       ".jpg",
	"image/png":        ".png",
	"image/gif":        ".gif",
	"image/webp":       ".webp",
	"image/avif":       ".avif",
	"video/mp4":        ".mp4",
	"video/webm":       ".webm",
	"video/quicktime":  ".mov",
	"video/x-msvideo":  ".avi",
	"video/x-matroska": ".mkv",
	"audio/mpeg":       ".mp3",
	"audio/mp4":        ".m4a",
	"audio/aac":        ".aac",
	"audio/ogg":        ".ogg",
	"application/pdf":  ".pdf",
	"application/zip":  ".zip",
}

// detectContentTypeFromBytes detects file type via magic bytes.
// Returns a MIME type string, or empty string if unrecognized.
// Used as a supplementary detection method when Content-Type header is absent.
func detectContentTypeFromBytes(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	switch {
	// PNG
	case len(data) >= 4 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G':
		return "image/png"
	// JPEG
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	// GIF87a / GIF89a
	case len(data) >= 4 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' && data[3] == '8':
		return "image/gif"
	// WebP
	case len(data) >= 12 && data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P':
		return "image/webp"
	// BMP
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return "image/bmp"
	// MP4 / M4A (ftyp box at offset 4)
	case len(data) >= 12 && string(data[4:8]) == "ftyp":
		return "video/mp4"
	// MKV / WebM (EBML header)
	case len(data) >= 4 && data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3:
		return "video/x-matroska"
	// MP3 (ID3 tag or MPEG sync)
	case len(data) >= 3 && data[0] == 'I' && data[1] == 'D' && data[2] == '3':
		return "audio/mpeg"
	case len(data) >= 2 && data[0] == 0xFF && (data[1]&0xE0) == 0xE0:
		return "audio/mpeg"
	// OGG
	case len(data) >= 4 && data[0] == 'O' && data[1] == 'g' && data[2] == 'g' && data[3] == 'S':
		return "audio/ogg"
	// FLAC
	case len(data) >= 4 && data[0] == 'f' && data[1] == 'L' && data[2] == 'a' && data[3] == 'C':
		return "audio/flac"
	// WAV
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE":
		return "audio/wav"
	// PDF
	case len(data) >= 5 && string(data[0:5]) == "%PDF-":
		return "application/pdf"
	// ZIP (PK\x03\x04)
	case len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04:
		return "application/zip"
	// RAR
	case len(data) >= 7 && data[0] == 'R' && data[1] == 'a' && data[2] == 'r' && data[3] == '!' && data[4] == 0x1A && data[5] == 0x07:
		return "application/x-rar-compressed"
	// 7z
	case len(data) >= 6 && data[0] == '7' && data[1] == 'z' && data[2] == 0xBC && data[3] == 0xAF && data[4] == 0x27 && data[5] == 0x1C:
		return "application/x-7z-compressed"
	// GZIP
	case len(data) >= 2 && data[0] == 0x1F && data[1] == 0x8B:
		return "application/gzip"
	default:
		return ""
	}
}

func (d *Engine) updateResourceSize(taskID, resourceID int, size int64) error {
	if store, ok := d.store.(ResourceStore); ok {
		return store.UpdateResourceSizeByID(resourceID, size)
	}
	return d.store.UpdateResourceSize(taskID, size)
}

func (d *Engine) updateResourceProgress(taskID, resourceID int, downloaded, speed int64) error {
	if store, ok := d.store.(ResourceStore); ok {
		return store.UpdateResourceProgress(resourceID, downloaded, speed)
	}
	return d.store.UpdateProgress(taskID, downloaded, speed)
}

func prepareWithRetry(ctx context.Context, driver ProtocolDriver, endpoint Endpoint) (PreparedResource, error) {
	var lastErr error
	for attempt := 0; attempt < maxReadAttempts; attempt++ {
		prepared, err := driver.Prepare(ctx, endpoint)
		if err == nil {
			return prepared, nil
		}
		if errors.Is(err, context.Canceled) {
			return PreparedResource{}, err
		}
		if ctx.Err() != nil {
			return PreparedResource{}, context.Cause(ctx)
		}
		lastErr = err
		if attempt < maxReadAttempts-1 && !waitForRetry(ctx, attempt) {
			return PreparedResource{}, context.Cause(ctx)
		}
	}
	return PreparedResource{}, lastErr
}

func (d *Engine) applyFilenameTemplate(task *Task, endpointURL string, meta map[string]string) string {
	// If template contains {{var}} syntax, use shared template var replacement
	if strings.Contains(task.FilenameTemplate, "{{") {
		return cleanPathSeparators(util.ReplaceTemplateVars(task.FilenameTemplate, meta))
	}

	// Fall through to JS VM evaluation for expression-based templates
	urlBasename := ""
	if u, err := url.Parse(endpointURL); err == nil {
		urlBasename = filepath.Base(u.Path)
	}

	vm := goja.New()
	vm.Set("name", task.Name)
	vm.Set("task_id", task.ID)
	vm.Set("resource_id", task.ResourceID)
	vm.Set("url_basename", urlBasename)

	vm.Set("formatTime", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		return vm.ToValue(time.Now().Format(call.Argument(0).String()))
	})

	vm.Set("padStart", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return call.Arguments[0]
		}
		s := call.Argument(0).String()
		length := int(call.Argument(1).ToInteger())
		pad := "0"
		if len(call.Arguments) >= 3 {
			pad = call.Argument(2).String()
		}
		for len(s) < length {
			s = pad + s
		}
		return vm.ToValue(s)
	})

	result, err := vm.RunString(task.FilenameTemplate)
	if err != nil {
		d.logger.Warn().Err(err).Msg("filename template error")
		return ""
	}

	return cleanPathSeparators(result.String())
}

// cleanPathSeparators trims whitespace around each / separator in a path string,
// so that e.g. "AuthorName / VideoTitle" becomes "AuthorName/VideoTitle".
// Leading/trailing whitespace is also trimmed.
func cleanPathSeparators(s string) string {
	parts := strings.Split(s, "/")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Trim(strings.Join(parts, "/"), "/")
}

func (d *Engine) endpointCandidates(info *Task) ([]endpointCandidate, error) {
	endpoints := append([]Endpoint(nil), info.Endpoints...)
	if len(endpoints) == 0 {
		return nil, errors.New("任务没有可用下载端点")
	}
	sort.SliceStable(endpoints, func(i, j int) bool { return endpoints[i].Priority < endpoints[j].Priority })

	candidates := make([]endpointCandidate, 0, len(endpoints))
	for _, endpoint := range endpoints {
		protocol := strings.ToLower(strings.TrimSpace(endpoint.Protocol))
		if protocol == "" {
			parsed, err := url.Parse(endpoint.URL)
			if err == nil {
				protocol = strings.ToLower(parsed.Scheme)
			}
		}
		d.mu.Lock()
		driver := d.drivers[protocol]
		d.mu.Unlock()
		candidates = append(candidates, endpointCandidate{endpoint: endpoint, protocol: protocol, driver: driver})
	}
	return candidates, nil
}

// ensureResourceSizes probes each resource to determine its size before the
// download loop starts. When all resource sizes are known upfront, the API can
// compute correct task-level aggregate progress (sum of all resource segments
// divided by sum of all resource sizes), avoiding the 100%→partial→100%
// oscillation that occurs when sizes are discovered one resource at a time.
// Failures are non-fatal; the download loop will retry Prepares as needed.
// Returns a map of resourceID→size for resources whose size was successfully
// determined.
func (d *Engine) ensureResourceSizes(ctx context.Context, taskID int, resources []Resource) map[int]int64 {
	sizes := make(map[int]int64)
	for i := range resources {
		res := &resources[i]
		candidates, err := d.endpointCandidates(&Task{Endpoints: res.Endpoints})
		if err != nil {
			continue
		}
		for _, c := range candidates {
			if ctx.Err() != nil {
				return sizes
			}
			if c.driver == nil {
				continue
			}
			prepared, err := c.driver.Prepare(ctx, c.endpoint)
			if err != nil {
				continue
			}
			if prepared.Size > 0 {
				_ = d.updateResourceSize(taskID, res.ID, prepared.Size)
				sizes[res.ID] = prepared.Size
				break
			}
		}
	}
	return sizes
}

// absFilePath constructs absolute path: basePath + savePath + name.
func (d *Engine) absFilePath(savePath, name string) string {
	return filepath.Join(d.basePath, savePath, name)
}

// relLogPath converts absolute absPath to relative (strips basePath prefix).
func (d *Engine) relLogPath(absPath string) string {
	if d.basePath == "" {
		return absPath
	}
	rel, err := filepath.Rel(d.basePath, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return absPath
	}
	return rel
}

// filePathForResource resolves the absolute on-disk path for a task's resource.
func (d *Engine) filePathForResource(info *Task, endpointURL string) (string, error) {
	name := strings.TrimSpace(info.Name)
	if name == "" {
		if parsed, err := url.Parse(endpointURL); err == nil {
			name = filepath.Base(parsed.Path)
		}
	}
	name = filepath.Clean(name)
	name = strings.TrimLeft(name, "/")
	// Strip leading path traversal prefixes (same effect as filepath.Base but preserves subdirectories)
	for strings.HasPrefix(name, "../") {
		name = name[3:]
	}
	// Prevent path traversal attacks
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, string(filepath.Separator)+"..") {
		return "", errors.New("无法确定下载文件名")
	}

	return d.absFilePath(info.SavePath, name), nil
}

// taskFilePath is the legacy function kept for backward compatibility.
func taskFilePath(info *Task, endpointURL string) (string, error) {
	if strings.TrimSpace(info.SavePath) == "" {
		return "", errors.New("保存路径不能为空")
	}
	name := strings.TrimSpace(info.Name)
	if name == "" {
		if parsed, err := url.Parse(endpointURL); err == nil {
			name = filepath.Base(parsed.Path)
		}
	}
	name = filepath.Clean(name)
	name = strings.TrimLeft(name, "/")
	// Strip leading path traversal prefixes (same effect as filepath.Base but preserves subdirectories)
	for strings.HasPrefix(name, "../") {
		name = name[3:]
	}
	// Prevent path traversal attacks
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, string(filepath.Separator)+"..") {
		return "", errors.New("无法确定下载文件名")
	}

	savePath := filepath.Clean(info.SavePath)
	return filepath.Join(savePath, name), nil
}

func chooseSegmentCount(prepared PreparedResource) int {
	if !prepared.SupportsRange || prepared.Size <= minimumSegmentSize {
		return 1
	}
	count := int((prepared.Size + minimumSegmentSize - 1) / minimumSegmentSize)
	if count > defaultSegmentCount {
		count = defaultSegmentCount
	}
	return count
}

// splitFile divides a file into n non-empty segments, distributing any remainder across the first segments.
func splitFile(fileSize int64, n int) []SegmentRange {
	if n <= 0 || fileSize <= 0 {
		return nil
	}
	if int64(n) > fileSize {
		n = int(fileSize)
	}
	baseSize := fileSize / int64(n)
	remainder := fileSize % int64(n)
	ranges := make([]SegmentRange, n)
	var offset int64
	for i := 0; i < n; i++ {
		size := baseSize
		if int64(i) < remainder {
			size++
		}
		ranges[i] = SegmentRange{Index: i, OffsetStart: offset, OffsetEnd: offset + size - 1, Size: size}
		offset += size
	}
	return ranges
}

func (d *Engine) downloadFile(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	filePath string,
	resourceID int,
	prepared PreparedResource,
	taskID int,
) error {
	segments, err := d.store.LoadSegmentInfo(resourceID)
	if err != nil {
		return fmt.Errorf("加载分片信息失败: %w", err)
	}
	ranges := []SegmentRange{{Index: 0, OffsetStart: 0, OffsetEnd: maxInt64(0, prepared.Size-1), Size: prepared.Size}}
	if !segmentsMatchRanges(segments, ranges) {
		ids, err := d.store.CreateSegments(resourceID, endpoint.URL, ranges)
		if err != nil {
			return fmt.Errorf("创建分片记录失败: %w", err)
		}
		if len(ids) != 1 {
			return errors.New("创建分片记录失败: 返回的 ID 数量不正确")
		}
		segments = []Segment{{ID: ids[0], Index: 0, URL: endpoint.URL, Size: prepared.Size, OffsetEnd: ranges[0].OffsetEnd}}
	}
	segment := &segments[0]
	partPath := filePath + partialFileSuffix

	// A crash may happen after persisting completion but before the atomic rename.
	// Prefer the durable partial file over an older same-sized destination.
	if prepared.Size > 0 && segment.Downloaded == prepared.Size && fileHasSize(partPath, prepared.Size) {
		if err := finalizePartialFile(partPath, filePath); err != nil {
			return fmt.Errorf("完成临时文件失败: %w", err)
		}
		d.updateTracker(taskID, resourceID, prepared.Size, 0)
		return nil
	}
	// A same-sized destination alone is not proof that this resource completed.
	// Only persisted completion state may make an existing destination reusable.
	if prepared.Size > 0 && segment.Downloaded == prepared.Size && fileHasSize(filePath, prepared.Size) {
		d.updateTracker(taskID, resourceID, prepared.Size, 0)
		return nil
	}

	var downloaded int64
	if fi, statErr := os.Stat(partPath); statErr == nil {
		downloaded = fi.Size()
	}
	if prepared.Size > 0 && (downloaded < 0 || downloaded > prepared.Size) {
		downloaded = 0
		if err := os.Truncate(partPath, 0); err != nil {
			return fmt.Errorf("重置临时文件失败: %w", err)
		}
	}
	if segment.Downloaded != downloaded {
		segment.Downloaded = downloaded
		if err := d.store.UpdateSegmentProgress(segment.ID, downloaded); err != nil {
			return fmt.Errorf("校准分片进度失败: %w", err)
		}
	}
	if !prepared.SupportsRange {
		downloaded = 0
	}

	for attempt := 0; attempt < maxReadAttempts; attempt++ {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		useRange := prepared.SupportsRange && downloaded > 0
		request := ReadRequest{OffsetStart: downloaded, OffsetEnd: prepared.Size - 1, UseRange: useRange}
		reader, err := driver.Open(ctx, endpoint, request)
		if err != nil {
			if !waitForRetry(ctx, attempt) {
				return context.Cause(ctx)
			}
			if attempt == maxReadAttempts-1 {
				return fmt.Errorf("打开下载源失败: %w", err)
			}
			continue
		}

		flags := os.O_CREATE | os.O_WRONLY
		if useRange {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
			downloaded = 0
		}
		file, openErr := os.OpenFile(partPath, flags, 0644)
		if openErr != nil {
			reader.Close()
			return fmt.Errorf("打开临时文件失败: %w", openErr)
		}

		err = d.copyReader(ctx, reader, file, prepared.Size, &downloaded, taskID, resourceID, func(total, speed int64) error {
			return d.persistProgress(taskID, resourceID, segment.ID, total, speed)
		})
		readerCloseErr := reader.Close()
		if err == nil && readerCloseErr != nil {
			err = readerCloseErr
		}
		if err == nil && (prepared.Size <= 0 || downloaded == prepared.Size) {
			if syncErr := file.Sync(); syncErr != nil {
				err = syncErr
			} else if persistErr := d.persistProgress(taskID, resourceID, segment.ID, downloaded, 0); persistErr != nil {
				err = persistErr
			}
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil && (prepared.Size <= 0 || downloaded == prepared.Size) {
			if renameErr := os.Rename(partPath, filePath); renameErr != nil {
				return fmt.Errorf("提交下载文件失败: %w", renameErr)
			}
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		if prepared.Size > 0 && downloaded >= prepared.Size {
			return fmt.Errorf("下载数据大小异常: 期望 %d 字节，实际 %d 字节", prepared.Size, downloaded)
		}
		if attempt == maxReadAttempts-1 {
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			return fmt.Errorf("下载读取失败: %w", err)
		}
		if !prepared.SupportsRange {
			downloaded = 0
		}
		if !waitForRetry(ctx, attempt) {
			return context.Cause(ctx)
		}
	}
	return io.ErrUnexpectedEOF
}

func (d *Engine) copyReader(
	ctx context.Context,
	reader io.Reader,
	writer io.Writer,
	expectedSize int64,
	downloaded *int64,
	taskID int,
	resourceID int,
	onProgress func(total, speed int64) error,
) error {
	buf := make([]byte, 32*1024)
	speedSampler := newProgressSpeedSampler(time.Now(), *downloaded)
	lastLog := time.Now()
	lastLogDownloaded := *downloaded
	for {
		chunkStart := time.Now()
		if err := context.Cause(ctx); err != nil {
			_ = onProgress(*downloaded, 0)
			return err
		}
		readBuf := buf
		if expectedSize > 0 {
			remaining := expectedSize - *downloaded
			if remaining == 0 {
				return nil
			}
			if remaining < int64(len(readBuf)) {
				readBuf = readBuf[:remaining]
			}
		}
		n, readErr := reader.Read(readBuf)
		if n > 0 {
			if _, err := writer.Write(readBuf[:n]); err != nil {
				return fmt.Errorf("写入文件失败: %w", err)
			}
			*downloaded += int64(n)
			if d.speedLimit > 0 {
				expected := time.Duration(float64(n) / float64(d.speedLimit) * float64(time.Second))
				elapsed := time.Since(chunkStart)
				if expected > elapsed {
					select {
					case <-ctx.Done():
						return context.Cause(ctx)
					case <-time.After(expected - elapsed):
					}
				}
			}
		}
		now := time.Now()
		// A single Read can finish in a few microseconds. Extrapolating that one
		// 32 KiB block to bytes/second produces meaningless GB/s spikes, so only
		// refresh the displayed speed after a representative sampling window.
		speed := speedSampler.Sample(now, *downloaded)
		if err := onProgress(*downloaded, speed); err != nil {
			return err
		}
		// Progress log every 3 seconds for diagnostics
		if now.Sub(lastLog) >= progressLogInterval {
			if expectedSize > 0 {
				pct := float64(*downloaded) * 100 / float64(expectedSize)
				logSpeed := calcSpeed(lastLog, lastLogDownloaded, now, *downloaded)
				d.logger.Info().
					Int("taskID", taskID).
					Int("resourceID", resourceID).
					Int64("downloaded", *downloaded).
					Int64("totalSize", expectedSize).
					Float64("percent", pct).
					Str("speed", formatSpeed(logSpeed)).
					Msg("download progress")
			} else {
				d.logger.Info().
					Int("taskID", taskID).
					Int("resourceID", resourceID).
					Int64("downloaded", *downloaded).
					Msg("download progress (size unknown)")
			}
			lastLog = now
			lastLogDownloaded = *downloaded
		}
		if readErr != nil {
			if readErr == io.EOF {
				if expectedSize > 0 && *downloaded != expectedSize {
					return io.ErrUnexpectedEOF
				}
				return nil
			}
			return readErr
		}
	}
}

func (d *Engine) downloadSegments(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	filePath string,
	resourceID int,
	fileSize int64,
	segmentCount int,
	taskID int,
) error {
	ranges := splitFile(fileSize, segmentCount)
	segments, err := d.store.LoadSegmentInfo(resourceID)
	if err != nil {
		return fmt.Errorf("加载分片信息失败: %w", err)
	}
	if !segmentsMatchRanges(segments, ranges) {
		ids, err := d.store.CreateSegments(resourceID, endpoint.URL, ranges)
		if err != nil {
			return fmt.Errorf("创建分片记录失败: %w", err)
		}
		if len(ids) != len(ranges) {
			return errors.New("创建分片记录失败: 返回的 ID 数量不正确")
		}
		segments = make([]Segment, len(ranges))
		for i, r := range ranges {
			segments[i] = Segment{ID: ids[i], Index: r.Index, URL: endpoint.URL, OffsetStart: r.OffsetStart, OffsetEnd: r.OffsetEnd, Size: r.Size}
		}
	}

	partPath := filePath + partialFileSuffix
	partInfo, statErr := os.Stat(partPath)
	partValid := statErr == nil && partInfo.Mode().IsRegular() && partInfo.Size() == fileSize
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("检查临时文件失败: %w", statErr)
	}
	// Prefer a completed .part file: an older same-sized destination may still
	// exist if the process stopped between progress persistence and rename.
	if segmentsComplete(segments) && partValid {
		if err := finalizePartialFile(partPath, filePath); err != nil {
			return fmt.Errorf("提交已完成的临时文件失败: %w", err)
		}
		d.updateTracker(taskID, resourceID, fileSize, 0)
		return nil
	}
	// A destination is reusable only when both its size and every persisted
	// segment prove completion. Size alone is unsafe for stale or sparse files.
	if segmentsComplete(segments) && fileHasSize(filePath, fileSize) {
		d.updateTracker(taskID, resourceID, fileSize, 0)
		return nil
	}
	if !partValid {
		for i := range segments {
			if segments[i].Downloaded == 0 {
				continue
			}
			segments[i].Downloaded = 0
			if err := d.store.UpdateSegmentProgress(segments[i].ID, 0); err != nil {
				return fmt.Errorf("重置分片进度失败: %w", err)
			}
		}
	}

	partFile, err := os.OpenFile(partPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("打开临时文件失败: %w", err)
	}
	partClosed := false
	defer func() {
		if !partClosed {
			_ = partFile.Close()
		}
	}()
	if !partValid {
		if err := partFile.Truncate(fileSize); err != nil {
			return fmt.Errorf("预分配临时文件失败: %w", err)
		}
	}

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	progressCh := make(chan segmentProgress, len(segments)*2)
	var wg sync.WaitGroup
	segSem := make(chan struct{}, d.segmentConcurrency)
	for slot, segment := range segments {
		if segment.Downloaded >= segment.Size {
			progressCh <- segmentProgress{slot: slot, downloaded: segment.Size, done: true}
			continue
		}
		wg.Add(1)
		go func(slot int, segment Segment) {
			defer wg.Done()
			select {
			case segSem <- struct{}{}:
			case <-workerCtx.Done():
				return
			}
			defer func() { <-segSem }()
			d.downloadSegment(workerCtx, driver, endpoint, partFile, segment, slot, progressCh)
		}(slot, segment)
	}
	go func() {
		wg.Wait()
		close(progressCh)
	}()

	states := make([]segmentProgress, len(segments))
	for i, segment := range segments {
		states[i].slot = i
		states[i].downloaded = segment.Downloaded
	}
	lastLog := time.Now()
	lastLogDownloaded := int64(0)
	for i := range states {
		lastLogDownloaded += states[i].downloaded
	}
	lastPersist := time.Now()
	var firstErr error
	var progressEventCount int64
	lastProgressEventCount := time.Now()
	lastProgressEventCountN := int64(0)
	for progress := range progressCh {
		progressEventCount++
		if progress.slot < 0 || progress.slot >= len(states) {
			if firstErr == nil {
				firstErr = errors.New("收到无效的分片进度索引")
				cancelWorkers()
			}
			continue
		}
		states[progress.slot] = progress
		if progress.err != nil && firstErr == nil {
			firstErr = progress.err
			cancelWorkers()
		}
		// Always update in-memory tracker for real-time WS progress.
		var totalStateDL, totalStateSpd int64
		for _, s := range states {
			totalStateDL += s.downloaded
			totalStateSpd += s.speed
		}
		d.updateTracker(taskID, resourceID, totalStateDL, totalStateSpd)
		// Throttle DB persistence to progressInterval to avoid excessive writes.
		if time.Since(lastPersist) >= progressInterval || progress.done || progress.err != nil {
			// The partial file is pre-sized, so its length cannot validate which
			// ranges survived a crash. Make the data durable before allowing the
			// persisted per-segment offsets to advance.
			persistErr := partFile.Sync()
			if persistErr == nil {
				persistErr = d.persistAggregate(taskID, resourceID, segments, states)
			}
			if persistErr != nil && firstErr == nil {
				firstErr = persistErr
				cancelWorkers()
			}
			lastPersist = time.Now()
		}
		// Progress log every 3 seconds for diagnostics
		if time.Since(lastLog) >= progressLogInterval {
			var totalDl int64
			for _, s := range states {
				totalDl += s.downloaded
			}
			pct := float64(totalDl) * 100 / float64(fileSize)
			logSpeed := calcSpeed(lastLog, lastLogDownloaded, time.Now(), totalDl)
			// Count how many progress events were received per second in this window
			eventWindow := time.Since(lastProgressEventCount)
			eventsPerSec := float64(progressEventCount-lastProgressEventCountN) / eventWindow.Seconds()
			d.logger.Info().
				Int("taskID", taskID).
				Int("resourceID", resourceID).
				Int64("downloaded", totalDl).
				Int64("totalSize", fileSize).
				Float64("percent", pct).
				Str("speed", formatSpeed(logSpeed)).
				Int("segmentCount", len(segments)).
				Int64("totalEvents", progressEventCount).
				Float64("eventsPerSec", eventsPerSec).
				Msg("segment download progress")
			// Per-slot detail for diagnostics.
			for _, s := range states {
				segSize := segments[s.slot].Size
				var segPct float64
				if segSize > 0 {
					segPct = float64(s.downloaded) * 100 / float64(segSize)
				}
				d.logger.Info().
					Int("slot", s.slot).
					Int64("downloaded", s.downloaded).
					Int64("size", segSize).
					Float64("pct", segPct).
					Str("speed", formatSpeed(s.speed)).
					Bool("done", s.done).
					Msg("seg: detail")
			}
			lastLog = time.Now()
			lastLogDownloaded = totalDl
			lastProgressEventCount = time.Now()
			lastProgressEventCountN = progressEventCount
		}
	}
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	if firstErr != nil {
		return fmt.Errorf("分片下载失败: %w", firstErr)
	}
	for _, state := range states {
		if !state.done {
			return errors.New("分片下载未完整结束")
		}
	}
	// Make data durable before marking all ranges complete. If persistence then
	// succeeds but rename is interrupted, the next run finalizes the .part file.
	if err := partFile.Sync(); err != nil {
		return fmt.Errorf("同步临时文件失败: %w", err)
	}
	if err := d.persistAggregate(taskID, resourceID, segments, states); err != nil {
		return err
	}
	if err := partFile.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	partClosed = true
	if err := os.Rename(partPath, filePath); err != nil {
		return fmt.Errorf("提交下载文件失败: %w", err)
	}
	return nil
}

func segmentsMatchRanges(segments []Segment, ranges []SegmentRange) bool {
	if len(segments) != len(ranges) {
		return false
	}
	for i, segment := range segments {
		r := ranges[i]
		if segment.Index != r.Index || segment.OffsetStart != r.OffsetStart || segment.OffsetEnd != r.OffsetEnd || segment.Size != r.Size || segment.Downloaded < 0 || segment.Downloaded > segment.Size {
			return false
		}
	}
	return true
}

func (d *Engine) downloadSegment(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	file *os.File,
	segment Segment,
	slot int,
	progressCh chan<- segmentProgress,
) {
	downloaded := segment.Downloaded
	speedSampler := newProgressSpeedSampler(time.Now(), downloaded)
	lastSegLog := time.Now()
	lastSegLogDownloaded := downloaded

	for attempt := 0; attempt < maxReadAttempts; attempt++ {
		if err := context.Cause(ctx); err != nil {
			progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: err}
			return
		}
		request := ReadRequest{OffsetStart: segment.OffsetStart + downloaded, OffsetEnd: segment.OffsetEnd, UseRange: true}
		openStart := time.Now()
		d.logger.Info().
			Int("slot", slot).
			Int("attempt", attempt+1).
			Int64("offset", segment.OffsetStart+downloaded).
			Int64("remaining", segment.Size-downloaded).
			Msg("seg: Open() starting")
		reader, err := driver.Open(ctx, endpoint, request)
		openElapsed := time.Since(openStart)
		if err != nil {
			d.logger.Info().
				Int("slot", slot).
				Int("attempt", attempt+1).
				Dur("elapsed", openElapsed).
				Err(err).
				Msg("seg: Open() failed")
			if attempt == maxReadAttempts-1 {
				progressCh <- segmentProgress{slot: slot, downloaded: downloaded, done: true, err: err}
				return
			}
			if !waitForRetry(ctx, attempt) {
				progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
				return
			}
			continue
		}
		d.logger.Info().
			Int("slot", slot).
			Int("attempt", attempt+1).
			Dur("openElapsed", openElapsed).
			Msg("seg: Open() done, reading")

		buf := make([]byte, 32*1024)
		for downloaded < segment.Size {
			chunkStart := time.Now()
			remaining := segment.Size - downloaded
			readBuf := buf
			if remaining < int64(len(readBuf)) {
				readBuf = readBuf[:remaining]
			}
			n, readErr := d.readWithTimeout(reader, readBuf)
			if n > 0 {
				written, err := file.WriteAt(readBuf[:n], segment.OffsetStart+downloaded)
				if err == nil && written != n {
					err = io.ErrShortWrite
				}
				if err != nil {
					reader.Close()
					progressCh <- segmentProgress{slot: slot, downloaded: downloaded, done: true, err: err}
					return
				}
				downloaded += int64(n)
				if d.speedLimit > 0 {
					expected := time.Duration(float64(n) / float64(d.speedLimit) * float64(time.Second))
					elapsed := time.Since(chunkStart)
					if expected > elapsed {
						select {
						case <-ctx.Done():
							progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
							return
						case <-time.After(expected - elapsed):
						}
					}
				}
			}
			now := time.Now()
			// Sample across at least progressInterval. Per-read timings are far too
			// short and can turn a normal 32 KiB read into an apparent multi-GB/s
			// transfer rate.
			speed := speedSampler.Sample(now, downloaded)
			progressCh <- segmentProgress{slot: slot, downloaded: downloaded, speed: speed}
			// Periodic per-segment progress log for diagnostics (every ~1.5s or at 5% increments).
			segPct := float64(downloaded) * 100 / float64(segment.Size)
			if time.Since(lastSegLog) >= 1500*time.Millisecond ||
				int64(segPct) >= int64(float64(lastSegLogDownloaded)*100/float64(segment.Size))+5 {
				segLogSpeed := calcSpeed(lastSegLog, lastSegLogDownloaded, now, downloaded)
				d.logger.Info().
					Int("slot", slot).
					Int("index", segment.Index).
					Int64("dl", downloaded).
					Int64("size", segment.Size).
					Float64("pct", segPct).
					Str("speed", formatSpeed(segLogSpeed)).
					Msg("seg: progress")
				lastSegLog = now
				lastSegLogDownloaded = downloaded
			}
			if readErr != nil {
				reader.Close()
				if errors.Is(readErr, errReadTimeout) {
					d.logger.Info().
						Int("slot", slot).
						Int64("progress", downloaded).
						Int64("total", segment.Size).
						Msg("seg: Read() timeout, will retry")
				} else {
					d.logger.Info().
						Int("slot", slot).
						Err(readErr).
						Int64("progress", downloaded).
						Int64("total", segment.Size).
						Msg("seg: Read() error, will retry")
				}
				if errors.Is(readErr, context.Canceled) || ctx.Err() != nil {
					progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
					return
				}
				break
			}
		}
		reader.Close()
		if downloaded == segment.Size {
			d.logger.Info().
				Int("slot", slot).
				Int64("size", segment.Size).
				Msg("seg: finished")
			progressCh <- segmentProgress{slot: slot, downloaded: downloaded, done: true}
			return
		}
		if attempt < maxReadAttempts-1 && !waitForRetry(ctx, attempt) {
			progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
			return
		}
	}
	progressCh <- segmentProgress{slot: slot, downloaded: downloaded, done: true, err: io.ErrUnexpectedEOF}
}

func fileHasSize(path string, size int64) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() == size
}

func segmentsComplete(segments []Segment) bool {
	if len(segments) == 0 {
		return false
	}
	for _, segment := range segments {
		if segment.Size < 0 || segment.Downloaded != segment.Size {
			return false
		}
	}
	return true
}

func finalizePartialFile(partPath, filePath string) error {
	file, err := os.OpenFile(partPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(partPath, filePath)
}

var errReadTimeout = errors.New("read timeout: CDN connection stalled")

// readWithTimeout performs a single Read with a deadline. If the Read does not
// return within the timeout, the reader is closed to unblock the goroutine and
// errReadTimeout is returned so the caller can retry.
func (d *Engine) readWithTimeout(reader io.Reader, buf []byte) (int, error) {
	type readResult struct {
		n   int
		err error
	}
	done := make(chan readResult, 1)
	go func() {
		n, err := reader.Read(buf)
		done <- readResult{n, err}
	}()

	timer := time.NewTimer(d.readTimeout)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.n, r.err
	case <-timer.C:
		return 0, errReadTimeout
	}
}

func waitForRetry(ctx context.Context, attempt int) bool {
	if attempt >= maxReadAttempts-1 {
		return true
	}
	delay := time.Duration(1<<attempt) * 100 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (d *Engine) persistProgress(taskID, resourceID, segmentID int, downloaded, speed int64) error {
	if err := d.store.UpdateSegmentProgress(segmentID, downloaded); err != nil {
		return fmt.Errorf("更新分片进度失败: %w", err)
	}
	if err := d.updateResourceProgress(taskID, resourceID, downloaded, speed); err != nil {
		return fmt.Errorf("更新任务进度失败: %w", err)
	}
	d.updateTracker(taskID, resourceID, downloaded, speed)
	return nil
}

func (d *Engine) persistAggregate(taskID, resourceID int, segments []Segment, states []segmentProgress) error {
	var totalDownloaded int64
	var totalSpeed int64
	for i, state := range states {
		totalDownloaded += state.downloaded
		totalSpeed += state.speed
		if err := d.store.UpdateSegmentProgress(segments[i].ID, state.downloaded); err != nil {
			return fmt.Errorf("更新分片进度失败: %w", err)
		}
	}
	if err := d.updateResourceProgress(taskID, resourceID, totalDownloaded, totalSpeed); err != nil {
		return fmt.Errorf("更新任务进度失败: %w", err)
	}
	d.updateTracker(taskID, resourceID, totalDownloaded, totalSpeed)
	return nil
}

func (d *Engine) finishTask(taskID int, filePath string, resourceExtensions map[int]string) error {
	// 1. Load task from Store to get full resource info
	info, err := d.store.LoadTask(taskID)
	if err != nil {
		return fmt.Errorf("加载任务信息失败: %w", err)
	}
	if info == nil {
		return errors.New("加载任务信息失败: task is nil")
	}

	// 2. Parse config and metadata
	d.logger.Info().
		Int("taskID", taskID).
		Str("rawConfig", info.Config).
		Str("rawMetadata", info.Metadata).
		Msg("finishTask: raw config and metadata")
	config, metadata := parseConfigAndMetadata(info.Config, info.Metadata)
	// Inject PlatformId from DB row into metadata for postprocessor routing.
	// MetadataJSON may not contain "platform" (it's a column on download_task_v1).
	if _, ok := metadata["platform"]; !ok && info.Platform != "" {
		metadata["platform"] = info.Platform
	}
	d.logger.Info().
		Int("taskID", taskID).
		Interface("config", config).
		Interface("metadata", metadata).
		Msg("finishTask: parsed config and metadata")

	// 3. Build PostprocessInfo with TargetExt from resourceExtensions
	ppResources := make([]PostprocessResource, 0, len(info.Resources))
	for _, r := range info.Resources {
		targetExt := resourceExtensions[r.ID]
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceID", r.ID).
			Str("resourceName", r.Name).
			Str("resourceType", r.ResourceType).
			Str("targetExt", targetExt).
			Msg("building postprocess resource info")
		ppResources = append(ppResources, PostprocessResource{
			ID:           r.ID,
			Name:         r.Name,
			Kind:         r.Kind,
			ResourceType: r.ResourceType,
			Extra:        r.Extra,
			TargetExt:    targetExt,
			FilePath:     d.absFilePath(info.SavePath, r.Name),
		})
	}

	// 4. Rename .tmp files to correct extensions (before postprocessing)
	if err := d.renameTempFiles(info.SavePath, ppResources); err != nil {
		return fmt.Errorf("重命名临时文件失败: %w", err)
	}

	// 5. Update resource names in DB (before postprocessor so assemble_html can read correct names)
	if err := d.persistResourceNames(taskID, ppResources); err != nil {
		d.logger.Warn().Int("taskID", taskID).Err(err).Msg("failed to update resource names")
	}

	// 6. Call postprocessor if set
	if d.postprocessor != nil {
		ppInfo := &PostprocessInfo{
			TaskID:    taskID,
			TaskName:  info.Name,
			SavePath:  info.SavePath,
			Config:    config,
			Metadata:  metadata,
			Resources: ppResources,
		}
		d.logger.Info().Int("taskID", taskID).Msg("starting postprocessing")
		if err := d.postprocessor.Process(context.Background(), ppInfo); err != nil {
			d.logger.Error().Int("taskID", taskID).Err(err).Msg("postprocessing failed")
			d.failTask(taskID, err.Error())
			return fmt.Errorf("后处理失败: %w", err)
		}
		d.logger.Info().Int("taskID", taskID).Msg("postprocessing completed")
	}

	// 6.5 Rename files from unique_id-based names to display names,
	// then apply filename template and hook for the final output name.
	d.finalizeResourceFilenames(info.SavePath, ppResources, config)

	// 7. Update resource names in DB again (postprocessor may have changed them)
	if err := d.persistResourceNames(taskID, ppResources); err != nil {
		d.logger.Warn().Int("taskID", taskID).Err(err).Msg("failed to update resource names (postprocessing)")
	}

	// Rebuild filePaths after possible postprocessing changes
	finalPaths := make([]string, 0, len(ppResources))
	for _, r := range ppResources {
		finalPaths = append(finalPaths, r.FilePath)
	}
	finalFilePath := strings.Join(finalPaths, ", ")

	// 8. Update task status
	d.logger.Info().Int("taskID", taskID).Msg("writing task completion status to DB")
	if err := d.store.FinishTask(taskID); err != nil {
		return fmt.Errorf("完成任务持久化失败: %w", err)
	}
	d.logger.Info().Int("taskID", taskID).Str("filePath", d.relLogPath(finalFilePath)).Msg("download completed")

	// 8. Post-download hook (async, non-blocking)
	if d.hooks != nil && d.hooks.HasFinishHook() {
		go d.invokeFinishHook(taskID, finalFilePath)
	}

	// 9. Emit final progress and EventFinished
	d.emitProgress(taskID)
	d.emit(taskID, EventFinished)
	d.deleteTracker(taskID)
	return nil
}

// renameTempFiles renames .tmp files to their correct extensions.
func (d *Engine) renameTempFiles(savePath string, resources []PostprocessResource) error {
	for i := range resources {
		r := &resources[i]
		if !strings.HasSuffix(r.Name, ".tmp") || r.TargetExt == "" {
			d.logger.Info().
				Int("resourceID", r.ID).
				Str("name", r.Name).
				Str("targetExt", r.TargetExt).
				Msg("renameTempFiles: skipped (not .tmp or no extension)")
			continue
		}
		newName := strings.TrimSuffix(r.Name, ".tmp") + r.TargetExt
		oldPath := d.absFilePath(savePath, r.Name)
		newPath := d.absFilePath(savePath, newName)

		if _, statErr := os.Stat(oldPath); os.IsNotExist(statErr) {
			d.logger.Warn().
			Int("resourceID", r.ID).
			Str("filePath", r.FilePath).
			Msg("temp file does not exist")
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("重命名 %s -> %s: %w", oldPath, newPath, err)
		}
		d.logger.Info().
			Int("resourceID", r.ID).
			Str("oldPath", d.relLogPath(oldPath)).
			Str("newPath", d.relLogPath(newPath)).
			Msg("file renamed")

		r.Name = newName
		r.FilePath = newPath
	}
	return nil
}

// finalizeResourceFilenames renames downloaded files from unique_id-based names
// to human-readable display names (from Extra["title"]), then applies filename
// template and hooks to produce the final output filename. This is done after
// download and postprocessing so that templates/hooks work on clean display names
// instead of internal unique IDs.
func (d *Engine) finalizeResourceFilenames(savePath string, resources []PostprocessResource, config map[string]any) {
	for i := range resources {
		r := &resources[i]
		title, ok := r.Extra["title"]
		if !ok || title == "" {
			continue
		}
		ext := r.TargetExt
		if ext == "" {
			ext = filepath.Ext(r.Name)
		}

		// Start with the display title as the base name
		baseName := title

		// Apply filename template using display title as {{filename}}
		if d.filenameTemplate != "" {
			meta := buildTemplateMeta(r.Extra, config, baseName)
			task := &Task{
				Name:             baseName,
				FilenameTemplate: d.filenameTemplate,
			}
			if newName := d.applyFilenameTemplate(task, "", meta); newName != "" {
				baseName = newName
			}
		}

		// Apply filename hook
		if d.hooks != nil && d.hooks.HasFilenameHook() {
			hookMeta := buildResourceMeta(r.Extra, config)
			params := &FilenameParams{
				Meta: hookMeta,
				Task: TaskInfo{
					Name:     baseName,
					SavePath: savePath,
					Config:   config,
				},
				Config: config,
			}
			if newName, err := d.hooks.InvokeFilenameHook(params, baseName); err == nil && newName != "" {
				baseName = newName
			}
		}

		// Sanitize each path component and build final name
		finalName := sanitizePathComponents(baseName) + ext

		// Handle duplicate: when duplicate mode is on and target file already
		// exists, append (1), (2), ... to avoid overwriting.
		if dup, ok := config["duplicate"]; ok {
			switch v := dup.(type) {
			case bool:
				if v {
					finalName = d.resolveDuplicateFilename(savePath, finalName, ext)
				}
			}
		}

		if finalName == r.Name {
			continue
		}

		oldPath := d.absFilePath(savePath, r.Name)
		newPath := d.absFilePath(savePath, finalName)
		if oldPath == newPath {
			continue
		}

		// Ensure parent directories exist
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			d.logger.Warn().
				Int("resourceID", r.ID).
				Str("dir", filepath.Dir(newPath)).
				Err(err).
				Msg("failed to create directory for final filename")
			continue
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			d.logger.Warn().
				Int("resourceID", r.ID).
				Str("oldPath", oldPath).
				Str("newPath", newPath).
				Err(err).
				Msg("rename to final filename failed")
			continue
		}
		d.logger.Info().
			Int("resourceID", r.ID).
			Str("oldName", r.Name).
			Str("newName", finalName).
			Msg("final filename applied")
		r.Name = finalName
		r.FilePath = newPath
	}
}

// sanitizePathComponents sanitizes each path component (separated by /),
// replacing characters unsafe for filenames.
func sanitizePathComponents(path string) string {
	parts := strings.Split(path, "/")
	replacer := strings.NewReplacer(
		"\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	for i, p := range parts {
		parts[i] = strings.TrimSpace(replacer.Replace(p))
	}
	return strings.Trim(strings.Join(parts, "/"), "/")
}

// persistResourceNames updates resource names in the database.
func (d *Engine) persistResourceNames(taskID int, resources []PostprocessResource) error {
	store, ok := d.store.(OutputNameStore)
	if !ok {
		return nil
	}
	for _, r := range resources {
		update := OutputNameUpdate{
			TaskID:       taskID,
			ResourceID:   r.ID,
			ResourceName: r.Name,
		}
		d.logger.Info().
			Int("taskID", taskID).
			Int("resourceID", r.ID).
			Str("resourceName", r.Name).
			Msg("updating resource name in DB")
		if err := store.UpdateOutputName(update); err != nil {
			return fmt.Errorf("更新资源名失败 resource_id=%d: %w", r.ID, err)
		}
	}
	return nil
}

func (d *Engine) invokeFinishHook(taskID int, filePathsStr string) {
	info, err := d.store.LoadTask(taskID)
	if err != nil {
		d.logger.Warn().Int("taskID", taskID).Err(err).Msg("failed to load task info, skipping finish hook")
		return
	}

	filePaths := strings.Split(filePathsStr, ", ")

	resources := make([]ResourceInfo, 0, len(info.Resources))
	for _, r := range info.Resources {
		endpoints := make([]EndpointInfo, 0, len(r.Endpoints))
		for _, e := range r.Endpoints {
			endpoints = append(endpoints, EndpointInfo{
				Protocol: e.Protocol,
				URL:      e.URL,
			})
		}
		resources = append(resources, ResourceInfo{
			ID:        r.ID,
			Name:      r.Name,
			Kind:      r.ResourceType,
			Extra:     r.Extra,
			Endpoints: endpoints,
		})
	}

	config, metadata := parseConfigAndMetadata(info.Config, info.Metadata)

	ctx := &FinishContext{
		Task: TaskInfo{
			Name:     info.Name,
			SavePath: info.SavePath,
			Config:   config,
		},
		Config:    config,
		Metadata:  metadata,
		Resources: resources,
		FilePaths: filePaths,
		SavePath:  info.SavePath,
	}

	if err := d.hooks.InvokeFinishHook(ctx); err != nil {
		d.logger.Warn().Err(err).Msg("finish hook execution failed")
	}
}

func (d *Engine) pauseTask(taskID int) {
	_ = d.store.UpdateStatus(taskID, TaskStatusPaused)
	_ = d.store.DeactivateConnections(taskID)
	d.logger.Info().Int("taskID", taskID).Msg("task paused")
	d.emit(taskID, EventPaused)
	d.deleteTracker(taskID)
}

func (d *Engine) failTask(taskID int, errMsg string) {
	_ = d.store.UpdateStatus(taskID, TaskStatusFailed)
	_ = d.store.DeactivateConnections(taskID)
	_ = d.store.RecordError(taskID, errMsg)
	d.logger.Error().Int("taskID", taskID).Str("error", errMsg).Msg("task failed")
	d.emit(taskID, EventFailed)
	d.deleteTracker(taskID)
}

func (d *Engine) emit(taskID int, event EventType) {
	d.eventMu.RLock()
	handler := d.onEvent
	d.eventMu.RUnlock()
	if handler != nil {
		handler(taskID, event, nil)
	}
}

// emitProgress emits an EventProgress event with the current in-memory
// progress snapshot, avoiding any database round-trip.
func (d *Engine) emitProgress(taskID int) {
	p := d.snapshotProgress(taskID)
	if p == nil {
		d.logger.Info().Int("taskID", taskID).Msg("progress: skip emit (no change)")
		return
	}
	msg := "progress: emit to handler"
	if p.Keepalive {
		msg = "progress: emit keepalive"
	}
	d.logger.Info().
		Int("taskID", taskID).
		Int64("downloaded", p.Downloaded).
		Int64("totalSize", p.TotalSize).
		Int64("speed", p.Speed).
		Float64("pct", float64(p.Downloaded)*100/float64(maxInt64(1, p.TotalSize))).
		Msg(msg)
	d.eventMu.RLock()
	handler := d.onEvent
	d.eventMu.RUnlock()
	if handler != nil {
		handler(taskID, EventProgress, p)
	}
}

// snapshotProgress builds a TaskProgress from the in-memory tracker.
// Speed values come from the download loop (copyReader/downloadSegment) which
// computes speed every progressInterval (500ms). The periodic ticker decouples
// emission frequency from the download loop, making it independently configurable
// via ProgressEmitInterval without losing speed accuracy.
// Returns nil if no progress tracker exists for the given task.
func (d *Engine) snapshotProgress(taskID int) *TaskProgress {
	d.progressMu.Lock()
	tracker, ok := d.progressCache[taskID]
	d.progressMu.Unlock()
	if !ok {
		return nil
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	var totalDownloaded int64
	var totalSpeed int64
	p := &TaskProgress{
		Resources: make([]ResourceProgress, 0, len(tracker.resources)),
	}
	for _, rID := range tracker.order {
		r := tracker.resources[rID]
		p.TotalSize += r.size
		p.Downloaded += r.downloaded
		p.Speed += r.speed
		totalDownloaded += r.downloaded
		totalSpeed += r.speed
		p.ResourceCount++
		p.Resources = append(p.Resources, ResourceProgress{
			ID:           rID,
			Name:         r.name,
			Kind:         r.kind,
			ResourceType: r.resourceType,
			Size:         r.size,
			Downloaded:   r.downloaded,
			Speed:        r.speed,
		})
	}
	// Skip emission if downloaded and speed haven't changed since last broadcast.
	// This prevents duplicate WS pushes when the progress emit interval (180ms) is
	// shorter than the segment progress reporting interval (500ms).
	// However, if >500ms has passed since the last real emission, emit anyway so
	// the frontend knows the download is still alive (e.g. during segment
	// connection establishment which can take 1-5 seconds).
	isKeepalive := false
	if totalDownloaded == tracker.lastEmitDownloaded && totalSpeed == tracker.lastEmitSpeed && totalDownloaded != p.TotalSize {
		if time.Since(tracker.lastEmitTime) < 500*time.Millisecond {
			return nil
		}
		isKeepalive = true
	}
	p.Keepalive = isKeepalive
	tracker.lastEmitDownloaded = totalDownloaded
	tracker.lastEmitSpeed = totalSpeed
	tracker.lastEmitTime = time.Now()
	return p
}

// initTracker creates a new progress tracker for the given task.
func (d *Engine) initTracker(taskID int, resourceSizes map[int]int64, resources []Resource) {
	tracker := &progressTracker{
		resources: make(map[int]*resourceTracker),
		order:     make([]int, 0, len(resources)),
	}
	for _, r := range resources {
		sz := resourceSizes[r.ID]
		tracker.resources[r.ID] = &resourceTracker{
			size:         sz,
			name:         r.Name,
			kind:         r.Kind,
			resourceType: r.ResourceType,
		}
		tracker.order = append(tracker.order, r.ID)
	}
	d.progressMu.Lock()
	d.progressCache[taskID] = tracker
	d.progressMu.Unlock()
}

// updateTracker updates a resource's downloaded bytes and speed in the in-memory tracker.
// speed comes from the download loop (copyReader/downloadSegment) computed at progressInterval (500ms).
func (d *Engine) updateTracker(taskID, resourceID int, downloaded, speed int64) {
	d.progressMu.Lock()
	tracker, ok := d.progressCache[taskID]
	d.progressMu.Unlock()
	if !ok {
		return
	}
	tracker.mu.Lock()
	if r, ok := tracker.resources[resourceID]; ok {
		r.downloaded = downloaded
		r.speed = speed
	}
	tracker.mu.Unlock()
}

// updateTrackerSize updates a resource's total size in the in-memory tracker.
// Called when Prepare discovers the actual file size during download, ensuring
// the WebSocket progress push reflects accurate size even for single-resource tasks.
func (d *Engine) updateTrackerSize(taskID, resourceID int, size int64) {
	d.progressMu.Lock()
	tracker, ok := d.progressCache[taskID]
	d.progressMu.Unlock()
	if !ok {
		return
	}
	tracker.mu.Lock()
	if r, ok := tracker.resources[resourceID]; ok {
		r.size = size
	}
	tracker.mu.Unlock()
}

// deleteTracker removes the progress tracker for a finished/failed task.
func (d *Engine) deleteTracker(taskID int) {
	d.progressMu.Lock()
	delete(d.progressCache, taskID)
	d.progressMu.Unlock()
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// buildResourceMeta builds a simplified ResourceMeta from resource.Extra and task config.
// resource.Extra is populated by each platform's BuildDownloadTask; fields vary by platform.
func buildResourceMeta(extra map[string]string, config map[string]any) ResourceMeta {
	meta := ResourceMeta{
		DownloadAt: time.Now().Unix(),
	}
	if extra != nil {
		meta.ID = extra["id"]
		meta.Title = extra["title"]
		meta.Spec = extra["spec"]
		meta.Author = extra["author"]
		if v, err := strconv.ParseInt(extra["created_at"], 10, 64); err == nil {
			meta.CreatedAt = v
		}
	}
	if config != nil {
		if platform, ok := config["platform"].(string); ok {
			meta.Platform = platform
		}
	}
	return meta
}

// buildTemplateMeta builds a metadata map for {{var}} template substitution from resource.Extra and task config.
func buildTemplateMeta(extra map[string]string, config map[string]any, currentName string) map[string]string {
	meta := make(map[string]string)
	meta["download_at"] = time.Now().Format("2006-01-02")
	meta["filename"] = currentName
	if extra != nil {
		meta["id"] = extra["id"]
		meta["title"] = extra["title"]
		meta["spec"] = extra["spec"]
		meta["author"] = extra["author"]
		meta["created_at"] = extra["created_at"]
	}
	// User config spec overrides resource metadata spec
	if config != nil {
		if spec, ok := config["spec"].(string); ok && spec != "" {
			meta["spec"] = spec
		}
	}
	return meta
}

// parseConfigAndMetadata parses download config and content metadata JSON.
// Returns the merged config map and separate metadata map.
func parseConfigAndMetadata(configJSON, metadataJSON string) (map[string]any, map[string]any) {
	result := make(map[string]any)
	meta := make(map[string]any)
	if configJSON != "" {
		json.Unmarshal([]byte(configJSON), &result)
	}
	if metadataJSON != "" {
		if json.Unmarshal([]byte(metadataJSON), &meta) == nil {
			for k, v := range meta {
				if _, exists := result[k]; !exists {
					result[k] = v
				}
			}
		}
	}
	// Protect against json.Unmarshal setting maps to nil when input is "null".
	if result == nil {
		result = make(map[string]any)
	}
	if meta == nil {
		meta = make(map[string]any)
	}
	return result, meta
}

// calcSpeed computes download speed (bytes/sec) between two points in time.
func calcSpeed(t0 time.Time, downloaded0 int64, t1 time.Time, downloaded1 int64) int64 {
	elapsed := t1.Sub(t0).Seconds()
	if elapsed <= 0 || downloaded1 <= downloaded0 {
		return 0
	}
	return int64(float64(downloaded1-downloaded0) / elapsed)
}

// progressSpeedSampler keeps the most recent speed calculated across a stable
// sampling window. Reads commonly complete in microseconds, so calculating a
// rate from every individual read greatly exaggerates normal burstiness.
type progressSpeedSampler struct {
	sampledAt  time.Time
	downloaded int64
	speed      int64
}

func newProgressSpeedSampler(now time.Time, downloaded int64) *progressSpeedSampler {
	return &progressSpeedSampler{sampledAt: now, downloaded: downloaded}
}

func (s *progressSpeedSampler) Sample(now time.Time, downloaded int64) int64 {
	if now.Sub(s.sampledAt) < progressInterval {
		return s.speed
	}
	s.speed = calcSpeed(s.sampledAt, s.downloaded, now, downloaded)
	s.sampledAt = now
	s.downloaded = downloaded
	return s.speed
}

// formatSpeed formats a speed value (bytes/sec) into a human-readable string.
func formatSpeed(bytesPerSec int64) string {
	if bytesPerSec <= 0 {
		return "0 B/s"
	}
	// Keep it simple: KB/s or MB/s
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", float64(bytesPerSec)/(1024*1024))
	}
	return fmt.Sprintf("%.0f KB/s", float64(bytesPerSec)/1024)
}

// formatSize formats a byte count into a human-readable string.
func formatSize(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// getConfigBool reads the value of a specified boolean key from the task's config JSON.
func getConfigBool(configJSON, key string) bool {
	if configJSON == "" {
		return false
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return false
	}
	if v, ok := cfg[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// getConfigString reads the value for the specified string key from the task's config JSON.
func getConfigString(configJSON, key string) string {
	if configJSON == "" {
		return ""
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return ""
	}
	if v, ok := cfg[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// findNextDuplicateName finds the next available numeric suffix filename when a file already exists.
// e.g., if baseName.mp4 exists, try baseName(1).mp4, baseName(2).mp4, ...
func (d *Engine) findNextDuplicateName(task *Task, existingPath, dir, baseName, ext string) string {
	tmpExt := ".tmp"
	for counter := 1; ; counter++ {
		candidate := fmt.Sprintf("%s(%d)%s", baseName, counter, tmpExt)
		candidatePath := d.absFilePath(task.SavePath, filepath.Join(dir, candidate))
		if _, err := os.Stat(candidatePath); os.IsNotExist(err) {
			return dir + candidate
		}
		d.logger.Info().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Str("fileName", dir+candidate).
			Msg("duplicate file name exists, incrementing counter")
	}
}

// resolveDuplicateFilename appends (1), (2), ... to the filename when a file
// with the same name already exists on disk, to avoid overwriting.
func (d *Engine) resolveDuplicateFilename(savePath, finalName, ext string) string {
	baseWithExt := finalName
	for counter := 1; ; counter++ {
		candidatePath := d.absFilePath(savePath, baseWithExt)
		if _, err := os.Stat(candidatePath); os.IsNotExist(err) {
			return baseWithExt
		}
		baseWithoutExt := strings.TrimSuffix(baseWithExt, ext)
		baseWithExt = fmt.Sprintf("%s(%d)%s", baseWithoutExt, counter, ext)
	}
}

// persistResourceName updates the resource name in the database. 'reason' is used in logs to annotate the trigger scenario.
func (d *Engine) persistResourceName(task *Task, resourceName, originalDBName, reason string) (bool, error) {
	if resourceName == originalDBName {
		d.logger.Info().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Str("reason", reason).
			Msg("filename matches DB, skipping DB update")
		return false, nil
	}
	update := OutputNameUpdate{
		TaskID:       task.ID,
		ResourceID:   task.ResourceID,
		ResourceName: resourceName,
	}
	d.logger.Info().
		Int("taskID", task.ID).
		Int("resourceID", task.ResourceID).
		Str("reason", reason).
		Int("updateTaskID", update.TaskID).
		Int("updateResourceID", update.ResourceID).
		Str("updateResourceName", update.ResourceName).
		Msg("updating resource name in DB")
	if store, ok := d.store.(OutputNameStore); ok {
		if err := store.UpdateOutputName(update); err != nil {
			return false, fmt.Errorf("更新下载文件名到数据库失败: %w", err)
		}
		d.logger.Info().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Str("oldName", originalDBName).
			Str("newName", resourceName).
			Msg("resource name updated in DB")
	} else {
		d.logger.Warn().
			Int("taskID", task.ID).
			Int("resourceID", task.ResourceID).
			Msg("store does not implement OutputNameStore, skipping DB update")
	}
	return true, nil
}
