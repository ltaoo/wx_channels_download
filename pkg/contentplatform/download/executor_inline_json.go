package download

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InlineJSONExecutor struct{}

func NewInlineJSONExecutor() *InlineJSONExecutor {
	return &InlineJSONExecutor{}
}

func (e *InlineJSONExecutor) Name() string {
	return "inline_json"
}

func (e *InlineJSONExecutor) CanHandle(source DownloadSpec) bool {
	return strings.EqualFold(source.Protocol, "inline_json") ||
		strings.HasPrefix(strings.ToLower(source.URL), "inline-json://")
}

func (e *InlineJSONExecutor) Execute(ctx context.Context, req ExecuteRequest) error {
	payload := inlineJSONFromRequest(req)
	if payload == nil {
		return fmt.Errorf("inline json body is empty")
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(req.DestPath, data, 0o644); err != nil {
		return err
	}
	if req.OnProgress != nil {
		req.OnProgress(Progress{
			DownloadedBytes: int64(len(data)),
			TotalBytes:      int64(len(data)),
			Percent:         100,
		})
	}
	return nil
}

func inlineJSONFromRequest(req ExecuteRequest) any {
	if req.Resolved == nil {
		return nil
	}
	if value := req.Resolved.Metadata["json"]; value != nil {
		return value
	}
	return req.Resolved.Content
}
