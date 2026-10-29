package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/database/model"
	"wx_channel/internal/mcpserver"
	"wx_channel/internal/services"
	"wx_channel/internal/workers/sph"
)

// mcp_automation_max_rows caps how many schedules or runs an MCP listing may
// return in one call.
const mcp_automation_max_rows = 200

type mcp_sph_deployer struct {
	config *config.Config
}

// NewMCPSphDeployer creates the application-owned deployment backend used by
// MCP transports. Cloudflare credentials remain in the application config.
func NewMCPSphDeployer(cfg *config.Config) mcpserver.SphDeployer {
	if cfg == nil {
		return nil
	}
	return &mcp_sph_deployer{config: cfg}
}

func (d *mcp_sph_deployer) DeploySphWorker(ctx context.Context) (*mcpserver.SphDeployResult, error) {
	if d == nil || d.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}
	result, err := sph.Deploy(ctx, sph.DeployOptions{
		AccountID:     d.config.GetString("cloudflare.accountId"),
		AuthToken:     d.config.GetString("cloudflare.apiToken"),
		WorkerName:    d.config.GetString("cloudflare.sphWorkerName"),
		Cookie:        d.config.GetString("cloudflare.sphCookie"),
		Credential:    d.config.GetString("cloudflare.sphCredential"),
		RepositoryDir: d.config.RootDir,
	})
	if err != nil {
		return nil, err
	}
	return &mcpserver.SphDeployResult{
		WorkerID:         result.WorkerID,
		WorkerName:       result.WorkerName,
		WorkerURL:        result.WorkerURL,
		WorkerURLWarning: result.WorkerURLWarning,
		ScriptBytes:      result.ScriptBytes,
	}, nil
}

// mcp_wxmp_runtime adapts the in-process official-account adapter to the MCP
// capability layer. Unlike the HTTP-backed video-channel tools, the wxmp
// capability has no api_client equivalent, so it is available in standalone and
// HTTP MCP modes alike.
type mcp_wxmp_runtime struct {
	adapter interface {
		FetchBizMsgList(username string, offset string) (json.RawMessage, error)
	}
}

// new_mcp_wxmp_runtime returns nil when no official-account adapter is
// installed, which hides the tool instead of exposing one that can only fail.
func new_mcp_wxmp_runtime() mcpserver.WXMPRuntime {
	handler := adapter.Get("wxmp")
	if handler == nil {
		return nil
	}
	// adapter.Get resolves the init-time singleton, so a runtime registered
	// after this lookup is still the same instance.
	wxmp_adapter, ok := handler.(interface {
		FetchBizMsgList(username string, offset string) (json.RawMessage, error)
	})
	if !ok {
		return nil
	}
	return &mcp_wxmp_runtime{adapter: wxmp_adapter}
}

func (r *mcp_wxmp_runtime) BizMsgList(ctx context.Context, username string, offset string) (json.RawMessage, error) {
	if r == nil || r.adapter == nil {
		return nil, fmt.Errorf("公众号历史消息能力未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.adapter.FetchBizMsgList(username, offset)
}

// mcp_wxchannels_adapter is the narrow view of the video-channel adapter that
// the MCP backend consumes. Each method maps 1:1 onto a ChannelsAdapter export.
type mcp_wxchannels_adapter interface {
	SearchChannelsContact(keyword string, next_marker string) (json.RawMessage, error)
	FetchChannelsFeedListOfContact(username string, next_marker string) (json.RawMessage, error)
	FetchChannelsLiveReplayList(username string, next_marker string) (json.RawMessage, error)
	FetchChannelsFeedProfile(oid string, nid string, request_url string, eid string) (json.RawMessage, error)
	FetchChannelsFeedCommentList(oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error)
	FetchChannelsFeedShareUrl(oid string) (json.RawMessage, error)
	FetchLiveProfile(oid string, nid string, live_id string) (json.RawMessage, error)
	FetchChannelsInteractionedFeedList(flag string, next_marker string) (json.RawMessage, error)
	FetchChannelsFollowList(next_marker string) (json.RawMessage, error)
	FetchChannelsPlayHistory(next_marker string) (json.RawMessage, error)
	PageAvailable() bool
	DecryptVideoInPlace(file_path string, key uint64) error
}

// mcp_wxchannels_backend calls the in-process video-channel adapter instead of
// looping back through this process's own HTTP API. The adapter is resolved on
// every call: the MCP runtime is constructed before the adapter is registered
// (the API server and the MCP service depend on each other), so a lookup at
// construction time would always miss. Resolving per call also avoids caching
// the *wxchannels.Client, which Stop() replaces under runtime_mu.
//
// resolved exists only so tests can inject a fake; production leaves it nil.
type mcp_wxchannels_backend struct {
	resolved mcp_wxchannels_adapter
}

// new_mcp_wxchannels_backend returns the HTTP host's backend. It is never nil:
// the tools it backs must stay advertised even before the adapter registers.
func new_mcp_wxchannels_backend() mcpserver.WXChannelsBackend {
	return &mcp_wxchannels_backend{}
}

func (b *mcp_wxchannels_backend) begin(ctx context.Context) (mcp_wxchannels_adapter, error) {
	if b == nil {
		return nil, fmt.Errorf("视频号查询能力未初始化")
	}
	target, err := b.resolve()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return target, nil
}

func (b *mcp_wxchannels_backend) resolve() (mcp_wxchannels_adapter, error) {
	if b != nil && b.resolved != nil {
		return b.resolved, nil
	}
	handler := adapter.Get("wxchannels")
	if handler == nil {
		return nil, fmt.Errorf("wxchannels runtime is not initialized")
	}
	target, ok := handler.(mcp_wxchannels_adapter)
	if !ok {
		return nil, fmt.Errorf("wxchannels runtime is not initialized")
	}
	return target, nil
}

// Status renders the page-connection state itself; it is the only call whose
// response the adapter does not already produce as JSON.
func (b *mcp_wxchannels_backend) Status(ctx context.Context) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]bool{"available": target.PageAvailable()})
	if err != nil {
		return nil, fmt.Errorf("编码视频号连接状态失败: %w", err)
	}
	return json.RawMessage(payload), nil
}

func (b *mcp_wxchannels_backend) SearchContact(ctx context.Context, keyword string, next_marker string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.SearchChannelsContact(keyword, next_marker)
}

func (b *mcp_wxchannels_backend) FeedListOfContact(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.FetchChannelsFeedListOfContact(username, next_marker)
}

func (b *mcp_wxchannels_backend) LiveReplayList(ctx context.Context, username string, next_marker string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.FetchChannelsLiveReplayList(username, next_marker)
}

func (b *mcp_wxchannels_backend) FeedProfile(ctx context.Context, oid string, nid string, request_url string, eid string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.FetchChannelsFeedProfile(oid, nid, request_url, eid)
}

func (b *mcp_wxchannels_backend) FeedCommentList(ctx context.Context, oid string, nid string, comment_id string, next_marker string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.FetchChannelsFeedCommentList(oid, nid, comment_id, next_marker)
}

func (b *mcp_wxchannels_backend) FeedShareUrl(ctx context.Context, oid string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.FetchChannelsFeedShareUrl(oid)
}

// LiveProfile ignores username: the platform call is addressed by the live
// object identifiers, and the HTTP route never forwarded the nickname either.
func (b *mcp_wxchannels_backend) LiveProfile(ctx context.Context, username string, oid string, nid string, live_id string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.FetchLiveProfile(oid, nid, live_id)
}

func (b *mcp_wxchannels_backend) InteractedFeedList(ctx context.Context, flag int, next_marker string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.FetchChannelsInteractionedFeedList(strconv.Itoa(flag), next_marker)
}

func (b *mcp_wxchannels_backend) FollowedAccounts(ctx context.Context, next_marker string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.FetchChannelsFollowList(next_marker)
}

func (b *mcp_wxchannels_backend) PlayHistory(ctx context.Context, next_marker string) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	return target.FetchChannelsPlayHistory(next_marker)
}

// DecryptVideo needs no page connection, matching the HTTP decrypt route.
func (b *mcp_wxchannels_backend) DecryptVideo(ctx context.Context, file_path string, key uint64) (json.RawMessage, error) {
	target, err := b.begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := target.DecryptVideoInPlace(file_path, key); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]string{"filepath": file_path})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(payload), nil
}

// mcp_data_reader adapts the transport-neutral data query service to the MCP
// consumer interface. Other transports can call DataQueryService directly.
type mcp_data_reader struct {
	data_service *services.DataQueryService
}

func new_mcp_data_reader(data_service *services.DataQueryService) *mcp_data_reader {
	return &mcp_data_reader{data_service: data_service}
}

func (r *mcp_data_reader) ListDownloadTasks(ctx context.Context, query mcpserver.DownloadTaskListQuery) (any, error) {
	if r == nil || r.data_service == nil {
		return nil, fmt.Errorf("数据查询服务未初始化")
	}
	return r.data_service.ListDownloadTasks(ctx, services.DownloadTaskListQuery{
		Page:         query.Page,
		PageSize:     query.PageSize,
		Statuses:     query.Statuses,
		ParentTaskID: query.ParentTaskID,
		RootTaskID:   query.RootTaskID,
		ContentID:    query.ContentID,
	})
}

func (r *mcp_data_reader) GetDownloadTaskDetail(ctx context.Context, task_id int) (any, error) {
	if r == nil || r.data_service == nil {
		return nil, fmt.Errorf("数据查询服务未初始化")
	}
	return r.data_service.GetDownloadTaskDetail(ctx, task_id)
}

func (r *mcp_data_reader) ListAccounts(ctx context.Context, query mcpserver.AccountListQuery) (any, error) {
	if r == nil || r.data_service == nil {
		return nil, fmt.Errorf("数据查询服务未初始化")
	}
	return r.data_service.ListAccounts(ctx, services.AccountListQuery{
		Page:      query.Page,
		PageSize:  query.PageSize,
		Keyword:   query.Keyword,
		AccountID: query.AccountID,
	})
}

func (r *mcp_data_reader) ListBrowseHistory(ctx context.Context, query mcpserver.BrowseHistoryListQuery) (any, error) {
	if r == nil || r.data_service == nil {
		return nil, fmt.Errorf("数据查询服务未初始化")
	}
	return r.data_service.ListBrowseHistory(ctx, services.BrowseHistoryListQuery{
		Page:        query.Page,
		PageSize:    query.PageSize,
		Keyword:     query.Keyword,
		Username:    query.Username,
		PlatformIDs: query.PlatformIDs,
	})
}

func (r *mcp_data_reader) ListLogs(ctx context.Context, query mcpserver.LogListQuery) (any, error) {
	if r == nil || r.data_service == nil {
		return nil, fmt.Errorf("数据查询服务未初始化")
	}
	return r.data_service.ListLogs(ctx, services.LogListQuery{
		Page:     query.Page,
		PageSize: query.PageSize,
		MaxBytes: query.MaxBytes,
		Keyword:  query.Keyword,
		Source:   query.Source,
		Levels:   query.Levels,
	})
}

func (r *mcp_data_reader) GetCertificateStatus(ctx context.Context) (any, error) {
	if r == nil || r.data_service == nil {
		return nil, fmt.Errorf("数据查询服务未初始化")
	}
	return r.data_service.GetCertificateStatus(ctx)
}

type mcp_download_task_deleter struct {
	download_task_service *services.DownloadTaskService
}

type mcp_download_task_creator struct {
	download_task_service *services.DownloadTaskService
}

func new_mcp_download_task_creator(download_task_service *services.DownloadTaskService) *mcp_download_task_creator {
	return &mcp_download_task_creator{download_task_service: download_task_service}
}

func (c *mcp_download_task_creator) CreateDownloadTask(
	ctx context.Context,
	request mcpserver.DownloadTaskCreateRequest,
) (*mcpserver.DownloadTaskCreateResult, error) {
	if c == nil || c.download_task_service == nil {
		return nil, fmt.Errorf("下载任务服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	created, err := c.download_task_service.CreateTask(services.CreateDownloadTaskBody{
		Platform:        request.Platform,
		Content:         request.Content,
		BuildFromFetch:  request.BuildFromFetch,
		ResourceIndexes: request.ResourceIndexes,
		DownloadDir:     request.DownloadDir,
		Filename:        request.Filename,
		Config:          request.Config,
		AutoStart:       request.AutoStart,
		ParentTaskID:    request.ParentTaskID,
		RelationType:    request.RelationType,
	})
	if err != nil {
		var duplicate_err *services.DuplicateTaskError
		existing_action, _ := request.Config["existing_action"].(string)
		if errors.As(err, &duplicate_err) && strings.TrimSpace(existing_action) == "skip" {
			return &mcpserver.DownloadTaskCreateResult{Task: map[string]any{
				"id":      duplicate_err.ExistingTaskID,
				"name":    duplicate_err.ExistingTaskName,
				"skipped": true,
				"action":  "skip",
			}, Skipped: true}, nil
		}
		return nil, err
	}
	item := services.BuildDownloadTaskItem(created)
	return &mcpserver.DownloadTaskCreateResult{
		Task: item,
		IDs:  []int{item.ID},
	}, nil
}

func new_mcp_download_task_deleter(download_task_service *services.DownloadTaskService) *mcp_download_task_deleter {
	return &mcp_download_task_deleter{download_task_service: download_task_service}
}

func (d *mcp_download_task_deleter) DeleteDownloadTasks(
	ctx context.Context,
	task_ids []int,
	delete_files bool,
) ([]mcpserver.DeleteDownloadTaskResult, error) {
	if d == nil || d.download_task_service == nil {
		return nil, fmt.Errorf("下载任务服务未初始化")
	}
	results := make([]mcpserver.DeleteDownloadTaskResult, 0, len(task_ids))
	for _, task_id := range task_ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item := mcpserver.DeleteDownloadTaskResult{TaskID: task_id}
		if err := d.download_task_service.DeleteTaskWithFiles(task_id, delete_files); err != nil {
			item.Error = err.Error()
		} else {
			item.Success = true
			item.StatusText = "cancelled"
		}
		results = append(results, item)
	}
	return results, nil
}

type mcp_scraper_job_backend struct {
	scraper_job_service *services.ScraperJobService
}

func new_mcp_scraper_job_backend(scraper_job_service *services.ScraperJobService) *mcp_scraper_job_backend {
	return &mcp_scraper_job_backend{scraper_job_service: scraper_job_service}
}

func (b *mcp_scraper_job_backend) CreateScraperJob(
	ctx context.Context,
	raw_url string,
	force_refresh bool,
) (*mcpserver.ScraperJob, error) {
	if b == nil || b.scraper_job_service == nil {
		return nil, fmt.Errorf("抓取任务服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	job, err := b.scraper_job_service.Create(services.ScraperFetchRequest{
		URL:          raw_url,
		ForceRefresh: force_refresh,
	})
	if err != nil {
		return nil, err
	}
	return mcp_scraper_job(job)
}

func (b *mcp_scraper_job_backend) GetScraperJob(ctx context.Context, job_id string) (*mcpserver.ScraperJob, error) {
	if b == nil || b.scraper_job_service == nil {
		return nil, fmt.Errorf("抓取任务服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	job := b.scraper_job_service.Get(job_id, true)
	if job == nil {
		return nil, fmt.Errorf("抓取任务不存在: %s", job_id)
	}
	return mcp_scraper_job(job)
}

func (b *mcp_scraper_job_backend) InterruptScraperJob(job_id string) {
	if b == nil || b.scraper_job_service == nil {
		return
	}
	b.scraper_job_service.Interrupt(job_id)
}

// mcp_automation_backend adapts the automation service to the MCP consumer
// interface. The service itself is transport agnostic, so this layer only
// reshapes models and honours request cancellation.
type mcp_automation_backend struct {
	automation_service *services.AutomationService
}

// new_mcp_automation_backend returns nil when no automation service is
// configured so the MCP server hides the automation tools instead of exposing
// ones that can only fail.
func new_mcp_automation_backend(automation_service *services.AutomationService) mcpserver.AutomationBackend {
	if automation_service == nil {
		return nil
	}
	return &mcp_automation_backend{automation_service: automation_service}
}

func (b *mcp_automation_backend) ListSchedules(ctx context.Context) ([]mcpserver.AutomationScheduleSummary, error) {
	if b == nil || b.automation_service == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	schedules, _, err := b.automation_service.ListSchedules(services.ListSchedulesInput{PageSize: mcp_automation_max_rows})
	if err != nil {
		return nil, err
	}
	summaries := make([]mcpserver.AutomationScheduleSummary, 0, len(schedules))
	for _, schedule := range schedules {
		summaries = append(summaries, mcp_automation_schedule_summary(schedule))
	}
	return summaries, nil
}

func (b *mcp_automation_backend) GetSchedule(ctx context.Context, id string) (*mcpserver.AutomationScheduleDetail, error) {
	if b == nil || b.automation_service == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	schedule, err := b.automation_service.GetSchedule(id)
	if err != nil {
		return nil, err
	}
	return mcp_automation_schedule_detail(schedule), nil
}

func (b *mcp_automation_backend) CreateSchedule(
	ctx context.Context,
	input mcpserver.AutomationCreateScheduleInput,
) (*mcpserver.AutomationScheduleDetail, error) {
	if b == nil || b.automation_service == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	schedule, err := b.automation_service.CreateSchedule(services.CreateScheduleInput{
		Name:        input.Name,
		Description: input.Description,
		CronExpr:    input.CronExpr,
		FlowID:      input.FlowID,
		InitialData: input.InitialData,
		Enabled:     input.Enabled,
		TimeoutSec:  input.TimeoutSec,
	})
	if err != nil {
		return nil, err
	}
	return mcp_automation_schedule_detail(schedule), nil
}

func (b *mcp_automation_backend) ToggleSchedule(ctx context.Context, id string) (*mcpserver.AutomationScheduleDetail, error) {
	if b == nil || b.automation_service == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	schedule, err := b.automation_service.ToggleSchedule(id)
	if err != nil {
		return nil, err
	}
	return mcp_automation_schedule_detail(schedule), nil
}

func (b *mcp_automation_backend) TriggerSchedule(ctx context.Context, id string) (*mcpserver.AutomationRunSummary, error) {
	if b == nil || b.automation_service == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	run, err := b.automation_service.TriggerSchedule(id)
	if err != nil {
		return nil, err
	}
	return mcp_automation_run_summary(run), nil
}

func (b *mcp_automation_backend) ListRuns(
	ctx context.Context,
	schedule_id string,
	limit int,
) ([]mcpserver.AutomationRunSummary, error) {
	if b == nil || b.automation_service == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > mcp_automation_max_rows {
		limit = mcp_automation_max_rows
	}
	runs, _, err := b.automation_service.ListRuns(services.ListRunsInput{
		PageSize:   limit,
		ScheduleID: schedule_id,
	})
	if err != nil {
		return nil, err
	}
	summaries := make([]mcpserver.AutomationRunSummary, 0, len(runs))
	for index := range runs {
		if summary := mcp_automation_run_summary(&runs[index]); summary != nil {
			summaries = append(summaries, *summary)
		}
	}
	return summaries, nil
}

func mcp_automation_schedule_summary(schedule model.FlowSchedule) mcpserver.AutomationScheduleSummary {
	return mcpserver.AutomationScheduleSummary{
		ID:            schedule.ID,
		Name:          schedule.Name,
		Description:   schedule.Description,
		CronExpr:      schedule.CronExpr,
		FlowID:        schedule.FlowID,
		Enabled:       schedule.Enabled,
		NextRunAt:     schedule.NextRunAt,
		LastRunID:     schedule.LastRunID,
		LastRunStatus: schedule.LastRunStatus,
	}
}

func mcp_automation_schedule_detail(schedule *model.FlowSchedule) *mcpserver.AutomationScheduleDetail {
	if schedule == nil {
		return nil
	}
	return &mcpserver.AutomationScheduleDetail{
		AutomationScheduleSummary: mcp_automation_schedule_summary(*schedule),
		InitialData:               decode_automation_initial_data(schedule.InitialData),
		TimeoutSec:                schedule.TimeoutSec,
		CreatedAt:                 schedule.CreatedAt,
		UpdatedAt:                 schedule.UpdatedAt,
	}
}

func mcp_automation_run_summary(run *model.FlowRunRecord) *mcpserver.AutomationRunSummary {
	if run == nil {
		return nil
	}
	return &mcpserver.AutomationRunSummary{
		ID:          run.ID,
		ScheduleID:  run.ScheduleID,
		FlowID:      run.FlowID,
		TriggerType: run.TriggerType,
		Status:      run.Status,
		CurrentNode: run.CurrentNode,
		Error:       run.Error,
		StartedAt:   run.StartedAt,
		CompletedAt: run.CompletedAt,
	}
}

// decode_automation_initial_data surfaces the stored JSON as a plain object.
// Malformed content is reported as empty rather than failing the whole read.
func decode_automation_initial_data(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil
	}
	return data
}

func mcp_scraper_job(job *services.ScraperFetchJob) (*mcpserver.ScraperJob, error) {
	if job == nil {
		return nil, fmt.Errorf("抓取任务为空")
	}
	progress, err := json.Marshal(job.Progress)
	if err != nil {
		return nil, fmt.Errorf("编码抓取任务进度失败: %w", err)
	}
	var output json.RawMessage
	if job.Output != nil {
		output, err = json.Marshal(job.Output)
		if err != nil {
			return nil, fmt.Errorf("编码抓取任务结果失败: %w", err)
		}
	}
	return &mcpserver.ScraperJob{
		ID:       job.ID,
		Platform: job.Platform,
		URL:      job.URL,
		Status:   job.Status,
		Progress: progress,
		Output:   output,
		Error:    job.Error,
	}, nil
}
