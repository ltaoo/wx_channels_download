package officialaccount

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/util"
)

const platformIDOfficialAccount = "wx_official_account"

// ArticleProfile 公众号文章的标准化数据，可直接用于插入 account、content、download_task
type ArticleProfile struct {
	ArticleID   string `json:"article_id"`   // 文章唯一标识（mid_idx 或短链 ID）
	Title       string `json:"title"`        // 文章标题
	Description string `json:"description"`  // 文章摘要/描述
	SourceURL   string `json:"source_url"`   // 文章原始链接
	CoverURL    string `json:"cover_url"`    // 封面图 URL
	ContentHTML string `json:"content_html"` // 文章正文 HTML
	ContentSize int    `json:"content_size"` // 正文长度
	PublishTime int64  `json:"publish_time"` // 发布时间（秒级时间戳）
	Author      ArticleAuthor `json:"author"`
}

// ArticleAuthor 文章作者信息
type ArticleAuthor struct {
	ExternalId string `json:"external_id"` // biz ID (UserName)
	Nickname   string `json:"nickname"`    // 公众号昵称
	AvatarURL  string `json:"avatar_url"`  // 头像 URL
}

// ArticleToProfile 从 WechatOfficialArticle 转换为标准化的 ArticleProfile
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

	// 作者信息
	authorExternalId := article.AuthorID
	authorNickname := firstNonEmpty(article.AuthorNickname, article.Creator)
	authorAvatar := article.AuthorAvatar
	if article.PageJSON != nil {
		authorExternalId = firstNonEmpty(authorExternalId, article.PageJSON.UserName)
		authorNickname = firstNonEmpty(authorNickname, article.PageJSON.NickName, article.PageJSON.Author)
		authorAvatar = firstNonEmpty(authorAvatar, article.PageJSON.RoundHeadImg, article.PageJSON.OriHeadImgUrl, article.PageJSON.HdHeadImg)
	}

	// 发布时间
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
		PublishTime:  publishTime,
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

// UpsertArticle 从 ArticleProfile 创建/更新 account + content + content_account
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
			PlatformId: platformIDOfficialAccount,
			ExternalId: profile.Author.ExternalId,
			Username:   profile.Author.ExternalId,
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
		PlatformId:  platformIDOfficialAccount,
		ContentType: "article",
		ExternalId:  profile.ArticleID,
		Title:       profile.Title,
		Description: profile.Description,
		SourceURL:   profile.SourceURL,
		URL:         profile.SourceURL,
		CoverURL:    profile.CoverURL,
		Size:        int64(profile.ContentSize),
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
			"size":        content.Size,
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

// ArticleDownloadTaskOpts 创建 download_task 时的可选参数
type ArticleDownloadTaskOpts struct {
	TaskId     string // 任务唯一标识，为空则自动生成
	Status     int    // 下载状态: 0=ready, 1=running, 2=pause, 3=wait, 4=done, 5=error
	Filepath   string // 文件保存路径
	OutputPath string // 文件输出路径
	Reason     string // 下载原因（如 "migrate", "manual", "batch"）
	Error      string // 错误信息
}

// UpsertArticleWithDownloadTask 从 ArticleProfile 生成 account、content、download_task 三种记录并关联
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

	// 2. 准备 download_task
	now := util.NowMillis()

	taskId := opts.TaskId
	if taskId == "" {
		taskId = fmt.Sprintf("officialaccount_%s_%d", profile.ArticleID, now)
	}

	taskURL := "officialaccount://" + profile.SourceURL

	size := int64(profile.ContentSize)
	if size <= 0 {
		size = content.Size
	}

	downloaded := int64(0)
	if opts.Status == 4 && size > 0 {
		downloaded = size
	}

	meta2Bytes, _ := json.Marshal(map[string]any{
		"platform":   platformIDOfficialAccount,
		"article_id": profile.ArticleID,
		"source_url": profile.SourceURL,
	})

	// 3. 查找或创建 download_task
	var rec model.DownloadTask
	err = c.db.Where("task_id = ?", taskId).First(&rec).Error

	if err == nil {
		// 已存在，更新
		updates := map[string]any{
			"url":         taskURL,
			"external_id": profile.ArticleID,
			"title":       content.Title,
			"cover_url":   content.CoverURL,
			"metadata2":   string(meta2Bytes),
			"updated_at":  now,
		}
		if opts.Status > 0 {
			updates["status"] = opts.Status
		}
		if size > 0 {
			updates["size"] = size
		}
		if downloaded > 0 {
			updates["downloaded"] = downloaded
		}
		if opts.Filepath != "" {
			updates["filepath"] = opts.Filepath
		}
		if opts.OutputPath != "" {
			updates["output_path"] = opts.OutputPath
		}
		if opts.Reason != "" {
			updates["reason"] = opts.Reason
		}
		if err := c.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(updates).Error; err != nil {
			return content, nil, fmt.Errorf("更新 download_task 失败: %w", err)
		}
		rec.Status = opts.Status
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// 新建
		rec = model.DownloadTask{
			TaskId:     taskId,
			Status:     opts.Status,
			Protocol:   "officialaccount",
			URL:        taskURL,
			ExternalId: profile.ArticleID,
			Title:      content.Title,
			CoverURL:   content.CoverURL,
			Size:       size,
			Downloaded: downloaded,
			Filepath:   opts.Filepath,
			OutputPath: opts.OutputPath,
			Reason:     opts.Reason,
			Error:      opts.Error,
			Metadata2:  string(meta2Bytes),
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

	// 4. 关联 content -> download_task
	downloadPath := rec.OutputPath
	if downloadPath == "" {
		downloadPath = rec.Filepath
	}
	if err := c.db.Model(&model.Content{}).Where("id = ?", content.Id).Updates(map[string]any{
		"download_task_id": rec.Id,
		"download_status":  rec.Status,
		"download_path":    downloadPath,
		"updated_at":       now,
	}).Error; err != nil {
		return content, &rec, fmt.Errorf("关联 content 和 download_task 失败: %w", err)
	}

	return content, &rec, nil
}

