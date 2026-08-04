package pipeline

import "sync"

// Context carries data through a pipeline run.
//
// Domain-specific code should put its typed request/result objects in Values or
// Metadata instead of extending the pipeline engine itself.
type Context struct {
	Values   map[string]any
	Metadata map[string]any
	Errors   []error

	NodeStates map[string]NodeState
	mu         sync.Mutex
}

func NewContext() *Context {
	return &Context{
		Values:     make(map[string]any),
		Metadata:   make(map[string]any),
		NodeStates: make(map[string]NodeState),
	}
}

func (c *Context) SetNodeState(nodeID string, state NodeState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.NodeStates == nil {
		c.NodeStates = make(map[string]NodeState)
	}
	c.NodeStates[nodeID] = state
}

func (c *Context) GetNodeState(nodeID string) NodeState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.NodeStates[nodeID]
}

func (c *Context) AddError(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Errors = append(c.Errors, err)
}

func (c *Context) Clone() *Context {
	clone := NewContext()
	for k, v := range c.Values {
		clone.Values[k] = v
	}
	for k, v := range c.Metadata {
		clone.Metadata[k] = v
	}
	return clone
}
