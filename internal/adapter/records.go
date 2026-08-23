// Package adapter converts scraper and interceptor payloads into database models.
package adapter

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

// BrowseHistoryInfo carries the information needed to record a browse history entry.
type BrowseHistoryInfo struct {
	PlatformId        string
	AccountExternalId string
	Account           *model.Account
	ContentType       string
	ContentTitle      string
	ContentURL        string
	ContentSourceURL  string
	ContentCoverURL   string
	ExtraData         map[string]any
	ExtraDataJSON     string
}

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
	AccountNickname   string      `json:"account_nickname"`
	AccountAvatarURL  string      `json:"account_avatar_url"`
	Raw               interface{} `json:"raw"`
}

type BrowseRecorder interface {
	RecordBrowseHistory(uniqueMark string, info BrowseHistoryInfo) error
}

type RecordResult struct {
	AccountReady      bool
	AccountExternalID string
}

func RecordLoadedProfile(db *gorm.DB, recorder BrowseRecorder, logger zerolog.Logger, profile *PlatformBrowserProfile) RecordResult {
	result := RecordResult{}
	if profile == nil || profile.PlatformId == "" || profile.ContentExternalId == "" {
		return result
	}

	result.AccountExternalID = strings.TrimSpace(profile.AccountExternalId)

	if db != nil && result.AccountExternalID != "" {
		result.AccountReady = UpsertAccount(db, logger, profile, result.AccountExternalID)
	}
	if recorder != nil {
		RecordBrowse(recorder, logger, profile, result.AccountExternalID)
	}
	return result
}

func UpsertAccount(db *gorm.DB, logger zerolog.Logger, profile *PlatformBrowserProfile, accountExternalID string) bool {
	now := util.NowMillis()
	acc := model.Account{
		Id:         profile.PlatformId + ":" + accountExternalID,
		PlatformId: profile.PlatformId,
		ExternalId: accountExternalID,
		Nickname:   profile.AccountNickname,
		AvatarURL:  profile.AccountAvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	transaction_err := db.Transaction(func(tx *gorm.DB) error {
		var existingAccount model.Account
		find_err := tx.
			Where("platform_id = ? AND external_id = ?", profile.PlatformId, accountExternalID).
			First(&existingAccount).Error
		if errors.Is(find_err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&acc).Error; err != nil {
				return fmt.Errorf("create platform account: %w", err)
			}
			return nil
		}
		if find_err != nil {
			return fmt.Errorf("find platform account: %w", find_err)
		}

		updates := map[string]any{
			"nickname":   profile.AccountNickname,
			"updated_at": now,
		}
		avatar_changed, err := existingAccount.ApplyObservedAvatarURL(profile.AccountAvatarURL)
		if err != nil {
			return err
		}
		if avatar_changed {
			updates["avatar_url"] = existingAccount.AvatarURL
			updates["past_avatars"] = existingAccount.PastAvatars
		}
		if err := tx.Model(&existingAccount).Updates(updates).Error; err != nil {
			return fmt.Errorf("update platform account: %w", err)
		}
		return nil
	})
	if transaction_err != nil {
		logger.Error().
			Err(transaction_err).
			Str("platform_id", profile.PlatformId).
			Str("account_external_id", accountExternalID).
			Msg("upsert platform account failed")
		return false
	}
	return true
}

func RecordBrowse(recorder BrowseRecorder, logger zerolog.Logger, profile *PlatformBrowserProfile, accountExternalID string) {
	now := util.NowMillis()
	account := &model.Account{
		Id:         profile.PlatformId + ":" + accountExternalID,
		PlatformId: profile.PlatformId,
		ExternalId: accountExternalID,
		Nickname:   profile.AccountNickname,
		AvatarURL:  profile.AccountAvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := recorder.RecordBrowseHistory(profile.ContentExternalId, BrowseHistoryInfo{
		PlatformId:        profile.PlatformId,
		AccountExternalId: accountExternalID,
		Account:           account,
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
