package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

var staged_directories = []string{
	"cmd",
	"frontend",
	"internal",
	"pkg",
}

var js_asset_paths = []string{
	"frontend/inject",
	"frontend/public",
	"frontend/src",
	"pkg/scraper/wxchannels/inject",
	"pkg/scraper/wxmp/inject",
	"pkg/scraper/zhihu/inject",
	"pkg/scraper/zhihu/pcweb_runtime.js",
	"pkg/scraper/youtube/jsc",
}

type minify_stats struct {
	file_count   int
	original_len int64
	minified_len int64
}

func main() {
	root_flag := flag.String("root", ".", "project root containing go.mod")
	minify_only_flag := flag.Bool("minify-only", false, "minify embedded JavaScript in the project tree without building")
	flag.Parse()

	var err error
	if *minify_only_flag {
		err = run_minify_only(*root_flag)
	} else {
		err = run_build(*root_flag, flag.Args())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildgo: %v\n", err)
		os.Exit(1)
	}
}

func run_minify_only(root string) error {
	root_path, err := resolve_project_root(root)
	if err != nil {
		return err
	}
	stats, err := minify_javascript(root_path)
	if err != nil {
		return err
	}
	print_minify_stats(stats)
	return nil
}

func run_build(root string, build_args []string) error {
	root_path, err := resolve_project_root(root)
	if err != nil {
		return err
	}

	normalized_args, err := normalize_output_args(root_path, build_args)
	if err != nil {
		return err
	}

	stage_parent, err := os.MkdirTemp("", "wx-video-go-build-")
	if err != nil {
		return fmt.Errorf("create build staging directory: %w", err)
	}
	defer os.RemoveAll(stage_parent)

	stage_root := filepath.Join(stage_parent, "project")
	if err := os.Mkdir(stage_root, 0o755); err != nil {
		return fmt.Errorf("create staged project: %w", err)
	}
	if err := stage_project(root_path, stage_root); err != nil {
		return err
	}

	stats, err := minify_javascript(stage_root)
	if err != nil {
		return err
	}
	print_minify_stats(stats)

	command := exec.Command("go", append([]string{"build"}, normalized_args...)...)
	command.Dir = stage_root
	command.Env = os.Environ()
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}
	return nil
}

func resolve_project_root(root string) (string, error) {
	root_path, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	if _, err := os.Stat(filepath.Join(root_path, "go.mod")); err != nil {
		return "", fmt.Errorf("project root does not contain go.mod: %s", root_path)
	}
	return root_path, nil
}

func print_minify_stats(stats minify_stats) {
	saved_len := stats.original_len - stats.minified_len
	percent := float64(0)
	if stats.original_len > 0 {
		percent = float64(saved_len) * 100 / float64(stats.original_len)
	}
	fmt.Printf(
		"Minified embedded JavaScript: %s -> %s, saved %s (%.1f%%) across %d files\n",
		format_bytes(stats.original_len),
		format_bytes(stats.minified_len),
		format_bytes(saved_len),
		percent,
		stats.file_count,
	)
}

func normalize_output_args(root_path string, build_args []string) ([]string, error) {
	normalized_args := append([]string(nil), build_args...)
	has_output := false
	for i := 0; i < len(normalized_args); i++ {
		if normalized_args[i] == "-o" {
			if i+1 >= len(normalized_args) {
				return nil, errors.New("go build -o requires an output path")
			}
			has_output = true
			normalized_args[i+1] = absolute_output_path(root_path, normalized_args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(normalized_args[i], "-o=") {
			output_path := strings.TrimPrefix(normalized_args[i], "-o=")
			if output_path == "" {
				return nil, errors.New("go build -o requires an output path")
			}
			has_output = true
			normalized_args[i] = "-o=" + absolute_output_path(root_path, output_path)
		}
	}
	if !has_output {
		return nil, errors.New("compressed build requires an explicit go build -o output path")
	}
	return normalized_args, nil
}

func absolute_output_path(root_path string, output_path string) string {
	if filepath.IsAbs(output_path) {
		return filepath.Clean(output_path)
	}
	return filepath.Join(root_path, output_path)
}

func stage_project(source_root string, stage_root string) error {
	root_entries, err := os.ReadDir(source_root)
	if err != nil {
		return fmt.Errorf("read project root: %w", err)
	}
	for _, entry := range root_entries {
		if entry.IsDir() || !is_root_build_file(entry.Name()) {
			continue
		}
		if err := copy_path(
			filepath.Join(source_root, entry.Name()),
			filepath.Join(stage_root, entry.Name()),
		); err != nil {
			return err
		}
	}

	for _, directory := range staged_directories {
		source_path := filepath.Join(source_root, directory)
		if _, err := os.Stat(source_path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("inspect %s: %w", source_path, err)
		}
		if err := copy_path(source_path, filepath.Join(stage_root, directory)); err != nil {
			return err
		}
	}

	vendor_path := filepath.Join(source_root, "vendor")
	if _, err := os.Stat(vendor_path); err == nil {
		if err := copy_path(vendor_path, filepath.Join(stage_root, "vendor")); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect vendor directory: %w", err)
	}
	return nil
}

func is_root_build_file(name string) bool {
	if name == "go.mod" || name == "go.sum" {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go", ".c", ".cc", ".cpp", ".h", ".hh", ".hpp", ".m", ".mm", ".s", ".syso":
		return true
	default:
		return false
	}
}

func copy_path(source_path string, destination_path string) error {
	return filepath.WalkDir(source_path, func(current_path string, entry fs.DirEntry, walk_err error) error {
		if walk_err != nil {
			return walk_err
		}
		if entry.IsDir() && current_path != source_path && should_skip_directory(entry.Name()) {
			return filepath.SkipDir
		}

		relative_path, err := filepath.Rel(source_path, current_path)
		if err != nil {
			return err
		}
		target_path := destination_path
		if relative_path != "." {
			target_path = filepath.Join(destination_path, relative_path)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := os.MkdirAll(target_path, info.Mode().Perm()); err != nil {
				return fmt.Errorf("create directory %s: %w", target_path, err)
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link_target, err := os.Readlink(current_path)
			if err != nil {
				return fmt.Errorf("read symlink %s: %w", current_path, err)
			}
			if err := os.Symlink(link_target, target_path); err != nil {
				return fmt.Errorf("copy symlink %s: %w", current_path, err)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := copy_file(current_path, target_path, info.Mode().Perm()); err != nil {
			return fmt.Errorf("copy %s: %w", current_path, err)
		}
		return nil
	})
}

func should_skip_directory(name string) bool {
	return name == ".git" || name == "node_modules" || name == "dist"
}

func copy_file(source_path string, destination_path string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination_path), 0o755); err != nil {
		return err
	}
	source_file, err := os.Open(source_path)
	if err != nil {
		return err
	}
	defer source_file.Close()

	destination_file, err := os.OpenFile(destination_path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(destination_file, source_file); err != nil {
		destination_file.Close()
		return err
	}
	return destination_file.Close()
}

func minify_javascript(stage_root string) (minify_stats, error) {
	stats := minify_stats{}
	for _, relative_path := range js_asset_paths {
		asset_path := filepath.Join(stage_root, filepath.FromSlash(relative_path))
		info, err := os.Stat(asset_path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return stats, fmt.Errorf("inspect JavaScript asset %s: %w", relative_path, err)
		}
		if info.IsDir() {
			err = filepath.WalkDir(asset_path, func(file_path string, entry fs.DirEntry, walk_err error) error {
				if walk_err != nil {
					return walk_err
				}
				if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".js") {
					return nil
				}
				return minify_javascript_file(stage_root, file_path, &stats)
			})
			if err != nil {
				return stats, err
			}
			continue
		}
		if strings.EqualFold(filepath.Ext(asset_path), ".js") {
			if err := minify_javascript_file(stage_root, asset_path, &stats); err != nil {
				return stats, err
			}
		}
	}
	return stats, nil
}

func minify_javascript_file(stage_root string, file_path string, stats *minify_stats) error {
	source, err := os.ReadFile(file_path)
	if err != nil {
		return fmt.Errorf("read %s: %w", file_path, err)
	}

	relative_path, relative_err := filepath.Rel(stage_root, file_path)
	if relative_err != nil {
		relative_path = file_path
	}
	result := api.Transform(string(source), api.TransformOptions{
		Sourcefile:        filepath.ToSlash(relative_path),
		Loader:            api.LoaderJS,
		Target:            api.ESNext,
		Charset:           api.CharsetUTF8,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		MinifyIdentifiers: false,
		KeepNames:         true,
		TreeShaking:       api.TreeShakingFalse,
		LegalComments:     api.LegalCommentsNone,
		IgnoreAnnotations: true,
	})
	if len(result.Errors) > 0 {
		messages := api.FormatMessages(result.Errors, api.FormatMessagesOptions{
			Kind:  api.ErrorMessage,
			Color: false,
		})
		return fmt.Errorf("minify %s: %s", filepath.ToSlash(relative_path), strings.TrimSpace(strings.Join(messages, "\n")))
	}

	minified := result.Code
	if len(minified) >= len(source) {
		minified = source
	}
	info, err := os.Stat(file_path)
	if err != nil {
		return fmt.Errorf("inspect %s after minification: %w", file_path, err)
	}
	if err := os.WriteFile(file_path, minified, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write minified JavaScript %s: %w", file_path, err)
	}

	stats.file_count++
	stats.original_len += int64(len(source))
	stats.minified_len += int64(len(minified))
	return nil
}

func format_bytes(length int64) string {
	const unit = int64(1024)
	if length < unit {
		return fmt.Sprintf("%d B", length)
	}
	if length < unit*unit {
		return fmt.Sprintf("%.1f KiB", float64(length)/float64(unit))
	}
	return fmt.Sprintf("%.2f MiB", float64(length)/float64(unit*unit))
}
