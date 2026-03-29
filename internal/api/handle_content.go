package api

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
)

func (c *APIClient) handleFetchContentList(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}

	parseInt := func(s string) (int, bool) {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, false
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	parseInt64 := func(s string) (int64, bool) {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, false
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}

	platformId := strings.TrimSpace(ctx.Query("platform_id"))
	contentType := strings.TrimSpace(ctx.Query("content_type"))
	keyword := strings.TrimSpace(ctx.Query("keyword"))

	accountId, hasAccountId := parseInt(ctx.Query("account_id"))
	downloadStatus, hasDownloadStatus := parseInt(ctx.Query("download_status"))
	unread, hasUnread := parseInt(ctx.Query("unread"))
	validated, hasValidated := parseInt(ctx.Query("validated"))

	startAt, hasStartAt := parseInt64(ctx.Query("start_at"))
	endAt, hasEndAt := parseInt64(ctx.Query("end_at"))

	page := 1
	pageSize := 20
	if v, ok := parseInt(ctx.Query("page")); ok && v > 0 {
		page = v
	}
	if v, ok := parseInt(ctx.Query("page_size")); ok && v > 0 {
		pageSize = v
	}
	offset := (page - 1) * pageSize
	limit := pageSize
	if v, ok := parseInt(ctx.Query("limit")); ok && v > 0 {
		limit = v
	}
	if v, ok := parseInt(ctx.Query("offset")); ok && v >= 0 {
		offset = v
	}

	type contentWithAccount struct {
		model.Content
		Accounts     []model.Account     `json:"accounts"`
		DownloadTask *model.DownloadTask `json:"download_task"`
	}

	if hasAccountId && accountId > 0 {
		countDb := c.db.DB().Table("content_account").
			Joins("JOIN content ON content.id = content_account.content_id").
			Where("content_account.account_id = ?", accountId).
			Where("content.deleted_at IS NULL")

		if platformId != "" {
			countDb = countDb.Where("content.platform_id = ?", platformId)
		}
		if contentType != "" {
			countDb = countDb.Where("content.content_type = ?", contentType)
		}
		if hasDownloadStatus {
			countDb = countDb.Where("content.download_status = ?", downloadStatus)
		}
		if hasUnread {
			countDb = countDb.Where("content.unread = ?", unread)
		}
		if hasValidated {
			countDb = countDb.Where("content.validated = ?", validated)
		}
		if hasStartAt {
			countDb = countDb.Where("content.created_at >= ?", startAt)
		}
		if hasEndAt {
			countDb = countDb.Where("content.created_at <= ?", endAt)
		}
		if keyword != "" {
			like := "%" + keyword + "%"
			countDb = countDb.Where("(content.title LIKE ? OR content.description LIKE ?)", like, like)
		}

		var total int64
		if err := countDb.Count(&total).Error; err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}

		var contentIDs []int
		if err := countDb.Select("content.id").
			Order("content.publish_time DESC, content.id DESC").
			Limit(limit).
			Offset(offset).
			Scan(&contentIDs).Error; err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}

		var contents []model.Content
		if len(contentIDs) > 0 {
			if err := c.db.DB().Where("id IN ?", contentIDs).Find(&contents).Error; err != nil {
				result.Err(ctx, 500, err.Error())
				return
			}
		}

		contentByID := make(map[int]model.Content, len(contents))
		for _, ct := range contents {
			contentByID[ct.Id] = ct
		}

		accountsMap := make(map[int][]model.Account)
		if len(contentIDs) > 0 {
			type accountRow struct {
				ContentId int `gorm:"column:content_id" json:"content_id"`
				model.Account
			}
			var rows []accountRow
			_ = c.db.DB().Table("content_account").
				Select("content_account.content_id, account.*").
				Joins("JOIN account ON account.id = content_account.account_id").
				Where("content_account.content_id IN ?", contentIDs).
				Find(&rows).Error
			for _, r := range rows {
				accountsMap[r.ContentId] = append(accountsMap[r.ContentId], r.Account)
			}
		}

		taskIDs := make([]int, 0, len(contents))
		for _, ct := range contents {
			if ct.DownloadTaskId != nil && *ct.DownloadTaskId > 0 {
				taskIDs = append(taskIDs, *ct.DownloadTaskId)
			}
		}
		taskMap := make(map[int]model.DownloadTask)
		if len(taskIDs) > 0 {
			var tasks []model.DownloadTask
			_ = c.db.DB().Where("id IN ?", taskIDs).Find(&tasks).Error
			for _, t := range tasks {
				taskMap[t.Id] = t
			}
		}

		items := make([]contentWithAccount, 0, len(contentIDs))
		for _, id := range contentIDs {
			ct, ok := contentByID[id]
			if !ok {
				continue
			}
			var downloadTask *model.DownloadTask
			if ct.DownloadTaskId != nil && *ct.DownloadTaskId > 0 {
				if t, ok := taskMap[*ct.DownloadTaskId]; ok {
					task := t
					downloadTask = &task
				}
			}
			items = append(items, contentWithAccount{
				Content:      ct,
				Accounts:     accountsMap[id],
				DownloadTask: downloadTask,
			})
		}

		result.Ok(ctx, gin.H{"list": items, "page": page, "page_size": pageSize, "total": total})
		return
	}

	countDb := c.db.DB().Model(&model.Content{}).Where("deleted_at IS NULL")
	if platformId != "" {
		countDb = countDb.Where("platform_id = ?", platformId)
	}
	if contentType != "" {
		countDb = countDb.Where("content_type = ?", contentType)
	}
	if hasDownloadStatus {
		countDb = countDb.Where("download_status = ?", downloadStatus)
	}
	if hasUnread {
		countDb = countDb.Where("unread = ?", unread)
	}
	if hasValidated {
		countDb = countDb.Where("validated = ?", validated)
	}
	if hasStartAt {
		countDb = countDb.Where("created_at >= ?", startAt)
	}
	if hasEndAt {
		countDb = countDb.Where("created_at <= ?", endAt)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		countDb = countDb.Where("(title LIKE ? OR description LIKE ?)", like, like)
	}

	var total int64
	if err := countDb.Count(&total).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	var contents []model.Content
	if err := countDb.Order("publish_time DESC, id DESC").Limit(limit).Offset(offset).Find(&contents).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	contentIDs := make([]int, 0, len(contents))
	for _, ct := range contents {
		contentIDs = append(contentIDs, ct.Id)
	}

	accountsMap := make(map[int][]model.Account)
	if len(contentIDs) > 0 {
		type accountRow struct {
			ContentId int `gorm:"column:content_id" json:"content_id"`
			model.Account
		}
		var rows []accountRow
		_ = c.db.DB().Table("content_account").
			Select("content_account.content_id, account.*").
			Joins("JOIN account ON account.id = content_account.account_id").
			Where("content_account.content_id IN ?", contentIDs).
			Find(&rows).Error
		for _, r := range rows {
			accountsMap[r.ContentId] = append(accountsMap[r.ContentId], r.Account)
		}
	}

	taskIDs := make([]int, 0, len(contents))
	for _, ct := range contents {
		if ct.DownloadTaskId != nil && *ct.DownloadTaskId > 0 {
			taskIDs = append(taskIDs, *ct.DownloadTaskId)
		}
	}
	taskMap := make(map[int]model.DownloadTask)
	if len(taskIDs) > 0 {
		var tasks []model.DownloadTask
		_ = c.db.DB().Where("id IN ?", taskIDs).Find(&tasks).Error
		for _, t := range tasks {
			taskMap[t.Id] = t
		}
	}

	items := make([]contentWithAccount, 0, len(contents))
	for _, ct := range contents {
		var downloadTask *model.DownloadTask
		if ct.DownloadTaskId != nil && *ct.DownloadTaskId > 0 {
			if t, ok := taskMap[*ct.DownloadTaskId]; ok {
				task := t
				downloadTask = &task
			}
		}
		items = append(items, contentWithAccount{
			Content:      ct,
			Accounts:     accountsMap[ct.Id],
			DownloadTask: downloadTask,
		})
	}
	result.Ok(ctx, gin.H{"list": items, "page": page, "page_size": pageSize, "total": total})
}
