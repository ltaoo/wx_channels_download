package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/ltaoo/velo"
	"github.com/ltaoo/velo/frontendserver"
	"gorm.io/gorm"

	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
)

//go:embed web
var embeddedFS embed.FS

const gopeedDBFile = "gopeed.db"
const defaultTargetBaseURL = "http://127.0.0.1:2022"
const defaultTargetDBFile = "data.db"
const profileCacheFile = ".gopeed_migration_profile_cache.json"
const platformWXChannels = "wxchannels"
const resourceStatusFinished = 2
const mediaTypePicture = 2
const mediaTypeVideo = 4
const mediaTypeLive = 9

type apiResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

type tableInfo struct {
	Name string `json:"name"`
	Rows int    `json:"rows"`
}

type columnInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type migrationLoadRequest struct {
	DataDir  string `json:"data_dir"`
	DBPath   string `json:"db_path"`
	Status   string `json:"status"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type migrationTableRequest struct {
	DataDir  string `json:"data_dir"`
	DBPath   string `json:"db_path"`
	Table    string `json:"table"`
	Status   string `json:"status"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type migrationExecuteRequest struct {
	DataDir      string   `json:"data_dir"`
	DBPath       string   `json:"db_path"`
	Status       string   `json:"status"`
	TargetURL    string   `json:"target_url"`
	TargetDB     string   `json:"target_db"`
	Limit        int      `json:"limit"`
	DryRun       bool     `json:"dry_run"`
	AllowMissing bool     `json:"allow_missing"`
	TaskIDs      []string `json:"task_ids"`
}

type taskQueryResult struct {
	Database   string           `json:"database"`
	DataDir    string           `json:"data_dir"`
	DBPath     string           `json:"db_path"`
	Tables     []tableInfo      `json:"tables"`
	Table      string           `json:"table"`
	Columns    []columnInfo     `json:"columns"`
	Rows       []map[string]any `json:"rows"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
	Stats      map[string]int   `json:"stats"`
	Status     string           `json:"status"`
}

type migrationExecuteResult struct {
	DataDir   string                 `json:"data_dir"`
	DBPath    string                 `json:"db_path"`
	TargetURL string                 `json:"target_url"`
	TargetDB  string                 `json:"target_db"`
	DryRun    bool                   `json:"dry_run"`
	Total     int                    `json:"total"`
	Success   int                    `json:"success"`
	Skipped   int                    `json:"skipped"`
	Failed    int                    `json:"failed"`
	Items     []migrationExecuteItem `json:"items"`
}

type migrationExecuteItem struct {
	TaskID          string `json:"task_id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	OID             string `json:"oid"`
	UID             string `json:"uid"`
	SavePath        string `json:"save_path"`
	ProfileURL      string `json:"profile_url,omitempty"`
	ProfileCacheKey string `json:"profile_cache_key,omitempty"`
	ProfileCacheHit bool   `json:"profile_cache_hit,omitempty"`
	Action          string `json:"action"`
	TargetID        int    `json:"target_id,omitempty"`
	Error           string `json:"error,omitempty"`
}

type fileListRequest struct {
	Dir string `json:"dir"`
}

type fileItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDir       bool   `json:"isDir"`
	IsDirectory bool   `json:"isDirectory"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	ModTime     string `json:"modTime"`
}

type migrationServer struct {
	defaultDataDir string
	targetBaseURL  string
	targetDBPath   string
	profileCacheMu sync.Mutex
}

var taskColumns = []columnInfo{
	{Name: "id", Type: "TEXT"},
	{Name: "name", Type: "TEXT"},
	{Name: "status", Type: "TEXT"},
	{Name: "protocol", Type: "TEXT"},
	{Name: "url", Type: "TEXT"},
	{Name: "save_path", Type: "TEXT"},
	{Name: "size", Type: "INTEGER"},
	{Name: "downloaded", Type: "INTEGER"},
	{Name: "progress", Type: "REAL"},
	{Name: "oid", Type: "TEXT"},
	{Name: "uid", Type: "TEXT"},
	{Name: "external_id", Type: "TEXT"},
	{Name: "nonce_id", Type: "TEXT"},
	{Name: "title", Type: "TEXT"},
	{Name: "labels", Type: "JSON"},
	{Name: "created_at", Type: "TEXT"},
	{Name: "updated_at", Type: "TEXT"},
	{Name: "error", Type: "TEXT"},
}

func main() {
	var host string
	var port int
	var dataDir string
	var targetURL string
	var targetDB string
	var mode string
	var frontendRoot string
	flag.StringVar(&host, "host", "127.0.0.1", "HTTP listen host")
	flag.IntVar(&port, "port", 8026, "HTTP listen port")
	flag.StringVar(&dataDir, "data-dir", ".", "default directory containing gopeed.db")
	flag.StringVar(&targetURL, "target-url", defaultTargetBaseURL, "wx_channels_download target API base URL")
	flag.StringVar(&targetDB, "target-db", "", "wx_channels_download target SQLite database path")
	flag.StringVar(&mode, "mode", "dev", "frontend mode: dev, release, or prod")
	flag.StringVar(&frontendRoot, "frontend-root", "", "frontend root directory in dev mode")
	flag.Parse()

	absDataDir, err := filepath.Abs(expandHome(dataDir))
	if err != nil {
		log.Fatalf("resolve data dir: %v", err)
	}
	targetDBPath, err := resolveTargetDBPath(targetDB)
	if err != nil {
		log.Fatalf("resolve target db: %v", err)
	}
	server := &migrationServer{
		defaultDataDir: absDataDir,
		targetBaseURL:  normalizeTargetBaseURL(targetURL),
		targetDBPath:   targetDBPath,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/migration/load", server.handleMigrationLoad)
	mux.HandleFunc("/api/v1/migration/table", server.handleMigrationTable)
	mux.HandleFunc("/api/v1/migration/execute", server.handleMigrationExecute)
	mux.HandleFunc("/api/v1/migration/file/list", server.handleMigrationFileList)
	mux.HandleFunc("/api/v1/fs/list", server.handleMigrationFileList)
	mux.HandleFunc("/api/v1/migration/common_dirs", server.handleMigrationCommonDirs)
	mux.HandleFunc("/api/channels/feed/profile", server.handleChannelsFeedProfile)
	mux.Handle("/", newMigrationFrontend(mode, frontendRoot))

	addr := fmt.Sprintf("%s:%d", host, port)
	log.Printf("Gopeed migration viewer listening on http://%s/migration", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func newMigrationFrontend(mode string, frontendRoot string) http.Handler {
	normalizedMode := strings.ToLower(strings.TrimSpace(mode))
	serverMode := frontendserver.ModeDev
	staticAssetPrefixes := []string(nil)
	root := ""
	var embedded embed.FS

	switch normalizedMode {
	case "", "dev", "development":
		root = resolveFrontendRoot(frontendRoot)
	case "release", "prod", "production":
		serverMode = frontendserver.ModeProd
		staticAssetPrefixes = []string{"/assets"}
		root = "web"
		embedded = embeddedFS
	default:
		log.Fatalf("unsupported frontend mode %q; use dev, release, or prod", mode)
	}

	return frontendserver.New(frontendserver.Options{
		Mode:                serverMode,
		Root:                root,
		Embedded:            embedded,
		EntryPage:           "migration.html",
		StaticAssetPrefixes: staticAssetPrefixes,
		NoFallbackPrefixes:  []string{"/api", "/assets"},
	})
}

func resolveFrontendRoot(frontendRoot string) string {
	if strings.TrimSpace(frontendRoot) != "" {
		root, err := filepath.Abs(expandHome(frontendRoot))
		if err != nil {
			log.Fatalf("resolve frontend root: %v", err)
		}
		if !isFrontendRoot(root) {
			log.Fatalf("frontend root %q does not contain migration.html", root)
		}
		return root
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("resolve working directory: %v", err)
	}
	candidates := []string{
		filepath.Join(wd, "web"),
		filepath.Join(wd, "gopeed-migration", "web"),
		filepath.Join(filepath.Dir(wd), "gopeed-migration", "web"),
	}
	for _, candidate := range candidates {
		if isFrontendRoot(candidate) {
			return candidate
		}
	}
	log.Fatalf("frontend root not found; pass --frontend-root /path/to/gopeed-migration/web")
	return ""
}

func isFrontendRoot(root string) bool {
	info, err := os.Stat(filepath.Join(root, "migration.html"))
	return err == nil && !info.IsDir()
}

func resolveTargetDBPath(raw string) (string, error) {
	raw = strings.TrimSpace(expandHome(raw))
	if raw != "" {
		return normalizeTargetDBPath(raw)
	}
	if env := strings.TrimSpace(os.Getenv("WX_CHANNEL_DB_PATH")); env != "" {
		return normalizeTargetDBPath(env)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	candidates := []string{
		filepath.Join(wd, defaultTargetDBFile),
		filepath.Join(filepath.Dir(wd), defaultTargetDBFile),
		filepath.Join(wd, "data", defaultTargetDBFile),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Dir(candidate)); err == nil && info.IsDir() {
			return filepath.Abs(candidate)
		}
	}
	return "", fmt.Errorf("target database path not found; pass --target-db /path/to/%s", defaultTargetDBFile)
}

func normalizeTargetDBPath(raw string) (string, error) {
	clean := filepath.Clean(expandHome(raw))
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve target database path: %w", err)
	}
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		abs = filepath.Join(abs, defaultTargetDBFile)
	}
	parent := filepath.Dir(abs)
	info, err := os.Stat(parent)
	if err != nil {
		return "", fmt.Errorf("target database directory is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("target database parent is not a directory: %s", parent)
	}
	return abs, nil
}

func openTargetDB(dbPath string) (*gorm.DB, error) {
	app := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	if err := app.Migrate(&velo.VeloDatabaseOpt{DBType: velo.DBTypeSQLite, DBPath: dbPath, Migrations: &database.Migrations}); err != nil {
		return nil, fmt.Errorf("target database initialization failed: %w", err)
	}
	if err := database.ConfigureSQLiteRuntime(app.DB); err != nil {
		return nil, fmt.Errorf("target database sqlite configuration failed: %w", err)
	}
	return app.DB, nil
}

func closeTargetDB(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}

func (s *migrationServer) handleMigrationLoad(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req migrationLoadRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, 400, "invalid request body: "+err.Error())
		return
	}
	result, err := s.queryTasks(firstNonEmpty(req.DataDir, req.DBPath), req.Status, req.Page, req.PageSize)
	if err != nil {
		writeAPIError(w, 500, err.Error())
		return
	}
	writeAPIOK(w, result)
}

func (s *migrationServer) handleMigrationTable(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req migrationTableRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, 400, "invalid request body: "+err.Error())
		return
	}
	table := strings.TrimSpace(req.Table)
	if table == "" {
		table = "task"
	}
	if table != "task" && table != "tasks" {
		writeAPIError(w, 404, "table not found: "+table)
		return
	}
	result, err := s.queryTasks(firstNonEmpty(req.DataDir, req.DBPath), req.Status, req.Page, req.PageSize)
	if err != nil {
		writeAPIError(w, 500, err.Error())
		return
	}
	writeAPIOK(w, result)
}

func (s *migrationServer) handleMigrationExecute(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req migrationExecuteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, 400, "invalid request body: "+err.Error())
		return
	}
	result, err := s.executeMigration(req)
	if err != nil {
		writeAPIError(w, 500, err.Error())
		return
	}
	writeAPIOK(w, result)
}

func (s *migrationServer) handleMigrationFileList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req fileListRequest
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, 400, "invalid request body: "+err.Error())
		return
	}
	dir, files, err := listFiles(req.Dir)
	if err != nil {
		writeAPIError(w, 500, err.Error())
		return
	}
	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}
	writeAPIOK(w, map[string]any{
		"dir":    dir,
		"parent": parent,
		"files":  files,
		"items":  files,
		"list":   files,
	})
}

func (s *migrationServer) handleMigrationCommonDirs(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeAPIOK(w, map[string]any{
		"default_data_dir": s.defaultDataDir,
		"target_url":       s.targetBaseURL,
		"target_db":        s.targetDBPath,
		"dirs":             commonDirs(s.defaultDataDir),
	})
}

func (s *migrationServer) handleChannelsFeedProfile(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	query := r.URL.Query()
	oid := strings.TrimSpace(query.Get("oid"))
	uid := firstNonEmpty(query.Get("uid"), query.Get("nid"), query.Get("nonce_id"))
	if oid == "" {
		writeAPIError(w, 400, "missing oid")
		return
	}
	cacheDir, err := s.resolveProfileCacheDir(firstNonEmpty(query.Get("data_dir"), query.Get("db_path")))
	if err != nil {
		writeAPIError(w, 400, err.Error())
		return
	}
	cacheKey := firstNonEmpty(query.Get("cache_key"), oid)
	targetBaseURL := normalizeTargetBaseURL(firstNonEmpty(query.Get("target_url"), s.targetBaseURL, defaultTargetBaseURL))
	profile, profileURL, cached, err := s.fetchProfileWithCache(&http.Client{Timeout: 30 * time.Second}, cacheDir, targetBaseURL, cacheKey, oid, uid)
	if err != nil {
		writeAPIError(w, 502, err.Error())
		return
	}
	writeAPIOK(w, map[string]any{
		"profile_url": profileURL,
		"cache_key":   profileCacheKey(cacheKey, oid),
		"cached":      cached,
		"profile":     profile,
	})
}

func (s *migrationServer) executeMigration(req migrationExecuteRequest) (*migrationExecuteResult, error) {
	dataDir, dbPath, err := s.resolveDataDir(firstNonEmpty(req.DataDir, req.DBPath))
	if err != nil {
		return nil, err
	}
	status := normalizeStatus(req.Status)
	tasks, _, err := loadGopeedTasks(dataDir, status)
	if err != nil {
		return nil, err
	}
	if len(req.TaskIDs) > 0 {
		taskIDs := make(map[string]bool, len(req.TaskIDs))
		for _, id := range req.TaskIDs {
			if id = strings.TrimSpace(id); id != "" {
				taskIDs[id] = true
			}
		}
		filtered := make([]*downloadpkg.Task, 0, len(tasks))
		for _, task := range tasks {
			if taskIDs[safeTaskID(task)] {
				filtered = append(filtered, task)
			}
		}
		tasks = filtered
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return taskCreatedAt(tasks[i]).Before(taskCreatedAt(tasks[j]))
	})
	if req.Limit > 0 && req.Limit < len(tasks) {
		tasks = tasks[:req.Limit]
	}

	targetBaseURL := normalizeTargetBaseURL(firstNonEmpty(req.TargetURL, s.targetBaseURL, defaultTargetBaseURL))
	targetDBPath, err := resolveTargetDBPath(firstNonEmpty(req.TargetDB, s.targetDBPath))
	if err != nil {
		return nil, err
	}
	var targetDB *gorm.DB
	if !req.DryRun {
		targetDB, err = openTargetDB(targetDBPath)
		if err != nil {
			return nil, err
		}
		defer closeTargetDB(targetDB)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	result := &migrationExecuteResult{
		DataDir:   dataDir,
		DBPath:    dbPath,
		TargetURL: targetBaseURL,
		TargetDB:  targetDBPath,
		DryRun:    req.DryRun,
		Total:     len(tasks),
		Items:     make([]migrationExecuteItem, 0, len(tasks)),
	}
	for _, task := range tasks {
		item := s.migrateOneTask(client, targetDB, targetBaseURL, dataDir, task, req.DryRun, req.AllowMissing)
		switch item.Action {
		case "migrated", "dry_run":
			result.Success++
		case "skipped":
			result.Skipped++
		default:
			result.Failed++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *migrationServer) queryTasks(rawPath string, status string, page int, pageSize int) (*taskQueryResult, error) {
	dataDir, dbPath, err := s.resolveDataDir(rawPath)
	if err != nil {
		return nil, err
	}
	page, pageSize = normalizePagination(page, pageSize)

	filtered, all, err := loadGopeedTasks(dataDir, normalizeStatus(status))
	if err != nil {
		return nil, err
	}
	stats := buildStatusStats(all)
	total := len(filtered)
	totalPages := 1
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	rows := make([]map[string]any, 0, end-start)
	for _, task := range filtered[start:end] {
		rows = append(rows, taskToRow(task))
	}
	return &taskQueryResult{
		Database:   gopeedDBFile,
		DataDir:    dataDir,
		DBPath:     dbPath,
		Tables:     []tableInfo{{Name: "task", Rows: len(all)}},
		Table:      "task",
		Columns:    taskColumns,
		Rows:       rows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		Stats:      stats,
		Status:     normalizeStatus(status),
	}, nil
}

type targetAPIEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type channelsFeedProfile struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type channelsObjectLite struct {
	ID            flexibleString       `json:"id"`
	Contact       channelsContactLite  `json:"contact"`
	ObjectDesc    channelsObjectDesc   `json:"objectDesc"`
	ObjectNonceID flexibleString       `json:"objectNonceId"`
	SourceURL     string               `json:"source_url"`
	CreateTime    flexibleInt64        `json:"createtime"`
	Type          string               `json:"type"`
	Spec          []channelsMediaSpec  `json:"spec"`
	Files         []channelsMediaItem  `json:"files"`
	LiveInfo      json.RawMessage      `json:"liveInfo"`
	AnchorContact *channelsContactLite `json:"anchorContact"`
}

type channelsContactLite struct {
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	HeadURL     string `json:"headUrl"`
	Signature   string `json:"signature"`
	CoverImgURL string `json:"coverImgUrl"`
}

type channelsObjectDesc struct {
	Description string              `json:"description"`
	Media       []channelsMediaItem `json:"media"`
	MediaType   int                 `json:"mediaType"`
}

type channelsMediaItem struct {
	URL          string              `json:"url"`
	URLToken     string              `json:"urlToken"`
	ThumbURL     string              `json:"thumbUrl"`
	CoverURL     string              `json:"coverUrl"`
	DecodeKey    flexibleString      `json:"decodeKey"`
	MediaType    int                 `json:"mediaType"`
	VideoPlayLen flexibleInt64       `json:"videoPlayLen"`
	Width        flexibleFloat64     `json:"width"`
	Height       flexibleFloat64     `json:"height"`
	FileSize     flexibleInt64       `json:"fileSize"`
	Spec         []channelsMediaSpec `json:"spec"`
}

type channelsMediaSpec struct {
	FileFormat string          `json:"fileFormat"`
	Width      flexibleFloat64 `json:"width"`
	Height     flexibleFloat64 `json:"height"`
	DurationMS flexibleInt64   `json:"durationMs"`
}

type flexibleString string

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*s = ""
		return nil
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*s = flexibleString(value)
		return nil
	}
	*s = flexibleString(raw)
	return nil
}

type flexibleInt64 int64

func (n *flexibleInt64) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*n = 0
		return nil
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		raw = strings.TrimSpace(value)
		if raw == "" {
			*n = 0
			return nil
		}
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return err
	}
	*n = flexibleInt64(value)
	return nil
}

type flexibleFloat64 float64

func (n *flexibleFloat64) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*n = 0
		return nil
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		raw = strings.TrimSpace(value)
		if raw == "" {
			*n = 0
			return nil
		}
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return err
	}
	*n = flexibleFloat64(value)
	return nil
}

type duplicateImportError struct {
	TaskID int
}

func (e duplicateImportError) Error() string {
	return fmt.Sprintf("download task already exists: %d", e.TaskID)
}

type profileCacheDocument struct {
	Version int                          `json:"version"`
	Items   map[string]profileCacheEntry `json:"items"`
}

type profileCacheEntry struct {
	OID        string          `json:"oid"`
	UID        string          `json:"uid,omitempty"`
	ProfileURL string          `json:"profile_url"`
	Profile    json.RawMessage `json:"profile"`
	CachedAt   int64           `json:"cached_at"`
}

func (s *migrationServer) migrateOneTask(client *http.Client, db *gorm.DB, targetBaseURL string, dataDir string, task *downloadpkg.Task, dryRun bool, allowMissing bool) migrationExecuteItem {
	labels := taskLabels(task)
	oid := firstLabel(labels, "oid", "id", "external_id", "objectid", "object_id")
	uid := firstLabel(labels, "uid", "nid", "nonce_id", "objectNonceId", "object_nonce_id")
	savePath := taskFilePath(task, dataDir)
	cacheKey := taskProfileCacheKey(labels, oid)
	item := migrationExecuteItem{
		TaskID:          safeTaskID(task),
		Name:            taskName(task),
		Status:          string(taskStatus(task)),
		OID:             oid,
		UID:             uid,
		SavePath:        savePath,
		ProfileCacheKey: cacheKey,
	}
	if task == nil || task.Meta == nil || task.Meta.Req == nil {
		item.Action = "skipped"
		item.Error = "task meta/req is empty"
		return item
	}
	if oid == "" {
		item.Action = "skipped"
		item.Error = "missing oid label"
		return item
	}
	if savePath == "" {
		item.Action = "skipped"
		item.Error = "missing downloaded file path"
		return item
	}
	fileInfo, statErr := os.Stat(savePath)
	if statErr != nil && !allowMissing {
		item.Action = "failed"
		item.Error = "downloaded file is not accessible: " + statErr.Error()
		return item
	}
	if statErr == nil && fileInfo.IsDir() && !allowMissing {
		item.Action = "failed"
		item.Error = "downloaded file path is a directory"
		return item
	}

	profileURLString := buildTargetProfileURL(targetBaseURL, oid, uid).String()
	item.ProfileURL = profileURLString
	if dryRun {
		item.Action = "dry_run"
		return item
	}
	if db == nil {
		item.Action = "failed"
		item.Error = "target database is not opened"
		return item
	}
	if existing, err := findMigratedDownloadTask(db, safeTaskID(task)); err == nil {
		item.TargetID = existing.Id
		item.Action = "skipped"
		item.Error = "gopeed task already migrated"
		return item
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		item.Action = "failed"
		item.Error = err.Error()
		return item
	}

	profile, profileURLString, cached, err := s.fetchProfileWithCache(client, dataDir, targetBaseURL, cacheKey, oid, uid)
	if err != nil {
		item.Action = "failed"
		item.Error = err.Error()
		return item
	}
	item.ProfileURL = profileURLString
	item.ProfileCacheHit = cached
	targetID, err := importTargetDownloadRecord(db, task, labels, oid, uid, savePath, profile.Data.Object)
	if err != nil {
		var duplicate duplicateImportError
		if errors.As(err, &duplicate) {
			item.TargetID = duplicate.TaskID
			item.Action = "skipped"
			item.Error = err.Error()
			return item
		}
		item.Action = "failed"
		item.Error = err.Error()
		return item
	}
	item.TargetID = targetID
	item.Action = "migrated"
	return item
}

func fetchTargetProfile(client *http.Client, rawURL string) (*channelsFeedProfile, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch profile: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read profile response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("profile http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var envelope targetAPIEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse profile response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("profile api error code=%d: %s", envelope.Code, envelope.Msg)
	}
	var profile channelsFeedProfile
	if err := json.Unmarshal(envelope.Data, &profile); err != nil {
		return nil, fmt.Errorf("parse profile data: %w", err)
	}
	if profile.ErrCode != 0 {
		return nil, fmt.Errorf("profile business error code=%d: %s", profile.ErrCode, profile.ErrMsg)
	}
	if len(profile.Data.Object) == 0 || string(profile.Data.Object) == "null" {
		return nil, fmt.Errorf("profile object is empty")
	}
	return &profile, nil
}

func (s *migrationServer) fetchProfileWithCache(client *http.Client, dataDir string, targetBaseURL string, cacheKey string, oid string, uid string) (*channelsFeedProfile, string, bool, error) {
	cacheKey = profileCacheKey(cacheKey, oid)
	profileURL := buildTargetProfileURL(targetBaseURL, oid, uid).String()
	if cacheKey != "" {
		profile, cachedURL, hit, err := s.readProfileCache(dataDir, cacheKey)
		if err != nil {
			return nil, profileURL, false, err
		}
		if hit {
			if cachedURL == "" {
				cachedURL = profileURL
			}
			return profile, cachedURL, true, nil
		}
	}

	profile, err := fetchTargetProfile(client, profileURL)
	if err != nil {
		return nil, profileURL, false, err
	}
	if cacheKey != "" {
		if err := s.writeProfileCache(dataDir, cacheKey, profileCacheEntry{
			OID:        strings.TrimSpace(oid),
			UID:        cleanNonceID(uid),
			ProfileURL: profileURL,
			Profile:    mustRawJSON(profile),
			CachedAt:   time.Now().UnixMilli(),
		}); err != nil {
			return nil, profileURL, false, err
		}
	}
	return profile, profileURL, false, nil
}

func (s *migrationServer) readProfileCache(dataDir string, key string) (*channelsFeedProfile, string, bool, error) {
	s.profileCacheMu.Lock()
	defer s.profileCacheMu.Unlock()

	cache, err := loadProfileCache(profileCachePath(dataDir))
	if err != nil {
		return nil, "", false, err
	}
	entry, ok := cache.Items[profileCacheKey(key, "")]
	if !ok || len(entry.Profile) == 0 {
		return nil, "", false, nil
	}
	var profile channelsFeedProfile
	if err := json.Unmarshal(entry.Profile, &profile); err != nil {
		return nil, "", false, fmt.Errorf("parse profile cache: %w", err)
	}
	if len(profile.Data.Object) == 0 || strings.TrimSpace(string(profile.Data.Object)) == "null" {
		return nil, "", false, fmt.Errorf("profile cache object is empty for %s", key)
	}
	return &profile, entry.ProfileURL, true, nil
}

func (s *migrationServer) writeProfileCache(dataDir string, key string, entry profileCacheEntry) error {
	s.profileCacheMu.Lock()
	defer s.profileCacheMu.Unlock()

	path := profileCachePath(dataDir)
	cache, err := loadProfileCache(path)
	if err != nil {
		return err
	}
	if cache.Items == nil {
		cache.Items = map[string]profileCacheEntry{}
	}
	cache.Version = 1
	cache.Items[profileCacheKey(key, "")] = entry
	return saveProfileCache(path, cache)
}

func loadProfileCache(path string) (*profileCacheDocument, error) {
	cache := &profileCacheDocument{
		Version: 1,
		Items:   map[string]profileCacheEntry{},
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cache, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read profile cache: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return cache, nil
	}
	if err := json.Unmarshal(data, cache); err != nil {
		return nil, fmt.Errorf("parse profile cache %s: %w", path, err)
	}
	if cache.Items == nil {
		cache.Items = map[string]profileCacheEntry{}
	}
	return cache, nil
}

func saveProfileCache(path string, cache *profileCacheDocument) error {
	if cache == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create profile cache directory: %w", err)
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile cache: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write profile cache: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace profile cache: %w", err)
	}
	return nil
}

func importTargetDownloadRecord(db *gorm.DB, task *downloadpkg.Task, labels map[string]string, oid string, uid string, savePath string, objectRaw json.RawMessage) (int, error) {
	objectRaw = normalizeRawJSONObject(objectRaw)
	if len(objectRaw) == 0 {
		return 0, fmt.Errorf("profile object is empty")
	}
	var obj channelsObjectLite
	if err := json.Unmarshal(objectRaw, &obj); err != nil {
		return 0, fmt.Errorf("parse profile object: %w", err)
	}
	oid = firstNonEmpty(string(obj.ID), oid)
	if oid == "" {
		return 0, fmt.Errorf("profile object missing id")
	}
	obj.ID = flexibleString(oid)
	uid = firstNonEmpty(uid, string(obj.ObjectNonceID))

	media := firstObjectMedia(obj)
	contentID := buildTargetContentID(oid)
	downloadDir := filepath.Dir(savePath)
	filename := filepath.Base(savePath)
	if filename == "." || filename == string(filepath.Separator) {
		filename = taskName(task)
	}
	title := firstNonEmpty(labels["title"], obj.ObjectDesc.Description, migrationCreateFilename(filename), taskName(task), oid)
	taskUniqueID := migratedTaskUniqueID(oid, labels, savePath)
	contentType := migratedContentType(obj)
	contentURL := firstNonEmpty(mediaDownloadURL(media), taskURL(task))
	sourceURL := firstNonEmpty(obj.SourceURL, buildChannelsFallbackSourceURL(oid, uid, accountExternalID(obj, uid)), taskURL(task))
	coverURL := firstNonEmpty(media.CoverURL, media.ThumbURL, liveCoverURL(obj))
	size := migratedSize(task, savePath, media)
	downloaded := migratedDownloaded(task, size)
	targetStatus := mapGopeedStatusToTargetStatus(taskStatus(task))
	resourceStatus := 0
	var finishTime *int64
	now := time.Now().UnixMilli()
	createdAt := timeToMillisOrDefault(taskCreatedAt(task), now)
	updatedAt := timeToMillisOrDefault(taskUpdatedAt(task), now)
	if updatedAt < createdAt {
		updatedAt = createdAt
	}
	if targetStatus == model.TaskStatusFinished {
		resourceStatus = resourceStatusFinished
		finished := updatedAt
		finishTime = &finished
	}

	configJSON := buildMigrationTaskConfigJSON(downloadDir, filename, labels)
	metadataJSON := mustJSON(map[string]any{
		"gopeedid": safeTaskID(task),
	})

	content := buildTargetContent(contentID, oid, uid, title, contentType, sourceURL, contentURL, coverURL, media, obj, objectRaw, createdAt, updatedAt)
	account := buildTargetAccount(obj, uid, createdAt, updatedAt)
	taskRecord := model.DownloadTask{
		ContentId:    &contentID,
		Name:         firstNonEmpty(taskName(task), title, filename, oid),
		PlatformId:   platformWXChannels,
		UniqueID:     taskUniqueID,
		Status:       targetStatus,
		SourceURL:    sourceURL,
		CoverURL:     coverURL,
		CoverWidth:   content.CoverWidth,
		CoverHeight:  content.CoverHeight,
		ConfigJSON:   configJSON,
		MetadataJSON: metadataJSON,
		ErrorMessage: taskError(task),
		Timestamps: model.Timestamps{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	resourceExtra := mustJSON(map[string]any{
		"platform":   platformWXChannels,
		"id":         oid,
		"nonce_id":   uid,
		"title":      title,
		"author":     accountNickname(account),
		"decode_key": string(media.DecodeKey),
		"media_type": obj.ObjectDesc.MediaType,
		"gopeedid":   safeTaskID(task),
		"file_path":  savePath,
		"migrated":   true,
	})
	resourceName := filename
	if resourceName == "" || resourceName == "." {
		resourceName = firstNonEmpty(title, oid)
	}
	resourceRecord := model.DownloadResource{
		ContentId:   &contentID,
		DownloadDir: downloadDir,
		Name:        resourceName,
		Kind:        resourceKindForPath(savePath),
		UniqueID:    taskUniqueID,
		Type:        "file",
		Size:        size,
		Downloaded:  downloaded,
		Speed:       0,
		Status:      resourceStatus,
		MergeOrder:  0,
		Extra:       resourceExtra,
		FinishTime:  finishTime,
		Timestamps: model.Timestamps{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}
	if targetStatus == model.TaskStatusFinished {
		started := createdAt
		resourceRecord.StartTime = &started
	}
	endpointURL := firstNonEmpty(taskURL(task), mediaDownloadURL(media), savePath)
	endpointRecord := model.DownloadEndpoint{
		Protocol: endpointProtocol(endpointURL, taskProtocol(task)),
		URL:      endpointURL,
		Priority: 0,
		Enabled:  1,
		Status:   resourceStatus,
		Timestamps: model.Timestamps{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	var targetID int
	err := db.Transaction(func(tx *gorm.DB) error {
		if existing, err := findMigratedDownloadTask(tx, safeTaskID(task)); err == nil {
			targetID = existing.Id
			return duplicateImportError{TaskID: existing.Id}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := ensureTargetPlatform(tx, now); err != nil {
			return err
		}
		if err := tx.Save(content).Error; err != nil {
			return fmt.Errorf("save content: %w", err)
		}
		if err := saveTargetContentDetail(tx, content, contentType, obj, media, size, contentURL); err != nil {
			return err
		}
		if err := upsertTargetAccountAndLink(tx, content.Id, account, now); err != nil {
			return err
		}
		if err := database.ApplyTaskLineage(tx, &taskRecord, nil, ""); err != nil {
			return err
		}
		if err := tx.Create(&taskRecord).Error; err != nil {
			return fmt.Errorf("create download task: %w", err)
		}
		if err := database.FinalizeTaskRoot(tx, &taskRecord); err != nil {
			return err
		}
		taskID := taskRecord.Id
		resourceRecord.TaskId = &taskID
		if err := tx.Create(&resourceRecord).Error; err != nil {
			return fmt.Errorf("create download resource: %w", err)
		}
		endpointRecord.ResourceId = resourceRecord.Id
		if err := tx.Create(&endpointRecord).Error; err != nil {
			return fmt.Errorf("create download endpoint: %w", err)
		}
		targetID = taskRecord.Id
		return nil
	})
	return targetID, err
}

func normalizeRawJSONObject(raw json.RawMessage) json.RawMessage {
	text := strings.TrimSpace(string(raw))
	for text != "" && text[0] == '"' {
		var decoded string
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			break
		}
		text = strings.TrimSpace(decoded)
	}
	if text == "null" {
		return nil
	}
	return json.RawMessage(text)
}

func buildTargetContent(contentID string, oid string, uid string, title string, contentType string, sourceURL string, contentURL string, coverURL string, media channelsMediaItem, obj channelsObjectLite, objectRaw json.RawMessage, createdAt int64, updatedAt int64) *model.Content {
	metadata := mustJSON(map[string]any{
		"key":            string(media.DecodeKey),
		"profile_object": objectRaw,
		"migrated":       true,
		"uid":            uid,
	})
	content := &model.Content{
		Id:          contentID,
		PlatformId:  platformWXChannels,
		Type:        contentType,
		ExternalId:  oid,
		ExternalId2: uid,
		Title:       title,
		Description: obj.ObjectDesc.Description,
		URL:         contentURL,
		SourceURL:   sourceURL,
		CoverURL:    coverURL,
		CoverWidth:  dimensionString(media.Width),
		CoverHeight: dimensionString(media.Height),
		Metadata:    metadata,
		Timestamps: model.Timestamps{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}
	if obj.CreateTime > 0 {
		publishTime := int64(obj.CreateTime)
		content.PublishTime = &publishTime
	}
	return content
}

func buildTargetAccount(obj channelsObjectLite, fallbackUID string, createdAt int64, updatedAt int64) *model.Account {
	contact, externalID := targetAccountContact(obj)
	externalID = firstNonEmpty(externalID, fallbackUID)
	if externalID == "" {
		return nil
	}
	return &model.Account{
		Id:         buildTargetAccountID(externalID),
		PlatformId: platformWXChannels,
		ExternalId: externalID,
		Nickname:   contact.Nickname,
		Signature:  strings.TrimSpace(contact.Signature),
		AvatarURL:  contact.HeadURL,
		Timestamps: model.Timestamps{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}
}

func saveTargetContentDetail(tx *gorm.DB, content *model.Content, contentType string, obj channelsObjectLite, media channelsMediaItem, size int64, contentURL string) error {
	if content == nil {
		return nil
	}
	switch contentType {
	case "album":
		files := objectFiles(obj)
		album := model.ContentAlbum{
			Id:          content.Id,
			ImageCount:  len(files),
			CoverWidth:  int(math.Round(float64(media.Width))),
			CoverHeight: int(math.Round(float64(media.Height))),
			Description: content.Description,
		}
		if err := tx.Save(&album).Error; err != nil {
			return fmt.Errorf("save content album: %w", err)
		}
		var imageCount int64
		if err := tx.Model(&model.ContentImage{}).Where("album_id = ? AND deleted_at IS NULL", content.Id).Count(&imageCount).Error; err != nil {
			return fmt.Errorf("count content images: %w", err)
		}
		if imageCount > 0 {
			return nil
		}
		for i, file := range files {
			imageURL := mediaDownloadURL(file)
			if imageURL == "" {
				continue
			}
			image := model.ContentImage{
				AlbumId:   content.Id,
				SortOrder: i,
				URL:       imageURL,
				Width:     int(math.Round(float64(file.Width))),
				Height:    int(math.Round(float64(file.Height))),
				Size:      int64(file.FileSize),
				Ext:       fileExtensionFromURL(imageURL),
				ImageType: model.ContentImageTypeStill,
			}
			if err := tx.Create(&image).Error; err != nil {
				return fmt.Errorf("create content image: %w", err)
			}
		}
	case "video":
		videoSize := int64(media.FileSize)
		if videoSize <= 0 {
			videoSize = size
		}
		video := model.ContentVideo{
			Id:       content.Id,
			Duration: int64(media.VideoPlayLen),
			Width:    int(math.Round(float64(media.Width))),
			Height:   int(math.Round(float64(media.Height))),
			Size:     videoSize,
			Format:   fileExtensionFromURL(contentURL),
			URL:      contentURL,
		}
		if err := tx.Save(&video).Error; err != nil {
			return fmt.Errorf("save content video: %w", err)
		}
	}
	return nil
}

func upsertTargetAccountAndLink(tx *gorm.DB, contentID string, account *model.Account, now int64) error {
	if account == nil || account.ExternalId == "" {
		return nil
	}
	if account.Id == "" {
		account.Id = buildTargetAccountID(account.ExternalId)
	}
	if account.CreatedAt == 0 {
		account.CreatedAt = now
	}
	account.UpdatedAt = now
	if err := tx.Save(account).Error; err != nil {
		return fmt.Errorf("save account: %w", err)
	}
	if contentID == "" {
		return nil
	}
	association := model.ContentAccount{
		ContentId: contentID,
		AccountId: account.Id,
		Role:      "owner",
		CreatedAt: now,
	}
	var existing model.ContentAccount
	err := tx.Where("content_id = ? AND account_id = ?", association.ContentId, association.AccountId).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Create(&association).Error; err != nil {
			return fmt.Errorf("create content account: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("query content account: %w", err)
	}
	if existing.Role != association.Role {
		if err := tx.Model(&existing).Update("role", association.Role).Error; err != nil {
			return fmt.Errorf("update content account: %w", err)
		}
	}
	return nil
}

func ensureTargetPlatform(tx *gorm.DB, now int64) error {
	platform := model.Platform{
		Id:       platformWXChannels,
		Code:     platformWXChannels,
		Name:     "微信视频号",
		Homepage: "https://channels.weixin.qq.com",
		EntryURL: "https://channels.weixin.qq.com",
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := tx.Save(&platform).Error; err != nil {
		return fmt.Errorf("save platform: %w", err)
	}
	return nil
}

func findMigratedDownloadTask(db *gorm.DB, gopeedID string) (*model.DownloadTask, error) {
	if db == nil {
		return nil, fmt.Errorf("target database is not opened")
	}
	gopeedID = strings.TrimSpace(gopeedID)

	if gopeedID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	for _, key := range []string{"gopeedid", "gopeed_task_id"} {
		var task model.DownloadTask
		pattern := "%\"" + key + "\":" + jsonStringLiteral(gopeedID) + "%"
		result := db.Where("platform_id = ? AND deleted_at IS NULL AND metadata_json LIKE ?", platformWXChannels, pattern).Limit(1).Find(&task)
		if result.Error != nil {
			return nil, fmt.Errorf("query migrated gopeed task metadata: %w", result.Error)
		}
		if result.RowsAffected > 0 {
			return &task, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func jsonStringLiteral(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(data)
}

func (s *migrationServer) resolveDataDir(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = s.defaultDataDir
	}
	clean := filepath.Clean(expandHome(raw))
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", "", fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", fmt.Errorf("path is not accessible: %w", err)
	}
	dataDir := abs
	if !info.IsDir() {
		if filepath.Base(abs) != gopeedDBFile {
			return "", "", fmt.Errorf("select %s or the directory containing it", gopeedDBFile)
		}
		dataDir = filepath.Dir(abs)
	}
	dbPath := filepath.Join(dataDir, gopeedDBFile)
	dbInfo, err := os.Stat(dbPath)
	if err != nil {
		return "", "", fmt.Errorf("%s not found in %s", gopeedDBFile, dataDir)
	}
	if dbInfo.IsDir() {
		return "", "", fmt.Errorf("%s is a directory", dbPath)
	}
	return dataDir, dbPath, nil
}

func (s *migrationServer) resolveProfileCacheDir(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = s.defaultDataDir
	}
	if dataDir, _, err := s.resolveDataDir(raw); err == nil {
		return dataDir, nil
	}
	clean := filepath.Clean(expandHome(raw))
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve profile cache directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("profile cache directory is not accessible: %w", err)
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	return abs, nil
}

func loadGopeedTasks(dataDir string, status string) (filtered []*downloadpkg.Task, all []*downloadpkg.Task, err error) {
	var storage *downloadpkg.BoltStorage
	var downloader *downloadpkg.Downloader
	defer func() {
		if recovered := recover(); recovered != nil {
			if storage != nil {
				_ = storage.Close()
			}
			err = fmt.Errorf("open gopeed storage: %v", recovered)
		}
	}()

	storage = downloadpkg.NewReadOnlyBoltStorage(dataDir)
	downloader = downloadpkg.NewDownloader(&downloadpkg.DownloaderConfig{
		RefreshInterval: 360,
		Storage:         storage,
		StorageDir:      dataDir,
	})
	if err = downloader.Setup(); err != nil {
		_ = storage.Close()
		return nil, nil, err
	}
	defer func() {
		if closeErr := downloader.CloseStorageOnly(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	all = append([]*downloadpkg.Task(nil), downloader.GetTasks()...)
	filter := &downloadpkg.TaskFilter{}
	if status != "" && status != "all" {
		filter.Statuses = []base.Status{base.Status(status)}
	}
	filtered = append([]*downloadpkg.Task(nil), downloader.GetTasksByFilter(filter)...)
	return filtered, all, nil
}

func taskToRow(task *downloadpkg.Task) map[string]any {
	labels := taskLabels(task)
	size := taskSize(task)
	downloaded := taskDownloaded(task)
	progress := 0.0
	if size > 0 {
		progress = math.Round((float64(downloaded)/float64(size))*10000) / 100
	} else if task != nil && task.Status == base.DownloadStatusDone {
		progress = 100
	}
	return map[string]any{
		"id":          safeTaskID(task),
		"name":        taskName(task),
		"status":      string(taskStatus(task)),
		"protocol":    taskProtocol(task),
		"url":         taskURL(task),
		"save_path":   taskSavePath(task),
		"size":        size,
		"downloaded":  downloaded,
		"progress":    progress,
		"oid":         firstLabel(labels, "oid", "id", "external_id", "objectid", "object_id"),
		"uid":         firstLabel(labels, "uid", "nid", "nonce_id", "objectNonceId", "object_nonce_id"),
		"external_id": labels["id"],
		"nonce_id":    labels["nonce_id"],
		"title":       firstNonEmpty(labels["title"], taskName(task)),
		"labels":      labels,
		"created_at":  formatTime(taskCreatedAt(task)),
		"updated_at":  formatTime(taskUpdatedAt(task)),
		"error":       taskError(task),
	}
}

func safeTaskID(task *downloadpkg.Task) string {
	if task == nil {
		return ""
	}
	return task.ID
}

func taskStatus(task *downloadpkg.Task) base.Status {
	if task == nil {
		return ""
	}
	return task.Status
}

func taskProtocol(task *downloadpkg.Task) string {
	if task == nil {
		return ""
	}
	return task.Protocol
}

func taskLabels(task *downloadpkg.Task) map[string]string {
	if task == nil || task.Meta == nil || task.Meta.Req == nil || task.Meta.Req.Labels == nil {
		return map[string]string{}
	}
	return task.Meta.Req.Labels
}

func taskURL(task *downloadpkg.Task) string {
	if task == nil || task.Meta == nil || task.Meta.Req == nil {
		return ""
	}
	return task.Meta.Req.URL
}

func taskName(task *downloadpkg.Task) string {
	if task == nil || task.Meta == nil {
		return "unknown"
	}
	if task.Meta.Opts != nil && strings.TrimSpace(task.Meta.Opts.Name) != "" {
		return task.Meta.Opts.Name
	}
	if task.Meta.Res != nil {
		if strings.TrimSpace(task.Meta.Res.Name) != "" {
			return task.Meta.Res.Name
		}
		for _, file := range task.Meta.Res.Files {
			if file != nil && strings.TrimSpace(file.Name) != "" {
				return file.Name
			}
		}
	}
	if name := nameFromURL(taskURL(task)); name != "" {
		return name
	}
	if task.ID != "" {
		return task.ID
	}
	return "unknown"
}

func taskSavePath(task *downloadpkg.Task) string {
	if task == nil || task.Meta == nil {
		return ""
	}
	downloadDir := ""
	if task.Meta.Opts != nil {
		downloadDir = strings.TrimSpace(task.Meta.Opts.Path)
	}
	if task.Meta.Res == nil {
		return downloadDir
	}
	if task.Meta.Res.Name != "" {
		name := task.Meta.Res.Name
		if task.Meta.Opts != nil && task.Meta.Opts.Name != "" {
			name = task.Meta.Opts.Name
		}
		return filepath.Join(downloadDir, name)
	}
	if len(task.Meta.Res.Files) == 0 || task.Meta.Res.Files[0] == nil {
		return downloadDir
	}
	file := task.Meta.Res.Files[0]
	name := file.Name
	if task.Meta.Opts != nil && task.Meta.Opts.Name != "" {
		name = task.Meta.Opts.Name
	}
	return filepath.Join(downloadDir, file.Path, name)
}

func taskFilePath(task *downloadpkg.Task, dataDir string) string {
	savePath := strings.TrimSpace(taskSavePath(task))
	if savePath == "" {
		return ""
	}
	savePath = expandHome(savePath)
	if filepath.IsAbs(savePath) {
		return filepath.Clean(savePath)
	}
	if dataDir != "" {
		candidate := filepath.Join(dataDir, savePath)
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(candidate)
		}
	}
	abs, err := filepath.Abs(savePath)
	if err != nil {
		return filepath.Clean(savePath)
	}
	return abs
}

func taskSize(task *downloadpkg.Task) int64 {
	if task == nil || task.Meta == nil || task.Meta.Res == nil {
		return taskDownloaded(task)
	}
	if task.Meta.Res.Size > 0 {
		return task.Meta.Res.Size
	}
	var size int64
	for _, file := range task.Meta.Res.Files {
		if file != nil {
			size += file.Size
		}
	}
	if size > 0 {
		return size
	}
	return taskDownloaded(task)
}

func taskDownloaded(task *downloadpkg.Task) int64 {
	if task == nil || task.Progress == nil {
		return 0
	}
	return task.Progress.Downloaded
}

func taskError(task *downloadpkg.Task) string {
	if task == nil {
		return ""
	}
	return task.Error
}

func taskCreatedAt(task *downloadpkg.Task) time.Time {
	if task == nil {
		return time.Time{}
	}
	return task.CreatedAt
}

func taskUpdatedAt(task *downloadpkg.Task) time.Time {
	if task == nil {
		return time.Time{}
	}
	return task.UpdatedAt
}

func firstLabel(labels map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(labels[key]); value != "" {
			return value
		}
	}
	return ""
}

func cleanNonceID(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexByte(value, '_'); idx > 0 {
		return value[:idx]
	}
	return value
}

func normalizeTargetBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = defaultTargetBaseURL
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return defaultTargetBaseURL
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func targetEndpoint(baseURL string, endpointPath string) *url.URL {
	parsed, err := url.Parse(normalizeTargetBaseURL(baseURL))
	if err != nil {
		parsed, _ = url.Parse(defaultTargetBaseURL)
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	parsed.Path = basePath + "/" + strings.TrimLeft(endpointPath, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed
}

func buildTargetProfileURL(targetBaseURL string, oid string, uid string) *url.URL {
	profileURL := targetEndpoint(targetBaseURL, "/api/channels/feed/profile")
	query := profileURL.Query()
	query.Set("oid", strings.TrimSpace(oid))
	query.Set("nid", cleanNonceID(uid))
	query.Set("uid", cleanNonceID(uid))
	profileURL.RawQuery = query.Encode()
	return profileURL
}

func profileCachePath(dataDir string) string {
	return filepath.Join(dataDir, profileCacheFile)
}

func taskProfileCacheKey(labels map[string]string, oid string) string {
	return profileCacheKey(firstLabel(labels, "id", "oid", "external_id", "objectid", "object_id"), oid)
}

func profileCacheKey(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	return value
}

func buildTargetContentID(externalID string) string {
	return platformWXChannels + ":" + strings.TrimSpace(externalID)
}

func buildTargetAccountID(externalID string) string {
	return platformWXChannels + ":" + strings.TrimSpace(externalID)
}

func migratedTaskUniqueID(oid string, labels map[string]string, savePath string) string {
	uniqueID := strings.TrimSpace(oid)
	if uniqueID == "" {
		uniqueID = filepath.Base(savePath)
	}
	spec := firstLabel(labels, "spec", "file_format", "format", "quality")
	if spec != "" && spec != "original" {
		uniqueID += "_" + spec
	}
	suffix := firstNonEmpty(labels["suffix"], migrationSuffixConfig(savePath))
	if strings.EqualFold(suffix, ".mp3") && !strings.HasSuffix(uniqueID, "_mp3") {
		uniqueID += "_mp3"
	}
	if strings.EqualFold(suffix, ".jpg") && !strings.HasSuffix(uniqueID, "_cover") {
		uniqueID += "_cover"
	}
	return uniqueID
}

func migratedContentType(obj channelsObjectLite) string {
	if hasJSONValue(obj.LiveInfo) {
		return "live"
	}
	switch obj.ObjectDesc.MediaType {
	case mediaTypePicture:
		return "album"
	case mediaTypeLive:
		return "live"
	case mediaTypeVideo, 0:
		return "video"
	default:
		if strings.TrimSpace(obj.Type) != "" {
			return strings.TrimSpace(obj.Type)
		}
		return "video"
	}
}

func firstObjectMedia(obj channelsObjectLite) channelsMediaItem {
	files := objectFiles(obj)
	if len(files) == 0 {
		return channelsMediaItem{}
	}
	return files[0]
}

func objectFiles(obj channelsObjectLite) []channelsMediaItem {
	if len(obj.Files) > 0 {
		return obj.Files
	}
	return obj.ObjectDesc.Media
}

func mediaDownloadURL(media channelsMediaItem) string {
	return strings.TrimSpace(media.URL + media.URLToken)
}

func liveCoverURL(obj channelsObjectLite) string {
	if obj.AnchorContact != nil {
		return strings.TrimSpace(obj.AnchorContact.CoverImgURL)
	}
	return ""
}

func targetAccountContact(obj channelsObjectLite) (channelsContactLite, string) {
	if hasJSONValue(obj.LiveInfo) && obj.AnchorContact != nil {
		return *obj.AnchorContact, strings.TrimSpace(obj.AnchorContact.Username)
	}
	return obj.Contact, strings.TrimSpace(obj.Contact.Username)
}

func accountExternalID(obj channelsObjectLite, fallbackUID string) string {
	_, externalID := targetAccountContact(obj)
	return firstNonEmpty(externalID, fallbackUID)
}

func accountNickname(account *model.Account) string {
	if account == nil {
		return ""
	}
	return account.Nickname
}

func migratedSize(task *downloadpkg.Task, savePath string, media channelsMediaItem) int64 {
	size := taskSize(task)
	if info, err := os.Stat(savePath); err == nil && !info.IsDir() && info.Size() > 0 {
		size = info.Size()
	}
	if size <= 0 {
		size = int64(media.FileSize)
	}
	return size
}

func migratedDownloaded(task *downloadpkg.Task, size int64) int64 {
	downloaded := taskDownloaded(task)
	if taskStatus(task) == base.DownloadStatusDone && size > downloaded {
		downloaded = size
	}
	return downloaded
}

func dimensionString(value flexibleFloat64) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(int(math.Round(float64(value))))
}

func fileExtensionFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		if ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(parsed.Path)), "."); ext != "" {
			return ext
		}
	}
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(rawURL)), ".")
}

func resourceKindForPath(savePath string) string {
	switch strings.ToLower(filepath.Ext(savePath)) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".zip":
		return "application/zip"
	default:
		return "file"
	}
}

func endpointProtocol(rawURL string, fallback string) string {
	if parsed, err := url.Parse(strings.TrimSpace(rawURL)); err == nil && parsed.Scheme != "" {
		return strings.ToLower(parsed.Scheme)
	}
	fallback = strings.TrimSpace(strings.ToLower(fallback))
	if fallback != "" {
		return fallback
	}
	return "file"
}

func buildChannelsFallbackSourceURL(oid string, uid string, username string) string {
	query := url.Values{}
	if strings.TrimSpace(username) != "" {
		query.Set("username", strings.TrimSpace(username))
	}
	if strings.TrimSpace(oid) != "" {
		query.Set("oid", strings.TrimSpace(oid))
	}
	if strings.TrimSpace(uid) != "" {
		query.Set("nid", cleanNonceID(uid))
	}
	if encoded := query.Encode(); encoded != "" {
		return "https://channels.weixin.qq.com/web/pages/feed?" + encoded
	}
	return ""
}

func hasJSONValue(raw json.RawMessage) bool {
	text := strings.TrimSpace(string(raw))
	return text != "" && text != "null" && text != "{}"
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func buildMigrationTaskConfigJSON(downloadDir string, filename string, labels map[string]string) string {
	config := map[string]any{
		"download_dir": downloadDir,
	}
	if suffix := migrationSuffixConfig(filename); suffix != "" {
		config["suffix"] = suffix
	}
	if spec := strings.TrimSpace(labels["spec"]); spec != "" {
		config["spec"] = spec
	}
	return mustJSON(config)
}

func mustRawJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(data)
}

func mapGopeedStatusToTargetStatus(status base.Status) int {
	switch status {
	case base.DownloadStatusRunning:
		return 2
	case base.DownloadStatusPause:
		return 3
	case base.DownloadStatusError:
		return 6
	case base.DownloadStatusDone:
		return 5
	default:
		return 0
	}
}

func migrationSuffixConfig(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp4":
		return ".mp4"
	default:
		return ""
	}
}

func migrationCreateFilename(filename string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == string(filepath.Separator) {
		return ""
	}
	ext := filepath.Ext(filename)
	if ext == "" {
		return filename
	}
	return strings.TrimSuffix(filename, ext)
}

func nameFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	name := path.Base(parsed.Path)
	if name == "." || name == "/" {
		return parsed.Hostname()
	}
	return name
}

func buildStatusStats(tasks []*downloadpkg.Task) map[string]int {
	stats := map[string]int{
		"total":   len(tasks),
		"ready":   0,
		"running": 0,
		"pause":   0,
		"wait":    0,
		"error":   0,
		"done":    0,
	}
	for _, task := range tasks {
		status := string(taskStatus(task))
		if _, ok := stats[status]; ok {
			stats[status]++
		}
	}
	return stats
}

func listFiles(rawDir string) (string, []fileItem, error) {
	dir := strings.TrimSpace(rawDir)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", nil, err
		}
		dir = home
	}
	dir = expandHome(dir)
	abs, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", nil, err
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", nil, err
	}
	items := make([]fileItem, 0, len(entries))
	for _, entry := range entries {
		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}
		isDir := entryInfo.IsDir()
		fileType := "file"
		if isDir {
			fileType = "dir"
		}
		items = append(items, fileItem{
			Name:        entry.Name(),
			Path:        filepath.Join(abs, entry.Name()),
			IsDir:       isDir,
			IsDirectory: isDir,
			Type:        fileType,
			Size:        entryInfo.Size(),
			ModTime:     entryInfo.ModTime().Format("2006-01-02 15:04"),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return abs, items, nil
}

func commonDirs(defaultDataDir string) []string {
	seen := map[string]bool{}
	dirs := make([]string, 0)
	add := func(dir string) {
		dir = strings.TrimSpace(expandHome(dir))
		if dir == "" {
			return
		}
		abs, err := filepath.Abs(dir)
		if err != nil || seen[abs] {
			return
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			return
		}
		seen[abs] = true
		dirs = append(dirs, abs)
	}
	add(defaultDataDir)
	if cwd, err := os.Getwd(); err == nil {
		add(cwd)
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(home)
		add(filepath.Join(home, "Downloads"))
		add(filepath.Join(home, "Documents"))
		add(filepath.Join(home, "Desktop"))
	}
	add("/Volumes")
	add("/")
	return dirs
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	writeAPIError(w, 405, "method not allowed")
	return false
}

func writeAPIOK(w http.ResponseWriter, data any) {
	writeAPIResponse(w, apiResponse{Code: 0, Msg: "success", Data: data})
}

func writeAPIError(w http.ResponseWriter, code int, msg string) {
	writeAPIResponse(w, apiResponse{Code: code, Msg: msg})
}

func writeAPIResponse(w http.ResponseWriter, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func normalizePagination(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	return page, pageSize
}

func normalizeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "", "all":
		return "all"
	case "waiting":
		return "wait"
	case "paused":
		return "pause"
	case "finished", "completed":
		return "done"
	case "failed", "failure":
		return "error"
	default:
		return status
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func timeToMillis(t time.Time) int64 {
	if t.IsZero() {
		return time.Now().UnixMilli()
	}
	return t.UnixMilli()
}

func timeToMillisOrDefault(t time.Time, fallback int64) int64 {
	if t.IsZero() {
		return fallback
	}
	return t.UnixMilli()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func expandHome(raw string) string {
	if raw == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(raw, "~/") || strings.HasPrefix(raw, `~\`) {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, raw[2:])
		}
	}
	return raw
}
