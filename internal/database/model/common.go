package model

type Timestamps struct {
	CreatedAt int64  `gorm:"autoCreateTime:milli" json:"created_at"`
	UpdatedAt int64  `gorm:"autoUpdateTime:milli" json:"updated_at"`
	DeletedAt *int64 `gorm:"column:deleted_at;index" json:"deleted_at"`
}
