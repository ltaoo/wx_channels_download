package services

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	utilpkg "wx_channel/pkg/util"
)

type BrowseService struct {
	db *gorm.DB
}

func NewBrowseService(db *gorm.DB) *BrowseService {
	return &BrowseService{
		db: db,
	}
}

type BrowseHistoryInfo struct {
	PlatformId        string
	AccountExternalId string
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

func (s *BrowseService) Record(uniqueMark string, info BrowseHistoryInfo) error {
	if s.db == nil {
		return ErrDBNotInitialized
	}
	if uniqueMark == "" || info.PlatformId == "" {
		return ErrInvalidInput
	}

	contentType := normalizeBrowseContentType(info.ContentType)

	var extraData string
	if len(info.ExtraData) > 0 {
		extraDataBytes, _ := json.Marshal(info.ExtraData)
		extraData = string(extraDataBytes)
	} else {
		extraData = info.ExtraDataJSON
	}

	now := utilpkg.NowMillis()
	browse := &model.BrowseHistory{
		PlatformId:        info.PlatformId,
		VisitedTimes:      1,
		AccountExternalId: info.AccountExternalId,
		AccountNickname:   info.AccountNickname,
		AccountAvatarURL:  info.AccountAvatarURL,
		Type:              contentType,
		ExternalId:        uniqueMark,
		Title:             info.ContentTitle,
		URL:               info.ContentURL,
		SourceURL:         info.ContentSourceURL,
		CoverURL:          info.ContentCoverURL,
		ExtraData:         extraData,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	return browse.Upsert(s.db)
}

func (s *BrowseService) List(platformId string, accountUsername *string) ([]model.BrowseHistory, error) {
	return s.ListPlatforms([]string{platformId}, accountUsername)
}

func (s *BrowseService) ListPlatforms(platformIds []string, accountUsername *string) ([]model.BrowseHistory, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	var normalizedPlatformIds []string
	for _, platformId := range platformIds {
		platformId = strings.TrimSpace(platformId)
		if platformId != "" {
			normalizedPlatformIds = append(normalizedPlatformIds, platformId)
		}
	}
	if len(normalizedPlatformIds) == 0 {
		return nil, ErrInvalidInput
	}

	query := s.db.Where("platform_id IN ?", normalizedPlatformIds)
	if accountUsername != nil {
		query = query.Where("account_external_id = ?", *accountUsername)
	}

	var browseHistories []model.BrowseHistory
	if err := query.Order("updated_at DESC, id DESC").Find(&browseHistories).Error; err != nil {
		return nil, err
	}
	return browseHistories, nil
}

func normalizeBrowseContentType(contentType string) string {
	switch contentType {
	case "picture":
		return "image"
	case "live":
		return "live"
	case "", "media":
		return "video"
	default:
		return contentType
	}
}
