package services

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	update_util "github.com/ltaoo/velo/updater/util"

	"wx_channel/pkg/hermes"
)

const default_update_restart_delay = 350 * time.Millisecond

// UpdateStatus is the frontend-facing snapshot of the application update flow.
type UpdateStatus struct {
	Status         string  `json:"status"`
	Available      bool    `json:"available"`
	CurrentVersion string  `json:"current_version"`
	LatestVersion  string  `json:"latest_version,omitempty"`
	Name           string  `json:"name,omitempty"`
	PublishedAt    string  `json:"published_at,omitempty"`
	Body           string  `json:"body,omitempty"`
	AssetName      string  `json:"asset_name,omitempty"`
	Downloaded     int64   `json:"downloaded"`
	TotalSize      int64   `json:"total_size"`
	Speed          int64   `json:"speed"`
	Percent        float64 `json:"percent"`
	Error          string  `json:"error,omitempty"`
}

// UpdateRelease describes a release returned by an application-provided source.
type UpdateRelease struct {
	Version     string
	Name        string
	PublishedAt time.Time
	Body        string
	Assets      []UpdateAsset
}

// UpdateAsset describes a downloadable release artifact.
type UpdateAsset struct {
	Name        string
	DownloadURL string
	Size        int64
}

// UpdateServiceOptions supplies application-specific update capabilities.
type UpdateServiceOptions struct {
	CurrentVersion  string
	Repository      string
	OperatingSystem string
	Architecture    string
	RestartDelay    time.Duration
	FetchReleases   func(context.Context, string) ([]UpdateRelease, error)
	DownloadUpdate  func(string, string, string, func(*hermes.TaskProgress)) error
	Executable      func() (string, error)
	RequestRestart  func() error
}

// UpdateService coordinates update checks, downloads, progress, and restarts.
type UpdateService struct {
	mu                 sync.RWMutex
	current_version    string
	repository         string
	operating_system   string
	architecture       string
	release            *UpdateRelease
	asset              UpdateAsset
	status             UpdateStatus
	restart_scheduled  bool
	fetch_releases_fn  func(context.Context, string) ([]UpdateRelease, error)
	download_update_fn func(string, string, string, func(*hermes.TaskProgress)) error
	executable_fn      func() (string, error)
	request_restart_fn func() error
	restart_delay      time.Duration
}

// NewUpdateService creates an update service from application-provided adapters.
func NewUpdateService(options UpdateServiceOptions) *UpdateService {
	operating_system := options.OperatingSystem
	if operating_system == "" {
		operating_system = runtime.GOOS
	}
	architecture := options.Architecture
	if architecture == "" {
		architecture = runtime.GOARCH
	}
	restart_delay := options.RestartDelay
	if restart_delay <= 0 {
		restart_delay = default_update_restart_delay
	}
	executable_fn := options.Executable
	if executable_fn == nil {
		executable_fn = os.Executable
	}

	return &UpdateService{
		current_version:    options.CurrentVersion,
		repository:         options.Repository,
		operating_system:   operating_system,
		architecture:       architecture,
		status:             UpdateStatus{Status: "idle", CurrentVersion: options.CurrentVersion},
		fetch_releases_fn:  options.FetchReleases,
		download_update_fn: options.DownloadUpdate,
		executable_fn:      executable_fn,
		request_restart_fn: options.RequestRestart,
		restart_delay:      restart_delay,
	}
}

// Check checks the configured release source for a compatible newer version.
func (s *UpdateService) Check(ctx context.Context) (UpdateStatus, error) {
	s.mu.Lock()
	if s.status.Status == "downloading" || s.status.Status == "ready" || s.status.Status == "restarting" {
		status := s.status
		s.mu.Unlock()
		return status, nil
	}
	s.status.Status = "checking"
	s.status.Error = ""
	s.mu.Unlock()

	if s.fetch_releases_fn == nil {
		return s.set_error(fmt.Errorf("更新检查服务未初始化"))
	}
	releases, err := s.fetch_releases_fn(ctx, s.repository)
	if err != nil {
		return s.set_error(fmt.Errorf("检查更新失败: %w", err))
	}
	if len(releases) == 0 {
		return s.set_error(fmt.Errorf("未找到发布版本"))
	}

	latest := releases[0]
	is_newer, err := update_util.CompareVersions(s.current_version, latest.Version)
	if err != nil {
		return s.set_error(fmt.Errorf("版本号格式不正确: %w", err))
	}

	status := UpdateStatus{
		Status:         "current",
		CurrentVersion: s.current_version,
		LatestVersion:  latest.Version,
		Name:           latest.Name,
		PublishedAt:    latest.PublishedAt.Format(time.RFC3339),
		Body:           latest.Body,
	}
	var asset UpdateAsset
	if is_newer {
		var ok bool
		asset, ok = find_update_asset(latest, s.operating_system, s.architecture)
		if !ok {
			return s.set_error(fmt.Errorf(
				"未找到适用于当前系统 (%s/%s) 的安装包",
				s.operating_system,
				s.architecture,
			))
		}
		status.Status = "available"
		status.Available = true
		status.AssetName = asset.Name
		status.TotalSize = asset.Size
	}

	s.mu.Lock()
	s.release = &latest
	s.asset = asset
	s.status = status
	s.mu.Unlock()
	return status, nil
}

// Start begins downloading and applying the selected update.
func (s *UpdateService) Start() (UpdateStatus, error) {
	s.mu.Lock()
	if s.status.Status == "downloading" || s.status.Status == "ready" || s.status.Status == "restarting" {
		status := s.status
		s.mu.Unlock()
		return status, nil
	}
	if s.release == nil || s.asset.DownloadURL == "" {
		status := s.status
		s.mu.Unlock()
		return status, fmt.Errorf("请先检查更新")
	}
	if s.download_update_fn == nil {
		status := s.status
		s.mu.Unlock()
		return status, fmt.Errorf("更新下载服务未初始化")
	}
	release := *s.release
	asset := s.asset
	s.status.Status = "downloading"
	s.status.Available = true
	s.status.Downloaded = 0
	s.status.TotalSize = asset.Size
	s.status.Speed = 0
	s.status.Percent = 0
	s.status.Error = ""
	status := s.status
	s.mu.Unlock()

	go s.run_download(release, asset)
	return status, nil
}

// Status returns the latest update state snapshot.
func (s *UpdateService) Status() UpdateStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Restart requests process replacement after the HTTP response can be returned.
func (s *UpdateService) Restart() (UpdateStatus, error) {
	s.mu.Lock()
	if s.status.Status == "restarting" && s.restart_scheduled {
		status := s.status
		s.mu.Unlock()
		return status, nil
	}
	if s.status.Status != "ready" {
		status := s.status
		s.mu.Unlock()
		return status, fmt.Errorf("更新尚未下载完成")
	}
	if s.request_restart_fn == nil {
		status := s.status
		s.mu.Unlock()
		return status, fmt.Errorf("应用重启服务未初始化")
	}
	s.status.Status = "restarting"
	s.status.Speed = 0
	s.restart_scheduled = true
	status := s.status
	s.mu.Unlock()

	time.AfterFunc(s.restart_delay, func() {
		if err := s.request_restart_fn(); err != nil {
			_, _ = s.set_error(fmt.Errorf("重启应用失败: %w", err))
		}
	})
	return status, nil
}

func (s *UpdateService) run_download(release UpdateRelease, asset UpdateAsset) {
	exe_path, err := s.executable_fn()
	if err == nil {
		err = s.download_update_fn(
			asset.DownloadURL,
			asset.Name,
			exe_path,
			s.update_progress,
		)
	}
	if err != nil {
		_, _ = s.set_error(fmt.Errorf("更新失败: %w", err))
		return
	}

	s.mu.Lock()
	s.status.Status = "ready"
	s.status.Available = true
	s.status.LatestVersion = release.Version
	s.status.Speed = 0
	s.status.Percent = 100
	if s.status.TotalSize > 0 {
		s.status.Downloaded = s.status.TotalSize
	}
	s.status.Error = ""
	s.mu.Unlock()
}

func (s *UpdateService) update_progress(progress *hermes.TaskProgress) {
	if progress == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.Status != "downloading" {
		return
	}
	s.status.Downloaded = progress.Downloaded
	s.status.Speed = progress.Speed
	if progress.TotalSize > 0 {
		s.status.TotalSize = progress.TotalSize
	}
	if s.status.TotalSize > 0 {
		percent := float64(s.status.Downloaded) * 100 / float64(s.status.TotalSize)
		s.status.Percent = math.Max(0, math.Min(100, percent))
	}
}

func (s *UpdateService) set_error(err error) (UpdateStatus, error) {
	s.mu.Lock()
	s.status.Status = "error"
	s.status.Speed = 0
	s.status.Error = err.Error()
	status := s.status
	s.mu.Unlock()
	return status, err
}

func find_update_asset(release UpdateRelease, operating_system string, architecture string) (UpdateAsset, bool) {
	target_architecture := architecture
	switch architecture {
	case "amd64":
		target_architecture = "x86_64"
	case "386":
		target_architecture = "x86"
	}

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, operating_system) && strings.Contains(name, target_architecture) {
			return asset, true
		}
	}
	return UpdateAsset{}, false
}
