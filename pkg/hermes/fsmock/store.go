package fsmock

import (
	"sync"

	"wx_channel/pkg/hermes"
)

// MockStore is a comprehensive in-memory implementation of hermes.Store,
// hermes.ResourceStore, and hermes.OutputNameStore for testing. It tracks
// all calls for assertions and supports error injection for failure paths.
type MockStore struct {
	mu sync.Mutex

	// Task data.
	taskInfo    *hermes.TaskJob
	loadTaskErr error

	// Status tracking.
	StatusCalls     []int
	lastStatus      int
	ActivateCalls   int
	FinishCalls     int
	DeactivateCalls int

	// Progress tracking.
	ProgressCalls         []ProgressCall
	ResourceProgressCalls map[int][]ProgressCall

	// Size tracking.
	resourceSizeVal  int64
	resourceSizes    map[int]int64
	updateSizeErr    error
	updateProgErr    error
	updateStatusErr  error
	finishErr        error
	activateErr      error
	createSegmentErr error

	// RecordError calls.
	RecordErrors []string

	// Segment tracking.
	segmentInfo     []hermes.Segment
	segmentStore    map[int]*hermes.Segment // segment ID -> segment
	segmentResource map[int]int             // segment ID -> resource ID
	nextSegmentID   int

	// ResourceJob finish tracking.
	FinishedResources []int

	// Output name updates.
	NameUpdates []hermes.OutputNameUpdate
}

// ProgressCall records a single progress update.
type ProgressCall struct {
	TaskID     int
	Downloaded int64
	Speed      int64
}

// NewMockStore creates a new MockStore and optionally pre-loads it with a
// task.
func NewMockStore(task *hermes.TaskJob) *MockStore {
	return &MockStore{
		taskInfo:              task,
		ResourceProgressCalls: make(map[int][]ProgressCall),
		resourceSizes:         make(map[int]int64),
		segmentStore:          make(map[int]*hermes.Segment),
		segmentResource:       make(map[int]int),
		nextSegmentID:         1,
	}
}

// ---------------------------------------------------------------------------
// Error injection
// ---------------------------------------------------------------------------

// SetLoadTaskError makes LoadTask return the given error.
func (m *MockStore) SetLoadTaskError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadTaskErr = err
}

// SetUpdateStatusError makes UpdateStatus return the given error.
func (m *MockStore) SetUpdateStatusError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateStatusErr = err
}

// SetUpdateProgressError makes UpdateProgress return the given error.
func (m *MockStore) SetUpdateProgressError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateProgErr = err
}

// SetUpdateSizeError makes UpdateResourceSize return the given error.
func (m *MockStore) SetUpdateSizeError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateSizeErr = err
}

// SetActivateError makes ActivateTask return the given error.
func (m *MockStore) SetActivateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activateErr = err
}

// SetFinishError makes FinishTask return the given error.
func (m *MockStore) SetFinishError(err error) { m.mu.Lock(); defer m.mu.Unlock(); m.finishErr = err }

// SetCreateSegmentError makes CreateSegments return the given error.
func (m *MockStore) SetCreateSegmentError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createSegmentErr = err
}

// ---------------------------------------------------------------------------
// hermes.Store implementation
// ---------------------------------------------------------------------------

// LoadTask returns the task configured for this store.
func (m *MockStore) LoadTask(taskID int) (*hermes.TaskJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadTaskErr != nil {
		return nil, m.loadTaskErr
	}
	return m.taskInfo, nil
}

// UpdateStatus records a status change.
func (m *MockStore) UpdateStatus(taskID int, status int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	m.StatusCalls = append(m.StatusCalls, status)
	m.lastStatus = status
	return nil
}

// ActivateTask records an activation call.
func (m *MockStore) ActivateTask(taskID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activateErr != nil {
		return m.activateErr
	}
	m.ActivateCalls++
	return nil
}

// UpdateProgress records a task-level progress update.
func (m *MockStore) UpdateProgress(taskID int, downloaded int64, speed int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateProgErr != nil {
		return m.updateProgErr
	}
	m.ProgressCalls = append(m.ProgressCalls, ProgressCall{taskID, downloaded, speed})
	return nil
}

// UpdateResourceSize records a task-level resource size update.
func (m *MockStore) UpdateResourceSize(taskID int, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateSizeErr != nil {
		return m.updateSizeErr
	}
	m.resourceSizeVal = size
	if taskID > 0 {
		m.resourceSizes[taskID] = size
	}
	return nil
}

// DeactivateConnections records a deactivation call.
func (m *MockStore) DeactivateConnections(taskID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeactivateCalls++
	return nil
}

// FinishTask records task completion.
func (m *MockStore) FinishTask(taskID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.finishErr != nil {
		return m.finishErr
	}
	m.FinishCalls++
	return nil
}

// RecordError records an error message for a task.
func (m *MockStore) RecordError(taskID int, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RecordErrors = append(m.RecordErrors, errMsg)
	return nil
}

// CreateSegments creates segment records with auto-incrementing IDs.
func (m *MockStore) CreateSegments(resourceID int, url string, ranges []hermes.SegmentRange) ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createSegmentErr != nil {
		return nil, m.createSegmentErr
	}
	ids := make([]int, len(ranges))
	for i, r := range ranges {
		id := m.nextSegmentID
		m.nextSegmentID++
		m.segmentStore[id] = &hermes.Segment{
			ID:          id,
			Index:       r.Index,
			URL:         url,
			OffsetStart: r.OffsetStart,
			OffsetEnd:   r.OffsetEnd,
			Size:        r.Size,
		}
		m.segmentResource[id] = resourceID
		ids[i] = id
	}
	return ids, nil
}

// LoadSegmentInfo returns the currently tracked segment state.
func (m *MockStore) LoadSegmentInfo(resourceID int) ([]hermes.Segment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.segmentInfo != nil {
		segments := make([]hermes.Segment, len(m.segmentInfo))
		copy(segments, m.segmentInfo)
		return segments, nil
	}
	segments := make([]hermes.Segment, 0, len(m.segmentStore))
	for id, s := range m.segmentStore {
		if m.segmentResource[id] == resourceID {
			segments = append(segments, *s)
		}
	}
	return segments, nil
}

// UpdateSegmentProgress updates a segment's downloaded byte count.
func (m *MockStore) UpdateSegmentProgress(segID int, downloaded int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.segmentStore[segID]; ok {
		s.Downloaded = downloaded
	}
	return nil
}

// ---------------------------------------------------------------------------
// hermes.ResourceStore implementation
// ---------------------------------------------------------------------------

// UpdateResourceProgress records a resource-level progress update.
func (m *MockStore) UpdateResourceProgress(resourceID int, downloaded int64, speed int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateProgErr != nil {
		return m.updateProgErr
	}
	m.ResourceProgressCalls[resourceID] = append(m.ResourceProgressCalls[resourceID], ProgressCall{0, downloaded, speed})
	return nil
}

// UpdateResourceSizeByID records a resource-level size update.
func (m *MockStore) UpdateResourceSizeByID(resourceID int, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateSizeErr != nil {
		return m.updateSizeErr
	}
	m.resourceSizes[resourceID] = size
	return nil
}

// FinishResource records a resource-level completion.
func (m *MockStore) FinishResource(resourceID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FinishedResources = append(m.FinishedResources, resourceID)
	return nil
}

// ---------------------------------------------------------------------------
// hermes.OutputNameStore implementation
// ---------------------------------------------------------------------------

// UpdateOutputName records a name update call.
func (m *MockStore) UpdateOutputName(update hermes.OutputNameUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NameUpdates = append(m.NameUpdates, update)
	return nil
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// LastStatus returns the most recent status recorded.
func (m *MockStore) LastStatus() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.StatusCalls) == 0 {
		return -1
	}
	return m.lastStatus
}

// LastProgress returns the most recent progress call.
func (m *MockStore) LastProgress() ProgressCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.ProgressCalls) == 0 {
		return ProgressCall{}
	}
	return m.ProgressCalls[len(m.ProgressCalls)-1]
}

// ResourceSize returns the recorded size for a resource.
func (m *MockStore) ResourceSize(resourceID int) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resourceSizes[resourceID]
}

// SetSegmentInfo overrides the segment state returned by LoadSegmentInfo.
func (m *MockStore) SetSegmentInfo(segments []hermes.Segment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.segmentInfo = segments
}

// HasStatus checks if the given status was recorded.
func (m *MockStore) HasStatus(status int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.StatusCalls {
		if s == status {
			return true
		}
	}
	return false
}

// Task returns the stored task (read-only).
func (m *MockStore) Task() *hermes.TaskJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.taskInfo
}

// ---------------------------------------------------------------------------
// Compile-time interface checks
// ---------------------------------------------------------------------------

var (
	_ hermes.Store           = (*MockStore)(nil)
	_ hermes.ResourceStore   = (*MockStore)(nil)
	_ hermes.OutputNameStore = (*MockStore)(nil)
)
