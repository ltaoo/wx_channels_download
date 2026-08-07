package hermes

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreateTaskDownloadsWithZeroConfig(t *testing.T) {
	content := []byte("high-level Hermes download")
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Hermes-Test") != "enabled" {
			http.Error(w, "missing test header", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Cookie") != "session=test" {
			http.Error(w, "missing test cookie", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		if r.Header.Get("Range") == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			_, _ = w.Write(content)
			return
		}
		var start, end int
		if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start || start >= len(content) {
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end >= len(content) {
			end = len(content) - 1
		}
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[start : end+1])
	}))
	defer target.Close()

	const proxyUsername = "download-user"
	const proxyPassword = "download-password"
	proxy, proxyRequests := newHighLevelTestProxy(t, proxyUsername, proxyPassword)
	defer proxy.Close()

	downloadDirectory := t.TempDir()
	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(downloadDirectory); err != nil {
		t.Fatal(err)
	}
	downloader := New(HermesNewConfig{})
	if err := os.Chdir(originalDirectory); err != nil {
		t.Fatal(err)
	}

	task := downloader.CreateTask(
		target.URL+"/source.bin",
		WithFilename("renamed.bin"),
		WithHeaders(map[string]string{"X-Hermes-Test": "enabled"}),
		WithCookies("session=test"),
		WithProxyServer(ProxyServer{
			Address:  proxy.URL,
			Username: proxyUsername,
			Password: proxyPassword,
		}),
	)
	if task.ID <= 0 {
		t.Fatalf("task ID = %d, want a positive ID", task.ID)
	}

	var eventMu sync.Mutex
	events := make([]EventType, 0, 4)
	terminal := make(chan EventType, 1)
	downloader.OnEvent(func(_ int, event EventType, _ *TaskProgress) {
		eventMu.Lock()
		events = append(events, event)
		eventMu.Unlock()
		if event == EventFinished || event == EventFailed {
			select {
			case terminal <- event:
			default:
			}
		}
	})

	select {
	case <-task.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("download did not finish")
	}
	if err := task.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	select {
	case event := <-terminal:
		if event != EventFinished {
			t.Fatalf("terminal event = %s, want %s", event, EventFinished)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal event was not emitted")
	}

	wantPath := filepath.Join(downloader.cfg.BasePath, "renamed.bin")
	if task.FilePath() != wantPath {
		t.Fatalf("FilePath() = %q, want %q", task.FilePath(), wantPath)
	}
	got, err := os.ReadFile(task.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("downloaded content = %q, want %q", got, content)
	}
	if proxyRequests.Load() == 0 {
		t.Fatal("proxy did not receive any requests")
	}

	eventMu.Lock()
	defer eventMu.Unlock()
	for _, want := range []EventType{EventCreated, EventStarted, EventFinished} {
		if !containsEvent(events, want) {
			t.Fatalf("events = %v, missing %s", events, want)
		}
	}
}

func TestCreateTaskReportsInvalidURL(t *testing.T) {
	task := New(HermesNewConfig{}).CreateTask("://invalid")
	if err := task.Wait(); err == nil {
		t.Fatal("Wait() error = nil, want an invalid URL error")
	}
}

func containsEvent(events []EventType, want EventType) bool {
	for _, event := range events {
		if event == want {
			return true
		}
	}
	return false
}

func newHighLevelTestProxy(t *testing.T, username, password string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	directTransport := &http.Transport{}
	t.Cleanup(directTransport.CloseIdleConnections)

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantAuthorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
		if r.Header.Get("Proxy-Authorization") != wantAuthorization {
			w.Header().Set("Proxy-Authenticate", `Basic realm="hermes-test"`)
			http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
			return
		}
		requests.Add(1)

		outbound := r.Clone(r.Context())
		outbound.RequestURI = ""
		outbound.Header.Del("Proxy-Authorization")
		resp, err := directTransport.RoundTrip(outbound)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	return proxy, &requests
}
