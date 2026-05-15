package services

import (
	"strconv"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	utilpkg "wx_channel/pkg/util"
)

type AccountService struct {
	db *gorm.DB
}

func NewAccountService(db DBClient) *AccountService {
	var d *gorm.DB
	if db != nil {
		d = db.DB()
	}
	return &AccountService{
		db: d,
	}
}

type Influencer struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	Sex         int    `json:"sex"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	DeletedAt   *int64 `json:"deleted_at"`
}

func (s *AccountService) ListInfluencers(page, pageSize int) (*PageResult, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	var total int64
	if err := s.db.Model(&model.Influencer{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.Influencer
	if err := s.db.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]Influencer, 0, len(list))
	for _, m := range list {
		out = append(out, Influencer{
			Id:          m.Id,
			Name:        m.Name,
			AvatarURL:   m.AvatarURL,
			Sex:         m.Sex,
			Description: m.Description,
			CreatedAt:   strconv.FormatInt(m.CreatedAt, 10),
			UpdatedAt:   strconv.FormatInt(m.UpdatedAt, 10),
			DeletedAt:   m.DeletedAt,
		})
	}
	return &PageResult{
		List:     out,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *AccountService) GetInfluencer(id int) (*Influencer, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	var m model.Influencer
	if err := s.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &Influencer{
		Id:          m.Id,
		Name:        m.Name,
		AvatarURL:   m.AvatarURL,
		Sex:         m.Sex,
		Description: m.Description,
		CreatedAt:   strconv.FormatInt(m.CreatedAt, 10),
		UpdatedAt:   strconv.FormatInt(m.UpdatedAt, 10),
		DeletedAt:   m.DeletedAt,
	}, nil
}

type CreateInfluencerInput struct {
	Name        string
	AvatarURL   string
	Description string
}

func (s *AccountService) CreateInfluencer(input *CreateInfluencerInput) (*Influencer, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	now := utilpkg.NowMillis()
	m := model.Influencer{
		Name:        input.Name,
		AvatarURL:   input.AvatarURL,
		Description: input.Description,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := s.db.Create(&m).Error; err != nil {
		return nil, err
	}
	return &Influencer{
		Id:          m.Id,
		Name:        m.Name,
		AvatarURL:   m.AvatarURL,
		Sex:         m.Sex,
		Description: m.Description,
		CreatedAt:   strconv.FormatInt(m.CreatedAt, 10),
		UpdatedAt:   strconv.FormatInt(m.UpdatedAt, 10),
		DeletedAt:   m.DeletedAt,
	}, nil
}

type UpdateInfluencerInput struct {
	Name        string
	AvatarURL   string
	Description string
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
	if input.AvatarURL != "" {
		updates["avatar_url"] = input.AvatarURL
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}
	if len(updates) > 1 {
		if err := s.db.Model(&model.Influencer{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	var m model.Influencer
	if err := s.db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &Influencer{
		Id:          m.Id,
		Name:        m.Name,
		AvatarURL:   m.AvatarURL,
		Sex:         m.Sex,
		Description: m.Description,
		CreatedAt:   strconv.FormatInt(m.CreatedAt, 10),
		UpdatedAt:   strconv.FormatInt(m.UpdatedAt, 10),
		DeletedAt:   m.DeletedAt,
	}, nil
}

var ErrDBNotInitialized = &ServiceError{"db not initialized"}

type ServiceError struct {
	msg string
}

func (e *ServiceError) Error() string {
	return e.msg
}
