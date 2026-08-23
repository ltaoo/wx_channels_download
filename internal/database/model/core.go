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
	Id                 int     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name               string  `gorm:"not null;index:idx_influencer_name" json:"name"`
	Alias              string  `json:"alias"`
	AvatarURL          string  `json:"avatar_url"`
	Sex                int     `json:"sex"`
	Description        string  `json:"description"`
	Biography          string  `json:"biography"`
	ProfilePath        string  `json:"profile_path"`
	Birthday           string  `json:"birthday"`
	PlaceOfBirth       string  `json:"place_of_birth"`
	KnownForDepartment string  `json:"known_for_department"`
	Profile            string  `json:"profile"`
	TMDBId             *string `gorm:"uniqueIndex:idx_influencer_tmdb_id" json:"tmdb_id,omitempty"`
	DoubanId           *string `gorm:"uniqueIndex:idx_influencer_douban_id" json:"douban_id,omitempty"`
	IMDBId             *string `gorm:"uniqueIndex:idx_influencer_imdb_id" json:"imdb_id,omitempty"`
	MetadataJSON       string  `json:"metadata_json"`
	Timestamps
}

func (Influencer) TableName() string { return "influencer" }

type Account struct {
	Id            string `gorm:"primaryKey" json:"id"`
	PlatformId    string `gorm:"not null;index:idx_account_platform_external,priority:1" json:"platform_id"`
	InfluencerId  *int   `json:"influencer_id"`
	ExternalId    string `gorm:"not null;index:idx_account_platform_external,priority:2" json:"external_id"`
	Alias         string `json:"alias"`
	Nickname      string `json:"nickname"`
	Signature     string `json:"signature"`
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
	if a.PastAvatars == "" {
		a.PastAvatars = "[]"
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
	Id           string `gorm:"primaryKey" json:"id"`
	PlatformId   string `gorm:"not null" json:"platform_id"`
	VisitedTimes int64  `gorm:"not null" json:"visited_times"`
	Type         string `json:"type"`
	ExternalId   string `json:"external_id"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	SourceURL    string `json:"source_url"`
	CoverURL     string `json:"cover_url"`
	CoverWidth   string `json:"cover_width"`
	CoverHeight  string `json:"cover_height"`
	PublishTime  *int64 `json:"publish_time"`
	ExtraData    string `json:"extra_data"`
	Timestamps
}

func (BrowseHistory) TableName() string { return "browse_history" }

type BrowseHistoryAccount struct {
	BrowseHistoryId string `gorm:"primaryKey" json:"browse_history_id"`
	AccountId       string `gorm:"primaryKey" json:"account_id"`
	Role            string `json:"role"`
	CreatedAt       int64  `json:"created_at"`
}

func (BrowseHistoryAccount) TableName() string { return "browse_history_account" }

func (b *BrowseHistory) Upsert(db *gorm.DB) error {
	if db == nil {
		return errors.New("db is nil")
	}
	if b.PlatformId == "" {
		return errors.New("missing platform_id")
	}
	if b.ExternalId == "" {
		return errors.New("missing external_id")
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
	err := db.Where("platform_id = ? AND external_id = ?", b.PlatformId, b.ExternalId).First(&existing).Error
	if err == nil {
		return db.Model(&existing).UpdateColumns(map[string]any{
			"visited_times": existing.VisitedTimes + 1,
			"type":          b.Type,
			"title":         b.Title,
			"url":           b.URL,
			"source_url":    b.SourceURL,
			"cover_url":     b.CoverURL,
			"cover_width":   b.CoverWidth,
			"cover_height":  b.CoverHeight,
			"publish_time":  b.PublishTime,
			"extra_data":    b.ExtraData,
			"updated_at":    b.UpdatedAt,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if b.Id == "" {
		b.Id = b.PlatformId + ":" + b.ExternalId + ":" + strconv.FormatInt(b.CreatedAt, 10)
	}
	return db.Create(b).Error
}
