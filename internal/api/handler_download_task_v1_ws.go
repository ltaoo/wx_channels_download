package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

const (
	downloadTaskWSUpsert = "task_upsert"
	downloadTaskWSDelete = "task_delete"
	downloadTaskWSStats  = "task_stats"

	// progressThrottle is the minimum interval between progress broadcasts
	// for a single task. Lower values give smoother UI updates.
	progressThrottle = 100 * time.Millisecond
)

// progressCacheEntry caches the DB-derived task metadata and endpoint URLs
// so that broadcastDownloadTaskProgress can build the WS message without
// any database queries. The cache is populated by broadcastDownloadTaskUpsert
// (which already queries the DB for non-progress events) and invalidated on
// terminal events (finished / failed).
type progressCacheEntry struct {
	task         model.DownloadTaskV1
	resourceURLs map[int]string // resourceID -> first URL
	taskURL      string
}

var (
	progressCacheMu sync.RWMutex
	progressCache   = make(map[int]*progressCacheEntry)
)

// cacheTaskProgressMeta loads and caches the task metadata + endpoint URLs for
// use by broadcastDownloadTaskProgress. Called from broadcastDownloadTaskUpsert
// after a successful DB load, so the cache is always fresh before progress
// events start arriving.
func (c *APIClient) cacheTaskProgressMeta(taskID int) {
	if c.db == nil {
		c.logger.Info().Int("taskID", taskID).Msg("progress_cache: skip (no db)")
		return
	}

	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		c.logger.Info().Int("taskID", taskID).Err(err).Msg("progress_cache: DB load failed")
		return
	}

	type resURL struct {
		ResourceID int    `gorm:"column:resource_id"`
		URL        string `gorm:"column:url"`
	}
	var resURLs []resURL
	_ = c.db.Raw(`SELECT r.id AS resource_id, e.url FROM download_endpoint e
		JOIN download_resource r ON e.resource_id = r.id
		WHERE r.task_id = ? AND r.deleted_at IS NULL AND e.deleted_at IS NULL AND e.enabled = 1
		ORDER BY r.id ASC, e.priority ASC, e.id ASC`, taskID).Scan(&resURLs)

	urlByResource := make(map[int]string)
	var taskURL string
	for _, u := range resURLs {
		if taskURL == "" {
			taskURL = u.URL
		}
		if _, exists := urlByResource[u.ResourceID]; !exists {
			urlByResource[u.ResourceID] = u.URL
		}
	}

	progressCacheMu.Lock()
	progressCache[taskID] = &progressCacheEntry{
		task:         task,
		resourceURLs: urlByResource,
		taskURL:      taskURL,
	}
	progressCacheMu.Unlock()

	c.logger.Info().
		Int("taskID", taskID).
		Str("taskName", task.Name).
		Int("status", task.Status).
		Int("resourceURLs", len(urlByResource)).
		Msg("progress_cache: populated")
}

// removeCachedTaskProgressMeta clears the cached metadata for a task.
func removeCachedTaskProgressMeta(taskID int) {
	progressCacheMu.Lock()
	delete(progressCache, taskID)
	progressCacheMu.Unlock()
}

// taskBroadcaster throttles and dispatches download task WebSocket broadcasts.
// It runs the heavy buildDownloadTaskRecord DB query in a goroutine so the
// download pipeline is never blocked on WS broadcast work. Only one in-flight
// broadcast per task is permitted, and EventProgress is throttled.
type taskBroadcaster struct {
	mu        sync.Mutex
	active    map[int]bool      // tasks currently broadcasting
	last      map[int]time.Time // last broadcast time per task
	statsLast time.Time         // last stats broadcast time
}

func newTaskBroadcaster() *taskBroadcaster {
	return &taskBroadcaster{
		active: make(map[int]bool),
		last:   make(map[int]time.Time),
	}
}

// notify schedules a broadcast for the given task. For terminal events
// (finished / failed) the broadcast always runs immediately. For other events
// (especially EventProgress) broadcasts are throttled to ≈100ms to balance
// UI smoothness against database load.
// progress carries in-memory download state from the HermesEngine; when non-nil for
// EventProgress, the lightweight broadcastDownloadTaskProgress path is used
// instead of the full DB query path.
func (b *taskBroadcaster) notify(c *APIClient, taskID int, event hermes.EventType, progress *hermes.TaskProgress) {
	var dl, spd int64
	if progress != nil {
		dl, spd = progress.Downloaded, progress.Speed
	}
	isProgress := event == hermes.EventProgress && progress != nil

	b.mu.Lock()
	// Skip if a broadcast for this task is already in-flight.
	if b.active[taskID] {
		b.mu.Unlock()
		if isProgress {
			c.logger.Info().
				Int("taskID", taskID).
				Int64("dl", dl).
				Int64("spd", spd).
				Str("event", string(event)).
				Msg("progress: skip (active broadcast in-flight)")
		}
		return
	}
	isTerminal := event == "finished" || event == "failed"
	if !isTerminal {
		if prev, ok := b.last[taskID]; ok && time.Since(prev) < progressThrottle {
			b.mu.Unlock()
			if isProgress {
				c.logger.Info().
					Int("taskID", taskID).
					Int64("dl", dl).
					Int64("spd", spd).
					Str("event", string(event)).
					Dur("sinceLast", time.Since(prev)).
					Msg("progress: skip (throttled)")
			}
			return
		}
	}
	b.active[taskID] = true
	b.last[taskID] = time.Now()
	b.mu.Unlock()

	if isProgress {
		c.logger.Info().
			Int("taskID", taskID).
			Int64("dl", dl).
			Int64("spd", spd).
			Msg("progress: dispatching broadcast")
	}

	go func() {
		defer func() {
			b.mu.Lock()
			delete(b.active, taskID)
			b.mu.Unlock()
		}()
		if isProgress {
			c.broadcastDownloadTaskProgress(taskID, progress)
		} else {
			c.broadcastDownloadTaskUpsert([]int{taskID})
			// Refresh cached metadata so subsequent progress broadcasts can
			// build the WS message without any DB queries.
			if isTerminal {
				removeCachedTaskProgressMeta(taskID)
			} else {
				c.cacheTaskProgressMeta(taskID)
			}
		}
		// Push stats whenever a task event fires (throttled to avoid excessive DB queries).
		c.maybeBroadcastStats(b, isTerminal)
	}()
}

// maybeBroadcastStats pushes stats at most once per second, unless it's a
// terminal event (finished / failed / deleted) in which case it always fires.
func (c *APIClient) maybeBroadcastStats(b *taskBroadcaster, force bool) {
	b.mu.Lock()
	if !force && time.Since(b.statsLast) < time.Second {
		b.mu.Unlock()
		return
	}
	b.statsLast = time.Now()
	b.mu.Unlock()
	c.broadcastDownloadTaskStats()
}

// DownloadTaskWSMessage only carries the event type; the Tasks array is fully isomorphic with REST data.list[].
type DownloadTaskWSMessage struct {
	Type  string               `json:"type"`
	Tasks []DownloadTaskRecord `json:"tasks,omitempty"`
	Stats *DownloadTaskStats   `json:"stats,omitempty"`
}

var v1DownloadTaskUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var v1TaskHub = newTaskWSPool()

// taskWSPool is a WebSocket connection pool.
type taskWSPool struct {
	mu      sync.RWMutex
	clients map[*v1TaskClient]bool
}

func newTaskWSPool() *taskWSPool {
	return &taskWSPool{clients: make(map[*v1TaskClient]bool)}
}

func (h *taskWSPool) add(client *v1TaskClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = true
}

func (h *taskWSPool) remove(client *v1TaskClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
}

// BroadcastTasks pushes a unified task record array to clients subscribed to the given taskIDs.
func (h *taskWSPool) BroadcastTasks(taskIDs []int, payload DownloadTaskWSMessage) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	taskIDSet := make(map[int]bool, len(taskIDs))
	for _, id := range taskIDs {
		taskIDSet[id] = true
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.taskID != 0 && !taskIDSet[client.taskID] {
			continue
		}
		select {
		case client.send <- data:
		default:
		}
	}
}

// BroadcastStats pushes task statistics to all clients.
func (h *taskWSPool) BroadcastStats(stats *DownloadTaskStats) {
	data, err := json.Marshal(DownloadTaskWSMessage{
		Type:  downloadTaskWSStats,
		Stats: stats,
	})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

type v1TaskClient struct {
	conn   *websocket.Conn
	send   chan []byte
	taskID int
}

// handleDownloadTaskV1WS is the download task record push WebSocket.
// GET /ws/v1/download_task?task_id=1
func (c *APIClient) handleDownloadTaskV1WS(ctx *gin.Context) {
	conn, err := v1DownloadTaskUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	taskID, _ := strconv.Atoi(ctx.Query("task_id"))
	client := &v1TaskClient{
		conn:   conn,
		send:   make(chan []byte, 256),
		taskID: taskID,
	}
	v1TaskHub.add(client)
	go client.writePump()

	if client.taskID != 0 {
		if record, recordErr := c.buildDownloadTaskRecord(client.taskID); recordErr == nil && record != nil {
			client.enqueue(DownloadTaskWSMessage{Type: downloadTaskWSUpsert, Tasks: []DownloadTaskRecord{*record}})
		}
		// Populate the progress cache so subsequent progress broadcasts are instant.
		c.cacheTaskProgressMeta(client.taskID)
	}

	client.readPump()
	v1TaskHub.remove(client)
}

func (c *APIClient) broadcastDownloadTaskUpsert(taskIDs []int) {
	records := make([]DownloadTaskRecord, 0, len(taskIDs))
	for _, id := range taskIDs {
		record, err := c.buildDownloadTaskRecord(id)
		if err != nil || record == nil {
			continue
		}
		records = append(records, *record)
	}
	if len(records) == 0 {
		return
	}
	v1TaskHub.BroadcastTasks(taskIDs, DownloadTaskWSMessage{
		Type:  downloadTaskWSUpsert,
		Tasks: records,
	})
}

// broadcastDownloadTaskProgress builds a DownloadTaskRecord using cached
// task metadata and in-memory progress data, without any database queries.
// The cache is populated by cacheTaskProgressMeta which is called from
// broadcastDownloadTaskUpsert before progress events start arriving.
func (c *APIClient) broadcastDownloadTaskProgress(taskID int, p *hermes.TaskProgress) {
	if p == nil {
		return
	}

	progressCacheMu.RLock()
	entry, ok := progressCache[taskID]
	progressCacheMu.RUnlock()

	// Cache miss: the task was started before this code was deployed or the
	// cache was evicted. Fall back to a one-time DB load and cache it.
	if !ok && c.db != nil {
		c.logger.Info().Int("taskID", taskID).Msg("progress_cache: miss, loading from DB")
		c.cacheTaskProgressMeta(taskID)
		progressCacheMu.RLock()
		entry, ok = progressCache[taskID]
		progressCacheMu.RUnlock()
	}
	if !ok {
		c.logger.Info().Int("taskID", taskID).Msg("progress_cache: miss, giving up")
		return
	}

	task := entry.task
	// Update cached status from in-memory progress: if resources are being
	// downloaded, the task is effectively in Downloading state. This avoids
	// stale status in the cache when the task transitions between states.
	cachedStatus := task.Status
	if p.Downloaded > 0 && p.Downloaded < p.TotalSize {
		cachedStatus = model.TaskStatusDownloading
	}

	errorMessage := ""
	if cachedStatus == model.TaskStatusFailed {
		errorMessage = task.ErrorMessage
	}

	files := make([]DownloadTaskFileRecord, 0, len(p.Resources))
	for _, rp := range p.Resources {
		status := "waiting"
		if rp.Size > 0 && rp.Downloaded >= rp.Size {
			status = "finished"
		} else if rp.Downloaded > 0 || rp.Speed > 0 {
			status = "downloading"
		}
		if status != "finished" {
			switch cachedStatus {
			case model.TaskStatusPaused:
				status = "paused"
			case model.TaskStatusFailed:
				status = "error"
			case model.TaskStatusCancelled:
				status = "cancelled"
			}
		}
		fileResourceStatus := model.TaskStatusWaiting
		if status == "finished" {
			fileResourceStatus = model.TaskStatusFinished
		} else if status == "downloading" {
			fileResourceStatus = model.TaskStatusDownloading
		}
		files = append(files, DownloadTaskFileRecord{
			ID:           rp.ID,
			Name:         rp.Name,
			Kind:         rp.Kind,
			ResourceType: rp.Type,
			Type:         "file",
			Status:       status,
			Size:         rp.Size,
			Downloaded:   rp.Downloaded,
			Speed:        rp.Speed,
			Progress:     taskProgressPercent(rp.Downloaded, rp.Size, fileResourceStatus),
			URL:          entry.resourceURLs[rp.ID],
			OutputPath:   rp.Name,
			Error:        errorMessage,
		})
	}

	effectiveStatus := computeEffectiveTaskStatus(cachedStatus, files)

	pct := taskProgressPercent(p.Downloaded, p.TotalSize, effectiveStatus)

	record := DownloadTaskRecord{
		ID:           task.Id,
		ContentID:    task.ContentId,
		ParentTaskID: task.ParentTaskID,
		RootTaskID:   task.RootTaskID,
		RelationType: task.RelationType,
		Name:         task.Name,
		PlatformID:   task.PlatformId,
		Status:       effectiveStatus,
		SourceURL:    task.SourceURL,
		CoverURL:     task.CoverURL,
		CoverWidth:   task.CoverWidth,
		CoverHeight:  task.CoverHeight,
		ConfigJSON:   task.ConfigJSON,
		MetadataJSON: task.MetadataJSON,
		URL:          entry.taskURL,
		Size:         p.TotalSize,
		Downloaded:   p.Downloaded,
		Speed:        p.Speed,
		Progress:     pct,
		Error:        errorMessage,
		Files:        files,
		FileCount:    len(files),
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}

	c.logger.Info().
		Int("taskID", taskID).
		Int64("dl", p.Downloaded).
		Int64("size", p.TotalSize).
		Int64("spd", p.Speed).
		Float64("pct", pct).
		Int("files", len(files)).
		Msg("progress: WS pushed")

	v1TaskHub.BroadcastTasks([]int{taskID}, DownloadTaskWSMessage{
		Type:  downloadTaskWSUpsert,
		Tasks: []DownloadTaskRecord{record},
	})
}

// broadcastDownloadTaskStats queries task counts by status from the database and pushes them to all WS clients.
func (c *APIClient) broadcastDownloadTaskStats() {
	if c.db == nil {
		return
	}
	type statusCount struct {
		Status int `gorm:"column:status"`
		Count  int `gorm:"column:count"`
	}
	var counts []statusCount
	if err := c.db.Model(&model.DownloadTaskV1{}).
		Select("status, COUNT(*) AS count").
		Where("deleted_at IS NULL").
		Group("status").
		Scan(&counts).Error; err != nil {
		c.logger.Error().Err(err).Msg("Failed to query download task statistics")
		return
	}
	stats := &DownloadTaskStats{}
	for _, sc := range counts {
		stats.Total += sc.Count
		switch sc.Status {
		case model.TaskStatusWaiting:
			stats.Waiting = sc.Count
		case model.TaskStatusPreparing, model.TaskStatusDownloading, model.TaskStatusMerging:
			stats.Downloading += sc.Count
		case model.TaskStatusPaused:
			stats.Paused = sc.Count
		case model.TaskStatusFinished:
			stats.Finished = sc.Count
		case model.TaskStatusFailed:
			stats.Error = sc.Count
		}
	}
	v1TaskHub.BroadcastStats(stats)
}

func (c *APIClient) broadcastDownloadTaskDelete(records []DownloadTaskRecord) {
	if len(records) == 0 {
		return
	}
	taskIDs := make([]int, len(records))
	for i, r := range records {
		taskIDs[i] = r.ID
	}
	v1TaskHub.BroadcastTasks(taskIDs, DownloadTaskWSMessage{
		Type:  downloadTaskWSDelete,
		Tasks: records,
	})
}

func (c *v1TaskClient) enqueue(payload DownloadTaskWSMessage) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *v1TaskClient) readPump() {
	defer c.conn.Close()
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var body struct {
			Type   string `json:"type"`
			TaskID int    `json:"task_id"`
		}
		if err := json.Unmarshal(message, &body); err != nil {
			continue
		}
		if body.Type == "subscribe" && body.TaskID > 0 {
			c.taskID = body.TaskID
		}
	}
}

func (c *v1TaskClient) writePump() {
	ticker := time.NewTicker(10 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			writer, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = writer.Write(message)
			if err := writer.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
