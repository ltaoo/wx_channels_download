package browsermgr

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Kind string

const (
	KindLocal  Kind = "local"
	KindDocker Kind = "docker"
)

type Status string

const (
	StatusIdle      Status = "idle"
	StatusBusy      Status = "busy"
	StatusInvalid   Status = "invalid"
	StatusRunning   Status = "running"
	StatusStopped   Status = "stopped"
	StatusError     Status = "error"
	StatusDestroyed Status = "destroyed"
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
}

type CreateRequest struct {
	Kind            Kind   `json:"kind"`
	Alias           string `json:"alias"`
	CDPURL          string `json:"cdp_url"`
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
	PageID           string `json:"page_id,omitempty"`
	PageWebSocketURL string `json:"page_websocket_url,omitempty"`
	BrowserServerURL string `json:"browser_server_url,omitempty"`
}

type Record struct {
	ID        string           `json:"id"`
	Alias     string           `json:"alias"`
	Kind      Kind             `json:"kind"`
	Status    Status           `json:"status"`
	Image     string           `json:"image,omitempty"`
	Endpoint  *RuntimeEndpoint `json:"endpoint,omitempty"`
	Error     string           `json:"error,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
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
		out = append(out, cloneRecord(rec))
	}
	return out
}

func (m *Manager) Get(id string) (*Record, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.refreshLocked()
	rec, ok := m.records[strings.TrimSpace(id)]
	return cloneRecord(rec), ok
}

func (m *Manager) Create(ctx context.Context, req CreateRequest) (*Record, error) {
	if req.Kind == "" || req.Kind == "browser" || req.Kind == "headless" {
		req.Kind = KindDocker
	}
	switch req.Kind {
	case KindLocal:
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
	id := newID()
	now := time.Now()
	rec := &Record{
		ID:        id,
		Alias:     firstNonEmpty(req.Alias, "local-"+id),
		Kind:      KindLocal,
		Status:    StatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
		Endpoint: &RuntimeEndpoint{
			CDPURL:           strings.TrimRight(cdpURL, "/"),
			CDPHostPort:      parseURLPort(u),
			BrowserServerURL: strings.TrimRight(cdpURL, "/"),
		},
	}
	return m.put(rec)
}

func (m *Manager) CreateDocker(ctx context.Context, req CreateRequest) (*Record, error) {
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
	containerID, err := m.runner.Run(ctx, "docker", args...)
	now := time.Now()
	rec := &Record{
		ID:        id,
		Alias:     firstNonEmpty(req.Alias, id),
		Kind:      KindDocker,
		Status:    StatusIdle,
		Image:     image,
		CreatedAt: now,
		UpdatedAt: now,
		Endpoint: &RuntimeEndpoint{
			ContainerID:      strings.TrimSpace(containerID),
			ContainerName:    name,
			CDPPort:          9222,
			CDPHostPort:      hostPort,
			CDPURL:           fmt.Sprintf("http://127.0.0.1:%d", hostPort),
			DesktopPort:      3000,
			DesktopHostPort:  desktopHostPort,
			DesktopURL:       fmt.Sprintf("http://127.0.0.1:%d", desktopHostPort),
			BrowserServerURL: fmt.Sprintf("http://127.0.0.1:%d", hostPort),
		},
	}
	if err != nil {
		rec.Status = StatusInvalid
		rec.Error = err.Error()
		_, _ = m.put(rec)
		return rec, err
	}
	return m.put(rec)
}

func (m *Manager) Stop(ctx context.Context, id string) error {
	rec, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if rec.Kind == KindDocker && rec.Endpoint != nil && rec.Endpoint.ContainerID != "" {
		if _, err := m.runner.Run(ctx, "docker", "stop", rec.Endpoint.ContainerID); err != nil {
			return err
		}
	}
	return m.updateStatus(id, StatusStopped, "")
}

func (m *Manager) Start(ctx context.Context, id string) error {
	rec, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if rec.Kind == KindDocker && rec.Endpoint != nil && rec.Endpoint.ContainerID != "" {
		if _, err := m.runner.Run(ctx, "docker", "start", rec.Endpoint.ContainerID); err != nil {
			return err
		}
	}
	return m.updateStatus(id, StatusIdle, "")
}

func (m *Manager) Restart(ctx context.Context, id string) error {
	rec, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if rec.Kind == KindDocker && rec.Endpoint != nil && rec.Endpoint.ContainerID != "" {
		if _, err := m.runner.Run(ctx, "docker", "restart", rec.Endpoint.ContainerID); err != nil {
			return err
		}
		return m.updateStatus(id, StatusIdle, "")
	}
	return nil
}

func (m *Manager) Destroy(ctx context.Context, id string) error {
	rec, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	if rec.Kind == KindDocker && rec.Endpoint != nil && rec.Endpoint.ContainerID != "" {
		_, _ = m.runner.Run(ctx, "docker", "rm", "-f", rec.Endpoint.ContainerID)
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
	if opts.Invalid {
		rec.Status = StatusInvalid
		rec.Error = strings.TrimSpace(opts.Error)
	} else {
		rec.Status = StatusIdle
		rec.Error = ""
	}
	rec.UpdatedAt = time.Now()
	return m.persist(rec)
}

func (m *Manager) put(rec *Record) (*Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[rec.ID] = rec
	return cloneRecord(rec), m.persist(rec)
}

func (m *Manager) updateStatus(id string, status Status, msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[strings.TrimSpace(id)]
	if !ok {
		return fmt.Errorf("browser not found: %s", id)
	}
	rec.Status = status
	rec.Error = msg
	rec.UpdatedAt = time.Now()
	return m.persist(rec)
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
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err == nil {
			_ = ln.Close()
			return port
		}
	}
	return min
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

func isIdleStatus(status Status) bool {
	return status == StatusIdle || status == StatusRunning
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
