package nodes

import (
	"reflect"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

// js_ast_package_path guards the reflection walk in js_walk_nodes: only nodes
// of this package are descended into, so an embedded *file.File (source text,
// positions) is never traversed.
const js_ast_package_path = "github.com/dop251/goja/ast"

// js_condition_builtin_name_list is the standard globals a JavaScript
// condition may reference without an explicit scope. It is a lint aid, not a
// security boundary: goja defines eval/Function, so a script can still reach
// arbitrary code through e.g. `[].constructor.constructor(...)` without naming
// any identifier. What this list buys is a typo like `username` being a save
// error instead of a silent undefined.
var js_condition_builtin_name_list = []string{
	"Math", "JSON", "Number", "String", "Boolean", "Date", "RegExp",
	"Array", "Object", "parseInt", "parseFloat", "isNaN", "isFinite",
	"Infinity", "NaN", "undefined",
}

// js_condition_builtin_names is the lookup form of the list above. null / true
// / false are literals and never reach an Identifier, so they are absent;
// globalThis and console are deliberately not allowed.
var js_condition_builtin_names = func() map[string]bool {
	names := make(map[string]bool, len(js_condition_builtin_name_list))
	for _, name := range js_condition_builtin_name_list {
		names[name] = true
	}
	return names
}()

// js_condition_scope_paths scans a JavaScript condition and returns the scope
// paths it reads (input.<key>, output.<node_id>.<key>, global.<key>), the root
// identifiers that name neither a scope nor a declared local, and the scope
// roots reached through a dynamic index (input[k]) that cannot be checked
// statically. The validator turns each list into its own diagnostic.
//
// It parses with the same goja parser that runs the script, so a condition the
// validator accepts is at least syntactically executable.
func js_condition_scope_paths(code string) (paths []string, bare []string, dynamic []string, err error) {
	program, err := parser.ParseFile(nil, "gateway-condition", code, 0, parser.WithDisableSourceMaps)
	if err != nil {
		return nil, nil, nil, err
	}
	if program == nil {
		return nil, nil, nil, nil
	}
	// Two passes: `let` can be read before its declaration, so every binding
	// anywhere in the program counts before any read is classified.
	declared := js_collect_declared(program)
	for _, statement := range program.Body {
		js_collect_reads(statement, declared, &paths, &bare, &dynamic)
	}
	return paths, bare, dynamic, nil
}

// js_collect_declared gathers every binding name in the program: var / let /
// const bindings (including destructuring patterns), function names and
// parameters, for-in/of loop variables and catch parameters. A name declared
// anywhere suppresses the bare-name report for that name everywhere; this
// biases toward under-reporting, which is the acceptable direction for a
// save-time lint.
func js_collect_declared(program *ast.Program) map[string]bool {
	declared := map[string]bool{}
	js_walk_nodes(program, func(node ast.Node) {
		switch typed := node.(type) {
		case *ast.Binding:
			js_collect_binding_names(typed.Target, declared)
		case *ast.CatchStatement:
			js_collect_binding_names(typed.Parameter, declared)
		case *ast.ForDeclaration:
			js_collect_binding_names(typed.Target, declared)
		case *ast.ParameterList:
			if typed.Rest != nil {
				js_collect_binding_names(typed.Rest, declared)
			}
		case *ast.FunctionDeclaration:
			js_collect_function_name(typed.Function, declared)
		case *ast.FunctionLiteral:
			js_collect_function_name(typed, declared)
		case *ast.ClassDeclaration:
			if typed.Class.Name != nil {
				declared[string(typed.Class.Name.Name)] = true
			}
		case *ast.ClassLiteral:
			if typed.Name != nil {
				declared[string(typed.Name.Name)] = true
			}
		}
	})
	return declared
}

func js_collect_function_name(function *ast.FunctionLiteral, declared map[string]bool) {
	if function != nil && function.Name != nil {
		declared[string(function.Name.Name)] = true
	}
}

// js_collect_binding_names records the identifiers bound by a binding target,
// descending into object/array destructuring patterns and their defaults. It
// mirrors the shape used by pkg/minib/javascript_compat.go.
func js_collect_binding_names(target ast.Expression, names map[string]bool) {
	if js_node_is_nil(target) {
		return
	}
	switch typed := target.(type) {
	case *ast.Identifier:
		names[string(typed.Name)] = true
	case *ast.ObjectPattern:
		for _, property := range typed.Properties {
			switch typed_property := property.(type) {
			case *ast.PropertyShort:
				js_collect_binding_names(&typed_property.Name, names)
			case *ast.PropertyKeyed:
				js_collect_binding_names(typed_property.Value, names)
			}
		}
		if typed.Rest != nil {
			js_collect_binding_names(typed.Rest, names)
		}
	case *ast.ArrayPattern:
		for _, element := range typed.Elements {
			if element != nil {
				js_collect_binding_names(element, names)
			}
		}
		if typed.Rest != nil {
			js_collect_binding_names(typed.Rest, names)
		}
	case *ast.AssignExpression:
		js_collect_binding_names(typed.Left, names)
	}
}

// js_walk_nodes visits every AST node reachable from root, in no particular
// order. Reflection is used so a declaration hidden in an unusual statement
// (a function inside a loop inside a block) is still found; only nodes from
// the ast package are descended into.
func js_walk_nodes(root ast.Node, visit func(ast.Node)) {
	visited := map[uintptr]bool{}
	var walk func(value reflect.Value)
	walk = func(value reflect.Value) {
		switch value.Kind() {
		case reflect.Interface:
			if value.IsNil() {
				return
			}
			walk(value.Elem())
		case reflect.Pointer:
			if value.IsNil() || value.Type().Elem().PkgPath() != js_ast_package_path {
				return
			}
			pointer := value.Pointer()
			if visited[pointer] {
				return
			}
			visited[pointer] = true
			if node, ok := value.Interface().(ast.Node); ok {
				visit(node)
			}
			walk(value.Elem())
		case reflect.Struct:
			if value.Type().PkgPath() != js_ast_package_path {
				return
			}
			for index := 0; index < value.NumField(); index++ {
				if value.Type().Field(index).PkgPath == "" {
					walk(value.Field(index))
				}
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				walk(value.Index(index))
			}
		}
	}
	walk(reflect.ValueOf(root))
}

// js_node_is_nil reports whether an AST node is absent. It covers the
// typed-nil pointer a bare `node == nil` misses: an optional AST field such as
// TryStatement.Finally is a nil *BlockStatement, and stored in an interface it
// compares non-nil while dereferencing it panics.
func js_node_is_nil(node any) bool {
	if node == nil {
		return true
	}
	value := reflect.ValueOf(node)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

// js_collect_reads classifies every identifier the expression can read. A
// member chain rooted at a scope becomes a dotted path; the base identifier is
// never also reported, so `input.a.b` yields `input.a.b` alone.
func js_collect_reads(node ast.Node, declared map[string]bool, paths, bare, dynamic *[]string) {
	if js_node_is_nil(node) {
		return
	}
	switch typed := node.(type) {
	case *ast.DotExpression:
		js_visit_member(node, declared, paths, bare, dynamic)
	case *ast.BracketExpression:
		js_visit_member(node, declared, paths, bare, dynamic)
	case *ast.Optional:
		js_collect_reads(typed.Expression, declared, paths, bare, dynamic)
	case *ast.OptionalChain:
		js_collect_reads(typed.Expression, declared, paths, bare, dynamic)
	case *ast.Identifier:
		js_visit_identifier(typed, declared, bare)

	// --- expressions --------------------------------------------------------
	case *ast.AssignExpression:
		js_collect_reads(typed.Left, declared, paths, bare, dynamic)
		js_collect_reads(typed.Right, declared, paths, bare, dynamic)
	case *ast.AwaitExpression:
		js_collect_reads(typed.Argument, declared, paths, bare, dynamic)
	case *ast.YieldExpression:
		js_collect_reads(typed.Argument, declared, paths, bare, dynamic)
	case *ast.BinaryExpression:
		js_collect_reads(typed.Left, declared, paths, bare, dynamic)
		js_collect_reads(typed.Right, declared, paths, bare, dynamic)
	case *ast.UnaryExpression:
		js_collect_reads(typed.Operand, declared, paths, bare, dynamic)
	case *ast.ConditionalExpression:
		js_collect_reads(typed.Test, declared, paths, bare, dynamic)
		js_collect_reads(typed.Consequent, declared, paths, bare, dynamic)
		js_collect_reads(typed.Alternate, declared, paths, bare, dynamic)
	case *ast.SequenceExpression:
		for _, expression := range typed.Sequence {
			js_collect_reads(expression, declared, paths, bare, dynamic)
		}
	case *ast.CallExpression:
		js_collect_reads(typed.Callee, declared, paths, bare, dynamic)
		js_collect_reads_all(typed.ArgumentList, declared, paths, bare, dynamic)
	case *ast.NewExpression:
		js_collect_reads(typed.Callee, declared, paths, bare, dynamic)
		js_collect_reads_all(typed.ArgumentList, declared, paths, bare, dynamic)
	case *ast.ArrayLiteral:
		js_collect_reads_all(typed.Value, declared, paths, bare, dynamic)
	case *ast.ObjectLiteral:
		for _, property := range typed.Value {
			js_collect_property_reads(property, declared, paths, bare, dynamic)
		}
	case *ast.TemplateLiteral:
		// Elements are the literal text; only the interpolations can read.
		js_collect_reads_all(typed.Expressions, declared, paths, bare, dynamic)
	case *ast.ArrowFunctionLiteral:
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.FunctionLiteral:
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.ClassLiteral:
		js_collect_reads(typed.SuperClass, declared, paths, bare, dynamic)
		for _, element := range typed.Body {
			js_collect_class_element_reads(element, declared, paths, bare, dynamic)
		}
	case *ast.PrivateDotExpression:
		js_collect_reads(typed.Left, declared, paths, bare, dynamic)

	// --- expressions that bind (patterns, mostly inert) ---------------------
	case *ast.Binding:
		js_collect_reads(typed.Target, declared, paths, bare, dynamic)
		js_collect_reads(typed.Initializer, declared, paths, bare, dynamic)
	case *ast.ObjectPattern:
		for _, property := range typed.Properties {
			js_collect_property_reads(property, declared, paths, bare, dynamic)
		}
		js_collect_reads(typed.Rest, declared, paths, bare, dynamic)
	case *ast.ArrayPattern:
		js_collect_reads_all(typed.Elements, declared, paths, bare, dynamic)
		js_collect_reads(typed.Rest, declared, paths, bare, dynamic)

	// --- statements ---------------------------------------------------------
	case *ast.ExpressionStatement:
		js_collect_reads(typed.Expression, declared, paths, bare, dynamic)
	case *ast.BlockStatement:
		js_collect_reads_all(typed.List, declared, paths, bare, dynamic)
	case *ast.VariableStatement:
		js_collect_reads_all(typed.List, declared, paths, bare, dynamic)
	case *ast.LexicalDeclaration:
		js_collect_reads_all(typed.List, declared, paths, bare, dynamic)
	case *ast.VariableDeclaration:
		js_collect_reads_all(typed.List, declared, paths, bare, dynamic)
	case *ast.IfStatement:
		js_collect_reads(typed.Test, declared, paths, bare, dynamic)
		js_collect_reads(typed.Consequent, declared, paths, bare, dynamic)
		js_collect_reads(typed.Alternate, declared, paths, bare, dynamic)
	case *ast.ForStatement:
		js_collect_reads(typed.Initializer, declared, paths, bare, dynamic)
		js_collect_reads(typed.Test, declared, paths, bare, dynamic)
		js_collect_reads(typed.Update, declared, paths, bare, dynamic)
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.ForLoopInitializerExpression:
		js_collect_reads(typed.Expression, declared, paths, bare, dynamic)
	case *ast.ForLoopInitializerVarDeclList:
		js_collect_reads_all(typed.List, declared, paths, bare, dynamic)
	case *ast.ForLoopInitializerLexicalDecl:
		js_collect_reads_all(typed.LexicalDeclaration.List, declared, paths, bare, dynamic)
	case *ast.ForInStatement:
		js_collect_reads(typed.Into, declared, paths, bare, dynamic)
		js_collect_reads(typed.Source, declared, paths, bare, dynamic)
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.ForOfStatement:
		js_collect_reads(typed.Into, declared, paths, bare, dynamic)
		js_collect_reads(typed.Source, declared, paths, bare, dynamic)
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.ForIntoVar:
		js_collect_reads(typed.Binding, declared, paths, bare, dynamic)
	case *ast.ForIntoExpression:
		js_collect_reads(typed.Expression, declared, paths, bare, dynamic)
	case *ast.ForDeclaration:
		js_collect_reads(typed.Target, declared, paths, bare, dynamic)
	case *ast.WhileStatement:
		js_collect_reads(typed.Test, declared, paths, bare, dynamic)
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.DoWhileStatement:
		js_collect_reads(typed.Test, declared, paths, bare, dynamic)
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.ReturnStatement:
		js_collect_reads(typed.Argument, declared, paths, bare, dynamic)
	case *ast.ThrowStatement:
		js_collect_reads(typed.Argument, declared, paths, bare, dynamic)
	case *ast.LabelledStatement:
		js_collect_reads(typed.Statement, declared, paths, bare, dynamic)
	case *ast.SwitchStatement:
		js_collect_reads(typed.Discriminant, declared, paths, bare, dynamic)
		for _, case_statement := range typed.Body {
			js_collect_reads(case_statement, declared, paths, bare, dynamic)
		}
	case *ast.CaseStatement:
		js_collect_reads(typed.Test, declared, paths, bare, dynamic)
		js_collect_reads_all(typed.Consequent, declared, paths, bare, dynamic)
	case *ast.TryStatement:
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
		js_collect_reads(typed.Catch, declared, paths, bare, dynamic)
		js_collect_reads(typed.Finally, declared, paths, bare, dynamic)
	case *ast.CatchStatement:
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.WithStatement:
		js_collect_reads(typed.Object, declared, paths, bare, dynamic)
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.FunctionDeclaration:
		js_collect_reads(typed.Function, declared, paths, bare, dynamic)
	case *ast.ClassDeclaration:
		js_collect_reads(typed.Class, declared, paths, bare, dynamic)
	case *ast.Program:
		js_collect_reads_all(typed.Body, declared, paths, bare, dynamic)
	case *ast.ExpressionBody:
		js_collect_reads(typed.Expression, declared, paths, bare, dynamic)
	}
}

func js_collect_reads_all[T ast.Node](nodes []T, declared map[string]bool, paths, bare, dynamic *[]string) {
	for _, node := range nodes {
		js_collect_reads(node, declared, paths, bare, dynamic)
	}
}

func js_collect_property_reads(property ast.Property, declared map[string]bool, paths, bare, dynamic *[]string) {
	if js_node_is_nil(property) {
		return
	}
	switch typed := property.(type) {
	case *ast.PropertyShort:
		js_collect_reads(&typed.Name, declared, paths, bare, dynamic)
		js_collect_reads(typed.Initializer, declared, paths, bare, dynamic)
	case *ast.PropertyKeyed:
		if typed.Computed {
			js_collect_reads(typed.Key, declared, paths, bare, dynamic)
		}
		js_collect_reads(typed.Value, declared, paths, bare, dynamic)
	case *ast.SpreadElement:
		js_collect_reads(typed.Expression, declared, paths, bare, dynamic)
	}
}

func js_collect_class_element_reads(element ast.ClassElement, declared map[string]bool, paths, bare, dynamic *[]string) {
	if js_node_is_nil(element) {
		return
	}
	switch typed := element.(type) {
	case *ast.FieldDefinition:
		if typed.Computed {
			js_collect_reads(typed.Key, declared, paths, bare, dynamic)
		}
		js_collect_reads(typed.Initializer, declared, paths, bare, dynamic)
	case *ast.MethodDefinition:
		if typed.Computed {
			js_collect_reads(typed.Key, declared, paths, bare, dynamic)
		}
		js_collect_reads(typed.Body, declared, paths, bare, dynamic)
	case *ast.ClassStaticBlock:
		js_collect_reads(typed.Block, declared, paths, bare, dynamic)
	}
}

// js_visit_member classifies one member access. Everything hinges on how much
// of the chain is statically known:
//
//   - rooted at a scope (and not shadowed by a local): the read is reported as
//     a dotted path and the traversal stops, so `input.a.b` does not also
//     report `input.a`.
//   - rooted at anything else: only the base is classified, so a whitelisted
//     built-in (`Math.max(...)`) passes and an undeclared name is a bare-name
//     report. Dot properties are never identifiers and are not descended into.
//   - a dynamic index on a scope (`input[k]`, `input.a[k]`): not checkable at
//     save time, reported on its own.
//   - anything else (a call result, a literal base): descend into the base,
//     plus the index expression of a bracket access.
func js_visit_member(node ast.Node, declared map[string]bool, paths, bare, dynamic *[]string) {
	root, segments, ok := js_static_chain(node)
	if ok {
		switch {
		case is_scope_root(root) && !declared[root]:
			*paths = append(*paths, js_join_path(root, segments))
		case is_scope_root(root):
			// The scope name is shadowed by a local binding; treat it as the
			// local and report nothing.
		default:
			js_descend_member_base(node, declared, paths, bare, dynamic)
		}
		return
	}
	if base_root, _, base_ok := js_static_chain(js_member_base(node)); base_ok && is_scope_root(base_root) && !declared[base_root] {
		*dynamic = append(*dynamic, base_root)
		return
	}
	js_descend_member_base(node, declared, paths, bare, dynamic)
}

// js_descend_member_base recurses into the parts of an unflattenable member
// access that can still read a scope: the base, and a bracket access's index
// when it is not a literal property name. A dot access's property is a name,
// not a value, and is skipped.
func js_descend_member_base(node ast.Node, declared map[string]bool, paths, bare, dynamic *[]string) {
	switch typed := js_unwrap_optional_node(node).(type) {
	case *ast.DotExpression:
		js_collect_reads(typed.Left, declared, paths, bare, dynamic)
	case *ast.BracketExpression:
		js_collect_reads(typed.Left, declared, paths, bare, dynamic)
		if _, is_literal := typed.Member.(*ast.StringLiteral); !is_literal {
			js_collect_reads(typed.Member, declared, paths, bare, dynamic)
		}
	}
}

// js_member_base returns the base expression of a member access, unwrapping
// optional-chain syntax, or nil when there is nothing to descend into.
func js_member_base(node ast.Node) ast.Expression {
	switch typed := js_unwrap_optional_node(node).(type) {
	case *ast.DotExpression:
		return typed.Left
	case *ast.BracketExpression:
		return typed.Left
	}
	return nil
}

// js_static_chain flattens a static member chain into its root identifier and
// the property names it reads, outside-in. Optional / OptionalChain wrappers
// (a?.b) are transparent. A bracket access breaks the chain unless its member
// is a string literal, so `input["username"]` flattens but `input[k]` does not.
func js_static_chain(node ast.Node) (root string, segments []string, ok bool) {
	expression := js_unwrap_optional_node(node)
	for {
		switch typed := expression.(type) {
		case *ast.DotExpression:
			segments = append(segments, string(typed.Identifier.Name))
			expression = js_unwrap_optional_node(typed.Left)
		case *ast.BracketExpression:
			literal, is_literal := typed.Member.(*ast.StringLiteral)
			if !is_literal {
				return "", nil, false
			}
			segments = append(segments, string(literal.Value))
			expression = js_unwrap_optional_node(typed.Left)
		case *ast.Identifier:
			return string(typed.Name), segments, true
		default:
			return "", nil, false
		}
	}
}

// js_unwrap_optional_node strips the Optional / OptionalChain wrappers goja
// puts around an optional access, leaving the underlying expression. It takes
// the generic ast.Node because a chain's base is reachable both as an
// Expression and as a Node.
func js_unwrap_optional_node(node ast.Node) ast.Node {
	for !js_node_is_nil(node) {
		switch typed := node.(type) {
		case *ast.OptionalChain:
			node = typed.Expression
		case *ast.Optional:
			node = typed.Expression
		default:
			return node
		}
	}
	return nil
}

// js_visit_identifier reports an identifier that is neither a declared local
// nor a whitelisted built-in. Scope roots reach this point only when they are
// read bare (e.g. `input && input.username`), which the message calls out.
func js_visit_identifier(identifier *ast.Identifier, declared map[string]bool, bare *[]string) {
	name := string(identifier.Name)
	if declared[name] || js_condition_builtin_names[name] {
		return
	}
	*bare = append(*bare, name)
}

// js_join_path reverses the outside-in segments and joins them onto the root.
func js_join_path(root string, segments []string) string {
	parts := make([]string, 0, len(segments)+1)
	parts = append(parts, root)
	for index := len(segments) - 1; index >= 0; index-- {
		parts = append(parts, segments[index])
	}
	return strings.Join(parts, ".")
}

func is_scope_root(name string) bool {
	switch name {
	case "input", "output", "global":
		return true
	}
	return false
}
