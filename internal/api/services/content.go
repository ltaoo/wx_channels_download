package services

import (
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

type ContentService struct {
	db *gorm.DB
}

func NewContentService(db *gorm.DB) *ContentService {
	return &ContentService{
		db: db,
	}
}

func (s *ContentService) DB() *gorm.DB {
	return s.db
}

func (s *ContentService) ListBrowseHistory(page, pageSize int) (*PageResult, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	var total int64
	if err := s.db.Model(&model.BrowseHistory{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.BrowseHistory
	if err := s.db.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return &PageResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

var ErrInvalidInput = &ServiceError{"invalid input"}
