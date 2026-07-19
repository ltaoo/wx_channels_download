package model

// TaskStatus enum for DownloadTaskV1
const (
	TaskStatusWaiting    = 0
	TaskStatusPreparing  = 1
	TaskStatusDownloading = 2
	TaskStatusPaused     = 3
	TaskStatusMerging    = 4
	TaskStatusFinished   = 5
	TaskStatusFailed     = 6
	TaskStatusCancelled  = 7
)

// ResourceType enum
const (
	ResourceTypeFile       = "FILE"
	ResourceTypeCollection = "COLLECTION"
	ResourceTypeStream     = "STREAM"
)

// DownloadTaskV1 下载任务主表
type DownloadTaskV1 struct {
	Id           int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string `gorm:"not null" json:"name"`
	ResourceType string `gorm:"not null;default:FILE" json:"resource_type"`
	Status       int    `gorm:"not null;default:0" json:"status"`
	SavePath     string `gorm:"not null" json:"save_path"`
	ConfigJSON   string `gorm:"column:config_json" json:"config_json"`
	Timestamps
}

func (DownloadTaskV1) TableName() string { return "download_task_v1" }

// DownloadResource 任务中的资源
type DownloadResource struct {
	Id         int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId     int    `gorm:"not null;index:idx_resource_task" json:"task_id"`
	Name       string `json:"name"`
	Kind       string `gorm:"not null;default:file" json:"kind"`
	Size       int64  `json:"size"`
	Status     int    `json:"status"`
	MergeOrder int    `gorm:"column:merge_order;default:0" json:"merge_order"`
	Timestamps
}

func (DownloadResource) TableName() string { return "download_resource" }

// DownloadEndpoint 资源下载源
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

// DownloadSegment 统一分片
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

// DownloadConnection 连接状态
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

// DownloadLive 直播信息
type DownloadLive struct {
	Id            int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId        int    `gorm:"not null;index:idx_live_task" json:"task_id"`
	StreamURL     string `gorm:"column:stream_url;not null" json:"stream_url"`
	RecordStart   *int64 `gorm:"column:record_start" json:"record_start"`
	RecordEnd     *int64 `gorm:"column:record_end" json:"record_end"`
	Duration      int64  `json:"duration"`
	RotateMinutes int    `gorm:"column:rotate_minutes;default:0" json:"rotate_minutes"`
	RotateSize    int64  `gorm:"column:rotate_size;default:0" json:"rotate_size"`
	IsLive        int    `gorm:"column:is_live;default:0" json:"is_live"`
	Timestamps
}

func (DownloadLive) TableName() string { return "download_live" }

// DownloadLog 任务日志
type DownloadLog struct {
	Id        int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId    int    `gorm:"not null;index:idx_log_task" json:"task_id"`
	Level     string `gorm:"not null;default:info" json:"level"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"`
}
