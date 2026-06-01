package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	"gorm.io/gorm"

	utilpkg "wx_channel/pkg/util"

	"wx_channel/internal/database/model"
)

type DownloadService struct {
	downloader  *downloadpkg.Downloader
	db          *gorm.DB
	downloadDir string
	formatter   *utilpkg.FilenameProcessor
	ws          WSBroadcaster
}

type WSBroadcaster interface {
	Broadcast(msg interface{})
}

func NewDownloadService(downloader *downloadpkg.Downloader, db *gorm.DB, downloadDir string, ws WSBroadcaster) *DownloadService {
	return &DownloadService{
		downloader:  downloader,
		db:          db,
		downloadDir: downloadDir,
		formatter:   utilpkg.NewFilenameProcessor(downloadDir, make(map[string]int)),
		ws:          ws,
	}
}

func (s *DownloadService) Setup() error {
	if err := s.downloader.Setup(); err != nil {
		return err
	}
	_ = s.downloader.PutConfig(&base.DownloaderStoreConfig{
		DownloadDir: s.downloadDir,
		ProtocolConfig: map[string]any{
			"http": map[string]any{
				"connections": 4,
			},
		},
	})
	return nil
}

func (s *DownloadService) Listener(fn func(evt *downloadpkg.Event)) {
	s.downloader.Listener(fn)
}

func (s *DownloadService) CreateTask(req *base.Request, opts *base.Options) (string, error) {
	if s.downloader == nil {
		return "", fmt.Errorf("downloader not initialized")
	}
	id, err := s.downloader.CreateDirect(req, opts)
	if err != nil {
		return "", err
	}
	s.broadcastTask(id)
	return id, nil
}

func (s *DownloadService) CreateTaskBatch(batch *base.CreateTaskBatch) ([]string, error) {
	return s.downloader.CreateDirectBatch(batch)
}

func (s *DownloadService) GetTask(id string) *downloadpkg.Task {
	return s.downloader.GetTask(id)
}

func (s *DownloadService) GetTasks() []*downloadpkg.Task {
	return s.downloader.GetTasks()
}

func (s *DownloadService) GetTasksByFilter(filter *downloadpkg.TaskFilter) []*downloadpkg.Task {
	return s.downloader.GetTasksByFilter(filter)
}

func (s *DownloadService) Continue(filter *downloadpkg.TaskFilter) error {
	return s.downloader.Continue(filter)
}

func (s *DownloadService) Pause(filter *downloadpkg.TaskFilter) error {
	return s.downloader.Pause(filter)
}

func (s *DownloadService) Delete(filter *downloadpkg.TaskFilter, deleteFiles bool) error {
	return s.downloader.Delete(filter, deleteFiles)
}

func (s *DownloadService) Clear() {
	s.downloader.Delete(nil, true)
	if s.ws != nil {
		s.ws.Broadcast(map[string]interface{}{
			"type": "clear",
			"data": s.downloader.GetTasks(),
		})
	}
}

func (s *DownloadService) CheckExisting(id, spec, suffix string) bool {
	tasks := s.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		sameID := t.Meta.Req.Labels["id"] == id
		sameSpec := t.Meta.Req.Labels["spec"] == spec
		sameSuffix := t.Meta.Req.Labels["suffix"] == suffix
		if sameID && sameSpec && sameSuffix {
			return true
		}
	}
	return false
}

func (s *DownloadService) BuildTaskLabels(body *FeedDownloadTaskBody) map[string]string {
	return map[string]string{
		"id":       body.Id,
		"nonce_id": body.NonceId,
		"title":    body.Title,
		"key":      strconv.Itoa(body.Key),
		"spec":     body.Spec,
		"suffix":   body.Suffix,
	}
}

func (s *DownloadService) BuildTaskOpts(body *FeedDownloadTaskBody) (*base.Options, error) {
	filename, dir, err := s.formatter.ProcessFilename(body.Filename)
	if err != nil {
		return nil, err
	}
	connections := s.resolveConnections(body.URL)
	return &base.Options{
		Name: filename + body.Suffix,
		Path: filepath.Join(s.downloadDir, dir),
		Extra: &gopeedhttp.OptsExtra{
			Connections: connections,
		},
	}, nil
}

func (s *DownloadService) resolveConnections(url string) int {
	return 1
}

func (s *DownloadService) broadcastTask(id string) {
	if s.ws == nil {
		return
	}
	task := s.downloader.GetTask(id)
	if task != nil {
		s.ws.Broadcast(map[string]interface{}{
			"type": "event",
			"data": map[string]interface{}{
				"task": ConvertTask(task),
			},
		})
	}
}

func (s *DownloadService) BroadcastBatchTasks(ids []string) {
	if s.ws == nil {
		return
	}
	var tasks []interface{}
	for _, id := range ids {
		task := s.downloader.GetTask(id)
		if task != nil {
			tasks = append(tasks, ConvertTask(task))
		}
	}
	if len(tasks) > 0 {
		s.ws.Broadcast(map[string]interface{}{
			"type": "batch_tasks",
			"data": tasks,
		})
	}
}

func (s *DownloadService) ListTasks(page, pageSize int, status string) *PageResult {
	filter := &downloadpkg.TaskFilter{}
	if status != "" && status != "all" {
		filter.Statuses = []base.Status{base.Status(status)}
	}
	list := s.downloader.GetTasksByFilter(filter)
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	total := len(list)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	var taskInfos []*TaskInfo
	for _, t := range list[start:end] {
		taskInfos = append(taskInfos, ConvertTask(t))
	}
	return &PageResult{
		List:     taskInfos,
		Total:    int64(total),
		Page:     page,
		PageSize: pageSize,
	}
}

func (s *DownloadService) GetTaskProfile(id string) (map[string]string, error) {
	task := s.downloader.GetTask(id)
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	if task.Meta == nil || task.Meta.Opts == nil {
		return nil, fmt.Errorf("invalid task meta")
	}
	return map[string]string{
		"path": task.Meta.Opts.Path,
		"name": task.Meta.Opts.Name,
	}, nil
}

func (s *DownloadService) BuildBatchTask(feeds []FeedDownloadTaskBody) (*base.CreateTaskBatch, error) {
	tasks := s.downloader.GetTasks()
	existingTaskMap := make(map[string]int)
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		key := fmt.Sprintf("%s|%s|%s", t.Meta.Req.Labels["id"], t.Meta.Req.Labels["spec"], t.Meta.Req.Labels["suffix"])
		existingTaskMap[key] = 1
	}

	var items []map[string]string
	for _, req := range feeds {
		key := fmt.Sprintf("%s|%s|%s", req.Id, req.Spec, req.Suffix)
		if _, exists := existingTaskMap[key]; exists {
			continue
		}
		items = append(items, map[string]string{
			"id":       req.Id,
			"nonce_id": req.NonceId,
			"title":    req.Title,
			"key":      strconv.Itoa(req.Key),
			"spec":     req.Spec,
			"suffix":   req.Suffix,
			"url":      req.URL,
			"name":     req.Filename,
		})
	}
	if len(items) == 0 {
		return &base.CreateTaskBatch{}, nil
	}
	task := base.CreateTaskBatch{}
	for _, item := range items {
		filename, dir, err := s.formatter.ProcessFilename(item["name"] + item["suffix"])
		if err != nil {
			continue
		}
		task.Reqs = append(task.Reqs, &base.CreateTaskBatchItem{
			Req: &base.Request{
				URL: item["url"],
				Labels: map[string]string{
					"id":       item["id"],
					"nonce_id": item["nonce_id"],
					"title":    item["title"],
					"key":      item["key"],
					"spec":     item["spec"],
					"suffix":   item["suffix"],
				},
			},
			Opts: &base.Options{
				Name: filename,
				Path: filepath.Join(s.downloadDir, dir),
			},
		})
	}
	return &task, nil
}

func (s *DownloadService) CreateDownloadTaskWithBody(body *FeedDownloadTaskBody) (string, error) {
	labels := s.BuildTaskLabels(body)
	opts, err := s.BuildTaskOpts(body)
	if err != nil {
		return "", err
	}
	return s.CreateTask(&base.Request{
		URL:    body.URL,
		Labels: labels,
	}, opts)
}

func (s *DownloadService) CreateBatchDownloadTask(feeds []FeedDownloadTaskBody) ([]string, error) {
	batch, err := s.BuildBatchTask(feeds)
	if err != nil {
		return nil, err
	}
	if len(batch.Reqs) == 0 {
		return []string{}, nil
	}
	ids, err := s.CreateTaskBatch(batch)
	if err != nil {
		return nil, err
	}
	s.BroadcastBatchTasks(ids)
	return ids, nil
}

func (s *DownloadService) StartTask(id string) error {
	return s.Continue(&downloadpkg.TaskFilter{
		IDs: []string{id},
	})
}

func (s *DownloadService) PauseTask(id string) error {
	return s.Pause(&downloadpkg.TaskFilter{
		IDs: []string{id},
	})
}

func (s *DownloadService) ResumeTask(id string) error {
	return s.Continue(&downloadpkg.TaskFilter{
		IDs: []string{id},
	})
}

func (s *DownloadService) DeleteTask(id string) error {
	return s.Delete(&downloadpkg.TaskFilter{
		IDs: []string{id},
	}, true)
}

func (s *DownloadService) PauseAll() error {
	return s.Pause(&downloadpkg.TaskFilter{
		Statuses: []base.Status{
			base.DownloadStatusRunning,
			base.DownloadStatusWait,
			base.DownloadStatusReady,
		},
	})
}

func (s *DownloadService) ResumeAll() error {
	return s.Continue(&downloadpkg.TaskFilter{
		Statuses: []base.Status{
			base.DownloadStatusPause,
		},
	})
}

func (s *DownloadService) DB() *gorm.DB {
	return s.db
}

func (s *DownloadService) CreateContentDownloadTask(content *model.Content, t *downloadpkg.Task, reason string) (*model.DownloadTask, error) {
	db := s.DB()
	if db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if content == nil {
		return nil, fmt.Errorf("content is nil")
	}
	if t == nil {
		return nil, fmt.Errorf("download task is nil")
	}

	title := ""
	if t.Meta != nil && t.Meta.Opts != nil {
		title = t.Meta.Opts.Name
	}
	if title == "" {
		title = content.Title
	}

	taskURL := content.ContentURL
	if taskURL == "" {
		taskURL = content.URL
	}
	if taskURL == "" && t.Meta != nil && t.Meta.Req != nil {
		taskURL = t.Meta.Req.URL
	}

	size := content.Size
	if size <= 0 {
		size = content.FileSize
	}

	var meta2Bytes []byte
	meta2Bytes, _ = json.Marshal(map[string]any{
		"platform":    content.PlatformId,
		"external_id": content.ExternalId,
		"nonce_id":    content.ExternalId2,
		"eid":         "",
	})

	statusToInt := func(st base.Status) int {
		switch st {
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

	var rec model.DownloadTask
	err := db.Where("task_id = ?", t.ID).First(&rec).Error
	updates := map[string]any{
		"url":         taskURL,
		"external_id": content.ExternalId,
		"title":       title,
		"cover_url":   content.CoverURL,
		"metadata2":   string(meta2Bytes),
		"reason":      reason,
		"updated_at":  utilpkg.TimeToMillisInt64(t.UpdatedAt),
	}
	if size > 0 {
		updates["size"] = size
	}

	if err == nil {
		if err := db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(updates).Error; err != nil {
			return nil, err
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		rec = model.DownloadTask{
			TaskId:     t.ID,
			Status:     statusToInt(t.Status),
			Protocol:   t.Protocol,
			URL:        taskURL,
			ExternalId: content.ExternalId,
			Title:      title,
			CoverURL:   content.CoverURL,
			Size:       size,
			Reason:     reason,
			Metadata2:  string(meta2Bytes),
			Timestamps: model.Timestamps{
				CreatedAt: utilpkg.TimeToMillisInt64(t.CreatedAt),
				UpdatedAt: utilpkg.TimeToMillisInt64(t.UpdatedAt),
			},
		}
		if err := db.Create(&rec).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	now := utilpkg.NowMillis()
	if err := db.Model(&model.Content{}).Where("id = ?", content.Id).Updates(map[string]any{
		"download_task_id": rec.Id,
		"download_status":  rec.Status,
		"download_path":    rec.Filepath,
		"updated_at":       now,
	}).Error; err != nil {
		return &rec, err
	}

	return &rec, nil
}
