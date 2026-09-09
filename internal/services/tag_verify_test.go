package services

import (
	"path/filepath"
	"testing"

	"github.com/ltaoo/velo"
	"gorm.io/gorm"

	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
)

func new_verify_tag_db(t *testing.T) *gorm.DB {
	t.Helper()
	app := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	db_path := filepath.Join(t.TempDir(), "tag.db")
	if err := app.Migrate(&velo.VeloDatabaseOpt{
		DBType:                    velo.DBTypeSQLite,
		DBPath:                    database.SQLiteDSN(db_path),
		Migrations:                &database.Migrations,
		DisableTimestampCallbacks: true,
	}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if err := database.ConfigureSQLiteRuntime(app.DB); err != nil {
		t.Fatalf("configure sqlite failed: %v", err)
	}
	if app.DB == nil {
		t.Fatal("migration produced no database handle")
	}
	return app.DB
}

func TestTagServiceCRUDAndAssociations(t *testing.T) {
	db := new_verify_tag_db(t)
	tag_service := NewTagService(db)
	content_service := NewContentService(db)

	// Empty catalog.
	all, err := tag_service.ListTags("")
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty tag catalog, got %d", len(all))
	}

	// Create + dedupe.
	go_tag, err := tag_service.CreateTag("Go")
	if err != nil {
		t.Fatalf("CreateTag failed: %v", err)
	}
	if go_tag.Id <= 0 {
		t.Fatal("expected a positive tag id")
	}
	dup, err := tag_service.CreateTag("Go")
	if err != nil {
		t.Fatalf("duplicate CreateTag failed: %v", err)
	}
	if dup.Id != go_tag.Id {
		t.Fatalf("expected dedupe to return same id, got %d vs %d", dup.Id, go_tag.Id)
	}

	// Keyword search.
	tag_service.CreateTag("Golang")
	matched, err := tag_service.ListTags("Go")
	if err != nil {
		t.Fatalf("ListTags keyword failed: %v", err)
	}
	if len(matched) != 2 {
		t.Fatalf("expected 2 tags matching 'Go', got %d", len(matched))
	}

	// Soft delete hides the tag from listing.
	if err := tag_service.DeleteTag(go_tag.Id); err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}
	after, err := tag_service.ListTags("")
	if err != nil {
		t.Fatalf("ListTags after delete failed: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("expected 1 remaining tag after soft delete, got %d", len(after))
	}
	restored, err := tag_service.CreateTag("Go")
	if err != nil {
		t.Fatalf("recreate tag failed: %v", err)
	}
	if restored.Id != go_tag.Id {
		t.Fatalf("expected soft-deleted tag to be reused, got %d vs %d", restored.Id, go_tag.Id)
	}

	// Content associations.
	if err := db.Create(&model.Content{Id: "c1", Type: "video", Title: "one"}).Error; err != nil {
		t.Fatalf("create content c1 failed: %v", err)
	}
	if err := db.Create(&model.Content{Id: "c2", Type: "video", Title: "two"}).Error; err != nil {
		t.Fatalf("create content c2 failed: %v", err)
	}
	if err := tag_service.SetContentTags("c1", []int{restored.Id}); err != nil {
		t.Fatalf("SetContentTags failed: %v", err)
	}
	if err := tag_service.SetContentTags("c2", []int{}); err != nil {
		t.Fatalf("SetContentTags empty failed: %v", err)
	}

	got, err := tag_service.GetContentTags("c1")
	if err != nil {
		t.Fatalf("GetContentTags failed: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Go" {
		t.Fatalf("expected c1 to have the Go tag, got %+v", got)
	}

	batch, err := tag_service.BatchGetContentTags([]string{"c1", "c2"})
	if err != nil {
		t.Fatalf("BatchGetContentTags failed: %v", err)
	}
	if len(batch["c1"]) != 1 || len(batch["c2"]) != 0 {
		t.Fatalf("unexpected batch tags: c1=%d c2=%d", len(batch["c1"]), len(batch["c2"]))
	}

	// Tag filter in ListContents returns only tagged content.
	filtered, err := content_service.ListContents(ContentListOptions{TagIDs: []int{restored.Id}})
	if err != nil {
		t.Fatalf("ListContents with TagIDs failed: %v", err)
	}
	if filtered.Total != 1 {
		t.Fatalf("expected exactly 1 content with the tag, got %d", filtered.Total)
	}
	if len(filtered.List) != 1 || filtered.List[0].ID != "c1" {
		t.Fatalf("expected c1, got %+v", filtered.List)
	}
	if len(filtered.List[0].Tags) != 1 {
		t.Fatalf("expected list item to carry its tags, got %+v", filtered.List[0].Tags)
	}

	// Without the filter, both content items are returned.
	all_content, err := content_service.ListContents(ContentListOptions{})
	if err != nil {
		t.Fatalf("ListContents without filter failed: %v", err)
	}
	if all_content.Total != 2 {
		t.Fatalf("expected 2 content items, got %d", all_content.Total)
	}
}
