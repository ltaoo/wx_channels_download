package hermes

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wx_channel/pkg/dm"
)

func TestHookReadStreamUpload(t *testing.T) {
	data := bytes.Repeat([]byte{0, 1, 255, 2, 0, 3}, 200000)
	path := filepath.Join(t.TempDir(), "binary.dat")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, test_case := range []struct {
		name       string
		options    string
		start      int
		end        int
		request    bool
		media_type string
	}{
		{name: "full", options: "{}", end: len(data)},
		{name: "range request", options: "{start: 2, end: 105, highWaterMark: 7}", start: 2, end: 106, request: true, media_type: "application/octet-stream"},
		{name: "after EOF", options: fmt.Sprintf("{start: %d}", len(data)+10), start: len(data), end: len(data)},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			var received_hash string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Content-Type") != test_case.media_type {
					t.Errorf("incorrect content type: %q", request.Header.Get("Content-Type"))
				}
				hash := sha256.New()
				count, err := io.Copy(hash, request.Body)
				if err != nil || count != int64(test_case.end-test_case.start) {
					t.Errorf("received %d bytes: %v", count, err)
				}
				received_hash = fmt.Sprintf("%x", hash.Sum(nil))
				_, _ = writer.Write([]byte("uploaded"))
			}))
			defer server.Close()
			hooks := NewHookManager()
			call := `fetch(url, options)`
			if test_case.request {
				call = `fetch(new Request(url, options))`
			}
			if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export async function onTaskSuccess() {
  const source = globalThis.source = fs.createReadStream(%q, %s);
  if (source.bytesRead !== 0 || source.destroyed) throw new Error("stream read eagerly");
  const url = %q;
  const options = {method: "PUT", body: source, duplex: "half"};
  if (%q) options.headers = {"Content-Type": %q};
  const response = await %s;
  if (!response.ok || await response.text() !== "uploaded") throw new Error("upload failed");
  if (source.bytesRead !== %d || !source.destroyed || !source.readableEnded) throw new Error("incorrect stream state");
  let rejected = false;
  try { await fetch(url, options); } catch (error) { rejected = true; }
  if (!rejected) throw new Error("consumed stream reused");
}`, path, test_case.options, server.URL, test_case.media_type, test_case.media_type, call, test_case.end-test_case.start)); err != nil {
				t.Fatal(err)
			}
			if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
				t.Fatal(err)
			}
			expected_hash := fmt.Sprintf("%x", sha256.Sum256(data[test_case.start:test_case.end]))
			if received_hash != expected_hash {
				t.Fatalf("hash mismatch: %s != %s", received_hash, expected_hash)
			}
		})
	}
}

func TestHookReadStreamFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		if request.URL.Path == "/redirect" {
			writer.Header().Set("Location", "/target")
			writer.WriteHeader(http.StatusTemporaryRedirect)
		} else if request.URL.Path == "/target" {
			t.Error("one-shot stream must not be replayed after a redirect")
		}
	}))
	defer server.Close()
	hooks := NewHookManager()
	if err := hooks.Load(fmt.Sprintf(`import fs from "node:fs";
export async function onTaskSuccess() {
  const path = %q, url = %q;
  const source = fs.createReadStream(path);
  await fetch(url, {method: "POST", body: source}).then(() => { throw new Error("duplex was ignored"); }, error => { if (!(error instanceof TypeError)) throw error; });
  if (source.destroyed || source.bytesRead !== 0) throw new Error("invalid request consumed stream");
  source.destroy();
  for (const body of [source, fs.createReadStream(path + ".missing"), fs.createReadStream(path)]) {
    let rejected = false;
    try { await fetch(url + "/redirect", {method: "POST", body, duplex: "half"}); } catch (error) { rejected = true; }
    if (!rejected || !body.destroyed) throw new Error("stream error was lost");
  }
  for (const options of [{start: -1}, {start: 5, end: 2}, {highWaterMark: 1.5}]) {
    let rejected = false;
    try { fs.createReadStream(path, options); } catch (error) { rejected = true; }
    if (!rejected) throw new Error("invalid range accepted");
  }
}`, path, server.URL)); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatal(err)
	}
}

func TestHookReadStreamBackpressureAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.bin")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	const size = 256 << 20
	if err := file.Truncate(size); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var first [1]byte
		_, _ = io.ReadFull(request.Body, first[:])
		started <- struct{}{}
		<-release
		_, _ = io.Copy(io.Discard, request.Body)
	}))
	defer server.Close()
	defer close(release)
	hooks := NewHookManager()
	if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export async function onTaskSuccess(task) {
  if (!task.id) return;
  const source = globalThis.source = fs.createReadStream(%q);
  await fetch(%q, {method: "PUT", body: source, duplex: "half"});
}`, path, server.URL)); err != nil {
		t.Fatal(err)
	}
	hooks.runtime.timeout = 200 * time.Millisecond
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{ID: 1}, ""); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout, got %v", err)
	}
	select {
	case <-started:
	default:
		t.Fatal("upload did not start")
	}
	source := hook_value_stream(hooks.vm.Get("source"))
	source.mu.Lock()
	if !source.destroyed || source.bytes_read >= size {
		t.Errorf("stream ignored backpressure or cancellation: destroyed=%t read=%d", source.destroyed, source.bytes_read)
	}
	if source.file.Fd() != ^uintptr(0) {
		t.Error("file was left open after cancellation")
	}
	source.mu.Unlock()
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatalf("runtime could not recover from upload cancellation: %v", err)
	}
}
