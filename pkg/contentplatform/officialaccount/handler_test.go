package officialaccount

import (
	"context"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	officialaccountpkg "wx_channel/pkg/scraper/officialaccount"
)

type fakeArticleFetcher struct{}

func (fakeArticleFetcher) FetchArticle(url string) (*officialaccountpkg.WechatOfficialArticle, error) {
	return &officialaccountpkg.WechatOfficialArticle{
		Title:          "article",
		AuthorNickname: "author",
		AuthorID:       "author-id",
		AuthorAvatar:   "https://example.com/avatar.jpg",
		Content:        "<p>official body</p>",
		Images:         []string{"https://example.com/cover.jpg"},
		PageJSON:       &officialaccountpkg.CgiDataNew{Title: "article", UserName: "author-id"},
		PageHTML:       "<html>raw official page</html>",
	}, nil
}

func TestResolve(t *testing.T) {
	h := New(fakeArticleFetcher{})
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: "https://mp.weixin.qq.com/s/demo"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Platform != PlatformID {
		t.Fatalf("platform = %s", resolved.Platform)
	}
	if resolved.Download.Protocol != "officialaccount" {
		t.Fatalf("protocol = %s", resolved.Download.Protocol)
	}
	if resolved.Pipeline == nil || len(resolved.Pipeline.Nodes) == 0 {
		t.Fatal("expected pipeline plan")
	}
	if _, ok := resolved.Content.(*ArticleContentEnvelope); !ok {
		t.Fatalf("resolved content = %T, want *ArticleContentEnvelope", resolved.Content)
	}
}

func TestProbeReturnsArticleBodyOutput(t *testing.T) {
	h := New(fakeArticleFetcher{})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://mp.weixin.qq.com/s/demo"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if probe.Content == nil {
		t.Fatal("expected probe content")
	}
	envelope, ok := probe.Content.(*ArticleContentEnvelope)
	if !ok {
		t.Fatalf("probe content = %T, want *ArticleContentEnvelope", probe.Content)
	}
	if envelope.Metadata.ArticleID != "demo" || envelope.Metadata.AuthorID != "author-id" {
		t.Fatalf("article metadata = %#v", envelope.Metadata)
	}
	const authorHomepageURL = "https://mp.weixin.qq.com/mp/profile_ext?__biz=author-id&action=home&scene=124"
	if envelope.Metadata.AuthorAvatarURL != "https://example.com/avatar.jpg" ||
		envelope.Metadata.AuthorHomepageURL != authorHomepageURL {
		t.Fatalf("article author metadata = %#v", envelope.Metadata)
	}
	if envelope.Output.ArticleID != "demo" || envelope.Output.BodyHTML != "<p>official body</p>" {
		t.Fatalf("article output = %#v", envelope.Output)
	}
	if envelope.Output.AuthorAvatarURL != "https://example.com/avatar.jpg" ||
		envelope.Output.AuthorHomepageURL != authorHomepageURL {
		t.Fatalf("article author output = %#v", envelope.Output)
	}
	if contentdownload.ContentAuthorAvatarURL(probe.Content) != "https://example.com/avatar.jpg" {
		t.Fatalf("author avatar summary = %q", contentdownload.ContentAuthorAvatarURL(probe.Content))
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	if output["body_html"] != "<p>official body</p>" {
		t.Fatalf("body_html = %#v", output["body_html"])
	}
	if output["content_type"] != "article" {
		t.Fatalf("content_type = %#v", output["content_type"])
	}
	if output["article_id"] != "demo" {
		t.Fatalf("article_id = %#v", output["article_id"])
	}
	if output["author_avatar_url"] != "https://example.com/avatar.jpg" ||
		output["author_homepage_url"] != authorHomepageURL {
		t.Fatalf("author output = %#v", output)
	}
	if probe.Internal["pagejson"] == nil || probe.Internal["pagehtml"] != "<html>raw official page</html>" {
		t.Fatalf("probe internal page data = %#v", probe.Internal)
	}
}
