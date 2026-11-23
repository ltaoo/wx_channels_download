package hermes

import "testing"

func TestBuildFinalResourceName_preserves_existing_extension(t *testing.T) {
	for _, test_case := range []struct {
		name          string
		resource_type string
		config        map[string]any
		template      string
	}{
		{name: "URL resource", config: map[string]any{"url": "https://example.com/a.mp4"}},
		{name: "STREAM resource", resource_type: ResourceTypeStream},
		{name: "URL template variable", config: map[string]any{"url": "https://example.com/a.mp4"}, template: "{{filename}}"},
		{name: "URL template expression", config: map[string]any{"url": "https://example.com/a.mp4"}, template: "name"},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			result := BuildFinalResourceName(FinalResourceNameInput{
				TaskName:         "a.mp4",
				TaskConfig:       test_case.config,
				FilenameTemplate: test_case.template,
				ResourceName:     "a.mp4",
				ResourceKind:     "video/mp4",
				ResourceType:     test_case.resource_type,
			})
			if result.TemplateError != nil {
				t.Fatal(result.TemplateError)
			}
			if result.BaseName != "a.mp4" || result.Name != "a.mp4.mp4" {
				t.Fatalf("got base name %q, name %q; want a.mp4, a.mp4.mp4", result.BaseName, result.Name)
			}
		})
	}
}
