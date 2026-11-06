package weiboadapter

import (
	"testing"

	"wx_channel/internal/database/model"
)

func TestPostTextAndGalleryDownloads(t *testing.T) {
	result := &FetchResult{
		ExternalID: "123", AuthorID: "author", AuthorName: "Author",
		BodyText: "正文😀", BodyHTML: "<p>正文😀</p>",
		Images: []Image{{URL: "https://image.example/1.jpg", Ext: "jpg"}, {URL: "https://image.example/2.jpg", Ext: "jpg"}},
	}
	task, err := (&handler{}).BuildDownloadTaskFromFetch(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.ContentDetails) != 2 {
		t.Fatalf("expected gallery and post details, got %+v", task.ContentDetails)
	}
	album, ok := task.ContentDetails[0].Data.(*model.ContentAlbum)
	if !ok || album.ImageCount != 2 || len(album.Images) != 2 || task.ContentDetail != album {
		t.Fatalf("gallery preview missing: %+v", task.ContentDetail)
	}
	article, ok := task.ContentDetails[1].Data.(*model.ContentArticle)
	if !ok || article.Type != model.ContentArticleTypeHTML || article.HTML != result.BodyHTML || article.Text != result.BodyText || article.WordCount != 3 {
		t.Fatalf("post text or HTML lost: %+v", task.ContentDetails[1].Data)
	}
	if task.ContentDetails[0].Key == task.ContentDetails[1].Key || len(task.Resources) != 3 {
		t.Fatalf("expected distinct detail keys and HTML plus two image resources: %+v", task)
	}
	for image_index, image := range album.Images {
		resource := task.Resources[image_index+1]
		if resource.Endpoints[0].URL != image.URL || len(resource.ContentAssets) != 1 {
			t.Fatalf("gallery image %d missing its downloaded resource", image_index)
		}
		asset := resource.ContentAssets[0]
		if asset.SubjectType != model.ContentAssetSubjectAlbumImage || asset.SubjectKey != image.ImageKey || asset.Role != model.ContentAssetRolePrimary {
			t.Fatalf("gallery image %d is not linked to its downloaded file: %+v", image_index, asset)
		}
	}
}

func TestTextOnlyPostHasDetails(t *testing.T) {
	result := &FetchResult{ExternalID: "456", AuthorID: "author", BodyText: "正文"}
	task, err := (&handler{}).BuildDownloadTaskFromFetch(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.ContentDetails) != 1 {
		t.Fatalf("text-only post has no detail record: %+v", task.ContentDetails)
	}
	article, ok := task.ContentDetail.(*model.ContentArticle)
	if !ok || article.Type != model.ContentArticleTypeText || article.Text != result.BodyText {
		t.Fatalf("text-only preview missing: %+v", task.ContentDetail)
	}
}
