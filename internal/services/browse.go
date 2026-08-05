package services

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	utilpkg "wx_channel/pkg/util"
)

type BrowseService struct {
	db     *gorm.DB
	logger zerolog.Logger
}

func NewBrowseService(db *gorm.DB, logger zerolog.Logger) *BrowseService {
	return &BrowseService{
		db:     db,
		logger: logger,
	}
}

type BrowseHistoryListOptions struct {
	PlatformIds     []string
	AccountUsername *string
	Page            int
	PageSize        int
	Offset          *int
}

// AccountBrief carries minimal account info for display in browse history lists.
type AccountBrief struct {
	ExternalId string `json:"external_id"`
	Nickname   string `json:"nickname"`
	AvatarURL  string `json:"avatar_url"`
}

// BrowseHistoryItem wraps a browse history record with its associated accounts.
type BrowseHistoryItem struct {
	model.BrowseHistory
	Accounts []AccountBrief `json:"accounts"`
}

type BrowseHistoryListResult struct {
	List     []BrowseHistoryItem `json:"list"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
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
		PlatformId:   info.PlatformId,
		VisitedTimes: 1,
		Type:         contentType,
		ExternalId:   uniqueMark,
		Title:        info.ContentTitle,
		URL:          info.ContentURL,
		SourceURL:    info.ContentSourceURL,
		CoverURL:     info.ContentCoverURL,
		ExtraData:    extraData,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := browse.Upsert(s.db); err != nil {
		s.logger.Error().
			Str("method", "BrowseService.Record").
			Str("platform_id", info.PlatformId).
			Str("external_id", uniqueMark).
			Str("content_title", info.ContentTitle).
			Err(err).
			Msg("failed to upsert browse history")
		return err
	}
	s.logger.Info().
		Str("method", "BrowseService.Record").
		Str("browse_id", browse.Id).
		Str("platform_id", info.PlatformId).
		Int64("visited_times", browse.VisitedTimes).
		Msg("browse history upserted")

	accountExternalID := strings.TrimSpace(info.AccountExternalId)
	if accountExternalID != "" {
		accountID := info.PlatformId + ":" + accountExternalID
		if err := s.ensureAccount(info.Account); err != nil {
			s.logger.Error().
				Str("method", "BrowseService.Record").
				Str("account_id", accountID).
				Str("platform_id", info.PlatformId).
				Str("external_id", accountExternalID).
				Err(err).
				Msg("failed to ensure account record")
			return err
		}
		joinRecord := model.BrowseHistoryAccount{
			BrowseHistoryId: browse.Id,
			AccountId:       accountID,
			Role:            "author",
			CreatedAt:       now,
		}
		if err := s.db.Where(joinRecord).FirstOrCreate(&joinRecord).Error; err != nil {
			s.logger.Error().
				Str("method", "BrowseService.Record").
				Str("browse_history_id", browse.Id).
				Str("account_id", accountID).
				Err(err).
				Msg("failed to create browse_history_account join record")
			return err
		}
		s.logger.Info().
			Str("method", "BrowseService.Record").
			Str("browse_history_id", browse.Id).
			Str("account_id", accountID).
			Msg("browse_history_account join record created")
	} else {
		s.logger.Info().
			Str("method", "BrowseService.Record").
			Str("browse_id", browse.Id).
			Str("platform_id", info.PlatformId).
			Msg("skip browse_history_account: account_external_id is empty")
	}

	return nil
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
		query = query.Where(
			"id IN (SELECT browse_history_id FROM browse_history_account WHERE account_id = ?)",
			*accountUsername,
		)
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

	items := s.attachAccounts(browseHistories)
	return &BrowseHistoryListResult{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// attachAccounts batch-loads account info for a set of browse history records
// via the browse_history_account join table.
func (s *BrowseService) attachAccounts(histories []model.BrowseHistory) []BrowseHistoryItem {
	if len(histories) == 0 {
		return nil
	}

	historyIds := make([]string, len(histories))
	for i, h := range histories {
		historyIds[i] = h.Id
	}

	type accountRow struct {
		BrowseHistoryId string
		ExternalId      string
		Nickname        string
		AvatarURL       string
	}

	var rows []accountRow
	s.db.Table("browse_history_account").
		Select("browse_history_account.browse_history_id, account.external_id, account.nickname, account.avatar_url").
		Joins("JOIN account ON account.id = browse_history_account.account_id").
		Where("browse_history_account.browse_history_id IN ?", historyIds).
		Scan(&rows)

	s.logger.Info().
		Str("method", "BrowseService.attachAccounts").
		Int("history_count", len(histories)).
		Int("account_rows", len(rows)).
		Msg("attachAccounts join result")

	accountMap := make(map[string][]AccountBrief, len(rows))
	for _, r := range rows {
		accountMap[r.BrowseHistoryId] = append(accountMap[r.BrowseHistoryId], AccountBrief{
			ExternalId: strings.TrimSpace(r.ExternalId),
			Nickname:   strings.TrimSpace(r.Nickname),
			AvatarURL:  strings.TrimSpace(r.AvatarURL),
		})
	}

	items := make([]BrowseHistoryItem, len(histories))
	for i, h := range histories {
		item := BrowseHistoryItem{BrowseHistory: h}
		if accounts, ok := accountMap[h.Id]; ok {
			item.Accounts = accounts
		} else {
			item.Accounts = []AccountBrief{}
		}
		items[i] = item
	}
	return items
}

// ensureAccount creates or updates an account record with available data.
func (s *BrowseService) ensureAccount(account *model.Account) error {
	if account == nil || account.Id == "" {
		return nil
	}
	var existing model.Account
	err := s.db.Where("id = ?", account.Id).First(&existing).Error
	if err == nil {
		updates := map[string]interface{}{"updated_at": account.UpdatedAt}
		needUpdate := false
		if nn := strings.TrimSpace(account.Nickname); nn != "" && strings.TrimSpace(existing.Nickname) == "" {
			updates["nickname"] = nn
			needUpdate = true
		}
		if av := strings.TrimSpace(account.AvatarURL); av != "" && strings.TrimSpace(existing.AvatarURL) == "" {
			updates["avatar_url"] = av
			needUpdate = true
		}
		if !needUpdate {
			return nil
		}
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			return err
		}
		s.logger.Info().
			Str("method", "BrowseService.ensureAccount").
			Str("account_id", account.Id).
			Msg("updated account record")
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := s.db.Create(account).Error; err != nil {
		return err
	}
	s.logger.Info().
		Str("method", "BrowseService.ensureAccount").
		Str("account_id", account.Id).
		Str("platform_id", account.PlatformId).
		Msg("created account record")
	return nil
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
