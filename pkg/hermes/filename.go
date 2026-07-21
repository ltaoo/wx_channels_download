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
// persist the returned name before handing the task to Engine so persisted
// resource metadata and the eventual output path stay aligned.
type FilenameProcessor struct {
	// usedFilenames stores relative path/name keys and their duplicate counts.
	usedFilenames  map[string]int
	forbiddenChars *regexp.Regexp
	maxNameLength  int
	mu             sync.Mutex
}

// NewFilenameProcessor creates a processor using existingFiles as its initial
// reservation set. rootDir is retained for API compatibility; names produced
// by the processor are always relative to the task's save path.
func NewFilenameProcessor(rootDir string, existingFiles map[string]int) *FilenameProcessor {
	_ = rootDir
	if existingFiles == nil {
		existingFiles = make(map[string]int)
	}

	return &FilenameProcessor{
		usedFilenames:  existingFiles,
		forbiddenChars: regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`),
		maxNameLength:  235,
	}
}

// truncateString truncates by bytes without splitting a UTF-8 rune.
func (fp *FilenameProcessor) truncateString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
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

	filename = fp.truncateString(filename, fp.maxNameLength)
	filename = fp.forbiddenChars.ReplaceAllString(filename, "")
	filename = strings.TrimSpace(filename)
	filename = strings.Trim(filename, ".")
	if filename == "" {
		return "", fmt.Errorf("filename contains only invalid characters")
	}

	reservedNames := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true,
		"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
		"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
	baseName := strings.ToUpper(strings.SplitN(filename, ".", 2)[0])
	if reservedNames[baseName] {
		return filename + "_", nil
	}

	return filename, nil
}

// AppendExtension appends a known extension without exceeding the filename
// length limit. The extension is included in the 235-byte limit.
func (fp *FilenameProcessor) AppendExtension(filename, extension string) (string, error) {
	extension = strings.TrimSpace(extension)
	if extension == "" {
		return fp.SanitizeFilename(filename)
	}
	if !strings.HasPrefix(extension, ".") {
		return "", fmt.Errorf("extension must start with a dot")
	}
	cleanName, err := fp.SanitizeFilename(filename)
	if err != nil {
		return "", err
	}
	maxBaseLength := fp.maxNameLength - len(extension)
	if maxBaseLength <= 0 {
		return "", fmt.Errorf("extension is too long")
	}
	cleanName = fp.truncateString(cleanName, maxBaseLength)
	if cleanName == "" {
		return "", fmt.Errorf("filename contains only invalid characters")
	}
	return cleanName + extension, nil
}

// NormalizeFilename sanitizes a relative path without changing its duplicate
// reservation state.
func (fp *FilenameProcessor) NormalizeFilename(inputName string) (string, string, error) {
	inputName = strings.ReplaceAll(inputName, "//", "_")
	dir, filename := filepath.Split(inputName)
	cleanName, err := fp.SanitizeFilename(filename)
	if err != nil {
		return "", "", fmt.Errorf("invalid filename %q: %w", filename, err)
	}
	if dir == "" {
		return cleanName, "", nil
	}

	dir = strings.TrimSuffix(dir, string(filepath.Separator))
	validDirs := make([]string, 0)
	for _, component := range strings.Split(dir, string(filepath.Separator)) {
		validDir, err := fp.SanitizeFilename(component)
		if err != nil {
			continue
		}
		validDirs = append(validDirs, validDir)
	}
	return cleanName, filepath.Join(validDirs...), nil
}

// ProcessFilename normalizes inputName and reserves a unique relative path.
func (fp *FilenameProcessor) ProcessFilename(inputName string) (string, string, error) {
	cleanName, dir, err := fp.NormalizeFilename(inputName)
	if err != nil {
		return "", "", err
	}

	fp.mu.Lock()
	defer fp.mu.Unlock()

	pathKey := filepath.Clean(filepath.Join(dir, cleanName))
	count, exists := fp.usedFilenames[pathKey]
	if exists {
		ext := filepath.Ext(cleanName)
		nameWithoutExt := cleanName[:len(cleanName)-len(ext)]
		for {
			count++
			newName := fmt.Sprintf("%s(%d)%s", nameWithoutExt, count, ext)
			newPathKey := filepath.Clean(filepath.Join(dir, newName))
			if _, used := fp.usedFilenames[newPathKey]; !used {
				cleanName = newName
				pathKey = newPathKey
				break
			}
		}
	}
	fp.usedFilenames[pathKey] = count
	if exists {
		fp.usedFilenames[pathKey] = 0
	}
	return cleanName, dir, nil
}

// RemoveFilename releases a previously reserved name, for example before an
// overwrite operation.
func (fp *FilenameProcessor) RemoveFilename(name, dir string) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	delete(fp.usedFilenames, filepath.Clean(filepath.Join(dir, name)))
}

// ProcessFilename normalizes a batch while preserving the input maps.
func ProcessFilename(existingTaskMap map[string]int, items []map[string]string, rootDir string) ([]map[string]string, error) {
	processor := NewFilenameProcessor(rootDir, existingTaskMap)
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
		finalName, dir, err := processor.ProcessFilename(name)
		if err != nil {
			return nil, fmt.Errorf("process filename for item %v: %w", item, err)
		}
		result["name"] = finalName
		result["original_name"] = name
		result["full_path"] = filepath.Join(dir, finalName)
		results = append(results, result)
	}
	return results, nil
}
