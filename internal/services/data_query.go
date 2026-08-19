package services

import (
	"bufio"
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

var default_data_browse_platform_ids = []string{
	"wxchannels",
	"wxmp",
	"zhihu",
	"xiaohongshu",
	"bilibili",
	"youtube",
	"weibo",
}

const (
	default_data_page_size     = 20
	max_data_page_size         = 200
	default_data_log_page_size = 300
	max_data_log_page_size     = 2000
	default_data_log_max_bytes = 2 * 1024 * 1024
	max_data_log_max_bytes     = 10 * 1024 * 1024
)

// DownloadTaskListQuery describes a read-only download task query.
type DownloadTaskListQuery struct {
	Page         int
	PageSize     int
	Statuses     []int
	ParentTaskID int
	RootTaskID   int
}

// AccountListQuery describes a read-only account query.
type AccountListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	AccountID string
}

// BrowseHistoryListQuery describes a read-only browse history query.
type BrowseHistoryListQuery struct {
	Page        int
	PageSize    int
	Keyword     string
	Username    string
	PlatformIDs []string
}

// LogListQuery describes a read-only application log query.
type LogListQuery struct {
	Page     int
	PageSize int
	MaxBytes int
	Keyword  string
	Source   string
	Levels   []string
}

// DataQueryServiceConfig contains the application services and runtime paths
// used by transport-independent read-only queries.
type DataQueryServiceConfig struct {
	DB                   *gorm.DB
	AccountService       *AccountService
	DownloadTaskService  *DownloadTaskService
	BrowseHistoryService *BrowseService
	CertificateService   *CertificateService
	LogPath              string
	WorkDir              string
}

// DataQueryService exposes application data independently of HTTP, MCP, or CLI
// transports.
type DataQueryService struct {
	db                     *gorm.DB
	account_service        *AccountService
	download_task_service  *DownloadTaskService
	browse_history_service *BrowseService
	certificate_service    *CertificateService
	log_path               string
	work_dir               string
}

type data_download_task_status_count struct {
	Status int   `gorm:"column:status"`
	Count  int64 `gorm:"column:count"`
}

type data_download_task_file struct {
	DownloadTaskFileRecord
	LocalPath string `json:"local_path"`
	FileType  string `json:"file_type"`
	FileURL   string `json:"file_url"`
	Exists    bool   `json:"exists"`
}

type data_log_entry struct {
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

type data_log_entry_heap []data_log_entry

func (entries data_log_entry_heap) Len() int {
	return len(entries)
}

func (entries data_log_entry_heap) Less(first_index, second_index int) bool {
	return data_log_entry_newer(entries[second_index], entries[first_index])
}

func (entries data_log_entry_heap) Swap(first_index, second_index int) {
	entries[first_index], entries[second_index] = entries[second_index], entries[first_index]
}

func (entries *data_log_entry_heap) Push(value interface{}) {
	*entries = append(*entries, value.(data_log_entry))
}

func (entries *data_log_entry_heap) Pop() interface{} {
	old_entries := *entries
	last_index := len(old_entries) - 1
	entry := old_entries[last_index]
	*entries = old_entries[:last_index]
	return entry
}

type data_log_file_info struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type data_download_task_account_row struct {
	AccountID  string `gorm:"column:account_id"`
	Nickname   string `gorm:"column:nickname"`
	AvatarURL  string `gorm:"column:avatar_url"`
	ExternalID string `gorm:"column:external_id"`
}

// NewDataQueryService constructs the process-local application data service.
func NewDataQueryService(config DataQueryServiceConfig) *DataQueryService {
	return &DataQueryService{
		db:                     config.DB,
		account_service:        config.AccountService,
		download_task_service:  config.DownloadTaskService,
		browse_history_service: config.BrowseHistoryService,
		certificate_service:    config.CertificateService,
		log_path:               strings.TrimSpace(config.LogPath),
		work_dir:               strings.TrimSpace(config.WorkDir),
	}
}

func (r *DataQueryService) ListDownloadTasks(ctx context.Context, input DownloadTaskListQuery) (any, error) {
	if r == nil || r.db == nil || r.download_task_service == nil {
		return nil, errors.New("下载任务数据库服务未初始化")
	}
	page, page_size, err := normalize_data_query_page(input.Page, input.PageSize, default_data_page_size, 100)
	if err != nil {
		return nil, err
	}
	input.Page = page
	input.PageSize = page_size
	if input.ParentTaskID < 0 || input.RootTaskID < 0 {
		return nil, fmt.Errorf("parent_task_id 和 root_task_id 不能为负数")
	}
	for _, status := range input.Statuses {
		if status < 0 || status > 7 {
			return nil, fmt.Errorf("任务状态必须在 0 到 7 之间")
		}
	}
	db := r.db.WithContext(ctx)
	filtered_query := data_download_task_base_query(db, input)
	if len(input.Statuses) == 1 {
		filtered_query = filtered_query.Where("status = ?", input.Statuses[0])
	} else if len(input.Statuses) > 1 {
		filtered_query = filtered_query.Where("status IN ?", input.Statuses)
	}

	var total int64
	if err := filtered_query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("查询下载任务总数失败: %w", err)
	}

	var status_rows []data_download_task_status_count
	if err := data_download_task_base_query(db, input).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&status_rows).Error; err != nil {
		return nil, fmt.Errorf("查询下载任务统计失败: %w", err)
	}
	stats := make(map[int]int64, len(status_rows))
	for _, row := range status_rows {
		stats[row.Status] = row.Count
	}

	var tasks []model.DownloadTask
	if err := filtered_query.
		Order("id DESC").
		Offset((input.Page - 1) * input.PageSize).
		Limit(input.PageSize).
		Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("查询下载任务失败: %w", err)
	}
	list, err := r.download_task_service.BuildTaskRecords(tasks)
	if err != nil {
		return nil, fmt.Errorf("构建下载任务记录失败: %w", err)
	}
	return map[string]any{
		"list":      list,
		"total":     total,
		"page":      input.Page,
		"page_size": input.PageSize,
		"stats":     stats,
	}, nil
}

func (r *DataQueryService) GetDownloadTaskDetail(ctx context.Context, task_id int) (any, error) {
	if r == nil || r.db == nil || r.download_task_service == nil {
		return nil, errors.New("下载任务数据库服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	record, err := r.download_task_service.BuildTaskRecord(task_id)
	if err != nil || record == nil {
		return record, err
	}
	db := r.db.WithContext(ctx)

	var content_data map[string]any
	if record.ContentID != nil && *record.ContentID != "" {
		var content model.Content
		if err := db.Where("id = ?", *record.ContentID).First(&content).Error; err == nil {
			publish_time := int64(0)
			if content.PublishTime != nil {
				publish_time = *content.PublishTime
			}
			content_data = map[string]any{
				"id":           content.Id,
				"platform_id":  content.PlatformId,
				"type":         content.Type,
				"title":        content.Title,
				"description":  content.Description,
				"cover_url":    content.CoverURL,
				"url":          content.URL,
				"source_url":   content.SourceURL,
				"publish_time": publish_time,
			}
			var account_rows []data_download_task_account_row
			if err := db.Table("content_account").
				Select("content_account.account_id, account.nickname, account.avatar_url, account.external_id").
				Joins("JOIN account ON account.id = content_account.account_id").
				Where("content_account.content_id = ? AND account.deleted_at IS NULL", *record.ContentID).
				Scan(&account_rows).Error; err == nil {
				accounts := make([]map[string]any, 0, len(account_rows))
				for _, row := range account_rows {
					accounts = append(accounts, map[string]any{
						"id":          row.AccountID,
						"nickname":    row.Nickname,
						"avatar_url":  row.AvatarURL,
						"external_id": row.ExternalID,
					})
				}
				content_data["accounts"] = accounts
			}
		}
	}

	files := make([]data_download_task_file, 0, len(record.Files))
	for _, file := range record.Files {
		local_path := filepath.Join(file.DownloadDir, file.Name)
		_, stat_err := os.Stat(local_path)
		files = append(files, data_download_task_file{
			DownloadTaskFileRecord: file,
			LocalPath:              local_path,
			FileType:               data_file_type_by_ext(file.Name),
			FileURL:                data_api_file_url(local_path),
			Exists:                 stat_err == nil,
		})
	}
	return map[string]any{
		"id":             record.ID,
		"content":        content_data,
		"content_id":     record.ContentID,
		"content_type":   record.ContentType,
		"parent_task_id": record.ParentTaskID,
		"root_task_id":   record.RootTaskID,
		"relation_type":  record.RelationType,
		"child_count":    record.ChildCount,
		"name":           record.Name,
		"platform_id":    record.PlatformID,
		"status":         record.Status,
		"source_url":     record.SourceURL,
		"cover_url":      record.CoverURL,
		"cover_width":    record.CoverWidth,
		"cover_height":   record.CoverHeight,
		"config_json":    record.ConfigJSON,
		"metadata_json":  record.MetadataJSON,
		"url":            record.URL,
		"size":           record.Size,
		"downloaded":     record.Downloaded,
		"speed":          record.Speed,
		"progress":       record.Progress,
		"error":          record.Error,
		"files":          files,
		"file_count":     len(files),
		"created_at":     record.CreatedAt,
		"updated_at":     record.UpdatedAt,
	}, nil
}

func (r *DataQueryService) ListAccounts(ctx context.Context, input AccountListQuery) (any, error) {
	if r == nil || r.account_service == nil {
		return nil, errors.New("账号服务未初始化")
	}
	page, page_size, err := normalize_data_query_page(input.Page, input.PageSize, 24, max_data_page_size)
	if err != nil {
		return nil, err
	}
	page_result, err := r.account_service.ListAccounts(ctx, AccountListInput{
		Page:      page,
		PageSize:  page_size,
		Keyword:   input.Keyword,
		AccountID: input.AccountID,
	})
	if err != nil {
		return nil, err
	}

	list := make([]map[string]any, 0, len(page_result.List))
	for _, item := range page_result.List {
		account := item.Account
		list = append(list, map[string]any{
			"id":               account.Id,
			"platform_id":      account.PlatformId,
			"nickname":         account.Nickname,
			"alias":            account.Alias,
			"signature":        account.Signature,
			"avatar_url":       account.AvatarURL,
			"profile_url":      account.ProfileURL,
			"external_id":      account.ExternalId,
			"follower_count":   account.FollowerCount,
			"created_at":       account.CreatedAt,
			"updated_at":       account.UpdatedAt,
			"content_count":    item.ContentCount,
			"has_content":      item.ContentCount > 0,
			"content_accounts": item.ContentCount,
		})
	}
	return map[string]any{
		"list":      list,
		"total":     page_result.Total,
		"page":      page_result.Page,
		"page_size": page_result.PageSize,
	}, nil
}

func (r *DataQueryService) ListBrowseHistory(ctx context.Context, input BrowseHistoryListQuery) (any, error) {
	if r == nil || r.browse_history_service == nil {
		return nil, errors.New("浏览记录数据库服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	page, page_size, err := normalize_data_query_page(input.Page, input.PageSize, default_data_page_size, max_data_page_size)
	if err != nil {
		return nil, err
	}
	platform_ids := input.PlatformIDs
	if len(platform_ids) == 0 {
		platform_ids = append([]string(nil), default_data_browse_platform_ids...)
	}
	var username *string
	if input.Username != "" {
		username_value := input.Username
		username = &username_value
	}
	result, err := r.browse_history_service.ListPlatforms(
		platform_ids,
		username,
		page,
		page_size,
		input.Keyword,
	)
	if err != nil {
		return nil, fmt.Errorf("查询浏览记录失败: %w", err)
	}
	return result, nil
}

func (r *DataQueryService) ListLogs(ctx context.Context, input LogListQuery) (any, error) {
	if r == nil {
		return nil, errors.New("配置未初始化")
	}
	page, page_size, err := normalize_data_query_page(input.Page, input.PageSize, default_data_log_page_size, max_data_log_page_size)
	if err != nil {
		return nil, err
	}
	input.Page = page
	input.PageSize = page_size
	if input.MaxBytes == 0 {
		input.MaxBytes = default_data_log_max_bytes
	}
	if input.MaxBytes < 64*1024 || input.MaxBytes > max_data_log_max_bytes {
		return nil, fmt.Errorf("max_bytes 必须在 %d 到 %d 之间", 64*1024, max_data_log_max_bytes)
	}
	levels := make(map[string]bool, len(input.Levels))
	for _, level := range input.Levels {
		level = strings.ToLower(strings.TrimSpace(level))
		if level != "" && level != "all" {
			levels[level] = true
		}
	}
	keyword := strings.ToLower(strings.TrimSpace(input.Keyword))
	source := strings.ToLower(strings.TrimSpace(input.Source))
	files := r.discover_log_files()
	retention_limit := input.Page * input.PageSize
	entries := make(data_log_entry_heap, 0, input.PageSize)
	total := 0
	sequence := 0
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		err := data_scan_tail_log_lines(ctx, file.Path, input.MaxBytes, func(line string) {
			entry := data_parse_log_line(file, line)
			if !data_match_log_entry(entry, levels, keyword, source) {
				return
			}
			sequence++
			entry.Index = sequence
			total++
			retain_data_log_entry(&entries, entry, retention_limit)
		})
		if err != nil {
			if context_err := ctx.Err(); context_err != nil {
				return nil, context_err
			}
			continue
		}
	}

	data_sort_log_entries(entries)
	page_count := 1
	if total > 0 {
		page_count = (total + input.PageSize - 1) / input.PageSize
	}
	if page > page_count {
		page = page_count
	}
	offset := (page - 1) * input.PageSize
	end := offset + input.PageSize
	if offset > len(entries) {
		offset = len(entries)
	}
	if end > len(entries) {
		end = len(entries)
	}
	entries = entries[offset:end]
	for index := range entries {
		entries[index].Index = offset + index + 1
	}
	return map[string]any{
		"entries":   entries,
		"files":     files,
		"total":     total,
		"page":      page,
		"page_size": input.PageSize,
		"limit":     input.PageSize,
	}, nil
}

func data_sort_log_entries(entries []data_log_entry) {
	for entry_index := range entries {
		prepare_data_log_entry_sort(&entries[entry_index])
	}
	sort.SliceStable(entries, func(first_index, second_index int) bool {
		return data_log_entry_newer(entries[first_index], entries[second_index])
	})
}

func prepare_data_log_entry_sort(entry *data_log_entry) {
	entry.has_time = false
	parsed_time, err := time.Parse(time.RFC3339Nano, entry.Time)
	if err == nil {
		entry.sort_time = parsed_time
		entry.has_time = true
	}
}

func data_log_entry_newer(first data_log_entry, second data_log_entry) bool {
	if first.has_time && second.has_time && !first.sort_time.Equal(second.sort_time) {
		return first.sort_time.After(second.sort_time)
	}
	return first.Index > second.Index
}

func retain_data_log_entry(entries *data_log_entry_heap, entry data_log_entry, limit int) {
	if limit <= 0 {
		return
	}
	prepare_data_log_entry_sort(&entry)
	if entries.Len() < limit {
		heap.Push(entries, entry)
		return
	}
	if data_log_entry_newer(entry, (*entries)[0]) {
		(*entries)[0] = entry
		heap.Fix(entries, 0)
	}
}

func (r *DataQueryService) GetCertificateStatus(ctx context.Context) (any, error) {
	if r == nil || r.certificate_service == nil {
		return nil, errors.New("证书状态服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.certificate_service.Status(), nil
}

func data_download_task_base_query(db *gorm.DB, input DownloadTaskListQuery) *gorm.DB {
	query := db.Model(&model.DownloadTask{}).Where("deleted_at IS NULL")
	if input.ParentTaskID > 0 {
		query = query.Where("parent_task_id = ?", input.ParentTaskID)
	}
	if input.RootTaskID > 0 {
		query = query.Where("root_task_id = ?", input.RootTaskID)
	}
	return query
}

func (r *DataQueryService) discover_log_files() []data_log_file_info {
	log_path := strings.TrimSpace(r.log_path)
	if log_path == "" {
		return nil
	}
	if !filepath.IsAbs(log_path) && r.work_dir != "" {
		log_path = filepath.Join(r.work_dir, log_path)
	}
	info, err := os.Stat(log_path)
	if err != nil || info.IsDir() {
		return nil
	}
	return []data_log_file_info{{
		Name: filepath.Base(log_path),
		Path: log_path,
		Size: info.Size(),
	}}
}

func data_scan_tail_log_lines(ctx context.Context, path string, max_bytes int, visit_line func(string)) error {
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
		if err := ctx.Err(); err != nil {
			return err
		}
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

func data_parse_log_line(file data_log_file_info, raw string) data_log_entry {
	entry := data_log_entry{
		Source:  data_source_from_log_file(file.Name),
		Level:   "info",
		Message: raw,
		Raw:     raw,
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &object); err == nil {
		if value := data_log_string_field(object, "time", "timestamp"); value != "" {
			entry.Time = value
		}
		if value := data_log_string_field(object, "level"); value != "" {
			entry.Level = strings.ToLower(value)
		}
		if value := data_log_string_field(object, "message", "msg"); value != "" {
			entry.Message = value
		}
		if value := data_log_string_field(object, "file"); value != "" {
			entry.File = value
		}
		if value := data_log_string_field(object, "component"); value != "" {
			entry.Component = value
		}
		if value := data_log_string_field(object, "service", "component", "Client"); value != "" {
			entry.Source = value
		}
		return entry
	}
	entry.Level = data_infer_text_log_level(raw)
	return entry
}

func data_log_string_field(object map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key]; ok && value != nil {
			return strings.TrimSpace(strings.Trim(strings.TrimSpace(fmt.Sprint(value)), `"`))
		}
	}
	for _, key := range keys {
		for actual, value := range object {
			if strings.EqualFold(actual, key) && value != nil {
				return strings.TrimSpace(strings.Trim(strings.TrimSpace(fmt.Sprint(value)), `"`))
			}
		}
	}
	return ""
}

func data_source_from_log_file(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func data_infer_text_log_level(raw string) string {
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

func data_match_log_entry(entry data_log_entry, levels map[string]bool, keyword string, source string) bool {
	if len(levels) > 0 {
		if !levels[entry.Level] && !levels[strings.ToLower(entry.Level)] {
			return false
		}
	}
	if source != "" &&
		source != "all" &&
		!data_contains_normalized_log_text(entry.Source, source) &&
		!data_contains_normalized_log_text(entry.File, source) &&
		!data_contains_normalized_log_text(entry.Component, source) {
		return false
	}
	if keyword == "" {
		return true
	}
	if data_contains_normalized_log_text(entry.Raw, keyword) {
		return true
	}
	if entry.Message != entry.Raw && data_contains_normalized_log_text(entry.Message, keyword) {
		return true
	}
	return data_contains_normalized_log_text(entry.Source, keyword) ||
		data_contains_normalized_log_text(entry.File, keyword) ||
		data_contains_normalized_log_text(entry.Component, keyword)
}

func data_contains_normalized_log_text(text string, normalized_query string) bool {
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

func normalize_data_query_page(page int, page_size int, default_page_size int, maximum_page_size int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if page_size == 0 {
		page_size = default_page_size
	}
	if page < 1 {
		return 0, 0, fmt.Errorf("page 必须是正整数")
	}
	if page_size < 1 || page_size > maximum_page_size {
		return 0, 0, fmt.Errorf("page_size 必须在 1 到 %d 之间", maximum_page_size)
	}
	return page, page_size, nil
}

func data_file_type_by_ext(name string) string {
	extension := strings.ToLower(filepath.Ext(name))
	switch extension {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return "image"
	case ".mp4", ".mkv", ".avi", ".mov", ".webm":
		return "video"
	case ".mp3", ".aac", ".ogg", ".wav", ".flac":
		return "audio"
	case ".html", ".htm":
		return "html"
	case ".zip":
		return "zip"
	case ".pdf":
		return "pdf"
	default:
		return "other"
	}
}

func data_api_file_url(path string) string {
	values := url.Values{}
	values.Set("path", path)
	return "/api/file?" + values.Encode()
}
