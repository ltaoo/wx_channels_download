package wxmp

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/wxmp"
	"wx_channel/pkg/util"
)

// HandleArticleProfileLoaded upserts the article publisher captured by the
// interceptor. Persistence belongs here rather than in the scraper package.
func HandleArticleProfileLoaded(db *gorm.DB, logger zerolog.Logger, profile *scraper.OfficialAccountArticleProfile) {
	if profile == nil || strings.TrimSpace(profile.UniqueMark) == "" || db == nil {
		return
	}

	accountExternalID := strings.TrimSpace(profile.Biz)
	accountUsername := strings.TrimSpace(profile.Username)
	if accountExternalID == "" {
		accountExternalID = accountUsername
	}
	if accountExternalID == "" {
		return
	}

	now := util.NowMillis()
	account := model.Account{
		Id:         BuildAccountID(accountExternalID),
		PlatformId: PlatformID,
		ExternalId: accountExternalID,
		Username:   accountUsername,
		Nickname:   profile.Nickname,
		AvatarURL:  profile.AvatarURL,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	var existing model.Account
	if err := db.Where("platform_id = ? AND external_id = ?", PlatformID, accountExternalID).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&account).Error; err != nil {
				logger.Error().Err(err).Str("platform_id", PlatformID).Str("account_external_id", accountExternalID).Msg("create official account failed")
			}
			return
		}
		logger.Error().Err(err).Str("platform_id", PlatformID).Str("account_external_id", accountExternalID).Msg("find official account failed")
		return
	}
	if err := db.Model(&existing).Updates(map[string]any{
		"username":   accountUsername,
		"nickname":   profile.Nickname,
		"avatar_url": profile.AvatarURL,
		"updated_at": now,
	}).Error; err != nil {
		logger.Error().Err(err).Str("account_id", existing.Id).Msg("update official account failed")
	}
}

// BuildBrowseRecord converts an intercepted official-account profile to the
// common browse-history event payload.
func BuildBrowseRecord(profile *scraper.OfficialAccountArticleProfile) *model.BrowseHistory {
	if profile == nil || strings.TrimSpace(profile.UniqueMark) == "" {
		return nil
	}
	accountExternalID := strings.TrimSpace(profile.Biz)
	accountUsername := strings.TrimSpace(profile.Username)
	if accountExternalID == "" {
		accountExternalID = accountUsername
	}
	extraData, _ := json.Marshal(map[string]any{
		"biz":        profile.Biz,
		"username":   profile.Username,
		"mid":        profile.Mid,
		"idx":        profile.Idx,
		"sn":         profile.Sn,
		"cgiDataNew": profile.RawCgiDataNew,
	})
	now := util.NowMillis()

	return &model.BrowseHistory{
		PlatformId:        PlatformID,
		VisitedTimes:      1,
		AccountExternalId: accountExternalID,
		AccountUsername:   accountUsername,
		AccountNickname:   profile.Nickname,
		AccountAvatarURL:  profile.AvatarURL,
		ContentType:       "article",
		ContentExternalId: profile.UniqueMark,
		ContentTitle:      profile.Title,
		ContentURL:        profile.URL,
		ContentSourceURL:  profile.SourceURL,
		ContentCoverURL:   profile.CoverURL,
		ExtraData:         string(extraData),
		Timestamps:        model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}
