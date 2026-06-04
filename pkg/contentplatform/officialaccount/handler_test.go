package officialaccount

import (
	"context"
	"testing"

	officialaccountdownload "github.com/GopeedLab/gopeed/pkg/officialaccount"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

type fakeArticleFetcher struct{}

func (fakeArticleFetcher) FetchArticle(url string) (*officialaccountdownload.WechatOfficialArticle, error) {
	return &officialaccountdownload.WechatOfficialArticle{
		Title:          "article",
		AuthorNickname: "author",
		Content:        "<p>official body</p>",
		Images:         []string{"https://example.com/cover.jpg"},
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
	output := contentdownload.ContentOutputOf(probe.Content)
	if output["body_html"] != "<p>official body</p>" {
		t.Fatalf("body_html = %#v", output["body_html"])
	}
	if output["content_type"] != "article" {
		t.Fatalf("content_type = %#v", output["content_type"])
	}
}
