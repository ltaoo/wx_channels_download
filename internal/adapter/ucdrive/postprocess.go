package ucdriveadapter

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Postprocess converts UC Drive text resources to UTF-8 before finalization.
func (a *UCDriveAdapter) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	if info == nil {
		return fmt.Errorf("ucdrive postprocess: task is nil")
	}
	for resource_index := range info.Resources {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		resource := &info.Resources[resource_index]
		if !is_ucdrive_text_resource(resource.Kind) {
			continue
		}
		file_path := strings.TrimSpace(resource.FilePath)
		if file_path == "" {
			return fmt.Errorf("ucdrive postprocess: resource %d has no downloaded file", resource.ID)
		}
		file_data, err := os.ReadFile(file_path)
		if err != nil {
			return fmt.Errorf("ucdrive postprocess: read resource %q: %w", resource.Name, err)
		}
		processed_data, err := decode_ucdrive_text(file_data)
		if err != nil {
			return fmt.Errorf("ucdrive postprocess: decode resource %q: %w", resource.Name, err)
		}
		if err := replace_ucdrive_file_contents(file_path, processed_data); err != nil {
			return fmt.Errorf("ucdrive postprocess: write resource %q: %w", resource.Name, err)
		}
		resource.Kind = "text/plain"
		resource.Size = int64(len(processed_data))
		resource.Downloaded = resource.Size
		if resource.Extra == nil {
			resource.Extra = make(map[string]string)
		}
		resource.Extra["encoding"] = "utf-8"
		resource.Extra["postprocessed"] = "true"
		deps.Logger.Info().
			Int("task_id", info.ID).
			Int("resource_id", resource.ID).
			Str("resource_name", resource.Name).
			Msg("Postprocessor.ucdrive: text resource converted to UTF-8")
	}
	return nil
}

func is_ucdrive_text_resource(kind string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if media_type, _, err := mime.ParseMediaType(kind); err == nil {
		kind = media_type
	}
	return kind == "text" ||
		strings.HasPrefix(kind, "text/") ||
		kind == "application/json" ||
		kind == "application/javascript" ||
		kind == "application/xml" ||
		kind == "application/yaml" ||
		kind == "application/x-yaml"
}

func decode_ucdrive_text(file_data []byte) ([]byte, error) {
	if utf8.Valid(file_data) {
		return file_data, nil
	}
	return simplifiedchinese.GB18030.NewDecoder().Bytes(file_data)
}

func replace_ucdrive_file_contents(file_path string, data []byte) error {
	file_info, err := os.Stat(file_path)
	if err != nil {
		return err
	}
	temporary_file, err := os.CreateTemp(filepath.Dir(file_path), ".ucdrive-text-*")
	if err != nil {
		return err
	}
	temporary_path := temporary_file.Name()
	defer os.Remove(temporary_path)
	if err := temporary_file.Chmod(file_info.Mode().Perm()); err != nil {
		temporary_file.Close()
		return err
	}
	if _, err := temporary_file.Write(data); err != nil {
		temporary_file.Close()
		return err
	}
	if err := temporary_file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary_path, file_path)
}
