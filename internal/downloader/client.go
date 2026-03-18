package downloader

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSClient struct {
	Conn *websocket.Conn
	Send chan []byte
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(5 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type DownloaderClient struct {
	ws_clients  map[*WSClient]bool
	ws_mu       sync.RWMutex
	OnConnected func(client *WSClient)
	OnMessage   func(client *WSClient, message []byte)
}

func NewDownloaderClient() *DownloaderClient {
	return &DownloaderClient{
		ws_clients: make(map[*WSClient]bool),
	}
}

func (c *DownloaderClient) HandleDownloaderWebsocket(ctx *gin.Context) {
	conn, err := wsUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	client := &WSClient{Conn: conn, Send: make(chan []byte, 256)}
	c.ws_clients[client] = true
	c.ws_mu.Unlock()

	go client.writePump()

	if c.OnConnected != nil {
		c.OnConnected(client)
	}

	defer func() {
		c.ws_mu.Lock()
		if _, ok := c.ws_clients[client]; ok {
			delete(c.ws_clients, client)
			close(client.Send)
		}
		c.ws_mu.Unlock()
		conn.Close()
	}()

	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if c.OnMessage != nil {
			c.OnMessage(client, message)
		}
	}
}

func (c *DownloaderClient) Stop() {
	c.ws_mu.Lock()
	for client := range c.ws_clients {
		close(client.Send)
		delete(c.ws_clients, client)
	}
	c.ws_mu.Unlock()
}

func (c *DownloaderClient) Broadcast(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	defer c.ws_mu.Unlock()
	for client := range c.ws_clients {
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(c.ws_clients, client)
		}
	}
}
