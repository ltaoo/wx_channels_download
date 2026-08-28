package services

import (
	"encoding/json"
	"errors"
	"fmt"
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

var ErrBrowseHistoryMissingID = errors.New("content id is missing")

type CreateBrowseHistoryBody struct {
	Platform string          `json:"platform"`
	Content  json.RawMessage `json:"content"`
}

type CreateBrowseHistoryResult struct {
	BrowseHistory *model.BrowseHistory `json:"browse_history"`
	Account       *model.Account       `json:"account"`
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

// CreateBrowseHistory lets the platform adapter translate the intercepted
// payload, then persists the resulting browse history and account records.
func (s *BrowseService) CreateBrowseHistory(body CreateBrowseHistoryBody) (*CreateBrowseHistoryResult, error) {
	platform := strings.ToLower(strings.TrimSpace(body.Platform))
	if platform == "" {
		return nil, fmt.Errorf("%w: platform is required", ErrInvalidInput)
	}
	if len(body.Content) == 0 {
		return nil, errors.New("browse history item missing content")
	}

	handler := adapter.Get(platform)
	if handler == nil {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	info, err := handler.BuildBrowseHistory(body.Content)
	if err != nil {
		if errors.Is(err, adapter.ErrBrowseHistoryNotSupported) {
			return nil, fmt.Errorf("platform does not support browse history: %s", platform)
		}
		return nil, fmt.Errorf("failed to build browse history from payload: %w", err)
	}
	if info == nil {
		return nil, errors.New("failed to build browse history record")
	}

	browse_history := info.BrowseHistory
	if browse_history == nil {
		return nil, errors.New("failed to build browse history record")
	}
	if strings.TrimSpace(browse_history.PlatformId) == "" {
		browse_history.PlatformId = platform
	}
	if strings.TrimSpace(browse_history.ExternalId) == "" {
		return nil, ErrBrowseHistoryMissingID
	}

	account_external_id := ""
	if info.Account != nil {
		account_external_id = info.Account.ExternalId
	}
	if err := s.createBrowseHistoryRecord(browse_history, account_external_id, info.Account); err != nil {
		return nil, err
	}

	return &CreateBrowseHistoryResult{
		BrowseHistory: browse_history,
		Account:       info.Account,
	}, nil
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

	return s.createBrowseHistoryRecord(browse, info.AccountExternalId, info.Account)
}

func (s *BrowseService) createBrowseHistoryRecord(browse *model.BrowseHistory, account_external_id string, account *model.Account) error {
	if s.db == nil {
		return ErrDBNotInitialized
	}
	if browse == nil || strings.TrimSpace(browse.PlatformId) == "" || strings.TrimSpace(browse.ExternalId) == "" {
		return ErrInvalidInput
	}

	now := utilpkg.NowMillis()
	browse.PlatformId = strings.TrimSpace(browse.PlatformId)
	browse.ExternalId = strings.TrimSpace(browse.ExternalId)
	browse.Type = normalizeBrowseContentType(browse.Type)
	if browse.VisitedTimes <= 0 {
		browse.VisitedTimes = 1
	}
	if browse.CreatedAt == 0 {
		browse.CreatedAt = now
	}
	if browse.UpdatedAt == 0 {
		browse.UpdatedAt = now
	}

	if err := browse.Upsert(s.db); err != nil {
		s.logger.Error().
			Str("method", "BrowseService.createBrowseHistoryRecord").
			Str("platform_id", browse.PlatformId).
			Str("external_id", browse.ExternalId).
			Str("content_title", browse.Title).
			Err(err).
			Msg("failed to upsert browse history")
		return err
	}
	if browse.Id == "" {
		var persisted model.BrowseHistory
		if err := s.db.Where("platform_id = ? AND external_id = ?", browse.PlatformId, browse.ExternalId).First(&persisted).Error; err != nil {
			return err
		}
		browse.Id = persisted.Id
		browse.VisitedTimes = persisted.VisitedTimes
	}
	s.logger.Info().
		Str("file", "/services/browse.go").
		Str("method", "BrowseService.createBrowseHistoryRecord").
		Str("browse_id", browse.Id).
		Str("platform_id", browse.PlatformId).
		Int64("visited_times", browse.VisitedTimes).
		Msg("browse history upserted")

	account_external_id = strings.TrimSpace(account_external_id)
	if account_external_id == "" && account != nil {
		account_external_id = strings.TrimSpace(account.ExternalId)
	}
	if account_external_id != "" {
		account_id := browse.PlatformId + ":" + account_external_id
		if account != nil {
			if strings.TrimSpace(account.PlatformId) == "" {
				account.PlatformId = browse.PlatformId
			}
			if strings.TrimSpace(account.ExternalId) == "" {
				account.ExternalId = account_external_id
			}
			if strings.TrimSpace(account.Id) == "" {
				account.Id = account_id
			}
			if account.CreatedAt == 0 {
				account.CreatedAt = now
			}
			if account.UpdatedAt == 0 {
				account.UpdatedAt = now
			}
		}
		if err := s.ensureAccount(account); err != nil {
			s.logger.Error().
				Str("method", "BrowseService.createBrowseHistoryRecord").
				Str("account_id", account_id).
				Str("platform_id", browse.PlatformId).
				Str("external_id", account_external_id).
				Err(err).
				Msg("failed to ensure account record")
			return err
		}
		joinRecord := model.BrowseHistoryAccount{
			BrowseHistoryId: browse.Id,
			AccountId:       account_id,
			Role:            "author",
			CreatedAt:       now,
		}
		if err := s.db.
			Where("browse_history_id = ? AND account_id = ?", joinRecord.BrowseHistoryId, joinRecord.AccountId).
			FirstOrCreate(&joinRecord).Error; err != nil {
			s.logger.Error().
				Str("method", "BrowseService.createBrowseHistoryRecord").
				Str("browse_history_id", browse.Id).
				Str("account_id", account_id).
				Err(err).
				Msg("failed to create browse_history_account join record")
			return err
		}
		s.logger.Info().
			Str("file", "/service/browse.go").
			Str("method", "BrowseService.createBrowseHistoryRecord").
			Str("browse_history_id", browse.Id).
			Str("account_id", account_id).
			Msg("browse_history_account join record created")
	} else {
		s.logger.Info().
			Str("file", "/service/browse.go").
			Str("method", "BrowseService.createBrowseHistoryRecord").
			Str("browse_id", browse.Id).
			Str("platform_id", browse.PlatformId).
			Msg("skip browse_history_account: account_external_id is empty")
	}

	return nil
}

func (s *BrowseService) List(platform_id string, account_username *string, page int, page_size int) (*BrowseHistoryListResult, error) {
	return s.ListPlatforms([]string{platform_id}, account_username, page, page_size)
}

func (s *BrowseService) ListPlatforms(platform_ids []string, account_username *string, page int, page_size int, keywords ...string) (*BrowseHistoryListResult, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	page = normalizePage(page)
	page_size = normalizePageSize(page_size)
	offset := (page - 1) * page_size
	var normalized_platform_ids []string
	for _, platform_id := range platform_ids {
		platform_id = strings.TrimSpace(platform_id)
		if platform_id != "" {
			normalized_platform_ids = append(normalized_platform_ids, platform_id)
		}
	}
	if len(normalized_platform_ids) == 0 {
		return nil, ErrInvalidInput
	}

	query := s.db.Where("platform_id IN ?", normalized_platform_ids)
	if account_username != nil {
		query = query.Where(
			"id IN (SELECT browse_history_id FROM browse_history_account WHERE account_id = ?)",
			*account_username,
		)
	}
	keyword := ""
	if len(keywords) > 0 {
		keyword = strings.TrimSpace(keywords[0])
	}
	if keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where(
			`title LIKE ? OR external_id LIKE ? OR url LIKE ? OR source_url LIKE ? OR browse_history.id IN (
				SELECT browse_history_account.browse_history_id
				FROM browse_history_account
				JOIN account ON account.id = browse_history_account.account_id
				WHERE account.id LIKE ? OR account.external_id LIKE ? OR account.alias LIKE ? OR account.nickname LIKE ?
			)`,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
		)
	}

	var total int64
	if err := query.Model(&model.BrowseHistory{}).Count(&total).Error; err != nil {
		return nil, err
	}

	var browse_histories []model.BrowseHistory
	query = query.Order("updated_at DESC, id DESC").Limit(page_size).Offset(offset)
	if err := query.Find(&browse_histories).Error; err != nil {
		return nil, err
	}

	items := s.attachAccounts(browse_histories)
	return &BrowseHistoryListResult{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: page_size,
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
	return s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.Account
		err := tx.Where("id = ?", account.Id).First(&existing).Error
		if err == nil {
			updates := map[string]interface{}{"updated_at": account.UpdatedAt}
			needUpdate := false
			if nn := strings.TrimSpace(account.Nickname); nn != "" && strings.TrimSpace(existing.Nickname) == "" {
				updates["nickname"] = nn
				needUpdate = true
			}
			avatar_changed, avatar_err := existing.ApplyObservedAvatarURL(account.AvatarURL)
			if avatar_err != nil {
				return avatar_err
			}
			if avatar_changed {
				updates["avatar_url"] = existing.AvatarURL
				updates["past_avatars"] = existing.PastAvatars
				needUpdate = true
			}
			if !needUpdate {
				return nil
			}
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
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
		candidate := *account
		candidate.PastAvatars = "[]"
		if err := tx.Create(&candidate).Error; err != nil {
			return err
		}
		s.logger.Info().
			Str("method", "BrowseService.ensureAccount").
			Str("account_id", account.Id).
			Str("platform_id", account.PlatformId).
			Msg("created account record")
		return nil
	})
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
