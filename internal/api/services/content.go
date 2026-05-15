package services

import (
	"encoding/json"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	utilpkg "wx_channel/pkg/util"
)

type ContentService struct {
	db *gorm.DB
}

func NewContentService(db DBClient) *ContentService {
	var d *gorm.DB
	if db != nil {
		d = db.DB()
	}
	return &ContentService{
		db: d,
	}
}

func (s *ContentService) DB() *gorm.DB {
	return s.db
}

type BrowseHistoryInput struct {
	ID            string
	ObjectNonceId string
	Contact       map[string]string
	ObjectDesc    map[string]interface{}
	SourceURL     string
	CreateTime    int
}

func (s *ContentService) CreateBrowseHistory(input *BrowseHistoryInput) error {
	if s.db == nil {
		return ErrDBNotInitialized
	}
	if input == nil || input.ID == "" {
		return ErrInvalidInput
	}

	now := utilpkg.NowMillis()
	var mediaURL, mediaCoverURL, decodeKey, urlToken string
	if media, ok := input.ObjectDesc["media"].([]interface{}); ok && len(media) > 0 {
		if m, ok := media[0].(map[string]interface{}); ok {
			if v, ok := m["url"].(string); ok {
				mediaURL = v
			}
			if v, ok := m["coverUrl"].(string); ok {
				mediaCoverURL = v
			}
			if v, ok := m["decodeKey"].(string); ok {
				decodeKey = v
			}
			if v, ok := m["urlToken"].(string); ok {
				urlToken = v
			}
		}
	}
	extraDataBytes, _ := json.Marshal(map[string]interface{}{
		"nonce_id":   input.ObjectNonceId,
		"decodeKey":  decodeKey,
		"urlToken":   urlToken,
		"source_url": input.SourceURL,
	})

	contactNickname := ""
	contactUsername := ""
	if input.Contact != nil {
		if v, ok := input.Contact["nickname"]; ok {
			contactNickname = v
		}
		if v, ok := input.Contact["username"]; ok {
			contactUsername = v
		}
	}

	browse := &model.BrowseHistory{
		PlatformId:        "weixin_channels",
		ContentId:         nil,
		ContentType:       "channels",
		ContentExternalId: input.ID,
		ContentTitle:      "",
		ContentURL:        mediaURL,
		ContentSourceURL:  input.SourceURL,
		ContentCoverURL:   mediaCoverURL,
		ExtraData:         string(extraDataBytes),
		AccountUsername:   contactUsername,
		AccountNickname:   contactNickname,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	return browse.Upsert(s.db)
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
