package hermes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wx_channel/pkg/dm"
)

func TestHookFSImportsAndReads(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "内容.txt"), []byte("文件内容"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{
		`import fs from "fs";
export async function onTaskSuccess() {
  const binary = fs.readFileSync("内容.txt");
  if (!Buffer.isBuffer(binary) || Buffer.isBuffer(new Uint8Array(1)) || binary.toString("utf8") !== "文件内容") throw new Error("wrong Buffer");
  if (fs.readFileSync("内容.txt", "utf8") !== "文件内容") throw new Error("wrong text");
  if (await fs.promises.readFile("内容.txt", {encoding: "utf8"}) !== "文件内容") throw new Error("wrong async text");
  if (!fs.existsSync(".") || !fs.existsSync("内容.txt") || fs.existsSync("missing.txt")) throw new Error("wrong existence");
}`,
		`import {readFileSync} from "node:fs";
import {readFile} from "fs/promises";
import {Buffer as NodeBuffer} from "node:buffer";
export async function onTaskSuccess() {
  if (readFileSync("内容.txt", {encoding: "utf8"}) !== "文件内容") throw new Error("wrong named import");
  if (!NodeBuffer.isBuffer(await readFile("内容.txt"))) throw new Error("wrong async Buffer");
}`,
		`import * as fs from "fs";
import {readFile} from "node:fs/promises";
export async function onTaskSuccess() {
  if (await readFile("内容.txt", "utf8") !== "文件内容") throw new Error("wrong promises alias");
  if (await new Blob([fs.readFileSync("内容.txt")]).text() !== "文件内容") throw new Error("wrong Blob text");
}`,
	} {
		hooks := NewHookManager()
		script = strings.ReplaceAll(script, `"内容.txt"`, fmt.Sprintf("%q", filepath.Join(directory, "内容.txt")))
		if err := hooks.Load(script); err != nil {
			t.Fatal(err)
		}
		if !hooks.HasSuccessHook() {
			t.Fatal("imported hook was not discovered")
		}
		if err := hooks.InvokeSuccessHook(&dm.DownloadTask{ID: 42}, directory); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHookRawFileUploads(t *testing.T) {
	directory := t.TempDir()
	data := []byte{0, 1, 2, 255, 0, 228, 184, 173}
	if err := os.WriteFile(filepath.Join(directory, "binary.dat"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "empty.dat"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`fs.readFileSync("binary.dat")`,
		`await fs.promises.readFile("binary.dat")`,
		`await fs.openAsBlob("binary.dat", {type: "application/octet-stream"})`,
		`new Blob([fs.readFileSync("binary.dat")], {type: "application/octet-stream"})`,
		`Buffer.concat([Buffer.from([99]), fs.readFileSync("binary.dat"), Buffer.from([99])]).subarray(1, -1)`,
		`fs.readFileSync("empty.dat")`,
		`await fs.openAsBlob("empty.dat")`,
	} {
		t.Run(body, func(t *testing.T) {
			expected := data
			expected_path := "binary.dat"
			if strings.Contains(body, "empty.dat") {
				expected = nil
				expected_path = "empty.dat"
			}
			expected_path = filepath.Join(directory, expected_path)
			for _, name := range []string{"binary.dat", "empty.dat"} {
				body = strings.ReplaceAll(body, fmt.Sprintf("%q", name), fmt.Sprintf("%q", filepath.Join(directory, name)))
			}
			var received []byte
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				var err error
				received, err = io.ReadAll(request.Body)
				if err != nil {
					t.Error(err)
				}
				if request.Method != "PUT" || request.ContentLength != int64(len(expected)) {
					t.Errorf("method=%s length=%d", request.Method, request.ContentLength)
				}
				writer.Header().Set("Content-Type", "application/octet-stream")
				_, _ = writer.Write(received)
			}))
			defer server.Close()
			hooks := NewHookManager()
			if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export async function onTaskSuccess() {
  const response = await fetch(%q, {method: "PUT", body: %s});
  if (!response.ok) throw new Error("upload failed");
  if (!Buffer.from(await response.arrayBuffer()).equals(fs.readFileSync(%q))) throw new Error("wrong binary response");
}`, server.URL, body, expected_path)); err != nil {
				t.Fatal(err)
			}
			if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, directory); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(received, expected) {
				t.Fatalf("received %v, want %v", received, expected)
			}
		})
	}
}

func TestHookMultipartUploadsAndRedirect(t *testing.T) {
	directory := t.TempDir()
	data := []byte("视频文件\x00\xff\x00")
	filename := "测试视频.mp4"
	if err := os.WriteFile(filepath.Join(directory, filename), data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, use_file_blob := range []bool{false, true} {
		t.Run(fmt.Sprint(use_file_blob), func(t *testing.T) {
			var received []byte
			var submitted dm.DownloadTask
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/redirect" {
					http.Redirect(writer, request, "/upload", http.StatusTemporaryRedirect)
					return
				}
				if request.ContentLength <= int64(len(data)) {
					t.Error("multipart content length is missing")
				}
				if request.Header.Get("Authorization") != "Bearer test" {
					t.Error("authorization header missing")
				}
				if err := request.ParseMultipartForm(1024); err != nil {
					t.Error(err)
					writer.WriteHeader(400)
					return
				}
				defer request.MultipartForm.RemoveAll()
				file, header, err := request.FormFile("file")
				if err != nil {
					t.Error(err)
					writer.WriteHeader(400)
					return
				}
				defer file.Close()
				received, err = io.ReadAll(file)
				if err != nil {
					t.Error(err)
				}
				if header.Filename != filename || header.Header.Get("Content-Type") != "video/mp4" {
					t.Errorf("wrong file header: %+v", header)
				}
				if err := json.Unmarshal([]byte(request.FormValue("task")), &submitted); err != nil {
					t.Error(err)
				}
				if strings.Join(request.MultipartForm.Value["label"], ",") != "a,b" {
					t.Error("duplicate form fields were lost")
				}
				_, _ = writer.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()
			blob := `new Blob([fs.readFileSync(file.file_path)], {type: file.kind})`
			if use_file_blob {
				blob = `await fs.openAsBlob(file.file_path, {type: file.kind})`
			}
			hooks := NewHookManager()
			if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export async function onTaskSuccess(task) {
  const file = task.files[0];
  const form = new FormData();
  form.append("file", %s, file.name);
  form.append("task", JSON.stringify(task));
  form.append("label", "a");
  form.append("label", "b");
  const response = await fetch(%q, {method: "POST", headers: {Authorization: "Bearer test"}, body: form});
  if (!response.ok || !(await response.json()).ok) throw new Error("upload failed");
}`, blob, server.URL+"/redirect")); err != nil {
				t.Fatal(err)
			}
			task := &dm.DownloadTask{ID: 42, Name: "任务", Status: 5, Files: []dm.DownloadTaskFile{{Name: filename, FilePath: filepath.Join(directory, filename), Kind: "video/mp4"}}}
			if err := hooks.InvokeSuccessHook(task, directory); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(received, data) || submitted.ID != task.ID || submitted.Files[0].FilePath != task.Files[0].FilePath {
				t.Fatalf("wrong upload: bytes=%v task=%+v", received, submitted)
			}
		})
	}
}

func TestHookFSNodePathsAndErrors(t *testing.T) {
	directory := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("readable"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	hooks := NewHookManager()
	if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export async function onTaskSuccess() {
  for (const path of [%q, %q]) {
    if (fs.readFileSync(path, "utf8") !== "readable") throw new Error("wrong absolute read");
    if (await fs.promises.readFile(path, "utf8") !== "readable") throw new Error("wrong async read");
    if (await (await fs.openAsBlob(path)).text() !== "readable") throw new Error("wrong Blob read");
  }
  await fs.promises.readFile("missing").then(() => { throw new Error("missing read allowed"); }, error => { if (error.code !== "ENOENT") throw error; });
  try { fs.readFileSync("missing", "invalid-encoding"); throw new Error("bad encoding allowed"); }
  catch (error) { if (!(error instanceof TypeError)) throw error; }
}`, outside, filepath.Join(directory, "escape.txt"))); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, directory); err != nil {
		t.Fatal(err)
	}
	if err := hooks.Load(`import fs from "child_process"; export function onTaskSuccess() {}`); err == nil {
		t.Fatal("unsupported imports must fail")
	}
}

func TestHookFileBlobStaysLazyAndChecksChanges(t *testing.T) {
	directory := t.TempDir()
	file_path := filepath.Join(directory, "large.bin")
	file, err := os.Create(file_path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(128 << 20); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	hooks := NewHookManager()
	if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export async function onTaskSuccess() {
  globalThis.test_blob = await fs.openAsBlob(%q);
  if (test_blob.size !== 134217728) throw new Error("wrong Blob size");
}`, file_path)); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, directory); err != nil {
		t.Fatal(err)
	}
	blob := hook_value_blob(hooks.vm.Get("test_blob"))
	if blob == nil || len(blob.parts) != 1 || len(blob.parts[0].data) != 0 {
		t.Fatal("file Blob loaded file bytes into memory")
	}
	if err := os.WriteFile(file_path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := open_hook_blob(blob); err == nil {
		t.Fatal("changed file Blob must not be uploaded")
	}
}
