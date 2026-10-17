package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

// TagService manages the flat tag catalog and its many-to-many associations
// with content and account entities.
type TagService struct {
	db *gorm.DB
}

func NewTagService(db *gorm.DB) *TagService {
	return &TagService{db: db}
}

func (s *TagService) DB() *gorm.DB {
	return s.db
}

// ListTags returns all non-deleted tags, optionally filtered by a name keyword.
func (s *TagService) ListTags(keyword string) ([]model.Tag, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	var tags []model.Tag
	query := s.db.Where("deleted_at IS NULL")
	if kw := strings.TrimSpace(keyword); kw != "" {
		query = query.Where("name LIKE ?", "%"+kw+"%")
	}
	if err := query.Order("id ASC").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// CreateTag inserts a new tag, or returns the existing tag when the name already
// exists. A soft-deleted tag sharing the name is restored rather than causing a
// UNIQUE(name) collision, so re-creating a deleted tag reuses its identity.
func (s *TagService) CreateTag(name string) (*model.Tag, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("标签名称不能为空")
	}
	var existing model.Tag
	err := s.db.Where("name = ?", name).First(&existing).Error
	if err == nil {
		if existing.DeletedAt != nil {
			now := time.Now().UnixMilli()
			if err := s.db.Model(&existing).Updates(map[string]any{
				"deleted_at": nil,
				"updated_at": now,
			}).Error; err != nil {
				return nil, err
			}
			existing.DeletedAt = nil
			existing.UpdatedAt = now
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := time.Now().UnixMilli()
	tag := model.Tag{
		Name: name,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := s.db.Create(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// DeleteTag soft-deletes a tag and removes every content/account association it
// participates in.
func (s *TagService) DeleteTag(tagId int) error {
	if s.db == nil {
		return ErrDBNotInitialized
	}
	if tagId <= 0 {
		return fmt.Errorf("标签 id 无效")
	}
	now := time.Now().UnixMilli()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Tag{}).
			Where("id = ? AND deleted_at IS NULL", tagId).
			Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("删除标签失败: %w", err)
		}
		if err := tx.Where("tag_id = ?", tagId).Delete(&model.ContentTag{}).Error; err != nil {
			return fmt.Errorf("清除内容标签关联失败: %w", err)
		}
		if err := tx.Where("tag_id = ?", tagId).Delete(&model.AccountTag{}).Error; err != nil {
			return fmt.Errorf("清除账号标签关联失败: %w", err)
		}
		return nil
	})
}

// RenameTag changes a tag's name, rejecting a name already used by another
// non-deleted tag.
func (s *TagService) RenameTag(tagId int, newName string) error {
	if s.db == nil {
		return ErrDBNotInitialized
	}
	if tagId <= 0 {
		return fmt.Errorf("标签 id 无效")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("标签名称不能为空")
	}
	var conflict model.Tag
	// A soft-deleted tag still holds the UNIQUE(name) slot, so it counts as a
	// conflict and prevents a rename that would violate the constraint.
	err := s.db.Where("name = ? AND id <> ?", newName, tagId).First(&conflict).Error
	if err == nil {
		return fmt.Errorf("标签名称已存在")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	now := time.Now().UnixMilli()
	if err := s.db.Model(&model.Tag{}).
		Where("id = ? AND deleted_at IS NULL", tagId).
		Updates(map[string]any{"name": newName, "updated_at": now}).Error; err != nil {
		return fmt.Errorf("重命名标签失败: %w", err)
	}
	return nil
}

// SetContentTags replaces the full set of tags attached to a content item.
func (s *TagService) SetContentTags(contentId string, tagIds []int) error {
	if s.db == nil {
		return ErrDBNotInitialized
	}
	contentId = strings.TrimSpace(contentId)
	if contentId == "" {
		return fmt.Errorf("内容 id 不能为空")
	}
	unique := dedupe_tag_ids(tagIds)
	now := time.Now().UnixMilli()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("content_id = ?", contentId).Delete(&model.ContentTag{}).Error; err != nil {
			return fmt.Errorf("清除内容标签失败: %w", err)
		}
		if len(unique) == 0 {
			return nil
		}
		rows := make([]model.ContentTag, 0, len(unique))
		for _, id := range unique {
			rows = append(rows, model.ContentTag{ContentId: contentId, TagId: id, CreatedAt: now})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("写入内容标签失败: %w", err)
		}
		return nil
	})
}

// SetAccountTags replaces the full set of tags attached to an account.
func (s *TagService) SetAccountTags(accountId string, tagIds []int) error {
	if s.db == nil {
		return ErrDBNotInitialized
	}
	accountId = strings.TrimSpace(accountId)
	if accountId == "" {
		return fmt.Errorf("账号 id 不能为空")
	}
	unique := dedupe_tag_ids(tagIds)
	now := time.Now().UnixMilli()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ?", accountId).Delete(&model.AccountTag{}).Error; err != nil {
			return fmt.Errorf("清除账号标签失败: %w", err)
		}
		if len(unique) == 0 {
			return nil
		}
		rows := make([]model.AccountTag, 0, len(unique))
		for _, id := range unique {
			rows = append(rows, model.AccountTag{AccountId: accountId, TagId: id, CreatedAt: now})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("写入账号标签失败: %w", err)
		}
		return nil
	})
}

// GetContentTags returns the tags attached to a single content item.
func (s *TagService) GetContentTags(contentId string) ([]model.Tag, error) {
	return s.tags_for_entity("content_tag", "content_id", contentId)
}

// GetAccountTags returns the tags attached to a single account.
func (s *TagService) GetAccountTags(accountId string) ([]model.Tag, error) {
	return s.tags_for_entity("account_tag", "account_id", accountId)
}

// BatchGetContentTags returns the tags for many content items keyed by content
// id, so a list endpoint can populate every row in one query.
func (s *TagService) BatchGetContentTags(contentIds []string) (map[string][]model.Tag, error) {
	result := make(map[string][]model.Tag, len(contentIds))
	for _, id := range contentIds {
		result[strings.TrimSpace(id)] = []model.Tag{}
	}
	if len(contentIds) == 0 {
		return result, nil
	}
	type tag_link_row struct {
		ContentID string `gorm:"column:content_id"`
		TagID     int    `gorm:"column:tag_id"`
		Name      string `gorm:"column:name"`
	}
	var rows []tag_link_row
	if err := s.db.Table("tag").
		Select("content_tag.content_id AS content_id, tag.id AS tag_id, tag.name AS name").
		Joins("JOIN content_tag ON content_tag.tag_id = tag.id").
		Where("content_tag.content_id IN ? AND tag.deleted_at IS NULL", contentIds).
		Order("tag.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ContentID] = append(result[row.ContentID], model.Tag{Id: row.TagID, Name: row.Name})
	}
	return result, nil
}

func (s *TagService) tags_for_entity(linkTable, idColumn, entityId string) ([]model.Tag, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	entityId = strings.TrimSpace(entityId)
	if entityId == "" {
		return []model.Tag{}, nil
	}
	var tags []model.Tag
	err := s.db.Table("tag").
		Joins("JOIN "+linkTable+" ON "+linkTable+".tag_id = tag.id").
		Where(linkTable+"."+idColumn+" = ? AND tag.deleted_at IS NULL", entityId).
		Order("tag.id ASC").
		Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func dedupe_tag_ids(tagIds []int) []int {
	seen := make(map[int]struct{}, len(tagIds))
	unique := make([]int, 0, len(tagIds))
	for _, id := range tagIds {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
