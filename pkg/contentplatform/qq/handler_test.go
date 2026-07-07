package qq

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	qqpkg "wx_channel/pkg/scraper/qq"
)

type fakeQQFetcher struct {
	key     string
	profile *qqpkg.TVProfile
}

func (f *fakeQQFetcher) FetchTVProfile(ctx context.Context, idOrURL string) (*qqpkg.TVProfile, error) {
	f.key = idOrURL
	return f.profile, nil
}

func TestProbeResolveQQJSON(t *testing.T) {
	rawURL := "https://v.qq.com/x/cover/mzc00200abcd123.html"
	fetcher := &fakeQQFetcher{profile: &qqpkg.TVProfile{
		Name:            "繁花",
		Overview:        "沪上故事",
		PosterPath:      "https://image.example/qq.jpg",
		NumberOfSeasons: 1,
	}}
	h := New(fetcher)
	if !h.Match(rawURL) {
		t.Fatalf("expected qq URL to match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: rawURL})
	if err != nil {
		t.Fatal(err)
	}
	if fetcher.key != "mzc00200abcd123" || probe.ContentID != "mzc00200abcd123" {
		t.Fatalf("unexpected id: fetch=%q probe=%q", fetcher.key, probe.ContentID)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: rawURL, Probe: probe})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Download.Protocol != "inline_json" || resolved.CanonicalURL != rawURL {
		t.Fatalf("unexpected resolved result: protocol=%q canonical=%q", resolved.Download.Protocol, resolved.CanonicalURL)
	}
}
