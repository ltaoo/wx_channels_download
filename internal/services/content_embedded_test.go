package services

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

func TestContentDetailAggregatesDirectEmbeddedMedia(t *testing.T) {
	db := new_content_service_test_db(t)
	service := NewContentService(db)
	now := int64(1_700_000_000_000)
	article_id := "wxmp:article"
	video_id := "wxmp:video"
	related_article_id := "wxmp:related-article"

	contents := []model.Content{
		{
			Id:         article_id,
			PlatformId: "wxmp",
			Type:       model.ContentTypeArticle,
			ExternalId: "article",
			Title:      "带视频的文章",
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		{
			Id:         video_id,
			PlatformId: "wxmp",
			Type:       model.ContentTypeVideo,
			ExternalId: "video",
			Title:      "文章内嵌视频",
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		{
			Id:         related_article_id,
			PlatformId: "wxmp",
			Type:       model.ContentTypeArticle,
			ExternalId: "related-article",
			Title:      "被包含的另一篇文章",
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
	}
	if err := db.Create(&contents).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ContentArticle{
		Id:   article_id,
		Type: model.ContentArticleTypeHTML,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := save_content_video(db, &model.ContentVideo{
		Id:     video_id,
		Format: "mp4",
		URL:    "https://example.com/video.mp4",
		Variants: []model.ContentVideoVariant{
			{
				VideoId:    video_id,
				VariantKey: "video:format:10002",
				Spec:       "10002",
				Quality:    "超清",
				Format:     "mp4",
				StreamType: model.ContentVideoVariantStreamTypeProgressive,
				HasVideo:   1,
				HasAudio:   1,
				IsDefault:  1,
				URL:        "https://example.com/video.mp4",
			},
			{
				VideoId:    video_id,
				VariantKey: "video:format:10004",
				Spec:       "10004",
				Quality:    "流畅",
				Format:     "mp4",
				StreamType: model.ContentVideoVariantStreamTypeProgressive,
				HasVideo:   1,
				HasAudio:   1,
				URL:        "https://example.com/video-low.mp4",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	relations := []model.ContentRelation{
		{
			SourceContentId: article_id,
			TargetContentId: video_id,
			Type:            model.ContentRelationContains,
			SortOrder:       1,
			CreatedAt:       now,
		},
		{
			SourceContentId: article_id,
			TargetContentId: related_article_id,
			Type:            model.ContentRelationContains,
			SortOrder:       2,
			CreatedAt:       now,
		},
	}
	if err := db.Create(&relations).Error; err != nil {
		t.Fatal(err)
	}

	resources := []model.DownloadResource{
		{
			ContentId:   &article_id,
			DownloadDir: "/tmp/article",
			Name:        "article.html",
			Kind:        "text/html",
			UniqueID:    "article-html",
			Type:        "file",
			Status:      2,
			Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		{
			ContentId:   &video_id,
			DownloadDir: "/tmp/article",
			Name:        "video.mp4",
			Kind:        "video/mp4",
			UniqueID:    "article-video",
			Type:        "file",
			Status:      2,
			Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		{
			ContentId:   &related_article_id,
			DownloadDir: "/tmp/article",
			Name:        "related.html",
			Kind:        "text/html",
			UniqueID:    "related-html",
			Type:        "file",
			Status:      2,
			Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatal(err)
	}
	var selected_variant model.ContentVideoVariant
	if err := db.Where("video_id = ? AND is_default <> 0", video_id).First(&selected_variant).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DownloadResourceAsset{
		ResourceId: resources[1].Id,
		AssetId:    selected_variant.AssetId,
		Relation:   model.DownloadResourceAssetRelationSource,
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := service.GetContentDetail(article_id)
	if err != nil {
		t.Fatal(err)
	}
	if detail.FileCount != 2 {
		t.Fatalf("FileCount = %d, want 2", detail.FileCount)
	}
	if len(detail.Resources) != 2 {
		t.Fatalf("resource count = %d, want 2", len(detail.Resources))
	}
	resource_names := make(map[string]bool, len(detail.Resources))
	for _, resource := range detail.Resources {
		resource_names[resource.Name] = true
	}
	if !resource_names["article.html"] || !resource_names["video.mp4"] {
		t.Fatalf("resources = %#v, want article.html and video.mp4", resource_names)
	}
	if resource_names["related.html"] {
		t.Fatal("non-media contained article was aggregated as an article file")
	}
	if len(detail.EmbeddedContents) != 1 {
		t.Fatalf("embedded content count = %d, want 1", len(detail.EmbeddedContents))
	}
	embedded := detail.EmbeddedContents[0]
	if embedded.Content.Id != video_id || embedded.DetailType != "content_video" {
		t.Fatalf("embedded content = %#v", embedded)
	}
	video, ok := embedded.Detail.(*model.ContentVideo)
	if !ok {
		t.Fatalf("embedded detail type = %T, want *model.ContentVideo", embedded.Detail)
	}
	if len(video.Variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(video.Variants))
	}
	linked_resource_count := 0
	for _, asset := range embedded.Content.Assets {
		linked_resource_count += len(asset.DownloadResources)
	}
	if linked_resource_count != 1 {
		t.Fatalf("embedded linked resource count = %d, want 1", linked_resource_count)
	}

	list, err := service.ListContents(ContentListOptions{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list.List {
		if item.ID == article_id {
			if item.FileCount != 2 {
				t.Fatalf("list article FileCount = %d, want 2", item.FileCount)
			}
			return
		}
	}
	t.Fatal("article missing from content list")
}

func new_content_service_test_db(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.Content{},
		&model.ContentArticle{},
		&model.ContentVideo{},
		&model.ContentVideoVariant{},
		&model.ContentAsset{},
		&model.ContentTextTrack{},
		&model.ContentTextTrackSource{},
		&model.ContentRelation{},
		&model.Account{},
		&model.ContentAccount{},
		&model.Influencer{},
		&model.ContentInfluencer{},
		&model.DownloadTask{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadResourceAsset{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}
