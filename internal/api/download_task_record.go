package api

import (
	"errors"
	"math"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

// DownloadTaskRecord is the unified summary structure for download tasks.
// Both REST list items and WebSocket task fields must use this structure.
type DownloadTaskRecord struct {
	ID           int                      `json:"id"`
	ContentID    *string                  `json:"content_id,omitempty"`
	ParentTaskID *int                     `json:"parent_task_id,omitempty"`
	RootTaskID   int                      `json:"root_task_id"`
	RelationType string                   `json:"relation_type,omitempty"`
	ChildCount   int                      `json:"child_count"`
	Name         string                   `json:"name"`
	PlatformID   string                   `json:"platform_id"`
	Status       int                      `json:"status"`
	SourceURL    string                   `json:"source_url"`
	CoverURL     string                   `json:"cover_url"`
	CoverWidth   string                   `json:"cover_width"`
	CoverHeight  string                   `json:"cover_height"`
	ConfigJSON   string                   `json:"config_json"`
	MetadataJSON string                   `json:"metadata_json"`
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

// DownloadTaskFileRecord is the frontend file node for a single Resource under a Task.
type DownloadTaskFileRecord struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	ResourceType string  `json:"resource_type"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	Size         int64   `json:"size"`
	Downloaded   int64   `json:"downloaded"`
	Speed        int64   `json:"speed"`
	Progress     float64 `json:"progress"`
	URL          string  `json:"url"`
	OutputPath   string  `json:"output_path"`
	Error        string  `json:"error"`
}

// DownloadTaskStats holds counts of download tasks by status.
type DownloadTaskStats struct {
	Total       int `json:"total"`
	Downloading int `json:"downloading"`
	Paused      int `json:"paused"`
	Waiting     int `json:"waiting"`
	Finished    int `json:"finished"`
	Error       int `json:"error"`
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
	var task model.DownloadTask
	if err := c.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	records, err := c.buildDownloadTaskRecords([]model.DownloadTask{task})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return &records[0], nil
}

func (c *APIClient) buildDownloadTaskRecords(tasks []model.DownloadTask) ([]DownloadTaskRecord, error) {
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
		ID           int    `gorm:"column:id"`
		TaskID       int    `gorm:"column:task_id"`
		Name         string `gorm:"column:name"`
		Kind         string `gorm:"column:kind"`
		ResourceType string `gorm:"column:type"`
		Size         int64  `gorm:"column:size"`
		Status       int    `gorm:"column:status"`
		MergeOrder   int    `gorm:"column:merge_order"`
	}
	var resources []resourceInfo
	if err := c.db.Table("download_resource").
		Select("id, task_id, name, kind, type, size, status, merge_order").
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

	type childAggregate struct {
		ParentTaskID int `gorm:"column:parent_task_id"`
		Count        int `gorm:"column:count"`
	}
	var childAggregates []childAggregate
	if err := c.db.Model(&model.DownloadTask{}).
		Select("parent_task_id, COUNT(*) AS count").
		Where("parent_task_id IN ? AND deleted_at IS NULL", taskIDs).
		Group("parent_task_id").
		Scan(&childAggregates).Error; err != nil {
		return nil, err
	}
	childCountByTask := make(map[int]int, len(childAggregates))
	for _, aggregate := range childAggregates {
		childCountByTask[aggregate.ParentTaskID] = aggregate.Count
	}

	for _, task := range tasks {
		totalSize := sizeByTask[task.Id]
		if totalSize <= 0 {
			totalSize = segmentSizeByTask[task.Id]
		}
		errorMessage := ""
		if task.Status == model.TaskStatusFailed {
			errorMessage = task.ErrorMessage
		}
		resourceRows := resourcesByTask[task.Id]
		files := make([]DownloadTaskFileRecord, 0, len(resourceRows))
		for _, resource := range resourceRows {
			outputPath := resource.Name
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
				ID:           resource.ID,
				Name:         resource.Name,
				Kind:         resource.Kind,
				ResourceType: resource.ResourceType,
				Type:         "file",
				Status:       status,
				Size:         resource.Size,
				Downloaded:   downloadedByResource[resource.ID],
				Speed:        speedByResource[resource.ID],
				Progress:     taskProgressPercent(downloadedByResource[resource.ID], resource.Size, mapResourceTaskStatus(resource.Status)),
				URL:          urlByResource[resource.ID],
				OutputPath:   outputPath,
				Error:        fileError,
			})
		}
		effectiveStatus := computeEffectiveTaskStatus(task.Status, files)
		records = append(records, DownloadTaskRecord{
			ID:           task.Id,
			ContentID:    task.ContentId,
			ParentTaskID: task.ParentTaskID,
			RootTaskID:   task.RootTaskID,
			RelationType: task.RelationType,
			ChildCount:   childCountByTask[task.Id],
			Name:         task.Name,
			PlatformID:   task.PlatformId,
			Status:       effectiveStatus,
			SourceURL:    task.SourceURL,
			CoverURL:     task.CoverURL,
			CoverWidth:   task.CoverWidth,
			CoverHeight:  task.CoverHeight,
			ConfigJSON:   task.ConfigJSON,
			MetadataJSON: task.MetadataJSON,
			URL:          urlByTask[task.Id],
			Size:         totalSize,
			Downloaded:   downloadedByTask[task.Id],
			Speed:        speedByTask[task.Id],
			Progress:     taskProgressPercent(downloadedByTask[task.Id], totalSize, effectiveStatus),
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

func computeEffectiveTaskStatus(dbStatus int, files []DownloadTaskFileRecord) int {
	switch dbStatus {
	case model.TaskStatusPaused, model.TaskStatusFailed,
		model.TaskStatusCancelled, model.TaskStatusMerging:
		return dbStatus
	}
	if len(files) == 0 {
		return dbStatus
	}
	allFinished := true
	hasDownloading := false
	for _, f := range files {
		switch f.Status {
		case "finished":
		case "downloading":
			hasDownloading = true
			allFinished = false
		default:
			allFinished = false
		}
	}
	if allFinished {
		return model.TaskStatusFinished
	}
	if hasDownloading {
		return model.TaskStatusDownloading
	}
	return model.TaskStatusWaiting
}
