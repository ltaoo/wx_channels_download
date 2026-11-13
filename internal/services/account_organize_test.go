package services

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/dm"
)

func TestOrganizeAccountFiles(t *testing.T) {
	directory := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(directory, "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, _ := db.DB()
	t.Cleanup(func() { _ = sql_db.Close() })
	if err := db.AutoMigrate(&model.Account{}, &model.Content{}, &model.ContentAccount{}, &model.DownloadTask{}, &model.DownloadResource{}, &model.DownloadEndpoint{}); err != nil {
		t.Fatal(err)
	}
	account_id, other_account_id, content_id, other_content_id := "wxchannels:author", "wxchannels:other", "wxchannels:video", "wxchannels:other-video"
	published := int64(1704067200)
	for _, record := range []any{
		&model.Account{Id: account_id, Nickname: "作者<&>/", Signature: "介绍<&>", AvatarURL: "https://example.com/avatar.jpg?a=1&b=2"},
		&model.Account{Id: other_account_id, Nickname: "作者<&>/"},
		&model.Content{Id: content_id, Title: "标题<&>/", Description: "简介<&>", PublishTime: &published, CoverURL: "https://example.com/cover.jpg?a=1&b=2"},
		&model.Content{Id: other_content_id, Title: "别人的视频"},
		&model.ContentAccount{AccountId: account_id, ContentId: content_id},
		&model.ContentAccount{AccountId: other_account_id, ContentId: other_content_id},
		&model.DownloadTask{Id: 1, ContentId: &content_id, Name: "finished", Status: model.TaskStatusFinished},
		&model.DownloadTask{Id: 2, ContentId: &content_id, Name: "active", Status: model.TaskStatusDownloading},
		&model.DownloadTask{Id: 3, ContentId: &other_content_id, Name: "other", Status: model.TaskStatusFinished},
	} {
		if task, ok := record.(*model.DownloadTask); ok {
			task.CreatedAt = 1
		}
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	task_id, active_id, other_id := 1, 2, 3
	for _, resource := range []model.DownloadResource{
		{Id: 12, TaskId: &task_id, Name: "source.mp4", Type: "FILE", Status: model.ResourceStatusFinished},
		{Id: 13, ContentId: &content_id, Name: "missing.mp4", Type: "file", Status: model.ResourceStatusFinished},
		{Id: 14, TaskId: &task_id, Name: "cover.jpg", Type: "FILE", Status: model.ResourceStatusFinished},
		{Id: 15, TaskId: &active_id, Name: "active.mp4", Type: "FILE", Status: model.ResourceStatusFinished},
		{Id: 16, TaskId: &other_id, Name: "other.mp4", Type: "FILE", Status: model.ResourceStatusFinished},
		{Id: 17, TaskId: &task_id, Name: "partial.mp4", Type: "FILE", Status: model.ResourceStatusDownloading},
		{Id: 18, TaskId: &task_id, Name: "live.mp4", Type: "STREAM", Status: model.ResourceStatusFinished},
		{Id: 19, ContentId: &content_id, Name: "source-other.mp4", Type: "FILE", Status: model.ResourceStatusFinished},
	} {
		if err := db.Create(&resource).Error; err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(directory, "source.mp4")
	if err := os.WriteFile(source, []byte("original video"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "source-other.mp4"), []byte("another video"), 0644); err != nil {
		t.Fatal(err)
	}
	service := NewDownloadTaskService(db, nil, nil, nil, directory, directory, nil)
	request := dm.OrganizeAccountFilesRequest{AccountID: account_id, OutputDir: filepath.Join(directory, "library")}
	preview, err := service.OrganizeAccountFiles(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || len(preview.Files) != 3 || preview.Files[0].Status != "planned" || preview.Files[1].Status != "error" || preview.Files[2].Status != "planned" {
		t.Fatalf("wrong selection/preview: %+v", preview)
	}
	if _, err := os.Stat(request.OutputDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview wrote files: %v", err)
	}
	var unchanged model.DownloadResource
	if err := db.First(&unchanged, 12).Error; err != nil || unchanged.Name != "source.mp4" || unchanged.DownloadDir != "" {
		t.Fatalf("preview changed database: %+v %v", unchanged, err)
	}
	shared := model.DownloadResource{Id: 20, ContentId: &content_id, DownloadDir: directory, Name: "source.mp4", Type: "FILE", Status: model.ResourceStatusFinished}
	if err := db.Create(&shared).Error; err != nil {
		t.Fatal(err)
	}
	shared_preview, err := service.OrganizeAccountFiles(context.Background(), request)
	if err != nil || shared_preview.Files[0].Status != "error" || !strings.Contains(shared_preview.Files[0].Error, "共用") {
		t.Fatalf("shared source was not protected: %+v %v", shared_preview, err)
	}
	if err := db.Delete(&shared).Error; err != nil {
		t.Fatal(err)
	}
	execute := false
	request.DryRun = &execute
	if err := db.Exec("CREATE TRIGGER fail_organize_update BEFORE UPDATE OF download_dir ON download_resource BEGIN SELECT RAISE(ABORT, 'database write failed'); END").Error; err != nil {
		t.Fatal(err)
	}
	failed, err := service.OrganizeAccountFiles(context.Background(), request)
	if err != nil || failed.Files[0].Status != "error" {
		t.Fatalf("database failure hidden: %+v %v", failed, err)
	}
	if data, err := os.ReadFile(source); err != nil || string(data) != "original video" {
		t.Fatalf("lost original on database failure: %q %v", data, err)
	}
	for _, path := range []string{failed.Files[0].Video, failed.Files[0].NFO} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("rollback left new file: %s %v", path, err)
		}
	}
	if err := db.First(&unchanged, 12).Error; err != nil || unchanged.Name != "source.mp4" || unchanged.DownloadDir != "" {
		t.Fatalf("database failure changed location: %+v %v", unchanged, err)
	}
	if err := os.WriteFile(failed.Files[0].Video, []byte("original video"), 0644); err != nil {
		t.Fatal(err)
	}
	failed_existing, err := service.OrganizeAccountFiles(context.Background(), request)
	if err != nil || failed_existing.Files[0].Status != "error" {
		t.Fatalf("database failure hidden with existing target: %+v %v", failed_existing, err)
	}
	if data, err := os.ReadFile(failed.Files[0].Video); err != nil || string(data) != "original video" {
		t.Fatalf("rollback removed pre-existing target: %q %v", data, err)
	}
	if err := db.Exec("DROP TRIGGER fail_organize_update").Error; err != nil {
		t.Fatal(err)
	}
	result, err := service.OrganizeAccountFiles(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	item := result.Files[0]
	if item.Status != "ready" || filepath.Base(item.Video) != "标题_&__ [12].mp4" || filepath.Base(filepath.Dir(item.Video)) != "标题_&__ [12]" || filepath.Dir(filepath.Dir(item.Video)) != result.Directory {
		t.Fatalf("invalid movie layout: %+v", item)
	}
	if result.Files[2].Status != "ready" || filepath.Dir(result.Files[2].Video) == filepath.Dir(item.Video) {
		t.Fatalf("same-title movies collide: %+v", result.Files)
	}
	if strings.TrimSuffix(item.Video, filepath.Ext(item.Video))+".nfo" != item.NFO {
		t.Fatalf("NFO does not match video filename: %+v", item)
	}
	for _, legacy_path := range []string{filepath.Join(result.Directory, "tvshow.nfo"), filepath.Join(result.Directory, "Season 01")} {
		if _, err := os.Stat(legacy_path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("created TV show artifact: %s %v", legacy_path, err)
		}
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("move retained old path: %v", err)
	}
	for _, path := range []string{item.Video} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "original video" {
			t.Fatalf("lost video: %s %q %v", path, data, err)
		}
	}
	var moved model.DownloadResource
	if err := db.First(&moved, 12).Error; err != nil || moved.DownloadDir != filepath.Dir(item.Video) || moved.Name != filepath.Base(item.Video) {
		t.Fatalf("new path not persisted: %+v %v", moved, err)
	}
	detail, err := service.BuildTaskDetail(context.Background(), task_id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Files) == 0 || detail.Files[0].FilePath != item.Video || !detail.Files[0].Exists {
		t.Fatalf("task detail still uses old location: %+v", detail.Files)
	}
	for _, movie := range []dm.OrganizeAccountFile{item, result.Files[2]} {
		data, err := os.ReadFile(movie.NFO)
		if err != nil {
			t.Fatal(err)
		}
		var nfo kodi_movie_nfo
		if err := xml.Unmarshal(data, &nfo); err != nil {
			t.Fatal(err)
		}
		if nfo.UniqueID.Type != "dm" || !nfo.UniqueID.Default || nfo.Thumb == nil {
			t.Fatalf("invalid metadata: %s", data)
		}
		if nfo.XMLName.Local != "movie" || nfo.Title != "标题<&>/" || nfo.Plot != "简介<&>" || nfo.Premiered != "2024-01-01" || nfo.Year != 2024 || nfo.Studio != "作者<&>/" || nfo.Thumb.Aspect != "poster" {
			t.Fatalf("wrong movie NFO: %s", data)
		}
		for _, tag := range []string{"<season>", "<episode>", "<showtitle>", "<aired>", "<episodedetails>", "<tvshow>"} {
			if bytes.Contains(data, []byte(tag)) {
				t.Fatalf("TV show tag in movie NFO: %s", data)
			}
		}
	}
	retry, err := service.OrganizeAccountFiles(context.Background(), request)
	if err != nil || retry.Files[0].Status != "ready" || retry.Files[0].Video != item.Video {
		t.Fatalf("retry failed: %+v %v", retry, err)
	}
	conflict_request := request
	conflict_request.OutputDir = filepath.Join(directory, "conflicting-library")
	conflict_preview, err := service.OrganizeAccountFiles(context.Background(), dm.OrganizeAccountFilesRequest{AccountID: account_id, OutputDir: conflict_request.OutputDir})
	if err != nil {
		t.Fatal(err)
	}
	conflict_path := conflict_preview.Files[0].Video
	if err := os.MkdirAll(filepath.Dir(conflict_path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conflict_path, []byte("user file"), 0644); err != nil {
		t.Fatal(err)
	}
	conflict, err := service.OrganizeAccountFiles(context.Background(), conflict_request)
	if err != nil || conflict.Files[0].Status != "error" {
		t.Fatalf("conflict hidden: %+v %v", conflict, err)
	}
	data, _ := os.ReadFile(conflict_path)
	if string(data) != "user file" {
		t.Fatal("overwrote a conflicting video")
	}
	if data, err := os.ReadFile(item.Video); err != nil || string(data) != "original video" {
		t.Fatalf("conflict removed source: %q %v", data, err)
	}
	if err := os.WriteFile(item.NFO, []byte("user metadata"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _ = service.OrganizeAccountFiles(context.Background(), request)
	data, _ = os.ReadFile(item.NFO)
	if string(data) != "user metadata" {
		t.Fatal("overwrote a conflicting NFO")
	}
	if err := db.Model(&model.Content{}).Where("id = ?", other_content_id).Update("title", "").Error; err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "other.mp4"), []byte("other video"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Update().After("gorm:update").Register("replace_organize_source", func(tx *gorm.DB) {
		replacement := filepath.Join(directory, "replacement.mp4")
		if err := os.WriteFile(replacement, []byte("replacement video"), 0644); err != nil {
			tx.AddError(err)
			return
		}
		if err := os.Rename(replacement, filepath.Join(directory, "other.mp4")); err != nil {
			tx.AddError(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	request.AccountID = other_account_id
	other, err := service.OrganizeAccountFiles(context.Background(), request)
	if remove_err := db.Callback().Update().Remove("replace_organize_source"); remove_err != nil {
		t.Fatal(remove_err)
	}
	if err != nil || other.Directory == result.Directory || len(other.Files) != 1 || other.Files[0].ResourceID != 16 {
		t.Fatalf("accounts mixed: %+v %v", other, err)
	}
	if other.Files[0].Status != "error" || !strings.Contains(other.Files[0].Error, "未移除") {
		t.Fatalf("source replacement was not detected: %+v", other.Files[0])
	}
	if data, err := os.ReadFile(filepath.Join(directory, "other.mp4")); err != nil || string(data) != "replacement video" {
		t.Fatalf("removed replaced source: %q %v", data, err)
	}
	if data, err := os.ReadFile(other.Files[0].Video); err != nil || string(data) != "other video" {
		t.Fatalf("lost relocated original: %q %v", data, err)
	}
	other_retry, err := service.OrganizeAccountFiles(context.Background(), request)
	if err != nil || other_retry.Files[0].Status != "ready" || other_retry.Files[0].Video != other.Files[0].Video {
		t.Fatalf("missing title changed path on retry: %+v %v", other_retry, err)
	}
	request.AccountID = "missing"
	if _, err := service.OrganizeAccountFiles(context.Background(), request); err == nil {
		t.Fatal("accepted unknown account")
	}
	request.AccountID, request.OutputDir = account_id, "relative"
	if _, err := service.OrganizeAccountFiles(context.Background(), request); err == nil {
		t.Fatal("accepted relative output directory")
	}
}

func TestKodiFileSafety(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(directory, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := kodi_write_file(context.Background(), filepath.Join(directory, "linked", "file.nfo"), bytes.NewReader([]byte("test"))); err == nil {
		t.Fatal("followed symlink parent")
	}
	path := filepath.Join(directory, "cancelled.mp4")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := kodi_write_file(ctx, path, bytes.NewReader([]byte("test"))); !errors.Is(err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("left incomplete output: %v", err)
	}
	for _, request := range []dm.OrganizeAccountFilesRequest{{OutputDir: directory}, {AccountID: "x"}} {
		if err := request.Validate(); err == nil {
			t.Fatal("accepted invalid request")
		}
	}
}
