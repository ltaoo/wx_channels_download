package iqiyi

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	iqiyipkg "wx_channel/pkg/scraper/iqiyi"
)

type fakeIQiyiFetcher struct {
	rawURL  string
	profile *iqiyipkg.ProfileWithSeasons
}

func (f *fakeIQiyiFetcher) FetchProfileWithSeasons(ctx context.Context, rawURL string) (*iqiyipkg.ProfileWithSeasons, error) {
	f.rawURL = rawURL
	return f.profile, nil
}

func TestProbeResolveIQiyiJSON(t *testing.T) {
	rawURL := "https://www.iqiyi.com/v_19rr7p8abc.html"
	fetcher := &fakeIQiyiFetcher{profile: &iqiyipkg.ProfileWithSeasons{
		Type:       "season",
		ID:         1001,
		Name:       "狂飙",
		Overview:   "刑侦故事",
		PosterPath: "https://image.example/iqiyi.jpg",
		Seasons:    []iqiyipkg.Season{{ID: 1001, Name: "正片"}},
	}}
	h := New(fetcher)
	if !h.Match(rawURL) {
		t.Fatalf("expected iqiyi URL to match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: rawURL})
	if err != nil {
		t.Fatal(err)
	}
	if fetcher.rawURL != rawURL || probe.ContentID != "1001" {
		t.Fatalf("unexpected id: fetch=%q probe=%q", fetcher.rawURL, probe.ContentID)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: rawURL, Probe: probe})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Download.Protocol != "inline_json" || contentdownload.ContentTitle(resolved.Content) != "狂飙" {
		t.Fatalf("unexpected resolved result: protocol=%q title=%q", resolved.Download.Protocol, contentdownload.ContentTitle(resolved.Content))
	}
}
