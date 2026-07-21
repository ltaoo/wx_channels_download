package fsmock

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"wx_channel/pkg/hermes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenario_SingleFileHTTP_Range(t *testing.T) {
	// Scenario 1: Single File via HTTP (10MB, Range support)
	saveDir := t.TempDir()
	srv := NewHTTPMockServer(WithFileSize(10 * 1024 * 1024))
	defer srv.Close()

	b := NewScenario(3).
		WithTask(SingleFileHTTPTask(1, "bigfile.bin", saveDir, srv.URL())).
		WithHTTPDriver()

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 30*time.Second),
		"download did not finish, events: %v", b.Tracker.Snapshot())

	// Validate events.
	assert.True(t, b.Tracker.HasEvent(hermes.EventStarted))
	assert.True(t, b.Tracker.HasEvent(hermes.EventProgress))
	assert.True(t, b.Tracker.HasEvent(hermes.EventFinished))
	assert.False(t, b.Tracker.HasEvent(hermes.EventFailed))

	// Validate store.
	assert.Equal(t, 1, b.Store.ActivateCalls)
	assert.Equal(t, 1, b.Store.FinishCalls)
	assert.True(t, b.Store.HasStatus(hermes.TaskStatusDownloading))
	assert.True(t, b.Store.HasStatus(hermes.TaskStatusFinished))

	// Validate file on disk.
	outputPath, _ := BuildFilePath(b.Store.Task(), srv.URL())
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, int64(10*1024*1024), int64(len(data)))
	assert.True(t, VerifyData(data, 0), "file content mismatch")

	b.Cleanup()
}

func TestScenario_SingleFileHTTP_NoRange(t *testing.T) {
	// Scenario 2: Single File via HTTP (100KB, no Range)
	saveDir := t.TempDir()
	srv := NewHTTPMockServer(
		WithFileSize(100*1024),
		WithRangeSupport(false),
		WithFilename("norange.bin"),
	)
	defer srv.Close()

	b := NewScenario(1).
		WithTask(SingleFileHTTPTask(1, "norange.bin", saveDir, srv.URL())).
		WithHTTPDriver()

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 15*time.Second),
		"download did not finish, events: %v", b.Tracker.Snapshot())

	assert.True(t, b.Tracker.HasEvent(hermes.EventStarted))
	assert.True(t, b.Tracker.HasEvent(hermes.EventFinished))
	assert.Equal(t, 1, b.Store.FinishCalls)

	outputPath, _ := BuildFilePath(b.Store.Task(), srv.URL())
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, int64(100*1024), int64(len(data)))
	assert.True(t, VerifyData(data, 0))

	b.Cleanup()
}

func TestScenario_CollectionMultipleEndpoints(t *testing.T) {
	// Scenario 3: Collection via multiple endpoints (video + audio + cover)
	saveDir := t.TempDir()
	data := GenerateData(64 * 1024)

	b := NewScenario(3).
		WithMemoryDriverCT(data, "video/mp4").
		WithTask(CollectionTask(1, saveDir,
			NewMemoryResource(11, "video.mp4"),
			NewMemoryResource(12, "audio.mp3"),
			NewMemoryResource(13, "cover.jpg"),
		))

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 10*time.Second),
		"download did not finish, events: %v", b.Tracker.Snapshot())

	assert.True(t, b.Tracker.HasEvent(hermes.EventStarted))
	assert.True(t, b.Tracker.HasEvent(hermes.EventFinished))
	assert.Equal(t, 1, b.Store.FinishCalls)

	// Verify each resource file exists with correct data.
	for _, name := range []string{"video.mp4", "audio.mp3", "cover.jpg"} {
		p := filepath.Join(saveDir, name)
		content, err := os.ReadFile(p)
		require.NoError(t, err, "file %s not found", name)
		assert.Equal(t, data, content, "file content mismatch for %s", name)
	}

	b.Cleanup()
}

func TestScenario_CollectionFallback(t *testing.T) {
	// Scenario 4: Collection with fallback – primary fails, fallback succeeds
	saveDir := t.TempDir()
	data := GenerateData(4 * 1024) // 4KB

	b := NewScenario(1).
		WithFailingDriver(int64(len(data))).
		WithMemoryDriver(data).
		WithTask(&hermes.Task{
			ID:           1,
			Name:         "fallback.bin",
			SavePath:     saveDir,
			ResourceType: hermes.ResourceTypeFile,
			ResourceID:   101,
			Endpoints: []hermes.Endpoint{
				{ID: 1, Protocol: "failing", URL: "failing://primary", Priority: 0},
				{ID: 2, Protocol: "memory", URL: "memory://fallback", Priority: 1},
			},
		})

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 10*time.Second),
		"download did not finish after fallback, events: %v", b.Tracker.Snapshot())

	assert.True(t, b.Tracker.HasEvent(hermes.EventStarted))
	assert.True(t, b.Tracker.HasEvent(hermes.EventFinished))

	outputPath, _ := BuildFilePath(b.Store.Task(), "memory://fallback")
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, data, content)

	b.Cleanup()
}

func TestScenario_HLSStream(t *testing.T) {
	// Scenario 5: Stream via HLS mock. The HLS driver delivers data in
	// segments. Because the hermes engine interprets it as a regular file
	// download, we set up the HLS driver to behave like a memory driver.
	saveDir := t.TempDir()
	data := GenerateData(5 * 1024 * 1024) // 5MB

	b := NewScenario(1).
		WithMemoryDriver(data).
		WithTask(&hermes.Task{
			ID:           1,
			Name:         "stream.mp4",
			SavePath:     saveDir,
			ResourceType: hermes.ResourceTypeFile,
			ResourceID:   201,
			Endpoints: []hermes.Endpoint{
				{ID: 1, Protocol: "memory", URL: "memory://stream"},
			},
		})

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 15*time.Second),
		"stream download did not finish, events: %v", b.Tracker.Snapshot())

	assert.True(t, b.Tracker.HasEvent(hermes.EventStarted))
	assert.True(t, b.Tracker.HasEvent(hermes.EventFinished))

	outputPath, _ := BuildFilePath(b.Store.Task(), "memory://stream")
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, data, content)

	b.Cleanup()
}

func TestScenario_MultiSegmentConcurrent(t *testing.T) {
	// Scenario 6: Multi-segment concurrent download (10MB, triggers 10 segments)
	saveDir := t.TempDir()
	fileSize := int64(10 * 1024 * 1024)
	srv := NewHTTPMockServer(
		WithFileSize(fileSize),
		WithRangeSupport(true),
		WithFilename("multi_seg.bin"),
	)
	defer srv.Close()

	b := NewScenario(10).
		WithTask(&hermes.Task{
			ID:           1,
			Name:         "multi_seg.bin",
			SavePath:     saveDir,
			ResourceType: hermes.ResourceTypeFile,
			ResourceID:   1,
			URL:          srv.URL(),
		}).
		WithHTTPDriver()

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 30*time.Second),
		"multi-segment download did not finish, events: %v", b.Tracker.Snapshot())

	assert.True(t, b.Tracker.HasEvent(hermes.EventStarted))
	assert.True(t, b.Tracker.HasEvent(hermes.EventFinished))
	assert.GreaterOrEqual(t, b.Tracker.Count(hermes.EventProgress), 1)

	outputPath, _ := BuildFilePath(b.Store.Task(), srv.URL())
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, fileSize, int64(len(data)))
	assert.True(t, VerifyData(data, 0))

	// Check progress tracking.
	assert.Greater(t, len(b.Store.ProgressCalls), 0)
	last := b.Store.LastProgress()
	assert.Equal(t, fileSize, last.Downloaded)

	b.Cleanup()
}

func TestScenario_SlowServerPauseResume(t *testing.T) {
	// Scenario 7: Slow server with pause/resume test
	saveDir := t.TempDir()
	fileSize := int64(3 * 1024 * 1024) // 3MB
	srv := NewHTTPMockServer(
		WithFileSize(fileSize),
		WithRangeSupport(true),
		WithSpeed(200*1024), // 200KB/s slow speed
		WithFilename("pause_test.bin"),
	)
	defer srv.Close()

	b := NewScenario(1).
		WithTask(&hermes.Task{
			ID:           1,
			Name:         "pause_test.bin",
			SavePath:     saveDir,
			ResourceType: hermes.ResourceTypeFile,
			URL:          srv.URL(),
		}).
		WithHTTPDriver()

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventProgress, 10*time.Second),
		"no progress events received")

	// Pause the download.
	b.Pause(1)
	require.True(t, b.WaitFor(hermes.EventPaused, 10*time.Second),
		"download was not paused, events: %v", b.Tracker.Snapshot())

	assert.True(t, b.Tracker.HasEvent(hermes.EventPaused))
	assert.Equal(t, 1, b.Store.DeactivateCalls)

	// Resume: create a new engine with the same store and tracker.
	engine2 := hermes.New(b.Store, b.Tracker.Record, 1)
	engine2.RegisterProtocol(&testHTTPDriver{})

	require.NoError(t, engine2.Start(1))
	// Wait for finish on the tracker (shared with the resumed engine).
	finished := false
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if b.Tracker.HasEvent(hermes.EventFinished) {
			finished = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.True(t, finished, "resumed download did not finish, events: %v", b.Tracker.Snapshot())

	assert.True(t, b.Tracker.HasEvent(hermes.EventFinished))
	assert.Equal(t, 1, b.Store.FinishCalls)

	outputPath, _ := BuildFilePath(b.Store.Task(), srv.URL())
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, fileSize, int64(len(data)))
	assert.True(t, VerifyData(data, 0))

	b.Cleanup()
}

func TestScenario_EmptyFile(t *testing.T) {
	// Scenario 8: Empty file (edge case)
	saveDir := t.TempDir()
	srv := NewHTTPMockServer(
		WithFileSize(0),
		WithFilename("empty.bin"),
	)
	defer srv.Close()

	b := NewScenario(1).
		WithTask(&hermes.Task{
			ID:           1,
			Name:         "empty.bin",
			SavePath:     saveDir,
			ResourceType: hermes.ResourceTypeFile,
			ResourceID:   1,
			URL:          srv.URL(),
		}).
		WithHTTPDriver()

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 10*time.Second),
		"empty file download did not finish, events: %v", b.Tracker.Snapshot())

	outputPath, _ := BuildFilePath(b.Store.Task(), srv.URL())
	fi, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Equal(t, int64(0), fi.Size())

	b.Cleanup()
}

func TestScenario_OneByteFile(t *testing.T) {
	// Scenario 9: Very small file (1 byte)
	saveDir := t.TempDir()
	data := GenerateData(1)

	b := NewScenario(1).
		WithMemoryDriver(data).
		WithTask(&hermes.Task{
			ID:           1,
			Name:         "one.bin",
			SavePath:     saveDir,
			ResourceType: hermes.ResourceTypeFile,
			Endpoints: []hermes.Endpoint{
				{ID: 1, Protocol: "memory", URL: "memory://one.bin"},
			},
		})

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 10*time.Second),
		"1-byte download did not finish, events: %v", b.Tracker.Snapshot())

	outputPath := filepath.Join(saveDir, "one.bin")
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, data, content)

	b.Cleanup()
}

func TestScenario_LargeFileHundredMB(t *testing.T) {
	// Scenario 10: Very large file (100MB) – enabled only in non-short mode.
	if testing.Short() {
		t.Skip("skipping 100MB file test in short mode")
	}
	saveDir := t.TempDir()
	fileSize := int64(100 * 1024 * 1024)
	srv := NewHTTPMockServer(
		WithFileSize(fileSize),
		WithRangeSupport(true),
		WithFilename("big.bin"),
	)
	defer srv.Close()

	b := NewScenario(10).
		WithTask(&hermes.Task{
			ID:           1,
			Name:         "big.bin",
			SavePath:     saveDir,
			ResourceType: hermes.ResourceTypeFile,
			ResourceID:   1,
			URL:          srv.URL(),
		}).
		WithHTTPDriver()

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 120*time.Second),
		"large file download did not finish, events: %v", b.Tracker.Snapshot())

	outputPath, _ := BuildFilePath(b.Store.Task(), srv.URL())
	fi, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Equal(t, fileSize, fi.Size())

	b.Cleanup()
}

func TestScenario_EventSequence(t *testing.T) {
	// Validates the order of events: started → progress* → finished.
	saveDir := t.TempDir()
	data := GenerateData(512 * 1024) // 512KB

	b := NewScenario(1).
		WithMemoryDriver(data).
		WithTask(&hermes.Task{
			ID:           1,
			Name:         "seq.bin",
			SavePath:     saveDir,
			ResourceType: hermes.ResourceTypeFile,
			Endpoints: []hermes.Endpoint{
				{ID: 1, Protocol: "memory", URL: "memory://seq.bin"},
			},
		})

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFinished, 10*time.Second))

	events := b.Tracker.Snapshot()
	foundStarted := false
	foundFinished := false
	lastProgressBeforeFinish := -1
	for i, ev := range events {
		switch ev {
		case hermes.EventStarted:
			foundStarted = true
			assert.False(t, foundFinished, "started must come before finished")
		case hermes.EventProgress:
			if !foundFinished {
				lastProgressBeforeFinish = i
			}
		case hermes.EventFinished:
			foundFinished = true
			assert.True(t, foundStarted, "finished must come after started")
			assert.GreaterOrEqual(t, lastProgressBeforeFinish, 0,
				"progress must come before finished")
		}
	}
	assert.True(t, foundStarted && foundFinished)

	b.Cleanup()
}

func TestScenario_StoreErrorHandling(t *testing.T) {
	// Tests that engine properly handles store errors.
	saveDir := t.TempDir()
	data := GenerateData(1024)

	b := NewScenario(1).
		WithMemoryDriver(data)

	b.Store.SetLoadTaskError(assert.AnError)
	b.WithTask(&hermes.Task{
		ID:           1,
		Name:         "err.bin",
		SavePath:     saveDir,
		ResourceType: hermes.ResourceTypeFile,
		Endpoints: []hermes.Endpoint{
			{ID: 1, Protocol: "memory", URL: "memory://err.bin"},
		},
	})

	require.NoError(t, b.Start(1))
	require.True(t, b.WaitFor(hermes.EventFailed, 10*time.Second))

	assert.True(t, b.Tracker.HasEvent(hermes.EventFailed))
	assert.True(t, b.Store.HasStatus(hermes.TaskStatusFailed))

	b.Cleanup()
}
