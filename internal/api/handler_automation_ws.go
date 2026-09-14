package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/internal/events"
	"wx_channel/pkg/flowengine"
)

const (
	automation_run_started_ws_update   = "automation_run_started"
	automation_run_completed_ws_update = "automation_run_completed"
	automation_run_failed_ws_update    = "automation_run_failed"
	automation_run_waiting_ws_update   = "automation_run_waiting"
	automation_node_status_ws_update   = "automation_node_status"
	automation_node_log_ws_update      = "automation_node_log"
)

type automation_ws_message struct {
	Type         string                          `json:"type"`
	FlowID       string                          `json:"flow_id"`
	RunID        string                          `json:"run_id"`
	ScheduleID   string                          `json:"schedule_id,omitempty"`
	Trigger      string                          `json:"trigger,omitempty"`
	RunStatus    string                          `json:"run_status,omitempty"`
	Error        string                          `json:"error,omitempty"`
	NodeStatus   *flowengine.NodeExecutionStatus `json:"node_status,omitempty"`
	ExecutionLog *flowengine.NodeExecutionLog    `json:"execution_log,omitempty"`
}

var automation_ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}

var automation_ws_bridge = new_automation_ws_pool()

// BroadcastAutomationEvent projects automation service events to connected
// flow editor clients.
func BroadcastAutomationEvent(event events.Event) {
	message, ok := automation_ws_message_from_event(event)
	if !ok {
		return
	}
	automation_ws_bridge.broadcast(message)
}

func automation_ws_message_from_event(event events.Event) (automation_ws_message, bool) {
	switch typed_event := event.(type) {
	case events.AutomationRunStarted:
		return automation_ws_message{
			Type:       automation_run_started_ws_update,
			FlowID:     typed_event.FlowID,
			RunID:      typed_event.RunID,
			ScheduleID: typed_event.ScheduleID,
			Trigger:    typed_event.Trigger,
			RunStatus:  "RUNNING",
		}, true
	case events.AutomationRunCompleted:
		return automation_ws_message{
			Type:       automation_run_completed_ws_update,
			FlowID:     typed_event.FlowID,
			RunID:      typed_event.RunID,
			ScheduleID: typed_event.ScheduleID,
			Trigger:    typed_event.Trigger,
			RunStatus:  "COMPLETED",
		}, true
	case events.AutomationRunWaiting:
		return automation_ws_message{
			Type:       automation_run_waiting_ws_update,
			FlowID:     typed_event.FlowID,
			RunID:      typed_event.RunID,
			ScheduleID: typed_event.ScheduleID,
			Trigger:    typed_event.Trigger,
			RunStatus:  "WAITING",
		}, true
	case events.AutomationRunFailed:
		run_status := typed_event.Status
		if run_status == "" {
			run_status = "FAILED"
		}
		return automation_ws_message{
			Type:       automation_run_failed_ws_update,
			FlowID:     typed_event.FlowID,
			RunID:      typed_event.RunID,
			ScheduleID: typed_event.ScheduleID,
			Trigger:    typed_event.Trigger,
			RunStatus:  run_status,
			Error:      typed_event.Error,
		}, true
	case events.AutomationNodeStatusChanged:
		status := typed_event.Node
		return automation_ws_message{
			Type:       automation_node_status_ws_update,
			FlowID:     status.FlowID,
			RunID:      status.RunID,
			NodeStatus: &status,
		}, true
	case events.AutomationNodeLogCreated:
		entry := typed_event.Log
		return automation_ws_message{
			Type:         automation_node_log_ws_update,
			FlowID:       entry.FlowID,
			RunID:        entry.RunID,
			ExecutionLog: &entry,
		}, true
	default:
		return automation_ws_message{}, false
	}
}

type automation_ws_pool struct {
	mu      sync.RWMutex
	clients map[*automation_ws_client]struct{}
}

func new_automation_ws_pool() *automation_ws_pool {
	return &automation_ws_pool{clients: make(map[*automation_ws_client]struct{})}
}

func (pool *automation_ws_pool) add(client *automation_ws_client) {
	if client == nil {
		return
	}
	pool.mu.Lock()
	pool.clients[client] = struct{}{}
	pool.mu.Unlock()
}

func (pool *automation_ws_pool) remove(client *automation_ws_client) {
	if client == nil {
		return
	}
	removed := false
	pool.mu.Lock()
	if _, exists := pool.clients[client]; exists {
		delete(pool.clients, client)
		removed = true
	}
	pool.mu.Unlock()
	if removed {
		client.close_send()
	}
}

func (pool *automation_ws_pool) broadcast(message automation_ws_message) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}
	pool.mu.RLock()
	clients := make([]*automation_ws_client, 0, len(pool.clients))
	for client := range pool.clients {
		clients = append(clients, client)
	}
	pool.mu.RUnlock()
	for _, client := range clients {
		if client.matches(message.FlowID, message.RunID) {
			client.enqueue(data)
		}
	}
}

type automation_ws_client struct {
	conn        *websocket.Conn
	send        chan []byte
	flow_id     string
	run_id      string
	send_mu     sync.RWMutex
	send_closed bool
}

func (client *automation_ws_client) matches(flow_id string, run_id string) bool {
	return (client.flow_id == "" || client.flow_id == flow_id) &&
		(client.run_id == "" || client.run_id == run_id)
}

func (client *automation_ws_client) enqueue(data []byte) {
	client.send_mu.RLock()
	defer client.send_mu.RUnlock()
	if client.send_closed {
		return
	}
	select {
	case client.send <- data:
	default:
		// REST run detail remains the recovery source if a slow client falls
		// behind the bounded real-time queue.
	}
}

func (client *automation_ws_client) close_send() {
	client.send_mu.Lock()
	if !client.send_closed {
		client.send_closed = true
		close(client.send)
	}
	client.send_mu.Unlock()
}

// handle_automation_ws streams flow/run/node execution changes. Optional
// flow_id and run_id query parameters reduce traffic for external clients.
func (c *APIClient) handle_automation_ws(ctx *gin.Context) {
	conn, err := automation_ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	client := &automation_ws_client{
		conn:    conn,
		send:    make(chan []byte, 256),
		flow_id: strings.TrimSpace(ctx.Query("flow_id")),
		run_id:  strings.TrimSpace(ctx.Query("run_id")),
	}
	automation_ws_bridge.add(client)
	go client.write_pump()
	client.read_pump()
	automation_ws_bridge.remove(client)
}

func (client *automation_ws_client) read_pump() {
	defer client.conn.Close()
	client.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})
	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (client *automation_ws_client) write_pump() {
	ticker := time.NewTicker(10 * time.Second)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()
	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = client.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
