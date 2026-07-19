package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var platformWorkflowUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var platformWorkflowWS = newPlatformWorkflowHub()

type platformWorkflowHub struct {
	mu      sync.RWMutex
	clients map[*platformWorkflowWSClient]bool
}

type platformWorkflowWSClient struct {
	conn  *websocket.Conn
	send  chan []byte
	runID string
}

func newPlatformWorkflowHub() *platformWorkflowHub {
	return &platformWorkflowHub{clients: make(map[*platformWorkflowWSClient]bool)}
}

func (h *platformWorkflowHub) add(client *platformWorkflowWSClient) {
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()
}

func (h *platformWorkflowHub) remove(client *platformWorkflowWSClient) {
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
	h.mu.Unlock()
}

func (h *platformWorkflowHub) broadcast(runID string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	runID = strings.TrimSpace(runID)
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		if client.runID != "" && client.runID != runID {
			continue
		}
		select {
		case client.send <- data:
		default:
			delete(h.clients, client)
			close(client.send)
		}
	}
}

func (c *APIClient) handlePlatformWorkflowWebsocket(ctx *gin.Context) {
	conn, err := platformWorkflowUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	client := &platformWorkflowWSClient{
		conn:  conn,
		send:  make(chan []byte, 256),
		runID: strings.TrimSpace(firstNonEmpty(ctx.Query("run_id"), ctx.Query("probe_id"))),
	}
	platformWorkflowWS.add(client)
	go client.writePump()
	// if client.runID != "" {
	// 	if run := c.lookupPlatformWorkflow(client.runID); run != nil {
	// 		client.enqueue(platformWorkflowMessage("snapshot", run, ""))
	// 	}
	// }
	client.readPump()
	platformWorkflowWS.remove(client)
}

func (c *platformWorkflowWSClient) enqueue(payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *platformWorkflowWSClient) readPump() {
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
			Type    string `json:"type"`
			RunID   string `json:"run_id"`
			ProbeID string `json:"probe_id"`
		}
		if err := json.Unmarshal(message, &body); err != nil {
			continue
		}
		if strings.EqualFold(body.Type, "subscribe") {
			c.runID = strings.TrimSpace(firstNonEmpty(body.RunID, body.ProbeID))
		}
	}
}

func (c *platformWorkflowWSClient) writePump() {
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

// func platformBroadcastWorkflowRun(run *platformWorkflowRun, event string, nodeID string) {
// 	if run == nil {
// 		return
// 	}
// 	platformWorkflowWS.broadcast(run.ID, platformWorkflowMessage(event, run, nodeID))
// }

// func platformWorkflowMessage(event string, run *platformWorkflowRun, nodeID string) gin.H {
// 	return gin.H{
// 		"type":   "pipeline_workflow",
// 		"event":  event,
// 		"run_id": run.ID,
// 		"data": gin.H{
// 			"workflow": run.snapshot(),
// 			"node":     run.nodeSnapshot(nodeID),
// 		},
// 	}
// }
