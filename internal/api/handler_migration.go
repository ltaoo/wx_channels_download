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
	result "wx_channel/internal/util"
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

// handleMigrationLoad opens a SQLite database at the given path and returns a list of tables with record counts.
// POST /api/v1/migration/load
func (c *APIClient) handleMigrationLoad(ctx *gin.Context) {
	var req MigrationLoadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "请求参数无效: "+err.Error())
		return
	}

	dbPath := sanitizeDBPath(req.DBPath)
	if dbPath == "" {
		result.Err(ctx, 400, "数据库路径不能为空")
		return
	}

	if _, err := os.Stat(dbPath); err != nil {
		result.Err(ctx, 400, "数据库文件不存在或无法访问: "+err.Error())
		return
	}

	db, err := openExternalDB(dbPath)
	if err != nil {
		result.Err(ctx, 500, "打开数据库失败: "+err.Error())
		return
	}
	defer closeExternalDB(db)

	tables, err := loadTableList(db)
	if err != nil {
		result.Err(ctx, 500, "读取表列表失败: "+err.Error())
		return
	}

	result.Ok(ctx, gin.H{
		"database": filepath.Base(dbPath),
		"path":     dbPath,
		"tables":   tables,
	})
}

// handleMigrationTable queries data from a specified table (paginated).
// POST /api/v1/migration/table
func (c *APIClient) handleMigrationTable(ctx *gin.Context) {
	var req MigrationTableRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "请求参数无效: "+err.Error())
		return
	}

	dbPath := sanitizeDBPath(req.DBPath)
	if dbPath == "" {
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

	if _, err := os.Stat(dbPath); err != nil {
		result.Err(ctx, 400, "数据库文件不存在或无法访问")
		return
	}

	db, err := openExternalDB(dbPath)
	if err != nil {
		result.Err(ctx, 500, "打开数据库失败: "+err.Error())
		return
	}
	defer closeExternalDB(db)

	// Verify table exists
	var tableCount int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", table).Scan(&tableCount).Error; err != nil || tableCount == 0 {
		result.Err(ctx, 404, "表不存在: "+table)
		return
	}

	// Column info
	columns, err := loadTableColumns(db, table)
	if err != nil {
		result.Err(ctx, 500, "读取列信息失败: "+err.Error())
		return
	}

	// Total row count
	var totalRows int64
	if err := db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", escapeIdent(table))).Scan(&totalRows).Error; err != nil {
		result.Err(ctx, 500, "查询行数失败: "+err.Error())
		return
	}

	// Paginated data
	offset := (req.Page - 1) * req.PageSize
	rows, err := queryTableRows(db, table, req.PageSize, offset)
	if err != nil {
		result.Err(ctx, 500, "查询数据失败: "+err.Error())
		return
	}

	totalPages := int64(1)
	if req.PageSize > 0 {
		totalPages = (totalRows + int64(req.PageSize) - 1) / int64(req.PageSize)
	}

	result.Ok(ctx, gin.H{
		"table":       table,
		"columns":     columns,
		"rows":        rows,
		"total":       totalRows,
		"page":        req.Page,
		"page_size":   req.PageSize,
		"total_pages": totalPages,
	})
}

// handleMigrationFileList lists files in a directory (using velo fileserver).
// POST /api/v1/migration/file/list
func (c *APIClient) handleMigrationFileList(ctx *gin.Context) {
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

// handleMigrationCommonDirs returns a list of common directories.
// GET /api/v1/migration/common_dirs
func (c *APIClient) handleMigrationCommonDirs(ctx *gin.Context) {
	dirs := fileserver.FetchCommonDirs()
	result.Ok(ctx, gin.H{
		"dirs": dirs,
	})
}

// ---- internal helpers ----

// sanitizeDBPath cleans and validates a db file path.
func sanitizeDBPath(raw string) string {
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

// openExternalDB opens a SQLite database at the given path.
func openExternalDB(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// closeExternalDB closes an external SQLite database connection.
func closeExternalDB(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	_ = sqlDB.Close()
}

// tableInfo holds summary metadata for a single table.
type tableInfo struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
}

// columnInfo holds metadata for a single column.
type columnInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// loadTableList returns all user tables with their row counts.
func loadTableList(db *gorm.DB) ([]tableInfo, error) {
	var names []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&names).Error; err != nil {
		return nil, err
	}

	tables := make([]tableInfo, 0, len(names))
	for _, name := range names {
		var count int64
		if err := db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", escapeIdent(name))).Scan(&count).Error; err != nil {
			// Table may be corrupted, skip
			continue
		}
		tables = append(tables, tableInfo{Name: name, Rows: count})
	}
	return tables, nil
}

// loadTableColumns returns column metadata from PRAGMA table_info.
func loadTableColumns(db *gorm.DB, table string) ([]columnInfo, error) {
	type pragmaRow struct {
		Cid  int    `gorm:"column:cid"`
		Name string `gorm:"column:name"`
		Type string `gorm:"column:type"`
	}

	var raw []pragmaRow
	sql := fmt.Sprintf("PRAGMA table_info(\"%s\")", escapeIdent(table))
	if err := db.Raw(sql).Scan(&raw).Error; err != nil {
		return nil, err
	}

	cols := make([]columnInfo, 0, len(raw))
	for _, r := range raw {
		cols = append(cols, columnInfo{Name: r.Name, Type: r.Type})
	}
	return cols, nil
}

// queryTableRows fetches a page of rows from the given table.
// Returns a slice of map[string]interface{} for JSON serialization.
func queryTableRows(db *gorm.DB, table string, limit, offset int) ([]map[string]interface{}, error) {
	sql := fmt.Sprintf("SELECT * FROM \"%s\" LIMIT %d OFFSET %d", escapeIdent(table), limit, offset)

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
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
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

// escapeIdent escapes a SQLite identifier (table or column name) by doubling any double-quote characters.
func escapeIdent(s string) string {
	return strings.ReplaceAll(s, "\"", "\"\"")
}
