package officialaccount

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/api/services"
	"wx_channel/internal/database/model"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/util"
)

// HandleArticleProfileLoaded upserts the account for a wechat official account article profile.
func HandleArticleProfileLoaded(db *gorm.DB, logger zerolog.Logger, profile *interceptor.OfficialAccountArticleProfile) {
	if profile == nil || profile.UniqueMark == "" {
		return
	}
	accountExternalID := strings.TrimSpace(profile.Biz)
	accountUsername := strings.TrimSpace(profile.Username)
	if accountExternalID == "" {
		accountExternalID = accountUsername
	}
	if db != nil && accountExternalID != "" {
		upsertOfficialAccount(db, logger, profile, accountExternalID, accountUsername)
	}
}

func upsertOfficialAccount(db *gorm.DB, logger zerolog.Logger, profile *interceptor.OfficialAccountArticleProfile, accountExternalID, accountUsername string) {
	now := util.NowMillis()
	acc := model.Account{
		PlatformId: platformIDOfficialAccount,
		ExternalId: accountExternalID,
		Username:   accountUsername,
		Nickname:   profile.Nickname,
		AvatarURL:  profile.AvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	var existingAccount model.Account
	if err := db.Where("platform_id = ? AND external_id = ?", platformIDOfficialAccount, accountExternalID).First(&existingAccount).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&acc).Error; err != nil {
				logger.Error().Err(err).Str("platform_id", platformIDOfficialAccount).Str("account_external_id", accountExternalID).Msg("create official account failed")
			}
		} else {
			logger.Error().Err(err).Str("platform_id", platformIDOfficialAccount).Str("account_external_id", accountExternalID).Msg("find official account failed")
		}
		return
	}
	if err := db.Model(&existingAccount).Updates(map[string]any{
		"username":   accountUsername,
		"nickname":   profile.Nickname,
		"avatar_url": profile.AvatarURL,
		"updated_at": now,
	}).Error; err != nil {
		logger.Error().Err(err).Int("account_id", existingAccount.Id).Msg("update official account failed")
	}
}

// CreateBrowseHistory builds a BrowseHistoryInfo from the article profile.
func CreateBrowseHistory(profile *interceptor.OfficialAccountArticleProfile) services.BrowseHistoryInfo {
	accountExternalID := strings.TrimSpace(profile.Biz)
	accountUsername := strings.TrimSpace(profile.Username)
	if accountExternalID == "" {
		accountExternalID = accountUsername
	}

	extraDataBytes, _ := json.Marshal(map[string]any{
		"biz":        profile.Biz,
		"username":   profile.Username,
		"mid":        profile.Mid,
		"idx":        profile.Idx,
		"sn":         profile.Sn,
		"cgiDataNew": profile.RawCgiDataNew,
	})

	return services.BrowseHistoryInfo{
		PlatformId:        platformIDOfficialAccount,
		AccountExternalId: accountExternalID,
		AccountUsername:   accountUsername,
		AccountNickname:   profile.Nickname,
		AccountAvatarURL:  profile.AvatarURL,
		ContentType:       "article",
		ContentTitle:      profile.Title,
		ContentURL:        profile.URL,
		ContentSourceURL:  profile.SourceURL,
		ContentCoverURL:   profile.CoverURL,
		ExtraDataJSON:     string(extraDataBytes),
	}
}
