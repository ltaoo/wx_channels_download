package hermes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"wx_channel/pkg/dm"
)

func TestHookFSMatchesNodeReadUsage(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "内容.txt")
	if err := os.WriteFile(path, []byte("文件内容\x00\xff"), 0600); err != nil {
		t.Fatal(err)
	}
	working_dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(working_dir, path)
	if err != nil {
		t.Fatal(err)
	}
	file_url := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	script := fmt.Sprintf(`import fs from "fs";
import {readFile} from "node:fs/promises";
import {URL} from "node:url";
const path = %q;
const result = globalThis.compat_result = {};
function failure(work) {
  try { work(); return null; }
  catch (error) { return {name: error.name, code: error.code}; }
}
result.preload = fs.readFileSync(%q, "hex");
result.bufferPath = fs.readFileSync(Buffer.from(path), "hex");
result.urlPath = fs.readFileSync(new URL(%q), "hex");
result.encodings = [null, "", "UTF8", "utf-8", "hex", "base64", "BASE64URL", "latin1", "binary", "ascii", "utf16le", "ucs2"].map(encoding => {
  const data = fs.readFileSync(path, {encoding});
  return Buffer.isBuffer(data) ? {buffer: data.toString("hex")} : data;
});
result.exists = [fs.existsSync(path), fs.existsSync(Buffer.from(path)), fs.existsSync(new URL(%q)), fs.existsSync(null), fs.existsSync(123), fs.existsSync(path + ".missing")];
result.errors = [
  failure(() => fs.readFileSync(null)),
  failure(() => fs.readFileSync(path, 123)),
  failure(() => fs.readFileSync(path, "unknown")),
  failure(() => fs.readFileSync(path, {flag: "unknown"})),
  failure(() => fs.readFileSync(path, {flag: 1.5})),
  failure(() => fs.readFileSync(path, {flag: "wx+"})),
  failure(() => fs.readFileSync(%q)),
  failure(() => fs.readFile(path)),
];
result.flag = fs.readFileSync(path, {flag: "r+", encoding: "hex"});
result.nullFlag = fs.readFileSync(path, {flag: null, encoding: "hex"});
result.created = fs.readFileSync(%q, {flag: "w+", encoding: "utf8"});
export function onTaskSuccess() {
  const returned = fs.readFile(path, (error, data) => {
    result.callback = {error, buffer: Buffer.isBuffer(data), data: data.toString("hex")};
    fs.readFile(path + ".missing", "utf8", (error, data) => {
      result.callbackError = {name: error.name, code: error.code, path: error.path, noData: data === undefined};
    });
  });
  result.returnUndefined = returned === undefined;
  fs.readFile(path, {encoding: "utf8"}, (error, data) => { result.callbackText = {error, data}; });
  readFile(path, "hex").then(data => { result.promise = data; });
  const bad = readFile(null);
  result.rejectionIsPromise = bad instanceof Promise;
  bad.catch(error => { result.promiseError = {name: error.name, code: error.code}; });
}`, path, relative, file_url, file_url, directory, filepath.Join(directory, "created.txt"))
	hooks := NewHookManager()
	if err := hooks.Load(script); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(hooks.vm.Get("compat_result").Export())
	if err != nil {
		t.Fatal(err)
	}
	var actual_value map[string]any
	if err := json.Unmarshal(actual, &actual_value); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"preload", "callback", "callbackError", "callbackText", "promise", "promiseError"} {
		if actual_value[key] == nil {
			t.Fatalf("%s did not complete: %s", key, actual)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Log("Node.js is unavailable; skipped comparison against Node")
		return
	}
	output, err := exec.Command(node, "--input-type=module", "-e", script+`
onTaskSuccess();
process.on("beforeExit", () => process.stdout.write(JSON.stringify(compat_result)));
`).CombinedOutput()
	if err != nil {
		t.Fatalf("Node.js execution failed: %v\n%s", err, output)
	}
	var expected_value map[string]any
	if err := json.Unmarshal(output, &expected_value); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual_value, expected_value) {
		t.Fatalf("Goja and Node.js differ:\nGoja: %s\nNode: %s", actual, output)
	}
}

func TestHookFSCallbackErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, callback := range []string{
		`() => { throw new Error("callback failed"); }`,
		`async () => { throw new Error("callback failed"); }`,
	} {
		hooks := NewHookManager()
		if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export function onTaskSuccess() { fs.readFile(%q, %s); }`, path, callback)); err != nil {
			t.Fatal(err)
		}
		if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err == nil || !strings.Contains(err.Error(), "callback failed") {
			t.Fatalf("callback error was lost: %v", err)
		}
	}
}

func TestHookFSCallbackUpload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.dat")
	data := "video\x00\xff"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(1024); err != nil {
			t.Error(err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		defer request.MultipartForm.RemoveAll()
		file, _, err := request.FormFile("file")
		if err != nil {
			t.Error(err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Error(err)
		}
		received <- string(body)
		_, _ = writer.Write([]byte("uploaded"))
	}))
	defer server.Close()
	hooks := NewHookManager()
	if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export function onTaskSuccess() {
  fs.readFile(%q, (error, data) => {
    if (error) throw error;
    const form = new FormData();
    form.append("file", new Blob([data]), "video.dat");
    fetch(%q, {method: "POST", body: form})
      .then(response => response.text())
      .then(text => { globalThis.upload_result = text; });
  });
}`, path, server.URL)); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatal(err)
	}
	if value := hooks.vm.Get("upload_result"); value == nil || value.String() != "uploaded" {
		t.Fatalf("callback upload did not finish: %v", value)
	}
	select {
	case actual := <-received:
		if actual != data {
			t.Fatalf("wrong upload: %q", actual)
		}
	default:
		t.Fatal("file was not uploaded")
	}
}

func TestHookFSReadsFromTimerAfterTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	hooks := NewHookManager()
	if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export function onTaskSuccess(task) {
  if (task.id === 1) {
    setInterval(() => {}, 1);
  } else {
    setTimeout(() => fs.readFile(%q, "utf8", (error, text) => {
      if (error) throw error;
      globalThis.timer_text = text;
    }), 0);
  }
}`, path)); err != nil {
		t.Fatal(err)
	}
	hooks.runtime.timeout = 100 * time.Millisecond
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{ID: 1}, ""); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timer deadline, got %v", err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatal(err)
	}
	if value := hooks.vm.Get("timer_text"); value == nil || value.String() != "content" {
		t.Fatalf("timer file read did not complete: %v", value)
	}
}
