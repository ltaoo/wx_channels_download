package hermes

import (
	"errors"
	"fmt"
	"sync"
)

type memory_store struct {
	mu               sync.RWMutex
	next_task_id     int
	next_resource_id int
	next_segment_id  int
	tasks            map[int]*memory_task_state
	resource_tasks   map[int]int
	segments         map[int][]Segment
}

type memory_task_state struct {
	mu         sync.RWMutex
	job        *TaskJob
	status     int
	error_text string
	done       chan struct{}
	done_once  sync.Once
}

func new_memory_store() *memory_store {
	return &memory_store{
		tasks:          make(map[int]*memory_task_state),
		resource_tasks: make(map[int]int),
		segments:       make(map[int][]Segment),
	}
}

func (s *memory_store) create_task(job *TaskJob) (*memory_task_state, error) {
	if job == nil {
		return nil, errors.New("task is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.next_task_id++
	job.ID = s.next_task_id
	for i := range job.Resources {
		s.next_resource_id++
		job.Resources[i].ID = s.next_resource_id
		for endpoint_index := range job.Resources[i].Endpoints {
			job.Resources[i].Endpoints[endpoint_index].ID = endpoint_index + 1
		}
		s.resource_tasks[job.Resources[i].ID] = job.ID
	}

	state := &memory_task_state{
		job:    clone_task_job(job),
		status: TaskStatusWaiting,
		done:   make(chan struct{}),
	}
	s.tasks[job.ID] = state
	return state, nil
}

func new_failed_memory_task_state(err error) *memory_task_state {
	state := &memory_task_state{
		status: TaskStatusFailed,
		done:   make(chan struct{}),
	}
	if err != nil {
		state.error_text = err.Error()
	}
	state.done_once.Do(func() { close(state.done) })
	return state
}

func (s *memory_store) task_state(task_id int) (*memory_task_state, error) {
	s.mu.RLock()
	state := s.tasks[task_id]
	s.mu.RUnlock()
	if state == nil {
		return nil, fmt.Errorf("download task %d not found", task_id)
	}
	return state, nil
}

func (s *memory_store) resource_state(resource_id int) (*memory_task_state, error) {
	s.mu.RLock()
	task_id := s.resource_tasks[resource_id]
	state := s.tasks[task_id]
	s.mu.RUnlock()
	if state == nil {
		return nil, fmt.Errorf("download resource %d not found", resource_id)
	}
	return state, nil
}

func (s *memory_store) LoadTask(task_id int) (*TaskJob, error) {
	state, err := s.task_state(task_id)
	if err != nil {
		return nil, err
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return clone_task_job(state.job), nil
}

func (s *memory_store) UpdateStatus(task_id int, status int) error {
	state, err := s.task_state(task_id)
	if err != nil {
		return err
	}
	state.mu.Lock()
	state.status = status
	if status == TaskStatusCancelled {
		state.error_text = "download task was cancelled"
		state.done_once.Do(func() { close(state.done) })
	}
	state.mu.Unlock()
	return nil
}

func (s *memory_store) ActivateTask(int) error {
	return nil
}

func (s *memory_store) UpdateProgress(int, int64, int64) error {
	return nil
}

func (s *memory_store) UpdateResourceSize(task_id int, size int64) error {
	state, err := s.task_state(task_id)
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

func (s *memory_store) DeactivateConnections(int) error {
	return nil
}

func (s *memory_store) FinishTask(task_id int) error {
	state, err := s.task_state(task_id)
	if err != nil {
		return err
	}
	state.mu.Lock()
	state.status = TaskStatusFinished
	state.done_once.Do(func() { close(state.done) })
	state.mu.Unlock()
	return nil
}

func (s *memory_store) RecordError(task_id int, err_msg string) error {
	state, err := s.task_state(task_id)
	if err != nil {
		return err
	}
	state.mu.Lock()
	state.status = TaskStatusFailed
	state.error_text = err_msg
	state.done_once.Do(func() { close(state.done) })
	state.mu.Unlock()
	return nil
}

func (s *memory_store) CreateSegments(resource_id int, raw_url string, ranges []SegmentRange) ([]int, error) {
	if _, err := s.resource_state(resource_id); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	segments := make([]Segment, len(ranges))
	ids := make([]int, len(ranges))
	for i, item := range ranges {
		s.next_segment_id++
		ids[i] = s.next_segment_id
		segments[i] = Segment{
			ID:          s.next_segment_id,
			Index:       item.Index,
			URL:         raw_url,
			OffsetStart: item.OffsetStart,
			OffsetEnd:   item.OffsetEnd,
			Size:        item.Size,
		}
	}
	s.segments[resource_id] = segments
	return ids, nil
}

func (s *memory_store) LoadSegmentInfo(resource_id int) ([]Segment, error) {
	if _, err := s.resource_state(resource_id); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Segment(nil), s.segments[resource_id]...), nil
}

func (s *memory_store) UpdateSegmentProgress(segment_id int, downloaded int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for resource_id, segments := range s.segments {
		for i := range segments {
			if segments[i].ID != segment_id {
				continue
			}
			segments[i].Downloaded = downloaded
			s.segments[resource_id] = segments
			return nil
		}
	}
	return fmt.Errorf("download segment %d not found", segment_id)
}

func (s *memory_store) UpdateResourceProgress(resource_id int, downloaded int64, speed int64) error {
	state, err := s.resource_state(resource_id)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if resource := find_memory_resource(state.job, resource_id); resource != nil {
		resource.Downloaded = downloaded
		resource.Speed = speed
	}
	state.mu.Unlock()
	return nil
}

func (s *memory_store) UpdateResourceSizeByID(resource_id int, size int64) error {
	state, err := s.resource_state(resource_id)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if resource := find_memory_resource(state.job, resource_id); resource != nil {
		resource.Size = size
	}
	state.mu.Unlock()
	return nil
}

func (s *memory_store) FinishResource(int) error {
	return nil
}

func (s *memory_store) UpdateOutputName(update OutputNameUpdate) error {
	state, err := s.task_state(update.TaskID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if resource := find_memory_resource(state.job, update.ResourceID); resource != nil && update.ResourceName != "" {
		resource.Name = update.ResourceName
	}
	if update.TaskName != "" {
		state.job.Name = update.TaskName
	}
	state.mu.Unlock()
	return nil
}

func (s *memory_store) UpdateResourceOutput(update ResourceOutputUpdate) error {
	state, err := s.task_state(update.TaskID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if resource := find_memory_resource(state.job, update.ResourceID); resource != nil {
		resource.DownloadDir = update.DownloadDir
		resource.Name = update.ResourceName
		resource.Kind = update.ResourceKind
		resource.Size = update.ResourceSize
	}
	state.mu.Unlock()
	return nil
}

func find_memory_resource(job *TaskJob, resource_id int) *ResourceJob {
	if job == nil {
		return nil
	}
	for i := range job.Resources {
		if job.Resources[i].ID == resource_id {
			return &job.Resources[i]
		}
	}
	return nil
}

func clone_task_job(job *TaskJob) *TaskJob {
	if job == nil {
		return nil
	}
	resources := make([]ResourceJob, len(job.Resources))
	for i, resource := range job.Resources {
		resources[i] = resource
		resources[i].Endpoints = make([]Endpoint, len(resource.Endpoints))
		for endpoint_index, endpoint := range resource.Endpoints {
			resources[i].Endpoints[endpoint_index] = endpoint
			resources[i].Endpoints[endpoint_index].Headers = clone_string_map(endpoint.Headers)
		}
		resources[i].Extra = clone_string_map(resource.Extra)
	}
	return &TaskJob{
		ID:          job.ID,
		Name:        job.Name,
		UniqueID:    job.UniqueID,
		DownloadDir: job.DownloadDir,
		Platform:    job.Platform,
		ProxyServer: job.ProxyServer,
		Resources:   resources,
		Config:      clone_any_map(job.Config),
		Metadata:    clone_any_map(job.Metadata),
	}
}

func clone_string_map(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func clone_any_map(values map[string]any) map[string]any {
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
	_ Store               = (*memory_store)(nil)
	_ ResourceStore       = (*memory_store)(nil)
	_ OutputNameStore     = (*memory_store)(nil)
	_ ResourceOutputStore = (*memory_store)(nil)
)
