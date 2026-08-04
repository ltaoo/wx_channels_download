package wxchannels

import (
	"encoding/json"
	"errors"
	"strconv"
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
		PlatformId: wxchannels,
		ExternalId: accountUsername,
		Nickname:   profile.Contact.Nickname,
		AvatarURL:  profile.Contact.AvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	var existingAccount model.Account
	if err := db.Where("platform_id = ? AND external_id = ?", wxchannels, accountUsername).First(&existingAccount).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&acc).Error; err != nil {
				logger.Error().Err(err).Str("platform_id", wxchannels).Str("external_id", accountUsername).Msg("create account failed")
			}
		} else {
			logger.Error().Err(err).Str("platform_id", wxchannels).Str("external_id", accountUsername).Msg("find account failed")
		}
		return
	}
	if err := db.Model(&existingAccount).Updates(map[string]any{
		"nickname":   profile.Contact.Nickname,
		"avatar_url": profile.Contact.AvatarURL,
		"updated_at": now,
	}).Error; err != nil {
		logger.Error().Err(err).Str("account_id", existingAccount.Id).Msg("update account failed")
	}
}

// BuildBrowseRecordFromObject constructs a model.BrowseHistory directly from a ChannelsObject,
// performing the conversion to browse record internally without an intermediate MediaProfile.
func BuildBrowseRecordFromObject(obj *scraper.ChannelsObject) *model.BrowseHistory {
	accountUsername := strings.TrimSpace(obj.Contact.Username)
	now := util.NowMillis()

	var key string
	if len(obj.ObjectDesc.Media) > 0 {
		key = obj.ObjectDesc.Media[0].DecodeKey
	}

	extraData, _ := json.Marshal(map[string]any{
		"id":         obj.ID,
		"nonce_id":   obj.ObjectNonceId,
		"decode_key": key,
	})

	browseID := wxchannels + ":" + obj.ID
	contentSourceURL := obj.SourceURL
	if contentSourceURL == "" {
		contentSourceURL = BuildJumpURLFromParts(obj.ID, obj.ObjectNonceId, "", accountUsername)
	}

	coverURL := ""
	coverWidth := ""
	coverHeight := ""
	if len(obj.ObjectDesc.Media) > 0 {
		media := obj.ObjectDesc.Media[0]
		coverURL = media.ThumbUrl
		coverWidth = strconv.Itoa(int(media.Width))
		coverHeight = strconv.Itoa(int(media.Height))
	}
	publishTime := int64(obj.CreateTime)

	return &model.BrowseHistory{
		Id:                browseID,
		PlatformId:        wxchannels,
		VisitedTimes:      1,
		AccountExternalId: accountUsername,
		AccountNickname:   obj.Contact.Nickname,
		AccountAvatarURL:  obj.Contact.HeadUrl,
		Type:              "video",
		ExternalId:        obj.ID,
		Title:             obj.ObjectDesc.Description,
		URL:               ObjectURL(obj),
		SourceURL:         contentSourceURL,
		CoverURL:          coverURL,
		CoverWidth:        coverWidth,
		CoverHeight:       coverHeight,
		PublishTime:       &publishTime,
		ExtraData:         string(extraData),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
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
	browseID := wxchannels + ":" + profile.Id
	contentSourceURL := profile.Pageurl
	if contentSourceURL == "" {
		contentSourceURL = BuildJumpURLFromParts(profile.Id, profile.NonceId, "", accountUsername)
	}

	return &model.BrowseHistory{
		Id:                browseID,
		PlatformId:        wxchannels,
		VisitedTimes:      1,
		AccountExternalId: accountUsername,
		AccountNickname:   profile.Contact.Nickname,
		AccountAvatarURL:  profile.Contact.AvatarURL,
		Type:              profile.Type,
		ExternalId:        profile.Id,
		Title:             profile.Title,
		URL:               profile.URL,
		SourceURL:         contentSourceURL,
		CoverURL:          profile.CoverURL,
		ExtraData:         string(extraData),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}
