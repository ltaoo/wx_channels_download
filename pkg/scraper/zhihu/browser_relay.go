package zhihu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const BrowserWebSocketPath = "/ws/zhihu/browser"

var ErrBrowserUnavailable = errors.New("知乎浏览器抓取通道不可用，请开启代理并在浏览器中打开任意知乎页面")

const browser_response_max_bytes = 32 << 20

// BrowserRequest describes one request that must run inside the user's real
// browser so Zhihu can validate its JavaScript and browser fingerprint.
type BrowserRequest struct {
	URL     string
	Referer string
	Kind    string
	Headers map[string]string
}

// BrowserResponse is returned by the injected browser bridge.
type BrowserResponse struct {
	StatusCode int
	Body       []byte
}

// BrowserFetcher is the browser-backed capability used by Client.
type BrowserFetcher interface {
	Available() bool
	Fetch(context.Context, BrowserRequest) (*BrowserResponse, error)
}

type browser_bridge_message struct {
	Type       string            `json:"type"`
	RequestID  string            `json:"request_id,omitempty"`
	URL        string            `json:"url,omitempty"`
	Referer    string            `json:"referer,omitempty"`
	Kind       string            `json:"kind,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	StatusCode int               `json:"status_code,omitempty"`
	Body       string            `json:"body,omitempty"`
	Error      string            `json:"error,omitempty"`
	Role       string            `json:"role,omitempty"`
	Hostname   string            `json:"hostname,omitempty"`
}

type browser_relay_result struct {
	response *BrowserResponse
	err      error
}

type browser_relay_client struct {
	conn      *websocket.Conn
	write_mu  sync.Mutex
	role      string
	hostname  string
	closed_mu sync.RWMutex
	closed    bool
}

// BrowserRelay coordinates fetch requests with injected Zhihu tabs.
type BrowserRelay struct {
	mu                      sync.RWMutex
	clients                 map[*browser_relay_client]struct{}
	pending                 map[string]chan browser_relay_result
	logger                  *zerolog.Logger
	on_availability_changed func(bool)
	closed                  bool
}

var browser_relay_upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     browser_relay_origin_allowed,
}

// NewBrowserRelay creates a relay for real-browser Zhihu requests.
func NewBrowserRelay(logger *zerolog.Logger) *BrowserRelay {
	return &BrowserRelay{
		clients: make(map[*browser_relay_client]struct{}),
		pending: make(map[string]chan browser_relay_result),
		logger:  logger,
	}
}

// SetAvailabilityCallback installs a callback invoked when the first usable
// browser connects or the last one disconnects.
func (r *BrowserRelay) SetAvailabilityCallback(callback func(bool)) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.on_availability_changed = callback
	available := r.available_locked()
	r.mu.Unlock()
	if callback != nil {
		callback(available)
	}
}

// Available reports whether a top-level Zhihu tab can accept fetch requests.
func (r *BrowserRelay) Available() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	available := r.available_locked()
	r.mu.RUnlock()
	return available
}

func (r *BrowserRelay) available_locked() bool {
	if r.closed {
		return false
	}
	for client := range r.clients {
		if client.role == "top" && !client.is_closed() {
			return true
		}
	}
	return false
}

// HandleWebSocket upgrades one injected Zhihu page to a relay connection.
func (r *BrowserRelay) HandleWebSocket(writer http.ResponseWriter, request *http.Request) {
	if r == nil {
		http.Error(writer, "zhihu browser relay is not initialized", http.StatusServiceUnavailable)
		return
	}
	conn, err := browser_relay_upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	client := &browser_relay_client{conn: conn}
	conn.SetReadLimit(browser_response_max_bytes + (1 << 20))
	r.add_client(client)
	client.read_messages(r)
	r.remove_client(client)
}

// Fetch executes one request in an attached real browser.
func (r *BrowserRelay) Fetch(fetch_context context.Context, request BrowserRequest) (*BrowserResponse, error) {
	if r == nil {
		return nil, ErrBrowserUnavailable
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	raw_url := strings.TrimSpace(request.URL)
	if !browser_request_url_allowed(raw_url) {
		return nil, fmt.Errorf("unsupported zhihu browser URL: %s", raw_url)
	}
	request_id, err := new_browser_request_id()
	if err != nil {
		return nil, fmt.Errorf("create zhihu browser request id: %w", err)
	}
	result_channel := make(chan browser_relay_result, 1)
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, ErrBrowserUnavailable
	}
	client := r.select_client_locked(raw_url)
	if client == nil {
		r.mu.Unlock()
		return nil, ErrBrowserUnavailable
	}
	r.pending[request_id] = result_channel
	r.mu.Unlock()

	message := browser_bridge_message{
		Type:      "fetch",
		RequestID: request_id,
		URL:       raw_url,
		Referer:   strings.TrimSpace(request.Referer),
		Kind:      strings.TrimSpace(request.Kind),
		Headers:   request.Headers,
	}
	if message.Kind == "" {
		message.Kind = "fetch"
	}
	if err := client.write_json(message); err != nil {
		r.remove_pending(request_id)
		r.remove_client(client)
		return nil, ErrBrowserUnavailable
	}

	select {
	case result := <-result_channel:
		return result.response, result.err
	case <-fetch_context.Done():
		r.remove_pending(request_id)
		r.broadcast_complete(request_id)
		return nil, fetch_context.Err()
	}
}

// Close shuts down active browser connections and fails pending requests.
func (r *BrowserRelay) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	was_available := r.available_locked()
	r.closed = true
	clients := make([]*browser_relay_client, 0, len(r.clients))
	for client := range r.clients {
		clients = append(clients, client)
	}
	pending := r.pending
	r.clients = make(map[*browser_relay_client]struct{})
	r.pending = make(map[string]chan browser_relay_result)
	callback := r.on_availability_changed
	r.mu.Unlock()

	for _, client := range clients {
		client.close()
	}
	for _, result_channel := range pending {
		select {
		case result_channel <- browser_relay_result{err: ErrBrowserUnavailable}:
		default:
		}
	}
	if was_available && callback != nil {
		callback(false)
	}
}

func (r *BrowserRelay) add_client(client *browser_relay_client) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		client.close()
		return
	}
	r.clients[client] = struct{}{}
	r.mu.Unlock()
}

func (r *BrowserRelay) remove_client(client *browser_relay_client) {
	if r == nil || client == nil {
		return
	}
	r.mu.Lock()
	was_available := r.available_locked()
	delete(r.clients, client)
	available := r.available_locked()
	callback := r.on_availability_changed
	r.mu.Unlock()
	client.close()
	if was_available && !available && callback != nil {
		callback(false)
	}
}

func (r *BrowserRelay) select_client_locked(raw_url string) *browser_relay_client {
	parsed_url, _ := url.Parse(raw_url)
	target_hostname := strings.ToLower(parsed_url.Hostname())
	var fallback_client *browser_relay_client
	for client := range r.clients {
		if client.role != "top" || client.is_closed() {
			continue
		}
		if fallback_client == nil {
			fallback_client = client
		}
		if strings.EqualFold(client.hostname, target_hostname) {
			return client
		}
	}
	return fallback_client
}

func (r *BrowserRelay) handle_message(client *browser_relay_client, message browser_bridge_message) {
	switch message.Type {
	case "hello":
		r.mu.Lock()
		was_available := r.available_locked()
		client.role = strings.ToLower(strings.TrimSpace(message.Role))
		client.hostname = strings.ToLower(strings.TrimSpace(message.Hostname))
		if client.role != "top" && client.role != "child" {
			client.role = "child"
		}
		available := r.available_locked()
		callback := r.on_availability_changed
		r.mu.Unlock()
		if r.logger != nil {
			r.logger.Info().
				Str("component", "zhihu_browser_relay").
				Str("role", client.role).
				Str("hostname", client.hostname).
				Msg("zhihu browser connected")
		}
		if !was_available && available && callback != nil {
			callback(true)
		}
	case "result":
		body := []byte(message.Body)
		if len(body) > browser_response_max_bytes {
			r.complete_pending(message.RequestID, browser_relay_result{err: fmt.Errorf("zhihu browser response exceeds %d bytes", browser_response_max_bytes)})
			return
		}
		r.complete_pending(message.RequestID, browser_relay_result{response: &BrowserResponse{
			StatusCode: message.StatusCode,
			Body:       body,
		}})
	case "error":
		message_text := strings.TrimSpace(message.Error)
		if message_text == "" {
			message_text = "知乎浏览器抓取失败"
		}
		r.complete_pending(message.RequestID, browser_relay_result{err: errors.New(message_text)})
	}
}

func (r *BrowserRelay) complete_pending(request_id string, result browser_relay_result) {
	request_id = strings.TrimSpace(request_id)
	if request_id == "" {
		return
	}
	r.mu.Lock()
	result_channel := r.pending[request_id]
	delete(r.pending, request_id)
	r.mu.Unlock()
	if result_channel != nil {
		select {
		case result_channel <- result:
		default:
		}
	}
	r.broadcast_complete(request_id)
}

func (r *BrowserRelay) remove_pending(request_id string) {
	r.mu.Lock()
	delete(r.pending, request_id)
	r.mu.Unlock()
}

func (r *BrowserRelay) broadcast_complete(request_id string) {
	r.mu.RLock()
	clients := make([]*browser_relay_client, 0, len(r.clients))
	for client := range r.clients {
		if client.role == "top" && !client.is_closed() {
			clients = append(clients, client)
		}
	}
	r.mu.RUnlock()
	message := browser_bridge_message{Type: "complete", RequestID: request_id}
	for _, client := range clients {
		_ = client.write_json(message)
	}
}

func (c *browser_relay_client) read_messages(relay *BrowserRelay) {
	for {
		var message browser_bridge_message
		if err := c.conn.ReadJSON(&message); err != nil {
			return
		}
		relay.handle_message(c, message)
	}
}

func (c *browser_relay_client) write_json(message browser_bridge_message) error {
	if c == nil || c.is_closed() {
		return ErrBrowserUnavailable
	}
	c.write_mu.Lock()
	defer c.write_mu.Unlock()
	if c.is_closed() {
		return ErrBrowserUnavailable
	}
	return c.conn.WriteJSON(message)
}

func (c *browser_relay_client) is_closed() bool {
	if c == nil {
		return true
	}
	c.closed_mu.RLock()
	closed := c.closed
	c.closed_mu.RUnlock()
	return closed
}

func (c *browser_relay_client) close() {
	if c == nil {
		return
	}
	c.closed_mu.Lock()
	if c.closed {
		c.closed_mu.Unlock()
		return
	}
	c.closed = true
	c.closed_mu.Unlock()
	_ = c.conn.Close()
}

func browser_relay_origin_allowed(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	parsed_origin, err := url.Parse(origin)
	if err != nil {
		return false
	}
	hostname := strings.ToLower(parsed_origin.Hostname())
	return hostname == "www.zhihu.com" || hostname == "zhuanlan.zhihu.com"
}

func browser_request_url_allowed(raw_url string) bool {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Scheme != "https" {
		return false
	}
	hostname := strings.ToLower(parsed_url.Hostname())
	return hostname == "www.zhihu.com" || hostname == "zhuanlan.zhihu.com"
}

func new_browser_request_id() (string, error) {
	random_bytes := make([]byte, 16)
	if _, err := rand.Read(random_bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(random_bytes), nil
}
