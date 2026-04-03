package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/download"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"wx_channel/internal/database/model"
	utilpkg "wx_channel/pkg/util"
)

const (
	bucketTask = "task"
)

type GopeedKV struct {
	Bucket    string `gorm:"primaryKey"`
	Key       string `gorm:"primaryKey"`
	Value     []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (GopeedKV) TableName() string {
	return "gopeed_kv"
}

type SqliteStorage struct {
	db          *gorm.DB
	lock        *sync.RWMutex
	logger      *zerolog.Logger
	downloadDir string
}

func NewSqliteStorage(db *gorm.DB, logger *zerolog.Logger, downloadDir string) *SqliteStorage {
	return &SqliteStorage{
		db:          db,
		lock:        &sync.RWMutex{},
		logger:      logger,
		downloadDir: downloadDir,
	}
}

func (s *SqliteStorage) Setup(buckets []string) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	return s.db.AutoMigrate(&GopeedKV{})
}

func (s *SqliteStorage) Put(bucket string, key string, v any) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if bucket == bucketTask {
		switch tv := v.(type) {
		case *download.Task:
			return s.putTask(tv)
		case download.Task:
			return s.putTask(&tv)
		default:
			return fmt.Errorf("invalid task type")
		}
	}

	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	kv := GopeedKV{
		Bucket: bucket,
		Key:    key,
		Value:  buf,
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "bucket"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&kv).Error
}

func (s *SqliteStorage) Get(bucket string, key string, v any) (bool, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if bucket == bucketTask {
		return s.getTask(key, v)
	}

	var kv GopeedKV
	err := s.db.Where("bucket = ? AND key = ?", bucket, key).First(&kv).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(kv.Value, v); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SqliteStorage) List(bucket string, v any) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if bucket == bucketTask {
		return s.listTasks(v)
	}

	var kvs []GopeedKV
	if err := s.db.Where("bucket = ?", bucket).Find(&kvs).Error; err != nil {
		return err
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("v must be a pointer to slice")
	}
	sliceVal := rv.Elem()
	elemType := sliceVal.Type().Elem()
	out := reflect.MakeSlice(sliceVal.Type(), 0, len(kvs))
	for _, kv := range kvs {
		if elemType.Kind() == reflect.Ptr {
			item := reflect.New(elemType.Elem())
			if err := json.Unmarshal(kv.Value, item.Interface()); err != nil {
				return err
			}
			out = reflect.Append(out, item)
		} else {
			item := reflect.New(elemType)
			if err := json.Unmarshal(kv.Value, item.Interface()); err != nil {
				return err
			}
			out = reflect.Append(out, item.Elem())
		}
	}
	sliceVal.Set(out)
	return nil
}

func (s *SqliteStorage) Pop(bucket string, key string, v any) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	exist, err := s.getUnlocked(bucket, key, v)
	if err != nil {
		return err
	}
	if !exist {
		return fmt.Errorf("key %s not found in bucket %s", key, bucket)
	}
	return s.deleteUnlocked(bucket, key)
}

func (s *SqliteStorage) Delete(bucket string, key string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.deleteUnlocked(bucket, key)
}

func (s *SqliteStorage) Close() error {
	return nil
}

func (s *SqliteStorage) Clear() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if err := s.db.Where("1=1").Delete(&GopeedKV{}).Error; err != nil {
		return err
	}
	return s.db.Where("1=1").Delete(&model.DownloadTask{}).Error
}

func (s *SqliteStorage) getUnlocked(bucket string, key string, v any) (bool, error) {
	if bucket == bucketTask {
		return s.getTask(key, v)
	}
	var kv GopeedKV
	err := s.db.Where("bucket = ? AND key = ?", bucket, key).First(&kv).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(kv.Value, v); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SqliteStorage) deleteUnlocked(bucket string, key string) error {
	if bucket == bucketTask {
		return s.db.Where("task_id = ?", key).Delete(&model.DownloadTask{}).Error
	}
	return s.db.Where("bucket = ? AND key = ?", bucket, key).Delete(&GopeedKV{}).Error
}

func (s *SqliteStorage) putTask(t *download.Task) error {
	var rec model.DownloadTask
	err := s.db.Where("task_id = ?", t.ID).First(&rec).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	progressBytes, _ := json.Marshal(t.Progress)
	metaBytes, _ := json.Marshal(t.Meta)

	title := ""
	if t.Meta != nil && t.Meta.Opts != nil && strings.TrimSpace(t.Meta.Opts.Name) != "" {
		title = t.Meta.Opts.Name
	}
	if title == "" && t.Meta != nil && t.Meta.Req != nil && t.Meta.Req.Labels != nil {
		title = t.Meta.Req.Labels["title"]
	}

	url := ""
	if t.Meta != nil && t.Meta.Req != nil {
		url = t.Meta.Req.URL
	}

	var size int64
	if t.Meta != nil && t.Meta.Res != nil {
		size = t.Meta.Res.Size
	}

	rec.TaskId = t.ID
	rec.Status = statusToInt(t.Status)
	rec.Protocol = t.Protocol
	if rec.URL == "" {
		rec.URL = url
	}
	if title != "" {
		rec.Title = title
	}
	rec.Progress = string(progressBytes)
	rec.Metadata1 = string(metaBytes)
	rec.Size = size
	rec.UpdatedAt = utilpkg.TimeToMillisInt64(t.UpdatedAt)
	if rec.CreatedAt == 0 {
		rec.CreatedAt = utilpkg.TimeToMillisInt64(t.CreatedAt)
	}

	if t.Status == base.DownloadStatusDone && t.Meta != nil && t.Meta.Opts != nil {
		fullPath, ok := buildTaskOutputPath(t.Meta)
		if ok {
			rec.Filepath = toRelativePath(s.downloadDir, fullPath)
		}
	}

	if rec.Id == 0 {
		return s.db.Create(&rec).Error
	}
	return s.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
		"status":     rec.Status,
		"protocol":   rec.Protocol,
		"url":        rec.URL,
		"title":      rec.Title,
		"size":       rec.Size,
		"progress":   rec.Progress,
		"filepath":   rec.Filepath,
		"metadata1":  rec.Metadata1,
		"updated_at": rec.UpdatedAt,
	}).Error
}

func (s *SqliteStorage) getTask(key string, v any) (bool, error) {
	var rec model.DownloadTask
	err := s.db.Where("task_id = ?", key).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	task := download.NewTask()
	task.ID = rec.TaskId
	task.Protocol = rec.Protocol
	task.Status = statusFromInt(rec.Status)
	task.CreatedAt = time.UnixMilli(rec.CreatedAt)
	task.UpdatedAt = time.UnixMilli(rec.UpdatedAt)

	task.Progress = &download.Progress{}
	if strings.TrimSpace(rec.Progress) != "" {
		_ = json.Unmarshal([]byte(rec.Progress), task.Progress)
	}
	if strings.TrimSpace(rec.Metadata1) != "" {
		_ = json.Unmarshal([]byte(rec.Metadata1), &task.Meta)
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return false, fmt.Errorf("v must be a pointer")
	}
	rv.Elem().Set(reflect.ValueOf(task).Elem())
	return true, nil
}

func (s *SqliteStorage) listTasks(v any) error {
	var recs []model.DownloadTask
	if err := s.db.Order("updated_at desc").Find(&recs).Error; err != nil {
		return err
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("v must be a pointer to slice")
	}
	out := reflect.MakeSlice(rv.Elem().Type(), 0, len(recs))

	for _, rec := range recs {
		task := download.NewTask()
		task.ID = rec.TaskId
		task.Protocol = rec.Protocol
		task.Status = statusFromInt(rec.Status)
		task.CreatedAt = time.UnixMilli(rec.CreatedAt)
		task.UpdatedAt = time.UnixMilli(rec.UpdatedAt)
		task.Progress = &download.Progress{}
		if strings.TrimSpace(rec.Progress) != "" {
			_ = json.Unmarshal([]byte(rec.Progress), task.Progress)
		}
		if strings.TrimSpace(rec.Metadata1) != "" {
			_ = json.Unmarshal([]byte(rec.Metadata1), &task.Meta)
		}
		out = reflect.Append(out, reflect.ValueOf(task))
	}

	rv.Elem().Set(out)
	return nil
}

func statusToInt(s base.Status) int {
	switch s {
	case base.DownloadStatusReady:
		return 0
	case base.DownloadStatusRunning:
		return 1
	case base.DownloadStatusPause:
		return 2
	case base.DownloadStatusWait:
		return 3
	case base.DownloadStatusDone:
		return 4
	case base.DownloadStatusError:
		return 5
	default:
		return 0
	}
}

func statusFromInt(v int) base.Status {
	switch v {
	case 0:
		return base.DownloadStatusReady
	case 1:
		return base.DownloadStatusRunning
	case 2:
		return base.DownloadStatusPause
	case 3:
		return base.DownloadStatusWait
	case 4:
		return base.DownloadStatusDone
	case 5:
		return base.DownloadStatusError
	default:
		return base.DownloadStatusReady
	}
}

func buildTaskOutputPath(metaPtr interface{}) (string, bool) {
	type metaLike interface {
		FolderPath() string
		SingleFilepath() string
	}
	if metaPtr == nil {
		return "", false
	}
	ml, ok := metaPtr.(metaLike)
	if !ok {
		return "", false
	}

	v := reflect.ValueOf(metaPtr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return "", false
	}
	resField := v.Elem().FieldByName("Res")
	if !resField.IsValid() || resField.IsNil() {
		return "", false
	}
	resName := ""
	if resField.Elem().Kind() == reflect.Struct {
		nameField := resField.Elem().FieldByName("Name")
		if nameField.IsValid() && nameField.Kind() == reflect.String {
			resName = nameField.String()
		}
	}
	if strings.TrimSpace(resName) != "" {
		return ml.FolderPath(), true
	}
	return ml.SingleFilepath(), true
}

func toRelativePath(baseDir string, fullPath string) string {
	if baseDir == "" {
		return fullPath
	}
	rel := strings.TrimPrefix(fullPath, baseDir)
	rel = strings.TrimPrefix(rel, string(os.PathSeparator))
	rel = strings.TrimPrefix(rel, "/")
	return rel
}
