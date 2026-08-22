package hermes

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// FilenameProcessor normalizes names for downloader output and reserves each
// normalized relative path in memory to avoid duplicate output names.
//
// It does not create directories or inspect the filesystem. Callers must
// persist the returned name before handing the task to HermesEngine so persisted
// resource metadata and the eventual output path stay aligned.
type FilenameProcessor struct {
	// usedFilenames stores relative path/name keys and their duplicate counts.
	used_filenames  map[string]int
	forbidden_chars *regexp.Regexp
	max_name_length int
	mu              sync.Mutex
}

// NewFilenameProcessor creates a processor using existingFiles as its initial
// reservation set. rootDir is retained for API compatibility; names produced
// by the processor are always relative to the task's download directory.
func NewFilenameProcessor(root_dir string, existing_files map[string]int) *FilenameProcessor {
	_ = root_dir
	if existing_files == nil {
		existing_files = make(map[string]int)
	}

	return &FilenameProcessor{
		used_filenames:  existing_files,
		forbidden_chars: regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`),
		max_name_length: 235,
	}
}

// truncateString truncates by bytes without splitting a UTF-8 rune.
func (fp *FilenameProcessor) truncate_string(s string, max_bytes int) string {
	if len(s) <= max_bytes {
		return s
	}
	s = s[:max_bytes]
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}

// SanitizeFilename removes cross-platform invalid filename characters.
func (fp *FilenameProcessor) SanitizeFilename(filename string) (string, error) {
	if filename == "" {
		return "", fmt.Errorf("filename cannot be empty")
	}

	filename = fp.forbidden_chars.ReplaceAllString(filename, "")
	filename = strings.TrimSpace(filename)
	filename = strings.Trim(filename, ".")
	if filename == "" {
		return "", fmt.Errorf("filename contains only invalid characters")
	}

	if is_windows_reserved_filename(filename) {
		// Prefixing makes names such as CON.txt safe. Appending after the
		// extension would still address the reserved CON device on Windows.
		filename = "_" + filename
	}

	return fp.truncate_string(filename, fp.max_name_length), nil
}

func is_windows_reserved_filename(filename string) bool {
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

// AppendExtension appends a known extension without exceeding the filename
// length limit. The extension is included in the 235-byte limit.
// If filename contains a subdirectory path (e.g., "dir/video"), the directory
// portion is preserved and only the base filename is sanitized and truncated.
func (fp *FilenameProcessor) AppendExtension(filename, extension string) (string, error) {
	extension = strings.TrimSpace(extension)
	if extension == "" {
		return fp.SanitizeFilename(filename)
	}
	if !strings.HasPrefix(extension, ".") {
		return "", fmt.Errorf("extension must start with a dot")
	}
	dir, base := filepath.Split(filename)
	clean_name, err := fp.SanitizeFilename(base)
	if err != nil {
		return "", err
	}
	max_base_length := fp.max_name_length - len(extension)
	if max_base_length <= 0 {
		return "", fmt.Errorf("extension is too long")
	}
	clean_name = fp.truncate_string(clean_name, max_base_length)
	if clean_name == "" {
		return "", fmt.Errorf("filename contains only invalid characters")
	}
	return dir + clean_name + extension, nil
}

// NormalizeFilename sanitizes a relative path without changing its duplicate
// reservation state.
func (fp *FilenameProcessor) NormalizeFilename(input_name string) (string, string, error) {
	input_name = strings.ReplaceAll(input_name, "//", "_")
	dir, filename := filepath.Split(input_name)
	clean_name, err := fp.SanitizeFilename(filename)
	if err != nil {
		return "", "", fmt.Errorf("invalid filename %q: %w", filename, err)
	}
	if dir == "" {
		return clean_name, "", nil
	}

	dir = strings.TrimSuffix(dir, string(filepath.Separator))
	valid_dirs := make([]string, 0)
	for _, component := range strings.Split(dir, string(filepath.Separator)) {
		valid_dir, err := fp.SanitizeFilename(component)
		if err != nil {
			continue
		}
		valid_dirs = append(valid_dirs, valid_dir)
	}
	return clean_name, filepath.Join(valid_dirs...), nil
}

// ProcessFilename normalizes inputName and reserves a unique relative path.
func (fp *FilenameProcessor) ProcessFilename(input_name string) (string, string, error) {
	clean_name, dir, err := fp.NormalizeFilename(input_name)
	if err != nil {
		return "", "", err
	}

	fp.mu.Lock()
	defer fp.mu.Unlock()

	path_key := filepath.Clean(filepath.Join(dir, clean_name))
	count, exists := fp.used_filenames[path_key]
	if exists {
		ext := filepath.Ext(clean_name)
		name_without_ext := clean_name[:len(clean_name)-len(ext)]
		for {
			count++
			new_name := fmt.Sprintf("%s(%d)%s", name_without_ext, count, ext)
			new_path_key := filepath.Clean(filepath.Join(dir, new_name))
			if _, used := fp.used_filenames[new_path_key]; !used {
				clean_name = new_name
				path_key = new_path_key
				break
			}
		}
	}
	fp.used_filenames[path_key] = count
	if exists {
		fp.used_filenames[path_key] = 0
	}
	return clean_name, dir, nil
}

// RemoveFilename releases a previously reserved name, for example before an
// overwrite operation.
func (fp *FilenameProcessor) RemoveFilename(name, dir string) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	delete(fp.used_filenames, filepath.Clean(filepath.Join(dir, name)))
}

// ProcessFilename normalizes a batch while preserving the input maps.
func ProcessFilename(existing_task_map map[string]int, items []map[string]string, root_dir string) ([]map[string]string, error) {
	processor := NewFilenameProcessor(root_dir, existing_task_map)
	results := make([]map[string]string, 0, len(items))
	for _, item := range items {
		result := make(map[string]string, len(item)+2)
		for key, value := range item {
			result[key] = value
		}

		name := item["name"]
		if name == "" {
			return nil, fmt.Errorf("item %v has no name field", item)
		}
		final_name, dir, err := processor.ProcessFilename(name)
		if err != nil {
			return nil, fmt.Errorf("process filename for item %v: %w", item, err)
		}
		result["name"] = final_name
		result["original_name"] = name
		result["full_path"] = filepath.Join(dir, final_name)
		results = append(results, result)
	}
	return results, nil
}
