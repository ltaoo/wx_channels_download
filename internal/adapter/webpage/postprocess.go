package webpageadapter

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
)

// Postprocess embeds every downloaded article image into the Markdown resource
// as a data URI, producing one self-contained archive file.
func (a *WebpageAdapter) Postprocess(postprocess_context context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	if info == nil {
		return fmt.Errorf("webpage postprocess: task is nil")
	}
	markdown_resource := webpage_markdown_resource(info.Resources)
	image_count := webpage_image_resource_count(info.Resources)
	if image_count == 0 {
		return nil
	}
	if markdown_resource == nil || strings.TrimSpace(markdown_resource.FilePath) == "" {
		return fmt.Errorf("webpage postprocess: task %d has no downloaded Markdown resource", info.ID)
	}
	markdown_data, err := os.ReadFile(markdown_resource.FilePath)
	if err != nil {
		return fmt.Errorf("webpage postprocess: read Markdown resource: %w", err)
	}
	markdown := string(markdown_data)
	embedded_resources := make(map[string]struct{}, image_count)
	for resource_index := range info.Resources {
		if err := context.Cause(postprocess_context); err != nil {
			return err
		}
		resource := &info.Resources[resource_index]
		if resource.Extra[webpage_postprocess_image_marker_key] != webpage_postprocess_image_marker_value {
			continue
		}
		placeholder := strings.TrimSpace(resource.Extra[webpage_image_placeholder_key])
		if placeholder == "" || !strings.Contains(markdown, placeholder) {
			return fmt.Errorf("webpage postprocess: Markdown 中缺少图片占位符: %s", resource.Name)
		}
		data_uri, err := webpage_image_data_uri(resource)
		if err != nil {
			return fmt.Errorf("webpage postprocess: embed image %q: %w", resource.Name, err)
		}
		markdown = strings.ReplaceAll(markdown, placeholder, data_uri)
		embedded_resources[webpage_resource_key(resource)] = struct{}{}
	}
	if len(embedded_resources) != image_count {
		return fmt.Errorf("webpage postprocess: embedded %d of %d downloaded images", len(embedded_resources), image_count)
	}
	processed_data := []byte(markdown)
	if err := replace_webpage_archive_file(markdown_resource.FilePath, processed_data); err != nil {
		return fmt.Errorf("webpage postprocess: write Markdown resource: %w", err)
	}
	markdown_resource.Kind = "text/markdown"
	markdown_resource.Size = int64(len(processed_data))
	markdown_resource.Downloaded = markdown_resource.Size
	if markdown_resource.Extra == nil {
		markdown_resource.Extra = make(map[string]string)
	}
	markdown_resource.Extra["postprocessed"] = "true"

	kept_resources := make([]hermes.ResourceJob, 0, len(info.Resources)-len(embedded_resources))
	for resource_index := range info.Resources {
		resource := info.Resources[resource_index]
		if _, embedded := embedded_resources[webpage_resource_key(&resource)]; !embedded {
			kept_resources = append(kept_resources, resource)
			continue
		}
		if err := os.Remove(resource.FilePath); err != nil && !os.IsNotExist(err) {
			deps.Logger.Warn().
				Int("task_id", info.ID).
				Int("resource_id", resource.ID).
				Str("file_path", resource.FilePath).
				Err(err).
				Msg("Postprocessor.webpage: failed to remove embedded image file")
		}
	}
	info.Resources = kept_resources
	deps.Logger.Info().
		Int("task_id", info.ID).
		Int("embedded_image_count", len(embedded_resources)).
		Int64("markdown_size", markdown_resource.Size).
		Msg("Postprocessor.webpage: images embedded into Markdown")
	return nil
}

func webpage_markdown_resource(resources []hermes.ResourceJob) *hermes.ResourceJob {
	for resource_index := range resources {
		resource := &resources[resource_index]
		if resource.Extra[webpage_postprocess_marker_key] == webpage_postprocess_marker_value {
			return resource
		}
	}
	return nil
}

func webpage_image_resource_count(resources []hermes.ResourceJob) int {
	image_count := 0
	for resource_index := range resources {
		if resources[resource_index].Extra[webpage_postprocess_image_marker_key] == webpage_postprocess_image_marker_value {
			image_count++
		}
	}
	return image_count
}

func webpage_image_data_uri(resource *hermes.ResourceJob) (string, error) {
	if resource == nil || strings.TrimSpace(resource.FilePath) == "" {
		return "", fmt.Errorf("downloaded image file is missing")
	}
	image_data, err := os.ReadFile(resource.FilePath)
	if err != nil {
		return "", err
	}
	mime_type := strings.ToLower(strings.TrimSpace(resource.Kind))
	if parsed_type, _, parse_err := mime.ParseMediaType(mime_type); parse_err == nil {
		mime_type = parsed_type
	}
	if !strings.HasPrefix(mime_type, "image/") {
		mime_type = strings.ToLower(mime.TypeByExtension(filepath.Ext(resource.FilePath)))
	}
	if !strings.HasPrefix(mime_type, "image/") {
		mime_type = strings.ToLower(http.DetectContentType(image_data))
	}
	if !strings.HasPrefix(mime_type, "image/") {
		return "", fmt.Errorf("downloaded resource is not an image: %s", mime_type)
	}
	return "data:" + mime_type + ";base64," + base64.StdEncoding.EncodeToString(image_data), nil
}

func webpage_resource_key(resource *hermes.ResourceJob) string {
	if resource == nil {
		return ""
	}
	if resource.ID > 0 {
		return fmt.Sprintf("id:%d", resource.ID)
	}
	if resource.UniqueID != "" {
		return "unique_id:" + resource.UniqueID
	}
	return "file_path:" + resource.FilePath
}

func replace_webpage_archive_file(file_path string, data []byte) error {
	file_info, err := os.Stat(file_path)
	if err != nil {
		return err
	}
	temporary_file, err := os.CreateTemp(filepath.Dir(file_path), ".webpage-postprocess-*.tmp")
	if err != nil {
		return err
	}
	temporary_path := temporary_file.Name()
	defer os.Remove(temporary_path)
	if err := temporary_file.Chmod(file_info.Mode().Perm()); err != nil {
		_ = temporary_file.Close()
		return err
	}
	if _, err := temporary_file.Write(data); err != nil {
		_ = temporary_file.Close()
		return err
	}
	if err := temporary_file.Sync(); err != nil {
		_ = temporary_file.Close()
		return err
	}
	if err := temporary_file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary_path, file_path)
}
