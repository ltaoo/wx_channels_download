package hermes

import (
	"errors"
	"fmt"
	"sync"
)

type memoryStore struct {
	mu             sync.RWMutex
	nextTaskID     int
	nextResourceID int
	nextSegmentID  int
	tasks          map[int]*memoryTaskState
	resourceTasks  map[int]int
	segments       map[int][]Segment
}

type memoryTaskState struct {
	mu        sync.RWMutex
	job       *TaskJob
	status    int
	errorText string
	done      chan struct{}
	doneOnce  sync.Once
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		tasks:         make(map[int]*memoryTaskState),
		resourceTasks: make(map[int]int),
		segments:      make(map[int][]Segment),
	}
}

func (s *memoryStore) createTask(job *TaskJob) (*memoryTaskState, error) {
	if job == nil {
		return nil, errors.New("task is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextTaskID++
	job.ID = s.nextTaskID
	for i := range job.Resources {
		s.nextResourceID++
		job.Resources[i].ID = s.nextResourceID
		for endpointIndex := range job.Resources[i].Endpoints {
			job.Resources[i].Endpoints[endpointIndex].ID = endpointIndex + 1
		}
		s.resourceTasks[job.Resources[i].ID] = job.ID
	}

	state := &memoryTaskState{
		job:    cloneTaskJob(job),
		status: TaskStatusWaiting,
		done:   make(chan struct{}),
	}
	s.tasks[job.ID] = state
	return state, nil
}

func newFailedMemoryTaskState(err error) *memoryTaskState {
	state := &memoryTaskState{
		status: TaskStatusFailed,
		done:   make(chan struct{}),
	}
	if err != nil {
		state.errorText = err.Error()
	}
	state.doneOnce.Do(func() { close(state.done) })
	return state
}

func (s *memoryStore) taskState(taskID int) (*memoryTaskState, error) {
	s.mu.RLock()
	state := s.tasks[taskID]
	s.mu.RUnlock()
	if state == nil {
		return nil, fmt.Errorf("download task %d not found", taskID)
	}
	return state, nil
}

func (s *memoryStore) resourceState(resourceID int) (*memoryTaskState, error) {
	s.mu.RLock()
	taskID := s.resourceTasks[resourceID]
	state := s.tasks[taskID]
	s.mu.RUnlock()
	if state == nil {
		return nil, fmt.Errorf("download resource %d not found", resourceID)
	}
	return state, nil
}

func (s *memoryStore) LoadTask(taskID int) (*TaskJob, error) {
	state, err := s.taskState(taskID)
	if err != nil {
		return nil, err
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return cloneTaskJob(state.job), nil
}

func (s *memoryStore) UpdateStatus(taskID int, status int) error {
	state, err := s.taskState(taskID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	state.status = status
	if status == TaskStatusCancelled {
		state.errorText = "download task was cancelled"
		state.doneOnce.Do(func() { close(state.done) })
	}
	state.mu.Unlock()
	return nil
}

func (s *memoryStore) ActivateTask(int) error {
	return nil
}

func (s *memoryStore) UpdateProgress(int, int64, int64) error {
	return nil
}

func (s *memoryStore) UpdateResourceSize(taskID int, size int64) error {
	state, err := s.taskState(taskID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if len(state.job.Resources) > 0 {
		state.job.Resources[0].Size = size
	}
	state.mu.Unlock()
	return nil
}

func (s *memoryStore) DeactivateConnections(int) error {
	return nil
}

func (s *memoryStore) FinishTask(taskID int) error {
	state, err := s.taskState(taskID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	state.status = TaskStatusFinished
	state.doneOnce.Do(func() { close(state.done) })
	state.mu.Unlock()
	return nil
}

func (s *memoryStore) RecordError(taskID int, errMsg string) error {
	state, err := s.taskState(taskID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	state.status = TaskStatusFailed
	state.errorText = errMsg
	state.doneOnce.Do(func() { close(state.done) })
	state.mu.Unlock()
	return nil
}

func (s *memoryStore) CreateSegments(resourceID int, rawURL string, ranges []SegmentRange) ([]int, error) {
	if _, err := s.resourceState(resourceID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	segments := make([]Segment, len(ranges))
	ids := make([]int, len(ranges))
	for i, item := range ranges {
		s.nextSegmentID++
		ids[i] = s.nextSegmentID
		segments[i] = Segment{
			ID:          s.nextSegmentID,
			Index:       item.Index,
			URL:         rawURL,
			OffsetStart: item.OffsetStart,
			OffsetEnd:   item.OffsetEnd,
			Size:        item.Size,
		}
	}
	s.segments[resourceID] = segments
	return ids, nil
}

func (s *memoryStore) LoadSegmentInfo(resourceID int) ([]Segment, error) {
	if _, err := s.resourceState(resourceID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Segment(nil), s.segments[resourceID]...), nil
}

func (s *memoryStore) UpdateSegmentProgress(segmentID int, downloaded int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for resourceID, segments := range s.segments {
		for i := range segments {
			if segments[i].ID != segmentID {
				continue
			}
			segments[i].Downloaded = downloaded
			s.segments[resourceID] = segments
			return nil
		}
	}
	return fmt.Errorf("download segment %d not found", segmentID)
}

func (s *memoryStore) UpdateResourceProgress(resourceID int, downloaded int64, speed int64) error {
	state, err := s.resourceState(resourceID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if resource := findMemoryResource(state.job, resourceID); resource != nil {
		resource.Downloaded = downloaded
		resource.Speed = speed
	}
	state.mu.Unlock()
	return nil
}

func (s *memoryStore) UpdateResourceSizeByID(resourceID int, size int64) error {
	state, err := s.resourceState(resourceID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if resource := findMemoryResource(state.job, resourceID); resource != nil {
		resource.Size = size
	}
	state.mu.Unlock()
	return nil
}

func (s *memoryStore) FinishResource(int) error {
	return nil
}

func (s *memoryStore) UpdateOutputName(update OutputNameUpdate) error {
	state, err := s.taskState(update.TaskID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if resource := findMemoryResource(state.job, update.ResourceID); resource != nil && update.ResourceName != "" {
		resource.Name = update.ResourceName
	}
	if update.TaskName != "" {
		state.job.Name = update.TaskName
	}
	if update.SavePath != "" {
		state.job.SavePath = update.SavePath
	}
	state.mu.Unlock()
	return nil
}

func (s *memoryStore) UpdateResourceOutput(update ResourceOutputUpdate) error {
	state, err := s.taskState(update.TaskID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if resource := findMemoryResource(state.job, update.ResourceID); resource != nil {
		resource.Name = update.ResourceName
		resource.Kind = update.ResourceKind
		resource.Size = update.ResourceSize
	}
	state.mu.Unlock()
	return nil
}

func findMemoryResource(job *TaskJob, resourceID int) *ResourceJob {
	if job == nil {
		return nil
	}
	for i := range job.Resources {
		if job.Resources[i].ID == resourceID {
			return &job.Resources[i]
		}
	}
	return nil
}

func cloneTaskJob(job *TaskJob) *TaskJob {
	if job == nil {
		return nil
	}
	resources := make([]ResourceJob, len(job.Resources))
	for i, resource := range job.Resources {
		resources[i] = resource
		resources[i].Endpoints = make([]Endpoint, len(resource.Endpoints))
		for endpointIndex, endpoint := range resource.Endpoints {
			resources[i].Endpoints[endpointIndex] = endpoint
			resources[i].Endpoints[endpointIndex].Headers = cloneStringMap(endpoint.Headers)
		}
		resources[i].Extra = cloneStringMap(resource.Extra)
	}
	return &TaskJob{
		ID:               job.ID,
		Name:             job.Name,
		UniqueID:         job.UniqueID,
		SavePath:         job.SavePath,
		FilenameTemplate: job.FilenameTemplate,
		Platform:         job.Platform,
		ProxyServer:      job.ProxyServer,
		Resources:        resources,
		Config:           cloneAnyMap(job.Config),
		Metadata:         cloneAnyMap(job.Metadata),
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneAnyMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

var (
	_ Store               = (*memoryStore)(nil)
	_ ResourceStore       = (*memoryStore)(nil)
	_ OutputNameStore     = (*memoryStore)(nil)
	_ ResourceOutputStore = (*memoryStore)(nil)
)
