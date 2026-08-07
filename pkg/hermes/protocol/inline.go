package protocol

import (
	"bytes"
	"context"
	"io"

	"wx_channel/pkg/hermes"
)

// InlineDriver is an inline content protocol driver for resources that do not need network download
// but must be persisted locally. Content data is stored directly in the Endpoint.URL field;
// both Prepare and Open read from memory.
type InlineDriver struct{}

// NewInlineDriver creates a new inline protocol driver.
func NewInlineDriver() *InlineDriver {
	return &InlineDriver{}
}

// Protocols returns the inline protocol identifier.
func (d *InlineDriver) Protocols() []string { return []string{"inline"} }

// Prepare returns the size and content type of the inline resource.
func (d *InlineDriver) Prepare(ctx context.Context, endpoint hermes.Endpoint) (hermes.PreparedResource, error) {
	data := endpoint.URL
	return hermes.PreparedResource{
		Size:          int64(len(data)),
		SupportsRange: true,
		ContentType:   "text/html",
	}, nil
}

// Open returns a ReadCloser reading from in-memory data, supporting Range requests.
func (d *InlineDriver) Open(ctx context.Context, endpoint hermes.Endpoint, req hermes.ReadRequest) (io.ReadCloser, error) {
	data := []byte(endpoint.URL)

	start := req.OffsetStart
	end := int64(len(data)) - 1

	if req.OffsetEnd > 0 && req.OffsetEnd < end {
		end = req.OffsetEnd
	}

	if start > end {
		start = end
	}

	return io.NopCloser(bytes.NewReader(data[start : end+1])), nil
}
