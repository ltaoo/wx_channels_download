package browsermgr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCreateDockerBuildsRemoteDebugContainerCommand(t *testing.T) {
	runner := &fakeRunner{output: "container-id"}
	cdpHostPort := startFakeCDPServer(t)
	mgr, err := New(Config{
		WorkDir:     t.TempDir(),
		DockerImage: "chrome-image",
		CDPPortMin:  40000,
		CDPPortMax:  40010,
		ShmSize:     "2g",
	}, runner)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.CreateDocker(context.Background(), CreateRequest{Alias: "cf-browser", CDPHostPort: cdpHostPort, DesktopHostPort: 30123})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Endpoint == nil || rec.Endpoint.CDPURL != fmt.Sprintf("http://127.0.0.1:%d", cdpHostPort) {
		t.Fatalf("endpoint = %#v", rec.Endpoint)
	}
	if rec.Endpoint.DesktopURL != "http://127.0.0.1:30123" || rec.Endpoint.DesktopHostPort != 30123 {
		t.Fatalf("desktop endpoint = %#v", rec.Endpoint)
	}
	if rec.Status != StatusCreating {
		t.Fatalf("status = %q", rec.Status)
	}
	got := waitBrowserStatus(t, mgr, rec.ID, StatusRunning)
	if got.Endpoint == nil || got.Endpoint.ContainerID != "container-id" {
		t.Fatalf("record after async create = %#v", got)
	}
	name, args := runner.Last()
	if name != "docker" || len(args) == 0 {
		t.Fatalf("runner = %#v", runner)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"run -d",
		"--name wx-browser-",
		fmt.Sprintf("-p %d:9222", cdpHostPort),
		"-p 30123:3000",
		"--shm-size 2g",
		"CHROME_CLI=--no-sandbox",
		"chrome-image",
		"--remote-debugging-address=0.0.0.0",
		"--remote-debugging-port=9222",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("docker args missing %q: %s", want, joined)
		}
	}
}

func startFakeCDPServer(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Browser":"Chrome/123","Protocol-Version":"1.3"}`))
	})
	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("fake cdp server: %v", err)
		}
	}()
	t.Cleanup(func() {
		_ = srv.Close()
	})
	return ln.Addr().(*net.TCPAddr).Port
}

func TestRegisterLocalBrowser(t *testing.T) {
	mgr, err := New(Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.RegisterLocal(CreateRequest{Alias: "local", CDPURL: "http://127.0.0.1:9222/", DesktopURL: "http://127.0.0.1:39017/"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Kind != KindLocal || rec.Endpoint == nil || rec.Endpoint.CDPURL != "http://127.0.0.1:9222" {
		t.Fatalf("record = %#v", rec)
	}
	if rec.Endpoint.SessionURL != "http://127.0.0.1:39017" || rec.Endpoint.SessionHostPort != 39017 {
		t.Fatalf("session endpoint = %#v", rec.Endpoint)
	}
	if rec.Status != StatusRunning {
		t.Fatalf("status = %q", rec.Status)
	}
}

func TestRegisterExistingSandboxFromPorts(t *testing.T) {
	mgr, err := New(Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.Create(context.Background(), CreateRequest{
		Kind:            KindExisting,
		Alias:           "existing",
		CDPURL:          "http://127.0.0.1:39228",
		DesktopHostPort: 39017,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Kind != KindExisting || rec.Endpoint == nil {
		t.Fatalf("record = %#v", rec)
	}
	if rec.Endpoint.CDPHostPort != 39228 || rec.Endpoint.SessionURL != "http://127.0.0.1:39017" {
		t.Fatalf("endpoint = %#v", rec.Endpoint)
	}
}

func TestLocalSandboxFromAnotherDeviceIsUnavailable(t *testing.T) {
	workDir := t.TempDir()
	mgrA, err := New(Config{WorkDir: workDir, DeviceID: "device-a", DeviceName: "Laptop A"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgrA.RegisterLocal(CreateRequest{
		Alias:  "local",
		CDPURL: "http://127.0.0.1:9222",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Device == nil || rec.Device.ID != "device-a" || !rec.DeviceBound || !rec.LocalDevice {
		t.Fatalf("record device = %#v", rec)
	}

	mgrB, err := New(Config{WorkDir: workDir, DeviceID: "device-b", DeviceName: "Laptop B"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	list := mgrB.List()
	if len(list) != 1 {
		t.Fatalf("list len = %d", len(list))
	}
	got := list[0]
	if got.Status != StatusUnavailable || got.LocalDevice || got.Device == nil || got.Device.ID != "device-a" {
		t.Fatalf("record on another device = %#v", got)
	}
	if _, err := mgrB.CDPURL(rec.ID); err == nil || !strings.Contains(err.Error(), "Laptop A") {
		t.Fatalf("CDPURL error = %v", err)
	}
	if _, err := mgrB.Acquire(context.Background()); err == nil {
		t.Fatal("expected no idle browser on another device")
	}
	refreshed, err := mgrB.Refresh(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != StatusUnavailable {
		t.Fatalf("refresh from another device = %#v", refreshed)
	}
	if gotA, _ := mgrA.Get(rec.ID); gotA.Status != StatusRunning || !gotA.LocalDevice {
		t.Fatalf("record on owner device after remote refresh = %#v", gotA)
	}
}

func TestRefreshLegacyLocalSandboxAdoptsCurrentDeviceWhenCDPWorks(t *testing.T) {
	cdpHostPort := startFakeCDPServer(t)
	mgr, err := New(Config{WorkDir: t.TempDir(), DeviceID: "device-a", DeviceName: "Laptop A"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.put(&Record{
		ID:     "legacy",
		Kind:   KindExisting,
		Status: StatusRunning,
		Endpoint: &RuntimeEndpoint{
			CDPURL: fmt.Sprintf("http://127.0.0.1:%d", cdpHostPort),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != StatusUnavailable || rec.Device != nil {
		t.Fatalf("legacy record before refresh = %#v", rec)
	}
	refreshed, err := mgr.Refresh(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != StatusRunning || !refreshed.LocalDevice || refreshed.Device == nil || refreshed.Device.ID != "device-a" {
		t.Fatalf("legacy record after refresh = %#v", refreshed)
	}
}

func TestAcquireReleaseBrowserLease(t *testing.T) {
	mgr, err := New(Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.RegisterLocal(CreateRequest{Alias: "local", CDPURL: "http://127.0.0.1:9222"})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := mgr.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if lease.ID != rec.ID || lease.CDPURL != "http://127.0.0.1:9222" {
		t.Fatalf("lease = %#v", lease)
	}
	if got, _ := mgr.Get(rec.ID); got.Status != StatusBusy {
		t.Fatalf("status after acquire = %q", got.Status)
	}
	if _, err := mgr.Acquire(context.Background()); err == nil {
		t.Fatal("expected no idle browser error")
	}
	if err := mgr.Release(rec.ID, ReleaseOptions{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := mgr.Get(rec.ID); got.Status != StatusRunning {
		t.Fatalf("status after release = %q", got.Status)
	}
}

func TestReleaseCanMarkBrowserInvalid(t *testing.T) {
	mgr, err := New(Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.RegisterLocal(CreateRequest{CDPURL: "http://127.0.0.1:9222"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Release(rec.ID, ReleaseOptions{Invalid: true, Error: errors.New("cdp failed").Error()}); err != nil {
		t.Fatal(err)
	}
	got, _ := mgr.Get(rec.ID)
	if got.Status != StatusInvalid || got.Error != "cdp failed" {
		t.Fatalf("record = %#v", got)
	}
}

func TestRefreshInvalidLocalBrowserRestoresRunningWhenCDPWorks(t *testing.T) {
	cdpHostPort := startFakeCDPServer(t)
	mgr, err := New(Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.RegisterLocal(CreateRequest{CDPURL: fmt.Sprintf("http://127.0.0.1:%d", cdpHostPort)})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Release(rec.ID, ReleaseOptions{Invalid: true, Error: "temporary cdp failure"}); err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Refresh(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRunning || got.Error != "" {
		t.Fatalf("record = %#v", got)
	}
}

func TestRefreshDockerBrowserRestoresRunningWhenContainerAndCDPWork(t *testing.T) {
	cdpHostPort := startFakeCDPServer(t)
	mgr, err := New(Config{WorkDir: t.TempDir()}, &fakeRunner{output: "true"})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.put(&Record{
		ID:     "docker1",
		Kind:   KindDocker,
		Status: StatusInvalid,
		Endpoint: &RuntimeEndpoint{
			ContainerID: "container-id",
			CDPHostPort: cdpHostPort,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Refresh(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRunning || got.Error != "" {
		t.Fatalf("record = %#v", got)
	}
}

func TestRefreshDockerBrowserMarksPausedWhenContainerIsNotRunning(t *testing.T) {
	mgr, err := New(Config{WorkDir: t.TempDir()}, &fakeRunner{output: "false"})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.put(&Record{
		ID:     "docker1",
		Kind:   KindDocker,
		Status: StatusInvalid,
		Endpoint: &RuntimeEndpoint{
			ContainerID: "container-id",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Refresh(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusPaused || got.Error != "" {
		t.Fatalf("record = %#v", got)
	}
}

type fakeRunner struct {
	mu     sync.Mutex
	name   string
	args   []string
	output string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.name = name
	r.args = append([]string(nil), args...)
	return r.output, nil
}

func (r *fakeRunner) Last() (string, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.name, append([]string(nil), r.args...)
}

func waitBrowserStatus(t *testing.T, mgr *Manager, id string, status Status) *Record {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		rec, ok := mgr.Get(id)
		if ok && rec.Status == status {
			return rec
		}
		time.Sleep(10 * time.Millisecond)
	}
	rec, _ := mgr.Get(id)
	t.Fatalf("browser %s status = %#v, want %s", id, rec, status)
	return nil
}
