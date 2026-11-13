package xadapter

import (
	"testing"

	"wx_channel/internal/database/model"
	x_scraper "wx_channel/pkg/scraper/x"
)

func TestArticleHTMLDetailsAndDownload(t *testing.T) {
	body_html := `<article data-n="x-article"><h1 data-n="article-title">Article title</h1><img data-n="article-image" src="https://pbs.twimg.com/media/photo.jpg" alt=""></article>`
	result := &x_scraper.FetchResult{
		ExternalID: "123",
		AuthorName: "Author",
		Article:    &x_scraper.Article{Title: "Article title", HTML: body_html},
	}
	h := &handler{}
	content, err := h.ToContent(result)
	if err != nil {
		t.Fatal(err)
	}
	if content.Title != result.Article.Title {
		t.Errorf("got title %q, want article title", content.Title)
	}
	details, err := h.ToContentDetails(result)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 1 {
		t.Fatalf("got %d details, want 1", len(details))
	}
	article, ok := details[0].Data.(*model.ContentArticle)
	if !ok || article.Type != model.ContentArticleTypeHTML || article.HTML != body_html {
		t.Fatalf("article HTML lost in details: %+v", details[0].Data)
	}
	task, err := h.BuildDownloadTaskFromFetch(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Resources) != 1 {
		t.Fatalf("got %d download resources, want 1", len(task.Resources))
	}
	resource := task.Resources[0]
	if resource.Resource.Kind != "text/html" || resource.Resource.UniqueID != "123_html" || resource.Resource.Size != int64(len(body_html)) {
		t.Errorf("unexpected HTML resource: %+v", resource.Resource)
	}
	if resource.Endpoints[0].Protocol != "inline" || resource.Endpoints[0].URL != body_html || resource.ContentAssets[0].AssetKey != "body:html" {
		t.Errorf("download lost article HTML: %+v", resource)
	}
}

func TestOrdinaryPostRemainsText(t *testing.T) {
	result := &x_scraper.FetchResult{ExternalID: "456", BodyText: "Ordinary post"}
	task, err := (&handler{}).BuildDownloadTaskFromFetch(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	article, ok := task.ContentDetail.(*model.ContentArticle)
	if !ok || article.Type != model.ContentArticleTypeText || article.Text != result.BodyText {
		t.Errorf("unexpected ordinary post detail: %+v", task.ContentDetail)
	}
	if task.Content.Title != result.BodyText || len(task.Resources) != 1 || task.Resources[0].Resource.Kind != "text/plain" || task.Resources[0].Endpoints[0].URL != result.BodyText {
		t.Errorf("ordinary post changed: %+v", task)
	}
}
