package mgtv

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	mgtvpkg "wx_channel/pkg/scraper/mgtv"
)

type fakeMGTVFetcher struct {
	rawURL  string
	profile *mgtvpkg.TVProfile
}

func (f *fakeMGTVFetcher) FetchTVProfile(ctx context.Context, rawURL string) (*mgtvpkg.TVProfile, error) {
	f.rawURL = rawURL
	return f.profile, nil
}

func TestProbeResolveMGTVJSON(t *testing.T) {
	rawURL := "https://www.mgtv.com/b/123456/789.html"
	fetcher := &fakeMGTVFetcher{profile: &mgtvpkg.TVProfile{
		Name:            "声生不息",
		Overview:        "音乐节目",
		PosterPath:      "https://image.example/mgtv.jpg",
		NumberOfSeasons: 1,
	}}
	h := New(fetcher)
	if !h.Match(rawURL) {
		t.Fatalf("expected mgtv URL to match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: rawURL})
	if err != nil {
		t.Fatal(err)
	}
	if fetcher.rawURL != rawURL || probe.ContentID != "123456" {
		t.Fatalf("unexpected id: fetch=%q probe=%q", fetcher.rawURL, probe.ContentID)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: rawURL, Probe: probe})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Download.Protocol != "inline_json" || contentdownload.ContentType(resolved.Content) != "tv" {
		t.Fatalf("unexpected resolved result: protocol=%q type=%q", resolved.Download.Protocol, contentdownload.ContentType(resolved.Content))
	}
}
