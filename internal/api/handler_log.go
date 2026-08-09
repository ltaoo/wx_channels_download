package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/util"
)

const (
	defaultLogLimit    = 300
	maxLogLimit        = 2000
	defaultLogMaxBytes = 2 * 1024 * 1024
	maxLogMaxBytes     = 10 * 1024 * 1024
)

type apiLogEntry struct {
	Index     int                    `json:"index"`
	File      string                 `json:"file"`
	Component string                 `json:"component"`
	Source    string                 `json:"source"`
	Time      string                 `json:"time"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Raw       string                 `json:"raw"`
	JSON      map[string]interface{} `json:"json,omitempty"`
	Formatted string                 `json:"formatted,omitempty"`
}

type apiLogFileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func (c *APIClient) handle_logs(ctx *gin.Context) {
	if c.cfg == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}

	query := ctx.Request.URL.Query()
	limit := boundedLogInt(query.Get("limit"), defaultLogLimit, 1, maxLogLimit)
	maxBytes := boundedLogInt(query.Get("max_bytes"), defaultLogMaxBytes, 64*1024, maxLogMaxBytes)
	keyword := strings.ToLower(strings.TrimSpace(query.Get("keyword")))
	sourceFilter := strings.ToLower(strings.TrimSpace(query.Get("source")))
	levels := parseLogLevelFilter(query.Get("levels"))
	formatJSON := parseLogBool(query.Get("format_json"))

	files := c.discoverLogFiles()
	entries := make([]apiLogEntry, 0, limit)
	total := 0
	seq := 0
	for _, file := range files {
		lines, err := tailLogLines(file.Path, maxBytes)
		if err != nil {
			continue
		}
		for _, line := range lines {
			entry := parseLogLine(file, line, formatJSON)
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

	result.Ok(ctx, gin.H{
		"entries": entries,
		"files":   files,
		"total":   total,
		"limit":   limit,
	})
}

func (c *APIClient) discoverLogFiles() []apiLogFileInfo {
	logPath := strings.TrimSpace(c.cfg.LogPath)
	if logPath == "" && c.cfg.Original != nil {
		logPath = strings.TrimSpace(c.cfg.Original.LogPath())
	}
	if logPath == "" {
		return nil
	}
	if !filepath.IsAbs(logPath) && strings.TrimSpace(c.cfg.WorkDir) != "" {
		logPath = filepath.Join(c.cfg.WorkDir, logPath)
	}
	info, err := os.Stat(logPath)
	if err != nil || info.IsDir() {
		return nil
	}
	return []apiLogFileInfo{{
		Name: filepath.Base(logPath),
		Path: logPath,
		Size: info.Size(),
	}}
}

func tailLogLines(path string, maxBytes int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	start := int64(0)
	if info.Size() > int64(maxBytes) {
		start = info.Size() - int64(maxBytes)
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
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

func parseLogLine(file apiLogFileInfo, raw string, formatJSON bool) apiLogEntry {
	entry := apiLogEntry{
		Source:  sourceFromLogFile(file.Name),
		Level:   "info",
		Message: raw,
		Raw:     raw,
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		entry.JSON = obj
		if value := logStringField(obj, "time", "timestamp"); value != "" {
			entry.Time = value
		}
		if value := logStringField(obj, "level"); value != "" {
			entry.Level = strings.ToLower(value)
		}
		if value := logStringField(obj, "message", "msg"); value != "" {
			entry.Message = value
		}
		if value := logStringField(obj, "file"); value != "" {
			entry.File = value
		}
		if value := logStringField(obj, "component"); value != "" {
			entry.Component = value
		}
		if value := logStringField(obj, "service", "component", "Client"); value != "" {
			entry.Source = value
		}
		if formatJSON {
			if data, err := json.MarshalIndent(obj, "", "  "); err == nil {
				entry.Formatted = string(data)
			}
		}
		return entry
	}
	entry.Level = inferTextLogLevel(raw)
	return entry
}

func logStringField(obj map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := obj[key]; ok && value != nil {
			return strings.TrimSpace(strings.Trim(strings.TrimSpace(logValueString(value)), `"`))
		}
	}
	for _, key := range keys {
		for actual, value := range obj {
			if strings.EqualFold(actual, key) && value != nil {
				return strings.TrimSpace(strings.Trim(strings.TrimSpace(logValueString(value)), `"`))
			}
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

func sourceFromLogFile(name string) string {
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

func parseLogLevelFilter(raw string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		level := strings.ToLower(strings.TrimSpace(part))
		if level != "" && level != "all" {
			out[level] = true
		}
	}
	return out
}

func matchLogEntry(entry apiLogEntry, levels map[string]bool, keyword string, source string) bool {
	if len(levels) > 0 && !levels[strings.ToLower(entry.Level)] {
		return false
	}
	if source != "" &&
		source != "all" &&
		!strings.Contains(strings.ToLower(entry.Source), source) &&
		!strings.Contains(strings.ToLower(entry.File), source) &&
		!strings.Contains(strings.ToLower(entry.Component), source) {
		return false
	}
	if keyword == "" {
		return true
	}
	haystack := strings.ToLower(entry.Raw + "\n" + entry.Message + "\n" + entry.Source + "\n" + entry.File + "\n" + entry.Component)
	return strings.Contains(haystack, keyword)
}

func boundedLogInt(raw string, fallback int, min int, max int) int {
	number, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		number = fallback
	}
	if number < min {
		return min
	}
	if number > max {
		return max
	}
	return number
}

func parseLogBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
