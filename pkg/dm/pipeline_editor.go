package dm

import (
	"context"
	"encoding/json"
	"net/http"
)

// PipelineEditor reads the live page, including its unsaved working copy.
// Multiple matching pages return candidates for explicit selection.
func (c *Client) PipelineEditor(ctx context.Context, editor_id string, flow_id string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/automation/get_editor", nil, map[string]string{"editor_id": editor_id, "flow_id": flow_id})
}

// UpdatePipelineEditor waits for the selected page to apply a revision-checked draft.
func (c *Client) UpdatePipelineEditor(ctx context.Context, editor_id string, revision int64, draft json.RawMessage) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/automation/update_editor", nil, map[string]interface{}{"editor_id": editor_id, "revision": revision, "draft": draft})
}
