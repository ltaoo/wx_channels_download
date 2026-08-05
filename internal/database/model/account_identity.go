package model

import (
	"strings"

	"gorm.io/gorm"
)

type AccountIdentity struct {
	ExternalId string
	Nickname   string
	AvatarURL  string
}

func ResolveAccountIdentityFromBrowseHistory(db *gorm.DB, platformId, contentExternalId string, fallback AccountIdentity) AccountIdentity {
	identity := AccountIdentity{
		ExternalId: strings.TrimSpace(fallback.ExternalId),
		Nickname:   strings.TrimSpace(fallback.Nickname),
		AvatarURL:  strings.TrimSpace(fallback.AvatarURL),
	}

	if db == nil || strings.TrimSpace(platformId) == "" || strings.TrimSpace(contentExternalId) == "" {
		return identity
	}

	type accountResult struct {
		ExternalId string
		Nickname   string
		AvatarURL  string
	}

	var result accountResult
	err := db.Table("browse_history").
		Select("account.external_id, account.nickname, account.avatar_url").
		Joins("JOIN browse_history_account ON browse_history_account.browse_history_id = browse_history.id").
		Joins("JOIN account ON account.id = browse_history_account.account_id").
		Where("browse_history.platform_id = ? AND browse_history.external_id = ? AND browse_history_account.account_id <> ''",
			strings.TrimSpace(platformId),
			strings.TrimSpace(contentExternalId),
		).
		Order("browse_history.updated_at DESC").
		Limit(1).
		Scan(&result).Error
	if err != nil {
		return identity
	}

	externalId := strings.TrimSpace(result.ExternalId)
	if externalId == "" {
		return identity
	}

	identity.ExternalId = externalId
	if nickname := strings.TrimSpace(result.Nickname); nickname != "" {
		identity.Nickname = nickname
	}
	if avatarURL := strings.TrimSpace(result.AvatarURL); avatarURL != "" {
		identity.AvatarURL = avatarURL
	}
	return identity
}
