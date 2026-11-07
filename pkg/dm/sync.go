package dm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// SyncServers lists configured targets.
func (c *Client) SyncServers(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "/api/v1/sync/servers", nil)
}

// SyncPreview counts the records and existing files in a full export.
func (c *Client) SyncPreview(ctx context.Context, task_ids ...int) (json.RawMessage, error) {
	query := url.Values{}
	if len(task_ids) > 0 {
		ids := make([]string, len(task_ids))
		for index, id := range task_ids {
			ids[index] = strconv.Itoa(id)
		}
		query.Set("task_ids", strings.Join(ids, ","))
	}
	return c.get(ctx, "/api/v1/sync/preview", query)
}

// ConfigureSyncServer saves a named target without requiring credentials.
func (c *Client) ConfigureSyncServer(ctx context.Context, name string, target_url string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/sync/servers", nil, map[string]string{"name": name, "url": target_url})
}

// StartSync starts a background export; poll SyncStatus until completed/failed.
func (c *Client) StartSync(ctx context.Context, server_id int, task_ids ...int) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/sync/servers/"+strconv.Itoa(server_id)+"/jobs", nil, map[string]any{"task_ids": task_ids})
}

// SyncStatus returns a background export's current state and final counts.
func (c *Client) SyncStatus(ctx context.Context, job_id string) (json.RawMessage, error) {
	return c.get(ctx, "/api/v1/sync/jobs/"+url.PathEscape(job_id), nil)
}
