package configapi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindConfigFileUsesExplicitPathBeforeEnvironment(t *testing.T) {
	directory := t.TempDir()
	explicit_path := filepath.Join(directory, "explicit.yaml")
	environment_path := filepath.Join(directory, "environment.yaml")
	if err := os.WriteFile(explicit_path, []byte("explicit: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIGAPI_TEST_FILE", environment_path)

	location, err := FindConfigFile(FindConfigFileOptions{
		ExplicitPath:        explicit_path,
		EnvironmentVariable: "CONFIGAPI_TEST_FILE",
	})
	if err != nil {
		t.Fatal(err)
	}
	if location.Path != explicit_path || !location.Exists || !location.Explicit {
		t.Fatalf("explicit location = %+v", location)
	}
}

func TestFindConfigFileUsesEnvironmentAsExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	t.Setenv("CONFIGAPI_TEST_FILE", path)

	location, err := FindConfigFile(FindConfigFileOptions{
		EnvironmentVariable: "CONFIGAPI_TEST_FILE",
	})
	if err != nil {
		t.Fatal(err)
	}
	if location.Path != path || location.Exists || !location.Explicit {
		t.Fatalf("environment location = %+v", location)
	}
}

func TestFindConfigFileSearchesDirectoriesInOrder(t *testing.T) {
	first_directory := t.TempDir()
	second_directory := t.TempDir()
	path := filepath.Join(second_directory, "settings.yaml")
	if err := os.WriteFile(path, []byte("found: true\n"), 0600); err != nil {
		t.Fatal(err)
	}

	location, err := FindConfigFile(FindConfigFileOptions{
		Filename:          "settings.yaml",
		SearchDirectories: []string{first_directory, second_directory},
		FallbackDirectory: first_directory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if location.Path != path || !location.Exists || location.Explicit {
		t.Fatalf("searched location = %+v", location)
	}
}

func TestFindConfigFileReturnsStableFallback(t *testing.T) {
	fallback_directory := t.TempDir()
	location, err := FindConfigFile(FindConfigFileOptions{
		Filename:          "settings.yaml",
		SearchDirectories: []string{t.TempDir()},
		FallbackDirectory: fallback_directory,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(fallback_directory, "settings.yaml")
	if location.Path != want || location.Exists || location.Explicit {
		t.Fatalf("fallback location = %+v, want %s", location, want)
	}
}
