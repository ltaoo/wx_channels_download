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

const channelsProfileAgentPath = "/ws/channels/profile-agent"
const channelsProfileAgentFetchProfile = "fetch_profile"

var errProfileAgentUnavailable = errors.New("profile agent is unavailable")

type channelsProfileAgentRequest struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	OID        string `json:"oid"`
	UID        string `json:"uid,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`
}

type channelsProfileAgentResponse struct {
	ID    string          `json:"id"`
	Type  string          `json:"type,omitempty"`
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

type channelsProfileAgentPending struct {
	client *channelsProfileAgentConn
	ch     chan channelsProfileAgentResponse
}

type channelsProfileAgentHub struct {
	token    string
	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[*channelsProfileAgentConn]bool
	pending map[string]channelsProfileAgentPending
	reqSeq  uint64
}

type channelsProfileAgentConn struct {
	hub         *channelsProfileAgentHub
	conn        *websocket.Conn
	send        chan []byte
	remoteAddr  string
	connectedAt time.Time
}

type channelsProfileAgentRunner struct {
	serverURL    string
	localBaseURL string
	token        string
	httpClient   *http.Client
}

func defaultProfileAgentToken() string {
	return strings.TrimSpace(os.Getenv("WX_CHANNEL_PROFILE_AGENT_TOKEN"))
}

func newChannelsProfileAgentHub(token string) *channelsProfileAgentHub {
	return &channelsProfileAgentHub{
		token: strings.TrimSpace(token),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		clients: make(map[*channelsProfileAgentConn]bool),
		pending: make(map[string]channelsProfileAgentPending),
		reqSeq:  uint64(time.Now().UnixNano()),
	}
}

func (h *channelsProfileAgentHub) HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		writeProfileAgentHTTPError(w, http.StatusServiceUnavailable, "profile agent hub is not initialized")
		return
	}
	if r.Method != http.MethodGet {
		writeProfileAgentHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.authorized(r) {
		writeProfileAgentHTTPError(w, http.StatusUnauthorized, "unauthorized profile agent")
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &channelsProfileAgentConn{
		hub:         h,
		conn:        conn,
		send:        make(chan []byte, 64),
		remoteAddr:  r.RemoteAddr,
		connectedAt: time.Now(),
	}
	h.addClient(client)
	log.Printf("profile agent connected: %s", client.remoteAddr)
	go client.writePump()
	client.readPump()
}

func writeProfileAgentHTTPError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Code: status, Msg: msg})
}

func (h *channelsProfileAgentHub) authorized(r *http.Request) bool {
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

func (h *channelsProfileAgentHub) addClient(client *channelsProfileAgentConn) {
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()
}

func (h *channelsProfileAgentHub) removeClient(client *channelsProfileAgentConn) {
	failed := make([]channelsProfileAgentPending, 0)
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
		case pending.ch <- channelsProfileAgentResponse{
			OK:    false,
			Error: "profile agent disconnected",
		}:
		default:
		}
	}
	log.Printf("profile agent disconnected: %s", client.remoteAddr)
}

func (h *channelsProfileAgentHub) Available() bool {
	return h.ClientCount() > 0
}

func (h *channelsProfileAgentHub) ClientCount() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *channelsProfileAgentHub) AuthRequired() bool {
	return h != nil && h.token != ""
}

func (h *channelsProfileAgentHub) FetchProfile(ctx context.Context, profileURL string, oid string, uid string) (*channelsFeedProfile, error) {
	if h == nil {
		return nil, errProfileAgentUnavailable
	}
	oid = strings.TrimSpace(oid)
	uid = cleanNonceID(uid)
	if oid == "" {
		return nil, errors.New("missing oid")
	}

	id := strconv.FormatUint(atomic.AddUint64(&h.reqSeq, 1), 10)
	req := channelsProfileAgentRequest{
		ID:         id,
		Type:       channelsProfileAgentFetchProfile,
		OID:        oid,
		UID:        uid,
		ProfileURL: profileURL,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	respCh := make(chan channelsProfileAgentResponse, 1)
	var sendErr error
	h.mu.Lock()
	client := h.pickClientLocked()
	if client == nil {
		h.mu.Unlock()
		return nil, errProfileAgentUnavailable
	}
	h.pending[id] = channelsProfileAgentPending{client: client, ch: respCh}
	select {
	case client.send <- data:
	default:
		delete(h.pending, id)
		sendErr = errors.New("profile agent send buffer is full")
	}
	h.mu.Unlock()
	if sendErr != nil {
		return nil, sendErr
	}

	cleanup := func() {
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
	}

	select {
	case resp := <-respCh:
		if !resp.OK {
			return nil, errors.New(firstNonEmpty(resp.Error, "profile agent returned an error"))
		}
		if len(resp.Data) == 0 || string(resp.Data) == "null" {
			return nil, errors.New("profile agent returned empty data")
		}
		var profile channelsFeedProfile
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

func (h *channelsProfileAgentHub) pickClientLocked() *channelsProfileAgentConn {
	for client := range h.clients {
		return client
	}
	return nil
}

func (h *channelsProfileAgentHub) deliverResponse(client *channelsProfileAgentConn, resp channelsProfileAgentResponse) {
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

func (c *channelsProfileAgentConn) readPump() {
	defer func() {
		c.hub.removeClient(c)
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
		var resp channelsProfileAgentResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			continue
		}
		c.hub.deliverResponse(c, resp)
	}
}

func (c *channelsProfileAgentConn) writePump() {
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

func (s *migrationServer) fetchProfileViaAgent(timeout time.Duration, profileURL string, oid string, uid string) (*channelsFeedProfile, error) {
	if s == nil || s.profileAgent == nil {
		return nil, errProfileAgentUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.profileAgent.FetchProfile(ctx, profileURL, oid, uid)
}

func runChannelsProfileAgent(rawServerURL string, localBaseURL string, token string) error {
	serverURL, err := normalizeChannelsProfileAgentURL(rawServerURL)
	if err != nil {
		return err
	}
	localBaseURL = normalizeTargetBaseURL(firstNonEmpty(localBaseURL, defaultTargetBaseURL))
	agent := &channelsProfileAgentRunner{
		serverURL:    serverURL,
		localBaseURL: localBaseURL,
		token:        strings.TrimSpace(token),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}

	log.Printf("profile agent using local API: %s", localBaseURL)
	retryDelay := time.Second
	for {
		if err := agent.runOnce(); err != nil {
			log.Printf("profile agent connection ended: %v; reconnecting in %s", err, retryDelay)
		}
		time.Sleep(retryDelay)
		if retryDelay < 30*time.Second {
			retryDelay *= 2
			if retryDelay > 30*time.Second {
				retryDelay = 30 * time.Second
			}
		}
	}
}

func (a *channelsProfileAgentRunner) runOnce() error {
	headers := http.Header{}
	if a.token != "" {
		headers.Set("Authorization", "Bearer "+a.token)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(a.serverURL, headers)
	if err != nil {
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			if text := strings.TrimSpace(string(body)); text != "" {
				return fmt.Errorf("dial %s: %w: %s", a.serverURL, err, text)
			}
		}
		return fmt.Errorf("dial %s: %w", a.serverURL, err)
	}
	defer conn.Close()

	log.Printf("profile agent connected to %s", a.serverURL)
	conn.SetReadLimit(1 << 20)
	for {
		var req channelsProfileAgentRequest
		if err := conn.ReadJSON(&req); err != nil {
			return err
		}
		resp := a.handleRequest(req)
		if err := conn.WriteJSON(resp); err != nil {
			return err
		}
	}
}

func (a *channelsProfileAgentRunner) handleRequest(req channelsProfileAgentRequest) channelsProfileAgentResponse {
	resp := channelsProfileAgentResponse{
		ID:   req.ID,
		Type: req.Type,
	}
	if req.ID == "" {
		resp.Error = "missing request id"
		return resp
	}
	if req.Type != channelsProfileAgentFetchProfile {
		resp.Error = "unsupported request type: " + req.Type
		return resp
	}
	oid := strings.TrimSpace(req.OID)
	if oid == "" {
		resp.Error = "missing oid"
		return resp
	}
	profileURL := buildTargetProfileURL(a.localBaseURL, oid, req.UID).String()
	data, err := fetchTargetProfileData(a.httpClient, profileURL)
	if err != nil {
		resp.Error = err.Error()
		return resp
	}
	resp.OK = true
	resp.Data = data
	return resp
}

func normalizeChannelsProfileAgentURL(raw string) (string, error) {
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
		parsed.Path = channelsProfileAgentPath
	}
	return parsed.String(), nil
}

func fetchTargetProfileData(client *http.Client, rawURL string) (json.RawMessage, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Get(rawURL)
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
	var envelope targetAPIEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse profile response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("profile api error code=%d: %s", envelope.Code, envelope.Msg)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, errors.New("profile data is empty")
	}
	var profile channelsFeedProfile
	if err := json.Unmarshal(envelope.Data, &profile); err != nil {
		return nil, fmt.Errorf("parse profile data: %w", err)
	}
	if err := validate_channels_feed_profile(&profile); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), envelope.Data...), nil
}

func validate_channels_feed_profile(profile *channelsFeedProfile) error {
	if profile == nil {
		return errors.New("profile is nil")
	}
	if profile.ErrCode != 0 {
		return fmt.Errorf("profile business error code=%d: %s", profile.ErrCode, profile.ErrMsg)
	}
	object_data := normalizeRawJSONObject(profile.Data.Object)
	if len(object_data) == 0 || strings.TrimSpace(string(object_data)) == "null" {
		return errors.New("profile object is empty")
	}
	var object_value struct {
		ID flexibleString `json:"id"`
	}
	if err := json.Unmarshal(object_data, &object_value); err != nil {
		return fmt.Errorf("parse profile object: %w", err)
	}
	if strings.TrimSpace(string(object_value.ID)) == "" {
		return errors.New("profile object id is empty")
	}
	return nil
}
