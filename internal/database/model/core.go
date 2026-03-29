package model

import (
	"errors"

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
	Id            int    `gorm:"primaryKey;autoIncrement" json:"id"`
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

type Video struct {
	Id             int    `gorm:"primaryKey;autoIncrement" json:"id"`
	PlatformId     string `gorm:"not null;index:idx_video_platform_external,priority:1" json:"platform_id"`
	DownloadTaskId *int   `json:"download_task_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	ExternalId1    string `gorm:"index:idx_video_platform_external,priority:2" json:"external_id1"`
	ExternalId2    string `json:"external_id2"`
	ExternalId3    string `json:"external_id3"`
	Metadata       string `json:"metadata"`
	URL            string `json:"url"`
	SourceURL      string `json:"source_url"`
	CoverURL       string `json:"cover_url"`
	CoverWidth     string `json:"cover_width"`
	CoverHeight    string `json:"cover_height"`
	Size           int64  `json:"size"`
	Duration       int64  `json:"duration"`
	PublishTime    int64  `json:"publish_time"`
	PlayTimes      int64  `json:"play_times"`
	Unread         int    `json:"unread"`
	SourceDeleted  int    `json:"source_deleted"`
	Validated      int    `json:"validated"`
	Timestamps
}

func (Video) TableName() string { return "video" }

type VideoAccount struct {
	VideoId   int    `gorm:"primaryKey" json:"video_id"`
	AccountId int    `gorm:"primaryKey" json:"account_id"`
	Role      string `json:"role"`
	DeletedAt *int64 `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (VideoAccount) TableName() string { return "video_account" }

type VideoInfluencer struct {
	VideoId      int    `gorm:"primaryKey" json:"video_id"`
	InfluencerId int    `gorm:"primaryKey" json:"influencer_id"`
	Role         string `json:"role"`
	DeletedAt    *int64 `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (VideoInfluencer) TableName() string { return "video_influencer" }

type WXVideoAccess struct {
	Id          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	AccountId   int    `gorm:"not null;uniqueIndex:idx_wx_video_access_account_url,priority:1" json:"account_id"`
	URL         string `gorm:"not null;uniqueIndex:idx_wx_video_access_account_url,priority:2" json:"url"`
	Description string `json:"description"`
	CoverURL    string `json:"cover_url"`
	Timestamps
}

func (WXVideoAccess) TableName() string { return "wx_video_access" }

type BrowseHistory struct {
	Id                int    `gorm:"primaryKey;autoIncrement" json:"id"`
	PlatformId        string `gorm:"not null" json:"platform_id"`
	VisitedTimes      int64  `gorm:"not null" json:"visited_times"`
	AccountId         *int   `json:"account_id"`
	InfluencerId      *int   `json:"influencer_id"`
	AccountExternalId string `json:"account_external_id"`
	AccountUsername   string `json:"account_username"`
	AccountNickname   string `json:"account_nickname"`
	AccountAvatarURL  string `json:"account_avatar_url"`
	ContentId         *int   `json:"content_id"`
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
		existing.VisitedTimes = existing.VisitedTimes + 1
		existing.AccountExternalId = b.AccountExternalId
		existing.AccountUsername = b.AccountUsername
		existing.AccountNickname = b.AccountNickname
		existing.AccountAvatarURL = b.AccountAvatarURL
		existing.ContentType = b.ContentType
		existing.ContentTitle = b.ContentTitle
		existing.ContentURL = b.ContentURL
		existing.ContentSourceURL = b.ContentSourceURL
		existing.ContentCoverURL = b.ContentCoverURL
		existing.ExtraData = b.ExtraData
		existing.UpdatedAt = b.UpdatedAt
		return db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(b).Error
}
