package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	utilpkg "wx_channel/pkg/util"
)

type AccountService struct {
	db *gorm.DB
}

const default_account_page_size = 24

func NewAccountService(db *gorm.DB) *AccountService {
	return &AccountService{
		db: db,
	}
}

type AccountListInput struct {
	Page      int
	PageSize  int
	Keyword   string
	AccountID string
}

type AccountListItem struct {
	Account      model.Account
	ContentCount int64
}

type AccountListPage struct {
	List     []AccountListItem
	Total    int64
	Page     int
	PageSize int
}

type account_content_count_row struct {
	AccountID    string `gorm:"column:account_id"`
	ContentCount int64  `gorm:"column:content_count"`
}

// ListAccounts loads one account page and its association counts with three
// SQL statements regardless of the number of accounts in the page.
func (s *AccountService) ListAccounts(ctx context.Context, input AccountListInput) (*AccountListPage, error) {
	if s == nil || s.db == nil {
		return nil, ErrDBNotInitialized
	}
	page := input.Page
	if page <= 0 {
		page = 1
	}
	page_size := input.PageSize
	if page_size <= 0 {
		page_size = default_account_page_size
	}
	if page_size > 200 {
		page_size = 200
	}

	db := s.db.WithContext(ctx)
	account_query := db.Model(&model.Account{})
	account_id := strings.TrimSpace(input.AccountID)
	if account_id != "" {
		account_query = account_query.Where("id = ?", account_id)
	}
	keyword := strings.TrimSpace(input.Keyword)
	if keyword != "" {
		pattern := "%" + keyword + "%"
		account_query = account_query.Where(
			"id LIKE ? OR external_id LIKE ? OR alias LIKE ? OR nickname LIKE ?",
			pattern,
			pattern,
			pattern,
			pattern,
		)
	}

	var total int64
	if err := account_query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("查询账号总数失败: %w", err)
	}
	page_count := 1
	if total > 0 {
		page_count = int((total + int64(page_size) - 1) / int64(page_size))
	}
	if page > page_count {
		page = page_count
	}

	var accounts []model.Account
	if err := account_query.
		Order("created_at DESC, id DESC").
		Limit(page_size).
		Offset((page - 1) * page_size).
		Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("查询账号失败: %w", err)
	}

	result_page := &AccountListPage{
		List:     make([]AccountListItem, len(accounts)),
		Total:    total,
		Page:     page,
		PageSize: page_size,
	}
	if len(accounts) == 0 {
		return result_page, nil
	}

	account_ids := make([]string, len(accounts))
	account_indexes := make(map[string]int, len(accounts))
	for account_index := range accounts {
		account := accounts[account_index]
		account_ids[account_index] = account.Id
		account_indexes[account.Id] = account_index
		result_page.List[account_index] = AccountListItem{
			Account: account,
		}
	}

	var content_count_rows []account_content_count_row
	if err := db.
		Table("content_account").
		Select("account_id, COUNT(*) AS content_count").
		Where("account_id IN ?", account_ids).
		Group("account_id").
		Scan(&content_count_rows).Error; err != nil {
		return nil, fmt.Errorf("查询账号关联内容数量失败: %w", err)
	}

	for _, content_count_row := range content_count_rows {
		account_index, exists := account_indexes[content_count_row.AccountID]
		if !exists {
			continue
		}
		result_page.List[account_index].ContentCount = content_count_row.ContentCount
	}

	return result_page, nil
}

type Influencer struct {
	Id                 int     `json:"id"`
	Name               string  `json:"name"`
	Alias              string  `json:"alias"`
	AvatarURL          string  `json:"avatar_url"`
	Sex                int     `json:"sex"`
	Description        string  `json:"description"`
	Biography          string  `json:"biography"`
	ProfilePath        string  `json:"profile_path"`
	Birthday           string  `json:"birthday"`
	PlaceOfBirth       string  `json:"place_of_birth"`
	KnownForDepartment string  `json:"known_for_department"`
	Profile            string  `json:"profile"`
	TMDBId             *string `json:"tmdb_id,omitempty"`
	DoubanId           *string `json:"douban_id,omitempty"`
	IMDBId             *string `json:"imdb_id,omitempty"`
	MetadataJSON       string  `json:"metadata_json"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	DeletedAt          *int64  `json:"deleted_at"`
}

func influencer_response(influencer model.Influencer) Influencer {
	return Influencer{
		Id:                 influencer.Id,
		Name:               influencer.Name,
		Alias:              influencer.Alias,
		AvatarURL:          influencer.AvatarURL,
		Sex:                influencer.Sex,
		Description:        influencer.Description,
		Biography:          influencer.Biography,
		ProfilePath:        influencer.ProfilePath,
		Birthday:           influencer.Birthday,
		PlaceOfBirth:       influencer.PlaceOfBirth,
		KnownForDepartment: influencer.KnownForDepartment,
		Profile:            influencer.Profile,
		TMDBId:             influencer.TMDBId,
		DoubanId:           influencer.DoubanId,
		IMDBId:             influencer.IMDBId,
		MetadataJSON:       influencer.MetadataJSON,
		CreatedAt:          strconv.FormatInt(influencer.CreatedAt, 10),
		UpdatedAt:          strconv.FormatInt(influencer.UpdatedAt, 10),
		DeletedAt:          influencer.DeletedAt,
	}
}

func (s *AccountService) ListInfluencers(page, page_size int) (*PageResult, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	var total int64
	if err := s.db.Model(&model.Influencer{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.Influencer
	if err := s.db.Order("id DESC").Limit(page_size).Offset((page - 1) * page_size).Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]Influencer, 0, len(list))
	for _, influencer := range list {
		out = append(out, influencer_response(influencer))
	}
	return &PageResult{
		List:     out,
		Total:    total,
		Page:     page,
		PageSize: page_size,
	}, nil
}

func (s *AccountService) GetInfluencer(id int) (*Influencer, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	var influencer model.Influencer
	if err := s.db.First(&influencer, id).Error; err != nil {
		return nil, err
	}
	response := influencer_response(influencer)
	return &response, nil
}

type CreateInfluencerInput struct {
	Name               string
	Alias              string
	AvatarURL          string
	Sex                int
	Description        string
	Biography          string
	ProfilePath        string
	Birthday           string
	PlaceOfBirth       string
	KnownForDepartment string
	Profile            string
	TMDBId             *string
	DoubanId           *string
	IMDBId             *string
	MetadataJSON       string
}

func (s *AccountService) CreateInfluencer(input *CreateInfluencerInput) (*Influencer, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	now := utilpkg.NowMillis()
	influencer := model.Influencer{
		Name:               input.Name,
		Alias:              input.Alias,
		AvatarURL:          input.AvatarURL,
		Sex:                input.Sex,
		Description:        input.Description,
		Biography:          input.Biography,
		ProfilePath:        input.ProfilePath,
		Birthday:           input.Birthday,
		PlaceOfBirth:       input.PlaceOfBirth,
		KnownForDepartment: input.KnownForDepartment,
		Profile:            input.Profile,
		TMDBId:             input.TMDBId,
		DoubanId:           input.DoubanId,
		IMDBId:             input.IMDBId,
		MetadataJSON:       input.MetadataJSON,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := s.db.Create(&influencer).Error; err != nil {
		return nil, err
	}
	response := influencer_response(influencer)
	return &response, nil
}

type UpdateInfluencerInput struct {
	Name               string
	Alias              string
	AvatarURL          string
	Sex                *int
	Description        string
	Biography          string
	ProfilePath        string
	Birthday           string
	PlaceOfBirth       string
	KnownForDepartment string
	Profile            string
	TMDBId             *string
	DoubanId           *string
	IMDBId             *string
	MetadataJSON       string
}

func (s *AccountService) UpdateInfluencer(id int, input *UpdateInfluencerInput) (*Influencer, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	updates := map[string]any{
		"updated_at": utilpkg.NowMillis(),
	}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Alias != "" {
		updates["alias"] = input.Alias
	}
	if input.AvatarURL != "" {
		updates["avatar_url"] = input.AvatarURL
	}
	if input.Sex != nil {
		updates["sex"] = *input.Sex
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}
	for column, value := range map[string]string{
		"biography":            input.Biography,
		"profile_path":         input.ProfilePath,
		"birthday":             input.Birthday,
		"place_of_birth":       input.PlaceOfBirth,
		"known_for_department": input.KnownForDepartment,
		"profile":              input.Profile,
		"metadata_json":        input.MetadataJSON,
	} {
		if value != "" {
			updates[column] = value
		}
	}
	for column, value := range map[string]*string{
		"tmdb_id":   input.TMDBId,
		"douban_id": input.DoubanId,
		"imdb_id":   input.IMDBId,
	} {
		if value != nil {
			updates[column] = *value
		}
	}
	if len(updates) > 1 {
		if err := s.db.Model(&model.Influencer{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	var influencer model.Influencer
	if err := s.db.First(&influencer, id).Error; err != nil {
		return nil, err
	}
	response := influencer_response(influencer)
	return &response, nil
}

var ErrDBNotInitialized = &ServiceError{"db not initialized"}

type ServiceError struct {
	msg string
}

func (e *ServiceError) Error() string {
	return e.msg
}
