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
	default_stream_rotate_minutes = 10
	default_stream_attempts       = 3
	stream_stop_grace_period      = 8 * time.Second
	stream_finalize_timeout       = 30 * time.Minute
	stream_stderr_limit           = 64 * 1024
)

// StreamDriver records live media with FFmpeg. Unlike finite protocol drivers,
// it writes time-based MKV chunks directly to disk, preserving closed chunks
// across pause/retry, then losslessly concatenates them into one MKV output.
type StreamDriver struct {
	ffmpeg_path  string
	ffprobe_path string
	max_attempts int
}

// NewStreamDriver creates a live-stream driver using FFmpeg binaries from PATH.
func NewStreamDriver() *StreamDriver {
	return &StreamDriver{
		ffmpeg_path:  "ffmpeg",
		ffprobe_path: "ffprobe",
		max_attempts: default_stream_attempts,
	}
}

func (d *StreamDriver) Protocols() []string { return []string{"livestream"} }

// Prepare reports the recorder's final container. Live resources have no
// predetermined size and cannot be resumed with byte-range requests.
func (d *StreamDriver) Prepare(context.Context, hermes.Endpoint) (hermes.PreparedResource, error) {
	if _, err := exec.LookPath(d.ffmpeg_binary()); err != nil {
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
	on_progress func(hermes.StreamRecordProgress) error,
) (hermes.StreamRecordResult, error) {
	if strings.TrimSpace(endpoint.URL) == "" {
		return hermes.StreamRecordResult{}, errors.New("live stream URL is empty")
	}
	if strings.TrimSpace(request.OutputPath) == "" {
		return hermes.StreamRecordResult{}, errors.New("live stream output path is empty")
	}
	if _, err := exec.LookPath(d.ffmpeg_binary()); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("ffmpeg is required for live recording: %w", err)
	}

	recording_dir := request.OutputPath + ".recording"
	if err := os.MkdirAll(recording_dir, 0755); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to create stream recording directory: %w", err)
	}

	rotate_minutes := request.RotateMinutes
	if rotate_minutes <= 0 {
		rotate_minutes = default_stream_rotate_minutes
	}
	started_at := time.Now()
	stop_at := request.StopAt
	if request.Duration > 0 {
		duration_stop := started_at.Add(request.Duration)
		if stop_at.IsZero() || duration_stop.Before(stop_at) {
			stop_at = duration_stop
		}
	}

	existing, err := stream_segment_states(recording_dir, true)
	if err != nil {
		return hermes.StreamRecordResult{}, err
	}
	if !stop_at.IsZero() && !stop_at.After(time.Now()) {
		if len(existing) == 0 {
			return hermes.StreamRecordResult{}, errors.New("live recording end time has already passed")
		}
		return d.finalize_recording(ctx, request.OutputPath, recording_dir, started_at, on_progress)
	}

	attempts := d.max_attempts
	if attempts <= 0 {
		attempts = default_stream_attempts
	}
	var last_err error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := context.Cause(ctx); err != nil {
			return d.finalize_stopped_recording(ctx, request.OutputPath, recording_dir, started_at, on_progress)
		}
		start_index, err := next_stream_segment_index(recording_dir)
		if err != nil {
			return hermes.StreamRecordResult{}, err
		}
		_, err = d.record_attempt(
			ctx, endpoint, recording_dir, start_index, rotate_minutes, stop_at, started_at, on_progress,
		)
		if err == nil {
			return d.finalize_recording(ctx, request.OutputPath, recording_dir, started_at, on_progress)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			return d.finalize_stopped_recording(ctx, request.OutputPath, recording_dir, started_at, on_progress)
		}
		var source_err *stream_source_http_error
		if errors.As(err, &source_err) {
			return hermes.StreamRecordResult{}, err
		}
		last_err = err
		if attempt == attempts-1 {
			break
		}
		delay := time.Duration(1<<attempt) * time.Second
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return d.finalize_stopped_recording(ctx, request.OutputPath, recording_dir, started_at, on_progress)
		case <-timer.C:
		}
	}
	return hermes.StreamRecordResult{}, fmt.Errorf("live recording failed after %d attempts: %w", attempts, last_err)
}

func (d *StreamDriver) finalize_stopped_recording(
	ctx context.Context,
	output_path string,
	recording_dir string,
	started_at time.Time,
	on_progress func(hermes.StreamRecordProgress) error,
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
	finalize_ctx, cancel := context.WithTimeout(context.Background(), stream_finalize_timeout)
	defer cancel()
	return d.finalize_recording(finalize_ctx, output_path, recording_dir, started_at, on_progress)
}

func (d *StreamDriver) record_attempt(
	ctx context.Context,
	endpoint hermes.Endpoint,
	recording_dir string,
	start_index int,
	rotate_minutes int,
	stop_at time.Time,
	started_at time.Time,
	on_progress func(hermes.StreamRecordProgress) error,
) (bool, error) {
	process_ctx, cancel_process := context.WithCancel(context.Background())
	defer cancel_process()

	pattern := filepath.Join(recording_dir, "segment-%06d.mkv")
	args, err := build_stream_ffmpeg_args(endpoint, pattern, start_index, rotate_minutes)
	if err != nil {
		return false, err
	}
	cmd := exec.CommandContext(process_ctx, d.ffmpeg_binary(), args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false, fmt.Errorf("failed to create ffmpeg stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("failed to create ffmpeg progress pipe: %w", err)
	}
	stderr := new_tail_buffer(stream_stderr_limit)
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("failed to start ffmpeg recording: %w", err)
	}

	callback_err := make(chan error, 1)
	progress_done := make(chan struct{})
	go func() {
		defer close(progress_done)
		if err := parse_ffmpeg_progress(stdout, recording_dir, start_index, started_at, on_progress); err != nil {
			select {
			case callback_err <- err:
			default:
			}
		}
	}()

	wait_ch := make(chan error, 1)
	go func() { wait_ch <- cmd.Wait() }()

	var stop_timer *time.Timer
	var stop_ch <-chan time.Time
	if !stop_at.IsZero() {
		stop_timer = time.NewTimer(time.Until(stop_at))
		stop_ch = stop_timer.C
		defer stop_timer.Stop()
	}

	for {
		select {
		case wait_err := <-wait_ch:
			cancel_process()
			<-progress_done
			if err := receive_progress_error(callback_err); err != nil {
				return false, err
			}
			if wait_err != nil {
				return false, ffmpeg_record_error(wait_err, stderr.String(), endpoint.URL)
			}
			if err := emit_stream_progress(recording_dir, started_at, true, false, on_progress); err != nil {
				return false, err
			}
			return false, nil
		case err := <-callback_err:
			wait_err, _ := stop_ffmpeg(stdin, cancel_process, wait_ch)
			<-progress_done
			if err != nil {
				return false, err
			}
			return false, ffmpeg_record_error(wait_err, stderr.String(), endpoint.URL)
		case <-ctx.Done():
			_, forced := stop_ffmpeg(stdin, cancel_process, wait_ch)
			<-progress_done
			_ = emit_stream_progress(recording_dir, started_at, !forced, false, on_progress)
			return false, context.Cause(ctx)
		case <-stop_ch:
			wait_err, forced := stop_ffmpeg(stdin, cancel_process, wait_ch)
			<-progress_done
			if err := receive_progress_error(callback_err); err != nil {
				return true, err
			}
			if err := emit_stream_progress(recording_dir, started_at, !forced, false, on_progress); err != nil {
				return true, err
			}
			if wait_err != nil && !forced {
				return true, ffmpeg_record_error(wait_err, stderr.String(), endpoint.URL)
			}
			return true, nil
		}
	}
}

func receive_progress_error(callback_err <-chan error) error {
	select {
	case err := <-callback_err:
		return err
	default:
		return nil
	}
}

func stop_ffmpeg(stdin io.WriteCloser, cancel context.CancelFunc, wait_ch <-chan error) (error, bool) {
	_, _ = io.WriteString(stdin, "q\n")
	_ = stdin.Close()
	timer := time.NewTimer(stream_stop_grace_period)
	defer timer.Stop()
	select {
	case err := <-wait_ch:
		return err, false
	case <-timer.C:
		cancel()
		return <-wait_ch, true
	}
}

func build_stream_ffmpeg_args(endpoint hermes.Endpoint, output_pattern string, start_index, rotate_minutes int) ([]string, error) {
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
		proxy_url, err := endpoint.ProxyServer.URL()
		if err != nil {
			return nil, err
		}
		if proxy_url != "" {
			args = append(args, "-http_proxy", proxy_url)
		}
		if headers := ffmpeg_headers(endpoint); headers != "" {
			args = append(args, "-headers", headers)
		}
	case "rtsp", "rtsps":
		args = append(args, "-rtsp_transport", "tcp")
	}

	segment_seconds := rotate_minutes * 60
	if segment_seconds <= 0 {
		segment_seconds = default_stream_rotate_minutes * 60
	}
	args = append(args,
		"-i", endpoint.URL,
		"-map", "0:v:0?",
		"-map", "0:a:0?",
		"-c", "copy",
		"-avoid_negative_ts", "make_non_negative",
		"-f", "segment",
		"-segment_format", "matroska",
		"-segment_time", strconv.Itoa(segment_seconds),
		"-segment_start_number", strconv.Itoa(start_index),
		"-reset_timestamps", "1",
		"-y",
		output_pattern,
	)
	return args, nil
}

func ffmpeg_headers(endpoint hermes.Endpoint) string {
	headers := make(map[string]string, len(endpoint.Headers)+1)
	for key, value := range endpoint.Headers {
		key = strings.TrimSpace(key)
		if valid_http_header_name(key) && !strings.ContainsAny(value, "\r\n") {
			headers[key] = strings.TrimSpace(value)
		}
	}
	cookies := strings.TrimSpace(endpoint.Cookies)
	if cookies != "" && !strings.ContainsAny(cookies, "\r\n") {
		has_cookie := false
		for key := range headers {
			if strings.EqualFold(key, "Cookie") {
				has_cookie = true
				break
			}
		}
		if !has_cookie {
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

func valid_http_header_name(name string) bool {
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

type stream_progress_tracker struct {
	recording_dir string
	states        []hermes.StreamSegmentState
	next_index    int
	total         int64
	stat_file     func(string) (os.FileInfo, error)
}

func new_stream_progress_tracker(recording_dir string, active_start_index int) (*stream_progress_tracker, error) {
	states, err := stream_segment_states(recording_dir, true)
	if err != nil {
		return nil, err
	}
	tracker := &stream_progress_tracker{
		recording_dir: recording_dir,
		states:        states,
		stat_file:     os.Stat,
	}
	for _, state := range states {
		tracker.total += state.Downloaded
		if state.Index >= tracker.next_index {
			tracker.next_index = state.Index + 1
		}
	}
	if len(tracker.states) > 0 && tracker.states[len(tracker.states)-1].Index >= active_start_index {
		tracker.states[len(tracker.states)-1].Complete = false
	}
	return tracker, nil
}

func (tracker *stream_progress_tracker) refresh(all_complete bool) ([]hermes.StreamSegmentState, int64, error) {
	if len(tracker.states) > 0 && !tracker.states[len(tracker.states)-1].Complete {
		last_state := &tracker.states[len(tracker.states)-1]
		info, err := tracker.stat_file(stream_segment_path(tracker.recording_dir, last_state.Index))
		if err != nil {
			return nil, 0, err
		}
		tracker.total += info.Size() - last_state.Downloaded
		last_state.Size = info.Size()
		last_state.Downloaded = info.Size()
	}

	for {
		info, err := tracker.stat_file(stream_segment_path(tracker.recording_dir, tracker.next_index))
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return nil, 0, err
		}
		if !info.Mode().IsRegular() {
			return nil, 0, fmt.Errorf("stream segment %d is not a regular file", tracker.next_index)
		}
		if len(tracker.states) > 0 {
			tracker.states[len(tracker.states)-1].Complete = true
		}
		tracker.states = append(tracker.states, hermes.StreamSegmentState{
			Index:      tracker.next_index,
			Size:       info.Size(),
			Downloaded: info.Size(),
		})
		tracker.total += info.Size()
		tracker.next_index++
	}
	if all_complete && len(tracker.states) > 0 {
		tracker.states[len(tracker.states)-1].Complete = true
	}
	states := append([]hermes.StreamSegmentState(nil), tracker.states...)
	return states, tracker.total, nil
}

func (tracker *stream_progress_tracker) emit_progress(
	started_at time.Time,
	all_complete bool,
	finalizing bool,
	on_progress func(hermes.StreamRecordProgress) error,
) error {
	states, total, err := tracker.refresh(all_complete)
	if err != nil {
		return err
	}
	elapsed := time.Since(started_at)
	var speed int64
	if elapsed > 0 {
		speed = int64(float64(total) / elapsed.Seconds())
	}
	return on_progress(hermes.StreamRecordProgress{
		Downloaded: total,
		Speed:      speed,
		Duration:   elapsed,
		Segments:   states,
		Finalizing: finalizing,
	})
}

func stream_segment_path(recording_dir string, index int) string {
	return filepath.Join(recording_dir, fmt.Sprintf("segment-%06d.mkv", index))
}

func parse_ffmpeg_progress(
	reader io.Reader,
	recording_dir string,
	active_start_index int,
	started_at time.Time,
	on_progress func(hermes.StreamRecordProgress) error,
) error {
	var tracker *stream_progress_tracker
	if on_progress != nil {
		var err error
		tracker, err = new_stream_progress_tracker(recording_dir, active_start_index)
		if err != nil {
			return err
		}
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, "=")
		if !ok || key != "progress" || (value != "continue" && value != "end") {
			continue
		}
		if tracker == nil {
			continue
		}
		if err := tracker.emit_progress(started_at, value == "end", false, on_progress); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func emit_stream_progress(recording_dir string, started_at time.Time, all_complete bool, finalizing bool, on_progress func(hermes.StreamRecordProgress) error) error {
	if on_progress == nil {
		return nil
	}
	states, err := stream_segment_states(recording_dir, all_complete)
	if err != nil {
		return err
	}
	var total int64
	for _, state := range states {
		total += state.Downloaded
	}
	elapsed := time.Since(started_at)
	var speed int64
	if elapsed > 0 {
		speed = int64(float64(total) / elapsed.Seconds())
	}
	return on_progress(hermes.StreamRecordProgress{
		Downloaded: total,
		Speed:      speed,
		Duration:   elapsed,
		Segments:   states,
		Finalizing: finalizing,
	})
}

func stream_segment_states(recording_dir string, all_complete bool) ([]hermes.StreamSegmentState, error) {
	entries, err := os.ReadDir(recording_dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read stream recording directory: %w", err)
	}
	type indexed_file struct {
		index int
		info  os.FileInfo
	}
	files := make([]indexed_file, 0)
	for _, entry := range entries {
		index, ok := stream_segment_index(entry.Name())
		if !ok || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, indexed_file{index: index, info: info})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].index < files[j].index })
	states := make([]hermes.StreamSegmentState, 0, len(files))
	for i, file := range files {
		complete := all_complete || i < len(files)-1
		states = append(states, hermes.StreamSegmentState{
			Index:      file.index,
			Size:       file.info.Size(),
			Downloaded: file.info.Size(),
			Complete:   complete,
		})
	}
	return states, nil
}

func next_stream_segment_index(recording_dir string) (int, error) {
	states, err := stream_segment_states(recording_dir, false)
	if err != nil {
		return 0, err
	}
	if len(states) == 0 {
		return 0, nil
	}
	return states[len(states)-1].Index + 1, nil
}

func stream_segment_index(name string) (int, bool) {
	if !strings.HasPrefix(name, "segment-") || !strings.HasSuffix(name, ".mkv") {
		return 0, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(name, "segment-"), ".mkv")
	index, err := strconv.Atoi(raw)
	return index, err == nil && index >= 0
}

func (d *StreamDriver) finalize_recording(
	ctx context.Context,
	output_path string,
	recording_dir string,
	started_at time.Time,
	on_progress func(hermes.StreamRecordProgress) error,
) (hermes.StreamRecordResult, error) {
	if err := emit_stream_progress(recording_dir, started_at, true, true, on_progress); err != nil {
		return hermes.StreamRecordResult{}, err
	}
	segments, err := d.valid_stream_segments(ctx, recording_dir)
	if err != nil {
		return hermes.StreamRecordResult{}, err
	}
	if len(segments) == 0 {
		return hermes.StreamRecordResult{}, errors.New("ffmpeg did not produce any playable stream segments")
	}

	manifest_path := filepath.Join(recording_dir, "segments.ffconcat")
	var manifest strings.Builder
	manifest.WriteString("ffconcat version 1.0\n")
	for _, segment := range segments {
		manifest.WriteString("file '")
		manifest.WriteString(filepath.Base(segment))
		manifest.WriteString("'\n")
	}
	if err := os.WriteFile(manifest_path, []byte(manifest.String()), 0644); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to write stream concat manifest: %w", err)
	}

	tmp_output := output_path + ".part"
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "concat", "-safe", "1",
		"-i", filepath.Base(manifest_path),
		"-map", "0", "-c", "copy",
		"-f", "matroska", "-y", tmp_output,
	}
	cmd := exec.CommandContext(ctx, d.ffmpeg_binary(), args...)
	cmd.Dir = recording_dir
	stderr := new_tail_buffer(stream_stderr_limit)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if context.Cause(ctx) != nil {
			return hermes.StreamRecordResult{}, context.Cause(ctx)
		}
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to concatenate stream segments: %w: %s", err, stderr.String())
	}
	if err := os.Rename(tmp_output, output_path); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to commit stream recording: %w", err)
	}
	info, err := os.Stat(output_path)
	if err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to stat stream recording: %w", err)
	}
	if err := emit_stream_progress(recording_dir, started_at, true, true, on_progress); err != nil {
		return hermes.StreamRecordResult{}, err
	}
	if err := os.RemoveAll(recording_dir); err != nil {
		return hermes.StreamRecordResult{}, fmt.Errorf("failed to clean stream recording chunks: %w", err)
	}
	return hermes.StreamRecordResult{
		FilePath: output_path,
		Size:     info.Size(),
		Duration: time.Since(started_at),
	}, nil
}

func (d *StreamDriver) valid_stream_segments(ctx context.Context, recording_dir string) ([]string, error) {
	states, err := stream_segment_states(recording_dir, true)
	if err != nil {
		return nil, err
	}
	ffprobe, probe_err := exec.LookPath(d.ffprobe_binary())
	segments := make([]string, 0, len(states))
	for _, state := range states {
		if state.Size <= 0 {
			continue
		}
		path := filepath.Join(recording_dir, fmt.Sprintf("segment-%06d.mkv", state.Index))
		if probe_err == nil {
			cmd := exec.CommandContext(ctx, ffprobe,
				"-v", "error", "-show_entries", "stream=index", "-of", "csv=p=0", path,
			)
			if err := cmd.Run(); err != nil {
				if context.Cause(ctx) != nil {
					return nil, context.Cause(ctx)
				}
				corrupt_path := path + ".corrupt"
				if rename_err := os.Rename(path, corrupt_path); rename_err != nil {
					return nil, fmt.Errorf("invalid stream segment %s could not be quarantined: %w", filepath.Base(path), rename_err)
				}
				continue
			}
		}
		segments = append(segments, path)
	}
	return segments, nil
}

func (d *StreamDriver) ffmpeg_binary() string {
	if strings.TrimSpace(d.ffmpeg_path) == "" {
		return "ffmpeg"
	}
	return d.ffmpeg_path
}

func (d *StreamDriver) ffprobe_binary() string {
	if strings.TrimSpace(d.ffprobe_path) == "" {
		return "ffprobe"
	}
	return d.ffprobe_path
}

type stream_source_http_error struct {
	status_code int
	cause       error
	diagnostics string
}

func (e *stream_source_http_error) Error() string {
	reason := http.StatusText(e.status_code)
	explanation := "the live-stream request is no longer authorized"
	switch e.status_code {
	case http.StatusUnauthorized:
		explanation = "the live-stream credentials or URL signature are invalid or expired"
	case http.StatusForbidden:
		explanation = "the signed live-stream URL may have expired, or the request is no longer authorized"
	case http.StatusGone:
		explanation = "the signed live-stream URL has expired, or the live stream is no longer available"
	}
	message := fmt.Sprintf("live stream source returned HTTP %d %s: %s", e.status_code, reason, explanation)
	if e.diagnostics != "" {
		message += "; ffmpeg diagnostics: " + e.diagnostics
	}
	return message
}

func (e *stream_source_http_error) Unwrap() error { return e.cause }

func ffmpeg_record_error(err error, stderr, endpoint_url string) error {
	stderr = sanitize_ffmpeg_diagnostics(stderr, endpoint_url)
	if status_code, ok := fatal_stream_http_status(stderr); ok {
		return &stream_source_http_error{
			status_code: status_code,
			cause:       err,
			diagnostics: stderr,
		}
	}
	if stderr == "" {
		return fmt.Errorf("ffmpeg recording failed: %w", err)
	}
	return fmt.Errorf("ffmpeg recording failed: %w: %s", err, stderr)
}

func fatal_stream_http_status(stderr string) (int, bool) {
	normalized := strings.ToLower(stderr)
	for _, status_code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusGone} {
		code := strconv.Itoa(status_code)
		reason := strings.ToLower(http.StatusText(status_code))
		for _, marker := range []string{
			"http error " + code,
			"server returned " + code,
			code + " " + reason,
		} {
			if strings.Contains(normalized, marker) {
				return status_code, true
			}
		}
	}
	return 0, false
}

func sanitize_ffmpeg_diagnostics(stderr, endpoint_url string) string {
	stderr = strings.TrimSpace(stderr)
	if endpoint_url != "" {
		stderr = strings.ReplaceAll(stderr, endpoint_url, "<redacted-live-url>")
	}
	return stderr
}

type tail_buffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func new_tail_buffer(limit int) *tail_buffer { return &tail_buffer{limit: limit} }

func (b *tail_buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := len(p)
	b.data = append(b.data, p...)
	if b.limit > 0 && len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return written, nil
}

func (b *tail_buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(bytes.Clone(b.data))
}
