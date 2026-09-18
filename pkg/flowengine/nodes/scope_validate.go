package nodes

import (
	"fmt"
	"sort"
	"strings"

	"wx_channel/pkg/flowengine/engine"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
)

// ValidateFlowScopes statically checks every variable read in a flow
// definition against the three-scope contract that ScopeEnv implements at
// runtime:
//
//	input.<key>            — key must be declared by the flow's ContextSchema
//	output.<node_id>.<key> — node_id must exist, be an ancestor of the reader
//	                         and declare <key> as one of its produced keys
//	global.<key>           — key must be declared by some SetVariableNode
//
// Anything else (a bare {{key}}, a {{ctx.key}} prefix, an undeclared key, a
// downstream or unknown producer) is rejected. Because the runtime resolvers
// raise the same errors, a flow that passes here cannot fail with a silent nil
// at run time.
//
// Sub-flow definitions embedded in WorkflowNode / LoopNode config are validated
// recursively against their own context schema.
func ValidateFlowScopes(def engine.FlowDefinition) error {
	validator := new_scope_validator(def)
	validator.collect_declarations()
	validator.collect_references()
	if len(validator.problems) == 0 {
		return nil
	}
	sort.Strings(validator.problems)
	return fmt.Errorf("流程变量作用域校验失败: %s", strings.Join(validator.problems, "; "))
}

type scope_validator struct {
	definition engine.FlowDefinition

	input_keys map[string]bool
	produced   map[string]map[string]bool
	globals    map[string]bool
	successors map[string][]string
	reachable  map[string]map[string]bool
	problems   []string
}

func new_scope_validator(def engine.FlowDefinition) *scope_validator {
	validator := &scope_validator{
		definition: def,
		input_keys: map[string]bool{},
		produced:   map[string]map[string]bool{},
		globals:    map[string]bool{},
		successors: map[string][]string{},
		reachable:  map[string]map[string]bool{},
	}
	for _, field := range def.ContextSchema {
		if key := strings.TrimSpace(field.Key); key != "" {
			validator.input_keys[key] = true
		}
	}
	return validator
}

func (v *scope_validator) problemf(format string, args ...any) {
	v.problems = append(v.problems, fmt.Sprintf(format, args...))
}

// collect_declarations records, for every node, the keys it declares as output
// (output_key plus OutputSchema) and the globals SetVariableNode declares.
// Produced keys must not shadow a context schema key: input and output are
// disjoint scopes, and a shadowed key would reintroduce the collision the
// scopes exist to prevent.
func (v *scope_validator) collect_declarations() {
	for node_id, node := range v.definition.Nodes {
		produced := map[string]bool{}
		output_key := scope_config_string(node.Config, "output_key")
		switch {
		case output_key != "":
			produced[output_key] = true
			if node.Type == "APICallNode" {
				produced[output_key+"_status"] = true
			}
		case node.Type == "ServiceNode":
			produced[default_service_output_key] = true
		case node.Type == "ExprNode" || node.Type == "ValueCalcNode":
			produced[default_expr_output_key] = true
		case node.Type == "APICallNode":
			produced[default_api_output_key] = true
			produced[default_api_output_key+"_status"] = true
		}
		for _, field := range node.OutputSchema {
			if key := strings.TrimSpace(field.Key); key != "" {
				produced[key] = true
			}
		}
		v.produced[node_id] = produced
		if output_key != "" && v.input_keys[output_key] {
			v.problemf("节点 %s 的 output_key %q 与流程入参名冲突，请改名以避免覆盖入参", node_id, output_key)
		}
		if node.Type == "SetVariableNode" {
			for name := range scope_config_object(node.Config, "variables") {
				v.globals[name] = true
			}
		}
	}
}

// collect_references walks every node type that carries a dynamic read and
// validates the scope paths it contains.
func (v *scope_validator) collect_references() {
	for _, node_id := range sorted_node_ids(v.definition) {
		node := v.definition.Nodes[node_id]
		switch node.Type {
		case "ExprNode", "ValueCalcNode":
			v.validate_expression(node_id, scope_config_string(node.Config, "expression"), "expression")
		case "GatewayNode":
			// The language is resolved with the same helper the runtime uses,
			// so a config cannot validate as one language and run as another.
			language := ConditionLanguage(node.Config)
			if language != ConditionLanguageExpr && language != ConditionLanguageJS {
				v.problemf("节点 %s 的 condition_language %q 不支持（仅支持 expr / js）", node_id, language)
			}
			for index, rule := range scope_gateway_rules(node.Config) {
				where := fmt.Sprintf("rules[%d].condition", index)
				v.validate_gateway_rule(node_id, scope_config_string(rule, "condition"), where, language)
			}
		case "ServiceNode":
			for name, value := range scope_config_object(node.Config, "arguments") {
				v.validate_template_value(node_id, value, fmt.Sprintf("arguments.%s", name))
			}
		case "SetVariableNode":
			for name, value := range scope_config_object(node.Config, "variables") {
				v.validate_template_value(node_id, value, fmt.Sprintf("variables.%s", name))
			}
		case "APICallNode":
			for name, value := range scope_config_object(node.Config, "keys") {
				path, ok := value.(string)
				if !ok || strings.TrimSpace(path) == "" {
					v.problemf("节点 %s 的 keys.%s 必须是作用域路径字符串", node_id, name)
					continue
				}
				v.validate_scope_value(node_id, path, fmt.Sprintf("keys.%s", name))
			}
		case "JSCodeNode":
			if scope_config_string(node.Config, "output_key") == "" {
				v.problemf("节点 %s 的 JSCodeNode 必须声明 output_key", node_id)
			}
		case "WorkflowNode", "LoopNode":
			v.validate_sub_flow(node)
		}
	}
}

func (v *scope_validator) validate_sub_flow(node engine.NodeDefinition) {
	sub_flow, ok := node.Config["workflow"].(engine.FlowDefinition)
	if !ok {
		return
	}
	if err := ValidateFlowScopes(sub_flow); err != nil {
		v.problemf("子流程 %s: %v", sub_flow.Name, err)
	}
}

func (v *scope_validator) validate_expression(node_id, code, where string) {
	if strings.TrimSpace(code) == "" {
		return
	}
	paths, bare, err := expression_scope_paths(code)
	if err != nil {
		v.problemf("节点 %s 的 %s 无法解析: %v", node_id, where, err)
		return
	}
	for _, name := range bare {
		v.problemf("节点 %s 的 %s 引用了未声明作用域的名称 %q，请写成 input./output./global. 形式", node_id, where, name)
	}
	for _, path := range paths {
		v.validate_path(node_id, path, where)
	}
}

// validate_gateway_rule validates one Exclusive-gateway rule condition in the
// node's declared language. expr goes through the unchanged validate_expression
// path (built-in flows keep their exact diagnostics); js is scanned with the
// goja AST, which trades the compiler's strict names for a save-time lint that
// catches the same class of typo: a bare identifier, a path that does not
// resolve, or a dynamic index that cannot be checked at all.
func (v *scope_validator) validate_gateway_rule(node_id, code, where, language string) {
	if strings.TrimSpace(code) == "" {
		return
	}
	switch language {
	case ConditionLanguageExpr:
		v.validate_expression(node_id, code, where)
	case ConditionLanguageJS:
		paths, bare, dynamic, err := js_condition_scope_paths(code)
		if err != nil {
			v.problemf("节点 %s 的 %s 无法解析: %v", node_id, where, err)
			return
		}
		for _, name := range bare {
			v.problemf(
				"节点 %s 的 %s（JavaScript 条件）引用了未声明的名称 %q；只能访问 input./output./global. 作用域或标准内置对象（%s）",
				node_id, where, name, strings.Join(js_condition_builtin_name_list, "、"),
			)
		}
		for range dynamic {
			v.problemf(
				"节点 %s 的 %s 使用了动态属性访问，无法在保存时校验，请写成 input.<键> / output.<节点id>.<键> / global.<键> 的静态路径",
				node_id, where,
			)
		}
		for _, path := range paths {
			v.validate_path(node_id, path, where)
		}
	default:
		// The unsupported language is already reported by the caller; the
		// condition cannot be checked without knowing how to read it.
	}
}

// validate_scope_value validates a config value that is either a bare scope
// path (APICallNode.keys) or a {{...}} template.
func (v *scope_validator) validate_scope_value(node_id, value, where string) {
	if strings.Contains(value, "{{") {
		v.validate_template_value(node_id, value, where)
		return
	}
	v.validate_path(node_id, value, where)
}

func (v *scope_validator) validate_template_value(node_id string, value any, where string) {
	paths, err := template_scope_paths(value)
	if err != nil {
		v.problemf("节点 %s 的 %s %v", node_id, where, err)
		return
	}
	for _, path := range paths {
		v.validate_path(node_id, path, where)
	}
}

func (v *scope_validator) validate_path(node_id, path, where string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	segments := strings.Split(path, ".")
	for index := range segments {
		segments[index] = strings.TrimSpace(segments[index])
	}
	prefix := fmt.Sprintf("节点 %s 的 %s 引用了 %s", node_id, where, path)
	switch segments[0] {
	case "input":
		if len(segments) < 2 {
			v.problemf("%s，缺少入参名", prefix)
			return
		}
		if !v.input_keys[segments[1]] {
			v.problemf("%s，但流程 context_schema 未声明入参 %q", prefix, segments[1])
		}
	case "output":
		if len(segments) < 3 {
			v.problemf("%s，缺少生产者节点或产出键", prefix)
			return
		}
		producer, key := segments[1], segments[2]
		produced, known := v.produced[producer]
		switch {
		case !known:
			v.problemf("%s，但不存在生产者节点 %q", prefix, producer)
		case !produced[key]:
			v.problemf("%s，但节点 %q 未声明产出键 %q", prefix, producer, key)
		case !v.can_reach(producer, node_id):
			v.problemf("%s，但节点 %q 不是它的上游（只能读取上游节点的输出）", prefix, producer)
		}
	case "global":
		if len(segments) < 2 {
			v.problemf("%s，缺少全局变量名", prefix)
			return
		}
		if !v.globals[segments[1]] {
			v.problemf("%s，但没有 SetVariableNode 声明全局变量 %q", prefix, segments[1])
		}
	default:
		if len(segments) == 1 {
			v.problemf("%s，但缺少显式作用域（请写成 input./output./global.）", prefix)
			return
		}
		v.problemf("%s，但作用域 %q 不存在", prefix, segments[0])
	}
}

// can_reach reports whether target is reachable from source by following node
// successors, i.e. whether source is an ancestor of target.
func (v *scope_validator) can_reach(source, target string) bool {
	if reachable, ok := v.reachable[source]; ok {
		return reachable[target]
	}
	reachable := map[string]bool{}
	queue := []string{source}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range v.node_successors(current) {
			if reachable[next] {
				continue
			}
			reachable[next] = true
			queue = append(queue, next)
		}
	}
	v.reachable[source] = reachable
	return reachable[target]
}

// node_successors lists every node a definition can drive to, covering both
// NextNodes/NextNodeIDs, the error edge and the rules of an Exclusive gateway
// (whose routing is expressed only in config.rules).
func (v *scope_validator) node_successors(node_id string) []string {
	if successors, ok := v.successors[node_id]; ok {
		return successors
	}
	node, exists := v.definition.Nodes[node_id]
	if !exists {
		v.successors[node_id] = nil
		return nil
	}
	seen := map[string]bool{}
	successors := make([]string, 0, len(node.NextNodeIDs)+len(node.NextNodes))
	append_target := func(target_id string) {
		target_id = strings.TrimSpace(target_id)
		if target_id == "" || seen[target_id] {
			return
		}
		seen[target_id] = true
		successors = append(successors, target_id)
	}
	for _, next_id := range node.NextNodeIDs {
		append_target(next_id)
	}
	for _, target := range node.NextNodes {
		append_target(target.TargetID)
	}
	append_target(node.ErrorNextNodeID)
	for _, rule := range scope_gateway_rules(node.Config) {
		append_target(scope_config_string(rule, "target_id"))
	}
	v.successors[node_id] = successors
	return successors
}

// expression_scope_paths returns the dotted scope paths an expression reads
// (member chains rooted at an identifier) and every root identifier that is not
// part of such a chain. The base of a member chain is the only place a name can
// enter an expression, so anything else is a bare reference.
func expression_scope_paths(code string) (paths []string, bare []string, err error) {
	tree, err := parser.Parse(code)
	if err != nil {
		return nil, nil, err
	}
	collect_expression_scope_paths(&tree.Node, &paths, &bare)
	return paths, bare, nil
}

func collect_expression_scope_paths(node *ast.Node, paths *[]string, bare *[]string) {
	if node == nil || *node == nil {
		return
	}
	switch typed := (*node).(type) {
	case *ast.IdentifierNode:
		*bare = append(*bare, typed.Value)
	case *ast.MemberNode:
		if path, ok := member_node_path(typed); ok {
			*paths = append(*paths, path)
			return
		}
		// Not a static a.b.c chain (a computed index/pointer, e.g. a[0]): the
		// base can still hold a chain. The property of a static access is a
		// string literal that must not be reported as a bare reference; any
		// other property is a computed expression and is scanned.
		collect_expression_scope_paths(&typed.Node, paths, bare)
		if _, is_static := typed.Property.(*ast.StringNode); !is_static {
			collect_expression_scope_paths(&typed.Property, paths, bare)
		}
	case *ast.UnaryNode:
		collect_expression_scope_paths(&typed.Node, paths, bare)
	case *ast.BinaryNode:
		collect_expression_scope_paths(&typed.Left, paths, bare)
		collect_expression_scope_paths(&typed.Right, paths, bare)
	case *ast.ChainNode:
		collect_expression_scope_paths(&typed.Node, paths, bare)
	case *ast.SliceNode:
		collect_expression_scope_paths(&typed.Node, paths, bare)
		collect_expression_scope_paths(&typed.From, paths, bare)
		collect_expression_scope_paths(&typed.To, paths, bare)
	case *ast.CallNode:
		collect_expression_scope_paths(&typed.Callee, paths, bare)
		for index := range typed.Arguments {
			collect_expression_scope_paths(&typed.Arguments[index], paths, bare)
		}
	case *ast.BuiltinNode:
		for index := range typed.Arguments {
			collect_expression_scope_paths(&typed.Arguments[index], paths, bare)
		}
	case *ast.ClosureNode:
		collect_expression_scope_paths(&typed.Node, paths, bare)
	case *ast.VariableDeclaratorNode:
		collect_expression_scope_paths(&typed.Value, paths, bare)
		collect_expression_scope_paths(&typed.Expr, paths, bare)
	case *ast.ConditionalNode:
		collect_expression_scope_paths(&typed.Cond, paths, bare)
		collect_expression_scope_paths(&typed.Exp1, paths, bare)
		collect_expression_scope_paths(&typed.Exp2, paths, bare)
	case *ast.ArrayNode:
		for index := range typed.Nodes {
			collect_expression_scope_paths(&typed.Nodes[index], paths, bare)
		}
	case *ast.MapNode:
		for index := range typed.Pairs {
			// Map keys are literals; only the values can read a scope.
			pair, ok := typed.Pairs[index].(*ast.PairNode)
			if !ok {
				continue
			}
			collect_expression_scope_paths(&pair.Value, paths, bare)
		}
	}
}

// member_node_path flattens an a.b.c member chain into a dotted path. The
// parser represents a static property access (a.b) with a string-literal
// property node, so only a non-literal property (e.g. the 0 of a[0]) breaks
// the chain.
func member_node_path(node *ast.MemberNode) (string, bool) {
	segments := make([]string, 0, 4)
	var current ast.Node = node
	for {
		member, ok := current.(*ast.MemberNode)
		if !ok {
			break
		}
		property, ok := member.Property.(*ast.StringNode)
		if !ok {
			return "", false
		}
		segments = append([]string{property.Value}, segments...)
		current = member.Node
	}
	root, ok := current.(*ast.IdentifierNode)
	if !ok {
		return "", false
	}
	return strings.Join(append([]string{root.Value}, segments...), "."), true
}

// template_scope_paths extracts the scope paths of every {{...}} token in a
// template value. A value containing "{{" but no valid token is an error, the
// same rule the runtime resolver applies.
func template_scope_paths(value any) ([]string, error) {
	text, ok := value.(string)
	if !ok || !strings.Contains(text, "{{") {
		return nil, nil
	}
	matches := service_template_re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("包含无效的模板表达式: %s", text)
	}
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		paths = append(paths, text[match[2]:match[3]])
	}
	return paths, nil
}

func sorted_node_ids(def engine.FlowDefinition) []string {
	node_ids := make([]string, 0, len(def.Nodes))
	for node_id := range def.Nodes {
		node_ids = append(node_ids, node_id)
	}
	sort.Strings(node_ids)
	return node_ids
}

func scope_config_string(config map[string]interface{}, key string) string {
	value, ok := config[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func scope_config_object(config map[string]interface{}, key string) map[string]interface{} {
	value, ok := config[key].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return value
}

func scope_gateway_rules(config map[string]interface{}) []map[string]interface{} {
	rules, err := gateway_rules(config["rules"])
	if err != nil {
		return nil
	}
	return rules
}
