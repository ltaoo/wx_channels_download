package model

type PlatformWorkflowRun struct {
	ID             string `gorm:"primaryKey;size:96" json:"id"`
	URL            string `gorm:"not null" json:"url"`
	Platform       string `gorm:"index" json:"platform"`
	Status         string `gorm:"index" json:"status"`
	CurrentNode    string `gorm:"column:current_node" json:"current_node"`
	TaskID         string `gorm:"column:task_id;index" json:"task_id"`
	DownloadTaskID int    `gorm:"column:download_task_id;index" json:"download_task_id"`
	Extra          string `gorm:"column:extra;type:text" json:"extra"`
	Output         string `gorm:"column:output;type:text" json:"output"`
	Selection      string `gorm:"column:selection;type:text" json:"selection"`
	Nodes          string `gorm:"column:nodes;type:text" json:"nodes"`
	Error          string `gorm:"column:error;type:text" json:"error"`
	Timestamps
}

func (PlatformWorkflowRun) TableName() string { return "platform_workflow_run" }
