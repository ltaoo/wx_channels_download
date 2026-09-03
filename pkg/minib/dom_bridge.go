package minib

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/andybalholm/cascadia"
	"github.com/dop251/goja"
	"golang.org/x/net/html"
)

func (runtime *page_runtime) install_shared_dom_bridge(window *goja.Object) {
	_ = window.Set("__minib_node_get", func(call goja.FunctionCall) goja.Value {
		node := runtime.object_node(call.Argument(0))
		return runtime.vm.ToValue(runtime.shared_node_property(node, call.Argument(1).String()))
	})
	_ = window.Set("__minib_node_set", func(call goja.FunctionCall) goja.Value {
		node := runtime.object_node(call.Argument(0))
		runtime.set_shared_node_property(node, call.Argument(1).String(), call.Argument(2))
		return goja.Undefined()
	})
	_ = window.Set("__minib_node_call", func(call goja.FunctionCall) goja.Value {
		node := runtime.object_node(call.Argument(0))
		return runtime.call_shared_node_method(node, call.Argument(1).String(), shared_bridge_arguments(call.Argument(2)))
	})
	browser_click := runtime.vm.ToValue(func(selector string) bool {
		node := query_first(runtime.page.Document, selector)
		if node == nil {
			return false
		}
		if err := runtime.click_node(node, true); err != nil {
			panic(runtime.vm.NewGoError(err))
		}
		return true
	})
	_ = window.DefineDataProperty("__minib_browser_click", browser_click, goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_FALSE)
}

func shared_bridge_arguments(value goja.Value) []goja.Value {
	if value == nil || goja.IsNull(value) || goja.IsUndefined(value) {
		return nil
	}
	object, ok := value.(*goja.Object)
	if !ok {
		return nil
	}
	length_value := object.Get("length")
	if length_value == nil {
		return nil
	}
	length := int(length_value.ToInteger())
	if length <= 0 {
		return nil
	}
	arguments := make([]goja.Value, length)
	for index := range arguments {
		arguments[index] = object.Get(strconv.Itoa(index))
	}
	return arguments
}

func (runtime *page_runtime) shared_node_property(node *html.Node, name string) any {
	if node == nil {
		return nil
	}
	switch name {
	case "nodeType":
		return dom_node_type(node)
	case "nodeName":
		return dom_node_name(node)
	case "parentNode":
		return runtime.node_object(node.Parent)
	case "parentElement":
		if node.Parent != nil && node.Parent.Type == html.ElementNode {
			return runtime.node_object(node.Parent)
		}
		return nil
	case "firstChild":
		return runtime.node_object(node.FirstChild)
	case "lastChild":
		return runtime.node_object(node.LastChild)
	case "nextSibling":
		return runtime.node_object(node.NextSibling)
	case "previousSibling":
		return runtime.node_object(node.PrevSibling)
	case "childNodes":
		return runtime.node_array(children(node, false))
	case "children":
		return runtime.node_array(children(node, true))
	case "ownerDocument":
		if node.Type == html.DocumentNode && !runtime.fragments[node] {
			return nil
		}
		return runtime.node_object(runtime.page.Document)
	case "isConnected":
		for root := node; root != nil; root = root.Parent {
			if host := runtime.shadow_hosts[root]; host != nil {
				return contains_node(runtime.page.Document, host)
			}
		}
		return contains_node(runtime.page.Document, node)
	case "textContent":
		return text_content(node)
	case "nodeValue":
		if node.Type == html.TextNode || node.Type == html.CommentNode {
			return node.Data
		}
		return nil
	case "data":
		if node.Type == html.TextNode || node.Type == html.CommentNode {
			return node.Data
		}
		return ""
	case "innerHTML":
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "template") {
			return render_children(runtime.template_content(node))
		}
		return render_children(node)
	case "outerHTML":
		return render_node(node)
	case "adoptedStyleSheets":
		if sheets := runtime.adopted_style_sheets[node]; sheets != nil {
			return sheets
		}
		sheets := runtime.vm.NewArray()
		runtime.adopted_style_sheets[node] = sheets
		return sheets
	case "host":
		return runtime.node_object(runtime.shadow_hosts[node])
	case "mode":
		return runtime.shadow_modes[node]
	}
	if node.Type != html.ElementNode {
		return nil
	}
	switch name {
	case "tagName":
		return strings.ToUpper(node.Data)
	case "localName":
		return strings.ToLower(node.Data)
	case "id":
		return attribute(node, "id")
	case "className":
		return attribute(node, "class")
	case "src", "value", "name", "type", "rel", "charset", "dir":
		return attribute(node, name)
	case "href":
		if strings.EqualFold(node.Data, "a") {
			return runtime.element_url(node).String()
		}
		return attribute(node, name)
	case "content":
		if strings.EqualFold(node.Data, "template") {
			return runtime.node_object(runtime.template_content(node))
		}
		return attribute(node, name)
	case "style":
		return runtime.style_object(node)
	case "sheet":
		if !strings.EqualFold(node.Data, "style") && !strings.EqualFold(node.Data, "link") {
			return nil
		}
		runtime.refresh_style_sheets()
		if sheet := runtime.style_sheet_by_node[node]; sheet != nil {
			return runtime.css_style_sheet_object(sheet)
		}
		return nil
	case "classList":
		return runtime.class_list_object(node)
	case "dataset":
		return runtime.dataset_object(node)
	case "attributes":
		return runtime.attributes_object(node)
	case "shadowRoot":
		root := runtime.shadow_roots[node]
		if runtime.shadow_modes[root] != "open" {
			return nil
		}
		return runtime.node_object(root)
	case "contentWindow":
		if strings.EqualFold(node.Data, "iframe") {
			return runtime.vm.GlobalObject()
		}
		return nil
	case "contentDocument":
		if strings.EqualFold(node.Data, "iframe") {
			return runtime.node_object(runtime.page.Document)
		}
		return nil
	case "protocol", "host", "hostname", "port", "pathname", "search", "hash", "origin":
		if strings.EqualFold(node.Data, "a") {
			return url_property(runtime.element_url(node), name)
		}
		return ""
	case "clientWidth", "clientHeight", "offsetWidth", "offsetHeight", "scrollWidth", "scrollHeight", "scrollTop", "scrollLeft":
		return 0
	}
	return nil
}

func (runtime *page_runtime) set_shared_node_property(node *html.Node, name string, value goja.Value) {
	if node == nil {
		return
	}
	switch name {
	case "adoptedStyleSheets":
		runtime.adopted_style_sheets[node] = value
		return
	case "textContent":
		if node.Type == html.TextNode || node.Type == html.CommentNode {
			node.Data = value.String()
			runtime.notify_mutation(node)
			return
		}
		runtime.remove_all_children(node)
		if text := value.String(); text != "" {
			node.AppendChild(&html.Node{Type: html.TextNode, Data: text})
		}
		runtime.invalidate_node_styles(node)
		return
	case "nodeValue", "data":
		if node.Type == html.TextNode || node.Type == html.CommentNode {
			node.Data = value.String()
			runtime.notify_mutation(node)
		}
		return
	case "innerHTML":
		if node.Type != html.ElementNode {
			return
		}
		if strings.EqualFold(node.Data, "template") {
			content := runtime.template_content(node)
			set_inner_html(content, value.String())
			for child := content.FirstChild; child != nil; child = child.NextSibling {
				runtime.queue_dynamic_resource(child)
			}
			return
		}
		runtime.remove_all_children(node)
		append_html(node, value.String())
		runtime.invalidate_styles()
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			runtime.queue_dynamic_resource(child)
		}
		return
	}
	if node.Type != html.ElementNode {
		return
	}
	switch name {
	case "id":
		runtime.set_element_attribute(node, "id", value.String())
	case "className":
		runtime.set_element_attribute(node, "class", value.String())
	case "src", "href", "value", "name", "type", "rel", "charset", "dir":
		runtime.set_element_attribute(node, name, value.String())
		runtime.queue_dynamic_resource(node)
	case "content":
		if !strings.EqualFold(node.Data, "template") {
			runtime.set_element_attribute(node, name, value.String())
			runtime.queue_dynamic_resource(node)
		}
	}
}

func (runtime *page_runtime) call_shared_node_method(node *html.Node, name string, arguments []goja.Value) goja.Value {
	if node == nil {
		return goja.Null()
	}
	argument := func(index int) goja.Value {
		if index < 0 || index >= len(arguments) || arguments[index] == nil {
			return goja.Undefined()
		}
		return arguments[index]
	}
	switch name {
	case "appendChild":
		child := runtime.object_node(argument(0))
		if child == nil {
			return goja.Null()
		}
		if runtime.fragments[child] {
			for child.FirstChild != nil {
				fragment_child := child.FirstChild
				child.RemoveChild(fragment_child)
				node.AppendChild(fragment_child)
				runtime.queue_dynamic_resource(fragment_child)
			}
			runtime.invalidate_styles()
			return runtime.node_object(child)
		}
		runtime.detach_node(child)
		node.AppendChild(child)
		runtime.invalidate_styles()
		runtime.queue_dynamic_resource(child)
		return runtime.node_object(child)
	case "removeChild":
		child := runtime.object_node(argument(0))
		if child == nil || child.Parent != node {
			return goja.Null()
		}
		runtime.detach_node(child)
		runtime.invalidate_styles()
		return runtime.node_object(child)
	case "replaceChild":
		return runtime.replace_shared_child(node, runtime.object_node(argument(0)), runtime.object_node(argument(1)))
	case "insertBefore":
		return runtime.insert_shared_child(node, runtime.object_node(argument(0)), runtime.object_node(argument(1)))
	case "insertAdjacentElement":
		return runtime.insert_shared_adjacent(node, argument(0).String(), runtime.object_node(argument(1)))
	case "cloneNode":
		return runtime.node_object(runtime.clone_node(node, argument(0).ToBoolean()))
	case "contains":
		return runtime.vm.ToValue(contains_node(node, runtime.object_node(argument(0))))
	case "addEventListener":
		if runtime.listeners[node] == nil {
			runtime.listeners[node] = make(map[string][]*event_listener)
		}
		return runtime.add_event_listener(runtime.listeners[node], goja.FunctionCall{Arguments: arguments})
	case "removeEventListener":
		return runtime.remove_event_listener(runtime.listeners[node], goja.FunctionCall{Arguments: arguments})
	case "dispatchEvent":
		event_name, ok := event_type(runtime.vm, argument(0))
		if !ok {
			panic(runtime.vm.NewTypeError("dispatchEvent requires an Event"))
		}
		return runtime.vm.ToValue(runtime.dispatch_node_event(node, argument(0).ToObject(runtime.vm), event_name))
	case "getAttribute":
		value, ok := find_attribute(node, argument(0).String())
		if !ok {
			return goja.Null()
		}
		return runtime.vm.ToValue(value)
	case "setAttribute":
		runtime.set_element_attribute(node, argument(0).String(), argument(1).String())
		runtime.queue_dynamic_resource(node)
		return goja.Undefined()
	case "getAttributeNS":
		value, ok := find_attribute(node, argument(1).String())
		if !ok {
			return goja.Null()
		}
		return runtime.vm.ToValue(value)
	case "setAttributeNS":
		runtime.set_element_attribute(node, argument(1).String(), argument(2).String())
		runtime.queue_dynamic_resource(node)
		return goja.Undefined()
	case "removeAttribute":
		runtime.remove_element_attribute(node, argument(0).String())
		return goja.Undefined()
	case "removeAttributeNS":
		runtime.remove_element_attribute(node, argument(1).String())
		return goja.Undefined()
	case "hasAttribute":
		_, ok := find_attribute(node, argument(0).String())
		return runtime.vm.ToValue(ok)
	case "hasAttributeNS":
		_, ok := find_attribute(node, argument(1).String())
		return runtime.vm.ToValue(ok)
	case "hasAttributes":
		return runtime.vm.ToValue(len(node.Attr) > 0)
	case "getAttributeNames":
		return runtime.vm.ToValue(element_attribute_names(node))
	case "querySelector":
		return runtime.nullable_node_value(query_first(node, argument(0).String()))
	case "querySelectorAll":
		return runtime.node_array(query_all(node, argument(0).String()))
	case "getElementsByTagName":
		return runtime.node_array(find_by_tag(node, argument(0).String()))
	case "getElementsByName":
		return runtime.node_array(find_all_by_attribute(node, "name", argument(0).String()))
	case "getElementsByClassName":
		return runtime.node_array(find_by_class(node, argument(0).String()))
	case "matches":
		matcher, err := cascadia.ParseGroup(argument(0).String())
		return runtime.vm.ToValue(err == nil && matcher.Match(node))
	case "closest":
		matcher, err := cascadia.ParseGroup(argument(0).String())
		if err != nil {
			return goja.Null()
		}
		for current := node; current != nil; current = current.Parent {
			if current.Type == html.ElementNode && matcher.Match(current) {
				return runtime.node_object(current)
			}
		}
		return goja.Null()
	case "attachShadow":
		if node.Type != html.ElementNode {
			panic(runtime.vm.NewTypeError("attachShadow called on an incompatible receiver"))
		}
		if runtime.shadow_roots[node] != nil {
			panic(runtime.vm.NewGoError(fmt.Errorf("NotSupportedError: element already hosts a shadow tree")))
		}
		options := argument(0).ToObject(runtime.vm)
		mode := options.Get("mode").String()
		if mode != "open" && mode != "closed" {
			panic(runtime.vm.NewTypeError("attachShadow mode must be open or closed"))
		}
		root := &html.Node{Type: html.DocumentNode, Data: "#document-fragment"}
		runtime.fragments[root] = true
		runtime.shadow_roots[node] = root
		runtime.shadow_hosts[root] = node
		runtime.shadow_modes[root] = mode
		return runtime.node_object(root)
	case "getBoundingClientRect":
		return runtime.vm.ToValue(runtime.shared_bounding_rect(node))
	case "getClientRects":
		if !contains_node(runtime.page.Document, node) {
			return runtime.vm.NewArray()
		}
		return runtime.vm.NewArray(runtime.shared_bounding_rect(node))
	case "focus", "blur":
		return goja.Undefined()
	case "click":
		if err := runtime.click_node(node, false); err != nil {
			panic(runtime.vm.NewGoError(err))
		}
		return goja.Undefined()
	case "getContext":
		if strings.EqualFold(node.Data, "canvas") && strings.EqualFold(argument(0).String(), "2d") {
			return runtime.canvas_context_object()
		}
		return goja.Null()
	case "toDataURL":
		return runtime.vm.ToValue("data:image/png;base64,")
	}
	return goja.Undefined()
}

func (runtime *page_runtime) replace_shared_child(node *html.Node, child *html.Node, old_child *html.Node) goja.Value {
	if child == nil || old_child == nil || old_child.Parent != node {
		return goja.Null()
	}
	if child == old_child {
		return runtime.node_object(old_child)
	}
	if runtime.fragments[child] {
		inserted := make([]*html.Node, 0)
		for child.FirstChild != nil {
			fragment_child := child.FirstChild
			child.RemoveChild(fragment_child)
			node.InsertBefore(fragment_child, old_child)
			inserted = append(inserted, fragment_child)
		}
		runtime.detach_node(old_child)
		for _, fragment_child := range inserted {
			runtime.queue_dynamic_resource(fragment_child)
		}
		runtime.invalidate_styles()
		return runtime.node_object(old_child)
	}
	runtime.detach_node(child)
	node.InsertBefore(child, old_child)
	runtime.detach_node(old_child)
	runtime.invalidate_styles()
	runtime.queue_dynamic_resource(child)
	return runtime.node_object(old_child)
}

func (runtime *page_runtime) insert_shared_child(node *html.Node, child *html.Node, mark *html.Node) goja.Value {
	if child == nil {
		return goja.Null()
	}
	if child == mark {
		return runtime.node_object(child)
	}
	if runtime.fragments[child] {
		for child.FirstChild != nil {
			fragment_child := child.FirstChild
			child.RemoveChild(fragment_child)
			if mark != nil && mark.Parent == node {
				node.InsertBefore(fragment_child, mark)
			} else {
				node.AppendChild(fragment_child)
			}
			runtime.queue_dynamic_resource(fragment_child)
		}
		runtime.invalidate_styles()
		return runtime.node_object(child)
	}
	runtime.detach_node(child)
	if mark != nil && mark.Parent == node {
		node.InsertBefore(child, mark)
	} else {
		node.AppendChild(child)
	}
	runtime.invalidate_styles()
	runtime.queue_dynamic_resource(child)
	return runtime.node_object(child)
}

func (runtime *page_runtime) insert_shared_adjacent(node *html.Node, position string, child *html.Node) goja.Value {
	if child == nil {
		return goja.Null()
	}
	position = strings.ToLower(position)
	if (position == "beforebegin" || position == "afterend") && node.Parent == nil {
		return goja.Null()
	}
	if position != "beforebegin" && position != "afterbegin" && position != "beforeend" && position != "afterend" {
		return goja.Null()
	}
	runtime.detach_node(child)
	switch position {
	case "beforebegin":
		node.Parent.InsertBefore(child, node)
	case "afterbegin":
		node.InsertBefore(child, node.FirstChild)
	case "beforeend":
		node.AppendChild(child)
	case "afterend":
		if node.NextSibling == nil {
			node.Parent.AppendChild(child)
		} else {
			node.Parent.InsertBefore(child, node.NextSibling)
		}
	}
	runtime.invalidate_styles()
	runtime.queue_dynamic_resource(child)
	return runtime.node_object(child)
}

func (runtime *page_runtime) shared_bounding_rect(node *html.Node) map[string]float64 {
	if contains_node(runtime.page.Document, node) {
		return map[string]float64{"x": 0, "y": 0, "top": 0, "right": 100, "bottom": 20, "left": 0, "width": 100, "height": 20}
	}
	return map[string]float64{"x": 0, "y": 0, "top": 0, "right": 0, "bottom": 0, "left": 0, "width": 0, "height": 0}
}
