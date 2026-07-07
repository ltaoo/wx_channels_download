package tmdb

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	tmdbpkg "wx_channel/pkg/scraper/tmdb"
)

type fakeTMDBFetcher struct {
	tvID int
}

func (f *fakeTMDBFetcher) FetchTVProfile(ctx context.Context, id int) (*tmdbpkg.TVProfile, error) {
	f.tvID = id
	return &tmdbpkg.TVProfile{
		ID:               id,
		Name:             "Friends",
		Overview:         "Six friends in New York.",
		PosterPath:       "https://image.example/poster.jpg",
		NumberOfSeasons:  10,
		NumberOfEpisodes: 236,
	}, nil
}

func (f *fakeTMDBFetcher) FetchMovieProfile(ctx context.Context, id int) (*tmdbpkg.MovieProfile, error) {
	return &tmdbpkg.MovieProfile{ID: id, Name: "Movie"}, nil
}

func (f *fakeTMDBFetcher) FetchSeasonProfile(ctx context.Context, tvID int, seasonNumber int) (*tmdbpkg.SeasonProfile, error) {
	return &tmdbpkg.SeasonProfile{ID: seasonNumber, Name: "Season"}, nil
}

func (f *fakeTMDBFetcher) FetchEpisodeProfile(ctx context.Context, tvID int, seasonNumber int, episodeNumber int) (*tmdbpkg.EpisodeProfile, error) {
	return &tmdbpkg.EpisodeProfile{ID: episodeNumber, Name: "Episode"}, nil
}

func TestMatchTMDBURLs(t *testing.T) {
	h := New(&fakeTMDBFetcher{})
	for _, rawURL := range []string{
		"https://www.themoviedb.org/tv/1668-friends",
		"https://www.themoviedb.org/movie/13-forrest-gump",
		"https://www.themoviedb.org/tv/1668/season/1",
		"https://www.themoviedb.org/tv/1668/season/1/episode/2",
	} {
		if !h.Match(rawURL) {
			t.Fatalf("expected %s to match", rawURL)
		}
	}
	if h.Match("https://movie.douban.com/subject/1393859/") {
		t.Fatalf("expected douban URL to be ignored")
	}
}

func TestProbeResolveTMDBJSON(t *testing.T) {
	fetcher := &fakeTMDBFetcher{}
	h := New(fetcher)
	rawURL := "https://www.themoviedb.org/tv/1668-friends"
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: rawURL})
	if err != nil {
		t.Fatal(err)
	}
	if fetcher.tvID != 1668 || probe.ContentID != "tv_1668" {
		t.Fatalf("unexpected id: fetch=%d probe=%q", fetcher.tvID, probe.ContentID)
	}
	if contentdownload.ContentType(probe.Content) != "tv" {
		t.Fatalf("unexpected content type: %q", contentdownload.ContentType(probe.Content))
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: rawURL, Probe: probe})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Download.Protocol != "inline_json" || resolved.Suffix != ".json" {
		t.Fatalf("unexpected resolved download: %+v suffix=%q", resolved.Download, resolved.Suffix)
	}
}
