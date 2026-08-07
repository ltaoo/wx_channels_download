package events

import "wx_channel/internal/database/model"

// Event type constants.
const (
	TypeProxyCommand          = "proxy.command"
	TypeProxyStatusChanged    = "proxy.status_changed"
	TypeBrowseHistoryRecorded = "browsehistory.recorded"
	TypeDownloadTaskFinished  = "downloadtask.finished"
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

// DownloadTaskFinished is published when a download task finishes.
type DownloadTaskFinished struct {
	TaskID int
}

func (e DownloadTaskFinished) Type() string { return TypeDownloadTaskFinished }
