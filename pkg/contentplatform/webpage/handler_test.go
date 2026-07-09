package webpage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

func TestHandlerMatchHTTPURLs(t *testing.T) {
	h := New(nil)
	if !h.Match("https://example.com/post") || !h.Match("http://example.com/post") {
		t.Fatal("expected http urls to match")
	}
	if h.Match("ftp://example.com/file") || h.Match("not a url") {
		t.Fatal("unexpected non-http match")
	}
	if h.Match("https://example.com/video.mp4") || h.Match("https://finder.video.qq.com/stodownload?encfilekey=x") {
		t.Fatal("unexpected direct asset match")
	}
}

func TestProbeExtractsReadableArticle(t *testing.T) {
	const longParagraph = "这是一段足够长的正文内容，用于验证可读性提取会选中真正的文章主体，而不是页面导航、页脚、推荐阅读或分享按钮。正文需要保留链接和图片，并且移除脚本和无关的布局内容。"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html lang="zh-CN"><head>
<title>页面标题</title>
<meta name="description" content="页面摘要">
<meta name="author" content="作者甲">
<meta property="og:image" content="/cover.jpg">
<link rel="canonical" href="/article/1">
</head><body>
<nav>首页 导航 下载</nav>
<main>
  <article class="post-content">
    <h1>文章标题</h1>
    <p>` + longParagraph + `</p>
    <p>` + longParagraph + `</p>
    <p><a href="/more">相关阅读</a> ` + longParagraph + `</p>
    <img src="/image.jpg" alt="配图">
    <script>alert(1)</script>
  </article>
</main>
<footer>页脚内容</footer>
</body></html>`))
	}))
	defer server.Close()

	probe, err := New(nil).Probe(context.Background(), contentdownload.ProbeInput{URL: server.URL + "/post"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if probe.Platform != PlatformID || probe.ContentID == "" {
		t.Fatalf("probe identity = %#v", probe)
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	if summary.Type != ContentTypeArticle || summary.Title != "页面标题" || summary.Author != "作者甲" {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.CoverURL != server.URL+"/cover.jpg" || probe.CanonicalURL != server.URL+"/article/1" {
		t.Fatalf("urls summary=%#v canonical=%q", summary, probe.CanonicalURL)
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	bodyHTML, _ := output["body_html"].(string)
	if !strings.Contains(bodyHTML, longParagraph) || strings.Contains(bodyHTML, "首页 导航") || strings.Contains(bodyHTML, "页脚内容") {
		t.Fatalf("body_html did not contain extracted article only: %q", bodyHTML)
	}
	if strings.Contains(bodyHTML, "<script") || !strings.Contains(bodyHTML, `href="`+server.URL+`/more"`) {
		t.Fatalf("body_html was not sanitized/absolutified: %q", bodyHTML)
	}
}

func TestResolveUsesInlineHTML(t *testing.T) {
	const articleHTML = `<!doctype html><html><head><title>A/B</title></head><body><article><p>Readable body content that is long enough to pass extraction. Readable body content that is long enough to pass extraction. Readable body content that is long enough to pass extraction.</p></article></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(articleHTML))
	}))
	defer server.Close()

	h := New(nil)
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: server.URL})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: server.URL, Probe: probe})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Download.Protocol != "inline_html" || resolved.Suffix != ".html" {
		t.Fatalf("download = %#v suffix=%q", resolved.Download, resolved.Suffix)
	}
	if strings.Contains(resolved.Filename, "/") {
		t.Fatalf("filename was not sanitized: %q", resolved.Filename)
	}
	if resolved.Labels["content_type"] != ContentTypeArticle || resolved.Labels["mime_type"] != "text/html" {
		t.Fatalf("labels = %#v", resolved.Labels)
	}
	if body, _ := resolved.Metadata["body_html"].(string); !strings.Contains(body, "Readable body content") {
		t.Fatalf("resolved body_html = %q", body)
	}
}
