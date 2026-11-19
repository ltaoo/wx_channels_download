package minib

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/clawreq"
)

func request_local_file(ctx context.Context, method string, request_url *url.URL) (*clawreq.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	normalized_method := strings.ToUpper(method)
	if normalized_method != http.MethodGet && normalized_method != http.MethodHead {
		return local_file_response(request_url, http.StatusMethodNotAllowed, nil, time.Now()), nil
	}

	file_path, path_err := local_file_path(request_url)
	if path_err != nil {
		return local_file_response(request_url, http.StatusBadRequest, nil, time.Now()), nil
	}

	file, open_err := os.Open(file_path)
	if open_err != nil {
		return local_file_error_response(request_url, open_err), nil
	}
	defer file.Close()

	file_info, stat_err := file.Stat()
	if stat_err != nil {
		return local_file_error_response(request_url, stat_err), nil
	}
	if file_info.IsDir() {
		return local_file_response(request_url, http.StatusForbidden, nil, time.Now()), nil
	}

	body := []byte(nil)
	if normalized_method == http.MethodGet {
		var read_err error
		body, read_err = io.ReadAll(file)
		if read_err != nil {
			return local_file_error_response(request_url, read_err), nil
		}
	}

	response := local_file_response(request_url, http.StatusOK, body, file_info.ModTime())
	response.Header.Set("Content-Type", local_file_content_type(file_path, body))
	if normalized_method == http.MethodHead {
		response.Header.Set("Content-Length", strconv.FormatInt(file_info.Size(), 10))
	}
	return response, nil
}

func local_file_path(request_url *url.URL) (string, error) {
	if request_url.Host != "" && !strings.EqualFold(request_url.Host, "localhost") {
		return "", fmt.Errorf("minib: unsupported file URL host %q", request_url.Host)
	}
	if request_url.Path == "" {
		return "", fmt.Errorf("minib: file URL has no path")
	}

	file_path := request_url.Path
	if runtime.GOOS == "windows" && len(file_path) >= 3 && file_path[0] == '/' &&
		((file_path[1] >= 'a' && file_path[1] <= 'z') || (file_path[1] >= 'A' && file_path[1] <= 'Z')) &&
		file_path[2] == ':' {
		file_path = file_path[1:]
	}
	return filepath.FromSlash(file_path), nil
}

func local_file_response(request_url *url.URL, status_code int, body []byte, modified_at time.Time) *clawreq.Response {
	response_headers := http.Header{}
	response_headers.Set("Content-Length", strconv.Itoa(len(body)))
	response_headers.Set("Cache-Control", "no-store")
	if !modified_at.IsZero() {
		response_headers.Set("Last-Modified", modified_at.UTC().Format(http.TimeFormat))
	}

	return &clawreq.Response{
		StatusCode: status_code,
		Status:     http.StatusText(status_code),
		Header:     response_headers,
		Body:       append([]byte(nil), body...),
		FinalURL:   request_url.String(),
	}
}

func local_file_error_response(request_url *url.URL, file_err error) *clawreq.Response {
	status_code := http.StatusInternalServerError
	switch {
	case os.IsNotExist(file_err):
		status_code = http.StatusNotFound
	case os.IsPermission(file_err):
		status_code = http.StatusForbidden
	}
	return local_file_response(request_url, status_code, nil, time.Now())
}

func local_file_content_type(file_path string, body []byte) string {
	if content_type := mime.TypeByExtension(filepath.Ext(file_path)); content_type != "" {
		return content_type
	}
	if content_type := http.DetectContentType(body); content_type != "" {
		return content_type
	}
	return "application/octet-stream"
}

func is_page_url_scheme(scheme string) bool {
	switch scheme {
	case "http", "https", "file":
		return true
	default:
		return false
	}
}
