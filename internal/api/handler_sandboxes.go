package api

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

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	result "wx_channel/internal/util"
	"wx_channel/pkg/browsermgr"
)

type sandboxTicket struct {
	SandboxID string
	ExpiresAt time.Time
}

var browserCDPTickets sync.Map

var browserWSUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (c *APIClient) handleListSandboxes(ctx *gin.Context) {
	if c.browserMgr == nil {
		result.Ok(ctx, []any{})
		return
	}
	result.Ok(ctx, c.browserMgr.List())
}

func (c *APIClient) handleCreateSandbox(ctx *gin.Context) {
	if c.browserMgr == nil {
		result.Err(ctx, 500, "browser manager not initialized")
		return
	}
	var req browsermgr.CreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && !strings.Contains(err.Error(), "EOF") {
		result.Err(ctx, 400, err.Error())
		return
	}
	rec, err := c.browserMgr.Create(ctx.Request.Context(), req)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, rec)
}

func (c *APIClient) handleGetSandbox(ctx *gin.Context) {
	rec, ok := c.browserRecord(ctx.Param("id"))
	if !ok {
		result.Err(ctx, 404, "browser not found")
		return
	}
	result.Ok(ctx, rec)
}

func (c *APIClient) handleLeaseSandbox(ctx *gin.Context) {
	if c.browserMgr == nil {
		result.Err(ctx, 500, "browser manager not initialized")
		return
	}
	lease, err := c.browserMgr.Acquire(ctx.Request.Context())
	if err != nil {
		result.Err(ctx, 409, err.Error())
		return
	}
	result.Ok(ctx, lease)
}

func (c *APIClient) handleReleaseSandbox(ctx *gin.Context) {
	if c.browserMgr == nil {
		result.Err(ctx, 500, "browser manager not initialized")
		return
	}
	var req struct {
		Invalid bool   `json:"invalid"`
		Error   string `json:"error"`
	}
	_ = ctx.ShouldBindJSON(&req)
	if err := c.browserMgr.Release(ctx.Param("id"), browsermgr.ReleaseOptions{Invalid: req.Invalid, Error: req.Error}); err != nil {
		result.Err(ctx, 404, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"status": "ok"})
}

func (c *APIClient) handleUpdateSandbox(ctx *gin.Context) {
	if c.browserMgr == nil {
		result.Err(ctx, 500, "browser manager not initialized")
		return
	}
	var req struct {
		Alias string `json:"alias"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if err := c.browserMgr.UpdateAlias(ctx.Param("id"), req.Alias); err != nil {
		result.Err(ctx, 404, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"status": "ok"})
}

func (c *APIClient) handleDeleteSandbox(ctx *gin.Context) {
	if c.browserMgr == nil {
		result.Err(ctx, 500, "browser manager not initialized")
		return
	}
	if err := c.browserMgr.Destroy(ctx.Request.Context(), ctx.Param("id")); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"status": "ok"})
}

func (c *APIClient) handlePauseSandbox(ctx *gin.Context) {
	if c.browserMgr == nil {
		result.Err(ctx, 500, "browser manager not initialized")
		return
	}
	if err := c.browserMgr.Stop(ctx.Request.Context(), ctx.Param("id")); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"status": "ok"})
}

func (c *APIClient) handleResumeSandbox(ctx *gin.Context) {
	if c.browserMgr == nil {
		result.Err(ctx, 500, "browser manager not initialized")
		return
	}
	if err := c.browserMgr.Start(ctx.Request.Context(), ctx.Param("id")); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"status": "ok"})
}

func (c *APIClient) handleRestartSandboxBrowser(ctx *gin.Context) {
	if c.browserMgr == nil {
		result.Err(ctx, 500, "browser manager not initialized")
		return
	}
	if err := c.browserMgr.Restart(ctx.Request.Context(), ctx.Param("id")); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"status": "ok"})
}

func (c *APIClient) handleSandboxBrowserActions(ctx *gin.Context) {
	var req struct {
		Actions []browserAction `json:"actions"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	wsURL, err := c.ensureSandboxPage(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	if err := runBrowserActions(ctx.Request.Context(), wsURL, req.Actions); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"status": "ok"})
}

func (c *APIClient) handleSandboxBrowserContent(ctx *gin.Context) {
	wsURL, err := c.ensureSandboxPage(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	htmlText, _ := cdpEvaluateString(ctx.Request.Context(), wsURL, "document.documentElement.outerHTML")
	title, _ := cdpEvaluateString(ctx.Request.Context(), wsURL, "document.title")
	currentURL, _ := cdpEvaluateString(ctx.Request.Context(), wsURL, "window.location.href")
	result.Ok(ctx, gin.H{"html": htmlText, "title": title, "url": currentURL})
}

func (c *APIClient) handleSandboxBrowserScreenshot(ctx *gin.Context) {
	wsURL, err := c.ensureSandboxPage(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	var req struct {
		Format  string `json:"format"`
		Quality int    `json:"quality"`
	}
	_ = ctx.ShouldBindJSON(&req)
	if req.Format == "" {
		req.Format = "png"
	}
	params := map[string]any{"format": req.Format}
	if req.Quality > 0 {
		params["quality"] = req.Quality
	}
	raw, err := cdpSend(ctx.Request.Context(), wsURL, "Page.captureScreenshot", params)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	var out struct {
		Data string `json:"data"`
	}
	_ = json.Unmarshal(raw, &out)
	result.Ok(ctx, gin.H{"data": out.Data, "format": req.Format})
}

func (c *APIClient) handleSandboxDiagnoseCDP(ctx *gin.Context) {
	cdpURL, err := c.browserMgr.CDPURL(ctx.Param("id"))
	if err != nil {
		result.Ok(ctx, gin.H{"error": err.Error()})
		return
	}
	diag := gin.H{}
	if body, err := httpGetLimited(ctx.Request.Context(), cdpURL+"/json/version"); err == nil {
		var version map[string]any
		_ = json.Unmarshal(body, &version)
		diag["version"] = version["Browser"]
		diag["cdp_port_open"] = true
	} else {
		diag["error"] = err.Error()
	}
	if body, err := httpGetLimited(ctx.Request.Context(), cdpURL+"/json"); err == nil {
		var targets []any
		_ = json.Unmarshal(body, &targets)
		diag["browser_targets"] = len(targets)
	}
	result.Ok(ctx, diag)
}

func (c *APIClient) handleSandboxCDPApply(ctx *gin.Context) {
	id := ctx.Param("id")
	if _, ok := c.browserRecord(id); !ok {
		result.Err(ctx, 404, "browser not found")
		return
	}
	var req struct {
		TTLSec int `json:"ttl_sec"`
	}
	_ = ctx.ShouldBindJSON(&req)
	if req.TTLSec <= 0 {
		req.TTLSec = 300
	}
	ticket := randomTicket()
	expires := time.Now().Add(time.Duration(req.TTLSec) * time.Second)
	browserCDPTickets.Store(ticket, sandboxTicket{SandboxID: id, ExpiresAt: expires})
	result.Ok(ctx, gin.H{"ticket": ticket, "expires_at": expires.Unix()})
}

func (c *APIClient) handleSandboxCDPRaw(ctx *gin.Context) {
	var req struct {
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	wsURL, err := c.ensureSandboxPage(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	raw, err := cdpSend(ctx.Request.Context(), wsURL, req.Method, req.Params)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		result.Ok(ctx, gin.H{})
		return
	}
	result.Ok(ctx, payload)
}

func (c *APIClient) handleSandboxCDPWebSocket(ctx *gin.Context) {
	id := ctx.Param("id")
	if !validSandboxTicket(id, ctx.Query("ticket")) {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid ticket"})
		return
	}
	wsURL, err := c.ensureSandboxPage(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	backend, _, err := websocket.DefaultDialer.DialContext(ctx.Request.Context(), wsURL, nil)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer backend.Close()
	client, err := browserWSUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	defer client.Close()
	done := make(chan struct{}, 2)
	go proxyWebSocketMessages(client, backend, done)
	go proxyWebSocketMessages(backend, client, done)
	<-done
}

func (c *APIClient) browserRecord(id string) (*browsermgr.Record, bool) {
	if c.browserMgr == nil {
		return nil, false
	}
	return c.browserMgr.Get(id)
}

func (c *APIClient) ensureSandboxPage(ctx context.Context, id string) (string, error) {
	rec, ok := c.browserRecord(id)
	if !ok {
		return "", fmt.Errorf("browser not found: %s", id)
	}
	if rec.Endpoint != nil && strings.TrimSpace(rec.Endpoint.PageWebSocketURL) != "" {
		return rec.Endpoint.PageWebSocketURL, nil
	}
	cdpURL, err := c.browserMgr.CDPURL(id)
	if err != nil {
		return "", err
	}
	target, err := createCDPPageTarget(ctx, cdpURL, "about:blank")
	if err != nil {
		return "", err
	}
	if err := c.browserMgr.SetActivePage(id, target.ID, target.WebSocketDebuggerURL); err != nil {
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
	value, ok := browserCDPTickets.Load(ticket)
	if !ok {
		return false
	}
	claims, ok := value.(sandboxTicket)
	if !ok || claims.SandboxID != sandboxID || time.Now().After(claims.ExpiresAt) {
		browserCDPTickets.Delete(ticket)
		return false
	}
	return true
}

func randomTicket() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
