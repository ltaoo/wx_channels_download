package hermes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFinalOutputNameTruncatesLeafWithExtension(t *testing.T) {
	fp := NewFilenameProcessor("", nil)
	name := final_output_name(strings.Repeat("a", 300), ".mp4")
	leaf := filepath.Base(name)

	if !strings.HasSuffix(leaf, ".mp4") {
		t.Fatalf("expected .mp4 suffix, got %q", leaf)
	}
	if len(leaf) > fp.max_name_length {
		t.Fatalf("leaf length = %d, want <= %d", len(leaf), fp.max_name_length)
	}
	if got, want := len(strings.TrimSuffix(leaf, ".mp4")), fp.max_name_length-len(".mp4"); got != want {
		t.Fatalf("base length = %d, want %d", got, want)
	}
}

func TestFinalOutputNameTruncatesUTF8WithoutSplittingRune(t *testing.T) {
	fp := NewFilenameProcessor("", nil)
	name := final_output_name(strings.Repeat("片", 100), ".mp4")
	leaf := filepath.Base(name)

	if !utf8.ValidString(leaf) {
		t.Fatalf("expected valid UTF-8, got %q", leaf)
	}
	if !strings.HasSuffix(leaf, ".mp4") {
		t.Fatalf("expected .mp4 suffix, got %q", leaf)
	}
	if len(leaf) > fp.max_name_length {
		t.Fatalf("leaf length = %d, want <= %d", len(leaf), fp.max_name_length)
	}
}

func TestFinalOutputNameTruncatesDirectoryComponents(t *testing.T) {
	fp := NewFilenameProcessor("", nil)
	name := final_output_name(strings.Repeat("d", 300)+"/"+strings.Repeat("v", 300), ".jpg")
	parts := strings.Split(name, "/")

	if len(parts) != 2 {
		t.Fatalf("parts = %#v, want two path components", parts)
	}
	for _, part := range parts {
		if len(part) > fp.max_name_length {
			t.Fatalf("component %q length = %d, want <= %d", part, len(part), fp.max_name_length)
		}
	}
	if !strings.HasSuffix(parts[1], ".jpg") {
		t.Fatalf("expected final component to keep .jpg suffix, got %q", parts[1])
	}
}

func TestFinalOutputNameBudgetsDuplicateSuffix(t *testing.T) {
	fp := NewFilenameProcessor("", nil)
	suffix := "(12345)"
	name := final_output_name_with_suffix(strings.Repeat("a", 300), ".mp4", suffix)
	leaf := filepath.Base(name)

	if !strings.HasSuffix(leaf, suffix+".mp4") {
		t.Fatalf("expected duplicate suffix before extension, got %q", leaf)
	}
	if len(leaf) > fp.max_name_length {
		t.Fatalf("leaf length = %d, want <= %d", len(leaf), fp.max_name_length)
	}
	if got, want := len(strings.TrimSuffix(leaf, suffix+".mp4")), fp.max_name_length-len(suffix)-len(".mp4"); got != want {
		t.Fatalf("base length = %d, want %d", got, want)
	}
}

func TestResolveDuplicateFilenameBudgetsSuffixForLongName(t *testing.T) {
	fp := NewFilenameProcessor("", nil)
	base_path := t.TempDir()
	engine := New(HermesNewConfig{
		Config: HermesEngineConfig{BasePath: base_path},
	})

	base_name := strings.Repeat("a", 300)
	first_name := final_output_name(base_name, ".mp4")
	if err := os.WriteFile(filepath.Join(base_path, first_name), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	duplicate_name := engine.resolve_duplicate_filename("", base_name, ".mp4")
	leaf := filepath.Base(duplicate_name)
	if duplicate_name == first_name {
		t.Fatalf("expected duplicate filename, got original %q", duplicate_name)
	}
	if !strings.HasSuffix(leaf, "(1).mp4") {
		t.Fatalf("expected duplicate suffix before extension, got %q", leaf)
	}
	if len(leaf) > fp.max_name_length {
		t.Fatalf("leaf length = %d, want <= %d", len(leaf), fp.max_name_length)
	}
}

func TestBuildFinalResourceNameAppliesTemplateAndHook(t *testing.T) {
	hook_path := filepath.Join(t.TempDir(), "hooks.js")
	if err := os.WriteFile(hook_path, []byte(`
function onFilename(systemName, meta, task, config) {
  return systemName + "_" + meta.author + "_" + config.suffix;
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	hooks := NewHookManager()
	if err := hooks.Load(hook_path); err != nil {
		t.Fatal(err)
	}

	result := BuildFinalResourceName(FinalResourceNameInput{
		TaskConfig:       map[string]any{"suffix": "hook"},
		FilenameTemplate: "{{author}}/{{filename}}",
		ResourceName:     "Title",
		ResourceKind:     "video/mp4",
		ResourceExtra:    map[string]string{"author": "Alice"},
		Hooks:            hooks,
	})

	if result.TemplateError != nil {
		t.Fatalf("template error: %v", result.TemplateError)
	}
	if result.HookError != nil {
		t.Fatalf("hook error: %v", result.HookError)
	}
	if got, want := result.Name, "Alice/Title_Alice_hook.mp4"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
}
