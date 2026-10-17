package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/pkg/flowengine/engine"
)

// PipelineEditorDraft replaces a live working copy, without saving or running it.
// Nil Triggers preserves the currently edited trigger forms.
type PipelineEditorDraft struct {
	StartNodeID   string                 `json:"start_node_id"`
	Nodes         []UserFlowNodeInput    `json:"nodes"`
	Triggers      []UserFlowTriggerInput `json:"triggers"`
	ContextSchema []engine.FieldSchema   `json:"context_schema,omitempty"`
}

func ValidatePipelineEditorDraft(draft *PipelineEditorDraft) error {
	if len(draft.Nodes) == 0 || len(draft.Nodes) > 500 {
		return fmt.Errorf("草稿节点数量必须为 1 到 500")
	}
	definition := &engine.FlowDefinition{Name: "Editor draft", ContextSchema: draft.ContextSchema}
	if err := apply_user_flow_update(definition, UpdateUserFlowInput{Nodes: draft.Nodes, StartNodeID: draft.StartNodeID}); err != nil {
		return err
	}
	if definition.Nodes[definition.StartNodeID].Type != "StartNode" {
		return fmt.Errorf("start_node_id 必须指向开始节点")
	}
	draft.StartNodeID = definition.StartNodeID
	for index := range draft.Nodes {
		node := &draft.Nodes[index]
		normalized := definition.Nodes[strings.TrimSpace(node.ID)]
		node.ID = normalized.ID
		node.Config = normalized.Config
		node.NextIDs = normalized.NextNodeIDs
	}
	for _, node := range draft.Nodes {
		if node.Type != "GatewayNode" {
			continue
		}
		raw, _ := json.Marshal(node.Config["rules"])
		var rules []struct {
			TargetID string `json:"target_id"`
		}
		if json.Unmarshal(raw, &rules) != nil {
			return fmt.Errorf("节点 %s 分支规则无效", node.ID)
		}
		for _, rule := range rules {
			if _, ok := definition.Nodes[rule.TargetID]; !ok {
				return fmt.Errorf("节点 %s 分支目标不存在: %s", node.ID, rule.TargetID)
			}
		}
	}
	return nil
}
