package assetsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushPullWithLocalStorage(t *testing.T) {
	repoRoot := t.TempDir()
	storeRoot := t.TempDir()
	cfg := testConfig(storeRoot)

	examplePath := filepath.Join(repoRoot, "scraper_examples", "youtube", "260614", "youtube.html")
	if err := os.MkdirAll(filepath.Dir(examplePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(examplePath, []byte("<html>youtube</html>"), 0644); err != nil {
		t.Fatal(err)
	}

	pushResult, err := Push(context.Background(), repoRoot, cfg, PushOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if pushResult.Uploaded != 1 || pushResult.LockChanged != true {
		t.Fatalf("unexpected push result: %+v", pushResult)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".asset-sync.lock.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(storeRoot, "repo-assets", "scraper_examples", "youtube", "260614", "youtube.html")); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(examplePath); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(repoRoot, cfg, false); err == nil {
		t.Fatal("expected verify to fail while the asset is missing")
	}

	pullResult, err := Pull(context.Background(), repoRoot, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if pullResult.Downloaded != 1 || pullResult.Verified != 1 {
		t.Fatalf("unexpected pull result: %+v", pullResult)
	}
	data, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<html>youtube</html>" {
		t.Fatalf("unexpected restored data: %q", string(data))
	}
	if _, err := Verify(repoRoot, cfg, false); err != nil {
		t.Fatal(err)
	}
}

func TestSyncUploadsLocalChangesAndDownloadsMissingLockedFiles(t *testing.T) {
	repoRoot := t.TempDir()
	storeRoot := t.TempDir()
	cfg := testConfig(storeRoot)

	first := filepath.Join(repoRoot, "scraper_examples", "first.html")
	if err := os.MkdirAll(filepath.Dir(first), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(context.Background(), repoRoot, cfg, PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(repoRoot, "scraper_examples", "second.html")
	if err := os.WriteFile(second, []byte("second"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Sync(context.Background(), repoRoot, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Uploaded != 1 || result.Downloaded != 1 || !result.LockChanged {
		t.Fatalf("unexpected sync result: %+v", result)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(cfg.ManifestPath(repoRoot))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 2 {
		t.Fatalf("sync should preserve missing locked files and add local files, got %d", len(manifest.Files))
	}
	if _, err := Verify(repoRoot, cfg, false); err != nil {
		t.Fatal(err)
	}
}

func TestSyncOpaqueRootDoesNotWriteFileNamesToManifest(t *testing.T) {
	repoRoot := t.TempDir()
	storeRoot := t.TempDir()
	cfg := testConfig(storeRoot)
	cfg.Roots[0].TrackFiles = boolPtr(false)

	localFile := filepath.Join(repoRoot, "scraper_examples", "private", "example.html")
	if err := os.MkdirAll(filepath.Dir(localFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localFile, []byte("private"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "scraper_examples", ".DS_Store"), []byte("skip"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Sync(context.Background(), repoRoot, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Uploaded != 1 || result.Downloaded != 1 || !result.LockChanged {
		t.Fatalf("unexpected opaque sync result: %+v", result)
	}
	manifest, err := LoadManifest(cfg.ManifestPath(repoRoot))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 0 {
		t.Fatalf("opaque root should not write file records: %+v", manifest.Files)
	}
	if len(manifest.OpaqueRoots) != 1 || manifest.OpaqueRoots[0].Path != "scraper_examples" {
		t.Fatalf("unexpected opaque roots: %+v", manifest.OpaqueRoots)
	}
	if _, err := os.Stat(filepath.Join(storeRoot, "repo-assets", "scraper_examples", "private", "example.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(storeRoot, "repo-assets", "scraper_examples", ".DS_Store")); !os.IsNotExist(err) {
		t.Fatalf("excluded file should not be copied, err=%v", err)
	}

	if err := os.Remove(localFile); err != nil {
		t.Fatal(err)
	}
	remoteOnly := filepath.Join(storeRoot, "repo-assets", "scraper_examples", "remote.html")
	if err := os.WriteFile(remoteOnly, []byte("remote"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err = Sync(context.Background(), repoRoot, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Downloaded != 1 {
		t.Fatalf("expected opaque root download operation, got %+v", result)
	}
	if _, err := os.Stat(localFile); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "scraper_examples", "remote.html")); err != nil {
		t.Fatal(err)
	}

	status, err := BuildStatus(repoRoot, cfg, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Added) != 0 || len(status.Modified) != 0 || len(status.Missing) != 0 || len(status.OpaqueRoots) != 1 {
		t.Fatalf("unexpected opaque status: %+v", status)
	}
}

func TestStatusDetectsAddedModifiedMissing(t *testing.T) {
	repoRoot := t.TempDir()
	storeRoot := t.TempDir()
	cfg := testConfig(storeRoot)

	tracked := filepath.Join(repoRoot, "scraper_examples", "qidian.html")
	if err := os.MkdirAll(filepath.Dir(tracked), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracked, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(context.Background(), repoRoot, cfg, PushOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(tracked, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	added := filepath.Join(repoRoot, "scraper_examples", "new.html")
	if err := os.WriteFile(added, []byte("added"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(cfg.ManifestPath(repoRoot))
	if err != nil {
		t.Fatal(err)
	}
	status, err := BuildStatus(repoRoot, cfg, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Modified) != 1 || len(status.Added) != 1 || len(status.Missing) != 0 {
		t.Fatalf("unexpected status before delete: %+v", status)
	}

	if err := os.Remove(tracked); err != nil {
		t.Fatal(err)
	}
	status, err = BuildStatus(repoRoot, cfg, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Missing) != 1 || len(status.Added) != 1 || len(status.Modified) != 0 {
		t.Fatalf("unexpected status after delete: %+v", status)
	}
}

func TestScanRespectsExcludePatterns(t *testing.T) {
	repoRoot := t.TempDir()
	storeRoot := t.TempDir()
	cfg := testConfig(storeRoot)

	files := map[string]string{
		"scraper_examples/keep.html":             "keep",
		"scraper_examples/.DS_Store":             "skip",
		"scraper_examples/nested/.DS_Store":      "skip",
		"scraper_examples/nested/cache.tmp":      "skip",
		"scraper_examples/nested/not_tmp.html":   "keep",
		"scraper_examples/nested/tmp.in.name.md": "keep",
	}
	for name, content := range files {
		path := filepath.Join(repoRoot, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	records, err := Scan(repoRoot, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, record := range records {
		paths = append(paths, record.Path)
	}
	got := strings.Join(paths, ",")
	want := "keep.html,nested/not_tmp.html,nested/tmp.in.name.md"
	if got != want {
		t.Fatalf("unexpected scan paths: got %q want %q", got, want)
	}
}

func TestInstallHooksRefusesUnmanagedHookWithoutForce(t *testing.T) {
	repoRoot := t.TempDir()
	hooksDir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	prePush := filepath.Join(hooksDir, "pre-push")
	if err := os.WriteFile(prePush, []byte("#!/bin/sh\necho existing\n"), 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallHooks(repoRoot, InstallHooksOptions{Command: "asset-sync"}); err == nil {
		t.Fatal("expected install to refuse an unmanaged hook")
	}
	if _, err := InstallHooks(repoRoot, InstallHooksOptions{Command: "asset-sync", Force: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(prePush)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, hookStartMarker) || !strings.Contains(content, "asset-sync") || !strings.Contains(content, "echo existing") {
		t.Fatalf("unexpected hook content:\n%s", content)
	}
}

func testConfig(storeRoot string) Config {
	cfg := Config{
		Version:  1,
		Manifest: ".asset-sync.lock.json",
		Roots: []RootConfig{
			{
				Path:    "scraper_examples",
				Include: []string{"**/*"},
				Exclude: []string{"**/.DS_Store", "**/*.tmp"},
			},
		},
		Storage: StorageConfig{
			Type:      "local",
			LocalPath: storeRoot,
			Prefix:    "repo-assets",
		},
	}
	if err := cfg.Normalize(""); err != nil {
		panic(err)
	}
	return cfg
}

func boolPtr(v bool) *bool {
	return &v
}
