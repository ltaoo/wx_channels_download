package hub

import (
	"context"
	"encoding/json"
	"time"
)

const (
	MethodWXChannelsFetch = "wxchannels.fetch"
	MethodDownloadCreate  = "download.create"

	max_message_bytes = 2 * 1024 * 1024
)

// Config describes one operating-system device's Hub connection and callable methods.
type Config struct {
	Enabled     bool
	URL         string
	DeviceID    string
	DeviceName  string
	DeviceOS    string
	Token       string
	Methods     []string
	HTTPTimeout time.Duration
	LegacyHubID string
}

// Task is the durable representation returned and pushed by the Hub.
type Task struct {
	ID                string          `json:"id"`
	Method            string          `json:"method"`
	PublisherDeviceID string          `json:"publisher_device_id"`
	TargetDeviceID    string          `json:"target_device_id,omitempty"`
	IdempotencyKey    string          `json:"idempotency_key,omitempty"`
	Args              json.RawMessage `json:"args"`
	Status            string          `json:"status"`
	AssignedDeviceID  string          `json:"assigned_device_id,omitempty"`
	LeaseExpiresAt    *int64          `json:"lease_expires_at,omitempty"`
	AttemptCount      int             `json:"attempt_count"`
	Result            json.RawMessage `json:"result,omitempty"`
	Error             string          `json:"error,omitempty"`
	CreatedAt         int64           `json:"created_at"`
	UpdatedAt         int64           `json:"updated_at"`
	CompletedAt       *int64          `json:"completed_at,omitempty"`
}

// SubmitTaskRequest is one generic method call accepted by the Cloudflare Hub.
type SubmitTaskRequest struct {
	Method         string `json:"method"`
	TargetDeviceID string `json:"target_device_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	Args           any    `json:"args"`
}

// UnmarshalJSON accepts both the method/args protocol and its task-kind legacy
// aliases so the device can be upgraded before the Worker.
func (t *Task) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	copy_legacy_json_field(fields, "method", "kind")
	copy_legacy_json_field(fields, "args", "payload")
	copy_legacy_json_field(fields, "publisher_device_id", "publisher_id")
	copy_legacy_json_field(fields, "target_device_id", "target_client_id")
	copy_legacy_json_field(fields, "assigned_device_id", "assigned_client_id")
	normalized, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	type task_without_unmarshal Task
	return json.Unmarshal(normalized, (*task_without_unmarshal)(t))
}

func copy_legacy_json_field(fields map[string]json.RawMessage, current_name string, legacy_name string) {
	if _, exists := fields[current_name]; exists {
		return
	}
	if value, exists := fields[legacy_name]; exists {
		fields[current_name] = value
	}
}

// Status is the local connection status exposed by the API.
type Status struct {
	Enabled     bool     `json:"enabled"`
	Connected   bool     `json:"connected"`
	DeviceID    string   `json:"device_id,omitempty"`
	DeviceName  string   `json:"device_name,omitempty"`
	DeviceOS    string   `json:"device_os,omitempty"`
	URL         string   `json:"url,omitempty"`
	Methods     []string `json:"methods"`
	ConnectedAt int64    `json:"connected_at,omitempty"`
	LastError   string   `json:"last_error,omitempty"`
}

// Executor performs a task assigned to this client and returns JSON result data.
type Executor func(context.Context, Task) (json.RawMessage, error)

// TerminalHandler receives completed and failed tasks published by this client.
type TerminalHandler func(Task)
