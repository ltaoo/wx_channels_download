package api

import (
	"bufio"
	"container/heap"
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
	default_log_page_size = 300
	max_log_page_size     = 2000
	default_log_max_bytes = 2 * 1024 * 1024
	max_log_max_bytes     = 10 * 1024 * 1024
)

type api_log_entry struct {
	Index     int    `json:"index"`
	File      string `json:"file"`
	Component string `json:"component"`
	Source    string `json:"source"`
	Time      string `json:"time"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Raw       string `json:"raw"`
	sort_time time.Time
	has_time  bool
}

type api_log_entry_heap []api_log_entry

func (entries api_log_entry_heap) Len() int {
	return len(entries)
}

func (entries api_log_entry_heap) Less(first_index, second_index int) bool {
	return api_log_entry_newer(entries[second_index], entries[first_index])
}

func (entries api_log_entry_heap) Swap(first_index, second_index int) {
	entries[first_index], entries[second_index] = entries[second_index], entries[first_index]
}

func (entries *api_log_entry_heap) Push(value interface{}) {
	*entries = append(*entries, value.(api_log_entry))
}

func (entries *api_log_entry_heap) Pop() interface{} {
	old_entries := *entries
	last_index := len(old_entries) - 1
	entry := old_entries[last_index]
	*entries = old_entries[:last_index]
	return entry
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
	page := bounded_log_int(query.Get("page"), 1, 1, 1000000)
	page_size_raw := query.Get("page_size")
	if strings.TrimSpace(page_size_raw) == "" {
		page_size_raw = query.Get("limit")
	}
	page_size := bounded_log_int(page_size_raw, default_log_page_size, 1, max_log_page_size)
	max_bytes := bounded_log_int(query.Get("max_bytes"), default_log_max_bytes, 64*1024, max_log_max_bytes)
	keyword := strings.ToLower(strings.TrimSpace(query.Get("keyword")))
	source_filter := strings.ToLower(strings.TrimSpace(query.Get("source")))
	levels := parse_log_level_filter(query.Get("levels"))

	files := c.discover_log_files()
	retention_limit := page * page_size
	entries := make(api_log_entry_heap, 0, page_size)
	total := 0
	seq := 0
	for _, file := range files {
		err := scan_tail_log_lines(file.Path, max_bytes, func(line string) {
			entry := parse_log_line(file, line)
			if !match_log_entry(entry, levels, keyword, source_filter) {
				return
			}
			seq++
			entry.Index = seq
			total++
			retain_api_log_entry(&entries, entry, retention_limit)
		})
		if err != nil {
			continue
		}
	}

	sort_log_entries(entries)
	page_count := 1
	if total > 0 {
		page_count = (total + page_size - 1) / page_size
	}
	if page > page_count {
		page = page_count
	}
	offset := (page - 1) * page_size
	end := offset + page_size
	if offset > len(entries) {
		offset = len(entries)
	}
	if end > len(entries) {
		end = len(entries)
	}
	entries = entries[offset:end]
	for i := range entries {
		entries[i].Index = offset + i + 1
	}
	result.Ok(ctx, gin.H{
		"entries":   entries,
		"files":     files,
		"total":     total,
		"page":      page,
		"page_size": page_size,
		"limit":     page_size,
	})
}

func (c *APIClient) handle_clear_logs(ctx *gin.Context) {
	if c.cfg == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}

	files := c.discover_log_files()
	if len(files) == 0 {
		result.Err(ctx, 404, "日志文件不存在或不可用")
		return
	}

	var active_log_file *os.File
	if c.cfg.Original != nil {
		active_log_file = c.cfg.Original.LogFile()
	}
	for file_index := range files {
		if err := truncate_log_file(files[file_index].Path, active_log_file); err != nil {
			result.Err(ctx, 500, "清空日志文件失败: "+err.Error())
			return
		}
		files[file_index].Size = 0
	}

	result.Ok(ctx, gin.H{
		"cleared": len(files),
		"files":   files,
	})
}

func truncate_log_file(log_path string, active_log_file *os.File) error {
	path_info, path_err := os.Stat(log_path)
	if path_err != nil {
		return path_err
	}
	if path_info.IsDir() {
		return fmt.Errorf("日志路径是目录")
	}

	if active_log_file != nil {
		active_info, active_err := active_log_file.Stat()
		if active_err == nil && os.SameFile(path_info, active_info) {
			if err := active_log_file.Truncate(0); err != nil {
				return err
			}
			if _, err := active_log_file.Seek(0, io.SeekStart); err != nil {
				return err
			}
			return nil
		}
	}

	return os.Truncate(log_path, 0)
}

func sort_log_entries(entries []api_log_entry) {
	for entry_index := range entries {
		prepare_api_log_entry_sort(&entries[entry_index])
	}
	sort.SliceStable(entries, func(first_index, second_index int) bool {
		return api_log_entry_newer(entries[first_index], entries[second_index])
	})
}

func prepare_api_log_entry_sort(entry *api_log_entry) {
	entry.has_time = false
	parsed_time, err := time.Parse(time.RFC3339Nano, entry.Time)
	if err == nil {
		entry.sort_time = parsed_time
		entry.has_time = true
	}
}

func api_log_entry_newer(first api_log_entry, second api_log_entry) bool {
	if first.has_time && second.has_time && !first.sort_time.Equal(second.sort_time) {
		return first.sort_time.After(second.sort_time)
	}
	return first.Index > second.Index
}

func retain_api_log_entry(entries *api_log_entry_heap, entry api_log_entry, limit int) {
	if limit <= 0 {
		return
	}
	prepare_api_log_entry_sort(&entry)
	if entries.Len() < limit {
		heap.Push(entries, entry)
		return
	}
	if api_log_entry_newer(entry, (*entries)[0]) {
		(*entries)[0] = entry
		heap.Fix(entries, 0)
	}
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

func scan_tail_log_lines(path string, max_bytes int, visit_line func(string)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	start := int64(0)
	if info.Size() > int64(max_bytes) {
		start = info.Size() - int64(max_bytes)
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReader(file)
	if start > 0 {
		_, _ = reader.ReadString('\n')
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if strings.TrimSpace(line) != "" {
			visit_line(line)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func parse_log_line(file api_log_file_info, raw string) api_log_entry {
	entry := api_log_entry{
		Source:  source_from_log_file(file.Name),
		Level:   "info",
		Message: raw,
		Raw:     raw,
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
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
	if len(levels) > 0 {
		if !levels[entry.Level] && !levels[strings.ToLower(entry.Level)] {
			return false
		}
	}
	if source != "" &&
		source != "all" &&
		!contains_normalized_log_text(entry.Source, source) &&
		!contains_normalized_log_text(entry.File, source) &&
		!contains_normalized_log_text(entry.Component, source) {
		return false
	}
	if keyword == "" {
		return true
	}
	if contains_normalized_log_text(entry.Raw, keyword) {
		return true
	}
	if entry.Message != entry.Raw && contains_normalized_log_text(entry.Message, keyword) {
		return true
	}
	return contains_normalized_log_text(entry.Source, keyword) ||
		contains_normalized_log_text(entry.File, keyword) ||
		contains_normalized_log_text(entry.Component, keyword)
}

func contains_normalized_log_text(text string, normalized_query string) bool {
	if normalized_query == "" {
		return true
	}
	if len(normalized_query) > len(text) {
		return false
	}
	query_is_ascii := true
	for query_index := 0; query_index < len(normalized_query); query_index++ {
		if normalized_query[query_index] >= 0x80 {
			query_is_ascii = false
			break
		}
	}
	if !query_is_ascii {
		return strings.Contains(strings.ToLower(text), normalized_query)
	}
	if strings.Contains(text, normalized_query) {
		return true
	}
	for text_index := 0; text_index <= len(text)-len(normalized_query); text_index++ {
		matched := true
		for query_index := 0; query_index < len(normalized_query); query_index++ {
			text_byte := text[text_index+query_index]
			if text_byte >= 'A' && text_byte <= 'Z' {
				text_byte += 'a' - 'A'
			}
			if text_byte != normalized_query[query_index] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
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
