package manager

type ServerStatus string

const (
	StatusStopped  ServerStatus = "stopped"
	StatusRunning  ServerStatus = "running"
	StatusStopping ServerStatus = "stopping"
	StatusError    ServerStatus = "error"
)
