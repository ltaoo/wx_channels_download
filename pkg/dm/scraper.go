package dm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ScraperJob is the transport-neutral snapshot of one async scraper job.
type ScraperJob struct {
	ID       string          `json:"id"`
	Platform string          `json:"platform"`
	URL      string          `json:"url"`
	Status   string          `json:"status"`
	Progress json.RawMessage `json:"progress"`
	Output   json.RawMessage `json:"output"`
	Error    string          `json:"error"`
}

// ScraperOutput is the decoded job output. Content is set by the CLI's fetch
// result and absent from the MCP-facing snapshot.
type ScraperOutput struct {
	JobID        string          `json:"job_id"`
	Platform     string          `json:"platform"`
	URL          string          `json:"url"`
	Result       json.RawMessage `json:"result"`
	Content      json.RawMessage `json:"content"`
	DownloadInfo json.RawMessage `json:"download_info"`
}

// JobError reports a scraper job that ended in failed or interrupted state.
type JobError struct {
	Status  string
	Message string
	Job     *ScraperJob
}

func (e *JobError) Error() string { return e.Message }

// CreateScraperJob enqueues one URL for scraping.
func (c *Client) CreateScraperJob(ctx context.Context, raw_url string, force_refresh bool) (*ScraperJob, error) {
	body := map[string]any{"url": raw_url, "force_refresh": force_refresh}
	raw_data, err := c.do(ctx, http.MethodPost, "/api/scraper/fetch", nil, body)
	if err != nil {
		return nil, err
	}
	return decode_scraper_job(raw_data)
}

// GetScraperJob reads the current snapshot of one job.
func (c *Client) GetScraperJob(ctx context.Context, job_id string) (*ScraperJob, error) {
	raw_data, err := c.get(ctx, "/api/scraper/job", url.Values{"id": []string{job_id}})
	if err != nil {
		return nil, err
	}
	return decode_scraper_job(raw_data)
}

// WaitScraperJob polls until the job finishes and returns its decoded output.
// A failed or interrupted job yields *JobError.
func (c *Client) WaitScraperJob(ctx context.Context, job *ScraperJob) (*ScraperOutput, error) {
	if job == nil || strings.TrimSpace(job.ID) == "" {
		return nil, fmt.Errorf("抓取任务响应缺少 id")
	}
	current_job := job
	poll_timer := time.NewTimer(c.PollInterval())
	defer poll_timer.Stop()
	for {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("等待解析完成超时或已取消: %w", ctx.Err())
		}
		switch current_job.Status {
		case "completed":
			return decode_scraper_output(current_job)
		case "failed", "interrupted":
			return nil, &JobError{
				Status:  current_job.Status,
				Message: value_or_default(current_job.Error, "抓取任务以状态 "+current_job.Status+" 结束"),
				Job:     current_job,
			}
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("等待解析完成超时或已取消: %w", ctx.Err())
		case <-poll_timer.C:
		}
		next_job, err := c.GetScraperJob(ctx, current_job.ID)
		if err != nil {
			return nil, err
		}
		current_job = next_job
		poll_timer.Reset(c.PollInterval())
	}
}

func decode_scraper_job(raw_data json.RawMessage) (*ScraperJob, error) {
	var job ScraperJob
	if err := json.Unmarshal(raw_data, &job); err != nil {
		return nil, fmt.Errorf("解析抓取任务响应失败: %w", err)
	}
	if strings.TrimSpace(job.ID) == "" {
		return nil, fmt.Errorf("抓取任务响应缺少 id")
	}
	return &job, nil
}

func decode_scraper_output(job *ScraperJob) (*ScraperOutput, error) {
	if !has_json_value(job.Output) {
		return nil, fmt.Errorf("抓取任务已完成，但缺少解析结果")
	}
	var output ScraperOutput
	if err := json.Unmarshal(job.Output, &output); err != nil {
		return nil, fmt.Errorf("解析抓取结果失败: %w", err)
	}
	if strings.TrimSpace(output.Platform) == "" {
		output.Platform = job.Platform
	}
	if !has_json_value(output.Result) {
		return nil, fmt.Errorf("抓取结果缺少 result")
	}
	return &output, nil
}
