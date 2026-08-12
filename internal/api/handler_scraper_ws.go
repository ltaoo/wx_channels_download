package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/internal/adapter"
	"wx_channel/internal/events"
	"wx_channel/internal/services"
)

const (
	scraper_job_ws_update     = "scraper_job"
	platform_status_ws_update = "platform_status"
)

// scraper_ws_message is the WebSocket envelope for fetch-job state changes.
type scraper_ws_message struct {
	Type           string                         `json:"type"`
	Job            *services.ScraperFetchJob      `json:"job,omitempty"`
	Event          *services.ScraperFetchJobEvent `json:"event,omitempty"`
	PlatformStatus *events.PlatformStatusChanged  `json:"platform_status,omitempty"`
}

var scraper_ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}

var scraper_ws_hub = new_scraper_ws_pool()

type scraper_ws_pool struct {
	mu      sync.RWMutex
	clients map[*scraper_ws_client]struct{}
}

func new_scraper_ws_pool() *scraper_ws_pool {
	return &scraper_ws_pool{clients: make(map[*scraper_ws_client]struct{})}
}

func (h *scraper_ws_pool) add(client *scraper_ws_client) {
	if client == nil {
		return
	}
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
}

func (h *scraper_ws_pool) remove(client *scraper_ws_client) {
	if client == nil {
		return
	}
	removed := false
	h.mu.Lock()
	if _, exists := h.clients[client]; exists {
		delete(h.clients, client)
		removed = true
	}
	h.mu.Unlock()
	if removed {
		client.close_send()
	}
}

func (h *scraper_ws_pool) broadcast_job(job *services.ScraperFetchJob) {
	h.broadcast_job_event(job, nil)
}

func (h *scraper_ws_pool) broadcast_job_event(job *services.ScraperFetchJob, event *services.ScraperFetchJobEvent) {
	if job == nil {
		return
	}
	data, err := json.Marshal(scraper_ws_message{
		Type:  scraper_job_ws_update,
		Job:   job,
		Event: event,
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := make([]*scraper_ws_client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		if event == nil || event.Stage == services.ScraperJobEventProgress {
			client.enqueue_data(data)
			continue
		}
		client.send_reliable(data)
	}
}

func (h *scraper_ws_pool) broadcast_platform_status(status *events.PlatformStatusChanged) {
	if status == nil {
		return
	}
	data, err := json.Marshal(scraper_ws_message{
		Type:           platform_status_ws_update,
		PlatformStatus: status,
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := make([]*scraper_ws_client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		client.send_reliable(data)
	}
}

type scraper_ws_client struct {
	conn        *websocket.Conn
	send        chan []byte
	send_mu     sync.RWMutex
	send_closed bool
}

// handle_scraper_ws pushes fetch-job and platform state changes over one
// application-level connection. Clients filter scraper_job messages by job ID.
// GET /ws/scraper
func (c *APIClient) handle_scraper_ws(ctx *gin.Context) {
	conn, err := scraper_ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	client := &scraper_ws_client{
		conn: conn,
		send: make(chan []byte, 256),
	}
	c.add_scraper_ws_client(client)

	go client.write_pump()
	client.read_pump()
	scraper_ws_hub.remove(client)
}

// add_scraper_ws_client queues platform snapshots before later live updates.
func (c *APIClient) add_scraper_ws_client(client *scraper_ws_client) {
	scraper_ws_hub.add(client)
	for _, status := range c.scraper_platform_status_snapshots() {
		status_snapshot := status
		client.enqueue(scraper_ws_message{
			Type:           platform_status_ws_update,
			PlatformStatus: &status_snapshot,
		})
	}
}

func (c *APIClient) scraper_platform_status_snapshots() []events.PlatformStatusChanged {
	descriptors := adapter.StatusDescriptors()
	statuses := make([]events.PlatformStatusChanged, 0, len(descriptors))
	c.platform_status_mu.RLock()
	defer c.platform_status_mu.RUnlock()
	for _, descriptor := range descriptors {
		status, exists := c.platform_statuses[descriptor.Key]
		if !exists {
			status = events.PlatformStatusChanged{
				Platform:  descriptor.Platform,
				Key:       descriptor.Key,
				Name:      descriptor.Name,
				Status:    "unavailable",
				Available: false,
				Reason:    "等待 adapter 状态上报",
			}
		} else {
			if status.Name == "" {
				status.Name = descriptor.Name
			}
			if status.Key == "" {
				status.Key = descriptor.Key
			}
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func (c *scraper_ws_client) enqueue(payload scraper_ws_message) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	c.enqueue_data(data)
}

func (c *scraper_ws_client) enqueue_data(data []byte) {
	c.send_mu.RLock()
	defer c.send_mu.RUnlock()
	if c.send_closed {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *scraper_ws_client) send_reliable(data []byte) {
	c.send_mu.RLock()
	defer c.send_mu.RUnlock()
	if c.send_closed {
		return
	}
	select {
	case c.send <- data:
	case <-time.After(2 * time.Second):
		// GET /api/scraper/job retains every artifact when a stalled
		// connection cannot accept an event within this window.
	}
}

func (c *scraper_ws_client) close_send() {
	c.send_mu.Lock()
	if !c.send_closed {
		c.send_closed = true
		close(c.send)
	}
	c.send_mu.Unlock()
}

func (c *scraper_ws_client) read_pump() {
	defer c.conn.Close()
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *scraper_ws_client) write_pump() {
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
