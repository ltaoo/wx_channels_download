package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoolLimitsPerHostConcurrency(t *testing.T) {
	var active_requests atomic.Int32
	var maximum_active atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		active := active_requests.Add(1)
		defer active_requests.Add(-1)
		for {
			maximum := maximum_active.Load()
			if active <= maximum || maximum_active.CompareAndSwap(maximum, active) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		_, _ = fmt.Fprint(writer, "<!doctype html><title>ok</title>")
	}))
	defer server.Close()

	options := DefaultPoolOptions()
	options.Workers = 6
	options.PerHostConcurrency = 2
	options.MinRequestInterval = time.Millisecond
	options.MaxRetries = 0
	pool, err := NewPool(options)
	if err != nil {
		t.Fatal(err)
	}
	jobs := make([]NavigateJob, 8)
	for job_index := range jobs {
		jobs[job_index].URL = fmt.Sprintf("%s/page/%d", server.URL, job_index)
	}
	results := pool.NavigateAll(context.Background(), jobs)
	for _, result := range results {
		if result.Err != nil {
			t.Fatalf("job %d: %v", result.Index, result.Err)
		}
		if result.Page == nil || result.Page.StatusCode != http.StatusOK {
			t.Fatalf("job %d page=%#v", result.Index, result.Page)
		}
	}
	if maximum := maximum_active.Load(); maximum > 2 {
		t.Fatalf("maximum active requests = %d, want <= 2", maximum)
	}
}

func TestPoolSpacesRequestStarts(t *testing.T) {
	var request_mutex sync.Mutex
	request_times := make([]time.Time, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request_mutex.Lock()
		request_times = append(request_times, time.Now())
		request_mutex.Unlock()
		_, _ = fmt.Fprint(writer, "<!doctype html><title>ok</title>")
	}))
	defer server.Close()

	options := DefaultPoolOptions()
	options.Workers = 3
	options.PerHostConcurrency = 3
	options.MinRequestInterval = 40 * time.Millisecond
	options.MaxRetries = 0
	pool, err := NewPool(options)
	if err != nil {
		t.Fatal(err)
	}
	jobs := []NavigateJob{{URL: server.URL + "/1"}, {URL: server.URL + "/2"}, {URL: server.URL + "/3"}}
	results := pool.NavigateAll(context.Background(), jobs)
	for _, result := range results {
		if result.Err != nil {
			t.Fatalf("job %d: %v", result.Index, result.Err)
		}
	}
	request_mutex.Lock()
	defer request_mutex.Unlock()
	if len(request_times) != len(jobs) {
		t.Fatalf("request count = %d, want %d", len(request_times), len(jobs))
	}
	for request_index := 1; request_index < len(request_times); request_index++ {
		if gap := request_times[request_index].Sub(request_times[request_index-1]); gap < 30*time.Millisecond {
			t.Fatalf("request gap %d = %s, want >= 30ms", request_index, gap)
		}
	}
}

func TestPoolRetriesRetryAfter(t *testing.T) {
	var request_count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request_count.Add(1) == 1 {
			writer.Header().Set("Retry-After", "1")
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(writer, "slow down")
			return
		}
		_, _ = fmt.Fprint(writer, "<!doctype html><title>ok</title>")
	}))
	defer server.Close()

	options := DefaultPoolOptions()
	options.Workers = 1
	options.PerHostConcurrency = 1
	options.MinRequestInterval = time.Millisecond
	options.MaxRetries = 1
	options.MaxBackoff = 2 * time.Second
	pool, err := NewPool(options)
	if err != nil {
		t.Fatal(err)
	}
	started_at := time.Now()
	results := pool.NavigateAll(context.Background(), []NavigateJob{{URL: server.URL}})
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if results[0].Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", results[0].Attempts)
	}
	if elapsed := time.Since(started_at); elapsed < 900*time.Millisecond {
		t.Fatalf("retry elapsed = %s, want >= 900ms", elapsed)
	}
	if count := request_count.Load(); count != 2 {
		t.Fatalf("request count = %d, want 2", count)
	}
}

func TestPoolDoesNotRetryForbidden(t *testing.T) {
	var request_count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request_count.Add(1)
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	options := DefaultPoolOptions()
	options.MinRequestInterval = time.Millisecond
	pool, err := NewPool(options)
	if err != nil {
		t.Fatal(err)
	}
	results := pool.NavigateAll(context.Background(), []NavigateJob{{URL: server.URL}})
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if results[0].Page == nil || results[0].Page.StatusCode != http.StatusForbidden {
		t.Fatalf("page=%#v", results[0].Page)
	}
	if count := request_count.Load(); count != 1 {
		t.Fatalf("request count = %d, want 1", count)
	}
}

func TestNewPoolRejectsAggressivePerHostConcurrency(t *testing.T) {
	options := DefaultPoolOptions()
	options.PerHostConcurrency = max_pool_per_host_concurrency + 1
	if _, err := NewPool(options); err == nil {
		t.Fatal("expected excessive per-host concurrency to fail")
	}
}
