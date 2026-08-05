package services

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/adapter"
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

type BrowseHistoryListOptions struct {
	PlatformIds     []string
	AccountUsername *string
	Page            int
	PageSize        int
	Offset          *int
}

type BrowseHistoryListResult struct {
	List     []model.BrowseHistory `json:"list"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

func (s *BrowseService) Record(uniqueMark string, info adapter.BrowseHistoryInfo) error {
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

func (s *BrowseService) List(platformId string, accountUsername *string, page int, pageSize int) (*BrowseHistoryListResult, error) {
	return s.ListPlatforms([]string{platformId}, accountUsername, page, pageSize)
}

func (s *BrowseService) ListPlatforms(platformIds []string, accountUsername *string, page int, pageSize int) (*BrowseHistoryListResult, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	page = normalizePage(page)
	pageSize = normalizePageSize(pageSize)
	offset := (page - 1) * pageSize
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

	var total int64
	if err := query.Model(&model.BrowseHistory{}).Count(&total).Error; err != nil {
		return nil, err
	}

	var browseHistories []model.BrowseHistory
	query = query.Order("updated_at DESC, id DESC").Limit(pageSize).Offset(offset)
	if err := query.Find(&browseHistories).Error; err != nil {
		return nil, err
	}
	return &BrowseHistoryListResult{
		List:     browseHistories,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int) int {
	if pageSize < 1 {
		return 20
	}
	return pageSize
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
