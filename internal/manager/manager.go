package manager

import (
	"fmt"
	"sync"
)

type ServerManager struct {
	servers map[string]Server
	mu      sync.RWMutex
}

func NewServerManager() *ServerManager {
	return &ServerManager{
		servers: make(map[string]Server),
	}
}

func (sm *ServerManager) RegisterServer(server Server) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.servers[server.Name()] = server
}

func (sm *ServerManager) StartServer(name string) error {
	sm.mu.RLock()
	server, exists := sm.servers[name]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("server %s not found", name)
	}

	return server.Start()
}

func (sm *ServerManager) StopServer(name string) error {
	sm.mu.RLock()
	server, exists := sm.servers[name]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("server %s not found", name)
	}

	return server.Stop()
}

func (sm *ServerManager) GetStatus(name string) (ServerStatus, error) {
	sm.mu.RLock()
	server, exists := sm.servers[name]
	sm.mu.RUnlock()

	if !exists {
		return StatusStopped, fmt.Errorf("server %s not found", name)
	}

	return server.Status(), nil
}

func (sm *ServerManager) GetAllStatus() map[string]ServerStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	statuses := make(map[string]ServerStatus)
	for name, server := range sm.servers {
		statuses[name] = server.Status()
	}
	return statuses
}

func (sm *ServerManager) ListServers() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	names := make([]string, 0, len(sm.servers))
	for name := range sm.servers {
		names = append(names, name)
	}
	return names
}
