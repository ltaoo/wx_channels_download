package model

type ChatSession struct {
	Id            int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name          string `gorm:"not null;index:idx_chat_session_platform_name,priority:2" json:"name"`
	Platform      string `gorm:"not null;index:idx_chat_session_platform_type,priority:1;index:idx_chat_session_platform_name,priority:1" json:"platform"`
	Type          string `gorm:"not null;index:idx_chat_session_platform_type,priority:2" json:"type"`
	GroupId       string `json:"group_id"`
	GroupAvatar   string `json:"group_avatar"`
	FormatVersion string `json:"format_version"`
	ExportedAt    *int64 `json:"exported_at"`
	Generator     string `json:"generator"`
	Description   string `json:"description"`
	ExtraData     string `json:"extra_data"`
	Timestamps
}

func (ChatSession) TableName() string { return "chat_session" }

type ChatMember struct {
	Id            int    `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionId     int    `gorm:"not null;uniqueIndex:idx_chat_member_session_platform_id,priority:1;index:idx_chat_member_session" json:"session_id"`
	PlatformId    string `gorm:"not null;uniqueIndex:idx_chat_member_session_platform_id,priority:2" json:"platform_id"`
	AccountName   string `gorm:"not null" json:"account_name"`
	GroupNickname string `json:"group_nickname"`
	Aliases       string `json:"aliases"`
	Avatar        string `json:"avatar"`
	Timestamps
}

func (ChatMember) TableName() string { return "chat_member" }

type ChatMessage struct {
	Id              int    `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionId       int    `gorm:"not null;uniqueIndex:idx_chat_message_session_source_message_id,priority:1;index:idx_chat_message_session_time,priority:1;index:idx_chat_message_session_sender_time,priority:1" json:"session_id"`
	SourceMessageId string `gorm:"uniqueIndex:idx_chat_message_session_source_message_id,priority:2" json:"source_message_id"`
	Sender          string `gorm:"not null;index:idx_chat_message_session_sender_time,priority:2" json:"sender"`
	AccountName     string `gorm:"not null" json:"account_name"`
	GroupNickname   string `json:"group_nickname"`
	Timestamp       int64  `gorm:"not null;index:idx_chat_message_session_time,priority:2;index:idx_chat_message_session_sender_time,priority:3" json:"timestamp"`
	Type            int    `gorm:"not null" json:"type"`
	Content         string `json:"content"`
	Payload         string `json:"payload"`
	Timestamps
}

func (ChatMessage) TableName() string { return "chat_message" }
