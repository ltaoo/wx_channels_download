package services

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/internal/mcpserver"
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
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	ConfigKeys  []UserFlowConfigKey    `json:"config_keys"`
	Tools       []mcpserver.Definition `json:"tools,omitempty"`
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
			Type:        "JSCodeNode",
			Name:        "执行 JS",
			Description: "用 JavaScript 处理流程上下文，返回值写回上下文",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "code", Type: "string", Required: true, Description: "JavaScript 代码，可通过 data 访问流程上下文"},
				{Key: "output_key", Type: "string", Required: false, Description: "结果写入上下文的键；留空且返回对象时，对象字段合并回上下文"},
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
			Type:        "ServiceNode",
			Name:        "服务调用",
			Description: "调用应用内 service tool，并将结构化结果写回流程上下文",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "tool_name", Type: "string", Required: true, Description: "要调用的 service tool 名称"},
				{Key: "arguments", Type: "object", Required: false, Description: "传给 tool 的静态参数"},
				{Key: "input_map", Type: "object", Required: false, Description: "tool 参数名到流程上下文键的映射"},
				{Key: "output_key", Type: "string", Required: false, Description: "结构化结果写入上下文的键，默认 service_result"},
				{Key: "timeout_seconds", Type: "number", Required: false, Description: "节点调用超时时间；不填时由具体 tool 控制"},
			},
			Tools: mcpserver.ToolCatalog(),
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
	Position    *engine.NodePosition   `json:"position,omitempty"`
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

// ImportUserFlow creates a new pipeline from a full flow definition JSON (the
// same shape persisted in a UserFlow.Definition). Node ids, edges, context
// schema and the start node are taken from the imported definition; a fresh
// flow id is always generated so an import never overwrites an existing flow.
func (s *AutomationService) ImportUserFlow(raw string) (*model.UserFlow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	definition, err := decode_user_flow_definition(raw)
	if err != nil {
		return nil, err
	}
	if err := normalize_user_flow_definition(definition); err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	flow := model.UserFlow{
		ID:          fmt.Sprintf("%s%d", user_flow_id_prefix, time.Now().UnixNano()),
		Name:        definition.Name,
		TriggerType: model.FlowRunTriggerCron,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	definition.ID = flow.ID
	encoded, err := json.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("流程定义无法序列化: %w", err)
	}
	flow.Definition = string(encoded)
	if err := s.db.Create(&flow).Error; err != nil {
		return nil, fmt.Errorf("导入流程失败: %w", err)
	}
	s.register_user_flow(*definition)
	return &flow, nil
}

// normalize_user_flow_definition validates an imported definition and rewrites
// each node so its id, config["id"], NextNodeIDs and NextNodes stay consistent.
// The engine drives ordinary nodes via NextNodes but ManualNode via NextNodeIDs,
// so both representations must agree for an imported graph to run correctly.
func normalize_user_flow_definition(definition *engine.FlowDefinition) error {
	name := strings.TrimSpace(definition.Name)
	if name == "" {
		return fmt.Errorf("流程定义缺少 name")
	}
	if definition.Nodes == nil || len(definition.Nodes) == 0 {
		return fmt.Errorf("流程定义缺少节点")
	}
	start_node_id := strings.TrimSpace(definition.StartNodeID)
	if start_node_id == "" {
		return fmt.Errorf("流程定义缺少开始节点")
	}
	if _, ok := definition.Nodes[start_node_id]; !ok {
		return fmt.Errorf("开始节点不存在: %s", start_node_id)
	}
	normalized := make(map[string]engine.NodeDefinition, len(definition.Nodes))
	for node_id, node := range definition.Nodes {
		node_id = strings.TrimSpace(node_id)
		if node_id == "" {
			return fmt.Errorf("节点 id 不能为空")
		}
		if !user_flow_allows_node_type(node.Type) {
			return fmt.Errorf("节点类型不支持: %s", node.Type)
		}
		if node.Config == nil {
			node.Config = map[string]interface{}{}
		}
		if err := validate_user_flow_node_config(node.Type, node.Config); err != nil {
			return fmt.Errorf("节点 %s 配置无效: %w", node_id, err)
		}
		node.ID = node_id
		node.Config["id"] = node_id
		next_ids := make([]string, 0)
		seen := map[string]bool{}
		for _, next := range node.NextNodeIDs {
			next = strings.TrimSpace(next)
			if next != "" && !seen[next] {
				seen[next] = true
				next_ids = append(next_ids, next)
			}
		}
		for _, target := range node.NextNodes {
			target_id := strings.TrimSpace(target.TargetID)
			if target_id != "" && !seen[target_id] {
				seen[target_id] = true
				next_ids = append(next_ids, target_id)
			}
		}
		next_nodes := make([]engine.TargetNode, 0, len(next_ids))
		for _, next := range next_ids {
			next_nodes = append(next_nodes, engine.TargetNode{TargetID: next})
		}
		node.NextNodeIDs = next_ids
		node.NextNodes = next_nodes
		normalized[node_id] = node
	}
	for node_id, node := range normalized {
		for _, next_id := range node.NextNodeIDs {
			if _, ok := normalized[next_id]; !ok {
				return fmt.Errorf("节点 %s 指向了不存在的节点: %s", node_id, next_id)
			}
		}
	}
	definition.Name = name
	definition.StartNodeID = start_node_id
	definition.Nodes = normalized
	return nil
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
			if err := validate_user_flow_node_config(node.Type, config); err != nil {
				return nil, fmt.Errorf("节点 %s 配置无效: %w", node_id, err)
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
				Position:    node.Position,
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
	case "StartNode", "EndNode", "ExprNode", "GatewayNode", "APICallNode", "ManualNode", "ServiceNode", "JSCodeNode":
		return true
	default:
		return false
	}
}

func validate_user_flow_node_config(node_type string, config map[string]interface{}) error {
	if node_type != "ServiceNode" {
		return nil
	}
	tool_name, _ := config["tool_name"].(string)
	tool_name = strings.TrimSpace(tool_name)
	if tool_name == "" {
		return fmt.Errorf("tool_name 不能为空")
	}
	known_tool := false
	for _, tool := range mcpserver.ToolCatalog() {
		if tool.Name == tool_name {
			known_tool = true
			break
		}
	}
	if !known_tool {
		return fmt.Errorf("tool_name 不支持: %s", tool_name)
	}
	if arguments, exists := config["arguments"]; exists && arguments != nil {
		if _, ok := arguments.(map[string]interface{}); !ok {
			return fmt.Errorf("arguments 必须是 JSON 对象")
		}
	}
	if input_map_value, exists := config["input_map"]; exists && input_map_value != nil {
		input_map, ok := input_map_value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("input_map 必须是 JSON 对象")
		}
		for argument_name, context_key := range input_map {
			if strings.TrimSpace(argument_name) == "" {
				return fmt.Errorf("input_map 参数名不能为空")
			}
			context_key_text, ok := context_key.(string)
			if !ok || strings.TrimSpace(context_key_text) == "" {
				return fmt.Errorf("input_map.%s 必须是上下文键字符串", argument_name)
			}
		}
	}
	if output_key, exists := config["output_key"]; exists && output_key != nil {
		output_key_text, ok := output_key.(string)
		if !ok || strings.TrimSpace(output_key_text) == "" {
			return fmt.Errorf("output_key 必须是非空字符串")
		}
	}
	if timeout_value, exists := config["timeout_seconds"]; exists && timeout_value != nil {
		valid_timeout := false
		switch timeout_seconds := timeout_value.(type) {
		case int:
			valid_timeout = timeout_seconds > 0
		case int64:
			valid_timeout = timeout_seconds > 0
		case float64:
			valid_timeout = timeout_seconds > 0
		}
		if !valid_timeout {
			return fmt.Errorf("timeout_seconds 必须是正数")
		}
	}
	return nil
}

// SortedUserFlowNodeCatalog keeps a stable order for the editor picker.
func SortedUserFlowNodeCatalog() []UserFlowNodeCatalogItem {
	catalog := user_flow_node_catalog()
	sort.SliceStable(catalog, func(i, j int) bool {
		return catalog[i].Type < catalog[j].Type
	})
	return catalog
}

// TriggerFlowDirect runs any user pipeline immediately as a manual/debug
// execution. The flow's persisted Cron/Event trigger configuration remains
// unchanged and continues to control automatic executions. initial_data, when
// non-empty, seeds the flow context with the caller-supplied input parameters.
func (s *AutomationService) TriggerFlowDirect(flow_id string, initial_data map[string]interface{}) (*model.FlowRunRecord, error) {
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
	encoded_initial_data, err := marshal_initial_data(initial_data)
	if err != nil {
		return nil, err
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
		InitialData: encoded_initial_data,
		TimeoutSec:  default_schedule_timeout_sec,
	}, model.FlowRunTriggerManual, ""), nil
}
