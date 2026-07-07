package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"wx_channel/internal/assetsync"
)

var (
	repoFlag   string
	configFlag string
	deviceFlag string
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "asset-sync",
		Short:         "Sync large repository assets outside git",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&repoFlag, "repo", "", "git repository path")
	root.PersistentFlags().StringVar(&configFlag, "config", "", "config file path")
	root.PersistentFlags().StringVar(&deviceFlag, "device", "", "device name from .asset-sync.yaml")
	root.AddCommand(
		newInitCommand(),
		newHostnameCommand(),
		newSyncCommand(),
		newStatusCommand(),
		newPushCommand(),
		newPullCommand(),
		newVerifyCommand(),
		newInstallHooksCommand(),
		newHookCommand(),
	)
	return root
}

func newSyncCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Upload local changes and download missing locked assets",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			result, err := assetsync.Sync(cmd.Context(), repoRoot, cfg)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "uploaded %d, downloaded %d, skipped %d, verified %d\n", result.Uploaded, result.Downloaded, result.Skipped, result.Verified)
			if result.LockChanged {
				fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", result.ManifestPath)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "lock unchanged\n")
			}
			return nil
		},
	}
	return cmd
}

func newInitCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a default .asset-sync.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveRepoRoot()
			if err != nil {
				return err
			}
			configPath := configFlag
			if configPath == "" {
				configPath = filepath.Join(repoRoot, assetsync.DefaultConfigPath)
			} else if !filepath.IsAbs(configPath) {
				configPath = filepath.Join(repoRoot, configPath)
			}
			if err := assetsync.WriteDefaultConfig(configPath, force); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", configPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config")
	return cmd
}

func newHostnameCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hostname",
		Short: "Print the current hostname used by device auto matching",
		RunE: func(cmd *cobra.Command, args []string) error {
			hostname, err := os.Hostname()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), hostname)
			return nil
		},
	}
	return cmd
}

func newStatusCommand() *cobra.Command {
	var porcelain bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show local asset changes against the lock file",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			manifest, ok, err := assetsync.LoadManifestIfExists(cfg.ManifestPath(repoRoot))
			if err != nil {
				return err
			}
			report, err := assetsync.BuildStatus(repoRoot, cfg, manifest)
			if err != nil {
				return err
			}
			printStatus(cmd.OutOrStdout(), report, porcelain)
			if !ok && !porcelain {
				fmt.Fprintf(cmd.OutOrStdout(), "lock file not found: %s\n", cfg.Manifest)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&porcelain, "porcelain", false, "print machine-readable status")
	return cmd
}

func newPushCommand() *cobra.Command {
	var all bool
	var allowEmpty bool
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Upload local assets and update the lock file",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			result, err := assetsync.Push(cmd.Context(), repoRoot, cfg, assetsync.PushOptions{
				All:        all,
				AllowEmpty: allowEmpty,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "uploaded %d, skipped %d\n", result.Uploaded, result.Skipped)
			if result.LockChanged {
				fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", result.ManifestPath)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "lock unchanged\n")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "upload every file, including unchanged files")
	cmd.Flags().BoolVar(&allowEmpty, "allow-empty", false, "allow writing an empty lock when no local assets are found")
	return cmd
}

func newPullCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Download missing or changed assets from storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			result, err := assetsync.Pull(cmd.Context(), repoRoot, cfg)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "downloaded %d, skipped %d, verified %d\n", result.Downloaded, result.Skipped, result.Verified)
			return nil
		},
	}
	return cmd
}

func newVerifyCommand() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify local assets against the lock file",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			result, err := assetsync.Verify(repoRoot, cfg, strict)
			if err != nil {
				if result.Status.HasChanges() || result.Status.Unchanged > 0 {
					printStatus(cmd.OutOrStdout(), result.Status, false)
				}
				return err
			}
			printStatus(cmd.OutOrStdout(), result.Status, false)
			fmt.Fprintln(cmd.OutOrStdout(), "verified")
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "fail on untracked local assets too")
	return cmd
}

func newInstallHooksCommand() *cobra.Command {
	var command string
	var force bool
	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install git hooks that call asset-sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := resolveRepoRoot()
			if err != nil {
				return err
			}
			installed, err := assetsync.InstallHooks(repoRoot, assetsync.InstallHooksOptions{
				Command: command,
				Force:   force,
			})
			if err != nil {
				return err
			}
			for _, path := range installed {
				fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&command, "command", "asset-sync", "command used by git hooks")
	cmd.Flags().BoolVar(&force, "force", false, "prepend asset-sync to existing unmanaged hooks")
	return cmd
}

func newHookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hook <event>",
		Short:  "Run an asset-sync git hook handler",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			return assetsync.RunHook(cmd.Context(), repoRoot, cfg, args[0], cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	return cmd
}

func loadConfig() (string, assetsync.Config, string, error) {
	repoRoot, err := resolveRepoRoot()
	if err != nil {
		return "", assetsync.Config{}, "", err
	}
	if err := loadDefaultEnv(repoRoot); err != nil {
		return "", assetsync.Config{}, "", err
	}
	cfg, configPath, err := assetsync.LoadConfigForDevice(repoRoot, configFlag, deviceFlag)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", assetsync.Config{}, "", fmt.Errorf("asset-sync config not found; run asset-sync init")
		}
		return "", assetsync.Config{}, "", err
	}
	return repoRoot, cfg, configPath, nil
}

func resolveRepoRoot() (string, error) {
	start := repoFlag
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	return assetsync.FindRepoRoot(start)
}

func printStatus(w io.Writer, report assetsync.StatusReport, porcelain bool) {
	if porcelain {
		for _, root := range report.OpaqueRoots {
			fmt.Fprintf(w, "O %s\n", root.Path)
		}
		for _, change := range report.Added {
			fmt.Fprintf(w, "A %s/%s\n", change.Root, change.Path)
		}
		for _, change := range report.Modified {
			fmt.Fprintf(w, "M %s/%s\n", change.Root, change.Path)
		}
		for _, change := range report.Missing {
			fmt.Fprintf(w, "! %s/%s\n", change.Root, change.Path)
		}
		return
	}
	if !report.HasChanges() {
		for _, root := range report.OpaqueRoots {
			fmt.Fprintf(w, "opaque   %s (file names not tracked)\n", root.Path)
		}
		fmt.Fprintf(w, "clean (%d files)\n", report.Unchanged)
		return
	}
	for _, root := range report.OpaqueRoots {
		fmt.Fprintf(w, "opaque   %s (file names not tracked)\n", root.Path)
	}
	for _, change := range report.Added {
		fmt.Fprintf(w, "added    %s/%s\n", change.Root, change.Path)
	}
	for _, change := range report.Modified {
		fmt.Fprintf(w, "modified %s/%s\n", change.Root, change.Path)
	}
	for _, change := range report.Missing {
		fmt.Fprintf(w, "missing  %s/%s\n", change.Root, change.Path)
	}
	fmt.Fprintf(w, "unchanged %d\n", report.Unchanged)
}

func init() {
	cobra.EnableCommandSorting = false
}
