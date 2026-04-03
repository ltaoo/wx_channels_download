package model

type DownloadTask struct {
	Id         int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId     string `gorm:"uniqueIndex;not null" json:"task_id"`
	Type       int    `json:"type"`
	Status     int    `json:"status"`
	ExternalId string `json:"external_id"`
	Protocol   string `json:"protocol"`
	URL        string `gorm:"not null" json:"url"`
	Title      string `json:"title"`
	CoverURL   string `json:"cover_url"`
	Size       int64  `json:"size"`
	Progress   string `json:"progress"`
	Filepath   string `json:"filepath"`
	Error      string `json:"error"`
	Reason     string `json:"reason"`
	Metadata1  string `json:"metadata1"`
	Metadata2  string `json:"metadata2"`
	Idx        int    `json:"idx"`
	Timestamps
}

func (DownloadTask) TableName() string { return "download_task" }

type DownloadTaskPiece struct {
	Id             int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId         int    `gorm:"not null;uniqueIndex:idx_download_task_piece_unique,priority:1;index:idx_download_task_piece_status,priority:1" json:"task_id"`
	PieceIndex     int    `gorm:"not null;uniqueIndex:idx_download_task_piece_unique,priority:2" json:"piece_index"`
	StartOffset    int64  `gorm:"not null" json:"start_offset"`
	EndOffset      int64  `gorm:"not null" json:"end_offset"`
	Size           int64  `gorm:"not null" json:"size"`
	Status         int    `json:"status"`
	RetryCount     int    `json:"retry_count"`
	Checksum       string `json:"checksum"`
	TempPath       string `json:"temp_path"`
	LockedBy       string `json:"locked_by"`
	LeaseExpiresAt int64  `json:"lease_expires_at"`
	Timestamps
}

func (DownloadTaskPiece) TableName() string { return "download_task_piece" }

type DownloadTaskEvent struct {
	Id        int    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId    int    `gorm:"not null;index:idx_download_task_event_task,priority:1" json:"task_id"`
	Type      string `gorm:"not null" json:"type"`
	Message   string `json:"message"`
	Data      string `json:"data"`
	CreatedAt int64  `json:"created_at"`
}

func (DownloadTaskEvent) TableName() string { return "download_task_event" }

type LiveDownloadTask struct {
	Id             int     `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskId         string  `gorm:"uniqueIndex;not null" json:"task_id"`
	PlatformId     string  `gorm:"not null" json:"platform_id"`
	AccountId      *int    `json:"account_id"`
	LiveURL        string  `gorm:"not null" json:"live_url"`
	Title          string  `json:"title"`
	StreamerName   string  `json:"streamer_name"`
	CoverURL       string  `json:"cover_url"`
	SavePath       string  `gorm:"not null" json:"save_path"`
	Filename       string  `gorm:"not null" json:"filename"`
	Quality        string  `json:"quality"`
	Status         int     `json:"status"`
	Progress       float64 `json:"progress"`
	DownloadedSize int64   `json:"downloaded_size"`
	DownloadSpeed  float64 `json:"download_speed"`
	EstimatedTime  int64   `json:"estimated_time"`
	StartTime      *int64  `json:"start_time"`
	EndTime        *int64  `json:"end_time"`
	PauseTime      *int64  `json:"pause_time"`
	ErrorMsg       string  `json:"error_msg"`
	RetryCount     int     `json:"retry_count"`
	Timestamps
}

func (LiveDownloadTask) TableName() string { return "live_download_task" }
