package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const channels_profile_agent_path = "/ws/channels/profile-agent"
const channels_profile_agent_fetch_profile = "fetch_profile"

var err_profile_agent_unavailable = errors.New("profile agent is unavailable")

type channels_profile_agent_request struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	OID        string `json:"oid"`
	UID        string `json:"uid,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`
}

type channels_profile_agent_response struct {
	ID    string          `json:"id"`
	Type  string          `json:"type,omitempty"`
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

type channels_profile_agent_pending struct {
	client *channels_profile_agent_conn
	ch     chan channels_profile_agent_response
}

type channels_profile_agent_hub struct {
	token    string
	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[*channels_profile_agent_conn]bool
	pending map[string]channels_profile_agent_pending
	req_seq uint64
}

type channels_profile_agent_conn struct {
	hub          *channels_profile_agent_hub
	conn         *websocket.Conn
	send         chan []byte
	remote_addr  string
	connected_at time.Time
}

type channels_profile_agent_runner struct {
	server_url     string
	local_base_url string
	token          string
	http_client    *http.Client
}

func default_profile_agent_token() string {
	return strings.TrimSpace(os.Getenv("WX_CHANNEL_PROFILE_AGENT_TOKEN"))
}

func new_channels_profile_agent_hub(token string) *channels_profile_agent_hub {
	return &channels_profile_agent_hub{
		token: strings.TrimSpace(token),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		clients: make(map[*channels_profile_agent_conn]bool),
		pending: make(map[string]channels_profile_agent_pending),
		req_seq: uint64(time.Now().UnixNano()),
	}
}

func (h *channels_profile_agent_hub) HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		write_profile_agent_http_error(w, http.StatusServiceUnavailable, "profile agent hub is not initialized")
		return
	}
	if r.Method != http.MethodGet {
		write_profile_agent_http_error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.authorized(r) {
		write_profile_agent_http_error(w, http.StatusUnauthorized, "unauthorized profile agent")
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &channels_profile_agent_conn{
		hub:          h,
		conn:         conn,
		send:         make(chan []byte, 64),
		remote_addr:  r.RemoteAddr,
		connected_at: time.Now(),
	}
	h.add_client(client)
	log.Printf("profile agent connected: %s", client.remote_addr)
	go client.write_pump()
	client.read_pump()
}

func write_profile_agent_http_error(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api_response{Code: status, Msg: msg})
}

func (h *channels_profile_agent_hub) authorized(r *http.Request) bool {
	if h.token == "" {
		return true
	}
	got := strings.TrimSpace(r.URL.Query().Get("token"))
	if got == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			got = strings.TrimSpace(auth[len("Bearer "):])
		}
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.token)) == 1
}

func (h *channels_profile_agent_hub) add_client(client *channels_profile_agent_conn) {
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()
}

func (h *channels_profile_agent_hub) remove_client(client *channels_profile_agent_conn) {
	failed := make([]channels_profile_agent_pending, 0)
	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
	for id, pending := range h.pending {
		if pending.client == client {
			delete(h.pending, id)
			failed = append(failed, pending)
		}
	}
	h.mu.Unlock()

	for _, pending := range failed {
		select {
		case pending.ch <- channels_profile_agent_response{
			OK:    false,
			Error: "profile agent disconnected",
		}:
		default:
		}
	}
	log.Printf("profile agent disconnected: %s", client.remote_addr)
}

func (h *channels_profile_agent_hub) Available() bool {
	return h.ClientCount() > 0
}

func (h *channels_profile_agent_hub) ClientCount() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *channels_profile_agent_hub) AuthRequired() bool {
	return h != nil && h.token != ""
}

func (h *channels_profile_agent_hub) FetchProfile(ctx context.Context, profile_url string, oid string, uid string) (*channels_feed_profile, error) {
	if h == nil {
		return nil, err_profile_agent_unavailable
	}
	oid = strings.TrimSpace(oid)
	uid = clean_nonce_id(uid)
	if oid == "" {
		return nil, errors.New("missing oid")
	}

	id := strconv.FormatUint(atomic.AddUint64(&h.req_seq, 1), 10)
	req := channels_profile_agent_request{
		ID:         id,
		Type:       channels_profile_agent_fetch_profile,
		OID:        oid,
		UID:        uid,
		ProfileURL: profile_url,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp_ch := make(chan channels_profile_agent_response, 1)
	var send_err error
	h.mu.Lock()
	client := h.pick_client_locked()
	if client == nil {
		h.mu.Unlock()
		return nil, err_profile_agent_unavailable
	}
	h.pending[id] = channels_profile_agent_pending{client: client, ch: resp_ch}
	select {
	case client.send <- data:
	default:
		delete(h.pending, id)
		send_err = errors.New("profile agent send buffer is full")
	}
	h.mu.Unlock()
	if send_err != nil {
		return nil, send_err
	}

	cleanup := func() {
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
	}

	select {
	case resp := <-resp_ch:
		if !resp.OK {
			return nil, errors.New(first_non_empty(resp.Error, "profile agent returned an error"))
		}
		if len(resp.Data) == 0 || string(resp.Data) == "null" {
			return nil, errors.New("profile agent returned empty data")
		}
		var profile channels_feed_profile
		if err := json.Unmarshal(resp.Data, &profile); err != nil {
			return nil, fmt.Errorf("parse profile agent data: %w", err)
		}
		if err := validate_channels_feed_profile(&profile); err != nil {
			return nil, err
		}
		return &profile, nil
	case <-ctx.Done():
		cleanup()
		return nil, fmt.Errorf("profile agent request timed out: %w", ctx.Err())
	}
}

func (h *channels_profile_agent_hub) pick_client_locked() *channels_profile_agent_conn {
	for client := range h.clients {
		return client
	}
	return nil
}

func (h *channels_profile_agent_hub) deliver_response(client *channels_profile_agent_conn, resp channels_profile_agent_response) {
	if resp.ID == "" {
		return
	}
	h.mu.Lock()
	pending, ok := h.pending[resp.ID]
	if ok && pending.client == client {
		delete(h.pending, resp.ID)
	} else {
		ok = false
	}
	h.mu.Unlock()
	if !ok {
		return
	}
	select {
	case pending.ch <- resp:
	default:
	}
}

func (c *channels_profile_agent_conn) read_pump() {
	defer func() {
		c.hub.remove_client(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(16 << 20)
	_ = c.conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var resp channels_profile_agent_response
		if err := json.Unmarshal(message, &resp); err != nil {
			continue
		}
		c.hub.deliver_response(c, resp)
	}
}

func (c *channels_profile_agent_conn) write_pump() {
	ticker := time.NewTicker(20 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *migration_server) fetch_profile_via_agent(timeout time.Duration, profile_url string, oid string, uid string) (*channels_feed_profile, error) {
	if s == nil || s.profile_agent == nil {
		return nil, err_profile_agent_unavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.profile_agent.FetchProfile(ctx, profile_url, oid, uid)
}

func run_channels_profile_agent(raw_server_url string, local_base_url string, token string) error {
	server_url, err := normalize_channels_profile_agent_url(raw_server_url)
	if err != nil {
		return err
	}
	local_base_url = normalize_target_base_url(first_non_empty(local_base_url, default_target_base_url))
	agent := &channels_profile_agent_runner{
		server_url:     server_url,
		local_base_url: local_base_url,
		token:          strings.TrimSpace(token),
		http_client:    &http.Client{Timeout: 30 * time.Second},
	}

	log.Printf("profile agent using local API: %s", local_base_url)
	retry_delay := time.Second
	for {
		if err := agent.run_once(); err != nil {
			log.Printf("profile agent connection ended: %v; reconnecting in %s", err, retry_delay)
		}
		time.Sleep(retry_delay)
		if retry_delay < 30*time.Second {
			retry_delay *= 2
			if retry_delay > 30*time.Second {
				retry_delay = 30 * time.Second
			}
		}
	}
}

func (a *channels_profile_agent_runner) run_once() error {
	headers := http.Header{}
	if a.token != "" {
		headers.Set("Authorization", "Bearer "+a.token)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(a.server_url, headers)
	if err != nil {
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			if text := strings.TrimSpace(string(body)); text != "" {
				return fmt.Errorf("dial %s: %w: %s", a.server_url, err, text)
			}
		}
		return fmt.Errorf("dial %s: %w", a.server_url, err)
	}
	defer conn.Close()

	log.Printf("profile agent connected to %s", a.server_url)
	conn.SetReadLimit(1 << 20)
	for {
		var req channels_profile_agent_request
		if err := conn.ReadJSON(&req); err != nil {
			return err
		}
		resp := a.handle_request(req)
		if err := conn.WriteJSON(resp); err != nil {
			return err
		}
	}
}

func (a *channels_profile_agent_runner) handle_request(req channels_profile_agent_request) channels_profile_agent_response {
	resp := channels_profile_agent_response{
		ID:   req.ID,
		Type: req.Type,
	}
	if req.ID == "" {
		resp.Error = "missing request id"
		return resp
	}
	if req.Type != channels_profile_agent_fetch_profile {
		resp.Error = "unsupported request type: " + req.Type
		return resp
	}
	oid := strings.TrimSpace(req.OID)
	if oid == "" {
		resp.Error = "missing oid"
		return resp
	}
	profile_url := build_target_profile_url(a.local_base_url, oid, req.UID).String()
	data, err := fetch_target_profile_data(a.http_client, profile_url)
	if err != nil {
		resp.Error = err.Error()
		return resp
	}
	resp.OK = true
	resp.Data = data
	return resp
}

func normalize_channels_profile_agent_url(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("missing -profile-agent-url")
	}
	if !strings.Contains(raw, "://") {
		raw = "wss://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported profile agent URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", errors.New("profile agent URL host is empty")
	}
	if strings.Trim(parsed.Path, "/") == "" {
		parsed.Path = channels_profile_agent_path
	}
	return parsed.String(), nil
}

func fetch_target_profile_data(client *http.Client, raw_url string) (json.RawMessage, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Get(raw_url)
	if err != nil {
		return nil, fmt.Errorf("fetch profile: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read profile response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("profile http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var envelope target_api_envelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse profile response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("profile api error code=%d: %s", envelope.Code, envelope.Msg)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, errors.New("profile data is empty")
	}
	var profile channels_feed_profile
	if err := json.Unmarshal(envelope.Data, &profile); err != nil {
		return nil, fmt.Errorf("parse profile data: %w", err)
	}
	if err := validate_channels_feed_profile(&profile); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), envelope.Data...), nil
}

func validate_channels_feed_profile(profile *channels_feed_profile) error {
	if profile == nil {
		return errors.New("profile is nil")
	}
	if profile.ErrCode != 0 {
		return fmt.Errorf("profile business error code=%d: %s", profile.ErrCode, profile.ErrMsg)
	}
	object_data := normalize_raw_json_object(profile.Data.Object)
	if len(object_data) == 0 || strings.TrimSpace(string(object_data)) == "null" {
		return errors.New("profile object is empty")
	}
	var object_value struct {
		ID flexible_string `json:"id"`
	}
	if err := json.Unmarshal(object_data, &object_value); err != nil {
		return fmt.Errorf("parse profile object: %w", err)
	}
	if strings.TrimSpace(string(object_value.ID)) == "" {
		return errors.New("profile object id is empty")
	}
	return nil
}
