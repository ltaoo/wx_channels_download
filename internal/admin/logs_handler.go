package admin

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLogLimit    = 300
	maxLogLimit        = 2000
	defaultLogMaxBytes = 2 * 1024 * 1024
	maxLogMaxBytes     = 10 * 1024 * 1024
)

type logEntry struct {
	Index     int                    `json:"index"`
	File      string                 `json:"file"`
	Source    string                 `json:"source"`
	Time      string                 `json:"time"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Raw       string                 `json:"raw"`
	JSON      map[string]interface{} `json:"json,omitempty"`
	Formatted string                 `json:"formatted,omitempty"`
}

type logFileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func (s *AdminServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.cfg == nil {
		s.writeError(w, http.StatusInternalServerError, "配置未初始化")
		return
	}

	query := r.URL.Query()
	limit := boundedInt(query.Get("limit"), defaultLogLimit, 1, maxLogLimit)
	maxBytes := boundedInt(query.Get("max_bytes"), defaultLogMaxBytes, 64*1024, maxLogMaxBytes)
	keyword := strings.ToLower(strings.TrimSpace(query.Get("keyword")))
	sourceFilter := strings.ToLower(strings.TrimSpace(query.Get("source")))
	levels := parseLevelFilter(query.Get("levels"))
	formatJSON := parseBool(query.Get("format_json"))

	files := s.discoverLogFiles()
	entries := make([]logEntry, 0, limit)
	total := 0
	seq := 0
	for _, fp := range files {
		lines, err := tailLines(fp.Path, maxBytes)
		if err != nil {
			continue
		}
		for _, line := range lines {
			entry := parseLogLine(fp, line, formatJSON)
			if !matchLogEntry(entry, levels, keyword, sourceFilter) {
				continue
			}
			seq++
			entry.Index = seq
			total++
			entries = append(entries, entry)
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		ti, ei := time.Parse(time.RFC3339Nano, entries[i].Time)
		tj, ej := time.Parse(time.RFC3339Nano, entries[j].Time)
		if ei == nil && ej == nil && !ti.Equal(tj) {
			return ti.After(tj)
		}
		return entries[i].Index > entries[j].Index
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	for i := range entries {
		entries[i].Index = i + 1
	}

	s.writeOK(w, map[string]interface{}{
		"entries": entries,
		"files":   files,
		"total":   total,
		"limit":   limit,
	})
}

func (s *AdminServer) discoverLogFiles() []logFileInfo {
	roots := uniqueStrings([]string{
		s.cfg.WorkDir,
		filepath.Join(s.cfg.WorkDir, "logs"),
		filepath.Join(s.cfg.WorkDir, "data", "logs"),
	})
	files := make(map[string]logFileInfo)
	for _, root := range roots {
		if root == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if isLogFile(root) {
				files[root] = logFileInfo{Name: filepath.Base(root), Path: root, Size: info.Size()}
			}
			continue
		}
		baseDepth := pathDepth(root)
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != root && pathDepth(path)-baseDepth > 3 {
					return filepath.SkipDir
				}
				return nil
			}
			if !isLogFile(path) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			files[path] = logFileInfo{Name: filepath.Base(path), Path: path, Size: info.Size()}
			return nil
		})
	}
	list := make([]logFileInfo, 0, len(files))
	for _, f := range files {
		list = append(list, f)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Name == "app.log" {
			return true
		}
		if list[j].Name == "app.log" {
			return false
		}
		return list[i].Path < list[j].Path
	})
	return list
}

func isLogFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".jsonl")
}

func pathDepth(path string) int {
	return len(strings.Split(filepath.Clean(path), string(os.PathSeparator)))
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = filepath.Clean(value)
		if value == "." || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func tailLines(path string, maxBytes int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := int64(0)
	if info.Size() > int64(maxBytes) {
		start = info.Size() - int64(maxBytes)
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(f)
	if start > 0 {
		_, _ = reader.ReadString('\n')
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lines := []string{}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return lines, nil
}

func parseLogLine(file logFileInfo, raw string, formatJSON bool) logEntry {
	entry := logEntry{
		File:    file.Name,
		Source:  sourceFromFile(file.Name),
		Level:   "info",
		Message: raw,
		Raw:     raw,
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		entry.JSON = obj
		if v := stringField(obj, "time", "timestamp"); v != "" {
			entry.Time = v
		}
		if v := stringField(obj, "level"); v != "" {
			entry.Level = strings.ToLower(v)
		}
		if v := stringField(obj, "message", "msg"); v != "" {
			entry.Message = v
		}
		if v := stringField(obj, "service", "component", "Client"); v != "" {
			entry.Source = v
		}
		if formatJSON {
			if b, err := json.MarshalIndent(obj, "", "  "); err == nil {
				entry.Formatted = string(b)
			}
		}
	} else {
		entry.Level = inferTextLogLevel(raw)
	}
	return entry
}

func stringField(obj map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := obj[key]; ok && value != nil {
			return strings.TrimSpace(strings.Trim(strings.TrimSpace(logValueString(value)), `"`))
		}
	}
	return ""
}

func logValueString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func sourceFromFile(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "app" {
		return "app"
	}
	return base
}

func inferTextLogLevel(raw string) string {
	text := strings.ToLower(raw)
	if strings.Contains(text, "error") || strings.Contains(text, "[error]") || strings.Contains(text, "失败") {
		return "error"
	}
	if strings.Contains(text, "warn") || strings.Contains(text, "warning") || strings.Contains(text, "警告") {
		return "warn"
	}
	if strings.Contains(text, "debug") {
		return "debug"
	}
	return "info"
}

func parseLevelFilter(raw string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		level := strings.ToLower(strings.TrimSpace(part))
		if level != "" && level != "all" {
			out[level] = true
		}
	}
	return out
}

func matchLogEntry(entry logEntry, levels map[string]bool, keyword string, source string) bool {
	if len(levels) > 0 && !levels[strings.ToLower(entry.Level)] {
		return false
	}
	if source != "" && source != "all" && !strings.Contains(strings.ToLower(entry.Source), source) && !strings.Contains(strings.ToLower(entry.File), source) {
		return false
	}
	if keyword == "" {
		return true
	}
	haystack := strings.ToLower(entry.Raw + "\n" + entry.Message + "\n" + entry.Source + "\n" + entry.File)
	return strings.Contains(haystack, keyword)
}

func boundedInt(raw string, fallback int, min int, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		n = fallback
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
