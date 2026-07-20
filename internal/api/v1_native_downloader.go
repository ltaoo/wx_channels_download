package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventType 下载器事件类型
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

// TaskInfo 下载器需要的任务信息（由外部通过 LoadTask 提供）
type TaskInfo struct {
	ID         int    // 任务 ID
	Name       string // 文件名
	SavePath   string // 保存目录
	URL        string // 下载端点 URL
	ResourceID int    // 资源 ID（用于创建分片）
}

// SegmentRange 分片范围信息
type SegmentRange struct {
	Index       int
	OffsetStart int64
	OffsetEnd   int64
	Size        int64
}

// segmentInfo 分片下载任务信息（用于 downloadSegment goroutine）
type segmentInfo struct {
	ID          int
	Index       int
	URL         string
	OffsetStart int64
	OffsetEnd   int64
	Size        int64
	Downloaded  int64
}

// segmentProgress 分片下载进度
type segmentProgress struct {
	index      int
	downloaded int64
	speed      int64
	done       bool
	err        error
}

const defaultSegmentCount = 10 // 默认分片数

// DownloadTaskStore 任务持久化接口，由外部实现（例如数据库）。
// 下载引擎不直接访问数据库，所有持久化操作通过此接口完成。
type DownloadTaskStore interface {
	LoadTask(taskID int) (*TaskInfo, error)
	UpdateStatus(taskID int, status int) error
	ActivateTask(taskID int) error
	UpdateProgress(taskID int, downloaded int64, speed int64) error
	UpdateResourceSize(taskID int, size int64) error
	DeactivateConnections(taskID int) error
	FinishTask(taskID int) error
	WriteLog(taskID int, level string, message string) error
	CreateSegments(resourceID int, url string, ranges []SegmentRange) ([]int, error)
	LoadSegmentInfo(resourceID int) ([]segmentInfo, error)
	UpdateSegmentProgress(segID int, downloaded int64) error
}

// v1NativeDownloader 原生 HTTP 下载引擎
type v1NativeDownloader struct {
	mu            sync.Mutex
	maxConcurrent int
	sem           chan struct{}
	jobs          map[int]*nativeJob
	store         DownloadTaskStore
	onEvent       func(taskID int, event EventType)
}

type nativeJob struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func newV1NativeDownloader(store DownloadTaskStore,
	onEvent func(taskID int, event EventType),
	maxConcurrent int) *v1NativeDownloader {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	return &v1NativeDownloader{
		maxConcurrent: maxConcurrent,
		sem:           make(chan struct{}, maxConcurrent),
		jobs:          make(map[int]*nativeJob),
		store:         store,
		onEvent:       onEvent,
	}
}

// Start 启动下载任务，阻塞直到获取并发槽位。
func (d *v1NativeDownloader) Start(taskID int) error {
	d.mu.Lock()
	if _, exists := d.jobs[taskID]; exists {
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()

	d.sem <- struct{}{}

	d.mu.Lock()
	if _, exists := d.jobs[taskID]; exists {
		d.mu.Unlock()
		<-d.sem
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &nativeJob{ctx: ctx, cancel: cancel}
	d.jobs[taskID] = job
	d.mu.Unlock()

	go d.run(taskID, job)
	return nil
}

// Pause 暂停下载任务。
func (d *v1NativeDownloader) Pause(taskID int) {
	d.mu.Lock()
	job, exists := d.jobs[taskID]
	if exists {
		job.cancel()
		delete(d.jobs, taskID)
	}
	d.mu.Unlock()
}

// PauseAll 暂停所有进行中的下载。
func (d *v1NativeDownloader) PauseAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for taskID, job := range d.jobs {
		job.cancel()
		delete(d.jobs, taskID)
	}
}

// Delete 删除下载任务。
func (d *v1NativeDownloader) Delete(taskID int) {
	d.mu.Lock()
	job, exists := d.jobs[taskID]
	if exists {
		job.cancel()
		delete(d.jobs, taskID)
	}
	d.mu.Unlock()
	if exists {
		d.emit(taskID, EventDeleted)
	}
}

func (d *v1NativeDownloader) run(taskID int, job *nativeJob) {
	ctx := job.ctx
	defer func() {
		if r := recover(); r != nil {
			d.failTask(taskID, fmt.Sprintf("下载协程异常退出: %v", r))
		}
		<-d.sem
		d.mu.Lock()
		if d.jobs[taskID] == job {
			delete(d.jobs, taskID)
		}
		d.mu.Unlock()
	}()

	info, err := d.store.LoadTask(taskID)
	fmt.Println("after LoadTask", taskID, info)
	if err != nil {
		d.failTask(taskID, "加载任务信息失败: "+err.Error())
		return
	}

	// 确定文件路径
	name := info.Name
	if name == "" {
		name = filepath.Base(info.URL)
	}
	filePath := filepath.Join(info.SavePath, name)

	// 创建下载目录
	if err := os.MkdirAll(info.SavePath, 0755); err != nil {
		d.failTask(taskID, "创建下载目录失败: "+err.Error())
		return
	}

	// 更新状态并激活任务
	d.store.UpdateStatus(taskID, 2)     // TaskStatusDownloading
	d.store.ActivateTask(taskID)
	d.emit(taskID, EventStarted)

	// 检查 ctx 是否已取消
	select {
	case <-ctx.Done():
		d.pauseTask(taskID)
		return
	default:
	}

	// 获取文件已下载大小（断点续传）
	var existingSize int64
	if fi, err := os.Stat(filePath); err == nil {
		existingSize = fi.Size()
	}

	// HEAD 请求获取 Content-Length
	contentLength := d.headContentLength(info.URL)
	if contentLength > 0 {
		d.store.UpdateResourceSize(taskID, contentLength)
	}

	// 检查 ctx
	select {
	case <-ctx.Done():
		d.pauseTask(taskID)
		return
	default:
	}

	// 如果文件大小未知，使用单分片下载路径；否则按固定分片数多段并发下载
	if contentLength <= 0 {
		// 创建单个分片记录，供 UpdateProgress 写入 downloaded
		ranges := []SegmentRange{{Index: 0, OffsetStart: 0, OffsetEnd: 0, Size: 0}}
		d.store.CreateSegments(info.ResourceID, info.URL, ranges)

		fmt.Println("before d.downloadFile", taskID, filePath)
		completed := d.downloadFile(ctx, info.URL, filePath, existingSize, taskID)
		if completed {
			d.finishTask(taskID, filePath)
		} else {
			select {
			case <-ctx.Done():
				d.pauseTask(taskID)
			default:
			}
		}
		return
	}

	// 多分片并发下载路径
	fmt.Println("multi-segment download: taskID", taskID, "fileSize", contentLength)

	// 检查是否已有分片信息（断点续传）
	segInfos, err := d.store.LoadSegmentInfo(info.ResourceID)
	if err != nil {
		d.failTask(taskID, "加载分片信息失败: "+err.Error())
		return
	}

	var file *os.File
	if len(segInfos) == 0 {
		// 首次下载，创建分片记录
		ranges := splitFile(contentLength, defaultSegmentCount)
		segIDs, err := d.store.CreateSegments(info.ResourceID, info.URL, ranges)
		if err != nil {
			d.failTask(taskID, "创建分片记录失败: "+err.Error())
			return
		}

		// 构建 segmentInfo
		for i := range ranges {
			segInfos = append(segInfos, segmentInfo{
				ID:          segIDs[i],
				Index:       ranges[i].Index,
				URL:         info.URL,
				OffsetStart: ranges[i].OffsetStart,
				OffsetEnd:   ranges[i].OffsetEnd,
				Size:        ranges[i].Size,
				Downloaded:  0,
			})
		}

		// 创建文件并预分配大小
		file, err = os.Create(filePath)
		if err != nil {
			d.failTask(taskID, "创建文件失败: "+err.Error())
			return
		}
		if err := file.Truncate(contentLength); err != nil {
			file.Close()
			d.failTask(taskID, "预分配文件大小失败: "+err.Error())
			return
		}
		file.Close()
	} else {
		// 断点续传：文件应已存在
		fmt.Println("multi-segment resume: found", len(segInfos), "existing segments")
	}

	numSegments := len(segInfos)
	progressCh := make(chan segmentProgress, numSegments*2)
	var wg sync.WaitGroup

	for i := range segInfos {
		// 跳过已完成的分片
		if segInfos[i].Downloaded >= segInfos[i].Size {
			fmt.Println("segment", segInfos[i].Index, "already completed, skip")
			progressCh <- segmentProgress{index: segInfos[i].Index, downloaded: segInfos[i].Size, speed: 0, done: true}
			continue
		}
		wg.Add(1)
		go func(seg segmentInfo) {
			defer wg.Done()
			d.downloadSegment(ctx, info.URL, filePath, seg, taskID, progressCh)
		}(segInfos[i])
	}

	go func() {
		wg.Wait()
		close(progressCh)
	}()

	d.aggregateProgress(ctx, progressCh, segInfos, taskID, filePath)
}

// newDownloadClient 创建带超时配置的 HTTP 客户端。
// ResponseHeaderTimeout 确保不响应的服务器快速失败。
func newDownloadClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ResponseHeaderTimeout: 10 * time.Second,
		},
		Timeout: 30 * time.Minute, // 大文件整体超时由 context 控制
	}
}

// headContentLength 发送 HEAD 请求获取内容长度。
func (d *v1NativeDownloader) headContentLength(url string) int64 {
	client := newDownloadClient()
	resp, err := client.Head(url)
	if err != nil {
		fmt.Println("headContentLength error:", err)
		return 0
	}
	defer resp.Body.Close()
	fmt.Println("headContentLength status:", resp.StatusCode, "content-length:", resp.ContentLength)
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	return resp.ContentLength
}

// splitFile 将文件等分为 n 个分片范围，余数分散到前面的分片中。
func splitFile(fileSize int64, n int) []SegmentRange {
	if n <= 0 || fileSize <= 0 {
		return nil
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
		end := offset + size - 1
		ranges[i] = SegmentRange{
			Index:       i,
			OffsetStart: offset,
			OffsetEnd:   end,
			Size:        size,
		}
		offset += size
	}
	return ranges
}

// downloadFile 下载文件，返回是否完整下载。
func (d *v1NativeDownloader) downloadFile(ctx context.Context, url, filePath string, offset int64, taskID int) bool {
	fmt.Println("downloadFile enter: taskID", taskID, "url", url, "offset", offset)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Println("downloadFile NewRequest error:", err)
		d.failTask(taskID, "创建请求失败: "+err.Error())
		return false
	}

	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	fmt.Println("downloadFile client.Do starting...")
	client := newDownloadClient()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("downloadFile client.Do error:", err)
		select {
		case <-ctx.Done():
			return false
		default:
		}
		d.failTask(taskID, "下载请求失败: "+err.Error())
		return false
	}
	defer resp.Body.Close()
	fmt.Println("downloadFile resp: status", resp.StatusCode, "content-length", resp.ContentLength)

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			fmt.Println("downloadFile: 416, falling back to downloadFileWithoutRange")
			return d.downloadFileWithoutRange(ctx, url, filePath, taskID)
		}
		fmt.Println("downloadFile: bad status code", resp.StatusCode)
		d.failTask(taskID, fmt.Sprintf("服务器返回错误状态码: %d", resp.StatusCode))
		return false
	}

	var file *os.File
	if resp.StatusCode == http.StatusPartialContent {
		file, err = os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	} else {
		file, err = os.Create(filePath)
		offset = 0
	}
	if err != nil {
		fmt.Println("downloadFile file open/create error:", err)
		d.failTask(taskID, "创建文件失败: "+err.Error())
		return false
	}
	defer file.Close()

	buf := make([]byte, 32*1024)
	downloaded := offset
	totalReads := 0
	lastProgress := time.Time{} // 零值，确保首次读取立即触发 progress 回调
	lastDownloaded := downloaded

	for {
		select {
		case <-ctx.Done():
			fmt.Println("downloadFile: ctx.Done(), pausing")
			return false
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				fmt.Println("downloadFile write error:", writeErr)
				d.failTask(taskID, "写入文件失败: "+writeErr.Error())
				return false
			}
			downloaded += int64(n)
			totalReads++
		}

		now := time.Now()
		if n == 0 || now.Sub(lastProgress) >= 500*time.Millisecond || readErr == io.EOF {
			elapsed := now.Sub(lastProgress).Seconds()
			var speed int64
			if elapsed > 0 {
				speed = int64(float64(downloaded-lastDownloaded) / elapsed)
			}

			fmt.Println("downloadFile progress:", downloaded, "bytes,", speed, "B/s, reads:", totalReads, "err:", readErr)
			d.store.UpdateProgress(taskID, downloaded, speed)
			d.emit(taskID, EventProgress)
			lastProgress = now
			lastDownloaded = downloaded
		}

		if readErr != nil {
			if readErr == io.EOF {
				fmt.Println("downloadFile: EOF, total", downloaded, "bytes,", totalReads, "reads")
				return true
			}
			if errors.Is(readErr, context.Canceled) {
				fmt.Println("downloadFile: context.Canceled")
				return false
			}
			fmt.Println("downloadFile read error:", readErr)
			d.failTask(taskID, "下载读取失败: "+readErr.Error())
			return false
		}
	}
}

// downloadFileWithoutRange 从头开始下载。
func (d *v1NativeDownloader) downloadFileWithoutRange(ctx context.Context, url, filePath string, taskID int) bool {
	fmt.Println("downloadFileWithoutRange enter: taskID", taskID, "url", url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Println("downloadFileWithoutRange NewRequest error:", err)
		d.failTask(taskID, "创建请求失败: "+err.Error())
		return false
	}

	fmt.Println("downloadFileWithoutRange client.Do starting...")
	client := newDownloadClient()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("downloadFileWithoutRange client.Do error:", err)
		select {
		case <-ctx.Done():
			return false
		default:
		}
		d.failTask(taskID, "下载请求失败: "+err.Error())
		return false
	}
	defer resp.Body.Close()
	fmt.Println("downloadFileWithoutRange resp: status", resp.StatusCode, "content-length", resp.ContentLength)

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		fmt.Println("downloadFileWithoutRange: bad status code", resp.StatusCode)
		d.failTask(taskID, fmt.Sprintf("服务器返回错误状态码: %d", resp.StatusCode))
		return false
	}

	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("downloadFileWithoutRange file create error:", err)
		d.failTask(taskID, "创建文件失败: "+err.Error())
		return false
	}
	defer file.Close()

	buf := make([]byte, 32*1024)
	downloaded := int64(0)
	totalReads := 0
	lastProgress := time.Time{} // 零值，确保首次读取立即触发 progress 回调
	lastDownloaded := int64(0)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("downloadFileWithoutRange: ctx.Done(), pausing")
			return false
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				fmt.Println("downloadFileWithoutRange write error:", writeErr)
				d.failTask(taskID, "写入文件失败: "+writeErr.Error())
				return false
			}
			downloaded += int64(n)
			totalReads++
		}

		now := time.Now()
		if n == 0 || now.Sub(lastProgress) >= 500*time.Millisecond || readErr == io.EOF {
			elapsed := now.Sub(lastProgress).Seconds()
			var speed int64
			if elapsed > 0 {
				speed = int64(float64(downloaded-lastDownloaded) / elapsed)
			}

			fmt.Println("downloadFileWithoutRange progress:", downloaded, "bytes,", speed, "B/s, reads:", totalReads, "err:", readErr)
			d.store.UpdateProgress(taskID, downloaded, speed)
			d.emit(taskID, EventProgress)
			lastProgress = now
			lastDownloaded = downloaded
		}

		if readErr != nil {
			if readErr == io.EOF {
				fmt.Println("downloadFileWithoutRange: EOF, total", downloaded, "bytes,", totalReads, "reads")
				return true
			}
			if errors.Is(readErr, context.Canceled) {
				fmt.Println("downloadFileWithoutRange: context.Canceled")
				return false
			}
			fmt.Println("downloadFileWithoutRange read error:", readErr)
			d.failTask(taskID, "下载读取失败: "+readErr.Error())
			return false
		}
	}
}

// downloadSegment 下载单个分片，将进度通过 channel 发送给汇总 goroutine。
func (d *v1NativeDownloader) downloadSegment(ctx context.Context, url, filePath string, seg segmentInfo, taskID int, progressCh chan<- segmentProgress) {
	fmt.Println("downloadSegment enter: taskID", taskID, "index", seg.Index, "offset", seg.OffsetStart, "downloaded", seg.Downloaded)

	startOffset := seg.OffsetStart + seg.Downloaded
	rangeHeader := fmt.Sprintf("bytes=%d-%d", startOffset, seg.OffsetEnd)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		fmt.Println("downloadSegment NewRequest error:", err)
		progressCh <- segmentProgress{index: seg.Index, done: true, err: err}
		return
	}
	req.Header.Set("Range", rangeHeader)

	client := newDownloadClient()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("downloadSegment client.Do error:", err)
		select {
		case <-ctx.Done():
			return
		default:
		}
		progressCh <- segmentProgress{index: seg.Index, done: true, err: err}
		return
	}
	defer resp.Body.Close()
	fmt.Println("downloadSegment resp: status", resp.StatusCode, "content-length", resp.ContentLength, "index", seg.Index)

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			fmt.Println("downloadSegment: 416, segment already downloaded, index", seg.Index)
			progressCh <- segmentProgress{index: seg.Index, downloaded: seg.Size, speed: 0, done: true}
			return
		}
		fmt.Println("downloadSegment: bad status code", resp.StatusCode, "index", seg.Index)
		progressCh <- segmentProgress{index: seg.Index, done: true, err: fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)}
		return
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("downloadSegment file open error:", err, "index", seg.Index)
		progressCh <- segmentProgress{index: seg.Index, done: true, err: err}
		return
	}
	defer file.Close()

	buf := make([]byte, 32*1024)
	downloaded := seg.Downloaded
	lastProgress := time.Time{}
	lastDownloaded := downloaded
	totalReads := 0

	for {
		select {
		case <-ctx.Done():
			fmt.Println("downloadSegment: ctx.Done(), index", seg.Index)
			return
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			writeOffset := seg.OffsetStart + downloaded
			if _, writeErr := file.WriteAt(buf[:n], writeOffset); writeErr != nil {
				fmt.Println("downloadSegment write error:", writeErr, "index", seg.Index)
				progressCh <- segmentProgress{index: seg.Index, done: true, err: writeErr}
				return
			}
			downloaded += int64(n)
			totalReads++
		}

		now := time.Now()
		if n == 0 || now.Sub(lastProgress) >= 500*time.Millisecond || readErr == io.EOF {
			elapsed := now.Sub(lastProgress).Seconds()
			var speed int64
			if elapsed > 0 {
				speed = int64(float64(downloaded-lastDownloaded) / elapsed)
			}

			fmt.Println("downloadSegment progress: index", seg.Index, "downloaded", downloaded, "speed", speed, "B/s, reads:", totalReads, "err:", readErr)
			progressCh <- segmentProgress{index: seg.Index, downloaded: downloaded, speed: speed}

			lastProgress = now
			lastDownloaded = downloaded
		}

		if readErr != nil {
			if readErr == io.EOF {
				fmt.Println("downloadSegment: EOF, index", seg.Index, "total", downloaded, "bytes")
				progressCh <- segmentProgress{index: seg.Index, downloaded: downloaded, speed: 0, done: true}
				return
			}
			if errors.Is(readErr, context.Canceled) {
				fmt.Println("downloadSegment: context.Canceled, index", seg.Index)
				return
			}
			fmt.Println("downloadSegment read error:", readErr, "index", seg.Index)
			progressCh <- segmentProgress{index: seg.Index, done: true, err: readErr}
			return
		}
	}
}

// aggregateProgress 汇总所有分片的下载进度，统一写入数据库并推送事件。
func (d *v1NativeDownloader) aggregateProgress(ctx context.Context, progressCh <-chan segmentProgress, segInfos []segmentInfo, taskID int, filePath string) {
	numSegments := len(segInfos)
	perSegment := make([]segmentProgress, numSegments)
	doneCount := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	emitAggregate := func() {
		var totalDownloaded int64
		var totalSpeed int64
		for _, s := range perSegment {
			totalDownloaded += s.downloaded
			totalSpeed += s.speed
		}
		// 先写入聚合的总量（连接、资源），再按分片单独修正 downloaded
		d.store.UpdateProgress(taskID, totalDownloaded, totalSpeed)
		for i, s := range perSegment {
			if s.downloaded > 0 {
				d.store.UpdateSegmentProgress(segInfos[i].ID, s.downloaded)
			}
		}
		d.emit(taskID, EventProgress)
	}

	for {
		select {
		case <-ctx.Done():
			d.pauseTask(taskID)
			return
		case prog, ok := <-progressCh:
			if !ok {
				// 所有分片 goroutine 已退出
				for _, s := range perSegment {
					if s.err != nil {
						d.failTask(taskID, "分片下载失败: "+s.err.Error())
						return
					}
				}
				emitAggregate()
				d.finishTask(taskID, filePath)
				return
			}
			perSegment[prog.index] = prog
			if prog.done {
				doneCount++
				if prog.err != nil {
					d.failTask(taskID, "分片下载失败: "+prog.err.Error())
					return
				}
			}
		case <-ticker.C:
			emitAggregate()
		}
	}
}

func (d *v1NativeDownloader) finishTask(taskID int, filePath string) {
	d.store.FinishTask(taskID)
	d.store.WriteLog(taskID, "info", fmt.Sprintf("下载完成, 文件: %s", filePath))
	d.emit(taskID, EventFinished)
}

func (d *v1NativeDownloader) pauseTask(taskID int) {
	d.store.UpdateStatus(taskID, 3) // TaskStatusPaused
	d.store.DeactivateConnections(taskID)
	d.emit(taskID, EventPaused)
}

func (d *v1NativeDownloader) failTask(taskID int, errMsg string) {
	d.store.UpdateStatus(taskID, 6) // TaskStatusFailed
	d.store.DeactivateConnections(taskID)
	d.store.WriteLog(taskID, "error", errMsg)
	d.emit(taskID, EventFailed)
}

func (d *v1NativeDownloader) emit(taskID int, event EventType) {
	if d.onEvent != nil {
		d.onEvent(taskID, event)
	}
}
