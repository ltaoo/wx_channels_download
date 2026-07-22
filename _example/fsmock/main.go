package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DownloadConfig holds the parameters for a mock download.
type DownloadConfig struct {
	FileSize  int64
	Speed     int64         // bytes per second, 0 = unlimited
	Delay     time.Duration // initial delay
	FailRate  float64       // 0.0–1.0 random failure probability
	Fail      bool          // randomly fail mid-stream
	ChunkSize int
	Filename  string
}

// MockDownloadServer is the main server struct.
type MockDownloadServer struct {
	port        int
	activeTasks sync.Map
	mu          sync.Mutex
	stats       ServerStats
}

// ServerStats tracks aggregate metrics.
type ServerStats struct {
	TotalRequests      int64
	TotalBytes         int64
	ActiveDownloads    int64
	CompletedDownloads int64
}

// DownloadTask represents an active download.
type DownloadTask struct {
	ID         string
	Config     DownloadConfig
	StartTime  time.Time
	BytesSent  int64
	IsActive   bool
	CancelChan chan struct{}
	mu         sync.Mutex
}

var defaultConfig = DownloadConfig{
	FileSize:  10 * 1024 * 1024, // 10MB
	Speed:     1024 * 1024,      // 1MB/s
	ChunkSize: 32 * 1024,        // 32KB
	FailRate:  0,
}

func main() {
	server := NewMockDownloadServer(7001)

	fmt.Println("Mock Download Server started!")
	fmt.Println("Usage examples:")
	fmt.Println("  1. Simple download:     http://localhost:7001/download")
	fmt.Println("  2. Custom size:         http://localhost:7001/download?size=50M")
	fmt.Println("  3. Custom speed:        http://localhost:7001/download?size=100M&speed=500K")
	fmt.Println("  4. Batch download:      http://localhost:7001/batch?count=5&size=20M")
	fmt.Println("  5. Stats:               http://localhost:7001/stats")
	fmt.Println("  6. List tasks:          http://localhost:7001/tasks")
	fmt.Println("  7. Test Cases:          http://localhost:7001/testcases")
	fmt.Println("  8. Stream:              http://localhost:7001/stream?size=1G&speed=5M")
	fmt.Println("\nSize units:  B, K, M, G (case-insensitive)")
	fmt.Println("Speed units: B/s, K/s, M/s, G/s")

	log.Fatal(server.Start())
}

// NewMockDownloadServer creates a new server instance.
func NewMockDownloadServer(port int) *MockDownloadServer {
	return &MockDownloadServer{
		port:  port,
		stats: ServerStats{},
	}
}

// Start begins listening.
func (s *MockDownloadServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/download", s.handleDownload)
	mux.HandleFunc("/batch", s.handleBatchDownload)
	mux.HandleFunc("/stream", s.handleStreamDownload)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/tasks", s.handleTasks)
	mux.HandleFunc("/task/", s.handleTaskControl)
	mux.HandleFunc("/cancel/", s.handleCancelDownload)
	mux.HandleFunc("/testcases", s.handleTestCases)

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Server listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *MockDownloadServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, rootHTML)
}

func (s *MockDownloadServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&s.stats.TotalRequests, 1)
	atomic.AddInt64(&s.stats.ActiveDownloads, 1)
	defer atomic.AddInt64(&s.stats.ActiveDownloads, -1)

	config := parseQueryConfig(r)

	if config.FailRate > 0 && rand.Float64() < config.FailRate {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Random failure injected"))
		return
	}

	if config.Delay > 0 {
		time.Sleep(config.Delay)
	}

	taskID := fmt.Sprintf("download-%d-%d", time.Now().UnixNano(), rand.Int63())
	task := &DownloadTask{
		ID:         taskID,
		Config:     config,
		StartTime:  time.Now(),
		IsActive:   true,
		CancelChan: make(chan struct{}, 1),
	}
	s.activeTasks.Store(taskID, task)
	defer s.activeTasks.Delete(taskID)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", config.FileSize))
	w.Header().Set("Accept-Ranges", "bytes")

	filename := config.Filename
	if filename == "" {
		filename = fmt.Sprintf("testfile_%s.bin", formatSize(config.FileSize))
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		s.handleRangeRequest(w, r, config, task)
		return
	}

	s.streamData(w, config, task)
	atomic.AddInt64(&s.stats.CompletedDownloads, 1)
	atomic.AddInt64(&s.stats.TotalBytes, config.FileSize)
}

func (s *MockDownloadServer) handleRangeRequest(w http.ResponseWriter, r *http.Request, config DownloadConfig, task *DownloadTask) {
	rangeHeader := r.Header.Get("Range")
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	rangeStr := rangeHeader[6:]
	var start, end int64
	n, _ := fmt.Sscanf(rangeStr, "%d-%d", &start, &end)

	if n != 2 || start < 0 || end >= config.FileSize || start > end {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	contentLength := end - start + 1
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, config.FileSize))
	w.WriteHeader(http.StatusPartialContent)
	s.streamRange(w, start, end, config, task)
	atomic.AddInt64(&s.stats.TotalBytes, contentLength)
}

func (s *MockDownloadServer) handleBatchDownload(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	count := 1
	if countStr := query.Get("count"); countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil && n > 0 && n <= 10 {
			count = n
		}
	}
	config := parseQueryConfig(r)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Batch Download Links</title>
<style>body{font-family:Arial,sans-serif;margin:40px}.link{margin:10px 0;padding:10px;background:#f0f0f0}.stats{margin:20px 0;padding:15px;background:#e8f4f8}button{margin:5px;padding:8px 16px}</style></head><body>
<h1>Batch Download Links (%d files)</h1>
<div class="stats"><p>File Size: %s | Speed: %s/s | Total: %s</p></div>`,
		count, formatSize(config.FileSize), formatSpeed(config.Speed), formatSize(config.FileSize*int64(count)))

	for i := 0; i < count; i++ {
		url := fmt.Sprintf("/download?size=%s&speed=%s&delay=%s",
			formatSizeForURL(config.FileSize), formatSpeedForURL(config.Speed),
			time.Duration(i)*time.Second)
		fmt.Fprintf(w, "<div class=\"link\"><strong>File %d:</strong> %s @ %s/s <a href=\"%s\">Download</a></div>\n",
			i+1, formatSize(config.FileSize), formatSpeed(config.Speed), url)
	}
	fmt.Fprint(w, `<p><a href="/">Back to Home</a></p></body></html>`)
}

func (s *MockDownloadServer) handleStreamDownload(w http.ResponseWriter, r *http.Request) {
	config := parseQueryConfig(r)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"stream_%s.bin\"", formatSize(config.FileSize)))

	chunkSize := 64 * 1024
	if config.ChunkSize > 0 {
		chunkSize = config.ChunkSize
	}

	totalSent := int64(0)
	startTime := time.Now()
	failAt := int64(-1)
	if config.Fail {
		min := int64(float64(config.FileSize) * 0.1)
		max := int64(float64(config.FileSize) * 0.9)
		if max > min {
			failAt = min + rand.Int63n(max-min)
		} else {
			failAt = rand.Int63n(config.FileSize)
		}
	}

	for totalSent < config.FileSize {
		if failAt >= 0 && totalSent >= failAt {
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					conn.Close()
				}
			}
			return
		}

		remaining := config.FileSize - totalSent
		currentChunk := chunkSize
		if int64(currentChunk) > remaining {
			currentChunk = int(remaining)
		}

		chunk := generateChunk(currentChunk, totalSent)
		fmt.Fprintf(w, "%x\r\n", len(chunk))
		w.Write(chunk)
		fmt.Fprint(w, "\r\n")

		totalSent += int64(currentChunk)

		if config.Speed > 0 {
			expectedTime := time.Duration(float64(totalSent) / float64(config.Speed) * float64(time.Second))
			elapsedTime := time.Since(startTime)
			if expectedTime > elapsedTime {
				time.Sleep(expectedTime - elapsedTime)
			}
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	fmt.Fprint(w, "0\r\n\r\n")
}

func (s *MockDownloadServer) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.getStats()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Server Statistics</title>
<style>body{font-family:Arial,sans-serif;margin:40px}table{border-collapse:collapse;width:100%%}th,td{border:1px solid #ddd;padding:12px;text-align:left}th{background:#4CAF50;color:white}tr:nth-child(even){background:#f2f2f2}</style></head><body>
<h1>Mock Download Server Statistics</h1>
<table>
<tr><th>Metric</th><th>Value</th></tr>
<tr><td>Total Requests</td><td>%d</td></tr>
<tr><td>Total Bytes Sent</td><td>%s</td></tr>
<tr><td>Active Downloads</td><td>%d</td></tr>
<tr><td>Completed Downloads</td><td>%d</td></tr>
</table><p><a href="/">Back to Home</a></p></body></html>`,
		stats.TotalRequests, formatSize(stats.TotalBytes), stats.ActiveDownloads, stats.CompletedDownloads)
}

func (s *MockDownloadServer) handleTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tasks := []map[string]interface{}{}
	s.activeTasks.Range(func(key, value interface{}) bool {
		if task, ok := value.(*DownloadTask); ok {
			tasks = append(tasks, map[string]interface{}{
				"id":         task.ID,
				"active":     task.IsActive,
				"start_time": task.StartTime.Format(time.RFC3339),
				"bytes_sent": atomic.LoadInt64(&task.BytesSent),
				"file_size":  task.Config.FileSize,
				"speed":      task.Config.Speed,
				"progress":   float64(atomic.LoadInt64(&task.BytesSent)) / float64(task.Config.FileSize) * 100,
			})
		}
		return true
	})

	fmt.Fprintf(w, `{"tasks": %d, "data": %v}`, len(tasks), tasks)
}

func (s *MockDownloadServer) handleTaskControl(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/task/")
	if taskID == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}
	if val, ok := s.activeTasks.Load(taskID); ok {
		task := val.(*DownloadTask)
		action := r.URL.Query().Get("action")
		switch action {
		case "cancel":
			select {
			case task.CancelChan <- struct{}{}:
				task.IsActive = false
				w.Write([]byte("Task cancelled"))
			default:
				w.Write([]byte("Task already cancelled"))
			}
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
		}
	} else {
		http.Error(w, "Task not found", http.StatusNotFound)
	}
}

func (s *MockDownloadServer) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/cancel/")
	if taskID == "" {
		http.Redirect(w, r, "/tasks", http.StatusFound)
		return
	}
	if val, ok := s.activeTasks.Load(taskID); ok {
		task := val.(*DownloadTask)
		select {
		case task.CancelChan <- struct{}{}:
			task.IsActive = false
		default:
		}
	}
	http.Redirect(w, r, "/tasks", http.StatusFound)
}

func (s *MockDownloadServer) handleTestCases(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("text") == "true" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for i := 1; i <= 20; i++ {
			fmt.Fprintf(w, "http://localhost:%d/download?filename=file_page_%02d.bin&size=2M\n", s.port, i)
		}
		for i := 1; i <= 10; i++ {
			failRate := float64(i) * 0.1
			url := fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&fail=%.1f", i, failRate)
			if i == 10 {
				url = fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&error=true", i)
			}
			fmt.Fprintf(w, "http://localhost:%d%s\n", s.port, url)
		}
		for i := 1; i <= 5; i++ {
			fmt.Fprintf(w, "http://localhost:%d/download?filename=slow_%02d.bin&size=5M&speed=%dK\n", s.port, i, 100*i)
		}
		for i := 1; i <= 5; i++ {
			fmt.Fprintf(w, "http://localhost:%d/download?filename=delay_%02d.bin&size=1M&delay=%ds\n", s.port, i, i)
		}
		sizes := []string{"1B", "100B", "1K", "10K", "1M", "10M", "100M", "1G", "2G", "100M"}
		for i, size := range sizes {
			fmt.Fprintf(w, "http://localhost:%d/download?filename=edge_%02d.bin&size=%s\n", s.port, i+1, size)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>50 Test Cases</title>
<style>body{font-family:monospace;margin:40px}.group{margin-bottom:20px;border-bottom:1px solid #eee;padding-bottom:10px}a{text-decoration:none;color:#06c}a:hover{text-decoration:underline}textarea{width:100%%;height:300px;margin-top:10px;font-family:monospace}</style></head><body>
<h1>50 Download Test Links</h1>
<h3>Copy All Links:</h3><textarea readonly id="all-links">`)

	for i := 1; i <= 20; i++ {
		fmt.Fprintf(w, "http://localhost:%d/download?filename=file_page_%02d.bin&size=2M\n", s.port, i)
	}
	for i := 1; i <= 10; i++ {
		failRate := float64(i) * 0.1
		url := fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&fail=%.1f", i, failRate)
		if i == 10 {
			url = fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&error=true", i)
		}
		fmt.Fprintf(w, "http://localhost:%d%s\n", s.port, url)
	}
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(w, "http://localhost:%d/download?filename=slow_%02d.bin&size=5M&speed=%dK\n", s.port, i, 100*i)
	}
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(w, "http://localhost:%d/download?filename=delay_%02d.bin&size=1M&delay=%ds\n", s.port, i, i)
	}
	sizes := []string{"1B", "100B", "1K", "10K", "1M", "10M", "100M", "1G", "2G", "100M"}
	for i, size := range sizes {
		fmt.Fprintf(w, "http://localhost:%d/download?filename=edge_%02d.bin&size=%s\n", s.port, i+1, size)
	}

	fmt.Fprint(w, `</textarea>
<button onclick="document.getElementById('all-links').select();document.execCommand('copy');">Copy All</button><hr>
<h3>Detailed List</h3>`)

	fmt.Fprint(w, "<div class='group'><h3>1. Pagination / Standard Files (1-20)</h3>")
	for i := 1; i <= 20; i++ {
		url := fmt.Sprintf("/download?filename=file_page_%02d.bin&size=2M", i)
		fmt.Fprintf(w, "<div>%02d. <a href='%s'>http://localhost:%d%s</a> (2MB)</div>\n", i, url, s.port, url)
	}
	fmt.Fprint(w, "</div>")

	fmt.Fprint(w, "<div class='group'><h3>2. Failure Scenarios (21-30)</h3>")
	for i := 1; i <= 10; i++ {
		failRate := float64(i) * 0.1
		url := fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&fail=%.1f", i, failRate)
		desc := fmt.Sprintf("Fail Rate %.0f%%", failRate*100)
		if i == 10 {
			url = fmt.Sprintf("/download?filename=fail_%02d.bin&size=10M&error=true", i)
			desc = "Immediate Error"
		}
		fmt.Fprintf(w, "<div>%d. <a href='%s'>http://localhost:%d%s</a> (%s)</div>\n", 20+i, url, s.port, url, desc)
	}
	fmt.Fprint(w, "</div>")

	fmt.Fprint(w, "<div class='group'><h3>3. Slow & Delayed (31-40)</h3>")
	for i := 1; i <= 5; i++ {
		url := fmt.Sprintf("/download?filename=slow_%02d.bin&size=5M&speed=%dK", i, 100*i)
		fmt.Fprintf(w, "<div>%d. <a href='%s'>http://localhost:%d%s</a> (Speed %dKB/s)</div>\n", 30+i, url, s.port, url, 100*i)
	}
	for i := 1; i <= 5; i++ {
		url := fmt.Sprintf("/download?filename=delay_%02d.bin&size=1M&delay=%ds", i, i)
		fmt.Fprintf(w, "<div>%d. <a href='%s'>http://localhost:%d%s</a> (Delay %ds)</div>\n", 35+i, url, s.port, url, i)
	}
	fmt.Fprint(w, "</div>")

	fmt.Fprint(w, "<div class='group'><h3>4. Edge Cases (41-50)</h3>")
	for i, size := range sizes {
		url := fmt.Sprintf("/download?filename=edge_%02d.bin&size=%s", i+1, size)
		fmt.Fprintf(w, "<div>%d. <a href='%s'>http://localhost:%d%s</a> (%s)</div>\n", 40+i+1, url, s.port, url, size)
	}
	fmt.Fprint(w, "</div>")

	fmt.Fprint(w, "<p><a href=\"/\">Back to Home</a></p></body></html>")
}

// ---------------------------------------------------------------------------
// Data streaming
// ---------------------------------------------------------------------------

func (s *MockDownloadServer) streamData(w http.ResponseWriter, config DownloadConfig, task *DownloadTask) {
	chunkSize := config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 32 * 1024
	}

	startTime := time.Now()
	totalSent := int64(0)

	failAt := int64(-1)
	if config.Fail {
		min := int64(float64(config.FileSize) * 0.1)
		max := int64(float64(config.FileSize) * 0.9)
		if max > min {
			failAt = min + rand.Int63n(max-min)
		} else {
			failAt = rand.Int63n(config.FileSize)
		}
	}

	for totalSent < config.FileSize {
		select {
		case <-task.CancelChan:
			return
		default:
		}

		if failAt >= 0 && totalSent >= failAt {
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					conn.Close()
				}
			}
			return
		}

		remaining := config.FileSize - totalSent
		currentChunk := chunkSize
		if int64(currentChunk) > remaining {
			currentChunk = int(remaining)
		}

		chunk := generateChunk(currentChunk, totalSent)

		n, err := w.Write(chunk)
		if err != nil {
			return
		}

		totalSent += int64(n)
		atomic.AddInt64(&task.BytesSent, int64(n))

		if config.Speed > 0 {
			expectedTime := time.Duration(float64(totalSent) / float64(config.Speed) * float64(time.Second))
			elapsedTime := time.Since(startTime)
			if expectedTime > elapsedTime {
				time.Sleep(expectedTime - elapsedTime)
			}
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	task.IsActive = false
}

func (s *MockDownloadServer) streamRange(w http.ResponseWriter, start, end int64, config DownloadConfig, task *DownloadTask) {
	chunkSize := config.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 32 * 1024
	}

	current := start
	startTime := time.Now()

	failAt := int64(-1)
	if config.Fail {
		rangeLen := end - start + 1
		min := int64(float64(rangeLen) * 0.1)
		max := int64(float64(rangeLen) * 0.9)
		var relativeFailAt int64
		if max > min {
			relativeFailAt = min + rand.Int63n(max-min)
		} else {
			relativeFailAt = rand.Int63n(rangeLen)
		}
		failAt = start + relativeFailAt
	}

	for current <= end {
		select {
		case <-task.CancelChan:
			return
		default:
		}

		if failAt >= 0 && current >= failAt {
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					conn.Close()
				}
			}
			return
		}

		remaining := end - current + 1
		currentChunk := chunkSize
		if int64(currentChunk) > remaining {
			currentChunk = int(remaining)
		}

		chunk := generateChunk(currentChunk, current)

		n, err := w.Write(chunk)
		if err != nil {
			return
		}

		current += int64(n)
		atomic.AddInt64(&task.BytesSent, int64(n))

		if config.Speed > 0 {
			sent := current - start
			expectedTime := time.Duration(float64(sent) / float64(config.Speed) * float64(time.Second))
			elapsedTime := time.Since(startTime)
			if expectedTime > elapsedTime {
				time.Sleep(expectedTime - elapsedTime)
			}
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *MockDownloadServer) getStats() ServerStats {
	return ServerStats{
		TotalRequests:      atomic.LoadInt64(&s.stats.TotalRequests),
		TotalBytes:         atomic.LoadInt64(&s.stats.TotalBytes),
		ActiveDownloads:    atomic.LoadInt64(&s.stats.ActiveDownloads),
		CompletedDownloads: atomic.LoadInt64(&s.stats.CompletedDownloads),
	}
}

func parseQueryConfig(r *http.Request) DownloadConfig {
	config := defaultConfig
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if size, err := parseSize(sizeStr); err == nil {
			config.FileSize = size
		}
	}
	if speedStr := r.URL.Query().Get("speed"); speedStr != "" {
		if speed, err := parseSpeed(speedStr); err == nil {
			config.Speed = speed
		}
	}
	if chunkStr := r.URL.Query().Get("chunk"); chunkStr != "" {
		if chunk, err := parseSize(chunkStr); err == nil {
			config.ChunkSize = int(chunk)
		}
	}
	if delayStr := r.URL.Query().Get("delay"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			config.Delay = delay
		}
	}
	if failStr := r.URL.Query().Get("fail"); failStr != "" {
		if failRate, err := strconv.ParseFloat(failStr, 64); err == nil && failRate >= 0 && failRate <= 1 {
			config.FailRate = failRate
		}
	}
	if r.URL.Query().Has("error") {
		config.Fail = true
	}
	if filename := r.URL.Query().Get("filename"); filename != "" {
		config.Filename = filename
	}
	return config
}

func parseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return defaultConfig.FileSize, nil
	}
	sizeStr = strings.ToUpper(sizeStr)
	var multiplier int64 = 1
	if strings.HasSuffix(sizeStr, "G") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "G")
	} else if strings.HasSuffix(sizeStr, "M") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "M")
	} else if strings.HasSuffix(sizeStr, "K") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "K")
	} else if strings.HasSuffix(sizeStr, "B") {
		sizeStr = strings.TrimSuffix(sizeStr, "B")
	}
	value, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0, err
	}
	return int64(value * float64(multiplier)), nil
}

func parseSpeed(speedStr string) (int64, error) {
	if speedStr == "" {
		return defaultConfig.Speed, nil
	}
	speedStr = strings.ToUpper(speedStr)
	speedStr = strings.TrimSuffix(speedStr, "/S")
	return parseSize(speedStr)
}

func generateChunk(size int, offset int64) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte((offset + int64(i)) % 256)
	}
	return data
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatSpeed(bytesPerSec int64) string {
	return formatSize(bytesPerSec)
}

func formatSizeForURL(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%d%c", bytes/div, "KMGTPE"[exp])
}

func formatSpeedForURL(bytesPerSec int64) string {
	return formatSizeForURL(bytesPerSec)
}

// ---------------------------------------------------------------------------
// Root HTML page
// ---------------------------------------------------------------------------

const rootHTML = `<!DOCTYPE html>
<html>
<head>
	<title>Mock Download Server</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 40px; }
		.card { background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0; }
		.code { background: #333; color: #fff; padding: 10px; border-radius: 4px; font-family: monospace; }
		table { border-collapse: collapse; width: 100%; margin: 20px 0; }
		th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
		th { background: #4CAF50; color: white; }
		.form-group { margin: 10px 0; }
		input, select { padding: 8px; width: 200px; }
		button { padding: 10px 20px; background: #4CAF50; color: white; border: none; cursor: pointer; }
	</style>
</head>
<body>
	<h1>Mock Download Server</h1>

	<div class="card">
		<h2>Quick Download</h2>
		<a href="/download?size=10M&speed=1M/s" target="_blank">10MB @ 1MB/s</a><br>
		<a href="/download?size=50M&speed=5M/s" target="_blank">50MB @ 5MB/s</a><br>
		<a href="/download?size=100M&speed=10M/s" target="_blank">100MB @ 10MB/s</a><br>
		<a href="/download?size=1G&speed=50M/s" target="_blank">1GB @ 50MB/s</a>
	</div>

	<div class="card">
		<h2>Custom Download</h2>
		<form action="/download" method="get" target="_blank">
			<div class="form-group">
				<label>File Size: </label>
				<input type="text" name="size" value="100M" placeholder="e.g., 100M, 1G">
			</div>
			<div class="form-group">
				<label>Speed: </label>
				<input type="text" name="speed" value="1M/s" placeholder="e.g., 1M/s, 500K/s">
			</div>
			<div class="form-group">
				<label>Chunk Size: </label>
				<select name="chunk">
					<option value="">Default (32KB)</option>
					<option value="64K">64KB</option>
					<option value="128K">128KB</option>
					<option value="1M">1MB</option>
				</select>
			</div>
			<button type="submit">Start Download</button>
		</form>
	</div>

	<div class="card">
		<h2>Batch Download</h2>
		<form action="/batch" method="get" target="_blank">
			<div class="form-group">
				<label>Count: </label>
				<input type="number" name="count" value="3" min="1" max="10">
			</div>
			<div class="form-group">
				<label>File Size: </label>
				<input type="text" name="size" value="10M">
			</div>
			<div class="form-group">
				<label>Speed: </label>
				<input type="text" name="speed" value="1M/s">
			</div>
			<button type="submit">Start Batch</button>
		</form>
	</div>

	<div class="card">
		<h2>API Examples</h2>
		<div class="code">
			# Single download<br>
			GET /download?size=100M&speed=1M/s<br>
			<br>
			# Batch download (3 files)<br>
			GET /batch?count=3&size=20M&speed=500K/s<br>
			<br>
			# Stream download<br>
			GET /stream?size=1G&speed=5M/s<br>
			<br>
			# With Range header<br>
			curl -H "Range: bytes=0-1048575" http://localhost:7001/download?size=10M<br>
			<br>
			# Get statistics<br>
			GET /stats<br>
			<br>
			# List active tasks<br>
			GET /tasks<br>
		</div>
	</div>

	<div class="card">
		<h2>Current Statistics</h2>
		<a href="/stats">View Detailed Stats</a>
		<br><br>
		<a href="/testcases">View 50 Test Cases</a>
	</div>
</body>
</html>`
