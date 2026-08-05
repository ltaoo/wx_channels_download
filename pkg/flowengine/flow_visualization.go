package flowengine

import (
	"fmt"
	"sort"
	"strconv"

	"wx_channel/pkg/flowengine/engine"
)

const (
	defaultFlowVisualizationPurpose = "flow-visualization"
)

// FlowVisualizationOptions declares optional metadata attached to the visualization payload.
type FlowVisualizationOptions struct {
	Platform string
	Purpose  string
	Editable bool
}

// FlowVisualizationPayload is a flow graph export model used for frontend render.
type FlowVisualizationPayload struct {
	Platform string            `json:"platform"`
	Purpose  string            `json:"purpose"`
	Editable bool              `json:"editable"`
	Flows    []FlowVisualization `json:"flows"`
}

// FlowVisualization contains rendered nodes and edges for one flow.
type FlowVisualization struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	StartNodeID   string                   `json:"start_node_id"`
	ContextSchema []engine.FieldSchema      `json:"context_schema"`
	Nodes         []FlowVisualizationNode   `json:"nodes"`
	Edges         []FlowVisualizationEdge   `json:"edges"`
}

// FlowVisualizationNode describes one node in a flow graph.
type FlowVisualizationNode struct {
	ID    string                  `json:"id"`
	Name  string                  `json:"name"`
	Type  string                  `json:"type"`
	Layout FlowVisualizationLayout `json:"layout"`
	InputSchema  []engine.FieldSchema `json:"input_schema"`
	OutputSchema []engine.FieldSchema `json:"output_schema"`
	Rules []FlowVisualizationRule `json:"rules,omitempty"`
	Next  []string                `json:"next_ids,omitempty"`
}

// FlowVisualizationRule describes one branch rule on a GatewayNode.
type FlowVisualizationRule struct {
	Condition string `json:"condition"`
	TargetID  string `json:"target_id"`
}

// FlowVisualizationEdge describes one directed graph edge.
type FlowVisualizationEdge struct {
	ID     string `json:"id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Type   string `json:"type"`
	Label  string `json:"label,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// FlowVisualizationLayout provides optional layout hints for node rendering.
type FlowVisualizationLayout struct {
	Layer int `json:"layer"`
	Order int `json:"order"`
	X     int `json:"x"`
	Y     int `json:"y"`
}

// BuildFlowVisualizationPayload creates a read-only flow graph payload for frontend render.
func BuildFlowVisualizationPayload(flows []engine.FlowDefinition, flowID string, opts FlowVisualizationOptions) (*FlowVisualizationPayload, error) {
	purpose := opts.Purpose
	if purpose == "" {
		purpose = defaultFlowVisualizationPurpose
	}

	result := make([]FlowVisualization, 0, len(flows))
	if flowID == "" {
		for _, flow := range flows {
			result = append(result, buildFlowVisualization(flow))
		}
		return &FlowVisualizationPayload{
			Platform: opts.Platform,
			Purpose:  purpose,
			Editable: opts.Editable,
			Flows:    result,
		}, nil
	}

	for _, flow := range flows {
		if flow.ID == flowID {
			result = append(result, buildFlowVisualization(flow))
			return &FlowVisualizationPayload{
				Platform: opts.Platform,
				Purpose:  purpose,
				Editable: opts.Editable,
				Flows:    result,
			}, nil
		}
	}

	return nil, fmt.Errorf("unsupported flow_id")
}

func buildFlowVisualization(flow engine.FlowDefinition) FlowVisualization {
	nodes := make([]FlowVisualizationNode, 0, len(flow.Nodes))
	edges := make([]FlowVisualizationEdge, 0, len(flow.Nodes)*2)

	for id, node := range flow.Nodes {
		viewNode := FlowVisualizationNode{
			ID:   id,
			Name: node.Name,
			Type: node.Type,
			InputSchema:  node.InputSchema,
			OutputSchema: node.OutputSchema,
		}

		for _, next := range node.NextNodes {
			viewNode.Next = append(viewNode.Next, next.TargetID)
			edges = append(edges, FlowVisualizationEdge{
				ID:     flowVisualizationEdgeID(id, next.TargetID, len(edges)),
				From:   id,
				To:     next.TargetID,
				Type:   "next",
				Reason: "next_nodes",
			})
		}

		if len(node.NextNodeIDs) > 0 {
			for _, next := range node.NextNodeIDs {
				viewNode.Next = append(viewNode.Next, next)
				edges = append(edges, FlowVisualizationEdge{
					ID:     flowVisualizationEdgeID(id, next, len(edges)),
					From:   id,
					To:     next,
					Type:   "next",
					Reason: "next_node_ids",
				})
			}
		}

		if node.Type == "GatewayNode" {
			rules := parseFlowVisualizationGatewayRules(node.Config)
			viewNode.Rules = make([]FlowVisualizationRule, 0, len(rules))
			for _, rule := range rules {
				if rule.TargetID == "" {
					continue
				}
				viewNode.Next = append(viewNode.Next, rule.TargetID)
				viewNode.Rules = append(viewNode.Rules, FlowVisualizationRule{
					Condition: rule.Condition,
					TargetID:  rule.TargetID,
				})
				edges = append(edges, FlowVisualizationEdge{
					ID:      flowVisualizationEdgeID(id, rule.TargetID, len(edges)),
					From:    id,
					To:      rule.TargetID,
					Type:    "rule",
					Label:   rule.Condition,
					Reason:  "gateway_condition",
				})
			}
		}

		nodes = append(nodes, viewNode)
	}

	layouts := buildFlowVisualizationLayoutHints(flow.StartNodeID, nodes, edges)
	for i, node := range nodes {
		if hint, ok := layouts[node.ID]; ok {
			nodes[i].Layout = hint
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	return FlowVisualization{
		ID:            flow.ID,
		Name:          flow.Name,
		StartNodeID:   flow.StartNodeID,
		ContextSchema: flow.ContextSchema,
		Nodes:         nodes,
		Edges:         edges,
	}
}

func buildFlowVisualizationLayoutHints(startNodeID string, nodes []FlowVisualizationNode, edges []FlowVisualizationEdge) map[string]FlowVisualizationLayout {
	const (
		horizontalGap = 220
		verticalGap   = 160
	)

	if len(edges) == 0 {
		hint := FlowVisualizationLayout{}
		if startNodeID != "" {
			return map[string]FlowVisualizationLayout{startNodeID: hint}
		}
		return map[string]FlowVisualizationLayout{}
	}

	allNodeSet := map[string]struct{}{}
	for _, node := range nodes {
		allNodeSet[node.ID] = struct{}{}
	}
	for _, edge := range edges {
		allNodeSet[edge.From] = struct{}{}
		allNodeSet[edge.To] = struct{}{}
	}

	// initialize unreachable nodes with -1
	layer := map[string]int{}
	for id := range allNodeSet {
		layer[id] = -1
	}
	if _, ok := layer[startNodeID]; !ok {
		for id := range allNodeSet {
			layer[id] = 0
		}
		layer[startNodeID] = 0
	} else {
		layer[startNodeID] = 0
	}

	outgoing := map[string][]string{}
	for _, edge := range edges {
		outgoing[edge.From] = append(outgoing[edge.From], edge.To)
	}

	changed := true
	for changed {
		changed = false
		for _, edge := range edges {
			if edge.From == edge.To {
				continue
			}
			fromLayer := layer[edge.From]
			if fromLayer < 0 {
				continue
			}
			nextLayer := fromLayer + 1
			if current, ok := layer[edge.To]; !ok || current < nextLayer {
				layer[edge.To] = nextLayer
				changed = true
			}
		}
	}

	maxKnownLayer := 0
	for _, value := range layer {
		if value >= 0 && value > maxKnownLayer {
			maxKnownLayer = value
		}
	}

	bucket := map[int][]string{}
	for id, l := range layer {
		if l < 0 {
			maxKnownLayer++
			l = maxKnownLayer
			layer[id] = l
		}
		bucket[l] = append(bucket[l], id)
	}

	for l := range bucket {
		sort.Strings(bucket[l])
	}

	layout := make(map[string]FlowVisualizationLayout, len(layer))
	for layerIdx := 0; layerIdx <= maxKnownLayer; layerIdx++ {
		ids := bucket[layerIdx]
		sort.Strings(ids)
		for order, id := range ids {
			layout[id] = FlowVisualizationLayout{
				Layer: layerIdx,
				Order: order,
				X:     order * horizontalGap,
				Y:     layerIdx * verticalGap,
			}
		}
	}

	unlinkedNodeIDs := make([]string, 0, len(allNodeSet))
	for id := range allNodeSet {
		if _, ok := layout[id]; !ok {
			unlinkedNodeIDs = append(unlinkedNodeIDs, id)
		}
	}

	if len(unlinkedNodeIDs) > 0 {
		sort.Strings(unlinkedNodeIDs)
		for _, id := range unlinkedNodeIDs {
			maxKnownLayer++
			layout[id] = FlowVisualizationLayout{
				Layer: maxKnownLayer,
				Order: 0,
				X:     0,
				Y:     maxKnownLayer * verticalGap,
			}
		}
	}

	// Apply sibling fan-out alignment when possible.
	for _, edge := range edges {
		out := outgoing[edge.From]
		if len(out) <= 1 {
			continue
		}
		parentLayer := layer[edge.From]
		parentHint, ok := layout[edge.From]
		if !ok {
			continue
		}
		for i, childID := range out {
			child := layout[childID]
			if child.Layer != parentLayer+1 {
				continue
			}
			child.X = parentHint.X + (i-(len(out)-1)/2)*horizontalGap/2
			child.Y = parentHint.Y + verticalGap
			child.Order = i
			layout[childID] = child
		}
	}

	return layout
}

type flowVisualizationRulePair struct {
	Condition string
	TargetID  string
}

func parseFlowVisualizationGatewayRules(config map[string]interface{}) []flowVisualizationRulePair {
	rules := []flowVisualizationRulePair{}
	if config == nil {
		return rules
	}
	raw, ok := config["rules"]
	if !ok {
		return rules
	}
	list, ok := raw.([]interface{})
	if !ok {
		return rules
	}
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rule := flowVisualizationRulePair{}
		if condition, ok := m["condition"].(string); ok {
			rule.Condition = condition
		}
		if target, ok := m["target_id"].(string); ok {
			rule.TargetID = target
		}
		rules = append(rules, rule)
	}

	return rules
}

func flowVisualizationEdgeID(from, to string, idx int) string {
	return from + ":" + to + ":" + strconv.Itoa(idx)
}
