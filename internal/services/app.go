package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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

// AppService bundles the application restart and update services so callers
// hold one handle for the whole application lifecycle.
type AppService struct {
	Update  *ApplicationUpdateService
	Restart *ApplicationRestartService
}

const default_application_restart_delay = 350 * time.Millisecond

// ApplicationRestartService schedules a graceful restart after the caller has
// had time to return its HTTP response.
type ApplicationRestartService struct {
	mu                 sync.Mutex
	request_restart_fn func() error
	restart_delay      time.Duration
	restart_scheduled  bool
	instance_id        string
	restart_error      string
}

// ApplicationRestartServiceOptions supplies the process-specific restart
// callback and the delay used before graceful shutdown begins.
type ApplicationRestartServiceOptions struct {
	RequestRestart func() error
	RestartDelay   time.Duration
}

// NewApplicationRestartService constructs an application-wide restart service.
func NewApplicationRestartService(options ApplicationRestartServiceOptions) *ApplicationRestartService {
	restart_delay := options.RestartDelay
	if restart_delay <= 0 {
		restart_delay = default_application_restart_delay
	}
	return &ApplicationRestartService{
		request_restart_fn: options.RequestRestart,
		restart_delay:      restart_delay,
		instance_id:        new_restart_instance_id(),
	}
}

type restart_confirmation_token struct {
	PreviousInstanceID     string `json:"previous_instance_id"`
	ExpectedConfigRevision string `json:"expected_config_revision"`
}

// RestartConfirmation describes whether a different process instance is
// running with the configuration revision saved before restart.
type RestartConfirmation struct {
	Status                string `json:"status"`
	RestartCompleted      bool   `json:"restart_completed"`
	ConfigApplied         bool   `json:"config_applied"`
	CurrentInstanceID     string `json:"current_instance_id"`
	CurrentConfigRevision string `json:"current_config_revision"`
	Message               string `json:"message"`
	Error                 string `json:"error,omitempty"`
}

// NewConfirmationToken creates an opaque token that a replacement process can
// use to verify both the process transition and the loaded configuration.
func (s *ApplicationRestartService) NewConfirmationToken(expected_config_revision string) (string, error) {
	if s == nil || s.instance_id == "" {
		return "", fmt.Errorf("应用重启服务未初始化")
	}
	if expected_config_revision == "" {
		return "", fmt.Errorf("配置摘要为空")
	}
	payload, err := json.Marshal(restart_confirmation_token{
		PreviousInstanceID:     s.instance_id,
		ExpectedConfigRevision: expected_config_revision,
	})
	if err != nil {
		return "", fmt.Errorf("编码重启确认令牌失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// CheckConfirmation verifies a token created by the process that scheduled the
// restart against this process instance and its currently loaded config.
func (s *ApplicationRestartService) CheckConfirmation(token string, current_config_revision string) (RestartConfirmation, error) {
	if s == nil || s.instance_id == "" {
		return RestartConfirmation{}, fmt.Errorf("应用重启服务未初始化")
	}
	if len(token) == 0 || len(token) > 4096 {
		return RestartConfirmation{}, fmt.Errorf("无效的重启确认令牌")
	}
	payload, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return RestartConfirmation{}, fmt.Errorf("无效的重启确认令牌")
	}
	var expected restart_confirmation_token
	if err := json.Unmarshal(payload, &expected); err != nil || expected.PreviousInstanceID == "" || expected.ExpectedConfigRevision == "" {
		return RestartConfirmation{}, fmt.Errorf("无效的重启确认令牌")
	}

	restart_completed := expected.PreviousInstanceID != s.instance_id
	revision_matches := expected.ExpectedConfigRevision == current_config_revision
	s.mu.Lock()
	restart_error := s.restart_error
	s.mu.Unlock()
	confirmation := RestartConfirmation{
		Status:                "pending",
		RestartCompleted:      restart_completed,
		ConfigApplied:         restart_completed && revision_matches,
		CurrentInstanceID:     s.instance_id,
		CurrentConfigRevision: current_config_revision,
		Message:               "应用仍在原进程中运行，重启尚未完成",
	}
	if !restart_completed && restart_error != "" {
		confirmation.Status = "failed"
		confirmation.Message = "应用重启请求失败"
		confirmation.Error = restart_error
	} else if restart_completed && revision_matches {
		confirmation.Status = "completed"
		confirmation.Message = "应用已完成重启，新配置已确认生效"
	} else if restart_completed {
		confirmation.Status = "config_mismatch"
		confirmation.Message = "应用已重启，但当前配置与保存后的配置不一致"
	}
	return confirmation, nil
}

func new_restart_instance_id() string {
	random_bytes := make([]byte, 18)
	if _, err := rand.Read(random_bytes); err == nil {
		return base64.RawURLEncoding.EncodeToString(random_bytes)
	}
	return fmt.Sprintf("instance-%d", time.Now().UnixNano())
}

// Schedule records one restart request. Repeated calls before the process exits
// are idempotent. on_error is invoked only when the delayed request fails.
func (s *ApplicationRestartService) Schedule(on_error func(error)) error {
	if s == nil || s.request_restart_fn == nil {
		return fmt.Errorf("应用重启服务未初始化")
	}

	s.mu.Lock()
	if s.restart_scheduled {
		s.mu.Unlock()
		return nil
	}
	s.restart_scheduled = true
	s.restart_error = ""
	restart_delay := s.restart_delay
	s.mu.Unlock()

	time.AfterFunc(restart_delay, func() {
		if err := s.request_restart_fn(); err != nil {
			s.mu.Lock()
			s.restart_scheduled = false
			s.restart_error = err.Error()
			s.mu.Unlock()
			if on_error != nil {
				on_error(err)
			}
		}
	})
	return nil
}

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

// ApplicationUpdateServiceOptions supplies application-specific update capabilities.
type ApplicationUpdateServiceOptions struct {
	CurrentVersion  string
	Repository      string
	OperatingSystem string
	Architecture    string
	RestartDelay    time.Duration
	FetchReleases   func(context.Context, string) ([]UpdateRelease, error)
	DownloadUpdate  func(string, string, string, func(*hermes.TaskProgress)) error
	Executable      func() (string, error)
	RequestRestart  func() error
	RestartService  *ApplicationRestartService
}

// ApplicationUpdateService coordinates application update checks, downloads, progress, and restarts.
type ApplicationUpdateService struct {
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
	restart_service    *ApplicationRestartService
	restart_delay      time.Duration
}

// NewApplicationUpdateService creates an application update service from application-provided adapters.
func NewApplicationUpdateService(options ApplicationUpdateServiceOptions) *ApplicationUpdateService {
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

	return &ApplicationUpdateService{
		current_version:    options.CurrentVersion,
		repository:         options.Repository,
		operating_system:   operating_system,
		architecture:       architecture,
		status:             UpdateStatus{Status: "idle", CurrentVersion: options.CurrentVersion},
		fetch_releases_fn:  options.FetchReleases,
		download_update_fn: options.DownloadUpdate,
		executable_fn:      executable_fn,
		request_restart_fn: options.RequestRestart,
		restart_service:    options.RestartService,
		restart_delay:      restart_delay,
	}
}

// Check checks the configured release source for a compatible newer version.
func (s *ApplicationUpdateService) Check(ctx context.Context) (UpdateStatus, error) {
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
func (s *ApplicationUpdateService) Start() (UpdateStatus, error) {
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
func (s *ApplicationUpdateService) Status() UpdateStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Restart requests process replacement after the HTTP response can be returned.
func (s *ApplicationUpdateService) Restart() (UpdateStatus, error) {
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
	if s.restart_service == nil && s.request_restart_fn == nil {
		status := s.status
		s.mu.Unlock()
		return status, fmt.Errorf("应用重启服务未初始化")
	}
	s.status.Status = "restarting"
	s.status.Speed = 0
	s.restart_scheduled = true
	status := s.status
	s.mu.Unlock()

	if s.restart_service != nil {
		err := s.restart_service.Schedule(func(err error) {
			_, _ = s.set_error(fmt.Errorf("重启应用失败: %w", err))
		})
		if err != nil {
			return s.set_error(fmt.Errorf("重启应用失败: %w", err))
		}
		return status, nil
	}

	time.AfterFunc(s.restart_delay, func() {
		if err := s.request_restart_fn(); err != nil {
			_, _ = s.set_error(fmt.Errorf("重启应用失败: %w", err))
		}
	})
	return status, nil
}

func (s *ApplicationUpdateService) run_download(release UpdateRelease, asset UpdateAsset) {
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

func (s *ApplicationUpdateService) update_progress(progress *hermes.TaskProgress) {
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

func (s *ApplicationUpdateService) set_error(err error) (UpdateStatus, error) {
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
