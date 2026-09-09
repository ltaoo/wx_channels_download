package model

type Tag struct {
	Id   int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name string `gorm:"not null;uniqueIndex:idx_tag_name" json:"name"`
	Timestamps
}

func (Tag) TableName() string { return "tag" }

// ContentTag is a many-to-many link between a content item and a tag. The
// compound primary key keeps the association idempotent for repeated writes.
type ContentTag struct {
	ContentId string `gorm:"primaryKey" json:"content_id"`
	TagId     int    `gorm:"primaryKey" json:"tag_id"`
	CreatedAt int64  `json:"created_at"`
}

func (ContentTag) TableName() string { return "content_tag" }

// AccountTag mirrors ContentTag for the account entity.
type AccountTag struct {
	AccountId string `gorm:"primaryKey" json:"account_id"`
	TagId     int    `gorm:"primaryKey" json:"tag_id"`
	CreatedAt int64  `json:"created_at"`
}

func (AccountTag) TableName() string { return "account_tag" }
