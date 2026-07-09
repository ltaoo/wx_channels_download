package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"wx_channel/pkg/browsermgr"
	contentshuba69 "wx_channel/pkg/contentplatform/69shuba"
)

type shuba69BrowserPoolFetcher struct {
	mgr     *browsermgr.Manager
	timeout time.Duration
	wait    time.Duration
}

func newShuba69BrowserPoolFetcher(mgr *browsermgr.Manager, timeout time.Duration, wait time.Duration) *shuba69BrowserPoolFetcher {
	return &shuba69BrowserPoolFetcher{mgr: mgr, timeout: timeout, wait: wait}
}

func (f *shuba69BrowserPoolFetcher) BeginHTMLFetchSession() (contentshuba69.HTMLFetcher, func(error), error) {
	if f == nil || f.mgr == nil {
		return nil, nil, fmt.Errorf("browser sandbox manager not initialized")
	}
	lease, err := f.mgr.Acquire(context.Background())
	if err != nil {
		return nil, nil, err
	}
	session := f.newSession(lease)
	done := func(_ error) {
		opts := browsermgr.ReleaseOptions{}
		if session.fetchErr != nil {
			opts.Invalid = true
			opts.Error = session.fetchErr.Error()
		}
		_ = f.mgr.Release(lease.ID, opts)
	}
	return session, done, nil
}

func (f *shuba69BrowserPoolFetcher) FetchHTML(rawURL string, referer string, headers http.Header) (htmlText string, err error) {
	fetcher, done, err := f.BeginHTMLFetchSession()
	if err != nil {
		return "", err
	}
	if done != nil {
		defer func() { done(err) }()
	}
	return fetcher.FetchHTML(rawURL, referer, headers)
}

func (f *shuba69BrowserPoolFetcher) newSession(lease *browsermgr.Lease) *shuba69BrowserPoolSessionFetcher {
	cdpFetcher := contentshuba69.NewCDPFetcher(lease.CDPURL)
	if f.timeout > 0 {
		cdpFetcher.Timeout = f.timeout
	}
	if f.wait >= 0 {
		cdpFetcher.WaitAfterLoad = f.wait
	}
	return &shuba69BrowserPoolSessionFetcher{lease: lease, fetcher: cdpFetcher}
}

type shuba69BrowserPoolSessionFetcher struct {
	lease    *browsermgr.Lease
	fetcher  *contentshuba69.CDPFetcher
	fetchErr error
}

func (f *shuba69BrowserPoolSessionFetcher) FetchHTML(rawURL string, referer string, headers http.Header) (string, error) {
	htmlText, err := f.fetcher.FetchHTML(rawURL, referer, headers)
	if err != nil && f.fetchErr == nil {
		f.fetchErr = err
	}
	return htmlText, err
}
