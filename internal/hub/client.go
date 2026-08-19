// Package hub connects one downloader instance to the Cloudflare Durable
// Objects task hub. Connections are outbound-only, so peers do not need to be
// reachable from each other or from the public Internet.
package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// Client maintains the Hub WebSocket and provides the Hub HTTP API.
type Client struct {
	config           Config
	executor         Executor
	terminal_handler TerminalHandler
	logger           *zerolog.Logger
	http_client      *http.Client

	state_mu       sync.RWMutex
	connected      bool
	connected_at   int64
	last_error     string
	connection     *websocket.Conn
	active_tasks   map[string]struct{}
	started        bool
	cancel         context.CancelFunc
	stopped        chan struct{}
	ws_write_mu    sync.Mutex
	active_task_mu sync.Mutex
}

// NewClient creates a dormant client. Call Start after adapters are registered.
func NewClient(config Config, executor Executor, terminal_handler TerminalHandler, logger *zerolog.Logger) *Client {
	http_timeout := config.HTTPTimeout
	if http_timeout <= 0 {
		http_timeout = 30 * time.Second
	}
	return &Client{
		config:           config,
		executor:         executor,
		terminal_handler: terminal_handler,
		logger:           logger,
		http_client:      &http.Client{Timeout: http_timeout},
		active_tasks:     make(map[string]struct{}),
	}
}

// Start validates the configuration and starts the reconnect loop.
func (c *Client) Start(parent_context context.Context) error {
	if c == nil || !c.config.Enabled {
		return nil
	}
	if err := c.validate_config(); err != nil {
		return err
	}
	c.state_mu.Lock()
	if c.started {
		c.state_mu.Unlock()
		return nil
	}
	run_context, cancel := context.WithCancel(parent_context)
	c.cancel = cancel
	c.stopped = make(chan struct{})
	c.started = true
	c.state_mu.Unlock()
	go c.run(run_context)
	return nil
}

// Close terminates the connection and waits briefly for the reconnect loop.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.state_mu.Lock()
	cancel := c.cancel
	connection := c.connection
	stopped := c.stopped
	c.started = false
	c.cancel = nil
	c.connection = nil
	c.connected = false
	c.state_mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if connection != nil {
		_ = connection.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "service stopping"),
			time.Now().Add(time.Second),
		)
		_ = connection.Close()
	}
	if stopped != nil {
		select {
		case <-stopped:
		case <-time.After(3 * time.Second):
		}
	}
}

// Status returns a snapshot without exposing the bearer token.
func (c *Client) Status() Status {
	if c == nil {
		return Status{Capabilities: []string{}}
	}
	c.state_mu.RLock()
	defer c.state_mu.RUnlock()
	capabilities := append([]string(nil), c.config.Capabilities...)
	return Status{
		Enabled:      c.config.Enabled,
		Connected:    c.connected,
		HubID:        c.config.HubID,
		ClientID:     c.config.ClientID,
		URL:          c.config.URL,
		Capabilities: capabilities,
		ConnectedAt:  c.connected_at,
		LastError:    c.last_error,
	}
}

// SubmitTask persists and dispatches a task through the Hub.
func (c *Client) SubmitTask(request_context context.Context, request SubmitTaskRequest) (*Task, error) {
	var response task_response
	if err := c.do_json(request_context, http.MethodPost, "/tasks", request, nil, &response); err != nil {
		return nil, err
	}
	return &response.Task, nil
}

// GetTask retrieves durable task state even if a completion push was missed.
func (c *Client) GetTask(request_context context.Context, task_id string) (*Task, error) {
	if strings.TrimSpace(task_id) == "" {
		return nil, errors.New("task id is required")
	}
	var response task_response
	if err := c.do_json(request_context, http.MethodGet, "/tasks/"+url.PathEscape(task_id), nil, nil, &response); err != nil {
		return nil, err
	}
	return &response.Task, nil
}

// ListTasks returns tasks published by this client.
func (c *Client) ListTasks(request_context context.Context, status string, limit int) ([]Task, error) {
	if limit <= 0 {
		limit = 50
	}
	query := url.Values{}
	query.Set("publisher_id", c.config.ClientID)
	query.Set("limit", strconv.Itoa(limit))
	if strings.TrimSpace(status) != "" {
		query.Set("status", strings.TrimSpace(status))
	}
	var response task_list_response
	if err := c.do_json(request_context, http.MethodGet, "/tasks", nil, query, &response); err != nil {
		return nil, err
	}
	return response.Tasks, nil
}

func (c *Client) validate_config() error {
	parsed_url, err := url.Parse(strings.TrimSpace(c.config.URL))
	if err != nil || parsed_url.Host == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
		return errors.New("hub.url must be an http or https URL")
	}
	if strings.TrimSpace(c.config.HubID) == "" {
		return errors.New("hub.id is required")
	}
	if strings.TrimSpace(c.config.ClientID) == "" {
		return errors.New("hub.clientId is required")
	}
	if strings.TrimSpace(c.config.Token) == "" {
		return errors.New("hub.token is required")
	}
	if c.executor == nil && len(c.config.Capabilities) > 0 {
		return errors.New("hub task executor is required when capabilities are enabled")
	}
	return nil
}

func (c *Client) run(run_context context.Context) {
	defer func() {
		c.state_mu.Lock()
		stopped := c.stopped
		c.state_mu.Unlock()
		if stopped != nil {
			close(stopped)
		}
	}()
	backoff := time.Second
	for {
		if run_context.Err() != nil {
			return
		}
		connection, _, err := websocket.DefaultDialer.DialContext(
			run_context,
			c.websocket_url(),
			c.request_headers(),
		)
		if err != nil {
			c.set_disconnected(nil, err)
			if !wait_context(run_context, backoff) {
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}

		backoff = time.Second
		connection.SetReadLimit(max_message_bytes)
		c.set_connected(connection)
		if c.logger != nil {
			c.logger.Info().Str("hub_id", c.config.HubID).Str("client_id", c.config.ClientID).Msg("hub connected")
		}
		err = c.read_messages(run_context, connection)
		_ = connection.Close()
		c.set_disconnected(connection, err)
		if run_context.Err() == nil && !wait_context(run_context, backoff) {
			return
		}
	}
}

func (c *Client) read_messages(run_context context.Context, connection *websocket.Conn) error {
	for {
		_, data, err := connection.ReadMessage()
		if err != nil {
			return err
		}
		var message server_message
		if err := json.Unmarshal(data, &message); err != nil {
			continue
		}
		switch message.Type {
		case "task.assigned":
			if message.Task != nil && message.LeaseToken != "" {
				c.start_task(run_context, *message.Task, message.LeaseToken, message.LeaseMilliseconds)
			}
		case "task.completed", "task.failed":
			if message.Task != nil && c.terminal_handler != nil {
				task := *message.Task
				go c.terminal_handler(task)
			}
		case "error":
			if c.logger != nil {
				c.logger.Warn().Str("error", message.Error).Msg("hub protocol error")
			}
		}
	}
}

func (c *Client) start_task(run_context context.Context, task Task, lease_token string, lease_milliseconds int64) {
	c.active_task_mu.Lock()
	if _, exists := c.active_tasks[task.ID]; exists {
		c.active_task_mu.Unlock()
		return
	}
	c.active_tasks[task.ID] = struct{}{}
	c.active_task_mu.Unlock()

	go func() {
		defer func() {
			c.active_task_mu.Lock()
			delete(c.active_tasks, task.ID)
			c.active_task_mu.Unlock()
		}()
		if err := c.send_message(client_message{
			Type:       "task.accept",
			TaskID:     task.ID,
			LeaseToken: lease_token,
		}); err != nil {
			return
		}

		heartbeat_interval := time.Duration(lease_milliseconds/3) * time.Millisecond
		if heartbeat_interval < 5*time.Second {
			heartbeat_interval = 30 * time.Second
		}
		heartbeat_context, stop_heartbeat := context.WithCancel(run_context)
		defer stop_heartbeat()
		go c.heartbeat_task(heartbeat_context, task.ID, lease_token, heartbeat_interval)

		result, err := c.executor(run_context, task)
		stop_heartbeat()
		if err != nil {
			_ = c.send_message(client_message{
				Type:       "task.fail",
				TaskID:     task.ID,
				LeaseToken: lease_token,
				Error:      err.Error(),
			})
			return
		}
		if len(result) == 0 {
			result = json.RawMessage("null")
		}
		_ = c.send_message(client_message{
			Type:       "task.complete",
			TaskID:     task.ID,
			LeaseToken: lease_token,
			Result:     result,
		})
	}()
}

func (c *Client) heartbeat_task(run_context context.Context, task_id string, lease_token string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-run_context.Done():
			return
		case <-ticker.C:
			_ = c.send_message(client_message{
				Type:       "task.heartbeat",
				TaskID:     task_id,
				LeaseToken: lease_token,
			})
		}
	}
}

func (c *Client) send_message(message client_message) error {
	c.state_mu.RLock()
	connection := c.connection
	c.state_mu.RUnlock()
	if connection == nil {
		return errors.New("hub websocket is disconnected")
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	c.ws_write_mu.Lock()
	defer c.ws_write_mu.Unlock()
	_ = connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return connection.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) set_connected(connection *websocket.Conn) {
	c.state_mu.Lock()
	c.connection = connection
	c.connected = true
	c.connected_at = time.Now().UnixMilli()
	c.last_error = ""
	c.state_mu.Unlock()
}

func (c *Client) set_disconnected(connection *websocket.Conn, err error) {
	c.state_mu.Lock()
	if connection == nil || c.connection == connection {
		c.connection = nil
		c.connected = false
	}
	if err != nil {
		c.last_error = err.Error()
	}
	c.state_mu.Unlock()
	if err != nil && c.logger != nil {
		c.logger.Warn().Err(err).Str("hub_id", c.config.HubID).Msg("hub disconnected")
	}
}

func (c *Client) request_headers() http.Header {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.config.Token)
	headers.Set("X-Hub-Client-ID", c.config.ClientID)
	headers.Set("X-Hub-Capabilities", strings.Join(c.config.Capabilities, ","))
	return headers
}

func (c *Client) websocket_url() string {
	http_url := c.hub_url("/connect", nil)
	parsed_url, _ := url.Parse(http_url)
	if parsed_url.Scheme == "https" {
		parsed_url.Scheme = "wss"
	} else {
		parsed_url.Scheme = "ws"
	}
	return parsed_url.String()
}

func (c *Client) hub_url(path string, query url.Values) string {
	base_url := strings.TrimRight(c.config.URL, "/")
	result := base_url + "/v1/hubs/" + url.PathEscape(c.config.HubID) + path
	if len(query) > 0 {
		result += "?" + query.Encode()
	}
	return result
}

func (c *Client) do_json(
	request_context context.Context,
	method string,
	path string,
	body any,
	query url.Values,
	result any,
) error {
	if c == nil || !c.config.Enabled {
		return errors.New("hub is disabled")
	}
	var body_reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		body_reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(request_context, method, c.hub_url(path, query), body_reader)
	if err != nil {
		return err
	}
	request.Header = c.request_headers()
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http_client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, max_message_bytes))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var hub_error error_response
		if json.Unmarshal(data, &hub_error) == nil && hub_error.Error != "" {
			return fmt.Errorf("hub request failed (%d): %s", response.StatusCode, hub_error.Error)
		}
		return fmt.Errorf("hub request failed (%d): %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	if result == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("decode hub response: %w", err)
	}
	return nil
}

func wait_context(run_context context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-run_context.Done():
		return false
	case <-timer.C:
		return true
	}
}
