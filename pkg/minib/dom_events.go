package minib

import (
	"github.com/dop251/goja"
	"golang.org/x/net/html"
)

const (
	event_phase_none      = 0
	event_phase_capture   = 1
	event_phase_at_target = 2
	event_phase_bubble    = 3
)

type event_dispatch_state struct {
	stopped           bool
	immediate_stopped bool
	passive           bool
}

func (runtime *page_runtime) install_window_events(window *goja.Object) {
	add_event_listener := func(call goja.FunctionCall) goja.Value {
		return runtime.add_event_listener(runtime.window_listeners, call)
	}
	remove_event_listener := func(call goja.FunctionCall) goja.Value {
		return runtime.remove_event_listener(runtime.window_listeners, call)
	}
	dispatch_event := func(call goja.FunctionCall) goja.Value {
		event_name, ok := event_type(runtime.vm, call.Argument(0))
		if !ok {
			panic(runtime.vm.NewTypeError("dispatchEvent requires an Event"))
		}
		return runtime.vm.ToValue(runtime.dispatch_window_event(call.Argument(0).ToObject(runtime.vm), event_name))
	}
	_ = window.Set("__minib_addEventListener", add_event_listener)
	_ = window.Set("__minib_removeEventListener", remove_event_listener)
	_ = window.Set("__minib_dispatchEvent", dispatch_event)
	// goja's host global and host-created DOM objects do not reliably expose
	// methods inherited only through JavaScript-assigned prototype chains. Bind
	// dynamic accessors for those methods while preserving prototype patches.
	runtime.install_host_event_target_methods(window)
}

func (runtime *page_runtime) install_node_events(object *goja.Object, node *html.Node) {
	add_event_listener := func(call goja.FunctionCall) goja.Value {
		if runtime.listeners[node] == nil {
			runtime.listeners[node] = make(map[string][]*event_listener)
		}
		return runtime.add_event_listener(runtime.listeners[node], call)
	}
	remove_event_listener := func(call goja.FunctionCall) goja.Value {
		return runtime.remove_event_listener(runtime.listeners[node], call)
	}
	dispatch_event := func(call goja.FunctionCall) goja.Value {
		event_name, ok := event_type(runtime.vm, call.Argument(0))
		if !ok {
			panic(runtime.vm.NewTypeError("dispatchEvent requires an Event"))
		}
		return runtime.vm.ToValue(runtime.dispatch_node_event(node, call.Argument(0).ToObject(runtime.vm), event_name))
	}
	_ = object.Set("__minib_addEventListener", add_event_listener)
	_ = object.Set("__minib_removeEventListener", remove_event_listener)
	_ = object.Set("__minib_dispatchEvent", dispatch_event)
	runtime.install_host_event_target_methods(object)
}

func (runtime *page_runtime) install_host_event_target_methods(object *goja.Object) {
	if runtime == nil || runtime.vm == nil || object == nil {
		return
	}
	for _, method_name := range []string{"addEventListener", "removeEventListener", "dispatchEvent"} {
		name := method_name
		define_getter(runtime.vm, object, name, func() any {
			constructor := runtime.vm.Get("EventTarget").ToObject(runtime.vm)
			return constructor.Get("prototype").ToObject(runtime.vm).Get(name)
		})
	}
}

func (runtime *page_runtime) add_event_listener(listeners map[string][]*event_listener, call goja.FunctionCall) goja.Value {
	callback := call.Argument(1)
	if goja.IsNull(callback) || goja.IsUndefined(callback) {
		return goja.Undefined()
	}
	if _, callable := goja.AssertFunction(callback); !callable {
		if _, object := callback.(*goja.Object); !object {
			return goja.Undefined()
		}
	}
	event_name := call.Argument(0).String()
	capture, once, passive := runtime.event_listener_options(call.Argument(2))
	for _, listener := range listeners[event_name] {
		if !listener.removed && listener.capture == capture && listener.callback.StrictEquals(callback) {
			return goja.Undefined()
		}
	}
	listeners[event_name] = append(listeners[event_name], &event_listener{
		callback: callback,
		capture:  capture,
		once:     once,
		passive:  passive,
	})
	return goja.Undefined()
}

func (runtime *page_runtime) remove_event_listener(listeners map[string][]*event_listener, call goja.FunctionCall) goja.Value {
	callback := call.Argument(1)
	if goja.IsNull(callback) || goja.IsUndefined(callback) {
		return goja.Undefined()
	}
	event_name := call.Argument(0).String()
	capture, _, _ := runtime.event_listener_options(call.Argument(2))
	for _, listener := range listeners[event_name] {
		if !listener.removed && listener.capture == capture && listener.callback.StrictEquals(callback) {
			listener.removed = true
			break
		}
	}
	return goja.Undefined()
}

func (runtime *page_runtime) event_listener_options(value goja.Value) (bool, bool, bool) {
	if value == nil || goja.IsNull(value) || goja.IsUndefined(value) {
		return false, false, false
	}
	if exported, ok := value.Export().(bool); ok {
		return exported, false, false
	}
	object, ok := value.(*goja.Object)
	if !ok {
		return value.ToBoolean(), false, false
	}
	return event_option_enabled(object.Get("capture")), event_option_enabled(object.Get("once")), event_option_enabled(object.Get("passive"))
}

func event_option_enabled(value goja.Value) bool {
	return value != nil && !goja.IsNull(value) && !goja.IsUndefined(value) && value.ToBoolean()
}

func (runtime *page_runtime) dispatch_window_event(event *goja.Object, event_name string) bool {
	window := runtime.vm.GlobalObject()
	return runtime.dispatch_event(event, event_name, []goja.Value{window}, false, func(index int) map[string][]*event_listener {
		return runtime.window_listeners
	})
}

func (runtime *page_runtime) dispatch_node_event(node *html.Node, event *goja.Object, event_name string) bool {
	return runtime.dispatch_node_event_with_trust(node, event, event_name, false)
}

func (runtime *page_runtime) dispatch_node_event_with_trust(node *html.Node, event *goja.Object, event_name string, trusted bool) bool {
	path := make([]goja.Value, 0)
	nodes := make([]*html.Node, 0)
	for current := node; current != nil; current = current.Parent {
		path = append(path, runtime.node_object(current))
		nodes = append(nodes, current)
	}
	path = append(path, runtime.vm.GlobalObject())
	return runtime.dispatch_event(event, event_name, path, trusted, func(index int) map[string][]*event_listener {
		if index == len(path)-1 {
			return runtime.window_listeners
		}
		return runtime.listeners[nodes[index]]
	})
}

func (runtime *page_runtime) dispatch_event(event *goja.Object, event_name string, path []goja.Value, trusted bool, listeners_at func(int) map[string][]*event_listener) bool {
	if event_name == "" || runtime.dispatching_events[event] {
		panic(runtime.vm.NewTypeError("event is already being dispatched or has an empty type"))
	}
	runtime.dispatching_events[event] = true
	defer delete(runtime.dispatching_events, event)

	state := &event_dispatch_state{}
	path_values := make([]any, len(path))
	for index, value := range path {
		path_values[index] = value
	}
	composed_path := runtime.vm.NewArray(path_values...)
	_ = event.Set("target", path[0])
	_ = event.Set("currentTarget", nil)
	_ = event.Set("eventPhase", event_phase_none)
	_ = event.Set("isTrusted", trusted)
	_ = event.Set("__composedPath", composed_path)
	_ = event.Set("composedPath", func() *goja.Object { return runtime.vm.NewArray(path_values...) })
	_ = event.Set("stopPropagation", func() { state.stopped = true })
	_ = event.Set("stopImmediatePropagation", func() {
		state.stopped = true
		state.immediate_stopped = true
	})
	_ = event.Set("preventDefault", func() {
		if !state.passive && event_option_enabled(event.Get("cancelable")) {
			_ = event.Set("defaultPrevented", true)
		}
	})
	_ = event.DefineAccessorProperty("cancelBubble", runtime.vm.ToValue(func() bool { return state.stopped }), runtime.vm.ToValue(func(value bool) {
		if value {
			state.stopped = true
		}
	}), goja.FLAG_TRUE, goja.FLAG_TRUE)
	_ = event.DefineAccessorProperty("returnValue", runtime.vm.ToValue(func() bool { return !event_option_enabled(event.Get("defaultPrevented")) }), runtime.vm.ToValue(func(value bool) {
		if !value && event_option_enabled(event.Get("cancelable")) {
			_ = event.Set("defaultPrevented", true)
		}
	}), goja.FLAG_TRUE, goja.FLAG_TRUE)

	for index := len(path) - 1; index > 0 && !state.stopped; index-- {
		state.immediate_stopped = false
		runtime.invoke_event_listeners(event, event_name, path[index], listeners_at(index), true, event_phase_capture, state)
	}
	if !state.stopped {
		state.immediate_stopped = false
		runtime.invoke_event_listeners(event, event_name, path[0], listeners_at(0), true, event_phase_at_target, state)
		if !state.immediate_stopped {
			runtime.invoke_event_listeners(event, event_name, path[0], listeners_at(0), false, event_phase_at_target, state)
		}
	}
	if event_option_enabled(event.Get("bubbles")) {
		for index := 1; index < len(path) && !state.stopped; index++ {
			state.immediate_stopped = false
			runtime.invoke_event_listeners(event, event_name, path[index], listeners_at(index), false, event_phase_bubble, state)
		}
	}
	_ = event.Set("currentTarget", nil)
	_ = event.Set("eventPhase", event_phase_none)
	return !event_option_enabled(event.Get("defaultPrevented"))
}

func (runtime *page_runtime) click_node(node *html.Node, trusted bool) error {
	event_init := runtime.vm.NewObject()
	_ = event_init.Set("bubbles", true)
	_ = event_init.Set("cancelable", true)
	_ = event_init.Set("composed", true)
	_ = event_init.Set("view", runtime.vm.GlobalObject())
	_ = event_init.Set("detail", 1)
	_ = event_init.Set("button", 0)
	_ = event_init.Set("buttons", 0)
	event, err := runtime.vm.New(runtime.vm.Get("MouseEvent"), runtime.vm.ToValue("click"), event_init)
	if err != nil {
		return err
	}
	runtime.dispatch_node_event_with_trust(node, event, "click", trusted)
	return nil
}

func (runtime *page_runtime) invoke_event_listeners(event *goja.Object, event_name string, current_target goja.Value, listeners map[string][]*event_listener, capture bool, phase int, state *event_dispatch_state) {
	_ = event.Set("currentTarget", current_target)
	_ = event.Set("eventPhase", phase)
	snapshot := append([]*event_listener(nil), listeners[event_name]...)
	for _, listener := range snapshot {
		if listener.removed || listener.capture != capture {
			continue
		}
		if listener.once {
			listener.removed = true
		}
		state.passive = listener.passive
		if err := runtime.call_event_listener(listener.callback, current_target, event); err != nil {
			runtime.fail_script(runtime.page.URL+"#"+event_name, err)
		}
		state.passive = false
		if state.immediate_stopped {
			return
		}
	}
	if !capture {
		current_object := current_target.ToObject(runtime.vm)
		if callback, ok := goja.AssertFunction(current_object.Get("on" + event_name)); ok {
			if _, err := runtime.call_javascript(runtime.ctx, callback, current_target, event); err != nil {
				runtime.fail_script(runtime.page.URL+"#"+event_name, err)
			}
		}
	}
}

func (runtime *page_runtime) call_event_listener(callback goja.Value, current_target goja.Value, event *goja.Object) error {
	if callable, ok := goja.AssertFunction(callback); ok {
		_, err := runtime.call_javascript(runtime.ctx, callable, current_target, event)
		return err
	}
	object, ok := callback.(*goja.Object)
	if !ok {
		return nil
	}
	handle_event, ok := goja.AssertFunction(object.Get("handleEvent"))
	if !ok {
		return nil
	}
	_, err := runtime.call_javascript(runtime.ctx, handle_event, object, event)
	return err
}
