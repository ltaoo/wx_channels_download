package services

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/flowengine/engine"
)

const (
	user_flow_start_node_id = "start"
	user_flow_id_prefix     = "flow-u-"
)

// user_flow_node_catalog lists the node types a user pipeline may contain.
// Only JSON-config driven node types are exposed; FuncNode is bound to Go
// functions and therefore not user-constructible.
type UserFlowNodeCatalogItem struct {
	Type        string              `json:"type"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	ConfigKeys  []UserFlowConfigKey `json:"config_keys"`
}

type UserFlowConfigKey struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

func user_flow_node_catalog() []UserFlowNodeCatalogItem {
	return []UserFlowNodeCatalogItem{
		{
			Type:        "ExprNode",
			Name:        "表达式计算",
			Description: "对流程上下文执行表达式并写回结果",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "expression", Type: "string", Required: true, Description: "表达式，如 len(items) > 0"},
				{Key: "output_key", Type: "string", Required: false, Description: "结果写入上下文的键，默认 calc_out"},
			},
		},
		{
			Type:        "APICallNode",
			Name:        "HTTP 请求",
			Description: "调用外部 HTTP 接口，可携带上下文参数",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "url", Type: "string", Required: true, Description: "请求地址"},
				{Key: "method", Type: "string", Required: false, Description: "GET 或 POST，默认 GET"},
				{Key: "keys", Type: "array", Required: false, Description: "随请求携带的上下文键列表"},
			},
		},
		{
			Type:        "GatewayNode",
			Name:        "条件分支",
			Description: "按条件表达式路由到不同节点",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "gateway_type", Type: "string", Required: false, Description: "Exclusive 或 Parallel"},
				{Key: "rules", Type: "array", Required: false, Description: "Exclusive 规则列表 [{condition, target_id}]"},
			},
		},
		{
			Type:        "ManualNode",
			Name:        "人工确认",
			Description: "流程暂停，等待人工处理后继续",
		},
		{
			Type:        "EndNode",
			Name:        "结束",
			Description: "流程结束节点",
		},
	}
}

// CreateUserFlowInput describes a new user pipeline. Creating a pipeline always
// produces a flow with a single start node; subsequent nodes are added through
// the editor (UpdateUserFlow).
type CreateUserFlowInput struct {
	Name          string
	Description   string
	TriggerType   string
	EventKey      string
	ContextSchema []engine.FieldSchema
}

// UpdateUserFlowInput replaces the editable parts of a user pipeline. Nodes is
// the authoritative node list; edges are expressed as next_ids on each node.
type UpdateUserFlowInput struct {
	Name          *string
	Description   *string
	ContextSchema []engine.FieldSchema
	StartNodeID   string
	Nodes         []UserFlowNodeInput
}

type UserFlowNodeInput struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Config      map[string]interface{} `json:"config"`
	InputSchema []engine.FieldSchema   `json:"input_schema"`
	NextIDs     []string               `json:"next_ids"`
}

func (s *AutomationService) load_user_flows() {
	if s == nil || s.db == nil || s.flow_engine == nil {
		return
	}
	var flows []model.UserFlow
	if err := s.db.Where("deleted_at IS NULL").Find(&flows).Error; err != nil {
		s.log_error(err, "failed to load user flows")
		return
	}
	for index := range flows {
		definition, err := decode_user_flow_definition(flows[index].Definition)
		if err != nil {
			s.log_error(err, "failed to decode user flow "+flows[index].ID)
			continue
		}
		s.flow_engine.FlowDefinitions[definition.ID] = *definition
	}
}

func decode_user_flow_definition(raw string) (*engine.FlowDefinition, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("流程定义为空")
	}
	var definition engine.FlowDefinition
	if err := json.Unmarshal([]byte(raw), &definition); err != nil {
		return nil, fmt.Errorf("流程定义无法解析: %w", err)
	}
	if definition.Nodes == nil {
		definition.Nodes = map[string]engine.NodeDefinition{}
	}
	return &definition, nil
}

// register_user_flow publishes the definition into the shared engine so a
// schedule can run it immediately after create/update.
func (s *AutomationService) register_user_flow(definition engine.FlowDefinition) {
	if s == nil || s.flow_engine == nil {
		return
	}
	s.flow_engine.FlowDefinitions[definition.ID] = definition
}

func normalize_user_flow_trigger(trigger_type string, event_key string) (string, string, error) {
	trigger_type = strings.TrimSpace(trigger_type)
	switch trigger_type {
	case "":
		trigger_type = model.FlowRunTriggerCron
	case model.FlowRunTriggerCron, model.FlowRunTriggerEvent, model.FlowRunTriggerManual:
	default:
		return "", "", fmt.Errorf("触发类型不支持: %s", trigger_type)
	}
	event_key = strings.TrimSpace(event_key)
	if trigger_type == model.FlowRunTriggerEvent && event_key == "" {
		return "", "", fmt.Errorf("事件触发必须提供 event_key")
	}
	return trigger_type, event_key, nil
}

// CreateUserFlow persists a new pipeline containing only a start node whose
// input schema is the caller-declared context schema.
func (s *AutomationService) CreateUserFlow(input CreateUserFlowInput) (*model.UserFlow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("name 不能为空")
	}
	trigger_type, event_key, err := normalize_user_flow_trigger(input.TriggerType, input.EventKey)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	flow := model.UserFlow{
		ID:          fmt.Sprintf("%s%d", user_flow_id_prefix, time.Now().UnixNano()),
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		TriggerType: trigger_type,
		EventKey:    event_key,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	start_node := engine.NodeDefinition{
		ID:   user_flow_start_node_id,
		Type: "StartNode",
		Name: "开始",
	}
	definition := engine.FlowDefinition{
		ID:            flow.ID,
		Name:          name,
		StartNodeID:   user_flow_start_node_id,
		ContextSchema: input.ContextSchema,
		Nodes:         map[string]engine.NodeDefinition{start_node.ID: start_node},
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("流程定义无法序列化: %w", err)
	}
	flow.Definition = string(encoded)
	if err := s.db.Create(&flow).Error; err != nil {
		return nil, fmt.Errorf("创建流程失败: %w", err)
	}
	s.register_user_flow(definition)
	return &flow, nil
}

// GetUserFlow loads one user pipeline by id.
func (s *AutomationService) GetUserFlow(id string) (*model.UserFlow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	id = strings.TrimSpace(id)
	var flow model.UserFlow
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&flow).Error; err != nil {
		return nil, fmt.Errorf("流程不存在: %s", id)
	}
	return &flow, nil
}

// ListUserFlows returns all user pipelines ordered by creation time.
func (s *AutomationService) ListUserFlows() ([]model.UserFlow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	flows := make([]model.UserFlow, 0)
	if err := s.db.Where("deleted_at IS NULL").Order("created_at DESC").Find(&flows).Error; err != nil {
		return nil, fmt.Errorf("查询流程失败: %w", err)
	}
	return flows, nil
}

// UpdateUserFlow replaces the flow definition from editor input, re-validates
// the graph and re-registers it into the engine.
func (s *AutomationService) UpdateUserFlow(id string, input UpdateUserFlowInput) (*model.UserFlow, error) {
	flow, err := s.GetUserFlow(id)
	if err != nil {
		return nil, err
	}
	definition, err := decode_user_flow_definition(flow.Definition)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, fmt.Errorf("name 不能为空")
		}
		flow.Name = name
		definition.Name = name
	}
	if input.Description != nil {
		flow.Description = strings.TrimSpace(*input.Description)
	}
	if input.ContextSchema != nil {
		definition.ContextSchema = input.ContextSchema
	}
	if len(input.Nodes) > 0 {
		nodes := map[string]engine.NodeDefinition{}
		for _, node := range input.Nodes {
			node_id := strings.TrimSpace(node.ID)
			if node_id == "" {
				return nil, fmt.Errorf("节点 id 不能为空")
			}
			if !user_flow_allows_node_type(node.Type) {
				return nil, fmt.Errorf("节点类型不支持: %s", node.Type)
			}
			if _, exists := nodes[node_id]; exists {
				return nil, fmt.Errorf("节点 id 重复: %s", node_id)
			}
			config := node.Config
			if config == nil {
				config = map[string]interface{}{}
			}
			config["id"] = node_id
			next_nodes := make([]engine.TargetNode, 0, len(node.NextIDs))
			for _, next := range node.NextIDs {
				next = strings.TrimSpace(next)
				if next == "" {
					continue
				}
				next_nodes = append(next_nodes, engine.TargetNode{TargetID: next})
			}
			nodes[node_id] = engine.NodeDefinition{
				ID:          node_id,
				Type:        node.Type,
				Name:        node.Name,
				Config:      config,
				NextNodes:   next_nodes,
				NextNodeIDs: node.NextIDs,
				InputSchema: node.InputSchema,
			}
		}
		// Every referenced target must exist to keep the graph runnable.
		for _, node := range nodes {
			for _, next_id := range node.NextNodeIDs {
				if _, ok := nodes[next_id]; !ok {
					return nil, fmt.Errorf("节点 %s 指向了不存在的节点: %s", node.ID, next_id)
				}
			}
			for _, target := range node.NextNodes {
				if _, ok := nodes[target.TargetID]; !ok {
					return nil, fmt.Errorf("节点 %s 指向了不存在的节点: %s", node.ID, target.TargetID)
				}
			}
		}
		start_node_id := strings.TrimSpace(input.StartNodeID)
		if start_node_id == "" {
			start_node_id = definition.StartNodeID
		}
		if _, ok := nodes[start_node_id]; !ok {
			return nil, fmt.Errorf("开始节点不存在: %s", start_node_id)
		}
		definition.StartNodeID = start_node_id
		definition.Nodes = nodes
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("流程定义无法序列化: %w", err)
	}
	flow.Definition = string(encoded)
	flow.UpdatedAt = time.Now().UnixMilli()
	if err := s.db.Model(&model.UserFlow{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"name":        flow.Name,
			"description": flow.Description,
			"definition":  flow.Definition,
			"updated_at":  flow.UpdatedAt,
		}).Error; err != nil {
		return nil, fmt.Errorf("更新流程失败: %w", err)
	}
	s.register_user_flow(*definition)
	return s.GetUserFlow(id)
}

// DeleteUserFlow soft-deletes a user pipeline and detaches it from the engine.
func (s *AutomationService) DeleteUserFlow(id string) error {
	flow, err := s.GetUserFlow(id)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	if err := s.db.Model(&model.UserFlow{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("删除流程失败: %w", err)
	}
	if s.flow_engine != nil {
		delete(s.flow_engine.FlowDefinitions, flow.ID)
	}
	return nil
}

// UserFlowVisualization renders user pipelines in the same graph shape the
// frontend uses for built-in flows.
func (s *AutomationService) UserFlowVisualization(flow_id string) (*flowengine.FlowVisualizationPayload, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	flows, err := s.ListUserFlows()
	if err != nil {
		return nil, err
	}
	definitions := make([]engine.FlowDefinition, 0, len(flows))
	for index := range flows {
		definition, err := decode_user_flow_definition(flows[index].Definition)
		if err != nil {
			continue
		}
		definitions = append(definitions, *definition)
	}
	return flowengine.BuildFlowVisualizationPayload(definitions, strings.TrimSpace(flow_id), flowengine.FlowVisualizationOptions{
		Platform: "user",
		Purpose:  "automation-flow-editor",
		Editable: true,
	})
}

// user_flow_allows_node_type restricts editor nodes to engine node types whose
// behaviour is fully described by JSON config.
func user_flow_allows_node_type(node_type string) bool {
	switch node_type {
	case "StartNode", "EndNode", "ExprNode", "GatewayNode", "APICallNode", "ManualNode":
		return true
	default:
		return false
	}
}

// SortedUserFlowNodeCatalog keeps a stable order for the editor picker.
func SortedUserFlowNodeCatalog() []UserFlowNodeCatalogItem {
	catalog := user_flow_node_catalog()
	sort.SliceStable(catalog, func(i, j int) bool {
		return catalog[i].Type < catalog[j].Type
	})
	return catalog
}

// TriggerFlowDirect runs a user pipeline immediately. It reuses the schedule
// execution path with an ephemeral schedule so run history and events behave
// exactly like a scheduled execution.
func (s *AutomationService) TriggerFlowDirect(flow_id string, trigger_type string, trigger_key string) (*model.FlowRunRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	if s.flow_engine == nil {
		return nil, fmt.Errorf("流程引擎未初始化")
	}
	flow, err := s.GetUserFlow(flow_id)
	if err != nil {
		return nil, err
	}
	if _, err := decode_user_flow_definition(flow.Definition); err != nil {
		return nil, err
	}
	switch trigger_type {
	case "", model.FlowRunTriggerManual:
		trigger_type = model.FlowRunTriggerManual
	case model.FlowRunTriggerEvent:
		if strings.TrimSpace(trigger_key) == "" {
			trigger_key = flow.EventKey
		}
	default:
		return nil, fmt.Errorf("不支持的触发类型: %s", trigger_type)
	}
	run_key := "direct-" + flow.ID
	if _, claimed := s.running.LoadOrStore(run_key, struct{}{}); claimed {
		return nil, fmt.Errorf("流程 %s 正在执行中", flow.ID)
	}
	defer s.running.Delete(run_key)
	return s.execute_schedule(model.FlowSchedule{
		ID:          "",
		FlowID:      flow.ID,
		CronExpr:    "@daily",
		InitialData: "{}",
		TimeoutSec:  default_schedule_timeout_sec,
	}, trigger_type, strings.TrimSpace(trigger_key)), nil
}
