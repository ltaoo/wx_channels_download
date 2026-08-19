package services

import (
	"strings"
	"sync"

	"wx_channel/internal/events"
)

// RuntimeStatusService stores the latest process-local component and platform states.
type RuntimeStatusService struct {
	proxy_status_mu sync.RWMutex
	proxy_status    events.ProxyStatusChanged

	service_status_mu sync.RWMutex
	service_statuses  map[string]events.ServiceStatusChanged

	platform_status_mu sync.RWMutex
	platform_statuses  map[string]events.PlatformStatusChanged
}

// NewRuntimeStatusService constructs an empty runtime status store.
func NewRuntimeStatusService() *RuntimeStatusService {
	return &RuntimeStatusService{
		service_statuses:  make(map[string]events.ServiceStatusChanged),
		platform_statuses: make(map[string]events.PlatformStatusChanged),
	}
}

// UpdateProxyStatus stores the latest proxy state.
func (s *RuntimeStatusService) UpdateProxyStatus(status events.ProxyStatusChanged) {
	if s == nil {
		return
	}
	s.proxy_status_mu.Lock()
	s.proxy_status = status
	s.proxy_status_mu.Unlock()
}

// ProxyStatus returns the latest proxy state.
func (s *RuntimeStatusService) ProxyStatus() events.ProxyStatusChanged {
	if s == nil {
		return events.ProxyStatusChanged{}
	}
	s.proxy_status_mu.RLock()
	defer s.proxy_status_mu.RUnlock()
	return s.proxy_status
}

// UpdateServiceStatus stores the latest state for a named service.
func (s *RuntimeStatusService) UpdateServiceStatus(status events.ServiceStatusChanged) {
	if s == nil {
		return
	}
	s.service_status_mu.Lock()
	if s.service_statuses == nil {
		s.service_statuses = make(map[string]events.ServiceStatusChanged)
	}
	s.service_statuses[status.Name] = status
	s.service_status_mu.Unlock()
}

// ServiceStatuses returns a copy of the latest named service states.
func (s *RuntimeStatusService) ServiceStatuses() map[string]string {
	if s == nil {
		return map[string]string{}
	}
	s.service_status_mu.RLock()
	defer s.service_status_mu.RUnlock()
	statuses := make(map[string]string, len(s.service_statuses))
	for name, status := range s.service_statuses {
		statuses[name] = status.Status
	}
	return statuses
}

// UpdatePlatformStatus normalizes and stores a changed platform state.
// The returned boolean is false for invalid or duplicate updates.
func (s *RuntimeStatusService) UpdatePlatformStatus(status events.PlatformStatusChanged) (events.PlatformStatusChanged, bool) {
	status, valid := normalize_runtime_platform_status(status)
	if s == nil || !valid {
		return status, false
	}

	s.platform_status_mu.Lock()
	defer s.platform_status_mu.Unlock()
	if s.platform_statuses == nil {
		s.platform_statuses = make(map[string]events.PlatformStatusChanged)
	}
	previous_status, exists := s.platform_statuses[status.Key]
	if exists &&
		previous_status.Available == status.Available &&
		previous_status.Status == status.Status &&
		previous_status.Name == status.Name &&
		previous_status.Reason == status.Reason {
		return status, false
	}
	s.platform_statuses[status.Key] = status
	return status, true
}

// PlatformStatuses returns a copy of the latest platform states keyed by descriptor key.
func (s *RuntimeStatusService) PlatformStatuses() map[string]events.PlatformStatusChanged {
	if s == nil {
		return map[string]events.PlatformStatusChanged{}
	}
	s.platform_status_mu.RLock()
	defer s.platform_status_mu.RUnlock()
	statuses := make(map[string]events.PlatformStatusChanged, len(s.platform_statuses))
	for key, status := range s.platform_statuses {
		statuses[key] = status
	}
	return statuses
}

func normalize_runtime_platform_status(status events.PlatformStatusChanged) (events.PlatformStatusChanged, bool) {
	status.Platform = strings.TrimSpace(status.Platform)
	status.Key = strings.TrimSpace(status.Key)
	status.Name = strings.TrimSpace(status.Name)
	status.Status = strings.TrimSpace(status.Status)
	status.Reason = strings.TrimSpace(status.Reason)
	if status.Platform == "" {
		return status, false
	}
	if status.Key == "" {
		status.Key = status.Platform
	}
	if status.Status == "" {
		if status.Available {
			status.Status = "available"
		} else {
			status.Status = "unavailable"
		}
	}
	switch status.Status {
	case "available":
		status.Available = true
	case "checking", "unavailable":
		status.Available = false
	}
	return status, true
}
