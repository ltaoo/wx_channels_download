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
func registerBuiltins(vm *goja.Runtime) {
	vm.Set("zipFiles", zipFilesFn(vm))
	vm.Set("removeFiles", removeFilesFn(vm))
	vm.Set("moveFile", moveFileFn(vm))
	vm.Set("writeTextFile", writeTextFileFn(vm))
	vm.Set("fileExists", fileExistsFn(vm))
	vm.Set("getFileName", getFileNameFn(vm))
	vm.Set("getDirName", getDirNameFn(vm))
	vm.Set("joinPath", joinPathFn(vm))
	vm.Set("readConfigJSON", readConfigJSONFn(vm))
	vm.Set("writeConfigJSON", writeConfigJSONFn(vm))
}

// resolveBasePath reads the current task's basePath from the VM and returns the canonical path.
func resolveBasePath(vm *goja.Runtime) (string, error) {
	val := vm.Get("__basePath")
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return "", fmt.Errorf("basePath is not set")
	}
	return filepath.Clean(val.String()), nil
}

// safeAbs resolves the given path to an absolute path and verifies it is within the basePath subtree.
func safeAbs(vm *goja.Runtime, p string) (string, error) {
	base, err := resolveBasePath(vm)
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
func zipFilesFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(fmt.Errorf("zipFiles requires srcPaths and destPath arguments"))
		}

		srcArgs := call.Argument(0).Export()
		srcList, ok := srcArgs.([]interface{})
		if !ok {
			return vm.ToValue(fmt.Errorf("srcPaths must be an array of strings"))
		}

		srcPaths := make([]string, len(srcList))
		for i, v := range srcList {
			srcPaths[i] = fmt.Sprint(v)
		}

		destPath := call.Argument(1).String()

		safeDest, err := safeAbs(vm, destPath)
		if err != nil {
			return vm.ToValue(err)
		}

		// Create the zip file
		f, err := os.Create(safeDest)
		if err != nil {
			return vm.ToValue(fmt.Errorf("failed to create zip file: %w", err))
		}
		defer f.Close()

		zw := zip.NewWriter(f)
		defer zw.Close()

		for _, src := range srcPaths {
			safeSrc, err := safeAbs(vm, src)
			if err != nil {
				return vm.ToValue(err)
			}

			if err := addFileToZip(zw, safeSrc); err != nil {
				return vm.ToValue(fmt.Errorf("failed to add %s to zip: %w", safeSrc, err))
			}
		}

		return goja.Undefined()
	}
}

func addFileToZip(zw *zip.Writer, filePath string) error {
	f, err := os.Open(filePath)
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
	header.Name = filepath.Base(filePath)
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, f)
	return err
}

// removeFilesFn deletes the specified files; directories are not deleted.
func removeFilesFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(fmt.Errorf("removeFiles requires a paths argument"))
		}

		pathsArg := call.Argument(0).Export()
		pathsList, ok := pathsArg.([]interface{})
		if !ok {
			return vm.ToValue(fmt.Errorf("paths must be an array of strings"))
		}

		for _, v := range pathsList {
			p := fmt.Sprint(v)
			safeP, err := safeAbs(vm, p)
			if err != nil {
				return vm.ToValue(err)
			}

			info, err := os.Stat(safeP)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return vm.ToValue(fmt.Errorf("failed to inspect file %s: %w", safeP, err))
			}
			if info.IsDir() {
				return vm.ToValue(fmt.Errorf("removing directories is not allowed: %s", safeP))
			}

			if err := os.Remove(safeP); err != nil {
				return vm.ToValue(fmt.Errorf("failed to remove file %s: %w", safeP, err))
			}
		}

		return goja.Undefined()
	}
}

// moveFileFn moves/renames a file.
func moveFileFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(fmt.Errorf("moveFile requires src and dst arguments"))
		}

		src := call.Argument(0).String()
		dst := call.Argument(1).String()

		safeSrc, err := safeAbs(vm, src)
		if err != nil {
			return vm.ToValue(err)
		}
		safeDst, err := safeAbs(vm, dst)
		if err != nil {
			return vm.ToValue(err)
		}

		// Ensure the destination directory exists
		if err := os.MkdirAll(filepath.Dir(safeDst), 0755); err != nil {
			return vm.ToValue(fmt.Errorf("failed to create destination directory: %w", err))
		}

		if err := os.Rename(safeSrc, safeDst); err != nil {
			return vm.ToValue(fmt.Errorf("failed to move file: %w", err))
		}

		return goja.Undefined()
	}
}

// writeTextFileFn writes a text file.
func writeTextFileFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(fmt.Errorf("writeTextFile requires path and text arguments"))
		}

		path := call.Argument(0).String()
		text := call.Argument(1).String()

		safePath, err := safeAbs(vm, path)
		if err != nil {
			return vm.ToValue(err)
		}

		if err := os.WriteFile(safePath, []byte(text), 0644); err != nil {
			return vm.ToValue(fmt.Errorf("failed to write file: %w", err))
		}

		return goja.Undefined()
	}
}

// fileExistsFn checks whether a file exists.
func fileExistsFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}

		path := call.Argument(0).String()
		safePath, err := safeAbs(vm, path)
		if err != nil {
			return vm.ToValue(false)
		}

		_, err = os.Stat(safePath)
		return vm.ToValue(err == nil)
	}
}

// getFileNameFn returns the filename portion of a path.
func getFileNameFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		return vm.ToValue(filepath.Base(call.Argument(0).String()))
	}
}

// getDirNameFn returns the directory portion of a path.
func getDirNameFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("")
		}
		return vm.ToValue(filepath.Dir(call.Argument(0).String()))
	}
}

// joinPathFn joins path segments.
func joinPathFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		elems := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			elems[i] = arg.String()
		}
		return vm.ToValue(filepath.Join(elems...))
	}
}

// readConfigJSONFn parses a JSON string into an object.
func readConfigJSONFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(nil)
		}

		jsonStr := call.Argument(0).String()
		var result interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return vm.ToValue(fmt.Errorf("failed to parse JSON: %w", err))
		}
		return vm.ToValue(result)
	}
}

// writeConfigJSONFn serializes an object to a JSON string.
func writeConfigJSONFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue("{}")
		}

		obj := call.Argument(0).Export()
		jsonBytes, err := json.Marshal(obj)
		if err != nil {
			return vm.ToValue(fmt.Errorf("failed to serialize JSON: %w", err))
		}
		return vm.ToValue(string(jsonBytes))
	}
}
