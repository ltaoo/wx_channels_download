package services

import (
	"bufio"
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
	"wx_channel/internal/mcpserver"
)

var default_mcp_browse_platform_ids = []string{
	"wxchannels",
	"wxmp",
	"zhihu",
	"xiaohongshu",
	"bilibili",
	"youtube",
	"weibo",
}

// MCPDataReaderConfig contains the application services and runtime paths used
// by the MCP read-only data tools.
type MCPDataReaderConfig struct {
	DB                      *gorm.DB
	DownloadTaskService     *DownloadTaskService
	BrowseHistoryService    *BrowseService
	LogPath                 string
	WorkDir                 string
	CertificateStatusReader func(context.Context) (any, error)
}

// MCPDataReader implements mcpserver.DataReader without depending on the API
// transport layer.
type MCPDataReader struct {
	db                        *gorm.DB
	download_task_service     *DownloadTaskService
	browse_history_service    *BrowseService
	log_path                  string
	work_dir                  string
	certificate_status_reader func(context.Context) (any, error)
}

type mcp_download_task_status_count struct {
	Status int   `gorm:"column:status"`
	Count  int64 `gorm:"column:count"`
}

type mcp_download_task_file struct {
	DownloadTaskFileRecord
	LocalPath string `json:"local_path"`
	FileType  string `json:"file_type"`
	FileURL   string `json:"file_url"`
	Exists    bool   `json:"exists"`
}

type mcp_log_entry struct {
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

type mcp_log_file_info struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type mcp_download_task_account_row struct {
	AccountID  string `gorm:"column:account_id"`
	Nickname   string `gorm:"column:nickname"`
	AvatarURL  string `gorm:"column:avatar_url"`
	ExternalID string `gorm:"column:external_id"`
}

type mcp_account_content_row struct {
	ContentID string `gorm:"column:content_id"`
	AccountID string `gorm:"column:account_id"`
	Role      string `gorm:"column:role"`
}

// NewMCPDataReader constructs the process-local MCP data reader.
func NewMCPDataReader(config MCPDataReaderConfig) *MCPDataReader {
	return &MCPDataReader{
		db:                        config.DB,
		download_task_service:     config.DownloadTaskService,
		browse_history_service:    config.BrowseHistoryService,
		log_path:                  strings.TrimSpace(config.LogPath),
		work_dir:                  strings.TrimSpace(config.WorkDir),
		certificate_status_reader: config.CertificateStatusReader,
	}
}

func (r *MCPDataReader) ListDownloadTasks(ctx context.Context, input mcpserver.DownloadTaskListQuery) (any, error) {
	if r == nil || r.db == nil || r.download_task_service == nil {
		return nil, errors.New("下载任务数据库服务未初始化")
	}
	db := r.db.WithContext(ctx)
	filtered_query := mcp_download_task_base_query(db, input)
	if len(input.Statuses) == 1 {
		filtered_query = filtered_query.Where("status = ?", input.Statuses[0])
	} else if len(input.Statuses) > 1 {
		filtered_query = filtered_query.Where("status IN ?", input.Statuses)
	}

	var total int64
	if err := filtered_query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("查询下载任务总数失败: %w", err)
	}

	var status_rows []mcp_download_task_status_count
	if err := mcp_download_task_base_query(db, input).
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

func (r *MCPDataReader) GetDownloadTaskDetail(ctx context.Context, task_id int) (any, error) {
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
			var account_rows []mcp_download_task_account_row
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

	files := make([]mcp_download_task_file, 0, len(record.Files))
	for _, file := range record.Files {
		local_path := filepath.Join(file.DownloadDir, file.Name)
		_, stat_err := os.Stat(local_path)
		files = append(files, mcp_download_task_file{
			DownloadTaskFileRecord: file,
			LocalPath:              local_path,
			FileType:               mcp_file_type_by_ext(file.Name),
			FileURL:                mcp_api_file_url(local_path),
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

func (r *MCPDataReader) ListAccounts(ctx context.Context, input mcpserver.AccountListQuery) (any, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("数据库未初始化")
	}
	db := r.db.WithContext(ctx)
	account_query := db.Model(&model.Account{})
	if input.AccountID != "" {
		account_query = account_query.Where("id = ?", input.AccountID)
	}
	if input.Keyword != "" {
		pattern := "%" + input.Keyword + "%"
		account_query = account_query.Where(
			"id LIKE ? OR external_id LIKE ? OR alias LIKE ? OR nickname LIKE ?",
			pattern,
			pattern,
			pattern,
			pattern,
		)
	}

	var total int64
	if err := account_query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("查询账号总数失败: %w", err)
	}
	page_count := 1
	if total > 0 {
		page_count = int((total + int64(input.PageSize) - 1) / int64(input.PageSize))
	}
	page := input.Page
	if page > page_count {
		page = page_count
	}

	var accounts []model.Account
	if err := account_query.
		Order("created_at DESC, id DESC").
		Limit(input.PageSize).
		Offset((page - 1) * input.PageSize).
		Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("查询账号失败: %w", err)
	}

	list := make([]map[string]any, 0, len(accounts))
	for _, account := range accounts {
		var content_rows []mcp_account_content_row
		_ = db.Table("content_account").
			Select("content_account.content_id, content_account.account_id, content_account.role").
			Joins("JOIN content ON content.id = content_account.content_id").
			Where("content_account.account_id = ?", account.Id).
			Order("COALESCE(content.publish_time, content.updated_at, content.created_at) DESC").
			Limit(24).
			Scan(&content_rows).Error

		var content_count int64
		_ = db.Table("content_account").Where("account_id = ?", account.Id).Count(&content_count).Error
		content_accounts := make([]map[string]any, 0, len(content_rows))
		for _, row := range content_rows {
			content_accounts = append(content_accounts, map[string]any{
				"content_id": row.ContentID,
				"account_id": row.AccountID,
				"role":       row.Role,
			})
		}
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
			"content_count":    content_count,
			"has_content":      content_count > 0,
			"content_accounts": content_accounts,
		})
	}
	return map[string]any{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": input.PageSize,
	}, nil
}

func (r *MCPDataReader) ListBrowseHistory(ctx context.Context, input mcpserver.BrowseHistoryListQuery) (any, error) {
	if r == nil || r.browse_history_service == nil {
		return nil, errors.New("浏览记录数据库服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	platform_ids := input.PlatformIDs
	if len(platform_ids) == 0 {
		platform_ids = append([]string(nil), default_mcp_browse_platform_ids...)
	}
	var username *string
	if input.Username != "" {
		username_value := input.Username
		username = &username_value
	}
	result, err := r.browse_history_service.ListPlatforms(
		platform_ids,
		username,
		input.Page,
		input.PageSize,
		input.Keyword,
	)
	if err != nil {
		return nil, fmt.Errorf("查询浏览记录失败: %w", err)
	}
	return result, nil
}

func (r *MCPDataReader) ListLogs(ctx context.Context, input mcpserver.LogListQuery) (any, error) {
	if r == nil {
		return nil, errors.New("配置未初始化")
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
	entries := make([]mcp_log_entry, 0, input.PageSize)
	total := 0
	sequence := 0
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lines, err := mcp_tail_log_lines(file.Path, input.MaxBytes)
		if err != nil {
			continue
		}
		for _, line := range lines {
			entry := mcp_parse_log_line(file, line, input.FormatJSON)
			if !mcp_match_log_entry(entry, levels, keyword, source) {
				continue
			}
			sequence++
			entry.Index = sequence
			total++
			entries = append(entries, entry)
		}
	}

	sort.SliceStable(entries, func(first_index int, second_index int) bool {
		first_time, first_err := time.Parse(time.RFC3339Nano, entries[first_index].Time)
		second_time, second_err := time.Parse(time.RFC3339Nano, entries[second_index].Time)
		if first_err == nil && second_err == nil && !first_time.Equal(second_time) {
			return first_time.After(second_time)
		}
		return entries[first_index].Index > entries[second_index].Index
	})
	page_count := 1
	if total > 0 {
		page_count = (total + input.PageSize - 1) / input.PageSize
	}
	page := input.Page
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

func (r *MCPDataReader) GetCertificateStatus(ctx context.Context) (any, error) {
	if r == nil || r.certificate_status_reader == nil {
		return nil, errors.New("证书状态服务未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.certificate_status_reader(ctx)
}

func mcp_download_task_base_query(db *gorm.DB, input mcpserver.DownloadTaskListQuery) *gorm.DB {
	query := db.Model(&model.DownloadTask{}).Where("deleted_at IS NULL")
	if input.ParentTaskID > 0 {
		query = query.Where("parent_task_id = ?", input.ParentTaskID)
	}
	if input.RootTaskID > 0 {
		query = query.Where("root_task_id = ?", input.RootTaskID)
	}
	return query
}

func (r *MCPDataReader) discover_log_files() []mcp_log_file_info {
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
	return []mcp_log_file_info{{
		Name: filepath.Base(log_path),
		Path: log_path,
		Size: info.Size(),
	}}
}

func mcp_tail_log_lines(path string, max_bytes int) ([]string, error) {
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

func mcp_parse_log_line(file mcp_log_file_info, raw string, format_json bool) mcp_log_entry {
	entry := mcp_log_entry{
		Source:  mcp_source_from_log_file(file.Name),
		Level:   "info",
		Message: raw,
		Raw:     raw,
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &object); err == nil {
		entry.JSON = object
		if value := mcp_log_string_field(object, "time", "timestamp"); value != "" {
			entry.Time = value
		}
		if value := mcp_log_string_field(object, "level"); value != "" {
			entry.Level = strings.ToLower(value)
		}
		if value := mcp_log_string_field(object, "message", "msg"); value != "" {
			entry.Message = value
		}
		if value := mcp_log_string_field(object, "file"); value != "" {
			entry.File = value
		}
		if value := mcp_log_string_field(object, "component"); value != "" {
			entry.Component = value
		}
		if value := mcp_log_string_field(object, "service", "component", "Client"); value != "" {
			entry.Source = value
		}
		if format_json {
			if data, err := json.MarshalIndent(object, "", "  "); err == nil {
				entry.Formatted = string(data)
			}
		}
		return entry
	}
	entry.Level = mcp_infer_text_log_level(raw)
	return entry
}

func mcp_log_string_field(object map[string]interface{}, keys ...string) string {
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

func mcp_source_from_log_file(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func mcp_infer_text_log_level(raw string) string {
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

func mcp_match_log_entry(entry mcp_log_entry, levels map[string]bool, keyword string, source string) bool {
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

func mcp_file_type_by_ext(name string) string {
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

func mcp_api_file_url(path string) string {
	values := url.Values{}
	values.Set("path", path)
	return "/api/file?" + values.Encode()
}
