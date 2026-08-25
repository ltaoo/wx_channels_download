package events

import "wx_channel/internal/database/model"

// Event type constants.
const (
	TypeProxyCommand          = "proxy.command"
	TypeProxyStatusChanged    = "proxy.status_changed"
	TypeBrowseHistoryRecorded = "browsehistory.recorded"
	TypeServiceCommand        = "service.command"
	TypeServiceStatusChanged  = "service.status_changed"
	TypeDownloadTaskCreated   = "downloadtask.created"
	TypeDownloadTaskFinished  = "downloadtask.finished"
	TypeDownloadTaskDeleted   = "downloadtask.deleted"
	TypeScraperFetchProgress  = "scraper.fetch_progress"
	TypePlatformStatusChanged = "platform.status_changed"
)

// ProxyAction represents a command to the proxy service.
type ProxyAction string

const (
	ProxyStart         ProxyAction = "start"
	ProxyStop          ProxyAction = "stop"
	ProxyRestart       ProxyAction = "restart"
	ProxyApplySettings ProxyAction = "applySettings"
)

// ProxyCommand is published when the API wants to control the proxy service.
type ProxyCommand struct {
	Action ProxyAction
}

func (e ProxyCommand) Type() string { return TypeProxyCommand }

// ProxyStatusChanged is published when the proxy service state changes.
type ProxyStatusChanged struct {
	Status string // "running", "stopped", "stopping", "error"
	Addr   string
}

func (e ProxyStatusChanged) Type() string { return TypeProxyStatusChanged }

// BrowseHistoryRecorded is published when the wxchannels adapter captures browse data.
type BrowseHistoryRecorded struct {
	Browse *model.BrowseHistory
}

func (e BrowseHistoryRecorded) Type() string { return TypeBrowseHistoryRecorded }

// ServiceCommand is published to control service lifecycle (start/stop).
type ServiceCommand struct {
	Name   string // "admin", "api", "interceptor"
	Action string // "start", "stop"
}

func (e ServiceCommand) Type() string { return TypeServiceCommand }

// ServiceStatusChanged is published when a service's status or address changes.
type ServiceStatusChanged struct {
	Name   string
	Title  string
	Addr   string
	Status string // "running", "stopped", "stopping", "error"
}

func (e ServiceStatusChanged) Type() string { return TypeServiceStatusChanged }

// DownloadTaskCreated is published when a task is persisted without starting Hermes.
type DownloadTaskCreated struct {
	TaskID int
}

func (e DownloadTaskCreated) Type() string { return TypeDownloadTaskCreated }

// DownloadTaskFinished is published when a download task finishes.
type DownloadTaskFinished struct {
	TaskID int
}

func (e DownloadTaskFinished) Type() string { return TypeDownloadTaskFinished }

// DownloadTaskDeleted is published after the task graph has been soft-deleted.
type DownloadTaskDeleted struct {
	TaskID int
}

func (e DownloadTaskDeleted) Type() string { return TypeDownloadTaskDeleted }

// ScraperFetchProgress reports the current stage of a potentially long-running
// scraper fetch. RequestID lets the application-level scraper WebSocket
// clients ignore unrelated jobs on their shared connection.
type ScraperFetchProgress struct {
	RequestID    string  `json:"request_id"`
	Platform     string  `json:"platform"`
	URL          string  `json:"url"`
	BookID       string  `json:"book_id,omitempty"`
	BookTitle    string  `json:"book_title,omitempty"`
	Stage        string  `json:"stage"`
	Status       string  `json:"status"`
	Current      int     `json:"current"`
	Total        int     `json:"total"`
	Percent      float64 `json:"percent"`
	VolumeTitle  string  `json:"volume_title,omitempty"`
	ChapterID    string  `json:"chapter_id,omitempty"`
	ChapterTitle string  `json:"chapter_title,omitempty"`
	Message      string  `json:"message"`
	Error        string  `json:"error,omitempty"`
	Cached       bool    `json:"cached,omitempty"`
	CacheHits    int     `json:"cache_hits,omitempty"`
}

func (e ScraperFetchProgress) Type() string { return TypeScraperFetchProgress }

// PlatformStatusChanged reports whether a platform scraper is currently
// fetchable and, when it is not, why users cannot use it.
type PlatformStatusChanged struct {
	Platform  string `json:"platform"`
	Key       string `json:"key,omitempty"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (e PlatformStatusChanged) Type() string { return TypePlatformStatusChanged }
