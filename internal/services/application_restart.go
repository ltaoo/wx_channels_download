package services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

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
