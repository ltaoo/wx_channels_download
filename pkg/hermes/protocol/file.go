package protocol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"wx_channel/pkg/hermes"
)

const file_probe_size = 512

// FileDriver reads an existing local file through the regular Hermes resource
// pipeline. It is used for materializing scraper caches without another
// network request.
type FileDriver struct{}

// NewFileDriver creates a local-file protocol driver.
func NewFileDriver() *FileDriver {
	return &FileDriver{}
}

// Protocols returns the local file protocol identifier.
func (d *FileDriver) Protocols() []string { return []string{"file"} }

// Prepare inspects the local source and returns its size and media hints.
func (d *FileDriver) Prepare(ctx context.Context, endpoint hermes.Endpoint) (hermes.PreparedResource, error) {
	if err := context.Cause(ctx); err != nil {
		return hermes.PreparedResource{}, err
	}
	file_path, err := local_file_path(endpoint.URL)
	if err != nil {
		return hermes.PreparedResource{}, err
	}
	file_info, err := os.Stat(file_path)
	if err != nil {
		return hermes.PreparedResource{}, err
	}
	if !file_info.Mode().IsRegular() {
		return hermes.PreparedResource{}, fmt.Errorf("local endpoint is not a regular file: %s", file_path)
	}

	probe_size := file_info.Size()
	if probe_size > file_probe_size {
		probe_size = file_probe_size
	}
	probe_data := make([]byte, probe_size)
	if probe_size > 0 {
		file, open_err := os.Open(file_path)
		if open_err != nil {
			return hermes.PreparedResource{}, open_err
		}
		read_size, read_err := io.ReadFull(file, probe_data)
		close_err := file.Close()
		if read_err != nil && !errors.Is(read_err, io.ErrUnexpectedEOF) {
			return hermes.PreparedResource{}, read_err
		}
		if close_err != nil {
			return hermes.PreparedResource{}, close_err
		}
		probe_data = probe_data[:read_size]
	}

	content_type := mime.TypeByExtension(strings.ToLower(filepath.Ext(file_path)))
	return hermes.PreparedResource{
		Size:          file_info.Size(),
		SupportsRange: true,
		ContentType:   content_type,
		ProbeData:     probe_data,
	}, nil
}

// Open opens the requested byte range from the local source file.
func (d *FileDriver) Open(ctx context.Context, endpoint hermes.Endpoint, request hermes.ReadRequest) (io.ReadCloser, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	file_path, err := local_file_path(endpoint.URL)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(file_path)
	if err != nil {
		return nil, err
	}
	file_info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !file_info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("local endpoint is not a regular file: %s", file_path)
	}

	offset_start := request.OffsetStart
	if offset_start < 0 || offset_start > file_info.Size() {
		_ = file.Close()
		return nil, fmt.Errorf("local endpoint range starts outside file: %d", offset_start)
	}
	read_length := file_info.Size() - offset_start
	if request.UseRange && request.OffsetEnd >= offset_start && request.OffsetEnd < file_info.Size() {
		read_length = request.OffsetEnd - offset_start + 1
	}
	return &file_range_reader{
		file:   file,
		reader: io.NewSectionReader(file, offset_start, read_length),
	}, nil
}

type file_range_reader struct {
	file   *os.File
	reader *io.SectionReader
}

func (r *file_range_reader) Read(data []byte) (int, error) {
	return r.reader.Read(data)
}

func (r *file_range_reader) Close() error {
	return r.file.Close()
}

func local_file_path(raw_url string) (string, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return "", errors.New("local endpoint path is empty")
	}
	file_path := raw_url
	if strings.HasPrefix(strings.ToLower(raw_url), "file://") {
		parsed_url, err := url.Parse(raw_url)
		if err != nil {
			return "", fmt.Errorf("parse local endpoint: %w", err)
		}
		if parsed_url.Host != "" && !strings.EqualFold(parsed_url.Host, "localhost") {
			return "", fmt.Errorf("local endpoint host is not supported: %s", parsed_url.Host)
		}
		file_path, err = url.PathUnescape(parsed_url.Path)
		if err != nil {
			return "", fmt.Errorf("decode local endpoint path: %w", err)
		}
	}
	if !filepath.IsAbs(file_path) {
		return "", fmt.Errorf("local endpoint path must be absolute: %s", file_path)
	}
	return filepath.Clean(file_path), nil
}
