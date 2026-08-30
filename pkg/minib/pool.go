package minib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	max_pool_workers              = 64
	max_pool_per_host_concurrency = 8
)

// PoolOptions controls a bounded group of independent MiniBrowser sessions.
// Every request, including scripts and images, passes through the shared
// per-host scheduler so page workers cannot multiply into an unbounded burst.
type PoolOptions struct {
	Workers            int
	PerHostConcurrency int
	MinRequestInterval time.Duration
	MaxRetries         int
	BaseBackoff        time.Duration
	MaxBackoff         time.Duration
	BrowserTimeout     time.Duration
}

// DefaultPoolOptions returns conservative defaults suitable for public sites.
func DefaultPoolOptions() PoolOptions {
	return PoolOptions{
		Workers:            4,
		PerHostConcurrency: 2,
		MinRequestInterval: time.Second,
		MaxRetries:         3,
		BaseBackoff:        time.Second,
		MaxBackoff:         30 * time.Second,
		BrowserTimeout:     30 * time.Second,
	}
}

// NavigateJob describes one independent page navigation.
type NavigateJob struct {
	URL     string
	Headers http.Header
	Options NavigateOptions
}

// NavigateResult is stored at the same index as its input NavigateJob.
type NavigateResult struct {
	Index    int
	Job      NavigateJob
	Page     *Page
	Attempts int
	Err      error
}

// Pool runs navigations through independent browser sessions and a scheduler
// shared by every document, script, style, image, XHR, and fetch request.
type Pool struct {
	options   PoolOptions
	scheduler *host_scheduler
}

type request_scheduler interface {
	before_request(context.Context, string) (func(), error)
	observe_response(string, int, http.Header)
}

type host_scheduler struct {
	per_host_concurrency int
	request_interval     time.Duration
	states_mutex         sync.Mutex
	states               map[string]*host_schedule_state
}

type host_schedule_state struct {
	semaphore     chan struct{}
	timing_mutex  sync.Mutex
	next_start    time.Time
	blocked_until time.Time
}

// NewPool validates options and creates a reusable navigation pool.
func NewPool(options PoolOptions) (*Pool, error) {
	normalized_options, err := normalize_pool_options(options)
	if err != nil {
		return nil, err
	}
	return &Pool{
		options: normalized_options,
		scheduler: &host_scheduler{
			per_host_concurrency: normalized_options.PerHostConcurrency,
			request_interval:     normalized_options.MinRequestInterval,
			states:               make(map[string]*host_schedule_state),
		},
	}, nil
}

// NavigateAll fetches jobs concurrently while preserving input order.
func (p *Pool) NavigateAll(ctx context.Context, jobs []NavigateJob) []NavigateResult {
	results := make([]NavigateResult, len(jobs))
	for job_index, job := range jobs {
		results[job_index] = NavigateResult{Index: job_index, Job: job}
	}
	if len(jobs) == 0 {
		return results
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if p == nil || p.scheduler == nil {
		pool_err := errors.New("minib: navigation pool is not initialized")
		for result_index := range results {
			results[result_index].Err = pool_err
		}
		return results
	}

	worker_count := p.options.Workers
	if worker_count > len(jobs) {
		worker_count = len(jobs)
	}
	job_indexes := make(chan int)
	var workers sync.WaitGroup
	workers.Add(worker_count)
	for worker_index := 0; worker_index < worker_count; worker_index++ {
		go func() {
			defer workers.Done()
			browser, err := NewMiniBrowser(p.options.BrowserTimeout)
			if err != nil {
				for job_index := range job_indexes {
					results[job_index].Err = fmt.Errorf("minib: initialize pool browser: %w", err)
				}
				return
			}
			defer browser.Close()
			browser.request_scheduler = p.scheduler
			for job_index := range job_indexes {
				results[job_index] = p.navigate_one(ctx, browser, job_index, jobs[job_index])
			}
		}()
	}

dispatch_loop:
	for job_index := range jobs {
		select {
		case job_indexes <- job_index:
		case <-ctx.Done():
			break dispatch_loop
		}
	}
	close(job_indexes)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		for result_index := range results {
			if results[result_index].Page == nil && results[result_index].Err == nil {
				results[result_index].Err = err
			}
		}
	}
	return results
}

func normalize_pool_options(options PoolOptions) (PoolOptions, error) {
	defaults := DefaultPoolOptions()
	if options.Workers <= 0 {
		options.Workers = defaults.Workers
	}
	if options.Workers > max_pool_workers {
		return PoolOptions{}, fmt.Errorf("minib: workers must not exceed %d", max_pool_workers)
	}
	if options.PerHostConcurrency <= 0 {
		options.PerHostConcurrency = defaults.PerHostConcurrency
	}
	if options.PerHostConcurrency > max_pool_per_host_concurrency {
		return PoolOptions{}, fmt.Errorf("minib: per-host concurrency must not exceed %d", max_pool_per_host_concurrency)
	}
	if options.MinRequestInterval < 0 {
		return PoolOptions{}, errors.New("minib: minimum request interval must not be negative")
	}
	if options.MinRequestInterval == 0 {
		options.MinRequestInterval = defaults.MinRequestInterval
	}
	if options.MaxRetries < 0 {
		return PoolOptions{}, errors.New("minib: maximum retries must not be negative")
	}
	if options.BaseBackoff < 0 || options.MaxBackoff < 0 {
		return PoolOptions{}, errors.New("minib: retry backoff must not be negative")
	}
	if options.BaseBackoff == 0 {
		options.BaseBackoff = defaults.BaseBackoff
	}
	if options.MaxBackoff == 0 {
		options.MaxBackoff = defaults.MaxBackoff
	}
	if options.MaxBackoff < options.BaseBackoff {
		return PoolOptions{}, errors.New("minib: maximum backoff must be greater than or equal to base backoff")
	}
	if options.BrowserTimeout <= 0 {
		options.BrowserTimeout = defaults.BrowserTimeout
	}
	return options, nil
}

func (p *Pool) navigate_one(ctx context.Context, browser *MiniBrowser, job_index int, job NavigateJob) NavigateResult {
	result := NavigateResult{Index: job_index, Job: job}
	job.URL = strings.TrimSpace(job.URL)
	result.Job.URL = job.URL
	if _, err := request_host(job.URL); err != nil {
		result.Err = err
		return result
	}

	for attempt := 1; attempt <= p.options.MaxRetries+1; attempt++ {
		result.Attempts = attempt
		page, err := browser.Navigate(ctx, job.URL, job.Headers, job.Options)
		result.Page = page
		if err == nil && !retryable_status(page.StatusCode) {
			return result
		}
		if attempt > p.options.MaxRetries || ctx.Err() != nil {
			if err != nil {
				result.Err = err
			} else {
				result.Err = fmt.Errorf("minib: navigation returned retryable HTTP status %d after %d attempt(s)", page.StatusCode, attempt)
			}
			return result
		}

		delay := retry_delay(p.options, attempt, page)
		p.scheduler.defer_host(job.URL, delay)
	}
	return result
}

func retryable_status(status_code int) bool {
	switch status_code {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retry_delay(options PoolOptions, attempt int, page *Page) time.Duration {
	if page != nil {
		if delay, ok := parse_retry_after(page.Headers.Get("Retry-After"), time.Now()); ok {
			if delay > options.MaxBackoff {
				return options.MaxBackoff
			}
			return delay
		}
	}
	delay := options.BaseBackoff
	for multiplier_index := 1; multiplier_index < attempt && delay < options.MaxBackoff; multiplier_index++ {
		if delay > options.MaxBackoff/2 {
			return options.MaxBackoff
		}
		delay *= 2
	}
	if delay > options.MaxBackoff {
		return options.MaxBackoff
	}
	return delay
}

func parse_retry_after(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	retry_at, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	delay := retry_at.Sub(now)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func (s *host_scheduler) before_request(ctx context.Context, raw_url string) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	host, err := request_host(raw_url)
	if err != nil {
		return nil, err
	}
	state := s.state_for(host)
	select {
	case state.semaphore <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	release := func() { <-state.semaphore }

	state.timing_mutex.Lock()
	now := time.Now()
	request_at := now
	if state.next_start.After(request_at) {
		request_at = state.next_start
	}
	if state.blocked_until.After(request_at) {
		request_at = state.blocked_until
	}
	state.next_start = request_at.Add(s.request_interval)
	state.timing_mutex.Unlock()

	if wait_duration := time.Until(request_at); wait_duration > 0 {
		wait_timer := time.NewTimer(wait_duration)
		defer wait_timer.Stop()
		select {
		case <-wait_timer.C:
		case <-ctx.Done():
			release()
			return nil, ctx.Err()
		}
	}
	return release, nil
}

func (s *host_scheduler) observe_response(raw_url string, status_code int, headers http.Header) {
	if status_code != http.StatusTooManyRequests && status_code != http.StatusServiceUnavailable {
		return
	}
	delay, ok := parse_retry_after(headers.Get("Retry-After"), time.Now())
	if !ok {
		return
	}
	s.defer_host(raw_url, delay)
}

func (s *host_scheduler) defer_host(raw_url string, delay time.Duration) {
	if s == nil || delay <= 0 {
		return
	}
	host, err := request_host(raw_url)
	if err != nil {
		return
	}
	state := s.state_for(host)
	blocked_until := time.Now().Add(delay)
	state.timing_mutex.Lock()
	if blocked_until.After(state.blocked_until) {
		state.blocked_until = blocked_until
	}
	state.timing_mutex.Unlock()
}

func (s *host_scheduler) state_for(host string) *host_schedule_state {
	s.states_mutex.Lock()
	defer s.states_mutex.Unlock()
	if state := s.states[host]; state != nil {
		return state
	}
	state := &host_schedule_state{semaphore: make(chan struct{}, s.per_host_concurrency)}
	s.states[host] = state
	return state
}

func request_host(raw_url string) (string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return "", fmt.Errorf("minib: parse request URL: %w", err)
	}
	if parsed_url.Scheme != "http" && parsed_url.Scheme != "https" && parsed_url.Scheme != "ws" && parsed_url.Scheme != "wss" {
		return "", fmt.Errorf("minib: unsupported request URL scheme %q", parsed_url.Scheme)
	}
	if parsed_url.Host == "" {
		return "", errors.New("minib: request URL has no host")
	}
	return strings.ToLower(parsed_url.Host), nil
}
