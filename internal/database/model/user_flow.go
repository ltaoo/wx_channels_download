package model

// UserFlow is a user-defined pipeline persisted as a JSON flow definition.
// The definition is registered into the flow engine so schedules can trigger
// it like any built-in flow.
type UserFlow struct {
	ID          string `gorm:"primaryKey;size:96" json:"id"`
	Name        string `gorm:"not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	// TriggerType and EventKey retain the legacy single-trigger configuration.
	// The start-node editor stores multiple rules in FlowSchedule and clears
	// these legacy fields on save. Manual execution is always available.
	TriggerType string `gorm:"column:trigger_type;not null;default:Cron" json:"trigger_type"`
	EventKey    string `gorm:"column:event_key" json:"event_key"`
	// Definition is the serialized engine.FlowDefinition (nodes, edges,
	// context schema).
	Definition string `gorm:"column:definition;type:text" json:"definition"`
	Timestamps
}

func (UserFlow) TableName() string { return "user_flow" }
