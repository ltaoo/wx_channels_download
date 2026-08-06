package wxmp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/wxmp"
)

func TestBuildDownloadTaskLivePhotoFromExample(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "scraper_examples", "a.json"))
	if err != nil {
		t.Fatalf("read live photo example: %v", err)
	}

	info, err := (&handler{}).BuildDownloadTask(raw, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("BuildDownloadTask: %v", err)
	}
	if len(info.AlbumImages) != 1 {
		t.Fatalf("AlbumImages len = %d, want 1", len(info.AlbumImages))
	}

	image := info.AlbumImages[0]
	if image.ImageType != model.ContentImageTypeLivePhoto {
		t.Fatalf("ImageType = %q, want %q", image.ImageType, model.ContentImageTypeLivePhoto)
	}
	if image.LivePhoto == nil {
		t.Fatal("LivePhoto is nil")
	}
	if image.LivePhoto.Vid != "live_4052c53963100007" || image.LivePhoto.Type != 1 {
		t.Fatalf("LivePhoto identity = (%q, %d)", image.LivePhoto.Vid, image.LivePhoto.Type)
	}
	if image.LivePhoto.FormatId != 0 || image.LivePhoto.Size != 7200103 || image.LivePhoto.Width != 2884 || image.LivePhoto.Height != 2160 || image.LivePhoto.DurationMs != 3090 {
		t.Fatalf("selected LivePhoto format = %+v", image.LivePhoto)
	}
	if len(image.LivePhoto.Formats) != 4 {
		t.Fatalf("LivePhoto formats len = %d, want 4", len(image.LivePhoto.Formats))
	}
	if strings.Contains(image.LivePhoto.URL, "&amp;") {
		t.Fatalf("LivePhoto URL was not HTML-decoded: %q", image.LivePhoto.URL)
	}

	var stillResource, liveResource *model.DownloadResource
	var liveEndpoint *model.DownloadEndpoint
	for _, resource := range info.Resources {
		switch {
		case strings.HasSuffix(resource.UniqueID, "_album_0"):
			stillResource = &resource.DownloadResource
		case strings.HasSuffix(resource.UniqueID, "_album_0_live"):
			liveResource = &resource.DownloadResource
			if len(resource.Endpoints) == 1 {
				liveEndpoint = &resource.Endpoints[0]
			}
		}
	}
	if stillResource == nil || liveResource == nil || liveEndpoint == nil {
		t.Fatalf("missing live-photo resources: still=%v live=%v endpoint=%v", stillResource != nil, liveResource != nil, liveEndpoint != nil)
	}
	if liveResource.Kind != "video/mp4" || liveResource.Name != stillResource.Name || liveResource.Size != 7200103 || liveResource.Duration != 3 {
		t.Fatalf("live resource = %+v", *liveResource)
	}
	if liveEndpoint.URL != image.LivePhoto.URL || liveEndpoint.Protocol != "https" {
		t.Fatalf("live endpoint = %+v", *liveEndpoint)
	}
}

func TestBuildDownloadTaskPageTypeAlbumNormalizesContentDetail(t *testing.T) {
	data := scraper.ArticleCgiDataNew{
		Bizuin:          "biz1",
		UserName:        "account1",
		NickName:        "author",
		Title:           "album title",
		Desc:            "album desc",
		Link:            "https://mp.weixin.qq.com/s/test",
		SourceURL:       "https://mp.weixin.qq.com/s/test",
		CdnURL:          "https://example.test/cover.jpg",
		Mid:             1,
		PageType:        2,
		BizType:         0,
		ImgFormat:       "jpeg",
		ContentNoencode: "<p>album desc</p>",
		PicturePageInfoList: []scraper.PicturePageInfo{
			{CdnUrl: "https://example.test/1.jpg", Width: 640, Height: 480},
			{CdnUrl: "https://example.test/2.jpg", Width: 800, Height: 600},
		},
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}

	info, err := (&handler{}).BuildDownloadTask(raw, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("BuildDownloadTask: %v", err)
	}
	if _, ok := info.ContentDetail.(*model.ContentAlbum); !ok {
		t.Fatalf("ContentDetail = %T, want *model.ContentAlbum", info.ContentDetail)
	}
	if len(info.AlbumImages) != 2 {
		t.Fatalf("AlbumImages len = %d, want 2", len(info.AlbumImages))
	}
	album := info.ContentDetail.(*model.ContentAlbum)
	if len(album.Images) != 2 {
		t.Fatalf("ContentDetail.Images len = %d, want 2", len(album.Images))
	}
	for i, image := range album.Images {
		if image.AlbumId != info.Content.Id {
			t.Fatalf("ContentDetail.Images[%d].AlbumId = %q, want %q", i, image.AlbumId, info.Content.Id)
		}
	}
	if !strings.Contains(info.Task.MetadataJSON, `"biz_type":2`) {
		t.Fatalf("MetadataJSON = %s, want biz_type 2", info.Task.MetadataJSON)
	}
}

func TestBuildDownloadTaskArticleIgnoresPictureListWithoutPageType(t *testing.T) {
	data := scraper.ArticleCgiDataNew{
		Bizuin:           "article-biz",
		UserName:         "article-account",
		NickName:         "author",
		Title:            "article title",
		Link:             "https://mp.weixin.qq.com/s/article",
		SourceURL:        "https://mp.weixin.qq.com/s/article",
		CdnURL:           "https://example.test/cover.jpg",
		Mid:              2,
		BizType:          1,
		ItemShowType:     0,
		RealItemShowType: 0,
		ContentNoencode:  `<p><img data-src="https://example.test/1.jpg"></p><p><img data-src="https://example.test/2.jpg"></p>`,
		PicturePageInfoList: []scraper.PicturePageInfo{
			{CdnUrl: "https://example.test/1.jpg"},
			{CdnUrl: "https://example.test/2.jpg"},
			{CdnUrl: "https://example.test/3.jpg"},
			{CdnUrl: "https://example.test/4.jpg"},
			{CdnUrl: "https://example.test/5.jpg"},
		},
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}

	info, err := (&handler{}).BuildDownloadTask(raw, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("BuildDownloadTask: %v", err)
	}
	if info.Content.Type != "article" {
		t.Fatalf("Content.Type = %q, want article", info.Content.Type)
	}
	if _, ok := info.ContentDetail.(*model.ContentArticle); !ok {
		t.Fatalf("ContentDetail = %T, want *model.ContentArticle", info.ContentDetail)
	}
	if len(info.AlbumImages) != 0 {
		t.Fatalf("AlbumImages len = %d, want 0", len(info.AlbumImages))
	}
	if !strings.Contains(info.Task.MetadataJSON, `"biz_type":1`) {
		t.Fatalf("MetadataJSON = %s, want biz_type 1", info.Task.MetadataJSON)
	}
	if len(info.Resources) != 3 {
		t.Fatalf("Resources len = %d, want HTML and two article images", len(info.Resources))
	}
	if !strings.HasSuffix(info.Resources[1].UniqueID, "_img_0") || !strings.HasSuffix(info.Resources[2].UniqueID, "_img_1") {
		t.Fatalf("article image resource IDs = %q, %q", info.Resources[1].UniqueID, info.Resources[2].UniqueID)
	}
}
