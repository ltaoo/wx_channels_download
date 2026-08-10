package hermes

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
)

// registerBuiltins registers all built-in utility functions with the goja VM.
func register_builtins(vm *goja.Runtime) {
	vm.Set("zipFiles", zip_files_fn(vm))
	vm.Set("removeFiles", remove_files_fn(vm))
	vm.Set("moveFile", move_file_fn(vm))
	vm.Set("writeTextFile", write_text_file_fn(vm))
	vm.Set("fileExists", file_exists_fn(vm))
	vm.Set("getFileName", get_file_name_fn(vm))
	vm.Set("getDirName", get_dir_name_fn(vm))
	vm.Set("joinPath", join_path_fn(vm))
	vm.Set("readConfigJSON", read_config_json_fn(vm))
	vm.Set("writeConfigJSON", write_config_json_fn(vm))
}

// resolveBasePath reads the current task's basePath from the VM and returns the canonical path.
func resolve_base_path(vm *goja.Runtime) (string, error) {
	val := vm.Get("__basePath")
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return "", fmt.Errorf("basePath is not set")
	}
	return filepath.Clean(val.String()), nil
}

// safeAbs resolves the given path to an absolute path and verifies it is within the basePath subtree.
func safe_abs(vm *goja.Runtime, p string) (string, error) {
	base, err := resolve_base_path(vm)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	// Ensure the path does not escape basePath
	rel, err := filepath.Rel(base, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %s is outside the allowed directory %s", abs, base)
	}
	return abs, nil
}

// zipFilesFn packs multiple source files into a single zip file.
func zip_files_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(fmt.Errorf("zipFiles requires srcPaths and destPath arguments"))
		}

		src_args := call.Argument(0).Export()
		src_list, ok := src_args.([]interface{})
		if !ok {
			return vm.ToValue(fmt.Errorf("srcPaths must be an array of strings"))
		}

		src_paths := make([]string, len(src_list))
		for i, v := range src_list {
			src_paths[i] = fmt.Sprint(v)
		}

		dest_path := call.Argument(1).String()

		safe_dest, err := safe_abs(vm, dest_path)
		if err != nil {
			return vm.ToValue(err)
		}

		// Create the zip file
		f, err := os.Create(safe_dest)
		if err != nil {
			return vm.ToValue(fmt.Errorf("failed to create zip file: %w", err))
		}
		defer f.Close()

		zw := zip.NewWriter(f)
		defer zw.Close()

		for _, src := range src_paths {
			safe_src, err := safe_abs(vm, src)
			if err != nil {
				return vm.ToValue(err)
			}

			if err := add_file_to_zip(zw, safe_src); err != nil {
				return vm.ToValue(fmt.Errorf("failed to add %s to zip: %w", safe_src, err))
			}
		}

		return goja.Undefined()
	}
}

func add_file_to_zip(zw *zip.Writer, file_path string) error {
	f, err := os.Open(file_path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(file_path)
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, f)
	return err
}

// removeFilesFn deletes the specified files; directories are not deleted.
func remove_files_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(fmt.Errorf("removeFiles requires a paths argument"))
		}

		paths_arg := call.Argument(0).Export()
		paths_list, ok := paths_arg.([]interface{})
		if !ok {
			return vm.ToValue(fmt.Errorf("paths must be an array of strings"))
		}

		for _, v := range paths_list {
			p := fmt.Sprint(v)
			safe_p, err := safe_abs(vm, p)
			if err != nil {
				return vm.ToValue(err)
			}

			info, err := os.Stat(safe_p)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return vm.ToValue(fmt.Errorf("failed to inspect file %s: %w", safe_p, err))
			}
			if info.IsDir() {
				return vm.ToValue(fmt.Errorf("removing directories is not allowed: %s", safe_p))
			}

			if err := os.Remove(safe_p); err != nil {
				return vm.ToValue(fmt.Errorf("failed to remove file %s: %w", safe_p, err))
			}
		}

		return goja.Undefined()
	}
}

// moveFileFn moves/renames a file.
func move_file_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(fmt.Errorf("moveFile requires src and dst arguments"))
		}

		src := call.Argument(0).String()
		dst := call.Argument(1).String()

		safe_src, err := safe_abs(vm, src)
		if err != nil {
			return vm.ToValue(err)
		}
		safe_dst, err := safe_abs(vm, dst)
		if err != nil {
			return vm.ToValue(err)
		}

		// Ensure the destination directory exists
		if err := os.MkdirAll(filepath.Dir(safe_dst), 0755); err != nil {
			return vm.ToValue(fmt.Errorf("failed to create destination directory: %w", err))
		}

		if err := os.Rename(safe_src, safe_dst); err != nil {
			return vm.ToValue(fmt.Errorf("failed to move file: %w", err))
		}

		return goja.Undefined()
	}
}

// writeTextFileFn writes a text file.
func write_text_file_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(fmt.Errorf("writeTextFile requires path and text arguments"))
		}

		path := call.Argument(0).String()
		text := call.Argument(1).String()

		safe_path, err := safe_abs(vm, path)
		if err != nil {
			return vm.ToValue(err)
		}

		if err := os.WriteFile(safe_path, []byte(text), 0644); err != nil {
			return vm.ToValue(fmt.Errorf("failed to write file: %w", err))
		}

		return goja.Undefined()
	}
}

// fileExistsFn checks whether a file exists.
func file_exists_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}

		path := call.Argument(0).String()
		safe_path, err := safe_abs(vm, path)
		if err != nil {
			return vm.ToValue(false)
		}

		_, err = os.Stat(safe_path)
		return vm.ToValue(err == nil)
	}
}

// getFileNameFn returns the filename portion of a path.
func get_file_name_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		return vm.ToValue(filepath.Base(call.Argument(0).String()))
	}
}

// getDirNameFn returns the directory portion of a path.
func get_dir_name_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		return vm.ToValue(filepath.Dir(call.Argument(0).String()))
	}
}

// joinPathFn joins path segments.
func join_path_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		elems := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			elems[i] = arg.String()
		}
		return vm.ToValue(filepath.Join(elems...))
	}
}

// readConfigJSONFn parses a JSON string into an object.
func read_config_json_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(nil)
		}

		json_str := call.Argument(0).String()
		var result interface{}
		if err := json.Unmarshal([]byte(json_str), &result); err != nil {
			return vm.ToValue(fmt.Errorf("failed to parse JSON: %w", err))
		}
		return vm.ToValue(result)
	}
}

// writeConfigJSONFn serializes an object to a JSON string.
func write_config_json_fn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("{}")
		}

		obj := call.Argument(0).Export()
		json_bytes, err := json.Marshal(obj)
		if err != nil {
			return vm.ToValue(fmt.Errorf("failed to serialize JSON: %w", err))
		}
		return vm.ToValue(string(json_bytes))
	}
}
