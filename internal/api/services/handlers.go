package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
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

func (s *ChannelsUploadService) HandleChannelsFeed(feed *apitypes.ChannelsFeedProfile) (*model.Video, error) {
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

	// Upsert Video by (platform_id, external_id1=objectId)
	var video model.Video
	err = s.db.Where("platform_id = ? AND external_id1 = ?", platformId, feed.ObjectId).First(&video).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		video = model.Video{
			PlatformId:  platformId,
			ExternalId1: feed.ObjectId,
			ExternalId2: feed.NonceId,
			Title:       feed.Title,
			URL:         feed.URL,
			SourceURL:   feed.SourceURL,
			CoverURL:    feed.CoverURL,
			Duration:    int64(feed.Duration),
			Size:        int64(feed.FileSize),
			PublishTime: int64(feed.CreatedAt),
			Timestamps: model.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		if err := s.db.Create(&video).Error; err != nil {
			return nil, err
		}
	} else {
		updates := map[string]any{
			"title":      feed.Title,
			"url":        feed.URL,
			"source_url": feed.SourceURL,
			"cover_url":  feed.CoverURL,
			"updated_at": now,
		}
		if err := s.db.Model(&video).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	// Create VideoAccount if new
	if account.Id == 0 {
		return &video, nil
	}
	if err := s.db.Where("video_id = ? AND account_id <> ? AND role = ?", video.Id, account.Id, "owner").Delete(&model.VideoAccount{}).Error; err != nil {
		return nil, err
	}
	var va model.VideoAccount
	err = s.db.Where("video_id = ? AND account_id = ?", video.Id, account.Id).First(&va).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		va = model.VideoAccount{
			VideoId:   video.Id,
			AccountId: account.Id,
			Role:      "owner",
		}
		if err := s.db.Create(&va).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if va.Role != "owner" {
		if err := s.db.Model(&model.VideoAccount{}).Where("video_id = ? AND account_id = ?", video.Id, account.Id).Update("role", "owner").Error; err != nil {
			return nil, err
		}
	}

	return &video, nil
}

func (s *ChannelsUploadService) CreateDownloadTaskWithVideo(video *model.Video, t *downloadpkg.Task, reason string) (*model.DownloadTask, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if video == nil {
		return nil, fmt.Errorf("video is nil")
	}
	if t == nil {
		return nil, fmt.Errorf("download task is nil")
	}

	title := ""
	if t.Meta != nil && t.Meta.Opts != nil {
		title = t.Meta.Opts.Name
	}
	if title == "" {
		title = video.Title
	}

	taskURL := video.URL
	if taskURL == "" && t.Meta != nil && t.Meta.Req != nil {
		taskURL = t.Meta.Req.URL
	}

	meta2Bytes, _ := json.Marshal(map[string]any{
		"platform":    video.PlatformId,
		"external_id": video.ExternalId1,
		"nonce_id":    video.ExternalId2,
		"eid":         "",
	})

	statusToInt := func(st base.Status) int {
		switch st {
		case base.DownloadStatusReady:
			return 0
		case base.DownloadStatusRunning:
			return 1
		case base.DownloadStatusPause:
			return 2
		case base.DownloadStatusWait:
			return 3
		case base.DownloadStatusDone:
			return 4
		case base.DownloadStatusError:
			return 5
		default:
			return 0
		}
	}

	var rec model.DownloadTask
	err := s.db.Where("task_id = ?", t.ID).First(&rec).Error
	updates := map[string]any{
		"url":         taskURL,
		"external_id": video.ExternalId1,
		"title":       title,
		"cover_url":   video.CoverURL,
		"metadata2":   string(meta2Bytes),
		"reason":      reason,
		"updated_at":  utilpkg.TimeToMillisInt64(t.UpdatedAt),
	}

	if err == nil {
		if err := s.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(updates).Error; err != nil {
			return nil, err
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		rec = model.DownloadTask{
			TaskId:     t.ID,
			Status:     statusToInt(t.Status),
			Protocol:   t.Protocol,
			URL:        taskURL,
			ExternalId: video.ExternalId1,
			Title:      title,
			CoverURL:   video.CoverURL,
			Reason:     reason,
			Metadata2:  string(meta2Bytes),
			Timestamps: model.Timestamps{
				CreatedAt: utilpkg.TimeToMillisInt64(t.CreatedAt),
				UpdatedAt: utilpkg.TimeToMillisInt64(t.UpdatedAt),
			},
		}
		if err := s.db.Create(&rec).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	// link Video to DownloadTask
	taskId := rec.Id
	if err := s.db.Model(&model.Video{}).Where("id = ?", video.Id).Update("download_task_id", taskId).Error; err != nil {
		return &rec, err
	}

	return &rec, nil
}
