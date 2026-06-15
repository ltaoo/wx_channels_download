package xiaohongshu

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseNoteURL(t *testing.T) {
	got, ok := ParseNoteURL("https://www.xiaohongshu.com/explore/6455109d00000000270293b5?xsec_token=abc")
	if !ok {
		t.Fatal("ParseNoteURL returned false")
	}
	if got.NoteID != "6455109d00000000270293b5" || got.Canonical != "https://www.xiaohongshu.com/explore/6455109d00000000270293b5" || got.XSecToken != "abc" {
		t.Fatalf("ParseNoteURL = %#v", got)
	}

	got, ok = ParseNoteURL("https://www.xiaohongshu.com/discovery/item/6455109d00000000270293b5")
	if !ok || got.NoteID != "6455109d00000000270293b5" {
		t.Fatalf("Parse discovery item = %#v ok=%v", got, ok)
	}
}

func TestParseInitialStateFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "xiaohongshu_260614.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("xiaohongshu_260614.html fixture not present")
		}
		t.Fatal(err)
	}
	page, err := ParseNotePage(body, NoteURL{NoteID: "6455109d00000000270293b5"})
	if err != nil {
		t.Fatalf("ParseNotePage: %v", err)
	}
	if !json.Valid(page.InitialStateJSON) {
		t.Fatal("InitialStateJSON is not valid JSON")
	}
	if page.Note.NoteID != "6455109d00000000270293b5" {
		t.Fatalf("note id = %q", page.Note.NoteID)
	}
	if !strings.Contains(page.Note.Title, "一衣两穿") {
		t.Fatalf("title = %q", page.Note.Title)
	}
	if page.Note.User.Nickname != "白白Crystal" {
		t.Fatalf("nickname = %q", page.Note.User.Nickname)
	}
	stream, ok := page.Note.BestVideoStream()
	if !ok || !strings.Contains(stream.MasterURL, "sns-video-v2.xhscdn.com") {
		t.Fatalf("stream = %#v ok=%v", stream, ok)
	}
	if stream.Width != 720 || stream.Height != 1280 || stream.Size != 1558948 {
		t.Fatalf("stream metadata = %#v", stream)
	}
	images := page.Note.ImageURLs()
	if len(images) != 1 || !strings.Contains(images[0], "sns-webpic-qc.xhscdn.com") {
		t.Fatalf("image urls = %#v", images)
	}
}

func TestInitialStateUndefinedNormalization(t *testing.T) {
	body := []byte(`<script>window.__INITIAL_STATE__ = {"note":{"firstNoteId":"1","noteDetailMap":{"1":{"note":{"noteId":"1","title":"literal undefined text","desc":"","user":{"nickname":"author"},"type":"normal","imageList":[]}}}},"value":undefined,"list":[1,],}</script>`)
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
	state, err := ParseInitialState(body)
	if err != nil {
		t.Fatalf("ParseInitialState: %v", err)
	}
	note, ok := NoteFromInitialState(state, "1")
	if !ok || note.Title != "literal undefined text" {
		t.Fatalf("note = %#v ok=%v", note, ok)
	}
}
