package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	douyin_scraper "wx_channel/pkg/scraper/douyin"
	youtube_scraper "wx_channel/pkg/scraper/youtube"
)

const (
	scraper_platform_wxchannels = "wxchannels"
	scraper_platform_wxmp       = "wxmp"
	scraper_platform_douyin     = "douyin"
	scraper_platform_bilibili   = "bilibili"
	scraper_platform_cctv       = "cctv"
	scraper_platform_zhihu      = "zhihu"
	scraper_platform_ttk        = "ttk"
	scraper_platform_youtube    = "youtube"
	scraper_platform_webpage    = "webpage"
)

const (
	ScraperJobStatusPending     = "pending"
	ScraperJobStatusRunning     = "running"
	ScraperJobStatusCompleted   = "completed"
	ScraperJobStatusFailed      = "failed"
	ScraperJobStatusInterrupted = "interrupted"
)

const (
	ScraperJobEventRaw           = "raw"
	ScraperJobEventStarted       = "started"
	ScraperJobEventProgress      = "progress"
	ScraperJobEventContent       = "content"
	ScraperJobEventAccount       = "account"
	ScraperJobEventContentDetail = "content_detail"
	ScraperJobEventCacheEntry    = "cache_entry"
	ScraperJobEventFinished      = "finished"
	ScraperJobEventFailed        = "failed"
	ScraperJobEventInterrupted   = "interrupted"
)

var scraper_fetch_job_sequence atomic.Uint64

// ScraperFetchOutput is the successful result produced by a scraper fetch job.
type ScraperFetchOutput struct {
	JobID          string                      `json:"job_id"`
	Platform       string                      `json:"platform"`
	URL            string                      `json:"url"`
	Result         any                         `json:"result"`
	Content        *model.Content              `json:"content"`
	Account        *model.Account              `json:"account"`
	ContentDetails []adapter.ContentDetail     `json:"content_details,omitempty"`
	CacheEntries   []adapter.FetchCacheEntry   `json:"cache_entries,omitempty"`
	DownloadInfo   *adapter.DownloadTaskResult `json:"download_info,omitempty"`
}

// ScraperFetchJobEvent is one ordered WebSocket event in a fetch job.
type ScraperFetchJobEvent struct {
	JobID         string                       `json:"job_id"`
	Sequence      int64                        `json:"sequence"`
	Stage         string                       `json:"stage"`
	Status        string                       `json:"status"`
	RawResult     any                          `json:"raw_result,omitempty"`
	Content       *model.Content               `json:"content,omitempty"`
	Account       *model.Account               `json:"account,omitempty"`
	ContentDetail *adapter.ContentDetail       `json:"content_detail,omitempty"`
	CacheEntry    *adapter.FetchCacheEntry     `json:"cache_entry,omitempty"`
	Progress      *events.ScraperFetchProgress `json:"progress,omitempty"`
	Current       int                          `json:"current,omitempty"`
	Total         int                          `json:"total,omitempty"`
	Error         string                       `json:"error,omitempty"`
	Timestamp     int64                        `json:"timestamp"`
}

// ScraperFetchJob is the in-memory state exposed by the scraper job API.
type ScraperFetchJob struct {
	ID                  string                       `json:"id"`
	Platform            string                       `json:"platform"`
	URL                 string                       `json:"url"`
	ForceRefresh        bool                         `json:"force_refresh"`
	Status              string                       `json:"status"`
	Progress            *events.ScraperFetchProgress `json:"progress,omitempty"`
	RawResult           any                          `json:"raw_result,omitempty"`
	Content             *model.Content               `json:"content,omitempty"`
	Account             *model.Account               `json:"account,omitempty"`
	ContentDetails      []adapter.ContentDetail      `json:"content_details,omitempty"`
	CacheEntries        []adapter.FetchCacheEntry    `json:"cache_entries,omitempty"`
	Output              *ScraperFetchOutput          `json:"output,omitempty"`
	Error               string                       `json:"error,omitempty"`
	CreatedAt           int64                        `json:"created_at"`
	UpdatedAt           int64                        `json:"updated_at"`
	StartedAt           int64                        `json:"started_at,omitempty"`
	FinishedAt          int64                        `json:"finished_at,omitempty"`
	cancel_fetch        context.CancelFunc
	detail_indexes      map[string]int
	cache_entry_indexes map[string]int
	event_sequence      int64
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
type ScraperPlatformChecker func(platform_id string, status_key string) error

// ScraperJobEventHandler receives immutable job snapshots and ordered events.
type ScraperJobEventHandler func(job *ScraperFetchJob, event *ScraperFetchJobEvent)

// ScraperJobService owns scraper job state and execution.
type ScraperJobService struct {
	job_mu           sync.RWMutex
	jobs             map[string]*ScraperFetchJob
	fetch_runner     ScraperFetchRunner
	platform_checker ScraperPlatformChecker
	event_handler    ScraperJobEventHandler
	logger           zerolog.Logger
}

func NewScraperJobService(
	platform_checker ScraperPlatformChecker,
	event_handler ScraperJobEventHandler,
	parent_loggers ...*zerolog.Logger,
) *ScraperJobService {
	logger := zerolog.Nop()
	var parent_logger *zerolog.Logger
	if len(parent_loggers) > 0 {
		parent_logger = parent_loggers[0]
	}
	if parent_logger != nil {
		logger = parent_logger.With().Str("component", "scraper_job").Logger()
	}
	return &ScraperJobService{
		jobs:             make(map[string]*ScraperFetchJob),
		platform_checker: platform_checker,
		event_handler:    event_handler,
		logger:           logger,
	}
}

// SetFetchRunner replaces the default adapter-backed runner.
func (s *ScraperJobService) SetFetchRunner(fetch_runner ScraperFetchRunner) {
	s.job_mu.Lock()
	s.fetch_runner = fetch_runner
	s.job_mu.Unlock()
}

type ScraperPlatformResolution struct {
	Platform  string
	StatusKey string
}

// DetectScraperPlatform maps a supported scraper URL to its adapter ID.
func DetectScraperPlatform(raw_url string) (string, error) {
	resolution, err := ResolveScraperPlatform(raw_url)
	if err != nil {
		return "", err
	}
	return resolution.Platform, nil
}

// ResolveScraperPlatform maps a supported scraper URL to its adapter ID and
// the status entry that should gate that URL.
func ResolveScraperPlatform(raw_url string) (ScraperPlatformResolution, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return ScraperPlatformResolution{}, fmt.Errorf("url 不能为空")
	}

	if _, err := douyin_scraper.ExtractURL(raw_url); err == nil {
		return scraper_platform_resolution(scraper_platform_douyin, ""), nil
	}

	if strings.HasPrefix(strings.ToLower(raw_url), "zhihu://") {
		return scraper_platform_resolution(scraper_platform_zhihu, ""), nil
	}

	if _, ok := youtube_scraper.ExtractVideoID(raw_url); ok {
		return scraper_platform_resolution(scraper_platform_youtube, ""), nil
	}

	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Hostname() == "" {
		return ScraperPlatformResolution{}, fmt.Errorf("无法解析 URL: %s", raw_url)
	}
	parsed_url.Scheme = strings.ToLower(strings.TrimSpace(parsed_url.Scheme))
	if parsed_url.Scheme != "http" && parsed_url.Scheme != "https" {
		return ScraperPlatformResolution{}, fmt.Errorf("仅支持 HTTP/HTTPS URL: %s", raw_url)
	}

	host := strings.ToLower(parsed_url.Hostname())
	switch {
	case host == "weixin.qq.com" && strings.HasPrefix(parsed_url.EscapedPath(), "/sph/"):
		return scraper_platform_resolution(scraper_platform_wxchannels, "wxchannels:sph"), nil
	case host == "channels.weixin.qq.com":
		return scraper_platform_resolution(scraper_platform_wxchannels, "wxchannels:page"), nil
	case host == "mp.weixin.qq.com":
		return scraper_platform_resolution(scraper_platform_wxmp, ""), nil
	case scraper_host_matches(host, "douyin.com") || scraper_host_matches(host, "iesdouyin.com"):
		return scraper_platform_resolution(scraper_platform_douyin, ""), nil
	case scraper_host_matches(host, "bilibili.com") || host == "b23.tv" || host == "bili2233.cn":
		return scraper_platform_resolution(scraper_platform_bilibili, ""), nil
	case host == "v.cctv.com":
		return scraper_platform_resolution(scraper_platform_cctv, ""), nil
	case host == "www.zhihu.com" || host == "zhuanlan.zhihu.com":
		return scraper_platform_resolution(scraper_platform_zhihu, ""), nil
	case host == "ttks.tw" || host == "www.ttks.tw":
		return scraper_platform_resolution(scraper_platform_ttk, ""), nil
	default:
		return scraper_platform_resolution(scraper_platform_webpage, ""), nil
	}
}

func scraper_platform_resolution(platform_id string, status_key string) ScraperPlatformResolution {
	if strings.TrimSpace(status_key) == "" {
		status_key = platform_id
	}
	return ScraperPlatformResolution{Platform: platform_id, StatusKey: status_key}
}

func scraper_host_matches(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// Create starts an asynchronous scraper fetch job.
func (s *ScraperJobService) Create(request ScraperFetchRequest) (*ScraperFetchJob, error) {
	raw_url := strings.TrimSpace(request.URL)
	resolution, err := ResolveScraperPlatform(raw_url)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("requested_job_id", strings.TrimSpace(request.ID)).
			Str("source_url", raw_url).
			Msg("scraper fetch: platform detection failed")
		return nil, err
	}
	platform_id := resolution.Platform
	s.job_mu.RLock()
	has_custom_runner := s.fetch_runner != nil
	s.job_mu.RUnlock()
	if adapter.Get(platform_id) == nil && !has_custom_runner {
		s.logger.Error().
			Str("platform", platform_id).
			Str("source_url", raw_url).
			Msg("scraper fetch: adapter is not registered")
		return nil, fmt.Errorf("未注册的平台 adapter: %s", platform_id)
	}
	if s.platform_checker != nil {
		if err := s.platform_checker(platform_id, resolution.StatusKey); err != nil {
			s.logger.Warn().
				Err(err).
				Str("platform", platform_id).
				Str("status_key", resolution.StatusKey).
				Str("source_url", raw_url).
				Msg("scraper fetch: platform is unavailable")
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
	job.Progress = new_scraper_fetch_progress(
		job,
		"queued",
		ScraperJobStatusPending,
		0,
		0,
		2,
		"抓取任务已创建，等待执行...",
	)

	s.job_mu.Lock()
	if _, exists := s.jobs[job_id]; exists {
		s.job_mu.Unlock()
		cancel_fetch()
		return nil, fmt.Errorf("id 正在使用中")
	}
	s.jobs[job_id] = job
	snapshot := clone_scraper_fetch_job(job, true)
	s.job_mu.Unlock()
	s.logger.Info().
		Str("job_id", job_id).
		Str("platform", platform_id).
		Str("source_url", raw_url).
		Bool("force_refresh", request.ForceRefresh).
		Msg("scraper fetch: job created")

	go s.run_scraper_fetch_job(fetch_context, job_id)
	return snapshot, nil
}

func (s *ScraperJobService) run_scraper_fetch_job(fetch_context context.Context, job_id string) {
	defer func() {
		if panic_value := recover(); panic_value != nil {
			s.logger.Error().
				Interface("panic", panic_value).
				Str("job_id", job_id).
				Msg("scraper fetch: job panicked")
			s.finish_scraper_fetch_job(job_id, nil, fmt.Errorf("scraper fetch panic: %v", panic_value))
		}
	}()

	job := s.start_scraper_fetch_job(job_id)
	if job == nil {
		s.logger.Warn().
			Str("job_id", job_id).
			Msg("scraper fetch: job could not be started")
		return
	}
	s.logger.Info().
		Str("job_id", job.ID).
		Str("platform", job.Platform).
		Str("source_url", job.URL).
		Msg("scraper fetch: job started")
	s.job_mu.RLock()
	runner := s.fetch_runner
	s.job_mu.RUnlock()
	if runner == nil {
		runner = s.execute_scraper_fetch
	}
	output, err := runner(fetch_context, job)
	if fetch_context.Err() != nil {
		s.logger.Warn().
			Err(fetch_context.Err()).
			Str("job_id", job.ID).
			Str("platform", job.Platform).
			Msg("scraper fetch: context canceled")
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
	job.Progress = new_scraper_fetch_progress(
		job,
		"started",
		ScraperJobStatusRunning,
		0,
		0,
		5,
		"抓取任务已开始，正在准备平台解析...",
	)
	snapshot := clone_scraper_fetch_job(job, false)
	event := new_scraper_fetch_job_event(job, ScraperJobEventStarted)
	if job.Progress != nil {
		progress := *job.Progress
		event.Progress = &progress
	}
	runner_job := clone_scraper_fetch_job(job, false)
	s.job_mu.Unlock()
	s.publish_event(snapshot, event)
	return runner_job
}

func (s *ScraperJobService) execute_scraper_fetch(fetch_context context.Context, job *ScraperFetchJob) (*ScraperFetchOutput, error) {
	handler := adapter.Get(job.Platform)
	if handler == nil {
		s.logger.Error().
			Str("job_id", job.ID).
			Str("platform", job.Platform).
			Msg("scraper fetch: adapter lookup failed")
		return nil, fmt.Errorf("未注册的平台 adapter: %s", job.Platform)
	}

	var data any
	var err error
	context_handler, supports_context := handler.(adapter.ContextProgressFetchAdapter)
	progress_handler, supports_progress := handler.(adapter.ProgressFetchAdapter)
	fetch_mode := "fetch"
	if supports_context {
		fetch_mode = "context_progress"
	} else if supports_progress {
		fetch_mode = "progress"
	}
	s.emit_scraper_fetch_progress(
		job,
		"fetch",
		ScraperJobStatusRunning,
		0,
		0,
		10,
		"正在请求平台内容...",
	)
	fetch_started_at := time.Now()
	s.logger.Info().
		Str("job_id", job.ID).
		Str("platform", job.Platform).
		Str("source_url", job.URL).
		Str("fetch_mode", fetch_mode).
		Str("adapter_type", fmt.Sprintf("%T", handler)).
		Msg("scraper fetch: adapter call started")
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
		s.emit_scraper_fetch_progress(
			job,
			"failed",
			ScraperJobStatusFailed,
			0,
			0,
			0,
			fmt.Sprintf("请求平台内容失败: %s", err.Error()),
		)
		s.logger.Error().
			Err(err).
			Str("job_id", job.ID).
			Str("platform", job.Platform).
			Str("fetch_mode", fetch_mode).
			Dur("fetch_elapsed", time.Since(fetch_started_at)).
			Msg("scraper fetch: adapter call failed")
		return nil, err
	}
	s.logger.Info().
		Str("job_id", job.ID).
		Str("platform", job.Platform).
		Str("fetch_mode", fetch_mode).
		Str("result_type", fmt.Sprintf("%T", data)).
		Dur("fetch_elapsed", time.Since(fetch_started_at)).
		Msg("scraper fetch: adapter call completed")
	s.emit_scraper_fetch_progress(
		job,
		"fetched",
		ScraperJobStatusRunning,
		0,
		0,
		55,
		"平台内容已返回，正在解析内容信息...",
	)
	raw_artifact_emitted := false
	emit_raw_artifact := func() {
		if raw_artifact_emitted {
			return
		}
		raw_artifact_emitted = true
		s.emit_scraper_fetch_artifact(job.ID, adapter.FetchArtifact{
			Stage:     adapter.FetchArtifactStageRaw,
			RawResult: data,
		})
	}

	s.emit_scraper_fetch_progress(
		job,
		"download_preview",
		ScraperJobStatusRunning,
		0,
		0,
		62,
		"正在生成下载任务预览...",
	)
	download_info, err := build_scraper_download_info(handler, data)
	if err != nil {
		emit_raw_artifact()
		s.emit_scraper_fetch_progress(
			job,
			"failed",
			ScraperJobStatusFailed,
			0,
			0,
			0,
			fmt.Sprintf("构建下载任务预览失败: %s", err.Error()),
		)
		s.logger.Error().
			Err(err).
			Str("job_id", job.ID).
			Str("platform", job.Platform).
			Msg("scraper fetch: download info build failed")
		return nil, fmt.Errorf("构建下载任务预览失败: %w", err)
	}
	content := download_info.Content
	account := download_info.Account
	content_details := download_info.ContentDetails
	if content != nil && strings.TrimSpace(content.SourceURL) == "" {
		content.SourceURL = job.URL
	}
	s.emit_scraper_fetch_progress(
		job,
		"content",
		ScraperJobStatusRunning,
		0,
		0,
		70,
		"内容信息已转换完成",
	)
	if content != nil {
		s.emit_scraper_fetch_artifact(job.ID, adapter.FetchArtifact{
			Stage:   adapter.FetchArtifactStageContent,
			Content: content,
		})
	}
	s.emit_scraper_fetch_progress(
		job,
		"account",
		ScraperJobStatusRunning,
		0,
		0,
		76,
		"账号信息已转换完成",
	)
	if account != nil {
		s.emit_scraper_fetch_artifact(job.ID, adapter.FetchArtifact{
			Stage:   adapter.FetchArtifactStageAccount,
			Account: account,
		})
	}
	s.emit_scraper_fetch_progress(
		job,
		"content_detail",
		ScraperJobStatusRunning,
		0,
		0,
		84,
		"内容详情已转换完成",
	)
	for detail_index := range content_details {
		detail := content_details[detail_index]
		s.emit_scraper_fetch_artifact(job.ID, adapter.FetchArtifact{
			Stage:         adapter.FetchArtifactStageContentDetail,
			ContentDetail: &detail,
			Current:       detail_index + 1,
			Total:         len(content_details),
		})
	}
	s.emit_scraper_fetch_progress(
		job,
		"cache",
		ScraperJobStatusRunning,
		0,
		0,
		92,
		"正在整理抓取缓存...",
	)
	var cache_entries []adapter.FetchCacheEntry
	if cache_handler, supports_cache := handler.(adapter.FetchCacheAdapter); supports_cache {
		cache_entries, err = cache_handler.FetchCacheEntries(job.URL, data)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("job_id", job.ID).
				Str("platform", job.Platform).
				Msg("scraper fetch: cache inspection failed")
			cache_entries = nil
		}
	}
	s.logger.Info().
		Str("job_id", job.ID).
		Str("platform", job.Platform).
		Bool("content_created", content != nil).
		Bool("account_created", account != nil).
		Bool("download_info_created", download_info != nil).
		Int("content_detail_count", len(content_details)).
		Int("cache_entry_count", len(cache_entries)).
		Dur("total_elapsed", time.Since(fetch_started_at)).
		Msg("scraper fetch: result conversion completed")
	return &ScraperFetchOutput{
		JobID:          job.ID,
		Platform:       job.Platform,
		URL:            job.URL,
		Result:         data,
		Content:        content,
		Account:        account,
		ContentDetails: content_details,
		CacheEntries:   cache_entries,
		DownloadInfo:   download_info,
	}, nil
}

// build_scraper_download_info builds the task preview and normalized models
// from one in-memory Fetch payload without another platform request.
func build_scraper_download_info(handler adapter.AdapterHandler, data any) (*adapter.DownloadTaskResult, error) {
	fetch_builder, ok := handler.(adapter.FetchDownloadTaskBuilder)
	if !ok {
		return nil, fmt.Errorf("平台不支持从抓取结果构建下载任务")
	}
	download_info, err := fetch_builder.BuildDownloadTaskFromFetch(data, json.RawMessage("{}"))
	if err != nil {
		return nil, err
	}
	if download_info == nil {
		return nil, fmt.Errorf("平台未返回下载任务")
	}
	if download_info.Content == nil {
		return nil, fmt.Errorf("平台下载任务缺少 content")
	}
	if download_info.Account == nil {
		return nil, fmt.Errorf("平台下载任务缺少 account")
	}
	if download_info.ContentDetail != nil && len(download_info.ContentDetails) == 0 {
		return nil, fmt.Errorf("平台下载任务缺少 content_details")
	}
	return download_info, nil
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
	for cache_entry_index := range output.CacheEntries {
		cache_entry := output.CacheEntries[cache_entry_index]
		s.emit_scraper_fetch_artifact(job_id, adapter.FetchArtifact{
			Stage:      adapter.FetchArtifactStageCacheEntry,
			CacheEntry: &cache_entry,
			Current:    cache_entry_index + 1,
			Total:      len(output.CacheEntries),
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
	emitted_cache_entry := (*adapter.FetchCacheEntry)(nil)
	switch stage {
	case adapter.FetchArtifactStageRaw:
		if artifact.RawResult == nil {
			s.job_mu.Unlock()
			return
		}
		should_broadcast = job.RawResult == nil
		job.RawResult = clone_scraper_raw_result(artifact.RawResult)
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
	case adapter.FetchArtifactStageCacheEntry:
		if artifact.CacheEntry == nil {
			s.job_mu.Unlock()
			return
		}
		cache_entry := *artifact.CacheEntry
		cache_entry.Key = strings.TrimSpace(cache_entry.Key)
		cache_entry.Name = strings.TrimSpace(cache_entry.Name)
		cache_entry.URL = strings.TrimSpace(cache_entry.URL)
		cache_entry.Path = strings.TrimSpace(cache_entry.Path)
		if cache_entry.Path == "" {
			s.job_mu.Unlock()
			return
		}
		if cache_entry.Key == "" {
			cache_entry.Key = cache_entry.Path
		}
		if job.cache_entry_indexes == nil {
			job.cache_entry_indexes = make(map[string]int)
		}
		if cache_entry_index, exists := job.cache_entry_indexes[cache_entry.Key]; exists {
			should_broadcast = job.CacheEntries[cache_entry_index] != cache_entry
			job.CacheEntries[cache_entry_index] = cache_entry
		} else {
			job.cache_entry_indexes[cache_entry.Key] = len(job.CacheEntries)
			job.CacheEntries = append(job.CacheEntries, cache_entry)
			should_broadcast = true
		}
		cache_entry_copy := cache_entry
		emitted_cache_entry = &cache_entry_copy
	default:
		s.job_mu.Unlock()
		return
	}
	job.UpdatedAt = now
	if should_broadcast {
		event = new_scraper_fetch_job_event(job, stage)
		event.RawResult = clone_scraper_raw_result(artifact.RawResult)
		event.Content = artifact.Content
		event.Account = artifact.Account
		if artifact.ContentDetail != nil {
			detail := *artifact.ContentDetail
			event.ContentDetail = &detail
		}
		if emitted_cache_entry != nil {
			cache_entry := *emitted_cache_entry
			event.CacheEntry = &cache_entry
		}
		event.Current = artifact.Current
		event.Total = artifact.Total
	}
	snapshot := clone_scraper_fetch_job(job, false)
	if event != nil && stage == adapter.FetchArtifactStageCacheEntry {
		snapshot.CacheEntries = append([]adapter.FetchCacheEntry(nil), job.CacheEntries...)
	}
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

func (s *ScraperJobService) emit_scraper_fetch_progress(
	job *ScraperFetchJob,
	stage string,
	status string,
	current int,
	total int,
	percent float64,
	message string,
) {
	if job == nil {
		return
	}
	progress := new_scraper_fetch_progress(job, stage, status, current, total, percent, message)
	if progress == nil {
		return
	}
	s.UpdateProgress(*progress)
}

func new_scraper_fetch_progress(
	job *ScraperFetchJob,
	stage string,
	status string,
	current int,
	total int,
	percent float64,
	message string,
) *events.ScraperFetchProgress {
	if job == nil {
		return nil
	}
	if status == "" {
		status = job.Status
	}
	if current < 0 {
		current = 0
	}
	if total < 0 {
		total = 0
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return &events.ScraperFetchProgress{
		RequestID: job.ID,
		Platform:  job.Platform,
		URL:       job.URL,
		Stage:     strings.TrimSpace(stage),
		Status:    status,
		Current:   current,
		Total:     total,
		Percent:   percent,
		Message:   strings.TrimSpace(message),
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
		job.Progress = new_scraper_fetch_progress(
			job,
			"failed",
			ScraperJobStatusFailed,
			0,
			0,
			0,
			fmt.Sprintf("抓取失败: %s", fetch_err.Error()),
		)
	} else {
		job.Status = ScraperJobStatusCompleted
		job.Output = output
		job.Progress = new_scraper_fetch_progress(
			job,
			"finished",
			ScraperJobStatusCompleted,
			1,
			1,
			100,
			"抓取完成，结果已生成",
		)
	}
	event_stage := ScraperJobEventFinished
	if fetch_err != nil {
		event_stage = ScraperJobEventFailed
	}
	event := new_scraper_fetch_job_event(job, event_stage)
	event.Error = job.Error
	if job.Progress != nil {
		progress := *job.Progress
		event.Progress = &progress
		event.Current = progress.Current
		event.Total = progress.Total
	}
	snapshot := clone_scraper_fetch_job(job, false)
	platform_id := job.Platform
	source_url := job.URL
	started_at := job.StartedAt
	final_status := job.Status
	s.job_mu.Unlock()
	elapsed := time.Duration(0)
	if started_at > 0 {
		elapsed = time.Duration(now-started_at) * time.Millisecond
	}
	if fetch_err != nil {
		s.logger.Error().
			Err(fetch_err).
			Str("job_id", job_id).
			Str("platform", platform_id).
			Str("source_url", source_url).
			Str("status", final_status).
			Dur("elapsed", elapsed).
			Msg("scraper fetch: job failed")
	} else {
		s.logger.Info().
			Str("job_id", job_id).
			Str("platform", platform_id).
			Str("source_url", source_url).
			Str("status", final_status).
			Dur("elapsed", elapsed).
			Msg("scraper fetch: job completed")
	}
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

// GetCacheEntry resolves one server-issued cache entry without accepting a
// filesystem path from the caller.
func (s *ScraperJobService) GetCacheEntry(job_id string, cache_key string) *adapter.FetchCacheEntry {
	job_id = strings.TrimSpace(job_id)
	cache_key = strings.TrimSpace(cache_key)
	if job_id == "" || cache_key == "" {
		return nil
	}

	s.job_mu.RLock()
	defer s.job_mu.RUnlock()
	job := s.jobs[job_id]
	if job == nil {
		return nil
	}
	find_cache_entry := func(entries []adapter.FetchCacheEntry) *adapter.FetchCacheEntry {
		for entry_index := range entries {
			entry := entries[entry_index]
			entry_key := strings.TrimSpace(entry.Key)
			if entry_key == "" {
				entry_key = strings.TrimSpace(entry.Path)
			}
			if entry_key == cache_key {
				entry_copy := entry
				return &entry_copy
			}
		}
		return nil
	}
	if entry := find_cache_entry(job.CacheEntries); entry != nil {
		return entry
	}
	if job.Output != nil {
		return find_cache_entry(job.Output.CacheEntries)
	}
	return nil
}

func clone_scraper_fetch_job(job *ScraperFetchJob, include_output bool) *ScraperFetchJob {
	if job == nil {
		return nil
	}
	cloned := *job
	cloned.cancel_fetch = nil
	cloned.detail_indexes = nil
	cloned.cache_entry_indexes = nil
	if job.Progress != nil {
		progress := *job.Progress
		cloned.Progress = &progress
	}
	if !include_output {
		cloned.RawResult = nil
		cloned.Content = nil
		cloned.Account = nil
		cloned.ContentDetails = nil
		cloned.CacheEntries = nil
		cloned.Output = nil
	} else {
		cloned.RawResult = clone_scraper_raw_result(job.RawResult)
		cloned.ContentDetails = append([]adapter.ContentDetail(nil), job.ContentDetails...)
		cloned.CacheEntries = append([]adapter.FetchCacheEntry(nil), job.CacheEntries...)
		if job.Output != nil {
			output := *job.Output
			output.ContentDetails = append([]adapter.ContentDetail(nil), job.Output.ContentDetails...)
			output.CacheEntries = append([]adapter.FetchCacheEntry(nil), job.Output.CacheEntries...)
			cloned.Output = &output
		}
	}
	return &cloned
}

func clone_scraper_raw_result(raw_result any) any {
	switch value := raw_result.(type) {
	case json.RawMessage:
		return append(json.RawMessage(nil), value...)
	case *json.RawMessage:
		if value == nil {
			return nil
		}
		cloned := append(json.RawMessage(nil), (*value)...)
		return &cloned
	case []byte:
		return append([]byte(nil), value...)
	default:
		return value
	}
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
		job.Progress.Stage = "interrupted"
		job.Progress.Percent = 0
	}
	event := new_scraper_fetch_job_event(job, ScraperJobEventInterrupted)
	if job.Progress != nil {
		progress := *job.Progress
		event.Progress = &progress
		event.Current = progress.Current
		event.Total = progress.Total
	}
	snapshot := clone_scraper_fetch_job(job, false)
	platform_id := job.Platform
	s.job_mu.Unlock()
	s.logger.Warn().
		Str("job_id", job_id).
		Str("platform", platform_id).
		Msg("scraper fetch: job interrupted")
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
