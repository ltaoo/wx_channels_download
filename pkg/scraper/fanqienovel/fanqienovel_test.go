package fanqienovel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseInitialStateFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "fanqienovel_260614.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("fanqienovel_260614.html fixture not present")
		}
		t.Fatal(err)
	}
	raw, err := ExtractInitialStateJSON(body)
	if err != nil {
		t.Fatalf("ExtractInitialStateJSON: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("initial state is invalid json: %s", raw)
	}
	profile, err := ParseBookProfileHTML("https://fanqienovel.com/page/7069948840148732967", string(body))
	if err != nil {
		t.Fatalf("ParseBookProfileHTML: %v", err)
	}
	if profile.Title != "部族荣光" || profile.Author.Name != "丧狐" {
		t.Fatalf("profile title/author = %#v", profile)
	}
	if profile.ChapterCount != 351 {
		t.Fatalf("chapter count = %d", profile.ChapterCount)
	}
	if len(profile.Volumes) != 1 || len(profile.Volumes[0].Chapters) == 0 {
		t.Fatalf("volumes = %#v", profile.Volumes)
	}
	if got := profile.Volumes[0].Chapters[0]; got.ID != "7535344092251685400" || !strings.Contains(got.Title, "第351章") {
		t.Fatalf("first chapter = %#v", got)
	}
	if profile.LatestChapter.ID != "7535344092251685400" || !strings.Contains(profile.LatestChapter.URL, "/reader/7535344092251685400") {
		t.Fatalf("latest chapter = %#v", profile.LatestChapter)
	}
	if len(profile.Tags) == 0 || profile.Tags[0] != "传统玄幻" {
		t.Fatalf("tags = %#v", profile.Tags)
	}
}

func TestInitialStateUndefinedNormalization(t *testing.T) {
	body := []byte(`<script>window.__INITIAL_STATE__ = {"page":{"bookId":"1","bookName":"literal undefined text","chapterList":[{"itemId":"2","title":"chapter","order":1,},],"value":undefined,},}</script>`)
	raw, err := ExtractInitialStateJSON(body)
	if err != nil {
		t.Fatalf("ExtractInitialStateJSON: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("raw is invalid json: %s", raw)
	}
	if !strings.Contains(string(raw), `"literal undefined text"`) {
		t.Fatalf("string literal was changed: %s", raw)
	}
	if strings.Contains(string(raw), ":undefined") {
		t.Fatalf("undefined was not normalized: %s", raw)
	}
	profile, err := ParseBookProfileHTML("https://fanqienovel.com/page/1", string(body))
	if err != nil {
		t.Fatalf("ParseBookProfileHTML: %v", err)
	}
	if profile.Title != "literal undefined text" || len(profile.Volumes) != 1 {
		t.Fatalf("profile = %#v", profile)
	}
}
