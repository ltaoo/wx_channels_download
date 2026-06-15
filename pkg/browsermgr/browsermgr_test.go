package browsermgr

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCreateDockerBuildsRemoteDebugContainerCommand(t *testing.T) {
	runner := &fakeRunner{output: "container-id"}
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
	rec, err := mgr.CreateDocker(context.Background(), CreateRequest{Alias: "cf-browser", CDPHostPort: 40123, DesktopHostPort: 30123})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Endpoint == nil || rec.Endpoint.CDPURL != "http://127.0.0.1:40123" {
		t.Fatalf("endpoint = %#v", rec.Endpoint)
	}
	if rec.Endpoint.DesktopURL != "http://127.0.0.1:30123" || rec.Endpoint.DesktopHostPort != 30123 {
		t.Fatalf("desktop endpoint = %#v", rec.Endpoint)
	}
	if rec.Status != StatusIdle {
		t.Fatalf("status = %q", rec.Status)
	}
	if runner.name != "docker" || len(runner.args) == 0 {
		t.Fatalf("runner = %#v", runner)
	}
	joined := strings.Join(runner.args, " ")
	for _, want := range []string{
		"run -d",
		"--name wx-browser-",
		"-p 40123:9222",
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

func TestRegisterLocalBrowser(t *testing.T) {
	mgr, err := New(Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.RegisterLocal(CreateRequest{Alias: "local", CDPURL: "http://127.0.0.1:9222/"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Kind != KindLocal || rec.Endpoint == nil || rec.Endpoint.CDPURL != "http://127.0.0.1:9222" {
		t.Fatalf("record = %#v", rec)
	}
	if rec.Status != StatusIdle {
		t.Fatalf("status = %q", rec.Status)
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
	if got, _ := mgr.Get(rec.ID); got.Status != StatusIdle {
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

type fakeRunner struct {
	name   string
	args   []string
	output string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return r.output, nil
}
