package xiaohongshuadapter

import (
	"encoding/json"
	"net/url"
	"testing"

	"wx_channel/pkg/scraper/xiaohongshu"
)

func TestHomeContentsUseNoteLinks(t *testing.T) {
	const note_id = "68abcdef0000000001234567"
	const profile_url = "https://www.xiaohongshu.com/user/profile/687ca6da000000000d02e967?xsec_token=profile-token"
	for _, item_url := range []string{"", profile_url, "https://untrusted.example/explore/" + note_id} {
		item := xiaohongshu.HomeNoteItem{ID: note_id, URL: item_url, XSecToken: "note+token/&", NoteCard: xiaohongshu.HomeNoteCard{DisplayTitle: "Note"}}
		contents := xiaohongshu_home_contents([]xiaohongshu.HomeNoteItem{item})
		if len(contents) != 1 {
			t.Fatalf("expected one note, got %d", len(contents))
		}
		content := contents[0]
		parsed_url, err := url.Parse(content.URL)
		if err != nil || parsed_url.Host != "www.xiaohongshu.com" || parsed_url.Path != "/explore/"+note_id ||
			parsed_url.Query().Get("xsec_token") != item.XSecToken || parsed_url.Query().Get("xsec_source") != "pc_user" || content.SourceURL != content.URL {
			t.Fatalf("incorrect note links: url=%q source_url=%q", content.URL, content.SourceURL)
		}
	}
	redacted := xiaohongshu.HomeNoteItem{URL: profile_url, NoteCard: xiaohongshu.HomeNoteCard{DisplayTitle: "Hidden ID"}}
	contents := xiaohongshu_home_contents([]xiaohongshu.HomeNoteItem{redacted})
	if len(contents) != 1 || contents[0].Title != "Hidden ID" || contents[0].URL != "" || contents[0].SourceURL != "" {
		t.Fatalf("a redacted card must remain visible with empty links: %+v", contents)
	}
	encoded, err := json.Marshal(contents[0])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["url"] != "" || fields["source_url"] != "" {
		t.Fatal("redacted note JSON must include empty url and source_url fields")
	}
}

func TestRedactedHomeCardsKeepStableDistinctIDs(t *testing.T) {
	first := xiaohongshu.HomeNoteItem{Index: 0, NoteCard: xiaohongshu.HomeNoteCard{DisplayTitle: "First", Cover: xiaohongshu.Image{URL: "https://media.example/first.jpg"}}}
	second := xiaohongshu.HomeNoteItem{Index: 1, NoteCard: xiaohongshu.HomeNoteCard{DisplayTitle: "Second", Type: "video"}}
	contents := xiaohongshu_home_contents([]xiaohongshu.HomeNoteItem{first, second, first})
	if len(contents) != 2 || contents[0].Id == contents[1].Id || contents[0].Id == "" || contents[1].Id == "" ||
		contents[0].CoverURL != first.NoteCard.Cover.URL || contents[1].Type != "video" {
		t.Fatalf("redacted card identity or metadata lost: %+v", contents)
	}
	for _, content := range contents {
		if content.URL != "" || content.SourceURL != "" {
			t.Fatal("synthetic card IDs must not be turned into note URLs")
		}
	}
	if repeated := xiaohongshu_home_contents([]xiaohongshu.HomeNoteItem{first}); len(repeated) != 1 || repeated[0].Id != contents[0].Id {
		t.Fatal("redacted card IDs must be stable across fetches")
	}
}
