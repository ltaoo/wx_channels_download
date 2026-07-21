package api

import (
	"errors"
	"math"
	"path/filepath"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

// DownloadTaskRecord 是下载任务的统一摘要结构。
// REST 列表项和 WebSocket task 字段必须共同使用该结构。
type DownloadTaskRecord struct {
	ID           int                      `json:"id"`
	Name         string                   `json:"name"`
	ResourceType string                   `json:"resource_type"`
	Status       int                      `json:"status"`
	SavePath     string                   `json:"save_path"`
	ConfigJSON   string                   `json:"config_json"`
	URL          string                   `json:"url"`
	Size         int64                    `json:"size"`
	Downloaded   int64                    `json:"downloaded"`
	Speed        int64                    `json:"speed"`
	Progress     float64                  `json:"progress"`
	Error        string                   `json:"error"`
	Files        []DownloadTaskFileRecord `json:"files"`
	FileCount    int                      `json:"file_count"`
	CreatedAt    int64                    `json:"created_at"`
	UpdatedAt    int64                    `json:"updated_at"`
}

// DownloadTaskFileRecord 是 Task 下单个 Resource 的前端文件节点。
type DownloadTaskFileRecord struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	Size       int64   `json:"size"`
	Downloaded int64   `json:"downloaded"`
	Speed      int64   `json:"speed"`
	Progress   float64 `json:"progress"`
	URL        string  `json:"url"`
	OutputPath string  `json:"output_path"`
	Error      string  `json:"error"`
}

func taskProgressPercent(downloaded, total int64, status int) float64 {
	if status == model.TaskStatusFinished {
		return 100
	}
	if downloaded <= 0 || total <= 0 {
		return 0
	}
	percent := float64(downloaded) * 100 / float64(total)
	if percent >= 100 {
		return 100
	}
	return math.Round(percent*100) / 100
}

func (c *APIClient) buildDownloadTaskRecord(taskID int) (*DownloadTaskRecord, error) {
	if c.db == nil {
		return nil, errors.New("数据库不可用")
	}
	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	records, err := c.buildDownloadTaskRecords([]model.DownloadTaskV1{task})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return &records[0], nil
}

func (c *APIClient) buildDownloadTaskRecords(tasks []model.DownloadTaskV1) ([]DownloadTaskRecord, error) {
	records := make([]DownloadTaskRecord, 0, len(tasks))
	if len(tasks) == 0 {
		return records, nil
	}

	taskIDs := make([]int, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.Id
	}

	type endpointInfo struct {
		TaskID     int    `gorm:"column:task_id"`
		ResourceID int    `gorm:"column:resource_id"`
		URL        string `gorm:"column:url"`
	}
	var endpoints []endpointInfo
	if err := c.db.Raw(`SELECT r.task_id, r.id AS resource_id, e.url FROM download_endpoint e
		JOIN download_resource r ON e.resource_id = r.id
		WHERE r.task_id IN ? AND r.deleted_at IS NULL AND e.deleted_at IS NULL AND e.enabled = 1
		ORDER BY r.task_id ASC, e.priority ASC, e.id ASC`, taskIDs).Scan(&endpoints).Error; err != nil {
		return nil, err
	}
	urlByTask := make(map[int]string, len(tasks))
	urlByResource := make(map[int]string)
	for _, endpoint := range endpoints {
		if _, exists := urlByTask[endpoint.TaskID]; !exists {
			urlByTask[endpoint.TaskID] = endpoint.URL
		}
		if _, exists := urlByResource[endpoint.ResourceID]; !exists {
			urlByResource[endpoint.ResourceID] = endpoint.URL
		}
	}

	type resourceInfo struct {
		ID         int    `gorm:"column:id"`
		TaskID     int    `gorm:"column:task_id"`
		Name       string `gorm:"column:name"`
		Kind       string `gorm:"column:kind"`
		Size       int64  `gorm:"column:size"`
		Status     int    `gorm:"column:status"`
		MergeOrder int    `gorm:"column:merge_order"`
	}
	var resources []resourceInfo
	if err := c.db.Table("download_resource").
		Select("id, task_id, name, kind, size, status, merge_order").
		Where("task_id IN ? AND deleted_at IS NULL", taskIDs).
		Order("task_id ASC, merge_order ASC, id ASC").
		Scan(&resources).Error; err != nil {
		return nil, err
	}
	sizeByTask := make(map[int]int64, len(tasks))
	resourcesByTask := make(map[int][]resourceInfo, len(tasks))
	for _, resource := range resources {
		resourcesByTask[resource.TaskID] = append(resourcesByTask[resource.TaskID], resource)
		if resource.Size > 0 {
			sizeByTask[resource.TaskID] += resource.Size
		}
	}

	type segmentAggregate struct {
		TaskID          int   `gorm:"column:task_id"`
		ResourceID      int   `gorm:"column:resource_id"`
		TotalSize       int64 `gorm:"column:total_size"`
		TotalDownloaded int64 `gorm:"column:total_downloaded"`
	}
	var segmentAggregates []segmentAggregate
	if err := c.db.Raw(`SELECT r.task_id, r.id AS resource_id, COALESCE(SUM(s.size), 0) AS total_size,
			COALESCE(SUM(s.downloaded), 0) AS total_downloaded
		FROM download_segment s
		JOIN download_resource r ON s.resource_id = r.id
		WHERE r.task_id IN ? AND r.deleted_at IS NULL AND s.deleted_at IS NULL
		GROUP BY r.task_id, r.id`, taskIDs).Scan(&segmentAggregates).Error; err != nil {
		return nil, err
	}
	segmentSizeByTask := make(map[int]int64, len(tasks))
	downloadedByTask := make(map[int]int64, len(tasks))
	downloadedByResource := make(map[int]int64)
	for _, aggregate := range segmentAggregates {
		segmentSizeByTask[aggregate.TaskID] += aggregate.TotalSize
		downloadedByTask[aggregate.TaskID] += aggregate.TotalDownloaded
		downloadedByResource[aggregate.ResourceID] = aggregate.TotalDownloaded
	}

	type speedAggregate struct {
		TaskID     int   `gorm:"column:task_id"`
		ResourceID int   `gorm:"column:resource_id"`
		Speed      int64 `gorm:"column:speed"`
	}
	var speedAggregates []speedAggregate
	if err := c.db.Raw(`SELECT r.task_id, r.id AS resource_id, COALESCE(MAX(c.speed), 0) AS speed
		FROM download_connection c
		JOIN download_endpoint e ON c.endpoint_id = e.id
		JOIN download_resource r ON e.resource_id = r.id
		WHERE r.task_id IN ? AND r.deleted_at IS NULL AND e.deleted_at IS NULL AND c.deleted_at IS NULL
		GROUP BY r.task_id, r.id`, taskIDs).Scan(&speedAggregates).Error; err != nil {
		return nil, err
	}
	speedByTask := make(map[int]int64, len(tasks))
	speedByResource := make(map[int]int64)
	for _, aggregate := range speedAggregates {
		speedByTask[aggregate.TaskID] += aggregate.Speed
		speedByResource[aggregate.ResourceID] = aggregate.Speed
	}

	errorByTask := make(map[int]string, len(tasks))
	var logs []model.DownloadLog
	if err := c.db.Where("task_id IN ? AND level = ?", taskIDs, "error").Order("id DESC").Find(&logs).Error; err != nil {
		return nil, err
	}
	for _, log := range logs {
		if _, exists := errorByTask[log.TaskId]; !exists {
			errorByTask[log.TaskId] = log.Message
		}
	}

	for _, task := range tasks {
		totalSize := sizeByTask[task.Id]
		if totalSize <= 0 {
			totalSize = segmentSizeByTask[task.Id]
		}
		errorMessage := ""
		if task.Status == model.TaskStatusFailed {
			errorMessage = errorByTask[task.Id]
		}
		resourceRows := resourcesByTask[task.Id]
		files := make([]DownloadTaskFileRecord, 0, len(resourceRows))
		for _, resource := range resourceRows {
			outputPath := task.SavePath
			if task.ResourceType != model.ResourceTypeFile || filepath.Base(task.SavePath) != filepath.Base(resource.Name) {
				outputPath = filepath.Join(task.SavePath, filepath.Base(resource.Name))
			}
			status := "waiting"
			switch resource.Status {
			case 1:
				status = "downloading"
			case 2:
				status = "finished"
			}
			fileError := ""
			if resource.Status != 2 {
				switch task.Status {
				case model.TaskStatusPaused:
					status = "paused"
				case model.TaskStatusFailed:
					status = "error"
					fileError = errorMessage
				case model.TaskStatusCancelled:
					status = "cancelled"
				}
			}
			files = append(files, DownloadTaskFileRecord{
				ID:         resource.ID,
				Name:       resource.Name,
				Kind:       resource.Kind,
				Type:       "file",
				Status:     status,
				Size:       resource.Size,
				Downloaded: downloadedByResource[resource.ID],
				Speed:      speedByResource[resource.ID],
				Progress:   taskProgressPercent(downloadedByResource[resource.ID], resource.Size, mapResourceTaskStatus(resource.Status)),
				URL:        urlByResource[resource.ID],
				OutputPath: outputPath,
				Error:      fileError,
			})
		}
		records = append(records, DownloadTaskRecord{
			ID:           task.Id,
			Name:         task.Name,
			ResourceType: task.ResourceType,
			Status:       task.Status,
			SavePath:     task.SavePath,
			ConfigJSON:   task.ConfigJSON,
			URL:          urlByTask[task.Id],
			Size:         totalSize,
			Downloaded:   downloadedByTask[task.Id],
			Speed:        speedByTask[task.Id],
			Progress:     taskProgressPercent(downloadedByTask[task.Id], totalSize, task.Status),
			Error:        errorMessage,
			Files:        files,
			FileCount:    len(files),
			CreatedAt:    task.CreatedAt,
			UpdatedAt:    task.UpdatedAt,
		})
	}
	return records, nil
}

func mapResourceTaskStatus(status int) int {
	if status == 2 {
		return model.TaskStatusFinished
	}
	return model.TaskStatusDownloading
}
