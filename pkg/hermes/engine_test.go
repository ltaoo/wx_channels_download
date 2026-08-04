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

	"github.com/rs/zerolog"

	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
)

// ---------------------------------------------------------------------------
// mockTaskStore — in-memory mock implementation of Store
// ---------------------------------------------------------------------------

type mockTaskStore struct {
	mu              sync.Mutex
	taskInfo        *TaskJob
	loadTaskErr     error
	statusCalls     []int
	activateCalls   int
	progressCalls   []progressCall
	resourceSizeVal int64
	nameUpdates     []OutputNameUpdate
	segmentInfo     []Segment
	deactivateCalls int
	finishCalls     int
	recordErrors    []string
}

type progressCall struct {
	taskID     int
	downloaded int64
	speed      int64
}

func (m *mockTaskStore) LoadTask(taskID int) (*TaskJob, error) {
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

func (m *mockTaskStore) RecordError(taskID int, errMsg string) error {
	m.mu.Lock()
	m.recordErrors = append(m.recordErrors, errMsg)
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

func (m *mockTaskStore) outputNameUpdates() []OutputNameUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	updates := make([]OutputNameUpdate, len(m.nameUpdates))
	copy(updates, m.nameUpdates)
	return updates
}

// ---------------------------------------------------------------------------
// eventTracker — onEvent callback tracker
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

// testHTTPDriver is a lightweight HTTP protocol driver for engine testing.
// It uses the standard net/http library instead of tls-client, making it compatible with httptest.Server.
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

func (t *eventTracker) record(taskID int, event EventType, _ *TaskProgress) {
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

func newTestEngine(store Store, logger *zerolog.Logger, handler EventHandler, maxConcurrent int, filenameTemplate string) *HermesEngine {
	engine := New(HermesNewConfig{
		Store:  store,
		Logger: logger,
		Config: HermesEngineConfig{
			MaxConcurrent:    maxConcurrent,
			FilenameTemplate: filenameTemplate,
		},
	})
	engine.SetEventHandler(handler)
	return engine
}

func TestEngineSetEventHandler(t *testing.T) {
	engine := New(HermesNewConfig{Config: HermesEngineConfig{MaxConcurrent: 1}})
	var gotTaskID int
	var gotEvent EventType

	engine.SetEventHandler(func(taskID int, event EventType, _ *TaskProgress) {
		gotTaskID = taskID
		gotEvent = event
	})
	engine.emit(42, EventFinished)

	assert.Equal(t, 42, gotTaskID)
	assert.Equal(t, EventFinished, gotEvent)

	engine.SetEventHandler(nil)
	engine.emit(99, EventFailed)
	assert.Equal(t, 42, gotTaskID)
	assert.Equal(t, EventFinished, gotEvent)
}

func TestEngineInfersExtensionFromContentTypeBeforeWriting(t *testing.T) {
	store := &mockTaskStore{}
	engine := newTestEngine(store, nil, nil, 1, "")
	saveDir := t.TempDir()
	task := &TaskJob{
		ID: 1, Name: "display-cover", SavePath: saveDir,
	}
	resource := &ResourceJob{ID: 2, Name: "display-cover", UniqueID: "cover"}

	extensions := make(map[int]string)
	changed, err := engine.processOutputFilename(task, resource, "https://example.com/media", PreparedResource{ContentType: "image/png"}, resource.Name, extensions)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "cover.tmp", resource.Name)
	assert.Equal(t, ".png", extensions[2])
	assert.Equal(t, []OutputNameUpdate{{
		TaskID:       1,
		ResourceName: "cover.tmp",
	}}, store.outputNameUpdates())
}

func TestEngineInfersExtensionFromContentTypeIgnoresUserExtension(t *testing.T) {
	// Even if the user-provided filename contains ".mp4", it should be treated as a plain filename;
	// the system appends the correct extension based on Content-Type.
	store := &mockTaskStore{}
	engine := newTestEngine(store, nil, nil, 1, "")
	for _, testCase := range []struct {
		name        string
		contentType string
		wantChanged bool
		wantName    string
	}{
		{name: "video.mp4", contentType: "image/png", wantChanged: true, wantName: "video.mp4.tmp"},
		{name: "video", contentType: "application/octet-stream", wantChanged: false, wantName: "video"},
		{name: "playlist", contentType: "application/vnd.apple.mpegurl", wantChanged: true, wantName: "playlist.tmp"},
	} {
		task := &TaskJob{ID: 1, Name: testCase.name, SavePath: filepath.Join(t.TempDir(), testCase.name)}
		resource := &ResourceJob{ID: 2, Name: testCase.name, UniqueID: testCase.name}
		extensions := make(map[int]string)
		changed, err := engine.processOutputFilename(task, resource, "https://example.com/media", PreparedResource{ContentType: testCase.contentType}, resource.Name, extensions)
		require.NoError(t, err)
		assert.Equal(t, testCase.wantChanged, changed, testCase)
		assert.Equal(t, testCase.wantName, resource.Name)
	}
}

func TestEngineDoesNotRenameResumedResource(t *testing.T) {
	store := &mockTaskStore{segmentInfo: []Segment{{ID: 1, Size: 8, Downloaded: 2}}}
	engine := newTestEngine(store, nil, nil, 1, "")
	task := &TaskJob{
		ID: 1, Name: "author/cover_transformed", SavePath: filepath.Join(t.TempDir(), "cover"),
	}
	resource := &ResourceJob{ID: 2, Name: "author/cover_transformed", UniqueID: "cover"}

	changed, err := engine.processOutputFilename(task, resource, "https://example.com/media", PreparedResource{ContentType: "image/png"}, "cover.tmp", nil)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, "cover.tmp", resource.Name)
	assert.Empty(t, store.outputNameUpdates())
}

func TestDownloadResourceResumePreservesPersistedFilenameBeforeTemplate(t *testing.T) {
	data := bytes.Repeat([]byte("resume-data-"), 128)
	persistedName := filepath.Join("Xinhua", "video_xWT111.tmp")
	saveDir := t.TempDir()
	persistedPath := filepath.Join(saveDir, persistedName)
	if err := os.MkdirAll(filepath.Dir(persistedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(persistedPath+partialFileSuffix, data, 0644); err != nil {
		t.Fatal(err)
	}

	store := &mockTaskStore{segmentInfo: []Segment{{
		ID:          1,
		Index:       0,
		OffsetStart: 0,
		OffsetEnd:   int64(len(data)) - 1,
		Size:        int64(len(data)),
		Downloaded:  int64(len(data)),
	}}}
	engine := newTestEngine(store, nil, nil, 1, "{{author}}/{{filename}}_{{spec}}")
	engine.RegisterProtocol(&memoryProtocolDriver{data: data})
	resource := ResourceJob{
		ID:        2,
		Name:      persistedName,
		UniqueID:  "resume-resource",
		Type:      ResourceTypeFile,
		Extension: ".mp4",
		Extra: map[string]string{
			"author": "Xinhua",
			"spec":   "xWT111",
		},
		Endpoints: []Endpoint{{Protocol: "memory", URL: "memory://video"}},
	}

	gotPath, err := engine.downloadResource(
		context.Background(), &TaskJob{ID: 1, SavePath: saveDir}, &resource,
	)
	require.NoError(t, err)
	assert.Equal(t, persistedPath, gotPath)
	assert.Empty(t, store.outputNameUpdates())
	got, err := os.ReadFile(persistedPath)
	require.NoError(t, err)
	assert.Equal(t, data, got)
	if _, err := os.Stat(filepath.Join(saveDir, "Xinhua", persistedName+"_xWT111")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resume unexpectedly created a template-derived path: %v", err)
	}
}

func TestDownloadResourceRejectsMissingUniqueID(t *testing.T) {
	engine := newTestEngine(&mockTaskStore{}, nil, nil, 1, "")
	_, err := engine.downloadResource(
		context.Background(),
		&TaskJob{ID: 1, SavePath: t.TempDir()},
		&ResourceJob{ID: 2, Name: "video"},
	)
	require.EqualError(t, err, "resource unique ID is required")
}

func TestEngineInfersExtensionForLongFilenames(t *testing.T) {
	for _, length := range []int{100, 200, 300} {
		t.Run(fmt.Sprintf("%d_characters", length), func(t *testing.T) {
			store := &mockTaskStore{}
			engine := newTestEngine(store, nil, nil, 1, "")
			name := strings.Repeat("a", length)
			task := &TaskJob{
				ID: 1, Name: name, SavePath: filepath.Join(t.TempDir(), name),
			}
			resource := &ResourceJob{ID: 2, Name: name, UniqueID: name}

			extensions := make(map[int]string)
			changed, err := engine.processOutputFilename(task, resource, "https://example.com/media", PreparedResource{ContentType: "image/png"}, resource.Name, extensions)
			require.NoError(t, err)
			assert.True(t, changed)
			assert.True(t, strings.HasSuffix(resource.Name, ".tmp"))
			assert.Equal(t, ".png", extensions[2])
			assert.LessOrEqual(t, len(resource.Name), 235)
			if length <= 200 {
				assert.Equal(t, length+len(".tmp"), len(resource.Name))
			} else {
				assert.Equal(t, 235, len(resource.Name))
			}
		})
	}
}

func TestDetectContentTypeFromBytes(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		data     []byte
		wantType string
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, "image/png"},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg"},
		{"gif", []byte{'G', 'I', 'F', '8', '9', 'a'}, "image/gif"},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBP"), "image/webp"},
		{"mp4", []byte{0x00, 0x00, 0x00, 0x00, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, "video/mp4"},
		{"mp3_id3", []byte{'I', 'D', '3', 0x04, 0x00, 0x00}, "audio/mpeg"},
		{"mp3_sync", []byte{0xFF, 0xFB, 0x90, 0x00}, "audio/mpeg"},
		{"pdf", []byte("%PDF-1.4"), "application/pdf"},
		{"zip", []byte{'P', 'K', 0x03, 0x04}, "application/zip"},
		{"empty", []byte{}, ""},
		{"unknown", []byte("some random data"), ""},
	} {
		got := detectContentTypeFromBytes(testCase.data)
		assert.Equal(t, testCase.wantType, got, testCase.name)
	}
}

func TestEngineExtensionFallback(t *testing.T) {
	store := &mockTaskStore{}
	engine := newTestEngine(store, nil, nil, 1, "")

	// Neither Content-Type nor magic bytes are available → use the user-specified fallback
	task := &TaskJob{
		ID: 1, Name: "myfile", SavePath: t.TempDir(),
	}
	resource := &ResourceJob{ID: 2, Name: "myfile", UniqueID: "myfile", Extension: ".mp4"}
	extensions := make(map[int]string)
	changed, err := engine.processOutputFilename(task, resource, "https://example.com/media",
		PreparedResource{ContentType: ""}, resource.Name, extensions)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "myfile.tmp", resource.Name)
	assert.Equal(t, ".mp4", extensions[2])
}

func TestEngineMagicBytesTakesPriorityOverFallback(t *testing.T) {
	store := &mockTaskStore{}
	engine := newTestEngine(store, nil, nil, 1, "")

	// Content-Type is empty, but ProbeData has PNG magic bytes → use magic bytes result
	task := &TaskJob{
		ID: 1, Name: "photo", SavePath: t.TempDir(),
	}
	resource := &ResourceJob{ID: 2, Name: "photo", UniqueID: "photo", Extension: ".mp4"}
	prepared := PreparedResource{
		ProbeData: []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A},
	}
	extensions := make(map[int]string)
	changed, err := engine.processOutputFilename(task, resource, "https://example.com/media", prepared, resource.Name, extensions)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "photo.tmp", resource.Name) // saved as .tmp; extension is renamed during the finishTask phase
	assert.Equal(t, ".png", extensions[2])      // should use magic bytes result, not the fallback .mp4
}

func TestEngineContentTypeTakesPriorityOverMagicBytes(t *testing.T) {
	store := &mockTaskStore{}
	engine := newTestEngine(store, nil, nil, 1, "")

	// Both Content-Type and magic bytes are available → Content-Type takes priority
	task := &TaskJob{
		ID: 1, Name: "file", SavePath: t.TempDir(),
	}
	resource := &ResourceJob{ID: 2, Name: "file", UniqueID: "file"}
	prepared := PreparedResource{
		ContentType: "image/jpeg",
		ProbeData:   []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, // actually PNG magic bytes
	}
	extensions := make(map[int]string)
	changed, err := engine.processOutputFilename(task, resource, "https://example.com/media", prepared, resource.Name, extensions)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "file.tmp", resource.Name) // saved as .tmp; extension is renamed during the finishTask phase
	assert.Equal(t, ".jpg", extensions[2])     // Content-Type takes priority
}

func TestEngine_RetriesEndpointPreparation(t *testing.T) {
	data := []byte("retry succeeded")
	driver := &flakyPrepareDriver{data: data}
	store := &mockTaskStore{taskInfo: &TaskJob{
		ID:       1,
		Name:     "retry.bin",
		SavePath: t.TempDir(),
		Resources: []ResourceJob{{
			ID:       1,
			Name:     "retry.bin",
			UniqueID: "retry.bin",
			Endpoints: []Endpoint{{
				ID:       1,
				Protocol: "flaky-prepare",
				URL:      "flaky-prepare://test/retry.bin",
			}},
		}},
	}}
	tracker := &eventTracker{}
	engine := newTestEngine(store, nil, tracker.record, 1, "")
	engine.RegisterProtocol(driver)

	if err := engine.StartTask(1); err != nil {
		t.Fatal(err)
	}
	if !tracker.waitFor(EventFinished, 5*time.Second) {
		t.Fatalf("download did not complete after retrying endpoint preparation; events: %v", tracker.snapshot())
	}
	assert.Equal(t, maxReadAttempts, driver.prepareAttempts())
	content, err := os.ReadFile(filepath.Join(store.taskInfo.SavePath, store.taskInfo.Name))
	assert.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestEngine_DownloadsCollectionResources(t *testing.T) {
	data := []byte("multi-resource")
	saveDir := t.TempDir()
	store := &mockTaskStore{taskInfo: &TaskJob{
		ID:       1,
		Name:     "video.bin",
		SavePath: saveDir,
		Resources: []ResourceJob{
			{ID: 11, Name: "video.bin", UniqueID: "video.bin", Endpoints: []Endpoint{{Protocol: "memory", URL: "memory://video"}}},
			{ID: 12, Name: "cover.jpg", UniqueID: "cover.jpg", Endpoints: []Endpoint{{Protocol: "memory", URL: "memory://cover"}}},
		},
	}}
	tracker := &eventTracker{}
	engine := newTestEngine(store, nil, tracker.record, 1, "")
	engine.RegisterProtocol(&memoryProtocolDriver{data: data})

	if err := engine.StartTask(1); err != nil {
		t.Fatal(err)
	}
	if !tracker.waitFor(EventFinished, 5*time.Second) {
		t.Fatalf("multi-resource task did not complete; events: %v", tracker.snapshot())
	}
	for _, name := range []string{"video.bin", "cover.jpg"} {
		content, err := os.ReadFile(filepath.Join(saveDir, name))
		assert.NoError(t, err)
		assert.Equal(t, data, content)
	}
	assert.Equal(t, 1, store.finishCalls)
}

func TestEngine_DownloadWithProgress(t *testing.T) {
	// Create a temporary directory and file for testing
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	// Create a test file (5MB, enough to generate multiple onEvent callbacks)
	tmpFile := filepath.Join(tmpDir, "test_data.bin")
	if err := createTempFile(tmpFile, 5*1024*1024); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Start a local HTTP test server
	ts := startFileServer(t, tmpFile, "test.bin")
	defer ts.Close()

	// mock store
	store := &mockTaskStore{
		taskInfo: &TaskJob{
			ID:        1,
			Name:      "test.bin",
			SavePath:  saveDir,
			Resources: []ResourceJob{{ID: 1, Name: "test.bin", UniqueID: "test.bin", Endpoints: []Endpoint{{Protocol: "http", URL: ts.URL}}}},
		},
	}

	// event tracker
	tracker := &eventTracker{}

	d := newTestEngine(store, nil, tracker.record, 1, "")
	d.RegisterProtocol(&testHTTPDriver{})

	// Start the download
	if err := d.StartTask(1); err != nil {
		t.Fatalf("failed to start download: %v", err)
	}

	// Wait for the download to complete
	if !tracker.waitFor(EventFinished, 30*time.Second) {
		events := tracker.snapshot()
		t.Fatalf("download did not complete before timeout; received events: %v", events)
	}

	// Assertions
	events := tracker.snapshot()

	assert.Contains(t, events, EventStarted, "should receive a started event")
	assert.Contains(t, events, EventFinished, "should receive a finished event")

	progressCount := tracker.count(EventProgress)
	assert.GreaterOrEqual(t, progressCount, 1, "should receive a progress event")
	t.Logf("received %d progress events", progressCount)

	// Verify store methods were called
	store.mu.Lock()
	defer store.mu.Unlock()

	assert.Equal(t, 1, store.activateCalls, "ActivateTask should be called once")
	assert.Equal(t, 1, store.finishCalls, "FinishTask should be called once")

	// Verify the downloaded file exists with the correct size
	downloadedFile := filepath.Join(saveDir, "test.bin")
	fi, err := os.Stat(downloadedFile)
	assert.NoError(t, err, "downloaded file should exist")
	if err == nil {
		assert.Equal(t, int64(5*1024*1024), fi.Size(), "downloaded file size should be correct")
	}

	// Verify progress updated downloaded and speed
	assert.Greater(t, len(store.progressCalls), 0, "should receive progress update callbacks")
	if len(store.progressCalls) > 0 {
		last := store.progressCalls[len(store.progressCalls)-1]
		assert.Equal(t, int64(5*1024*1024), last.downloaded, "final downloaded value should equal the file size")
	}

	t.Logf("received %d events in this order: %v", len(events), events)
}

func TestEngine_FileSmallerThanBuffer(t *testing.T) {
	// Test files smaller than 32KB (completed in a single read)
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	tmpFile := filepath.Join(tmpDir, "small.bin")
	if err := createTempFile(tmpFile, 1024); err != nil { // 1KB
		t.Fatalf("failed to create test file: %v", err)
	}

	ts := startFileServer(t, tmpFile, "small.bin")
	defer ts.Close()

	store := &mockTaskStore{
		taskInfo: &TaskJob{
			ID:        1,
			Name:      "small.bin",
			SavePath:  saveDir,
			Resources: []ResourceJob{{ID: 1, Name: "small.bin", UniqueID: "small.bin", Endpoints: []Endpoint{{Protocol: "http", URL: ts.URL}}}},
		},
	}

	tracker := &eventTracker{}
	d := newTestEngine(store, nil, tracker.record, 1, "")
	d.RegisterProtocol(&testHTTPDriver{})

	if err := d.StartTask(1); err != nil {
		t.Fatalf("failed to start download: %v", err)
	}

	if !tracker.waitFor(EventFinished, 10*time.Second) {
		t.Fatal("download did not complete")
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

	store := &mockTaskStore{taskInfo: &TaskJob{
		ID: 1, Name: "empty.bin", SavePath: filepath.Join(tmpDir, "downloads"),
		Resources: []ResourceJob{{ID: 1, Name: "empty.bin", UniqueID: "empty.bin", Endpoints: []Endpoint{{Protocol: "http", URL: ts.URL}}}},
	}}
	tracker := &eventTracker{}
	d := newTestEngine(store, nil, tracker.record, 1, "")
	d.RegisterProtocol(&testHTTPDriver{})
	if err := d.StartTask(1); err != nil {
		t.Fatal(err)
	}
	if !tracker.waitFor(EventFinished, 5*time.Second) {
		t.Fatalf("empty-file download did not complete; events: %v", tracker.snapshot())
	}
	fileInfo, err := os.Stat(filepath.Join(store.taskInfo.SavePath, "empty.bin"))
	assert.NoError(t, err)
	if err == nil {
		assert.Zero(t, fileInfo.Size())
	}
}

func TestEngine_ConcurrencyLimit(t *testing.T) {
	// Test concurrency limit: start 3 tasks simultaneously; all 3 should run
	tmpDir := t.TempDir()

	// Create a small file for multiple tasks to download
	tmpFile := filepath.Join(tmpDir, "shared.bin")
	if err := createTempFile(tmpFile, 500*1024); err != nil { // 500KB
		t.Fatalf("failed to create test file: %v", err)
	}

	ts := startFileServer(t, tmpFile, "shared.bin")
	defer ts.Close()

	trackers := make([]*eventTracker, 3)
	stores := make([]*mockTaskStore, 3)

	for i := 0; i < 3; i++ {
		// Each task gets its own save directory to avoid file conflicts.
		saveDir := filepath.Join(tmpDir, "downloads", fmt.Sprintf("task_%d", i+1))
		os.MkdirAll(saveDir, 0755)
		stores[i] = &mockTaskStore{
			taskInfo: &TaskJob{
				ID:        i + 1,
				Name:      "shared.bin",
				SavePath:  saveDir,
				Resources: []ResourceJob{{ID: i + 1, Name: "shared.bin", UniqueID: "shared.bin", Endpoints: []Endpoint{{Protocol: "http", URL: ts.URL}}}},
			},
		}
		trackers[i] = &eventTracker{}
	}

	// Start multiple download tasks to verify concurrency limit does not block
	for i := 0; i < 3; i++ {
		d := newTestEngine(stores[i], nil, trackers[i].record, 3, "")
		d.RegisterProtocol(&testHTTPDriver{})
		if err := d.StartTask(i + 1); err != nil {
			t.Fatalf("failed to start task %d: %v", i+1, err)
		}
	}

	// Wait for all tasks to complete
	for i := 0; i < 3; i++ {
		if !trackers[i].waitFor(EventFinished, 15*time.Second) {
			t.Fatalf("task %d did not complete", i+1)
		}
	}

	for i := 0; i < 3; i++ {
		assert.Greater(t, trackers[i].count(EventProgress), 0,
			"task %d should have a progress event", i+1)
	}
}

func TestEngine_PauseAndResume(t *testing.T) {
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	// Create a slow HTTP server to ensure pause has enough time to take effect
	ts := startSlowServer(t, 5*1024*1024) // 5MB data at slow pace
	defer ts.Close()

	store := &mockTaskStore{
		taskInfo: &TaskJob{
			ID:        1,
			Name:      "pause_test.bin",
			SavePath:  saveDir,
			Resources: []ResourceJob{{ID: 1, Name: "pause_test.bin", UniqueID: "pause_test.bin", Endpoints: []Endpoint{{Protocol: "http", URL: ts.URL}}}},
		},
	}

	tracker := &eventTracker{}
	d := newTestEngine(store, nil, tracker.record, 1, "")
	d.RegisterProtocol(&testHTTPDriver{})

	if err := d.StartTask(1); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Wait for at least one progress event
	if !tracker.waitFor(EventProgress, 5*time.Second) {
		t.Fatal("did not receive a progress event")
	}

	// Pause
	d.PauseTask(1)

	// Wait for the paused event
	if !tracker.waitFor(EventPaused, 10*time.Second) {
		t.Fatal("did not receive a paused event")
	}

	assert.Contains(t, tracker.snapshot(), EventPaused)
	assert.Equal(t, 1, store.deactivateCalls, "DeactivateConnections should be called when pausing")

	// Resume
	d2 := newTestEngine(store, nil, tracker.record, 1, "")
	d2.RegisterProtocol(&testHTTPDriver{})
	if err := d2.StartTask(1); err != nil {
		t.Fatalf("failed to resume: %v", err)
	}

	// Wait for completion
	if !tracker.waitFor(EventFinished, 30*time.Second) {
		t.Fatal("download did not complete after resuming")
	}
}

func TestEngine_LoadTaskError(t *testing.T) {
	store := &mockTaskStore{
		loadTaskErr: errors.New("load error"),
	}
	tracker := &eventTracker{}
	d := newTestEngine(store, nil, tracker.record, 1, "")

	if err := d.StartTask(1); err != nil {
		t.Fatalf("Start should not return an error: %v", err)
	}

	if !tracker.waitFor(EventFailed, 5*time.Second) {
		t.Fatal("did not receive a failed event")
	}

	assert.Contains(t, tracker.snapshot(), EventFailed)
	assert.Equal(t, TaskStatusFailed, store.lastStatus())
	assert.Equal(t, []string{"failed to load task information: load error"}, store.recordErrors)
}

func TestEngine_EventSequence(t *testing.T) {
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	tmpFile := filepath.Join(tmpDir, "sequence.bin")
	if err := createTempFile(tmpFile, 1*1024*1024); err != nil { // 1MB
		t.Fatalf("failed to create test file: %v", err)
	}

	ts := startFileServer(t, tmpFile, "sequence.bin")
	defer ts.Close()

	store := &mockTaskStore{
		taskInfo: &TaskJob{
			ID:        1,
			Name:      "sequence.bin",
			SavePath:  saveDir,
			Resources: []ResourceJob{{ID: 1, Name: "sequence.bin", UniqueID: "sequence.bin", Endpoints: []Endpoint{{Protocol: "http", URL: ts.URL}}}},
		},
	}

	tracker := &eventTracker{}
	d := newTestEngine(store, nil, tracker.record, 1, "")
	d.RegisterProtocol(&testHTTPDriver{})

	if err := d.StartTask(1); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !tracker.waitFor(EventFinished, 15*time.Second) {
		t.Fatal("download did not complete")
	}

	events := tracker.snapshot()

	// Verify event sequence: started -> progress* -> finished
	foundStarted := false
	foundFinished := false
	lastProgressBeforeFinish := -1

	for i, ev := range events {
		switch ev {
		case EventStarted:
			foundStarted = true
			assert.False(t, foundFinished, "started should occur before finished")
		case EventProgress:
			if !foundFinished {
				lastProgressBeforeFinish = i
			}
		case EventFinished:
			foundFinished = true
			assert.True(t, foundStarted, "finished should occur after started")
			assert.GreaterOrEqual(t, lastProgressBeforeFinish, 0, "a progress event should occur before finished")
		}
	}

	assert.True(t, foundStarted && foundFinished, "should include started and finished events")
}

func TestEngine_MultiSegmentConcurrent(t *testing.T) {
	// Test multi-segment concurrent download: default 10 segments; for a 10MB file each segment is ~1MB
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	os.MkdirAll(saveDir, 0755)

	// Create a 10MB temporary file (10 segments, ~1MB each)
	fileSize := int64(10 * 1024 * 1024)
	tmpFile := filepath.Join(tmpDir, "multi_seg.bin")
	if err := createTempFile(tmpFile, fileSize); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ts := startFileServer(t, tmpFile, "multi_seg.bin")
	defer ts.Close()

	store := &mockTaskStore{
		taskInfo: &TaskJob{
			ID:        1,
			Name:      "multi_seg.bin",
			SavePath:  saveDir,
			Resources: []ResourceJob{{ID: 1, Name: "multi_seg.bin", UniqueID: "multi_seg.bin", Endpoints: []Endpoint{{Protocol: "http", URL: ts.URL}}}},
		},
	}

	tracker := &eventTracker{}
	d := newTestEngine(store, nil, tracker.record, 10, "")
	d.RegisterProtocol(&testHTTPDriver{})

	if err := d.StartTask(1); err != nil {
		t.Fatalf("failed to start download: %v", err)
	}

	// Wait for the download to complete
	if !tracker.waitFor(EventFinished, 30*time.Second) {
		events := tracker.snapshot()
		t.Fatalf("download did not complete before timeout; events: %v", events)
	}

	events := tracker.snapshot()

	assert.Contains(t, events, EventStarted, "should receive a started event")
	assert.Contains(t, events, EventFinished, "should receive a finished event")

	progressCount := tracker.count(EventProgress)
	assert.GreaterOrEqual(t, progressCount, 1, "should receive a progress event")
	t.Logf("multi-segment download received %d progress events", progressCount)

	// Verify store methods were called
	store.mu.Lock()
	defer store.mu.Unlock()

	assert.Equal(t, 1, store.activateCalls, "ActivateTask should be called once")
	assert.Equal(t, 1, store.finishCalls, "FinishTask should be called once")

	// Verify the downloaded file exists with the correct size
	downloadedFile := filepath.Join(saveDir, "multi_seg.bin")
	fi, err := os.Stat(downloadedFile)
	assert.NoError(t, err, "downloaded file should exist")
	if err == nil {
		assert.Equal(t, fileSize, fi.Size(), "downloaded file size should be correct")
	}

	// Verify progress updated downloaded
	assert.Greater(t, len(store.progressCalls), 0, "should receive progress update callbacks")
	if len(store.progressCalls) > 0 {
		last := store.progressCalls[len(store.progressCalls)-1]
		assert.Equal(t, fileSize, last.downloaded, "final downloaded value should equal the file size")
	}

	t.Logf("received %d events in this order: %v", len(events), events)
}

func TestEngine_ServerWithoutRangeUsesSingleDownload(t *testing.T) {
	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "downloads")
	data := bytes.Repeat([]byte("native-downloader-no-range\n"), 80*1024)

	var mu sync.Mutex
	getCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.Header().Set("Content-Type", "application/octet-stream")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		mu.Lock()
		getCount++
		mu.Unlock()
		// Deliberately ignore Range and return 200, verifying that the downloader does not write the full response into each segment.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	store := &mockTaskStore{taskInfo: &TaskJob{
		ID: 1, Name: "no-range.bin", SavePath: saveDir,
		Resources: []ResourceJob{{ID: 1, Name: "no-range.bin", UniqueID: "no-range.bin", Endpoints: []Endpoint{{Protocol: "http", URL: ts.URL}}}},
	}}
	tracker := &eventTracker{}
	d := newTestEngine(store, nil, tracker.record, 1, "")
	d.RegisterProtocol(&testHTTPDriver{})
	if err := d.StartTask(1); err != nil {
		t.Fatalf("failed to start download: %v", err)
	}
	if !tracker.waitFor(EventFinished, 15*time.Second) {
		t.Fatalf("download did not complete; events: %v", tracker.snapshot())
	}

	got, err := os.ReadFile(filepath.Join(saveDir, "no-range.bin"))
	assert.NoError(t, err)
	assert.Equal(t, data, got)
	mu.Lock()
	assert.Equal(t, 3, getCount, "should perform one size probe, one prepare request, and one actual download")
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
	path, err := taskFilePath(&TaskJob{Name: "../../video.mp4", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/downloads", "video.mp4"), path)

	path, err = taskFilePath(&TaskJob{Name: "../video.mp4", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/downloads", "video.mp4"), path)

	path, err = taskFilePath(&TaskJob{
		Name:     "video.mp4",
		SavePath: "/downloads",
	}, "https://example.com/ignored")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/downloads", "video.mp4"), path)

	_, err = taskFilePath(&TaskJob{Name: "..", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.Error(t, err)

	_, err = taskFilePath(&TaskJob{Name: ".", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.Error(t, err)

	path, err = taskFilePath(&TaskJob{Name: "chapters/0001.html", SavePath: "/downloads"}, "https://example.com/ignored")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/downloads", "chapters", "0001.html"), path)
}

func TestFilePathForResourceUsesUniqueID(t *testing.T) {
	engine := newTestEngine(&mockTaskStore{}, nil, nil, 1, "")
	path, err := engine.filePathForJobResource(
		&TaskJob{Name: "display-name.mp4", SavePath: "/downloads"},
		&ResourceJob{Name: "display-name.mp4", UniqueID: "resource-unique-id"},
		"https://example.com/ignored",
	)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/downloads", "resource-unique-id"), path)
}

func TestEngine_RegisteredProtocolAndEndpointFallback(t *testing.T) {
	data := []byte("downloaded by a registered protocol driver")
	store := &mockTaskStore{taskInfo: &TaskJob{
		ID: 1, Name: "plugin.bin", SavePath: t.TempDir(),
		Resources: []ResourceJob{{ID: 1, Name: "plugin.bin", UniqueID: "plugin.bin", Endpoints: []Endpoint{
			{ID: 1, Protocol: "failing", URL: "failing://resource", Priority: 0},
			{Protocol: "memory", URL: "memory://resource", Priority: 1},
		}}},
	}}
	tracker := &eventTracker{}
	d := newTestEngine(store, nil, tracker.record, 1, "")
	d.RegisterProtocol(&failingProtocolDriver{size: int64(len(data))})
	d.RegisterProtocol(&memoryProtocolDriver{data: data})

	if err := d.StartTask(1); err != nil {
		t.Fatalf("failed to start download: %v", err)
	}
	if !tracker.waitFor(EventFinished, 5*time.Second) {
		t.Fatalf("plugin protocol download did not complete; events: %v", tracker.snapshot())
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

// startSlowServer creates a slow HTTP server used for testing pause/resume functionality.
// It writes only 32KB of data at a time and sleeps 50ms between chunks, ensuring the download takes long enough.
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

// ---------------------------------------------------------------------------
// Filename template syntax tests
// ---------------------------------------------------------------------------

func TestApplyFilenameTemplate_CurlyBraceSyntaxWithSubdirectory(t *testing.T) {
	engine := newTestEngine(nil, nil, nil, 1, "")
	task := &TaskJob{
		Name:             "video.mp4",
		FilenameTemplate: "{{author}}/{{filename}}_{{spec}}",
	}
	resource := &ResourceJob{Name: "video.mp4"}
	meta := map[string]string{
		"author":   "AuthorName",
		"filename": "video",
		"spec":     "1080p",
	}
	result := engine.applyFilenameTemplate(task, resource, "https://example.com/video.mp4", meta)
	assert.Equal(t, "AuthorName/video_1080p", result)
}

func TestApplyFilenameTemplate_CurlyBraceSyntaxWithSpacesAroundSeparator(t *testing.T) {
	engine := newTestEngine(nil, nil, nil, 1, "")
	task := &TaskJob{
		Name:             "video.mp4",
		FilenameTemplate: "{{author}} / {{filename}}_{{spec}}",
	}
	resource := &ResourceJob{Name: "video.mp4"}
	meta := map[string]string{
		"author":   "AuthorName",
		"filename": "video",
		"spec":     "1080p",
	}
	result := engine.applyFilenameTemplate(task, resource, "https://example.com/video.mp4", meta)
	assert.Equal(t, "AuthorName/video_1080p", result)
}

func TestApplyFilenameTemplate_JavascriptExpressionWithSpacesAroundSeparator(t *testing.T) {
	engine := newTestEngine(nil, nil, nil, 1, "")
	task := &TaskJob{
		ID:               42,
		Name:             "video",
		FilenameTemplate: "name + ' / ' + task_id",
	}
	result := engine.applyFilenameTemplate(task, &ResourceJob{Name: "video"}, "https://example.com/video.mp4", nil)
	assert.Equal(t, "video/42", result)
}

func TestApplyFilenameTemplate_JavascriptExpressionWithoutCurlyBrace(t *testing.T) {
	engine := newTestEngine(nil, nil, nil, 1, "")
	task := &TaskJob{
		ID:               42,
		Name:             "video",
		FilenameTemplate: "name + '_' + task_id",
	}
	result := engine.applyFilenameTemplate(task, &ResourceJob{Name: "video"}, "https://example.com/video.mp4", nil)
	assert.Equal(t, "video_42", result)
}

func TestApplyFilenameTemplate_PlainStringWithoutCurlyBraces(t *testing.T) {
	// A JS string literal without {{}} falls through to JS VM evaluation.
	engine := newTestEngine(nil, nil, nil, 1, "")
	task := &TaskJob{
		Name:             "video",
		FilenameTemplate: "'hardcoded_name'",
	}
	result := engine.applyFilenameTemplate(task, &ResourceJob{Name: "video"}, "https://example.com/video.mp4", nil)
	assert.Equal(t, "hardcoded_name", result)
}

func TestBuildTemplateMeta(t *testing.T) {
	extra := map[string]string{
		"id":         "obj_123",
		"title":      "My Video",
		"spec":       "1080p",
		"author":     "AuthorName",
		"created_at": "1700000000",
	}
	config := map[string]any{
		"platform": "wxchannels",
	}

	meta := buildTemplateMeta(extra, config, "video.mp4")

	assert.Equal(t, "obj_123", meta["id"])
	assert.Equal(t, "My Video", meta["title"])
	assert.Equal(t, "1080p", meta["spec"])
	assert.Equal(t, "AuthorName", meta["author"])
	assert.Equal(t, "1700000000", meta["created_at"])
	assert.Equal(t, "video.mp4", meta["filename"])
	assert.NotEmpty(t, meta["download_at"])
}

func TestBuildTemplateMeta_NilExtra(t *testing.T) {
	meta := buildTemplateMeta(nil, nil, "video.mp4")
	assert.Equal(t, "video.mp4", meta["filename"])
	assert.NotEmpty(t, meta["download_at"])
}

func TestProgressSpeedSamplerIgnoresPerReadBursts(t *testing.T) {
	startedAt := time.Unix(100, 0)
	sampler := newProgressSpeedSampler(startedAt, 0)

	// 32 KiB in 10 microseconds would be reported as roughly 3 GiB/s if each
	// read were extrapolated independently. It is intentionally not sampled.
	assert.Equal(t, int64(0), sampler.Sample(startedAt.Add(10*time.Microsecond), 32*1024))
	assert.Equal(t, int64(0), sampler.Sample(startedAt.Add(250*time.Millisecond), 512*1024))

	// Once the 500 ms window is complete, use all bytes transferred in that
	// window. 1 MiB / 0.5 s = 2 MiB/s.
	assert.Equal(t, int64(2*1024*1024), sampler.Sample(startedAt.Add(progressInterval), 1024*1024))
}

func TestProgressSpeedSamplerRetainsLastStableSample(t *testing.T) {
	startedAt := time.Unix(200, 0)
	sampler := newProgressSpeedSampler(startedAt, 0)
	stableSpeed := sampler.Sample(startedAt.Add(progressInterval), 1024*1024)

	// UI updates between sampling boundaries should retain the stable value,
	// not replace it with a per-read spike.
	assert.Equal(t, stableSpeed, sampler.Sample(startedAt.Add(progressInterval+time.Microsecond), 1024*1024+32*1024))
	assert.Equal(t, int64(2*1024*1024), stableSpeed)
}
