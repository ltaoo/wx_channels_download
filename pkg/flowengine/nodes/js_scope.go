package nodes

import "github.com/dop251/goja"

// node_scope_names is the ordered set of variable scopes exposed to JavaScript
// nodes, matching ScopeEnv and the template namespaces.
var node_scope_names = [3]string{"input", "output", "global"}

// new_scope_vm builds a goja runtime with the three scopes bound as global
// objects. JSCodeNode and JS gateway conditions both go through here so the two
// can never drift apart; a scope added for one is automatically available to
// the other.
func new_scope_vm(scope_env map[string]any) (*goja.Runtime, error) {
	vm := goja.New()
	for _, scope := range node_scope_names {
		if err := vm.Set(scope, scope_env[scope]); err != nil {
			return nil, err
		}
	}
	return vm, nil
}
