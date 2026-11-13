package hermes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dop251/goja"
	"wx_channel/pkg/dm"
)

func TestSuccessHookFetch(t *testing.T) {
	for _, status_code := range []int{http.StatusOK, http.StatusNoContent, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status_code), func(t *testing.T) {
			ctx := &dm.DownloadTask{
				ID: 42, Name: "测试任务", Status: TaskStatusFinished,
				ConfigJSON: `{"suffix":".mp4"}`, MetadataJSON: `{"title":"测试内容"}`,
				Files: []dm.DownloadTaskFile{{ID: 7, Name: "成片.mp4", Kind: "video/mp4", Size: 123, FilePath: "/downloads/成片.mp4"}},
			}

			var received map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost || request.URL.Path != "/webhook/download_task/success" || request.Header.Get("Content-Type") != "application/json" {
					t.Errorf("unexpected request: %s %s %v", request.Method, request.URL, request.Header)
				}
				if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
					t.Error(err)
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(status_code)
				if status_code != http.StatusNoContent {
					_, _ = writer.Write([]byte(`{"message":"收到任务"}`))
				}
			}))
			defer server.Close()
			hooks := NewHookManager()
			if err := hooks.Load(fmt.Sprintf(`export async function onTaskSuccess(ctx) {
  const response = await fetch(%q, {
    method: "POST", headers: new Headers({"Content-Type": "application/json"}), body: JSON.stringify(ctx)
  });
  if (response.status !== %d) throw new Error("wrong status");
  if (response.headers.get("Content-Type") !== "application/json") throw new Error("wrong headers");
  if (!response.ok) throw new Error("HTTP " + response.status + " " + await response.text());
  if (response.status !== 204) {
    const data = await response.json();
    if (data.message !== "收到任务" || !response.bodyUsed) throw new Error("wrong body");
    try { await response.text(); throw new Error("body read twice"); }
    catch (error) { if (!(error instanceof TypeError)) throw error; }
  }
}`, server.URL+"/webhook/download_task/success", status_code)); err != nil {
				t.Fatal(err)
			}
			err := hooks.InvokeSuccessHook(ctx, "/downloads")
			if status_code < 300 {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "HTTP 500") || !strings.Contains(err.Error(), "收到任务") {
				t.Fatalf("expected HTTP 500 and response body, got %v", err)
			}
			encoded, _ := json.Marshal(ctx)
			var expected map[string]any
			if err := json.Unmarshal(encoded, &expected); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(received, expected) {
				t.Fatalf("received %v, want %v", received, expected)
			}
			server.Close()
			if err := hooks.InvokeSuccessHook(ctx, "/downloads"); err == nil || !strings.Contains(err.Error(), "Network request failed") {
				t.Fatalf("expected network rejection, got %v", err)
			}
		})
	}
}

func TestHookFetchPromiseBehavior(t *testing.T) {
	var in_flight atomic.Int32
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirect" {
			http.Redirect(writer, request, "/json", http.StatusFound)
			return
		}
		if request.URL.Path == "/concurrent" {
			if in_flight.Add(1) == 2 {
				close(ready)
			}
			select {
			case <-ready:
			case <-request.Context().Done():
				return
			}
		}
		if request.URL.Path == "/invalid-json" {
			_, _ = writer.Write([]byte("invalid"))
			return
		}
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	for _, test_case := range []struct{ name, script, expected_error string }{
		{"concurrent", `export async function onTaskSuccess() {
  const responses = await Promise.all([fetch(BASE + "/concurrent"), fetch(BASE + "/concurrent")]);
  if (responses.length !== 2 || !responses.every(r => r.ok)) throw new Error("not concurrent");
}`, ""},
		{"redirect", `export async function onTaskSuccess() {
  const response = await fetch(new Request(BASE + "/redirect"));
  if (response.url !== BASE + "/json" || !(await response.json()).ok) throw new Error("redirect failed");
}`, ""},
		{"http-error-is-response", `export async function onTaskSuccess() {
  if (new Response("failure", {status: 500}).ok) throw new Error("wrong ok");
}`, ""},
		{"invalid-json", `export async function onTaskSuccess() { await (await fetch(BASE + "/invalid-json")).json(); }`, "SyntaxError"},
		{"invalid-url", `export async function onTaskSuccess() { await fetch("://bad", {headers: {accept: "application/json"}}); }`, "TypeError"},
		{"async-throw", `export async function onTaskSuccess() { await Promise.resolve(); throw new Error("async failure"); }`, "async failure"},
		{"timer", `export async function onTaskSuccess() { await new Promise(resolve => setTimeout(resolve, 1)); }`, ""},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			hooks := NewHookManager()
			if err := hooks.Load(fmt.Sprintf("const BASE = %q;\n%s", server.URL, test_case.script)); err != nil {
				t.Fatal(err)
			}
			hooks.runtime.timeout = time.Second
			err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, "")
			if test_case.expected_error == "" && err != nil || test_case.expected_error != "" && (err == nil || !strings.Contains(err.Error(), test_case.expected_error)) {
				t.Fatalf("got %v, want error containing %q", err, test_case.expected_error)
			}
		})
	}
}

func TestAsyncHookReturnValuesAndOrder(t *testing.T) {
	hooks := NewHookManager()
	if err := hooks.Load(`let finished = false;
export async function onTaskCreate(task, resources, config) { await Promise.resolve(); return {task, resources, config}; }
export async function onFilename() { await Promise.resolve(); return {name: "成片", directories: []}; }
export async function onTaskSuccess() { await new Promise(resolve => setTimeout(resolve, 1)); finished = true; }
export async function onTaskFinish() { if (!finished) throw new Error("success not finished"); }
`); err != nil {
		t.Fatal(err)
	}
	input := &TaskInput{Task: TaskInfo{Name: "test"}}
	output, err := hooks.InvokeCreateHook(input)
	if err != nil || output.Task.Name != input.Task.Name {
		t.Fatalf("create output=%v, err=%v", output, err)
	}
	name, err := hooks.InvokeFilenameHook(&FilenameParams{})
	if err != nil || name.Name != "成片" {
		t.Fatalf("filename output=%v, err=%v", name, err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeFinishHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatal(err)
	}
	if err := hooks.Load("export function onTaskSuccess() {}"); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatal(err)
	}
	value, err := hooks.runtime.run(func() (goja.Value, error) {
		return hooks.vm.RunString(`try { require("./untrusted.js"); } catch (error) { error.message; }`)
	})
	if err != nil || !strings.Contains(value.String(), "Unsupported hook module") {
		t.Fatalf("external modules must stay unavailable, got %v, err=%v", value, err)
	}
}

func TestHookFetchTimeout(t *testing.T) {
	request_cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
		close(request_cancelled)
	}))
	defer server.Close()
	hooks := NewHookManager()
	if err := hooks.Load(fmt.Sprintf(`export async function onTaskSuccess(ctx) {
  if (ctx.id === 1) await fetch(%q);
}`, server.URL)); err != nil {
		t.Fatal(err)
	}
	hooks.runtime.timeout = 50 * time.Millisecond
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{ID: 1}, ""); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected hook timeout, got %v", err)
	}
	select {
	case <-request_cancelled:
	case <-time.After(time.Second):
		t.Fatal("timed-out request was not cancelled")
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatalf("runtime cannot be reused after timeout: %v", err)
	}
}
