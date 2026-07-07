package services

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	apitypes "wx_channel/internal/api/types"
	"wx_channel/internal/database/model"
	utilpkg "wx_channel/pkg/util"
)

type ChannelsUploadService struct {
	db     *gorm.DB
	logger *zerolog.Logger
}

func NewChannelsUploadService(db *gorm.DB, logger *zerolog.Logger) *ChannelsUploadService {
	return &ChannelsUploadService{
		db:     db,
		logger: logger,
	}
}

func (s *ChannelsUploadService) HandleChannelsFeed(feed *apitypes.ChannelsFeedProfile) (*model.Content, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if feed == nil {
		return nil, fmt.Errorf("feed is nil")
	}

	now := utilpkg.NowMillis()
	platformId := "wx_channels"
	accountIdentity := model.ResolveAccountIdentityFromBrowseHistory(s.db, platformId, feed.ObjectId, model.AccountIdentity{
		ExternalId: feed.Contact.Username,
		Username:   feed.Contact.Username,
		Nickname:   feed.Contact.Nickname,
		AvatarURL:  feed.Contact.AvatarURL,
	})

	// Upsert Account by (platform_id, external_id=username)
	var account model.Account
	var err error
	if accountIdentity.ExternalId != "" {
		err = s.db.Where("platform_id = ? AND external_id = ?", platformId, accountIdentity.ExternalId).First(&account).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			account = model.Account{
				PlatformId: platformId,
				ExternalId: accountIdentity.ExternalId,
				Username:   accountIdentity.Username,
				Nickname:   accountIdentity.Nickname,
				AvatarURL:  accountIdentity.AvatarURL,
				Timestamps: model.Timestamps{
					CreatedAt: now,
					UpdatedAt: now,
				},
			}
			if err := s.db.Create(&account).Error; err != nil {
				return nil, err
			}
		} else {
			updates := map[string]any{
				"username":   accountIdentity.Username,
				"nickname":   accountIdentity.Nickname,
				"avatar_url": accountIdentity.AvatarURL,
				"updated_at": now,
			}
			if err := s.db.Model(&account).Updates(updates).Error; err != nil {
				return nil, err
			}
		}
	}

	pub := int64(feed.CreatedAt)
	content := model.Content{
		PlatformId:  platformId,
		ContentType: "video",
		ExternalId:  feed.ObjectId,
		ExternalId2: feed.NonceId,
		Title:       feed.Title,
		ContentURL:  feed.URL,
		URL:         feed.URL,
		SourceURL:   feed.SourceURL,
		CoverURL:    feed.CoverURL,
		Duration:    int64(feed.Duration),
		Size:        int64(feed.FileSize),
		PublishTime: &pub,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	var existing model.Content
	err = s.db.Where("platform_id = ? AND external_id = ?", platformId, feed.ObjectId).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.db.Create(&content).Error; err != nil {
			return nil, err
		}
	} else {
		content.Id = existing.Id
		updates := map[string]any{
			"content_type": "video",
			"external_id2": content.ExternalId2,
			"title":        content.Title,
			"content_url":  content.ContentURL,
			"url":          content.URL,
			"source_url":   content.SourceURL,
			"cover_url":    content.CoverURL,
			"duration":     content.Duration,
			"size":         content.Size,
			"publish_time": content.PublishTime,
			"updated_at":   now,
		}
		if err := s.db.Model(&model.Content{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	if account.Id == 0 {
		return &content, nil
	}
	if err := s.db.Where("content_id = ? AND account_id <> ? AND role = ?", content.Id, account.Id, "owner").Delete(&model.ContentAccount{}).Error; err != nil {
		return nil, err
	}
	var ca model.ContentAccount
	err = s.db.Where("content_id = ? AND account_id = ?", content.Id, account.Id).First(&ca).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ca = model.ContentAccount{
			ContentId: content.Id,
			AccountId: account.Id,
			Role:      "owner",
			CreatedAt: now,
		}
		if err := s.db.Create(&ca).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if ca.Role != "owner" {
		if err := s.db.Model(&model.ContentAccount{}).Where("content_id = ? AND account_id = ?", content.Id, account.Id).Update("role", "owner").Error; err != nil {
			return nil, err
		}
	}

	return &content, nil
}
