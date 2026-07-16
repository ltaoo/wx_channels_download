package model

import (
	"errors"
	"strconv"

	"gorm.io/gorm"
)

type Platform struct {
	Id       string `gorm:"primaryKey" json:"id"`
	Code     string `gorm:"uniqueIndex;not null" json:"code"`
	Name     string `gorm:"not null" json:"name"`
	Homepage string `json:"homepage"`
	LogoURL  string `json:"logo_url"`
	EntryURL string `json:"entry_url"`
	Timestamps
}

func (Platform) TableName() string { return "platform" }

type AuthCredential struct {
	Id         int    `gorm:"primaryKey;autoIncrement" json:"id"`
	PlatformId string `gorm:"not null;index:idx_auth_credential_platform_status_default,priority:1" json:"platform_id"`
	Name       string `gorm:"not null" json:"name"`
	Kind       string `gorm:"not null" json:"kind"`
	Secret     string `json:"secret"`
	Payload    string `json:"payload"`
	ExpiresAt  *int64 `json:"expires_at"`
	Status     int    `gorm:"not null;index:idx_auth_credential_platform_status_default,priority:2" json:"status"`
	IsDefault  int    `gorm:"not null;index:idx_auth_credential_platform_status_default,priority:3" json:"is_default"`
	LastUsedAt *int64 `json:"last_used_at"`
	Timestamps
}

func (AuthCredential) TableName() string { return "auth_credential" }

type Influencer struct {
	Id          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"not null" json:"name"`
	AvatarURL   string `json:"avatar_url"`
	Sex         int    `json:"sex"`
	Description string `json:"description"`
	Timestamps
}

func (Influencer) TableName() string { return "influencer" }

type Account struct {
	Id            string `gorm:"primaryKey" json:"id"`
	PlatformId    string `gorm:"not null;index:idx_account_platform_external,priority:1" json:"platform_id"`
	InfluencerId  *int   `json:"influencer_id"`
	ExternalId    string `gorm:"not null;index:idx_account_platform_external,priority:2" json:"external_id"`
	Username      string `json:"username"`
	Alias         string `json:"alias"`
	Nickname      string `json:"nickname"`
	AvatarURL     string `json:"avatar_url"`
	ProfileURL    string `json:"profile_url"`
	IsListen      int    `json:"is_listen"`
	FollowerCount int64  `json:"follower_count"`
	PastNames     string `json:"past_names"`
	PastAvatars   string `json:"past_avatars"`
	Timestamps
}

func (Account) TableName() string { return "account" }

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.Id == "" {
		a.Id = a.PlatformId + ":" + a.ExternalId
	}
	return nil
}

type WXVideoAccess struct {
	Id          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	AccountId   string `gorm:"not null;uniqueIndex:idx_wx_video_access_account_url,priority:1" json:"account_id"`
	URL         string `gorm:"not null;uniqueIndex:idx_wx_video_access_account_url,priority:2" json:"url"`
	Description string `json:"description"`
	CoverURL    string `json:"cover_url"`
	Timestamps
}

func (WXVideoAccess) TableName() string { return "wx_video_access" }

type BrowseHistory struct {
	Id                string  `gorm:"primaryKey" json:"id"`
	PlatformId        string  `gorm:"not null" json:"platform_id"`
	VisitedTimes      int64   `gorm:"not null" json:"visited_times"`
	AccountId         *string `json:"account_id"`
	InfluencerId      *int    `json:"influencer_id"`
	AccountExternalId string  `json:"account_external_id"`
	AccountUsername   string  `json:"account_username"`
	AccountNickname   string  `json:"account_nickname"`
	AccountAvatarURL  string  `json:"account_avatar_url"`
	ContentId         *string `json:"content_id"`
	ContentType       string `json:"content_type"`
	ContentExternalId string `json:"content_external_id"`
	ContentTitle      string `json:"content_title"`
	ContentURL        string `json:"content_url"`
	ContentSourceURL  string `json:"content_source_url"`
	ContentCoverURL   string `json:"content_cover_url"`
	ExtraData         string `json:"extra_data"`
	Timestamps
}

func (BrowseHistory) TableName() string { return "browse_history" }

func (b *BrowseHistory) Upsert(db *gorm.DB) error {
	if db == nil {
		return errors.New("db is nil")
	}
	if b.PlatformId == "" {
		return errors.New("missing platform_id")
	}
	if b.ContentExternalId == "" {
		return errors.New("missing content_external_id")
	}
	if b.VisitedTimes <= 0 {
		b.VisitedTimes = 1
	}
	if b.CreatedAt == 0 {
		return errors.New("missing created_at")
	}
	if b.UpdatedAt == 0 {
		b.UpdatedAt = b.CreatedAt
	}

	var existing BrowseHistory
	err := db.Where("platform_id = ? AND content_external_id = ?", b.PlatformId, b.ContentExternalId).First(&existing).Error
	if err == nil {
		return db.Model(&existing).UpdateColumns(map[string]any{
			"visited_times":       existing.VisitedTimes + 1,
			"account_external_id": b.AccountExternalId,
			"account_username":    b.AccountUsername,
			"account_nickname":    b.AccountNickname,
			"account_avatar_url":  b.AccountAvatarURL,
			"content_type":        b.ContentType,
			"content_title":       b.ContentTitle,
			"content_url":         b.ContentURL,
			"content_source_url":  b.ContentSourceURL,
			"content_cover_url":   b.ContentCoverURL,
			"extra_data":          b.ExtraData,
			"updated_at":          b.UpdatedAt,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if b.Id == "" {
		b.Id = b.PlatformId + ":" + b.ContentExternalId + ":" + strconv.FormatInt(b.CreatedAt, 10)
	}
	return db.Create(b).Error
}
