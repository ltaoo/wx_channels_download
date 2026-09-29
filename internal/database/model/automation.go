package model

// Trigger types recorded on a flow run record.
const (
	FlowRunTriggerCron   = "Cron"
	FlowRunTriggerAPI    = "API"
	FlowRunTriggerEvent  = "Event"
	FlowRunTriggerManual = "Manual"
)

// Flow run lifecycle statuses.
const (
	FlowRunStatusQueued    = "QUEUED"
	FlowRunStatusRunning   = "RUNNING"
	FlowRunStatusWaiting   = "WAITING"
	FlowRunStatusCompleted = "COMPLETED"
	FlowRunStatusFailed    = "FAILED"
	FlowRunStatusCancelled = "CANCELLED"
)

// FlowSchedule is a persisted crontab rule that starts a workflow.
type FlowSchedule struct {
	ID            string `gorm:"primaryKey;size:64" json:"id"`
	Name          string `gorm:"not null" json:"name"`
	Description   string `gorm:"type:text" json:"description"`
	CronExpr      string `gorm:"column:cron_expr;not null" json:"cron_expr"`
	FlowID        string `gorm:"column:flow_id;not null;index" json:"flow_id"`
	InitialData   string `gorm:"column:initial_data;type:text" json:"initial_data"`
	Enabled       bool   `gorm:"not null" json:"enabled"`
	TimeoutSec    int    `gorm:"column:timeout_sec;not null;default:3600" json:"timeout_sec"`
	NextRunAt     *int64 `gorm:"column:next_run_at;index" json:"next_run_at"`
	LastRunID     string `gorm:"column:last_run_id" json:"last_run_id"`
	LastRunStatus string `gorm:"column:last_run_status" json:"last_run_status"`
	Timestamps
}

func (FlowSchedule) TableName() string { return "flow_schedule" }

// FlowRunRecord is one execution attempt of a flow, scheduled or on demand.
// ScheduleID is empty for runs that were not started by a schedule.
type FlowRunRecord struct {
	ID           string `gorm:"primaryKey;size:96" json:"id"`
	ScheduleID   string `gorm:"column:schedule_id;index" json:"schedule_id"`
	FlowID       string `gorm:"column:flow_id;not null;index" json:"flow_id"`
	TriggerType  string `gorm:"column:trigger_type" json:"trigger_type"`
	TriggerKey   string `gorm:"column:trigger_key" json:"trigger_key"`
	Status       string `gorm:"not null;index" json:"status"`
	CurrentNode  string `gorm:"column:current_node" json:"current_node"`
	NodeAttempts string `gorm:"column:node_attempts;type:text" json:"node_attempts"`
	NodeOutputs  string `gorm:"column:node_outputs;type:text" json:"node_outputs"`
	Error        string `gorm:"column:error;type:text" json:"error"`
	StartedAt    *int64 `gorm:"column:started_at" json:"started_at"`
	CompletedAt  *int64 `gorm:"column:completed_at" json:"completed_at"`
	Timestamps
}

func (FlowRunRecord) TableName() string { return "flow_run_record" }
