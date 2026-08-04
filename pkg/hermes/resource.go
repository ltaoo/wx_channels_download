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
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"

	"wx_channel/pkg/util"
)

func (d *HermesEngine) downloadResource(ctx context.Context, task *TaskJob, resource *ResourceJob) (string, error) {
	if task == nil {
		return "", errors.New("task is nil")
	}
	if resource == nil {
		return "", errors.New("resource is nil")
	}
	if strings.TrimSpace(resource.UniqueID) == "" {
		return "", errors.New("resource unique ID is required")
	}
	candidates, err := d.endpointCandidates(resource.Endpoints)
	if err != nil {
		return "", err
	}

	var endpointErrors []string
	var filePath string
	var expectedSize int64
	for _, candidate := range candidates {
		if err := context.Cause(ctx); err != nil {
			return "", err
		}
		if candidate.driver == nil {
			endpointErrors = append(endpointErrors, fmt.Sprintf("%s: protocol driver is not registered", candidate.protocol))
			continue
		}

		prepared, prepareErr := prepareWithRetry(ctx, candidate.driver, candidate.endpoint)
		if prepareErr != nil {
			if errors.Is(prepareErr, context.Canceled) {
				return "", prepareErr
			}
			endpointErrors = append(endpointErrors, fmt.Sprintf("%s: %v", candidate.protocol, prepareErr))
			continue
		}
		if prepared.Size < 0 {
			prepared.Size = 0
		}
		// Kind is normalized to the canonical MIME value persisted after
		// filename finalization. Extension is derived from Kind at finalize time.
		resource.Kind = preparedTargetKind(prepared)
		if expectedSize > 0 && prepared.Size > 0 && prepared.Size != expectedSize {
			endpointErrors = append(endpointErrors, fmt.Sprintf("%s: mirror resource size mismatch", candidate.protocol))
			continue
		}
		if expectedSize == 0 && prepared.Size > 0 {
			expectedSize = prepared.Size
		}
		if prepared.Size > 0 {
			resource.Size = prepared.Size
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Int64("resource_size", prepared.Size).
				Str("resource_size_readable", formatSize(prepared.Size)).
				Msg("run - updating resource size from prepared endpoint")
			if err := d.updateResourceSize(task.ID, resource.ID, prepared.Size); err != nil {
				return "", fmt.Errorf("failed to update resource size: %w", err)
			}
			d.updateTrackerSize(task.ID, resource.ID, prepared.Size)
		}

		// Once segment records exist, resource.Name is the canonical path that was
		// persisted when the download first started. Reapplying filename templates
		// or hooks after a restart can produce a different path, making the existing
		// .part file appear missing and causing downloadSegments to reset every
		// persisted offset to zero.
		existingSegments, segmentErr := d.store.LoadSegmentInfo(resource.ID)
		if segmentErr != nil {
			return "", fmt.Errorf("failed to load existing download segments: %w", segmentErr)
		}
		resuming := len(existingSegments) > 0
		if resuming {
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Int("segment_count", len(existingSegments)).
				Str("resource_name", resource.Name).
				Msg("run - existing segments found, preserving persisted filename")
		}
		filePath, err = d.filePathForJobResource(task, resource, candidate.endpoint.URL)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return "", fmt.Errorf("failed to create download directory: %w", err)
		}

		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Str("endpoint", candidate.endpoint.URL).
			Str("file_path", d.relLogPath(filePath)).
			Msg("run - starting resource download")

		segmentCount := chooseSegmentCount(prepared)
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Bool("segmented", segmentCount > 1).
			Int("segment_count", segmentCount).
			Int64("segment_size", minimumSegmentSize).
			Msg("run - download mode selected")
		downloadStart := time.Now()
		if segmentCount > 1 {
			err = d.downloadSegments(ctx, candidate.driver, candidate.endpoint, filePath, task, resource, prepared, segmentCount)
		} else {
			err = d.downloadFile(ctx, candidate.driver, candidate.endpoint, filePath, task, resource, prepared)
		}
		if err == nil {
			resource.FilePath = filePath
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Str("file_path", d.relLogPath(filePath)).
				Dur("elapsed", time.Since(downloadStart)).
				Msg("run - data transfer completed")
			if prepared.Size <= 0 {
				if fileInfo, statErr := os.Stat(filePath); statErr == nil {
					resource.Size = fileInfo.Size()
					if err := d.updateResourceSize(task.ID, resource.ID, fileInfo.Size()); err != nil {
						return "", fmt.Errorf("failed to update final resource size: %w", err)
					}
					d.updateTrackerSize(task.ID, resource.ID, fileInfo.Size())
				}
			}
			if err := d.finishDownloadResource(task.ID, resource.ID); err != nil {
				return "", err
			}
			return filePath, nil
		}
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return "", context.Cause(ctx)
		}
		endpointErrors = append(endpointErrors, fmt.Sprintf("%s: %v", candidate.protocol, err))
		d.logger.Warn().
			Int("endpoint_id", candidate.endpoint.ID).
			Str("endpoint", candidate.endpoint.URL).
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Err(err).
			Msg("run - download resource from endpoint failed, trying next mirror")
	}
	return "", fmt.Errorf("all download endpoints are unavailable: %s", strings.Join(endpointErrors, "; "))
}

func (d *HermesEngine) finishDownloadResource(taskID, resourceID int) error {
	store, ok := d.store.(ResourceStore)
	if !ok {
		return nil
	}

	d.logger.Info().
		Int("task_id", taskID).
		Int("resource_id", resourceID).
		Msg("persisting resource state")
	if err := store.FinishResource(resourceID); err != nil {
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
func (d *HermesEngine) processOutputFilename(task *TaskJob, resource *ResourceJob, endpointURL string, prepared PreparedResource, originalDBName string, resourceExtensions map[int]string) (bool, error) {
	if task == nil || resource == nil || resource.ID <= 0 {
		return false, nil
	}

	rawUniqueId := strings.TrimSpace(resource.UniqueID)
	if rawUniqueId == "" {
		return false, nil
	}

	// Step 1: Separate directory and base filename (unique ID is a plain filename without extension)
	dir, baseName := filepath.Split(rawUniqueId)
	d.logger.Info().
		Int("task_id", task.ID).
		Int("resource_id", resource.ID).
		Str("raw_unique_id", rawUniqueId).
		Str("dir", dir).
		Str("base_name", baseName).
		Msg("run - output filename processing started")

	// Step 2: Determine extension
	// Priority: Content-Type -> magic bytes -> user-specified fallback suffix
	ext := extensionForContentType(prepared.ContentType)
	if ext != "" {
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Str("extension", ext).
			Str("content_type", prepared.ContentType).
			Msg("run - extension from content type")
	}
	if ext == "" {
		if detectedType := detectContentTypeFromBytes(prepared.ProbeData); detectedType != "" {
			ext = extensionForContentType(detectedType)
			if ext != "" {
				d.logger.Info().
					Int("task_id", task.ID).
					Int("resource_id", resource.ID).
					Str("extension", ext).
					Str("detected_type", detectedType).
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
	if ext != "" && resourceExtensions != nil {
		resourceExtensions[resource.ID] = ext
	}

	// Step 3: Check for existing segments (resume skips filename processing)
	if ext != "" && resource.ID > 0 {
		segments, err := d.store.LoadSegmentInfo(resource.ID)
		if err != nil {
			return false, fmt.Errorf("failed to load existing download segments: %w", err)
		}
		if len(segments) > 0 {
			persistedName := strings.TrimSpace(originalDBName)
			if persistedName != "" && resource.Name != persistedName {
				d.logger.Warn().
					Int("task_id", task.ID).
					Int("resource_id", resource.ID).
					Str("derived_name", resource.Name).
					Str("persisted_name", persistedName).
					Msg("discarding derived filename while resuming")
				resource.Name = persistedName
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
		tmpExt := ".tmp"
		// Potential filenames to check: temp file .tmp and post-processed final file (with config suffix)
		candidateNames := []string{dir + baseName + tmpExt}
		if cfgSuffix := getConfigString(task.Config, "suffix"); cfgSuffix != "" && cfgSuffix != tmpExt {
			candidateNames = append(candidateNames, dir+baseName+cfgSuffix)
		}
		// Also check the actual detected extension (e.g. .jpg from magic bytes), so
		// that duplicate downloads can find files renamed by a prior completed task.
		if ext != "" && ext != tmpExt {
			candidateExt := dir + baseName + ext
			already := false
			for _, c := range candidateNames {
				if c == candidateExt {
					already = true
					break
				}
			}
			if !already {
				candidateNames = append(candidateNames, candidateExt)
			}
		}

		var currentPath string
		var fileExists bool
		for _, tryName := range candidateNames {
			if path, err := d.resolveResourcePath(task.SavePath, tryName, endpointURL); err == nil {
				if info, statErr := os.Stat(path); statErr == nil && info.Size() > 0 {
					currentPath = path
					fileExists = true
					d.logger.Info().
						Int("task_id", task.ID).
						Int("resource_id", resource.ID).
						Str("file_path", currentPath).
						Msg("existing output file detected")
					break
				}
			}
		}

		if fileExists {
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Str("file_path", currentPath).
				Interface("config", task.Config).
				Msg("run - file exists with config")
			isDup := getConfigBool(task.Config, "duplicate")
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Bool("duplicate", isDup).
				Msg("run - duplicate config parsed")
			// duplicate=true: when temp file exists, auto-append numeric suffix (1), (2), ...
			if isDup {
				newName := d.findNextDuplicateName(task, resource, currentPath, dir, baseName, tmpExt)
				d.logger.Info().
					Int("task_id", task.ID).
					Int("resource_id", resource.ID).
					Str("existing_path", currentPath).
					Str("new_name", newName).
					Msg("file exists, duplicate enabled")
				resource.Name = newName
				// Persist temp filename to DB; final extension written by finishTask
				if _, err := d.persistResourceName(task, resource, newName, originalDBName, "duplicate"); err != nil {
					d.logger.Warn().
						Int("task_id", task.ID).
						Int("resource_id", resource.ID).
						Err(err).
						Msg("failed to update resource name")
				}
				return false, nil
			}
			// duplicate=false: file exists, skip download but update DB resource name for consistency
			resource.Name = dir + baseName + tmpExt
			d.logger.Info().
				Int("task_id", task.ID).
				Int("resource_id", resource.ID).
				Str("existing_path", currentPath).
				Str("old_db_name", originalDBName).
				Str("new_db_name", resource.Name).
				Msg("file exists, duplicate disabled, resource name persisted to DB")
			if _, err := d.persistResourceName(task, resource, resource.Name, originalDBName, "overwrite"); err != nil {
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
	cleanBase, err := fp.SanitizeFilename(baseName)
	if err != nil {
		return false, fmt.Errorf("failed to sanitize filename: %w", err)
	}
	d.logger.Info().
		Int("task_id", task.ID).
		Int("resource_id", resource.ID).
		Str("old_name", baseName).
		Str("clean_name", cleanBase).
		Msg("filename sanitized")

	// Truncate overly long filenames (235 byte limit must include .tmp extension)
	tmpExt := ".tmp"
	maxBaseLen := fp.maxNameLength - len(tmpExt)
	if maxBaseLen > 0 && len(cleanBase) > maxBaseLen {
		truncated := fp.truncateString(cleanBase, maxBaseLen)
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Int("old_len", len(cleanBase)).
			Int("new_len", len(truncated)).
			Msg("filename truncated due to length")
		cleanBase = truncated
	}
	if cleanBase == "" {
		return false, fmt.Errorf("filename contains only invalid characters")
	}

	// Step 7: Reconstruct full temp file path (.tmp suffix, final extension written by finishTask)
	resourceName := dir + cleanBase + tmpExt
	d.logger.Info().
		Int("task_id", task.ID).
		Int("resource_id", resource.ID).
		Str("resource_name", resourceName).
		Str("base_name", cleanBase).
		Str("tmp_ext", tmpExt).
		Str("dir", dir).
		Msg("final temp output filename")

	// Step 8: Compare with original DB name; skip DB update if unchanged
	if resourceName == originalDBName {
		d.logger.Info().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Msg("filename matches DB, skipping DB update")
		resource.Name = resourceName
		return false, nil
	}

	// Step 9: Update temp resource name in database
	resource.Name = resourceName
	if updated, err := d.persistResourceName(task, resource, resourceName, originalDBName, "new"); err != nil {
		return false, err
	} else {
		return updated, nil
	}
}

// CanonicalExtensionForMIMEType maps a MIME type through the application's
// explicit one-to-one table. It never guesses from the operating-system MIME
// registry, where one MIME type may have multiple extensions.
func CanonicalExtensionForMIMEType(contentType string) string {
	mediaType := canonicalMIMEType(contentType)
	return contentTypeExtMap[mediaType]
}

// MIMETypeForExtension reverse-maps a file extension to the canonical MIME type.
func MIMETypeForExtension(ext string) string {
	for mimeType, canonicalExt := range contentTypeExtMap {
		if canonicalExt == ext {
			return mimeType
		}
	}
	return ""
}

func canonicalMIMEType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if _, exists := contentTypeExtMap[mediaType]; !exists {
		return ""
	}
	return mediaType
}

func preparedTargetKind(prepared PreparedResource) string {
	if mediaType := canonicalMIMEType(prepared.ContentType); mediaType != "" {
		return mediaType
	}
	if detectedType := detectContentTypeFromBytes(prepared.ProbeData); detectedType != "" {
		if mediaType := canonicalMIMEType(detectedType); mediaType != "" {
			return mediaType
		}
	}
	return ""
}

func extensionForContentType(contentType string) string {
	if ext := CanonicalExtensionForMIMEType(contentType); ext != "" {
		return ext
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType == "application/octet-stream" {
		return ""
	}
	exts, err := mime.ExtensionsByType(mediaType)
	if err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ""
}

// contentTypeExtMap is a precise MIME type -> extension mapping.
// Takes priority over mime.ExtensionsByType for special cases (e.g., .jpg vs .jpe).
var contentTypeExtMap = map[string]string{
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
func detectContentTypeFromBytes(data []byte) string {
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

func (d *HermesEngine) updateResourceSize(taskID, resourceID int, size int64) error {
	if store, ok := d.store.(ResourceStore); ok {
		return store.UpdateResourceSizeByID(resourceID, size)
	}
	return d.store.UpdateResourceSize(taskID, size)
}

func (d *HermesEngine) updateResourceProgress(taskID, resourceID int, downloaded, speed int64) error {
	if store, ok := d.store.(ResourceStore); ok {
		return store.UpdateResourceProgress(resourceID, downloaded, speed)
	}
	return d.store.UpdateProgress(taskID, downloaded, speed)
}

func prepareWithRetry(ctx context.Context, driver ProtocolDriver, endpoint Endpoint) (PreparedResource, error) {
	var lastErr error
	for attempt := 0; attempt < maxReadAttempts; attempt++ {
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
		lastErr = err
		if attempt < maxReadAttempts-1 && !waitForRetry(ctx, attempt) {
			return PreparedResource{}, context.Cause(ctx)
		}
	}
	return PreparedResource{}, lastErr
}

func (d *HermesEngine) applyFilenameTemplate(task *TaskJob, resource *ResourceJob, endpointURL string, meta map[string]string) string {
	return d.applyJobFilenameTemplate(task, resource, task.FilenameTemplate, resource.Name, endpointURL, meta)
}

func (d *HermesEngine) applyJobFilenameTemplate(task *TaskJob, resource *ResourceJob, template, name, endpointURL string, meta map[string]string) string {
	// If template contains {{var}} syntax, use shared template var replacement
	if strings.Contains(template, "{{") {
		return cleanPathSeparators(util.ReplaceTemplateVars(template, meta))
	}

	// Fall through to JS VM evaluation for expression-based templates
	urlBasename := ""
	if u, err := url.Parse(endpointURL); err == nil {
		urlBasename = filepath.Base(u.Path)
	}

	vm := goja.New()
	vm.Set("name", name)
	vm.Set("task_id", task.ID)
	vm.Set("resource_id", resource.ID)
	vm.Set("url_basename", urlBasename)

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
		d.logger.Warn().Err(err).Msg("filename template error")
		return ""
	}

	return cleanPathSeparators(result.String())
}

// cleanPathSeparators trims whitespace around each / separator in a path string,
// so that e.g. "AuthorName / VideoTitle" becomes "AuthorName/VideoTitle".
// Leading/trailing whitespace is also trimmed.
func cleanPathSeparators(s string) string {
	parts := strings.Split(s, "/")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Trim(strings.Join(parts, "/"), "/")
}

func (d *HermesEngine) endpointCandidates(resourceEndpoints []Endpoint) ([]endpointCandidate, error) {
	endpoints := append([]Endpoint(nil), resourceEndpoints...)
	if len(endpoints) == 0 {
		return nil, errors.New("task has no available download endpoints")
	}
	sort.SliceStable(endpoints, func(i, j int) bool { return endpoints[i].Priority < endpoints[j].Priority })

	candidates := make([]endpointCandidate, 0, len(endpoints))
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
		candidates = append(candidates, endpointCandidate{endpoint: endpoint, protocol: protocol, driver: driver})
	}
	return candidates, nil
}

// ensureResourceSizes probes each resource to determine its size before the
// download loop starts. When all resource sizes are known upfront, the API can
// compute correct task-level aggregate progress (sum of all resource segments
// divided by sum of all resource sizes), avoiding the 100%→partial→100%
// oscillation that occurs when sizes are discovered one resource at a time.
// Failures are non-fatal; the download loop will retry Prepares as needed.
// Returns a map of resourceID→size for resources whose size was successfully
// determined.
func (d *HermesEngine) ensureResourceSizes(ctx context.Context, taskID int, resources []ResourceJob) map[int]int64 {
	sizes := make(map[int]int64)
	for i := range resources {
		res := &resources[i]
		candidates, err := d.endpointCandidates(res.Endpoints)
		if err != nil {
			continue
		}
		for _, c := range candidates {
			if ctx.Err() != nil {
				return sizes
			}
			if c.driver == nil {
				continue
			}
			prepared, err := c.driver.Prepare(ctx, c.endpoint)
			if err != nil {
				continue
			}
			if prepared.Size > 0 {
				_ = d.updateResourceSize(taskID, res.ID, prepared.Size)
				res.Size = prepared.Size
				sizes[res.ID] = prepared.Size
				break
			}
		}
	}
	return sizes
}

// absFilePath constructs absolute path: basePath + savePath + name.
func (d *HermesEngine) absFilePath(savePath, name string) string {
	return filepath.Join(d.cfg.BasePath, savePath, name)
}

// relLogPath converts absolute absPath to relative (strips basePath prefix).
func (d *HermesEngine) relLogPath(absPath string) string {
	if d.cfg.BasePath == "" {
		return absPath
	}
	rel, err := filepath.Rel(d.cfg.BasePath, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return absPath
	}
	return rel
}

// filePathForResource resolves the absolute on-disk path for a task's resource.
func (d *HermesEngine) filePathForJobResource(task *TaskJob, resource *ResourceJob, endpointURL string) (string, error) {
	if task == nil || resource == nil {
		return "", errors.New("task and resource jobs are required")
	}
	return d.resolveResourcePath(task.SavePath, resource.UniqueID, endpointURL)
}

func (d *HermesEngine) resolveResourcePath(savePath, uniqueID, endpointURL string) (string, error) {
	rawUniqueId := strings.TrimSpace(uniqueID)
	if rawUniqueId == "" {
		if parsed, err := url.Parse(endpointURL); err == nil {
			rawUniqueId = filepath.Base(parsed.Path)
		}
	}
	rawUniqueId = filepath.Clean(rawUniqueId)
	rawUniqueId = strings.TrimLeft(rawUniqueId, "/")
	// Strip leading path traversal prefixes (same effect as filepath.Base but preserves subdirectories)
	for strings.HasPrefix(rawUniqueId, "../") {
		rawUniqueId = rawUniqueId[3:]
	}
	// Prevent path traversal attacks
	if rawUniqueId == "" || rawUniqueId == "." || rawUniqueId == ".." || strings.HasPrefix(rawUniqueId, "../") || strings.Contains(rawUniqueId, string(filepath.Separator)+"..") {
		return "", errors.New("unable to determine download filename")
	}

	return d.absFilePath(savePath, rawUniqueId), nil
}

// taskFilePath is the legacy function kept for backward compatibility.
func taskFilePath(info *TaskJob, endpointURL string) (string, error) {
	if strings.TrimSpace(info.SavePath) == "" {
		return "", errors.New("save path cannot be empty")
	}
	name := strings.TrimSpace(info.Name)
	if name == "" {
		if parsed, err := url.Parse(endpointURL); err == nil {
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

	savePath := filepath.Clean(info.SavePath)
	return filepath.Join(savePath, name), nil
}

func chooseSegmentCount(prepared PreparedResource) int {
	if !prepared.SupportsRange || prepared.Size <= minimumSegmentSize {
		return 1
	}
	count := int((prepared.Size + minimumSegmentSize - 1) / minimumSegmentSize)
	if count > defaultSegmentCount {
		count = defaultSegmentCount
	}
	return count
}

// splitFile divides a file into n non-empty segments, distributing any remainder across the first segments.
func splitFile(fileSize int64, n int) []SegmentRange {
	if n <= 0 || fileSize <= 0 {
		return nil
	}
	if int64(n) > fileSize {
		n = int(fileSize)
	}
	baseSize := fileSize / int64(n)
	remainder := fileSize % int64(n)
	ranges := make([]SegmentRange, n)
	var offset int64
	for i := 0; i < n; i++ {
		size := baseSize
		if int64(i) < remainder {
			size++
		}
		ranges[i] = SegmentRange{Index: i, OffsetStart: offset, OffsetEnd: offset + size - 1, Size: size}
		offset += size
	}
	return ranges
}

func buildResourceMeta(extra map[string]string, config map[string]any) ResourceMeta {
	meta := ResourceMeta{
		DownloadAt: time.Now().Unix(),
	}
	if extra != nil {
		meta.ID = extra["id"]
		meta.Title = extra["title"]
		meta.Spec = extra["spec"]
		meta.Author = extra["author"]
		if v, err := strconv.ParseInt(extra["created_at"], 10, 64); err == nil {
			meta.CreatedAt = v
		}
	}
	if config != nil {
		if platform, ok := config["platform"].(string); ok {
			meta.Platform = platform
		}
	}
	return meta
}

// buildTemplateMeta builds a metadata map for {{var}} template substitution from resource.Extra and task config.
func buildTemplateMeta(extra map[string]string, config map[string]any, currentName string) map[string]string {
	meta := make(map[string]string)
	meta["download_at"] = time.Now().Format("2006-01-02")
	meta["filename"] = currentName
	if extra != nil {
		meta["id"] = extra["id"]
		meta["title"] = extra["title"]
		meta["spec"] = extra["spec"]
		meta["author"] = extra["author"]
		meta["created_at"] = extra["created_at"]
	}
	// User config spec overrides resource metadata spec
	if config != nil {
		if spec, ok := config["spec"].(string); ok && spec != "" {
			meta["spec"] = spec
		}
	}
	return meta
}

// calcSpeed computes download speed (bytes/sec) between two points in time.
func getConfigBool(config map[string]any, key string) bool {
	if v, ok := config[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// getConfigString reads the string value for the specified task config key.
func getConfigString(config map[string]any, key string) string {
	if v, ok := config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// findNextDuplicateName finds the next available numeric suffix filename when a file already exists.
// e.g., if baseName.mp4 exists, try baseName(1).mp4, baseName(2).mp4, ...
func (d *HermesEngine) findNextDuplicateName(task *TaskJob, resource *ResourceJob, existingPath, dir, baseName, ext string) string {
	tmpExt := ".tmp"
	for counter := 1; ; counter++ {
		candidate := fmt.Sprintf("%s(%d)%s", baseName, counter, tmpExt)
		candidatePath := d.absFilePath(task.SavePath, filepath.Join(dir, candidate))
		if _, err := os.Stat(candidatePath); os.IsNotExist(err) {
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
func (d *HermesEngine) resolveDuplicateFilename(savePath, baseName, ext string) string {
	for counter := 0; ; counter++ {
		candidate := baseName + ext
		if counter > 0 {
			candidate = fmt.Sprintf("%s(%d)%s", baseName, counter, ext)
		}
		candidatePath := d.absFilePath(savePath, candidate)
		if _, err := os.Stat(candidatePath); os.IsNotExist(err) {
			return candidate
		}
	}
}

// persistResourceName updates the resource name in the database. 'reason' is used in logs to annotate the trigger scenario.
func (d *HermesEngine) persistResourceName(task *TaskJob, resource *ResourceJob, resourceName, originalDBName, reason string) (bool, error) {
	if resourceName == originalDBName {
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
		ResourceName: resourceName,
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
			Str("old_name", originalDBName).
			Str("new_name", resourceName).
			Msg("resource name updated in DB")
	} else {
		d.logger.Warn().
			Int("task_id", task.ID).
			Int("resource_id", resource.ID).
			Msg("store does not implement OutputNameStore, skipping DB update")
	}
	return true, nil
}
