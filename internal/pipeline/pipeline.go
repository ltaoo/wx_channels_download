package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type EventKind string

const (
	EventPipelineStart EventKind = "pipeline_start"
	EventPipelineDone  EventKind = "pipeline_done"
	EventNodeStart     EventKind = "node_start"
	EventNodeDone      EventKind = "node_done"
	EventNodeError     EventKind = "node_error"
	EventNodeWaiting   EventKind = "node_waiting"
	EventNodeSkipped   EventKind = "node_skipped"
)

type Event struct {
	Kind      EventKind
	Pipeline  string
	NodeID    string
	NodeType  string
	State     NodeState
	Error     error
	Timestamp time.Time
}

type EventHandler func(Event)

type Pipeline struct {
	Name        string
	StartNodeID string
	Nodes       map[string]*NodeDef
	OnEvent     EventHandler
}

type Result struct {
	Duration time.Duration
	Errors   []error
}

func (p *Pipeline) Run(ctx context.Context, pc *Context) (*Result, error) {
	start := time.Now()
	if pc == nil {
		pc = NewContext()
	}
	if pc.Values == nil {
		pc.Values = make(map[string]any)
	}
	if pc.Metadata == nil {
		pc.Metadata = make(map[string]any)
	}
	if pc.NodeStates == nil {
		pc.NodeStates = make(map[string]NodeState)
	}

	p.emit(Event{Kind: EventPipelineStart})
	if p.StartNodeID == "" {
		err := errors.New("pipeline has no start node")
		p.emit(Event{Kind: EventNodeError, Error: err})
		return nil, err
	}
	if err := p.driveFlow(ctx, pc, []string{p.StartNodeID}); err != nil {
		return nil, err
	}

	result := &Result{
		Duration: time.Since(start),
		Errors:   pc.Errors,
	}
	p.emit(Event{Kind: EventPipelineDone})
	return result, nil
}

func (p *Pipeline) driveFlow(ctx context.Context, pc *Context, nodeIDs []string) error {
	for _, nodeID := range nodeIDs {
		if err := ctx.Err(); err != nil {
			return err
		}

		nodeDef, ok := p.Nodes[nodeID]
		if !ok {
			return &NodeError{
				Pipeline: p.Name,
				NodeID:   nodeID,
				NodeType: "unknown",
				Err:      fmt.Errorf("node not found"),
			}
		}

		if pc.GetNodeState(nodeID) == StateCompleted {
			p.emitNode(EventNodeSkipped, nodeDef, StateSkipped, nil)
			continue
		}

		pc.SetNodeState(nodeID, StateRunning)
		p.emitNode(EventNodeStart, nodeDef, StateRunning, nil)

		nextIDs, err := nodeDef.Node.Execute(ctx, pc)
		if err != nil {
			pc.SetNodeState(nodeID, StateFailed)
			wrapped := &NodeError{
				Pipeline: p.Name,
				NodeID:   nodeID,
				NodeType: nodeDef.Type,
				Err:      err,
			}
			p.emitNode(EventNodeError, nodeDef, StateFailed, wrapped)
			return wrapped
		}

		if pc.GetNodeState(nodeID) == StateWaiting {
			p.emitNode(EventNodeWaiting, nodeDef, StateWaiting, nil)
			continue
		}

		pc.SetNodeState(nodeID, StateCompleted)
		p.emitNode(EventNodeDone, nodeDef, StateCompleted, nil)

		if nextIDs == nil {
			nextIDs = nodeDef.NextNodes
		}
		if len(nextIDs) == 0 {
			continue
		}
		if err := p.driveFlow(ctx, pc, nextIDs); err != nil {
			return err
		}
	}
	return nil
}

func (p *Pipeline) emitNode(kind EventKind, nodeDef *NodeDef, state NodeState, err error) {
	p.emit(Event{
		Kind:     kind,
		NodeID:   nodeDef.ID,
		NodeType: nodeDef.Type,
		State:    state,
		Error:    err,
	})
}

func (p *Pipeline) emit(evt Event) {
	if p.OnEvent == nil {
		return
	}
	evt.Pipeline = p.Name
	evt.Timestamp = time.Now()
	p.OnEvent(evt)
}
