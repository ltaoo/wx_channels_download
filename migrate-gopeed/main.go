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
	"strings"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/ltaoo/velo"
	"github.com/ltaoo/velo/frontendserver"
	"gorm.io/gorm"

	wxmpadapter "wx_channel/internal/adapter/wxmp"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxmp"
)

//go:embed web
var embedded_fs embed.FS

var Mode = default_mode

const gopeed_db_file = "gopeed.db"
const default_target_base_url = "http://127.0.0.1:2022"
const default_target_db_file = "data.db"
const profile_cache_file = ".gopeed_migration_profile_cache.json"
const platform_wx_channels = "wxchannels"
const platform_wxmp = "wxmp"
const resource_status_finished = 2

type api_response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

type table_info struct {
	Name string `json:"name"`
	Rows int    `json:"rows"`
}

type column_info struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type migration_load_request struct {
	DataDir     string `json:"data_dir"`
	DBPath      string `json:"db_path"`
	Status      string `json:"status"`
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
	ForceReload bool   `json:"force_reload"`
}

type migration_table_request struct {
	DataDir     string `json:"data_dir"`
	DBPath      string `json:"db_path"`
	Table       string `json:"table"`
	Status      string `json:"status"`
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
	ForceReload bool   `json:"force_reload"`
}

type migration_execute_request struct {
	DataDir      string   `json:"data_dir"`
	DBPath       string   `json:"db_path"`
	Status       string   `json:"status"`
	TargetURL    string   `json:"target_url"`
	TargetDB     string   `json:"target_db"`
	Limit        int      `json:"limit"`
	DryRun       bool     `json:"dry_run"`
	AllowMissing bool     `json:"allow_missing"`
	TaskIDs      []string `json:"task_ids"`
	ForceReload  bool     `json:"force_reload"`
}

type task_query_result struct {
	Database   string           `json:"database"`
	DataDir    string           `json:"data_dir"`
	DBPath     string           `json:"db_path"`
	Tables     []table_info     `json:"tables"`
	Table      string           `json:"table"`
	Columns    []column_info    `json:"columns"`
	Rows       []map[string]any `json:"rows"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
	Stats      map[string]int   `json:"stats"`
	Status     string           `json:"status"`
	CacheHit   bool             `json:"cache_hit"`
	LoadedAt   string           `json:"loaded_at,omitempty"`
}

type migration_execute_result struct {
	DataDir   string                   `json:"data_dir"`
	DBPath    string                   `json:"db_path"`
	TargetURL string                   `json:"target_url"`
	TargetDB  string                   `json:"target_db"`
	DryRun    bool                     `json:"dry_run"`
	Total     int                      `json:"total"`
	Success   int                      `json:"success"`
	Skipped   int                      `json:"skipped"`
	Failed    int                      `json:"failed"`
	Items     []migration_execute_item `json:"items"`
}

type migration_execute_item struct {
	TaskID          string `json:"task_id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	OID             string `json:"oid"`
	UID             string `json:"uid"`
	ArticleID       string `json:"article_id,omitempty"`
	SavePath        string `json:"save_path"`
	ProfileURL      string `json:"profile_url,omitempty"`
	ProfileCacheKey string `json:"profile_cache_key,omitempty"`
	ProfileCacheHit bool   `json:"profile_cache_hit,omitempty"`
	Action          string `json:"action"`
	TargetID        int    `json:"target_id,omitempty"`
	Error           string `json:"error,omitempty"`
}

type migration_profile_cache_status struct {
	CacheKey   string
	ProfileURL string
	CacheHit   bool
	Error      string
}

type task_cache_entry struct {
	DataDir  string
	DBPath   string
	ModTime  time.Time
	Size     int64
	Tasks    []*downloadpkg.Task
	Stats    map[string]int
	LoadedAt time.Time
}

type file_list_request struct {
	Dir string `json:"dir"`
}

type profile_cache_cleanup_request struct {
	DataDir string `json:"data_dir"`
	DBPath  string `json:"db_path"`
}

type profile_cache_cleanup_result struct {
	DataDir        string   `json:"data_dir"`
	CachePath      string   `json:"cache_path"`
	Total          int      `json:"total"`
	Checked        int      `json:"checked"`
	SkippedArticle int      `json:"skipped_article"`
	Removed        int      `json:"removed"`
	Remaining      int      `json:"remaining"`
	RemovedKeys    []string `json:"removed_keys"`
}

type file_item struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDir       bool   `json:"isDir"`
	IsDirectory bool   `json:"isDirectory"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	ModTime     string `json:"modTime"`
}

type migration_server struct {
	default_data_dir string
	target_base_url  string
	target_db_path   string
	profile_cache_mu sync.Mutex
	profile_agent    *channels_profile_agent_hub
	task_cache_mu    sync.Mutex
	task_cache       map[string]*task_cache_entry
}

var task_columns = []column_info{
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
	{Name: "article_id", Type: "TEXT"},
	{Name: "detail_fetched", Type: "BOOLEAN"},
	{Name: "profile_cached", Type: "BOOLEAN"},
	{Name: "profile_cache_key", Type: "TEXT"},
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
	var data_dir string
	var target_url string
	var target_db string
	var mode string
	var frontend_root string
	var run_profile_agent bool
	var profile_agent_url string
	var profile_agent_token string
	var profile_agent_local_url string
	flag.StringVar(&host, "host", "127.0.0.1", "HTTP listen host")
	flag.IntVar(&port, "port", 8026, "HTTP listen port")
	flag.StringVar(&data_dir, "data-dir", gopeed_db_file, "default Gopeed .db database file path")
	flag.StringVar(&target_url, "target-url", default_target_base_url, "wx_channels_download target API base URL")
	flag.StringVar(&target_db, "target-db", "", "wx_channels_download target SQLite database path")
	flag.StringVar(&mode, "mode", Mode, "frontend mode: dev, debug, release, or prod")
	flag.StringVar(&frontend_root, "frontend-root", "", "frontend root directory in dev mode")
	flag.BoolVar(&run_profile_agent, "profile-agent", false, "run as a macOS Channels profile agent instead of the migration HTTP server")
	flag.StringVar(&profile_agent_url, "profile-agent-url", "", "profile agent websocket URL, e.g. wss://example.com/ws/channels/profile-agent")
	flag.StringVar(&profile_agent_token, "profile-agent-token", default_profile_agent_token(), "profile agent bearer token, also read from WX_CHANNEL_PROFILE_AGENT_TOKEN")
	flag.StringVar(&profile_agent_local_url, "profile-agent-local-url", "", "local wx_channels_download API base URL used by the profile agent")
	flag.Parse()

	if run_profile_agent {
		local_target_url := normalize_target_base_url(first_non_empty(profile_agent_local_url, target_url, default_target_base_url))
		if err := run_channels_profile_agent(profile_agent_url, local_target_url, profile_agent_token); err != nil {
			log.Fatalf("profile agent: %v", err)
		}
		return
	}

	abs_data_dir, err := filepath.Abs(expand_home(data_dir))
	if err != nil {
		log.Fatalf("resolve data dir: %v", err)
	}
	target_db_path, err := resolve_target_db_path(target_db)
	if err != nil {
		log.Fatalf("resolve target db: %v", err)
	}
	server := &migration_server{
		default_data_dir: abs_data_dir,
		target_base_url:  normalize_target_base_url(target_url),
		target_db_path:   target_db_path,
		profile_agent:    new_channels_profile_agent_hub(profile_agent_token),
		task_cache:       make(map[string]*task_cache_entry),
	}
	if profile_agent_token == "" {
		log.Printf("WARNING: profile agent websocket is enabled without a token; set WX_CHANNEL_PROFILE_AGENT_TOKEN or -profile-agent-token before exposing it")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/migration/load", server.handle_migration_load)
	mux.HandleFunc("/api/v1/migration/table", server.handle_migration_table)
	mux.HandleFunc("/api/v1/migration/execute", server.handle_migration_execute)
	mux.HandleFunc("/api/v1/migration/file/list", server.handle_migration_file_list)
	mux.HandleFunc("/api/v1/fs/list", server.handle_migration_file_list)
	mux.HandleFunc("/api/v1/migration/common_dirs", server.handle_migration_common_dirs)
	mux.HandleFunc("/api/v1/migration/profile-cache/cleanup", server.handle_migration_profile_cache_cleanup)
	mux.HandleFunc("/api/channels/feed/profile", server.handle_channels_feed_profile)
	mux.HandleFunc(channels_profile_agent_path, server.profile_agent.HandleWebsocket)
	mux.HandleFunc("/api/channels/profile-agent/status", server.handle_channels_profile_agent_status)
	mux.HandleFunc("/api/mp/article/profile", server.handle_wxmp_article_profile)
	mux.Handle("/", new_migration_frontend(mode, frontend_root))

	addr := fmt.Sprintf("%s:%d", host, port)
	log.Printf("Gopeed migration viewer listening on http://%s/migration", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func new_migration_frontend(mode string, frontend_root string) http.Handler {
	normalized_mode := strings.ToLower(strings.TrimSpace(mode))
	server_mode := frontendserver.ModeDev
	static_asset_prefixes := []string(nil)
	root := ""
	var embedded embed.FS

	switch normalized_mode {
	case "", "dev", "development", "debug":
		root = resolve_frontend_root(frontend_root)
	case "release", "prod", "production":
		server_mode = frontendserver.ModeProd
		static_asset_prefixes = []string{"/assets"}
		root = "web"
		embedded = embedded_fs
	default:
		log.Fatalf("unsupported frontend mode %q; use dev, debug, release, or prod", mode)
	}

	return frontendserver.New(frontendserver.Options{
		Mode:                server_mode,
		Root:                root,
		Embedded:            embedded,
		EntryPage:           "index.html",
		StaticAssetPrefixes: static_asset_prefixes,
		NoFallbackPrefixes:  []string{"/api", "/assets"},
	})
}

func resolve_frontend_root(frontend_root string) string {
	if strings.TrimSpace(frontend_root) != "" {
		root, err := filepath.Abs(expand_home(frontend_root))
		if err != nil {
			log.Fatalf("resolve frontend root: %v", err)
		}
		if !is_frontend_root(root) {
			log.Fatalf("frontend root %q does not contain index.html", root)
		}
		return root
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("resolve working directory: %v", err)
	}
	candidates := []string{
		filepath.Join(wd, "web"),
		filepath.Join(wd, "migrate-gopeed", "web"),
		filepath.Join(filepath.Dir(wd), "migrate-gopeed", "web"),
	}
	for _, candidate := range candidates {
		if is_frontend_root(candidate) {
			return candidate
		}
	}
	log.Fatalf("frontend root not found; pass --frontend-root /path/to/migrate-gopeed/web")
	return ""
}

func is_frontend_root(root string) bool {
	info, err := os.Stat(filepath.Join(root, "index.html"))
	return err == nil && !info.IsDir()
}

func resolve_target_db_path(raw string) (string, error) {
	raw = strings.TrimSpace(expand_home(raw))
	if raw != "" {
		return normalize_target_db_path(raw)
	}
	if env := strings.TrimSpace(os.Getenv("WX_CHANNEL_DB_PATH")); env != "" {
		return normalize_target_db_path(env)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	candidates := []string{
		filepath.Join(wd, default_target_db_file),
		filepath.Join(filepath.Dir(wd), default_target_db_file),
		filepath.Join(wd, "data", default_target_db_file),
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
	return "", fmt.Errorf("target database path not found; pass --target-db /path/to/%s", default_target_db_file)
}

func normalize_target_db_path(raw string) (string, error) {
	clean := filepath.Clean(expand_home(raw))
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve target database path: %w", err)
	}
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		abs = filepath.Join(abs, default_target_db_file)
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

func open_target_db(db_path string) (*gorm.DB, error) {
	app := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	if err := app.Migrate(&velo.VeloDatabaseOpt{DBType: velo.DBTypeSQLite, DBPath: db_path, Migrations: &database.Migrations}); err != nil {
		return nil, fmt.Errorf("target database initialization failed: %w", err)
	}
	if err := database.ConfigureSQLiteRuntime(app.DB); err != nil {
		return nil, fmt.Errorf("target database sqlite configuration failed: %w", err)
	}
	return app.DB, nil
}

func close_target_db(db *gorm.DB) {
	if db == nil {
		return
	}
	sql_db, err := db.DB()
	if err == nil {
		_ = sql_db.Close()
	}
}

func (s *migration_server) handle_migration_load(w http.ResponseWriter, r *http.Request) {
	if !require_method(w, r, http.MethodPost) {
		return
	}
	var req migration_load_request
	if err := decode_json(r, &req); err != nil {
		write_api_error(w, 400, "invalid request body: "+err.Error())
		return
	}
	result, err := s.query_tasks(first_non_empty(req.DataDir, req.DBPath), req.Status, req.Page, req.PageSize, req.ForceReload)
	if err != nil {
		write_api_error(w, 500, err.Error())
		return
	}
	write_apiok(w, result)
}

func (s *migration_server) handle_migration_table(w http.ResponseWriter, r *http.Request) {
	if !require_method(w, r, http.MethodPost) {
		return
	}
	var req migration_table_request
	if err := decode_json(r, &req); err != nil {
		write_api_error(w, 400, "invalid request body: "+err.Error())
		return
	}
	table := strings.TrimSpace(req.Table)
	if table == "" {
		table = "task"
	}
	if table != "task" && table != "tasks" {
		write_api_error(w, 404, "table not found: "+table)
		return
	}
	result, err := s.query_tasks(first_non_empty(req.DataDir, req.DBPath), req.Status, req.Page, req.PageSize, req.ForceReload)
	if err != nil {
		write_api_error(w, 500, err.Error())
		return
	}
	write_apiok(w, result)
}

func (s *migration_server) handle_migration_execute(w http.ResponseWriter, r *http.Request) {
	if !require_method(w, r, http.MethodPost) {
		return
	}
	var req migration_execute_request
	if err := decode_json(r, &req); err != nil {
		write_api_error(w, 400, "invalid request body: "+err.Error())
		return
	}
	result, err := s.execute_migration(req)
	if err != nil {
		write_api_error(w, 500, err.Error())
		return
	}
	write_apiok(w, result)
}

func (s *migration_server) handle_migration_file_list(w http.ResponseWriter, r *http.Request) {
	if !require_method(w, r, http.MethodPost) {
		return
	}
	var req file_list_request
	if err := decode_json(r, &req); err != nil {
		write_api_error(w, 400, "invalid request body: "+err.Error())
		return
	}
	dir, files, err := list_files(req.Dir)
	if err != nil {
		write_api_error(w, 500, err.Error())
		return
	}
	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}
	write_apiok(w, map[string]any{
		"dir":    dir,
		"parent": parent,
		"files":  files,
		"items":  files,
		"list":   files,
	})
}

func (s *migration_server) handle_migration_common_dirs(w http.ResponseWriter, r *http.Request) {
	if !require_method(w, r, http.MethodGet) {
		return
	}
	default_db_path := ""
	if _, db_path, err := s.resolve_data_dir(""); err == nil {
		default_db_path = db_path
	}
	write_apiok(w, map[string]any{
		"default_data_dir": s.default_data_dir,
		"default_db_path":  default_db_path,
		"target_url":       s.target_base_url,
		"target_db":        s.target_db_path,
		"dirs":             common_dirs(s.default_data_dir),
	})
}

func (s *migration_server) handle_migration_profile_cache_cleanup(w http.ResponseWriter, r *http.Request) {
	if !require_method(w, r, http.MethodPost) {
		return
	}
	var req profile_cache_cleanup_request
	if err := decode_json(r, &req); err != nil {
		write_api_error(w, 400, "invalid request body: "+err.Error())
		return
	}
	data_dir, err := s.resolve_profile_cache_dir(first_non_empty(req.DataDir, req.DBPath))
	if err != nil {
		write_api_error(w, 400, err.Error())
		return
	}
	result, err := s.cleanup_invalid_profile_cache(data_dir)
	if err != nil {
		write_api_error(w, 500, err.Error())
		return
	}
	write_apiok(w, result)
}

func (s *migration_server) handle_channels_feed_profile(w http.ResponseWriter, r *http.Request) {
	if !require_method(w, r, http.MethodGet) {
		return
	}
	query := r.URL.Query()
	oid := strings.TrimSpace(query.Get("oid"))
	uid := first_non_empty(query.Get("uid"), query.Get("nid"), query.Get("nonce_id"))
	if oid == "" {
		write_api_error(w, 400, "missing oid")
		return
	}
	cache_dir, err := s.resolve_profile_cache_dir(first_non_empty(query.Get("data_dir"), query.Get("db_path")))
	if err != nil {
		write_api_error(w, 400, err.Error())
		return
	}
	cache_key := first_non_empty(query.Get("cache_key"), oid)
	target_base_url := normalize_target_base_url(first_non_empty(query.Get("target_url"), s.target_base_url, default_target_base_url))
	force_agent := truthy_query_value(first_non_empty(query.Get("force_agent"), query.Get("require_agent")))
	profile, profile_url, cached, source, err := s.fetch_profile_with_cache(&http.Client{Timeout: 30 * time.Second}, cache_dir, target_base_url, cache_key, oid, uid, force_agent)
	if err != nil {
		write_api_error(w, 502, err.Error())
		return
	}
	write_apiok(w, map[string]any{
		"profile_url": profile_url,
		"cache_key":   profile_cache_key(cache_key, oid),
		"cached":      cached,
		"source":      source,
		"profile":     profile,
	})
}

func (s *migration_server) handle_channels_profile_agent_status(w http.ResponseWriter, r *http.Request) {
	if !require_method(w, r, http.MethodGet) {
		return
	}
	clients := 0
	connected := false
	if s != nil && s.profile_agent != nil {
		clients = s.profile_agent.ClientCount()
		connected = clients > 0
	}
	auth_required := s != nil && s.profile_agent != nil && s.profile_agent.AuthRequired()
	write_apiok(w, map[string]any{
		"connected":     connected,
		"clients":       clients,
		"auth_required": auth_required,
		"path":          channels_profile_agent_path,
	})
}

func (s *migration_server) handle_wxmp_article_profile(w http.ResponseWriter, r *http.Request) {
	if !require_method(w, r, http.MethodGet) {
		return
	}
	query := r.URL.Query()
	article_id := first_non_empty(query.Get("article_id"), query.Get("articleid"), query.Get("mp_article_id"))
	article_url := first_non_empty(query.Get("url"), build_wxmp_article_url(article_id))
	if strings.TrimSpace(article_url) == "" {
		write_api_error(w, 400, "missing article_id")
		return
	}
	cache_dir, err := s.resolve_profile_cache_dir(first_non_empty(query.Get("data_dir"), query.Get("db_path")))
	if err != nil {
		write_api_error(w, 400, err.Error())
		return
	}
	content_raw, article_url, cached, err := s.fetch_wxmp_article_content_with_cache(cache_dir, article_id, article_url)
	if err != nil {
		write_api_error(w, 502, err.Error())
		return
	}
	write_apiok(w, map[string]any{
		"article_id":  article_id,
		"article_url": article_url,
		"cache_key":   wxmp_article_profile_cache_key(article_id, article_url),
		"cached":      cached,
		"profile":     content_raw,
	})
}

func (s *migration_server) execute_migration(req migration_execute_request) (*migration_execute_result, error) {
	data_dir, db_path, err := s.resolve_data_dir(first_non_empty(req.DataDir, req.DBPath))
	if err != nil {
		return nil, err
	}
	status := normalize_status(req.Status)
	snapshot, _, err := s.load_gopeed_task_snapshot(data_dir, db_path, req.ForceReload)
	if err != nil {
		return nil, err
	}
	tasks := filter_gopeed_tasks(snapshot.Tasks, status)
	if len(req.TaskIDs) > 0 {
		task_i_ds := make(map[string]bool, len(req.TaskIDs))
		for _, id := range req.TaskIDs {
			if id = strings.TrimSpace(id); id != "" {
				task_i_ds[id] = true
			}
		}
		filtered := make([]*downloadpkg.Task, 0, len(tasks))
		for _, task := range tasks {
			if task_i_ds[safe_task_id(task)] {
				filtered = append(filtered, task)
			}
		}
		tasks = filtered
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return task_created_at(tasks[i]).Before(task_created_at(tasks[j]))
	})
	if req.Limit > 0 && req.Limit < len(tasks) {
		tasks = tasks[:req.Limit]
	}

	target_base_url := normalize_target_base_url(first_non_empty(req.TargetURL, s.target_base_url, default_target_base_url))
	target_db_path, err := resolve_target_db_path(first_non_empty(req.TargetDB, s.target_db_path))
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	profile_cache_statuses := s.prefetch_migration_profile_cache(client, data_dir, target_base_url, tasks)

	var target_db *gorm.DB
	if !req.DryRun {
		target_db, err = open_target_db(target_db_path)
		if err != nil {
			return nil, err
		}
		defer close_target_db(target_db)
	}
	result := &migration_execute_result{
		DataDir:   data_dir,
		DBPath:    db_path,
		TargetURL: target_base_url,
		TargetDB:  target_db_path,
		DryRun:    req.DryRun,
		Total:     len(tasks),
		Items:     make([]migration_execute_item, 0, len(tasks)),
	}
	for _, task := range tasks {
		item := s.migrate_one_task(target_db, target_base_url, data_dir, task, req.DryRun, req.AllowMissing, profile_cache_statuses)
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

func (s *migration_server) prefetch_migration_profile_cache(client *http.Client, data_dir string, target_base_url string, tasks []*downloadpkg.Task) map[string]migration_profile_cache_status {
	statuses := make(map[string]migration_profile_cache_status, len(tasks))
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	for _, task := range tasks {
		labels := task_labels(task)
		if article_id := first_label(labels, "article_id", "articleid", "mp_article_id"); article_id != "" {
			article_url := gopeed_article_url(task, article_id)
			status := migration_profile_cache_status{
				CacheKey:   wxmp_article_profile_cache_key(article_id, article_url),
				ProfileURL: article_url,
			}
			_, profile_url, cached, err := s.fetch_wxmp_article_content_with_cache(data_dir, article_id, article_url)
			if profile_url != "" {
				status.ProfileURL = profile_url
			}
			status.CacheHit = cached
			if err != nil {
				status.Error = err.Error()
			}
			if task_id := safe_task_id(task); task_id != "" {
				statuses[task_id] = status
			}
			continue
		}
		oid := first_label(labels, "oid", "id", "external_id", "objectid", "object_id")
		if oid == "" {
			continue
		}
		uid := first_label(labels, "uid", "nid", "nonce_id", "objectNonceId", "object_nonce_id")
		cache_key := profile_cache_key(task_profile_cache_key(labels, oid), oid)
		status := migration_profile_cache_status{
			CacheKey:   cache_key,
			ProfileURL: build_target_profile_url(target_base_url, oid, uid).String(),
		}
		_, profile_url, cached, _, err := s.fetch_profile_with_cache(client, data_dir, target_base_url, cache_key, oid, uid, false)
		if profile_url != "" {
			status.ProfileURL = profile_url
		}
		status.CacheHit = cached
		if err != nil {
			status.Error = err.Error()
		}
		if task_id := safe_task_id(task); task_id != "" {
			statuses[task_id] = status
		}
	}
	return statuses
}

func (s *migration_server) query_tasks(raw_path string, status string, page int, page_size int, force_reload bool) (*task_query_result, error) {
	data_dir, db_path, err := s.resolve_data_dir(raw_path)
	if err != nil {
		return nil, err
	}
	page, page_size = normalize_pagination(page, page_size)
	status = normalize_status(status)

	snapshot, cache_hit, err := s.load_gopeed_task_snapshot(data_dir, db_path, force_reload)
	if err != nil {
		return nil, err
	}
	filtered := filter_gopeed_tasks(snapshot.Tasks, status)
	stats := clone_status_stats(snapshot.Stats)
	total := len(filtered)
	total_pages := 1
	if total > 0 {
		total_pages = int(math.Ceil(float64(total) / float64(page_size)))
	}
	if page > total_pages {
		page = total_pages
	}
	start := (page - 1) * page_size
	if start > total {
		start = total
	}
	end := start + page_size
	if end > total {
		end = total
	}
	profile_cache, _ := load_profile_cache(profile_cache_path(data_dir))
	rows := make([]map[string]any, 0, end-start)
	for _, task := range filtered[start:end] {
		row := task_to_row(task)
		if cached, cache_key := task_profile_cached(profile_cache, task); cached {
			row["detail_fetched"] = true
			row["profile_cached"] = true
			row["profile_cache_key"] = cache_key
		}
		rows = append(rows, row)
	}
	return &task_query_result{
		Database:   filepath.Base(db_path),
		DataDir:    data_dir,
		DBPath:     db_path,
		Tables:     []table_info{{Name: "task", Rows: len(snapshot.Tasks)}},
		Table:      "task",
		Columns:    task_columns,
		Rows:       rows,
		Total:      total,
		Page:       page,
		PageSize:   page_size,
		TotalPages: total_pages,
		Stats:      stats,
		Status:     status,
		CacheHit:   cache_hit,
		LoadedAt:   format_time(snapshot.LoadedAt),
	}, nil
}

type target_api_envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type channels_feed_profile struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type flexible_string string

func (s *flexible_string) UnmarshalJSON(data []byte) error {
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
		*s = flexible_string(value)
		return nil
	}
	*s = flexible_string(raw)
	return nil
}

type duplicate_import_error struct {
	TaskID int
}

func (e duplicate_import_error) Error() string {
	return fmt.Sprintf("download task already exists: %d", e.TaskID)
}

type profile_cache_document struct {
	Version int                            `json:"version"`
	Items   map[string]profile_cache_entry `json:"items"`
}

type profile_cache_entry struct {
	OID        string          `json:"oid"`
	UID        string          `json:"uid,omitempty"`
	ProfileURL string          `json:"profile_url"`
	Profile    json.RawMessage `json:"profile"`
	CachedAt   int64           `json:"cached_at"`
}

func (s *migration_server) migrate_one_task(db *gorm.DB, target_base_url string, data_dir string, task *downloadpkg.Task, dry_run bool, allow_missing bool, profile_cache_statuses map[string]migration_profile_cache_status) migration_execute_item {
	labels := task_labels(task)
	article_id := first_label(labels, "article_id", "articleid", "mp_article_id")
	if article_id != "" {
		return s.migrate_wxmp_article_task(db, data_dir, task, labels, article_id, dry_run, allow_missing)
	}

	oid := first_label(labels, "oid", "id", "external_id", "objectid", "object_id")
	uid := first_label(labels, "uid", "nid", "nonce_id", "objectNonceId", "object_nonce_id")
	save_path := task_file_path(task, data_dir)
	cache_key := profile_cache_key(task_profile_cache_key(labels, oid), oid)
	cache_status := profile_cache_statuses[safe_task_id(task)]
	if cache_status.CacheKey != "" {
		cache_key = cache_status.CacheKey
	}
	item := migration_execute_item{
		TaskID:          safe_task_id(task),
		Name:            task_name(task),
		Status:          string(task_status(task)),
		OID:             oid,
		UID:             uid,
		SavePath:        save_path,
		ProfileCacheKey: cache_key,
	}
	if task == nil || task.Meta == nil || task.Meta.Req == nil {
		item.Action = "skipped"
		item.Error = "task meta/req is empty"
		return item
	}
	var profile_object json.RawMessage
	if oid != "" {
		profile_url := build_target_profile_url(target_base_url, oid, uid).String()
		if cache_status.ProfileURL != "" {
			profile_url = cache_status.ProfileURL
		}
		item.ProfileURL = profile_url

		profile, cached_url, hit, cache_err := s.read_profile_cache(data_dir, cache_key)
		if cached_url != "" {
			item.ProfileURL = cached_url
		}
		if cache_err == nil && hit {
			item.ProfileCacheHit = true
			profile_object = profile.Data.Object
		}
	}

	if len(profile_object) > 0 {
		if save_path == "" {
			item.Action = "skipped"
			item.Error = "missing downloaded file path"
			return item
		}
		file_info, stat_err := os.Stat(save_path)
		if stat_err != nil && !allow_missing {
			item.Action = "failed"
			item.Error = "downloaded file is not accessible: " + stat_err.Error()
			return item
		}
		if stat_err == nil && file_info.IsDir() && !allow_missing {
			item.Action = "failed"
			item.Error = "downloaded file path is a directory"
			return item
		}
	}

	if dry_run {
		item.Action = "dry_run"
		return item
	}
	if db == nil {
		item.Action = "failed"
		item.Error = "target database is not opened"
		return item
	}
	if existing, err := find_migrated_download_task(db, safe_task_id(task)); err == nil {
		item.TargetID = existing.Id
		item.Action = "skipped"
		item.Error = "gopeed task already migrated"
		return item
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		item.Action = "failed"
		item.Error = err.Error()
		return item
	}
	var target_id int
	var err error
	if len(profile_object) > 0 {
		target_id, err = import_target_download_record(db, task, labels, oid, uid, save_path, profile_object)
	} else {
		target_id, err = import_target_download_task_from_history(
			db,
			task,
			labels,
			platform_wx_channels,
			oid,
			item.ProfileURL,
			save_path,
		)
	}
	if err != nil {
		var duplicate duplicate_import_error
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
	item.TargetID = target_id
	item.Action = "migrated"
	return item
}

func (s *migration_server) migrate_wxmp_article_task(db *gorm.DB, data_dir string, task *downloadpkg.Task, labels map[string]string, article_id string, dry_run bool, allow_missing bool) migration_execute_item {
	save_path := task_file_path(task, data_dir)
	article_url := gopeed_article_url(task, article_id)
	cache_key := wxmp_article_profile_cache_key(article_id, article_url)
	item := migration_execute_item{
		TaskID:          safe_task_id(task),
		Name:            task_name(task),
		Status:          string(task_status(task)),
		ArticleID:       article_id,
		SavePath:        save_path,
		ProfileURL:      article_url,
		ProfileCacheKey: cache_key,
	}
	if task == nil || task.Meta == nil || task.Meta.Req == nil {
		item.Action = "skipped"
		item.Error = "task meta/req is empty"
		return item
	}
	if article_url == "" {
		item.Action = "skipped"
		item.Error = "missing article_id label"
		return item
	}
	content_raw, cached_url, hit, cache_err := s.read_wxmp_article_cache(data_dir, cache_key)
	if cached_url != "" {
		item.ProfileURL = cached_url
	}
	if cache_err != nil || !hit {
		content_raw = nil
	} else {
		item.ProfileCacheHit = true
	}

	if len(content_raw) > 0 {
		if save_path == "" {
			item.Action = "skipped"
			item.Error = "missing downloaded file path"
			return item
		}
		file_info, stat_err := os.Stat(save_path)
		if stat_err != nil && !allow_missing {
			item.Action = "failed"
			item.Error = "downloaded file is not accessible: " + stat_err.Error()
			return item
		}
		if stat_err == nil && file_info.IsDir() && !allow_missing {
			item.Action = "failed"
			item.Error = "downloaded file path is a directory"
			return item
		}
	}
	if dry_run {
		item.Action = "dry_run"
		return item
	}
	if db == nil {
		item.Action = "failed"
		item.Error = "target database is not opened"
		return item
	}
	if existing, err := find_migrated_download_task_by_platform(db, platform_wxmp, safe_task_id(task)); err == nil {
		item.TargetID = existing.Id
		item.Action = "skipped"
		item.Error = "gopeed task already migrated"
		return item
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		item.Action = "failed"
		item.Error = err.Error()
		return item
	}
	var target_id int
	var err error
	if len(content_raw) > 0 {
		target_id, err = import_target_wxmp_download_record(db, task, labels, article_id, article_url, save_path, content_raw)
	} else {
		target_id, err = import_target_download_task_from_history(
			db,
			task,
			labels,
			platform_wxmp,
			article_id,
			article_url,
			save_path,
		)
	}
	if err != nil {
		var duplicate duplicate_import_error
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
	item.TargetID = target_id
	item.Action = "migrated"
	return item
}

func fetch_wxmp_article_content(article_url string) (json.RawMessage, error) {
	raw, err := wxmpadapter.NewOfficialAccountAdapter().Fetch(article_url)
	if err != nil {
		return nil, fmt.Errorf("fetch wxmp article: %w", err)
	}
	content, err := wxmp_fetch_data_to_content_json(raw)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(content))) == 0 || strings.TrimSpace(string(content)) == "null" {
		return nil, fmt.Errorf("wxmp article content is empty")
	}
	var profile wxmp.ArticleCgiDataNew
	if err := json.Unmarshal(content, &profile); err != nil || profile.UserName == "" || profile.Title == "" {
		return nil, fmt.Errorf("公众号接口未返回完整详情")
	}
	return content, nil
}

func (s *migration_server) fetch_wxmp_article_content_with_cache(data_dir string, article_id string, article_url string) (json.RawMessage, string, bool, error) {
	article_url = first_non_empty(article_url, build_wxmp_article_url(article_id))
	cache_key := wxmp_article_profile_cache_key(article_id, article_url)
	if cache_key != "" {
		content, cached_url, hit, err := s.read_wxmp_article_cache(data_dir, cache_key)
		if err != nil {
			return nil, article_url, false, err
		}
		if hit {
			if cached_url == "" {
				cached_url = article_url
			}
			return content, cached_url, true, nil
		}
	}

	content, err := fetch_wxmp_article_content(article_url)
	if err != nil {
		return nil, article_url, false, err
	}
	if cache_key != "" {
		if err := s.write_profile_cache(data_dir, cache_key, profile_cache_entry{
			OID:        strings.TrimSpace(article_id),
			ProfileURL: article_url,
			Profile:    content,
			CachedAt:   time.Now().UnixMilli(),
		}); err != nil {
			return nil, article_url, false, err
		}
	}
	return content, article_url, false, nil
}

func normalize_raw_json_object(raw json.RawMessage) json.RawMessage {
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

func wxmp_fetch_data_to_content_json(data any) (json.RawMessage, error) {
	switch value := data.(type) {
	case nil:
		return nil, fmt.Errorf("wxmp article data is nil")
	case json.RawMessage:
		return normalize_raw_json_object(value), nil
	case []byte:
		return normalize_raw_json_object(json.RawMessage(value)), nil
	case *wxmp.ArticleCgiDataNew:
		if value == nil {
			return nil, fmt.Errorf("wxmp article data is nil")
		}
		return must_raw_json(value), nil
	case wxmp.ArticleCgiDataNew:
		return must_raw_json(value), nil
	default:
		return must_raw_json(value), nil
	}
}

func fetch_target_profile(client *http.Client, raw_url string) (*channels_feed_profile, error) {
	resp, err := client.Get(raw_url)
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
	var envelope target_api_envelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse profile response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("profile api error code=%d: %s", envelope.Code, envelope.Msg)
	}
	var profile channels_feed_profile
	if err := json.Unmarshal(envelope.Data, &profile); err != nil {
		return nil, fmt.Errorf("parse profile data: %w", err)
	}
	if err := validate_channels_feed_profile(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *migration_server) fetch_profile_with_cache(client *http.Client, data_dir string, target_base_url string, cache_key string, oid string, uid string, force_agent bool) (*channels_feed_profile, string, bool, string, error) {
	cache_key = profile_cache_key(cache_key, oid)
	profile_url := build_target_profile_url(target_base_url, oid, uid).String()
	if cache_key != "" {
		profile, cached_url, hit, err := s.read_profile_cache(data_dir, cache_key)
		if err != nil {
			return nil, profile_url, false, "", err
		}
		if hit {
			if cached_url == "" {
				cached_url = profile_url
			}
			return profile, cached_url, true, "cache", nil
		}
	}

	var agent_err error
	var profile *channels_feed_profile
	source := ""
	if s != nil && s.profile_agent != nil && s.profile_agent.Available() {
		profile, agent_err = s.fetch_profile_via_agent(30*time.Second, profile_url, oid, uid)
		if agent_err != nil {
			if force_agent {
				return nil, profile_url, false, "", agent_err
			}
			log.Printf("profile agent fetch failed, fallback to target http: %v", agent_err)
		} else {
			source = "agent"
		}
	} else if force_agent {
		return nil, profile_url, false, "", err_profile_agent_unavailable
	}
	var err error
	if profile == nil {
		profile, err = fetch_target_profile(client, profile_url)
		if err != nil && agent_err != nil {
			return nil, profile_url, false, "", fmt.Errorf("profile agent: %v; fallback http: %w", agent_err, err)
		}
		source = "http"
	}
	if err != nil {
		return nil, profile_url, false, "", err
	}
	if err := validate_channels_feed_profile(profile); err != nil {
		return nil, profile_url, false, "", err
	}
	if cache_key != "" {
		if err := s.write_profile_cache(data_dir, cache_key, profile_cache_entry{
			OID:        strings.TrimSpace(oid),
			UID:        clean_nonce_id(uid),
			ProfileURL: profile_url,
			Profile:    must_raw_json(profile),
			CachedAt:   time.Now().UnixMilli(),
		}); err != nil {
			return nil, profile_url, false, "", err
		}
	}
	return profile, profile_url, false, source, nil
}

func (s *migration_server) read_profile_cache(data_dir string, key string) (*channels_feed_profile, string, bool, error) {
	s.profile_cache_mu.Lock()
	defer s.profile_cache_mu.Unlock()

	path := profile_cache_path(data_dir)
	cache, err := load_profile_cache(path)
	if err != nil {
		return nil, "", false, err
	}
	cache_key := profile_cache_key(key, "")
	entry, ok := cache.Items[cache_key]
	if !ok || len(entry.Profile) == 0 {
		return nil, "", false, nil
	}
	var profile channels_feed_profile
	if err := json.Unmarshal(entry.Profile, &profile); err != nil {
		delete(cache.Items, cache_key)
		if save_err := save_profile_cache(path, cache); save_err != nil {
			return nil, "", false, fmt.Errorf("discard invalid profile cache: %w", save_err)
		}
		log.Printf("discard invalid profile cache %q: %v", cache_key, err)
		return nil, "", false, nil
	}
	if err := validate_channels_feed_profile(&profile); err != nil {
		delete(cache.Items, cache_key)
		if save_err := save_profile_cache(path, cache); save_err != nil {
			return nil, "", false, fmt.Errorf("discard invalid profile cache: %w", save_err)
		}
		log.Printf("discard invalid profile cache %q: %v", cache_key, err)
		return nil, "", false, nil
	}
	return &profile, entry.ProfileURL, true, nil
}

func (s *migration_server) read_wxmp_article_cache(data_dir string, key string) (json.RawMessage, string, bool, error) {
	s.profile_cache_mu.Lock()
	defer s.profile_cache_mu.Unlock()

	cache, err := load_profile_cache(profile_cache_path(data_dir))
	if err != nil {
		return nil, "", false, err
	}
	entry, ok := cache.Items[profile_cache_key(key, "")]
	if !ok || len(entry.Profile) == 0 {
		return nil, "", false, nil
	}
	content := normalize_raw_json_object(entry.Profile)
	var profile wxmp.ArticleCgiDataNew
	if json.Unmarshal(content, &profile) != nil || profile.UserName == "" || profile.Title == "" {
		return nil, "", false, nil
	}
	if len(content) == 0 || strings.TrimSpace(string(content)) == "null" {
		return nil, "", false, fmt.Errorf("article cache is empty for %s", key)
	}
	return content, entry.ProfileURL, true, nil
}

func (s *migration_server) write_profile_cache(data_dir string, key string, entry profile_cache_entry) error {
	s.profile_cache_mu.Lock()
	defer s.profile_cache_mu.Unlock()

	path := profile_cache_path(data_dir)
	cache, err := load_profile_cache(path)
	if err != nil {
		return err
	}
	if cache.Items == nil {
		cache.Items = map[string]profile_cache_entry{}
	}
	cache.Version = 1
	cache.Items[profile_cache_key(key, "")] = entry
	return save_profile_cache(path, cache)
}

func (s *migration_server) cleanup_invalid_profile_cache(data_dir string) (*profile_cache_cleanup_result, error) {
	s.profile_cache_mu.Lock()
	defer s.profile_cache_mu.Unlock()

	cache_path := profile_cache_path(data_dir)
	cache, err := load_profile_cache(cache_path)
	if err != nil {
		return nil, err
	}
	result := &profile_cache_cleanup_result{
		DataDir:     data_dir,
		CachePath:   cache_path,
		Total:       len(cache.Items),
		RemovedKeys: []string{},
	}
	for cache_key, entry := range cache.Items {
		if strings.HasPrefix(cache_key, "wxmp_article:") {
			result.SkippedArticle += 1
			continue
		}
		result.Checked += 1
		var profile channels_feed_profile
		validation_err := json.Unmarshal(entry.Profile, &profile)
		if validation_err == nil {
			validation_err = validate_channels_feed_profile(&profile)
		}
		if validation_err == nil {
			continue
		}
		delete(cache.Items, cache_key)
		result.RemovedKeys = append(result.RemovedKeys, cache_key)
	}
	sort.Strings(result.RemovedKeys)
	result.Removed = len(result.RemovedKeys)
	result.Remaining = len(cache.Items)
	if result.Removed > 0 {
		if err := save_profile_cache(cache_path, cache); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func load_profile_cache(path string) (*profile_cache_document, error) {
	cache := &profile_cache_document{
		Version: 1,
		Items:   map[string]profile_cache_entry{},
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
		cache.Items = map[string]profile_cache_entry{}
	}
	return cache, nil
}

func save_profile_cache(path string, cache *profile_cache_document) error {
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

func find_migrated_download_task(db *gorm.DB, gopeed_id string) (*model.DownloadTask, error) {
	return find_migrated_download_task_by_platform(db, platform_wx_channels, gopeed_id)
}

func find_migrated_download_task_by_platform(db *gorm.DB, platform_id string, gopeed_id string) (*model.DownloadTask, error) {
	if db == nil {
		return nil, fmt.Errorf("target database is not opened")
	}
	platform_id = strings.TrimSpace(platform_id)
	if platform_id == "" {
		platform_id = platform_wx_channels
	}
	gopeed_id = strings.TrimSpace(gopeed_id)

	if gopeed_id == "" {
		return nil, gorm.ErrRecordNotFound
	}
	for _, key := range []string{"gopeedid", "gopeed_task_id"} {
		var task model.DownloadTask
		pattern := "%\"" + key + "\":" + json_string_literal(gopeed_id) + "%"
		result := db.Where("platform_id = ? AND deleted_at IS NULL AND metadata_json LIKE ?", platform_id, pattern).Limit(1).Find(&task)
		if result.Error != nil {
			return nil, fmt.Errorf("query migrated gopeed task metadata: %w", result.Error)
		}
		if result.RowsAffected > 0 {
			return &task, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func json_string_literal(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(data)
}

func (s *migration_server) resolve_data_dir(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = s.default_data_dir
	}
	clean := filepath.Clean(expand_home(raw))
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", "", fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", fmt.Errorf("path is not accessible: %w", err)
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("select a .db database file, not a directory: %s", abs)
	}
	if !strings.EqualFold(filepath.Ext(abs), ".db") {
		return "", "", fmt.Errorf("select a .db database file: %s", abs)
	}
	return filepath.Dir(abs), abs, nil
}

func (s *migration_server) resolve_profile_cache_dir(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = s.default_data_dir
	}
	if data_dir, _, err := s.resolve_data_dir(raw); err == nil {
		return data_dir, nil
	}
	clean := filepath.Clean(expand_home(raw))
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

func (s *migration_server) load_gopeed_task_snapshot(data_dir string, db_path string, force_reload bool) (*task_cache_entry, bool, error) {
	db_info, err := os.Stat(db_path)
	if err != nil {
		return nil, false, fmt.Errorf("stat %s: %w", db_path, err)
	}

	s.task_cache_mu.Lock()
	defer s.task_cache_mu.Unlock()
	if s.task_cache == nil {
		s.task_cache = make(map[string]*task_cache_entry)
	}
	if !force_reload {
		if cached := s.task_cache[db_path]; cached != nil && cached.ModTime.Equal(db_info.ModTime()) && cached.Size == db_info.Size() {
			return cached, true, nil
		}
	}

	tasks, err := load_all_gopeed_tasks(db_path)
	if err != nil {
		return nil, false, err
	}
	entry := &task_cache_entry{
		DataDir:  data_dir,
		DBPath:   db_path,
		ModTime:  db_info.ModTime(),
		Size:     db_info.Size(),
		Tasks:    tasks,
		Stats:    build_status_stats(tasks),
		LoadedAt: time.Now(),
	}
	s.task_cache[db_path] = entry
	return entry, false, nil
}

func load_gopeed_tasks(db_path string, status string) (filtered []*downloadpkg.Task, all []*downloadpkg.Task, err error) {
	all, err = load_all_gopeed_tasks(db_path)
	if err != nil {
		return nil, nil, err
	}
	return filter_gopeed_tasks(all, status), all, nil
}

func load_all_gopeed_tasks(db_path string) (all []*downloadpkg.Task, err error) {
	data_dir := filepath.Dir(db_path)
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

	storage = downloadpkg.NewReadOnlyBoltStorageFromPath(db_path)
	downloader = downloadpkg.NewDownloader(&downloadpkg.DownloaderConfig{
		RefreshInterval: 360,
		Storage:         storage,
		StorageDir:      data_dir,
	})
	if err = downloader.Setup(); err != nil {
		_ = storage.Close()
		return nil, err
	}
	defer func() {
		if close_err := downloader.CloseStorageOnly(); close_err != nil && err == nil {
			err = close_err
		}
	}()

	return append([]*downloadpkg.Task(nil), downloader.GetTasks()...), nil
}

func filter_gopeed_tasks(tasks []*downloadpkg.Task, status string) []*downloadpkg.Task {
	status = normalize_status(status)
	if status == "" || status == "all" {
		return append([]*downloadpkg.Task(nil), tasks...)
	}
	filtered := make([]*downloadpkg.Task, 0, len(tasks))
	for _, task := range tasks {
		if string(task_status(task)) == status {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func clone_status_stats(stats map[string]int) map[string]int {
	cloned := make(map[string]int, len(stats))
	for key, value := range stats {
		cloned[key] = value
	}
	return cloned
}

func task_to_row(task *downloadpkg.Task) map[string]any {
	labels := task_labels(task)
	size := task_size(task)
	downloaded := task_downloaded(task)
	progress := 0.0
	if size > 0 {
		progress = math.Round((float64(downloaded)/float64(size))*10000) / 100
	} else if task != nil && task.Status == base.DownloadStatusDone {
		progress = 100
	}
	return map[string]any{
		"id":          safe_task_id(task),
		"name":        task_name(task),
		"status":      string(task_status(task)),
		"protocol":    task_protocol(task),
		"url":         task_url(task),
		"save_path":   task_save_path(task),
		"size":        size,
		"downloaded":  downloaded,
		"progress":    progress,
		"oid":         first_label(labels, "oid", "id", "external_id", "objectid", "object_id"),
		"uid":         first_label(labels, "uid", "nid", "nonce_id", "objectNonceId", "object_nonce_id"),
		"article_id":  first_label(labels, "article_id", "articleid", "mp_article_id"),
		"external_id": labels["id"],
		"nonce_id":    labels["nonce_id"],
		"title":       first_non_empty(labels["title"], task_name(task)),
		"labels":      labels,
		"created_at":  format_time(task_created_at(task)),
		"updated_at":  format_time(task_updated_at(task)),
		"error":       task_error(task),
	}
}

func task_profile_cached(cache *profile_cache_document, task *downloadpkg.Task) (bool, string) {
	if cache == nil || cache.Items == nil {
		return false, ""
	}
	labels := task_labels(task)
	if article_id := first_label(labels, "article_id", "articleid", "mp_article_id"); article_id != "" {
		cache_key := wxmp_article_profile_cache_key(article_id, build_wxmp_article_url(article_id))
		if cache_key == "" {
			return false, ""
		}
		entry, ok := cache.Items[cache_key]
		if !ok {
			return false, cache_key
		}
		profile := strings.TrimSpace(string(entry.Profile))
		return profile != "" && profile != "null", cache_key
	}
	oid := first_label(labels, "oid", "id", "external_id", "objectid", "object_id")
	cache_key := profile_cache_key(task_profile_cache_key(labels, oid), oid)
	if cache_key == "" {
		return false, ""
	}
	entry, ok := cache.Items[cache_key]
	if !ok {
		return false, cache_key
	}
	var profile channels_feed_profile
	if err := json.Unmarshal(entry.Profile, &profile); err != nil {
		return false, cache_key
	}
	return validate_channels_feed_profile(&profile) == nil, cache_key
}

func safe_task_id(task *downloadpkg.Task) string {
	if task == nil {
		return ""
	}
	return task.ID
}

func task_status(task *downloadpkg.Task) base.Status {
	if task == nil {
		return ""
	}
	return task.Status
}

func task_protocol(task *downloadpkg.Task) string {
	if task == nil {
		return ""
	}
	return task.Protocol
}

func task_labels(task *downloadpkg.Task) map[string]string {
	if task == nil || task.Meta == nil || task.Meta.Req == nil || task.Meta.Req.Labels == nil {
		return map[string]string{}
	}
	return task.Meta.Req.Labels
}

func task_url(task *downloadpkg.Task) string {
	if task == nil || task.Meta == nil || task.Meta.Req == nil {
		return ""
	}
	return task.Meta.Req.URL
}

func task_name(task *downloadpkg.Task) string {
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
	if name := name_from_url(task_url(task)); name != "" {
		return name
	}
	if task.ID != "" {
		return task.ID
	}
	return "unknown"
}

func task_save_path(task *downloadpkg.Task) string {
	if task == nil || task.Meta == nil {
		return ""
	}
	download_dir := ""
	if task.Meta.Opts != nil {
		download_dir = strings.TrimSpace(task.Meta.Opts.Path)
	}
	if task.Meta.Res == nil {
		return download_dir
	}
	if task.Meta.Res.Name != "" {
		name := task.Meta.Res.Name
		if task.Meta.Opts != nil && task.Meta.Opts.Name != "" {
			name = task.Meta.Opts.Name
		}
		return filepath.Join(download_dir, name)
	}
	if len(task.Meta.Res.Files) == 0 || task.Meta.Res.Files[0] == nil {
		return download_dir
	}
	file := task.Meta.Res.Files[0]
	name := file.Name
	if task.Meta.Opts != nil && task.Meta.Opts.Name != "" {
		name = task.Meta.Opts.Name
	}
	return filepath.Join(download_dir, file.Path, name)
}

func task_file_path(task *downloadpkg.Task, data_dir string) string {
	save_path := strings.TrimSpace(task_save_path(task))
	if save_path == "" {
		return ""
	}
	save_path = expand_home(save_path)
	if filepath.IsAbs(save_path) {
		return filepath.Clean(save_path)
	}
	if data_dir != "" {
		candidate := filepath.Join(data_dir, save_path)
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(candidate)
		}
	}
	abs, err := filepath.Abs(save_path)
	if err != nil {
		return filepath.Clean(save_path)
	}
	return abs
}

func task_size(task *downloadpkg.Task) int64 {
	if task == nil || task.Meta == nil || task.Meta.Res == nil {
		return task_downloaded(task)
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
	return task_downloaded(task)
}

func task_downloaded(task *downloadpkg.Task) int64 {
	if task == nil || task.Progress == nil {
		return 0
	}
	return task.Progress.Downloaded
}

func task_error(task *downloadpkg.Task) string {
	if task == nil {
		return ""
	}
	return task.Error
}

func task_created_at(task *downloadpkg.Task) time.Time {
	if task == nil {
		return time.Time{}
	}
	return task.CreatedAt
}

func task_updated_at(task *downloadpkg.Task) time.Time {
	if task == nil {
		return time.Time{}
	}
	return task.UpdatedAt
}

func first_label(labels map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(labels[key]); value != "" {
			return value
		}
	}
	return ""
}

func clean_nonce_id(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexByte(value, '_'); idx > 0 {
		return value[:idx]
	}
	return value
}

func normalize_target_base_url(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = default_target_base_url
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return default_target_base_url
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func target_endpoint(base_url string, endpoint_path string) *url.URL {
	parsed, err := url.Parse(normalize_target_base_url(base_url))
	if err != nil {
		parsed, _ = url.Parse(default_target_base_url)
	}
	base_path := strings.TrimRight(parsed.Path, "/")
	parsed.Path = base_path + "/" + strings.TrimLeft(endpoint_path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed
}

func build_target_profile_url(target_base_url string, oid string, uid string) *url.URL {
	profile_url := target_endpoint(target_base_url, "/api/channels/feed/profile")
	query := profile_url.Query()
	query.Set("oid", strings.TrimSpace(oid))
	query.Set("nid", clean_nonce_id(uid))
	query.Set("uid", clean_nonce_id(uid))
	profile_url.RawQuery = query.Encode()
	return profile_url
}

func build_wxmp_article_url(article_id string) string {
	article_id = strings.TrimSpace(article_id)
	if article_id == "" {
		return ""
	}
	lower := strings.ToLower(article_id)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return article_id
	}
	article_id = strings.TrimLeft(article_id, "/")
	if strings.HasPrefix(strings.ToLower(article_id), "mp.weixin.qq.com/") {
		return "https://" + article_id
	}
	return "https://mp.weixin.qq.com/s/" + url.PathEscape(article_id)
}

func wxmp_article_profile_cache_key(article_id string, article_url string) string {
	value := first_non_empty(article_id, article_url)
	if value == "" {
		return ""
	}
	return "wxmp_article:" + value
}

func profile_cache_path(data_dir string) string {
	return filepath.Join(data_dir, profile_cache_file)
}

func task_profile_cache_key(labels map[string]string, oid string) string {
	return profile_cache_key(first_label(labels, "id", "oid", "external_id", "objectid", "object_id"), oid)
}

func profile_cache_key(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	return value
}

func migrated_task_unique_id(oid string, labels map[string]string, save_path string) string {
	unique_id := strings.TrimSpace(oid)
	if unique_id == "" {
		unique_id = filepath.Base(save_path)
	}
	spec := first_label(labels, "spec", "file_format", "format", "quality")
	if spec != "" && spec != "original" {
		unique_id += "_" + spec
	}
	suffix := first_non_empty(labels["suffix"], migration_suffix_config(save_path))
	if strings.EqualFold(suffix, ".mp3") && !strings.HasSuffix(unique_id, "_mp3") {
		unique_id += "_mp3"
	}
	if strings.EqualFold(suffix, ".jpg") && !strings.HasSuffix(unique_id, "_cover") {
		unique_id += "_cover"
	}
	return unique_id
}

func migrated_wxmp_task_unique_id(article_id string, save_path string) string {
	unique_id := strings.TrimSpace(article_id)
	if unique_id == "" {
		unique_id = filepath.Base(save_path)
	}
	suffix := migration_suffix_config(save_path)
	if suffix != "" {
		suffix = strings.TrimPrefix(strings.ToLower(suffix), ".")
		if suffix != "" && !strings.HasSuffix(unique_id, "_"+suffix) {
			unique_id += "_" + suffix
		}
	}
	return unique_id
}

func migrated_size(task *downloadpkg.Task, save_path string) int64 {
	size := task_size(task)
	if info, err := os.Stat(save_path); err == nil && !info.IsDir() && info.Size() > 0 {
		size = info.Size()
	}
	return size
}

func resource_kind_for_path(save_path string) string {
	switch strings.ToLower(filepath.Ext(save_path)) {
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
	case ".html", ".htm":
		return "html"
	default:
		return "file"
	}
}

func endpoint_protocol(raw_url string, fallback string) string {
	if parsed, err := url.Parse(strings.TrimSpace(raw_url)); err == nil && parsed.Scheme != "" {
		return strings.ToLower(parsed.Scheme)
	}
	fallback = strings.TrimSpace(strings.ToLower(fallback))
	if fallback != "" {
		return fallback
	}
	return "file"
}

func build_channels_fallback_source_url(oid string, uid string, username string) string {
	query := url.Values{}
	if strings.TrimSpace(username) != "" {
		query.Set("username", strings.TrimSpace(username))
	}
	if strings.TrimSpace(oid) != "" {
		query.Set("oid", strings.TrimSpace(oid))
	}
	if strings.TrimSpace(uid) != "" {
		query.Set("nid", clean_nonce_id(uid))
	}
	if encoded := query.Encode(); encoded != "" {
		return "https://channels.weixin.qq.com/web/pages/feed?" + encoded
	}
	return ""
}

func must_json(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func build_migration_task_config_json(download_dir string, filename string, labels map[string]string) string {
	config := map[string]any{
		"download_dir": download_dir,
	}
	if suffix := migration_suffix_config(filename); suffix != "" {
		config["suffix"] = suffix
	}
	if spec := strings.TrimSpace(labels["spec"]); spec != "" {
		config["spec"] = spec
	}
	return must_json(config)
}

func build_wxmp_migration_task_config_json(download_dir string, filename string, labels map[string]string) string {
	config := map[string]any{
		"download_dir": download_dir,
	}
	if filename = strings.TrimSpace(filename); filename != "" && filename != "." {
		config["filename"] = filename
	}
	if suffix := migration_suffix_config(filename); suffix != "" {
		config["suffix"] = suffix
	}
	if template := strings.TrimSpace(labels["filename_template"]); template != "" {
		config["filename_template"] = template
	}
	return must_json(config)
}

func merge_migration_metadata_json(base string, extra map[string]any) string {
	metadata := map[string]any{}
	if strings.TrimSpace(base) != "" {
		_ = json.Unmarshal([]byte(base), &metadata)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return must_json(metadata)
}

func must_raw_json(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(data)
}

func migration_suffix_config(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp4", ".mp3", ".jpg", ".jpeg", ".png":
		return strings.ToLower(filepath.Ext(filename))
	default:
		return ""
	}
}

func name_from_url(raw string) string {
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

func build_status_stats(tasks []*downloadpkg.Task) map[string]int {
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
		status := string(task_status(task))
		if _, ok := stats[status]; ok {
			stats[status]++
		}
	}
	return stats
}

func list_files(raw_dir string) (string, []file_item, error) {
	dir := strings.TrimSpace(raw_dir)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", nil, err
		}
		dir = home
	}
	dir = expand_home(dir)
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
	items := make([]file_item, 0, len(entries))
	for _, entry := range entries {
		entry_info, err := entry.Info()
		if err != nil {
			continue
		}
		is_dir := entry_info.IsDir()
		file_type := "file"
		if is_dir {
			file_type = "dir"
		}
		items = append(items, file_item{
			Name:        entry.Name(),
			Path:        filepath.Join(abs, entry.Name()),
			IsDir:       is_dir,
			IsDirectory: is_dir,
			Type:        file_type,
			Size:        entry_info.Size(),
			ModTime:     entry_info.ModTime().Format("2006-01-02 15:04"),
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

func common_dirs(default_data_dir string) []string {
	seen := map[string]bool{}
	dirs := make([]string, 0)
	add := func(dir string) {
		dir = strings.TrimSpace(expand_home(dir))
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
	add(default_data_dir)
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

func decode_json(r *http.Request, v any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func require_method(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	write_api_error(w, 405, "method not allowed")
	return false
}

func write_apiok(w http.ResponseWriter, data any) {
	write_api_response(w, api_response{Code: 0, Msg: "success", Data: data})
}

func write_api_error(w http.ResponseWriter, code int, msg string) {
	write_api_response(w, api_response{Code: code, Msg: msg})
}

func write_api_response(w http.ResponseWriter, resp api_response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func normalize_pagination(page int, page_size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if page_size < 1 {
		page_size = 100
	}
	if page_size > 1000 {
		page_size = 1000
	}
	return page, page_size
}

func normalize_status(status string) string {
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

func format_time(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func time_to_millis(t time.Time) int64 {
	if t.IsZero() {
		return time.Now().UnixMilli()
	}
	return t.UnixMilli()
}

func time_to_millis_or_default(t time.Time, fallback int64) int64 {
	if t.IsZero() {
		return fallback
	}
	return t.UnixMilli()
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func truthy_query_value(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func expand_home(raw string) string {
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
