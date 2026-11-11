package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine"
)

type automation_run_scope_definition struct {
	ID    string `json:"id"`
	Nodes map[string]struct {
		Config struct {
			Workflow *automation_run_scope_definition `json:"workflow"`
		} `json:"config"`
	} `json:"nodes"`
}

func (definition *automation_run_scope_definition) find(flow_id string) (*automation_run_scope_definition, []string) {
	if definition.ID == flow_id {
		return definition, []string{flow_id}
	}
	for _, node := range definition.Nodes {
		if child := node.Config.Workflow; child != nil {
			if found, path := child.find(flow_id); found != nil {
				return found, append([]string{definition.ID}, path...)
			}
		}
	}
	return nil, nil
}

func automation_run_scope(run model.FlowRunRecord, flow_ids []string) (*automation_run_scope_definition, bool, []string, error) {
	var definition automation_run_scope_definition
	_ = json.Unmarshal([]byte(run.Definition), &definition)
	flow_id := ""
	if len(flow_ids) > 0 {
		flow_id = strings.TrimSpace(flow_ids[0])
	}
	if flow_id == "" || flow_id == run.FlowID || flow_id == definition.ID {
		return &definition, true, []string{run.FlowID, definition.ID}, nil
	}
	if child, path := definition.find(flow_id); child != nil {
		return child, false, append([]string{run.FlowID}, path...), nil
	}
	return nil, false, nil, fmt.Errorf("运行记录中不存在子流程: %s", flow_id)
}

// Keep current scoped states separate from the bounded audit log.
func (s *AutomationService) persist_node_status(status flowengine.NodeExecutionStatus) {
	if s == nil || s.db == nil || !strings.HasPrefix(status.RunID, "run-") {
		return
	}
	s.log_mu.Lock()
	defer s.log_mu.Unlock()
	var run model.FlowRunRecord
	if err := s.db.Select("id", "flow_id", "definition", "node_states").Where("id = ? AND deleted_at IS NULL", status.RunID).First(&run).Error; err != nil {
		s.log_error(err, "load automation node states")
		return
	}
	states := map[string]map[string]string{}
	_ = json.Unmarshal([]byte(run.NodeStates), &states)
	if states == nil {
		states = map[string]map[string]string{}
	}
	current := states[status.FlowID]
	if current == nil || (status.NodeStates != nil && status.NodeID == "") {
		current = map[string]string{}
	}
	for node_id, state := range status.NodeStates {
		current[node_id] = string(state)
	}
	if status.NodeID != "" {
		current[status.NodeID] = string(status.Status)
	}
	states[status.FlowID] = current
	if status.NodeStates != nil {
		if definition, _, _, err := automation_run_scope(run, []string{status.FlowID}); err == nil {
			var reset_children func(*automation_run_scope_definition)
			reset_children = func(child *automation_run_scope_definition) {
				pending := map[string]string{}
				for node_id, node := range child.Nodes {
					pending[node_id] = "PENDING"
					if node.Config.Workflow != nil {
						reset_children(node.Config.Workflow)
					}
				}
				states[child.ID] = pending
			}
			for node_id, state := range status.NodeStates {
				if child := definition.Nodes[node_id].Config.Workflow; child != nil && state == flowengine.StatePending {
					reset_children(child)
				}
			}
		}
	}
	if err := s.db.Model(&model.FlowRunRecord{}).Where("id = ?", run.ID).Update("node_states", marshal_json(states)).Error; err != nil {
		s.log_error(err, "persist automation node states")
	}
}
