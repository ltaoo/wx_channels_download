package mcpserver

import "context"

// ScraperJobBackend provides process-local scraper job access. Implementations
// may call the application service directly without an HTTP API listener.
type ScraperJobBackend interface {
	CreateScraperJob(ctx context.Context, raw_url string, force_refresh bool) (*ScraperJob, error)
	GetScraperJob(ctx context.Context, job_id string) (*ScraperJob, error)
	InterruptScraperJob(job_id string)
}
