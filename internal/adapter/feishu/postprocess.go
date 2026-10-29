package feishuadapter

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/util"
)

const (
	feishu_resource_extra_key = "feishu_resource"
	feishu_resource_document  = "document"
	feishu_resource_asset     = "asset"
	feishu_asset_path_key     = "feishu_asset_path"
	feishu_asset_name_key     = "feishu_asset_name"
	feishu_postprocessed_key  = "postprocessed"
)

var feishu_output_forbidden_chars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// Postprocess embeds downloaded document images into the HTML as data URIs and
// rewrites file and video links to the final downloaded-file names so the
// saved document can open every attachment beside it.
func (a *FeishuAdapter) Postprocess(ctx context.Context, info *hermes.TaskJob, deps adapter.PostprocessDeps) error {
	if info == nil {
		return fmt.Errorf("feishu postprocess: task is nil")
	}
	document_indexes := make([]int, 0, 1)
	assets_by_path := make(map[string]*hermes.ResourceJob)
	for resource_index := range info.Resources {
		resource := &info.Resources[resource_index]
		switch resource.Extra[feishu_resource_extra_key] {
		case feishu_resource_document:
			if resource.FilePath != "" && resource.Extra[feishu_postprocessed_key] != "true" {
				document_indexes = append(document_indexes, resource_index)
			}
		case feishu_resource_asset:
			if resource.FilePath != "" {
				assets_by_path[asset_reference_key(resource)] = resource
			}
		}
	}
	if len(document_indexes) == 0 {
		deps.Logger.Info().
			Int("task_id", info.ID).
			Int("resource_count", len(info.Resources)).
			Msg("Postprocessor.feishu: no document resource to process")
		return nil
	}

	embedded_files := make([]string, 0)
	embedded_ids := make(map[int]bool)
	for _, resource_index := range document_indexes {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		resource := &info.Resources[resource_index]
		html_data, err := os.ReadFile(resource.FilePath)
		if err != nil {
			return fmt.Errorf("feishu postprocess: read document resource %q: %w", resource.Name, err)
		}
		processed, embedded, err := a.process_document_html(string(html_data), assets_by_path)
		if err != nil {
			return fmt.Errorf("feishu postprocess: process document resource %q: %w", resource.Name, err)
		}
		if err := replace_document_file(resource.FilePath, []byte(processed)); err != nil {
			return fmt.Errorf("feishu postprocess: write document resource %q: %w", resource.Name, err)
		}
		if resource.Extra == nil {
			resource.Extra = make(map[string]string)
		}
		resource.Extra[feishu_postprocessed_key] = "true"
		resource.Kind = "text/html"
		resource.Size = int64(len(processed))
		resource.Downloaded = resource.Size
		for embedded_resource := range embedded {
			embedded_ids[embedded_resource.ID] = true
			if embedded_resource.FilePath != "" {
				embedded_files = append(embedded_files, embedded_resource.FilePath)
			}
		}
		deps.Logger.Info().
			Int("task_id", info.ID).
			Int("resource_id", resource.ID).
			Int("embedded_image_count", len(embedded)).
			Int64("resource_size", resource.Size).
			Msg("Postprocessor.feishu: document images embedded")
	}
	remove_embedded_resources(info, deps, embedded_ids, embedded_files)
	return nil
}

func asset_reference_key(resource *hermes.ResourceJob) string {
	if path := strings.TrimSpace(resource.Extra[feishu_asset_path_key]); path != "" {
		return path
	}
	name := strings.TrimSpace(resource.Name)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func (a *FeishuAdapter) process_document_html(source string, assets_by_path map[string]*hermes.ResourceJob) (string, map[*hermes.ResourceJob]bool, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(source))
	if err != nil {
		return "", nil, err
	}
	embedded := make(map[*hermes.ResourceJob]bool)
	data_uri_cache := make(map[*hermes.ResourceJob]string)
	for _, image_node := range document.Find("img[src]").Nodes {
		image := goquery.NewDocumentFromNode(image_node).Selection
		source_reference := strings.TrimSpace(image.AttrOr("src", ""))
		_, resource := document_asset(assets_by_path, source_reference)
		if resource == nil {
			continue
		}
		data_uri, cached := data_uri_cache[resource]
		if !cached {
			embedded_data_uri, _ := image_data_uri(resource)
			data_uri_cache[resource] = embedded_data_uri
			data_uri = embedded_data_uri
		}
		if data_uri != "" {
			image.SetAttr("src", data_uri)
			image.RemoveAttr("loading")
			embedded[resource] = true
			continue
		}
		// Keep the image accessible even when it could not be embedded.
		image.SetAttr("src", url.PathEscape(a.final_resource_filename(resource)))
	}
	for _, link_node := range document.Find("a[href]").Nodes {
		link := goquery.NewDocumentFromNode(link_node).Selection
		source_reference := strings.TrimSpace(link.AttrOr("href", ""))
		asset, resource := document_asset(assets_by_path, source_reference)
		if resource == nil {
			continue
		}
		final_name := a.final_resource_filename(resource)
		link.SetAttr("href", url.PathEscape(final_name))
		if asset != "" {
			link.SetAttr("download", asset)
		}
		if strings.HasPrefix(strings.ToLower(resource.Kind), "video/") {
			upgrade_video_block(link, final_name)
		}
	}
	processed, err := document.Html()
	if err != nil {
		return "", nil, err
	}
	return processed, embedded, nil
}

// document_asset resolves one HTML reference (images/token.png or
// files/token.pdf) to the downloaded resource behind it.
func document_asset(assets_by_path map[string]*hermes.ResourceJob, reference string) (string, *hermes.ResourceJob) {
	reference = strings.TrimSpace(reference)
	if reference == "" || strings.HasPrefix(strings.ToLower(reference), "data:") {
		return "", nil
	}
	if resource := assets_by_path[reference]; resource != nil {
		return strings.TrimSpace(resource.Extra[feishu_asset_name_key]), resource
	}
	key := strings.TrimSuffix(reference, filepath.Ext(reference))
	if resource := assets_by_path[key]; resource != nil {
		return strings.TrimSpace(resource.Extra[feishu_asset_name_key]), resource
	}
	return "", nil
}

func image_data_uri(resource *hermes.ResourceJob) (string, error) {
	data, err := os.ReadFile(resource.FilePath)
	if err != nil {
		return "", err
	}
	media_type := strings.ToLower(strings.TrimSpace(resource.Kind))
	if !strings.HasPrefix(media_type, "image/") {
		media_type = http.DetectContentType(data)
	}
	if !strings.HasPrefix(media_type, "image/") {
		return "", nil
	}
	return "data:" + media_type + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// upgrade_video_block replaces a video file placeholder with an inline video
// player while keeping the independent download link.
func upgrade_video_block(link *goquery.Selection, video_source string) {
	view_block := link.Closest(`section[data-n="view-block"]`)
	if view_block.Length() == 0 {
		return
	}
	placeholder := view_block.Find(`div[data-n="file-address-placeholder"]`)
	if placeholder.Length() == 0 {
		return
	}
	placeholder.AfterHtml(`<video data-n="view-video" class="view-video" controls preload="metadata" src="` + html.EscapeString(video_source) + `">当前浏览器不支持视频播放</video>`)
	placeholder.Remove()
}

// final_resource_filename mirrors the filename Hermes finalize_resource_filenames
// assigns after postprocessing: the configured template applied to the resource
// name, sanitized, plus the canonical extension for the resource kind.
func (a *FeishuAdapter) final_resource_filename(resource *hermes.ResourceJob) string {
	base_name := strings.TrimSpace(resource.Name)
	if template := strings.TrimSpace(a.filename_template_value()); template != "" && strings.Contains(template, "{{") {
		meta := make(map[string]string, len(resource.Extra)+2)
		for key, value := range resource.Extra {
			meta[key] = value
		}
		meta["download_at"] = time.Now().Format("2006-01-02")
		meta["filename"] = base_name
		base_name = util.ReplaceTemplateVars(template, meta)
	}
	return sanitize_output_name(base_name) + hermes.CanonicalExtensionForMIMEType(resource.Kind)
}

func sanitize_output_name(value string) string {
	sanitized := feishu_output_forbidden_chars.ReplaceAllString(value, "_")
	sanitized = strings.TrimSpace(sanitized)
	if sanitized == "." || sanitized == ".." {
		return strings.Repeat("_", len(sanitized))
	}
	sanitized = strings.TrimRight(sanitized, ". ")
	if is_windows_reserved_name(sanitized) {
		sanitized = "_" + sanitized
	}
	return sanitized
}

func is_windows_reserved_name(filename string) bool {
	base_name := strings.ToUpper(strings.SplitN(filename, ".", 2)[0])
	switch base_name {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

func remove_embedded_resources(info *hermes.TaskJob, deps adapter.PostprocessDeps, embedded_ids map[int]bool, embedded_files []string) {
	if len(embedded_ids) == 0 {
		return
	}
	kept_resources := make([]hermes.ResourceJob, 0, len(info.Resources))
	for _, resource := range info.Resources {
		if embedded_ids[resource.ID] {
			continue
		}
		kept_resources = append(kept_resources, resource)
	}
	info.Resources = kept_resources
	for _, file_path := range embedded_files {
		if err := os.Remove(file_path); err != nil && !os.IsNotExist(err) {
			deps.Logger.Warn().
				Int("task_id", info.ID).
				Str("file_path", file_path).
				Err(err).
				Msg("Postprocessor.feishu: failed to remove embedded image file")
		}
	}
}

func replace_document_file(file_path string, data []byte) error {
	file_info, err := os.Stat(file_path)
	if err != nil {
		return err
	}
	temporary_file, err := os.CreateTemp(filepath.Dir(file_path), ".feishu-postprocess-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temporary_file.Name())
	defer temporary_file.Close()
	if err := temporary_file.Chmod(file_info.Mode().Perm()); err != nil {
		return err
	}
	if _, err := temporary_file.Write(data); err != nil {
		return err
	}
	if err := temporary_file.Sync(); err != nil {
		return err
	}
	if err := temporary_file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary_file.Name(), file_path)
}
