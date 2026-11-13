package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
)

// DownloadTaskArchive contains the saved inputs needed to recreate one task.
// It contains no browsing history or unrelated account/content records.
type DownloadTaskArchive struct {
	Task      model.DownloadTask      `json:"task"`
	Resources []*adapter.ResourceInfo `json:"resources"`
	Contents  []TaskContentArchive    `json:"contents"`
}

type TaskContentArchive struct {
	Content     model.Content                        `json:"content"`
	Details     map[string]json.RawMessage           `json:"details"`
	Accounts    []adapter.ContentAccountReference    `json:"accounts"`
	Influencers []adapter.ContentInfluencerReference `json:"influencers"`
	Relations   []model.ContentRelation              `json:"relations"`
}

func archive_detail_model(kind string) any {
	switch kind {
	case "video":
		return &model.ContentVideo{}
	case "episode":
		return &model.ContentEpisode{}
	case "series":
		return &model.ContentSeries{}
	case "audio":
		return &model.ContentAudio{}
	case "article":
		return &model.ContentArticle{}
	case "live":
		return &model.ContentLive{}
	case "novel":
		return &model.ContentNovel{}
	case "album":
		return &model.ContentAlbum{}
	case "podcast":
		return &model.ContentPodcast{}
	case "document":
		return &model.ContentDocument{}
	case "course":
		return &model.ContentCourse{}
	case "comic":
		return &model.ContentComic{}
	case "post":
		return &model.ContentPost{}
	case "conversation":
		return &model.ContentConversation{}
	}
	return nil
}

var archive_detail_kinds = []string{"video", "episode", "series", "audio", "article", "live", "novel", "album", "podcast", "document", "course", "comic", "post", "conversation"}

func ExportDownloadTaskArchive(db *gorm.DB, task model.DownloadTask) (*DownloadTaskArchive, error) {
	archive := &DownloadTaskArchive{Task: task}
	var resources []model.DownloadResource
	if err := db.Where("task_id = ?", task.Id).Order("id").Find(&resources).Error; err != nil {
		return nil, err
	}
	content_ids := make(map[string]bool)
	if task.ContentId != nil && *task.ContentId != "" {
		content_ids[*task.ContentId] = true
	}
	for _, resource := range resources {
		info := &adapter.ResourceInfo{Resource: resource}
		if err := db.Where("resource_id = ?", resource.Id).Find(&info.Endpoints).Error; err != nil {
			return nil, err
		}
		if resource.ContentId != nil && *resource.ContentId != "" {
			content_ids[*resource.ContentId] = true
		}
		var links []model.DownloadResourceAsset
		if err := db.Where("resource_id = ?", resource.Id).Find(&links).Error; err != nil {
			return nil, err
		}
		for _, link := range links {
			var asset model.ContentAsset
			if err := db.First(&asset, link.AssetId).Error; err != nil {
				return nil, err
			}
			reference := adapter.ContentAssetReference{Kind: asset.Kind, Role: asset.Role, AssetKey: asset.AssetKey, Relation: link.Relation}
			var subjects []model.ContentAssetLink
			if err := db.Where("asset_id = ?", asset.Id).Find(&subjects).Error; err != nil {
				return nil, err
			}
			if len(subjects) == 0 {
				info.ContentAssets = append(info.ContentAssets, reference)
			}
			for _, subject := range subjects {
				copy_reference := reference
				copy_reference.SubjectType, copy_reference.SubjectKey, copy_reference.SubjectRelation = subject.SubjectType, subject.SubjectKey, subject.Relation
				info.ContentAssets = append(info.ContentAssets, copy_reference)
			}
		}
		archive.Resources = append(archive.Resources, info)
	}
	// Outgoing relations include parent series and other content referenced by
	// the task, without pulling in every sibling episode from the database.
	processed := make(map[string]bool)
	for {
		pending := []string{}
		for id := range content_ids {
			if !processed[id] {
				pending = append(pending, id)
			}
		}
		if len(pending) == 0 {
			break
		}
		for _, id := range pending {
			processed[id] = true
			content, err := export_task_content(db, id)
			if err != nil {
				return nil, err
			}
			for _, relation := range content.Relations {
				content_ids[relation.TargetContentId] = true
			}
			archive.Contents = append(archive.Contents, *content)
		}
	}
	return archive, nil
}

func export_task_content(db *gorm.DB, id string) (*TaskContentArchive, error) {
	archive := &TaskContentArchive{Details: make(map[string]json.RawMessage)}
	if err := db.Preload("TextTracks.Sources.Asset").Where("id = ?", id).First(&archive.Content).Error; err != nil {
		return nil, err
	}
	for _, kind := range archive_detail_kinds {
		detail := archive_detail_model(kind)
		query := db
		switch kind {
		case "video":
			query = query.Preload("Variants.Asset")
		case "novel":
			query = query.Preload("Volumes").Preload("Chapters")
		case "album":
			query = query.Preload("Images")
		case "conversation":
			query = query.Preload("Branches").Preload("Messages.Parts")
		}
		err := query.Where("id = ?", id).First(detail).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(detail)
		if err != nil {
			return nil, err
		}
		archive.Details[kind] = raw
	}
	var links []model.ContentAccount
	if err := db.Where("content_id = ?", id).Find(&links).Error; err != nil {
		return nil, err
	}
	for _, link := range links {
		var account model.Account
		if err := db.Where("id = ?", link.AccountId).First(&account).Error; err != nil {
			return nil, err
		}
		account.InfluencerId = nil
		archive.Accounts = append(archive.Accounts, adapter.ContentAccountReference{Account: &account, Role: link.Role})
	}
	var people []model.ContentInfluencer
	if err := db.Where("content_id = ?", id).Find(&people).Error; err != nil {
		return nil, err
	}
	for _, link := range people {
		var person model.Influencer
		if err := db.First(&person, link.InfluencerId).Error; err != nil {
			return nil, err
		}
		person.Id = 0
		archive.Influencers = append(archive.Influencers, adapter.ContentInfluencerReference{Influencer: &person, Roles: []adapter.ContentInfluencerRole{{Role: link.Role, SortOrder: link.SortOrder, MetadataJSON: link.MetadataJSON}}})
	}
	if err := db.Where("source_content_id = ?", id).Find(&archive.Relations).Error; err != nil {
		return nil, err
	}
	return archive, nil
}

// RecreateArchivedTask uses the same persistence pipeline as CreateTask, with
// already-resolved content and file destinations. It cannot start a downloader
// or call a platform adapter. The caller owns the transaction and file install.
func RecreateArchivedTask(tx *gorm.DB, archive DownloadTaskArchive, download_dir string, parent_id *int) (*CreateTaskResult, error) {
	if archive.Task.CreatedAt <= 0 {
		return nil, fmt.Errorf("归档下载任务创建时间无效")
	}
	// Creation helpers assign IDs on the supplied graph. Keep the source
	// snapshot reusable for retries and for other tasks sharing this content.
	raw_archive, err := json.Marshal(archive)
	if err != nil {
		return nil, err
	}
	var cloned DownloadTaskArchive
	if err := json.Unmarshal(raw_archive, &cloned); err != nil {
		return nil, err
	}
	archive = cloned
	now := time.Now().UnixMilli()
	info := &adapter.DownloadTaskResult{Resources: archive.Resources}
	content_ids := make(map[string]string)
	for index := range archive.Contents {
		content := &archive.Contents[index].Content
		source_id := content.Id
		var existing model.Content
		find_err := tx.Where("platform_id = ? AND external_id = ?", content.PlatformId, content.ExternalId).First(&existing).Error
		if content.ExternalId == "" {
			find_err = tx.Where("id = ?", source_id).First(&existing).Error
		}
		if find_err == nil {
			content.Id = existing.Id
			// Updating an existing content record must not change when it was
			// first saved on the target. New content keeps the archived timestamp.
			content.CreatedAt = existing.CreatedAt
		} else if !errors.Is(find_err, gorm.ErrRecordNotFound) {
			return nil, find_err
		} else {
			var count int64
			if err := tx.Model(&model.Content{}).Where("id = ?", source_id).Count(&count).Error; err != nil {
				return nil, err
			}
			if count > 0 {
				content.Id = uuid.NewString()
			}
		}
		content_ids[source_id] = content.Id
	}
	for index := range archive.Contents {
		saved := &archive.Contents[index]
		content := &saved.Content
		reset_archive_text_tracks(content)
		for _, reference := range saved.Accounts {
			if reference.Account == nil {
				continue
			}
			reference.Account.Id = reference.Account.PlatformId + ":" + reference.Account.ExternalId
			var occupied model.Account
			find_err := tx.Where("id = ?", reference.Account.Id).First(&occupied).Error
			if find_err == nil && (occupied.PlatformId != reference.Account.PlatformId || occupied.ExternalId != reference.Account.ExternalId) {
				reference.Account.Id = uuid.NewString()
			} else if find_err != nil && !errors.Is(find_err, gorm.ErrRecordNotFound) {
				return nil, find_err
			}
			reference.Account.InfluencerId = nil
		}
		for _, reference := range saved.Influencers {
			if reference.Influencer != nil {
				reference.Influencer.Id = 0
			}
		}
		for relation_index := range saved.Relations {
			relation := &saved.Relations[relation_index]
			relation.SourceContentId, relation.TargetContentId = content_ids[relation.SourceContentId], content_ids[relation.TargetContentId]
		}
		base := adapter.ContentDetail{Key: content.Id, Content: content, Accounts: saved.Accounts, Influencers: saved.Influencers, Relations: saved.Relations}
		info.ContentDetails = append(info.ContentDetails, base)
		for _, kind := range archive_detail_kinds {
			raw, ok := saved.Details[kind]
			if !ok {
				continue
			}
			detail := archive_detail_model(kind)
			if err := json.Unmarshal(raw, detail); err != nil {
				return nil, err
			}
			reset_archive_detail(detail, content.Id)
			info.ContentDetails = append(info.ContentDetails, adapter.ContentDetail{Key: content.Id, Data: detail})
			if archive.Task.ContentId != nil && content_ids[*archive.Task.ContentId] == content.Id && (info.ContentDetail == nil || kind == content.Type) {
				info.ContentDetail = detail
			}
		}
		if archive.Task.ContentId != nil && content_ids[*archive.Task.ContentId] == content.Id {
			info.Content = content
		}
	}
	// Preserve the source creation time, including millisecond precision. It
	// identifies this download attempt when the same archive is imported again.
	task := archive.Task
	task.Id, task.RootTaskID, task.ParentTaskID = 0, 0, nil
	task.Status = model.TaskStatusFinished
	task.ErrorMessage = ""
	if info.Content != nil {
		task.ContentId = &info.Content.Id
	}
	if archive.Task.ContentId != nil && info.Content == nil {
		return nil, fmt.Errorf("归档任务缺少关联内容")
	}
	if len(info.Resources) == 0 && archive.Task.Status != model.TaskStatusFinished {
		task.Status = model.TaskStatusPaused
	}
	for _, resource := range info.Resources {
		resource.Resource.Id = 0
		resource.Resource.Speed = 0
		if resource.Resource.ContentId != nil {
			mapped, ok := content_ids[*resource.Resource.ContentId]
			if !ok {
				return nil, fmt.Errorf("归档资源缺少关联内容")
			}
			resource.Resource.ContentId = &mapped
		}
		if resource.Resource.Status != model.ResourceStatusFinished {
			task.Status = model.TaskStatusPaused
		}
		for index := range resource.Endpoints {
			resource.Endpoints[index].Id = 0
		}
	}
	var existing model.DownloadTask
	query := tx.Where("platform_id = ? AND unique_id = ? AND created_at = ?", task.PlatformId, task.UniqueID, task.CreatedAt)
	if task.UniqueID == "" {
		query = tx.Where("platform_id = ? AND source_url = ? AND name = ? AND created_at = ?", task.PlatformId, task.SourceURL, task.Name, task.CreatedAt)
	}
	find_err := query.First(&existing).Error
	if find_err == nil {
		task.Id = existing.Id
		for _, resource := range info.Resources {
			var existing_resource model.DownloadResource
			query := tx.Where("task_id = ? AND unique_id = ? AND created_at = ?", task.Id, resource.Resource.UniqueID, resource.Resource.CreatedAt)
			if resource.Resource.UniqueID == "" {
				query = tx.Where("task_id = ? AND name = ? AND created_at = ?", task.Id, resource.Resource.Name, resource.Resource.CreatedAt)
			}
			err := query.First(&existing_resource).Error
			if err == nil {
				resource.Resource.Id = existing_resource.Id
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}
	} else if !errors.Is(find_err, gorm.ErrRecordNotFound) {
		return nil, find_err
	}
	config_json, err := task_config_with_download_dir(task.ConfigJSON, download_dir)
	if err != nil {
		return nil, err
	}
	task.ConfigJSON = config_json
	return persist_download_task_graph(tx, info, &task, download_dir, parent_id, task.RelationType, now, true)
}

func reset_archive_text_tracks(content *model.Content) {
	content.Assets = nil
	for index := range content.TextTracks {
		track := &content.TextTracks[index]
		track.Id, track.ContentId = 0, content.Id
		for source_index := range track.Sources {
			source := &track.Sources[source_index]
			source.TrackId, source.AssetId, source.Asset.Id, source.Asset.ContentId = 0, 0, 0, content.Id
		}
	}
}

func reset_archive_detail(detail any, content_id string) {
	switch value := detail.(type) {
	case *model.ContentVideo:
		value.Id = content_id
		for index := range value.Variants {
			variant := &value.Variants[index]
			variant.AssetId, variant.VideoId, variant.Asset.Id, variant.Asset.ContentId = 0, content_id, 0, content_id
		}
	case *model.ContentNovel:
		value.Id = content_id
		for index := range value.Volumes {
			value.Volumes[index].Id, value.Volumes[index].NovelId = 0, content_id
		}
		for index := range value.Chapters {
			chapter := &value.Chapters[index]
			chapter.Id, chapter.NovelId, chapter.VolumeId, chapter.Assets = 0, content_id, nil, nil
		}
	case *model.ContentAlbum:
		value.Id = content_id
		for index := range value.Images {
			value.Images[index].Id, value.Images[index].AlbumId, value.Images[index].Assets = 0, content_id, nil
		}
	case *model.ContentConversation:
		value.Id = content_id
		for index := range value.Branches {
			value.Branches[index].Id, value.Branches[index].ConversationId = 0, content_id
		}
		for index := range value.Messages {
			message := &value.Messages[index]
			message.Id, message.ConversationId, message.Assets = 0, content_id, nil
			for part_index := range message.Parts {
				part := &message.Parts[part_index]
				part.Id, part.MessageId, part.ConversationId, part.Assets = 0, 0, content_id, nil
			}
		}
	case *model.ContentEpisode:
		value.Id = content_id
	case *model.ContentSeries:
		value.Id = content_id
	case *model.ContentAudio:
		value.Id = content_id
	case *model.ContentArticle:
		value.Id = content_id
	case *model.ContentLive:
		value.Id = content_id
	case *model.ContentPodcast:
		value.Id = content_id
	case *model.ContentDocument:
		value.Id = content_id
	case *model.ContentCourse:
		value.Id = content_id
	case *model.ContentComic:
		value.Id = content_id
	case *model.ContentPost:
		value.Id = content_id
	}
}
