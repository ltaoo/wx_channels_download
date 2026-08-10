package model

import (
	"errors"

	"gorm.io/gorm"
)

// TaskStatus enum for DownloadTask
const (
	TaskStatusWaiting     = 0
	TaskStatusPreparing   = 1
	TaskStatusDownloading = 2
	TaskStatusPaused      = 3
	TaskStatusMerging     = 4
	TaskStatusFinished    = 5
	TaskStatusFailed      = 6
	TaskStatusCancelled   = 7
)

// ResourceType enum
const (
	ResourceTypeFile       = "FILE"
	ResourceTypeCollection = "COLLECTION"
	ResourceTypeStream     = "STREAM"
)

// TaskRelationType enum for DownloadTask lineage.
const (
	TaskRelationDiscovered = "discovered"
	TaskRelationDerived    = "derived"
	TaskRelationDependency = "dependency"
)

// DownloadTask is the main download task table (pure container; type and stream fields have been pushed down to DownloadResource)
type DownloadTask struct {
	Id           int     `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentId    *string `gorm:"column:content_id;index:idx_download_task_content" json:"content_id,omitempty"`
	ParentTaskID *int    `gorm:"column:parent_task_id;index:idx_download_task_parent" json:"parent_task_id,omitempty"`
	RootTaskID   int     `gorm:"column:root_task_id;index:idx_download_task_root" json:"root_task_id"`
	RelationType string  `gorm:"column:relation_type" json:"relation_type,omitempty"`
	Name         string  `gorm:"not null" json:"name"`
	PlatformId   string  `gorm:"not null" json:"platform_id"`
	UniqueID     string  `gorm:"column:unique_id;index:idx_task_unique_id" json:"unique_id"`
	Status       int     `gorm:"not null;default:0" json:"status"`
	SourceURL    string  `json:"source_url"`
	CoverURL     string  `json:"cover_url"`
	CoverWidth   string  `json:"cover_width"`
	CoverHeight  string  `json:"cover_height"`
	ConfigJSON   string  `gorm:"column:config_json" json:"config_json"`
	MetadataJSON string  `gorm:"column:metadata_json" json:"metadata_json"`
	ErrorMessage string  `gorm:"column:error_message" json:"error_message"`
	Timestamps
}

func (DownloadTask) TableName() string { return "download_task" }

func (task *DownloadTask) BeforeCreate(_ *gorm.DB) error {
	if task.CreatedAt == 0 {
		return errors.New("download_task.created_at must be explicitly assigned")
	}
	return nil
}

// DownloadResource is an independently persisted downloadable resource.
// TaskId is optional so resources may exist without a DownloadTask container.
type DownloadResource struct {
	Id          int     `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId      *int    `gorm:"index:idx_resource_task" json:"task_id,omitempty"`
	ContentId   *string `gorm:"column:content_id" json:"content_id,omitempty"`
	DownloadDir string  `gorm:"column:download_dir;not null;default:''" json:"download_dir"` // Directory only; Name is stored separately.
	Name        string  `json:"name"`
	Kind        string  `gorm:"not null;default:file" json:"kind"`
	UniqueID    string  `gorm:"column:unique_id;index:idx_resource_unique_id" json:"unique_id"`
	// Type indicates the resource type: "file" | "stream"
	Type       string `gorm:"column:type;not null;default:file" json:"type"`
	Size       int64  `json:"size"`
	Downloaded int64  `json:"downloaded"`
	Speed      int64  `json:"speed"`
	Status     int    `json:"status"`
	MergeOrder int    `gorm:"column:merge_order;default:0" json:"merge_order"`
	Extra      string `gorm:"type:text" json:"extra"` // JSON: map[string]string, user-defined fields
	// Stream fields (moved from DownloadTask)
	StreamURL     string `gorm:"column:stream_url" json:"stream_url"`
	RecordStart   *int64 `gorm:"column:record_start" json:"record_start"`
	RecordEnd     *int64 `gorm:"column:record_end" json:"record_end"`
	Duration      int64  `json:"duration"`
	RotateMinutes int    `gorm:"column:rotate_minutes;default:0" json:"rotate_minutes"`
	RotateSize    int64  `gorm:"column:rotate_size;default:0" json:"rotate_size"`
	StartTime     *int64 `gorm:"column:start_time" json:"start_time"`
	FinishTime    *int64 `gorm:"column:finish_time" json:"finish_time"`
	Timestamps
}

func (DownloadResource) TableName() string { return "download_resource" }

// DownloadEndpoint represents a download source for a resource
type DownloadEndpoint struct {
	Id         int    `gorm:"primaryKey;autoIncrement" json:"id"`
	ResourceId int    `gorm:"not null;index:idx_endpoint_resource" json:"resource_id"`
	Protocol   string `gorm:"not null" json:"protocol"`
	URL        string `gorm:"not null" json:"url"`
	Priority   int    `gorm:"default:0" json:"priority"`
	Enabled    int    `gorm:"default:1" json:"enabled"`
	Headers    string `json:"headers"`
	Cookies    string `json:"cookies"`
	Status     int    `json:"status"`
	Timestamps
}

func (DownloadEndpoint) TableName() string { return "download_endpoint" }

// DownloadSegment represents a unified download segment
type DownloadSegment struct {
	Id          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	ResourceId  int    `gorm:"not null;index:idx_segment_resource" json:"resource_id"`
	Index       int    `gorm:"not null" json:"index"`
	URL         string `json:"url"`
	OffsetStart int64  `gorm:"column:offset_start" json:"offset_start"`
	OffsetEnd   int64  `gorm:"column:offset_end" json:"offset_end"`
	Size        int64  `json:"size"`
	Downloaded  int64  `json:"downloaded"`
	Status      int    `json:"status"`
	Retry       int    `gorm:"default:0" json:"retry"`
	Timestamps
}

func (DownloadSegment) TableName() string { return "download_segment" }

// DownloadConnection represents a connection state
type DownloadConnection struct {
	Id         int    `gorm:"primaryKey;autoIncrement" json:"id"`
	EndpointId int    `gorm:"not null;index:idx_conn_endpoint" json:"endpoint_id"`
	WorkerId   string `gorm:"column:worker_id" json:"worker_id"`
	Host       string `json:"host"`
	IP         string `gorm:"column:ip" json:"ip"`
	Speed      int64  `json:"speed"`
	Bytes      int64  `json:"bytes"`
	Status     int    `json:"status"`
	LastActive int64  `gorm:"column:last_active" json:"last_active"`
	Timestamps
}

func (DownloadConnection) TableName() string { return "download_connection" }
