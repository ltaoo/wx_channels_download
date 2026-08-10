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
	"wx_channel/internal/services"
	"wx_channel/pkg/hermes"
)

const (
	download_task_ws_create = "task_create"
	download_task_ws_upsert = "task_upsert"
	download_task_ws_update = "task_update"
	download_task_ws_delete = "task_delete"
	download_task_ws_stats  = "task_stats"

	// progress_throttle is the minimum interval between progress broadcasts
	// for a single task. Lower values give smoother UI updates.
	progress_throttle = 100 * time.Millisecond
)

// progress_cache_entry caches the DB-derived task state needed to combine an
// in-memory progress snapshot with its current lifecycle status. Progress WS
// updates intentionally exclude stable task/resource metadata.
type progress_cache_entry struct {
	task model.DownloadTask
}

type task_broadcast_request struct {
	event    hermes.EventType
	progress *hermes.TaskProgress
}

var (
	progress_cache_mu sync.RWMutex
	progress_cache    = make(map[int]*progress_cache_entry)
)

// cache_task_progress_meta loads the task state used by progress patches.
func (c *APIClient) cache_task_progress_meta(task_id int) {
	if c.db == nil {
		c.logger.Info().Int("task_id", task_id).Msg("progress_cache: skip (no db)")
		return
	}

	var task model.DownloadTask
	if err := c.db.Where("id = ?", task_id).First(&task).Error; err != nil {
		c.logger.Info().Int("task_id", task_id).Err(err).Msg("progress_cache: DB load failed")
		return
	}

	progress_cache_mu.Lock()
	progress_cache[task_id] = &progress_cache_entry{task: task}
	progress_cache_mu.Unlock()

	c.logger.Info().
		Int("task_id", task_id).
		Str("task_name", task.Name).
		Int("status", task.Status).
		Msg("progress_cache: populated")
}

// remove_cached_task_progress_meta clears the cached metadata for a task.
func remove_cached_task_progress_meta(task_id int) {
	progress_cache_mu.Lock()
	delete(progress_cache, task_id)
	progress_cache_mu.Unlock()
}

// task_broadcaster throttles and dispatches download task WebSocket broadcasts.
// It runs the heavy build_download_task_record DB query in a goroutine so the
// download pipeline is never blocked on WS broadcast work. Only one in-flight
// broadcast per task is permitted, terminal events are queued behind in-flight
// broadcasts, lifecycle state changes are queued, and EventProgress is
// throttled.
type task_broadcaster struct {
	mu      sync.Mutex
	active  map[int]bool // tasks currently broadcasting
	pending map[int][]task_broadcast_request
	last    map[int]time.Time // last broadcast time per task
}

func new_task_broadcaster() *task_broadcaster {
	return &task_broadcaster{
		active:  make(map[int]bool),
		pending: make(map[int][]task_broadcast_request),
		last:    make(map[int]time.Time),
	}
}

func is_terminal_task_event(event hermes.EventType) bool {
	return event == hermes.EventFinished || event == hermes.EventFailed || event == hermes.EventDeleted
}

// should_broadcast_download_task_stats reports lifecycle events whose persisted
// status changes the aggregate task counters. Deletion stats are broadcast by
// the API handler after the soft delete is persisted, rather than on the
// earlier Hermes cancellation event.
func should_broadcast_download_task_stats(event hermes.EventType) bool {
	switch event {
	case hermes.EventCreated,
		hermes.EventPreparing,
		hermes.EventStarted,
		hermes.EventPaused,
		hermes.EventFinished,
		hermes.EventFailed:
		return true
	default:
		return false
	}
}

// notify schedules a broadcast for the given task. Terminal events
// (finished / failed / deleted) are never dropped; if a progress broadcast is
// already in-flight, the terminal event is queued and sent immediately after it.
// For other events
// (especially EventProgress) broadcasts are throttled to ≈100ms to balance
// UI smoothness against database load.
// progress carries in-memory download state from the HermesEngine; when non-nil for
// EventProgress, the lightweight broadcast_download_task_progress path is used
// instead of the full DB query path.
func (b *task_broadcaster) notify(c *APIClient, task_id int, event hermes.EventType, progress *hermes.TaskProgress) {
	req := task_broadcast_request{event: event, progress: progress}
	var dl, spd int64
	if progress != nil {
		dl, spd = progress.Downloaded, progress.Speed
	}
	is_progress := event == hermes.EventProgress && progress != nil
	is_terminal := is_terminal_task_event(event)

	b.mu.Lock()
	// Progress snapshots may be dropped because a newer one will follow. State
	// transitions must be delivered after the in-flight broadcast so start,
	// pause, resume, and terminal states remain observable.
	if b.active[task_id] {
		if !is_progress {
			b.pending[task_id] = append(b.pending[task_id], req)
		}
		b.mu.Unlock()
		if is_progress {
			c.logger.Info().
				Int("task_id", task_id).
				Int64("dl", dl).
				Int64("spd", spd).
				Str("event", string(event)).
				Msg("progress: skip (active broadcast in-flight)")
		}
		return
	}
	if is_progress && !is_terminal {
		if prev, ok := b.last[task_id]; ok && time.Since(prev) < progress_throttle {
			b.mu.Unlock()
			if is_progress {
				c.logger.Info().
					Int("task_id", task_id).
					Int64("dl", dl).
					Int64("spd", spd).
					Str("event", string(event)).
					Dur("since_last", time.Since(prev)).
					Msg("progress: skip (throttled)")
			}
			return
		}
	}
	b.active[task_id] = true
	b.last[task_id] = time.Now()
	b.mu.Unlock()

	if is_progress {
		c.logger.Info().
			Int("task_id", task_id).
			Int64("dl", dl).
			Int64("spd", spd).
			Msg("progress: dispatching broadcast")
	}

	go b.run_task_broadcast(c, task_id, req)
}

func (b *task_broadcaster) run_task_broadcast(c *APIClient, task_id int, req task_broadcast_request) {
	for {
		is_progress := req.event == hermes.EventProgress && req.progress != nil
		is_terminal := is_terminal_task_event(req.event)
		switch {
		case req.event == hermes.EventCreated:
			c.broadcast_download_task_create(task_id)
			c.cache_task_progress_meta(task_id)
		case is_progress:
			c.broadcast_download_task_progress(task_id, req.progress)
		case req.event == hermes.EventPreparing || req.event == hermes.EventStarted || req.event == hermes.EventPaused:
			c.broadcast_download_task_state(task_id)
			c.cache_task_progress_meta(task_id)
		default:
			c.broadcast_download_task_upsert([]int{task_id})
			if is_terminal {
				remove_cached_task_progress_meta(task_id)
			} else {
				c.cache_task_progress_meta(task_id)
			}
		}
		if should_broadcast_download_task_stats(req.event) {
			c.broadcast_download_task_stats()
		}

		b.mu.Lock()
		pending_requests := b.pending[task_id]
		if len(pending_requests) == 0 {
			delete(b.active, task_id)
			b.mu.Unlock()
			return
		}
		next := pending_requests[0]
		if len(pending_requests) == 1 {
			delete(b.pending, task_id)
		} else {
			b.pending[task_id] = pending_requests[1:]
		}
		b.last[task_id] = time.Now()
		b.mu.Unlock()
		req = next
	}
}

// DownloadTaskWSUpdate is a lightweight patch for an existing task. Stable
// fields such as names, URLs, configuration, and creation time are supplied
// only by a full task_create/task_upsert record.
type DownloadTaskWSUpdate struct {
	ID         int                        `json:"id"`
	Status     int                        `json:"status"`
	Size       int64                      `json:"size"`
	Downloaded int64                      `json:"downloaded"`
	Speed      int64                      `json:"speed"`
	Progress   float64                    `json:"progress"`
	Error      string                     `json:"error"`
	Files      []DownloadTaskFileWSUpdate `json:"files,omitempty"`
	UpdatedAt  int64                      `json:"updated_at,omitempty"`
}

// DownloadTaskFileWSUpdate is the mutable subset of a task file record.
type DownloadTaskFileWSUpdate struct {
	ID         int     `json:"id"`
	Status     string  `json:"status"`
	Size       int64   `json:"size"`
	Downloaded int64   `json:"downloaded"`
	Speed      int64   `json:"speed"`
	Progress   float64 `json:"progress"`
	Error      string  `json:"error"`
}

// DownloadTaskWSMessage carries full REST-isomorphic task records, lightweight
// updates, deleted task IDs, or aggregate stats.
type DownloadTaskWSMessage struct {
	Type    string                        `json:"type"`
	Tasks   []services.DownloadTaskRecord `json:"tasks,omitempty"`
	Updates []DownloadTaskWSUpdate        `json:"updates,omitempty"`
	TaskIDs []int                         `json:"task_ids,omitempty"`
	Stats   *services.DownloadTaskStats   `json:"stats,omitempty"`
}

var v1_download_task_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var v1_task_hub = new_task_ws_pool()

// task_ws_pool is a WebSocket connection pool.
type task_ws_pool struct {
	mu      sync.RWMutex
	clients map[*v1_task_client]bool
}

func new_task_ws_pool() *task_ws_pool {
	return &task_ws_pool{clients: make(map[*v1_task_client]bool)}
}

func (h *task_ws_pool) add(client *v1_task_client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = true
}

func (h *task_ws_pool) remove(client *v1_task_client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
}

// BroadcastTasks pushes a unified task record array to clients subscribed to the given task_ids.
func (h *task_ws_pool) BroadcastTasks(task_ids []int, payload DownloadTaskWSMessage) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	task_id_set := make(map[int]bool, len(task_ids))
	for _, id := range task_ids {
		task_id_set[id] = true
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.task_id != 0 && !task_id_set[client.task_id] {
			continue
		}
		select {
		case client.send <- data:
		default:
		}
	}
}

// BroadcastStats pushes task statistics to all clients.
func (h *task_ws_pool) BroadcastStats(stats *services.DownloadTaskStats) {
	data, err := json.Marshal(DownloadTaskWSMessage{
		Type:  download_task_ws_stats,
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

type v1_task_client struct {
	conn    *websocket.Conn
	send    chan []byte
	task_id int
}

// handle_download_task_ws is the download task record push WebSocket.
// GET /ws/v1/download_task?task_id=1
func (c *APIClient) handle_download_task_ws(ctx *gin.Context) {
	conn, err := v1_download_task_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	task_id, _ := strconv.Atoi(ctx.Query("task_id"))
	client := &v1_task_client{
		conn:    conn,
		send:    make(chan []byte, 256),
		task_id: task_id,
	}
	v1_task_hub.add(client)
	go client.write_pump()

	if client.task_id != 0 {
		if record, record_err := c.download_task_service.BuildTaskRecord(client.task_id); record_err == nil && record != nil {
			client.enqueue(DownloadTaskWSMessage{Type: download_task_ws_upsert, Tasks: []services.DownloadTaskRecord{*record}})
		}
		// Populate the progress cache so subsequent progress broadcasts are instant.
		c.cache_task_progress_meta(client.task_id)
	}

	client.read_pump()
	v1_task_hub.remove(client)
}

func (c *APIClient) broadcast_download_task_upsert(task_ids []int) {
	records := make([]services.DownloadTaskRecord, 0, len(task_ids))
	for _, id := range task_ids {
		record, err := c.download_task_service.BuildTaskRecord(id)
		if err != nil || record == nil {
			continue
		}
		records = append(records, *record)
	}
	if len(records) == 0 {
		return
	}
	v1_task_hub.BroadcastTasks(task_ids, DownloadTaskWSMessage{
		Type:  download_task_ws_upsert,
		Tasks: records,
	})
}

// broadcast_download_task_create sends a complete REST-isomorphic record so
// the frontend can insert a task without an additional list request.
func (c *APIClient) broadcast_download_task_create(task_id int) {
	record, err := c.download_task_service.BuildTaskRecord(task_id)
	if err != nil || record == nil {
		return
	}
	v1_task_hub.BroadcastTasks([]int{task_id}, DownloadTaskWSMessage{
		Type:  download_task_ws_create,
		Tasks: []services.DownloadTaskRecord{*record},
	})
}

func download_task_ws_update_from_record(record services.DownloadTaskRecord) DownloadTaskWSUpdate {
	files := make([]DownloadTaskFileWSUpdate, 0, len(record.Files))
	for _, file := range record.Files {
		files = append(files, DownloadTaskFileWSUpdate{
			ID:         file.ID,
			Status:     file.Status,
			Size:       file.Size,
			Downloaded: file.Downloaded,
			Speed:      file.Speed,
			Progress:   file.Progress,
			Error:      file.Error,
		})
	}
	return DownloadTaskWSUpdate{
		ID:         record.ID,
		Status:     record.Status,
		Size:       record.Size,
		Downloaded: record.Downloaded,
		Speed:      record.Speed,
		Progress:   record.Progress,
		Error:      record.Error,
		Files:      files,
		UpdatedAt:  record.UpdatedAt,
	}
}

// broadcast_download_task_state sends only mutable fields for start/resume and
// pause transitions. BuildTaskRecord remains the source of truth, but stable
// list fields are not serialized again.
func (c *APIClient) broadcast_download_task_state(task_id int) {
	record, err := c.download_task_service.BuildTaskRecord(task_id)
	if err != nil || record == nil {
		return
	}
	v1_task_hub.BroadcastTasks([]int{task_id}, DownloadTaskWSMessage{
		Type:    download_task_ws_update,
		Updates: []DownloadTaskWSUpdate{download_task_ws_update_from_record(*record)},
	})
}

// broadcast_download_task_progress builds a lightweight update from cached task
// state and in-memory progress, without database queries or stable metadata.
func (c *APIClient) broadcast_download_task_progress(task_id int, p *hermes.TaskProgress) {
	if p == nil {
		return
	}

	progress_cache_mu.RLock()
	entry, ok := progress_cache[task_id]
	progress_cache_mu.RUnlock()

	// Cache miss: the task was started before this code was deployed or the
	// cache was evicted. Fall back to a one-time DB load and cache it.
	if !ok && c.db != nil {
		c.logger.Info().Int("task_id", task_id).Msg("progress_cache: miss, loading from DB")
		c.cache_task_progress_meta(task_id)
		progress_cache_mu.RLock()
		entry, ok = progress_cache[task_id]
		progress_cache_mu.RUnlock()
	}
	if !ok {
		c.logger.Info().Int("task_id", task_id).Msg("progress_cache: miss, giving up")
		return
	}

	task := entry.task
	// Update cached status from in-memory progress: if resources are being
	// downloaded, the task is effectively in Downloading state. This avoids
	// stale status in the cache when the task transitions between states.
	cached_status := task.Status
	if p.Downloaded > 0 && p.Downloaded < p.TotalSize {
		cached_status = model.TaskStatusDownloading
	}

	error_message := ""
	if cached_status == model.TaskStatusFailed {
		error_message = task.ErrorMessage
	}

	files := make([]DownloadTaskFileWSUpdate, 0, len(p.Resources))
	status_files := make([]services.DownloadTaskFileRecord, 0, len(p.Resources))
	for _, rp := range p.Resources {
		status := download_task_file_ws_status(cached_status, rp)
		file_resource_status := model.TaskStatusWaiting
		if status == "finished" {
			file_resource_status = model.TaskStatusFinished
		} else if status == "downloading" {
			file_resource_status = model.TaskStatusDownloading
		}
		file_progress := services.TaskProgressPercent(rp.Downloaded, rp.Size, file_resource_status)
		files = append(files, DownloadTaskFileWSUpdate{
			ID:         rp.ID,
			Status:     status,
			Size:       rp.Size,
			Downloaded: rp.Downloaded,
			Speed:      rp.Speed,
			Progress:   file_progress,
			Error:      error_message,
		})
		status_files = append(status_files, services.DownloadTaskFileRecord{Status: status})
	}

	effective_status := services.ComputeEffectiveTaskStatus(cached_status, status_files)

	pct := services.TaskProgressPercent(p.Downloaded, p.TotalSize, effective_status)

	update := DownloadTaskWSUpdate{
		ID:         task.Id,
		Status:     effective_status,
		Size:       p.TotalSize,
		Downloaded: p.Downloaded,
		Speed:      p.Speed,
		Progress:   pct,
		Error:      error_message,
		Files:      files,
	}

	// Cache invalidation is also used as a terminal-event barrier. Holding the
	// read lock through the send guarantees that a stop handler which removes
	// this entry can publish the final DB-backed record after any older progress.
	progress_cache_mu.RLock()
	current_entry, still_current := progress_cache[task_id]
	if !still_current || current_entry != entry {
		progress_cache_mu.RUnlock()
		return
	}
	v1_task_hub.BroadcastTasks([]int{task_id}, DownloadTaskWSMessage{
		Type:    download_task_ws_update,
		Updates: []DownloadTaskWSUpdate{update},
	})
	progress_cache_mu.RUnlock()
}

func download_task_file_ws_status(cached_status int, resource_progress hermes.ResourceProgress) string {
	status := "waiting"
	if resource_progress.Size > 0 && resource_progress.Downloaded >= resource_progress.Size {
		status = "finished"
	} else if resource_progress.Downloaded > 0 ||
		resource_progress.Speed > 0 ||
		cached_status == model.TaskStatusDownloading {
		status = "downloading"
	}
	if status == "finished" {
		return status
	}
	switch cached_status {
	case model.TaskStatusPaused:
		return "paused"
	case model.TaskStatusFailed:
		return "error"
	case model.TaskStatusCancelled:
		return "cancelled"
	default:
		return status
	}
}

type download_task_status_count struct {
	Status int `gorm:"column:status"`
	Count  int `gorm:"column:count"`
}

func add_download_task_stats_count(stats *services.DownloadTaskStats, status, count int) {
	stats.Total += count
	switch status {
	case model.TaskStatusWaiting, model.TaskStatusPreparing:
		stats.Waiting += count
	case model.TaskStatusDownloading, model.TaskStatusMerging:
		stats.Downloading += count
	case model.TaskStatusPaused:
		stats.Paused += count
	case model.TaskStatusFinished:
		stats.Finished += count
	case model.TaskStatusFailed, model.TaskStatusCancelled:
		stats.Error += count
	}
}

// broadcast_download_task_stats queries task counts by status from the database and pushes them to all WS clients.
func (c *APIClient) broadcast_download_task_stats() {
	if c.db == nil {
		return
	}
	var counts []download_task_status_count
	if err := c.db.Model(&model.DownloadTask{}).
		Select("status, COUNT(*) AS count").
		Where("deleted_at IS NULL").
		Group("status").
		Scan(&counts).Error; err != nil {
		c.logger.Error().Err(err).Msg("Failed to query download task statistics")
		return
	}
	stats := &services.DownloadTaskStats{}
	for _, sc := range counts {
		add_download_task_stats_count(stats, sc.Status, sc.Count)
	}
	v1_task_hub.BroadcastStats(stats)
}

func (c *APIClient) broadcast_download_task_delete(task_ids []int) {
	if len(task_ids) == 0 {
		return
	}
	v1_task_hub.BroadcastTasks(task_ids, DownloadTaskWSMessage{
		Type:    download_task_ws_delete,
		TaskIDs: task_ids,
	})
}

func (c *v1_task_client) enqueue(payload DownloadTaskWSMessage) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *v1_task_client) read_pump() {
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
			c.task_id = body.TaskID
		}
	}
}

func (c *v1_task_client) write_pump() {
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
