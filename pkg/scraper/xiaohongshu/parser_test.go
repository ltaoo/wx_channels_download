package xiaohongshu

import "testing"

func TestHomeCardLinks(t *testing.T) {
	const note_id = "68abcdef0000000001234567"
	for _, test_case := range []struct {
		name    string
		href    string
		want_id string
	}{
		{"note_without_ld_class", "/user/profile/687ca6da000000000d02e967/" + note_id + "?xsec_token=note-token&xsec_source=pc_user", note_id},
		{"explore", "/explore/" + note_id + "?xsec_token=note-token", note_id},
		{"profile_only", "/user/profile/687ca6da000000000d02e967/?xsec_token=profile-token", ""},
		{"empty_note", "/explore/", ""},
		{"foreign_host", "https://untrusted.example/explore/" + note_id, ""},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			document_html := `<section class="note-item" data-index="0"><a class="cover mask" href="` + test_case.href + `"></a></section>` +
				`<script>window.__INITIAL_STATE__={"user":{"activeTab":{"index":0,"query":"note"},"notes":[[{"index":0,"id":"","noteCard":{"noteId":"","displayTitle":"Note"}}]]}};</script>`
			result, err := ParseHomeContents(document_html, "notes")
			if err != nil {
				t.Fatal(err)
			}
			item := result.Items[0]
			if item.ID != test_case.want_id || item.NoteCard.NoteID != test_case.want_id || result.Redacted != (test_case.want_id == "") {
				t.Fatalf("incorrect note identity: %+v redacted=%t", item, result.Redacted)
			}
			if test_case.want_id == "" {
				if item.URL != "" || item.XSecToken != "" {
					t.Fatal("non-note links must not become note URLs or supply note tokens")
				}
			} else if item.URL != "https://www.xiaohongshu.com"+test_case.href || item.XSecToken != "note-token" || item.NoteCard.XSecToken != "note-token" {
				t.Fatalf("lost note link or token: %+v", item)
			}
		})
	}
}

func TestHomeCardLinkMustMatchNoteID(t *testing.T) {
	const note_id = "68abcdef0000000001234567"
	items := []HomeNoteItem{{Index: 0, ID: note_id, XSecToken: "original-token"}}
	merge_home_card_links(`<section class="note-item" data-index="0"><a class="cover mask ld" href="/explore/68abcdef0000000001234568?xsec_token=wrong-token"></a></section>`, items)
	if items[0].URL != "" || items[0].ID != note_id || items[0].XSecToken != "original-token" {
		t.Fatal("a link for a different note must not overwrite the note identity or token")
	}
}
