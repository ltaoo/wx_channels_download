package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchTV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/tv" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("api_key"); got != "token" {
			t.Fatalf("api_key = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"page":1,
			"total_results":1,
			"results":[{
				"id":130330,
				"name":"华灯初上",
				"original_name":"華燈初上",
				"overview":"desc",
				"poster_path":"/poster.jpg",
				"backdrop_path":"/backdrop.jpg",
				"first_air_date":"2021-11-26",
				"origin_country":["TW"],
				"popularity":10.5,
				"vote_average":8.2
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(WithHostname(server.URL), WithToken("token"))
	result, err := client.SearchTV(context.Background(), "hua", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.List) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.List))
	}
	item := result.List[0]
	if item.ID != 130330 || item.Name != "华灯初上" || item.Source != "tmdb" {
		t.Fatalf("unexpected item %#v", item)
	}
	if item.PosterPath != "https://www.themoviedb.org/t/p/w600_and_h900_bestv2/poster.jpg" {
		t.Fatalf("unexpected poster %q", item.PosterPath)
	}
}

func TestFetchSeasonProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1/season/2" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":22,
			"name":"Season 2",
			"overview":"season desc",
			"air_date":"2022-01-01",
			"poster_path":"/season.jpg",
			"season_number":2,
			"episodes":[{
				"id":100,
				"name":"Episode 1",
				"overview":"episode desc",
				"air_date":"2022-01-01",
				"episode_number":1,
				"season_number":2,
				"runtime":45,
				"still_path":"/still.jpg"
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(WithHostname(server.URL), WithToken("token"))
	season, err := client.FetchSeasonProfile(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if season == nil || season.ID != 22 || len(season.Episodes) != 1 {
		t.Fatalf("unexpected season %#v", season)
	}
	if season.Episodes[0].StillPath != "https://www.themoviedb.org/t/p/w227_and_h127_bestv2/still.jpg" {
		t.Fatalf("unexpected still path %q", season.Episodes[0].StillPath)
	}
}
