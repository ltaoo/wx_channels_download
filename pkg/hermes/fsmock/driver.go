package fsmock

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wx_channel/pkg/hermes"
)

// ---------------------------------------------------------------------------
// MemoryDriver – in-memory data serving protocol driver
// ---------------------------------------------------------------------------

// MemoryDriver implements hermes.ProtocolDriver by serving data from an
// in-memory buffer. It never supports Range and always returns the full
// buffer on Open.
type MemoryDriver struct {
	Data        []byte
	ContentType string
}

// Protocols returns the protocol identifiers for this driver.
func (d *MemoryDriver) Protocols() []string { return []string{"memory"} }

// Prepare returns the prepared resource information for the in-memory data.
func (d *MemoryDriver) Prepare(_ context.Context, _ hermes.Endpoint) (hermes.PreparedResource, error) {
	return hermes.PreparedResource{
		Size:        int64(len(d.Data)),
		ContentType: d.ContentType,
	}, nil
}

// Open returns a ReadCloser over the in-memory data. ReadRequest is ignored.
func (d *MemoryDriver) Open(_ context.Context, _ hermes.Endpoint, _ hermes.ReadRequest) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(d.Data)), nil
}

// ---------------------------------------------------------------------------
// HTTPMockServer – httptest-based HTTP server with configurable behaviour
// ---------------------------------------------------------------------------

// HTTPConfig holds configuration for the HTTP mock server.
type HTTPConfig struct {
	FileSize      int64
	Speed         int64 // bytes per second, 0 = unlimited
	ChunkSize     int
	Delay         time.Duration
	FailRate      float64 // 0.0 – 1.0
	FailAtPercent float64 // fail after this percent, 0 = don't fail mid-stream
	Filename      string
	ContentType   string
	SupportsRange bool
}

// HTTPOption is a functional option for HTTPConfig.
type HTTPOption func(*HTTPConfig)

// WithFileSize sets the total file size for the mock server.
func WithFileSize(size int64) HTTPOption {
	return func(c *HTTPConfig) { c.FileSize = size }
}

// WithSpeed sets the throttled delivery speed in bytes per second.
func WithSpeed(speed int64) HTTPOption {
	return func(c *HTTPConfig) { c.Speed = speed }
}

// WithChunkSize sets the chunk size for streaming (default 32KB).
func WithChunkSize(size int) HTTPOption {
	return func(c *HTTPConfig) { c.ChunkSize = size }
}

// WithDelay adds an initial delay before sending data.
func WithDelay(d time.Duration) HTTPOption {
	return func(c *HTTPConfig) { c.Delay = d }
}

// WithFailRate sets the probability (0-1) that the server returns 500.
func WithFailRate(rate float64) HTTPOption {
	return func(c *HTTPConfig) { c.FailRate = rate }
}

// WithMidStreamFailure causes the server to close the connection after
// delivering the specified percentage of the file.
func WithMidStreamFailure(percent float64) HTTPOption {
	return func(c *HTTPConfig) { c.FailAtPercent = percent }
}

// WithFilename sets the Content-Disposition filename.
func WithFilename(name string) HTTPOption {
	return func(c *HTTPConfig) { c.Filename = name }
}

// WithContentType sets the response Content-Type header.
func WithContentType(ct string) HTTPOption {
	return func(c *HTTPConfig) { c.ContentType = ct }
}

// WithRangeSupport enables Range request handling.
func WithRangeSupport(enabled bool) HTTPOption {
	return func(c *HTTPConfig) { c.SupportsRange = enabled }
}

// DefaultHTTPConfig returns the default HTTP server configuration.
func DefaultHTTPConfig() HTTPConfig {
	return HTTPConfig{
		FileSize:      10 * 1024 * 1024,
		ChunkSize:     32 * 1024,
		Filename:      "test.bin",
		ContentType:   "application/octet-stream",
		SupportsRange: true,
	}
}

// HTTPMockServer wraps an httptest.Server with configurable download
// simulation abilities.
type HTTPMockServer struct {
	Server *httptest.Server
	Config HTTPConfig
	Data   []byte // pre-generated patterned data
}

// NewHTTPMockServer creates a new HTTP mock server with the given options.
func NewHTTPMockServer(opts ...HTTPOption) *HTTPMockServer {
	cfg := DefaultHTTPConfig()
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 32 * 1024
	}
	// Pre-generate patterned data for consistent verification.
	data := GenerateData(cfg.FileSize)
	srv := &HTTPMockServer{Config: cfg, Data: data}
	srv.Server = httptest.NewServer(http.HandlerFunc(srv.handler))
	return srv
}

// URL returns the server's base URL.
func (s *HTTPMockServer) URL() string { return s.Server.URL }

// Close shuts down the server.
func (s *HTTPMockServer) Close() { s.Server.Close() }

func (s *HTTPMockServer) handler(w http.ResponseWriter, r *http.Request) {
	cfg := s.Config

	// Initial delay.
	if cfg.Delay > 0 {
		time.Sleep(cfg.Delay)
	}

	// Random failure.
	if cfg.FailRate > 0 && rand.Float64() < cfg.FailRate {
		http.Error(w, "random failure", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", cfg.ContentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"%s\"", cfg.Filename))

	if cfg.SupportsRange {
		w.Header().Set("Accept-Ranges", "bytes")
	}

	// Handle Range request.
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" && cfg.SupportsRange {
		s.handleRange(w, r, rangeHeader)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(cfg.FileSize, 10))
	w.WriteHeader(http.StatusOK)
	s.streamData(w, 0, cfg.FileSize)
}

func (s *HTTPMockServer) handleRange(w http.ResponseWriter, r *http.Request, rangeHeader string) {
	cfg := s.Config

	if !strings.HasPrefix(rangeHeader, "bytes=") {
		http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	rangeStr := rangeHeader[6:]
	var start, end int64
	n, _ := fmt.Sscanf(rangeStr, "%d-%d", &start, &end)

	if n < 1 || start < 0 || start >= cfg.FileSize {
		http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if n < 2 || end >= cfg.FileSize {
		end = cfg.FileSize - 1
	}
	if start > end {
		http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	contentLength := end - start + 1
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("Content-Range",
		fmt.Sprintf("bytes %d-%d/%d", start, end, cfg.FileSize))
	w.WriteHeader(http.StatusPartialContent)
	s.streamData(w, start, end-start+1)
}

func (s *HTTPMockServer) streamData(w io.Writer, offset, length int64) {
	cfg := s.Config
	chunkSize := cfg.ChunkSize
	remaining := length
	startTime := time.Now()

	// Mid-stream failure point.
	failAt := int64(-1)
	if cfg.FailAtPercent > 0 && cfg.FailAtPercent < 1 {
		failAt = int64(float64(length) * cfg.FailAtPercent)
	}

	for remaining > 0 {
		// Check mid-stream failure.
		if failAt >= 0 && length-remaining >= failAt {
			return // force connection close
		}

		chunk := chunkSize
		if int64(chunk) > remaining {
			chunk = int(remaining)
		}

		dataChunk := make([]byte, chunk)
		for i := 0; i < chunk; i++ {
			dataChunk[i] = byte((offset + length - remaining + int64(i)) % 256)
		}

		if _, err := w.Write(dataChunk); err != nil {
			return
		}

		remaining -= int64(chunk)

		// Throttle.
		if cfg.Speed > 0 {
			sent := length - remaining
			expected := time.Duration(float64(sent)/float64(cfg.Speed)) * time.Second
			elapsed := time.Since(startTime)
			if expected > elapsed {
				time.Sleep(expected - elapsed)
			}
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// ---------------------------------------------------------------------------
// FailingDriver – always returns an error on Open
// ---------------------------------------------------------------------------

// FailingDriver is a protocol driver whose Open method always fails. Useful
// for testing endpoint fallback logic.
type FailingDriver struct {
	Size        int64
	ContentType string
}

// Protocols returns "failing".
func (d *FailingDriver) Protocols() []string { return []string{"failing"} }

// Prepare returns the configured resource info.
func (d *FailingDriver) Prepare(_ context.Context, _ hermes.Endpoint) (hermes.PreparedResource, error) {
	return hermes.PreparedResource{
		Size:        d.Size,
		ContentType: d.ContentType,
	}, nil
}

// Open always returns an error.
func (d *FailingDriver) Open(_ context.Context, _ hermes.Endpoint, _ hermes.ReadRequest) (io.ReadCloser, error) {
	return nil, fmt.Errorf("failing driver: endpoint unavailable")
}

// ---------------------------------------------------------------------------
// FTPMockDriver – mock FTP protocol driver backed by in-memory assets
// ---------------------------------------------------------------------------

// FTPMockDriver implements hermes.ProtocolDriver for the "ftp" protocol. It
// serves files from an AssetBundle by extracting the filename from the
// endpoint URL.
type FTPMockDriver struct {
	Assets *AssetBundle
}

// Protocols returns "ftp".
func (d *FTPMockDriver) Protocols() []string { return []string{"ftp"} }

// Prepare looks up the file in the asset bundle by URL filename.
func (d *FTPMockDriver) Prepare(_ context.Context, endpoint hermes.Endpoint) (hermes.PreparedResource, error) {
	f := d.lookup(endpoint.URL)
	if f == nil {
		return hermes.PreparedResource{}, fmt.Errorf("ftp: file not found in assets")
	}
	return hermes.PreparedResource{
		Size:        f.Size,
		ContentType: f.ContentType,
	}, nil
}

// Open returns a ReadCloser over the asset data.
func (d *FTPMockDriver) Open(_ context.Context, endpoint hermes.Endpoint, req hermes.ReadRequest) (io.ReadCloser, error) {
	f := d.lookup(endpoint.URL)
	if f == nil {
		return nil, fmt.Errorf("ftp: file not found in assets")
	}
	if req.UseRange {
		start := req.OffsetStart
		if start < 0 {
			start = 0
		}
		if start >= int64(len(f.Data)) {
			return io.NopCloser(bytes.NewReader(nil)), nil
		}
		return io.NopCloser(bytes.NewReader(f.Data[start:])), nil
	}
	return io.NopCloser(bytes.NewReader(f.Data)), nil
}

func (d *FTPMockDriver) lookup(url string) *TestFile {
	// Extract filename from URL: "ftp://host/path/filename.bin"
	idx := strings.LastIndex(url, "/")
	if idx < 0 {
		return nil
	}
	filename := url[idx+1:]
	return d.Assets.Files[filename]
}

// ---------------------------------------------------------------------------
// S3MockDriver – mock S3 protocol driver backed by in-memory assets
// ---------------------------------------------------------------------------

// S3MockDriver implements hermes.ProtocolDriver for the "s3" protocol.
type S3MockDriver struct {
	Assets *AssetBundle
}

// Protocols returns "s3".
func (d *S3MockDriver) Protocols() []string { return []string{"s3"} }

// Prepare looks up the asset by extracting the key from the S3 URL.
func (d *S3MockDriver) Prepare(_ context.Context, endpoint hermes.Endpoint) (hermes.PreparedResource, error) {
	f := d.lookup(endpoint.URL)
	if f == nil {
		return hermes.PreparedResource{}, fmt.Errorf("s3: object not found in assets")
	}
	return hermes.PreparedResource{
		Size:          f.Size,
		ContentType:   f.ContentType,
		SupportsRange: true,
	}, nil
}

// Open returns a ReadCloser over the asset data, supporting Range requests.
func (d *S3MockDriver) Open(_ context.Context, endpoint hermes.Endpoint, req hermes.ReadRequest) (io.ReadCloser, error) {
	f := d.lookup(endpoint.URL)
	if f == nil {
		return nil, fmt.Errorf("s3: object not found in assets")
	}
	if req.UseRange {
		start := req.OffsetStart
		end := req.OffsetEnd
		if start < 0 {
			start = 0
		}
		if end >= int64(len(f.Data)) {
			end = int64(len(f.Data)) - 1
		}
		if start > end || start >= int64(len(f.Data)) {
			return io.NopCloser(bytes.NewReader(nil)), nil
		}
		return io.NopCloser(bytes.NewReader(f.Data[start : end+1])), nil
	}
	return io.NopCloser(bytes.NewReader(f.Data)), nil
}

func (d *S3MockDriver) lookup(url string) *TestFile {
	idx := strings.LastIndex(url, "/")
	if idx < 0 {
		return nil
	}
	filename := url[idx+1:]
	return d.Assets.Files[filename]
}

// ---------------------------------------------------------------------------
// HLSMockDriver – mock HLS stream protocol driver
// ---------------------------------------------------------------------------

// HLSMockDriver implements hermes.ProtocolDriver for the "hls" protocol. It
// simulates a live HLS stream by generating a playlist and segment data on
// demand.
type HLSMockDriver struct {
	mu          sync.Mutex
	SegmentSize int64
	NumSegments int
	streamData  []byte // pre-generated stream content
	totalSent   atomic.Int64
}

// Protocols returns "hls".
func (d *HLSMockDriver) Protocols() []string { return []string{"hls"} }

// Prepare reports a stream with unknown total size.
func (d *HLSMockDriver) Prepare(_ context.Context, _ hermes.Endpoint) (hermes.PreparedResource, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.SegmentSize <= 0 {
		d.SegmentSize = 1024 * 1024
	}
	if d.NumSegments <= 0 {
		d.NumSegments = 10
	}
	d.streamData = GenerateData(d.SegmentSize * int64(d.NumSegments))
	return hermes.PreparedResource{
		Size:        d.SegmentSize * int64(d.NumSegments),
		ContentType: "application/vnd.apple.mpegurl",
	}, nil
}

// Open returns a reader delivering HLS stream segments. Each call reads the
// next segment worth of data from the pre-generated stream.
func (d *HLSMockDriver) Open(_ context.Context, _ hermes.Endpoint, _ hermes.ReadRequest) (io.ReadCloser, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	sent := d.totalSent.Load()
	if d.streamData == nil {
		d.SegmentSize = 1024 * 1024
		d.NumSegments = 10
		d.streamData = GenerateData(d.SegmentSize * int64(d.NumSegments))
	}

	remaining := int64(len(d.streamData)) - sent
	if remaining <= 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	chunkSize := d.SegmentSize
	if remaining < chunkSize {
		chunkSize = remaining
	}

	d.totalSent.Add(chunkSize)
	return io.NopCloser(bytes.NewReader(d.streamData[sent : sent+chunkSize])), nil
}

// Reset resets the stream so it can be re-opened from the beginning.
func (d *HLSMockDriver) Reset() {
	d.totalSent.Store(0)
}
