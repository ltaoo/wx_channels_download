package download

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const multiSourcesMetadataKey = "sources"

type commandRunner func(ctx context.Context, name string, args ...string) error

type MultiHTTPExecutor struct {
	client     *http.Client
	runCommand commandRunner
}

func NewMultiHTTPExecutor(client *http.Client) *MultiHTTPExecutor {
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	return &MultiHTTPExecutor{
		client:     client,
		runCommand: runExternalCommand,
	}
}

func (e *MultiHTTPExecutor) Name() string {
	return ProtocolMultiHTTP
}

func (e *MultiHTTPExecutor) CanHandle(source DownloadSpec) bool {
	return strings.EqualFold(source.Protocol, ProtocolMultiHTTP)
}

func (e *MultiHTTPExecutor) Execute(ctx context.Context, req ExecuteRequest) error {
	sources, err := multiSourcesFromRequest(req)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return fmt.Errorf("multi_http download has no sources")
	}
	if len(sources) == 1 {
		return NewHTTPExecutor(e.client).Execute(ctx, ExecuteRequest{
			Resolved:   req.Resolved,
			Source:     sourceDownloadSpec(sources[0]),
			DestPath:   req.DestPath,
			OnProgress: req.OnProgress,
			OnFiles:    req.OnFiles,
		})
	}
	if e.runCommand == nil {
		e.runCommand = runExternalCommand
	}
	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(req.DestPath), ".multi-http-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	paths := make([]string, 0, len(sources))
	downloaded := make([]int64, len(sources))
	total := multiSourcesTotalSize(sources)
	httpExecutor := NewHTTPExecutor(e.client)
	for i, source := range sources {
		path := filepath.Join(tempDir, multiSourceFilename(i, source))
		paths = append(paths, path)
		index := i
		if err := httpExecutor.Execute(ctx, ExecuteRequest{
			Resolved: req.Resolved,
			Source:   sourceDownloadSpec(source),
			DestPath: path,
			OnProgress: func(progress Progress) {
				downloaded[index] = progress.DownloadedBytes
				if req.OnProgress != nil {
					req.OnProgress(aggregateMultiProgress(downloaded, total))
				}
			},
			OnFiles: req.OnFiles,
		}); err != nil {
			return fmt.Errorf("download source %q: %w", firstNonEmpty(source.ID, source.URL), err)
		}
		if source.Size == 0 {
			if info, statErr := os.Stat(path); statErr == nil {
				downloaded[index] = info.Size()
			}
		}
	}

	mergedPath := filepath.Join(tempDir, "merged"+filepath.Ext(req.DestPath))
	if err := e.mergeWithFFmpeg(ctx, paths, sources, mergedPath); err != nil {
		return err
	}
	if err := renameReplace(mergedPath, req.DestPath); err != nil {
		return err
	}
	if req.OnProgress != nil {
		size := existingFileSize(req.DestPath)
		req.OnProgress(Progress{DownloadedBytes: size, TotalBytes: size, Percent: 100})
	}
	return nil
}

func (e *MultiHTTPExecutor) mergeWithFFmpeg(ctx context.Context, paths []string, sources []MultiSourceSpec, outputPath string) error {
	args := []string{"-y"}
	for _, path := range paths {
		args = append(args, "-i", path)
	}
	mapped := false
	for i, source := range sources {
		if source.HasVideo {
			args = append(args, "-map", fmt.Sprintf("%d:v:0", i))
			mapped = true
		}
		if source.HasAudio {
			args = append(args, "-map", fmt.Sprintf("%d:a:0", i))
			mapped = true
		}
	}
	if !mapped {
		for i := range sources {
			args = append(args, "-map", strconvItoa(i))
		}
	}
	args = append(args, "-c", "copy")
	switch strings.ToLower(filepath.Ext(outputPath)) {
	case ".mp4", ".m4v", ".mov":
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, outputPath)
	if err := e.runCommand(ctx, "ffmpeg", args...); err != nil {
		return fmt.Errorf("ffmpeg merge failed: %w", err)
	}
	return nil
}

func multiSourcesFromRequest(req ExecuteRequest) ([]MultiSourceSpec, error) {
	if req.Resolved != nil && req.Resolved.Metadata != nil {
		if raw, ok := req.Resolved.Metadata[multiSourcesMetadataKey]; ok {
			return parseMultiSources(raw)
		}
		if raw, ok := req.Resolved.Metadata["multi_sources"]; ok {
			return parseMultiSources(raw)
		}
	}
	if len(req.Source.Body) > 0 {
		return parseMultiSources(req.Source.Body)
	}
	return nil, nil
}

func parseMultiSources(raw any) ([]MultiSourceSpec, error) {
	switch v := raw.(type) {
	case []MultiSourceSpec:
		return append([]MultiSourceSpec(nil), v...), nil
	case []*MultiSourceSpec:
		out := make([]MultiSourceSpec, 0, len(v))
		for _, source := range v {
			if source != nil {
				out = append(out, *source)
			}
		}
		return out, nil
	case []byte:
		var out []MultiSourceSpec
		if err := json.Unmarshal(v, &out); err != nil {
			return nil, err
		}
		return out, nil
	default:
		rawJSON, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var out []MultiSourceSpec
		if err := json.Unmarshal(rawJSON, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
}

func sourceDownloadSpec(source MultiSourceSpec) DownloadSpec {
	method := source.Method
	if method == "" {
		method = http.MethodGet
	}
	return DownloadSpec{
		URL:         source.URL,
		Method:      method,
		Headers:     cloneStringMap(source.Headers),
		Protocol:    "http",
		Connections: 1,
		ChunkSize:   source.ChunkSize,
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func multiSourcesTotalSize(sources []MultiSourceSpec) int64 {
	var total int64
	for _, source := range sources {
		if source.Size <= 0 {
			return 0
		}
		total += source.Size
	}
	return total
}

func aggregateMultiProgress(downloaded []int64, total int64) Progress {
	var done int64
	for _, value := range downloaded {
		done += value
	}
	progress := Progress{DownloadedBytes: done, TotalBytes: total}
	if total > 0 {
		progress.Percent = float64(done) * 100 / float64(total)
	}
	return progress
}

var unsafeFilenameCharRE = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func multiSourceFilename(index int, source MultiSourceSpec) string {
	name := strings.TrimSpace(source.ID)
	if name == "" {
		name = fmt.Sprintf("source-%d", index)
	}
	name = unsafeFilenameCharRE.ReplaceAllString(name, "_")
	ext := strings.TrimPrefix(strings.TrimSpace(source.Ext), ".")
	if ext == "" {
		ext = "bin"
	}
	return fmt.Sprintf("%02d-%s.%s", index, name, ext)
}

func runExternalCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}
	return nil
}

func strconvItoa(value int) string {
	return fmt.Sprintf("%d", value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
