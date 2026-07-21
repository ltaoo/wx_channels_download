package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"wx_channel/internal/events"
)

func (s *AdminServer) handleDaemonStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := adminAPIDaemonName(r)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":    name,
		"running": s.serviceRunning(name),
	})
}

func (s *AdminServer) handleDaemonStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := adminAPIBodyName(r)
	if s.bus != nil {
		s.bus.Publish(events.ServiceCommand{Name: name, Action: "start"})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"name": name, "started": true})
}

func (s *AdminServer) handleDaemonStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := adminAPIBodyName(r)
	if name == "admin" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "admin service cannot stop itself from HTTP"})
		return
	}
	if s.bus != nil {
		s.bus.Publish(events.ServiceCommand{Name: name, Action: "stop"})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"name": name, "stopped": true})
}

func (s *AdminServer) handleDaemonRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := adminAPIBodyName(r)
	if name == "admin" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "admin service cannot restart itself from HTTP"})
		return
	}
	if s.bus != nil {
		s.bus.Publish(events.ServiceCommand{Name: name, Action: "stop"})
		s.bus.Publish(events.ServiceCommand{Name: name, Action: "start"})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"name": name, "restarted": true})
}

func (s *AdminServer) handleDaemonRemoteUnsupported(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]interface{}{"error": "remote daemon is not implemented in this admin service"})
}

func (s *AdminServer) handleDaemonTabs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tabs": []interface{}{}, "count": 0})
}

func (s *AdminServer) handleDaemonPageInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

func (s *AdminServer) handleDaemonDebuggerURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]interface{}{"error": "daemon debugger discovery is not implemented in this admin service"})
}

func (s *AdminServer) handleDaemonFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]interface{}{"error": "daemon fetch is not implemented in this admin service"})
}

func (s *AdminServer) handleApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"apps": []interface{}{}, "installed": []interface{}{}, "count": 0})
}

func (s *AdminServer) handleAppTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": []interface{}{}, "count": 0})
}

func (s *AdminServer) handleAppTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	taskID := strings.TrimPrefix(r.URL.Path, "/api/v1/apps/tasks/")
	if taskID == "" {
		s.handleAppTasks(w, r)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "task not found"})
}

func (s *AdminServer) handleAppTaskUnsupported(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]interface{}{"error": "appstore is not implemented in this admin service"})
}

func (s *AdminServer) serviceRunning(name string) bool {
	for _, svc := range s.listServices() {
		if normalizeServiceName(svc.Name) == normalizeServiceName(name) {
			return string(svc.Status) == "running"
		}
	}
	return false
}

func adminAPIDaemonName(r *http.Request) string {
	name := normalizeServiceName(r.URL.Query().Get("name"))
	if name == "" || name == "default" {
		return "api"
	}
	return name
}

func adminAPIBodyName(r *http.Request) string {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	name := normalizeServiceName(body.Name)
	if name == "" || name == "default" {
		return "api"
	}
	return name
}
