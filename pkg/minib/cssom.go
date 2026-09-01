package minib

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/andybalholm/cascadia"
	douceur_css "github.com/aymerick/douceur/css"
	"github.com/aymerick/douceur/parser"
	"github.com/dop251/goja"
	"golang.org/x/net/html"

	"wx_channel/pkg/clawreq"
)

type css_property struct {
	value     string
	important bool
}

type css_declaration_block struct {
	runtime    *page_runtime
	owner_node *html.Node
	properties map[string]css_property
	order      []string
	raw        string
	read_only  bool
	on_change  func()
	object     *goja.Object
}

type css_compiled_selector struct {
	selector    cascadia.Sel
	specificity cascadia.Specificity
}

type css_style_rule struct {
	selector_text string
	selectors     []css_compiled_selector
	declarations  *css_declaration_block
	parent_sheet  *css_style_sheet
	object        *goja.Object
}

type css_style_sheet struct {
	runtime    *page_runtime
	owner_node *html.Node
	href       string
	media      string
	title      string
	disabled   bool
	source     string
	rules      []*css_style_rule
	object     *goja.Object
}

type css_cascade_value struct {
	property    css_property
	inline      bool
	specificity cascadia.Specificity
	order       int
}

func (runtime *page_runtime) install_cssom(window *goja.Object) {
	_ = window.Set("__minib_construct_css_style_sheet", func() *goja.Object {
		sheet := runtime.new_css_style_sheet(nil, "", "", "")
		return runtime.css_style_sheet_object(sheet)
	})
	css := runtime.vm.NewObject()
	_ = css.Set("escape", css_escape)
	_ = css.Set("supports", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 || strings.TrimSpace(call.Argument(0).String()) == "" {
			return runtime.vm.ToValue(false)
		}
		// ponytail: syntactic support is enough without a rendering engine; parse properties when CSS layout exists.
		return runtime.vm.ToValue(len(call.Arguments) == 1 || strings.TrimSpace(call.Argument(1).String()) != "")
	})
	_ = css.Set("registerProperty", func(goja.FunctionCall) goja.Value { return goja.Undefined() })
	_ = window.Set("CSS", css)
}

func css_escape(value string) string {
	code_points := []rune(value)
	var result strings.Builder
	for index, code_point := range code_points {
		if code_point == 0 {
			result.WriteRune('\uFFFD')
			continue
		}
		if code_point <= 31 || code_point == 127 || index == 0 && code_point >= '0' && code_point <= '9' || index == 1 && code_points[0] == '-' && code_point >= '0' && code_point <= '9' {
			result.WriteByte('\\')
			result.WriteString(strconv.FormatInt(int64(code_point), 16))
			result.WriteByte(' ')
			continue
		}
		if index == 0 && code_point == '-' && len(code_points) == 1 {
			result.WriteString(`\-`)
			continue
		}
		if code_point >= 128 || code_point == '-' || code_point == '_' || code_point >= '0' && code_point <= '9' || code_point >= 'A' && code_point <= 'Z' || code_point >= 'a' && code_point <= 'z' {
			result.WriteRune(code_point)
			continue
		}
		result.WriteByte('\\')
		result.WriteRune(code_point)
	}
	return result.String()
}

func (runtime *page_runtime) style_object(node *html.Node) *goja.Object {
	if node == nil {
		return runtime.style_declaration_object(runtime.new_css_declaration_block("", true, nil))
	}
	if object := runtime.styles[node]; object != nil {
		if block := runtime.style_blocks[node]; block != nil {
			block.sync_owner_attribute()
		}
		return object
	}
	block := runtime.new_css_declaration_block(attribute(node, "style"), false, func() {
		runtime.invalidate_node_styles(node)
	})
	block.owner_node = node
	runtime.style_blocks[node] = block
	object := runtime.style_declaration_object(block)
	runtime.styles[node] = object
	return object
}

func (runtime *page_runtime) computed_style_object(node *html.Node) *goja.Object {
	if runtime.disable_css {
		return runtime.scraper_computed_style_object(node)
	}
	runtime.refresh_style_sheets()
	runtime.recompute_style_tree()
	properties := make(map[string]css_property)
	if node != nil {
		for name, property := range runtime.computed_styles[node] {
			properties[name] = property
		}
	}
	block := runtime.new_css_declaration_block("", true, nil)
	block.properties = properties
	block.order = sorted_css_property_names(properties)
	return runtime.style_declaration_object(block)
}

func (runtime *page_runtime) scraper_computed_style_object(node *html.Node) *goja.Object {
	properties := default_computed_style(node)
	if node != nil {
		for _, declaration := range parse_css_declarations(attribute(node, "style")) {
			name := normalize_css_property_name(declaration.Property)
			if name == "" {
				continue
			}
			properties[name] = css_property{
				value:     strings.TrimSpace(declaration.Value),
				important: declaration.Important,
			}
		}
	}
	for name, property := range properties {
		property.value = resolve_css_variables(property.value, properties, 0)
		properties[name] = property
	}
	block := runtime.new_css_declaration_block("", true, nil)
	block.properties = properties
	block.order = sorted_css_property_names(properties)
	return runtime.style_declaration_object(block)
}

func (runtime *page_runtime) new_css_declaration_block(source string, read_only bool, on_change func()) *css_declaration_block {
	block := &css_declaration_block{
		runtime:    runtime,
		properties: make(map[string]css_property),
		read_only:  read_only,
		on_change:  on_change,
	}
	block.replace(source, false)
	return block
}

func (block *css_declaration_block) sync_owner_attribute() {
	if block.owner_node == nil {
		return
	}
	source := attribute(block.owner_node, "style")
	if source != block.raw {
		block.replace(source, false)
		block.runtime.invalidate_node_styles(block.owner_node)
	}
}

func (block *css_declaration_block) replace(source string, notify bool) {
	block.properties = make(map[string]css_property)
	block.order = block.order[:0]
	for _, declaration := range parse_css_declarations(source) {
		name := normalize_css_property_name(declaration.Property)
		if name == "" {
			continue
		}
		if _, exists := block.properties[name]; !exists {
			block.order = append(block.order, name)
		}
		block.properties[name] = css_property{value: strings.TrimSpace(declaration.Value), important: declaration.Important}
	}
	block.raw = block.css_text()
	if notify {
		block.changed()
	}
}

func (block *css_declaration_block) set(name string, value string, priority string) {
	if block.read_only {
		return
	}
	name = normalize_css_property_name(name)
	value = strings.TrimSpace(value)
	priority = strings.TrimSpace(strings.ToLower(priority))
	if name == "" || priority != "" && priority != "important" {
		return
	}
	if value == "" {
		block.remove(name)
		return
	}
	if _, exists := block.properties[name]; !exists {
		block.order = append(block.order, name)
	}
	block.properties[name] = css_property{value: value, important: priority == "important"}
	block.raw = block.css_text()
	block.changed()
}

func (block *css_declaration_block) remove(name string) string {
	if block.read_only {
		return ""
	}
	name = normalize_css_property_name(name)
	old_property, exists := block.properties[name]
	if !exists {
		return ""
	}
	delete(block.properties, name)
	for index, current_name := range block.order {
		if current_name == name {
			block.order = append(block.order[:index], block.order[index+1:]...)
			break
		}
	}
	block.raw = block.css_text()
	block.changed()
	return old_property.value
}

func (block *css_declaration_block) changed() {
	if block.owner_node != nil {
		if block.raw == "" {
			block.runtime.remove_element_attribute(block.owner_node, "style")
		} else {
			block.runtime.set_element_attribute(block.owner_node, "style", block.raw)
		}
	}
	if block.on_change != nil {
		block.on_change()
	}
}

func (block *css_declaration_block) css_text() string {
	var builder strings.Builder
	for _, name := range block.order {
		property, exists := block.properties[name]
		if !exists {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(name)
		builder.WriteString(": ")
		builder.WriteString(property.value)
		if property.important {
			builder.WriteString(" !important")
		}
		builder.WriteByte(';')
	}
	return builder.String()
}

func (runtime *page_runtime) style_declaration_object(block *css_declaration_block) *goja.Object {
	if block.object != nil {
		return block.object
	}
	bridge := runtime.vm.NewObject()
	_ = bridge.Set("get", func(name string) string {
		block.sync_owner_attribute()
		return block.properties[normalize_css_property_name(name)].value
	})
	_ = bridge.Set("priority", func(name string) string {
		block.sync_owner_attribute()
		if block.properties[normalize_css_property_name(name)].important {
			return "important"
		}
		return ""
	})
	_ = bridge.Set("set", func(name string, value string, priority string) { block.set(name, value, priority) })
	_ = bridge.Set("remove", func(name string) string { return block.remove(name) })
	_ = bridge.Set("item", func(index int) string {
		block.sync_owner_attribute()
		if index < 0 || index >= len(block.order) {
			return ""
		}
		return block.order[index]
	})
	_ = bridge.Set("length", func() int { block.sync_owner_attribute(); return len(block.order) })
	_ = bridge.Set("cssText", func() string { block.sync_owner_attribute(); return block.css_text() })
	_ = bridge.Set("setCssText", func(source string) {
		if !block.read_only {
			block.replace(source, true)
		}
	})
	factory, ok := goja.AssertFunction(runtime.vm.Get("__minib_create_style_declaration"))
	if !ok {
		return bridge
	}
	value, err := factory(goja.Undefined(), bridge)
	if err != nil {
		runtime.fail_script(runtime.page.URL+"#cssom", err)
		return bridge
	}
	block.object = value.ToObject(runtime.vm)
	return block.object
}

func normalize_css_property_name(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "--") {
		return name
	}
	return strings.ToLower(name)
}

func parse_css_declarations(source string) []*douceur_css.Declaration {
	// The fallback tokenizer preserves custom properties and modern function
	// values that douceur's older declaration parser can silently omit.
	return parse_css_declarations_fallback(source)
}

func parse_css_declarations_fallback(source string) []*douceur_css.Declaration {
	parts := split_css_top_level(source, ';')
	declarations := make([]*douceur_css.Declaration, 0, len(parts))
	for _, part := range parts {
		name_value := split_css_declaration(part)
		if len(name_value) != 2 {
			continue
		}
		value := strings.TrimSpace(name_value[1])
		lower_value := strings.ToLower(value)
		important := strings.HasSuffix(strings.TrimSpace(lower_value), "!important")
		if important {
			value = strings.TrimSpace(value[:strings.LastIndex(lower_value, "!important")])
		}
		declarations = append(declarations, &douceur_css.Declaration{Property: strings.TrimSpace(name_value[0]), Value: value, Important: important})
	}
	return declarations
}

func split_css_top_level(source string, delimiter byte) []string {
	parts := make([]string, 0)
	start := 0
	quote := byte(0)
	escaped := false
	comment := false
	depth := 0
	for index := 0; index < len(source); index++ {
		character := source[index]
		if comment {
			if character == '*' && index+1 < len(source) && source[index+1] == '/' {
				comment = false
				index++
			}
			continue
		}
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '/' && index+1 < len(source) && source[index+1] == '*' {
			comment = true
			index++
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			continue
		}
		switch character {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if character == delimiter && depth == 0 {
				parts = append(parts, source[start:index])
				start = index + 1
			}
		}
	}
	parts = append(parts, source[start:])
	return parts
}

func split_css_declaration(source string) []string {
	parts := split_css_top_level(source, ':')
	if len(parts) < 2 {
		return nil
	}
	return []string{parts[0], strings.Join(parts[1:], ":")}
}

func (runtime *page_runtime) refresh_style_sheets() {
	if runtime.disable_css {
		if len(runtime.style_sheets) > 0 {
			runtime.style_sheets = nil
		}
		if len(runtime.style_sheet_by_node) > 0 {
			runtime.style_sheet_by_node = make(map[*html.Node]*css_style_sheet)
		}
		if len(runtime.computed_styles) > 0 {
			runtime.computed_styles = make(map[*html.Node]map[string]css_property)
		}
		runtime.styles_dirty = false
		runtime.dirty_style_roots = make(map[*html.Node]bool)
		return
	}
	next_sheets := make([]*css_style_sheet, 0)
	changed := false
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			tag_name := strings.ToLower(node.Data)
			if tag_name == "style" || tag_name == "link" && has_rel_value(node, "stylesheet") {
				source, href := runtime.style_sheet_source(node)
				sheet := runtime.style_sheet_by_node[node]
				media := strings.TrimSpace(attribute(node, "media"))
				title := attribute(node, "title")
				if sheet == nil {
					sheet = runtime.new_css_style_sheet(node, href, media, title)
					runtime.style_sheet_by_node[node] = sheet
					changed = true
				}
				if sheet.source != source || sheet.href != href || sheet.media != media || sheet.title != title {
					sheet.href = href
					sheet.media = media
					sheet.title = title
					runtime.replace_style_sheet(sheet, source)
					changed = true
				}
				sheet.disabled = has_attribute(node, "disabled")
				next_sheets = append(next_sheets, sheet)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(runtime.page.Document)
	if len(next_sheets) != len(runtime.style_sheets) {
		changed = true
	}
	runtime.style_sheets = next_sheets
	if changed {
		runtime.styles_dirty = true
	}
}

func (runtime *page_runtime) style_sheet_source(node *html.Node) (string, string) {
	if strings.EqualFold(node.Data, "style") {
		return text_content(node), ""
	}
	resource_url, ok := resolve_resource_url(runtime.page_url, attribute(node, "href"))
	if !ok {
		return "", ""
	}
	for _, resource := range runtime.page.Resources {
		if resource.URL != resource_url || resource.Kind != StyleResource || resource.Err != nil {
			continue
		}
		decoded, err := clawreq.DecodeText(resource.Body, resource.ContentType)
		if err == nil {
			return decoded, resource_url
		}
	}
	return "", resource_url
}

func has_rel_value(node *html.Node, expected string) bool {
	for _, value := range strings.Fields(strings.ToLower(attribute(node, "rel"))) {
		if value == expected {
			return true
		}
	}
	return false
}

func (runtime *page_runtime) new_css_style_sheet(owner_node *html.Node, href string, media string, title string) *css_style_sheet {
	return &css_style_sheet{runtime: runtime, owner_node: owner_node, href: href, media: media, title: title}
}

func (runtime *page_runtime) replace_style_sheet(sheet *css_style_sheet, source string) {
	sheet.source = source
	sheet.rules = nil
	stylesheet, err := parser.Parse(source)
	if err != nil {
		stylesheet = parse_css_stylesheet_fallback(source)
	}
	if stylesheet != nil {
		runtime.append_parsed_css_rules(sheet, stylesheet.Rules, true)
	}
	runtime.styles_dirty = true
}

func (runtime *page_runtime) append_parsed_css_rules(sheet *css_style_sheet, parsed_rules []*douceur_css.Rule, active bool) {
	for _, parsed_rule := range parsed_rules {
		if parsed_rule.Kind == douceur_css.AtRule {
			next_active := active
			switch strings.ToLower(parsed_rule.Name) {
			case "@media":
				next_active = active && runtime.css_media_matches(parsed_rule.Prelude)
			case "@supports", "@layer", "@container", "@scope", "@document":
				next_active = active
			default:
				next_active = false
			}
			if len(parsed_rule.Rules) > 0 {
				runtime.append_parsed_css_rules(sheet, parsed_rule.Rules, next_active)
			}
			continue
		}
		if !active {
			continue
		}
		rule := &css_style_rule{selector_text: strings.TrimSpace(parsed_rule.Prelude), parent_sheet: sheet}
		for _, selector_text := range parsed_rule.Selectors {
			selector, err := cascadia.Parse(strings.TrimSpace(selector_text))
			if err == nil && selector.PseudoElement() == "" {
				rule.selectors = append(rule.selectors, css_compiled_selector{selector: selector, specificity: selector.Specificity()})
			}
		}
		if len(rule.selectors) == 0 {
			continue
		}
		rule.declarations = runtime.new_css_declaration_block(declarations_css_text(parsed_rule.Declarations), false, func() {
			runtime.styles_dirty = true
		})
		sheet.rules = append(sheet.rules, rule)
	}
}

func declarations_css_text(declarations []*douceur_css.Declaration) string {
	var builder strings.Builder
	for _, declaration := range declarations {
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(declaration.String())
	}
	return builder.String()
}

func parse_css_stylesheet_fallback(source string) *douceur_css.Stylesheet {
	stylesheet := douceur_css.NewStylesheet()
	for position := 0; position < len(source); {
		open_index := strings.IndexByte(source[position:], '{')
		if open_index < 0 {
			break
		}
		open_index += position
		selector_text := strings.TrimSpace(source[position:open_index])
		depth := 1
		close_index := open_index + 1
		quote := byte(0)
		for ; close_index < len(source) && depth > 0; close_index++ {
			character := source[close_index]
			if quote != 0 {
				if character == quote && source[close_index-1] != '\\' {
					quote = 0
				}
				continue
			}
			if character == '\'' || character == '"' {
				quote = character
			} else if character == '{' {
				depth++
			} else if character == '}' {
				depth--
			}
		}
		if depth != 0 {
			break
		}
		body := source[open_index+1 : close_index-1]
		position = close_index
		if selector_text == "" || strings.HasPrefix(selector_text, "@") {
			continue
		}
		stylesheet.Rules = append(stylesheet.Rules, &douceur_css.Rule{
			Kind:         douceur_css.QualifiedRule,
			Prelude:      selector_text,
			Selectors:    split_css_top_level(selector_text, ','),
			Declarations: parse_css_declarations(body),
		})
	}
	return stylesheet
}

func (runtime *page_runtime) style_sheet_list_object() *goja.Object {
	runtime.refresh_style_sheets()
	values := make([]any, 0, len(runtime.style_sheets))
	for _, sheet := range runtime.style_sheets {
		values = append(values, runtime.css_style_sheet_object(sheet))
	}
	list := runtime.vm.NewArray(values...)
	_ = list.SetPrototype(runtime.vm.Get("StyleSheetList").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	_ = list.Set("item", func(index int) any {
		if index < 0 || index >= len(runtime.style_sheets) {
			return nil
		}
		return runtime.css_style_sheet_object(runtime.style_sheets[index])
	})
	return list
}

func (runtime *page_runtime) css_style_sheet_object(sheet *css_style_sheet) *goja.Object {
	if sheet.object != nil {
		return sheet.object
	}
	object := runtime.vm.NewObject()
	_ = object.SetPrototype(runtime.vm.Get("CSSStyleSheet").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	sheet.object = object
	define_getter(runtime.vm, object, "ownerNode", func() any { return runtime.node_object(sheet.owner_node) })
	define_getter(runtime.vm, object, "ownerRule", func() any { return nil })
	define_getter(runtime.vm, object, "href", func() any {
		if sheet.href == "" {
			return nil
		}
		return sheet.href
	})
	define_getter(runtime.vm, object, "type", func() any { return "text/css" })
	define_getter(runtime.vm, object, "title", func() any { return sheet.title })
	define_accessor(runtime.vm, object, "disabled", func() any { return sheet.disabled }, func(value goja.Value) {
		sheet.disabled = value.ToBoolean()
		runtime.styles_dirty = true
	})
	define_getter(runtime.vm, object, "media", func() any { return runtime.media_list_object(sheet) })
	define_getter(runtime.vm, object, "cssRules", func() any { return runtime.css_rule_list_object(sheet) })
	define_getter(runtime.vm, object, "rules", func() any { return runtime.css_rule_list_object(sheet) })
	_ = object.Set("insertRule", func(call goja.FunctionCall) goja.Value {
		index := len(sheet.rules)
		if !goja.IsUndefined(call.Argument(1)) {
			index = int(call.Argument(1).ToInteger())
		}
		if index < 0 || index > len(sheet.rules) {
			panic(runtime.vm.NewGoError(fmt.Errorf("IndexSizeError: CSS rule index %d", index)))
		}
		parsed, err := parser.Parse(call.Argument(0).String())
		if err != nil || len(parsed.Rules) != 1 {
			panic(runtime.vm.NewGoError(fmt.Errorf("SyntaxError: invalid CSS rule")))
		}
		temporary := runtime.new_css_style_sheet(nil, "", "", "")
		runtime.append_parsed_css_rules(temporary, parsed.Rules, true)
		if len(temporary.rules) != 1 {
			panic(runtime.vm.NewGoError(fmt.Errorf("SyntaxError: unsupported CSS rule")))
		}
		rule := temporary.rules[0]
		rule.parent_sheet = sheet
		sheet.rules = append(sheet.rules, nil)
		copy(sheet.rules[index+1:], sheet.rules[index:])
		sheet.rules[index] = rule
		runtime.styles_dirty = true
		return runtime.vm.ToValue(index)
	})
	_ = object.Set("deleteRule", func(index int) {
		if index < 0 || index >= len(sheet.rules) {
			panic(runtime.vm.NewGoError(fmt.Errorf("IndexSizeError: CSS rule index %d", index)))
		}
		sheet.rules = append(sheet.rules[:index], sheet.rules[index+1:]...)
		runtime.styles_dirty = true
	})
	_ = object.Set("replaceSync", func(source string) { runtime.replace_style_sheet(sheet, source) })
	_ = object.Set("replace", func(source string) *goja.Promise {
		runtime.replace_style_sheet(sheet, source)
		promise, resolve, _ := runtime.vm.NewPromise()
		_ = resolve(object)
		return promise
	})
	_ = object.Set("addRule", func(selector string, style string, index int) int {
		insert_rule, _ := goja.AssertFunction(object.Get("insertRule"))
		_, _ = insert_rule(object, runtime.vm.ToValue(selector+"{"+style+"}"), runtime.vm.ToValue(index))
		return index
	})
	_ = object.Set("removeRule", func(index int) {
		delete_rule, _ := goja.AssertFunction(object.Get("deleteRule"))
		_, _ = delete_rule(object, runtime.vm.ToValue(index))
	})
	return object
}

func (sheet *css_style_sheet) css_text() string {
	parts := make([]string, 0, len(sheet.rules))
	for _, rule := range sheet.rules {
		parts = append(parts, rule.css_text())
	}
	return strings.Join(parts, "\n")
}

func (runtime *page_runtime) css_rule_list_object(sheet *css_style_sheet) *goja.Object {
	values := make([]any, 0, len(sheet.rules))
	for _, rule := range sheet.rules {
		values = append(values, runtime.css_style_rule_object(rule))
	}
	list := runtime.vm.NewArray(values...)
	_ = list.SetPrototype(runtime.vm.Get("CSSRuleList").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	_ = list.Set("item", func(index int) any {
		if index < 0 || index >= len(sheet.rules) {
			return nil
		}
		return runtime.css_style_rule_object(sheet.rules[index])
	})
	return list
}

func (runtime *page_runtime) css_style_rule_object(rule *css_style_rule) *goja.Object {
	if rule.object != nil {
		return rule.object
	}
	object := runtime.vm.NewObject()
	_ = object.SetPrototype(runtime.vm.Get("CSSStyleRule").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	rule.object = object
	define_getter(runtime.vm, object, "type", func() any { return 1 })
	define_getter(runtime.vm, object, "parentRule", func() any { return nil })
	define_getter(runtime.vm, object, "parentStyleSheet", func() any { return runtime.css_style_sheet_object(rule.parent_sheet) })
	define_accessor(runtime.vm, object, "selectorText", func() any { return rule.selector_text }, func(value goja.Value) {
		selector_text := value.String()
		selectors := compile_css_selectors(split_css_top_level(selector_text, ','))
		if len(selectors) > 0 {
			rule.selector_text = selector_text
			rule.selectors = selectors
			runtime.styles_dirty = true
		}
	})
	define_getter(runtime.vm, object, "style", func() any { return runtime.style_declaration_object(rule.declarations) })
	define_getter(runtime.vm, object, "styleMap", func() any { return runtime.vm.NewObject() })
	define_accessor(runtime.vm, object, "cssText", func() any { return rule.css_text() }, func(value goja.Value) {
		parsed, err := parser.Parse(value.String())
		if err != nil || len(parsed.Rules) != 1 || parsed.Rules[0].Kind != douceur_css.QualifiedRule {
			return
		}
		parsed_rule := parsed.Rules[0]
		selectors := compile_css_selectors(parsed_rule.Selectors)
		if len(selectors) == 0 {
			return
		}
		rule.selector_text = strings.TrimSpace(parsed_rule.Prelude)
		rule.selectors = selectors
		rule.declarations.replace(declarations_css_text(parsed_rule.Declarations), true)
	})
	return object
}

func (rule *css_style_rule) css_text() string {
	return rule.selector_text + " { " + rule.declarations.css_text() + " }"
}

func compile_css_selectors(selector_texts []string) []css_compiled_selector {
	selectors := make([]css_compiled_selector, 0, len(selector_texts))
	for _, selector_text := range selector_texts {
		selector, err := cascadia.Parse(strings.TrimSpace(selector_text))
		if err == nil && selector.PseudoElement() == "" {
			selectors = append(selectors, css_compiled_selector{selector: selector, specificity: selector.Specificity()})
		}
	}
	return selectors
}

func (runtime *page_runtime) media_list_object(sheet *css_style_sheet) *goja.Object {
	object := runtime.vm.NewObject()
	_ = object.SetPrototype(runtime.vm.Get("MediaList").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	media_values := func() []string {
		values := split_css_top_level(sheet.media, ',')
		result := make([]string, 0, len(values))
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				result = append(result, value)
			}
		}
		return result
	}
	define_accessor(runtime.vm, object, "mediaText", func() any { return sheet.media }, func(value goja.Value) {
		sheet.media = value.String()
		runtime.styles_dirty = true
	})
	define_getter(runtime.vm, object, "length", func() any { return len(media_values()) })
	_ = object.Set("item", func(index int) any {
		values := media_values()
		if index < 0 || index >= len(values) {
			return nil
		}
		return values[index]
	})
	_ = object.Set("appendMedium", func(value string) {
		values := media_values()
		if !slice_contains(values, value) {
			values = append(values, value)
			sheet.media = strings.Join(values, ", ")
			runtime.styles_dirty = true
		}
	})
	_ = object.Set("deleteMedium", func(value string) {
		values := media_values()
		kept := values[:0]
		for _, current := range values {
			if current != value {
				kept = append(kept, current)
			}
		}
		sheet.media = strings.Join(kept, ", ")
		runtime.styles_dirty = true
	})
	return object
}

func (runtime *page_runtime) invalidate_styles() {
	if runtime.disable_css {
		return
	}
	runtime.styles_dirty = true
	runtime.dirty_style_roots = make(map[*html.Node]bool)
}

func (runtime *page_runtime) invalidate_node_styles(node *html.Node) {
	if runtime.disable_css || node == nil || runtime.styles_dirty {
		return
	}
	if len(runtime.computed_styles) == 0 {
		runtime.invalidate_styles()
		return
	}
	if node.Type != html.ElementNode {
		node = node.Parent
	}
	if node == nil {
		runtime.invalidate_styles()
		return
	}
	for root := range runtime.dirty_style_roots {
		if contains_node(root, node) {
			return
		}
		if contains_node(node, root) {
			delete(runtime.dirty_style_roots, root)
		}
	}
	runtime.dirty_style_roots[node] = true
}

func (runtime *page_runtime) recompute_style_tree() {
	if !runtime.styles_dirty && len(runtime.dirty_style_roots) == 0 {
		return
	}
	var walk func(*html.Node, map[string]css_property)
	walk = func(node *html.Node, parent_style map[string]css_property) {
		next_parent := parent_style
		if node.Type == html.ElementNode {
			computed := runtime.compute_element_style(node, parent_style)
			runtime.computed_styles[node] = computed
			next_parent = computed
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, next_parent)
		}
	}
	if runtime.styles_dirty || len(runtime.computed_styles) == 0 {
		runtime.computed_styles = make(map[*html.Node]map[string]css_property)
		walk(runtime.page.Document, nil)
		runtime.styles_dirty = false
		runtime.dirty_style_roots = make(map[*html.Node]bool)
		return
	}
	for root := range runtime.dirty_style_roots {
		var parent_style map[string]css_property
		for parent := root.Parent; parent != nil; parent = parent.Parent {
			if parent.Type == html.ElementNode {
				parent_style = runtime.computed_styles[parent]
				break
			}
		}
		walk(root, parent_style)
	}
	runtime.dirty_style_roots = make(map[*html.Node]bool)
}

func (runtime *page_runtime) compute_element_style(node *html.Node, parent_style map[string]css_property) map[string]css_property {
	computed := default_computed_style(node)
	for name, property := range parent_style {
		if strings.HasPrefix(name, "--") || inherited_css_properties[name] {
			computed[name] = property
		}
	}
	winners := make(map[string]css_cascade_value)
	order := 0
	for _, sheet := range runtime.style_sheets {
		if sheet.disabled || !runtime.css_media_matches(sheet.media) {
			continue
		}
		for _, rule := range sheet.rules {
			order++
			specificity, matches := matching_css_specificity(rule.selectors, node)
			if !matches {
				continue
			}
			for _, name := range rule.declarations.order {
				property := rule.declarations.properties[name]
				candidate := css_cascade_value{property: property, specificity: specificity, order: order}
				if current, exists := winners[name]; !exists || css_candidate_wins(candidate, current) {
					winners[name] = candidate
				}
			}
		}
	}
	inline_block := runtime.style_blocks[node]
	if inline_block == nil {
		inline_block = runtime.new_css_declaration_block(attribute(node, "style"), false, func() { runtime.invalidate_node_styles(node) })
		inline_block.owner_node = node
		runtime.style_blocks[node] = inline_block
	} else {
		inline_block.sync_owner_attribute()
	}
	order++
	for _, name := range inline_block.order {
		property := inline_block.properties[name]
		candidate := css_cascade_value{property: property, inline: true, specificity: cascadia.Specificity{1, 0, 0}, order: order}
		if current, exists := winners[name]; !exists || css_candidate_wins(candidate, current) {
			winners[name] = candidate
		}
	}
	for name, winner := range winners {
		value := strings.TrimSpace(winner.property.value)
		switch strings.ToLower(value) {
		case "inherit":
			if inherited, exists := parent_style[name]; exists {
				computed[name] = inherited
			} else {
				computed[name] = initial_css_property(node, name)
			}
		case "initial", "revert", "revert-layer":
			computed[name] = initial_css_property(node, name)
		case "unset":
			if inherited_css_properties[name] || strings.HasPrefix(name, "--") {
				if inherited, exists := parent_style[name]; exists {
					computed[name] = inherited
				}
			} else {
				computed[name] = initial_css_property(node, name)
			}
		default:
			computed[name] = winner.property
		}
	}
	for name, property := range computed {
		property.value = resolve_css_variables(property.value, computed, 0)
		computed[name] = property
	}
	return computed
}

func matching_css_specificity(selectors []css_compiled_selector, node *html.Node) (cascadia.Specificity, bool) {
	var specificity cascadia.Specificity
	matches := false
	for _, selector := range selectors {
		if !selector.selector.Match(node) {
			continue
		}
		if !matches || specificity.Less(selector.specificity) {
			specificity = selector.specificity
		}
		matches = true
	}
	return specificity, matches
}

func css_candidate_wins(candidate css_cascade_value, current css_cascade_value) bool {
	if candidate.property.important != current.property.important {
		return candidate.property.important
	}
	if candidate.inline != current.inline {
		return candidate.inline
	}
	if candidate.specificity != current.specificity {
		return current.specificity.Less(candidate.specificity)
	}
	return candidate.order >= current.order
}

var inherited_css_properties = map[string]bool{
	"border-collapse": true, "border-spacing": true, "caption-side": true,
	"color": true, "cursor": true, "direction": true, "empty-cells": true,
	"font": true, "font-family": true, "font-feature-settings": true,
	"font-kerning": true, "font-size": true, "font-stretch": true,
	"font-style": true, "font-variant": true, "font-weight": true,
	"hyphens": true, "letter-spacing": true, "line-height": true,
	"list-style": true, "list-style-image": true, "list-style-position": true,
	"list-style-type": true, "orphans": true, "quotes": true,
	"tab-size": true, "text-align": true, "text-indent": true,
	"text-rendering": true, "text-transform": true, "visibility": true,
	"white-space": true, "widows": true, "word-break": true,
	"word-spacing": true, "word-wrap": true, "writing-mode": true,
}

func default_computed_style(node *html.Node) map[string]css_property {
	return map[string]css_property{
		"color":       {value: "rgb(0, 0, 0)"},
		"display":     {value: default_display(node)},
		"font-family": {value: "Arial"},
		"font-size":   {value: "16px"},
		"font-style":  {value: "normal"},
		"font-weight": {value: "400"},
		"line-height": {value: "normal"},
		"opacity":     {value: "1"},
		"position":    {value: "static"},
		"visibility":  {value: "visible"},
		"white-space": {value: "normal"},
	}
}

func initial_css_property(node *html.Node, name string) css_property {
	if property, exists := default_computed_style(node)[name]; exists {
		return property
	}
	return css_property{value: ""}
}

func default_display(node *html.Node) string {
	if node == nil {
		return "inline"
	}
	tag_name := strings.ToLower(node.Data)
	switch tag_name {
	case "html", "body", "address", "article", "aside", "blockquote", "details", "dialog", "dd", "div", "dl", "dt", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hgroup", "hr", "main", "nav", "ol", "p", "pre", "section", "summary", "ul":
		return "block"
	case "head", "base", "link", "meta", "title", "noscript", "script", "style", "template":
		return "none"
	case "li":
		return "list-item"
	case "table":
		return "table"
	case "thead":
		return "table-header-group"
	case "tbody":
		return "table-row-group"
	case "tfoot":
		return "table-footer-group"
	case "tr":
		return "table-row"
	case "td", "th":
		return "table-cell"
	case "colgroup":
		return "table-column-group"
	case "col":
		return "table-column"
	case "caption":
		return "table-caption"
	default:
		return "inline"
	}
}

func resolve_css_variables(value string, properties map[string]css_property, depth int) string {
	if depth > 16 || !strings.Contains(value, "var(") {
		return value
	}
	start := strings.Index(value, "var(")
	open := start + len("var(")
	level := 1
	end := open
	for ; end < len(value) && level > 0; end++ {
		if value[end] == '(' {
			level++
		} else if value[end] == ')' {
			level--
		}
	}
	if level != 0 {
		return value
	}
	arguments := split_css_top_level(value[open:end-1], ',')
	name := strings.TrimSpace(arguments[0])
	replacement := ""
	if property, exists := properties[name]; exists {
		replacement = resolve_css_variables(property.value, properties, depth+1)
	} else if len(arguments) > 1 {
		replacement = resolve_css_variables(strings.Join(arguments[1:], ","), properties, depth+1)
	}
	return resolve_css_variables(value[:start]+replacement+value[end:], properties, depth+1)
}

func sorted_css_property_names(properties map[string]css_property) []string {
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (runtime *page_runtime) css_media_matches(media string) bool {
	media = strings.TrimSpace(strings.ToLower(media))
	if media == "" || media == "all" || media == "screen" {
		return true
	}
	for _, query := range split_css_top_level(media, ',') {
		query = strings.TrimSpace(query)
		if strings.Contains(query, "print") && !strings.Contains(query, "not print") {
			continue
		}
		if strings.Contains(query, "screen") || strings.HasPrefix(query, "(") || strings.HasPrefix(query, "not print") {
			if media_width_matches(query, 1440) {
				return true
			}
		}
	}
	return false
}

func media_width_matches(query string, viewport_width float64) bool {
	for _, condition := range []struct {
		name string
		min  bool
	}{{"min-width", true}, {"max-width", false}} {
		position := strings.Index(query, condition.name)
		if position < 0 {
			continue
		}
		remainder := query[position+len(condition.name):]
		colon := strings.IndexByte(remainder, ':')
		if colon < 0 {
			continue
		}
		value_text := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(remainder[colon+1:]), ")"))
		value_text = strings.TrimSuffix(value_text, "px")
		width, err := strconv.ParseFloat(strings.TrimSpace(value_text), 64)
		if err != nil {
			continue
		}
		if condition.min && viewport_width < width || !condition.min && viewport_width > width {
			return false
		}
	}
	return true
}
