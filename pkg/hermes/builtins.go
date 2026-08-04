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
		return "", fmt.Errorf("basePath 未设置")
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
		return "", fmt.Errorf("解析路径失败: %w", err)
	}
	// Ensure the path does not escape basePath
	rel, err := filepath.Rel(base, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("路径 %s 不在允许的目录 %s 内", abs, base)
	}
	return abs, nil
}

// zipFilesFn packs multiple source files into a single zip file.
func zipFilesFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(fmt.Errorf("zipFiles 需要 srcPaths 和 destPath 参数"))
		}

		srcArgs := call.Argument(0).Export()
		srcList, ok := srcArgs.([]interface{})
		if !ok {
			return vm.ToValue(fmt.Errorf("srcPaths 必须是字符串数组"))
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
			return vm.ToValue(fmt.Errorf("创建 zip 文件失败: %w", err))
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
				return vm.ToValue(fmt.Errorf("添加 %s 到 zip 失败: %w", safeSrc, err))
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
			return vm.ToValue(fmt.Errorf("removeFiles 需要 paths 参数"))
		}

		pathsArg := call.Argument(0).Export()
		pathsList, ok := pathsArg.([]interface{})
		if !ok {
			return vm.ToValue(fmt.Errorf("paths 必须是字符串数组"))
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
				return vm.ToValue(fmt.Errorf("检查文件 %s 失败: %w", safeP, err))
			}
			if info.IsDir() {
				return vm.ToValue(fmt.Errorf("不允许删除目录: %s", safeP))
			}

			if err := os.Remove(safeP); err != nil {
				return vm.ToValue(fmt.Errorf("删除文件 %s 失败: %w", safeP, err))
			}
		}

		return goja.Undefined()
	}
}

// moveFileFn moves/renames a file.
func moveFileFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(fmt.Errorf("moveFile 需要 src 和 dst 参数"))
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
			return vm.ToValue(fmt.Errorf("创建目标目录失败: %w", err))
		}

		if err := os.Rename(safeSrc, safeDst); err != nil {
			return vm.ToValue(fmt.Errorf("移动文件失败: %w", err))
		}

		return goja.Undefined()
	}
}

// writeTextFileFn writes a text file.
func writeTextFileFn(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(fmt.Errorf("writeTextFile 需要 path 和 text 参数"))
		}

		path := call.Argument(0).String()
		text := call.Argument(1).String()

		safePath, err := safeAbs(vm, path)
		if err != nil {
			return vm.ToValue(err)
		}

		if err := os.WriteFile(safePath, []byte(text), 0644); err != nil {
			return vm.ToValue(fmt.Errorf("写入文件失败: %w", err))
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
			return vm.ToValue(fmt.Errorf("解析 JSON 失败: %w", err))
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
			return vm.ToValue(fmt.Errorf("序列化 JSON 失败: %w", err))
		}
		return vm.ToValue(string(jsonBytes))
	}
}
