package soundgasm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	got, ok := ParseURL("https://soundgasm.net/u/BrittanyBabbles/Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy?x=1#top")
	if !ok {
		t.Fatal("expected soundgasm audio url match")
	}
	if got.Username != "BrittanyBabbles" || got.Slug != "Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy" {
		t.Fatalf("url parts = %#v", got)
	}
	if got.Canonical != "https://soundgasm.net/u/BrittanyBabbles/Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy" {
		t.Fatalf("canonical = %q", got.Canonical)
	}
	if _, ok := ParseURL("https://soundgasm.net/u/BrittanyBabbles"); ok {
		t.Fatal("unexpected match for profile url")
	}
	if _, ok := ParseURL("https://example.com/u/BrittanyBabbles/title"); ok {
		t.Fatal("unexpected match for other host")
	}
}

func TestParseAudioFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "soundgasm_260617.html"))
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("soundgasm_260617.html fixture not present")
		}
		t.Fatal(err)
	}
	page, err := ParseAudioHTML("https://soundgasm.net/u/BrittanyBabbles/Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy", string(body))
	if err != nil {
		t.Fatalf("ParseAudioHTML: %v", err)
	}
	if page.ID != "BrittanyBabbles_Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy" {
		t.Fatalf("id = %q", page.ID)
	}
	if page.Title != "Cum Quietly, Baby. We Can\u2019t Wake Up Daddy" {
		t.Fatalf("title = %q", page.Title)
	}
	if page.Author.Name != "BrittanyBabbles" || page.Author.URL != "https://soundgasm.net/u/BrittanyBabbles" {
		t.Fatalf("author = %#v", page.Author)
	}
	const mediaURL = "https://media.soundgasm.net/sounds/11c9b6c0b3bf5ed43264cfa7af46b33c4adef90a.m4a"
	if page.AudioURL != mediaURL || page.AudioType != "m4a" {
		t.Fatalf("audio url/type = %q %q", page.AudioURL, page.AudioType)
	}
	if !strings.Contains(page.Description, "start having nightmares") || !strings.Contains(page.DescriptionHTML, "brittanybabbles.com") {
		t.Fatalf("description text/html = %q / %q", page.Description, page.DescriptionHTML)
	}
	if !hasTag(page.Tags, "Fdom") || !hasTag(page.Tags, "Stealth Orgasm") {
		t.Fatalf("tags = %#v", page.Tags)
	}
	if !hasLink(page.Links, "https://www.brittanybabbles.com/") || !hasLink(page.Links, "https://www.patreon.com/brittanybabbles") {
		t.Fatalf("links = %#v", page.Links)
	}
	out := BuildHTML(page)
	for _, want := range []string{"<!doctype html>", "<audio controls", mediaURL, "BrittanyBabbles", "Stealth Orgasm"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered html missing %q: %s", want, out)
		}
	}
	for _, unwanted := range []string{"<script", "onclick="} {
		if strings.Contains(strings.ToLower(out), unwanted) {
			t.Fatalf("rendered html contains %q: %s", unwanted, out)
		}
	}
}

func TestParseAudioHTMLEscapedMediaURL(t *testing.T) {
	const htmlText = `<!doctype html>
<html><body>
<a href="/u/demo">demo</a>
<div class="jp-title">Sample Title</div>
<div class="jp-description"><p>hello <a href="/contact">link</a></p></div>
<script>$("#x").jPlayer({ready:function(){$(this).jPlayer("setMedia",{m4a:"https:\/\/media.soundgasm.net\/sounds\/demo.m4a"});}});</script>
</body></html>`
	page, err := ParseAudioHTML("https://soundgasm.net/u/demo/sample-title", htmlText)
	if err != nil {
		t.Fatalf("ParseAudioHTML: %v", err)
	}
	if page.AudioURL != "https://media.soundgasm.net/sounds/demo.m4a" || page.Title != "Sample Title" {
		t.Fatalf("page = %#v", page)
	}
	if len(page.Links) != 1 || page.Links[0].URL != "https://soundgasm.net/contact" {
		t.Fatalf("links = %#v", page.Links)
	}
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

func hasLink(links []Link, want string) bool {
	for _, link := range links {
		if link.URL == want {
			return true
		}
	}
	return false
}
