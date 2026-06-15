package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ltaoo/velo"
	"github.com/ltaoo/velo/frontendserver"
	"github.com/spf13/viper"

	"wx_channel/frontend"
	"wx_channel/internal/config"
	"wx_channel/internal/manager"
	"wx_channel/pkg/browsermgr"
)

type ServiceSnapshot struct {
	Name   string               `json:"name"`
	Title  string               `json:"title"`
	Addr   string               `json:"addr"`
	Status manager.ServerStatus `json:"status"`
}

type ServiceController interface {
	ListServices() []ServiceSnapshot
	StartService(name string) error
	StopService(name string) error
}

type AdminConfig struct {
	Hostname string
	Port     int
}

type AdminServer struct {
	*manager.HTTPServer
	cfg        *config.Config
	app        *velo.Box
	controller ServiceController
	browserMgr *browsermgr.Manager
}

func NewAdminServer(cfg *config.Config, app *velo.Box, controller ServiceController) *AdminServer {
	adminCfg := NewAdminConfig()
	srv := manager.NewHTTPServer("GUI/Admin服务", "admin", adminCfg.Hostname+":"+strconv.Itoa(adminCfg.Port))
	admin := &AdminServer{
		HTTPServer: srv,
		cfg:        cfg,
		app:        app,
		controller: controller,
	}
	browserMgr, _ := browsermgr.New(browsermgr.Config{
		WorkDir:          cfg.WorkDir,
		DockerImage:      viper.GetString("sandbox.dockerImage"),
		DockerEntrypoint: viper.GetString("sandbox.dockerEntrypoint"),
		DockerNetwork:    viper.GetString("sandbox.dockerNetwork"),
		CDPPortMin:       viper.GetInt("sandbox.cdpPortMin"),
		CDPPortMax:       viper.GetInt("sandbox.cdpPortMax"),
		DesktopPortMin:   viper.GetInt("sandbox.desktopPortMin"),
		DesktopPortMax:   viper.GetInt("sandbox.desktopPortMax"),
		Resolution:       viper.GetString("sandbox.resolution"),
		ShmSize:          viper.GetString("sandbox.shmSize"),
		MemoryLimit:      viper.GetString("sandbox.memoryLimit"),
		ChromeCommand:    viper.GetString("sandbox.chromeCommand"),
	}, nil)
	admin.browserMgr = browserMgr
	srv.SetHandler(admin.routes())
	return admin
}

func NewAdminConfig() *AdminConfig {
	return &AdminConfig{
		Hostname: viper.GetString("admin.hostname"),
		Port:     viper.GetInt("admin.port"),
	}
}

func (s *AdminServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/admin/status", s.handleStatus)
	mux.HandleFunc("/api/admin/services", s.handleServices)
	mux.HandleFunc("/api/admin/service/start", s.handleServiceStart)
	mux.HandleFunc("/api/admin/service/stop", s.handleServiceStop)
	mux.HandleFunc("/api/admin/config", s.handleConfig)
	mux.HandleFunc("/api/admin/config/repair", s.handleConfigRepair)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/v1/daemons/status", s.handleDaemonStatus)
	mux.HandleFunc("/api/v1/daemons/start", s.handleDaemonStart)
	mux.HandleFunc("/api/v1/daemons/stop", s.handleDaemonStop)
	mux.HandleFunc("/api/v1/daemons/restart", s.handleDaemonRestart)
	mux.HandleFunc("/api/v1/daemons/remote/start", s.handleDaemonRemoteUnsupported)
	mux.HandleFunc("/api/v1/daemons/remote/stop", s.handleDaemonRemoteUnsupported)
	mux.HandleFunc("/api/v1/daemons/tabs", s.handleDaemonTabs)
	mux.HandleFunc("/api/v1/daemons/page-info", s.handleDaemonPageInfo)
	mux.HandleFunc("/api/v1/daemons/debugger-url", s.handleDaemonDebuggerURL)
	mux.HandleFunc("/api/v1/daemons/fetch", s.handleDaemonFetch)
	mux.HandleFunc("/api/v1/apps", s.handleApps)
	mux.HandleFunc("/api/v1/apps/install", s.handleAppTaskUnsupported)
	mux.HandleFunc("/api/v1/apps/remove", s.handleAppTaskUnsupported)
	mux.HandleFunc("/api/v1/apps/update", s.handleAppTaskUnsupported)
	mux.HandleFunc("/api/v1/apps/run", s.handleAppTaskUnsupported)
	mux.HandleFunc("/api/v1/apps/tasks", s.handleAppTasks)
	mux.HandleFunc("/api/v1/apps/tasks/", s.handleAppTask)
	mux.HandleFunc("/api/v1/sandboxes", s.handleSandboxes)
	mux.HandleFunc("/api/v1/sandboxes/", s.handleSandbox)
	mux.Handle("/", s.frontendHandler())
	return mux
}

func (s *AdminServer) frontendHandler() http.Handler {
	root := filepath.Join(s.cfg.RootDir, "frontend")
	if s.cfg.Mode == "debug" {
		if _, err := os.Stat(filepath.Join(root, "index.html")); err == nil {
			return frontendserver.New(frontendserver.Options{
				Mode:               frontendserver.ModeDev,
				Root:               root,
				EntryPage:          "index.html",
				NoFallbackPrefixes: []string{"/api"},
			})
		}
	}
	return frontendserver.New(frontendserver.Options{
		Mode:               frontendserver.ModeProd,
		Root:               ".",
		Embedded:           frontend.FS,
		EntryPage:          "index.html",
		NoFallbackPrefixes: []string{"/api"},
	})
}

func (s *AdminServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.writeOK(w, map[string]interface{}{
		"version":      s.cfg.Version,
		"mode":         s.cfg.Mode,
		"configPath":   s.configPath(),
		"veloVersion":  velo.GetVersion(),
		"veloDatabase": s.app != nil && s.app.DB != nil,
		"services":     s.controller.ListServices(),
	})
}

func (s *AdminServer) handleServices(w http.ResponseWriter, r *http.Request) {
	s.writeOK(w, s.controller.ListServices())
}

func (s *AdminServer) handleServiceStart(w http.ResponseWriter, r *http.Request) {
	name := requestServiceName(r)
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "service is required")
		return
	}
	if err := s.controller.StartService(name); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeOK(w, s.controller.ListServices())
}

func (s *AdminServer) handleServiceStop(w http.ResponseWriter, r *http.Request) {
	name := requestServiceName(r)
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "service is required")
		return
	}
	if name == "admin" {
		s.writeError(w, http.StatusBadRequest, "admin service cannot stop itself from HTTP")
		return
	}
	if err := s.controller.StopService(name); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeOK(w, s.controller.ListServices())
}

func (s *AdminServer) handleConfigRepair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.writeConfig(); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeOK(w, map[string]interface{}{
		"configPath": s.configPath(),
		"message":    "config repaired",
	})
}

func (s *AdminServer) writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

func (s *AdminServer) writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{
		"code": 100,
		"msg":  message,
		"data": nil,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func requestServiceName(r *http.Request) string {
	if r.Method == http.MethodPost {
		var body struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "" {
			return normalizeServiceName(body.Name)
		}
	}
	return normalizeServiceName(r.URL.Query().Get("name"))
}

func normalizeServiceName(name string) string {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "proxy":
		return "interceptor"
	default:
		return strings.TrimSpace(strings.ToLower(name))
	}
}

func (s *AdminServer) configPath() string {
	if s.cfg.FullPath != "" {
		return s.cfg.FullPath
	}
	return filepath.Join(s.cfg.RootDir, s.cfg.Filename)
}

func (s *AdminServer) writeConfig() error {
	path := s.configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return viper.WriteConfigAs(path)
	}
	return viper.SafeWriteConfigAs(path)
}
