package wxchannels

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/util"
)

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
		Id:         BuildAccountID(accountUsername),
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
		logger.Error().Err(err).Str("account_id", existingAccount.Id).Msg("update account failed")
	}
}

// BuildBrowseRecord constructs a model.BrowseHistory from the feed profile.
func BuildBrowseRecord(profile *scraper.MediaProfile) *model.BrowseHistory {
	accountUsername := strings.TrimSpace(profile.Contact.Id)
	now := util.NowMillis()
	extraData, _ := json.Marshal(map[string]any{
		"id":         profile.Id,
		"nonce_id":   profile.NonceId,
		"decode_key": profile.Key,
	})
	browseID := platformIDWxChannels + ":" + profile.Id
	contentSourceURL := profile.Pageurl
	if contentSourceURL == "" {
		contentSourceURL = BuildJumpURLFromParts(profile.Id, profile.NonceId, "", accountUsername)
	}

	return &model.BrowseHistory{
		Id:                browseID,
		PlatformId:        platformIDWxChannels,
		VisitedTimes:      1,
		AccountExternalId: accountUsername,
		AccountUsername:   accountUsername,
		AccountNickname:   profile.Contact.Nickname,
		AccountAvatarURL:  profile.Contact.AvatarURL,
		ContentType:       profile.Type,
		ContentExternalId: profile.Id,
		ContentTitle:      profile.Title,
		ContentURL:        profile.URL,
		ContentSourceURL:  contentSourceURL,
		ContentCoverURL:   profile.CoverURL,
		ExtraData:         string(extraData),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}
