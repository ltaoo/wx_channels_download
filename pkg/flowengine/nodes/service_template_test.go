package nodes

import (
	"reflect"
	"testing"

	"wx_channel/pkg/flowengine/engine"
)

func new_template_context() *engine.ProcessContext {
	return &engine.ProcessContext{
		Data:    map[string]interface{}{"bare": "from-data"},
		Inputs:  map[string]interface{}{"user": "alice", "count": 3},
		Outputs: map[string]interface{}{"platform": "youtube"},
		Globals: map[string]interface{}{"api_key": "secret-123"},
	}
}

func TestResolveServiceTemplateOutputNamespace(t *testing.T) {
	ctx := new_template_context()
	got, err := resolve_service_template("{{output.platform}}", ctx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "youtube" {
		t.Fatalf("unexpected value: %#v", got)
	}
}

func TestResolveServiceTemplateGlobalNamespace(t *testing.T) {
	ctx := new_template_context()
	got, err := resolve_service_template("{{global.api_key}}", ctx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "secret-123" {
		t.Fatalf("unexpected value: %#v", got)
	}
}

func TestResolveServiceTemplateInputNamespaceWholeValuePreservesType(t *testing.T) {
	ctx := new_template_context()
	got, err := resolve_service_template("{{input.count}}", ctx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !reflect.DeepEqual(got, 3) {
		t.Fatalf("unexpected value: %#v", got)
	}
}

func TestResolveServiceTemplateInlineMixedNamespaces(t *testing.T) {
	ctx := new_template_context()
	got, err := resolve_service_template("{{input.user}} via {{output.platform}}", ctx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "alice via youtube" {
		t.Fatalf("unexpected value: %#v", got)
	}
}

func TestResolveServiceTemplateAllowsSpacesAroundNamespace(t *testing.T) {
	ctx := new_template_context()
	got, err := resolve_service_template("{{ input.user }}", ctx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "alice" {
		t.Fatalf("unexpected value: %#v", got)
	}
}

func TestResolveServiceTemplateMissingNamespaceKey(t *testing.T) {
	ctx := new_template_context()
	for _, tmpl := range []string{
		"{{input.missing}}",
		"{{output.missing}}",
		"{{global.missing}}",
	} {
		if _, err := resolve_service_template(tmpl, ctx); err == nil {
			t.Fatalf("expected missing key to fail for %s", tmpl)
		}
	}
}

func TestResolveServiceTemplateUnknownNamespace(t *testing.T) {
	ctx := new_template_context()
	if _, err := resolve_service_template("{{bogus.x}}", ctx); err == nil {
		t.Fatal("expected unknown namespace to fail")
	}
}

func TestResolveServiceTemplateBareKeyStillResolvesFromData(t *testing.T) {
	ctx := new_template_context()
	got, err := resolve_service_template("{{bare}}", ctx)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "from-data" {
		t.Fatalf("unexpected value: %#v", got)
	}
}
