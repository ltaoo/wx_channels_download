package hermes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type partialFileTestStore struct {
	mu       sync.Mutex
	segments []Segment
}

func (s *partialFileTestStore) LoadTask(int) (*TaskJob, error)         { return nil, nil }
func (s *partialFileTestStore) UpdateStatus(int, int) error            { return nil }
func (s *partialFileTestStore) ActivateTask(int) error                 { return nil }
func (s *partialFileTestStore) UpdateProgress(int, int64, int64) error { return nil }
func (s *partialFileTestStore) UpdateResourceSize(int, int64) error    { return nil }
func (s *partialFileTestStore) DeactivateConnections(int) error        { return nil }
func (s *partialFileTestStore) FinishTask(int) error                   { return nil }
func (s *partialFileTestStore) RecordError(int, string) error          { return nil }

func (s *partialFileTestStore) CreateSegments(_ int, url string, ranges []SegmentRange) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.segments = make([]Segment, len(ranges))
	ids := make([]int, len(ranges))
	for i, r := range ranges {
		ids[i] = i + 1
		s.segments[i] = Segment{
			ID: ids[i], Index: r.Index, URL: url, OffsetStart: r.OffsetStart,
			OffsetEnd: r.OffsetEnd, Size: r.Size,
		}
	}
	return ids, nil
}

func (s *partialFileTestStore) LoadSegmentInfo(int) ([]Segment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Segment, len(s.segments))
	copy(result, s.segments)
	return result, nil
}

func (s *partialFileTestStore) UpdateSegmentProgress(id int, downloaded int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.segments {
		if s.segments[i].ID == id {
			s.segments[i].Downloaded = downloaded
			return nil
		}
	}
	return fmt.Errorf("unknown segment %d", id)
}

type partialFileRangeDriver struct {
	mu       sync.Mutex
	data     []byte
	requests []ReadRequest
}

func (d *partialFileRangeDriver) Protocols() []string { return []string{"range-memory"} }

func (d *partialFileRangeDriver) Prepare(context.Context, Endpoint) (PreparedResource, error) {
	return PreparedResource{Size: int64(len(d.data)), SupportsRange: true}, nil
}

func (d *partialFileRangeDriver) Open(_ context.Context, _ Endpoint, request ReadRequest) (io.ReadCloser, error) {
	d.mu.Lock()
	d.requests = append(d.requests, request)
	d.mu.Unlock()
	start := request.OffsetStart
	end := request.OffsetEnd
	if !request.UseRange {
		start = 0
		end = int64(len(d.data)) - 1
	}
	if start < 0 || end < start || end >= int64(len(d.data)) {
		return nil, fmt.Errorf("invalid range %d-%d", start, end)
	}
	return io.NopCloser(bytes.NewReader(d.data[start : end+1])), nil
}

func (d *partialFileRangeDriver) openCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.requests)
}

func (d *partialFileRangeDriver) requestSnapshot() []ReadRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make([]ReadRequest, len(d.requests))
	copy(result, d.requests)
	return result
}

func TestDownloadSegmentsWritesRangesIntoSinglePartialFile(t *testing.T) {
	data := bytes.Repeat([]byte("hermes-range-data-"), 8192)
	store := &partialFileTestStore{}
	engine := New(HermesNewConfig{Store: store, Config: HermesEngineConfig{MaxConcurrent: 1, SegmentConcurrency: 4}})
	driver := &partialFileRangeDriver{data: data}
	filePath := filepath.Join(t.TempDir(), "segmented.bin")

	// A stale destination with the right size must not be treated as complete.
	if err := os.WriteFile(filePath, make([]byte, len(data)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := engine.downloadSegments(
		context.Background(), driver, Endpoint{}, filePath,
		&TaskJob{ID: 1}, &ResourceJob{ID: 1},
		PreparedResource{Size: int64(len(data))}, 4,
	); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("downloaded content does not match source")
	}
	if _, err := os.Stat(filePath + partialFileSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful download left partial file: %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := os.Stat(fmt.Sprintf("%s.seg%d", filePath, i)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("segment file %d was created: %v", i, err)
		}
	}
}

func TestDownloadFileDoesNotTrustSameSizedDestinationWithoutCompletedSegment(t *testing.T) {
	data := bytes.Repeat([]byte("single-file-data-"), 1024)
	store := &partialFileTestStore{}
	engine := New(HermesNewConfig{Store: store, Config: HermesEngineConfig{MaxConcurrent: 1}})
	driver := &partialFileRangeDriver{data: data}
	filePath := filepath.Join(t.TempDir(), "single.bin")
	if err := os.WriteFile(filePath, make([]byte, len(data)), 0644); err != nil {
		t.Fatal(err)
	}

	if err := engine.downloadFile(
		context.Background(), driver, Endpoint{}, filePath,
		&TaskJob{ID: 1}, &ResourceJob{ID: 1},
		PreparedResource{Size: int64(len(data)), SupportsRange: true},
	); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("same-sized stale destination was incorrectly reused")
	}
	if _, err := os.Stat(filePath + partialFileSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful download left partial file: %v", err)
	}
}

func TestCompletedPartialFileReplacesOlderSameSizedDestination(t *testing.T) {
	data := bytes.Repeat([]byte("completed-partial-"), 4096)
	ranges := splitFile(int64(len(data)), 3)
	store := &partialFileTestStore{segments: make([]Segment, len(ranges))}
	for i, r := range ranges {
		store.segments[i] = Segment{
			ID: i + 1, Index: r.Index, OffsetStart: r.OffsetStart,
			OffsetEnd: r.OffsetEnd, Size: r.Size, Downloaded: r.Size,
		}
	}
	engine := New(HermesNewConfig{Store: store, Config: HermesEngineConfig{MaxConcurrent: 1, SegmentConcurrency: 3}})
	driver := &partialFileRangeDriver{data: data}
	filePath := filepath.Join(t.TempDir(), "recovered.bin")
	if err := os.WriteFile(filePath, make([]byte, len(data)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath+partialFileSuffix, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := engine.downloadSegments(
		context.Background(), driver, Endpoint{}, filePath,
		&TaskJob{ID: 1}, &ResourceJob{ID: 1},
		PreparedResource{Size: int64(len(data))}, len(ranges),
	); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("completed partial file did not replace stale destination")
	}
	if driver.openCount() != 0 {
		t.Fatalf("recovery unexpectedly opened the download source %d times", driver.openCount())
	}
}

func TestDownloadSegmentsResumesPersistedRangesInPartialFile(t *testing.T) {
	data := bytes.Repeat([]byte("resume-range-"), 8192)
	ranges := splitFile(int64(len(data)), 3)
	resumeBytes := ranges[0].Size / 2
	store := &partialFileTestStore{segments: make([]Segment, len(ranges))}
	for i, r := range ranges {
		store.segments[i] = Segment{
			ID: i + 1, Index: r.Index, OffsetStart: r.OffsetStart,
			OffsetEnd: r.OffsetEnd, Size: r.Size,
		}
	}
	store.segments[0].Downloaded = resumeBytes
	engine := New(HermesNewConfig{Store: store, Config: HermesEngineConfig{MaxConcurrent: 1, SegmentConcurrency: 3}})
	driver := &partialFileRangeDriver{data: data}
	filePath := filepath.Join(t.TempDir(), "resumed.bin")
	partPath := filePath + partialFileSuffix
	part, err := os.OpenFile(partPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if err := part.Truncate(int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if _, err := part.WriteAt(data[:resumeBytes], ranges[0].OffsetStart); err != nil {
		t.Fatal(err)
	}
	if err := part.Close(); err != nil {
		t.Fatal(err)
	}

	if err := engine.downloadSegments(
		context.Background(), driver, Endpoint{}, filePath,
		&TaskJob{ID: 1}, &ResourceJob{ID: 1},
		PreparedResource{Size: int64(len(data))}, len(ranges),
	); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("resumed content does not match source")
	}
	wantOffset := ranges[0].OffsetStart + resumeBytes
	foundResume := false
	for _, request := range driver.requestSnapshot() {
		if request.OffsetStart == wantOffset && request.OffsetEnd == ranges[0].OffsetEnd {
			foundResume = true
			break
		}
	}
	if !foundResume {
		t.Fatalf("no request resumed first segment at offset %d", wantOffset)
	}
}
