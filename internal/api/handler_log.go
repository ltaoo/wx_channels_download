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

	result "wx_channel/internal/apiresult"
)

const (
	default_log_limit     = 300
	max_log_limit         = 2000
	default_log_max_bytes = 2 * 1024 * 1024
	max_log_max_bytes     = 10 * 1024 * 1024
)

type api_log_entry struct {
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

type api_log_file_info struct {
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
	limit := bounded_log_int(query.Get("limit"), default_log_limit, 1, max_log_limit)
	max_bytes := bounded_log_int(query.Get("max_bytes"), default_log_max_bytes, 64*1024, max_log_max_bytes)
	keyword := strings.ToLower(strings.TrimSpace(query.Get("keyword")))
	source_filter := strings.ToLower(strings.TrimSpace(query.Get("source")))
	levels := parse_log_level_filter(query.Get("levels"))
	format_json := parse_log_bool(query.Get("format_json"))

	files := c.discover_log_files()
	entries := make([]api_log_entry, 0, limit)
	total := 0
	seq := 0
	for _, file := range files {
		lines, err := tail_log_lines(file.Path, max_bytes)
		if err != nil {
			continue
		}
		for _, line := range lines {
			entry := parse_log_line(file, line, format_json)
			if !match_log_entry(entry, levels, keyword, source_filter) {
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

func (c *APIClient) discover_log_files() []api_log_file_info {
	log_path := strings.TrimSpace(c.cfg.LogPath)
	if log_path == "" && c.cfg.Original != nil {
		log_path = strings.TrimSpace(c.cfg.Original.LogPath())
	}
	if log_path == "" {
		return nil
	}
	if !filepath.IsAbs(log_path) && strings.TrimSpace(c.cfg.WorkDir) != "" {
		log_path = filepath.Join(c.cfg.WorkDir, log_path)
	}
	info, err := os.Stat(log_path)
	if err != nil || info.IsDir() {
		return nil
	}
	return []api_log_file_info{{
		Name: filepath.Base(log_path),
		Path: log_path,
		Size: info.Size(),
	}}
}

func tail_log_lines(path string, max_bytes int) ([]string, error) {
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
	if info.Size() > int64(max_bytes) {
		start = info.Size() - int64(max_bytes)
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

func parse_log_line(file api_log_file_info, raw string, format_json bool) api_log_entry {
	entry := api_log_entry{
		Source:  source_from_log_file(file.Name),
		Level:   "info",
		Message: raw,
		Raw:     raw,
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		entry.JSON = obj
		if value := log_string_field(obj, "time", "timestamp"); value != "" {
			entry.Time = value
		}
		if value := log_string_field(obj, "level"); value != "" {
			entry.Level = strings.ToLower(value)
		}
		if value := log_string_field(obj, "message", "msg"); value != "" {
			entry.Message = value
		}
		if value := log_string_field(obj, "file"); value != "" {
			entry.File = value
		}
		if value := log_string_field(obj, "component"); value != "" {
			entry.Component = value
		}
		if value := log_string_field(obj, "service", "component", "Client"); value != "" {
			entry.Source = value
		}
		if format_json {
			if data, err := json.MarshalIndent(obj, "", "  "); err == nil {
				entry.Formatted = string(data)
			}
		}
		return entry
	}
	entry.Level = infer_text_log_level(raw)
	return entry
}

func log_string_field(obj map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := obj[key]; ok && value != nil {
			return strings.TrimSpace(strings.Trim(strings.TrimSpace(log_value_string(value)), `"`))
		}
	}
	for _, key := range keys {
		for actual, value := range obj {
			if strings.EqualFold(actual, key) && value != nil {
				return strings.TrimSpace(strings.Trim(strings.TrimSpace(log_value_string(value)), `"`))
			}
		}
	}
	return ""
}

func log_value_string(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func source_from_log_file(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "app" {
		return "app"
	}
	return base
}

func infer_text_log_level(raw string) string {
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

func parse_log_level_filter(raw string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		level := strings.ToLower(strings.TrimSpace(part))
		if level != "" && level != "all" {
			out[level] = true
		}
	}
	return out
}

func match_log_entry(entry api_log_entry, levels map[string]bool, keyword string, source string) bool {
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

func bounded_log_int(raw string, fallback int, min int, max int) int {
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

func parse_log_bool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
