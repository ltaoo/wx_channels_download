package protocol

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"wx_channel/pkg/hermes"
)

const (
	defaultStreamRotateMinutes = 10
	defaultStreamAttempts      = 3
	streamStopGracePeriod      = 8 * time.Second
	streamFinalizeTimeout      = 30 * time.Minute
	streamStderrLimit          = 64 * 1024
)

// StreamDriver records live media with FFmpeg. Unlike finite protocol drivers,
// it writes time-based MKV chunks directly to disk, preserving closed chunks
// across pause/retry, then losslessly concatenates them into one MKV output.
type StreamDriver struct {
	ffmpegPath  string
	ffprobePath string
	maxAttempts int
}

// NewStreamDriver creates a live-stream driver using FFmpeg binaries from PATH.
func NewStreamDriver() *StreamDriver {
	return &StreamDriver{
		ffmpegPath:  "ffmpeg",
		ffprobePath: "ffprobe",
		maxAttempts: defaultStreamAttempts,
	}
}

func (d *StreamDriver) Protocols() []string { return []string{"livestream"} }

// Prepare reports the recorder's final container. Live resources have no
// predetermined size and cannot be resumed with byte-range requests.
func (d *StreamDriver) Prepare(context.Context, hermes.Endpoint) (hermes.PreparedResource, error) {
	if _, err := exec.LookPath(d.ffmpegBinary()); err != nil {
		return hermes.PreparedResource{}, fmt.Errorf("ffmpeg is required for live recording: %w", err)
	}
	return hermes.PreparedResource{
		ContentType:   "video/x-matroska",
		SupportsRange: false,
	}, nil
}

// Open is retained for ProtocolDriver compatibility. STREAM resources are
// routed through RecordStream and should never use this finite-reader method.
func (d *StreamDriver) Open(context.Context, hermes.Endpoint, hermes.ReadRequest) (io.ReadCloser, error) {
	return nil, errors.New("live streams must be recorded through RecordStream")
}

func (d *StreamDriver) RecordStream(
	ctx context.Context,
	endpoint hermes.Endpoint,
	request hermes.StreamRecordRequest,
	onProgress func(hermes.StreamRecordProgress) error,
) (hermes.StreamRecordResult, error) {
	if strings.TrimSpace(endpoint.URL) == "" {
		return hermes.StreamRecordResult{}, errors.New("live stream URL is empty")
	}
	if strings.TrimSpace(request.OutputPath) == "" {
		return hermes.StreamRecordResult{}, errors.New("live stream output path is empty")
	}
	if _, err := exec.LookPath(d.ffmpegBinary()); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("ffmpeg is required for live recording: %w", err)
	}

	recordingDir := request.OutputPath + ".recording"
	if err := os.MkdirAll(recordingDir, 0755); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to create stream recording directory: %w", err)
	}

	rotateMinutes := request.RotateMinutes
	if rotateMinutes <= 0 {
		rotateMinutes = defaultStreamRotateMinutes
	}
	startedAt := time.Now()
	stopAt := request.StopAt
	if request.Duration > 0 {
		durationStop := startedAt.Add(request.Duration)
		if stopAt.IsZero() || durationStop.Before(stopAt) {
			stopAt = durationStop
		}
	}

	existing, err := streamSegmentStates(recordingDir, true)
	if err != nil {
		return hermes.StreamRecordResult{}, err
	}
	if !stopAt.IsZero() && !stopAt.After(time.Now()) {
		if len(existing) == 0 {
			return hermes.StreamRecordResult{}, errors.New("live recording end time has already passed")
		}
		return d.finalizeRecording(ctx, request.OutputPath, recordingDir, startedAt, onProgress)
	}

	attempts := d.maxAttempts
	if attempts <= 0 {
		attempts = defaultStreamAttempts
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := context.Cause(ctx); err != nil {
			return d.finalizeStoppedRecording(ctx, request.OutputPath, recordingDir, startedAt, onProgress)
		}
		startIndex, err := nextStreamSegmentIndex(recordingDir)
		if err != nil {
			return hermes.StreamRecordResult{}, err
		}
		_, err = d.recordAttempt(
			ctx, endpoint, recordingDir, startIndex, rotateMinutes, stopAt, startedAt, onProgress,
		)
		if err == nil {
			return d.finalizeRecording(ctx, request.OutputPath, recordingDir, startedAt, onProgress)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			return d.finalizeStoppedRecording(ctx, request.OutputPath, recordingDir, startedAt, onProgress)
		}
		var sourceErr *streamSourceHTTPError
		if errors.As(err, &sourceErr) {
			return hermes.StreamRecordResult{}, err
		}
		lastErr = err
		if attempt == attempts-1 {
			break
		}
		delay := time.Duration(1<<attempt) * time.Second
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return d.finalizeStoppedRecording(ctx, request.OutputPath, recordingDir, startedAt, onProgress)
		case <-timer.C:
		}
	}
	return hermes.StreamRecordResult{}, fmt.Errorf("live recording failed after %d attempts: %w", attempts, lastErr)
}

func (d *StreamDriver) finalizeStoppedRecording(
	ctx context.Context,
	outputPath string,
	recordingDir string,
	startedAt time.Time,
	onProgress func(hermes.StreamRecordProgress) error,
) (hermes.StreamRecordResult, error) {
	cause := context.Cause(ctx)
	if !errors.Is(cause, hermes.ErrTaskStopRequested) {
		if cause == nil {
			cause = context.Canceled
		}
		return hermes.StreamRecordResult{}, cause
	}

	// The recording context is intentionally cancelled to stop FFmpeg. Merge
	// the durable chunks with a fresh bounded context so manual stop can still
	// produce the final media file.
	finalizeCtx, cancel := context.WithTimeout(context.Background(), streamFinalizeTimeout)
	defer cancel()
	return d.finalizeRecording(finalizeCtx, outputPath, recordingDir, startedAt, onProgress)
}

func (d *StreamDriver) recordAttempt(
	ctx context.Context,
	endpoint hermes.Endpoint,
	recordingDir string,
	startIndex int,
	rotateMinutes int,
	stopAt time.Time,
	startedAt time.Time,
	onProgress func(hermes.StreamRecordProgress) error,
) (bool, error) {
	processCtx, cancelProcess := context.WithCancel(context.Background())
	defer cancelProcess()

	pattern := filepath.Join(recordingDir, "segment-%06d.mkv")
	args := buildStreamFFmpegArgs(endpoint, pattern, startIndex, rotateMinutes)
	cmd := exec.CommandContext(processCtx, d.ffmpegBinary(), args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false, fmt.Errorf("failed to create ffmpeg stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("failed to create ffmpeg progress pipe: %w", err)
	}
	stderr := newTailBuffer(streamStderrLimit)
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("failed to start ffmpeg recording: %w", err)
	}

	callbackErr := make(chan error, 1)
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		if err := parseFFmpegProgress(stdout, recordingDir, startedAt, onProgress); err != nil {
			select {
			case callbackErr <- err:
			default:
			}
		}
	}()

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	var stopTimer *time.Timer
	var stopCh <-chan time.Time
	if !stopAt.IsZero() {
		stopTimer = time.NewTimer(time.Until(stopAt))
		stopCh = stopTimer.C
		defer stopTimer.Stop()
	}

	for {
		select {
		case waitErr := <-waitCh:
			cancelProcess()
			<-progressDone
			if err := receiveProgressError(callbackErr); err != nil {
				return false, err
			}
			if waitErr != nil {
				return false, ffmpegRecordError(waitErr, stderr.String(), endpoint.URL)
			}
			if err := emitStreamProgress(recordingDir, startedAt, true, false, onProgress); err != nil {
				return false, err
			}
			return false, nil
		case err := <-callbackErr:
			waitErr, _ := stopFFmpeg(stdin, cancelProcess, waitCh)
			<-progressDone
			if err != nil {
				return false, err
			}
			return false, ffmpegRecordError(waitErr, stderr.String(), endpoint.URL)
		case <-ctx.Done():
			_, forced := stopFFmpeg(stdin, cancelProcess, waitCh)
			<-progressDone
			_ = emitStreamProgress(recordingDir, startedAt, !forced, false, onProgress)
			return false, context.Cause(ctx)
		case <-stopCh:
			waitErr, forced := stopFFmpeg(stdin, cancelProcess, waitCh)
			<-progressDone
			if err := receiveProgressError(callbackErr); err != nil {
				return true, err
			}
			if err := emitStreamProgress(recordingDir, startedAt, !forced, false, onProgress); err != nil {
				return true, err
			}
			if waitErr != nil && !forced {
				return true, ffmpegRecordError(waitErr, stderr.String(), endpoint.URL)
			}
			return true, nil
		}
	}
}

func receiveProgressError(callbackErr <-chan error) error {
	select {
	case err := <-callbackErr:
		return err
	default:
		return nil
	}
}

func stopFFmpeg(stdin io.WriteCloser, cancel context.CancelFunc, waitCh <-chan error) (error, bool) {
	_, _ = io.WriteString(stdin, "q\n")
	_ = stdin.Close()
	timer := time.NewTimer(streamStopGracePeriod)
	defer timer.Stop()
	select {
	case err := <-waitCh:
		return err, false
	case <-timer.C:
		cancel()
		return <-waitCh, true
	}
}

func buildStreamFFmpegArgs(endpoint hermes.Endpoint, outputPattern string, startIndex, rotateMinutes int) []string {
	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-nostats",
		"-stats_period", "1",
		"-progress", "pipe:1",
	}
	parsed, _ := url.Parse(endpoint.URL)
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		args = append(args,
			"-rw_timeout", "15000000",
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_on_network_error", "1",
			"-reconnect_on_http_error", "429,500,502,503,504",
			"-reconnect_delay_max", "30",
		)
		if headers := ffmpegHeaders(endpoint); headers != "" {
			args = append(args, "-headers", headers)
		}
	case "rtsp", "rtsps":
		args = append(args, "-rtsp_transport", "tcp")
	}

	segmentSeconds := rotateMinutes * 60
	if segmentSeconds <= 0 {
		segmentSeconds = defaultStreamRotateMinutes * 60
	}
	args = append(args,
		"-i", endpoint.URL,
		"-map", "0:v:0?",
		"-map", "0:a:0?",
		"-c", "copy",
		"-avoid_negative_ts", "make_non_negative",
		"-f", "segment",
		"-segment_format", "matroska",
		"-segment_time", strconv.Itoa(segmentSeconds),
		"-segment_start_number", strconv.Itoa(startIndex),
		"-reset_timestamps", "1",
		"-y",
		outputPattern,
	)
	return args
}

func ffmpegHeaders(endpoint hermes.Endpoint) string {
	headers := make(map[string]string, len(endpoint.Headers)+1)
	for key, value := range endpoint.Headers {
		key = strings.TrimSpace(key)
		if validHTTPHeaderName(key) && !strings.ContainsAny(value, "\r\n") {
			headers[key] = strings.TrimSpace(value)
		}
	}
	cookies := strings.TrimSpace(endpoint.Cookies)
	if cookies != "" && !strings.ContainsAny(cookies, "\r\n") {
		hasCookie := false
		for key := range headers {
			if strings.EqualFold(key, "Cookie") {
				hasCookie = true
				break
			}
		}
		if !hasCookie {
			headers["Cookie"] = cookies
		}
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return strings.ToLower(keys[i]) < strings.ToLower(keys[j]) })
	var result strings.Builder
	for _, key := range keys {
		if headers[key] == "" {
			continue
		}
		result.WriteString(key)
		result.WriteString(": ")
		result.WriteString(headers[key])
		result.WriteString("\r\n")
	}
	return result.String()
}

func validHTTPHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' {
			continue
		}
		switch char {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}

func parseFFmpegProgress(
	reader io.Reader,
	recordingDir string,
	startedAt time.Time,
	onProgress func(hermes.StreamRecordProgress) error,
) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, "=")
		if !ok || key != "progress" || (value != "continue" && value != "end") {
			continue
		}
		if err := emitStreamProgress(recordingDir, startedAt, value == "end", false, onProgress); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func emitStreamProgress(recordingDir string, startedAt time.Time, allComplete bool, finalizing bool, onProgress func(hermes.StreamRecordProgress) error) error {
	if onProgress == nil {
		return nil
	}
	states, err := streamSegmentStates(recordingDir, allComplete)
	if err != nil {
		return err
	}
	var total int64
	for _, state := range states {
		total += state.Downloaded
	}
	elapsed := time.Since(startedAt)
	var speed int64
	if elapsed > 0 {
		speed = int64(float64(total) / elapsed.Seconds())
	}
	return onProgress(hermes.StreamRecordProgress{
		Downloaded: total,
		Speed:      speed,
		Duration:   elapsed,
		Segments:   states,
		Finalizing: finalizing,
	})
}

func streamSegmentStates(recordingDir string, allComplete bool) ([]hermes.StreamSegmentState, error) {
	entries, err := os.ReadDir(recordingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read stream recording directory: %w", err)
	}
	type indexedFile struct {
		index int
		info  os.FileInfo
	}
	files := make([]indexedFile, 0)
	for _, entry := range entries {
		index, ok := streamSegmentIndex(entry.Name())
		if !ok || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, indexedFile{index: index, info: info})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].index < files[j].index })
	states := make([]hermes.StreamSegmentState, 0, len(files))
	for i, file := range files {
		complete := allComplete || i < len(files)-1
		states = append(states, hermes.StreamSegmentState{
			Index:      file.index,
			Size:       file.info.Size(),
			Downloaded: file.info.Size(),
			Complete:   complete,
		})
	}
	return states, nil
}

func nextStreamSegmentIndex(recordingDir string) (int, error) {
	states, err := streamSegmentStates(recordingDir, false)
	if err != nil {
		return 0, err
	}
	if len(states) == 0 {
		return 0, nil
	}
	return states[len(states)-1].Index + 1, nil
}

func streamSegmentIndex(name string) (int, bool) {
	if !strings.HasPrefix(name, "segment-") || !strings.HasSuffix(name, ".mkv") {
		return 0, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(name, "segment-"), ".mkv")
	index, err := strconv.Atoi(raw)
	return index, err == nil && index >= 0
}

func (d *StreamDriver) finalizeRecording(
	ctx context.Context,
	outputPath string,
	recordingDir string,
	startedAt time.Time,
	onProgress func(hermes.StreamRecordProgress) error,
) (hermes.StreamRecordResult, error) {
	if err := emitStreamProgress(recordingDir, startedAt, true, true, onProgress); err != nil {
		return hermes.StreamRecordResult{}, err
	}
	segments, err := d.validStreamSegments(ctx, recordingDir)
	if err != nil {
		return hermes.StreamRecordResult{}, err
	}
	if len(segments) == 0 {
		return hermes.StreamRecordResult{}, errors.New("ffmpeg did not produce any playable stream segments")
	}

	manifestPath := filepath.Join(recordingDir, "segments.ffconcat")
	var manifest strings.Builder
	manifest.WriteString("ffconcat version 1.0\n")
	for _, segment := range segments {
		manifest.WriteString("file '")
		manifest.WriteString(filepath.Base(segment))
		manifest.WriteString("'\n")
	}
	if err := os.WriteFile(manifestPath, []byte(manifest.String()), 0644); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to write stream concat manifest: %w", err)
	}

	tmpOutput := outputPath + ".part"
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "concat", "-safe", "1",
		"-i", filepath.Base(manifestPath),
		"-map", "0", "-c", "copy",
		"-f", "matroska", "-y", tmpOutput,
	}
	cmd := exec.CommandContext(ctx, d.ffmpegBinary(), args...)
	cmd.Dir = recordingDir
	stderr := newTailBuffer(streamStderrLimit)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if context.Cause(ctx) != nil {
			return hermes.StreamRecordResult{}, context.Cause(ctx)
		}
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to concatenate stream segments: %w: %s", err, stderr.String())
	}
	if err := os.Rename(tmpOutput, outputPath); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to commit stream recording: %w", err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to stat stream recording: %w", err)
	}
	if err := emitStreamProgress(recordingDir, startedAt, true, true, onProgress); err != nil {
		return hermes.StreamRecordResult{}, err
	}
	if err := os.RemoveAll(recordingDir); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to clean stream recording chunks: %w", err)
	}
	return hermes.StreamRecordResult{
		FilePath: outputPath,
		Size:     info.Size(),
		Duration: time.Since(startedAt),
	}, nil
}

func (d *StreamDriver) validStreamSegments(ctx context.Context, recordingDir string) ([]string, error) {
	states, err := streamSegmentStates(recordingDir, true)
	if err != nil {
		return nil, err
	}
	ffprobe, probeErr := exec.LookPath(d.ffprobeBinary())
	segments := make([]string, 0, len(states))
	for _, state := range states {
		if state.Size <= 0 {
			continue
		}
		path := filepath.Join(recordingDir, fmt.Sprintf("segment-%06d.mkv", state.Index))
		if probeErr == nil {
			cmd := exec.CommandContext(ctx, ffprobe,
				"-v", "error", "-show_entries", "stream=index", "-of", "csv=p=0", path,
			)
			if err := cmd.Run(); err != nil {
				if context.Cause(ctx) != nil {
					return nil, context.Cause(ctx)
				}
				corruptPath := path + ".corrupt"
				if renameErr := os.Rename(path, corruptPath); renameErr != nil {
					return nil, fmt.Errorf("invalid stream segment %s could not be quarantined: %w", filepath.Base(path), renameErr)
				}
				continue
			}
		}
		segments = append(segments, path)
	}
	return segments, nil
}

func (d *StreamDriver) ffmpegBinary() string {
	if strings.TrimSpace(d.ffmpegPath) == "" {
		return "ffmpeg"
	}
	return d.ffmpegPath
}

func (d *StreamDriver) ffprobeBinary() string {
	if strings.TrimSpace(d.ffprobePath) == "" {
		return "ffprobe"
	}
	return d.ffprobePath
}

type streamSourceHTTPError struct {
	statusCode  int
	cause       error
	diagnostics string
}

func (e *streamSourceHTTPError) Error() string {
	reason := http.StatusText(e.statusCode)
	explanation := "the live-stream request is no longer authorized"
	switch e.statusCode {
	case http.StatusUnauthorized:
		explanation = "the live-stream credentials or URL signature are invalid or expired"
	case http.StatusForbidden:
		explanation = "the signed live-stream URL may have expired, or the request is no longer authorized"
	case http.StatusGone:
		explanation = "the signed live-stream URL has expired, or the live stream is no longer available"
	}
	message := fmt.Sprintf("live stream source returned HTTP %d %s: %s", e.statusCode, reason, explanation)
	if e.diagnostics != "" {
		message += "; ffmpeg diagnostics: " + e.diagnostics
	}
	return message
}

func (e *streamSourceHTTPError) Unwrap() error { return e.cause }

func ffmpegRecordError(err error, stderr, endpointURL string) error {
	stderr = sanitizeFFmpegDiagnostics(stderr, endpointURL)
	if statusCode, ok := fatalStreamHTTPStatus(stderr); ok {
		return &streamSourceHTTPError{
			statusCode:  statusCode,
			cause:       err,
			diagnostics: stderr,
		}
	}
	if stderr == "" {
		return fmt.Errorf("ffmpeg recording failed: %w", err)
	}
	return fmt.Errorf("ffmpeg recording failed: %w: %s", err, stderr)
}

func fatalStreamHTTPStatus(stderr string) (int, bool) {
	normalized := strings.ToLower(stderr)
	for _, statusCode := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusGone} {
		code := strconv.Itoa(statusCode)
		reason := strings.ToLower(http.StatusText(statusCode))
		for _, marker := range []string{
			"http error " + code,
			"server returned " + code,
			code + " " + reason,
		} {
			if strings.Contains(normalized, marker) {
				return statusCode, true
			}
		}
	}
	return 0, false
}

func sanitizeFFmpegDiagnostics(stderr, endpointURL string) string {
	stderr = strings.TrimSpace(stderr)
	if endpointURL != "" {
		stderr = strings.ReplaceAll(stderr, endpointURL, "<redacted-live-url>")
	}
	return stderr
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newTailBuffer(limit int) *tailBuffer { return &tailBuffer{limit: limit} }

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := len(p)
	b.data = append(b.data, p...)
	if b.limit > 0 && len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return written, nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(bytes.Clone(b.data))
}
