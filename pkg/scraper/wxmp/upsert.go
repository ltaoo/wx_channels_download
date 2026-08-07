package wxmp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

const platformIDOfficialAccount = "wxmp"

// ArticleProfile is standardized public account article data, ready for inserting account, content, and download_task.
type ArticleProfile struct {
	ArticleID   string        `json:"article_id"`   // unique article identifier (mid_idx or short link ID)
	Title       string        `json:"title"`        // article title
	Description string        `json:"description"`  // article summary/description
	SourceURL   string        `json:"source_url"`   // original article link
	CoverURL    string        `json:"cover_url"`    // cover image URL
	ContentHTML string        `json:"content_html"` // article body HTML
	ContentSize int           `json:"content_size"` // body length
	PublishTime int64         `json:"publish_time"` // publish time (seconds timestamp)
	Author      ArticleAuthor `json:"author"`
}

// ArticleAuthor is article author information.
type ArticleAuthor struct {
	ExternalId string `json:"external_id"` // biz ID (UserName)
	Nickname   string `json:"nickname"`    // public account nickname
	AvatarURL  string `json:"avatar_url"`  // avatar URL
}

// ArticleToProfile converts a WechatOfficialArticle to a normalized ArticleProfile
func ArticleToProfile(article *WechatOfficialArticle, sourceURL string) (*ArticleProfile, error) {
	if article == nil {
		return nil, errors.New("article is nil")
	}

	articleID := ExtractArticleID(sourceURL)
	if articleID == "" {
		return nil, errors.New("无法从 URL 提取 article_id")
	}

	title := article.Title
	if title == "" && article.PageJSON != nil {
		title = article.PageJSON.Title
	}
	if title == "" {
		title = articleID
	}

	description := ""
	if article.PageJSON != nil {
		description = article.PageJSON.Desc
	}

	coverURL := ""
	if article.PageJSON != nil && article.PageJSON.CdnUrl != "" {
		coverURL = article.PageJSON.CdnUrl
	} else if len(article.Images) > 0 {
		coverURL = article.Images[0]
	}

	// author info
	authorExternalId := article.AuthorID
	authorNickname := firstNonEmpty(article.AuthorNickname, article.Creator)
	authorAvatar := article.AuthorAvatar
	if article.PageJSON != nil {
		authorExternalId = firstNonEmpty(authorExternalId, article.PageJSON.UserName)
		authorNickname = firstNonEmpty(authorNickname, article.PageJSON.NickName, article.PageJSON.Author)
		authorAvatar = firstNonEmpty(authorAvatar, article.PageJSON.RoundHeadImg, article.PageJSON.OriHeadImgUrl, article.PageJSON.HdHeadImg)
	}

	// publish time
	var publishTime int64
	if article.PageJSON != nil && int64(article.PageJSON.OriCreateTime) > 0 {
		publishTime = int64(article.PageJSON.OriCreateTime)
	}

	contentSize := article.ContentLength
	if contentSize == 0 && article.Content != "" {
		contentSize = len(article.Content)
	}

	return &ArticleProfile{
		ArticleID:   articleID,
		Title:       title,
		Description: description,
		SourceURL:   sourceURL,
		CoverURL:    coverURL,
		ContentHTML: article.Content,
		ContentSize: contentSize,
		PublishTime: publishTime,
		Author: ArticleAuthor{
			ExternalId: authorExternalId,
			Nickname:   authorNickname,
			AvatarURL:  authorAvatar,
		},
	}, nil
}

func (c *OfficialAccountClient) SetDB(db *gorm.DB) {
	c.db = db
}

// UpsertArticle creates or updates account + content + content_account from an ArticleProfile
func (c *OfficialAccountClient) UpsertArticle(profile *ArticleProfile) (*model.Content, error) {
	if c.db == nil {
		return nil, errors.New("db is nil")
	}
	if profile == nil {
		return nil, errors.New("profile is nil")
	}
	if strings.TrimSpace(profile.ArticleID) == "" {
		return nil, errors.New("missing article_id")
	}

	now := util.NowMillis()

	// 1. Upsert account
	var existingAccount model.Account
	if profile.Author.ExternalId != "" {
		acc := model.Account{
			Id:         platformIDOfficialAccount + ":" + profile.Author.ExternalId,
			PlatformId: platformIDOfficialAccount,
			ExternalId: profile.Author.ExternalId,
			Nickname:   profile.Author.Nickname,
			AvatarURL:  profile.Author.AvatarURL,
			Timestamps: model.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		if err := c.db.Where("platform_id = ? AND external_id = ?", platformIDOfficialAccount, acc.ExternalId).First(&existingAccount).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := c.db.Create(&acc).Error; err != nil {
					return nil, fmt.Errorf("创建 account 失败: %w", err)
				}
				existingAccount = acc
			} else {
				return nil, fmt.Errorf("查询 account 失败: %w", err)
			}
		} else {
			updates := map[string]any{"updated_at": now}
			if acc.Nickname != "" {
				updates["nickname"] = acc.Nickname
			}
			if acc.AvatarURL != "" {
				updates["avatar_url"] = acc.AvatarURL
			}
			if err := c.db.Model(&existingAccount).Updates(updates).Error; err != nil {
				return nil, fmt.Errorf("更新 account 失败: %w", err)
			}
		}
	}

	// 2. Upsert content
	var publishTimePtr *int64
	if profile.PublishTime > 0 {
		publishTimeMilli := profile.PublishTime * 1000
		publishTimePtr = &publishTimeMilli
	}

	var existing model.Content
	if err := c.db.Where("platform_id = ? AND external_id = ?", platformIDOfficialAccount, profile.ArticleID).First(&existing).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询 content 失败: %w", err)
		}
	}

	content := model.Content{
		Id:          platformIDOfficialAccount + ":" + profile.ArticleID,
		PlatformId:  platformIDOfficialAccount,
		Type:        "article",
		ExternalId:  profile.ArticleID,
		Title:       profile.Title,
		Description: profile.Description,
		SourceURL:   profile.SourceURL,
		URL:         profile.SourceURL,
		CoverURL:    profile.CoverURL,
		PublishTime: publishTimePtr,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if existing.Id == "" {
		if err := c.db.Create(&content).Error; err != nil {
			return nil, fmt.Errorf("创建 content 失败: %w", err)
		}
	} else {
		content.Id = existing.Id
		updates := map[string]any{
			"title":       content.Title,
			"description": content.Description,
			"source_url":  content.SourceURL,
			"url":         content.URL,
			"cover_url":   content.CoverURL,
			"updated_at":  now,
		}
		if publishTimePtr != nil {
			updates["publish_time"] = *publishTimePtr
		}
		if err := c.db.Model(&model.Content{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("更新 content 失败: %w", err)
		}
	}

	// 3. Link content_account
	if existingAccount.Id != "" {
		ac := model.ContentAccount{
			AccountId: existingAccount.Id,
			ContentId: content.Id,
			Role:      "owner",
			CreatedAt: now,
		}
		if err := c.db.FirstOrCreate(&ac, model.ContentAccount{AccountId: existingAccount.Id, ContentId: content.Id}).Error; err != nil {
			return nil, fmt.Errorf("创建 content_account 关联失败: %w", err)
		}
	}

	return &content, nil
}

// ArticleDownloadTaskOpts are optional parameters when creating a download_task.
type ArticleDownloadTaskOpts struct {
	TaskId     string // unique task identifier, auto-generated if empty
	Status     int    // download status; use model.TaskStatus* values
	Filepath   string // file save path
	OutputPath string // file output path
	Reason     string // download reason (e.g. "migrate", "manual", "batch")
	Error      string // error message
}

// UpsertArticleWithDownloadTask generates account, content, and download_task records from an ArticleProfile and links them
func (c *OfficialAccountClient) UpsertArticleWithDownloadTask(profile *ArticleProfile, opts *ArticleDownloadTaskOpts) (*model.Content, *model.DownloadTask, error) {
	if c.db == nil {
		return nil, nil, errors.New("db is nil")
	}
	if profile == nil {
		return nil, nil, errors.New("profile is nil")
	}
	if opts == nil {
		opts = &ArticleDownloadTaskOpts{}
	}

	// 1. Upsert account + content
	content, err := c.UpsertArticle(profile)
	if err != nil {
		return nil, nil, fmt.Errorf("upsert article 失败: %w", err)
	}

	// 2. Prepare download_task
	now := util.NowMillis()

	taskId := opts.TaskId
	if taskId == "" {
		taskId = fmt.Sprintf("officialaccount_%s_%d", profile.ArticleID, now)
	}

	metadataBytes, _ := json.Marshal(map[string]any{
		"platform":   platformIDOfficialAccount,
		"article_id": profile.ArticleID,
		"source_url": profile.SourceURL,
	})
	configBytes, _ := json.Marshal(map[string]any{
		"filepath":    opts.Filepath,
		"output_path": opts.OutputPath,
		"reason":      opts.Reason,
	})

	// 3. Find or create download_task
	var rec model.DownloadTask
	err = c.db.Where("platform_id = ? AND unique_id = ?", platformIDOfficialAccount, taskId).First(&rec).Error

	if err == nil {
		// already exists, update
		updates := map[string]any{
			"content_id":    content.Id,
			"name":          content.Title,
			"source_url":    profile.SourceURL,
			"cover_url":     content.CoverURL,
			"config_json":   string(configBytes),
			"metadata_json": string(metadataBytes),
			"error_message": opts.Error,
			"updated_at":    now,
		}
		updates["status"] = opts.Status
		if err := c.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(updates).Error; err != nil {
			return content, nil, fmt.Errorf("更新 download_task 失败: %w", err)
		}
		rec.Status = opts.Status
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// create new
		contentID := content.Id
		rec = model.DownloadTask{
			ContentId:    &contentID,
			Name:         content.Title,
			PlatformId:   platformIDOfficialAccount,
			UniqueID:     taskId,
			Status:       opts.Status,
			SourceURL:    profile.SourceURL,
			CoverURL:     content.CoverURL,
			ConfigJSON:   string(configBytes),
			MetadataJSON: string(metadataBytes),
			ErrorMessage: opts.Error,
			Timestamps: model.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		if err := c.db.Create(&rec).Error; err != nil {
			return content, nil, fmt.Errorf("创建 download_task 失败: %w", err)
		}
	} else {
		return content, nil, fmt.Errorf("查询 download_task 失败: %w", err)
	}

	// 4. Update content download status
	downloadPath := opts.OutputPath
	if downloadPath == "" {
		downloadPath = opts.Filepath
	}
	if err := c.db.Model(&model.Content{}).Where("id = ?", content.Id).Updates(map[string]any{
		"download_status": rec.Status,
		"download_path":   downloadPath,
		"updated_at":      now,
	}).Error; err != nil {
		return content, &rec, fmt.Errorf("更新 content 下载状态失败: %w", err)
	}

	return content, &rec, nil
}
