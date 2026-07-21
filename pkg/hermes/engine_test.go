package hermes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// mockTaskStore — Store 的内存 mock 实现
// ---------------------------------------------------------------------------

type mockTaskStore struct {
	mu              sync.Mutex
	taskInfo        *Task
	loadTaskErr     error
	statusCalls     []int
	activateCalls   int
	progressCalls   []progressCall
	resourceSizeVal int64
	nameUpdates     []OutputNameUpdate
	segmentInfo     []Segment
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

func (m *mockTaskStore) LoadTask(taskID int) (*Task, error) {
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

func (m *mockTaskStore) UpdateOutputName(update OutputNameUpdate) error {
	m.mu.Lock()
	m.nameUpdates = append(m.nameUpdates, update)
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

func (m *mockTaskStore) LoadSegmentInfo(resourceID int) ([]Segment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	segments := make([]Segment, len(m.segmentInfo))
	copy(segments, m.segmentInfo)
	return segments, nil
}

func (m *mockTaskStore) lastStatus() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.statusCalls) == 0 {
		return -1
	}
	return m.statusCalls[len(m.statusCalls)-1]
}

func (m *mockTaskStore) lastLog() logCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.logCalls) == 0 {
		return logCall{}
	}
	return m.logCalls[len(m.logCalls)-1]
}

func (m *mockTaskStore) outputNameUpdates() []OutputNameUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	updates := make([]OutputNameUpdate, len(m.nameUpdates))
	copy(updates, m.nameUpdates)
	return updates
}

// ---------------------------------------------------------------------------
// eventTracker — onEvent 回调追踪器
// ---------------------------------------------------------------------------

type eventTracker struct {
	mu     sync.Mutex
	events []EventType
}

type memoryProtocolDriver struct {
	data []byte
}

type failingProtocolDriver struct {
	size int64
}

type flakyPrepareDriver struct {
	mu       sync.Mutex
	attempts int
	data     []byte
}

func (d *memoryProtocolDriver) Protocols() []string { return []string{"memory"} }

func (d *memoryProtocolDriver) Prepare(context.Context, Endpoint) (PreparedResource, error) {
	return PreparedResource{Size: int64(len(d.data))}, nil
}

func (d *memoryProtocolDriver) Open(context.Context, Endpoint, ReadRequest) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(d.data)), nil
}

func (d *failingProtocolDriver) Protocols() []string { return []string{"failing"} }

func (d *failingProtocolDriver) Prepare(context.Context, Endpoint) (PreparedResource, error) {
	return PreparedResource{Size: d.size}, nil
}

func (d *failingProtocolDriver) Open(context.Context, Endpoint, ReadRequest) (io.ReadCloser, error) {
	return nil, errors.New("endpoint unavailable")
}

func (d *flakyPrepareDriver) Protocols() []string { return []string{"flaky-prepare"} }

func (d *flakyPrepareDriver) Prepare(context.Context, Endpoint) (PreparedResource, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.attempts++
	if d.attempts < maxReadAttempts {
		return PreparedResource{}, errors.New("temporary endpoint failure")
	}
	return PreparedResource{Size: int64(len(d.data))}, nil
}

func (d *flakyPrepareDriver) Open(context.Context, Endpoint, ReadRequest) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(d.data)), nil
}

func (d *flakyPrepareDriver) prepareAttempts() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.attempts
}

// testHTTPDriver 是供 engine 测试使用的轻量 HTTP 协议驱动。
// 它使用标准 net/http 库而非 tls-client，因此可以与 httptest.Server 兼容。
type testHTTPDriver struct{}

func (d *testHTTPDriver) Protocols() []string { return []string{"http", "https"} }

func (d *testHTTPDriver) Prepare(ctx context.Context, endpoint Endpoint) (PreparedResource, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Range", "bytes=0-0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return PreparedResource{}, err
	}
	defer resp.Body.Close()
	prepared := PreparedResource{ContentType: resp.Header.Get("Content-Type")}
	if resp.StatusCode == http.StatusPartialContent {
		var start, end, total int64
		if _, err := fmt.Sscanf(resp.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total); err == nil && start == 0 && end == 0 && total > 0 {
			prepared.Size = total
			prepared.SupportsRange = true
			return prepared, nil
		}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if resp.ContentLength > 0 {
			prepared.Size = resp.ContentLength
		}
		return prepared, nil
	}
	return PreparedResource{}, fmt.Errorf("HTTP probe returned status %d", resp.StatusCode)
}

func (d *testHTTPDriver) Open(ctx context.Context, endpoint Endpoint, request ReadRequest) (io.ReadCloser, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
	req.Header.Set("Accept-Encoding", "identity")
	if request.UseRange {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", request.OffsetStart, request.OffsetEnd))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
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

func TestEngineInfersExtensionFromContentTypeBeforeWriting(t *testing.T) {
	store := &mockTaskStore{}
	engine := New(store, nil, 1)
	saveDir := t.TempDir()
	task := &Task{
		ID:           1,
		ResourceID:   2,
		Name:         "cover",
		SavePath:     filepath.Join(saveDir, "cover"),
		ResourceType: ResourceTypeFile,
	}

	changed, err := engine.applyContentTypeFilename(task, "https://example.com/media", PreparedResource{ContentType: "image/png"})
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "cover.png", task.Name)
	assert.Equal(t, filepath.Join(saveDir, "cover.png"), task.SavePath)
	assert.Equal(t, []OutputNameUpdate{{
		TaskID:       1,
		ResourceID:   2,
		ResourceName: "cover.png",
		TaskName:     "cover.png",
		SavePath:     task.SavePath,
	}}, store.outputNameUpdates())
}

func TestEngineDoesNotGuessExtensionForKnownNamesOrUnknownMIMETypes(t *testing.T) {
	store := &mockTaskStore{}
	engine := New(store, nil, 1)
	for _, testCase := range []struct {
		name        string
		contentType string
	}{
		{name: "video.mp4", contentType: "image/png"},
		{name: "video", contentType: "application/octet-stream"},
		{name: "playlist", contentType: "application/vnd.apple.mpegurl"},
	} {
		task := &Task{ID: 1, ResourceID: 2, Name: testCase.name, SavePath: filepath.Join(t.TempDir(), testCase.name), ResourceType: ResourceTypeFile}
		changed, err := engine.applyContentTypeFilename(task, "https://example.com/media", PreparedResource{ContentType: testCase.contentType})
		require.NoError(t, err)
		assert.False(t, changed, testCase)
		assert.Equal(t, testCase.name, task.Name)
	}
	assert.Empty(t, store.outputNameUpdates())
}

func TestEngineDoesNotRenameResumedResource(t *testing.T) {
	store := &mockTaskStore{segmentInfo: []Segment{{ID: 1, Size: 8, Downloaded: 2}}}
	engine := New(store, nil, 1)
	task := &Task{
		ID:           1,
		ResourceID:   2,
		Name:         "cover",
		SavePath:     filepath.Join(t.TempDir(), "cover"),
		ResourceType: ResourceTypeFile,
	}

	changed, err := engine.applyContentTypeFilename(task, "https://example.com/media", PreparedResource{ContentType: "image/png"})
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, "cover", task.Name)
	assert.Empty(t, store.outputNameUpdates())
}

func TestEngineInfersExtensionForLongFilenames(t *testing.T) {
	for _, length := range []int{100, 200, 300} {
		t.Run(fmt.Sprintf("%d_characters", length), func(t *testing.T) {
			store := &mockTaskStore{}
			engine := New(store, nil, 1)
			name := strings.Repeat("a", length)
			task := &Task{
				ID:           1,
				ResourceID:   2,
				Name:         name,
				SavePath:     filepath.Join(t.TempDir(), name),
				ResourceType: ResourceTypeFile,
			}

			changed, err := engine.applyContentTypeFilename(task, "https://example.com/media", PreparedResource{ContentType: "image/png"})
			require.NoError(t, err)
			assert.True(t, changed)
			assert.True(t, strings.HasSuffix(task.Name, ".png"))
			assert.LessOrEqual(t, len(task.Name), 235)
			if length <= 200 {
				assert.Equal(t, length+len(".png"), len(task.Name))
			} else {
				assert.Equal(t, 235, len(task.Name))
			}
		})
	}
}

func TestEngine_RetriesEndpointPreparation(t *testing.T) {
	data := []byte("retry succeeded")
	driver := &flakyPrepareDriver{data: data}
	store := &mockTaskStore{taskInfo: &Task{
		ID:           1,
		Name:         "retry.bin",
		SavePath:     t.TempDir(),
		ResourceType: ResourceTypeFile,
		ResourceID:   1,
		Endpoints: []Endpoint{{
			ID:       1,
			Protocol: "flaky-prepare",
			URL:      "flaky-prepare://test/retry.bin",
		}},
	}}
	tracker := &eventTracker{}
	engine := New(store, tracker.record, 1)
	engine.RegisterProtocol(driver)

	if err := engine.Start(1); err != nil {
		t.Fatal(err)
	}
	if !tracker.waitFor(EventFinished, 5*time.Second) {
		t.Fatalf("端点探测重试后未完成下载，事件: %v", tracker.snapshot())
	}
	assert.Equal(t, maxReadAttempts, driver.prepareAttempts())
	content, err := os.ReadFile(filepath.Join(store.taskInfo.SavePath, store.taskInfo.Name))
	assert.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestEngine_DownloadsCollectionResources(t *testing.T) {
	data := []byte("multi-resource")
	saveDir := t.TempDir()
	store := &mockTaskStore{taskInfo: &Task{
		ID:           1,
		Name:         "video.bin",
		SavePath:     saveDir,
		ResourceType: ResourceTypeCollection,
		Resources: []Resource{
			{ID: 11, Name: "video.bin", Endpoints: []Endpoint{{Protocol: "memory", URL: "memory://video"}}},
			{ID: 12, Name: "cover.jpg", Endpoints: []Endpoint{{Protocol: "memory", URL: "memory://cover"}}},
		},
	}}
	tracker := &eventTracker{}
	engine := New(store, tracker.record, 1)
	engine.RegisterProtocol(&memoryProtocolDriver{data: data})

	if err := engine.Start(1); err != nil {
		t.Fatal(err)
	}
	if !tracker.waitFor(EventFinished, 5*time.Second) {
		t.Fatalf("多资源任务未完成，事件: %v", tracker.snapshot())
	}
	for _, name := range []string{"video.bin", "cover.jpg"} {
		content, err := os.ReadFile(filepath.Join(saveDir, name))
		assert.NoError(t, err)
		assert.Equal(t, data, content)
	}
	assert.Equal(t, 1, store.finishCalls)
}

func TestEngine_DownloadWithProgress(t *testing.T) {
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
		taskInfo: &Task{
			ID:       1,
			Name:     "test.bin",
			SavePath: saveDir,
			URL:      ts.URL,
		},
	}

	// event tracker
	tracker := &eventTracker{}

	d := New(store, tracker.record, 1)
	d.RegisterProtocol(&testHTTPDriver{})

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

func TestEngine_FileSmallerThanBuffer(t *testing.T) {
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
		taskInfo: &Task{
			ID:       1,
			Name:     "small.bin",
			SavePath: saveDir,
			URL:      ts.URL,
		},
	}

	tracker := &eventTracker{}
	d := New(store, tracker.record, 1)
	d.RegisterProtocol(&testHTTPDriver{})

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

func TestEngine_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "empty-source.bin")
	if err := createTempFile(source, 0); err != nil {
		t.Fatal(err)
	}
	ts := startFileServer(t, source, "empty.bin")
	defer ts.Close()

	store := &mockTaskStore{taskInfo: &Task{
		ID: 1, Name: "empty.bin", SavePath: filepath.Join(tmpDir, "downloads"), URL: ts.URL, ResourceID: 1,
	}}
	tracker := &eventTracker{}
	d := New(store, tracker.record, 1)
	d.RegisterProtocol(&testHTTPDriver{})
	if err := d.Start(1); err != nil {
		t.Fatal(err)
	}
	if !tracker.waitFor(EventFinished, 5*time.Second) {
		t.Fatalf("空文件下载未完成, 事件: %v", tracker.snapshot())
	}
	fileInfo, err := os.Stat(filepath.Join(store.taskInfo.SavePath, "empty.bin"))
	assert.NoError(t, err)
	if err == nil {
		assert.Zero(t, fileInfo.Size())
	}
}

func TestEngine_ConcurrencyLimit(t *testing.T) {
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
			taskInfo: &Task{
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
		d := New(stores[i], trackers[i].record, 3)
		d.RegisterProtocol(&testHTTPDriver{})
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

func TestEngine_PauseAndResume(t *testing.T) {
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	// 创建一个慢速 HTTP 服务器，确保 pause 有足够时间生效
	ts := startSlowServer(t, 5*1024*1024) // 5MB data at slow pace
	defer ts.Close()

	store := &mockTaskStore{
		taskInfo: &Task{
			ID:       1,
			Name:     "pause_test.bin",
			SavePath: saveDir,
			URL:      ts.URL,
		},
	}

	tracker := &eventTracker{}
	d := New(store, tracker.record, 1)
	d.RegisterProtocol(&testHTTPDriver{})

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
	d2 := New(store, tracker.record, 1)
	d2.RegisterProtocol(&testHTTPDriver{})
	if err := d2.Start(1); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	// 等待完成
	if !tracker.waitFor(EventFinished, 30*time.Second) {
		t.Fatal("恢复后下载未完成")
	}
}

func TestEngine_LoadTaskError(t *testing.T) {
	store := &mockTaskStore{
		loadTaskErr: errors.New("load error"),
	}
	tracker := &eventTracker{}
	d := New(store, tracker.record, 1)

	if err := d.Start(1); err != nil {
		t.Fatalf("Start 不应返回错误: %v", err)
	}

	if !tracker.waitFor(EventFailed, 5*time.Second) {
		t.Fatal("未收到 failed 事件")
	}

	assert.Contains(t, tracker.snapshot(), EventFailed)
	assert.Equal(t, TaskStatusFailed, store.lastStatus())
	assert.Equal(t, logCall{
		taskID:  1,
		level:   "error",
		message: "加载任务信息失败: load error",
	}, store.lastLog())
}

func TestEngine_EventSequence(t *testing.T) {
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
		taskInfo: &Task{
			ID:       1,
			Name:     "sequence.bin",
			SavePath: saveDir,
			URL:      ts.URL,
		},
	}

	tracker := &eventTracker{}
	d := New(store, tracker.record, 1)
	d.RegisterProtocol(&testHTTPDriver{})

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

func TestEngine_MultiSegmentConcurrent(t *testing.T) {
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
		taskInfo: &Task{
			ID:         1,
			Name:       "multi_seg.bin",
			SavePath:   saveDir,
			URL:        ts.URL,
			ResourceID: 1,
		},
	}

	tracker := &eventTracker{}
	d := New(store, tracker.record, 10)
	d.RegisterProtocol(&testHTTPDriver{})

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

func TestEngine_ServerWithoutRangeUsesSingleDownload(t *testing.T) {
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	data := bytes.Repeat([]byte("native-downloader-no-range\n"), 80*1024)

	var mu sync.Mutex
	getCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		mu.Lock()
		getCount++
		mu.Unlock()
		// 故意忽略 Range 并返回 200，验证下载器不会把完整响应写入每个分片。
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	store := &mockTaskStore{taskInfo: &Task{
		ID: 1, Name: "no-range.bin", SavePath: saveDir, URL: ts.URL, ResourceID: 1,
	}}
	tracker := &eventTracker{}
	d := New(store, tracker.record, 1)
	d.RegisterProtocol(&testHTTPDriver{})
	if err := d.Start(1); err != nil {
		t.Fatalf("启动下载失败: %v", err)
	}
	if !tracker.waitFor(EventFinished, 15*time.Second) {
		t.Fatalf("下载未完成, 事件: %v", tracker.snapshot())
	}

	got, err := os.ReadFile(filepath.Join(saveDir, "no-range.bin"))
	assert.NoError(t, err)
	assert.Equal(t, data, got)
	mu.Lock()
	assert.Equal(t, 2, getCount, "应只有一次 Range 能力探测和一次实际下载")
	mu.Unlock()
}

func TestSplitFileDoesNotCreateEmptySegments(t *testing.T) {
	ranges := splitFile(3, defaultSegmentCount)
	assert.Len(t, ranges, 3)
	assert.Equal(t, []SegmentRange{
		{Index: 0, OffsetStart: 0, OffsetEnd: 0, Size: 1},
		{Index: 1, OffsetStart: 1, OffsetEnd: 1, Size: 1},
		{Index: 2, OffsetStart: 2, OffsetEnd: 2, Size: 1},
	}, ranges)
}

func TestTaskFilePathCannotEscapeSaveDirectory(t *testing.T) {
	path, err := taskFilePath(&Task{Name: "../../video.mp4", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/downloads", "video.mp4"), path)

	path, err = taskFilePath(&Task{Name: "../video.mp4", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/downloads", "video.mp4"), path)

	path, err = taskFilePath(&Task{
		Name:         "video.mp4",
		SavePath:     filepath.Join("/downloads", "video.mp4"),
		ResourceType: "FILE",
	}, "https://example.com/ignored")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/downloads", "video.mp4"), path)

	_, err = taskFilePath(&Task{Name: "..", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.Error(t, err)

	_, err = taskFilePath(&Task{Name: ".", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.Error(t, err)

	path, err = taskFilePath(&Task{Name: "chapters/0001.html", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/downloads", "chapters", "0001.html"), path)
}

func TestEngine_RegisteredProtocolAndEndpointFallback(t *testing.T) {
	data := []byte("downloaded by a registered protocol driver")
	store := &mockTaskStore{taskInfo: &Task{
		ID: 1, Name: "plugin.bin", SavePath: t.TempDir(), ResourceID: 1,
		Endpoints: []Endpoint{
			{ID: 1, Protocol: "failing", URL: "failing://resource", Priority: 0},
			{Protocol: "memory", URL: "memory://resource", Priority: 1},
		},
	}}
	tracker := &eventTracker{}
	d := New(store, tracker.record, 1)
	d.RegisterProtocol(&failingProtocolDriver{size: int64(len(data))})
	d.RegisterProtocol(&memoryProtocolDriver{data: data})

	if err := d.Start(1); err != nil {
		t.Fatalf("启动下载失败: %v", err)
	}
	if !tracker.waitFor(EventFinished, 5*time.Second) {
		t.Fatalf("插件协议下载未完成, 事件: %v", tracker.snapshot())
	}
	got, err := os.ReadFile(filepath.Join(store.taskInfo.SavePath, "plugin.bin"))
	assert.NoError(t, err)
	assert.Equal(t, data, got)
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
