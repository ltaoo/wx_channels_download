package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

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

// UpsertAccountAndLinkContent saves an account and establishes a content association in content_account.
// When contentID is empty, only the account is saved.
func (s *ContentService) UpsertAccountAndLinkContent(contentID string, account *model.Account, role string, now int64) (*model.Account, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	if account == nil {
		return nil, fmt.Errorf("account is nil")
	}
	if strings.TrimSpace(account.PlatformId) == "" || strings.TrimSpace(account.ExternalId) == "" {
		return nil, fmt.Errorf("account platform_id and external_id are required")
	}
	if now <= 0 {
		now = time.Now().UnixMilli()
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = "owner"
	}

	var persisted model.Account
	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.
			Where("platform_id = ? AND external_id = ? AND deleted_at IS NULL", account.PlatformId, account.ExternalId).
			First(&persisted).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			persisted = *account
			if persisted.CreatedAt == 0 {
				persisted.CreatedAt = now
			}
			persisted.UpdatedAt = now
			if err := tx.Create(&persisted).Error; err != nil {
				return fmt.Errorf("保存账号失败: %w", err)
			}
		case err != nil:
			return fmt.Errorf("查询账号失败: %w", err)
		default:
			updates := accountUpdates(account, now)
			if len(updates) > 0 {
				if err := tx.Model(&persisted).Updates(updates).Error; err != nil {
					return fmt.Errorf("更新账号失败: %w", err)
				}
				if err := tx.Where("id = ?", persisted.Id).First(&persisted).Error; err != nil {
					return fmt.Errorf("读取更新后的账号失败: %w", err)
				}
			}
		}

		contentID = strings.TrimSpace(contentID)
		if contentID == "" {
			return nil
		}

		var association model.ContentAccount
		err = tx.
			Where("content_id = ? AND account_id = ?", contentID, persisted.Id).
			First(&association).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			association = model.ContentAccount{
				ContentId: contentID,
				AccountId: persisted.Id,
				Role:      role,
				CreatedAt: now,
			}
			if err := tx.Create(&association).Error; err != nil {
				return fmt.Errorf("创建 content_account 关联失败: %w", err)
			}
		case err != nil:
			return fmt.Errorf("查询 content_account 关联失败: %w", err)
		case association.Role != role:
			if err := tx.Model(&association).Update("role", role).Error; err != nil {
				return fmt.Errorf("更新 content_account 关联失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &persisted, nil
}

func accountUpdates(account *model.Account, now int64) map[string]any {
	updates := map[string]any{"updated_at": now}
	if account.InfluencerId != nil {
		updates["influencer_id"] = account.InfluencerId
	}
	if account.Alias != "" {
		updates["alias"] = account.Alias
	}
	if account.Nickname != "" {
		updates["nickname"] = account.Nickname
	}
	if account.Signature != "" {
		updates["signature"] = account.Signature
	}
	if account.AvatarURL != "" {
		updates["avatar_url"] = account.AvatarURL
	}
	if account.ProfileURL != "" {
		updates["profile_url"] = account.ProfileURL
	}
	if account.IsListen != 0 {
		updates["is_listen"] = account.IsListen
	}
	if account.FollowerCount != 0 {
		updates["follower_count"] = account.FollowerCount
	}
	if account.PastNames != "" {
		updates["past_names"] = account.PastNames
	}
	if account.PastAvatars != "" {
		updates["past_avatars"] = account.PastAvatars
	}
	return updates
}

type ContentListOptions struct {
	AccountID   string
	Type string
	Keyword     string
	StartAt     *time.Time
	EndAt       *time.Time
	Page        int
	PageSize    int
	Offset      *int
}

type ContentAccountRecord struct {
	ID            string `json:"id"`
	PlatformID    string `json:"platform_id"`
	InfluencerID  *int   `json:"influencer_id,omitempty"`
	ExternalID    string `json:"external_id"`
	Alias         string `json:"alias"`
	Nickname      string `json:"nickname"`
	Signature     string `json:"signature"`
	AvatarURL     string `json:"avatar_url"`
	ProfileURL    string `json:"profile_url"`
	IsListen      int    `json:"is_listen"`
	FollowerCount int64  `json:"follower_count"`
	PastNames     string `json:"past_names"`
	PastAvatars   string `json:"past_avatars"`
	Role          string `json:"role"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type ContentDownloadTaskRecord struct {
	ID           int     `json:"id"`
	ContentID    *string `json:"content_id,omitempty"`
	ParentTaskID *int    `json:"parent_task_id,omitempty"`
	RootTaskID   int     `json:"root_task_id"`
	RelationType string  `json:"relation_type,omitempty"`
	Name         string  `json:"name"`
	PlatformID   string  `json:"platform_id"`
	Status       int     `json:"status"`
	SourceURL    string  `json:"source_url"`
	CoverURL     string  `json:"cover_url"`
	CoverWidth   string  `json:"cover_width"`
	CoverHeight  string  `json:"cover_height"`
	Error        string  `json:"error"`
	CreatedAt    int64   `json:"created_at"`
	UpdatedAt    int64   `json:"updated_at"`
}

type ContentResourceRecord struct {
	ID            int     `json:"id"`
	TaskID        int     `json:"task_id"`
	ContentID     *string `json:"content_id,omitempty"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	UniqueID      string  `json:"unique_id"`
	Type          string  `json:"type"`
	Size          int64   `json:"size"`
	Downloaded    int64   `json:"downloaded"`
	Speed         int64   `json:"speed"`
	Status        int     `json:"status"`
	MergeOrder    int     `json:"merge_order"`
	Extra         string  `json:"extra"`
	StreamURL     string  `json:"stream_url"`
	RecordStart   *int64  `json:"record_start"`
	RecordEnd     *int64  `json:"record_end"`
	Duration      int64   `json:"duration"`
	RotateMinutes int     `json:"rotate_minutes"`
	RotateSize    int64   `json:"rotate_size"`
	StartTime     *int64  `json:"start_time"`
	FinishTime    *int64  `json:"finish_time"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
}

type ContentListItem struct {
	ID             string                      `json:"id"`
	PlatformID     string                      `json:"platform_id"`
	Type           string                      `json:"type"`
	ExternalID     string                      `json:"external_id"`
	ExternalID2    string                      `json:"external_id2"`
	ExternalID3    string                      `json:"external_id3"`
	Title          string                      `json:"title"`
	Description    string                      `json:"description"`
	URL            string                      `json:"url"`
	SourceURL      string                      `json:"source_url"`
	CoverURL       string                      `json:"cover_url"`
	CoverWidth   string  `json:"cover_width"`
	CoverHeight  string  `json:"cover_height"`
	PublishTime    int64                       `json:"publish_time"`
	Accounts       []ContentAccountRecord      `json:"accounts"`
	DownloadTasks  []ContentDownloadTaskRecord `json:"download_tasks"`
	Resources      []ContentResourceRecord     `json:"resources"`
}

type ContentListResult struct {
	List     []ContentListItem `json:"list"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// loadContentRelations loads accounts, download tasks, and resources for the given
// content IDs. It is shared by ListContents and GetContentDetail.
func (s *ContentService) loadContentRelations(contentIDs []string) (
	map[string][]ContentAccountRecord,
	map[string][]ContentDownloadTaskRecord,
	map[string][]ContentResourceRecord,
	error,
) {
	accountsByContentID := make(map[string][]ContentAccountRecord, len(contentIDs))
	downloadTasksByContentID := make(map[string][]ContentDownloadTaskRecord, len(contentIDs))
	resourcesByContentID := make(map[string][]ContentResourceRecord, len(contentIDs))

	if len(contentIDs) == 0 {
		return accountsByContentID, downloadTasksByContentID, resourcesByContentID, nil
	}

	type contentAccountRow struct {
		ContentID     string `gorm:"column:content_id"`
		AccountID     string `gorm:"column:account_id"`
		Role          string `gorm:"column:role"`
		PlatformID    string `gorm:"column:platform_id"`
		InfluencerID  *int   `gorm:"column:influencer_id"`
		ExternalID    string `gorm:"column:external_id"`
		Alias         string `gorm:"column:alias"`
		Nickname      string `gorm:"column:nickname"`
		Signature     string `gorm:"column:signature"`
		AvatarURL     string `gorm:"column:avatar_url"`
		ProfileURL    string `gorm:"column:profile_url"`
		IsListen      int    `gorm:"column:is_listen"`
		FollowerCount int64  `gorm:"column:follower_count"`
		PastNames     string `gorm:"column:past_names"`
		PastAvatars   string `gorm:"column:past_avatars"`
		CreatedAt     int64  `gorm:"column:created_at"`
		UpdatedAt     int64  `gorm:"column:updated_at"`
	}
	var rows []contentAccountRow
	if err := s.db.Table("content_account").
		Select(`content_account.content_id, content_account.account_id, content_account.role,
			account.platform_id, account.influencer_id, account.external_id, account.alias,
			account.nickname, account.signature, account.avatar_url, account.profile_url,
			account.is_listen, account.follower_count, account.past_names, account.past_avatars,
			account.created_at, account.updated_at`).
		Joins("JOIN account ON account.id = content_account.account_id").
		Where("content_account.content_id IN ? AND account.deleted_at IS NULL", contentIDs).
		Order("content_account.content_id ASC, content_account.account_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, nil, nil, err
	}
	for _, row := range rows {
		accountsByContentID[row.ContentID] = append(accountsByContentID[row.ContentID], ContentAccountRecord{
			ID:            row.AccountID,
			PlatformID:    row.PlatformID,
			InfluencerID:  row.InfluencerID,
			ExternalID:    row.ExternalID,
			Alias:         row.Alias,
			Nickname:      row.Nickname,
			Signature:     row.Signature,
			AvatarURL:     row.AvatarURL,
			ProfileURL:    row.ProfileURL,
			IsListen:      row.IsListen,
			FollowerCount: row.FollowerCount,
			PastNames:     row.PastNames,
			PastAvatars:   row.PastAvatars,
			Role:          row.Role,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		})
	}

	var tasks []model.DownloadTask
	if err := s.db.
		Where("content_id IN ? AND deleted_at IS NULL", contentIDs).
		Order("content_id ASC, id DESC").
		Find(&tasks).Error; err != nil {
		return nil, nil, nil, err
	}
	for _, task := range tasks {
		if task.ContentId == nil {
			continue
		}
		downloadTasksByContentID[*task.ContentId] = append(
			downloadTasksByContentID[*task.ContentId],
			ContentDownloadTaskRecord{
				ID:           task.Id,
				ContentID:    task.ContentId,
				ParentTaskID: task.ParentTaskID,
				RootTaskID:   task.RootTaskID,
				RelationType: task.RelationType,
				Name:         task.Name,
				PlatformID:   task.PlatformId,
				Status:       task.Status,
				SourceURL:    task.SourceURL,
				CoverURL:     task.CoverURL,
				CoverWidth:   task.CoverWidth,
				CoverHeight:  task.CoverHeight,
				Error:        task.ErrorMessage,
				CreatedAt:    task.CreatedAt,
				UpdatedAt:    task.UpdatedAt,
			},
		)
	}

	var resources []model.DownloadResource
	if err := s.db.
		Where("content_id IN ? AND deleted_at IS NULL", contentIDs).
		Order("content_id ASC, merge_order ASC").
		Find(&resources).Error; err != nil {
		return nil, nil, nil, err
	}
	for _, r := range resources {
		if r.ContentId == nil {
			continue
		}
		resourcesByContentID[*r.ContentId] = append(
			resourcesByContentID[*r.ContentId],
			ContentResourceRecord{
				ID:            r.Id,
				TaskID:        r.TaskId,
				ContentID:     r.ContentId,
				Name:          r.Name,
				Kind:          r.Kind,
				UniqueID:      r.UniqueID,
				Type:          r.Type,
				Size:          r.Size,
				Downloaded:    r.Downloaded,
				Speed:         r.Speed,
				Status:        r.Status,
				MergeOrder:    r.MergeOrder,
				Extra:         r.Extra,
				StreamURL:     r.StreamURL,
				RecordStart:   r.RecordStart,
				RecordEnd:     r.RecordEnd,
				Duration:      r.Duration,
				RotateMinutes: r.RotateMinutes,
				RotateSize:    r.RotateSize,
				StartTime:     r.StartTime,
				FinishTime:    r.FinishTime,
				CreatedAt:     r.CreatedAt,
				UpdatedAt:     r.UpdatedAt,
			},
		)
	}

	return accountsByContentID, downloadTasksByContentID, resourcesByContentID, nil
}

func (s *ContentService) GetContentDetail(contentID string) (*ContentListItem, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}

	contentID = strings.TrimSpace(contentID)
	if contentID == "" {
		return nil, fmt.Errorf("content id is required")
	}

	var content model.Content
	if err := s.db.Where("id = ?", contentID).First(&content).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("content not found: %s", contentID)
		}
		return nil, err
	}

	accountsByContentID, downloadTasksByContentID, resourcesByContentID, err := s.loadContentRelations([]string{content.Id})
	if err != nil {
		return nil, err
	}

	publishTime := int64(0)
	if content.PublishTime != nil {
		publishTime = *content.PublishTime
	}
	accounts := accountsByContentID[content.Id]
	if accounts == nil {
		accounts = make([]ContentAccountRecord, 0)
	}
	downloadTasks := downloadTasksByContentID[content.Id]
	if downloadTasks == nil {
		downloadTasks = make([]ContentDownloadTaskRecord, 0)
	}
	resources := resourcesByContentID[content.Id]
	if resources == nil {
		resources = make([]ContentResourceRecord, 0)
	}

	return &ContentListItem{
		ID:            content.Id,
		PlatformID:    content.PlatformId,
		Type:          content.Type,
		ExternalID:    content.ExternalId,
		ExternalID2:   content.ExternalId2,
		ExternalID3:   content.ExternalId3,
		Title:         content.Title,
		Description:   content.Description,
		URL:           content.URL,
		SourceURL:     content.SourceURL,
		CoverURL:      content.CoverURL,
		CoverWidth:    content.CoverWidth,
		CoverHeight:   content.CoverHeight,
		PublishTime:   publishTime,
		Accounts:      accounts,
		DownloadTasks: downloadTasks,
		Resources:     resources,
	}, nil
}

func (s *ContentService) ListContents(options ContentListOptions) (*ContentListResult, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}

	page := options.Page
	if page < 1 {
		page = 1
	}
	pageSize := options.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	if options.Offset != nil && *options.Offset >= 0 {
		offset = *options.Offset
	}

	buildQuery := func() *gorm.DB {
		query := s.db.Model(&model.Content{})
		if contentType := strings.TrimSpace(options.Type); contentType != "" {
			query = query.Where("content.type = ?", contentType)
		}
		if accountID := strings.TrimSpace(options.AccountID); accountID != "" {
			query = query.
				Joins("JOIN content_account ON content_account.content_id = content.id").
				Where("content_account.account_id = ?", accountID)
		}
		if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
			pattern := "%" + keyword + "%"
			query = query.Where("content.title LIKE ? OR content.description LIKE ?", pattern, pattern)
		}
		if options.StartAt != nil {
			query = query.Where("content.created_at >= ?", options.StartAt.UnixMilli())
		}
		if options.EndAt != nil {
			query = query.Where("content.created_at <= ?", options.EndAt.UnixMilli())
		}
		return query
	}

	var total int64
	if err := buildQuery().Distinct("content.id").Count(&total).Error; err != nil {
		return nil, err
	}

	var contents []model.Content
	if err := buildQuery().
		Distinct("content.*").
		Order("content.created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&contents).Error; err != nil {
		return nil, err
	}

	contentIDs := make([]string, 0, len(contents))
	for _, content := range contents {
		contentIDs = append(contentIDs, content.Id)
	}

	accountsByContentID, downloadTasksByContentID, resourcesByContentID, err := s.loadContentRelations(contentIDs)
	if err != nil {
		return nil, err
	}

	list := make([]ContentListItem, 0, len(contents))
	for _, content := range contents {
		publishTime := int64(0)
		if content.PublishTime != nil {
			publishTime = *content.PublishTime
		}
		accounts := accountsByContentID[content.Id]
		if accounts == nil {
			accounts = make([]ContentAccountRecord, 0)
		}
		downloadTasks := downloadTasksByContentID[content.Id]
		if downloadTasks == nil {
			downloadTasks = make([]ContentDownloadTaskRecord, 0)
		}
		resourceList := resourcesByContentID[content.Id]
		if resourceList == nil {
			resourceList = make([]ContentResourceRecord, 0)
		}
		list = append(list, ContentListItem{
			ID:             content.Id,
			PlatformID:     content.PlatformId,
			Type:           content.Type,
			ExternalID:     content.ExternalId,
			ExternalID2:    content.ExternalId2,
			ExternalID3:    content.ExternalId3,
			Title:          content.Title,
			Description:    content.Description,
			URL:            content.URL,
			SourceURL:      content.SourceURL,
			CoverURL:       content.CoverURL,
			CoverWidth:       content.CoverWidth,
			CoverHeight:       content.CoverHeight,
			PublishTime:    publishTime,
			Accounts:       accounts,
			DownloadTasks:  downloadTasks,
			Resources:      resourceList,
		})
	}

	return &ContentListResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
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
