package minib

import (
	"testing"

	"github.com/dop251/goja"
)

func TestCompileJavascriptAllowsDestructuredParameterRedeclaration(t *testing.T) {
	source := `(function() {
		function join({ answer: value }) {
			var prefix = "answer=", value = (value === 42 && "yes") || "no";
			return prefix + value;
		}
		return join({ answer: 42 });
	})()
	//# sourceMappingURL=missing-source-map.js.map`

	program, err := compile_javascript("redeclared-parameter.js", source)
	if err != nil {
		t.Fatalf("compile_javascript() error = %v", err)
	}
	runtime := goja.New()
	result, err := runtime.RunProgram(program)
	if err != nil {
		t.Fatalf("RunProgram() error = %v", err)
	}
	if result.String() != "answer=yes" {
		t.Fatalf("result = %q, want %q", result.String(), "answer=yes")
	}
}

func TestCompileJavascriptAllowsDestructuredVarRedeclaration(t *testing.T) {
	source := `(function() {
		function join({ answer: value }) {
			var prefix = "answer=", [value, suffix] = [42, "!"];
			return prefix + value + suffix;
		}
		return join({ answer: 1 });
	})()`

	program, err := compile_javascript("redeclared-var-pattern.js", source)
	if err != nil {
		t.Fatalf("compile_javascript() error = %v", err)
	}
	runtime := goja.New()
	result, err := runtime.RunProgram(program)
	if err != nil {
		t.Fatalf("RunProgram() error = %v", err)
	}
	if result.String() != "answer=42!" {
		t.Fatalf("result = %q, want %q", result.String(), "answer=42!")
	}
}
