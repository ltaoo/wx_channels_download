package api

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

func (c *APIClient) attachPlatformWorkflowPersistence(run *platformWorkflowRun) *platformWorkflowRun {
	if run == nil {
		return nil
	}
	run.onChange = func(next *platformWorkflowRun) {
		_ = c.persistPlatformWorkflowRun(next)
	}
	return run
}

func (c *APIClient) persistPlatformWorkflowRun(run *platformWorkflowRun) error {
	if c == nil || c.db == nil || run == nil || strings.TrimSpace(run.ID) == "" {
		return nil
	}
	rec := platformWorkflowRecordFromRun(run)
	if err := c.db.Save(&rec).Error; err != nil {
		if platformWorkflowPersistenceTableMissing(err) {
			return nil
		}
		return err
	}
	return nil
}

func (c *APIClient) loadPlatformWorkflowRun(runID string) (*platformWorkflowRun, error) {
	runID = normalizePlatformWorkflowRunID(runID)
	if c == nil || c.db == nil || runID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var rec model.PlatformWorkflowRun
	if err := c.db.First(&rec, "id = ?", runID).Error; err != nil {
		if platformWorkflowPersistenceTableMissing(err) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	run := platformWorkflowRunFromRecord(rec)
	c.attachPlatformWorkflowPersistence(run)
	platformWorkflowRuns.Store(run.ID, run)
	return run, nil
}

func (c *APIClient) loadPersistedPlatformWorkflowRuns() {
	if c == nil || c.db == nil {
		return
	}
	var records []model.PlatformWorkflowRun
	err := c.db.
		Where("status IN ?", []string{"running", "paused", "failed"}).
		Order("updated_at DESC").
		Find(&records).Error
	if err != nil {
		return
	}
	for _, rec := range records {
		run := platformWorkflowRunFromRecord(rec)
		c.attachPlatformWorkflowPersistence(run)
		platformWorkflowRuns.Store(run.ID, run)
	}
}

func platformWorkflowPersistenceTableMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "unknown table")
}

func platformWorkflowRecordFromRun(run *platformWorkflowRun) model.PlatformWorkflowRun {
	run.mu.Lock()
	defer run.mu.Unlock()

	now := time.Now()
	createdAt := run.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := run.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = now
	}
	selection := run.Selection
	if len(selection) == 0 {
		selection = platformWorkflowSelectionFromNodes(run.Nodes)
	}
	errText := platformWorkflowErrorFromNodes(run.Nodes)
	return model.PlatformWorkflowRun{
		ID:             run.ID,
		URL:            run.URL,
		Platform:       run.Platform,
		Status:         run.Status,
		CurrentNode:    run.CurrentNode,
		TaskID:         run.TaskID,
		DownloadTaskID: run.DownloadTaskID,
		Extra:          platformWorkflowJSON(run.Extra),
		Output:         platformWorkflowJSON(run.Output),
		Selection:      platformWorkflowJSON(selection),
		Nodes:          platformWorkflowJSON(run.Nodes),
		Error:          errText,
		Timestamps: model.Timestamps{
			CreatedAt: platformWorkflowTimeMillis(createdAt),
			UpdatedAt: platformWorkflowTimeMillis(updatedAt),
		},
	}
}

func platformWorkflowRunFromRecord(rec model.PlatformWorkflowRun) *platformWorkflowRun {
	var extra map[string]any
	var output map[string]any
	var selection map[string]any
	var nodes []platformWorkflowNode
	_ = json.Unmarshal([]byte(rec.Extra), &extra)
	_ = json.Unmarshal([]byte(rec.Output), &output)
	_ = json.Unmarshal([]byte(rec.Selection), &selection)
	_ = json.Unmarshal([]byte(rec.Nodes), &nodes)
	if len(selection) == 0 {
		selection = platformWorkflowSelectionFromNodes(nodes)
	}
	return &platformWorkflowRun{
		ID:             rec.ID,
		URL:            rec.URL,
		Platform:       rec.Platform,
		Status:         firstNonEmpty(rec.Status, "running"),
		CurrentNode:    rec.CurrentNode,
		TaskID:         rec.TaskID,
		DownloadTaskID: rec.DownloadTaskID,
		Extra:          extra,
		Output:         output,
		Selection:      selection,
		Nodes:          nodes,
		CreatedAt:      platformWorkflowTimeFromMillis(rec.CreatedAt),
		UpdatedAt:      platformWorkflowTimeFromMillis(rec.UpdatedAt),
	}
}

func platformWorkflowJSON(value any) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func platformWorkflowTimeMillis(t time.Time) int64 {
	if t.IsZero() {
		return util.NowMillis()
	}
	return t.UnixNano() / int64(time.Millisecond)
}

func platformWorkflowTimeFromMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Now()
	}
	return time.Unix(0, ms*int64(time.Millisecond))
}

func platformWorkflowSelectionFromNodes(nodes []platformWorkflowNode) map[string]any {
	for _, node := range nodes {
		if node.ID == "pause_after_probe" && len(node.Output) > 0 {
			return clonePlatformWorkflowMap(node.Output)
		}
	}
	return nil
}

func platformWorkflowErrorFromNodes(nodes []platformWorkflowNode) string {
	for i := len(nodes) - 1; i >= 0; i-- {
		if strings.TrimSpace(nodes[i].Error) != "" {
			return nodes[i].Error
		}
	}
	return ""
}

func clonePlatformWorkflowMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func platformWorkflowCreateBodyFromSelection(run *platformWorkflowRun) (platformCreateTaskBody, bool) {
	if run == nil {
		return platformCreateTaskBody{}, false
	}
	run.mu.Lock()
	selection := clonePlatformWorkflowMap(run.Selection)
	if len(selection) == 0 {
		selection = platformWorkflowSelectionFromNodes(run.Nodes)
	}
	body := platformCreateTaskBody{
		URL:   run.URL,
		RunID: run.ID,
		Extra: clonePlatformWorkflowMap(run.Extra),
	}
	run.mu.Unlock()
	if len(selection) == 0 {
		return body, false
	}

	body.VariantID = platformWorkflowString(selection["variant_id"])
	body.Spec = platformWorkflowString(selection["spec"])
	body.Suffix = platformWorkflowString(selection["suffix"])
	body.Filename = platformWorkflowString(selection["filename"])
	if rawOptions, ok := selection["options"]; ok && rawOptions != nil {
		data, _ := json.Marshal(rawOptions)
		_ = json.Unmarshal(data, &body.Options)
	}
	if body.Options.VariantID == "" {
		body.Options.VariantID = body.VariantID
	}
	if body.Options.Spec == "" {
		body.Options.Spec = body.Spec
	}
	if body.Options.Suffix == "" {
		body.Options.Suffix = body.Suffix
	}
	if body.Options.Filename == "" {
		body.Options.Filename = body.Filename
	}
	if rawExtra, ok := selection["extra"]; ok && rawExtra != nil {
		data, _ := json.Marshal(rawExtra)
		var extra map[string]any
		if err := json.Unmarshal(data, &extra); err == nil && len(extra) > 0 {
			body.Extra = extra
		}
	}
	if body.Options.Extra == nil {
		body.Options.Extra = body.Extra
	}
	return body, true
}

func platformWorkflowString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case nil:
		return ""
	default:
		return strings.TrimSpace(toCompatString(v))
	}
}

func platformWorkflowHasUserSelection(body platformCreateTaskBody) bool {
	if strings.TrimSpace(body.VariantID) != "" ||
		strings.TrimSpace(body.Spec) != "" ||
		strings.TrimSpace(body.Suffix) != "" ||
		strings.TrimSpace(body.Filename) != "" ||
		body.Cover {
		return true
	}
	return strings.TrimSpace(body.Options.VariantID) != "" ||
		strings.TrimSpace(body.Options.Spec) != "" ||
		strings.TrimSpace(body.Options.Suffix) != "" ||
		strings.TrimSpace(body.Options.Filename) != ""
}
