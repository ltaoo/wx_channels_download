// Package adapter converts scraper and interceptor payloads into database models.
package adapter

import (
	"errors"
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/api/services"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

// PlatformBrowserProfile is a common data structure extracted by the interceptor
// when a user browses a platform page. Platform adapters translate their
// scraper-specific payloads into this shared shape for persistence.
type PlatformBrowserProfile struct {
	PlatformId        string      `json:"platform_id"`
	PlatformName      string      `json:"platform_name"`
	ContentExternalId string      `json:"content_external_id"`
	ContentType       string      `json:"content_type"`
	ContentTitle      string      `json:"content_title"`
	ContentURL        string      `json:"content_url"`
	ContentSourceURL  string      `json:"content_source_url"`
	ContentCoverURL   string      `json:"content_cover_url"`
	AccountExternalId string      `json:"account_external_id"`
	AccountUsername   string      `json:"account_username"`
	AccountNickname   string      `json:"account_nickname"`
	AccountAvatarURL  string      `json:"account_avatar_url"`
	Raw               interface{} `json:"raw"`
}

type BrowseRecorder interface {
	RecordBrowseHistory(uniqueMark string, info services.BrowseHistoryInfo) error
}

type RecordResult struct {
	AccountReady      bool
	AccountExternalID string
	AccountUsername   string
}

func RecordLoadedProfile(db *gorm.DB, recorder BrowseRecorder, logger zerolog.Logger, profile *PlatformBrowserProfile) RecordResult {
	result := RecordResult{}
	if profile == nil || profile.PlatformId == "" || profile.ContentExternalId == "" {
		return result
	}

	result.AccountExternalID = strings.TrimSpace(profile.AccountExternalId)
	result.AccountUsername = strings.TrimSpace(profile.AccountUsername)
	if result.AccountExternalID == "" {
		result.AccountExternalID = result.AccountUsername
	}

	if db != nil && result.AccountExternalID != "" {
		result.AccountReady = UpsertAccount(db, logger, profile, result.AccountExternalID, result.AccountUsername)
	}
	if recorder != nil {
		RecordBrowse(recorder, logger, profile, result.AccountExternalID, result.AccountUsername)
	}
	return result
}

func UpsertAccount(db *gorm.DB, logger zerolog.Logger, profile *PlatformBrowserProfile, accountExternalID, accountUsername string) bool {
	now := util.NowMillis()
	acc := model.Account{
		Id:         profile.PlatformId + ":" + accountExternalID,
		PlatformId: profile.PlatformId,
		ExternalId: accountExternalID,
		Username:   accountUsername,
		Nickname:   profile.AccountNickname,
		AvatarURL:  profile.AccountAvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	var existingAccount model.Account
	if err := db.Where("platform_id = ? AND external_id = ?", profile.PlatformId, accountExternalID).First(&existingAccount).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&acc).Error; err != nil {
				logger.Error().Err(err).Str("platform_id", profile.PlatformId).Str("account_external_id", accountExternalID).Msg("create platform account failed")
				return false
			}
			return true
		}
		logger.Error().Err(err).Str("platform_id", profile.PlatformId).Str("account_external_id", accountExternalID).Msg("find platform account failed")
		return false
	}

	if err := db.Model(&existingAccount).Updates(map[string]any{
		"username":   accountUsername,
		"nickname":   profile.AccountNickname,
		"avatar_url": profile.AccountAvatarURL,
		"updated_at": now,
	}).Error; err != nil {
		logger.Error().Err(err).Str("account_id", existingAccount.Id).Msg("update platform account failed")
	}
	return true
}

func RecordBrowse(recorder BrowseRecorder, logger zerolog.Logger, profile *PlatformBrowserProfile, accountExternalID, accountUsername string) {
	if err := recorder.RecordBrowseHistory(profile.ContentExternalId, services.BrowseHistoryInfo{
		PlatformId:        profile.PlatformId,
		AccountExternalId: accountExternalID,
		AccountUsername:   accountUsername,
		AccountNickname:   profile.AccountNickname,
		AccountAvatarURL:  profile.AccountAvatarURL,
		ContentType:       profile.ContentType,
		ContentTitle:      profile.ContentTitle,
		ContentURL:        profile.ContentURL,
		ContentSourceURL:  profile.ContentSourceURL,
		ContentCoverURL:   profile.ContentCoverURL,
		ExtraData: map[string]any{
			"platform_name": profile.PlatformName,
			"raw":           profile.Raw,
		},
	}); err != nil {
		logger.Error().Err(err).Str("platform_id", profile.PlatformId).Str("content_external_id", profile.ContentExternalId).Msg("create platform browse history failed")
	}
}
