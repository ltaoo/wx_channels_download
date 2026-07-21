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

const (
	downloadTaskWSUpsert = "task_upsert"
	downloadTaskWSDelete = "task_delete"
)

// DownloadTaskWSMessage 只负责事件类型；Task 与 REST data.list[] 完全同构。
type DownloadTaskWSMessage struct {
	Type string             `json:"type"`
	Task DownloadTaskRecord `json:"task"`
}

var v1DownloadTaskUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var v1TaskHub = newTaskWSPool()

// taskWSPool WebSocket 连接池。
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

// BroadcastTask 向订阅指定 taskID 的客户端推送统一任务记录。
func (h *taskWSPool) BroadcastTask(taskID int, payload DownloadTaskWSMessage) {
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

// handleDownloadTaskV1WS 下载任务记录推送 WebSocket。
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
			client.enqueue(DownloadTaskWSMessage{Type: downloadTaskWSUpsert, Task: *record})
		}
	}

	client.readPump()
	v1TaskHub.remove(client)
}

func (c *APIClient) broadcastDownloadTaskUpsert(taskID int) {
	record, err := c.buildDownloadTaskRecord(taskID)
	if err != nil || record == nil {
		return
	}
	v1TaskHub.BroadcastTask(taskID, DownloadTaskWSMessage{
		Type: downloadTaskWSUpsert,
		Task: *record,
	})
}

func (c *APIClient) broadcastDownloadTaskDelete(record DownloadTaskRecord) {
	v1TaskHub.BroadcastTask(record.ID, DownloadTaskWSMessage{
		Type: downloadTaskWSDelete,
		Task: record,
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
