package pipeline

import (
	"context"
	"fmt"
)

type Node interface {
	ID() string
	Type() string
	Execute(ctx context.Context, pc *Context) (nextNodeIDs []string, err error)
}

type NodeDef struct {
	ID        string
	Type      string
	Node      Node
	NextNodes []string
	Config    map[string]any
}

type NodeState string

const (
	StatePending   NodeState = "PENDING"
	StateRunning   NodeState = "RUNNING"
	StateCompleted NodeState = "COMPLETED"
	StateFailed    NodeState = "FAILED"
	StateWaiting   NodeState = "WAITING"
	StateSkipped   NodeState = "SKIPPED"
)

type NodeError struct {
	Pipeline string
	NodeID   string
	NodeType string
	Err      error
}

func (e *NodeError) Error() string {
	return fmt.Sprintf("pipeline %q node %q (%s): %v", e.Pipeline, e.NodeID, e.NodeType, e.Err)
}

func (e *NodeError) Unwrap() error {
	return e.Err
}

type FuncNode struct {
	id   string
	typ  string
	fn   func(context.Context, *Context) error
	next func(context.Context, *Context) []string
}

func NewFuncNode(id, typ string, fn func(context.Context, *Context) error) *FuncNode {
	return &FuncNode{id: id, typ: typ, fn: fn}
}

func (n *FuncNode) WithNext(fn func(context.Context, *Context) []string) *FuncNode {
	n.next = fn
	return n
}

func (n *FuncNode) ID() string { return n.id }

func (n *FuncNode) Type() string {
	if n.typ == "" {
		return "func"
	}
	return n.typ
}

func (n *FuncNode) Execute(ctx context.Context, pc *Context) ([]string, error) {
	if n.fn != nil {
		if err := n.fn(ctx, pc); err != nil {
			return nil, err
		}
	}
	if n.next != nil {
		return n.next(ctx, pc), nil
	}
	return nil, nil
}
