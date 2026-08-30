package minib

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"golang.org/x/net/html"
)

// custom_element_definition is the immutable definition captured by
// CustomElementRegistry.define. The HTML standard captures lifecycle callbacks
// and observedAttributes at definition time; looking them up on every element
// invocation produces observably different behavior.
type custom_element_definition struct {
	name                       string
	constructor                goja.Value
	prototype                  *goja.Object
	connected_callback         goja.Callable
	disconnected_callback      goja.Callable
	adopted_callback           goja.Callable
	attribute_changed_callback goja.Callable
	observed_attributes        map[string]bool
}

type custom_element_reaction struct {
	node       *html.Node
	callback   goja.Callable
	this_value goja.Value
	arguments  []goja.Value
	source     string
}

func (runtime *page_runtime) create_custom_element_definition(name string, constructor goja.Value) (*custom_element_definition, error) {
	constructor_object := constructor.ToObject(runtime.vm)
	prototype_value := constructor_object.Get("prototype")
	if prototype_value == nil || goja.IsUndefined(prototype_value) || goja.IsNull(prototype_value) {
		return nil, fmt.Errorf("custom element constructor has no prototype")
	}
	prototype := prototype_value.ToObject(runtime.vm)
	connected_callback, err := runtime.read_custom_element_callback(prototype, "connectedCallback")
	if err != nil {
		return nil, err
	}
	disconnected_callback, err := runtime.read_custom_element_callback(prototype, "disconnectedCallback")
	if err != nil {
		return nil, err
	}
	adopted_callback, err := runtime.read_custom_element_callback(prototype, "adoptedCallback")
	if err != nil {
		return nil, err
	}
	attribute_changed_callback, err := runtime.read_custom_element_callback(prototype, "attributeChangedCallback")
	if err != nil {
		return nil, err
	}
	definition := &custom_element_definition{
		name:                       name,
		constructor:                constructor,
		prototype:                  prototype,
		connected_callback:         connected_callback,
		disconnected_callback:      disconnected_callback,
		adopted_callback:           adopted_callback,
		attribute_changed_callback: attribute_changed_callback,
		observed_attributes:        make(map[string]bool),
	}
	if attribute_changed_callback == nil {
		return definition, nil
	}
	observed := constructor_object.Get("observedAttributes")
	if observed == nil || goja.IsUndefined(observed) || goja.IsNull(observed) {
		return definition, nil
	}
	array := observed.ToObject(runtime.vm)
	for index, length := int64(0), array.Get("length").ToInteger(); index < length; index++ {
		attribute_name := strings.ToLower(array.Get(fmt.Sprintf("%d", index)).String())
		definition.observed_attributes[attribute_name] = true
	}
	return definition, nil
}

func (runtime *page_runtime) read_custom_element_callback(prototype *goja.Object, name string) (goja.Callable, error) {
	value := prototype.Get(name)
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil, nil
	}
	callback, ok := goja.AssertFunction(value)
	if !ok {
		return nil, fmt.Errorf("custom element %s is not callable", name)
	}
	return callback, nil
}

func (runtime *page_runtime) custom_element_callback(node *html.Node, name string) (goja.Callable, goja.Value) {
	definition := runtime.custom_elements[strings.ToLower(node.Data)]
	if definition == nil {
		return nil, nil
	}
	var callback goja.Callable
	switch name {
	case "connectedCallback":
		callback = definition.connected_callback
	case "disconnectedCallback":
		callback = definition.disconnected_callback
	case "adoptedCallback":
		callback = definition.adopted_callback
	case "attributeChangedCallback":
		callback = definition.attribute_changed_callback
	}
	object := runtime.node_object(node)
	if callback != nil {
		return callback, object
	}

	// Polymer's generated host constructor keeps lifecycle methods on a
	// controller and exposes it through polymerController. Native browsers call
	// this bridge from the generated custom-element host. Keeping the bridge here
	// lets the DOM reaction machinery retain the same host/controller boundary.
	controller_value := object.Get("polymerController")
	if controller_value == nil || goja.IsUndefined(controller_value) || goja.IsNull(controller_value) {
		return nil, nil
	}
	controller := controller_value.ToObject(runtime.vm)
	callback, ok := goja.AssertFunction(controller.Get(name))
	if !ok {
		return nil, nil
	}
	return callback, controller
}

func (runtime *page_runtime) enqueue_custom_element_reaction(node *html.Node, name string, arguments ...goja.Value) {
	callback, this_value := runtime.custom_element_callback(node, name)
	if callback == nil {
		return
	}
	runtime.custom_reactions = append(runtime.custom_reactions, custom_element_reaction{
		node:       node,
		callback:   callback,
		this_value: this_value,
		arguments:  arguments,
		source:     name,
	})
}

func (runtime *page_runtime) flush_custom_element_reactions() {
	if runtime.running_reactions {
		return
	}
	runtime.running_reactions = true
	defer func() {
		runtime.running_reactions = false
	}()
	for len(runtime.custom_reactions) > 0 {
		reaction := runtime.custom_reactions[0]
		runtime.custom_reactions = runtime.custom_reactions[1:]
		if _, err := runtime.call_javascript(runtime.ctx, reaction.callback, reaction.this_value, reaction.arguments...); err != nil {
			runtime.fail_script(runtime.page.URL+"#"+reaction.source, err)
		}
	}
}

func (runtime *page_runtime) disconnect_custom_elements(root *html.Node) {
	if root == nil {
		return
	}
	if runtime.custom_connected[root] {
		runtime.custom_connected[root] = false
		runtime.enqueue_custom_element_reaction(root, "disconnectedCallback")
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		runtime.disconnect_custom_elements(child)
	}
}

func (runtime *page_runtime) detach_node(node *html.Node) {
	if node == nil || node.Parent == nil {
		return
	}
	was_connected := contains_node(runtime.page.Document, node)
	if was_connected {
		runtime.disconnect_custom_elements(node)
	}
	node.Parent.RemoveChild(node)
	if was_connected {
		runtime.flush_custom_element_reactions()
	}
}

func (runtime *page_runtime) remove_all_children(parent *html.Node) {
	if parent == nil {
		return
	}
	was_connected := contains_node(runtime.page.Document, parent)
	if was_connected {
		for child := parent.FirstChild; child != nil; child = child.NextSibling {
			runtime.disconnect_custom_elements(child)
		}
	}
	remove_children(parent)
	if was_connected {
		runtime.flush_custom_element_reactions()
	}
}
