package dm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Status probes the downloader liveness endpoint. A reachable downloader always
// answers with a valid envelope.
func (c *Client) Status(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "/api/status", nil)
}

// PlatformStatus returns the scraper platform availability snapshot.
func (c *Client) PlatformStatus(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "/api/scraper/platform/status", nil)
}

// Config returns the resolvable application configuration fields.
func (c *Client) Config(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "/api/config", nil)
}

// UpdateConfig saves non-read-only configuration fields.
func (c *Client) UpdateConfig(ctx context.Context, values map[string]any) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/config", nil, map[string]any{"values": values})
}

// RestartStatus confirms the outcome of a previously requested restart.
func (c *Client) RestartStatus(ctx context.Context, restart_token string) (json.RawMessage, error) {
	return c.get(ctx, "/api/restart/status", url.Values{"restart_token": []string{restart_token}})
}
