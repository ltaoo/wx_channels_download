package hub

import (
	"context"
	"encoding/json"
	"time"
)

const (
	KindWXChannelsFetch = "wxchannels.fetch"
	KindDownloadCreate  = "download.create"

	CapabilityWXChannelsFetch = KindWXChannelsFetch
	CapabilityDownloadCreate  = KindDownloadCreate

	max_message_bytes = 2 * 1024 * 1024
)

// Config describes one local service's Hub identity and capabilities.
type Config struct {
	Enabled      bool
	URL          string
	HubID        string
	ClientID     string
	Token        string
	Capabilities []string
	HTTPTimeout  time.Duration
}

// Task is the durable representation returned and pushed by the Hub.
type Task struct {
	ID                 string          `json:"id"`
	Kind               string          `json:"kind"`
	PublisherID        string          `json:"publisher_id"`
	TargetClientID     string          `json:"target_client_id,omitempty"`
	RequiredCapability string          `json:"required_capability"`
	IdempotencyKey     string          `json:"idempotency_key,omitempty"`
	Payload            json.RawMessage `json:"payload"`
	Status             string          `json:"status"`
	AssignedClientID   string          `json:"assigned_client_id,omitempty"`
	LeaseExpiresAt     *int64          `json:"lease_expires_at,omitempty"`
	AttemptCount       int             `json:"attempt_count"`
	Result             json.RawMessage `json:"result,omitempty"`
	Error              string          `json:"error,omitempty"`
	CreatedAt          int64           `json:"created_at"`
	UpdatedAt          int64           `json:"updated_at"`
	CompletedAt        *int64          `json:"completed_at,omitempty"`
}

// SubmitTaskRequest is the payload accepted by the Cloudflare Hub.
type SubmitTaskRequest struct {
	Kind               string `json:"kind"`
	TargetClientID     string `json:"target_client_id,omitempty"`
	RequiredCapability string `json:"required_capability,omitempty"`
	IdempotencyKey     string `json:"idempotency_key,omitempty"`
	Payload            any    `json:"payload"`
}

// Status is the local connection status exposed by the API.
type Status struct {
	Enabled      bool     `json:"enabled"`
	Connected    bool     `json:"connected"`
	HubID        string   `json:"hub_id,omitempty"`
	ClientID     string   `json:"client_id,omitempty"`
	URL          string   `json:"url,omitempty"`
	Capabilities []string `json:"capabilities"`
	ConnectedAt  int64    `json:"connected_at,omitempty"`
	LastError    string   `json:"last_error,omitempty"`
}

// Executor performs a task assigned to this client and returns JSON result data.
type Executor func(context.Context, Task) (json.RawMessage, error)

// TerminalHandler receives completed and failed tasks published by this client.
type TerminalHandler func(Task)
