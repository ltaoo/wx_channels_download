package manager

type ServerStatus string

const (
	StatusStopped  ServerStatus = "stopped"
	StatusStarting ServerStatus = "starting"
	StatusRunning  ServerStatus = "running"
	StatusStopping ServerStatus = "stopping"
	StatusError    ServerStatus = "error"
)

type Server interface {
	Name() string
	Addr() string
	Start() error
	Stop() error
	Status() ServerStatus
	HealthCheck() error
}
