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
	Account      model.Account `gorm:"embedded"`
	ContentCount int64
}

type AccountListPage struct {
	List     []AccountListItem
	Total    int64
	Page     int
	PageSize int
}

// ListAccounts loads one account page and its association counts.
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

	var items []AccountListItem
	if err := account_query.
		Select(`account.id, account.platform_id, account.external_id, account.alias,
			account.nickname, account.signature, account.avatar_url, account.profile_url,
			account.follower_count, account.created_at, account.updated_at,
			(SELECT COUNT(*) FROM content_account
				WHERE content_account.account_id = account.id) AS content_count`).
		Order("created_at DESC, id DESC").
		Limit(page_size).
		Offset((page - 1) * page_size).
		Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("查询账号失败: %w", err)
	}

	return &AccountListPage{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: page_size,
	}, nil
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
