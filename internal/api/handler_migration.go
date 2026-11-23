//go:build ignore

package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ltaoo/velo/fileserver"
	result "wx_channel/internal/apiresult"
)

// MigrationLoadRequest is the request to load a database.
type MigrationLoadRequest struct {
	DBPath string `json:"db_path"`
}

// MigrationTableRequest is the request to query table data.
type MigrationTableRequest struct {
	DBPath   string `json:"db_path"`
	Table    string `json:"table"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// handle_migration_load opens a SQLite database at the given path and returns a list of tables with record counts.
// POST /api/v1/migration/load
func (c *APIClient) handle_migration_load(ctx *gin.Context) {
	var req MigrationLoadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "请求参数无效: "+err.Error())
		return
	}

	db_path := sanitize_db_path(req.DBPath)
	if db_path == "" {
		result.Err(ctx, 400, "数据库路径不能为空")
		return
	}

	if _, err := os.Stat(db_path); err != nil {
		result.Err(ctx, 400, "数据库文件不存在或无法访问: "+err.Error())
		return
	}

	db, err := open_external_db(db_path)
	if err != nil {
		result.Err(ctx, 500, "打开数据库失败: "+err.Error())
		return
	}
	defer close_external_db(db)

	tables, err := load_table_list(db)
	if err != nil {
		result.Err(ctx, 500, "读取表列表失败: "+err.Error())
		return
	}

	result.Ok(ctx, gin.H{
		"database": filepath.Base(db_path),
		"path":     db_path,
		"tables":   tables,
	})
}

// handle_migration_table queries data from a specified table (paginated).
// POST /api/v1/migration/table
func (c *APIClient) handle_migration_table(ctx *gin.Context) {
	var req MigrationTableRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "请求参数无效: "+err.Error())
		return
	}

	db_path := sanitize_db_path(req.DBPath)
	if db_path == "" {
		result.Err(ctx, 400, "数据库路径不能为空")
	}

	table := strings.TrimSpace(req.Table)
	if table == "" {
		result.Err(ctx, 400, "表名不能为空")
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 1000 {
		req.PageSize = 200
	}

	if _, err := os.Stat(db_path); err != nil {
		result.Err(ctx, 400, "数据库文件不存在或无法访问")
		return
	}

	db, err := open_external_db(db_path)
	if err != nil {
		result.Err(ctx, 500, "打开数据库失败: "+err.Error())
		return
	}
	defer close_external_db(db)

	// Verify table exists
	var table_count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", table).Scan(&table_count).Error; err != nil || table_count == 0 {
		result.Err(ctx, 404, "表不存在: "+table)
		return
	}

	// Column info
	columns, err := load_table_columns(db, table)
	if err != nil {
		result.Err(ctx, 500, "读取列信息失败: "+err.Error())
		return
	}

	// Total row count
	var total_rows int64
	if err := db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", escape_ident(table))).Scan(&total_rows).Error; err != nil {
		result.Err(ctx, 500, "查询行数失败: "+err.Error())
		return
	}

	// Paginated data
	offset := (req.Page - 1) * req.PageSize
	rows, err := query_table_rows(db, table, req.PageSize, offset)
	if err != nil {
		result.Err(ctx, 500, "查询数据失败: "+err.Error())
		return
	}

	total_pages := int64(1)
	if req.PageSize > 0 {
		total_pages = (total_rows + int64(req.PageSize) - 1) / int64(req.PageSize)
	}

	result.Ok(ctx, gin.H{
		"table":       table,
		"columns":     columns,
		"rows":        rows,
		"total":       total_rows,
		"page":        req.Page,
		"page_size":   req.PageSize,
		"total_pages": total_pages,
	})
}

// handle_migration_file_list lists files in a directory (using velo fileserver).
// POST /api/v1/migration/file/list
func (c *APIClient) handle_migration_file_list(ctx *gin.Context) {
	var req struct {
		Dir string `json:"dir"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "请求参数无效: "+err.Error())
		return
	}

	dir := strings.TrimSpace(req.Dir)
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = home
	}

	files, _, err := fileserver.FetchFiles(fileserver.FetchFilesOption{
		Dir: dir,
	})
	if err != nil {
		result.Err(ctx, 500, "读取目录失败: "+err.Error())
		return
	}

	parent := filepath.Dir(dir)
	if parent == dir {
		parent = "" // root directory
	}

	result.Ok(ctx, gin.H{
		"dir":    dir,
		"parent": parent,
		"files":  files,
	})
}

// handle_migration_common_dirs returns a list of common directories.
// GET /api/v1/migration/common_dirs
func (c *APIClient) handle_migration_common_dirs(ctx *gin.Context) {
	dirs := fileserver.FetchCommonDirs()
	result.Ok(ctx, gin.H{
		"dirs": dirs,
	})
}

// ---- internal helpers ----

// sanitize_db_path cleans and validates a db file path.
func sanitize_db_path(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	clean := filepath.Clean(raw)
	if clean == "." || clean == ".." {
		return ""
	}
	return clean
}

// open_external_db opens a SQLite database at the given path.
func open_external_db(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// close_external_db closes an external SQLite database connection.
func close_external_db(db *gorm.DB) {
	if db == nil {
		return
	}
	sql_db, err := db.DB()
	if err != nil {
		return
	}
	_ = sql_db.Close()
}

// table_info holds summary metadata for a single table.
type table_info struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
}

// column_info holds metadata for a single column.
type column_info struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// load_table_list returns all user tables with their row counts.
func load_table_list(db *gorm.DB) ([]table_info, error) {
	var names []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&names).Error; err != nil {
		return nil, err
	}

	tables := make([]table_info, 0, len(names))
	for _, name := range names {
		var count int64
		if err := db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", escape_ident(name))).Scan(&count).Error; err != nil {
			// Table may be corrupted, skip
			continue
		}
		tables = append(tables, table_info{Name: name, Rows: count})
	}
	return tables, nil
}

// load_table_columns returns column metadata from PRAGMA table_info.
func load_table_columns(db *gorm.DB, table string) ([]column_info, error) {
	type pragma_row struct {
		Cid  int    `gorm:"column:cid"`
		Name string `gorm:"column:name"`
		Type string `gorm:"column:type"`
	}

	var raw []pragma_row
	sql := fmt.Sprintf("PRAGMA table_info(\"%s\")", escape_ident(table))
	if err := db.Raw(sql).Scan(&raw).Error; err != nil {
		return nil, err
	}

	cols := make([]column_info, 0, len(raw))
	for _, r := range raw {
		cols = append(cols, column_info{Name: r.Name, Type: r.Type})
	}
	return cols, nil
}

// query_table_rows fetches a page of rows from the given table.
// Returns a slice of map[string]interface{} for JSON serialization.
func query_table_rows(db *gorm.DB, table string, limit, offset int) ([]map[string]interface{}, error) {
	sql := fmt.Sprintf("SELECT * FROM \"%s\" LIMIT %d OFFSET %d", escape_ident(table), limit, offset)

	// Use raw query via gorm.Raw + Scan into a slice of maps.
	rows, err := db.Raw(sql).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		value_ptrs := make([]interface{}, len(columns))
		for i := range values {
			value_ptrs[i] = &values[i]
		}

		if err := rows.Scan(value_ptrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for JSON compatibility
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Ensure we never return nil (JSON serializer should output [])
	if results == nil {
		results = []map[string]interface{}{}
	}

	return results, nil
}

// escape_ident escapes a SQLite identifier (table or column name) by doubling any double-quote characters.
func escape_ident(s string) string {
	return strings.ReplaceAll(s, "\"", "\"\"")
}
