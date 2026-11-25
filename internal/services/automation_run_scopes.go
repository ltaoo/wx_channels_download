package services

import (
	"encoding/json"
	"strings"

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

// automation_run_node_states is the live, scope-aware node state map of one
// run. Status events update it in memory; it rides along with the next
// checkpoint write instead of costing a database round trip per node.
type automation_run_node_states struct {
	states     map[string]map[string]string
	definition *automation_run_scope_definition
}

// prime_run_node_states snapshots the initial states created by prepare_run so
// later status events can be cached without reading the run back from the
// database. Runs are only cached after priming, so late events for finished
// runs cannot leak entries.
func (s *AutomationService) prime_run_node_states(run_id, node_states, definition string) {
	if s == nil || !strings.HasPrefix(run_id, "run-") {
		return
	}
	states := map[string]map[string]string{}
	if err := json.Unmarshal([]byte(node_states), &states); err != nil || states == nil {
		states = map[string]map[string]string{}
	}
	entry := &automation_run_node_states{states: states}
	if strings.TrimSpace(definition) != "" {
		var scoped automation_run_scope_definition
		if json.Unmarshal([]byte(definition), &scoped) == nil {
			entry.definition = &scoped
		}
	}
	s.node_states_mu.Lock()
	defer s.node_states_mu.Unlock()
	if s.node_states == nil {
		s.node_states = map[string]*automation_run_node_states{}
	}
	s.node_states[run_id] = entry
}

// cache_node_status folds one live status event into memory. Subflow starts
// carry a full state map and reset the downstream scopes of their children.
func (s *AutomationService) cache_node_status(status flowengine.NodeExecutionStatus) {
	if s == nil || !strings.HasPrefix(status.RunID, "run-") {
		return
	}
	s.node_states_mu.Lock()
	defer s.node_states_mu.Unlock()
	entry := s.node_states[status.RunID]
	if entry == nil {
		return
	}
	current := entry.states[status.FlowID]
	if current == nil || (status.NodeStates != nil && status.NodeID == "") {
		current = map[string]string{}
	}
	for node_id, state := range status.NodeStates {
		current[node_id] = string(state)
	}
	if status.NodeID != "" {
		current[status.NodeID] = string(status.Status)
	}
	entry.states[status.FlowID] = current
	if status.NodeStates == nil || entry.definition == nil {
		return
	}
	scope, _ := entry.definition.find(status.FlowID)
	if scope == nil {
		return
	}
	for node_id, state := range status.NodeStates {
		if child := scope.Nodes[node_id].Config.Workflow; child != nil && state == flowengine.StatePending {
			reset_automation_scope_states(entry.states, child)
		}
	}
}

func reset_automation_scope_states(states map[string]map[string]string, child *automation_run_scope_definition) {
	pending := map[string]string{}
	for node_id, node := range child.Nodes {
		pending[node_id] = "PENDING"
		if node.Config.Workflow != nil {
			reset_automation_scope_states(states, node.Config.Workflow)
		}
	}
	states[child.ID] = pending
}

// node_states_snapshot renders the cached states for the next run-row write.
func (s *AutomationService) node_states_snapshot(run_id string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.node_states_mu.Lock()
	defer s.node_states_mu.Unlock()
	entry := s.node_states[run_id]
	if entry == nil {
		return "", false
	}
	return marshal_json(entry.states), true
}

// node_states_for_scope exposes fresh states to the detail endpoint without a
// database read. The bool is false when the run has no live cache entry.
func (s *AutomationService) node_states_for_scope(run_id, scope_id string) (map[string]string, bool) {
	if s == nil {
		return nil, false
	}
	s.node_states_mu.Lock()
	defer s.node_states_mu.Unlock()
	entry := s.node_states[run_id]
	if entry == nil {
		return nil, false
	}
	states := entry.states[scope_id]
	if states == nil {
		return map[string]string{}, true
	}
	copied := make(map[string]string, len(states))
	for node_id, state := range states {
		copied[node_id] = state
	}
	return copied, true
}

func (s *AutomationService) forget_run_node_states(run_id string) {
	if s == nil {
		return
	}
	s.node_states_mu.Lock()
	delete(s.node_states, run_id)
	s.node_states_mu.Unlock()
}
