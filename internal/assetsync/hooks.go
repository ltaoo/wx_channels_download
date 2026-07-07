package assetsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	hookStartMarker = "# asset-sync hook start"
	hookEndMarker   = "# asset-sync hook end"
)

type InstallHooksOptions struct {
	Command string
	Force   bool
}

func InstallHooks(repoRoot string, opts InstallHooksOptions) ([]string, error) {
	if opts.Command == "" {
		opts.Command = "asset-sync"
	}
	gitDir, err := GitDir(repoRoot)
	if err != nil {
		return nil, err
	}
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return nil, err
	}
	events := []string{"post-merge", "post-rewrite", "post-checkout", "pre-push"}
	var installed []string
	for _, event := range events {
		path := filepath.Join(hooksDir, event)
		if err := installHook(path, event, opts); err != nil {
			return installed, err
		}
		installed = append(installed, path)
	}
	return installed, nil
}

func installHook(path, event string, opts InstallHooksOptions) error {
	block := hookBlock(event, opts.Command)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		return os.WriteFile(path, []byte("#!/bin/sh\n\n"+block), 0755)
	}
	content := string(data)
	if strings.Contains(content, hookStartMarker) && strings.Contains(content, hookEndMarker) {
		content = replaceHookBlock(content, block)
		return os.WriteFile(path, []byte(content), 0755)
	}
	if !opts.Force {
		return fmt.Errorf("%s already exists and is not managed by asset-sync; rerun with --force to prepend asset-sync", path)
	}
	if strings.HasPrefix(content, "#!") {
		if idx := strings.Index(content, "\n"); idx >= 0 {
			content = content[:idx+1] + "\n" + block + "\n" + content[idx+1:]
		} else {
			content = content + "\n\n" + block
		}
	} else {
		content = "#!/bin/sh\n\n" + block + "\n" + content
	}
	return os.WriteFile(path, []byte(content), 0755)
}

func hookBlock(event, command string) string {
	return fmt.Sprintf(`%s
ASSET_SYNC_CMD=${ASSET_SYNC_CMD:-%q}
# shellcheck disable=SC2086
$ASSET_SYNC_CMD hook %s "$@"
status=$?
if [ $status -ne 0 ]; then
  exit $status
fi
%s
`, hookStartMarker, command, event, hookEndMarker)
}

func replaceHookBlock(content, block string) string {
	start := strings.Index(content, hookStartMarker)
	end := strings.Index(content, hookEndMarker)
	if start < 0 || end < 0 || end < start {
		return content
	}
	end += len(hookEndMarker)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:start] + block + content[end:]
}

func RunHook(ctx context.Context, repoRoot string, cfg Config, event string, out, errOut io.Writer) error {
	switch event {
	case "pre-push":
		return runPrePushHook(ctx, repoRoot, cfg, out, errOut)
	case "post-merge":
		return runPullHook(ctx, repoRoot, cfg, cfg.Hooks.PostMerge, out, errOut)
	case "post-rewrite":
		return runPullHook(ctx, repoRoot, cfg, cfg.Hooks.PostRewrite, out, errOut)
	case "post-checkout":
		return runPullHook(ctx, repoRoot, cfg, cfg.Hooks.PostCheckout, out, errOut)
	default:
		return fmt.Errorf("unsupported hook event %q", event)
	}
}

func runPrePushHook(ctx context.Context, repoRoot string, cfg Config, out, errOut io.Writer) error {
	mode := cfg.Hooks.PrePush
	switch mode {
	case "", "off", "none":
		return nil
	case "auto":
		result, err := Push(ctx, repoRoot, cfg, PushOptions{})
		if err != nil {
			fmt.Fprintf(errOut, "asset-sync pre-push failed: %v\n", err)
			return err
		}
		fmt.Fprintf(out, "asset-sync: uploaded %d, skipped %d\n", result.Uploaded, result.Skipped)
		if result.LockChanged {
			err := fmt.Errorf("%s changed; commit it before git push", cfg.Manifest)
			fmt.Fprintf(errOut, "asset-sync pre-push blocked: %v\n", err)
			return err
		}
		return nil
	case "check", "strict":
		status, err := CheckPrePush(repoRoot, cfg)
		if err != nil {
			fmt.Fprintf(errOut, "asset-sync pre-push blocked: %v\n", err)
			writeStatusSummary(errOut, status)
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported hooks.pre_push mode %q", mode)
	}
}

func runPullHook(ctx context.Context, repoRoot string, cfg Config, mode string, out, errOut io.Writer) error {
	switch mode {
	case "", "off", "none":
		return nil
	case "pull", "auto":
		result, skipped, err := PullIfManifestChanged(ctx, repoRoot, cfg)
		if err != nil {
			fmt.Fprintf(errOut, "asset-sync hook warning: %v\n", err)
			return nil
		}
		if skipped {
			return nil
		}
		fmt.Fprintf(out, "asset-sync: downloaded %d, skipped %d, verified %d\n", result.Downloaded, result.Skipped, result.Verified)
		return nil
	default:
		return fmt.Errorf("unsupported hook pull mode %q", mode)
	}
}

type hookState struct {
	ManifestDigest string `json:"manifest_digest"`
}

func PullIfManifestChanged(ctx context.Context, repoRoot string, cfg Config) (PullResult, bool, error) {
	manifestPath := cfg.ManifestPath(repoRoot)
	digest, err := ManifestDigest(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return PullResult{}, true, nil
		}
		return PullResult{}, false, err
	}
	statePath, err := hookStatePath(repoRoot)
	if err != nil {
		return PullResult{}, false, err
	}
	state := readHookState(statePath)
	if state.ManifestDigest == digest {
		return PullResult{}, true, nil
	}
	result, err := Pull(ctx, repoRoot, cfg)
	if err != nil {
		return result, false, err
	}
	state.ManifestDigest = digest
	if err := writeHookState(statePath, state); err != nil {
		return result, false, err
	}
	return result, false, nil
}

func hookStatePath(repoRoot string) (string, error) {
	gitDir, err := GitDir(repoRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(gitDir, "asset-sync-state.json"), nil
}

func readHookState(path string) hookState {
	data, err := os.ReadFile(path)
	if err != nil {
		return hookState{}
	}
	var state hookState
	if err := json.Unmarshal(data, &state); err != nil {
		return hookState{}
	}
	return state
}

func writeHookState(path string, state hookState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func writeStatusSummary(w io.Writer, status StatusReport) {
	for _, change := range status.Added {
		fmt.Fprintf(w, "  added    %s/%s\n", change.Root, change.Path)
	}
	for _, change := range status.Modified {
		fmt.Fprintf(w, "  modified %s/%s\n", change.Root, change.Path)
	}
}
