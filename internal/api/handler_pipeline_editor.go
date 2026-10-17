package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
)

type pipeline_editor_session struct {
	ID     string `json:"editor_id"`
	FlowID string `json:"flow_id"`
}
type pipeline_editor_reply struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	FlowID    string          `json:"flow_id"`
	Error     string          `json:"error,omitempty"`
	Snapshot  json.RawMessage `json:"snapshot,omitempty"`
}
type pipeline_editor_pending struct {
	client *automation_ws_client
	reply  chan pipeline_editor_reply
}
type pipeline_editor_pool struct {
	mu       sync.Mutex
	sessions map[*automation_ws_client]pipeline_editor_session
	pending  map[string]pipeline_editor_pending
}

func new_pipeline_editor_pool() *pipeline_editor_pool {
	return &pipeline_editor_pool{sessions: map[*automation_ws_client]pipeline_editor_session{}, pending: map[string]pipeline_editor_pending{}}
}

var pipeline_editors = new_pipeline_editor_pool()

func pipeline_editor_id() string {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(random[:])
}
func (pool *pipeline_editor_pool) receive(client *automation_ws_client, raw []byte) {
	var message pipeline_editor_reply
	if json.Unmarshal(raw, &message) != nil {
		return
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if message.Type == "pipeline_editor_register" && strings.TrimSpace(message.FlowID) != "" {
		if previous, ok := pool.sessions[client]; ok && previous.FlowID == message.FlowID {
			return
		}
		pool.sessions[client] = pipeline_editor_session{ID: pipeline_editor_id(), FlowID: message.FlowID}
		return
	}
	if message.Type != "pipeline_editor_reply" {
		return
	}
	if pending, ok := pool.pending[message.RequestID]; ok && pending.client == client {
		select {
		case pending.reply <- message:
		default:
		}
	}
}
func (pool *pipeline_editor_pool) remove(client *automation_ws_client) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	delete(pool.sessions, client)
	for _, pending := range pool.pending {
		if pending.client == client {
			select {
			case pending.reply <- pipeline_editor_reply{Error: "编辑页面已断开，无法确认结果；重新连接后先读取草稿"}:
			default:
			}
		}
	}
}
func (pool *pipeline_editor_pool) list(flow_id string) []pipeline_editor_session {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	sessions := []pipeline_editor_session{}
	for _, session := range pool.sessions {
		if flow_id == "" || session.FlowID == flow_id {
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })
	return sessions
}
func (pool *pipeline_editor_pool) request(ctx context.Context, editor_id string, message map[string]interface{}) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request_id := pipeline_editor_id()
	pool.mu.Lock()
	var client *automation_ws_client
	var session pipeline_editor_session
	for candidate, info := range pool.sessions {
		if info.ID == editor_id {
			client = candidate
			session = info
			break
		}
	}
	if client == nil {
		pool.mu.Unlock()
		return nil, fmt.Errorf("编辑页面不存在或已断开，请重新获取编辑器")
	}
	replies := make(chan pipeline_editor_reply, 1)
	pool.pending[request_id] = pipeline_editor_pending{client: client, reply: replies}
	pool.mu.Unlock()
	defer func() { pool.mu.Lock(); delete(pool.pending, request_id); pool.mu.Unlock() }()
	message["request_id"] = request_id
	message["flow_id"] = session.FlowID
	// A background page must not apply writes after the delivery window expires.
	deadline, _ := ctx.Deadline()
	message["expires_at"] = deadline.Add(-time.Second).UnixMilli()
	encoded, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	if !client.enqueue(encoded) {
		return nil, fmt.Errorf("编辑页面消息队列繁忙或已断开，请重试")
	}
	select {
	case reply := <-replies:
		if reply.Error != "" {
			return nil, fmt.Errorf("%s", reply.Error)
		}
		if len(reply.Snapshot) == 0 || string(reply.Snapshot) == "null" {
			return nil, fmt.Errorf("编辑页面返回空草稿")
		}
		return reply.Snapshot, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("编辑页面未确认操作结果；请重新读取草稿后再决定是否重试")
	}
}
func (c *APIClient) handle_get_pipeline_editor(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 16*1024)
	var body struct {
		EditorID string `json:"editor_id"`
		FlowID   string `json:"flow_id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	sessions := pipeline_editors.list(strings.TrimSpace(body.FlowID))
	editor_id := strings.TrimSpace(body.EditorID)
	if editor_id == "" {
		if len(sessions) == 0 {
			result.Err(ctx, api_code_invalid_params, "没有已连接的 Pipeline 编辑页面，请打开 /automation/edit 页面")
			return
		}
		if len(sessions) > 1 {
			result.Ok(ctx, gin.H{"editors": sessions, "selection_required": true})
			return
		}
		editor_id = sessions[0].ID
	}
	snapshot, err := pipeline_editors.request(ctx.Request.Context(), editor_id, map[string]interface{}{"type": "pipeline_editor_read"})
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"editor_id": editor_id, "snapshot": snapshot, "catalog": services.SortedUserFlowNodeCatalog()})
}
func (c *APIClient) handle_update_pipeline_editor(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1024*1024)
	var body struct {
		EditorID string                       `json:"editor_id"`
		Revision *int64                       `json:"revision"`
		Draft    services.PipelineEditorDraft `json:"draft"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil || body.Revision == nil || *body.Revision < 1 || strings.TrimSpace(body.EditorID) == "" {
		result.Err(ctx, api_code_invalid_params, "必须提供 editor_id、读取到的 revision 和完整 draft")
		return
	}
	if err := services.ValidatePipelineEditorDraft(&body.Draft); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	snapshot, err := pipeline_editors.request(ctx.Request.Context(), body.EditorID, map[string]interface{}{"type": "pipeline_editor_apply", "revision": *body.Revision, "draft": body.Draft})
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"editor_id": body.EditorID, "snapshot": snapshot, "applied": true, "saved": false})
}
