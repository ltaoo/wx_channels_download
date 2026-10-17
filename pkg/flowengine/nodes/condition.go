package nodes

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"wx_channel/pkg/flowengine/engine"

	"github.com/expr-lang/expr"
)

// The languages a GatewayNode condition can be written in. expr is the
// default and the only language the built-in wxchannels flows use; js is
// opt-in per node via config["condition_language"].
const (
	ConditionLanguageExpr = "expr"
	ConditionLanguageJS   = "js"
)

// ConditionLanguage normalizes config["condition_language"]: a missing, empty
// or blank value means expr. Anything else is trimmed and lower-cased and
// returned as-is, so an unsupported value ("python", a number) reaches the
// validator instead of being silently treated as expr.
//
// The runtime (EvaluateCondition) and the static validator share this one
// function: if they disagreed about what a config means, a flow could validate
// under one language and run under the other.
func ConditionLanguage(config map[string]interface{}) string {
	value, ok := config["condition_language"]
	if !ok {
		return ConditionLanguageExpr
	}
	language, ok := value.(string)
	if !ok {
		return fmt.Sprintf("%v", value)
	}
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return ConditionLanguageExpr
	}
	return language
}

// EvaluateCondition evaluates one Exclusive-gateway rule condition in the given
// language. language must be ConditionLanguageExpr or ConditionLanguageJS;
// anything else is a configuration error (the validator rejects it at save
// time, so it only surfaces on flows written before validation existed).
//
// The two languages differ in more than syntax, and the difference can flip a
// branch when a node is switched:
//
//   - expr returns only a bool; any other result is silently false.
//   - js uses JavaScript truthiness, so a non-empty string, an object or a
//     non-empty array is true (e.g. `input.username` passes where the same
//     condition in expr is always false).
//
// A syntax error or a thrown exception fails the node in both languages.
func EvaluateCondition(ctx *engine.ProcessContext, condition, language string) (bool, error) {
	switch language {
	case ConditionLanguageExpr:
		return evaluate_expr_condition(ctx, condition)
	case ConditionLanguageJS:
		return evaluate_js_condition(ctx, condition)
	default:
		return false, fmt.Errorf("GatewayNode: 不支持的条件语言 %q（仅支持 expr / js）", language)
	}
}

// evaluate_expr_condition is the GatewayNode's original evaluator, moved here
// verbatim. expr.Env turns on strict name resolution so a condition can only
// read the three declared scopes; a non-bool result is false, a compile or run
// error fails the node. Built-in flows take exactly this path, which is why it
// must stay byte-for-byte equivalent.
func evaluate_expr_condition(ctx *engine.ProcessContext, condition string) (bool, error) {
	scope_env := ctx.ScopeEnv()
	program, err := expr.Compile(condition, expr.Env(scope_env))
	if err != nil {
		return false, err
	}
	out, err := expr.Run(program, scope_env)
	if err != nil {
		return false, err
	}
	b, ok := out.(bool)
	if !ok {
		return false, nil
	}
	return b, nil
}

// evaluate_js_condition runs a JavaScript condition in a scope-bound goja VM
// and reduces its completion value to a bool with JavaScript truthiness
// (undefined / null / 0 / "" / NaN are false; a non-empty string, object or
// array is true). Each rule gets a fresh VM, so a `let` in one rule cannot
// leak into the next; this matches JSCodeNode.
//
// Scope access is `input.x` / `output.<node_id>.x` / `global.x`, the same three
// objects JSCodeNode binds (see new_scope_vm). Note the Go maps are exposed as
// live objects without the full Object prototype: member access, `Object.keys`
// and `JSON.stringify` work, while `"x" in input` and `input.hasOwnProperty`
// are not reliable. A missing key reads as `undefined`, so conditions should
// prefer `input.x != null` or `typeof input.x !== "undefined"` over `!= ""`.
//
// The static validator is a lint, not a sandbox: goja defines eval/Function, so
// a script can reach arbitrary code through `[].constructor.constructor(...)`
// without naming any identifier. That is not a new privilege — JSCodeNode can
// already do it — but the whitelist must not be sold as a security boundary.
func evaluate_js_condition(ctx *engine.ProcessContext, condition string) (bool, error) {
	if strings.TrimSpace(condition) == "" {
		return false, errors.New("GatewayNode: condition is empty")
	}
	vm, err := new_scope_vm(ctx.ScopeEnv())
	if err != nil {
		return false, err
	}
	// A condition that never returns must not occupy the flow goroutine: the
	// gateway sits on the main path of every run (the built-in wxchannels
	// flows end with one), so a `while (true) {}` would hang the download
	// postprocess with no way out. Interrupt only breaks out of JavaScript
	// code, which is where an infinite loop lives.
	watchdog := time.AfterFunc(time.Second, func() { vm.Interrupt("condition timeout") })
	defer watchdog.Stop()
	result, err := vm.RunString(condition)
	if err != nil {
		return false, err
	}
	return result.ToBoolean(), nil
}
