package ttk

import (
	"context"
	"sync"
	"time"
)

const ttk_request_interval = 1500 * time.Millisecond

var ttk_outbound_request_limiter = new_ttk_request_limiter(ttk_request_interval)

// ttk_request_limiter spaces request starts across all TTK clients so parallel
// scraper jobs cannot collectively produce an unthrottled burst.
type ttk_request_limiter struct {
	request_mu       sync.Mutex
	request_interval time.Duration
	last_request_at  time.Time
}

func new_ttk_request_limiter(request_interval time.Duration) *ttk_request_limiter {
	return &ttk_request_limiter{request_interval: request_interval}
}

func (r *ttk_request_limiter) wait(request_context context.Context) error {
	if request_context == nil {
		request_context = context.Background()
	}
	if err := request_context.Err(); err != nil {
		return err
	}
	if r == nil || r.request_interval <= 0 {
		return nil
	}

	r.request_mu.Lock()
	defer r.request_mu.Unlock()

	if !r.last_request_at.IsZero() {
		wait_duration := r.request_interval - time.Since(r.last_request_at)
		if wait_duration > 0 {
			wait_timer := time.NewTimer(wait_duration)
			defer wait_timer.Stop()
			select {
			case <-request_context.Done():
				return request_context.Err()
			case <-wait_timer.C:
			}
		}
	}
	if err := request_context.Err(); err != nil {
		return err
	}
	r.last_request_at = time.Now()
	return nil
}
