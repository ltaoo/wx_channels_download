package protocol

import (
	"bytes"
	"context"
	"fmt"
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
func (d *InlineDriver) Open(ctx context.Context, endpoint hermes.Endpoint, request hermes.ReadRequest) (io.ReadCloser, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	data := []byte(endpoint.URL)
	data_size := int64(len(data))
	offset_start := request.OffsetStart
	if offset_start < 0 || offset_start > data_size {
		return nil, fmt.Errorf("inline endpoint range starts outside content: %d", offset_start)
	}

	offset_end := data_size
	if request.UseRange && request.OffsetEnd >= offset_start && request.OffsetEnd < data_size {
		offset_end = request.OffsetEnd + 1
	}

	return io.NopCloser(bytes.NewReader(data[offset_start:offset_end])), nil
}
