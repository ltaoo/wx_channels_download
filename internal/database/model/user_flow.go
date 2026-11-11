package model

// UserFlowDefinition is the reusable graph and its live default configuration.
type UserFlowDefinition struct {
	ID                 string `gorm:"primaryKey;size:96" json:"id"`
	Name               string `gorm:"not null" json:"name"`
	Description        string `gorm:"type:text" json:"description"`
	Definition         string `gorm:"type:text" json:"definition"`
	DefaultNodeConfigs string `gorm:"type:text;not null;default:'{}'" json:"default_node_configs"`
	InstanceCount      int64  `gorm:"->;-:migration" json:"instance_count"`
	Timestamps
}

func (UserFlowDefinition) TableName() string { return "user_flow" }

// UserFlowInstance stores overrides and trigger settings, never a copy of the graph.
type UserFlowInstance struct {
	ID                 string `gorm:"primaryKey;size:96" json:"id"`
	FlowID             string `gorm:"not null;index" json:"flow_id"`
	Name               string `gorm:"not null" json:"name"`
	Description        string `gorm:"type:text" json:"description"`
	TriggerType        string `gorm:"not null;default:Manual" json:"trigger_type"`
	EventKey           string `json:"event_key"`
	NodeConfigs        string `gorm:"type:text;not null;default:'{}'" json:"node_configs"`
	Definition         string `gorm:"->;-:migration" json:"definition,omitempty"`
	DefinitionName     string `gorm:"->;-:migration" json:"definition_name"`
	DefaultNodeConfigs string `gorm:"->;-:migration" json:"default_node_configs"`
	ActiveRunID        string `gorm:"->;-:migration" json:"active_run_id"`
	Timestamps
}

func (UserFlowInstance) TableName() string { return "user_flow_instance" }

// Each execution attempt gets an independent, ordered log row.
type FlowRunNodeLog struct {
	ID     int64  `gorm:"primaryKey;autoIncrement;index:idx_flow_run_node_log_node,priority:4" json:"log_id"`
	RunID  string `gorm:"not null;index:idx_flow_run_node_log_node,priority:1" json:"run_id"`
	FlowID string `gorm:"not null;index:idx_flow_run_node_log_node,priority:2" json:"flow_id"`
	NodeID string `gorm:"not null;index:idx_flow_run_node_log_node,priority:3" json:"node_id"`
	Entry  string `gorm:"type:text;not null" json:"-"`
}

func (FlowRunNodeLog) TableName() string { return "flow_run_node_log" }
