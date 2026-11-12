package services

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/mcpserver"
	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/flowengine/engine"
	"wx_channel/pkg/flowengine/nodes"
)

const (
	user_flow_start_node_id = "start"
	user_flow_id_prefix     = "flow-u-"
)

// user_flow_node_catalog lists the node types a user pipeline may contain.
// Only JSON-config driven node types are exposed; FuncNode is bound to Go
// functions and therefore not user-constructible.
type UserFlowNodeCatalogItem struct {
	Type          string                   `json:"type"`
	Name          string                   `json:"name"`
	Description   string                   `json:"description"`
	ConfigKeys    []UserFlowConfigKey      `json:"config_keys"`
	Tools         []mcpserver.Definition   `json:"tools,omitempty"`
	DefaultConfig map[string]any           `json:"default_config,omitempty"`
	ConfigSchema  []engine.NodeConfigField `json:"config_schema,omitempty"`
}

type UserFlowConfigKey struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

func user_flow_node_catalog() []UserFlowNodeCatalogItem {
	return append(automation_additional_node_catalog(), []UserFlowNodeCatalogItem{
		{
			Type:        "ExprNode",
			Name:        "表达式计算",
			Description: "对流程上下文执行表达式并写回结果",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "expression", Type: "string", Required: true, Description: "表达式，只能读 input.<键> / output.<节点id>.<键> / global.<键>，如 len(output.fetch.videos) > 0"},
				{Key: "output_key", Type: "string", Required: false, Description: "结果写入上下文的键，默认 calc_out"},
			},
		},
		{
			Type:        "APICallNode",
			Name:        "HTTP 请求",
			Description: "调用外部 HTTP 接口，可携带上下文参数",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "url", Type: "string", Required: true, Description: "请求地址"},
				{Key: "method", Type: "string", Required: false, Description: "GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS，默认 GET"},
				{Key: "headers", Type: "object", Description: "请求头对象或引用对象的完整作用域模板"},
				{Key: "query", Type: "object", Description: "查询参数，支持作用域模板"},
				{Key: "body", Type: "any", Description: "请求体，支持嵌套模板"},
				{Key: "body_template", Type: "string", Description: "JSON 模板文本或引用配置字符串的完整模板，先解析 JSON 再渲染，优先于 body"},
				{Key: "body_type", Type: "string", Description: "json（默认）、form 或 raw"},
				{Key: "authentication", Type: "object", Description: "{type: basic, username, password} 或 {type: bearer, token}"},
				{Key: "timeout_seconds", Type: "number", Description: "节点超时，默认 15 秒"},
				{Key: "fail_on_status", Type: "boolean", Description: "默认 true，非 2xx 走重试/错误分支"},
				{Key: "output_key", Type: "string", Description: "输出响应体，另含 _status 和 _headers"},
				{Key: "keys", Type: "object", Required: false, Description: "请求参数名到作用域路径的对象映射，如 {\"username\": \"output.cleanup.username\"}"},
			},
		},
		{
			Type:        "JSCodeNode",
			Name:        "执行 JS",
			Description: "用 JavaScript 处理流程上下文，返回值写入 output_key",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "code", Type: "string", Required: true, Description: "JavaScript 代码，可通过 input / output / global 访问三个作用域"},
				{Key: "output_key", Type: "string", Required: true, Description: "结果写入上下文的键，下游用 output.<节点id>.<键> 读取"},
			},
		},
		{
			Type:        "GatewayNode",
			Name:        "条件分支",
			Description: "按条件表达式路由到不同节点",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "gateway_type", Type: "string", Required: false, Description: "Exclusive 或 Parallel"},
				{Key: "join_node_id", Type: "string", Description: "Parallel 分支共同汇聚节点，全部成功后只执行一次"},
				{Key: "condition_language", Type: "string", Required: false, Description: "条件语言：expr（默认）或 js。js 下只能访问 input./output./global. 作用域和 Math/JSON/Number 等内置对象，按 JS 真值判断（非空字符串/对象/数组为真，而 expr 只认 bool），单条条件超过 1 秒会被中断"},
				{Key: "rules", Type: "array", Required: false, Description: "Exclusive 规则列表 [{condition, target_id}]，condition 按 condition_language 求值"},
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
				{Key: "arguments", Type: "object", Required: false, Description: "传给 tool 的参数；值可用 {{input.<键>}} / {{output.<节点id>.<键>}} / {{global.<键>}} 模板"},
				{Key: "output_key", Type: "string", Required: false, Description: "结构化结果写入上下文的键，默认 service_result"},
				{Key: "timeout_seconds", Type: "number", Required: false, Description: "节点调用超时时间；不填时由具体 tool 控制"},
			},
			Tools: mcpserver.ToolCatalog(),
		},
		{
			Type:        "SetVariableNode",
			Name:        "设置全局变量",
			Description: "写入显式全局变量，下游用 {{global.<键>}} 读取",
			ConfigKeys: []UserFlowConfigKey{
				{Key: "variables", Type: "object", Required: true, Description: "变量名到值的对象映射，值可用作用域模板"},
			},
		},
		{
			Type:        "EndNode",
			Name:        "结束",
			Description: "流程结束节点",
		},
	}...)
}

// CreateUserFlowInput describes a new user pipeline. Creating a pipeline always
// produces a flow with a single start node; subsequent nodes are added through
// the editor (UpdateUserFlowInstance).
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
	Subflows           map[string]engine.FlowDefinition
	NodeConfigs        map[string]map[string]interface{}
	DefaultNodeConfigs map[string]map[string]interface{}
	Name               *string
	Description        *string
	ContextSchema      []engine.FieldSchema
	StartNodeID        string
	Nodes              []UserFlowNodeInput
	// Nil preserves existing rules; an empty slice removes all extra triggers.
	Triggers         []UserFlowTriggerInput
	InheritTriggers  *bool
	TriggerOverrides map[string]map[string]interface{}
}

type UserFlowNodeInput struct {
	ConfigSchema    []engine.NodeConfigField `json:"config_schema"`
	RetryPolicy     *engine.RetryPolicy      `json:"retry,omitempty"`
	ErrorNextNodeID *string                  `json:"error_next_node,omitempty"`
	OutputSchema    []engine.FieldSchema     `json:"output_schema,omitempty"`

	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Config      map[string]interface{} `json:"config"`
	Position    *engine.NodePosition   `json:"position,omitempty"`
	InputSchema []engine.FieldSchema   `json:"input_schema"`
	NextIDs     []string               `json:"next_ids"`
}

func (s *AutomationService) load_user_flows(flow_ids ...string) {
	if s == nil || s.db == nil || s.flow_engine == nil {
		return
	}
	var flows []model.UserFlowDefinition
	query := s.db.Where("deleted_at IS NULL")
	if len(flow_ids) > 0 {
		query = query.Where("id IN ?", flow_ids)
	}
	if err := query.Find(&flows).Error; err != nil {
		s.log_error(err, "load flow definitions")
		return
	}
	for _, flow := range flows {
		definition, err := decode_user_flow_definition(flow.Definition)
		if err != nil {
			s.log_error(err, "decode flow definition")
			continue
		}
		if s.custom_nodes.ApplyConfigSchemas(definition) {
			if err := s.db.Model(&flow).Update("definition", marshal_json(definition)).Error; err != nil {
				s.log_error(err, "refresh node schemas")
				continue
			}
		}
		s.register_user_flow(*definition)
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
	s.flow_engine.RegisterFlow(definition)
}

func normalize_user_flow_trigger(trigger_type string, event_key string) (string, string, error) {
	trigger_type = strings.TrimSpace(trigger_type)
	switch trigger_type {
	case "":
		trigger_type = model.FlowRunTriggerCron
	case model.FlowRunTriggerCron, model.FlowRunTriggerEvent, model.FlowRunTriggerManual, model.FlowRunTriggerWebhook:
	default:
		return "", "", fmt.Errorf("触发类型不支持: %s", trigger_type)
	}
	event_key = strings.TrimSpace(event_key)
	if trigger_type == model.FlowRunTriggerEvent && event_key == "" {
		return "", "", fmt.Errorf("事件触发必须提供 event_key")
	}
	return trigger_type, event_key, nil
}

// CreateFlowDefinition creates only a reusable flow, without an execution instance.
func (s *AutomationService) CreateFlowDefinition(input CreateUserFlowInput) (*model.UserFlowDefinition, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("name 不能为空")
	}
	now := time.Now().UnixMilli()
	flow := model.UserFlowDefinition{ID: fmt.Sprintf("%s%d", user_flow_id_prefix, time.Now().UnixNano()), Name: name, Description: strings.TrimSpace(input.Description), DefaultNodeConfigs: "{}", Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now}}
	definition := engine.FlowDefinition{ID: flow.ID, Name: name, StartNodeID: user_flow_start_node_id, ContextSchema: input.ContextSchema, Nodes: map[string]engine.NodeDefinition{user_flow_start_node_id: {ID: user_flow_start_node_id, Type: "StartNode", Name: "开始"}}}
	flow.Definition = marshal_json(definition)
	if err := s.db.Create(&flow).Error; err != nil {
		return nil, err
	}
	s.register_user_flow(definition)
	return &flow, nil
}

func (s *AutomationService) ImportFlowDefinition(raw string) (*model.UserFlowDefinition, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	definition, err := decode_user_flow_definition(raw)
	if err != nil {
		return nil, err
	}
	if err := normalize_user_flow_definition(definition, s.custom_nodes); err != nil {
		return nil, err
	}
	var envelope struct {
		Triggers           []UserFlowTriggerInput            `json:"triggers"`
		DefaultNodeConfigs map[string]map[string]interface{} `json:"default_node_configs"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, err
	}
	triggers, err := s.normalize_default_triggers(envelope.Triggers)
	if err != nil {
		return nil, err
	}
	defaults, err := normalize_node_configs(*definition, envelope.DefaultNodeConfigs, true, false)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	definition.ID = fmt.Sprintf("%s%d", user_flow_id_prefix, time.Now().UnixNano())
	clear_runtime_node_configs(definition)
	flow := model.UserFlowDefinition{ID: definition.ID, Name: definition.Name, Definition: marshal_json(definition), DefaultNodeConfigs: marshal_json(defaults), DefaultTriggers: marshal_json(triggers), Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now}}
	if err := s.db.Create(&flow).Error; err != nil {
		return nil, err
	}
	s.register_user_flow(*definition)
	return &flow, nil
}

// normalize_user_flow_definition validates an imported definition and rewrites
// each node so its id, config["id"], NextNodeIDs and NextNodes stay consistent.
// The engine drives ordinary nodes via NextNodes but ManualNode via NextNodeIDs,
// so both representations must agree for an imported graph to run correctly.
func normalize_user_flow_definition(definition *engine.FlowDefinition, custom_nodes ...*flowengine.CustomNodeRegistry) error {
	return normalize_user_flow_definition_at_depth(definition, 0, custom_nodes...)
}

func normalize_user_flow_definition_at_depth(definition *engine.FlowDefinition, depth int, custom_nodes ...*flowengine.CustomNodeRegistry) error {
	if len(custom_nodes) > 0 {
		custom_nodes[0].ApplyConfigSchemas(definition)
	}
	if depth > 16 {
		return fmt.Errorf("子流程嵌套不能超过 16 层")
	}
	if len(definition.Nodes) > 500 {
		return fmt.Errorf("流程节点数量不能超过 500")
	}
	if len(definition.Subflows) > 500 {
		return fmt.Errorf("子流程数量不能超过 500")
	}
	for id, child := range definition.Subflows {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("子流程 id 不能为空")
		}
		if child.ID == "" {
			child.ID = id
		}
		if child.Name == "" {
			child.Name = id
		}
		if err := normalize_user_flow_definition_at_depth(&child, depth+1, custom_nodes...); err != nil {
			return fmt.Errorf("子流程 %s: %w", id, err)
		}
		definition.Subflows[id] = child
	}

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
		if err := validate_node_config_schema(node.ConfigSchema); err != nil {
			return fmt.Errorf("节点 %s 配置声明无效: %w", node_id, err)
		}
		node_id = strings.TrimSpace(node_id)
		if node_id == "" {
			return fmt.Errorf("节点 id 不能为空")
		}
		if !user_flow_allows_node_type(node.Type, custom_nodes...) {
			return fmt.Errorf("节点类型不支持: %s", node.Type)
		}
		if node.Config == nil {
			node.Config = map[string]interface{}{}
		}
		if err := validate_user_flow_node_config(node.Type, node.Config, custom_nodes...); err != nil {
			return fmt.Errorf("节点 %s 配置无效: %w", node_id, err)
		}
		if err := validate_node_execution_policy(node); err != nil {
			return fmt.Errorf("节点 %s: %w", node_id, err)
		}
		if node.Type == "WorkflowNode" || node.Type == "LoopNode" {
			if raw, exists := node.Config["workflow"]; exists {
				encoded, err := json.Marshal(raw)
				if err != nil {
					return err
				}
				child := engine.FlowDefinition{}
				if err := json.Unmarshal(encoded, &child); err != nil {
					return err
				}
				if child.Name == "" {
					child.Name = node_id + " 子流程"
				}
				if child.ID == "" {
					child.ID = definition.ID + ":" + node_id
				}
				if err := normalize_user_flow_definition_at_depth(&child, depth+1, custom_nodes...); err != nil {
					return fmt.Errorf("子流程 %s: %w", node_id, err)
				}
				for _, child_node := range child.Nodes {
					if child_node.Type == "ManualNode" {
						return fmt.Errorf("子流程或循环内暂不支持人工等待，请在主流程中添加人工确认")
					}
				}
				node.Config["workflow"] = child
			}
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
		if node.ErrorNextNodeID != "" {
			if _, ok := normalized[node.ErrorNextNodeID]; !ok {
				return fmt.Errorf("节点 %s 的错误分支不存在: %s", node_id, node.ErrorNextNodeID)
			}
		}
		if node.Type == "GatewayNode" {
			if join_id, _ := node.Config["join_node_id"].(string); join_id != "" {
				if _, ok := normalized[join_id]; !ok {
					return fmt.Errorf("并行汇聚节点不存在: %s", join_id)
				}
			}
			raw, _ := json.Marshal(node.Config["rules"])
			var rules []struct {
				TargetID string `json:"target_id"`
			}
			if node.Config["rules"] != nil && json.Unmarshal(raw, &rules) != nil {
				return fmt.Errorf("节点 %s 的分支规则必须为数组", node_id)
			}
			for _, rule := range rules {
				if _, ok := normalized[rule.TargetID]; !ok {
					return fmt.Errorf("节点 %s 的分支目标不存在: %s", node_id, rule.TargetID)
				}
			}
		}
		for _, next_id := range node.NextNodeIDs {
			if _, ok := normalized[next_id]; !ok {
				return fmt.Errorf("节点 %s 指向了不存在的节点: %s", node_id, next_id)
			}
		}
	}
	definition.Name = name
	definition.StartNodeID = start_node_id
	definition.Nodes = normalized
	return nodes.ValidateFlowScopes(*definition)
}

func (s *AutomationService) flow_definition_query() *gorm.DB {
	return s.db.Model(&model.UserFlowDefinition{}).Select("user_flow.*, (SELECT COUNT(*) FROM user_flow_instance WHERE flow_id=user_flow.id AND deleted_at IS NULL) AS instance_count")
}
func (s *AutomationService) GetFlowDefinition(id string) (*model.UserFlowDefinition, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	var flow model.UserFlowDefinition
	if err := s.flow_definition_query().Where("user_flow.id=? AND user_flow.deleted_at IS NULL", strings.TrimSpace(id)).First(&flow).Error; err != nil {
		return nil, fmt.Errorf("流程不存在: %s", id)
	}
	return &flow, nil
}
func (s *AutomationService) ListFlowDefinitions() ([]model.UserFlowDefinition, error) {
	flows := make([]model.UserFlowDefinition, 0)
	if err := s.flow_definition_query().Where("user_flow.deleted_at IS NULL").Order("user_flow.created_at DESC").Find(&flows).Error; err != nil {
		return nil, err
	}
	return flows, nil
}
func (s *AutomationService) UpdateFlowDefinition(id string, input UpdateUserFlowInput) (*model.UserFlowDefinition, error) {
	flow, err := s.GetFlowDefinition(id)
	if err != nil {
		return nil, err
	}
	definition, err := decode_user_flow_definition(flow.Definition)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		flow.Name = strings.TrimSpace(*input.Name)
		if flow.Name == "" {
			return nil, fmt.Errorf("name 不能为空")
		}
		definition.Name = flow.Name
	}
	if input.Description != nil {
		flow.Description = strings.TrimSpace(*input.Description)
	}
	if input.ContextSchema != nil {
		definition.ContextSchema = input.ContextSchema
	}
	if err := apply_user_flow_update(definition, input, s.custom_nodes); err != nil {
		return nil, err
	}
	defaults := decode_node_configs(flow.DefaultNodeConfigs)
	if input.DefaultNodeConfigs != nil {
		defaults = input.DefaultNodeConfigs
	}
	defaults, err = normalize_node_configs(*definition, defaults, input.DefaultNodeConfigs != nil, false)
	if err != nil {
		return nil, err
	}
	clear_runtime_node_configs(definition)
	flow.Definition = marshal_json(definition)
	flow.DefaultNodeConfigs = marshal_json(defaults)
	if input.Triggers != nil {
		triggers, err := s.normalize_default_triggers(input.Triggers)
		if err != nil {
			return nil, err
		}
		flow.DefaultTriggers = marshal_json(triggers)
	}
	flow.UpdatedAt = time.Now().UnixMilli()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(flow).Updates(map[string]interface{}{"name": flow.Name, "description": flow.Description, "definition": flow.Definition, "default_node_configs": flow.DefaultNodeConfigs, "default_triggers": flow.DefaultTriggers, "updated_at": flow.UpdatedAt}).Error; err != nil {
			return err
		}
		var instances []model.UserFlowInstance
		if err := tx.Where("flow_id=? AND deleted_at IS NULL", id).Find(&instances).Error; err != nil {
			return err
		}
		for _, instance := range instances {
			if instance.InheritTriggers {
				if err := s.sync_instance_default_triggers(tx, instance.ID, definition.StartNodeID, flow.DefaultTriggers); err != nil {
					return err
				}
			}
			values, err := normalize_node_configs(*definition, decode_node_configs(instance.NodeConfigs), false, false)
			if err != nil {
				return err
			}
			if err := tx.Model(&instance).Updates(map[string]interface{}{"node_configs": marshal_json(values), "updated_at": flow.UpdatedAt}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.load_user_flows(flow.ID)
	return s.GetFlowDefinition(flow.ID)
}
func (s *AutomationService) CopyFlowDefinition(id string) (*model.UserFlowDefinition, error) {
	source, err := s.GetFlowDefinition(id)
	if err != nil {
		return nil, err
	}
	definition, err := decode_user_flow_definition(source.Definition)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	source.ID = fmt.Sprintf("%s%d", user_flow_id_prefix, time.Now().UnixNano())
	source.Name += "（副本）"
	source.InstanceCount = 0
	source.Timestamps = model.Timestamps{CreatedAt: now, UpdatedAt: now}
	definition.ID, definition.Name = source.ID, source.Name
	source.Definition = marshal_json(definition)
	if err := s.db.Create(source).Error; err != nil {
		return nil, err
	}
	s.register_user_flow(*definition)
	return source, nil
}
func (s *AutomationService) DeleteFlowDefinition(id string) error {
	flow, err := s.GetFlowDefinition(id)
	if err != nil {
		return err
	}
	if flow.InstanceCount > 0 {
		return fmt.Errorf("请先删除此流程的实例")
	}
	now := time.Now().UnixMilli()
	if err := s.db.Model(flow).Updates(map[string]interface{}{"deleted_at": now, "updated_at": now}).Error; err != nil {
		return err
	}
	if s.flow_engine != nil {
		s.flow_engine.RemoveFlow(flow.ID)
	}
	return nil
}
func (s *AutomationService) user_flow_query() *gorm.DB {
	return s.db.Model(&model.UserFlowInstance{}).Joins("JOIN user_flow ON user_flow.id=user_flow_instance.flow_id AND user_flow.deleted_at IS NULL").Select("user_flow_instance.*,user_flow.definition,user_flow.name AS definition_name,user_flow.default_node_configs,user_flow.default_triggers,COALESCE((SELECT id FROM flow_run_record WHERE instance_id=user_flow_instance.id AND deleted_at IS NULL AND status IN ('QUEUED','RUNNING','WAITING') ORDER BY created_at DESC LIMIT 1),'') AS active_run_id")
}
func (s *AutomationService) GetUserFlowInstance(id string) (*model.UserFlowInstance, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	var instance model.UserFlowInstance
	if err := s.user_flow_query().Where("user_flow_instance.id=? AND user_flow_instance.deleted_at IS NULL", strings.TrimSpace(id)).First(&instance).Error; err != nil {
		return nil, fmt.Errorf("流程实例不存在: %s", id)
	}
	return &instance, nil
}
func (s *AutomationService) ListUserFlowInstances() ([]model.UserFlowInstance, error) {
	instances := make([]model.UserFlowInstance, 0)
	// Lists do not repeat the shared graph for every configuration instance.
	if err := s.user_flow_query().Select("user_flow_instance.*,user_flow.name AS definition_name,user_flow.default_node_configs,user_flow.default_triggers,COALESCE((SELECT id FROM flow_run_record WHERE instance_id=user_flow_instance.id AND deleted_at IS NULL AND status IN ('QUEUED','RUNNING','WAITING') ORDER BY created_at DESC LIMIT 1),'') AS active_run_id").Where("user_flow_instance.deleted_at IS NULL").Order("user_flow_instance.created_at DESC").Find(&instances).Error; err != nil {
		return nil, err
	}
	return instances, nil
}
func (s *AutomationService) CreateUserFlowInstance(flow_id string) (*model.UserFlowInstance, error) {
	flow, err := s.GetFlowDefinition(flow_id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	instance := model.UserFlowInstance{ID: fmt.Sprintf("instance-u-%d", time.Now().UnixNano()), FlowID: flow.ID, Name: flow.Name + "（实例）", NodeConfigs: "{}", InheritTriggers: true, TriggerType: model.FlowRunTriggerManual, Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now}}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&instance).Error; err != nil {
			return err
		}
		definition, err := decode_user_flow_definition(flow.Definition)
		if err != nil {
			return err
		}
		return s.sync_instance_default_triggers(tx, instance.ID, definition.StartNodeID, flow.DefaultTriggers)
	}); err != nil {
		return nil, err
	}
	return s.GetUserFlowInstance(instance.ID)
}
func (s *AutomationService) CopyUserFlowInstance(id string) (*model.UserFlowInstance, error) {
	var copied model.UserFlowInstance
	err := s.db.Transaction(func(tx *gorm.DB) error {
		scoped := &AutomationService{db: tx, cron_parser: s.cron_parser, custom_nodes: s.custom_nodes}
		source, err := scoped.GetUserFlowInstance(id)
		if err != nil {
			return err
		}
		copied = *source
		copied.ID = fmt.Sprintf("instance-u-%d", time.Now().UnixNano())
		copied.Name += "（副本）"
		copied.ActiveRunID = ""
		now := time.Now().UnixMilli()
		copied.Timestamps = model.Timestamps{CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&copied).Error; err != nil {
			return err
		}
		var schedules []model.FlowSchedule
		if err := tx.Where("instance_id=? AND deleted_at IS NULL", id).Find(&schedules).Error; err != nil {
			return err
		}
		for _, schedule := range schedules {
			data := decode_initial_data(schedule.InitialData)
			if metadata, ok := data[automation_metadata_key].(map[string]interface{}); ok && metadata["type"] == model.FlowRunTriggerWebhook {
				delete(metadata, "webhook_token")
			}
			if _, err := scoped.CreateSchedule(CreateScheduleInput{Name: schedule.Name, Description: schedule.Description, InstanceID: copied.ID, CronExpr: schedule.CronExpr, InitialData: data, Enabled: &schedule.Enabled, TimeoutSec: schedule.TimeoutSec}); err != nil {
				return err
			}
		}
		return tx.Model(&copied).Update("inherit_triggers", source.InheritTriggers).Error
	})
	if err != nil {
		return nil, err
	}
	return s.GetUserFlowInstance(copied.ID)
}

// Instance settings and shared graph updates have separate persisted ownership.
func (s *AutomationService) UpdateUserFlowInstance(id string, input UpdateUserFlowInput) (*model.UserFlowInstance, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		scoped := &AutomationService{db: tx, cron_parser: s.cron_parser, custom_nodes: s.custom_nodes}
		instance, err := scoped.GetUserFlowInstance(id)
		if err != nil {
			return err
		}
		if input.ContextSchema != nil || input.Subflows != nil || len(input.Nodes) > 0 || input.DefaultNodeConfigs != nil {
			graph_input := input
			graph_input.Name = nil
			graph_input.Description = nil
			graph_input.NodeConfigs = nil
			graph_input.Triggers = nil
			if _, err := scoped.UpdateFlowDefinition(instance.FlowID, graph_input); err != nil {
				return err
			}
			instance, err = scoped.GetUserFlowInstance(id)
			if err != nil {
				return err
			}
		}
		definition, err := decode_user_flow_definition(instance.Definition)
		if err != nil {
			return err
		}
		values := decode_node_configs(instance.NodeConfigs)
		if input.NodeConfigs != nil {
			values = input.NodeConfigs
		}
		values, err = normalize_node_configs(*definition, values, input.NodeConfigs != nil, false)
		if err != nil {
			return err
		}
		if input.NodeConfigs != nil {
			if _, err := normalize_node_configs(*definition, merge_node_configs(decode_node_configs(instance.DefaultNodeConfigs), values), true, true); err != nil {
				return err
			}
		}
		if input.Name != nil {
			instance.Name = strings.TrimSpace(*input.Name)
			if instance.Name == "" {
				return fmt.Errorf("name 不能为空")
			}
		}
		if input.Description != nil {
			instance.Description = strings.TrimSpace(*input.Description)
		}
		if input.TriggerOverrides != nil {
			if input.Triggers != nil || input.InheritTriggers != nil {
				return fmt.Errorf("trigger_overrides 不能与 triggers 或 inherit_triggers 同时提交")
			}
			var defaults []UserFlowTriggerInput
			if err := json.Unmarshal([]byte(instance.DefaultTriggers), &defaults); err != nil {
				return err
			}
			_, overrides, err := scoped.merge_trigger_overrides(defaults, input.TriggerOverrides, true)
			if err != nil {
				return err
			}
			instance.TriggerOverrides = marshal_json(overrides)
			if err := tx.Model(instance).Update("trigger_overrides", instance.TriggerOverrides).Error; err != nil {
				return err
			}
			if err := scoped.sync_instance_default_triggers(tx, id, definition.StartNodeID, instance.DefaultTriggers); err != nil {
				return err
			}
			instance.InheritTriggers = true
		} else if input.InheritTriggers != nil && *input.InheritTriggers {
			if input.Triggers != nil {
				return fmt.Errorf("继承流程触发器时不能同时提交自定义触发器")
			}
			instance.TriggerOverrides = "{}"
			if err := tx.Model(instance).Update("trigger_overrides", instance.TriggerOverrides).Error; err != nil {
				return err
			}
			if err := scoped.sync_instance_default_triggers(tx, id, definition.StartNodeID, instance.DefaultTriggers); err != nil {
				return err
			}
			instance.InheritTriggers = true
		} else if input.Triggers != nil {
			if err := scoped.replace_user_flow_triggers(tx, id, definition.StartNodeID, input.Triggers); err != nil {
				return err
			}
			instance.TriggerType = model.FlowRunTriggerManual
			instance.EventKey = ""
			instance.InheritTriggers = false
			instance.TriggerOverrides = "{}"
		} else if input.InheritTriggers != nil {
			instance.InheritTriggers = false
		}
		return tx.Model(instance).Updates(map[string]interface{}{"name": instance.Name, "description": instance.Description, "node_configs": marshal_json(values), "trigger_type": instance.TriggerType, "event_key": instance.EventKey, "inherit_triggers": instance.InheritTriggers, "trigger_overrides": instance.TriggerOverrides, "updated_at": time.Now().UnixMilli()}).Error
	})
	if err != nil {
		return nil, err
	}
	instance, err := s.GetUserFlowInstance(id)
	if err == nil {
		s.load_user_flows(instance.FlowID)
	}
	return instance, err
}

// apply_user_flow_update folds an editor save into definition: it rebuilds the
// node map from the submitted nodes and then runs the shared normalization so
// the save path enforces the same graph and scope contract as an import.
// Skipping that normalization is how a definition the engine only rejects at
// run time could get persisted.
func apply_user_flow_update(definition *engine.FlowDefinition, input UpdateUserFlowInput, custom_nodes ...*flowengine.CustomNodeRegistry) error {
	if input.Subflows != nil {
		definition.Subflows = input.Subflows
	}
	if len(input.Nodes) > 0 {
		rebuilt := map[string]engine.NodeDefinition{}
		for _, node := range input.Nodes {
			node_id := strings.TrimSpace(node.ID)
			if node_id == "" {
				return fmt.Errorf("节点 id 不能为空")
			}
			if !user_flow_allows_node_type(node.Type, custom_nodes...) {
				return fmt.Errorf("节点类型不支持: %s", node.Type)
			}
			if _, exists := rebuilt[node_id]; exists {
				return fmt.Errorf("节点 id 重复: %s", node_id)
			}
			config := node.Config
			if config == nil {
				config = map[string]interface{}{}
			}
			if err := validate_user_flow_node_config(node.Type, config, custom_nodes...); err != nil {
				return fmt.Errorf("节点 %s 配置无效: %w", node_id, err)
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
			previous := definition.Nodes[node_id]
			config_schema := node.ConfigSchema
			if config_schema == nil {
				config_schema = previous.ConfigSchema
			}
			retry_policy := node.RetryPolicy
			if retry_policy == nil {
				retry_policy = previous.RetryPolicy
			}
			error_next := previous.ErrorNextNodeID
			if node.ErrorNextNodeID != nil {
				error_next = *node.ErrorNextNodeID
			}
			output_schema := node.OutputSchema
			if output_schema == nil {
				output_schema = previous.OutputSchema
			}
			rebuilt[node_id] = engine.NodeDefinition{
				ConfigSchema: config_schema,
				RetryPolicy:  retry_policy, ErrorNextNodeID: error_next, OutputSchema: output_schema,
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
		start_node_id := strings.TrimSpace(input.StartNodeID)
		if start_node_id == "" {
			start_node_id = definition.StartNodeID
		}
		definition.StartNodeID = start_node_id
		definition.Nodes = rebuilt
	}
	return normalize_user_flow_definition(definition, custom_nodes...)
}

// DeleteUserFlowInstance removes an instance and its trigger rules, retaining run history.
func (s *AutomationService) DeleteUserFlowInstance(id string) error {
	if _, err := s.GetUserFlowInstance(id); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.FlowSchedule{}).Where("instance_id=? AND deleted_at IS NULL", id).Updates(map[string]interface{}{"deleted_at": now, "updated_at": now, "enabled": false, "next_run_at": nil}).Error; err != nil {
			return err
		}
		return tx.Model(&model.UserFlowInstance{}).Where("id=? AND deleted_at IS NULL", id).Updates(map[string]interface{}{"deleted_at": now, "updated_at": now}).Error
	})
}

// UserFlowVisualization renders user pipelines in the same graph shape the
// frontend uses for built-in flows.
func (s *AutomationService) UserFlowVisualization(flow_id string) (*flowengine.FlowVisualizationPayload, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	flows, err := s.ListFlowDefinitions()
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
func user_flow_allows_node_type(node_type string, custom_nodes ...*flowengine.CustomNodeRegistry) bool {
	switch node_type {
	case "StartNode", "EndNode", "ExprNode", "GatewayNode", "APICallNode", "ManualNode", "ServiceNode", "JSCodeNode", "SetVariableNode", "LoopNode", "WorkflowNode", "WaitNode", "FailNode":
		return true
	default:
		return len(custom_nodes) > 0 && custom_nodes[0].Has(node_type)
	}
}

func validate_user_flow_node_config(node_type string, config map[string]interface{}, custom_nodes ...*flowengine.CustomNodeRegistry) error {
	if len(custom_nodes) > 0 && custom_nodes[0].Has(node_type) {
		if raw, exists := config["arguments"]; exists {
			if _, ok := raw.(map[string]interface{}); !ok {
				return fmt.Errorf("arguments 必须是 JSON 对象")
			}
		}
		if raw, exists := config["output_key"]; exists {
			if key, ok := raw.(string); !ok || strings.TrimSpace(key) == "" {
				return fmt.Errorf("output_key 必须是非空字符串")
			}
		} else {
			config["output_key"] = "result"
		}
		return node_config_integer(config, "timeout_seconds", 600, 1, 86400)
	}
	if node_type != "ServiceNode" {
		return validate_automation_node_config(node_type, config)
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
func SortedUserFlowNodeCatalog(custom_nodes ...*flowengine.CustomNodeRegistry) []UserFlowNodeCatalogItem {
	catalog := user_flow_node_catalog()
	if len(custom_nodes) > 0 {
		for _, node := range custom_nodes[0].Catalog() {
			keys := make([]UserFlowConfigKey, 0, len(node.ConfigKeys))
			for _, key := range node.ConfigKeys {
				keys = append(keys, UserFlowConfigKey{Key: key.Key, Type: key.Type, Required: key.Required, Description: key.Description})
			}
			catalog = append(catalog, UserFlowNodeCatalogItem{Type: node.Type, Name: node.Name, Description: node.Description, ConfigKeys: keys, DefaultConfig: node.DefaultConfig, ConfigSchema: node.ConfigSchema})
		}
	}
	sort.SliceStable(catalog, func(i, j int) bool {
		return catalog[i].Type < catalog[j].Type
	})
	return catalog
}

func (s *AutomationService) NodeCatalog() []UserFlowNodeCatalogItem {
	if s == nil {
		return SortedUserFlowNodeCatalog()
	}
	return SortedUserFlowNodeCatalog(s.custom_nodes)
}

// TriggerFlowInstance starts any user pipeline immediately as a manual/debug
// execution. The flow's persisted Cron/Event trigger configuration remains
// unchanged and continues to control automatic executions. initial_data, when
// non-empty, seeds the flow context with the caller-supplied input parameters.
//
// The run is committed before launching the execution goroutine, so callers
// receive the real run ID immediately and can query progress without a socket.
func (s *AutomationService) TriggerFlowInstance(flow_id string, initial_data map[string]interface{}) (*model.FlowRunRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	if s.flow_engine == nil {
		return nil, fmt.Errorf("流程引擎未初始化")
	}
	flow, err := s.GetUserFlowInstance(flow_id)
	if err != nil {
		return nil, err
	}
	if _, err := decode_user_flow_definition(flow.Definition); err != nil {
		return nil, err
	}

	schedule := model.FlowSchedule{FlowID: flow.FlowID, InstanceID: flow.ID, CronExpr: "@daily", InitialData: marshal_json(initial_data), TimeoutSec: default_schedule_timeout_sec}
	run, err := s.prepare_run(schedule, model.FlowRunTriggerManual, "", "", "")
	if err != nil {
		return nil, err
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.execute_prepared_run(schedule, run) }()
	return run, nil
}
