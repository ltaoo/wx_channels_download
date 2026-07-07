package qq

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestParseTVProfileHTML(t *testing.T) {
	html := `<script>window.__PINIA__ = {global:{coverInfo:{title:"剧名",description:"简介"}},episodeMain:{epTabs:[{text:"第一季",pageContext:"cid1"}]}}</script><`
	got, err := ParseTVProfileHTML("cid", html)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "剧名" || got.NumberOfSeasons != 1 || got.Seasons[0].ID != "cid1" {
		t.Fatalf("unexpected profile %#v", got)
	}
}

func TestParseSeasonProfileHTML(t *testing.T) {
	html := `<script>window.__PINIA__ = {global:{coverInfo:{cover_id:"cid",title:"剧名",description:"简介",publish_date:"2023-01-01",episode_all:"2",type_name:"电视剧",area_name:"大陆"}},episodeMain:{listData:[{list:[[{vid:"v1",title:"第一集",pic:"p.jpg",publishDate:"2023-01-01",index:1,duration:120}]]}]}}</script><`
	got, err := ParseSeasonProfileHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "cid" || got.NumberOfEpisode != 2 || len(got.Episodes) != 1 || got.OriginCountry[0] != "CN" {
		t.Fatalf("unexpected season %#v", got)
	}
}

func TestParseVideoPageURL(t *testing.T) {
	got, ok := ParseVideoPageURL("https://v.qq.com/x/cover/mzc00200whxf2zp/k41026bh3p0.html")
	if !ok {
		t.Fatal("expected URL to match")
	}
	if got.CID != "mzc00200whxf2zp" || got.VID != "k41026bh3p0" {
		t.Fatalf("unexpected ids: %#v", got)
	}
	if got.Canonical != "https://v.qq.com/x/cover/mzc00200whxf2zp/k41026bh3p0.html" {
		t.Fatalf("unexpected canonical URL: %s", got.Canonical)
	}
}

func TestParseVideoDetailPageGetPageFixture(t *testing.T) {
	fixture, err := os.ReadFile("../../../scraper_examples/vqq/260619/getpage.md")
	if err != nil {
		t.Fatal(err)
	}
	var resp pageServiceResponse
	if err := json.Unmarshal(extractJSONCodeBlock(t, string(fixture)), &resp); err != nil {
		t.Fatal(err)
	}
	pageURL, ok := ParseVideoPageURL("https://v.qq.com/x/cover/mzc00200whxf2zp/k41026bh3p0.html")
	if !ok {
		t.Fatal("expected URL to match")
	}
	got, err := parseVideoDetailPageResponse(pageURL, "fixture://getpage", resp)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "问心2" || got.CID != "mzc00200whxf2zp" || got.VID != "k41026bh3p0" {
		t.Fatalf("unexpected detail: %#v", got)
	}
	if got.EpisodeAll != 40 || len(got.Episodes) < 4 {
		t.Fatalf("unexpected episode data: all=%d parsed=%d", got.EpisodeAll, len(got.Episodes))
	}
	if got.Episodes[0].VID != "k41026bh3p0" {
		t.Fatalf("unexpected first episode: %#v", got.Episodes[0])
	}
	if got.CurrentEpisode == nil || got.CurrentEpisode.PlayTitle != "问心2 第01集" {
		t.Fatalf("unexpected current episode: %#v", got.CurrentEpisode)
	}
	if got.Score != "9.7分" || got.CoverURL == "" {
		t.Fatalf("unexpected score/cover: score=%q cover=%q", got.Score, got.CoverURL)
	}
}

func extractJSONCodeBlock(t *testing.T, content string) []byte {
	t.Helper()
	start := strings.Index(content, "```json")
	if start < 0 {
		t.Fatal("missing json code block")
	}
	start += len("```json")
	end := strings.Index(content[start:], "```")
	if end < 0 {
		t.Fatal("missing json code block terminator")
	}
	return []byte(strings.TrimSpace(content[start : start+end]))
}
