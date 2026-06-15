package v2ex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTopicFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "v2ex_260614.html"))
	if err != nil {
		t.Skip("v2ex fixture not present")
	}
	page, err := ParseTopicHTML("https://www.v2ex.com/t/1219463", string(body))
	if err != nil {
		t.Fatalf("ParseTopicHTML: %v", err)
	}
	if page.ID != "1219463" || page.CanonicalURL != "https://www.v2ex.com/t/1219463" {
		t.Fatalf("topic url fields = %#v", page)
	}
	if page.Title != "[记录贴] 思考独立开发者要怎么放大 AI 产出" {
		t.Fatalf("title = %q", page.Title)
	}
	if page.Author.Name != "hazellin549" || page.NodeName != "分享发现" {
		t.Fatalf("author/node = %#v %#v", page.Author, page.NodeName)
	}
	if !strings.Contains(page.ContentHTML, "稳定的商业化工作流") {
		t.Fatalf("content html = %q", page.ContentHTML)
	}
	if len(page.Replies) != 1 || page.Replies[0].Author.Name != "shoushen" {
		t.Fatalf("replies = %#v", page.Replies)
	}
	if page.ViewCount != 1344 || page.ReplyCount != 1 {
		t.Fatalf("stats view=%d replies=%d", page.ViewCount, page.ReplyCount)
	}
	out := BuildHTML(page)
	for _, want := range []string{"<!doctype html>", "稳定的商业化工作流", "shoushen", "https://www.v2ex.com/t/1219463"} {
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

func TestParseTopicURL(t *testing.T) {
	got, ok := ParseTopicURL("https://www.v2ex.com/t/1219463?p=2#reply1")
	if !ok {
		t.Fatal("expected topic url match")
	}
	if got.TopicID != "1219463" || got.Canonical != "https://www.v2ex.com/t/1219463" {
		t.Fatalf("topic url = %#v", got)
	}
	if _, ok := ParseTopicURL("https://www.example.com/t/1219463"); ok {
		t.Fatal("unexpected match for non-v2ex host")
	}
}
