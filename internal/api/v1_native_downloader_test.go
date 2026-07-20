package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// mockTaskStore — DownloadTaskStore 的内存 mock 实现
// ---------------------------------------------------------------------------

type mockTaskStore struct {
	mu              sync.Mutex
	taskInfo        *TaskInfo
	loadTaskErr     error
	statusCalls     []int
	activateCalls   int
	progressCalls   []progressCall
	resourceSizeVal int64
	deactivateCalls int
	finishCalls     int
	logCalls        []logCall
}

type progressCall struct {
	taskID     int
	downloaded int64
	speed      int64
}

type logCall struct {
	taskID  int
	level   string
	message string
}

func (m *mockTaskStore) LoadTask(taskID int) (*TaskInfo, error) {
	if m.loadTaskErr != nil {
		return nil, m.loadTaskErr
	}
	return m.taskInfo, nil
}

func (m *mockTaskStore) UpdateStatus(taskID int, status int) error {
	m.mu.Lock()
	m.statusCalls = append(m.statusCalls, status)
	m.mu.Unlock()
	return nil
}

func (m *mockTaskStore) ActivateTask(taskID int) error {
	m.mu.Lock()
	m.activateCalls++
	m.mu.Unlock()
	return nil
}

func (m *mockTaskStore) UpdateProgress(taskID int, downloaded int64, speed int64) error {
	m.mu.Lock()
	m.progressCalls = append(m.progressCalls, progressCall{taskID, downloaded, speed})
	m.mu.Unlock()
	return nil
}

func (m *mockTaskStore) UpdateResourceSize(taskID int, size int64) error {
	m.mu.Lock()
	m.resourceSizeVal = size
	m.mu.Unlock()
	return nil
}

func (m *mockTaskStore) DeactivateConnections(taskID int) error {
	m.mu.Lock()
	m.deactivateCalls++
	m.mu.Unlock()
	return nil
}

func (m *mockTaskStore) FinishTask(taskID int) error {
	m.mu.Lock()
	m.finishCalls++
	m.mu.Unlock()
	return nil
}

func (m *mockTaskStore) WriteLog(taskID int, level string, message string) error {
	m.mu.Lock()
	m.logCalls = append(m.logCalls, logCall{taskID, level, message})
	m.mu.Unlock()
	return nil
}

func (m *mockTaskStore) CreateSegments(resourceID int, url string, ranges []SegmentRange) ([]int, error) {
	var ids []int
	for i := range ranges {
		ids = append(ids, i+1)
	}
	return ids, nil
}

func (m *mockTaskStore) UpdateSegmentProgress(segID int, downloaded int64) error {
	return nil
}

func (m *mockTaskStore) LoadSegmentInfo(resourceID int) ([]segmentInfo, error) {
	return nil, nil
}

func (m *mockTaskStore) lastStatus() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.statusCalls) == 0 {
		return -1
	}
	return m.statusCalls[len(m.statusCalls)-1]
}

// ---------------------------------------------------------------------------
// eventTracker — onEvent 回调追踪器
// ---------------------------------------------------------------------------

type eventTracker struct {
	mu     sync.Mutex
	events []EventType
}

func (t *eventTracker) record(taskID int, event EventType) {
	t.mu.Lock()
	t.events = append(t.events, event)
	t.mu.Unlock()
}

func (t *eventTracker) snapshot() []EventType {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := make([]EventType, len(t.events))
	copy(c, t.events)
	return c
}

func (t *eventTracker) count(event EventType) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, e := range t.events {
		if e == event {
			n++
		}
	}
	return n
}

func (t *eventTracker) waitFor(event EventType, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		t.mu.Lock()
		for _, e := range t.events {
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

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestV1NativeDownloader_DownloadWithProgress(t *testing.T) {
	// 创建测试用临时目录和文件
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	// 创建一个测试文件（5MB，足够产生多次 onEvent 回调）
	tmpFile := filepath.Join(tmpDir, "test_data.bin")
	if err := createTempFile(tmpFile, 5*1024*1024); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 启动本地 HTTP 测试服务器
	ts := startFileServer(t, tmpFile, "test.bin")
	defer ts.Close()

	// mock store
	store := &mockTaskStore{
		taskInfo: &TaskInfo{
			ID:       1,
			Name:     "test.bin",
			SavePath: saveDir,
			URL:      ts.URL,
		},
	}

	// event tracker
	tracker := &eventTracker{}

	d := newV1NativeDownloader(store, tracker.record, 1)

	// 启动下载
	if err := d.Start(1); err != nil {
		t.Fatalf("启动下载失败: %v", err)
	}

	// 等待下载完成
	if !tracker.waitFor(EventFinished, 30*time.Second) {
		events := tracker.snapshot()
		t.Fatalf("下载未在超时时间内完成, 收到的事件: %v", events)
	}

	// 断言
	events := tracker.snapshot()

	assert.Contains(t, events, EventStarted, "应收到 started 事件")
	assert.Contains(t, events, EventFinished, "应收到 finished 事件")

	progressCount := tracker.count(EventProgress)
	assert.GreaterOrEqual(t, progressCount, 1, "应收到 progress 事件")
	t.Logf("收到 %d 次 progress 事件", progressCount)

	// 断言 store 方法被调用
	store.mu.Lock()
	defer store.mu.Unlock()

	assert.Equal(t, 1, store.activateCalls, "ActivateTask 应被调用 1 次")
	assert.Equal(t, 1, store.finishCalls, "FinishTask 应被调用 1 次")

	// 断言下载文件存在且大小正确
	downloadedFile := filepath.Join(saveDir, "test.bin")
	fi, err := os.Stat(downloadedFile)
	assert.NoError(t, err, "下载文件应存在")
	if err == nil {
		assert.Equal(t, int64(5*1024*1024), fi.Size(), "下载文件大小应正确")
	}

	// 断言 progress 更新了 downloaded 和 speed
	assert.Greater(t, len(store.progressCalls), 0, "应有进度更新回调")
	if len(store.progressCalls) > 0 {
		last := store.progressCalls[len(store.progressCalls)-1]
		assert.Equal(t, int64(5*1024*1024), last.downloaded, "最终 downloaded 应等于文件大小")
	}

	t.Logf("总共 %d 个事件, 顺序: %v", len(events), events)
}

func TestV1NativeDownloader_FileSmallerThanBuffer(t *testing.T) {
	// 测试小于 32KB 的文件（单次 read 完成）
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	tmpFile := filepath.Join(tmpDir, "small.bin")
	if err := createTempFile(tmpFile, 1024); err != nil { // 1KB
		t.Fatalf("创建测试文件失败: %v", err)
	}

	ts := startFileServer(t, tmpFile, "small.bin")
	defer ts.Close()

	store := &mockTaskStore{
		taskInfo: &TaskInfo{
			ID:       1,
			Name:     "small.bin",
			SavePath: saveDir,
			URL:      ts.URL,
		},
	}

	tracker := &eventTracker{}
	d := newV1NativeDownloader(store, tracker.record, 1)

	if err := d.Start(1); err != nil {
		t.Fatalf("启动下载失败: %v", err)
	}

	if !tracker.waitFor(EventFinished, 10*time.Second) {
		t.Fatal("下载未完成")
	}

	assert.Contains(t, tracker.snapshot(), EventStarted)
	assert.Contains(t, tracker.snapshot(), EventFinished)
	assert.Equal(t, 1, store.finishCalls)
}

func TestV1NativeDownloader_ConcurrencyLimit(t *testing.T) {
	// 测试并发限制：同时启动 3 个任务，3 个都能运行
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	// 创建一个小文件供多个任务下载
	tmpFile := filepath.Join(tmpDir, "shared.bin")
	if err := createTempFile(tmpFile, 500*1024); err != nil { // 500KB
		t.Fatalf("创建测试文件失败: %v", err)
	}

	ts := startFileServer(t, tmpFile, "shared.bin")
	defer ts.Close()

	trackers := make([]*eventTracker, 3)
	stores := make([]*mockTaskStore, 3)

	for i := 0; i < 3; i++ {
		stores[i] = &mockTaskStore{
			taskInfo: &TaskInfo{
				ID:       i + 1,
				Name:     "shared.bin",
				SavePath: saveDir,
				URL:      ts.URL,
			},
		}
		trackers[i] = &eventTracker{}
	}

	_ = ts

	// 启动多个下载任务验证并发限制不阻塞
	for i := 0; i < 3; i++ {
		d := newV1NativeDownloader(stores[i], trackers[i].record, 3)
		if err := d.Start(i + 1); err != nil {
			t.Fatalf("启动任务 %d 失败: %v", i+1, err)
		}
	}

	// 等待所有任务完成
	for i := 0; i < 3; i++ {
		if !trackers[i].waitFor(EventFinished, 15*time.Second) {
			t.Fatalf("任务 %d 未完成", i+1)
		}
	}

	for i := 0; i < 3; i++ {
		assert.Greater(t, trackers[i].count(EventProgress), 0,
			"任务 %d 应有 progress 事件", i+1)
	}
}

func TestV1NativeDownloader_PauseAndResume(t *testing.T) {
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	// 创建一个慢速 HTTP 服务器，确保 pause 有足够时间生效
	ts := startSlowServer(t, 5*1024*1024) // 5MB data at slow pace
	defer ts.Close()

	store := &mockTaskStore{
		taskInfo: &TaskInfo{
			ID:       1,
			Name:     "pause_test.bin",
			SavePath: saveDir,
			URL:      ts.URL,
		},
	}

	tracker := &eventTracker{}
	d := newV1NativeDownloader(store, tracker.record, 1)

	if err := d.Start(1); err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	// 等待至少收到一次 progress 事件
	if !tracker.waitFor(EventProgress, 5*time.Second) {
		t.Fatal("未收到 progress 事件")
	}

	// 暂停
	d.Pause(1)

	// 等待 paused 事件
	if !tracker.waitFor(EventPaused, 10*time.Second) {
		t.Fatal("未收到 paused 事件")
	}

	assert.Contains(t, tracker.snapshot(), EventPaused)
	assert.Equal(t, 1, store.deactivateCalls, "暂停时应调用 DeactivateConnections")

	// 恢复
	d2 := newV1NativeDownloader(store, tracker.record, 1)
	if err := d2.Start(1); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	// 等待完成
	if !tracker.waitFor(EventFinished, 30*time.Second) {
		t.Fatal("恢复后下载未完成")
	}
}

func TestV1NativeDownloader_LoadTaskError(t *testing.T) {
	store := &mockTaskStore{
		loadTaskErr: errors.New("load error"),
	}
	tracker := &eventTracker{}
	d := newV1NativeDownloader(store, tracker.record, 1)

	if err := d.Start(1); err != nil {
		t.Fatalf("Start 不应返回错误: %v", err)
	}

	if !tracker.waitFor(EventFailed, 5*time.Second) {
		t.Fatal("未收到 failed 事件")
	}

	assert.Contains(t, tracker.snapshot(), EventFailed)
}

func TestV1NativeDownloader_EventSequence(t *testing.T) {
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	tmpFile := filepath.Join(tmpDir, "sequence.bin")
	if err := createTempFile(tmpFile, 1*1024*1024); err != nil { // 1MB
		t.Fatalf("创建测试文件失败: %v", err)
	}

	ts := startFileServer(t, tmpFile, "sequence.bin")
	defer ts.Close()

	store := &mockTaskStore{
		taskInfo: &TaskInfo{
			ID:       1,
			Name:     "sequence.bin",
			SavePath: saveDir,
			URL:      ts.URL,
		},
	}

	tracker := &eventTracker{}
	d := newV1NativeDownloader(store, tracker.record, 1)

	if err := d.Start(1); err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	if !tracker.waitFor(EventFinished, 15*time.Second) {
		t.Fatal("下载未完成")
	}

	events := tracker.snapshot()

	// 验证事件顺序：started -> progress* -> finished
	foundStarted := false
	foundFinished := false
	lastProgressBeforeFinish := -1

	for i, ev := range events {
		switch ev {
		case EventStarted:
			foundStarted = true
			assert.False(t, foundFinished, "started 应在 finished 之前")
		case EventProgress:
			if !foundFinished {
				lastProgressBeforeFinish = i
			}
		case EventFinished:
			foundFinished = true
			assert.True(t, foundStarted, "finished 应在 started 之后")
			assert.GreaterOrEqual(t, lastProgressBeforeFinish, 0, "finished 前应有 progress 事件")
		}
	}

	assert.True(t, foundStarted && foundFinished, "应包含 started 和 finished 事件")
}

func TestV1NativeDownloader_MultiSegmentConcurrent(t *testing.T) {
	// 测试多分片并发下载：默认 10 个分片，文件 10MB 时分片各 1MB
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	// 创建一个 10MB 的临时文件（10 个分片，每个约 1MB）
	fileSize := int64(10 * 1024 * 1024)
	tmpFile := filepath.Join(tmpDir, "multi_seg.bin")
	if err := createTempFile(tmpFile, fileSize); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	ts := startFileServer(t, tmpFile, "multi_seg.bin")
	defer ts.Close()

	store := &mockTaskStore{
		taskInfo: &TaskInfo{
			ID:         1,
			Name:       "multi_seg.bin",
			SavePath:   saveDir,
			URL:        ts.URL,
			ResourceID: 1,
		},
	}

	tracker := &eventTracker{}
	d := newV1NativeDownloader(store, tracker.record, 10)

	if err := d.Start(1); err != nil {
		t.Fatalf("启动下载失败: %v", err)
	}

	// 等待下载完成
	if !tracker.waitFor(EventFinished, 30*time.Second) {
		events := tracker.snapshot()
		t.Fatalf("下载未在超时时间内完成, 事件: %v", events)
	}

	events := tracker.snapshot()

	assert.Contains(t, events, EventStarted, "应收到 started 事件")
	assert.Contains(t, events, EventFinished, "应收到 finished 事件")

	progressCount := tracker.count(EventProgress)
	assert.GreaterOrEqual(t, progressCount, 1, "应收到 progress 事件")
	t.Logf("多分片下载收到 %d 次 progress 事件", progressCount)

	// 断言 store 方法被调用
	store.mu.Lock()
	defer store.mu.Unlock()

	assert.Equal(t, 1, store.activateCalls, "ActivateTask 应被调用 1 次")
	assert.Equal(t, 1, store.finishCalls, "FinishTask 应被调用 1 次")

	// 断言下载文件存在且大小正确
	downloadedFile := filepath.Join(saveDir, "multi_seg.bin")
	fi, err := os.Stat(downloadedFile)
	assert.NoError(t, err, "下载文件应存在")
	if err == nil {
		assert.Equal(t, fileSize, fi.Size(), "下载文件大小应正确")
	}

	// 断言 progress 更新了 downloaded
	assert.Greater(t, len(store.progressCalls), 0, "应有进度更新回调")
	if len(store.progressCalls) > 0 {
		last := store.progressCalls[len(store.progressCalls)-1]
		assert.Equal(t, fileSize, last.downloaded, "最终 downloaded 应等于文件大小")
	}

	t.Logf("总共 %d 个事件, 顺序: %v", len(events), events)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func createTempFile(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if size > 0 {
		return f.Truncate(size)
	}
	return nil
}

func startFileServer(t *testing.T, filePath, filename string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filePath)
	}))
}

// startSlowServer 创建一个慢速 HTTP 服务器，用于测试暂停/恢复功能。
// 每次只写入 32KB 数据，并在 chunk 之间 sleep 50ms，确保下载时间足够长。
func startSlowServer(t *testing.T, totalSize int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", totalSize))
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)

		buf := make([]byte, 32*1024)
		remaining := totalSize
		for remaining > 0 {
			chunk := 32 * 1024
			if remaining < chunk {
				chunk = remaining
			}
			if _, err := w.Write(buf[:chunk]); err != nil {
				return
			}
			remaining -= chunk
			time.Sleep(30 * time.Millisecond)
		}
	}))
}
