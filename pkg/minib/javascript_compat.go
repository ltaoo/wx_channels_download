package minib

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

var source_map_comment_pattern = regexp.MustCompile(`(?m)^//# sourceMappingURL=.*\r?\n?$`)

// normalize_goja_destructured_vars works around a goja compiler issue: a
// function with a destructured parameter rejects a later var declaration that
// redeclares one of the parameter's binding names. Browsers accept this form.
//
// Goja represents the redundant declaration as a syntax error, so this
// transformation is only attempted after a direct compilation has failed. It
// splits the affected var declarations into assignments while preserving each
// binding's original source (including parentheses that affect precedence).
func normalize_goja_destructured_vars(source string) (string, bool) {
	// Goja eagerly resolves sourceMappingURL comments. A missing local map
	// would otherwise mask the actual compiler error and perturb source offsets.
	stripped_source := source_map_comment_pattern.ReplaceAllString(source, "")
	source_changed := stripped_source != source
	source = stripped_source
	source_bytes := []byte(source)
	program, parse_err := parser.ParseFile(nil, "minib-goja-compat", source, 0)
	if program == nil || parse_err != nil {
		return source, false
	}

	functions := collect_javascript_functions(program)
	edits := make(map[int]javascript_source_edit)
	for _, function := range functions {
		parameter_names := destructured_parameter_names(function.parameter_list)
		if len(parameter_names) == 0 {
			continue
		}
		for _, declaration := range function.declaration_list {
			if !var_declaration_overlaps_names(declaration, parameter_names) {
				continue
			}
			start := int(declaration.Var) - 1
			if !standalone_var_statement(source_bytes, start) {
				continue
			}
			end := int(declaration.Idx1()) - 1
			if start < 0 || end < start || end > len(source_bytes) {
				continue
			}

			parts := make([]string, 0, len(declaration.List))
			for _, binding := range declaration.List {
				binding_source := javascript_node_source(source_bytes, binding, binding)
				if binding_target_overlaps_names(binding.Target, parameter_names) {
					if binding.Initializer != nil {
						parts = append(parts, "("+binding_source+")")
					}
					continue
				}
				parts = append(parts, "var "+binding_source)
			}
			if _, already_edited := edits[start]; !already_edited {
				edits[start] = javascript_source_edit{
					start:       start,
					end:         end,
					replacement: strings.Join(parts, ";"),
				}
			}
		}
	}
	if len(edits) == 0 {
		return source, source_changed
	}

	ordered_edits := make([]javascript_source_edit, 0, len(edits))
	for _, edit := range edits {
		ordered_edits = append(ordered_edits, edit)
	}
	// Apply edits in reverse order so byte offsets remain valid.
	sort_javascript_source_edits(ordered_edits)
	for _, edit := range ordered_edits {
		source_bytes = append(
			source_bytes[:edit.start],
			append([]byte(edit.replacement), source_bytes[edit.end:]...)...,
		)
	}
	return string(source_bytes), true
}

type javascript_source_edit struct {
	start       int
	end         int
	replacement string
}

func sort_javascript_source_edits(edits []javascript_source_edit) {
	for left := 1; left < len(edits); left++ {
		for right := left; right > 0 && edits[right].start > edits[right-1].start; right-- {
			edits[right], edits[right-1] = edits[right-1], edits[right]
		}
	}
}

type javascript_function_nodes struct {
	parameter_list   *ast.ParameterList
	declaration_list []*ast.VariableDeclaration
}

func collect_javascript_functions(program *ast.Program) []javascript_function_nodes {
	var functions []javascript_function_nodes
	seen := make(map[any]struct{})
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		switch value.Kind() {
		case reflect.Interface:
			walk(value.Elem())
		case reflect.Pointer:
			if value.IsNil() {
				return
			}
			switch node := value.Interface().(type) {
			case *ast.FunctionLiteral:
				if mark_javascript_function(value, seen) {
					functions = append(functions, javascript_function_nodes{
						parameter_list:   node.ParameterList,
						declaration_list: node.DeclarationList,
					})
				}
			case *ast.ArrowFunctionLiteral:
				if mark_javascript_function(value, seen) {
					functions = append(functions, javascript_function_nodes{
						parameter_list:   node.ParameterList,
						declaration_list: node.DeclarationList,
					})
				}
			}
			walk(value.Elem())
		case reflect.Struct:
			for field_index := 0; field_index < value.NumField(); field_index++ {
				if value.Type().Field(field_index).PkgPath == "" {
					walk(value.Field(field_index))
				}
			}
		case reflect.Slice, reflect.Array:
			for element_index := 0; element_index < value.Len(); element_index++ {
				walk(value.Index(element_index))
			}
		}
	}
	walk(reflect.ValueOf(program))
	return functions
}

func mark_javascript_function(value reflect.Value, seen map[any]struct{}) bool {
	key := value.Interface()
	if _, duplicate := seen[key]; duplicate {
		return false
	}
	seen[key] = struct{}{}
	return true
}

func destructured_parameter_names(parameter_list *ast.ParameterList) map[string]bool {
	if parameter_list == nil {
		return nil
	}
	names := make(map[string]bool)
	for _, binding := range parameter_list.List {
		if binding != nil {
			if _, is_pattern := binding.Target.(ast.Pattern); !is_pattern {
				continue
			}
			collect_binding_pattern_names(binding.Target, names)
		}
	}
	if _, is_pattern := parameter_list.Rest.(ast.Pattern); is_pattern {
		collect_binding_pattern_names(parameter_list.Rest, names)
	}
	return names
}

func collect_binding_pattern_names(target ast.Expression, names map[string]bool) {
	if target == nil {
		return
	}
	switch target := target.(type) {
	case *ast.Identifier:
		names[string(target.Name)] = true
	case *ast.ObjectPattern:
		for _, property := range target.Properties {
			switch property := property.(type) {
			case *ast.PropertyShort:
				collect_binding_pattern_names(&property.Name, names)
			case *ast.PropertyKeyed:
				collect_binding_pattern_names(property.Value, names)
			}
		}
		if target.Rest != nil {
			collect_binding_pattern_names(target.Rest, names)
		}
	case *ast.ArrayPattern:
		for _, element := range target.Elements {
			if element != nil {
				collect_binding_pattern_names(element, names)
			}
		}
		if target.Rest != nil {
			collect_binding_pattern_names(target.Rest, names)
		}
	case *ast.AssignExpression:
		collect_binding_pattern_names(target.Left, names)
	}
}

func var_declaration_overlaps_names(declaration *ast.VariableDeclaration, parameter_names map[string]bool) bool {
	if declaration == nil {
		return false
	}
	for _, binding := range declaration.List {
		if binding_target_overlaps_names(binding.Target, parameter_names) {
			return true
		}
	}
	return false
}

func binding_target_overlaps_names(target ast.BindingTarget, parameter_names map[string]bool) bool {
	if target == nil || len(parameter_names) == 0 {
		return false
	}
	if identifier, ok := target.(*ast.Identifier); ok {
		return parameter_names[string(identifier.Name)]
	}
	if _, is_pattern := target.(ast.Pattern); !is_pattern {
		return false
	}
	target_names := make(map[string]bool)
	collect_binding_pattern_names(target, target_names)
	for name := range target_names {
		if parameter_names[name] {
			return true
		}
	}
	return false
}

func standalone_var_statement(source []byte, start int) bool {
	if start <= 0 {
		return true
	}
	for index := start - 1; index >= 0; index-- {
		switch source[index] {
		case ' ', '\t', '\r', '\n':
			continue
		case ';', '{', '}':
			return true
		default:
			return false
		}
	}
	return true
}

func javascript_node_source(source []byte, start ast.Node, end ast.Node) string {
	start_index := int(start.Idx0()) - 1
	end_index := int(end.Idx1()) - 1
	if start_index < 0 || end_index < start_index || end_index > len(source) {
		return ""
	}
	return string(source[start_index:end_index])
}
