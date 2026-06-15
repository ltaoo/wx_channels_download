package api

import (
	"errors"
	"testing"
	"time"

	"wx_channel/pkg/browsermgr"
)

func TestShuba69BrowserPoolFetcherLeasesAndReleasesIdleBrowser(t *testing.T) {
	mgr, err := browsermgr.New(browsermgr.Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.RegisterLocal(browsermgr.CreateRequest{Alias: "local", CDPURL: "http://127.0.0.1:9222"})
	if err != nil {
		t.Fatal(err)
	}
	fetcher := newShuba69BrowserPoolFetcher(mgr, 5*time.Second, time.Second)
	session, done, err := fetcher.BeginHTMLFetchSession()
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || done == nil {
		t.Fatalf("session nil=%t done nil=%t", session == nil, done == nil)
	}
	if got, _ := mgr.Get(rec.ID); got.Status != browsermgr.StatusBusy {
		t.Fatalf("status after begin = %q", got.Status)
	}
	done(nil)
	if got, _ := mgr.Get(rec.ID); got.Status != browsermgr.StatusIdle {
		t.Fatalf("status after done = %q", got.Status)
	}
}

func TestShuba69BrowserPoolFetcherMarksBrowserInvalidOnCDPFetchError(t *testing.T) {
	mgr, err := browsermgr.New(browsermgr.Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.RegisterLocal(browsermgr.CreateRequest{CDPURL: "http://127.0.0.1:9222"})
	if err != nil {
		t.Fatal(err)
	}
	fetcher := newShuba69BrowserPoolFetcher(mgr, 5*time.Second, time.Second)
	session, done, err := fetcher.BeginHTMLFetchSession()
	if err != nil {
		t.Fatal(err)
	}
	pooledSession, ok := session.(*shuba69BrowserPoolSessionFetcher)
	if !ok {
		t.Fatalf("session = %T", session)
	}
	pooledSession.fetchErr = errors.New("cdp unavailable")
	done(nil)
	got, _ := mgr.Get(rec.ID)
	if got.Status != browsermgr.StatusInvalid || got.Error != "cdp unavailable" {
		t.Fatalf("record = %#v", got)
	}
}

func TestShuba69BrowserPoolFetcherReturnsNoIdleBrowserError(t *testing.T) {
	mgr, err := browsermgr.New(browsermgr.Config{WorkDir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fetcher := newShuba69BrowserPoolFetcher(mgr, 0, 0)
	_, _, err = fetcher.BeginHTMLFetchSession()
	if err == nil {
		t.Fatal("expected no idle browser error")
	}
}
