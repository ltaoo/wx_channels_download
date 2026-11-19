// Adapted from dramadl; see ../NOTICE and ../LICENSE.
package vqq

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// range_transport presents one streaming response to Hermes while requesting
// the bounded CDN ranges required by affected Tencent edges. It writes no files.
type range_transport struct {
	client   *backend
	mu       sync.Mutex
	versions map[string]string
}

func (t *range_transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	start, end := int64(0), int64(-1)
	if value := req.Header.Get("Range"); value != "" {
		if _, err := fmt.Sscanf(value, "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start {
			return nil, fmt.Errorf("腾讯媒体请求范围无效")
		}
	}
	reader := &range_reader{client: t.client, ctx: req.Context(), address: req.URL.String(), offset: start, target: end, total: -1}
	if err := reader.open_next(); err != nil {
		return nil, err
	}
	version := fmt.Sprintf("%d|%s|%s", reader.total, reader.etag, reader.modified)
	t.mu.Lock()
	if t.versions == nil {
		t.versions = map[string]string{}
	}
	previous := t.versions[reader.address]
	if previous == "" {
		t.versions[reader.address] = version
	}
	t.mu.Unlock()
	if previous != "" && previous != version {
		reader.Close()
		return nil, fmt.Errorf("腾讯媒体资源版本已变化")
	}
	last := reader.total - 1
	if end >= 0 && end < last {
		last = end
	}
	reader.target = last
	length := last - start + 1
	header := reader.headers.Clone()
	header.Set("Content-Length", strconv.FormatInt(length, 10))
	status := http.StatusOK
	if req.Header.Get("Range") != "" {
		status = http.StatusPartialContent
		header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, last, reader.total))
	} else {
		header.Del("Content-Range")
	}
	return &http.Response{StatusCode: status, Header: header, ContentLength: length, Body: reader, Request: req}, nil
}

type range_reader struct {
	mu                    sync.Mutex
	closed                bool
	client                *backend
	ctx                   context.Context
	address               string
	offset, target, total int64
	remaining             int64
	etag, modified        string
	body                  io.ReadCloser
	cancel                context.CancelFunc
	headers               http.Header
}

func (r *range_reader) open_next() error {
	end := r.offset + tencent_range_size - 1
	if r.target >= 0 && end > r.target {
		end = r.target
	}
	ctx, cancel := context.WithTimeout(r.ctx, 8*time.Second)
	response, err := r.client.Request(ctx, r.address, nil, false, fmt.Sprintf("bytes=%d-%d", r.offset, end))
	if err != nil {
		cancel()
		return err
	}
	start, actual_end, total, err := parse_content_range(response.Header.Get("Content-Range"))
	expected_end := min_int64(end, total-1)
	if err != nil || response.StatusCode != http.StatusPartialContent || start != r.offset || actual_end != expected_end || (r.total >= 0 && (total != r.total || response.Header.Get("ETag") != r.etag || response.Header.Get("Last-Modified") != r.modified)) {
		response.Body.Close()
		cancel()
		return fmt.Errorf("腾讯分段响应位置、长度或资源版本不一致")
	}
	length := actual_end - start + 1
	if response.ContentLength >= 0 && response.ContentLength != length {
		response.Body.Close()
		cancel()
		return fmt.Errorf("腾讯分段 Content-Length 不匹配")
	}
	if r.total < 0 {
		r.total = total
		r.etag = response.Header.Get("ETag")
		r.modified = response.Header.Get("Last-Modified")
		r.headers = response.Header.Clone()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		response.Body.Close()
		cancel()
		return io.ErrClosedPipe
	}
	r.body, r.cancel, r.remaining = response.Body, cancel, length
	return nil
}

func (r *range_reader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	r.mu.Lock()
	body, cancel, closed := r.body, r.cancel, r.closed
	r.mu.Unlock()
	if closed {
		return 0, io.ErrClosedPipe
	}
	if body == nil && r.offset > r.target {
		return 0, io.EOF
	}
	if r.remaining == 0 {
		// Read once beyond the declared span to reject oversized CDN bodies.
		var extra [1]byte
		count, err := body.Read(extra[:])
		body.Close()
		cancel()
		r.mu.Lock()
		r.body = nil
		r.mu.Unlock()
		if count != 0 || (err != nil && err != io.EOF) {
			return 0, fmt.Errorf("腾讯分段响应字节数不匹配")
		}
		if r.offset > r.target {
			return 0, io.EOF
		}
		if err := r.open_next(); err != nil {
			return 0, err
		}
		r.mu.Lock()
		body = r.body
		r.mu.Unlock()
	}
	if int64(len(buffer)) > r.remaining {
		buffer = buffer[:r.remaining]
	}
	count, err := body.Read(buffer)
	r.offset += int64(count)
	r.remaining -= int64(count)
	if err == io.EOF {
		if r.remaining > 0 {
			return count, io.ErrUnexpectedEOF
		}
		// The final span is complete. A following read advances the response.
		return count, nil
	}
	return count, err
}

func (r *range_reader) Close() error {
	r.mu.Lock()
	r.closed = true
	body, cancel := r.body, r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if body != nil {
		return body.Close()
	}
	return nil
}

var _ http.RoundTripper = (*range_transport)(nil)
