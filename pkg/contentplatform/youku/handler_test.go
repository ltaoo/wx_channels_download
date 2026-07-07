package youku

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	youkupkg "wx_channel/pkg/scraper/youku"
)

type fakeYoukuFetcher struct {
	rawURL  string
	profile *youkupkg.ProfileWithSeasons
}

func (f *fakeYoukuFetcher) FetchProfileWithSeasons(ctx context.Context, rawURL string) (*youkupkg.ProfileWithSeasons, error) {
	f.rawURL = rawURL
	return f.profile, nil
}

func TestProbeResolveYoukuJSON(t *testing.T) {
	rawURL := "https://v.youku.com/v_show/id_XNjQ0.html"
	fetcher := &fakeYoukuFetcher{profile: &youkupkg.ProfileWithSeasons{
		Type:       "season",
		ID:         "abcdef",
		Name:       "长安十二时辰",
		Overview:   "长安故事",
		PosterPath: "https://image.example/youku.jpg",
		Seasons:    []youkupkg.Season{{ID: "abcdef", Name: "正片"}},
	}}
	h := New(fetcher)
	if !h.Match(rawURL) {
		t.Fatalf("expected youku URL to match")
	}
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: rawURL})
	if err != nil {
		t.Fatal(err)
	}
	if fetcher.rawURL != rawURL || probe.ContentID != "abcdef" {
		t.Fatalf("unexpected id: fetch=%q probe=%q", fetcher.rawURL, probe.ContentID)
	}
	if author := contentdownload.ContentAuthor(probe.Content); author != "youku" {
		t.Fatalf("unexpected author %q", author)
	}
	if homepage := contentdownload.ContentAuthorHomepageURL(probe.Content); homepage != "https://www.youku.com/" {
		t.Fatalf("unexpected author homepage %q", homepage)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: rawURL, Probe: probe})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Download.Protocol != "inline_json" || resolved.Suffix != ".json" {
		t.Fatalf("unexpected resolved download: %+v suffix=%q", resolved.Download, resolved.Suffix)
	}
}
