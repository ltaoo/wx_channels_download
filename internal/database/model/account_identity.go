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

	var history BrowseHistory
	err := db.Where(
		"platform_id = ? AND external_id = ? AND account_external_id <> ''",
		strings.TrimSpace(platformId),
		strings.TrimSpace(contentExternalId),
	).Order("updated_at DESC").First(&history).Error
	if err != nil {
		return identity
	}

	externalId := strings.TrimSpace(history.AccountExternalId)
	if externalId == "" {
		return identity
	}

	identity.ExternalId = externalId
	if nickname := strings.TrimSpace(history.AccountNickname); nickname != "" {
		identity.Nickname = nickname
	}
	if avatarURL := strings.TrimSpace(history.AccountAvatarURL); avatarURL != "" {
		identity.AvatarURL = avatarURL
	}
	return identity
}
