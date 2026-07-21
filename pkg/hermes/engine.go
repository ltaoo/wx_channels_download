package hermes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// EventType 下载器事件类型。
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

// 任务状态值与持久化层的 download_task_v1 状态保持稳定映射。
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
	defaultSegmentCount = 10
	minimumSegmentSize  = int64(1024 * 1024)
	progressInterval    = 500 * time.Millisecond
	maxReadAttempts     = 3
)

// Endpoint 是协议驱动需要的下载源信息。Headers 和 Cookies 仅传给驱动，
// 不会被写入日志或事件。
type Endpoint struct {
	ID       int
	Protocol string
	URL      string
	Priority int
	Headers  map[string]string
	Cookies  string
}

// Task 下载器需要的任务信息（由外部通过 LoadTask 提供）。
// URL 和 ResourceID 保留用于兼容旧的 store；新实现优先使用 Endpoints。
type Task struct {
	ID           int
	Name         string
	SavePath     string
	ResourceType string
	URL          string
	ResourceID   int
	Endpoints    []Endpoint
	Resources    []Resource
}

// Resource 是 Task 中可独立下载的文件资源。
type Resource struct {
	ID        int
	Name      string
	Endpoints []Endpoint
}

// SegmentRange 是协议无关的有限字节范围，两端均包含。
type SegmentRange struct {
	Index       int
	OffsetStart int64
	OffsetEnd   int64
	Size        int64
}

// Segment 是 Store 中可恢复的统一分片状态。
type Segment struct {
	ID          int
	Index       int
	URL         string
	OffsetStart int64
	OffsetEnd   int64
	Size        int64
	Downloaded  int64
}

// EventHandler 接收任务生命周期和进度事件。
type EventHandler func(taskID int, event EventType)

type segmentProgress struct {
	slot       int
	downloaded int64
	speed      int64
	done       bool
	err        error
}

// PreparedResource 是驱动探测得到的协议无关资源信息。
type PreparedResource struct {
	Size          int64
	SupportsRange bool
	ContentType   string
}

// ReadRequest 描述 Writer 希望从协议驱动读取的范围。
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

// ProtocolDriver 只负责连接、认证、探测和读取；文件布局、并发和状态机由下载器负责。
// 新协议注册驱动即可接入调度器，不应在驱动中直接写目标文件。
type ProtocolDriver interface {
	Protocols() []string
	Prepare(ctx context.Context, endpoint Endpoint) (PreparedResource, error)
	Open(ctx context.Context, endpoint Endpoint, request ReadRequest) (io.ReadCloser, error)
}

// Store 隔离下载执行层与数据库。
type Store interface {
	LoadTask(taskID int) (*Task, error)
	UpdateStatus(taskID int, status int) error
	ActivateTask(taskID int) error
	UpdateProgress(taskID int, downloaded int64, speed int64) error
	UpdateResourceSize(taskID int, size int64) error
	DeactivateConnections(taskID int) error
	FinishTask(taskID int) error
	WriteLog(taskID int, level string, message string) error
	CreateSegments(resourceID int, url string, ranges []SegmentRange) ([]int, error)
	LoadSegmentInfo(resourceID int) ([]Segment, error)
	UpdateSegmentProgress(segID int, downloaded int64) error
}

// ResourceStore 为多资源任务提供精确到资源的进度更新能力。
// 未实现时 Engine 回退到 Store 的任务级更新方法。
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

// Engine 是协议无关的有限资源下载调度器。
// FILE 和 COLLECTION 由同一任务调度，STREAM 由录制调度器处理。
type Engine struct {
	mu            sync.Mutex
	maxConcurrent int
	sem           chan struct{}
	jobs          map[int]*job
	store         Store
	onEvent       EventHandler
	drivers       map[string]ProtocolDriver
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

func New(store Store, onEvent EventHandler, maxConcurrent int) *Engine {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	d := &Engine{
		maxConcurrent: maxConcurrent,
		sem:           make(chan struct{}, maxConcurrent),
		jobs:          make(map[int]*job),
		store:         store,
		onEvent:       onEvent,
		drivers:       make(map[string]ProtocolDriver),
	}
	return d
}

// RegisterProtocol 注册或替换协议驱动。协议名不区分大小写。
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

// Start 将任务交给调度器。获取并发槽位在后台进行，因此队列不会阻塞 API 请求。
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
	go d.schedule(taskID, job)
	return nil
}

// Pause 取消并等待当前执行实例退出，保证随后 Resume 不会与旧 Writer 同时写文件。
func (d *Engine) Pause(taskID int) {
	if job := d.findJob(taskID); job != nil {
		job.stop(cancelPause)
		<-job.done
	}
}

// PauseAll 暂停所有进行中或排队中的下载。
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

// Delete 停止执行实例。数据库实体的取消和删除仍由 API handler 负责。
func (d *Engine) Delete(taskID int) {
	if job := d.findJob(taskID); job != nil {
		job.stop(cancelDelete)
		<-job.done
		d.emit(taskID, EventDeleted)
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
	if info.ResourceType != "" && info.ResourceType != ResourceTypeFile && info.ResourceType != ResourceTypeCollection {
		return fmt.Errorf("Hermes 暂不支持资源类型 %s", info.ResourceType)
	}
	resources := info.Resources
	if len(resources) == 0 {
		endpoints := info.Endpoints
		if len(endpoints) == 0 && strings.TrimSpace(info.URL) != "" {
			endpoints = []Endpoint{{URL: info.URL}}
		}
		resources = []Resource{{ID: info.ResourceID, Name: info.Name, Endpoints: endpoints}}
	}
	if len(resources) == 0 {
		return errors.New("任务没有可下载资源")
	}

	if err := d.store.UpdateStatus(taskID, TaskStatusDownloading); err != nil {
		return fmt.Errorf("更新下载状态失败: %w", err)
	}
	if err := d.store.ActivateTask(taskID); err != nil {
		return fmt.Errorf("激活任务失败: %w", err)
	}
	d.emit(taskID, EventStarted)

	filePaths := make([]string, 0, len(resources))
	for _, resource := range resources {
		filePath, err := d.downloadResource(ctx, taskID, info.SavePath, info.ResourceType, resource)
		if err != nil {
			return fmt.Errorf("资源 %s 下载失败: %w", resource.Name, err)
		}
		filePaths = append(filePaths, filePath)
	}
	return d.finishTask(taskID, strings.Join(filePaths, ", "))
}

func (d *Engine) downloadResource(ctx context.Context, taskID int, savePath string, resourceType string, resource Resource) (string, error) {
	resourceTask := &Task{
		ID:           taskID,
		Name:         resource.Name,
		SavePath:     savePath,
		ResourceType: resourceType,
		ResourceID:   resource.ID,
		Endpoints:    resource.Endpoints,
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
		}
		nameUpdated, err := d.applyContentTypeFilename(resourceTask, candidate.endpoint.URL, prepared)
		if err != nil {
			return "", err
		}
		if nameUpdated {
			d.emit(taskID, EventProgress)
		}

		filePath, err = taskFilePath(resourceTask, candidate.endpoint.URL)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return "", fmt.Errorf("创建下载目录失败: %w", err)
		}

		segmentCount := chooseSegmentCount(prepared)
		if segmentCount > 1 {
			err = d.downloadSegments(ctx, candidate.driver, candidate.endpoint, filePath, resource.ID, prepared.Size, segmentCount, taskID)
		} else {
			err = d.downloadFile(ctx, candidate.driver, candidate.endpoint, filePath, resource.ID, prepared, taskID)
		}
		if err == nil {
			if prepared.Size <= 0 {
				if fileInfo, statErr := os.Stat(filePath); statErr == nil {
					if err := d.updateResourceSize(taskID, resource.ID, fileInfo.Size()); err != nil {
						return "", fmt.Errorf("更新资源最终大小失败: %w", err)
					}
				}
			}
			if store, ok := d.store.(ResourceStore); ok {
				if err := store.FinishResource(resource.ID); err != nil {
					return "", fmt.Errorf("完成资源持久化失败: %w", err)
				}
				d.emit(taskID, EventProgress)
			}
			return filePath, nil
		}
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return "", context.Cause(ctx)
		}
		endpointErrors = append(endpointErrors, fmt.Sprintf("%s: %v", candidate.protocol, err))
		_ = d.store.WriteLog(taskID, "warn", fmt.Sprintf("下载端点 %d 失败，尝试下一个镜像: %v", candidate.endpoint.ID, err))
	}
	return "", fmt.Errorf("所有下载端点均不可用: %s", strings.Join(endpointErrors, "; "))
}

func (d *Engine) applyContentTypeFilename(task *Task, endpointURL string, prepared PreparedResource) (bool, error) {
	if task == nil || filepath.Ext(strings.TrimSpace(task.Name)) != "" {
		return false, nil
	}
	extension := extensionForContentType(prepared.ContentType)
	if extension == "" {
		return false, nil
	}
	if task.ResourceID > 0 {
		segments, err := d.store.LoadSegmentInfo(task.ResourceID)
		if err != nil {
			return false, fmt.Errorf("读取已有下载分片失败: %w", err)
		}
		if len(segments) > 0 {
			return false, nil
		}
	}
	currentPath, err := taskFilePath(task, endpointURL)
	if err != nil {
		return false, err
	}
	if fileInfo, err := os.Stat(currentPath); err == nil && fileInfo.Size() > 0 {
		return false, nil
	}

	previousName := task.Name
	resourceName, err := NewFilenameProcessor("", nil).AppendExtension(previousName, extension)
	if err != nil {
		return false, fmt.Errorf("生成下载文件名失败: %w", err)
	}
	update := OutputNameUpdate{
		TaskID:       task.ID,
		ResourceID:   task.ResourceID,
		ResourceName: resourceName,
	}
	newSavePath := task.SavePath
	if strings.EqualFold(task.ResourceType, ResourceTypeFile) {
		newSavePath = filepath.Join(filepath.Dir(currentPath), resourceName)
		update.TaskName = resourceName
		update.SavePath = newSavePath
	}
	if store, ok := d.store.(OutputNameStore); ok {
		if err := store.UpdateOutputName(update); err != nil {
			return false, fmt.Errorf("更新下载文件名失败: %w", err)
		}
	}
	task.Name = resourceName
	task.SavePath = newSavePath
	return true, nil
}

func extensionForContentType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	switch strings.ToLower(mediaType) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/avif":
		return ".avif"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "video/x-msvideo":
		return ".avi"
	case "video/x-matroska":
		return ".mkv"
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4":
		return ".m4a"
	case "audio/aac":
		return ".aac"
	case "audio/ogg":
		return ".ogg"
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
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

func (d *Engine) endpointCandidates(info *Task) ([]endpointCandidate, error) {
	endpoints := append([]Endpoint(nil), info.Endpoints...)
	if len(endpoints) == 0 && strings.TrimSpace(info.URL) != "" {
		endpoints = []Endpoint{{URL: info.URL}}
	}
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
	// 新任务的 FILE SavePath 是完整文件路径；保留对目录形式的兼容。
	if strings.EqualFold(info.ResourceType, "FILE") && filepath.Base(savePath) == name {
		return savePath, nil
	}
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

// splitFile 将文件等分为 n 个非空分片，余数分散到前面的分片中。
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
	if len(segments) != 1 || segments[0].OffsetStart != 0 || segments[0].Size != prepared.Size {
		ids, err := d.store.CreateSegments(resourceID, endpoint.URL, ranges)
		if err != nil {
			return fmt.Errorf("创建分片记录失败: %w", err)
		}
		if len(ids) != 1 {
			return errors.New("创建分片记录失败: 返回的 ID 数量不正确")
		}
		segments = []Segment{{ID: ids[0], Index: 0, URL: endpoint.URL, Size: prepared.Size, OffsetEnd: ranges[0].OffsetEnd}}
	}

	var downloaded int64
	if fi, statErr := os.Stat(filePath); statErr == nil {
		downloaded = fi.Size()
	}
	if prepared.Size > 0 && downloaded == prepared.Size {
		if err := d.persistProgress(taskID, resourceID, segments[0].ID, downloaded, 0); err != nil {
			return err
		}
		return nil
	}
	if !prepared.SupportsRange || downloaded < 0 || (prepared.Size > 0 && downloaded > prepared.Size) {
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
		file, openErr := os.OpenFile(filePath, flags, 0644)
		if openErr != nil {
			reader.Close()
			return fmt.Errorf("打开目标文件失败: %w", openErr)
		}

		err = d.copyReader(ctx, reader, file, prepared.Size, &downloaded, func(total, speed int64) error {
			return d.persistProgress(taskID, resourceID, segments[0].ID, total, speed)
		})
		closeErr := errors.Join(reader.Close(), file.Close())
		if err == nil {
			err = closeErr
		}
		if err == nil && (prepared.Size <= 0 || downloaded == prepared.Size) {
			return d.persistProgress(taskID, resourceID, segments[0].ID, downloaded, 0)
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
	onProgress func(total, speed int64) error,
) error {
	buf := make([]byte, 32*1024)
	lastProgress := time.Now()
	lastDownloaded := *downloaded
	for {
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
		}
		now := time.Now()
		if now.Sub(lastProgress) >= progressInterval || readErr != nil {
			elapsed := now.Sub(lastProgress).Seconds()
			var speed int64
			if elapsed > 0 {
				speed = int64(float64(*downloaded-lastDownloaded) / elapsed)
			}
			if err := onProgress(*downloaded, speed); err != nil {
				return err
			}
			lastProgress = now
			lastDownloaded = *downloaded
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

	fileValid := false
	if fi, statErr := os.Stat(filePath); statErr == nil && fi.Size() == fileSize {
		fileValid = true
	}
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("打开目标文件失败: %w", err)
	}
	defer file.Close()
	if err := file.Truncate(fileSize); err != nil {
		return fmt.Errorf("预分配文件大小失败: %w", err)
	}
	if !fileValid {
		for i := range segments {
			segments[i].Downloaded = 0
			if err := d.store.UpdateSegmentProgress(segments[i].ID, 0); err != nil {
				return fmt.Errorf("重置分片进度失败: %w", err)
			}
		}
	}

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	progressCh := make(chan segmentProgress, len(segments)*2)
	var wg sync.WaitGroup
	for slot, segment := range segments {
		if segment.Downloaded >= segment.Size {
			progressCh <- segmentProgress{slot: slot, downloaded: segment.Size, done: true}
			continue
		}
		wg.Add(1)
		go func(slot int, segment Segment) {
			defer wg.Done()
			d.downloadSegment(workerCtx, driver, endpoint, file, segment, slot, progressCh)
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
	var firstErr error
	for progress := range progressCh {
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
		if err := d.persistAggregate(taskID, resourceID, segments, states); err != nil && firstErr == nil {
			firstErr = err
			cancelWorkers()
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
	return d.persistAggregate(taskID, resourceID, segments, states)
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
	lastProgress := time.Now()
	lastDownloaded := downloaded

	for attempt := 0; attempt < maxReadAttempts; attempt++ {
		if err := context.Cause(ctx); err != nil {
			progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: err}
			return
		}
		request := ReadRequest{OffsetStart: segment.OffsetStart + downloaded, OffsetEnd: segment.OffsetEnd, UseRange: true}
		reader, err := driver.Open(ctx, endpoint, request)
		if err != nil {
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

		buf := make([]byte, 32*1024)
		for downloaded < segment.Size {
			remaining := segment.Size - downloaded
			readBuf := buf
			if remaining < int64(len(readBuf)) {
				readBuf = readBuf[:remaining]
			}
			n, readErr := reader.Read(readBuf)
			if n > 0 {
				if _, err := file.WriteAt(readBuf[:n], segment.OffsetStart+downloaded); err != nil {
					reader.Close()
					progressCh <- segmentProgress{slot: slot, downloaded: downloaded, done: true, err: err}
					return
				}
				downloaded += int64(n)
			}
			now := time.Now()
			if now.Sub(lastProgress) >= progressInterval || readErr != nil || downloaded == segment.Size {
				elapsed := now.Sub(lastProgress).Seconds()
				var speed int64
				if elapsed > 0 {
					speed = int64(float64(downloaded-lastDownloaded) / elapsed)
				}
				progressCh <- segmentProgress{slot: slot, downloaded: downloaded, speed: speed}
				lastProgress = now
				lastDownloaded = downloaded
			}
			if readErr != nil {
				reader.Close()
				if errors.Is(readErr, context.Canceled) || ctx.Err() != nil {
					progressCh <- segmentProgress{slot: slot, downloaded: downloaded, err: context.Cause(ctx)}
					return
				}
				break
			}
		}
		reader.Close()
		if downloaded == segment.Size {
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
	d.emit(taskID, EventProgress)
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
	d.emit(taskID, EventProgress)
	return nil
}

func (d *Engine) finishTask(taskID int, filePath string) error {
	if err := d.store.FinishTask(taskID); err != nil {
		return fmt.Errorf("完成任务持久化失败: %w", err)
	}
	_ = d.store.WriteLog(taskID, "info", fmt.Sprintf("下载完成, 文件: %s", filePath))
	d.emit(taskID, EventFinished)
	return nil
}

func (d *Engine) pauseTask(taskID int) {
	_ = d.store.UpdateStatus(taskID, TaskStatusPaused)
	_ = d.store.DeactivateConnections(taskID)
	d.emit(taskID, EventPaused)
}

func (d *Engine) failTask(taskID int, errMsg string) {
	_ = d.store.UpdateStatus(taskID, TaskStatusFailed)
	_ = d.store.DeactivateConnections(taskID)
	_ = d.store.WriteLog(taskID, "error", errMsg)
	d.emit(taskID, EventFailed)
}

func (d *Engine) emit(taskID int, event EventType) {
	if d.onEvent != nil {
		d.onEvent(taskID, event)
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
