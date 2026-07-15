package wxchannels

import (
	"errors"
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/util"
)

// BrowseRecordInfo carries the information needed to record a browse history entry.
type BrowseRecordInfo struct {
	PlatformId        string
	AccountExternalId string
	AccountUsername   string
	AccountNickname   string
	AccountAvatarURL  string
	ContentType       string
	ContentTitle      string
	ContentURL        string
	ContentSourceURL  string
	ContentCoverURL   string
	ExtraData         map[string]any
	ExtraDataJSON     string
}

// HandleFeedProfileLoaded upserts the account for a wechat channels feed profile.
func HandleFeedProfileLoaded(db *gorm.DB, logger zerolog.Logger, profile *scraper.MediaProfile) {
	if profile == nil || profile.Id == "" {
		return
	}
	accountUsername := strings.TrimSpace(profile.Contact.Id)
	if db != nil && accountUsername != "" {
		upsertChannelsAccount(db, logger, profile, accountUsername)
	}
}

func upsertChannelsAccount(db *gorm.DB, logger zerolog.Logger, profile *scraper.MediaProfile, accountUsername string) {
	now := util.NowMillis()
	acc := model.Account{
		PlatformId: platformIDWxChannels,
		ExternalId: accountUsername,
		Username:   accountUsername,
		Nickname:   profile.Contact.Nickname,
		AvatarURL:  profile.Contact.AvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	var existingAccount model.Account
	if err := db.Where("platform_id = ? AND external_id = ?", platformIDWxChannels, accountUsername).First(&existingAccount).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&acc).Error; err != nil {
				logger.Error().Err(err).Str("platform_id", platformIDWxChannels).Str("username", accountUsername).Msg("create account failed")
			}
		} else {
			logger.Error().Err(err).Str("platform_id", platformIDWxChannels).Str("username", accountUsername).Msg("find account failed")
		}
		return
	}
	if err := db.Model(&existingAccount).Updates(map[string]any{
		"username":   accountUsername,
		"nickname":   profile.Contact.Nickname,
		"avatar_url": profile.Contact.AvatarURL,
		"updated_at": now,
	}).Error; err != nil {
		logger.Error().Err(err).Int("account_id", existingAccount.Id).Msg("update account failed")
	}
}

// CreateBrowseRecord builds a BrowseRecordInfo from the feed profile.
func CreateBrowseRecord(profile *scraper.MediaProfile) (uniqueMark string, info BrowseRecordInfo) {
	accountUsername := strings.TrimSpace(profile.Contact.Id)
	return profile.Id, BrowseRecordInfo{
		PlatformId:        platformIDWxChannels,
		AccountExternalId: accountUsername,
		AccountUsername:   accountUsername,
		AccountNickname:   profile.Contact.Nickname,
		AccountAvatarURL:  profile.Contact.AvatarURL,
		ContentType:       profile.Type,
		ContentTitle:      profile.Title,
		ContentURL:        profile.URL,
		ContentSourceURL:  profile.Pageurl,
		ContentCoverURL:   profile.CoverURL,
		ExtraData: map[string]any{
			"id":         profile.Id,
			"nonce_id":   profile.NonceId,
			"decode_key": profile.Key,
		},
	}
}
