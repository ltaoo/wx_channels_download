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
// When content_id is empty, only the account is saved.
func (s *ContentService) UpsertAccountAndLinkContent(content_id string, account *model.Account, role string, now int64) (*model.Account, error) {
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
			updates := account_updates(account, now)
			if len(updates) > 0 {
				if err := tx.Model(&persisted).Updates(updates).Error; err != nil {
					return fmt.Errorf("更新账号失败: %w", err)
				}
				if err := tx.Where("id = ?", persisted.Id).First(&persisted).Error; err != nil {
					return fmt.Errorf("读取更新后的账号失败: %w", err)
				}
			}
		}

		content_id = strings.TrimSpace(content_id)
		if content_id == "" {
			return nil
		}

		var association model.ContentAccount
		err = tx.
			Where("content_id = ? AND account_id = ?", content_id, persisted.Id).
			First(&association).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			association = model.ContentAccount{
				ContentId: content_id,
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

func account_updates(account *model.Account, now int64) map[string]any {
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
	AccountID string
	Type      string
	Keyword   string
	StartAt   *int64 // Inclusive Unix timestamp in milliseconds.
	EndAt     *int64 // Exclusive Unix timestamp in milliseconds.
	Page      int
	PageSize  int
	Offset    *int
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
	TaskID        *int    `json:"task_id,omitempty"`
	ContentID     *string `json:"content_id,omitempty"`
	DownloadDir   string  `json:"download_dir"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	UniqueID      string  `json:"unique_id"`
	Type          string  `json:"type"`
	URL           string  `json:"url"`
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

// ContentInfluencerRecord exposes one person-role association attached to a
// content item. A person may appear multiple times with different Role values.
type ContentInfluencerRecord struct {
	ID                 int     `json:"id"`
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
	Role               string  `json:"role"`
	SortOrder          int     `json:"sort_order"`
	RoleMetadataJSON   string  `json:"role_metadata_json"`
	MetadataJSON       string  `json:"metadata_json"`
	CreatedAt          int64   `json:"created_at"`
	UpdatedAt          int64   `json:"updated_at"`
}

type ContentListItem struct {
	ID            string                      `json:"id"`
	PlatformID    string                      `json:"platform_id"`
	Type          string                      `json:"type"`
	Subtype       string                      `json:"subtype"`
	ExternalID    string                      `json:"external_id"`
	ExternalID2   string                      `json:"external_id2"`
	ExternalID3   string                      `json:"external_id3"`
	Title         string                      `json:"title"`
	Description   string                      `json:"description"`
	URL           string                      `json:"url"`
	SourceURL     string                      `json:"source_url"`
	CoverURL      string                      `json:"cover_url"`
	CoverWidth    string                      `json:"cover_width"`
	CoverHeight   string                      `json:"cover_height"`
	PublishTime   int64                       `json:"publish_time"`
	Accounts      []ContentAccountRecord      `json:"accounts"`
	Influencers   []ContentInfluencerRecord   `json:"influencers"`
	DownloadTasks []ContentDownloadTaskRecord `json:"download_tasks"`
	Resources     []ContentResourceRecord     `json:"resources"`
}

type ContentDetailItem struct {
	ContentListItem
	Content    model.Content `json:"content"`
	DetailType string        `json:"detail_type"`
	Detail     any           `json:"detail"`
}

type ContentListResult struct {
	List     []ContentListItem `json:"list"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// load_content_relations loads accounts, influencers, download tasks, and
// resources for the given content IDs. It is shared by ListContents and
// GetContentDetail.
func (s *ContentService) load_content_relations(content_ids []string) (
	map[string][]ContentAccountRecord,
	map[string][]ContentInfluencerRecord,
	map[string][]ContentDownloadTaskRecord,
	map[string][]ContentResourceRecord,
	error,
) {
	accounts_by_content_id := make(map[string][]ContentAccountRecord, len(content_ids))
	influencers_by_content_id := make(map[string][]ContentInfluencerRecord, len(content_ids))
	download_tasks_by_content_id := make(map[string][]ContentDownloadTaskRecord, len(content_ids))
	resources_by_content_id := make(map[string][]ContentResourceRecord, len(content_ids))

	if len(content_ids) == 0 {
		return accounts_by_content_id, influencers_by_content_id, download_tasks_by_content_id, resources_by_content_id, nil
	}

	type content_account_row struct {
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
	var rows []content_account_row
	if err := s.db.Table("content_account").
		Select(`content_account.content_id, content_account.account_id, content_account.role,
			account.platform_id, account.influencer_id, account.external_id, account.alias,
			account.nickname, account.signature, account.avatar_url, account.profile_url,
			account.is_listen, account.follower_count, account.past_names, account.past_avatars,
			account.created_at, account.updated_at`).
		Joins("JOIN account ON account.id = content_account.account_id").
		Where("content_account.content_id IN ? AND account.deleted_at IS NULL", content_ids).
		Order("content_account.content_id ASC, content_account.account_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	type content_influencer_row struct {
		ContentID          string  `gorm:"column:content_id"`
		InfluencerID       int     `gorm:"column:influencer_id"`
		Role               string  `gorm:"column:role"`
		SortOrder          int     `gorm:"column:sort_order"`
		RoleMetadataJSON   string  `gorm:"column:role_metadata_json"`
		Name               string  `gorm:"column:name"`
		Alias              string  `gorm:"column:alias"`
		AvatarURL          string  `gorm:"column:avatar_url"`
		Sex                int     `gorm:"column:sex"`
		Description        string  `gorm:"column:description"`
		Biography          string  `gorm:"column:biography"`
		ProfilePath        string  `gorm:"column:profile_path"`
		Birthday           string  `gorm:"column:birthday"`
		PlaceOfBirth       string  `gorm:"column:place_of_birth"`
		KnownForDepartment string  `gorm:"column:known_for_department"`
		Profile            string  `gorm:"column:profile"`
		TMDBId             *string `gorm:"column:tmdb_id"`
		DoubanId           *string `gorm:"column:douban_id"`
		IMDBId             *string `gorm:"column:imdb_id"`
		MetadataJSON       string  `gorm:"column:metadata_json"`
		CreatedAt          int64   `gorm:"column:created_at"`
		UpdatedAt          int64   `gorm:"column:updated_at"`
	}
	var influencer_rows []content_influencer_row
	if err := s.db.Table("content_influencer").
		Select(`content_influencer.content_id, content_influencer.influencer_id,
			content_influencer.role, content_influencer.sort_order,
			content_influencer.metadata_json AS role_metadata_json,
			influencer.name, influencer.alias, influencer.avatar_url, influencer.sex,
			influencer.description, influencer.biography, influencer.profile_path,
			influencer.birthday, influencer.place_of_birth,
			influencer.known_for_department, influencer.profile,
			influencer.tmdb_id, influencer.douban_id, influencer.imdb_id,
			influencer.metadata_json, content_influencer.created_at,
			content_influencer.updated_at`).
		Joins("JOIN influencer ON influencer.id = content_influencer.influencer_id").
		Where("content_influencer.content_id IN ? AND influencer.deleted_at IS NULL", content_ids).
		Order("content_influencer.content_id ASC, content_influencer.sort_order ASC, content_influencer.influencer_id ASC, content_influencer.role ASC").
		Scan(&influencer_rows).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	for _, row := range influencer_rows {
		influencers_by_content_id[row.ContentID] = append(influencers_by_content_id[row.ContentID], ContentInfluencerRecord{
			ID:                 row.InfluencerID,
			Name:               row.Name,
			Alias:              row.Alias,
			AvatarURL:          row.AvatarURL,
			Sex:                row.Sex,
			Description:        row.Description,
			Biography:          row.Biography,
			ProfilePath:        row.ProfilePath,
			Birthday:           row.Birthday,
			PlaceOfBirth:       row.PlaceOfBirth,
			KnownForDepartment: row.KnownForDepartment,
			Profile:            row.Profile,
			TMDBId:             row.TMDBId,
			DoubanId:           row.DoubanId,
			IMDBId:             row.IMDBId,
			Role:               row.Role,
			SortOrder:          row.SortOrder,
			RoleMetadataJSON:   row.RoleMetadataJSON,
			MetadataJSON:       row.MetadataJSON,
			CreatedAt:          row.CreatedAt,
			UpdatedAt:          row.UpdatedAt,
		})
	}
	for _, row := range rows {
		accounts_by_content_id[row.ContentID] = append(accounts_by_content_id[row.ContentID], ContentAccountRecord{
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
		Where("content_id IN ? AND deleted_at IS NULL", content_ids).
		Order("content_id ASC, id DESC").
		Find(&tasks).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	for _, task := range tasks {
		if task.ContentId == nil {
			continue
		}
		download_tasks_by_content_id[*task.ContentId] = append(
			download_tasks_by_content_id[*task.ContentId],
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
		Where("content_id IN ? AND deleted_at IS NULL", content_ids).
		Order("content_id ASC, merge_order ASC").
		Find(&resources).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	resource_ids := make([]int, 0, len(resources))
	for _, r := range resources {
		resource_ids = append(resource_ids, r.Id)
	}
	url_by_resource_id := make(map[int]string, len(resource_ids))
	if len(resource_ids) > 0 {
		type endpoint_row struct {
			ResourceID int    `gorm:"column:resource_id"`
			URL        string `gorm:"column:url"`
		}
		var endpoints []endpoint_row
		if err := s.db.Table("download_endpoint").
			Select("resource_id, url").
			Where("resource_id IN ? AND deleted_at IS NULL AND enabled = 1", resource_ids).
			Order("resource_id ASC, priority ASC, id ASC").
			Scan(&endpoints).Error; err != nil {
			return nil, nil, nil, nil, err
		}
		for _, ep := range endpoints {
			if _, exists := url_by_resource_id[ep.ResourceID]; !exists {
				url_by_resource_id[ep.ResourceID] = ep.URL
			}
		}
	}
	for _, r := range resources {
		if r.ContentId == nil {
			continue
		}
		resource_url := url_by_resource_id[r.Id]
		if resource_url == "" {
			resource_url = r.StreamURL
		}
		resources_by_content_id[*r.ContentId] = append(
			resources_by_content_id[*r.ContentId],
			ContentResourceRecord{
				ID:            r.Id,
				TaskID:        r.TaskId,
				ContentID:     r.ContentId,
				DownloadDir:   r.DownloadDir,
				Name:          r.Name,
				Kind:          r.Kind,
				UniqueID:      r.UniqueID,
				Type:          r.Type,
				URL:           resource_url,
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

	return accounts_by_content_id, influencers_by_content_id, download_tasks_by_content_id, resources_by_content_id, nil
}

func (s *ContentService) load_content_extension(content model.Content) (string, any, error) {
	content_type := strings.ToLower(strings.TrimSpace(content.Type))
	content_subtype := strings.ToLower(strings.TrimSpace(content.Subtype))
	find := func(detail any) (any, error) {
		if err := s.db.Where("id = ?", content.Id).First(detail).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return detail, nil
	}
	if content_subtype == model.ContentSubtypeEpisode {
		detail, err := find(&model.ContentEpisode{})
		if err != nil {
			return "content_episode", nil, err
		}
		if detail != nil {
			return "content_episode", detail, nil
		}
	}
	if content_type == model.ContentTypeCollection && content_subtype == model.ContentSubtypeSeries {
		detail, err := find(&model.ContentSeries{})
		if err != nil {
			return "content_series", nil, err
		}
		if detail != nil {
			return "content_series", detail, nil
		}
	}

	switch content_type {
	case "video", "short_video":
		var detail model.ContentVideo
		if err := s.db.
			Preload("Variants", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL").Order("is_default DESC, height DESC, bitrate DESC, asset_id ASC")
			}).
			Preload("Variants.Asset", "deleted_at IS NULL").
			Preload("Variants.Asset.DownloadResources", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL").Order("created_at DESC, id DESC")
			}).
			Preload("SubtitleTracks", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL").Order("is_default DESC, language_code ASC, id ASC")
			}).
			Preload("SubtitleTracks.Sources", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL").Order("format ASC, asset_id ASC")
			}).
			Preload("SubtitleTracks.Sources.Asset", "deleted_at IS NULL").
			Preload("SubtitleTracks.Sources.Asset.DownloadResources", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL").Order("created_at DESC, id DESC")
			}).
			Where("id = ? AND deleted_at IS NULL", content.Id).
			First(&detail).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "content_video", nil, nil
			}
			return "content_video", nil, err
		}
		return "content_video", &detail, nil
	case "image", "image_set", "album":
		var detail model.ContentAlbum
		if err := s.db.
			Preload("Images", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL").Order("sort_order ASC, id ASC")
			}).
			Where("id = ?", content.Id).
			First(&detail).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "content_album", nil, nil
			}
			return "content_album", nil, err
		}
		if err := s.load_content_album_image_assets(&detail); err != nil {
			return "content_album", nil, err
		}
		return "content_album", &detail, nil
	case "audio", "music", "audiobook":
		detail, err := find(&model.ContentAudio{})
		return "content_audio", detail, err
	case "article", "blog", "question", "answer", "news", "newsletter", "webpage":
		detail, err := find(&model.ContentArticle{})
		return "content_article", detail, err
	case "live":
		detail, err := find(&model.ContentLive{})
		return "content_live", detail, err
	case "novel":
		var detail model.ContentNovel
		if err := s.db.
			Preload("Volumes", func(db *gorm.DB) *gorm.DB {
				return db.Order("idx ASC, id ASC")
			}).
			Preload("Chapters", func(db *gorm.DB) *gorm.DB {
				return db.Order("idx ASC, id ASC")
			}).
			Where("id = ?", content.Id).
			First(&detail).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "content_novel", nil, nil
			}
			return "content_novel", nil, err
		}
		if err := s.load_content_novel_chapter_assets(&detail); err != nil {
			return "content_novel", nil, err
		}
		return "content_novel", &detail, nil
	case "podcast":
		detail, err := find(&model.ContentPodcast{})
		return "content_podcast", detail, err
	case "document":
		detail, err := find(&model.ContentDocument{})
		return "content_document", detail, err
	case "course":
		detail, err := find(&model.ContentCourse{})
		return "content_course", detail, err
	case "comic":
		detail, err := find(&model.ContentComic{})
		return "content_comic", detail, err
	case "post":
		detail, err := find(&model.ContentPost{})
		return "content_post", detail, err
	case "conversation", "chat", "ai_chat", "human_chat", "email_thread":
		var detail model.ContentConversation
		if err := s.db.
			Preload("Branches", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL").Order("is_current DESC, sort_order ASC, id ASC")
			}).
			Preload("Messages", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL").Order("sequence ASC, id ASC")
			}).
			Preload("Messages.Parts", func(db *gorm.DB) *gorm.DB {
				return db.Where("deleted_at IS NULL").Order("sort_order ASC, id ASC")
			}).
			Where("id = ?", content.Id).
			First(&detail).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "content_conversation", nil, nil
			}
			return "content_conversation", nil, err
		}
		if err := s.load_content_conversation_assets(&detail); err != nil {
			return "content_conversation", nil, err
		}
		return "content_conversation", &detail, nil
	default:
		return "", nil, nil
	}
}

func (s *ContentService) load_content_conversation_assets(content_conversation *model.ContentConversation) error {
	if content_conversation == nil || len(content_conversation.Messages) == 0 {
		return nil
	}

	message_keys := make([]string, 0, len(content_conversation.Messages))
	part_subject_keys := make([]string, 0)
	for message_index := range content_conversation.Messages {
		message := &content_conversation.Messages[message_index]
		if message_key := strings.TrimSpace(message.MessageKey); message_key != "" {
			message_keys = append(message_keys, message_key)
		}
		for part_index := range message.Parts {
			part_subject_key := strings.TrimSpace(message.Parts[part_index].SubjectKey)
			if part_subject_key != "" {
				part_subject_keys = append(part_subject_keys, part_subject_key)
			}
		}
	}

	message_assets, err := s.load_content_asset_links(
		content_conversation.Id,
		model.ContentAssetSubjectConversationMessage,
		message_keys,
	)
	if err != nil {
		return err
	}
	part_assets, err := s.load_content_asset_links(
		content_conversation.Id,
		model.ContentAssetSubjectConversationMessagePart,
		part_subject_keys,
	)
	if err != nil {
		return err
	}

	for message_index := range content_conversation.Messages {
		message := &content_conversation.Messages[message_index]
		message.Assets = message_assets[message.MessageKey]
		if message.Assets == nil {
			message.Assets = make([]model.ContentAssetLink, 0)
		}
		for part_index := range message.Parts {
			part := &message.Parts[part_index]
			part.Assets = part_assets[part.SubjectKey]
			if part.Assets == nil {
				part.Assets = make([]model.ContentAssetLink, 0)
			}
		}
	}
	return nil
}

// GetContentConversationBranch returns one ordered root-to-leaf path from an
// archived conversation tree. An empty branch_key selects CurrentBranchKey,
// then the branch marked current, then the first stored branch.
func (s *ContentService) GetContentConversationBranch(
	content_id string,
	branch_key string,
) ([]model.ContentConversationMessage, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	content_id = strings.TrimSpace(content_id)
	branch_key = strings.TrimSpace(branch_key)
	if content_id == "" {
		return nil, fmt.Errorf("content id is required")
	}

	_, detail_value, err := s.load_content_extension(model.Content{
		Id:   content_id,
		Type: model.ContentTypeConversation,
	})
	if err != nil {
		return nil, err
	}
	content_conversation, ok := detail_value.(*model.ContentConversation)
	if !ok || content_conversation == nil {
		return nil, fmt.Errorf("content conversation not found: %s", content_id)
	}
	if len(content_conversation.Branches) == 0 {
		return content_conversation.Messages, nil
	}

	var selected_branch *model.ContentConversationBranch
	if branch_key != "" {
		for branch_index := range content_conversation.Branches {
			branch := &content_conversation.Branches[branch_index]
			if branch.BranchKey == branch_key {
				selected_branch = branch
				break
			}
		}
	} else {
		current_branch_key := strings.TrimSpace(content_conversation.CurrentBranchKey)
		if current_branch_key != "" {
			for branch_index := range content_conversation.Branches {
				branch := &content_conversation.Branches[branch_index]
				if branch.BranchKey == current_branch_key {
					selected_branch = branch
					break
				}
			}
		}
		if selected_branch == nil {
			for branch_index := range content_conversation.Branches {
				branch := &content_conversation.Branches[branch_index]
				if branch.IsCurrent == 1 {
					selected_branch = branch
					break
				}
			}
		}
		if selected_branch == nil {
			selected_branch = &content_conversation.Branches[0]
		}
	}
	if selected_branch == nil {
		return nil, fmt.Errorf("conversation branch not found: %s", branch_key)
	}

	messages_by_key := make(map[string]model.ContentConversationMessage, len(content_conversation.Messages))
	for message_index := range content_conversation.Messages {
		message := content_conversation.Messages[message_index]
		messages_by_key[message.MessageKey] = message
	}
	message_key := strings.TrimSpace(selected_branch.LeafMessageKey)
	if message_key == "" {
		return nil, fmt.Errorf("conversation branch %s has no leaf message", selected_branch.BranchKey)
	}
	reversed_messages := make([]model.ContentConversationMessage, 0)
	visited_message_keys := make(map[string]struct{})
	root_message_key := strings.TrimSpace(selected_branch.RootMessageKey)
	reached_root := false
	for message_key != "" {
		if _, visited := visited_message_keys[message_key]; visited {
			return nil, fmt.Errorf("conversation branch contains a message cycle at %s", message_key)
		}
		visited_message_keys[message_key] = struct{}{}
		message, exists := messages_by_key[message_key]
		if !exists {
			return nil, fmt.Errorf("conversation message not found in branch: %s", message_key)
		}
		reversed_messages = append(reversed_messages, message)
		if root_message_key != "" && message_key == root_message_key {
			reached_root = true
			break
		}
		message_key = strings.TrimSpace(message.ParentMessageKey)
	}
	if root_message_key != "" && !reached_root {
		return nil, fmt.Errorf(
			"conversation branch %s cannot reach root message %s",
			selected_branch.BranchKey,
			root_message_key,
		)
	}

	ordered_messages := make([]model.ContentConversationMessage, len(reversed_messages))
	for message_index := range reversed_messages {
		ordered_messages[len(reversed_messages)-1-message_index] = reversed_messages[message_index]
	}
	return ordered_messages, nil
}

func (s *ContentService) load_content_novel_chapter_assets(content_novel *model.ContentNovel) error {
	if content_novel == nil || len(content_novel.Chapters) == 0 {
		return nil
	}
	chapter_keys := make([]string, 0, len(content_novel.Chapters))
	for chapter_index := range content_novel.Chapters {
		chapter_key := strings.TrimSpace(content_novel.Chapters[chapter_index].ChapterKey)
		if chapter_key != "" {
			chapter_keys = append(chapter_keys, chapter_key)
		}
	}
	if len(chapter_keys) == 0 {
		return nil
	}

	asset_links_by_chapter_key, err := s.load_content_asset_links(
		content_novel.Id,
		model.ContentAssetSubjectNovelChapter,
		chapter_keys,
	)
	if err != nil {
		return err
	}
	for chapter_index := range content_novel.Chapters {
		chapter := &content_novel.Chapters[chapter_index]
		chapter.Assets = asset_links_by_chapter_key[chapter.ChapterKey]
		if chapter.Assets == nil {
			chapter.Assets = make([]model.ContentAssetLink, 0)
		}
	}
	return nil
}

func (s *ContentService) load_content_album_image_assets(content_album *model.ContentAlbum) error {
	if content_album == nil || len(content_album.Images) == 0 {
		return nil
	}
	image_keys := make([]string, 0, len(content_album.Images))
	for image_index := range content_album.Images {
		image_key := strings.TrimSpace(content_album.Images[image_index].ImageKey)
		if image_key != "" {
			image_keys = append(image_keys, image_key)
		}
	}
	if len(image_keys) == 0 {
		return nil
	}
	asset_links_by_image_key, err := s.load_content_asset_links(
		content_album.Id,
		model.ContentAssetSubjectAlbumImage,
		image_keys,
	)
	if err != nil {
		return err
	}
	for image_index := range content_album.Images {
		content_image := &content_album.Images[image_index]
		content_image.Assets = asset_links_by_image_key[content_image.ImageKey]
		if content_image.Assets == nil {
			content_image.Assets = make([]model.ContentAssetLink, 0)
		}
	}
	return nil
}

func (s *ContentService) load_content_asset_links(
	content_id string,
	subject_type string,
	subject_keys []string,
) (map[string][]model.ContentAssetLink, error) {
	asset_links_by_subject_key := make(map[string][]model.ContentAssetLink, len(subject_keys))
	if len(subject_keys) == 0 {
		return asset_links_by_subject_key, nil
	}
	var asset_links []model.ContentAssetLink
	if err := s.db.
		Preload("Asset", "deleted_at IS NULL").
		Preload("Asset.DownloadResources", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL").Order("created_at DESC, id DESC")
		}).
		Where(
			"content_id = ? AND subject_type = ? AND subject_key IN ?",
			content_id,
			subject_type,
			subject_keys,
		).
		Order("created_at ASC, asset_id ASC").
		Find(&asset_links).Error; err != nil {
		return nil, err
	}
	for link_index := range asset_links {
		link := asset_links[link_index]
		asset_links_by_subject_key[link.SubjectKey] = append(asset_links_by_subject_key[link.SubjectKey], link)
	}
	return asset_links_by_subject_key, nil
}

func (s *ContentService) GetContentDetail(content_id string) (*ContentDetailItem, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}

	content_id = strings.TrimSpace(content_id)
	if content_id == "" {
		return nil, fmt.Errorf("content id is required")
	}

	var content model.Content
	if err := s.db.
		Preload("Assets", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL").Order("sort_order ASC, id ASC")
		}).
		Preload("Assets.DownloadResources", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL").Order("created_at DESC, id DESC")
		}).
		Where("id = ?", content_id).
		First(&content).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("content not found: %s", content_id)
		}
		return nil, err
	}

	accounts_by_content_id, influencers_by_content_id, download_tasks_by_content_id, resources_by_content_id, err := s.load_content_relations([]string{content.Id})
	if err != nil {
		return nil, err
	}

	publish_time := int64(0)
	if content.PublishTime != nil {
		publish_time = *content.PublishTime
	}
	accounts := accounts_by_content_id[content.Id]
	if accounts == nil {
		accounts = make([]ContentAccountRecord, 0)
	}
	influencers := influencers_by_content_id[content.Id]
	if influencers == nil {
		influencers = make([]ContentInfluencerRecord, 0)
	}
	download_tasks := download_tasks_by_content_id[content.Id]
	if download_tasks == nil {
		download_tasks = make([]ContentDownloadTaskRecord, 0)
	}
	resources := resources_by_content_id[content.Id]
	if resources == nil {
		resources = make([]ContentResourceRecord, 0)
	}
	detail_type, detail, err := s.load_content_extension(content)
	if err != nil {
		return nil, err
	}

	return &ContentDetailItem{
		ContentListItem: ContentListItem{
			ID:            content.Id,
			PlatformID:    content.PlatformId,
			Type:          content.Type,
			Subtype:       content.Subtype,
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
			PublishTime:   publish_time,
			Accounts:      accounts,
			Influencers:   influencers,
			DownloadTasks: download_tasks,
			Resources:     resources,
		},
		Content:    content,
		DetailType: detail_type,
		Detail:     detail,
	}, nil
}

func (s *ContentService) ListContents(options ContentListOptions) (*ContentListResult, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	if options.StartAt != nil && *options.StartAt < 0 {
		return nil, fmt.Errorf("start_at must be a non-negative Unix timestamp in milliseconds")
	}
	if options.EndAt != nil && *options.EndAt < 0 {
		return nil, fmt.Errorf("end_at must be a non-negative Unix timestamp in milliseconds")
	}
	if options.StartAt != nil && options.EndAt != nil && *options.StartAt >= *options.EndAt {
		return nil, fmt.Errorf("start_at must be less than end_at")
	}

	page := options.Page
	if page < 1 {
		page = 1
	}
	page_size := options.PageSize
	if page_size < 1 {
		page_size = 20
	}
	offset := (page - 1) * page_size
	if options.Offset != nil && *options.Offset >= 0 {
		offset = *options.Offset
	}

	build_query := func() *gorm.DB {
		query := s.db.Model(&model.Content{})
		if content_type := strings.TrimSpace(options.Type); content_type != "" {
			query = query.Where("content.type = ?", content_type)
		}
		if account_id := strings.TrimSpace(options.AccountID); account_id != "" {
			query = query.
				Joins("JOIN content_account ON content_account.content_id = content.id").
				Where("content_account.account_id = ?", account_id)
		}
		if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
			pattern := "%" + keyword + "%"
			query = query.Where("content.title LIKE ? OR content.description LIKE ?", pattern, pattern)
		}
		if options.StartAt != nil {
			query = query.Where("content.created_at >= ?", *options.StartAt)
		}
		if options.EndAt != nil {
			query = query.Where("content.created_at < ?", *options.EndAt)
		}
		return query
	}

	var total int64
	if err := build_query().Distinct("content.id").Count(&total).Error; err != nil {
		return nil, err
	}

	var contents []model.Content
	if err := build_query().
		Distinct("content.*").
		Order("content.created_at DESC, content.id DESC").
		Limit(page_size).
		Offset(offset).
		Find(&contents).Error; err != nil {
		return nil, err
	}

	content_ids := make([]string, 0, len(contents))
	for _, content := range contents {
		content_ids = append(content_ids, content.Id)
	}

	accounts_by_content_id, influencers_by_content_id, download_tasks_by_content_id, resources_by_content_id, err := s.load_content_relations(content_ids)
	if err != nil {
		return nil, err
	}

	list := make([]ContentListItem, 0, len(contents))
	for _, content := range contents {
		publish_time := int64(0)
		if content.PublishTime != nil {
			publish_time = *content.PublishTime
		}
		accounts := accounts_by_content_id[content.Id]
		if accounts == nil {
			accounts = make([]ContentAccountRecord, 0)
		}
		influencers := influencers_by_content_id[content.Id]
		if influencers == nil {
			influencers = make([]ContentInfluencerRecord, 0)
		}
		download_tasks := download_tasks_by_content_id[content.Id]
		if download_tasks == nil {
			download_tasks = make([]ContentDownloadTaskRecord, 0)
		}
		resource_list := resources_by_content_id[content.Id]
		if resource_list == nil {
			resource_list = make([]ContentResourceRecord, 0)
		}
		list = append(list, ContentListItem{
			ID:            content.Id,
			PlatformID:    content.PlatformId,
			Type:          content.Type,
			Subtype:       content.Subtype,
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
			PublishTime:   publish_time,
			Accounts:      accounts,
			Influencers:   influencers,
			DownloadTasks: download_tasks,
			Resources:     resource_list,
		})
	}

	return &ContentListResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: page_size,
	}, nil
}

func (s *ContentService) ListBrowseHistory(page, page_size int) (*PageResult, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}
	var total int64
	if err := s.db.Model(&model.BrowseHistory{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.BrowseHistory
	if err := s.db.Order("id DESC").Limit(page_size).Offset((page - 1) * page_size).Find(&list).Error; err != nil {
		return nil, err
	}
	return &PageResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: page_size,
	}, nil
}

var ErrInvalidInput = &ServiceError{"invalid input"}
