package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"wx_channel/pkg/browsermgr"
)

type sandboxTicket struct {
	SandboxID string
	ExpiresAt time.Time
}

var adminBrowserCDPTickets sync.Map

var adminBrowserWSUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (s *AdminServer) handleSandboxes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if s.browserMgr == nil {
			s.writeOK(w, []any{})
			return
		}
		s.writeOK(w, s.browserMgr.List())
	case http.MethodPost:
		if s.browserMgr == nil {
			s.writeError(w, http.StatusInternalServerError, "browser manager not initialized")
			return
		}
		var req browsermgr.CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !strings.Contains(err.Error(), "EOF") {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		rec, err := s.browserMgr.Create(r.Context(), req)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.writeOK(w, rec)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *AdminServer) handleSandbox(w http.ResponseWriter, r *http.Request) {
	if strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/sandboxes/"), "/") == "lease" {
		s.handleLeaseSandbox(w, r)
		return
	}
	id, rest := sandboxPath(r.URL.Path)
	if id == "" {
		s.writeError(w, http.StatusNotFound, "browser not found")
		return
	}
	switch {
	case rest == "":
		s.handleSandboxRecord(w, r, id)
	case rest == "release":
		s.handleReleaseSandbox(w, r, id)
	case rest == "pause":
		s.handlePauseSandbox(w, r, id)
	case rest == "resume":
		s.handleResumeSandbox(w, r, id)
	case rest == "browser/restart":
		s.handleRestartSandboxBrowser(w, r, id)
	case rest == "browser/screenshot":
		s.handleSandboxBrowserScreenshot(w, r, id)
	case rest == "browser/actions":
		s.handleSandboxBrowserActions(w, r, id)
	case rest == "browser/content":
		s.handleSandboxBrowserContent(w, r, id)
	case rest == "browser/diagnose":
		s.handleSandboxDiagnoseCDP(w, r, id)
	case rest == "cdp/apply":
		s.handleSandboxCDPApply(w, r, id)
	case rest == "cdp/browser":
		if r.Method == http.MethodGet {
			s.handleSandboxCDPWebSocket(w, r, id)
		} else if r.Method == http.MethodPost {
			s.handleSandboxCDPRaw(w, r, id)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	default:
		s.writeError(w, http.StatusNotFound, "sandbox route not found")
	}
}

func (s *AdminServer) handleSandboxRecord(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		rec, ok := s.browserRecord(id)
		if !ok {
			s.writeError(w, http.StatusNotFound, "browser not found")
			return
		}
		s.writeOK(w, rec)
	case http.MethodPatch:
		var req struct {
			Alias string `json:"alias"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.browserMgr.UpdateAlias(id, req.Alias); err != nil {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeOK(w, map[string]any{"status": "ok"})
	case http.MethodDelete:
		if s.browserMgr == nil {
			s.writeError(w, http.StatusInternalServerError, "browser manager not initialized")
			return
		}
		if err := s.browserMgr.Destroy(r.Context(), id); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.writeOK(w, map[string]any{"status": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *AdminServer) handleLeaseSandbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.browserMgr == nil {
		s.writeError(w, http.StatusInternalServerError, "browser manager not initialized")
		return
	}
	lease, err := s.browserMgr.Acquire(r.Context())
	if err != nil {
		s.writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.writeOK(w, lease)
}

func (s *AdminServer) handleReleaseSandbox(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Invalid bool   `json:"invalid"`
		Error   string `json:"error"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := s.browserMgr.Release(id, browsermgr.ReleaseOptions{Invalid: req.Invalid, Error: req.Error}); err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeOK(w, map[string]any{"status": "ok"})
}

func (s *AdminServer) handlePauseSandbox(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.browserMgr.Stop(r.Context(), id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeOK(w, map[string]any{"status": "ok"})
}

func (s *AdminServer) handleResumeSandbox(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.browserMgr.Start(r.Context(), id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeOK(w, map[string]any{"status": "ok"})
}

func (s *AdminServer) handleRestartSandboxBrowser(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.browserMgr.Restart(r.Context(), id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeOK(w, map[string]any{"status": "ok"})
}

func (s *AdminServer) handleSandboxBrowserActions(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Actions []browserAction `json:"actions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	wsURL, err := s.ensureSandboxPage(r.Context(), id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := runBrowserActions(r.Context(), wsURL, req.Actions); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeOK(w, map[string]any{"status": "ok"})
}

func (s *AdminServer) handleSandboxBrowserContent(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	wsURL, err := s.ensureSandboxPage(r.Context(), id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	htmlText, _ := cdpEvaluateString(r.Context(), wsURL, "document.documentElement.outerHTML")
	title, _ := cdpEvaluateString(r.Context(), wsURL, "document.title")
	currentURL, _ := cdpEvaluateString(r.Context(), wsURL, "window.location.href")
	s.writeOK(w, map[string]any{"html": htmlText, "title": title, "url": currentURL})
}

func (s *AdminServer) handleSandboxBrowserScreenshot(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	wsURL, err := s.ensureSandboxPage(r.Context(), id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var req struct {
		Format  string `json:"format"`
		Quality int    `json:"quality"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Format == "" {
		req.Format = "png"
	}
	params := map[string]any{"format": req.Format}
	if req.Quality > 0 {
		params["quality"] = req.Quality
	}
	raw, err := cdpSend(r.Context(), wsURL, "Page.captureScreenshot", params)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var out struct {
		Data string `json:"data"`
	}
	_ = json.Unmarshal(raw, &out)
	s.writeOK(w, map[string]any{"data": out.Data, "format": req.Format})
}

func (s *AdminServer) handleSandboxDiagnoseCDP(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	cdpURL, err := s.browserMgr.CDPURL(id)
	if err != nil {
		s.writeOK(w, map[string]any{"error": err.Error()})
		return
	}
	diag := map[string]any{}
	if body, err := httpGetLimited(r.Context(), cdpURL+"/json/version"); err == nil {
		var version map[string]any
		_ = json.Unmarshal(body, &version)
		diag["version"] = version["Browser"]
		diag["cdp_port_open"] = true
	} else {
		diag["error"] = err.Error()
	}
	if body, err := httpGetLimited(r.Context(), cdpURL+"/json"); err == nil {
		var targets []any
		_ = json.Unmarshal(body, &targets)
		diag["browser_targets"] = len(targets)
	}
	s.writeOK(w, diag)
}

func (s *AdminServer) handleSandboxCDPApply(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.browserRecord(id); !ok {
		s.writeError(w, http.StatusNotFound, "browser not found")
		return
	}
	var req struct {
		TTLSec int `json:"ttl_sec"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.TTLSec <= 0 {
		req.TTLSec = 300
	}
	ticket := randomTicket()
	expires := time.Now().Add(time.Duration(req.TTLSec) * time.Second)
	adminBrowserCDPTickets.Store(ticket, sandboxTicket{SandboxID: id, ExpiresAt: expires})
	s.writeOK(w, map[string]any{"ticket": ticket, "expires_at": expires.Unix()})
}

func (s *AdminServer) handleSandboxCDPRaw(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	wsURL, err := s.ensureSandboxPage(r.Context(), id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	raw, err := cdpSend(r.Context(), wsURL, req.Method, req.Params)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		s.writeOK(w, map[string]any{})
		return
	}
	s.writeOK(w, payload)
}

func (s *AdminServer) handleSandboxCDPWebSocket(w http.ResponseWriter, r *http.Request, id string) {
	if !validSandboxTicket(id, r.URL.Query().Get("ticket")) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid ticket"})
		return
	}
	wsURL, err := s.ensureSandboxPage(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	backend, _, err := websocket.DefaultDialer.DialContext(r.Context(), wsURL, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	defer backend.Close()
	client, err := adminBrowserWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer client.Close()
	done := make(chan struct{}, 2)
	go proxyWebSocketMessages(client, backend, done)
	go proxyWebSocketMessages(backend, client, done)
	<-done
}

func (s *AdminServer) browserRecord(id string) (*browsermgr.Record, bool) {
	if s.browserMgr == nil {
		return nil, false
	}
	return s.browserMgr.Get(id)
}

func (s *AdminServer) ensureSandboxPage(ctx context.Context, id string) (string, error) {
	rec, ok := s.browserRecord(id)
	if !ok {
		return "", fmt.Errorf("browser not found: %s", id)
	}
	if rec.Endpoint != nil && strings.TrimSpace(rec.Endpoint.PageWebSocketURL) != "" {
		return rec.Endpoint.PageWebSocketURL, nil
	}
	cdpURL, err := s.browserMgr.CDPURL(id)
	if err != nil {
		return "", err
	}
	target, err := createCDPPageTarget(ctx, cdpURL, "about:blank")
	if err != nil {
		return "", err
	}
	if err := s.browserMgr.SetActivePage(id, target.ID, target.WebSocketDebuggerURL); err != nil {
		return "", err
	}
	return target.WebSocketDebuggerURL, nil
}

type browserAction struct {
	Type     string `json:"type"`
	URL      string `json:"url,omitempty"`
	X        int    `json:"x,omitempty"`
	Y        int    `json:"y,omitempty"`
	Text     string `json:"text,omitempty"`
	Key      string `json:"key,omitempty"`
	Duration int    `json:"duration,omitempty"`
	Script   string `json:"script,omitempty"`
}

func runBrowserActions(ctx context.Context, wsURL string, actions []browserAction) error {
	_, _ = cdpSend(ctx, wsURL, "Page.enable", nil)
	_, _ = cdpSend(ctx, wsURL, "Runtime.enable", nil)
	for _, action := range actions {
		switch strings.ToLower(action.Type) {
		case "navigate":
			if action.URL != "" {
				if _, err := cdpSend(ctx, wsURL, "Page.navigate", map[string]any{"url": action.URL}); err != nil {
					return err
				}
				time.Sleep(500 * time.Millisecond)
			}
		case "wait":
			d := time.Duration(action.Duration) * time.Millisecond
			if d <= 0 {
				d = time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
			}
		case "evaluate":
			if action.Script != "" {
				if _, err := cdpSend(ctx, wsURL, "Runtime.evaluate", map[string]any{"expression": action.Script, "returnByValue": true}); err != nil {
					return err
				}
			}
		case "click":
			if _, err := cdpSend(ctx, wsURL, "Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": action.X, "y": action.Y, "button": "left", "clickCount": 1}); err != nil {
				return err
			}
			if _, err := cdpSend(ctx, wsURL, "Input.dispatchMouseEvent", map[string]any{"type": "mouseReleased", "x": action.X, "y": action.Y, "button": "left", "clickCount": 1}); err != nil {
				return err
			}
		case "type":
			if action.Text != "" {
				if _, err := cdpSend(ctx, wsURL, "Input.insertText", map[string]any{"text": action.Text}); err != nil {
					return err
				}
			}
		case "key_press":
			if action.Key != "" {
				if _, err := cdpSend(ctx, wsURL, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": action.Key}); err != nil {
					return err
				}
				if _, err := cdpSend(ctx, wsURL, "Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": action.Key}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type cdpPageTarget struct {
	ID                   string `json:"id"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func createCDPPageTarget(ctx context.Context, cdpURL string, pageURL string) (cdpPageTarget, error) {
	targetURL := strings.TrimRight(cdpURL, "/") + "/json/new?" + url.QueryEscape(pageURL)
	var lastErr error
	for _, method := range []string{http.MethodPut, http.MethodGet} {
		req, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
		if err != nil {
			return cdpPageTarget{}, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return cdpPageTarget{}, readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("create cdp target HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			continue
		}
		var target cdpPageTarget
		if err := json.Unmarshal(body, &target); err != nil {
			return cdpPageTarget{}, err
		}
		if target.WebSocketDebuggerURL == "" {
			return cdpPageTarget{}, fmt.Errorf("cdp target missing websocket url")
		}
		return target, nil
	}
	return cdpPageTarget{}, lastErr
}

func cdpEvaluateString(ctx context.Context, wsURL string, expression string) (string, error) {
	raw, err := cdpSend(ctx, wsURL, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true})
	if err != nil {
		return "", err
	}
	var resp struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	return resp.Result.Value, nil
}

func cdpSend(ctx context.Context, wsURL string, method string, params map[string]any) (json.RawMessage, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cdp dial: %w", err)
	}
	defer conn.Close()
	id := time.Now().UnixNano()
	req := map[string]any{"id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	_ = conn.SetWriteDeadline(deadline)
	if err := conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("cdp write: %w", err)
	}
	for {
		_ = conn.SetReadDeadline(deadline)
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("cdp read: %w", err)
		}
		var resp struct {
			ID     int64           `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(msg, &resp); err != nil || resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("cdp %s: %s", method, resp.Error.Message)
		}
		if len(resp.Result) == 0 {
			return json.RawMessage("{}"), nil
		}
		return resp.Result, nil
	}
}

func httpGetLimited(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func proxyWebSocketMessages(src, dst *websocket.Conn, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	for {
		msgType, msg, err := src.ReadMessage()
		if err != nil {
			return
		}
		if err := dst.WriteMessage(msgType, msg); err != nil {
			return
		}
	}
}

func validSandboxTicket(sandboxID string, ticket string) bool {
	if strings.TrimSpace(ticket) == "" {
		return false
	}
	value, ok := adminBrowserCDPTickets.Load(ticket)
	if !ok {
		return false
	}
	claims, ok := value.(sandboxTicket)
	if !ok || claims.SandboxID != sandboxID || time.Now().After(claims.ExpiresAt) {
		adminBrowserCDPTickets.Delete(ticket)
		return false
	}
	return true
}

func randomTicket() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sandboxPath(rawPath string) (string, string) {
	rest := strings.Trim(strings.TrimPrefix(rawPath, "/api/v1/sandboxes/"), "/")
	if rest == "" || rest == rawPath {
		return "", ""
	}
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	if len(parts) == 1 {
		return id, ""
	}
	return id, parts[1]
}
