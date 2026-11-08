package services

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"wx_channel/internal/database/model"
)

func TestDownloadTaskBatchExistenceUsesBatchedQueries(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tasks.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, _ := db.DB()
	t.Cleanup(func() { _ = sql_db.Close() })
	if err := db.AutoMigrate(&model.DownloadTask{}); err != nil {
		t.Fatal(err)
	}
	wide_id, cancelled_id, deleted_id := "wxchannels:18446744073709551614", "wxchannels:cancelled", "wxchannels:deleted"
	deleted_at := int64(2)
	for _, task := range []model.DownloadTask{
		{Name: "finished", ContentId: &wide_id, Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 1}},
		{Name: "failed", ContentId: &wide_id, Status: model.TaskStatusFailed, Timestamps: model.Timestamps{CreatedAt: 1}},
		{Name: "cancelled", ContentId: &cancelled_id, Status: 7, Timestamps: model.Timestamps{CreatedAt: 1}},
		{Name: "deleted", ContentId: &deleted_id, Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 1, DeletedAt: &deleted_at}},
	} {
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	queries := 0
	if err := db.Callback().Query().Before("gorm:query").Register("count_existence_queries", func(query *gorm.DB) {
		if query.Statement.Table == "download_task" {
			queries++
		}
	}); err != nil {
		t.Fatal(err)
	}
	service := NewDownloadTaskService(db, nil, nil, nil, "", "", nil)
	result, err := service.CheckContentIDs(context.Background(), []string{" " + wide_id + " ", "wxchannels:missing", wide_id, cancelled_id, deleted_id})
	if err != nil {
		t.Fatal(err)
	}
	if queries != 1 || result.Total != 4 || len(result.List) != 4 {
		t.Fatalf("expected one batch and four unique IDs: queries=%d result=%+v", queries, result)
	}
	if result.List[0].ContentID != wide_id || !result.List[0].Exists || result.List[0].TaskCount != 2 || result.List[1].Exists || !result.List[2].Exists || result.List[3].Exists {
		t.Fatalf("wrong existence/count/order/deletion semantics: %+v", result.List)
	}
	empty, err := service.CheckContentIDs(context.Background(), []string{})
	if err != nil || empty.List == nil || empty.Total != 0 || queries != 1 {
		t.Fatalf("empty batch queried database or lost []: %+v %v", empty, err)
	}
	ids := make([]string, 501)
	for index := range ids {
		ids[index] = fmt.Sprintf("wxchannels:bulk-%d", index)
	}
	queries = 0
	large, err := service.CheckContentIDs(context.Background(), ids)
	if err != nil || large.Total != len(ids) || queries != 2 {
		t.Fatalf("large batch must use two bounded IN queries: queries=%d result=%+v err=%v", queries, large, err)
	}
	for _, ids := range [][]string{{" "}, make([]string, MaxDownloadTaskContentChecks+1)} {
		if _, err := service.CheckContentIDs(context.Background(), ids); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.CheckContentIDs(ctx, []string{wide_id}); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatalf("cancellation returned a false result: %+v %v", result, err)
	}
	if err := db.Migrator().DropTable(&model.DownloadTask{}); err != nil {
		t.Fatal(err)
	}
	if result, err := service.CheckContentIDs(context.Background(), []string{wide_id}); err == nil || result != nil {
		t.Fatalf("database failure returned missing records: %+v %v", result, err)
	}
}
