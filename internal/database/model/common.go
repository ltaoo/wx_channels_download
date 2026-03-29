package model

type Timestamps struct {
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	DeletedAt *int64 `gorm:"column:deleted_at;index" json:"deleted_at"`
}
