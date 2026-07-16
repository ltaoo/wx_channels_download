package services

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
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

func (s *ChannelsUploadService) HandleChannelsFeed(content *model.Content, account *model.Account) (*model.Content, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if content == nil {
		return nil, fmt.Errorf("content is nil")
	}
	if account == nil {
		return nil, fmt.Errorf("account is nil")
	}

	platformId := "wx_channels"
	// Upsert Account by (platform_id, external_id=username)
	var existingAccount model.Account
	var err error
	if account.ExternalId != "" {
		err = s.db.Where("platform_id = ? AND external_id = ?", platformId, account.ExternalId).First(&existingAccount).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if account.Id == "" {
				account.Id = platformId + ":" + account.ExternalId
			}
			if err := s.db.Create(account).Error; err != nil {
				return nil, err
			}
			existingAccount = *account
		} else {
			updates := map[string]any{
				"username":   account.Username,
				"nickname":   account.Nickname,
				"avatar_url": account.AvatarURL,
				"updated_at": account.UpdatedAt,
			}
			if err := s.db.Model(&existingAccount).Updates(updates).Error; err != nil {
				return nil, err
			}
		}
	}

	var existing model.Content
	err = s.db.Where("platform_id = ? AND external_id = ?", platformId, content.ExternalId).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if content.Id == "" {
			content.Id = platformId + ":" + content.ExternalId
		}
		if err := s.db.Create(content).Error; err != nil {
			return nil, err
		}
	} else {
		content.Id = existing.Id
		updates := map[string]any{
			"content_type": content.ContentType,
			"external_id2": content.ExternalId2,
			"title":        content.Title,
			"content_url":  content.ContentURL,
			"url":          content.URL,
			"source_url":   content.SourceURL,
			"cover_url":    content.CoverURL,
			"duration":     content.Duration,
			"size":         content.Size,
			"publish_time": content.PublishTime,
			"updated_at":   content.UpdatedAt,
		}
		if err := s.db.Model(&model.Content{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	if existingAccount.Id == "" {
		return content, nil
	}
	if err := s.db.Where("content_id = ? AND account_id <> ? AND role = ?", content.Id, existingAccount.Id, "owner").Delete(&model.ContentAccount{}).Error; err != nil {
		return nil, err
	}
	var ca model.ContentAccount
	err = s.db.Where("content_id = ? AND account_id = ?", content.Id, existingAccount.Id).First(&ca).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ca = model.ContentAccount{
			ContentId: content.Id,
			AccountId: existingAccount.Id,
			Role:      "owner",
			CreatedAt: content.CreatedAt,
		}
		if err := s.db.Create(&ca).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if ca.Role != "owner" {
		if err := s.db.Model(&model.ContentAccount{}).Where("content_id = ? AND account_id = ?", content.Id, existingAccount.Id).Update("role", "owner").Error; err != nil {
			return nil, err
		}
	}

	return content, nil
}
