package services

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
)

const (
	scraper_platform_wxchannels  = "wxchannels"
	scraper_platform_wxmp        = "wxmp"
	scraper_platform_douyin      = "douyin"
	scraper_platform_bilibili    = "bilibili"
	scraper_platform_zhihu       = "zhihu"
	scraper_platform_69shuba     = "69shuba"
	scraper_platform_fanqienovel = "fanqienovel"
)

const (
	ScraperJobStatusPending     = "pending"
	ScraperJobStatusRunning     = "running"
	ScraperJobStatusCompleted   = "completed"
	ScraperJobStatusFailed      = "failed"
	ScraperJobStatusInterrupted = "interrupted"
)

const (
	ScraperJobEventStarted       = "started"
	ScraperJobEventProgress      = "progress"
	ScraperJobEventContent       = "content"
	ScraperJobEventAccount       = "account"
	ScraperJobEventContentDetail = "content_detail"
	ScraperJobEventFinished      = "finished"
	ScraperJobEventFailed        = "failed"
	ScraperJobEventInterrupted   = "interrupted"
)

var scraper_fetch_job_sequence atomic.Uint64

// ScraperFetchOutput is the successful result produced by a scraper fetch job.
type ScraperFetchOutput struct {
	JobID          string                  `json:"job_id"`
	Platform       string                  `json:"platform"`
	URL            string                  `json:"url"`
	Result         any                     `json:"result"`
	Content        *model.Content          `json:"content"`
	Account        *model.Account          `json:"account"`
	ContentDetails []adapter.ContentDetail `json:"content_details,omitempty"`
}

// ScraperFetchJobEvent is one ordered WebSocket event in a fetch job.
type ScraperFetchJobEvent struct {
	JobID         string                       `json:"job_id"`
	Sequence      int64                        `json:"sequence"`
	Stage         string                       `json:"stage"`
	Status        string                       `json:"status"`
	Content       *model.Content               `json:"content,omitempty"`
	Account       *model.Account               `json:"account,omitempty"`
	ContentDetail *adapter.ContentDetail       `json:"content_detail,omitempty"`
	Progress      *events.ScraperFetchProgress `json:"progress,omitempty"`
	Current       int                          `json:"current,omitempty"`
	Total         int                          `json:"total,omitempty"`
	Error         string                       `json:"error,omitempty"`
	Timestamp     int64                        `json:"timestamp"`
}

// ScraperFetchJob is the in-memory state exposed by the scraper job API.
type ScraperFetchJob struct {
	ID             string                       `json:"id"`
	Platform       string                       `json:"platform"`
	URL            string                       `json:"url"`
	ForceRefresh   bool                         `json:"force_refresh"`
	Status         string                       `json:"status"`
	Progress       *events.ScraperFetchProgress `json:"progress,omitempty"`
	Content        *model.Content               `json:"content,omitempty"`
	Account        *model.Account               `json:"account,omitempty"`
	ContentDetails []adapter.ContentDetail      `json:"content_details,omitempty"`
	Output         *ScraperFetchOutput          `json:"output,omitempty"`
	Error          string                       `json:"error,omitempty"`
	CreatedAt      int64                        `json:"created_at"`
	UpdatedAt      int64                        `json:"updated_at"`
	StartedAt      int64                        `json:"started_at,omitempty"`
	FinishedAt     int64                        `json:"finished_at,omitempty"`
	cancel_fetch   context.CancelFunc
	detail_indexes map[string]int
	event_sequence int64
}

// ScraperFetchRequest describes a request to start an asynchronous scraper job.
type ScraperFetchRequest struct {
	ID           string
	URL          string
	ForceRefresh bool
}

// ScraperFetchRunner executes the fetch stage for a job.
type ScraperFetchRunner func(fetch_context context.Context, job *ScraperFetchJob) (*ScraperFetchOutput, error)

// ScraperPlatformChecker rejects a fetch when a platform is known to be unavailable.
type ScraperPlatformChecker func(platform_id string) error

// ScraperJobEventHandler receives immutable job snapshots and ordered events.
type ScraperJobEventHandler func(job *ScraperFetchJob, event *ScraperFetchJobEvent)

// ScraperJobService owns scraper job state and execution.
type ScraperJobService struct {
	job_mu           sync.RWMutex
	jobs             map[string]*ScraperFetchJob
	fetch_runner     ScraperFetchRunner
	platform_checker ScraperPlatformChecker
	event_handler    ScraperJobEventHandler
}

func NewScraperJobService(
	platform_checker ScraperPlatformChecker,
	event_handler ScraperJobEventHandler,
) *ScraperJobService {
	return &ScraperJobService{
		jobs:             make(map[string]*ScraperFetchJob),
		platform_checker: platform_checker,
		event_handler:    event_handler,
	}
}

// SetFetchRunner replaces the default adapter-backed runner.
func (s *ScraperJobService) SetFetchRunner(fetch_runner ScraperFetchRunner) {
	s.job_mu.Lock()
	s.fetch_runner = fetch_runner
	s.job_mu.Unlock()
}

// DetectScraperPlatform maps a supported scraper URL to its adapter ID.
func DetectScraperPlatform(raw_url string) (string, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return "", fmt.Errorf("url 不能为空")
	}

	if strings.HasPrefix(strings.ToLower(raw_url), "zhihu://") {
		return scraper_platform_zhihu, nil
	}

	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Hostname() == "" {
		return "", fmt.Errorf("无法解析 URL: %s", raw_url)
	}

	host := strings.ToLower(parsed_url.Hostname())
	switch {
	case host == "weixin.qq.com" && strings.HasPrefix(parsed_url.EscapedPath(), "/sph/"):
		return scraper_platform_wxchannels, nil
	case host == "channels.weixin.qq.com":
		return scraper_platform_wxchannels, nil
	case host == "mp.weixin.qq.com":
		return scraper_platform_wxmp, nil
	case scraper_host_matches(host, "douyin.com") || scraper_host_matches(host, "iesdouyin.com"):
		return scraper_platform_douyin, nil
	case scraper_host_matches(host, "bilibili.com") || host == "b23.tv" || host == "bili2233.cn":
		return scraper_platform_bilibili, nil
	case host == "www.zhihu.com" || host == "zhuanlan.zhihu.com":
		return scraper_platform_zhihu, nil
	case scraper_host_matches(host, "fanqienovel.com"):
		return scraper_platform_fanqienovel, nil
	case strings.Contains(host, "69shuba"):
		return scraper_platform_69shuba, nil
	default:
		return "", fmt.Errorf("暂不支持该 URL: %s", raw_url)
	}
}

func scraper_host_matches(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// Create starts an asynchronous scraper fetch job.
func (s *ScraperJobService) Create(request ScraperFetchRequest) (*ScraperFetchJob, error) {
	raw_url := strings.TrimSpace(request.URL)
	platform_id, err := DetectScraperPlatform(raw_url)
	if err != nil {
		return nil, err
	}
	s.job_mu.RLock()
	has_custom_runner := s.fetch_runner != nil
	s.job_mu.RUnlock()
	if adapter.Get(platform_id) == nil && !has_custom_runner {
		return nil, fmt.Errorf("未注册的平台 adapter: %s", platform_id)
	}
	if s.platform_checker != nil {
		if err := s.platform_checker(platform_id); err != nil {
			return nil, err
		}
	}

	job_id := strings.TrimSpace(request.ID)
	if job_id == "" {
		job_id = new_scraper_fetch_job_id()
	}
	created_at := time.Now().UnixMilli()
	fetch_context, cancel_fetch := context.WithCancel(context.Background())
	job := &ScraperFetchJob{
		ID:           job_id,
		Platform:     platform_id,
		URL:          raw_url,
		ForceRefresh: request.ForceRefresh,
		Status:       ScraperJobStatusPending,
		CreatedAt:    created_at,
		UpdatedAt:    created_at,
		cancel_fetch: cancel_fetch,
	}

	s.job_mu.Lock()
	if _, exists := s.jobs[job_id]; exists {
		s.job_mu.Unlock()
		cancel_fetch()
		return nil, fmt.Errorf("id 正在使用中")
	}
	s.jobs[job_id] = job
	snapshot := clone_scraper_fetch_job(job, true)
	s.job_mu.Unlock()

	go s.run_scraper_fetch_job(fetch_context, job_id)
	return snapshot, nil
}

func (s *ScraperJobService) run_scraper_fetch_job(fetch_context context.Context, job_id string) {
	defer func() {
		if panic_value := recover(); panic_value != nil {
			s.finish_scraper_fetch_job(job_id, nil, fmt.Errorf("scraper fetch panic: %v", panic_value))
		}
	}()

	job := s.start_scraper_fetch_job(job_id)
	if job == nil {
		return
	}
	s.job_mu.RLock()
	runner := s.fetch_runner
	s.job_mu.RUnlock()
	if runner == nil {
		runner = s.execute_scraper_fetch
	}
	output, err := runner(fetch_context, job)
	if fetch_context.Err() != nil {
		s.Interrupt(job_id)
		return
	}
	if err == nil {
		s.emit_scraper_output_artifacts(job_id, output)
	}
	s.finish_scraper_fetch_job(job_id, output, err)
}

func (s *ScraperJobService) start_scraper_fetch_job(job_id string) *ScraperFetchJob {
	now := time.Now().UnixMilli()
	s.job_mu.Lock()
	job := s.jobs[job_id]
	if job == nil || job.Status != ScraperJobStatusPending {
		s.job_mu.Unlock()
		return nil
	}
	job.Status = ScraperJobStatusRunning
	job.StartedAt = now
	job.UpdatedAt = now
	snapshot := clone_scraper_fetch_job(job, false)
	event := new_scraper_fetch_job_event(job, ScraperJobEventStarted)
	runner_job := clone_scraper_fetch_job(job, false)
	s.job_mu.Unlock()
	s.publish_event(snapshot, event)
	return runner_job
}

func (s *ScraperJobService) execute_scraper_fetch(fetch_context context.Context, job *ScraperFetchJob) (*ScraperFetchOutput, error) {
	handler := adapter.Get(job.Platform)
	if handler == nil {
		return nil, fmt.Errorf("未注册的平台 adapter: %s", job.Platform)
	}

	var data any
	var err error
	context_handler, supports_context := handler.(adapter.ContextProgressFetchAdapter)
	progress_handler, supports_progress := handler.(adapter.ProgressFetchAdapter)
	switch {
	case supports_context:
		data, err = context_handler.FetchWithProgressContext(fetch_context, job.URL, adapter.FetchOptions{
			RequestID:    job.ID,
			ForceRefresh: job.ForceRefresh,
			ArtifactHandler: func(artifact adapter.FetchArtifact) {
				s.emit_scraper_fetch_artifact(job.ID, artifact)
			},
		})
	case supports_progress:
		data, err = progress_handler.FetchWithProgress(job.URL, job.ID)
	default:
		data, err = handler.Fetch(job.URL)
	}
	if err != nil {
		return nil, err
	}

	content, err := handler.ToContent(data)
	if err != nil {
		return nil, fmt.Errorf("转换 content 失败: %w", err)
	}
	if content != nil && strings.TrimSpace(content.SourceURL) == "" {
		content.SourceURL = job.URL
	}
	if content != nil {
		s.emit_scraper_fetch_artifact(job.ID, adapter.FetchArtifact{
			Stage:   adapter.FetchArtifactStageContent,
			Content: content,
		})
	}
	account, err := handler.ToAccount(data)
	if err != nil {
		return nil, fmt.Errorf("转换 account 失败: %w", err)
	}
	if account != nil {
		s.emit_scraper_fetch_artifact(job.ID, adapter.FetchArtifact{
			Stage:   adapter.FetchArtifactStageAccount,
			Account: account,
		})
	}
	content_details, err := handler.ToContentDetails(data)
	if err != nil {
		return nil, fmt.Errorf("转换 content detail 失败: %w", err)
	}
	for detail_index := range content_details {
		detail := content_details[detail_index]
		s.emit_scraper_fetch_artifact(job.ID, adapter.FetchArtifact{
			Stage:         adapter.FetchArtifactStageContentDetail,
			ContentDetail: &detail,
			Current:       detail_index + 1,
			Total:         len(content_details),
		})
	}
	return &ScraperFetchOutput{
		JobID:          job.ID,
		Platform:       job.Platform,
		URL:            job.URL,
		Result:         data,
		Content:        content,
		Account:        account,
		ContentDetails: content_details,
	}, nil
}

func (s *ScraperJobService) emit_scraper_output_artifacts(job_id string, output *ScraperFetchOutput) {
	if output == nil {
		return
	}
	if output.Content != nil {
		s.emit_scraper_fetch_artifact(job_id, adapter.FetchArtifact{
			Stage:   adapter.FetchArtifactStageContent,
			Content: output.Content,
		})
	}
	if output.Account != nil {
		s.emit_scraper_fetch_artifact(job_id, adapter.FetchArtifact{
			Stage:   adapter.FetchArtifactStageAccount,
			Account: output.Account,
		})
	}
	for detail_index := range output.ContentDetails {
		detail := output.ContentDetails[detail_index]
		s.emit_scraper_fetch_artifact(job_id, adapter.FetchArtifact{
			Stage:         adapter.FetchArtifactStageContentDetail,
			ContentDetail: &detail,
			Current:       detail_index + 1,
			Total:         len(output.ContentDetails),
		})
	}
}

func (s *ScraperJobService) emit_scraper_fetch_artifact(job_id string, artifact adapter.FetchArtifact) {
	now := time.Now().UnixMilli()
	s.job_mu.Lock()
	job := s.jobs[job_id]
	if job == nil || scraper_job_is_terminal(job.Status) {
		s.job_mu.Unlock()
		return
	}

	stage := strings.TrimSpace(artifact.Stage)
	should_broadcast := false
	event := (*ScraperFetchJobEvent)(nil)
	switch stage {
	case adapter.FetchArtifactStageContent:
		if artifact.Content == nil {
			s.job_mu.Unlock()
			return
		}
		should_broadcast = job.Content == nil
		job.Content = artifact.Content
	case adapter.FetchArtifactStageAccount:
		if artifact.Account == nil {
			s.job_mu.Unlock()
			return
		}
		should_broadcast = job.Account == nil
		job.Account = artifact.Account
	case adapter.FetchArtifactStageContentDetail:
		if artifact.ContentDetail == nil {
			s.job_mu.Unlock()
			return
		}
		detail := *artifact.ContentDetail
		detail.Key = strings.TrimSpace(detail.Key)
		if detail.Key == "" {
			detail.Key = strings.TrimSpace(detail.Type)
		}
		if job.detail_indexes == nil {
			job.detail_indexes = make(map[string]int)
		}
		if detail_index, exists := job.detail_indexes[detail.Key]; exists {
			job.ContentDetails[detail_index] = detail
		} else {
			job.detail_indexes[detail.Key] = len(job.ContentDetails)
			job.ContentDetails = append(job.ContentDetails, detail)
			should_broadcast = true
		}
	default:
		s.job_mu.Unlock()
		return
	}
	job.UpdatedAt = now
	if should_broadcast {
		event = new_scraper_fetch_job_event(job, stage)
		event.Content = artifact.Content
		event.Account = artifact.Account
		if artifact.ContentDetail != nil {
			detail := *artifact.ContentDetail
			event.ContentDetail = &detail
		}
		event.Current = artifact.Current
		event.Total = artifact.Total
	}
	snapshot := clone_scraper_fetch_job(job, false)
	s.job_mu.Unlock()
	if event != nil {
		s.publish_event(snapshot, event)
	}
}

func (s *ScraperJobService) publish_event(job *ScraperFetchJob, event *ScraperFetchJobEvent) {
	if s.event_handler != nil {
		s.event_handler(job, event)
	}
}

func new_scraper_fetch_job_event(job *ScraperFetchJob, stage string) *ScraperFetchJobEvent {
	if job == nil {
		return nil
	}
	job.event_sequence++
	return &ScraperFetchJobEvent{
		JobID:     job.ID,
		Sequence:  job.event_sequence,
		Stage:     stage,
		Status:    job.Status,
		Timestamp: time.Now().UnixMilli(),
	}
}

func (s *ScraperJobService) finish_scraper_fetch_job(job_id string, output *ScraperFetchOutput, fetch_err error) {
	if fetch_err == nil && output == nil {
		fetch_err = fmt.Errorf("scraper fetch returned an empty output")
	}
	now := time.Now().UnixMilli()
	s.job_mu.Lock()
	job := s.jobs[job_id]
	if job == nil || scraper_job_is_terminal(job.Status) {
		s.job_mu.Unlock()
		return
	}
	job.FinishedAt = now
	job.UpdatedAt = now
	cancel_fetch := job.cancel_fetch
	job.cancel_fetch = nil
	if fetch_err != nil {
		job.Status = ScraperJobStatusFailed
		job.Error = fetch_err.Error()
	} else {
		job.Status = ScraperJobStatusCompleted
		job.Output = output
	}
	event_stage := ScraperJobEventFinished
	if fetch_err != nil {
		event_stage = ScraperJobEventFailed
	}
	event := new_scraper_fetch_job_event(job, event_stage)
	event.Error = job.Error
	snapshot := clone_scraper_fetch_job(job, false)
	s.job_mu.Unlock()
	if cancel_fetch != nil {
		cancel_fetch()
	}
	s.publish_event(snapshot, event)
}

// UpdateProgress applies a progress event to its matching fetch job.
func (s *ScraperJobService) UpdateProgress(progress events.ScraperFetchProgress) {
	job_id := strings.TrimSpace(progress.RequestID)
	if job_id == "" {
		return
	}
	s.job_mu.Lock()
	job := s.jobs[job_id]
	if job == nil || scraper_job_is_terminal(job.Status) {
		s.job_mu.Unlock()
		return
	}
	progress_copy := progress
	job.Progress = &progress_copy
	job.UpdatedAt = time.Now().UnixMilli()
	event := new_scraper_fetch_job_event(job, ScraperJobEventProgress)
	event.Progress = &progress_copy
	event.Current = progress.Current
	event.Total = progress.Total
	snapshot := clone_scraper_fetch_job(job, false)
	s.job_mu.Unlock()
	s.publish_event(snapshot, event)
}

// Get returns a snapshot of a scraper fetch job.
func (s *ScraperJobService) Get(job_id string, include_output bool) *ScraperFetchJob {
	s.job_mu.RLock()
	job := clone_scraper_fetch_job(s.jobs[strings.TrimSpace(job_id)], include_output)
	s.job_mu.RUnlock()
	return job
}

func clone_scraper_fetch_job(job *ScraperFetchJob, include_output bool) *ScraperFetchJob {
	if job == nil {
		return nil
	}
	cloned := *job
	cloned.cancel_fetch = nil
	cloned.detail_indexes = nil
	if job.Progress != nil {
		progress := *job.Progress
		cloned.Progress = &progress
	}
	if !include_output {
		cloned.Content = nil
		cloned.Account = nil
		cloned.ContentDetails = nil
		cloned.Output = nil
	} else {
		cloned.ContentDetails = append([]adapter.ContentDetail(nil), job.ContentDetails...)
		if job.Output != nil {
			output := *job.Output
			output.ContentDetails = append([]adapter.ContentDetail(nil), job.Output.ContentDetails...)
			cloned.Output = &output
		}
	}
	return &cloned
}

func scraper_job_is_terminal(status string) bool {
	switch status {
	case ScraperJobStatusCompleted, ScraperJobStatusFailed, ScraperJobStatusInterrupted:
		return true
	default:
		return false
	}
}

// Interrupt stops a non-terminal scraper fetch job.
func (s *ScraperJobService) Interrupt(job_id string) bool {
	job_id = strings.TrimSpace(job_id)
	now := time.Now().UnixMilli()
	s.job_mu.Lock()
	job := s.jobs[job_id]
	if job == nil || scraper_job_is_terminal(job.Status) {
		s.job_mu.Unlock()
		return false
	}
	cancel_fetch := job.cancel_fetch
	job.cancel_fetch = nil
	job.Status = ScraperJobStatusInterrupted
	job.UpdatedAt = now
	job.FinishedAt = now
	job.Error = ""
	if job.Progress != nil {
		job.Progress.Status = ScraperJobStatusInterrupted
		job.Progress.Message = "获取已中断"
	}
	event := new_scraper_fetch_job_event(job, ScraperJobEventInterrupted)
	snapshot := clone_scraper_fetch_job(job, false)
	s.job_mu.Unlock()
	if cancel_fetch != nil {
		cancel_fetch()
	}
	s.publish_event(snapshot, event)
	return true
}

// InterruptAll stops every non-terminal scraper fetch job.
func (s *ScraperJobService) InterruptAll() {
	s.job_mu.RLock()
	job_ids := make([]string, 0, len(s.jobs))
	for job_id, job := range s.jobs {
		if job != nil && !scraper_job_is_terminal(job.Status) {
			job_ids = append(job_ids, job_id)
		}
	}
	s.job_mu.RUnlock()
	for _, job_id := range job_ids {
		s.Interrupt(job_id)
	}
}

func new_scraper_fetch_job_id() string {
	sequence := scraper_fetch_job_sequence.Add(1)
	return fmt.Sprintf("fetch-%d-%d", time.Now().UnixNano(), sequence)
}
