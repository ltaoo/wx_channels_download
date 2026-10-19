package nodes

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"wx_channel/pkg/flowengine/engine"
)

func config_integer(config map[string]interface{}, key string, fallback int) int {
	switch value := config[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case int64:
		return int(value)
	}
	return fallback
}

func (n *LoopNode) execute_iteration(ctx *engine.ProcessContext) (bool, []string, error) {
	resolved, err := resolve_service_template(n.Config["items"], ctx)
	if err != nil {
		return false, nil, err
	}
	items, err := toInterfaceSlice(resolved)
	if err != nil {
		return false, nil, err
	}
	limit := config_integer(n.Config, "max_iterations", 1000)
	if limit < 1 || limit > 10000 || len(items) > limit {
		return false, nil, fmt.Errorf("iteration count %d exceeds max_iterations %d (allowed 1..10000)", len(items), limit)
	}
	concurrency := config_integer(n.Config, "concurrency", 1)
	if concurrency < 1 || concurrency > 32 {
		return false, nil, fmt.Errorf("concurrency must be 1..32")
	}
	definition, err := configured_workflow(n.Config, ctx)
	if err != nil {
		return false, nil, err
	}
	base_arguments, err := service_arguments(n.Config, ctx)
	if err != nil {
		return false, nil, err
	}
	results := make([]interface{}, len(items))
	failures := make([]error, len(items))
	continue_on_error, _ := n.Config["continue_on_error"].(bool)
	jobs := make(chan int)
	var wg sync.WaitGroup
	var failed atomic.Bool
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				if failed.Load() && !continue_on_error {
					continue
				}
				if err := ctx.ExecutionContext().Err(); err != nil {
					failures[index] = err
					failed.Store(true)
					continue
				}
				input := make(map[string]interface{}, len(base_arguments)+2)
				for key, value := range base_arguments {
					input[key] = value
				}
				input["item"] = items[index]
				input["index"] = index
				result, err := ctx.EngineRef.ExecuteSubflow(ctx, definition, input)
				if err != nil {
					failures[index] = err
					failed.Store(true)
					results[index] = map[string]interface{}{"error": err.Error(), "index": index}
				} else {
					results[index] = result
				}
			}
		}()
	}
	for index := range items {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	if !continue_on_error {
		for index, failure := range failures {
			if failure != nil {
				return false, nil, fmt.Errorf("iteration %d: %w", index, failure)
			}
		}
	}
	if err := ctx.ExecutionContext().Err(); err != nil {
		return false, nil, err
	}
	output_key, _ := n.Config["output_key"].(string)
	if output_key == "" {
		output_key = "loop_result"
	}
	ctx.SetOutput(n.Id, output_key, results)
	return true, ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id), nil
}

type WaitNode struct {
	node_id string
	config  map[string]interface{}
}

func NewWaitNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &WaitNode{node_id: id, config: config}
}
func (n *WaitNode) ID() string   { return n.node_id }
func (n *WaitNode) Type() string { return "WaitNode" }
func (n *WaitNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	seconds := config_integer(n.config, "seconds", 1)
	if seconds < 0 || seconds > 86400 {
		return false, nil, fmt.Errorf("wait seconds must be 0..86400")
	}
	if err := engine.SleepContext(ctx.ExecutionContext(), time.Duration(seconds)*time.Second); err != nil {
		return false, nil, err
	}
	return true, ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.node_id), nil
}

type FailNode struct {
	node_id string
	config  map[string]interface{}
}

func NewFailNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &FailNode{node_id: id, config: config}
}
func (n *FailNode) ID() string   { return n.node_id }
func (n *FailNode) Type() string { return "FailNode" }
func (n *FailNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	message, err := resolve_service_template(n.config["message"], ctx)
	if err != nil {
		return false, nil, err
	}
	if message == nil || message == "" {
		message = "workflow explicitly failed"
	}
	return false, nil, fmt.Errorf("%v", message)
}
