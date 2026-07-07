package browsermgr

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Kind string

const (
	KindLocal    Kind = "local"
	KindDocker   Kind = "docker"
	KindExisting Kind = "existing"
)

type Status string

const (
	StatusCreating    Status = "creating"
	StatusIdle        Status = "idle"
	StatusBusy        Status = "busy"
	StatusInvalid     Status = "invalid"
	StatusRunning     Status = "running"
	StatusStopped     Status = "stopped"
	StatusPaused      Status = "paused"
	StatusError       Status = "error"
	StatusUnavailable Status = "unavailable"
	StatusDestroyed   Status = "destroyed"
)

type Config struct {
	WorkDir          string
	DockerImage      string
	DockerEntrypoint string
	DockerNetwork    string
	CDPPortMin       int
	CDPPortMax       int
	DesktopPortMin   int
	DesktopPortMax   int
	Resolution       string
	ShmSize          string
	MemoryLimit      string
	ChromeCommand    string
	DeviceID         string
	DeviceName       string
}

type CreateRequest struct {
	Kind            Kind   `json:"kind"`
	Alias           string `json:"alias"`
	CDPURL          string `json:"cdp_url"`
	DesktopURL      string `json:"desktop_url"`
	SessionURL      string `json:"session_url"`
	PreviewURL      string `json:"preview_url"`
	Image           string `json:"image"`
	CDPHostPort     int    `json:"cdp_host_port"`
	DesktopHostPort int    `json:"desktop_host_port"`
}

type RuntimeEndpoint struct {
	ContainerID      string `json:"container_id,omitempty"`
	ContainerName    string `json:"container_name,omitempty"`
	CDPPort          int    `json:"cdp_port,omitempty"`
	CDPHostPort      int    `json:"cdp_host_port,omitempty"`
	CDPURL           string `json:"cdp_url,omitempty"`
	DesktopPort      int    `json:"desktop_port,omitempty"`
	DesktopHostPort  int    `json:"desktop_host_port,omitempty"`
	DesktopURL       string `json:"desktop_url,omitempty"`
	SessionPort      int    `json:"session_port,omitempty"`
	SessionHostPort  int    `json:"session_host_port,omitempty"`
	SessionURL       string `json:"session_url,omitempty"`
	PageID           string `json:"page_id,omitempty"`
	PageWebSocketURL string `json:"page_websocket_url,omitempty"`
	BrowserServerURL string `json:"browser_server_url,omitempty"`
}

type DeviceInfo struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Record struct {
	ID          string           `json:"id"`
	Alias       string           `json:"alias"`
	Kind        Kind             `json:"kind"`
	Status      Status           `json:"status"`
	Image       string           `json:"image,omitempty"`
	Device      *DeviceInfo      `json:"device,omitempty"`
	DeviceBound bool             `json:"device_bound,omitempty"`
	LocalDevice bool             `json:"local_device,omitempty"`
	Endpoint    *RuntimeEndpoint `json:"endpoint,omitempty"`
	Error       string           `json:"error,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type Lease struct {
	ID     string `json:"id"`
	Alias  string `json:"alias,omitempty"`
	Kind   Kind   `json:"kind"`
	CDPURL string `json:"cdp_url"`
}

type ReleaseOptions struct {
	Invalid bool
	Error   string
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

type Manager struct {
	cfg     Config
	runner  CommandRunner
	mu      sync.Mutex
	records map[string]*Record
}

func New(cfg Config, runner CommandRunner) (*Manager, error) {
	if runner == nil {
		runner = ExecRunner{}
	}
	if cfg.CDPPortMin == 0 {
		cfg.CDPPortMin = 39222
	}
	if cfg.CDPPortMax == 0 {
		cfg.CDPPortMax = 39322
	}
	if cfg.DesktopPortMin == 0 {
		cfg.DesktopPortMin = 39000
	}
	if cfg.DesktopPortMax == 0 {
		cfg.DesktopPortMax = 39122
	}
	if cfg.DockerImage == "" {
		cfg.DockerImage = "lscr.io/linuxserver/chromium:latest"
	}
	if cfg.ShmSize == "" {
		cfg.ShmSize = "1g"
	}
	if cfg.Resolution == "" {
		cfg.Resolution = "1920x1080x24"
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = "."
	}
	device := normalizeDevice(cfg.DeviceID, cfg.DeviceName)
	cfg.DeviceID = device.ID
	cfg.DeviceName = device.Name
	m := &Manager{cfg: cfg, runner: runner, records: map[string]*Record{}}
	_ = os.MkdirAll(m.registryDir(), 0755)
	_ = m.load()
	return m, nil
}

func (m *Manager) List() []*Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.refreshLocked()
	out := make([]*Record, 0, len(m.records))
	for _, rec := range m.records {
		out = append(out, m.decorateRecord(cloneRecord(rec)))
	}
	return out
}

func (m *Manager) Get(id string) (*Record, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.refreshLocked()
	rec, ok := m.records[strings.TrimSpace(id)]
	return m.decorateRecord(cloneRecord(rec)), ok
}

func (m *Manager) Create(ctx context.Context, req CreateRequest) (*Record, error) {
	if req.Kind == "" || req.Kind == "browser" || req.Kind == "headless" {
		req.Kind = KindDocker
	}
	switch req.Kind {
	case KindLocal, KindExisting:
		return m.RegisterLocal(req)
	case KindDocker:
		return m.CreateDocker(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported browser kind %q", req.Kind)
	}
}

func (m *Manager) RegisterLocal(req CreateRequest) (*Record, error) {
	cdpURL := strings.TrimSpace(req.CDPURL)
	if cdpURL == "" {
		return nil, fmt.Errorf("cdp_url is required for local browser")
	}
	u, err := url.Parse(cdpURL)
	if err != nil {
		return nil, fmt.Errorf("parse cdp_url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("local cdp_url must be http or https")
	}
	sessionURL := firstNonEmpty(req.SessionURL, req.DesktopURL, req.PreviewURL)
	var session *url.URL
	if sessionURL != "" {
		sessionURL = strings.TrimRight(sessionURL, "/")
		session, err = url.Parse(sessionURL)
		if err != nil {
			return nil, fmt.Errorf("parse session url: %w", err)
		}
		if session.Scheme != "http" && session.Scheme != "https" {
			return nil, fmt.Errorf("session url must be http or https")
		}
	}
	desktopHostPort := req.DesktopHostPort
	if desktopHostPort <= 0 {
		desktopHostPort = parseURLPort(session)
	}
	if sessionURL == "" && desktopHostPort > 0 {
		sessionURL = fmt.Sprintf("http://127.0.0.1:%d", desktopHostPort)
	}
	kind := req.Kind
	if kind == "" {
		kind = KindLocal
	}
	id := newID()
	now := time.Now()
	rec := &Record{
		ID:        id,
		Alias:     firstNonEmpty(req.Alias, "local-"+id),
		Kind:      kind,
		Status:    StatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
		Endpoint: &RuntimeEndpoint{
			CDPURL:           strings.TrimRight(cdpURL, "/"),
			CDPHostPort:      parseURLPort(u),
			DesktopHostPort:  desktopHostPort,
			DesktopURL:       sessionURL,
			SessionHostPort:  desktopHostPort,
			SessionURL:       sessionURL,
			BrowserServerURL: strings.TrimRight(cdpURL, "/"),
		},
	}
	if recordHasLocalEndpoint(rec) {
		m.bindToCurrentDevice(rec)
	}
	return m.put(rec)
}

func (m *Manager) CreateDocker(ctx context.Context, req CreateRequest) (*Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id := newID()
	name := "wx-browser-" + id
	image := firstNonEmpty(req.Image, m.cfg.DockerImage)
	hostPort := req.CDPHostPort
	if hostPort <= 0 {
		hostPort = m.pickPort(id, m.cfg.CDPPortMin, m.cfg.CDPPortMax)
	}
	desktopHostPort := req.DesktopHostPort
	if desktopHostPort <= 0 {
		desktopHostPort = m.pickPort(id+"-desktop", m.cfg.DesktopPortMin, m.cfg.DesktopPortMax)
	}
	args := []string{
		"run", "-d",
		"--name", name,
		"-p", fmt.Sprintf("%d:9222", hostPort),
		"-p", fmt.Sprintf("%d:3000", desktopHostPort),
		"--label", "wx_channels_download.browser=1",
		"--label", "wx_channels_download.browser_id=" + id,
	}
	if m.cfg.ShmSize != "" {
		args = append(args, "--shm-size", m.cfg.ShmSize)
	}
	if m.cfg.MemoryLimit != "" {
		args = append(args, "--memory", m.cfg.MemoryLimit)
	}
	if m.cfg.DockerNetwork != "" {
		args = append(args, "--network", m.cfg.DockerNetwork)
	}
	args = append(args,
		"-e", "PUID=1000",
		"-e", "PGID=1000",
		"-e", "TZ=Asia/Shanghai",
		"-e", "RESOLUTION="+m.cfg.Resolution,
		"-e", "CHROME_CLI="+m.chromeCLI(),
	)
	if m.cfg.DockerEntrypoint != "" {
		args = append(args, "--entrypoint", m.cfg.DockerEntrypoint)
	}
	args = append(args, image)
	if cmd := m.chromeCommand(); cmd != "" {
		args = append(args, "-lc", cmd)
	}
	now := time.Now()
	rec := &Record{
		ID:        id,
		Alias:     firstNonEmpty(req.Alias, id),
		Kind:      KindDocker,
		Status:    StatusCreating,
		Image:     image,
		CreatedAt: now,
		UpdatedAt: now,
		Endpoint: &RuntimeEndpoint{
			ContainerName:    name,
			CDPPort:          9222,
			CDPHostPort:      hostPort,
			CDPURL:           fmt.Sprintf("http://127.0.0.1:%d", hostPort),
			DesktopPort:      3000,
			DesktopHostPort:  desktopHostPort,
			DesktopURL:       fmt.Sprintf("http://127.0.0.1:%d", desktopHostPort),
			SessionPort:      3000,
			SessionHostPort:  desktopHostPort,
			SessionURL:       fmt.Sprintf("http://127.0.0.1:%d", desktopHostPort),
			BrowserServerURL: fmt.Sprintf("http://127.0.0.1:%d", hostPort),
		},
	}
	m.bindToCurrentDevice(rec)
	if _, err := m.put(rec); err != nil {
		return nil, err
	}
	go m.createDockerContainer(id, args)
	return m.decorateRecord(cloneRecord(rec)), nil
}

func (m *Manager) createDockerContainer(id string, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	containerID, err := m.runner.Run(ctx, "docker", args...)
	if err != nil {
		_ = m.updateRecord(id, func(rec *Record) {
			rec.Status = StatusError
			rec.Error = err.Error()
			rec.UpdatedAt = time.Now()
		})
		return
	}

	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		_ = m.updateRecord(id, func(rec *Record) {
			rec.Status = StatusError
			rec.Error = "docker run returned empty container id"
			rec.UpdatedAt = time.Now()
		})
		return
	}
	if err := m.updateRecord(id, func(rec *Record) {
		if rec.Endpoint == nil {
			rec.Endpoint = &RuntimeEndpoint{}
		}
		rec.Endpoint.ContainerID = containerID
		rec.Error = ""
		rec.UpdatedAt = time.Now()
	}); err != nil && containerID != "" {
		_, _ = m.runner.Run(context.Background(), "docker", "rm", "-f", containerID)
		return
	}
	rec, ok := m.Get(id)
	if !ok {
		if containerID != "" {
			_, _ = m.runner.Run(context.Background(), "docker", "rm", "-f", containerID)
		}
		return
	}
	if err := m.ensureDockerCDPAvailable(ctx, rec); err != nil {
		_ = m.updateRecord(id, func(rec *Record) {
			rec.Status = StatusError
			rec.Error = err.Error()
			rec.UpdatedAt = time.Now()
		})
		return
	}
	_ = m.updateStatus(id, StatusRunning, "")
}

func (m *Manager) Stop(ctx context.Context, id string) error {
	rec, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if err := m.requireCurrentDevice(rec); err != nil {
		return err
	}
	if rec.Kind == KindDocker && rec.Endpoint != nil && rec.Endpoint.ContainerID != "" {
		if _, err := m.runner.Run(ctx, "docker", "stop", rec.Endpoint.ContainerID); err != nil {
			return err
		}
	}
	return m.updateStatus(id, StatusPaused, "")
}

func (m *Manager) Start(ctx context.Context, id string) error {
	rec, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if err := m.requireCurrentDevice(rec); err != nil {
		return err
	}
	if rec.Kind == KindDocker && rec.Endpoint != nil && rec.Endpoint.ContainerID != "" {
		if _, err := m.runner.Run(ctx, "docker", "start", rec.Endpoint.ContainerID); err != nil {
			return err
		}
		if err := m.ensureDockerCDPAvailable(ctx, rec); err != nil {
			_ = m.updateStatus(id, StatusError, err.Error())
			return err
		}
	}
	return m.updateStatus(id, StatusRunning, "")
}

func (m *Manager) Restart(ctx context.Context, id string) error {
	rec, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if err := m.requireCurrentDevice(rec); err != nil {
		return err
	}
	if rec.Kind == KindDocker && rec.Endpoint != nil && rec.Endpoint.ContainerID != "" {
		if _, err := m.runner.Run(ctx, "docker", "restart", rec.Endpoint.ContainerID); err != nil {
			return err
		}
		if err := m.ensureDockerCDPAvailable(ctx, rec); err != nil {
			_ = m.updateStatus(id, StatusError, err.Error())
			return err
		}
		return m.updateStatus(id, StatusRunning, "")
	}
	return nil
}

func (m *Manager) Refresh(ctx context.Context, id string) (*Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rec, ok := m.rawRecord(id)
	if !ok {
		return nil, fmt.Errorf("browser not found: %s", id)
	}
	unknownDevice := m.recordNeedsDevice(rec) && !recordHasDevice(rec)
	if !unknownDevice && !m.recordOnCurrentDevice(rec) {
		return m.decorateRecord(rec), nil
	}
	nextStatus, nextError := m.probeStatus(ctx, rec)
	if unknownDevice && nextError != "" {
		return m.decorateRecord(rec), nil
	}
	if err := m.updateRecord(id, func(rec *Record) {
		if unknownDevice {
			m.bindToCurrentDevice(rec)
		}
		rec.Status = nextStatus
		rec.Error = nextError
		rec.UpdatedAt = time.Now()
	}); err != nil {
		return nil, err
	}
	rec, ok = m.Get(id)
	if !ok {
		return nil, fmt.Errorf("browser not found: %s", id)
	}
	return rec, nil
}

func (m *Manager) probeStatus(ctx context.Context, rec *Record) (Status, string) {
	nextStatus := StatusRunning
	nextError := ""
	if rec.Status == StatusBusy {
		nextStatus = StatusBusy
	}
	switch rec.Kind {
	case KindDocker:
		ref := ""
		if rec.Endpoint != nil {
			ref = firstNonEmpty(rec.Endpoint.ContainerID, rec.Endpoint.ContainerName)
		}
		if ref == "" {
			nextStatus = StatusInvalid
			nextError = "docker container reference is empty"
			break
		}
		out, err := m.runner.Run(ctx, "docker", "inspect", "--format", "{{.State.Running}}", ref)
		if err != nil {
			nextStatus = StatusInvalid
			nextError = err.Error()
			break
		}
		if strings.TrimSpace(out) != "true" {
			nextStatus = StatusPaused
			break
		}
		if err := m.ensureDockerCDPAvailable(ctx, rec); err != nil {
			nextStatus = StatusInvalid
			nextError = err.Error()
		}
	case KindLocal, KindExisting:
		if rec.Endpoint == nil || strings.TrimSpace(rec.Endpoint.CDPURL) == "" {
			nextStatus = StatusInvalid
			nextError = "cdp_url is empty"
			break
		}
		if err := waitCDPEndpoint(ctx, strings.TrimRight(rec.Endpoint.CDPURL, "/"), 3*time.Second); err != nil {
			nextStatus = StatusInvalid
			nextError = err.Error()
		}
	default:
		nextStatus = StatusInvalid
		nextError = fmt.Sprintf("unsupported browser kind %q", rec.Kind)
	}
	return nextStatus, nextError
}

func (m *Manager) Destroy(ctx context.Context, id string) error {
	rec, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if rec.Kind == KindDocker && rec.Endpoint != nil && m.recordOnCurrentDevice(rec) {
		containerRef := strings.TrimSpace(rec.Endpoint.ContainerID)
		if containerRef == "" {
			containerRef = strings.TrimSpace(rec.Endpoint.ContainerName)
		}
		if containerRef != "" {
			_, _ = m.runner.Run(ctx, "docker", "rm", "-f", containerRef)
		}
	}
	m.mu.Lock()
	delete(m.records, id)
	m.mu.Unlock()
	_ = os.Remove(m.recordPath(id))
	return nil
}

func (m *Manager) UpdateAlias(id string, alias string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[strings.TrimSpace(id)]
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	rec.Alias = strings.TrimSpace(alias)
	rec.UpdatedAt = time.Now()
	return m.persist(rec)
}

func (m *Manager) SetActivePage(id, pageID, wsURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[strings.TrimSpace(id)]
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if rec.Endpoint == nil {
		rec.Endpoint = &RuntimeEndpoint{}
	}
	if err := m.requireCurrentDevice(rec); err != nil {
		return err
	}
	rec.Endpoint.PageID = pageID
	rec.Endpoint.PageWebSocketURL = wsURL
	rec.UpdatedAt = time.Now()
	return m.persist(rec)
}

func (m *Manager) CDPURL(id string) (string, error) {
	rec, ok := m.Get(id)
	if !ok {
		return "", fmt.Errorf("browser not found: %s", id)
	}
	if rec.Endpoint == nil || strings.TrimSpace(rec.Endpoint.CDPURL) == "" {
		return "", fmt.Errorf("browser %s has no cdp endpoint", id)
	}
	if err := m.requireCurrentDevice(rec); err != nil {
		return "", err
	}
	if !isUsableStatus(rec.Status) {
		return "", fmt.Errorf("browser %s is not running: %s", id, rec.Status)
	}
	return strings.TrimRight(rec.Endpoint.CDPURL, "/"), nil
}

func (m *Manager) Acquire(ctx context.Context) (*Lease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.refreshLocked()
	for _, rec := range m.records {
		if !m.recordOnCurrentDevice(rec) {
			continue
		}
		if !isIdleStatus(rec.Status) {
			continue
		}
		if rec.Endpoint == nil || strings.TrimSpace(rec.Endpoint.CDPURL) == "" {
			continue
		}
		rec.Status = StatusBusy
		rec.Error = ""
		rec.UpdatedAt = time.Now()
		if err := m.persist(rec); err != nil {
			return nil, err
		}
		return &Lease{
			ID:     rec.ID,
			Alias:  rec.Alias,
			Kind:   rec.Kind,
			CDPURL: strings.TrimRight(rec.Endpoint.CDPURL, "/"),
		}, nil
	}
	return nil, fmt.Errorf("no idle browser sandbox available; add a local or docker browser sandbox first")
}

func (m *Manager) Release(id string, opts ReleaseOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[strings.TrimSpace(id)]
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if err := m.requireCurrentDevice(rec); err != nil {
		return err
	}
	if opts.Invalid {
		rec.Status = StatusInvalid
		rec.Error = strings.TrimSpace(opts.Error)
	} else {
		rec.Status = StatusRunning
		rec.Error = ""
	}
	rec.UpdatedAt = time.Now()
	return m.persist(rec)
}

func (m *Manager) put(rec *Record) (*Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[rec.ID] = rec
	return m.decorateRecord(cloneRecord(rec)), m.persist(rec)
}

func (m *Manager) updateStatus(id string, status Status, msg string) error {
	return m.updateRecord(id, func(rec *Record) {
		rec.Status = status
		rec.Error = msg
		rec.UpdatedAt = time.Now()
	})
}

func (m *Manager) updateRecord(id string, mutate func(*Record)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[strings.TrimSpace(id)]
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	mutate(rec)
	return m.persist(rec)
}

func (m *Manager) rawRecord(id string) (*Record, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.refreshLocked()
	rec, ok := m.records[strings.TrimSpace(id)]
	return cloneRecord(rec), ok
}

func (m *Manager) decorateRecord(rec *Record) *Record {
	if rec == nil {
		return nil
	}
	if !m.recordNeedsDevice(rec) {
		return rec
	}
	rec.DeviceBound = true
	if m.recordOnCurrentDevice(rec) {
		rec.LocalDevice = true
		return rec
	}
	rec.LocalDevice = false
	rec.Status = StatusUnavailable
	rec.Error = m.deviceUnavailableMessage(rec)
	return rec
}

func (m *Manager) requireCurrentDevice(rec *Record) error {
	if m.recordOnCurrentDevice(rec) {
		return nil
	}
	return fmt.Errorf("%s", m.deviceUnavailableMessage(rec))
}

func (m *Manager) bindToCurrentDevice(rec *Record) {
	if rec == nil {
		return
	}
	device := m.currentDevice()
	rec.Device = &DeviceInfo{ID: device.ID, Name: device.Name}
	rec.DeviceBound = true
	rec.LocalDevice = false
}

func (m *Manager) currentDevice() DeviceInfo {
	return DeviceInfo{ID: m.cfg.DeviceID, Name: m.cfg.DeviceName}
}

func (m *Manager) recordNeedsDevice(rec *Record) bool {
	if rec == nil {
		return false
	}
	return rec.DeviceBound || rec.Device != nil || rec.Kind == KindDocker || recordHasLocalEndpoint(rec)
}

func (m *Manager) recordOnCurrentDevice(rec *Record) bool {
	if rec == nil {
		return false
	}
	if !m.recordNeedsDevice(rec) {
		return true
	}
	return recordHasDevice(rec) && strings.TrimSpace(rec.Device.ID) == strings.TrimSpace(m.cfg.DeviceID)
}

func (m *Manager) deviceUnavailableMessage(rec *Record) string {
	current := firstNonEmpty(m.cfg.DeviceName, m.cfg.DeviceID, "current device")
	if !recordHasDevice(rec) {
		return fmt.Sprintf("sandbox is bound to a local endpoint but has no device; current device is %s", current)
	}
	owner := firstNonEmpty(rec.Device.Name, rec.Device.ID, "unknown device")
	return fmt.Sprintf("sandbox belongs to device %s; current device is %s", owner, current)
}

func recordHasDevice(rec *Record) bool {
	return rec != nil && rec.Device != nil && strings.TrimSpace(rec.Device.ID) != ""
}

func recordHasLocalEndpoint(rec *Record) bool {
	if rec == nil || rec.Endpoint == nil {
		return false
	}
	for _, rawURL := range []string{
		rec.Endpoint.CDPURL,
		rec.Endpoint.DesktopURL,
		rec.Endpoint.SessionURL,
		rec.Endpoint.BrowserServerURL,
	} {
		if isLocalEndpointURL(rawURL) {
			return true
		}
	}
	return false
}

func isLocalEndpointURL(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return isLocalEndpointHost(u.Hostname())
}

func isLocalEndpointHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsUnspecified()
}

func (m *Manager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshLocked()
}

func (m *Manager) refreshLocked() error {
	entries, err := os.ReadDir(m.registryDir())
	if err != nil {
		return err
	}
	next := make(map[string]*Record, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(m.registryDir(), entry.Name()))
		if err != nil {
			continue
		}
		var rec Record
		if err := json.Unmarshal(body, &rec); err == nil && rec.ID != "" {
			rec.LocalDevice = false
			next[rec.ID] = &rec
		}
	}
	m.records = next
	return nil
}

func (m *Manager) persist(rec *Record) error {
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.recordPath(rec.ID), body, 0644)
}

func (m *Manager) registryDir() string {
	return filepath.Join(m.cfg.WorkDir, "browser-sandboxes")
}

func (m *Manager) recordPath(id string) string {
	return filepath.Join(m.registryDir(), id+".json")
}

func (m *Manager) pickPort(seed string, min int, max int) int {
	if max <= min {
		return min
	}
	for i := 0; i < max-min; i++ {
		port := min + ((hashString(seed) + i) % (max - min))
		if m.portReserved(port) {
			continue
		}
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err == nil {
			_ = ln.Close()
			return port
		}
	}
	return min
}

func (m *Manager) portReserved(port int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rec := range m.records {
		if rec.Endpoint == nil {
			continue
		}
		if rec.Endpoint.CDPHostPort == port || rec.Endpoint.DesktopHostPort == port || rec.Endpoint.SessionHostPort == port {
			return true
		}
	}
	return false
}

func (m *Manager) ensureDockerCDPAvailable(ctx context.Context, rec *Record) error {
	if rec == nil || rec.Kind != KindDocker || rec.Endpoint == nil || rec.Endpoint.CDPHostPort <= 0 {
		return nil
	}
	containerRef := firstNonEmpty(rec.Endpoint.ContainerID, rec.Endpoint.ContainerName)
	if containerRef == "" {
		return nil
	}
	cdpURL := fmt.Sprintf("http://127.0.0.1:%d", rec.Endpoint.CDPHostPort)
	if err := waitCDPEndpoint(ctx, cdpURL, 5*time.Second); err == nil {
		return nil
	}
	if _, err := m.runner.Run(ctx, "docker", "exec", containerRef, "sh", "-lc", cdpLoopbackProxyScript()); err != nil {
		if retryErr := waitCDPEndpoint(ctx, cdpURL, 2*time.Second); retryErr == nil {
			return nil
		}
		return fmt.Errorf("start cdp loopback proxy: %w", err)
	}
	if err := waitCDPEndpoint(ctx, cdpURL, 30*time.Second); err != nil {
		return fmt.Errorf("cdp endpoint %s unavailable after loopback proxy: %w", cdpURL, err)
	}
	return nil
}

func waitCDPEndpoint(ctx context.Context, cdpURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cdpURL, "/")+"/json/version", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 && json.Valid(body) {
				return nil
			} else {
				lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("timeout")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func cdpLoopbackProxyScript() string {
	return `set -eu
command -v python3 >/dev/null || { echo "python3 not found"; exit 1; }
ip="$(ip -4 addr show eth0 | awk '/inet / {sub(/\/.*/, "", $2); print $2; exit}')"
if [ -z "$ip" ]; then
  ip="$(hostname -i 2>/dev/null | awk '{print $1}')"
fi
if [ -z "$ip" ]; then
  echo "container ip not found"
  exit 1
fi
if ps -ef | grep -F "/tmp/wx-cdp-loopback-proxy.py $ip 9222" | grep -v grep >/dev/null 2>&1; then
  exit 0
fi
cat >/tmp/wx-cdp-loopback-proxy.py <<'PY'
import socket
import sys
import threading

listen_host = sys.argv[1]
listen_port = int(sys.argv[2])
target_host = sys.argv[3]
target_port = int(sys.argv[4])

def pipe(src, dst):
    try:
        while True:
            data = src.recv(65536)
            if not data:
                break
            dst.sendall(data)
    except Exception:
        pass
    finally:
        try:
            dst.shutdown(socket.SHUT_WR)
        except Exception:
            pass

def handle(client):
    try:
        upstream = socket.create_connection((target_host, target_port), timeout=10)
    except Exception:
        client.close()
        return
    threading.Thread(target=pipe, args=(client, upstream), daemon=True).start()
    threading.Thread(target=pipe, args=(upstream, client), daemon=True).start()

server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
server.bind((listen_host, listen_port))
server.listen(128)
while True:
    client, _ = server.accept()
    threading.Thread(target=handle, args=(client,), daemon=True).start()
PY
nohup python3 /tmp/wx-cdp-loopback-proxy.py "$ip" 9222 127.0.0.1 9222 >/tmp/wx-cdp-loopback-proxy.log 2>&1 &
`
}

func (m *Manager) chromeCommand() string {
	if strings.TrimSpace(m.cfg.ChromeCommand) != "" {
		return m.cfg.ChromeCommand
	}
	if strings.TrimSpace(m.cfg.DockerEntrypoint) == "" {
		return ""
	}
	return `CHROME="$(command -v google-chrome || command -v chromium-browser || command -v chromium || command -v chrome)" && exec "$CHROME" --no-sandbox --disable-dev-shm-usage --remote-debugging-address=0.0.0.0 --remote-debugging-port=9222 --user-data-dir=/tmp/wx-browser-profile about:blank`
}

func (m *Manager) chromeCLI() string {
	return "--no-sandbox --disable-dev-shm-usage --remote-debugging-address=0.0.0.0 --remote-debugging-port=9222 --user-data-dir=/config/wx-browser-profile"
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashString(s string) int {
	h := 0
	for _, r := range s {
		h = h*31 + int(r)
	}
	if h < 0 {
		return -h
	}
	return h
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func normalizeDevice(id string, name string) DeviceInfo {
	defaultDevice := defaultDeviceInfo()
	id = firstNonEmpty(id, defaultDevice.ID)
	name = firstNonEmpty(name, defaultDevice.Name, id, "unknown-device")
	return DeviceInfo{ID: id, Name: name}
}

func defaultDeviceInfo() DeviceInfo {
	hostname, _ := os.Hostname()
	username := firstNonEmpty(os.Getenv("USER"), os.Getenv("USERNAME"))
	source := strings.Join([]string{runtime.GOOS, runtime.GOARCH, hostname, username, firstHardwareAddr()}, "|")
	if strings.Trim(source, "|") == "" {
		source = "unknown-device"
	}
	sum := sha256.Sum256([]byte(source))
	return DeviceInfo{
		ID:   hex.EncodeToString(sum[:8]),
		Name: firstNonEmpty(hostname, username, runtime.GOOS, "unknown-device"),
	}
}

func firstHardwareAddr() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
			continue
		}
		return iface.HardwareAddr.String()
	}
	return ""
}

func isIdleStatus(status Status) bool {
	return status == StatusIdle || status == StatusRunning
}

func isUsableStatus(status Status) bool {
	return status == StatusIdle || status == StatusRunning || status == StatusBusy
}

func parseURLPort(u *url.URL) int {
	if u == nil || u.Port() == "" {
		return 0
	}
	port, _ := strconv.Atoi(u.Port())
	return port
}

func cloneRecord(rec *Record) *Record {
	if rec == nil {
		return nil
	}
	cp := *rec
	if rec.Endpoint != nil {
		ep := *rec.Endpoint
		cp.Endpoint = &ep
	}
	return &cp
}
