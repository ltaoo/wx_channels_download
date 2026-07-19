package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var v1DownloadTaskUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool { return true },
}

var v1TaskHub = newTaskWSPool()

// taskWSPool WebSocket 连接池
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

// BroadcastProgress 向订阅指定 taskId 的客户端推送下载进度
func (h *taskWSPool) BroadcastProgress(taskID int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.taskID != 0 && client.taskID != taskID {
			continue
		}
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

// handleDownloadTaskV1WS 下载任务进度推送 WebSocket
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
		// 发送当前任务快照
		if snapshot := c.buildTaskProgressSnapshot(client.taskID); snapshot != nil {
			client.enqueue(snapshot)
		}
	}

	client.readPump()
	v1TaskHub.remove(client)
}

// buildTaskProgressSnapshot 构建任务进度快照
func (c *APIClient) buildTaskProgressSnapshot(taskID int) *V1TaskProgress {
	if c.db == nil {
		return nil
	}

	var task struct {
		Id     int
		Status int
	}
	if err := c.db.Table("download_task_v1").Select("id, status").Where("id = ?", taskID).Scan(&task).Error; err != nil || task.Id == 0 {
		return nil
	}

	snapshot := &V1TaskProgress{
		Type:   "task_snapshot",
		TaskID: task.Id,
		Status: task.Status,
	}

	// 查询 resources
	type resRow struct {
		Id         int
		Name       string
		Kind       string
		Size       int64
		Status     int
		MergeOrder int
	}
	var resources []resRow
	c.db.Table("download_resource").Where("task_id = ?", taskID).Order("merge_order ASC").Scan(&resources)

	// 查询 segments
	type segRow struct {
		ResourceId int
		Index      int
		Size       int64
		Downloaded int64
		Status     int
	}
	var segments []segRow
	c.db.Table("download_segment").Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", taskID).Scan(&segments)
	segByResource := map[int][]segRow{}
	for _, s := range segments {
		segByResource[s.ResourceId] = append(segByResource[s.ResourceId], s)
	}

	// 查询 connections
	type connRow struct {
		EndpointId int
		Speed      int64
		Bytes      int64
		Status     int
	}
	var connections []connRow
	c.db.Table("download_connection").Where("endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?))", taskID).Scan(&connections)
	connByEndpoint := map[int]connRow{}
	for _, cn := range connections {
		connByEndpoint[cn.EndpointId] = cn
	}

	// 查询 endpoints
	type epRow struct {
		Id         int
		ResourceId int
		URL        string
		Protocol   string
	}
	var endpoints []epRow
	c.db.Table("download_endpoint").Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", taskID).Scan(&endpoints)
	epByResource := map[int][]epRow{}
	for _, ep := range endpoints {
		epByResource[ep.ResourceId] = append(epByResource[ep.ResourceId], ep)
	}

	for _, r := range resources {
		ri := V1ResourceProgress{
			ID:         r.Id,
			Name:       r.Name,
			Kind:       r.Kind,
			Size:       r.Size,
			Status:     r.Status,
			MergeOrder: r.MergeOrder,
		}

		// segments
		for _, s := range segByResource[r.Id] {
			ri.Segments = append(ri.Segments, V1SegmentProgress{
				Index:      s.Index,
				Size:       s.Size,
				Downloaded: s.Downloaded,
				Status:     s.Status,
			})
			ri.Downloaded += s.Downloaded
		}

		// connection speed
		for _, ep := range epByResource[r.Id] {
			if cn, ok := connByEndpoint[ep.Id]; ok {
				ri.Speed = cn.Speed
			}
		}

		snapshot.Resources = append(snapshot.Resources, ri)
	}

	return snapshot
}

// V1TaskProgress 下载进度推送消息
type V1TaskProgress struct {
	Type      string              `json:"type"`
	TaskID    int                 `json:"task_id"`
	Status    int                 `json:"status"`
	Resources []V1ResourceProgress `json:"resources"`
}

// V1ResourceProgress 资源进度
type V1ResourceProgress struct {
	ID         int                  `json:"id"`
	Name       string               `json:"name"`
	Kind       string               `json:"kind"`
	Size       int64                `json:"size"`
	Downloaded int64                `json:"downloaded"`
	Status     int                  `json:"status"`
	MergeOrder int                  `json:"merge_order"`
	Speed      int64                `json:"speed"`
	Segments   []V1SegmentProgress   `json:"segments"`
}

// V1SegmentProgress 分片进度
type V1SegmentProgress struct {
	Index      int   `json:"index"`
	Size       int64 `json:"size"`
	Downloaded int64 `json:"downloaded"`
	Status     int   `json:"status"`
}

func (c *v1TaskClient) enqueue(payload any) {
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
