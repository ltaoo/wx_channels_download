package database

import (
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"wx_channel/internal/database/model"
	dbpkg "wx_channel/pkg/database"
)

func TestMigrationsCreateContentImageLivePhotoColumns(t *testing.T) {
	logger := zerolog.Nop()
	client := NewClientDatabase(&dbpkg.DatabaseConfig{
		DBType: "sqlite",
		DBPath: filepath.Join(t.TempDir(), "live-photo.db"),
	}, &logger)
	if err := client.Setup(); err != nil {
		t.Fatalf("setup database: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	for _, column := range []string{
		"live_photo_vid",
		"live_photo_type",
		"live_photo_url",
		"live_photo_format_id",
		"live_photo_width",
		"live_photo_height",
		"live_photo_size",
		"live_photo_duration_ms",
		"live_photo_formats",
	} {
		if !client.DB().Migrator().HasColumn(&model.ContentImage{}, column) {
			t.Errorf("content_image is missing column %q", column)
		}
	}
}
