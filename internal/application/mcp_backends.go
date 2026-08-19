package application

import (
	"context"
	"encoding/json"
	"fmt"

	"wx_channel/internal/mcpserver"
	"wx_channel/internal/services"
)

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
