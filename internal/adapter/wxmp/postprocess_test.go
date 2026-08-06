package wxmp

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

func TestPostprocessAlbumSkipsHTMLImageEmbedding(t *testing.T) {
	db := openPostprocessTestDB(t)
	externalID := "album-biz"

	downloadDir := t.TempDir()
	htmlPath := filepath.Join(downloadDir, externalID+"_html")
	imagePath := filepath.Join(downloadDir, externalID+"_album_0")
	coverPath := filepath.Join(downloadDir, externalID+"_cover")
	mustWritePostprocessFile(t, htmlPath, []byte("old html"))
	mustWritePostprocessFile(t, imagePath, []byte("gif image bytes"))
	mustWritePostprocessFile(t, coverPath, []byte("cover bytes"))

	task := &hermes.TaskJob{
		ID:       11,
		Name:     "album title",
		Metadata: map[string]any{"external_id": externalID, "biz_type": float64(2)},
		Resources: []hermes.ResourceJob{
			{ID: 1, Name: "album title", Kind: "text/html", UniqueID: externalID + "_html", FilePath: htmlPath},
			{
				ID: 2, Name: "image-hash", Kind: "image/gif", UniqueID: externalID + "_album_0", FilePath: imagePath,
				Endpoints: []hermes.Endpoint{{URL: "https://example.test/album.gif"}},
			},
			{
				ID: 3, Name: "album title", Kind: "image/jpeg", UniqueID: externalID + "_cover", FilePath: coverPath,
				Endpoints: []hermes.Endpoint{{URL: "https://example.test/cover.jpg"}},
			},
		},
	}

	if err := (&handler{}).Postprocess(context.Background(), task, adapter.PostprocessDeps{
		DB: db, Logger: zerolog.Nop(), BasePath: downloadDir,
	}); err != nil {
		t.Fatalf("Postprocess: %v", err)
	}

	output, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read output HTML: %v", err)
	}
	if string(output) != "old html" {
		t.Fatalf("album HTML was unexpectedly rewritten: %s", output)
	}
	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("album image should be retained: %v", err)
	}
	if _, err := os.Stat(coverPath); err != nil {
		t.Fatalf("cover should be retained: %v", err)
	}
	if len(task.Resources) != 3 {
		t.Fatalf("resources after postprocess = %+v, want all album resources retained", task.Resources)
	}
}

func TestPostprocessArticleMatchesEndpointAndUsesRuntimeFilePath(t *testing.T) {
	db := openPostprocessTestDB(t)
	externalID := "article-biz"
	imageURL := "https://mmbiz.qpic.cn/example/image?wx_fmt=png&from=appmsg"
	articleHTML := `<section><img data-src="https://mmbiz.qpic.cn/example/image?wx_fmt=png&amp;from=appmsg"></section>`

	downloadDir := t.TempDir()
	htmlPath := filepath.Join(downloadDir, externalID+"_html")
	imagePath := filepath.Join(downloadDir, externalID+"_img_0")
	imageBytes := []byte("png image bytes")
	mustWritePostprocessFile(t, htmlPath, []byte(articleHTML))
	mustWritePostprocessFile(t, imagePath, imageBytes)

	task := &hermes.TaskJob{
		ID:       12,
		Name:     "article title",
		Metadata: map[string]any{"external_id": externalID, "biz_type": float64(1)},
		Resources: []hermes.ResourceJob{
			{ID: 4, Name: "article title", Kind: "text/html", UniqueID: externalID + "_html", FilePath: htmlPath},
			{
				ID: 5, Name: "name-no-longer-equals-url-hash", Kind: "image/png", UniqueID: externalID + "_img_0", FilePath: imagePath,
				Endpoints: []hermes.Endpoint{{URL: imageURL}},
			},
		},
	}

	if err := (&handler{}).Postprocess(context.Background(), task, adapter.PostprocessDeps{
		DB: db, Logger: zerolog.Nop(), BasePath: downloadDir,
	}); err != nil {
		t.Fatalf("Postprocess: %v", err)
	}

	output, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read output HTML: %v", err)
	}
	wantDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes)
	if !strings.Contains(string(output), wantDataURI) {
		t.Fatalf("output HTML does not contain embedded article image: %s", output)
	}
	if strings.Contains(string(output), "data-src=") {
		t.Fatalf("output HTML still contains data-src: %s", output)
	}
	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		t.Fatalf("embedded image still exists, stat error = %v", err)
	}
	if len(task.Resources) != 1 || task.Resources[0].ID != 4 {
		t.Fatalf("resources after cleanup = %+v, want only HTML", task.Resources)
	}
}

func openPostprocessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "postprocess.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database connection: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	if err := db.AutoMigrate(
		&model.ContentArticle{},
		&model.ContentAlbum{},
		&model.ContentImage{},
		&model.DownloadResource{},
	); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return db
}

func mustWritePostprocessFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
