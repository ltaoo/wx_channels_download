package hermes

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"

	"wx_channel/pkg/util"
)

func (d *HermesEngine) download_resource(ctx context.Context, task *TaskJob, resource *ResourceJob) (string, error) {
	if task == nil {
		return "", errors.New("task is nil")
	}
	if resource == nil {
		return "", errors.New("resource is nil")
	}
	if strings.TrimSpace(resource.UniqueID) == "" {
		return "", errors.New("resource unique ID is required")
	}
	candidates, err := d.endpoint_candidates(resource.Endpoints)
	if err != nil {
		return "", err
	}

	var endpoint_errors []string
	var file_path string
	var expected_size int64
	for _, candidate := range candidates {
		if err := context.Cause(ctx); err != nil &&
			!(strings.EqualFold(resource.Type, ResourceTypeStream) && errors.Is(err, ErrTaskStopRequested)) {
			return "", err
		}
		if candidate.driver == nil {
			endpoint_errors = append(endpoint_errors, fmt.Sprintf("%s: protocol driver is not registered", candidate.protocol))
			continue
		}

		prepared, prepare_err := prepare_with_retry(ctx, candidate.driver, candidate.endpoint)
		if prepare_err != nil {
			if errors.Is(prepare_err, context.Canceled) {
				return "", prepare_err
			}
			endpoint_errors = append(endpoint_errors, fmt.Sprintf("%s: %v", candidate.protocol, prepare_err))
			continue
		}
		if prepared.Size < 0 {
			prepared.Size = 0
		}
		// Kind is normalized to the canonical MIME value persisted after
		// filename finalization. Extension is derived from Kind at finalize time.
		resource.Kind = prepared_target_kind(prepared)
		if expected_size > 0 && prepared.Size > 0 && prepared.Size != expected_size {
			endpoint_errors = append(endpoint_errors, fmt.Sprintf("%s: mirror resource size mismatch", candidate.protocol))
			continue
		}
		if expected_size == 0 && prepared.Size > 0 {
			expected_size = prepared.Size
		}
		if prepared.Size > 0 {
			resource.Size = prepared.Size
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Int64("resource_size", prepared.Size).
				Str("resource_size_readable", format_size(prepared.Size)).
				Msg("run - updating resource size from prepared endpoint")
			if err := d.update_resource_size(task.ID, resource.ID, prepared.Size); err != nil {
				return "", fmt.Errorf("failed to update resource size: %w", err)
			}
			d.update_tracker_size(task.ID, resource.ID, prepared.Size)
		}

		// Once segment records exist, resource.Name is the canonical path that was
		// persisted when the download first started. Reapplying filename templates
		// or hooks after a restart can produce a different path, making the existing
		// .part file appear missing and causing downloadSegments to reset every
		// persisted offset to zero.
		existing_segments, segment_err := d.store.LoadSegmentInfo(resource.ID)
		if segment_err != nil {
			return "", fmt.Errorf("failed to load existing download segments: %w", segment_err)
		}
		resuming := len(existing_segments) > 0
		if resuming {
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Int("segment_count", len(existing_segments)).
				Str("resource_name", resource.Name).
				Msg("run - existing segments found, preserving persisted filename")
		}
		file_path, err = d.file_path_for_job_resource(task, resource, candidate.endpoint.URL)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(file_path), 0755); err != nil {
			return "", fmt.Errorf("failed to create download directory: %w", err)
		}

		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Str("endpoint", candidate.endpoint.URL).
			Str("file_path", d.rel_log_path(file_path)).
			Msg("run - starting resource download")

		segment_count := choose_segment_count(prepared)
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Bool("segmented", segment_count > 1).
			Int("segment_count", segment_count).
			Int64("segment_size", minimum_segment_size).
			Msg("run - download mode selected")
		download_start := time.Now()
		if strings.EqualFold(resource.Type, ResourceTypeStream) {
			recorder, ok := candidate.driver.(StreamRecorder)
			if !ok {
				err = fmt.Errorf("protocol %s does not implement stream recording", candidate.protocol)
			} else {
				err = d.download_stream(ctx, recorder, candidate.endpoint, file_path, task, resource)
			}
		} else if segment_count > 1 {
			err = d.download_segments(ctx, candidate.driver, candidate.endpoint, file_path, task, resource, prepared, segment_count)
		} else {
			err = d.download_file(ctx, candidate.driver, candidate.endpoint, file_path, task, resource, prepared)
		}
		if err == nil {
			resource.FilePath = file_path
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Str("file_path", d.rel_log_path(file_path)).
				Dur("elapsed", time.Since(download_start)).
				Msg("run - data transfer completed")
			if prepared.Size <= 0 {
				if file_info, stat_err := os.Stat(file_path); stat_err == nil {
					resource.Size = file_info.Size()
					if err := d.update_resource_size(task.ID, resource.ID, file_info.Size()); err != nil {
						return "", fmt.Errorf("failed to update final resource size: %w", err)
					}
					d.update_tracker_size(task.ID, resource.ID, file_info.Size())
				}
			}
			if err := d.finish_download_resource(task.ID, resource.ID); err != nil {
				return "", err
			}
			return file_path, nil
		}
		// A live stop uses cancellation only to halt the recorder. Preserve any
		// merge/finalization error returned after that cancellation so the task
		// records the real failure instead of the stop sentinel.
		if errors.Is(context.Cause(ctx), ErrTaskStopRequested) {
			return "", err
		}
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return "", context.Cause(ctx)
		}
		endpoint_errors = append(endpoint_errors, fmt.Sprintf("%s: %v", candidate.protocol, err))
		d.logger.Warn().
			Int("endpoint_id", candidate.endpoint.ID).
			Str("endpoint", candidate.endpoint.URL).
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Err(err).
			Msg("run - download resource from endpoint failed, trying next mirror")
	}
	return "", fmt.Errorf("all download endpoints are unavailable: %s", strings.Join(endpoint_errors, "; "))
}

func (d *HermesEngine) finish_download_resource(task_id, resource_id int) error {
	store, ok := d.store.(ResourceStore)
	if !ok {
		return nil
	}

	d.logger.Info().
		Int("task_id", task_id).
		Int("resource_id", resource_id).
		Msg("persisting resource state")
	if err := store.FinishResource(resource_id); err != nil {
		return fmt.Errorf("failed to persist resource completion: %w", err)
	}
	return nil
}

// processOutputFilename handles download resource output filenames uniformly.
// Called after filenameTemplate and onFilename hook processing; completes:
//  1. Determine file extension (Content-Type -> magic bytes -> user-specified fallback)
//  2. User input is always treated as a plain filename (ignoring any embedded extension); the system appends the extension
//  3. Clean and truncate the base filename (preserving directory portion)
//  4. Reconstruct the full path and update task/resource info in the database
//
// Each step outputs logs for easy troubleshooting.
func (d *HermesEngine) process_output_filename(task *TaskJob, resource *ResourceJob, endpoint_url string, prepared PreparedResource, original_db_name string, resource_extensions map[int]string) (bool, error) {
	if task == nil || resource == nil || resource.ID <= 0 {
		return false, nil
	}

	raw_unique_id := strings.TrimSpace(resource.UniqueID)
	if raw_unique_id == "" {
		return false, nil
	}

	// Step 1: Separate directory and base filename (unique ID is a plain filename without extension)
	dir, base_name := filepath.Split(raw_unique_id)
	d.logger.Info().
		Int("task_id", task.ID).
		Int("resource_id", resource.ID).
		Str("raw_unique_id", raw_unique_id).
		Str("dir", dir).
		Str("base_name", base_name).
		Msg("run - output filename processing started")

	// Step 2: Determine extension
	// Priority: Content-Type -> magic bytes -> user-specified fallback suffix
	ext := extension_for_content_type(prepared.ContentType)
	if ext != "" {
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Str("extension", ext).
			Str("content_type", prepared.ContentType).
			Msg("run - extension from content type")
	}
	if ext == "" {
		if detected_type := detect_content_type_from_bytes(prepared.ProbeData); detected_type != "" {
			ext = extension_for_content_type(detected_type)
			if ext != "" {
				d.logger.Info().
					Int("task_id", task.ID).
					Int("resource_id", resource.ID).
					Str("extension", ext).
					Str("detected_type", detected_type).
					Msg("run - extension from magic bytes")
			}
		}
	}
	if ext == "" {
		ext = CanonicalExtensionForMIMEType(resource.Kind)
		if ext != "" {
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Str("extension", ext).
				Str("kind", resource.Kind).
				Msg("run - extension derived from resource kind")
		}
	}

	// Persist extension for file rename during finishTask
	if ext != "" && resource_extensions != nil {
		resource_extensions[resource.ID] = ext
	}

	// Step 3: Check for existing segments (resume skips filename processing)
	if ext != "" && resource.ID > 0 {
		segments, err := d.store.LoadSegmentInfo(resource.ID)
		if err != nil {
			return false, fmt.Errorf("failed to load existing download segments: %w", err)
		}
		if len(segments) > 0 {
			persisted_name := strings.TrimSpace(original_db_name)
			if persisted_name != "" && resource.Name != persisted_name {
				d.logger.Warn().
					Int("task_id", task.ID).
					Int("resource_id", resource.ID).
					Str("derived_name", resource.Name).
					Str("persisted_name", persisted_name).
					Msg("discarding derived filename while resuming")
				resource.Name = persisted_name
			}
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Int("segment_count", len(segments)).
				Msg("existing segments found, skipping filename processing (resume)")
			return false, nil
		}
	}

	// Step 4: Check if output file already exists (check .tmp and post-processed final files)
	if ext != "" {
		tmp_ext := ".tmp"
		// Potential filenames to check: temp file .tmp and post-processed final file (with config suffix)
		candidate_names := []string{dir + base_name + tmp_ext}
		if cfg_suffix := get_config_string(task.Config, "suffix"); cfg_suffix != "" && cfg_suffix != tmp_ext {
			candidate_names = append(candidate_names, dir+base_name+cfg_suffix)
		}
		// Also check the actual detected extension (e.g. .jpg from magic bytes), so
		// that duplicate downloads can find files renamed by a prior completed task.
		if ext != "" && ext != tmp_ext {
			candidate_ext := dir + base_name + ext
			already := false
			for _, c := range candidate_names {
				if c == candidate_ext {
					already = true
					break
				}
			}
			if !already {
				candidate_names = append(candidate_names, candidate_ext)
			}
		}

		var current_path string
		var file_exists bool
		for _, try_name := range candidate_names {
			if path, err := d.resolve_resource_path(resource_save_path(task, resource), try_name, endpoint_url); err == nil {
				if info, stat_err := os.Stat(path); stat_err == nil && info.Size() > 0 {
					current_path = path
					file_exists = true
					d.logger.Info().
						Int("task_id", task.ID).
						Int("resource_id", resource.ID).
						Str("file_path", current_path).
						Msg("existing output file detected")
					break
				}
			}
		}

		if file_exists {
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Str("file_path", current_path).
				Interface("config", task_config_for_log(task.Config)).
				Msg("run - file exists with config")
			is_dup := get_config_bool(task.Config, "duplicate")
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Bool("duplicate", is_dup).
				Msg("run - duplicate config parsed")
			// duplicate=true: when temp file exists, auto-append numeric suffix (1), (2), ...
			if is_dup {
				new_name := d.find_next_duplicate_name(task, resource, current_path, dir, base_name, tmp_ext)
				d.logger.Info().
					Int("task_id", task.ID).
					Int("resource_id", resource.ID).
					Str("existing_path", current_path).
					Str("new_name", new_name).
					Msg("file exists, duplicate enabled")
				resource.Name = new_name
				// Persist temp filename to DB; final extension written by finishTask
				if _, err := d.persist_resource_name(task, resource, new_name, original_db_name, "duplicate"); err != nil {
					d.logger.Warn().
						Int("task_id", task.ID).
						Int("resource_id", resource.ID).
						Err(err).
						Msg("failed to update resource name")
				}
				return false, nil
			}
			// duplicate=false: file exists, skip download but update DB resource name for consistency
			resource.Name = dir + base_name + tmp_ext
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Str("existing_path", current_path).
				Str("old_db_name", original_db_name).
				Str("new_db_name", resource.Name).
				Msg("file exists, duplicate disabled, resource name persisted to DB")
			if _, err := d.persist_resource_name(task, resource, resource.Name, original_db_name, "overwrite"); err != nil {
				d.logger.Warn().
					Int("task_id", task.ID).
					Int("resource_id", resource.ID).
					Err(err).
					Msg("failed to update resource name")
			}
			return false, nil
		}
	}

	// Step 5: If no extension, abandon filename processing
	if ext == "" {
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Msg("cannot determine extension, skipping filename processing")
		return false, nil
	}

	// Step 6: Sanitize and truncate base filename (keep directory portion unchanged)
	fp := NewFilenameProcessor("", nil)
	clean_base, err := fp.SanitizeFilename(base_name)
	if err != nil {
		return false, fmt.Errorf("failed to sanitize filename: %w", err)
	}
	d.logger.Info().
		Int("task_id", task.ID).
		Int("resource_id", resource.ID).
		Str("old_name", base_name).
		Str("clean_name", clean_base).
		Msg("filename sanitized")

	// Truncate overly long filenames (235 byte limit must include .tmp extension)
	tmp_ext := ".tmp"
	max_base_len := fp.max_name_length - len(tmp_ext)
	if max_base_len > 0 && len(clean_base) > max_base_len {
		truncated := fp.truncate_string(clean_base, max_base_len)
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Int("old_len", len(clean_base)).
			Int("new_len", len(truncated)).
			Msg("filename truncated due to length")
		clean_base = truncated
	}
	if clean_base == "" {
		return false, fmt.Errorf("filename contains only invalid characters")
	}

	// Step 7: Reconstruct full temp file path (.tmp suffix, final extension written by finishTask)
	resource_name := dir + clean_base + tmp_ext
	d.logger.Info().
		Int("task_id", task.ID).
		Int("resource_id", resource.ID).
		Str("resource_name", resource_name).
		Str("base_name", clean_base).
		Str("tmp_ext", tmp_ext).
		Str("dir", dir).
		Msg("final temp output filename")

	// Step 8: Compare with original DB name; skip DB update if unchanged
	if resource_name == original_db_name {
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Msg("filename matches DB, skipping DB update")
		resource.Name = resource_name
		return false, nil
	}

	// Step 9: Update temp resource name in database
	resource.Name = resource_name
	if updated, err := d.persist_resource_name(task, resource, resource_name, original_db_name, "new"); err != nil {
		return false, err
	} else {
		return updated, nil
	}
}

// CanonicalExtensionForMIMEType maps a MIME type through the application's
// explicit one-to-one table. It never guesses from the operating-system MIME
// registry, where one MIME type may have multiple extensions.
func CanonicalExtensionForMIMEType(content_type string) string {
	media_type := canonical_mime_type(content_type)
	return content_type_ext_map[media_type]
}

// MIMETypeForExtension reverse-maps a file extension to the canonical MIME type.
func MIMETypeForExtension(ext string) string {
	for mime_type, canonical_ext := range content_type_ext_map {
		if canonical_ext == ext {
			return mime_type
		}
	}
	return ""
}

func canonical_mime_type(content_type string) string {
	media_type, _, err := mime.ParseMediaType(content_type)
	if err != nil {
		return ""
	}
	media_type = strings.ToLower(strings.TrimSpace(media_type))
	if _, exists := content_type_ext_map[media_type]; !exists {
		return ""
	}
	return media_type
}

func prepared_target_kind(prepared PreparedResource) string {
	if media_type := canonical_mime_type(prepared.ContentType); media_type != "" {
		return media_type
	}
	if detected_type := detect_content_type_from_bytes(prepared.ProbeData); detected_type != "" {
		if media_type := canonical_mime_type(detected_type); media_type != "" {
			return media_type
		}
	}
	return ""
}

func extension_for_content_type(content_type string) string {
	if ext := CanonicalExtensionForMIMEType(content_type); ext != "" {
		return ext
	}
	media_type, _, err := mime.ParseMediaType(content_type)
	if err != nil || media_type == "application/octet-stream" {
		return ""
	}
	exts, err := mime.ExtensionsByType(media_type)
	if err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ""
}

// contentTypeExtMap is a precise MIME type -> extension mapping.
// Takes priority over mime.ExtensionsByType for special cases (e.g., .jpg vs .jpe).
var content_type_ext_map = map[string]string{
	"image/jpeg":       ".jpg",
	"image/png":        ".png",
	"image/gif":        ".gif",
	"image/webp":       ".webp",
	"image/avif":       ".avif",
	"video/mp4":        ".mp4",
	"video/webm":       ".webm",
	"video/quicktime":  ".mov",
	"video/x-msvideo":  ".avi",
	"video/x-matroska": ".mkv",
	"audio/mpeg":       ".mp3",
	"audio/mp4":        ".m4a",
	"audio/aac":        ".aac",
	"audio/ogg":        ".ogg",
	"audio/wav":        ".wav",
	"audio/flac":       ".flac",
	"text/html":        ".html",
	"text/plain":       ".txt",
	"text/css":         ".css",
	"text/csv":         ".csv",
	"text/markdown":    ".md",
	"application/json": ".json",
	"application/xml":  ".xml",
	"application/pdf":  ".pdf",
	"application/zip":  ".zip",
	"image/svg+xml":    ".svg",
	"image/bmp":        ".bmp",
	"image/tiff":       ".tiff",
	"video/mp2t":       ".ts",
	"video/x-flv":      ".flv",
}

// detectContentTypeFromBytes detects file type via magic bytes.
// Returns a MIME type string, or empty string if unrecognized.
// Used as a supplementary detection method when Content-Type header is absent.
func detect_content_type_from_bytes(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	switch {
	// PNG
	case len(data) >= 4 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G':
		return "image/png"
	// JPEG
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	// GIF87a / GIF89a
	case len(data) >= 4 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' && data[3] == '8':
		return "image/gif"
	// WebP
	case len(data) >= 12 && data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P':
		return "image/webp"
	// BMP
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return "image/bmp"
	// MP4 / M4A (ftyp box at offset 4)
	case len(data) >= 12 && string(data[4:8]) == "ftyp":
		return "video/mp4"
	// MKV / WebM (EBML header)
	case len(data) >= 4 && data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3:
		return "video/x-matroska"
	// MP3 (ID3 tag or MPEG sync)
	case len(data) >= 3 && data[0] == 'I' && data[1] == 'D' && data[2] == '3':
		return "audio/mpeg"
	case len(data) >= 2 && data[0] == 0xFF && (data[1]&0xE0) == 0xE0:
		return "audio/mpeg"
	// OGG
	case len(data) >= 4 && data[0] == 'O' && data[1] == 'g' && data[2] == 'g' && data[3] == 'S':
		return "audio/ogg"
	// FLAC
	case len(data) >= 4 && data[0] == 'f' && data[1] == 'L' && data[2] == 'a' && data[3] == 'C':
		return "audio/flac"
	// WAV
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE":
		return "audio/wav"
	// PDF
	case len(data) >= 5 && string(data[0:5]) == "%PDF-":
		return "application/pdf"
	// ZIP (PK\x03\x04)
	case len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04:
		return "application/zip"
	// RAR
	case len(data) >= 7 && data[0] == 'R' && data[1] == 'a' && data[2] == 'r' && data[3] == '!' && data[4] == 0x1A && data[5] == 0x07:
		return "application/x-rar-compressed"
	// 7z
	case len(data) >= 6 && data[0] == '7' && data[1] == 'z' && data[2] == 0xBC && data[3] == 0xAF && data[4] == 0x27 && data[5] == 0x1C:
		return "application/x-7z-compressed"
	// GZIP
	case len(data) >= 2 && data[0] == 0x1F && data[1] == 0x8B:
		return "application/gzip"
	default:
		return ""
	}
}

func (d *HermesEngine) update_resource_size(task_id, resource_id int, size int64) error {
	if store, ok := d.store.(ResourceStore); ok {
		return store.UpdateResourceSizeByID(resource_id, size)
	}
	return d.store.UpdateResourceSize(task_id, size)
}

func (d *HermesEngine) update_resource_progress(task_id, resource_id int, downloaded, speed int64) error {
	if store, ok := d.store.(ResourceStore); ok {
		return store.UpdateResourceProgress(resource_id, downloaded, speed)
	}
	return d.store.UpdateProgress(task_id, downloaded, speed)
}

func prepare_with_retry(ctx context.Context, driver ProtocolDriver, endpoint Endpoint) (PreparedResource, error) {
	var last_err error
	for attempt := 0; attempt < max_read_attempts; attempt++ {
		prepared, err := driver.Prepare(ctx, endpoint)
		if err == nil {
			return prepared, nil
		}
		if errors.Is(err, context.Canceled) {
			return PreparedResource{}, err
		}
		if ctx.Err() != nil {
			return PreparedResource{}, context.Cause(ctx)
		}
		last_err = err
		if attempt < max_read_attempts-1 && !wait_for_retry(ctx, attempt) {
			return PreparedResource{}, context.Cause(ctx)
		}
	}
	return PreparedResource{}, last_err
}

func (d *HermesEngine) apply_filename_template(task *TaskJob, resource *ResourceJob, endpoint_url string, meta map[string]string) string {
	return d.apply_job_filename_template(task, resource, task.FilenameTemplate, resource.Name, endpoint_url, meta)
}

func (d *HermesEngine) apply_job_filename_template(task *TaskJob, resource *ResourceJob, template, name, endpoint_url string, meta map[string]string) string {
	task_id := 0
	resource_id := 0
	if task != nil {
		task_id = task.ID
	}
	if resource != nil {
		resource_id = resource.ID
	}
	result, err := apply_filename_template_value(template, name, endpoint_url, meta, task_id, resource_id)
	if err != nil {
		d.logger.Warn().Err(err).Msg("filename template error")
		return ""
	}
	return result
}

func apply_filename_template_value(template, name, endpoint_url string, meta map[string]string, task_id, resource_id int) (string, error) {
	// If template contains {{var}} syntax, use shared template var replacement
	if strings.Contains(template, "{{") {
		return clean_path_separators(util.ReplaceTemplateVars(template, meta)), nil
	}

	// Fall through to JS VM evaluation for expression-based templates
	url_basename := ""
	if u, err := url.Parse(endpoint_url); err == nil {
		url_basename = filepath.Base(u.Path)
	}

	vm := goja.New()
	vm.Set("name", name)
	vm.Set("task_id", task_id)
	vm.Set("resource_id", resource_id)
	vm.Set("url_basename", url_basename)

	vm.Set("formatTime", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		return vm.ToValue(time.Now().Format(call.Argument(0).String()))
	})

	vm.Set("padStart", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return call.Arguments[0]
		}
		s := call.Argument(0).String()
		length := int(call.Argument(1).ToInteger())
		pad := "0"
		if len(call.Arguments) >= 3 {
			pad = call.Argument(2).String()
		}
		for len(s) < length {
			s = pad + s
		}
		return vm.ToValue(s)
	})

	result, err := vm.RunString(template)
	if err != nil {
		return "", err
	}

	return clean_path_separators(result.String()), nil
}

// cleanPathSeparators trims whitespace around each / separator in a path string,
// so that e.g. "AuthorName / VideoTitle" becomes "AuthorName/VideoTitle".
// Leading/trailing whitespace is also trimmed.
func clean_path_separators(s string) string {
	parts := strings.Split(s, "/")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Trim(strings.Join(parts, "/"), "/")
}

func (d *HermesEngine) endpoint_candidates(resource_endpoints []Endpoint) ([]endpoint_candidate, error) {
	endpoints := append([]Endpoint(nil), resource_endpoints...)
	if len(endpoints) == 0 {
		return nil, errors.New("task has no available download endpoints")
	}
	sort.SliceStable(endpoints, func(i, j int) bool { return endpoints[i].Priority < endpoints[j].Priority })

	candidates := make([]endpoint_candidate, 0, len(endpoints))
	for _, endpoint := range endpoints {
		protocol := strings.ToLower(strings.TrimSpace(endpoint.Protocol))
		if protocol == "" {
			parsed, err := url.Parse(endpoint.URL)
			if err == nil {
				protocol = strings.ToLower(parsed.Scheme)
			}
		}
		d.mu.Lock()
		driver := d.drivers[protocol]
		d.mu.Unlock()
		candidates = append(candidates, endpoint_candidate{endpoint: endpoint, protocol: protocol, driver: driver})
	}
	return candidates, nil
}

// probeConcurrency caps the number of concurrent Prepare requests during the
// upfront size-discovery phase. This balances latency (parallel probes) against
// server-side rate-limiting (aggressive concurrency may trigger CDN throttling).
const probe_concurrency = 5

// ensureResourceSizes probes each resource to determine its size before the
// download loop starts. When all resource sizes are known upfront, the API can
// compute correct task-level aggregate progress (sum of all resource segments
// divided by sum of all resource sizes), avoiding the 100%→partial→100%
// oscillation that occurs when sizes are discovered one resource at a time.
// Failures are non-fatal; the download loop will retry Prepares as needed.
// Returns a map of resourceID→size for resources whose size was successfully
// determined.
func (d *HermesEngine) ensure_resource_sizes(ctx context.Context, task_id int, resources []ResourceJob) map[int]int64 {
	if len(resources) == 0 {
		return nil
	}
	if len(resources) == 1 {
		// Single resource: no benefit from parallelism overhead.
		return d.probe_resource_sizes_seq(ctx, task_id, resources)
	}
	return d.probe_resource_sizes_parallel(ctx, task_id, resources)
}

func (d *HermesEngine) probe_resource_sizes_seq(ctx context.Context, task_id int, resources []ResourceJob) map[int]int64 {
	var mu sync.Mutex
	sizes := make(map[int]int64)
	for i := range resources {
		res := &resources[i]
		if err := ctx.Err(); err != nil {
			return sizes
		}
		d.probe_one_resource(ctx, task_id, res, &mu, &sizes)
	}
	return sizes
}

func (d *HermesEngine) probe_resource_sizes_parallel(ctx context.Context, task_id int, resources []ResourceJob) map[int]int64 {
	sizes := make(map[int]int64)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, probe_concurrency)

	for i := range resources {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(res *ResourceJob) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			d.probe_one_resource(ctx, task_id, res, &mu, &sizes)
		}(&resources[i])
	}
	wg.Wait()
	return sizes
}

func (d *HermesEngine) probe_one_resource(ctx context.Context, task_id int, res *ResourceJob, mu *sync.Mutex, sizes *map[int]int64) {
	candidates, err := d.endpoint_candidates(res.Endpoints)
	if err != nil {
		return
	}
	for _, c := range candidates {
		if ctx.Err() != nil {
			return
		}
		if c.driver == nil {
			continue
		}
		prepared, err := c.driver.Prepare(ctx, c.endpoint)
		if err != nil {
			continue
		}
		if prepared.Size > 0 {
			_ = d.update_resource_size(task_id, res.ID, prepared.Size)
			res.Size = prepared.Size
			mu.Lock()
			(*sizes)[res.ID] = prepared.Size
			mu.Unlock()
			return
		}
	}
}

// ResolveOutputPath constructs a resource path under its output container. An
// absolute save_path overrides the engine base path; a relative save_path
// remains relative to the engine base for backwards compatibility.
func ResolveOutputPath(base_path, save_path, name string) string {
	container_path := strings.TrimSpace(save_path)
	if container_path == "" {
		container_path = base_path
	} else if !filepath.IsAbs(container_path) {
		container_path = filepath.Join(base_path, container_path)
	}
	return filepath.Join(container_path, name)
}

func (d *HermesEngine) abs_file_path(save_path, name string) string {
	return ResolveOutputPath(d.cfg.BasePath, save_path, name)
}

func resource_save_path(task *TaskJob, resource *ResourceJob) string {
	if resource != nil && strings.TrimSpace(resource.SavePath) != "" {
		return resource.SavePath
	}
	if task == nil {
		return ""
	}
	return task.SavePath
}

// relLogPath converts absolute absPath to relative (strips basePath prefix).
func (d *HermesEngine) rel_log_path(abs_path string) string {
	if d.cfg.BasePath == "" {
		return abs_path
	}
	rel, err := filepath.Rel(d.cfg.BasePath, abs_path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return abs_path
	}
	return rel
}

// filePathForResource resolves the absolute on-disk path for a task's resource.
func (d *HermesEngine) file_path_for_job_resource(task *TaskJob, resource *ResourceJob, endpoint_url string) (string, error) {
	if task == nil || resource == nil {
		return "", errors.New("task and resource jobs are required")
	}
	return d.resolve_resource_path(resource_save_path(task, resource), resource.UniqueID, endpoint_url)
}

func (d *HermesEngine) resolve_resource_path(save_path, unique_id, endpoint_url string) (string, error) {
	raw_unique_id := strings.TrimSpace(unique_id)
	if raw_unique_id == "" {
		if parsed, err := url.Parse(endpoint_url); err == nil {
			raw_unique_id = filepath.Base(parsed.Path)
		}
	}
	raw_unique_id = filepath.Clean(raw_unique_id)
	raw_unique_id = strings.TrimLeft(raw_unique_id, "/")
	// Strip leading path traversal prefixes (same effect as filepath.Base but preserves subdirectories)
	for strings.HasPrefix(raw_unique_id, "../") {
		raw_unique_id = raw_unique_id[3:]
	}
	// Prevent path traversal attacks
	if raw_unique_id == "" || raw_unique_id == "." || raw_unique_id == ".." || strings.HasPrefix(raw_unique_id, "../") || strings.Contains(raw_unique_id, string(filepath.Separator)+"..") {
		return "", errors.New("unable to determine download filename")
	}

	return d.abs_file_path(save_path, raw_unique_id), nil
}

// taskFilePath is the legacy function kept for backward compatibility.
func task_file_path(info *TaskJob, endpoint_url string) (string, error) {
	if strings.TrimSpace(info.SavePath) == "" {
		return "", errors.New("save path cannot be empty")
	}
	name := strings.TrimSpace(info.Name)
	if name == "" {
		if parsed, err := url.Parse(endpoint_url); err == nil {
			name = filepath.Base(parsed.Path)
		}
	}
	name = filepath.Clean(name)
	name = strings.TrimLeft(name, "/")
	// Strip leading path traversal prefixes (same effect as filepath.Base but preserves subdirectories)
	for strings.HasPrefix(name, "../") {
		name = name[3:]
	}
	// Prevent path traversal attacks
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, string(filepath.Separator)+"..") {
		return "", errors.New("unable to determine download filename")
	}

	save_path := filepath.Clean(info.SavePath)
	return filepath.Join(save_path, name), nil
}

func choose_segment_count(prepared PreparedResource) int {
	if !prepared.SupportsRange || prepared.Size <= minimum_segment_size {
		return 1
	}
	count := int((prepared.Size + minimum_segment_size - 1) / minimum_segment_size)
	max_count := default_segment_count
	// For very large files (≥ 2 GiB), allow more segments so each segment
	// stays near the minimum size and bandwidth saturation improves.
	if prepared.Size >= 2*1024*1024*1024 {
		max_count = 64
	}
	if count > max_count {
		count = max_count
	}
	return count
}

// splitFile divides a file into n non-empty segments, distributing any remainder across the first segments.
func split_file(file_size int64, n int) []SegmentRange {
	if n <= 0 || file_size <= 0 {
		return nil
	}
	if int64(n) > file_size {
		n = int(file_size)
	}
	base_size := file_size / int64(n)
	remainder := file_size % int64(n)
	ranges := make([]SegmentRange, n)
	var offset int64
	for i := 0; i < n; i++ {
		size := base_size
		if int64(i) < remainder {
			size++
		}
		ranges[i] = SegmentRange{Index: i, OffsetStart: offset, OffsetEnd: offset + size - 1, Size: size}
		offset += size
	}
	return ranges
}

func build_resource_meta(extra map[string]string) ResourceMeta {
	meta := make(ResourceMeta, len(extra)+1)
	for key, value := range extra {
		meta[key] = value
	}
	meta["download_at"] = time.Now().Unix()
	return meta
}

// buildTemplateMeta builds a metadata map for {{var}} substitution from resource.Extra.
func build_template_meta(extra map[string]string, current_name string) map[string]string {
	meta := make(map[string]string, len(extra)+2)
	for key, value := range extra {
		meta[key] = value
	}
	meta["download_at"] = time.Now().Format("2006-01-02")
	meta["filename"] = current_name
	return meta
}

// calcSpeed computes download speed (bytes/sec) between two points in time.
func get_config_bool(config map[string]any, key string) bool {
	if v, ok := config[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// getConfigString reads the string value for the specified task config key.
func get_config_string(config map[string]any, key string) string {
	if v, ok := config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// findNextDuplicateName finds the next available numeric suffix filename when a file already exists.
// e.g., if baseName.mp4 exists, try baseName(1).mp4, baseName(2).mp4, ...
func (d *HermesEngine) find_next_duplicate_name(task *TaskJob, resource *ResourceJob, existing_path, dir, base_name, ext string) string {
	tmp_ext := ".tmp"
	for counter := 1; ; counter++ {
		candidate := fmt.Sprintf("%s(%d)%s", base_name, counter, tmp_ext)
		candidate_path := d.abs_file_path(resource_save_path(task, resource), filepath.Join(dir, candidate))
		if _, err := os.Stat(candidate_path); os.IsNotExist(err) {
			return dir + candidate
		}
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Str("file_name", dir+candidate).
			Msg("duplicate file name exists, incrementing counter")
	}
}

// resolveDuplicateFilename appends (1), (2), ... to baseName when the final
// filename already exists on disk. ext is appended after the duplicate suffix.
func (d *HermesEngine) resolve_duplicate_filename(save_path string, directories []string, base_name, ext string) string {
	for counter := 0; ; counter++ {
		candidate_name := final_output_name(base_name, ext)
		if counter > 0 {
			candidate_name = final_output_name_with_suffix(base_name, ext, fmt.Sprintf("(%d)", counter))
		}
		candidate := join_output_path(directories, candidate_name)
		candidate_path := d.abs_file_path(save_path, candidate)
		if _, err := os.Stat(candidate_path); os.IsNotExist(err) {
			return candidate
		}
	}
}

// persistResourceName updates the resource name in the database. 'reason' is used in logs to annotate the trigger scenario.
func (d *HermesEngine) persist_resource_name(task *TaskJob, resource *ResourceJob, resource_name, original_db_name, reason string) (bool, error) {
	if resource_name == original_db_name {
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Str("reason", reason).
			Msg("filename matches DB, skipping DB update")
		return false, nil
	}
	update := OutputNameUpdate{
		TaskID:       task.ID,
		ResourceID:   resource.ID,
		ResourceName: resource_name,
	}
	d.logger.Info().
		Int("task_id", task.ID).
		Int("resource_id", resource.ID).
		Str("reason", reason).
		Int("update_task_id", update.TaskID).
		Int("update_resource_id", update.ResourceID).
		Str("update_resource_name", update.ResourceName).
		Msg("updating resource name in DB")
	if store, ok := d.store.(OutputNameStore); ok {
		if err := store.UpdateOutputName(update); err != nil {
			return false, fmt.Errorf("failed to update download filename in database: %w", err)
		}
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Str("old_name", original_db_name).
			Str("new_name", resource_name).
			Msg("resource name updated in DB")
	} else {
		d.logger.Warn().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Msg("store does not implement OutputNameStore, skipping DB update")
	}
	return true, nil
}
