package protocol

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"wx_channel/pkg/hermes"
)

// StreamDriver uses ffmpeg to download live streams and remux to MKV in one pass.
// Matroska (MKV) muxer works with pipe output (no seeking needed) and supports
// HEVC video natively.  Note: MP4 muxer cannot write to a pipe because it requires
// seeking to write the moov atom header.
//
// The postprocess pipeline can later convert MKV→MP4 with -c copy (writing to a
// file is fine).
type StreamDriver struct{}

// NewStreamDriver creates a new StreamDriver instance.
func NewStreamDriver() *StreamDriver {
	return &StreamDriver{}
}

// Protocols returns the protocol identifiers supported by this driver.
func (d *StreamDriver) Protocols() []string {
	return []string{"livestream"}
}

// Prepare returns a zero-size, no-range resource. Live streams have an unknown
// total size and do not support byte-range requests.
func (d *StreamDriver) Prepare(ctx context.Context, endpoint hermes.Endpoint) (hermes.PreparedResource, error) {
	return hermes.PreparedResource{
		Size:          0,
		SupportsRange: false,
		ContentType:   "video/x-matroska",
	}, nil
}

// Open launches ffmpeg to download the live stream and returns a reader
// connected to ffmpeg's stdout pipe. Remuxes to MKV without re-encoding:
//
//	ffmpeg -v error -i <URL> -c copy -f matroska pipe:1
//
// The returned ReadCloser pipes ffmpeg's stdout. When closed, ffmpeg
// is terminated.
func (d *StreamDriver) Open(ctx context.Context, endpoint hermes.Endpoint, request hermes.ReadRequest) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-v", "error",
		"-i", endpoint.URL,
		"-c", "copy",
		"-f", "matroska",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 ffmpeg stdout 管道失败: %w", err)
	}

	// Capture stderr for error diagnostics
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 ffmpeg 下载失败: %w", err)
	}

	return &streamReadCloser{reader: stdout, cmd: cmd, stderrBuf: &stderrBuf}, nil
}

// streamReadCloser wraps ffmpeg's stdout reader and the ffmpeg process.
type streamReadCloser struct {
	reader    io.ReadCloser
	cmd       *exec.Cmd
	stderrBuf *bytes.Buffer
}

func (r *streamReadCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

// Close terminates the ffmpeg process and closes the stdout pipe.
// If ffmpeg exited with an error, the error message (from stderr) is returned.
func (r *streamReadCloser) Close() error {
	r.reader.Close()
	if r.cmd.Process != nil {
		r.cmd.Process.Kill()
	}
	// Wait for the process to exit to avoid zombie processes.
	err := r.cmd.Wait()
	if err != nil && r.stderrBuf != nil && r.stderrBuf.Len() > 0 {
		return fmt.Errorf("ffmpeg: %w\nstderr: %s", err, r.stderrBuf.String())
	}
	return nil
}
